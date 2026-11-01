#!/usr/bin/env python3
"""Build and verify a downloadable nonmobile Space release candidate.

Run only after implementation is complete. Never deploys, pushes, signs with
an unrequested key, changes live services, or writes credentials into bundles.
"""
import argparse
import datetime as dt
import gzip
import hashlib
import json
import os
import platform as host_platform
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tarfile
import tempfile
import zipfile
from urllib.parse import unquote, urlsplit

ROOT = Path(__file__).resolve().parents[2]
SERVER_PACKAGE = 'github.com/himasifunej/himasif-space/internal/server'


class ReleaseError(Exception):
    pass


def require(condition, message):
    if not condition:
        raise ReleaseError(message)


def sha256(path):
    result = hashlib.sha256()
    with Path(path).open('rb') as source:
        for chunk in iter(lambda: source.read(1024 * 1024), b''):
            result.update(chunk)
    return result.hexdigest()


def redact(text):
    # Test logs belong to the package, but still never retain session env secrets.
    for name, value in os.environ.items():
        if value and (any(word in name for word in ('PASSWORD', 'TOKEN', 'SECRET', 'PRIVATE_KEY', 'DATABASE_URL'))):
            text = text.replace(value, '[redacted]')
            if 'DATABASE_URL' in name:
                try:
                    password = urlsplit(value).password
                    if password:
                        text = text.replace(unquote(password), '[redacted]')
                except ValueError:
                    pass
    return re.sub(r'(?i)(postgres(?:ql)?://[^\s:/]+:)[^@\s]+@', r'\1[redacted]@', text)


def execute(args, cwd, env, logs, name):
    print(name, flush=True)
    path = logs / (name + '.log')
    with path.open('w', encoding='utf-8') as output:
        process = subprocess.Popen([str(arg) for arg in args], cwd=cwd, env=env,
                                   stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True,
                                   errors='replace', bufsize=1)
        try:
            for line in process.stdout:
                output.write(redact(line))
            result = process.wait()
        finally:
            if process.poll() is None:
                process.terminate()
                process.wait()
    require(result == 0, f'{name} failed; inspect the redacted log at {path}')
    return path


def source_files():
    raw = subprocess.check_output(['git', 'ls-files', '-co', '--exclude-standard', '-z'], cwd=ROOT)
    selected = []
    for name in sorted(set(os.fsdecode(part) for part in raw.split(b'\0') if part)):
        relative = Path(name)
        require(not relative.is_absolute() and '..' not in relative.parts, 'unsafe source path')
        if relative.name == '.env.example':
            pass
        elif (relative.name == '.env' or relative.name.startswith('.env.') or
              relative.suffix.lower() in ('.env', '.key', '.pem', '.pfx', '.p12', '.age', '.dump')):
            continue
        if any(part in ('node_modules', '.git', 'space-data', 'build', 'dist', '__pycache__') for part in relative.parts):
            continue
        path = ROOT / relative
        require(not path.is_symlink(), 'source snapshot does not follow symlinks')
        if path.is_file():
            selected.append(relative)
    return selected


def snapshot(destination):
    files = source_files()
    for relative in files:
        target = destination / relative
        target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(ROOT / relative, target)
    return files


def source_manifest(source, files):
    records = {path.as_posix(): sha256(source / path) for path in sorted(set(files))}
    digest = hashlib.sha256(json.dumps(records, sort_keys=True, separators=(',', ':')).encode()).hexdigest()
    return digest, records


def zip_tree(source, target, prefix=''):
    with zipfile.ZipFile(target, 'x', compression=zipfile.ZIP_DEFLATED, compresslevel=6) as archive:
        for path in sorted(source.rglob('*')):
            require(not path.is_symlink(), 'release does not follow artifact symlinks')
            if path.is_file():
                archive.write(path, str(Path(prefix) / path.relative_to(source)))


def tar_tree(source, target, prefix):
    with tarfile.open(target, 'x:gz') as archive:
        for path in sorted(source.rglob('*')):
            require(not path.is_symlink(), 'release does not follow artifact symlinks')
            if path.is_file():
                archive.add(path, arcname=str(Path(prefix) / path.relative_to(source)), recursive=False)


def go_test_summary(log):
    summary = {'passed': 0, 'failed': 0, 'skipped': []}
    for line in log.read_text(encoding='utf-8').splitlines():
        try:
            event = json.loads(line)
        except ValueError:
            continue
        if event.get('Test'):
            if event.get('Action') == 'pass':
                summary['passed'] += 1
            elif event.get('Action') == 'fail':
                summary['failed'] += 1
            elif event.get('Action') == 'skip':
                summary['skipped'].append({'package': event.get('Package'), 'test': event['Test']})
    require(summary['passed'] > 0 and summary['failed'] == 0, 'no successful Go test execution was recorded')
    require(not any(item['package'].endswith('/internal/server') for item in summary['skipped']),
            'server tests were skipped; database verification is required for a release candidate')
    return summary


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--api-url', required=True, help='desktop API origin; web uses its own origin')
    parser.add_argument('--output', default=str(ROOT / 'dist'))
    parser.add_argument('--go', default=shutil.which('go'))
    parser.add_argument('--resume-checks', help='reuse passed checks only when all source except this release driver is byte-identical')
    parser.add_argument('--linux-desktop', choices=('amd64', 'arm64'))
    args = parser.parse_args()
    uri = urlsplit(args.api_url)
    require(uri.scheme in ('http', 'https') and uri.hostname and not uri.username and not uri.password
            and not uri.query and not uri.fragment and not any(c in args.api_url for c in " \t\r\n'\"`"),
            'provide an API URL without credentials, query parameters or linker metacharacters')
    if uri.scheme == 'http':
        require(uri.hostname in ('localhost', '127.0.0.1', '::1'), 'nonlocal desktop API URLs must use HTTPS')
    require(args.go and Path(args.go).is_file(), 'provide --go with the Go compiler path')
    for tool in ('git', 'node', 'npm', 'python3', 'makensis', 'age', 'age-keygen', 'pg_dump', 'pg_restore', 'psql', 'createdb'):
        require(shutil.which(tool), f'required build tool is missing: {tool}')
    test_uri = urlsplit(os.environ.get('SPACE_TEST_DATABASE_URL', ''))
    require(test_uri.scheme in ('postgres', 'postgresql') and unquote(test_uri.path[1:]).startswith('space_test_'),
            'SPACE_TEST_DATABASE_URL must name an explicitly isolated space_test_ database')
    for guide in ('distribution.md', 'pilot-cutover.md'):
        require((ROOT / 'docs/operations' / guide).is_file(), 'required release guide is missing: ' + guide)
    destination = Path(args.output).resolve()
    destination.mkdir(mode=0o700, parents=True, exist_ok=True)
    version = json.loads((ROOT / 'app/mygo.json').read_text())['version']
    require(re.fullmatch(r'[0-9]+\.[0-9]+\.[0-9]+(?:-[A-Za-z0-9.-]+)?', version), 'invalid release version')
    revision = subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=ROOT, text=True).strip()
    dirty = bool(subprocess.check_output(['git', 'status', '--porcelain'], cwd=ROOT))
    built_at = dt.datetime.now(dt.timezone.utc).isoformat()
    run_id = dt.datetime.now(dt.timezone.utc).strftime('%Y%m%dT%H%M%SZ')
    work = destination / ('candidate-' + version + '-' + run_id)
    work.mkdir(mode=0o700)
    logs = work / 'verification-logs'
    logs.mkdir(mode=0o700)
    env = {key: value for key, value in os.environ.items()
           if not key.startswith(('SPACE_', 'MYGO_', 'AWS_', 'GOOGLE_', 'TRELLO_', 'SMTP_'))}
    env.update(SPACE_TEST_DATABASE_URL=os.environ['SPACE_TEST_DATABASE_URL'],
               GOMAXPROCS='1', GOMEMLIMIT='256MiB', GOGC='50', GOFLAGS='-buildvcs=false -p=1',
               NODE_OPTIONS='--max-old-space-size=384', npm_config_jobs='1',
               PATH=str(Path(args.go).resolve().parent) + os.pathsep + os.environ.get('PATH', ''))
    report = {'version': version, 'revision': revision, 'dirtySource': dirty, 'builtAt': built_at,
              'desktopAPIURL': args.api_url, 'status': 'incomplete', 'checks': {},
              'productionReady': False, 'mobileIncluded': False,
              'requiresAcceptance': ['Windows tray/toast/autostart/device behavior',
                                     'real WA two-session and receipt acceptance',
                                     'real broadcast/source mapping and pilot sign-off',
                                     'production-scale restore/service recovery and cutover/rollback approval']}
    report_path = work / 'verification.json'
    try:
        with tempfile.TemporaryDirectory(prefix='space-release-source-') as temporary:
            source = Path(temporary)
            files = snapshot(source)
            bridge = source / 'bridge'
            if not (bridge / 'package-lock.json').exists():
                execute(['npm', 'install', '--package-lock-only', '--ignore-scripts'], bridge, env, logs, 'bridge-lock')
                files.append(Path('bridge/package-lock.json'))
            execute(['npm', 'ci', '--omit=dev'], bridge, env, logs, 'bridge-dependencies')
            digest, manifest = source_manifest(source, files)
            report['sourceDigest'] = digest
            (work / 'source-manifest.json').write_text(json.dumps(manifest, indent=2) + '\n')
            execute([args.go, 'version'], source, env, logs, 'go-version')
            execute(['node', '--version'], source, env, logs, 'node-version')
            execute(['npm', '--version'], source, env, logs, 'npm-version')
            execute(['pg_restore', '--version'], source, env, logs, 'postgres-version')
            execute(['makensis', '-VERSION'], source, env, logs, 'nsis-version')
            if args.resume_checks:
                previous = Path(args.resume_checks).resolve()
                old_report = json.loads((previous / 'verification.json').read_text())
                old_manifest = json.loads((previous / 'source-manifest.json').read_text())
                old_digest = hashlib.sha256(json.dumps(old_manifest, sort_keys=True, separators=(',', ':')).encode()).hexdigest()
                require(old_digest == old_report['sourceDigest'], 'previous source manifest digest differs')
                allowed = {'scripts/release/build_release.py'}
                differences = {name for name in set(manifest) | set(old_manifest) if manifest.get(name) != old_manifest.get(name)}
                require(differences <= allowed, 'source changed beyond the release driver; run the full checks again')
                require(old_report['revision'] == revision and old_report['version'] == version, 'previous revision/version differ')
                old_logs = previous / 'verification-logs'
                for name in ('go-version', 'node-version', 'npm-version', 'postgres-version', 'nsis-version'):
                    require((logs / (name + '.log')).read_text() == (old_logs / (name + '.log')).read_text(), 'toolchain changed; full checks required')
                for name in ('goVet', 'windows-vet', 'web-vet'):
                    require(old_report['checks'].get(name) == 'passed', 'previous check did not pass: ' + name)
                require(old_report['checks'].get('bridgeTests', {}).get('executed', 0) > 0, 'no previous bridge tests')
                reused_go = go_test_summary(old_logs / 'go-tests.log')
                require(reused_go == old_report['checks']['goTests'], 'previous Go test report differs from logs')
                for name in ('go-vet', 'go-tests', 'windows-vet', 'web-vet', 'bridge-tests'):
                    shutil.copy2(old_logs / (name + '.log'), logs / (name + '.log'))
                report['checks'].update({name: old_report['checks'][name] for name in ('goVet', 'goTests', 'windows-vet', 'web-vet', 'bridgeTests')})
                report['verificationReuse'] = {'sourceDigest': old_digest, 'unchangedExcept': sorted(differences), 'toolchainsMatch': True}
                print('verified-reuse-of-unchanged-application-checks', flush=True)
            else:
                execute([args.go, 'vet', './...'], source, env, logs, 'go-vet')
                report['checks']['goVet'] = 'passed'
                log = execute([args.go, 'test', '-json', '-count=1', '-timeout=30m', './...'], source, env, logs, 'go-tests')
                report['checks']['goTests'] = go_test_summary(log)
                for name, platform in [('windows-vet', {'GOOS': 'windows', 'GOARCH': 'amd64'}),
                                       ('web-vet', {'GOOS': 'js', 'GOARCH': 'wasm'})]:
                    execute([args.go, 'vet', './app', './internal/appui', './internal/client', './internal/api'],
                            source, dict(env, **platform), logs, name)
                    report['checks'][name] = 'passed'
                node_log = execute(['node', '--test', '--test-reporter=tap'], bridge, env, logs, 'bridge-tests')
                match = re.search(r'^# tests (\d+)$', node_log.read_text(), flags=re.MULTILINE)
                require(match and int(match.group(1)) > 0, 'bridge test suite did not execute any tests')
                report['checks']['bridgeTests'] = {'executed': int(match.group(1))}
            raw = work / 'raw'
            raw.mkdir()
            for arch in ('amd64', 'arm64'):
                target = raw / ('server-linux-' + arch)
                target.mkdir()
                flags = ' '.join(('-s -w', '-X ' + SERVER_PACKAGE + '.Version=' + version,
                                  '-X ' + SERVER_PACKAGE + '.SourceRevision=' + revision,
                                  '-X ' + SERVER_PACKAGE + '.SourceDigest=' + digest,
                                  '-X ' + SERVER_PACKAGE + '.BuiltAt=' + built_at))
                execute([args.go, 'build', '-trimpath', '-ldflags', flags, '-o', target / 'space-server', './cmd/space-server'],
                        source, dict(env, GOOS='linux', GOARCH=arch, CGO_ENABLED='0'), logs, 'server-linux-' + arch)
                shutil.copytree(source / 'scripts/ops', target / 'scripts/ops', ignore=shutil.ignore_patterns('__pycache__', 'test_*.py'))
                shutil.copytree(source / 'docs/operations', target / 'docs/operations')
                shutil.copytree(bridge, target / 'bridge', ignore=shutil.ignore_patterns('node_modules', 'state', '*.env', '.env', 'test', 'tests'))
                shutil.copy2(source / '.env.example', target / 'server.env.example')
            host_arch = {'aarch64': 'arm64', 'arm64': 'arm64', 'x86_64': 'amd64', 'amd64': 'amd64'}.get(host_platform.machine())
            require(sys.platform == 'linux' and host_arch, 'restore verification requires a supported Linux build host')
            ops_env = dict(env, SPACE_OPS_TEST_SERVER_BIN=str(raw / ('server-linux-' + host_arch) / 'space-server'))
            ops_log = execute(['python3', '-m', 'unittest', 'discover', '-s', 'scripts/ops', '-p', 'test_*.py', '-v'],
                              source, ops_env, logs, 'operations-tests')
            match = re.search(r'Ran (\d+) tests?', ops_log.read_text())
            require(match and int(match.group(1)) > 0 and 'skipped=' not in ops_log.read_text(), 'operations tests were empty or skipped; a real isolated restore drill is required')
            report['checks']['operationsTests'] = {'executed': int(match.group(1))}
            installer_log = execute(['python3', '-m', 'unittest', 'discover', '-s', 'scripts/release', '-p', 'test_*.py', '-v'],
                                    source, env, logs, 'installer-tooling-tests')
            require('OK' in installer_log.read_text() and 'Ran 0 tests' not in installer_log.read_text(),
                    'installer tooling checks did not run successfully')
            report['checks']['installerToolingTests'] = 'passed'
            native_env = dict(env, MYGO_LDFLAGS='-X main.defaultAPIURL=' + args.api_url + ' -X main.version=' + version)
            compiler = shutil.which('makensis', path=env['PATH'])
            installer_tools = work / 'installer-tools'
            installer_tools.mkdir()
            wrapper = installer_tools / 'makensis'
            wrapper.write_text('#!' + sys.executable + '\nimport runpy\nrunpy.run_path(' +
                               repr(str(source / 'scripts/release/windows_installer.py')) + ", run_name='__main__')\n")
            wrapper.chmod(0o700)
            native_env.update(PATH=str(installer_tools) + os.pathsep + env['PATH'],
                              SPACE_NSIS_COMPILER=compiler,
                              SPACE_NSIS_EVIDENCE=str(logs / 'windows-installer.nsi'))
            platforms = 'windows/amd64,web'
            if args.linux_desktop:
                platforms += ',linux/' + args.linux_desktop
            execute([args.go, 'tool', 'mygo', 'build', '-platform', platforms, '-o', str(raw / 'clients'), './app'],
                    source, native_env, logs, 'desktop-and-web-build')
            for wasm in (raw / 'clients/web').glob('*.wasm'):
                with wasm.open('rb') as incoming, (wasm.with_suffix(wasm.suffix + '.gz')).open('xb') as outgoing:
                    with gzip.GzipFile(fileobj=outgoing, mode='wb', mtime=0, compresslevel=9) as compressed:
                        shutil.copyfileobj(incoming, compressed)
            report['checks']['serverBuilds'] = ['linux/amd64', 'linux/arm64']
            report['checks']['clientBuilds'] = platforms.split(',')
            require(list((raw / 'clients/windows-amd64').glob('*setup*.exe')), 'Windows installer was not built')
            source_export = work / 'source'
            source_export.mkdir()
            for relative in files:
                target = source_export / relative
                target.parent.mkdir(parents=True, exist_ok=True)
                shutil.copy2(source / relative, target)
            packages = work / 'packages'
            packages.mkdir()
            zip_tree(raw / 'clients/windows-amd64', packages / ('space-' + version + '-windows-amd64.zip'))
            zip_tree(raw / 'clients/web', packages / ('space-' + version + '-web.zip'))
            for arch in ('amd64', 'arm64'):
                tar_tree(raw / ('server-linux-' + arch), packages / ('space-' + version + '-server-linux-' + arch + '.tar.gz'), 'space-' + version)
            if args.linux_desktop:
                tar_tree(raw / ('clients/linux-' + args.linux_desktop), packages / ('space-' + version + '-desktop-linux-' + args.linux_desktop + '.tar.gz'), 'space-' + version)
            zip_tree(source_export, packages / ('space-' + version + '-source.zip'))
            report['status'] = 'candidate-built-checks-passed-acceptance-pending'
            report['artifacts'] = {path.name: {'sha256': sha256(path), 'bytes': path.stat().st_size}
                                   for path in packages.iterdir() if path.is_file()}
    finally:
        report_path.write_text(json.dumps(report, ensure_ascii=False, indent=2) + '\n')
    zip_tree(logs, packages / 'verification-logs.zip')
    shutil.copy2(report_path, packages / 'verification.json')
    shutil.copy2(work / 'source-manifest.json', packages / 'source-manifest.json')
    for name in ('distribution.md', 'pilot-cutover.md'):
        document = source_export / 'docs/operations' / name
        require(document.is_file(), 'required release guide is missing: ' + name)
        shutil.copy2(document, packages / name)
    with (packages / 'SHA256SUMS').open('x', encoding='utf-8') as output:
        for path in sorted(packages.iterdir()):
            if path.is_file() and path.name != 'SHA256SUMS':
                output.write(sha256(path) + '  ' + path.name + '\n')
    full = destination / ('space-' + version + '-full-' + run_id + '.zip')
    zip_tree(packages, full)
    full.with_suffix(full.suffix + '.sha256').write_text(sha256(full) + '  ' + full.name + '\n')
    print(json.dumps({'download': str(full), 'sha256': sha256(full),
                      'verification': str(report_path), 'productionReady': False}))


if __name__ == '__main__':
    try:
        main()
    except (ReleaseError, OSError, ValueError, subprocess.CalledProcessError) as error:
        message = str(error) if isinstance(error, ReleaseError) else type(error).__name__
        print('release: ' + message, file=sys.stderr)
        sys.exit(1)

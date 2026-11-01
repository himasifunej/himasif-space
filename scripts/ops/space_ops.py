#!/usr/bin/env python3
"""Space backup/isolated restore. Python stdlib + PostgreSQL tools + age.

No shell evaluation, no passwords in command arguments, no automatic restore
promotion, and no retention deletion unless prune --apply is requested.
"""
import argparse
import contextlib
import datetime as dt
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import re
import select
import signal
import shutil
import stat
import subprocess
import sys
import tarfile
import tempfile
import uuid
from urllib.parse import parse_qs, unquote, urlsplit, urlunsplit

LEASE_SQL = "select pg_try_advisory_lock(hashtextextended('space-runtime-mode',0));\n"

SAFE_CONFIG = ('SPACE_ENV', 'SPACE_HTTP_ADDR', 'SPACE_PUBLIC_URL', 'SPACE_WEB_DIR',
               'SPACE_ATTACHMENT_MAX_BYTES', 'SPACE_SESSION_TTL', 'SPACE_RUN_MODE')
POLICY_KEYS = {'backup_dir', 'staging_dir', 'attachments_dir', 'wa_state_dir',
               'age_recipients_file', 'server_units', 'bridge_units', 'user_services',
               'retention_days', 'minimum_copies', 'operator', 'rpo_minutes',
               'rto_minutes', 'release_id', 'max_restore_bytes', 'max_restore_files', 'backup_volume_id'}


class OpsError(Exception):
    pass


def utc():
    return dt.datetime.now(dt.timezone.utc)


def require(condition, message):
    if not condition:
        raise OpsError(message)


def command(args, env=None, **kwargs):
    # Tool stderr can contain operator configuration. Keep it off session logs.
    try:
        return subprocess.run(args, env=env, check=True, stderr=subprocess.DEVNULL,
                              **kwargs)
    except (OSError, subprocess.CalledProcessError) as error:
        raise OpsError(f'{Path(args[0]).name} failed; inspect protected operator configuration') from error


def tools(*names):
    for name in names:
        require(shutil.which(name), f'required tool is missing: {name}')


def private_directory(path, create=False):
    path = Path(path)
    require(path.is_absolute() and path != Path('/'), 'use an absolute private directory')
    if create:
        path.mkdir(mode=0o700, parents=True, exist_ok=True)
    info = path.lstat()
    require(stat.S_ISDIR(info.st_mode) and not path.is_symlink(), 'storage must be a real directory')
    require(info.st_mode & 0o077 == 0, 'storage directory must have mode 0700')
    return path


def backup_destination(config):
    root = private_directory(config['backup_dir'])
    expected = config.get('backup_volume_id')
    require(isinstance(expected, str) and expected and len(expected) <= 200,
            'configure a backup_volume_id before enabling backup/retention')
    marker = root / '.space-backup-volume-id'
    require(marker.is_file() and not marker.is_symlink() and marker.stat().st_size <= 256,
            'backup volume marker is missing; no local fallback directory is created')
    require(marker.read_text(encoding='utf-8').strip() == expected,
            'backup volume identity mismatch; no files were written/deleted')
    return root


def digest(path):
    hasher = hashlib.sha256()
    with Path(path).open('rb') as source:
        for chunk in iter(lambda: source.read(1024 * 1024), b''):
            hasher.update(chunk)
    return hasher.hexdigest()


def write_json(path, value):
    with Path(path).open('x', encoding='utf-8') as target:
        os.chmod(path, 0o600)
        json.dump(value, target, ensure_ascii=False, indent=2)
        target.write('\n')
        target.flush()
        os.fsync(target.fileno())


def policy(path):
    with Path(path).open(encoding='utf-8') as source:
        data = json.load(source)
    require(isinstance(data, dict) and not set(data) - POLICY_KEYS, 'unknown backup policy fields')
    for key in ('backup_dir', 'staging_dir', 'attachments_dir', 'age_recipients_file'):
        require(isinstance(data.get(key), str) and Path(data[key]).is_absolute(), f'{key} must be absolute')
    data.setdefault('wa_state_dir', None)
    data.setdefault('minimum_copies', 1)
    data.setdefault('max_restore_bytes', 20 * 1024**3)
    data.setdefault('max_restore_files', 100000)
    require(type(data['minimum_copies']) is int and data['minimum_copies'] >= 1, 'minimum_copies must be positive')
    for key in ('server_units', 'bridge_units'):
        require(isinstance(data.get(key), list), f'{key} must be a list')
        for unit in data[key]:
            require(isinstance(unit, str) and re.fullmatch(r'[A-Za-z0-9_.@-]+\.service', unit), 'invalid systemd service unit')
    require(data['server_units'], 'declare all local Space server service units')
    require(not data['wa_state_dir'] or data['bridge_units'], 'WA state requires declared bridge service units')
    if data.get('retention_days') is not None:
        require(type(data['retention_days']) is int and data['retention_days'] > 0, 'retention_days must be positive or null')
    for key in ('max_restore_bytes', 'max_restore_files'):
        require(type(data[key]) is int and data[key] > 0, f'{key} must be positive')
    roots = [Path(data[k]).resolve() for k in ('backup_dir', 'staging_dir', 'attachments_dir')]
    if data['wa_state_dir']:
        roots.append(Path(data['wa_state_dir']).resolve())
    for i, root in enumerate(roots):
        for other in roots[i + 1:]:
            require(root != other and root not in other.parents and other not in root.parents,
                    'backup, staging, attachment and WA directories must be disjoint')
    return data


def pg_environment(variable):
    """Translate an explicit PostgreSQL URI to libpq env, never argv."""
    raw = os.environ.get(variable, '')
    require(raw, f'{variable} must be provided through a protected environment')
    uri = urlsplit(raw)
    require(uri.scheme in ('postgres', 'postgresql') and uri.hostname and uri.path not in ('', '/'),
            f'{variable} must be an explicit PostgreSQL URI with a database')
    env = {key: value for key, value in os.environ.items() if not key.startswith('PG')}
    env.update(PGHOST=uri.hostname, PGPORT=str(uri.port or 5432), PGDATABASE=unquote(uri.path[1:]),
               PGCONNECT_TIMEOUT='15', PGAPPNAME='space-ops')
    if uri.username is not None:
        env['PGUSER'] = unquote(uri.username)
    if uri.password is not None:
        env['PGPASSWORD'] = unquote(uri.password)
    options = {'sslmode': 'PGSSLMODE', 'sslrootcert': 'PGSSLROOTCERT', 'sslcert': 'PGSSLCERT',
               'sslkey': 'PGSSLKEY', 'connect_timeout': 'PGCONNECT_TIMEOUT'}
    for key, values in parse_qs(uri.query).items():
        require(key in options and len(values) == 1, 'unsupported or repeated PostgreSQL URI option')
        env[options[key]] = values[0]
    return env, uri


def query(env, sql):
    return command(['psql', '-X', '--no-password', '-qAt', '-v', 'ON_ERROR_STOP=1'],
                   env=env, input=sql, text=True, stdout=subprocess.PIPE).stdout.strip()


@contextlib.contextmanager
def exclusive_runtime_lease(env):
    process = subprocess.Popen(['psql', '-X', '--no-password', '-qAt', '-v', 'ON_ERROR_STOP=1'],
                               env=env, stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                               stderr=subprocess.DEVNULL, text=True, bufsize=1)
    try:
        process.stdin.write(LEASE_SQL)
        process.stdin.flush()
        readable, _, _ = select.select([process.stdout], [], [], 20)
        require(readable and process.stdout.readline().strip() == 't',
                'a server is still using this database, or the runtime lease could not be acquired')
        yield
        require(process.poll() is None, 'runtime lease connection ended before the snapshot finished')
    finally:
        process.stdin.close()
        try:
            process.wait(timeout=5)
        except subprocess.TimeoutExpired:
            process.terminate()
            process.wait(timeout=5)


def systemctl(config, *args):
    return command(['systemctl'] + (['--user'] if config.get('user_services') else []) + list(args),
                   text=True, stdout=subprocess.PIPE).stdout.strip()


def process_identity(pid):
    try:
        value = Path(f'/proc/{pid}/stat').read_text()
        started = value[value.rfind(')') + 2:].split()[19]
        boot = Path('/proc/sys/kernel/random/boot_id').read_text().strip()
        return boot + ':' + started
    except (OSError, IndexError):
        return None


def resume_services(config, internal=False):
    journal = private_directory(config['staging_dir']) / 'backup-services.json'
    if not journal.exists():
        return
    require(not journal.is_symlink() and journal.stat().st_size <= 16384, 'invalid service restart journal')
    record = json.loads(journal.read_text(encoding='utf-8'))
    require(internal or process_identity(record['pid']) != record['processIdentity'],
            'the owning backup process is still alive; service recovery refused')
    allowed = set(config['server_units'] + config['bridge_units'])
    require(isinstance(record.get('units'), list) and set(record['units']) <= allowed,
            'restart journal does not match the declared service policy')
    failures = []
    for unit in reversed(record['units']):
        try:
            systemctl(config, 'start', unit)
        except OpsError:
            failures.append(unit)
    require(not failures, 'service recovery still required: ' + ', '.join(failures))
    journal.unlink()


@contextlib.contextmanager
def quiesced_services(config, stop):
    units = list(dict.fromkeys(config['server_units'] + config['bridge_units']))
    restart = []
    for unit in units:
        loaded = systemctl(config, 'show', '--property=LoadState', '--value', unit)
        require(loaded == 'loaded', f'service unit is not loaded: {unit}')
        state = systemctl(config, 'show', '--property=ActiveState', '--value', unit)
        if state in ('active', 'activating', 'reloading'):
            require(stop, f'stop {unit} first or explicitly use --stop-services')
            restart.append(unit)
        else:
            require(state in ('inactive', 'failed'), f'service is not quiescent: {unit}')
    journal = private_directory(config['staging_dir']) / 'backup-services.json'
    write_json(journal, {'units': restart, 'pid': os.getpid(),
                        'processIdentity': process_identity(os.getpid()), 'createdAt': utc().isoformat()})
    try:
        for unit in restart:
            systemctl(config, 'stop', unit)
        for unit in units:
            state = systemctl(config, 'show', '--property=ActiveState', '--value', unit)
            require(state in ('inactive', 'failed'), f'service has not stopped: {unit}')
        yield
    finally:
        # The journal survives SIGKILL/power loss. ExecStopPost or an operator
        # can resume only the services that were active before this snapshot.
        resume_services(config, internal=True)


def copy_private_tree(source, destination, suffix, temporary_prefix):
    source = private_directory(source)
    destination.mkdir(mode=0o700)
    for path in sorted(source.rglob('*')):
        require(not path.is_symlink(), 'symlinks are not allowed in backup source trees')
        if path.is_dir():
            continue
        require(path.is_file(), 'only regular source files can be backed up')
        if path.name.startswith(temporary_prefix) or path.name.endswith('.tmp'):
            continue
        require(path.suffix == suffix, 'unexpected file in private storage; review the source layout before backup')
        relative = path.relative_to(source)
        target = destination / relative
        target.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
        with path.open('rb') as incoming, target.open('xb') as outgoing:
            os.chmod(target, 0o600)
            shutil.copyfileobj(incoming, outgoing, 1024 * 1024)
            outgoing.flush()
            os.fsync(outgoing.fileno())


def counts(env):
    tables = json.loads(query(env, "select coalesce(json_agg(tablename order by tablename),'[]'::json) from pg_tables where schemaname='public';"))
    require(0 < len(tables) <= 200 and all(re.fullmatch(r'[a-z_][a-z0-9_]*', name) for name in tables),
            'unexpected public schema table names/count; review the database before backup/restore')
    parts = [f"select '{name}'::text as name, (select count(*) from public.\"{name}\") as n" for name in tables]
    sql = "select json_object_agg(name,n) from (" + " union all ".join(parts) + ") counts;"
    return json.loads(query(env, sql))


def check_blob_references(env, stage):
    records = json.loads(query(env, "select coalesce(json_agg(json_build_object('key',storage_key,'size',size_bytes,'sha256',sha256)),'[]'::json) from note_attachments where storage_key is not null;"))
    for item in records:
        key = str(uuid.UUID(item['key']))
        path = stage / 'attachments' / (key + '.blob')
        require(path.is_file() and path.stat().st_size == item['size'] and digest(path) == item['sha256'],
                'a database attachment reference has no matching complete blob')
    return len(records)


def make_manifest(stage, metadata):
    files = {}
    for path in sorted(stage.rglob('*')):
        if path.is_file():
            files[path.relative_to(stage).as_posix()] = {'size': path.stat().st_size, 'sha256': digest(path)}
    metadata.update(schemaVersion=1, files=files)
    write_json(stage / 'manifest.json', metadata)


def encrypt_stage(stage, recipients, destination):
    require(Path(recipients).is_file() and not Path(recipients).is_symlink(), 'recipient list must be a regular public-key file')
    partial = destination.with_suffix(destination.suffix + '.partial')
    try:
        with partial.open('xb') as outgoing:
            os.chmod(partial, 0o600)
            age = subprocess.Popen(['age', '--encrypt', '--recipients-file', recipients],
                                   stdin=subprocess.PIPE, stdout=outgoing, stderr=subprocess.DEVNULL)
            try:
                with tarfile.open(fileobj=age.stdin, mode='w|') as archive:
                    for path in sorted(stage.rglob('*')):
                        if path.is_file():
                            archive.add(path, arcname=path.relative_to(stage).as_posix(), recursive=False)
                age.stdin.close()
                require(age.wait() == 0, 'backup encryption failed')
            finally:
                if age.poll() is None:
                    age.kill()
                    age.wait()
            outgoing.flush()
            os.fsync(outgoing.fileno())
        os.rename(partial, destination)
    finally:
        partial.unlink(missing_ok=True)


def backup(args):
    config = policy(args.policy)
    tools('pg_dump', 'psql', 'age', 'systemctl')
    backup_root = backup_destination(config)
    staging_root = private_directory(config['staging_dir'], True)
    env, _ = pg_environment('SPACE_DATABASE_URL')
    backup_id = utc().strftime('%Y%m%dT%H%M%SZ') + '-' + uuid.uuid4().hex
    destination = backup_root / ('space-' + backup_id + '.tar.age')
    with tempfile.TemporaryDirectory(prefix='space-backup-', dir=staging_root) as temporary:
        stage = Path(temporary)
        started = utc().isoformat()
        with quiesced_services(config, args.stop_services), exclusive_runtime_lease(env):
            baseline_counts = counts(env)
            command(['pg_dump', '--format=custom', '--no-owner', '--no-privileges', '--file', str(stage / 'database.dump')], env=env)
            os.chmod(stage / 'database.dump', 0o600)
            copy_private_tree(config['attachments_dir'], stage / 'attachments', '.blob', '.upload-')
            if config['wa_state_dir']:
                copy_private_tree(config['wa_state_dir'], stage / 'wa-state', '.enc', '.upload-')
            reference_count = check_blob_references(env, stage)
            require(counts(env) == baseline_counts, 'database row counts changed during the quiesced snapshot')
        # Services resume after capture, before potentially slow encryption/offsite IO.
        write_json(stage / 'deployment.json', {'releaseId': config.get('release_id'),
                   'configuration': {key: os.environ[key] for key in SAFE_CONFIG if key in os.environ},
                   'secretsIncluded': False, 'waStateKeyIncluded': False})
        make_manifest(stage, {'backupId': backup_id, 'startedAt': started, 'capturedAt': utc().isoformat(),
                      'strategy': 'stopped_services_and_exclusive_runtime_lease', 'rowCounts': baseline_counts,
                      'referencedAttachments': reference_count, 'waStateIncluded': bool(config['wa_state_dir']),
                      'sourceRelease': config.get('release_id')})
        encrypt_stage(stage, config['age_recipients_file'], destination)
    checksum = digest(destination)
    write_json(destination.with_suffix(destination.suffix + '.json'),
               {'backupId': backup_id, 'createdAt': utc().isoformat(), 'sha256': checksum,
                'file': destination.name, 'size': destination.stat().st_size})
    print(json.dumps({'backup': str(destination), 'sha256': checksum, 'restoreTested': False}))


@contextlib.contextmanager
def verified_archive(args, config):
    archive = Path(args.archive)
    require(re.fullmatch('[a-fA-F0-9]{64}', args.expected_sha256 or ''), 'provide a SHA-256 from a trusted separate backup record')
    require(archive.is_file() and not archive.is_symlink() and digest(archive) == args.expected_sha256.lower(), 'encrypted archive checksum mismatch')
    staging_root = private_directory(config['staging_dir'], True)
    with tempfile.TemporaryDirectory(prefix='space-verify-', dir=staging_root) as temporary:
        stage = Path(temporary)
        process = subprocess.Popen(['age', '--decrypt', '--identity', args.identity, str(archive)],
                                   stdout=subprocess.PIPE, stderr=subprocess.DEVNULL)
        seen = set()
        total = 0
        try:
            with tarfile.open(fileobj=process.stdout, mode='r|') as incoming:
                for member in incoming:
                    name = member.name
                    relative = PurePosixPath(name)
                    require(not relative.is_absolute() and '..' not in relative.parts and '\\' not in name,
                            'unsafe backup path')
                    require(member.isfile() and name not in seen and relative.parts and
                            (name in ('database.dump', 'deployment.json', 'manifest.json') or
                             relative.parts[0] in ('attachments', 'wa-state')), 'unsupported or repeated archive entry')
                    seen.add(name)
                    total += member.size
                    require(member.size >= 0 and total <= config['max_restore_bytes'] and len(seen) <= config['max_restore_files'], 'restore staging limit exceeded')
                    target = stage / relative
                    target.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
                    with incoming.extractfile(member) as source, target.open('xb') as destination:
                        os.chmod(target, 0o600)
                        shutil.copyfileobj(source, destination, 1024 * 1024)
                    require(target.stat().st_size == member.size, 'truncated archive member')
            tail = process.stdout.read(10241)
            require(len(tail) <= 10240 and not tail.strip(b'\0'), 'unexpected trailing archive data')
            process.stdout.close()
            require(process.wait() == 0, 'archive decryption/authentication did not complete')
        finally:
            if process.poll() is None:
                process.kill()
                process.wait()
        require((stage / 'manifest.json').is_file(), 'backup manifest is missing')
        require((stage / 'manifest.json').stat().st_size <= 32 * 1024**2, 'manifest is too large')
        manifest = json.loads((stage / 'manifest.json').read_text(encoding='utf-8'))
        require(manifest.get('schemaVersion') == 1 and isinstance(manifest.get('files'), dict), 'unsupported manifest')
        require(set(manifest['files']) == seen - {'manifest.json'}, 'manifest file set mismatch')
        require({'database.dump', 'deployment.json'} <= set(manifest['files']), 'required backup files are missing')
        for name, expected in manifest['files'].items():
            path = stage / name
            require(path.stat().st_size == expected['size'] and digest(path) == expected['sha256'], 'backup content checksum mismatch')
        # Archives contain regular files only. An empty storage tree therefore
        # has no entry, but is still required by restore and reference checks.
        (stage / 'attachments').mkdir(mode=0o700, exist_ok=True)
        if manifest.get('waStateIncluded'):
            (stage / 'wa-state').mkdir(mode=0o700, exist_ok=True)
        yield stage, manifest


def verify(args):
    config = policy(args.policy)
    tools('age', 'pg_restore')
    with verified_archive(args, config) as (stage, manifest):
        command(['pg_restore', '--list', str(stage / 'database.dump')], stdout=subprocess.DEVNULL)
        print(json.dumps({'backupId': manifest['backupId'], 'filesVerified': len(manifest['files']),
                          'databaseRestored': False, 'runtimeTested': False}))


def restore(args):
    config = policy(args.policy)
    tools('age', 'psql', 'pg_restore', 'createdb')
    require(re.fullmatch(r'space_(restore|shadow)_[a-z0-9_]+', args.target_db or '') and len(args.target_db) <= 63,
            'target must be a new space_restore_ / space_shadow_ database')
    target = Path(args.target_root)
    require(target.is_absolute() and not target.exists() and not target.is_symlink(), 'restore storage must be a new absolute directory')
    binary = Path(args.server_bin)
    require(binary.is_absolute() and binary.is_file() and os.access(binary, os.X_OK), 'provide the matching built space-server binary')
    env, uri = pg_environment('SPACE_RESTORE_DATABASE_URL')
    require(env['PGDATABASE'] != args.target_db, 'restore URL must point to an administration database, not the new target')
    exists = query(env, f"select exists(select 1 from pg_database where datname='{args.target_db}');")
    require(exists == 'f', 'target database already exists; no overwrite/drop is supported')
    with verified_archive(args, config) as (stage, manifest):
        command(['createdb', '--maintenance-db', env['PGDATABASE'], '--template=template0', '--encoding=UTF8', '--', args.target_db], env=env)
        query(env, f"alter database \"{args.target_db}\" set space.restore_quarantined='true';")
        target.mkdir(mode=0o700, parents=True)
        write_json(target / 'restore-incomplete.json', {'backupId': manifest['backupId'], 'targetDatabase': args.target_db})
        target_env = dict(env, PGDATABASE=args.target_db)
        command(['pg_restore', '--exit-on-error', '--single-transaction', '--no-owner', '--no-privileges',
                 '--dbname', args.target_db, str(stage / 'database.dump')], env=target_env)
        require(counts(target_env) == manifest['rowCounts'], 'restored database row counts differ from the snapshot')
        references = check_blob_references(target_env, stage)
        # Quarantine precedes serving. It retains historical delivery/audit rows
        # and revokes copied sessions; no production bridge is launched here.
        server_env = {key: value for key, value in os.environ.items() if not key.startswith('SPACE_')}
        restored_uri = urlunsplit((uri.scheme, uri.netloc, '/' + args.target_db, uri.query, ''))
        server_env.update(SPACE_DATABASE_URL=restored_uri, SPACE_RUN_MODE='isolated_restore',
                          SPACE_HTTP_ADDR='127.0.0.1:8790', SPACE_PUBLIC_URL='http://localhost:8790',
                          SPACE_ATTACHMENT_DIR=str(target / 'attachments'), SPACE_MAIL_DRIVER='disabled')
        command([str(binary), 'quarantine-restore', '-database-name', args.target_db,
                 '-reason', 'Isolated restore drill for backup ' + manifest['backupId']], env=server_env, stdout=subprocess.DEVNULL)
        shutil.copytree(stage / 'attachments', target / 'attachments')
        if (stage / 'wa-state').exists():
            shutil.copytree(stage / 'wa-state', target / 'wa-state-quarantined')
        content = ('SPACE_RUN_MODE=isolated_restore\nSPACE_ENV=development\nSPACE_HTTP_ADDR=127.0.0.1:8790\n'
                   'SPACE_PUBLIC_URL=http://localhost:8790\nSPACE_MAIL_DRIVER=disabled\n'
                   'SPACE_WA_BRIDGE_URL=\nSPACE_WA_BRIDGE_TOKEN=\n'
                   'SPACE_ATTACHMENT_DIR=' + str(target / 'attachments') + '\n'
                   '# Supply SPACE_DATABASE_URL separately using a restricted role for ' + args.target_db + '.\n')
        with (target / 'restore.env').open('x', encoding='utf-8') as output:
            os.chmod(output.name, 0o600)
            output.write(content)
        report = {'backupId': manifest['backupId'], 'targetDatabase': args.target_db,
                  'restoredAt': utc().isoformat(), 'rowCountsBeforeQuarantine': manifest['rowCounts'],
                  'referencedAttachmentsVerified': references, 'quarantined': True,
                  'outboundEnabled': False, 'runtimeAcceptanceTested': False}
        write_json(target / 'restore-report.json', report)
        (target / 'restore-incomplete.json').unlink()
        print(json.dumps({'report': str(target / 'restore-report.json'), 'quarantined': True,
                          'runtimeAcceptanceTested': False}))


def prune(args):
    config = policy(args.policy)
    root = backup_destination(config)
    days = config.get('retention_days')
    if days is None:
        print(json.dumps({'retentionEnabled': False, 'candidates': []}))
        return
    records = []
    for index in root.glob('space-*.tar.age.json'):
        require(not index.is_symlink(), 'retention index must not be a symlink')
        record = json.loads(index.read_text(encoding='utf-8'))
        require(re.fullmatch(r'space-[0-9]{8}T[0-9]{6}Z-[a-f0-9]{32}\.tar\.age', record['file']), 'invalid archive name in retention record')
        file = root / record['file']
        require(file.parent == root and file.is_file() and not file.is_symlink(), 'invalid retention archive path')
        records.append((dt.datetime.fromisoformat(record['createdAt']), file, index, record['sha256']))
    records.sort(reverse=True)
    cutoff = utc() - dt.timedelta(days=days)
    candidates = records[config['minimum_copies']:]
    selected = [(created, file, index, checksum) for created, file, index, checksum in candidates if created < cutoff]
    if args.apply:
        for _, file, index, checksum in selected:
            require(digest(file) == checksum, 'retention archive checksum mismatch; deletion refused')
            file.unlink()
            index.unlink()
    print(json.dumps({'retentionEnabled': True, 'applied': args.apply,
                      'candidates': [file.name for _, file, _, _ in selected]}))


def resume(args):
    config = policy(args.policy)
    tools('systemctl')
    resume_services(config)
    print(json.dumps({'serviceRecoveryComplete': True}))


def main():
    os.umask(0o077)
    def interrupted(signum, frame):
        raise OpsError('operation interrupted; check the service restart journal if recovery is incomplete')
    signal.signal(signal.SIGTERM, interrupted)
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--policy', required=True)
    commands = parser.add_subparsers(dest='action', required=True)
    commands.add_parser('backup').add_argument('--stop-services', action='store_true')
    for name in ('verify', 'restore'):
        command_parser = commands.add_parser(name)
        command_parser.add_argument('--archive', required=True)
        command_parser.add_argument('--identity', required=True)
        command_parser.add_argument('--expected-sha256', required=True)
        if name == 'restore':
            command_parser.add_argument('--target-db', required=True)
            command_parser.add_argument('--target-root', required=True)
            command_parser.add_argument('--server-bin', required=True)
    commands.add_parser('prune').add_argument('--apply', action='store_true')
    commands.add_parser('resume')
    args = parser.parse_args()
    globals()[args.action](args)


if __name__ == '__main__':
    try:
        main()
    except (OpsError, OSError, ValueError, tarfile.TarError) as error:
        # Do not print connection strings, subprocess stderr, or exception
        # reprs with credentials. Validation messages above are explicit.
        message = str(error) if isinstance(error, OpsError) else type(error).__name__
        print('space-ops: ' + message, file=sys.stderr)
        sys.exit(1)

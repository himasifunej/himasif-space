#!/usr/bin/env python3
"""Publish a Windows installer as an in-app update (docs/operations/auto-update.md).

1. Uploads the installer to the GitHub release v<version> (made when
   missing: a pre-release for the rc channel, never marked latest).
2. Signs the manifest of the channel with tools/updatesign; the private
   key comes from SPACE_UPDATE_SIGNING_KEY (from the vault:
   enowx creds use space-update-signing -- python3 ...) or --key-file.
3. Checks the envelope and the installer against the keys built into the
   app, and refuses a version not newer than the one the feed offers.
4. Replaces <channel>.json on the release "update-feed", then reads it
   back from the public URL and checks it again.

--dry-run does steps 2 and 3 only, into --out, and changes nothing on
GitHub. Never prints the key.
"""
import argparse
import hashlib
import json
import os
import shutil
from pathlib import Path
import re
import subprocess
import sys
import tempfile
import time
import urllib.request

ROOT = Path(__file__).resolve().parents[2]
REPO = 'himasifunej/himasif-space'
FEED_TAG = 'update-feed'
FEED_BASE = f'https://github.com/{REPO}/releases/download/{FEED_TAG}/'


class PublishError(Exception):
    pass


def run(args, **kw):
    result = subprocess.run(args, cwd=ROOT, text=True, capture_output=True, **kw)
    if result.returncode != 0:
        raise PublishError(f'{args[0]} {args[1] if len(args) > 1 else ""} failed:\n{result.stderr.strip()}')
    return result.stdout


def sha256(path):
    h = hashlib.sha256()
    with open(path, 'rb') as f:
        for chunk in iter(lambda: f.read(1 << 20), b''):
            h.update(chunk)
    return h.hexdigest()


def updatesign_command():
    """tools/updatesign: a binary built elsewhere (SPACE_UPDATESIGN), for a
    machine that must not compile, else go run."""
    built = os.environ.get('SPACE_UPDATESIGN')
    if built:
        if not Path(built).is_file():
            raise PublishError(f'SPACE_UPDATESIGN names no file: {built}')
        return [built]
    return ['go', 'run', './tools/updatesign']


def updatesign(*args):
    return run([*updatesign_command(), *args])


def release_exists(tag):
    return subprocess.run(['gh', 'release', 'view', tag, '--repo', REPO], cwd=ROOT,
                          capture_output=True).returncode == 0


def published_version(channel):
    """The version the feed offers now, checked like the app does, or None."""
    try:
        with urllib.request.urlopen(FEED_BASE + channel + '.json', timeout=30) as response:
            data = response.read(64 << 10)
    except Exception:
        return None
    with tempfile.TemporaryDirectory() as tmp:
        path = Path(tmp) / 'current.json'
        path.write_bytes(data)
        out = updatesign('verify', '-channel', channel, '-in', str(path))
    match = re.match(r'ok: \S+ (\S+),', out)
    return match.group(1) if match else None


def newer(a, b):
    """Whether version a is newer than b, by the app's own rule."""
    out = run([*updatesign_command(), 'compare', a, b])
    return out.strip() == '1'


# The app's limit (update.MaxNotes): longer notes are refused when signed.
MAX_NOTES = 600


def changelog_notes(version):
    """The version's notes as the app bundles them, one bullet a line, as
    the "Yang baru" dialog and older clients' single line both read them. A
    release candidate takes its release's (0.4.6-rc2: 0.4.6)."""
    base = version.lstrip('v').split('-', 1)[0]
    entries = json.loads((ROOT / 'internal' / 'appui' / 'changelog.json').read_text(encoding='utf-8'))
    for entry in entries:
        if entry['version'] == base:
            return '\n'.join('• ' + line for line in entry['notes'])
    raise PublishError(f'internal/appui/changelog.json has no notes for {base}: add them, or pass --notes')


def main():
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument('--installer', required=True, help='space-setup-*.exe built for this version')
    parser.add_argument('--version', required=True, help='version the app reports, e.g. 0.4.6 or 0.4.6-rc')
    parser.add_argument('--channel', required=True, choices=('stable', 'rc'))
    parser.add_argument('--notes', help="short notes, at most 600 characters; by default the version's bullets "
                        'from internal/appui/changelog.json')
    parser.add_argument('--target', default='HEAD', help='commit for a new release tag (must be pushed)')
    parser.add_argument('--key-file', help='private key file instead of SPACE_UPDATE_SIGNING_KEY')
    parser.add_argument('--out', default=str(ROOT / 'dist' / 'update'), help='where the envelope is written')
    parser.add_argument('--dry-run', action='store_true')
    args = parser.parse_args()
    if not args.notes:
        args.notes = changelog_notes(args.version)
    if len(args.notes) > MAX_NOTES:
        raise PublishError(f'notes are {len(args.notes)} characters; the manifest takes {MAX_NOTES}')

    installer = Path(args.installer).resolve()
    if not installer.is_file():
        raise PublishError(f'no installer at {installer}')
    # MyGo names the installer after app/mygo.json's version, which is also
    # the version the app reports (mygo.App.Version): they must match.
    if f'-{args.version}-' not in installer.name:
        raise PublishError(f'{installer.name} is not the installer of {args.version}: set app/mygo.json "version" '
                           f'to {args.version} and build again')
    if args.channel == 'stable' and '-' in args.version:
        raise PublishError('the stable channel takes releases only, not ' + args.version)
    tag = 'v' + args.version
    asset = f'space-setup-{args.version}-amd64.exe'
    url = f'https://github.com/{REPO}/releases/download/{tag}/{asset}'
    notes_url = f'https://github.com/{REPO}/releases/tag/{tag}'

    current = published_version(args.channel)
    if current and not newer(args.version, current):
        raise PublishError(f'the {args.channel} feed already offers {current}; {args.version} is not newer')

    out = Path(args.out)
    out.mkdir(parents=True, exist_ok=True)
    envelope = out / (args.channel + '.json')
    sign = ['sign', '-channel', args.channel, '-version', args.version, '-url', url, '-file', str(installer),
            '-notes', args.notes, '-notes-url', notes_url, '-out', str(envelope)]
    if args.key_file:
        sign += ['-key-file', args.key_file]
    print(updatesign(*sign).strip())
    print(updatesign('verify', '-channel', args.channel, '-in', str(envelope), '-file', str(installer)).strip())
    if args.dry_run:
        print(f'dry run: would upload {asset} to {tag} and {envelope.name} to {FEED_TAG}')
        return

    if not release_exists(tag):
        create = ['gh', 'release', 'create', tag, '--repo', REPO, '--target', run(['git', 'rev-parse', args.target]).strip(),
                  '--title', f'Space {args.version}', '--notes', args.notes]
        if args.channel == 'rc':
            create += ['--prerelease', '--latest=false']
        run(create)
    with tempfile.TemporaryDirectory() as tmp:
        named = Path(tmp) / asset
        shutil.copy2(installer, named)
        run(['gh', 'release', 'upload', tag, str(named), '--repo', REPO])
    if not release_exists(FEED_TAG):
        run(['gh', 'release', 'create', FEED_TAG, '--repo', REPO, '--prerelease', '--latest=false',
             '--title', 'Update feed (otomatis)',
             '--notes', 'Manifest pembaruan Space desktop yang ditandatangani (stable.json, rc.json). '
                        'Diganti oleh scripts/release/publish_update.py; jangan diubah tangan.'])
    run(['gh', 'release', 'upload', FEED_TAG, str(envelope), '--repo', REPO, '--clobber'])

    # GitHub's CDN takes a little while to serve the replaced asset.
    for _ in range(18):
        if published_version(args.channel) == args.version:
            break
        time.sleep(5)
    else:
        raise PublishError('the public feed does not offer the new version after 90 s; check the update-feed release')
    with tempfile.TemporaryDirectory() as tmp:
        back = Path(tmp) / asset
        urllib.request.urlretrieve(url, back)
        if sha256(back) != sha256(installer):
            raise PublishError('the uploaded installer differs from the local one')
    print(f'published {args.channel} {args.version}: {FEED_BASE}{args.channel}.json -> {url}')


if __name__ == '__main__':
    try:
        main()
    except PublishError as error:
        print('publish_update: ' + str(error), file=sys.stderr)
        sys.exit(1)

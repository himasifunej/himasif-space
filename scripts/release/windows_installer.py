#!/usr/bin/env python3
"""Add Space-owned registry cleanup to MyGo's NSIS installer, then compile it.

Used only by build_release.py's temporary makensis wrapper. The SDK remains
unchanged. Fail on an unexpected template rather than shipping partial cleanup.
"""
import os
from pathlib import Path
import sys

MARKER = '; Space desktop registration cleanup'
# Card links from the web open the app (0.4.6): the template must register
# space:// (urlSchemes in app/mygo.json) as well as himasif-space://.
SCHEMES = ('Software\\Classes\\himasif-space\\shell\\open\\command', 'Software\\Classes\\space\\shell\\open\\command')
ANCHOR = 'Section "Uninstall"\n  Push "$INSTDIR"\n  Call un.CloseApp\n'
CLEANUP = r'''
  ; Space desktop registration cleanup
  ; A portable/other installation may own the entry: compare before deleting.
  ReadRegStr $0 HKCU "Software\Microsoft\Windows\CurrentVersion\Run" "HIMASIF Space"
  StrCmp $0 '$\"$INSTDIR\Space.exe$\" --background' 0 space_keep_startup
    DeleteRegValue HKCU "Software\Microsoft\Windows\CurrentVersion\Run" "HIMASIF Space"
    DeleteRegValue HKCU "Software\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\Run" "HIMASIF Space"
  space_keep_startup:
  ReadRegStr $0 HKCU "Software\Classes\AppUserModelId\id.himasif.space" "SpaceExecutable"
  StrCmp $0 "$INSTDIR\Space.exe" 0 space_keep_notifier
    DeleteRegKey HKCU "Software\Classes\AppUserModelId\id.himasif.space"
  space_keep_notifier:
'''


def patch_installer(text):
    if MARKER in text:
        raise ValueError('Space cleanup is already present; refusing a second rewrite')
    if (not text.startswith('Unicode true\n') or text.count(ANCHOR) != 1 or
            'Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\id.himasif.space' not in text or
            'CreateShortCut "$SMPROGRAMS\\Space.lnk" "$INSTDIR\\Space.exe"' not in text):
        raise ValueError('unexpected Space installer template; review the SDK installer before releasing')
    for scheme in SCHEMES:
        if scheme not in text:
            raise ValueError('the installer does not register ' + scheme + '; check urlSchemes in app/mygo.json')
    return 'Target amd64-unicode\n' + text.replace(ANCHOR, ANCHOR + CLEANUP, 1)


def main():
    compiler = os.environ.get('SPACE_NSIS_COMPILER')
    if not compiler or not Path(compiler).is_absolute():
        raise ValueError('SPACE_NSIS_COMPILER must name the real NSIS compiler')
    scripts = [Path(arg) for arg in sys.argv[1:] if arg.endswith('.nsi')]
    if len(scripts) != 1:
        raise ValueError('expected exactly one Space NSIS script')
    script = scripts[0]
    patched = patch_installer(script.read_text(encoding='utf-8-sig'))
    script.write_text(patched, encoding='utf-8-sig')
    # Preserve the actual compiler input alongside verification logs.
    evidence = os.environ.get('SPACE_NSIS_EVIDENCE')
    if evidence:
        Path(evidence).write_text(patched, encoding='utf-8')
    os.execv(compiler, [compiler, *sys.argv[1:]])


if __name__ == '__main__':
    try:
        main()
    except (ValueError, OSError) as error:
        print('Space installer: ' + str(error), file=sys.stderr)
        sys.exit(1)

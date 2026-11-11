import unittest
import windows_installer as installer


class SpaceInstallerGuards(unittest.TestCase):
    def fixture(self):
        return ('Unicode true\nCreateShortCut "$SMPROGRAMS\\Space.lnk" "$INSTDIR\\Space.exe"\n'
                'WriteRegStr HKCU "Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\id.himasif.space" "DisplayName" "Space"\n'
                'WriteRegStr HKCU "Software\\Classes\\himasif-space\\shell\\open\\command" "" "$INSTDIR\\Space.exe"\n'
                'WriteRegStr HKCU "Software\\Classes\\space\\shell\\open\\command" "" "$INSTDIR\\Space.exe"\n'
                + installer.ANCHOR + '  RMDir /r "$INSTDIR"\nSectionEnd\n')

    def test_cleanup_precedes_removal_and_checks_current_owner(self):
        patched = installer.patch_installer(self.fixture())
        self.assertTrue(patched.startswith('Target amd64-unicode\n'))
        self.assertLess(patched.index(installer.MARKER), patched.index('RMDir /r'))
        self.assertIn('StrCmp $0', patched)
        self.assertIn('space_keep_startup', patched)
        self.assertIn('space_keep_notifier', patched)
        self.assertNotIn('himasif-space\\device', patched)

    def test_unknown_app_template_is_refused(self):
        for text in [self.fixture().replace('id.himasif.space', 'another.app'),
                     self.fixture().replace('Space.exe', 'Other.exe'),
                     self.fixture().replace(installer.ANCHOR, ''),
                     self.fixture().replace('Classes\\space\\', 'Classes\\other\\')]:
            with self.assertRaises(ValueError):
                installer.patch_installer(text)

    def test_repeated_injection_is_refused(self):
        with self.assertRaises(ValueError):
            installer.patch_installer(installer.patch_installer(self.fixture()))

import argparse
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest
from unittest.mock import patch
import space_ops as ops
import compare_legacy as legacy


class OperationsGuards(unittest.TestCase):
    @unittest.skipUnless(shutil.which('age') and shutil.which('age-keygen'), 'requires age tools')
    def test_empty_storage_survives_encrypted_archive(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            capture = root / 'capture'
            capture.mkdir(mode=0o700)
            (capture / 'database.dump').write_bytes(b'synthetic dump')
            ops.write_json(capture / 'deployment.json', {'secretsIncluded': False})
            identity, recipients = root / 'identity', root / 'recipients'
            subprocess.run(['age-keygen', '-o', str(identity)], check=True, stderr=subprocess.DEVNULL)
            recipients.write_bytes(subprocess.check_output(['age-keygen', '-y', str(identity)]))
            for wa in (False, True):
                with self.subTest(empty_wa_state=wa):
                    (capture / 'manifest.json').unlink(missing_ok=True)
                    ops.make_manifest(capture, {'waStateIncluded': wa})
                    archive = root / ('wa.tar.age' if wa else 'plain.tar.age')
                    ops.encrypt_stage(capture, recipients, archive)
                    args = argparse.Namespace(archive=str(archive), identity=str(identity),
                                              expected_sha256=ops.digest(archive))
                    config = {'staging_dir': str(root / 'staging'),
                              'max_restore_bytes': 1024**2, 'max_restore_files': 10}
                    with ops.verified_archive(args, config) as (stage, manifest):
                        self.assertTrue((stage / 'attachments').is_dir())
                        self.assertEqual(list((stage / 'attachments').iterdir()), [])
                        self.assertEqual((stage / 'wa-state').is_dir(), wa)
                        self.assertEqual((stage / 'database.dump').read_bytes(), b'synthetic dump')

    def test_volume_marker_blocks_fallback(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary) / 'backup'
            config = {'backup_dir': str(root), 'backup_volume_id': 'test-volume'}
            with self.assertRaises(FileNotFoundError):
                ops.backup_destination(config)
            self.assertFalse(root.exists())
            root.mkdir(mode=0o700)
            (root / '.space-backup-volume-id').write_text('wrong-volume')
            with self.assertRaises(ops.OpsError):
                ops.backup_destination(config)
            (root / '.space-backup-volume-id').write_text('test-volume')
            self.assertEqual(ops.backup_destination(config), root)

    def test_database_uri_never_becomes_argv_or_implicit_fallback(self):
        with patch.dict(os.environ, {'SPACE_TEST_URI': 'postgres://operator:p%40ss@localhost:55479/space_test_fixture?sslmode=disable', 'PGPASSWORD': 'wrong'}, clear=True):
            env, uri = ops.pg_environment('SPACE_TEST_URI')
            self.assertEqual(env['PGPASSWORD'], 'p@ss')
            self.assertEqual(env['PGDATABASE'], 'space_test_fixture')
            self.assertEqual(env['PGSSLMODE'], 'disable')
            with self.assertRaises(ops.OpsError):
                ops.pg_environment('SPACE_RESTORE_DATABASE_URL')
        with patch.dict(os.environ, {'URI': 'postgres://localhost/db?options=-c%20anything'}):
            with self.assertRaises(ops.OpsError):
                ops.pg_environment('URI')

    def test_private_storage_rejects_symlink(self):
        with tempfile.TemporaryDirectory() as temporary:
            target = Path(temporary) / 'real'
            target.mkdir(mode=0o700)
            link = Path(temporary) / 'link'
            link.symlink_to(target, target_is_directory=True)
            with self.assertRaises(ops.OpsError):
                ops.private_directory(link)

    def test_backup_failure_resumes_only_services_active_before_capture(self):
        with tempfile.TemporaryDirectory() as temporary:
            config = {'staging_dir': temporary, 'server_units': ['fixture-api.service'],
                      'bridge_units': ['fixture-bridge.service']}
            Path(temporary).chmod(0o700)
            states = {'fixture-api.service': 'active', 'fixture-bridge.service': 'inactive'}
            actions = []
            def control(config, *args):
                if args[0] == 'show':
                    return 'loaded' if 'LoadState' in args[1] else states[args[-1]]
                actions.append(args)
                states[args[-1]] = 'inactive' if args[0] == 'stop' else 'active'
                return ''
            with patch.object(ops, 'systemctl', side_effect=control):
                with self.assertRaisesRegex(ops.OpsError, 'capture failed'):
                    with ops.quiesced_services(config, True):
                        self.assertEqual(states['fixture-api.service'], 'inactive')
                        raise ops.OpsError('capture failed')
            self.assertEqual(actions, [('stop', 'fixture-api.service'), ('start', 'fixture-api.service')])
            self.assertFalse((Path(temporary) / 'backup-services.json').exists())
            self.assertEqual(states['fixture-bridge.service'], 'inactive')

    def test_backup_refuses_unapproved_stop(self):
        with tempfile.TemporaryDirectory() as temporary:
            config = {'staging_dir': temporary, 'server_units': ['fixture-api.service'], 'bridge_units': []}
            def control(config, *args):
                return 'loaded' if 'LoadState' in args[1] else 'active'
            with patch.object(ops, 'systemctl', side_effect=control):
                with self.assertRaises(ops.OpsError):
                    with ops.quiesced_services(config, False):
                        self.fail('capture entered while server active')

    def test_legacy_date_preserves_deadline_and_precision(self):
        self.assertEqual(legacy.legacy_date('2026-11-08', 'iso_date'), '2026-11-08')
        self.assertEqual(legacy.legacy_date('1', 'google_serial'), '1899-12-31')
        for value in ('1.5', 'NaN', 'Infinity'):
            with self.assertRaises(legacy.ComparisonError):
                legacy.legacy_date(value, 'google_serial')
        with self.assertRaises(legacy.ComparisonError):
            legacy.legacy_date('08/11/26', 'iso_date')

    def test_csv_formula_and_duplicate_header_guards(self):
        self.assertEqual(legacy.spreadsheet_text(' =HYPERLINK("x")'), '\' =HYPERLINK("x")')
        with tempfile.TemporaryDirectory() as temporary:
            file = Path(temporary) / 'bad.csv'
            file.write_text('id,id\n1,2\n')
            with self.assertRaises(legacy.ComparisonError):
                legacy.rows(file)
            file.write_text('id,value\n1,2,3\n')
            with self.assertRaises(legacy.ComparisonError):
                legacy.rows(file)

if __name__ == '__main__':
    unittest.main()

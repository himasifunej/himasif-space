"""Real PostgreSQL/age restore drill of disposable synthetic data.

No production service control, network integrations, or WA pairing is invoked.
The archive capture below has no running application and uses the real lease.
"""
import argparse
import contextlib
import hashlib
import io
import json
import os
from pathlib import Path
import shutil
import socket
import subprocess
import tempfile
import time
import unittest
from unittest.mock import patch
from urllib.parse import urlsplit, urlunsplit
from urllib.request import urlopen
import uuid
import space_ops as ops


@unittest.skipUnless(os.environ.get('SPACE_OPS_TEST_SERVER_BIN'), 'restore integration needs the matching built server')
class IsolatedRestoreDrill(unittest.TestCase):
    def test_encrypted_dump_blob_restore_and_runtime_quarantine(self):
        for tool in ('age', 'age-keygen', 'psql', 'pg_dump', 'pg_restore', 'createdb'):
            self.assertIsNotNone(shutil.which(tool), tool)
        binary = str(Path(os.environ['SPACE_OPS_TEST_SERVER_BIN']).resolve())
        base = os.environ['SPACE_TEST_DATABASE_URL']
        uri = urlsplit(base)
        self.assertTrue(uri.path.startswith('/space_test_'))
        suffix = uuid.uuid4().hex
        source, target = 'space_test_ops_' + suffix, 'space_restore_ops_' + suffix
        with patch.dict(os.environ, {'OPS_ADMIN': base}):
            admin, _ = ops.pg_environment('OPS_ADMIN')
        ops.query(admin, 'create database ' + source)
        try:
            with tempfile.TemporaryDirectory(prefix='space-real-restore-') as temporary:
                root = Path(temporary)
                source_uri = urlunsplit((uri.scheme, uri.netloc, '/' + source, uri.query, ''))
                fixture_env = {key: value for key, value in os.environ.items() if not key.startswith('SPACE_')}
                fixture_env.update(SPACE_DATABASE_URL=source_uri, SPACE_RUN_MODE='maintenance',
                                   SPACE_ENV='development', SPACE_HTTP_ADDR='127.0.0.1:8787',
                                   SPACE_PUBLIC_URL='http://localhost:8787', SPACE_ADMIN_EMAIL='restore-fixture@himasif.test',
                                   SPACE_ADMIN_PASSWORD='synthetic-restore-fixture-password')
                subprocess.run([binary, 'seed'], env=fixture_env, check=True, stdout=subprocess.DEVNULL)
                with patch.dict(os.environ, {'OPS_SOURCE': source_uri}):
                    database, _ = ops.pg_environment('OPS_SOURCE')
                nid, aid, blob = [str(uuid.uuid4()) for _ in range(3)]
                data = b'synthetic private attachment survives encrypted restore'
                digest = hashlib.sha256(data).hexdigest()
                ops.query(database, f"""insert into notes(id,space_id,period_id,owner_division_id,title,content,template_kind,version,created_by,created_at,updated_at)
                    select '{nid}',s.id,s.period_id,sd.division_id,'Restore fixture','[]','free',1,u.id,now(),now()
                    from spaces s join space_divisions sd on sd.space_id=s.id cross join users u where s.kind='division' and u.kind='organization_admin' limit 1;
                    insert into note_attachments(id,note_id,name,url,content_type,size_bytes,status,added_by,created_at,storage_key,sha256)
                    select '{aid}',id,'fixture.txt','/private-fixture','text/plain',{len(data)},'ready',created_by,now(),'{blob}','{digest}' from notes where id='{nid}';""")
                stage = root / 'capture';stage.mkdir(mode=0o700)
                attachments = stage / 'attachments';attachments.mkdir(mode=0o700)
                file = attachments / (blob + '.blob');file.write_bytes(data);file.chmod(0o600)
                with ops.exclusive_runtime_lease(database):
                    counts = ops.counts(database)
                    ops.command(['pg_dump', '--format=custom', '--no-owner', '--no-privileges', '--file', str(stage / 'database.dump')], env=database)
                    self.assertEqual(ops.check_blob_references(database, stage), 1)
                ops.write_json(stage / 'deployment.json', {'releaseId': 'restore-fixture', 'secretsIncluded': False})
                ops.make_manifest(stage, {'backupId': suffix, 'rowCounts': counts, 'strategy': 'isolated_fixture_no_server'})
                identity, recipients = root / 'identity', root / 'recipients'
                subprocess.run(['age-keygen', '-o', str(identity)], check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
                recipients.write_bytes(subprocess.check_output(['age-keygen', '-y', str(identity)]))
                archive = root / 'fixture.tar.age';ops.encrypt_stage(stage, recipients, archive)
                policy = root / 'policy.json'
                config = {'backup_dir': str(root / 'backup'), 'staging_dir': str(root / 'staging'),
                          'attachments_dir': str(attachments), 'age_recipients_file': str(recipients),
                          'wa_state_dir': None, 'server_units': ['synthetic-fixture.service'], 'bridge_units': [],
                          'minimum_copies': 1, 'retention_days': None}
                policy.write_text(json.dumps(config))
                args = argparse.Namespace(policy=str(policy), archive=str(archive), expected_sha256=ops.digest(archive),
                                          identity=str(identity), target_db=target, target_root=str(root / 'restored'), server_bin=binary)
                with contextlib.redirect_stdout(io.StringIO()):
                    ops.verify(args)
                    with patch.dict(os.environ, {'SPACE_RESTORE_DATABASE_URL': base}):
                        ops.restore(args)
                restored = root / 'restored'
                self.assertEqual((restored / 'attachments' / (blob + '.blob')).read_bytes(), data)
                target_uri = urlunsplit((uri.scheme, uri.netloc, '/' + target, uri.query, ''))
                with patch.dict(os.environ, {'OPS_TARGET': target_uri}):
                    target_env, _ = ops.pg_environment('OPS_TARGET')
                self.assertEqual(ops.query(target_env, 'select mode from runtime_control'), 'isolated_restore')
                self.assertEqual(ops.query(target_env, "select current_setting('space.restore_quarantined')"), 'true')
                # A live attempt must fail closed before it can listen or start workers.
                live = dict(fixture_env, SPACE_DATABASE_URL=target_uri, SPACE_RUN_MODE='live')
                failed = subprocess.run([binary, 'serve'], env=live, capture_output=True, timeout=15)
                self.assertNotEqual(failed.returncode, 0)
                self.assertIn(b'quarantined', failed.stderr)
                with socket.socket() as probe:
                    probe.bind(('127.0.0.1', 0));port = probe.getsockname()[1]
                isolated = dict(live, SPACE_RUN_MODE='isolated_restore', SPACE_HTTP_ADDR=f'127.0.0.1:{port}',
                                SPACE_PUBLIC_URL=f'http://localhost:{port}', SPACE_ATTACHMENT_DIR=str(restored / 'attachments'))
                process = subprocess.Popen([binary, 'serve'], env=isolated, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
                try:
                    deadline = time.monotonic() + 15
                    while True:
                        try:
                            with urlopen(f'http://localhost:{port}/api/health', timeout=2) as response:
                                self.assertEqual(response.headers['X-Space-Run-Mode'], 'isolated_restore')
                                self.assertEqual(json.load(response)['database'], 'ok')
                            break
                        except OSError:
                            if time.monotonic() >= deadline:raise
                            time.sleep(0.1)
                    self.assertEqual(ops.query(target_env, "select count(*) from worker_health where state not in ('disabled','stopped')"), '0')
                    self.assertEqual(ops.query(target_env, 'select count(*) from wa_outbox'), '0')
                finally:
                    process.terminate();process.wait(timeout=20)
        finally:
            for name in (source, target):
                ops.query(admin, 'drop database if exists ' + name + ' with (force)')

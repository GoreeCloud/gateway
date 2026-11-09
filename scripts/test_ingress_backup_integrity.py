"""Synthetic local backups only. No production ingress, certificates or credentials."""
import hashlib
import io
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest import mock
from contextlib import redirect_stdout

from ingress_backup_integrity import build_report, main, verify_archive


class OfflineIngressBackupDigestTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.root = Path(self.temp.name)
        self.config = self.root / "confidential-config.tar"
        self.acme = self.root / "confidential-acme.tar"
        self.config.write_bytes(b"synthetic-secret-NEVER-LOG-abc")
        self.acme.write_bytes(b"synthetic-cert-NEVER-LOG-xyz")
        self.config.chmod(0o600)
        self.acme.chmod(0o600)

    def tearDown(self):
        self.temp.cleanup()

    @staticmethod
    def checksum(path):
        return hashlib.sha256(path.read_bytes()).hexdigest()

    def args(self):
        return [
            "--config-archive", str(self.config),
            "--config-sha256", self.checksum(self.config),
            "--acme-archive", str(self.acme),
            "--acme-sha256", self.checksum(self.acme),
        ]

    def capture(self, args):
        output = io.StringIO()
        with redirect_stdout(output):
            status = main(args)
        report = json.loads(output.getvalue())
        for secret in ("synthetic-secret", "synthetic-cert", str(self.root),
                       self.config.name, self.acme.name):
            self.assertNotIn(secret, output.getvalue())
        return status, report

    def test_verified_synthetic_archives_are_not_restore_acceptance(self):
        code, result = self.capture(self.args())
        self.assertEqual(code, 0)
        self.assertTrue(result["both_hashes_match"])
        self.assertEqual(result["archive_restoration"], "not_verified")
        self.assertEqual(result["porkbun_dns01_renewal"], "not_verified")
        self.assertEqual(result["rollback_test"], "not_verified")
        self.assertFalse(result["production_changed"])
        self.assertEqual(result["cutover_readiness"], "not_established")

    def test_altered_backups_fail_closed(self):
        original = self.args()
        self.acme.write_bytes(self.acme.read_bytes() + b"tampered")
        status, report = self.capture(original)
        self.assertEqual(status, 1)
        self.assertEqual(report["acme_archive_integrity"], "digest_mismatch")
        self.assertFalse(report["both_hashes_match"])

    def test_expected_checksum_must_be_canonical_lowercase_sha256(self):
        for value in ("", "x" * 64, "A" * 64, "0" * 63, "0" * 65, 0, None):
            with self.subTest(value=value):
                self.assertEqual(verify_archive(self.config, value), "invalid_expected_digest")

    def test_symlink_hardlink_directory_and_missing_are_rejected(self):
        link = self.root / "linked"
        link.symlink_to(self.config)
        self.assertEqual(verify_archive(link, self.checksum(self.config)), "unavailable_or_unsafe")
        hard = self.root / "hardlinked"
        os.link(self.acme, hard)
        self.assertEqual(verify_archive(hard, self.checksum(self.acme)), "unsafe_file_properties")
        self.assertEqual(verify_archive(self.root, self.checksum(self.config)), "unsafe_file_properties")
        self.assertEqual(verify_archive(self.root / "absent", self.checksum(self.config)), "unavailable_or_unsafe")
        # Opening a FIFO without O_NONBLOCK can hang the verifier indefinitely.
        fifo = self.root / "named-pipe"
        os.mkfifo(fifo, 0o600)
        self.assertEqual(verify_archive(fifo, self.checksum(self.config)), "unsafe_file_properties")

    def test_world_or_group_readable_archive_rejected(self):
        for mode in (0o640, 0o604, 0o644):
            with self.subTest(mode=oct(mode)):
                self.config.chmod(mode)
                self.assertEqual(verify_archive(self.config, self.checksum(self.config)), "unsafe_file_properties")
        self.config.chmod(0o600)

    def test_non_owner_archive_is_rejected_before_hashing(self):
        # Simulate a privileged process that can read someone else's 0600
        # archive: filesystem readability alone is not ownership evidence.
        with mock.patch("ingress_backup_integrity.os.geteuid",
                        return_value=os.geteuid() + 1):
            self.assertEqual(
                verify_archive(self.config, self.checksum(self.config)),
                "unsafe_file_properties",
            )
            code, report = self.capture(self.args())
            self.assertEqual(code, 1)
            self.assertEqual(report["config_archive_integrity"], "unsafe_file_properties")
            self.assertEqual(report["acme_archive_integrity"], "unsafe_file_properties")
            self.assertFalse(report["both_hashes_match"])
            self.assertEqual(report["cutover_readiness"], "not_established")

    def test_empty_file_rejected_and_report_fail_closed(self):
        self.config.write_bytes(b"")
        self.assertEqual(verify_archive(self.config, hashlib.sha256(b"").hexdigest()), "unsafe_file_properties")
        result = build_report("matched", "digest_mismatch")
        self.assertFalse(result["both_hashes_match"])
        self.assertEqual(result["cutover_readiness"], "not_established")

    def test_output_never_exposes_input_paths_even_on_error(self):
        args = self.args()
        args[3] = "deadbeef"
        status, report = self.capture(args)
        self.assertEqual(status, 1)
        self.assertEqual(report["config_archive_integrity"], "invalid_expected_digest")


if __name__ == "__main__":
    unittest.main()

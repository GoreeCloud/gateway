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
        self.assertTrue(result["distinct_archive_sources_verified"])
        self.assertEqual(result["archive_restoration"], "not_verified")
        self.assertEqual(result["porkbun_dns01_renewal"], "not_verified")
        self.assertEqual(result["rollback_test"], "not_verified")
        self.assertFalse(result["production_changed"])
        self.assertEqual(result["cutover_readiness"], "not_established")

    def test_reused_same_inode_cannot_claim_two_independent_backups(self):
        digest = self.checksum(self.config)
        same_file = [
            "--config-archive", str(self.config),
            "--config-sha256", digest,
            "--acme-archive", str(self.config),
            "--acme-sha256", digest,
        ]
        status, report = self.capture(same_file)
        self.assertEqual(status, 1)
        self.assertEqual(report["config_archive_integrity"], "matched")
        self.assertEqual(report["acme_archive_integrity"], "matched")
        self.assertFalse(report["distinct_archive_sources_verified"])
        self.assertFalse(report["both_hashes_match"])
        self.assertEqual(report["cutover_readiness"], "not_established")
        # A syntactically different path resolving to the same inode fails too.
        child = self.root / "alias-dir"
        child.mkdir()
        same_file[5] = str(child / ".." / self.config.name)
        status, report = self.capture(same_file)
        self.assertEqual(status, 1)
        self.assertFalse(report["distinct_archive_sources_verified"])

    def test_distinct_inodes_remain_distinct_even_with_identical_bytes(self):
        self.acme.write_bytes(self.config.read_bytes())
        self.acme.chmod(0o600)
        status, report = self.capture(self.args())
        self.assertEqual(status, 0)
        self.assertTrue(report["distinct_archive_sources_verified"])
        self.assertTrue(report["both_hashes_match"])
        # This does not establish that either file has the correct backup role.
        self.assertEqual(report["archive_restoration"], "not_verified")

    def test_report_helper_defaults_to_unverified_pair_independence(self):
        report = build_report("matched", "matched")
        self.assertFalse(report["both_hashes_match"])
        self.assertFalse(report["distinct_archive_sources_verified"])
        self.assertEqual(report["cutover_readiness"], "not_established")

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

    def test_parent_directory_must_be_private_and_owner_controlled(self):
        unsafe = self.root / "public-parent"
        unsafe.mkdir(mode=0o755)
        candidate = unsafe / "config.tar"
        candidate.write_bytes(self.config.read_bytes())
        candidate.chmod(0o600)
        self.assertEqual(
            verify_archive(candidate, self.checksum(candidate)),
            "unsafe_parent_directory",
        )
        # A public parent is refused even with matching expected hash.
        args = self.args()
        args[1] = str(candidate)
        args[3] = self.checksum(candidate)
        code, result = self.capture(args)
        self.assertEqual(code, 1)
        self.assertEqual(result["config_archive_integrity"],
                         "unsafe_parent_directory")
        self.assertEqual(result["cutover_readiness"], "not_established")
        unsafe.chmod(0o700)
        self.assertEqual(
            verify_archive(candidate, self.checksum(candidate)), "matched"
        )

    def test_parent_symlink_is_rejected_even_if_archive_is_private(self):
        linked_parent = self.root / "symlink-parent"
        linked_parent.symlink_to(self.root, target_is_directory=True)
        candidate = linked_parent / self.config.name
        self.assertEqual(
            verify_archive(candidate, self.checksum(self.config)),
            "unavailable_or_unsafe",
        )
        self.assertEqual(
            verify_archive(self.config, self.checksum(self.config)), "matched"
        )

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

"""Only local, synthetic tar fixtures. No backups, secrets or restoration."""
import io
import json
import os
from pathlib import Path
import tarfile
import tempfile
import unittest
from contextlib import redirect_stdout

from offline_archive_structure import (
    ArchiveGateError, inspect_archive, main, safe_member_name,
)


class OfflineTarHeadersTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.root = Path(self.temp.name)
        self.archive = self.root / "private-sensitive-backup.tar"

    def tearDown(self):
        self.temp.cleanup()

    def make_archive(self, items, mode="w"):
        with tarfile.open(self.archive, mode) as tf:
            for name, kind, content in items:
                member = tarfile.TarInfo(name)
                if kind == "file":
                    data = content.encode("utf-8")
                    member.size = len(data)
                    tf.addfile(member, io.BytesIO(data))
                elif kind == "dir":
                    member.type = tarfile.DIRTYPE
                    tf.addfile(member)
                elif kind in ("symlink", "hardlink", "fifo"):
                    member.type = {"symlink": tarfile.SYMTYPE,
                                   "hardlink": tarfile.LNKTYPE,
                                   "fifo": tarfile.FIFOTYPE}[kind]
                    member.linkname = "outside-source"
                    tf.addfile(member)
                else:
                    raise AssertionError(kind)
        self.archive.chmod(0o600)

    def capture(self):
        output = io.StringIO()
        with redirect_stdout(output):
            status = main([str(self.archive)])
        body = json.loads(output.getvalue())
        for secret in ("confidential-content", "private-sensitive",
                       str(self.root), "outside-source"):
            self.assertNotIn(secret, output.getvalue())
        return status, body

    def test_private_regular_tar_directory_and_file_pass_without_restore_claim(self):
        self.make_archive([("config", "dir", ""), ("config/Caddyfile", "file",
                           "confidential-content"), ("acme/cert.pem", "file", "test")])
        status, report = self.capture()
        self.assertEqual(status, 0)
        self.assertTrue(report["structural_gate_passed"])
        self.assertEqual(report["regular_member_count"], 2)
        self.assertEqual(report["directory_member_count"], 1)
        for flag in ("archive_sha256_verified", "archive_content_semantics_verified",
                     "isolated_restore_verified", "acme_renewal_verified",
                     "rollback_verified", "production_changed",
                     "production_cutover_authorized"):
            self.assertFalse(report[flag])

    def test_gzip_archive_is_only_header_checked(self):
        self.make_archive([("synthetic.txt", "file", "confidential-content")], mode="w:gz")
        self.assertEqual(self.capture()[0], 0)

    def test_path_traversal_absolute_backslash_controls_and_casefold_collision(self):
        for path in ("../etc/passwd", "/root/.ssh/id_rsa", "folder/../../private",
                     "C:/secrets", "folder\\secret", "foo//bar", "foo//", "foo/./bar",
                     "line\nfeed", "raw\x00nul"):
            with self.subTest(path=repr(path)), self.assertRaises(ArchiveGateError):
                safe_member_name(path)
        self.make_archive([("TEST", "file", "x"), ("test", "file", "y")])
        self.assertEqual(self.capture()[0], 1)

    def test_archive_members_symlink_hardlink_fifo_fail_closed(self):
        for kind in ("symlink", "hardlink", "fifo"):
            with self.subTest(kind=kind):
                self.make_archive([("valid.txt", "file", "hello"),
                                   ("unsafe", kind, "")])
                self.assertEqual(self.capture()[0], 1)

    def test_owner_unsafe_perms_aliases_and_missing_fail_closed(self):
        self.make_archive([("synthetic", "file", "data")])
        self.archive.chmod(0o644)
        self.assertEqual(self.capture()[0], 1)
        self.archive.chmod(0o600)
        alias = self.root / "alias"
        alias.symlink_to(self.archive)
        with self.assertRaises(ArchiveGateError):
            inspect_archive(alias)
        os.link(self.archive, self.root / "hardlinked")
        self.assertEqual(self.capture()[0], 1)

    def test_fifo_and_nonregular_fail_without_blocking(self):
        fifo = self.root / "fifo"
        os.mkfifo(fifo, 0o600)
        with self.assertRaises(ArchiveGateError):
            inspect_archive(fifo)
        with self.assertRaises(ArchiveGateError):
            inspect_archive(self.root)

    def test_random_file_and_empty_archive_rejected(self):
        self.archive.write_bytes(b"not a tar!")
        self.archive.chmod(0o600)
        self.assertEqual(self.capture()[0], 1)
        self.make_archive([])
        self.assertEqual(self.capture()[0], 1)

    def test_file_with_only_safe_dir_is_rejected(self):
        self.make_archive([("empty", "dir", "")])
        self.assertEqual(self.capture()[0], 1)

    def test_oversized_entry_is_rejected_before_unbounded_processing(self):
        # A crafted tar header declares >4 GiB but contains no payload.
        # Reject on header metadata, before streaming its absent contents.
        from offline_archive_structure import MAX_ENTRY_BYTES
        oversized = tarfile.TarInfo("huge.bin")
        oversized.size = MAX_ENTRY_BYTES + 1
        self.archive.write_bytes(oversized.tobuf(format=tarfile.USTAR_FORMAT)
                                 + b"\\0" * 1024)
        self.archive.chmod(0o600)
        self.assertEqual(self.capture()[0], 1)

    def test_reporting_is_generic_even_if_filename_contains_sensitive_data(self):
        self.make_archive([("confidential-content", "file", "private-sensitive")])
        status, report = self.capture()
        self.assertEqual(status, 0)
        self.assertFalse(report["production_cutover_authorized"])


if __name__ == "__main__":
    unittest.main()

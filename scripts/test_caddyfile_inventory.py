import io
import json
import os
import tempfile
import unittest
from types import SimpleNamespace
from unittest.mock import patch
from contextlib import redirect_stdout
from pathlib import Path

from caddyfile_inventory import count_features, main, read_bounded, MAX_SIZE

FIXTURE = """
# This example uses PRIVATE-CUSTOMER.test and contains a synthetic token NEVER-PRINT-THIS.
https://PRIVATE-CUSTOMER.test {
    tls {
        dns porkbun {
            api_key {env.PORKBUN_API_KEY}
            api_secret_key {env.PORKBUN_API_SECRET_KEY}
        }
    }
    encode gzip
    header {
        X-Frame-Options "DENY"
    }
    @private path /internal/*
    handle @private {
        basic_auth {
            test {env.GATEWAY_TEST_HASH}
        }
        reverse_proxy 127.0.0.1:18002
    }
    handle {
        respond "NEVER-PRINT-THIS" 200
        reverse_proxy 127.0.0.1:18003
    }
}
"""


class InventoryTest(unittest.TestCase):
    def test_synthetic_directives_and_redaction(self):
        result = count_features(FIXTURE)
        self.assertEqual(result["analysis"], "text_counts_only_not_config_validation")
        self.assertEqual(result["porkbun_dns_directives"], 1)
        self.assertEqual(result["dns_provider_directives"], 1)
        self.assertEqual(result["directives"]["reverse_proxy"], 2)
        self.assertEqual(result["directives"]["handle"], 2)
        self.assertEqual(result["directives"]["basic_auth"], 1)
        self.assertEqual(result["directives"]["header"], 1)
        self.assertEqual(result["directives"]["tls"], 1)
        self.assertTrue(result["braces_balanced_textually"])
        self.assertFalse(result["inline_provider_credentials_suspected"])
        output = json.dumps(result)
        for private in ("PRIVATE-CUSTOMER", "NEVER-PRINT-THIS", "18002", "PORKBUN_API_KEY"):
            self.assertNotIn(private, output)

    def test_detects_inline_provider_values_without_leaking(self):
        source = 'dns porkbun {\n api_key danger-secret-value\n api_secret_key {env.SAFE_NAME}\n}\n'
        output = json.dumps(count_features(source))
        self.assertIn('"inline_provider_credentials_suspected": true', output)
        self.assertNotIn("danger-secret-value", output)

    def test_matches_only_leading_complete_directives(self):
        s = "  # reverse_proxy fake\nxreverse_proxy nope\nheader_up nope\n  reverse_proxy 127.0.0.1:1\n"
        self.assertEqual(count_features(s)["directives"]["reverse_proxy"], 1)
        self.assertEqual(count_features(s)["directives"]["header"], 0)

    def test_bounds_file_and_handles_decode_error(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "test.Caddyfile"
            path.write_bytes(b"x" * (MAX_SIZE + 1))
            with self.assertRaises(ValueError):
                read_bounded(path)
            path.write_bytes(b"\xff")
            with self.assertRaises(UnicodeError):
                read_bounded(path)

    def test_final_symlink_and_fifo_do_not_redirect_or_block(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            real = root / "private.Caddyfile"
            real.write_text(FIXTURE)
            alias = root / "link.Caddyfile"
            alias.symlink_to(real)
            with self.assertRaises(OSError):
                read_bounded(alias)
            fifo = root / "fifo.Caddyfile"
            os.mkfifo(fifo, 0o600)
            with self.assertRaises(ValueError):
                read_bounded(fifo)
            self.assertEqual(read_bounded(real), FIXTURE)

    def test_changed_inode_metadata_rejected_even_without_content_difference(self):
        with tempfile.TemporaryDirectory() as tmp:
            target = Path(tmp) / "private.Caddyfile"
            target.write_text(FIXTURE)
            original_fstat = os.fstat
            count = 0

            def changed_after_read(fd):
                nonlocal count
                count += 1
                original = original_fstat(fd)
                if count == 2:
                    return SimpleNamespace(
                        st_dev=original.st_dev, st_ino=original.st_ino,
                        st_size=original.st_size,
                        st_mtime_ns=original.st_mtime_ns + 1,
                        st_ctime_ns=original.st_ctime_ns,
                    )
                return original

            with patch("caddyfile_inventory.os.fstat", side_effect=changed_after_read):
                with self.assertRaises(ValueError):
                    read_bounded(target)
            self.assertEqual(count, 2)
            self.assertEqual(read_bounded(target), FIXTURE)

    def test_fdopen_failure_closes_owned_descriptor_without_path_disclosure(self):
        with tempfile.TemporaryDirectory() as tmp:
            target = Path(tmp) / "secret-do-not-log.Caddyfile"
            target.write_text(FIXTURE)
            opened = []
            def fail_wrapper(fd, *_args, **_kwargs):
                opened.append(fd)
                raise OSError("synthetic confidential wrapper failure")
            with patch("caddyfile_inventory.os.fdopen", side_effect=fail_wrapper):
                with self.assertRaises(OSError):
                    read_bounded(target)
            self.assertEqual(len(opened), 1)
            with self.assertRaises(OSError):
                os.fstat(opened[0])
            self.assertEqual(read_bounded(target), FIXTURE)

    def test_empty_file_counts_only(self):
        result = count_features("# comment\n")
        self.assertEqual(result["active_lines"], 0)
        self.assertTrue(result["braces_balanced_textually"])


if __name__ == "__main__":
    unittest.main()

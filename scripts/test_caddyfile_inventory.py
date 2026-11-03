import io
import json
import tempfile
import unittest
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

    def test_empty_file_counts_only(self):
        result = count_features("# comment\n")
        self.assertEqual(result["active_lines"], 0)
        self.assertTrue(result["braces_balanced_textually"])


if __name__ == "__main__":
    unittest.main()

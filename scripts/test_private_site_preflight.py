"""Offline, synthetic tests for read-only, hostname-redacted Caddy preflight."""
import io
import json
import tempfile
import unittest
from contextlib import redirect_stdout
from pathlib import Path
from unittest.mock import patch

from private_site_preflight import assess, classify_address, main, valid_target


TARGET = "openchat-dev.goreecloud.com"
SECRET = "PRIVATE-NOT-FOR-LOGS-12345"


class PrivateSitePreflightTests(unittest.TestCase):
    def test_explicit_and_implicit_root_sites(self):
        for declaration, expected in (
            ("https://openchat-dev.goreecloud.com", "explicit_https_candidate"),
            ("openchat-dev.goreecloud.com", "implicit_https_candidate"),
            ("openchat-dev.goreecloud.com:443", "implicit_https_candidate"),
            ("example.test, https://openchat-dev.goreecloud.com", "explicit_https_candidate"),
        ):
            with self.subTest(declaration=declaration):
                caddyfile = declaration + " {\n tls {\n dns porkbun {\n api_key " + SECRET + "\n }\n }\n }\n"
                report = assess(caddyfile, TARGET)
                self.assertEqual(report["site_declaration"], expected)
                serialized = json.dumps(report)
                self.assertNotIn(TARGET, serialized)
                self.assertNotIn(SECRET, serialized)
                self.assertNotIn("porkbun", serialized)

    def test_does_not_confuse_comments_nested_rules_or_similar_hostnames(self):
        caddyfile = (
            "# https://openchat-dev.goreecloud.com {\n"
            "https://other.goreecloud.com {\n"
            '  respond "https://openchat-dev.goreecloud.com { ' + SECRET + '" 200\n'
            "  @private {\n"
            "    host openchat-dev.goreecloud.com\n"
            "  }\n"
            "}\n"
            "https://evilopenchat-dev.goreecloud.com {\n respond ok\n}\n"
        )
        report = assess(caddyfile, TARGET)
        self.assertEqual(report["site_declaration"], "absent_in_visible_root_headers")
        self.assertTrue(report["balanced_braces_textually"])
        self.assertFalse(report["unresolved_imports"])

    def test_wildcard_one_label_only_and_scheme(self):
        self.assertEqual(classify_address("*.goreecloud.com", TARGET), "wildcard_candidate")
        self.assertEqual(classify_address("*.goreecloud.com", "goreecloud.com"), None)
        self.assertEqual(classify_address("*.goreecloud.com", "sub.openchat-dev.goreecloud.com"), None)
        self.assertEqual(classify_address("http://openchat-dev.goreecloud.com", TARGET), "http_only")
        self.assertEqual(classify_address("openchat-dev.goreecloud.com:80", TARGET), "http_only")
        self.assertIsNone(classify_address("openchat-dev.goreecloud.com:99999", TARGET))
        self.assertIsNone(classify_address("fake-openchat-dev.goreecloud.com", TARGET))

    def test_imports_and_malformed_braces_fail_indeterminate(self):
        self.assertEqual(assess("import private/*.caddy\n", TARGET)["site_declaration"], "indeterminate")
        self.assertEqual(assess("https://other.test {\n", TARGET)["site_declaration"], "indeterminate")
        self.assertEqual(
            assess("https://openchat-dev.goreecloud.com {\n", TARGET)["site_declaration"],
            "indeterminate",
        )
        self.assertTrue(assess("import private/*.caddy\n", TARGET)["unresolved_imports"])

    def test_matching_root_site_with_unresolved_import_fails_closed(self):
        source = (
            "https://openchat-dev.goreecloud.com {\\n respond ok\\n}\\n"
            "import hidden/*.caddy\\n"
        )
        report = assess(source, TARGET)
        self.assertEqual(report["site_declaration"], "indeterminate")
        self.assertTrue(report["unresolved_imports"])
        self.assertTrue(report["balanced_braces_textually"])
        self.assertNotIn(TARGET, json.dumps(report))
        with tempfile.TemporaryDirectory() as tmp:
            config = Path(tmp) / "private.Caddyfile"
            config.write_text(source, encoding="utf-8")
            stream = io.StringIO()
            with patch("sys.argv", ["private_site_preflight.py", str(config), TARGET]):
                with redirect_stdout(stream):
                    self.assertEqual(main(), 2)
            self.assertEqual(json.loads(stream.getvalue())["site_declaration"], "indeterminate")

    def test_invalid_target_rejected_without_echoing(self):
        for value in ("evil.com@local", "one", "*.goreecloud.com", "openchat-dev.goreecloud.com/path",
                      "bad..goreecloud.com", "a" * 260):
            with self.subTest(value=value):
                self.assertIsNone(valid_target(value))
                with self.assertRaises(ValueError):
                    assess("example.test {\n}\n", value)

    def test_cli_returns_redacted_fixed_field_json_and_exit_code(self):
        with tempfile.TemporaryDirectory() as tmp:
            config = Path(tmp) / "private.Caddyfile"
            for body, expected_exit, expected_status in (
                ("other.test {\n respond ok\n}\n", 1, "absent_in_visible_root_headers"),
                ("https://openchat-dev.goreecloud.com {\n respond ok\n}\n", 0, "explicit_https_candidate"),
                ("import another.Caddyfile\n", 2, "indeterminate"),
            ):
                config.write_text(body + "# " + SECRET, encoding="utf-8")
                stream = io.StringIO()
                with patch("sys.argv", ["private_site_preflight.py", str(config), TARGET]):
                    with redirect_stdout(stream):
                        code = main()
                report = json.loads(stream.getvalue())
                self.assertEqual(code, expected_exit)
                self.assertEqual(report["site_declaration"], expected_status)
                self.assertNotIn(TARGET, stream.getvalue())
                self.assertNotIn(SECRET, stream.getvalue())

    def test_cli_does_not_leak_input_path_on_error(self):
        out = io.StringIO()
        with patch("sys.argv", ["private_site_preflight.py", "/nonexistent/private-path", TARGET]):
            with redirect_stdout(out):
                code = main()
        self.assertEqual(code, 2)
        self.assertEqual(json.loads(out.getvalue())["error"], "cannot_assess_safe_input")
        self.assertNotIn("private-path", out.getvalue())


if __name__ == "__main__":
    unittest.main()

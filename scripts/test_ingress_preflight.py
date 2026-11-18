"""Synthetic Docker fixtures only; these tests never contact a live host."""
import contextlib
import io
import unittest
from unittest import mock
import ingress_preflight as audit


class PreflightTest(unittest.TestCase):
    def test_known_version_and_no_readiness_claim(self):
        result = audit.assess(
            True, False,
            "80/tcp -> 0.0.0.0:80\n443/tcp -> [::]:443\n443/udp -> 0.0.0.0:443\n",
            "/etc/caddy|bind\n/data|bind\n/config|bind\n",
            "v2.11.4 h1:some-hash", "dns.providers.porkbun\n")
        self.assertEqual(result["caddy_binary_version"], "v2.11.4")
        self.assertTrue(result["caddy_pre_2_11_5"])
        self.assertFalse(result["ready_for_production_cutover"])
        self.assertTrue(result["porkbun_module_registered"])
        self.assertTrue(all(result["published_port_presence"].values()))
        self.assertEqual(result["backup_restore"], "not_verified")
        self.assertEqual(result["certificate_health"], "not_verified")

    def test_updated_engine_does_not_imply_safe_cutover(self):
        result = audit.assess(True, True, "80/tcp -> IP:80", "", "v2.11.9 h1:x", "")
        self.assertFalse(result["caddy_pre_2_11_5"])
        self.assertFalse(result["ready_for_production_cutover"])
        self.assertFalse(result["porkbun_module_registered"])

    def test_unparseable_version_fails_closed(self):
        result = audit.assess(True, None, "", "", "secret-error: credential", "")
        self.assertIsNone(result["caddy_binary_version"])
        self.assertIn("caddy_version_unverified", result["open_gates"])
        self.assertNotIn("secret-error", str(result))

    def test_invalid_name_before_docker(self):
        with mock.patch.object(audit, "docker_read", side_effect=AssertionError("Docker called")):
            with self.assertRaises(audit.InspectionError):
                audit.inspect("../bad")

    def test_command_outputs_summarized_only(self):
        fixture = [
            "true|false", "80/tcp -> 10.0.0.8:80\n443/tcp -> 10.0.0.8:443",
            "/data|bind\n/very-private-host-location|bind\n",
            "v2.11.4 h1:private-hash",
            "dns.providers.porkbun\nhttp.handlers.reverse_proxy\n",
        ]
        with mock.patch.object(audit, "docker_read", side_effect=fixture) as run:
            summary = audit.inspect("caddy")
            self.assertEqual(run.call_count, 5)
        output = str(summary)
        self.assertNotIn("10.0.0.8", output)
        self.assertNotIn("very-private-host-location", output)
        self.assertNotIn("private-hash", output)

    def test_default_does_not_read_docker(self):
        with mock.patch.object(audit, "inspect", side_effect=AssertionError("Inspected live!")):
            with contextlib.redirect_stdout(io.StringIO()) as output:
                code = audit.main([])
        self.assertEqual(code, 0)
        self.assertIn("--live", output.getvalue())

    def test_failed_docker_commands_redact_stderr(self):
        fake = mock.Mock(returncode=1, stdout="", stderr="sensitive credentials abc123")
        with mock.patch.object(audit.subprocess, "run", return_value=fake):
            with self.assertRaisesRegex(audit.InspectionError, "^docker_read_failed$"):
                audit.docker_read(["inspect", "caddy"])


if __name__ == "__main__":
    unittest.main()

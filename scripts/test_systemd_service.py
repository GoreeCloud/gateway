"""Static, offline fail-closed checks for the optional Linux service example."""

import configparser
from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[1]
UNIT = ROOT / "examples" / "systemd" / "goreecloud-gateway.service"


class SystemdTemplateTests(unittest.TestCase):
    def setUp(self):
        self.assertTrue(UNIT.is_file())
        parser = configparser.ConfigParser(interpolation=None)
        parser.optionxform = str
        with UNIT.open(encoding="utf-8") as stream:
            parser.read_file(stream)
        self.unit = parser
        self.service = parser["Service"]

    def test_dedicated_user_and_private_state(self):
        service = self.service
        self.assertEqual(service["User"], "goreecloud-gateway")
        self.assertEqual(service["Group"], "goreecloud-gateway")
        self.assertEqual(service["StateDirectory"], "goreecloud-gateway")
        self.assertEqual(service["StateDirectoryMode"], "0700")
        self.assertEqual(service["UMask"], "0077")
        self.assertEqual(service["ReadWritePaths"], "/var/lib/goreecloud-gateway")
        self.assertEqual(service["ProtectSystem"], "strict")
        self.assertEqual(service["ProtectHome"], "true")

    def test_loopback_only_default_and_separate_secret(self):
        service = self.service
        command = service["ExecStart"]
        self.assertIn("127.0.0.1:8787", command)
        self.assertIn("--state-dir /var/lib/goreecloud-gateway/managed", command)
        self.assertIn("--sites /var/lib/goreecloud-gateway/sites.json", command)
        self.assertNotIn("--allow-write", command)
        self.assertNotIn("0.0.0.0:8787", command)
        self.assertEqual(service["EnvironmentFile"], "/etc/goreecloud-gateway/gateway.env")
        text = UNIT.read_text(encoding="utf-8")
        self.assertNotIn("GATEWAY_ADMIN_TOKEN=", text)

    def test_restricted_capabilities_and_safe_privileges(self):
        service = self.service
        self.assertEqual(service["NoNewPrivileges"], "true")
        self.assertEqual(service["CapabilityBoundingSet"], "CAP_NET_BIND_SERVICE")
        self.assertEqual(service["AmbientCapabilities"], "CAP_NET_BIND_SERVICE")
        self.assertEqual(service["RestrictAddressFamilies"], "AF_UNIX AF_INET AF_INET6")
        self.assertEqual(service["PrivateTmp"], "true")
        self.assertEqual(service["PrivateDevices"], "true")


if __name__ == "__main__":
    unittest.main()

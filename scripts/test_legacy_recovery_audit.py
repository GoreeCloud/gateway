import io
import json
import unittest
from contextlib import redirect_stdout
from unittest.mock import patch

from legacy_recovery_audit import (
    build_report,
    category,
    listener_counts,
    live_report,
    main,
    mount_state,
)


class LegacyRecoveryAuditTest(unittest.TestCase):
    def setUp(self):
        self.metadata = {
            "status": "exited",
            "exit_code": 255,
            "error": "failed to set up networking: Address already in use; private-key-not-for-logs",
            "restart_policy": "unless-stopped",
            "network_mode": "private-ingress",
            "mounts": [
                {"Destination": "/data", "Source": "/secret/caddy-certs"},
                {"Destination": "/config", "Source": "/secret/caddy-config"},
                {"Destination": "/etc/caddy/Caddyfile", "Source": "/secret/real-domain-Caddyfile"},
            ],
            "networks": {
                "private-ingress": {"EndpointID": ""}
            },
            "environment": ["PORKBUN_API_KEY=secret_value_must_never_escape"],
        }
        self.sockets = "\n".join([
            "tcp LISTEN 0 4096 0.0.0.0:80 0.0.0.0:*",
            "tcp LISTEN 0 4096 [::]:443 [::]:*",
            "udp UNCONN 0 0 0.0.0.0:443 0.0.0.0:*",
            "tcp LISTEN 0 4096 127.0.0.1:2019 0.0.0.0:*",
            "tcp LISTEN 0 4096 0.0.0.0:18080 0.0.0.0:*",
        ])

    def test_fixed_redacted_output_and_unknown_backup(self):
        def fake_stat(path):
            if path == "/secret/caddy-config":
                raise PermissionError("Never print private Caddy file path")
            return object()

        report = build_report(self.metadata, self.sockets, 1, fake_stat)
        self.assertEqual(report["failure_category"], "address_conflict")
        self.assertEqual(report["container_state"], "exited")
        self.assertEqual(report["last_exit_code"], 255)
        self.assertEqual(report["required_mount_roots"]["data"], "present_unverified")
        self.assertEqual(report["required_mount_roots"]["config"], "inaccessible")
        self.assertEqual(report["web_listeners"], {"tcp_80": 1, "tcp_443": 1, "udp_443": 1})
        self.assertFalse(report["network"]["endpoint_currently_assigned"])
        self.assertEqual(report["network"]["network_endpoint_count"], 1)
        self.assertEqual(report["certificate_contents"], "not_inspected")
        self.assertEqual(report["backup_restoration"], "not_verified")
        self.assertEqual(report["recovery_readiness"], "not_established")
        self.assertEqual(report["automated_actions"], "none")
        data = json.dumps(report)
        for secret in [
            "private-ingress", "private-key-not-for-logs",
            "/secret/", "PORKBUN_API_KEY", "real-domain",
            "secret_value_must_never_escape",
        ]:
            self.assertNotIn(secret, data)

    def test_root_mount_not_found_and_unavailable_socket_output(self):
        values = {
            "/secret/caddy-certs": FileNotFoundError(),
            "/secret/caddy-config": PermissionError(),
            "/secret/real-domain-Caddyfile": OSError(),
        }

        def fake_stat(path):
            raise values[path]

        result = build_report(self.metadata, None, None, fake_stat)
        self.assertEqual(result["required_mount_roots"], {
            "caddyfile": "unknown",
            "data": "not_found",
            "config": "inaccessible",
        })
        self.assertIsNone(result["web_listeners"])
        self.assertIsNone(result["network"]["network_endpoint_count"])
        self.assertEqual(result["recovery_readiness"], "not_established")

    def test_safe_fixed_categories(self):
        self.assertEqual(category(""), "none")
        self.assertEqual(category("Port is already allocated"), "port_allocation")
        self.assertEqual(category("bad secret token xyz"), "other_redacted")
        self.assertNotIn("secret", category("bad secret token xyz"))

    def test_reject_ambiguous_ports_and_wrong_lines(self):
        rows = "\n".join([
            "tcp LISTEN 0 2048 [::]:80 [::]:*",
            "udp UNCONN 0 0 [::]:443 [::]:*",
            "tcp LISTEN 0 2048 0.0.0.0:18080 0.0.0.0:*",
            "udp UNCONN 0 0 0.0.0.0:4443 0.0.0.0:*",
            "garbage",
        ])
        self.assertEqual(listener_counts(rows), {
            "tcp_80": 1, "tcp_443": 0, "udp_443": 1,
        })

    def test_live_calls_only_read_only_apis(self):
        expected = [
            json.dumps(self.metadata),
            "1",
            self.sockets,
        ]
        with patch("legacy_recovery_audit.readonly", side_effect=expected) as reader:
            result = live_report("caddy")
        self.assertEqual(result["container_state"], "exited")
        commands = [a.args[0] for a in reader.call_args_list]
        self.assertEqual([c[:2] for c in commands], [
            ["docker", "inspect"],
            ["docker", "network"],
            ["ss", "-H"],
        ])
        for command in commands:
            self.assertNotIn("start", command)
            self.assertNotIn("restart", command)
            self.assertNotIn("rm", command)
            self.assertNotIn("exec", command)

    def test_dangerous_container_arguments_rejected(self):
        for name in ("-a", "--", "a/b", "a space", "", "a" * 90):
            with self.assertRaises(ValueError):
                live_report(name)

    def test_failed_inspection_is_sanitized_and_not_a_recovery_claim(self):
        with patch("legacy_recovery_audit.readonly", side_effect=RuntimeError("secret in an error")), \
             patch("sys.argv", ["legacy_recovery_audit.py", "caddy"]):
            out = io.StringIO()
            with redirect_stdout(out):
                code = main()
        self.assertEqual(code, 2)
        result = json.loads(out.getvalue())
        self.assertEqual(result["recovery_readiness"], "not_established")
        self.assertEqual(result["automated_actions"], "none")
        self.assertNotIn("secret", out.getvalue())


if __name__ == "__main__":
    unittest.main()

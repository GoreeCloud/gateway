"""Synthetic, no-network tests for a read-only AdGuard proxy bridge preflight."""
from __future__ import annotations

import json
import subprocess
import unittest
from types import SimpleNamespace

from adguard_bridge_socket_preflight import inspect, listener_observed, parse_unit


def output(text="", status=0, stderr=""):
    return SimpleNamespace(returncode=status, stdout=text, stderr=stderr)


GOOD_UNIT = (
    "LoadState=loaded\nActiveState=active\n"
    "SubState=listening\nResult=success\n"
)
FAILED_UNIT = (
    "Result=resources\nSubState=failed\n"
    "ActiveState=failed\nLoadState=loaded\n"
)
LISTENER = "tcp LISTEN 0 4096 172.19.0.1:3001 0.0.0.0:*\n"
UNRELATED = "tcp LISTEN 0 4096 172.19.0.1:13001 0.0.0.0:*\n"


class BridgePreflightTests(unittest.TestCase):
    def simulated(self, rows):
        calls = []

        def fake(args, **kwargs):
            calls.append(tuple(args))
            self.assertTrue(kwargs["capture_output"])
            self.assertTrue(kwargs["text"])
            self.assertLessEqual(kwargs["timeout"], 4)
            return rows.pop(0)

        report = inspect(fake)
        self.assertEqual(len(calls), 1 if len(rows) == 2 else 3)
        self.assertEqual(calls[0][0], "systemctl")
        return report

    def test_failed_systemd_socket_does_not_claim_proxy_ready(self):
        values = iter([output(FAILED_UNIT), output(UNRELATED), output(UNRELATED)])
        report = inspect(lambda *a, **kw: next(values))
        self.assertEqual(report["socket"]["failure_class"], "resources")
        self.assertEqual(report["socket"]["active_state"], "failed")
        self.assertEqual(report["listener_tcp_3001"], "not_observed")
        self.assertFalse(report["functional_proxy_verified"])
        self.assertFalse(report["cutover_authorized"])
        self.assertNotIn("172.19.0.1", json.dumps(report))

    def test_observed_port_and_active_unit_still_not_functional_proof(self):
        values = iter([output(GOOD_UNIT), output(LISTENER), output(LISTENER)])
        report = inspect(lambda *args, **kwargs: next(values))
        self.assertEqual(report["listener_tcp_3001"], "observed")
        self.assertEqual(report["socket"]["sub_state"], "listening")
        self.assertFalse(report["functional_proxy_verified"])

    def test_other_port_cannot_satisfy_bridge(self):
        self.assertFalse(listener_observed(UNRELATED))
        self.assertTrue(listener_observed(LISTENER))

    def test_unknown_or_changed_socket_data_fails_closed(self):
        for inputs in (
            [output(FAILED_UNIT, status=1)],
            [output(FAILED_UNIT), output(LISTENER), output(UNRELATED)],
            [output(FAILED_UNIT), output(UNRELATED, stderr="private-address")],
            [output(FAILED_UNIT), output("unexpected-private-secret")],
        ):
            with self.subTest(count=len(inputs)):
                values = iter(inputs)
                report = inspect(lambda *args, **kwargs: next(values))
                self.assertEqual(report["socket"], "unknown")
                self.assertEqual(report["listener_tcp_3001"], "unknown")
                self.assertNotIn("private", json.dumps(report))

    def test_unit_field_duplicates_and_missing_keys_rejected(self):
        for raw in (GOOD_UNIT + "Result=success\n",
                    "ActiveState=failed\n",
                    GOOD_UNIT + "PrivateKey=secret\n",
                    GOOD_UNIT.replace("Result=success\n", "")):
            with self.subTest(raw_len=len(raw)):
                with self.assertRaises(ValueError):
                    parse_unit(raw)


if __name__ == "__main__":
    unittest.main()

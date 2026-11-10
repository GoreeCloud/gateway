"""Synthetic Docker metadata tests. Never inspect or change a real container."""
from __future__ import annotations

import os
from pathlib import Path
import subprocess
import tempfile
import unittest

SCRIPT = Path(__file__).with_name("inspect_legacy_ingress.sh")
FAKE_DOCKER = """#!/bin/sh
if [ "$1" != "inspect" ] || [ "$2" != "--type" ] || [ "$3" != "container" ]; then
  exit 3
fi
if [ "$#" -eq 4 ]; then
  exit 0
fi
if [ "$4" != "--format" ]; then
  exit 3
fi
case "$5" in
  '{{.State.Status}}') printf '%s\n' "$MOCK_STATUS" ;;
  '{{.State.ExitCode}}') printf '0\n' ;;
  '{{.RestartCount}}') printf '%s\n' "$MOCK_RESTARTS" ;;
  '{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}')
    printf '%s\n' "$MOCK_HEALTH" ;;
  '{{.HostConfig.RestartPolicy.Name}}') printf 'unless-stopped\n' ;;
  '{{.State.Error}}') printf '%s\n' "$MOCK_ERROR" ;;
  '{{len .Mounts}}') printf '3\n' ;;
  '{{len .HostConfig.PortBindings}}') printf '2\n' ;;
  '{{.HostConfig.NetworkMode}}') printf 'bridge\n' ;;
  *) exit 3 ;;
esac
"""
FAKE_SS = """#!/bin/sh
printf 'tcp LISTEN 0 4096 127.0.0.1:80 0.0.0.0:*\n'
"""


class LegacyIngressHealthInspectionTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.dir = Path(self.temp.name)
        for name, contents in (("docker", FAKE_DOCKER), ("ss", FAKE_SS)):
            file = self.dir / name
            file.write_text(contents, encoding="utf-8")
            file.chmod(0o700)

    def tearDown(self):
        self.temp.cleanup()

    def inspect(self, status="running", health="none", restarts="0", error="", name="caddy"):
        env = os.environ.copy()
        env.update({
            "PATH": str(self.dir) + os.pathsep + env.get("PATH", ""),
            "MOCK_STATUS": status,
            "MOCK_HEALTH": health,
            "MOCK_RESTARTS": restarts,
            "MOCK_ERROR": error,
        })
        return subprocess.run(
            ["bash", str(SCRIPT), name], env=env,
            capture_output=True, text=True, timeout=5, check=False,
        )

    def test_running_without_healthcheck_is_not_claimed_healthy(self):
        result = self.inspect()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("container_status=running\n", result.stdout)
        self.assertIn("docker_health_state=none\n", result.stdout)
        self.assertIn("restart_count=0\n", result.stdout)
        self.assertIn("recovery_approval=required\n", result.stdout)

    def test_docker_health_unhealthy_or_starting_is_not_ready(self):
        for state in ("unhealthy", "starting", "invalid"):
            with self.subTest(state=state):
                result = self.inspect(health=state)
                self.assertEqual(result.returncode, 1, result.stderr)
                category = state if state != "invalid" else "unknown"
                self.assertIn(f"docker_health_state={category}\n", result.stdout)

    def test_stopped_container_is_not_ready_even_when_health_says_healthy(self):
        result = self.inspect(status="exited", health="healthy", restarts="4")
        self.assertEqual(result.returncode, 1)
        self.assertIn("restart_count=4\n", result.stdout)

    def test_out_of_range_restart_count_is_redacted(self):
        result = self.inspect(restarts="999999999999999", error="secret-owner-specific")
        self.assertEqual(result.returncode, 0)
        self.assertIn("restart_count=unknown\n", result.stdout)
        self.assertIn("error_category=other\n", result.stdout)
        self.assertNotIn("secret-owner-specific", result.stdout)

    def test_invalid_container_name_is_rejected_without_inspection(self):
        result = self.inspect(name="../unsafe")
        self.assertEqual(result.returncode, 2)
        self.assertIn("invalid_container_name", result.stderr)
        self.assertNotIn("container_status=", result.stdout)


if __name__ == "__main__":
    unittest.main()

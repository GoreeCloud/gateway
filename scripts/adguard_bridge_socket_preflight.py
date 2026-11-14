#!/usr/bin/env python3
"""Redacted, read-only AdGuard-to-Caddy socket diagnostic.

Never starts/restarts services, queries DNS, reads unit files or logs, or
prints network endpoints. Socket presence is NOT route or TLS readiness.
"""
from __future__ import annotations

import json
import re
import subprocess

UNIT = "adguard-caddy-proxy.socket"
FIELDS = ("LoadState", "ActiveState", "SubState", "Result")
PORT = 3001
PORT_RE = re.compile(r":([0-9]{1,5})\\Z")


def parse_unit(text: str) -> dict[str, str]:
    if not isinstance(text, str) or len(text) > 8192:
        raise ValueError("invalid systemd response")
    values: dict[str, str] = {}
    for line in text.splitlines():
        if not line:
            continue
        key, sep, value = line.partition("=")
        if not sep or key not in FIELDS or key in values or not value or len(value) > 64:
            raise ValueError("unexpected unit field")
        values[key] = value
    if set(values) != set(FIELDS):
        raise ValueError("incomplete unit fields")
    return {
        "load_state": "loaded" if values["LoadState"] == "loaded" else "other",
        "active_state": values["ActiveState"]
        if values["ActiveState"] in ("active", "inactive", "failed", "activating")
        else "other",
        "sub_state": values["SubState"]
        if values["SubState"] in ("listening", "dead", "failed", "start", "stop")
        else "other",
        "failure_class": "resources" if values["Result"] == "resources"
        else "success" if values["Result"] == "success" else "other",
    }


def listener_observed(text: str) -> bool:
    if not isinstance(text, str) or len(text) > 524288:
        raise ValueError("invalid socket listing")
    present = False
    for line in text.splitlines():
        fields = line.split()
        if len(fields) < 5 or fields[0].lower() != "tcp" or fields[1] != "LISTEN":
            raise ValueError("unexpected socket row")
        match = PORT_RE.search(fields[4])
        if not match:
            raise ValueError("unrecognized socket endpoint")
        if int(match.group(1)) == PORT:
            present = True
    return present


def inspect(run=subprocess.run) -> dict:
    report = {
        "schema": "goreecloud.gateway.adguard-bridge-preflight.v1",
        "inspection": "read_only",
        "unit": "adguard-caddy-proxy.socket",
        "socket": "unknown",
        "listener_tcp_3001": "unknown",
        "cutover_authorized": False,
        "functional_proxy_verified": False,
    }
    try:
        unit = run(
            ["systemctl", "show", UNIT,
             "--property=" + ",".join(FIELDS), "--no-pager"],
            capture_output=True, text=True, timeout=4, check=False,
        )
        if unit.returncode or unit.stderr:
            raise ValueError("unit inspection unavailable")
        report["socket"] = parse_unit(unit.stdout)
        observations = []
        for _ in range(2):
            sample = run(
                ["ss", "-H", "-lnt"], capture_output=True, text=True,
                timeout=4, check=False,
            )
            if sample.returncode or sample.stderr:
                raise ValueError("socket inspection unavailable")
            observations.append(listener_observed(sample.stdout))
        if observations[0] != observations[1]:
            raise ValueError("socket state changed during inspection")
        report["listener_tcp_3001"] = (
            "observed" if observations[0] else "not_observed"
        )
    except (OSError, subprocess.TimeoutExpired, ValueError, UnicodeError):
        # No raw exceptions, systemd diagnostics, paths, IPs or log lines.
        report["socket"] = "unknown"
        report["listener_tcp_3001"] = "unknown"
    return report


def main() -> int:
    result = inspect()
    print(json.dumps(result, sort_keys=True, separators=(",", ":")))
    if result["socket"] == "unknown":
        return 2
    unit = result["socket"]
    return 0 if (
        unit["load_state"] == "loaded"
        and unit["active_state"] == "active"
        and unit["sub_state"] == "listening"
        and result["listener_tcp_3001"] == "observed"
    ) else 1


if __name__ == "__main__":
    raise SystemExit(main())

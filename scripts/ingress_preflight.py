#!/usr/bin/env python3
"""Non-mutating, credential-redacted Docker/Caddy readiness inventory.

Without --live, this tool displays help and never contacts Docker.
With --live, only fixed bounded read commands execute. No Docker stderr,
hostnames, container environments, secret values, certificate contents,
mount sources, or host IP addresses are emitted.
"""
from __future__ import annotations

import argparse
from datetime import datetime, timezone
import json
import re
import subprocess
import sys

_VERSION_RE = re.compile(r"^v(\d+)\.(\d+)\.(\d+)(?:[-+][A-Za-z0-9._-]+)?$")
_CONTAINER_RE = re.compile(r"^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$")


class InspectionError(Exception):
    """Only safe reason codes, never Docker stderr."""


def docker_read(args: list[str]) -> str:
    try:
        proc = subprocess.run(
            ["docker", *args], capture_output=True, text=True,
            check=False, timeout=8, encoding="utf-8", errors="replace",
        )
    except (FileNotFoundError, subprocess.TimeoutExpired, OSError):
        raise InspectionError("docker_read_unavailable") from None
    if proc.returncode != 0:
        raise InspectionError("docker_read_failed")
    if len(proc.stdout) > (512 << 10):
        raise InspectionError("unexpectedly_large_docker_response")
    return proc.stdout


def parse_version(raw: str) -> tuple[str | None, bool | None]:
    first = raw.strip().split(" ", 1)[0]
    match = _VERSION_RE.fullmatch(first)
    if not match:
        return None, None
    major, minor, patch = (int(n) for n in match.groups())
    return first, (major, minor, patch) < (2, 11, 5)


def assess(running: bool | None, healthcheck: bool | None,
           port_lines: str, mount_lines: str, version_line: str,
           module_lines: str) -> dict[str, object]:
    """Return a non-secret summary; do not claim backup or TLS health."""
    version, known_vulnerable = parse_version(version_line)
    ports = {"http_tcp_80": False, "https_tcp_443": False, "http3_udp_443": False}
    for line in port_lines.splitlines():
        mapping = line.split(" -> ", 1)[0].strip()
        if mapping == "80/tcp":
            ports["http_tcp_80"] = True
        elif mapping == "443/tcp":
            ports["https_tcp_443"] = True
        elif mapping == "443/udp":
            ports["http3_udp_443"] = True

    mounts = {"caddy_config": False, "caddy_data": False, "caddy_runtime_config": False}
    for line in mount_lines.splitlines():
        destination = line.split("|", 1)[0].strip()
        if destination in ("/etc/caddy", "/etc/caddy/Caddyfile"):
            mounts["caddy_config"] = True
        elif destination == "/data":
            mounts["caddy_data"] = True
        elif destination == "/config":
            mounts["caddy_runtime_config"] = True

    blockers = [
        "actual_route_and_auth_parity_unverified",
        "live_certificate_and_renewal_health_unverified",
        "independent_backup_restore_unverified",
        "tested_rollback_unverified",
        "change_window_and_authorization_unverified",
    ]
    if known_vulnerable is True:
        blockers.insert(0, "verified_legacy_caddy_requires_security_upgrade")
    elif known_vulnerable is None:
        blockers.insert(0, "caddy_version_unverified")
    if running is not True:
        blockers.insert(0, "container_running_state_unverified_or_false")
    return {
        "schema_version": 1,
        "observed_at_utc": datetime.now(timezone.utc).isoformat(timespec="seconds"),
        "scope": "read_only_docker_metadata_only",
        "container_running": running,
        "docker_healthcheck_configured": healthcheck,
        "published_port_presence": ports,
        "expected_mount_destination_presence": mounts,
        "caddy_binary_version": version,
        "caddy_pre_2_11_5": known_vulnerable,
        "porkbun_module_registered": "dns.providers.porkbun" in module_lines.splitlines(),
        "route_health": "not_verified",
        "certificate_health": "not_verified",
        "backup_restore": "not_verified",
        "upgrade_rollback": "not_verified",
        "ready_for_production_cutover": False,
        "open_gates": blockers,
    }


def inspect(container: str) -> dict[str, object]:
    if not _CONTAINER_RE.fullmatch(container):
        raise InspectionError("invalid_container_name")
    # Fixed templates disclose only selected non-secret fields.
    # Do not query Env, Labels, host Mount Source, or arbitrary config.
    state = docker_read(["inspect", "--format",
        "{{.State.Running}}|{{if .State.Health}}true{{else}}false{{end}}", container]).strip()
    if state not in ("true|true", "true|false", "false|true", "false|false"):
        raise InspectionError("unexpected_docker_state_format")
    running, health = (x == "true" for x in state.split("|"))
    ports = docker_read(["port", container])
    mounts = docker_read(["inspect", "--format",
        "{{range .Mounts}}{{.Destination}}|{{.Type}}{{println}}{{end}}", container])
    version = docker_read(["exec", container, "caddy", "version"])
    modules = docker_read(["exec", container, "caddy", "list-modules"])
    return assess(running, health, ports, mounts, version, modules)


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--live", action="store_true", help="run fixed read-only Docker commands")
    parser.add_argument("--container", default="caddy", help="container name (default: caddy)")
    args = parser.parse_args(argv)
    if not args.live:
        parser.print_help()
        return 0
    try:
        data = inspect(args.container)
    except InspectionError as exc:
        print(json.dumps({"schema_version": 1, "inspection_error_code": str(exc),
                          "ready_for_production_cutover": False}, sort_keys=True))
        return 2
    print(json.dumps(data, sort_keys=True, indent=2))
    return 0


if __name__ == "__main__":
    sys.exit(main())

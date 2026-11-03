#!/usr/bin/env python3
"""Read-only and redacted Docker ingress evidence, never a recovery decision.

Only consumes inspect metadata and socket state. Does not read certificate files,
environment values, hostnames, proxy config or Docker logs. Does not mutate Docker.
"""

import argparse
import json
import os
import re
import subprocess

SCHEMA_VERSION = 1
NAME = re.compile(r"^[A-Za-z0-9][A-Za-z0-9_.-]{0,62}$")
CONTAINER_TEMPLATE = (
    '{"status":{{json .State.Status}},'
    '"exit_code":{{.State.ExitCode}},'
    '"error":{{json .State.Error}},'
    '"restart_policy":{{json .HostConfig.RestartPolicy.Name}},'
    '"network_mode":{{json .HostConfig.NetworkMode}},'
    '"mounts":{{json .Mounts}},'
    '"networks":{{json .NetworkSettings.Networks}}}'
)
DESTINATIONS = {
    "caddyfile": "/etc/caddy/Caddyfile",
    "data": "/data",
    "config": "/config",
}
KNOWN_STATUSES = {"created", "running", "paused", "restarting", "removing", "exited", "dead"}
KNOWN_RESTART = {"no", "always", "unless-stopped", "on-failure", ""}


def category(message):
    if not message:
        return "none"
    if "address already in use" in message.lower():
        return "address_conflict"
    if "port is already allocated" in message.lower():
        return "port_allocation"
    return "other_redacted"


def mount_state(source, stat_fn=os.stat):
    if not source or not isinstance(source, str):
        return "not_mounted"
    try:
        stat_fn(source)
    except PermissionError:
        return "inaccessible"
    except FileNotFoundError:
        return "not_found"
    except OSError:
        return "unknown"
    return "present_unverified"


def listener_counts(stdout):
    values = {"tcp_80": 0, "tcp_443": 0, "udp_443": 0}
    for line in stdout.splitlines():
        parts = line.split()
        if len(parts) < 5:
            continue
        protocol = parts[0].lower()
        if protocol not in ("tcp", "udp"):
            continue
        match = re.search(r":(80|443)$", parts[4])
        if match:
            key = f"{protocol}_{match.group(1)}"
            if key in values:
                values[key] += 1
    return values


def build_report(metadata, sockets=None, peer_count=None, stat_fn=os.stat):
    """Return only fixed-field, non-secret statuses; no raw Docker attributes."""
    state = metadata.get("status")
    mounts = {
        x.get("Destination"): x.get("Source")
        for x in (metadata.get("mounts") or [])
        if isinstance(x, dict) and isinstance(x.get("Destination"), str)
    }
    networks = metadata.get("networks") or {}
    mode = metadata.get("network_mode")
    network_record = networks.get(mode) if isinstance(networks, dict) else None
    endpoint = network_record.get("EndpointID") if isinstance(network_record, dict) else None
    image_exit = metadata.get("exit_code")
    try:
        image_exit = int(image_exit)
    except (TypeError, ValueError, OverflowError):
        image_exit = None
    return {
        "schema_version": SCHEMA_VERSION,
        "inspection": "read_only",
        "container_state": state if state in KNOWN_STATUSES else "unknown",
        "last_exit_code": image_exit if image_exit is not None and 0 <= image_exit <= 255 else None,
        "failure_category": category(str(metadata.get("error") or "")),
        "restart_policy": (
            metadata.get("restart_policy")
            if metadata.get("restart_policy") in KNOWN_RESTART else "unknown"
        ),
        "required_mount_roots": {
            key: mount_state(mounts.get(destination), stat_fn)
            for key, destination in DESTINATIONS.items()
        },
        "network": {
            "configured": isinstance(mode, str) and bool(mode),
            "endpoint_currently_assigned": bool(endpoint),
            "other_endpoint_count": peer_count if isinstance(peer_count, int) and peer_count >= 0 else None,
        },
        "web_listeners": listener_counts(sockets) if isinstance(sockets, str) else None,
        "certificate_contents": "not_inspected",
        "backup_restoration": "not_verified",
        "route_parity": "not_verified",
        "rollback": "not_verified",
        "recovery_readiness": "not_established",
        "automated_actions": "none",
    }


def readonly(args):
    result = subprocess.run(
        args, capture_output=True, text=True, check=False, timeout=8
    )
    if result.returncode != 0:
        raise RuntimeError("read_only_inspection_unavailable")
    return result.stdout


def live_report(name):
    if not NAME.fullmatch(name):
        raise ValueError("invalid_container_name")
    metadata = json.loads(readonly([
        "docker", "inspect", "--type", "container",
        "--format", CONTAINER_TEMPLATE, name,
    ]))
    mode = metadata.get("network_mode")
    peer_count = None
    if isinstance(mode, str) and NAME.fullmatch(mode) and mode not in ("host", "none", "default"):
        try:
            peer_count = int(readonly([
                "docker", "network", "inspect", "--format",
                "{{len .Containers}}", mode,
            ]).strip())
        except (RuntimeError, ValueError, subprocess.TimeoutExpired):
            pass
    try:
        sockets = readonly(["ss", "-H", "-lnut"])
    except (RuntimeError, FileNotFoundError, subprocess.TimeoutExpired):
        sockets = None
    return build_report(metadata, sockets, peer_count)


def main():
    parser = argparse.ArgumentParser(
        description="Read-only Caddy Docker recovery facts (never prints secrets or host paths)"
    )
    parser.add_argument("container", nargs="?", default="caddy")
    arguments = parser.parse_args()
    try:
        report = live_report(arguments.container)
    except (RuntimeError, ValueError, OSError, subprocess.TimeoutExpired, json.JSONDecodeError):
        print(json.dumps({
            "schema_version": SCHEMA_VERSION,
            "inspection": "unavailable",
            "recovery_readiness": "not_established",
            "automated_actions": "none",
        }, sort_keys=True))
        return 2
    print(json.dumps(report, sort_keys=True))
    # Report is useful even for stopped containers; the exit code indicates state
    # only, never permission to restart or an accepted recovery plan.
    return 0 if report["container_state"] == "running" else 1


if __name__ == "__main__":
    raise SystemExit(main())

#!/usr/bin/env python3
"""Run steady loopback-only GoreeCloud Gateway load evidence collection."""

from __future__ import annotations

import argparse
import concurrent.futures
import hashlib
import http.client
import http.server
import json
import os
from pathlib import Path
import re
import socket
import subprocess
import tempfile
import threading
import time

SCHEMA = "goreecloud-gateway-isolated-sustained-load-evidence/v1"
HEX40 = re.compile(r"^[0-9a-f]{40}$")
WORKERS = 16
DURATION_SECONDS = 8.0
BACKEND_HOLD_SECONDS = 0.01
MINIMUM_REQUESTS = 256
REQUEST_TIMEOUT_SECONDS = 3.0
HARNESS_P95_LIMIT_MS = 2000.0


class AcceptanceHTTPServer(http.server.ThreadingHTTPServer):
    request_queue_size = 256


class Backend(http.server.BaseHTTPRequestHandler):
    def do_GET(self) -> None:  # noqa: N802 - stdlib handler contract
        if self.path == "/health":
            body = b"healthy\n"
        else:
            time.sleep(BACKEND_HOLD_SECONDS)
            body = b"goreecloud-gateway-sustained-load\n"
        self.send_response(200)
        self.send_header("Content-Type", "text/plain")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_HEAD(self) -> None:  # noqa: N802 - stdlib handler contract
        self.send_response(200)
        self.end_headers()

    def log_message(self, _format: str, *_args: object) -> None:
        return


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def reserve_loopback_port() -> int:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as sock:
        sock.bind(("127.0.0.1", 0))
        return int(sock.getsockname()[1])


def request(port: int, path: str = "/load", timeout: float = REQUEST_TIMEOUT_SECONDS) -> tuple[int, bytes]:
    connection = http.client.HTTPConnection("127.0.0.1", port, timeout=timeout)
    try:
        connection.request("GET", path, headers={"Host": "gateway.acceptance.test"})
        response = connection.getresponse()
        return response.status, response.read()
    finally:
        connection.close()


def wait_for_gateway(port: int, process: subprocess.Popen[bytes]) -> None:
    deadline = time.monotonic() + 10
    while time.monotonic() < deadline:
        if process.poll() is not None:
            raise RuntimeError(f"Gateway exited before load acceptance with code {process.returncode}")
        try:
            status, body = request(port, "/ready")
            if status == 200 and body == b"goreecloud-gateway-sustained-load\n":
                return
        except (ConnectionError, OSError):
            pass
        time.sleep(0.1)
    raise RuntimeError("Gateway loopback listener did not become ready")


def percentile(values: list[float], percent: float) -> float:
    if not values:
        return 0.0
    ordered = sorted(values)
    rank = int(round((len(ordered) - 1) * percent))
    return ordered[max(0, min(rank, len(ordered) - 1))]


def worker(port: int, deadline: float) -> tuple[list[float], int]:
    latencies: list[float] = []
    failures = 0
    while time.monotonic() < deadline:
        started = time.perf_counter()
        try:
            status, body = request(port)
            elapsed_ms = (time.perf_counter() - started) * 1000.0
            latencies.append(elapsed_ms)
            if status != 200 or body != b"goreecloud-gateway-sustained-load\n":
                failures += 1
        except (ConnectionError, OSError, TimeoutError):
            latencies.append((time.perf_counter() - started) * 1000.0)
            failures += 1
    return latencies, failures


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--binary", required=True, type=Path)
    parser.add_argument("--evidence", required=True, type=Path)
    args = parser.parse_args()

    source_revision = os.environ.get("GOREECLOUD_GATEWAY_SOURCE_REVISION", "").strip().lower()
    if not HEX40.fullmatch(source_revision):
        raise SystemExit("GOREECLOUD_GATEWAY_SOURCE_REVISION must be an exact 40-character commit SHA")
    binary = args.binary.resolve()
    if not binary.is_file():
        raise SystemExit(f"Gateway binary not found: {binary}")

    backend = AcceptanceHTTPServer(("127.0.0.1", 0), Backend)
    backend_port = int(backend.server_address[1])
    backend_thread = threading.Thread(target=backend.serve_forever, daemon=True)
    backend_thread.start()

    gateway_port = reserve_loopback_port()
    process: subprocess.Popen[bytes] | None = None
    started_at = time.monotonic()
    try:
        with tempfile.TemporaryDirectory(prefix="goreecloud-gateway-sustained-") as temp:
            config_path = Path(temp) / "gateway.json"
            config = {
                "schema": "goreecloud-gateway-config/v1",
                "services": [
                    {
                        "id": "isolated-load-service",
                        "name": "Isolated sustained-load service",
                        "backend_ids": ["isolated-load-backend"],
                    }
                ],
                "routes": [
                    {
                        "id": "isolated-load-route",
                        "service_id": "isolated-load-service",
                        "hostname": "gateway.acceptance.test",
                        "path_prefix": "/",
                        "methods": ["GET", "HEAD"],
                        "exposure": "private",
                        "allowed_client_cidrs": ["127.0.0.1/32"],
                        "enabled": True,
                        "tls": {"mode": "disabled"},
                    }
                ],
                "backends": [
                    {
                        "id": "isolated-load-backend",
                        "url": f"http://127.0.0.1:{backend_port}",
                        "enabled": True,
                        "health_path": "/health",
                    }
                ],
                "certificate_profiles": [],
            }
            config_path.write_text(json.dumps(config), encoding="utf-8")
            process = subprocess.Popen(
                [str(binary), "-config", str(config_path), "-http", f"127.0.0.1:{gateway_port}"],
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
            )
            wait_for_gateway(gateway_port, process)

            load_started = time.monotonic()
            deadline = load_started + DURATION_SECONDS
            with concurrent.futures.ThreadPoolExecutor(max_workers=WORKERS) as executor:
                results = [executor.submit(worker, gateway_port, deadline) for _ in range(WORKERS)]
                completed = [future.result(timeout=DURATION_SECONDS + 10) for future in results]
            load_duration = time.monotonic() - load_started

            latencies = [latency for worker_latencies, _ in completed for latency in worker_latencies]
            failures = sum(worker_failures for _, worker_failures in completed)
            requests = len(latencies)
            if requests < MINIMUM_REQUESTS:
                raise RuntimeError(f"isolated sustained load produced only {requests} requests")
            if failures != 0:
                raise RuntimeError(f"isolated sustained load produced {failures} failed requests")

            p50 = percentile(latencies, 0.50)
            p95 = percentile(latencies, 0.95)
            p99 = percentile(latencies, 0.99)
            if p95 > HARNESS_P95_LIMIT_MS:
                raise RuntimeError(
                    f"isolated harness p95 exceeded {HARNESS_P95_LIMIT_MS:.0f} ms: observed={p95:.2f} ms"
                )

            evidence = {
                "schema": SCHEMA,
                "source_revision": source_revision,
                "runtime_artifact_sha256": sha256(binary),
                "listener_scope": "127.0.0.1",
                "worker_count": WORKERS,
                "target_duration_seconds": DURATION_SECONDS,
                "observed_duration_seconds": round(load_duration, 3),
                "request_count": requests,
                "failure_count": failures,
                "error_rate": 0.0,
                "latency_ms": {
                    "p50": round(p50, 3),
                    "p95": round(p95, 3),
                    "p99": round(p99, 3),
                },
                "harness_p95_limit_ms": HARNESS_P95_LIMIT_MS,
                "credentials_included": False,
                "production_capacity_claimed": False,
                "production_cutover_authorized": False,
            }
            args.evidence.parent.mkdir(parents=True, exist_ok=True)
            args.evidence.write_text(json.dumps(evidence, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    finally:
        if process is not None and process.poll() is None:
            process.terminate()
            try:
                process.wait(timeout=10)
            except subprocess.TimeoutExpired:
                process.kill()
                process.wait(timeout=5)
        backend.shutdown()
        backend.server_close()
        backend_thread.join(timeout=5)

    elapsed = time.monotonic() - started_at
    print(f"GoreeCloud Gateway isolated sustained-load acceptance: PASS ({elapsed:.2f}s)")


if __name__ == "__main__":
    main()

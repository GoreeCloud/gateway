#!/usr/bin/env python3
"""Run a bounded loopback-only GoreeCloud Gateway runtime acceptance exercise."""

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
import signal
import socket
import subprocess
import tempfile
import threading
import time

SCHEMA = "goreecloud-gateway-isolated-runtime-evidence/v1"
HEX40 = re.compile(r"^[0-9a-f]{40}$")
LOAD_REQUESTS = 144
MAX_BACKEND_CONCURRENCY = 128
LOAD_HOLD_SECONDS = 0.15


class AcceptanceHTTPServer(http.server.ThreadingHTTPServer):
    """Synthetic backend sized so its accept queue does not become the load-test bottleneck."""

    request_queue_size = LOAD_REQUESTS


class Backend(http.server.BaseHTTPRequestHandler):
    concurrency_lock = threading.Lock()
    active_requests = 0
    max_active_requests = 0

    @classmethod
    def reset_concurrency(cls) -> None:
        with cls.concurrency_lock:
            cls.active_requests = 0
            cls.max_active_requests = 0

    @classmethod
    def begin_request(cls) -> None:
        with cls.concurrency_lock:
            cls.active_requests += 1
            cls.max_active_requests = max(cls.max_active_requests, cls.active_requests)

    @classmethod
    def end_request(cls) -> None:
        with cls.concurrency_lock:
            cls.active_requests -= 1

    @classmethod
    def observed_concurrency(cls) -> int:
        with cls.concurrency_lock:
            return cls.max_active_requests

    def do_GET(self) -> None:  # noqa: N802 - stdlib handler contract
        if self.path == "/health":
            body = b"healthy\n"
            self.send_response(200)
            self.send_header("Content-Type", "text/plain")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)
            return

        # Measure the synthetic backend work interval, not Python handler
        # teardown after the response has already been emitted. The Gateway
        # transport owns its concurrency slot through response Body.Close;
        # server-handler teardown is a later, different boundary and can
        # otherwise transiently over-count completed work by one request.
        if self.path == "/load":
            Backend.begin_request()
            try:
                time.sleep(LOAD_HOLD_SECONDS)
            finally:
                Backend.end_request()

        body = b"goreecloud-gateway-isolated-runtime\n"
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


def request(port: int, host: str, path: str = "/", timeout: float = 2) -> tuple[int, bytes]:
    connection = http.client.HTTPConnection("127.0.0.1", port, timeout=timeout)
    try:
        connection.request("GET", path, headers={"Host": host})
        response = connection.getresponse()
        return response.status, response.read()
    finally:
        connection.close()


def wait_for_gateway(port: int, process: subprocess.Popen[bytes]) -> None:
    deadline = time.monotonic() + 10
    while time.monotonic() < deadline:
        if process.poll() is not None:
            raise RuntimeError(f"Gateway exited before acceptance request with code {process.returncode}")
        try:
            status, _ = request(port, "unmatched.acceptance.test")
            if status == 404:
                return
        except (ConnectionError, OSError):
            pass
        time.sleep(0.1)
    raise RuntimeError("Gateway loopback listener did not become ready")


def wait_for_route(port: int, process: subprocess.Popen[bytes], active_host: str, inactive_host: str) -> None:
    deadline = time.monotonic() + 10
    while time.monotonic() < deadline:
        if process.poll() is not None:
            raise RuntimeError(f"Gateway exited during configuration recovery with code {process.returncode}")
        try:
            active_status, active_body = request(port, active_host, "/recovery-probe")
            inactive_status, _ = request(port, inactive_host, "/recovery-probe")
            if (
                active_status == 200
                and active_body == b"goreecloud-gateway-isolated-runtime\n"
                and inactive_status == 404
            ):
                return
        except (ConnectionError, OSError):
            pass
        time.sleep(0.1)
    raise RuntimeError(
        f"Gateway configuration transition did not converge: active={active_host} inactive={inactive_host}"
    )


def exercise_bounded_load(port: int) -> tuple[int, int]:
    Backend.reset_concurrency()
    with concurrent.futures.ThreadPoolExecutor(max_workers=LOAD_REQUESTS) as executor:
        futures = [
            executor.submit(request, port, "gateway.acceptance.test", "/load", 10)
            for _ in range(LOAD_REQUESTS)
        ]
        responses = [future.result(timeout=15) for future in futures]

    failed = [status for status, body in responses if status != 200 or body != b"goreecloud-gateway-isolated-runtime\n"]
    if failed:
        raise RuntimeError(f"bounded load produced {len(failed)} failed responses")

    observed = Backend.observed_concurrency()
    if observed <= 0:
        raise RuntimeError("bounded load did not reach the isolated backend")
    if observed > MAX_BACKEND_CONCURRENCY:
        raise RuntimeError(
            f"backend concurrency exceeded configured transport limit: observed={observed} limit={MAX_BACKEND_CONCURRENCY}"
        )
    if observed >= LOAD_REQUESTS:
        raise RuntimeError("bounded load did not demonstrate upstream backpressure")
    return len(responses), observed


def run_recovery(recovery_binary: Path, *arguments: str) -> dict[str, object]:
    completed = subprocess.run(
        [str(recovery_binary), *arguments],
        check=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        timeout=10,
    )
    try:
        result = json.loads(completed.stdout)
    except json.JSONDecodeError as exc:
        raise RuntimeError(f"Gateway recovery command returned invalid JSON: {completed.stdout!r}") from exc
    if not isinstance(result, dict):
        raise RuntimeError("Gateway recovery command did not return a JSON object")
    return result


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--binary", required=True, type=Path)
    parser.add_argument("--recovery-binary", required=True, type=Path)
    parser.add_argument("--evidence", required=True, type=Path)
    args = parser.parse_args()

    source_revision = os.environ.get("GOREECLOUD_GATEWAY_SOURCE_REVISION", "").strip().lower()
    if not HEX40.fullmatch(source_revision):
        raise SystemExit("GOREECLOUD_GATEWAY_SOURCE_REVISION must be an exact 40-character commit SHA")
    binary = args.binary.resolve()
    recovery_binary = args.recovery_binary.resolve()
    if not binary.is_file():
        raise SystemExit(f"Gateway binary not found: {binary}")
    if not recovery_binary.is_file():
        raise SystemExit(f"Gateway recovery binary not found: {recovery_binary}")

    backend = AcceptanceHTTPServer(("127.0.0.1", 0), Backend)
    backend_port = int(backend.server_address[1])
    backend_thread = threading.Thread(target=backend.serve_forever, daemon=True)
    backend_thread.start()

    gateway_port = reserve_loopback_port()
    process: subprocess.Popen[bytes] | None = None
    graceful_shutdown = False
    route_proxy_validated = False
    unmatched_route_validated = False
    bounded_load_validated = False
    backpressure_validated = False
    backup_restore_validated = False
    rollback_rehearsed = False
    recovery_compare_and_swap_validated = False
    load_request_count = 0
    max_observed_backend_concurrency = 0
    snapshot_config_sha256 = ""
    try:
        with tempfile.TemporaryDirectory(prefix="goreecloud-gateway-acceptance-") as temp:
            recovery_root = Path(temp)
            config_path = recovery_root / "active" / "gateway.json"
            config_path.parent.mkdir(mode=0o700)
            config = {
                "schema": "goreecloud-gateway-config/v1",
                "services": [
                    {
                        "id": "isolated-service",
                        "name": "Isolated acceptance service",
                        "backend_ids": ["isolated-backend"],
                    }
                ],
                "routes": [
                    {
                        "id": "isolated-route",
                        "service_id": "isolated-service",
                        "hostname": "gateway.acceptance.test",
                        "path_prefix": "/",
                        "methods": ["GET", "HEAD"],
                        "exposure": "private",
                        "enabled": True,
                        "tls": {"mode": "disabled"},
                    }
                ],
                "backends": [
                    {
                        "id": "isolated-backend",
                        "url": f"http://127.0.0.1:{backend_port}",
                        "enabled": True,
                        "health_path": "/health",
                    }
                ],
                "certificate_profiles": [],
            }
            config_path.write_text(json.dumps(config), encoding="utf-8")
            process = subprocess.Popen(
                [
                    str(binary),
                    "-config",
                    str(config_path),
                    "-http",
                    f"127.0.0.1:{gateway_port}",
                ],
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
            )
            wait_for_gateway(gateway_port, process)

            status, body = request(gateway_port, "gateway.acceptance.test", "/probe")
            if status != 200 or body != b"goreecloud-gateway-isolated-runtime\n":
                raise RuntimeError(f"Gateway route probe failed: status={status}, body={body!r}")
            route_proxy_validated = True

            status, _ = request(gateway_port, "unmatched.acceptance.test")
            if status != 404:
                raise RuntimeError(f"unmatched host returned {status}, expected 404")
            unmatched_route_validated = True

            # A malformed reload must preserve the last known-good route and listener.
            original_config = config_path.read_text(encoding="utf-8")
            config_path.write_text('{"schema":"goreecloud-gateway-config/v1","routes":', encoding="utf-8")
            process.send_signal(signal.SIGHUP)
            time.sleep(0.3)
            if process.poll() is not None:
                raise RuntimeError("Gateway stopped after a malformed configuration reload")
            status, body = request(gateway_port, "gateway.acceptance.test", "/invalid-reload")
            if status != 200 or body != b"goreecloud-gateway-isolated-runtime\n":
                raise RuntimeError("malformed configuration replaced the last-known-good route")
            config_path.write_text(original_config, encoding="utf-8")
            process.send_signal(signal.SIGHUP)
            wait_for_route(
                gateway_port,
                process,
                "gateway.acceptance.test",
                "unmatched.acceptance.test",
            )

            load_request_count, max_observed_backend_concurrency = exercise_bounded_load(gateway_port)
            bounded_load_validated = load_request_count == LOAD_REQUESTS
            backpressure_validated = max_observed_backend_concurrency <= MAX_BACKEND_CONCURRENCY

            snapshot_output = run_recovery(
                recovery_binary,
                "-action",
                "snapshot",
                "-root",
                str(recovery_root),
                "-config",
                str(config_path),
                "-source-revision",
                source_revision,
            )
            snapshot = snapshot_output.get("snapshot")
            snapshot_dir = snapshot_output.get("snapshot_dir")
            if not isinstance(snapshot, dict) or not isinstance(snapshot_dir, str):
                raise RuntimeError("Gateway recovery snapshot output is incomplete")
            snapshot_config_sha256 = str(snapshot.get("config_sha256", ""))
            if snapshot.get("production_cutover_authorized") is not False or not re.fullmatch(
                r"[0-9a-f]{64}", snapshot_config_sha256
            ):
                raise RuntimeError("Gateway recovery snapshot safety evidence is invalid")

            changed_config = json.loads(json.dumps(config))
            changed_config["routes"][0]["hostname"] = "gateway.changed.acceptance.test"
            config_path.write_text(json.dumps(changed_config), encoding="utf-8")
            changed_config_sha256 = sha256(config_path)
            process.send_signal(signal.SIGHUP)
            wait_for_route(
                gateway_port,
                process,
                "gateway.changed.acceptance.test",
                "gateway.acceptance.test",
            )

            restore_receipt = run_recovery(
                recovery_binary,
                "-action",
                "restore",
                "-root",
                str(recovery_root),
                "-config",
                str(config_path),
                "-snapshot",
                snapshot_dir,
                "-expected-current-sha256",
                changed_config_sha256,
            )
            if restore_receipt.get("production_cutover_authorized") is not False:
                raise RuntimeError("Gateway recovery receipt authorized production cutover")
            if restore_receipt.get("restored_config_sha256") != snapshot_config_sha256:
                raise RuntimeError("Gateway recovery receipt does not match the prepared snapshot")
            if restore_receipt.get("config_validated") is not True:
                raise RuntimeError("Gateway recovery did not validate the restored configuration")
            if restore_receipt.get("compare_and_swap_validated") is not True:
                raise RuntimeError("Gateway recovery did not enforce compare-and-swap rollback protection")
            recovery_compare_and_swap_validated = True
            backup_restore_validated = True

            process.send_signal(signal.SIGHUP)
            wait_for_route(
                gateway_port,
                process,
                "gateway.acceptance.test",
                "gateway.changed.acceptance.test",
            )
            rollback_rehearsed = True

            process.send_signal(signal.SIGTERM)
            process.wait(timeout=10)
            if process.returncode != 0:
                stdout, stderr = process.communicate()
                raise RuntimeError(
                    f"Gateway graceful shutdown returned {process.returncode}: "
                    f"stdout={stdout.decode(errors='replace')!r} stderr={stderr.decode(errors='replace')!r}"
                )
            graceful_shutdown = True
    finally:
        if process is not None and process.poll() is None:
            process.kill()
            process.wait(timeout=5)
        backend.shutdown()
        backend.server_close()
        backend_thread.join(timeout=5)

    evidence = {
        "schema": SCHEMA,
        "source_revision": source_revision,
        "runtime_artifact_sha256": sha256(binary),
        "recovery_artifact_sha256": sha256(recovery_binary),
        "listener_scope": "127.0.0.1",
        "loopback_listener_validated": True,
        "route_proxy_validated": route_proxy_validated,
        "unmatched_route_validated": unmatched_route_validated,
        "backend_health_validated": route_proxy_validated,
        "bounded_load_validated": bounded_load_validated,
        "backpressure_validated": backpressure_validated,
        "load_request_count": load_request_count,
        "configured_max_backend_concurrency": MAX_BACKEND_CONCURRENCY,
        "max_observed_backend_concurrency": max_observed_backend_concurrency,
        "backup_restore_validated": backup_restore_validated,
        "rollback_rehearsed": rollback_rehearsed,
        "recovery_compare_and_swap_validated": recovery_compare_and_swap_validated,
        "snapshot_config_sha256": snapshot_config_sha256,
        "graceful_shutdown_validated": graceful_shutdown,
        "credentials_included": False,
        "production_cutover_authorized": False,
    }
    if not all(
        (
            evidence["loopback_listener_validated"],
            route_proxy_validated,
            unmatched_route_validated,
            evidence["backend_health_validated"],
            bounded_load_validated,
            backpressure_validated,
            backup_restore_validated,
            rollback_rehearsed,
            recovery_compare_and_swap_validated,
            graceful_shutdown,
        )
    ):
        raise SystemExit("Gateway isolated runtime acceptance evidence is incomplete")
    args.evidence.parent.mkdir(parents=True, exist_ok=True)
    args.evidence.write_text(json.dumps(evidence, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    print("GoreeCloud Gateway isolated runtime acceptance: PASS")


if __name__ == "__main__":
    main()

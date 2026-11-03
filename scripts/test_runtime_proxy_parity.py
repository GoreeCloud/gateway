#!/usr/bin/env python3
"""Synthetic HTTP proxy route, forwarded request-ID and WebSocket upgrade QA.

No DNS/ACME/network outside 127.0.0.1, private data or production Docker.
"""

import base64
import hashlib
import http.client
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import os
from pathlib import Path
import re
import secrets
import signal
import socket
import subprocess
import tempfile
import threading
import time

ROOT = Path(__file__).resolve().parent.parent
PROXY_PORT = 18880
UPSTREAM_PUBLIC = 18881
UPSTREAM_INTERNAL = 18882
WEBSOCKET_GUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"


class LocalBackend(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    def log_message(self, *_args):
        return  # Never print request paths, query parameters or headers.

    def do_GET(self):
        seen_id = self.headers.get("X-Request-ID", "")
        if self.path == "/ws":
            if self.headers.get("Upgrade", "").lower() != "websocket":
                self.send_error(400)
                return
            key = self.headers.get("Sec-WebSocket-Key", "")
            if not key:
                self.send_error(400)
                return
            digest = base64.b64encode(
                hashlib.sha1((key + WEBSOCKET_GUID).encode("ascii")).digest()
            ).decode("ascii")
            self.send_response(101, "Switching Protocols")
            self.send_header("Connection", "Upgrade")
            self.send_header("Upgrade", "websocket")
            self.send_header("Sec-WebSocket-Accept", digest)
            self.send_header("X-Backend-Request-ID", seen_id)
            self.end_headers()
            self.wfile.flush()
            time.sleep(0.2)
            self.close_connection = True
            return

        if self.path == "/redirect":
            self.send_response(307)
            self.send_header("Location", "/target")
            self.send_header("Content-Length", "0")
            self.send_header("X-Backend-Request-ID", seen_id)
            self.end_headers()
            return

        if self.path == "/stream":
            self.send_response(200)
            self.send_header("Content-Type", "text/plain; charset=utf-8")
            self.send_header("Transfer-Encoding", "chunked")
            self.send_header("X-Backend-Request-ID", seen_id)
            self.end_headers()
            for payload in (b"first ", b"second"):
                self.wfile.write(f"{len(payload):x}\r\n".encode("ascii"))
                self.wfile.write(payload + b"\r\n")
                self.wfile.flush()
                time.sleep(0.02)
            self.wfile.write(b"0\r\n\r\n")
            self.wfile.flush()
            return

        text = self.server.role + ":" + self.path
        body = text.encode("utf-8")
        self.send_response(200)
        self.send_header("Content-Type", "text/plain; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        self.send_header("X-Backend-Request-ID", seen_id)
        self.end_headers()
        self.wfile.write(body)


def get(path, headers=None):
    connection = http.client.HTTPConnection("127.0.0.1", PROXY_PORT, timeout=4)
    try:
        connection.request("GET", path, headers=headers or {})
        response = connection.getresponse()
        body = response.read()
        return response.status, body, response.headers
    finally:
        connection.close()


def verify_correlated_request(path, identity, *, headers=None):
    status, body, response_headers = get(path, headers)
    assert status == 200, "Unexpected proxy response status"
    assert body == identity.encode("utf-8"), "Wrong upstream selection"
    req_id = response_headers.get("X-Request-ID", "")
    assert re.fullmatch(r"[0-9a-f]{32}", req_id), "Malformed response request ID"
    assert response_headers.get("X-Backend-Request-ID") == req_id, (
        "Upstream request ID must agree with the response"
    )
    assert response_headers.get("X-Frame-Options") == "DENY"
    assert response_headers.get("X-Content-Type-Options") == "nosniff"
    return req_id


def verify_websocket_upgrade():
    key = base64.b64encode(secrets.token_bytes(16)).decode("ascii")
    expect = base64.b64encode(
        hashlib.sha1((key + WEBSOCKET_GUID).encode("ascii")).digest()
    ).decode("ascii")
    connection = http.client.HTTPConnection("127.0.0.1", PROXY_PORT, timeout=5)
    try:
        connection.request("GET", "/ws", headers={
            "Connection": "Upgrade",
            "Upgrade": "websocket",
            "Sec-WebSocket-Key": key,
            "Sec-WebSocket-Version": "13",
        })
        response = connection.getresponse()
        assert response.status == 101, "WebSocket upgrade not proxied"
        assert response.headers.get("Upgrade", "").lower() == "websocket"
        assert response.headers.get("Sec-WebSocket-Accept") == expect
        req_id = response.headers.get("X-Request-ID", "")
        assert re.fullmatch(r"[0-9a-f]{32}", req_id)
        assert response.headers.get("X-Backend-Request-ID") == req_id
    finally:
        connection.close()


def check_ports():
    for port in (PROXY_PORT, UPSTREAM_PUBLIC, UPSTREAM_INTERNAL):
        with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as sock:
            sock.settimeout(0.2)
            if sock.connect_ex(("127.0.0.1", port)) == 0:
                raise RuntimeError("A loopback test port is already in use")


def main():
    if not (ROOT / "gateway").is_file():
        raise RuntimeError("Build the audited development Gateway binary first")
    check_ports()
    servers = []
    processes = []
    threads = []
    with tempfile.TemporaryDirectory(prefix="goreecloud-runtime-parity-") as temp:
        base = Path(temp)
        log = (base / "gateway.log").open("wb")
        try:
            for port, role in ((UPSTREAM_PUBLIC, "public"), (UPSTREAM_INTERNAL, "internal")):
                server = ThreadingHTTPServer(("127.0.0.1", port), LocalBackend)
                server.daemon_threads = True
                server.role = role
                servers.append(server)
                worker = threading.Thread(target=server.serve_forever, daemon=True)
                worker.start()
                threads.append(worker)

            env = {**os.environ,
                   "XDG_DATA_HOME": str(base / "data"),
                   "XDG_CONFIG_HOME": str(base / "config")}
            child = subprocess.Popen([
                str(ROOT / "gateway"), "run", "--config",
                "configs/RuntimeParity.example", "--adapter", "caddyfile",
            ], cwd=ROOT, env=env, stdout=log, stderr=subprocess.STDOUT,
                start_new_session=True)
            processes.append(child)

            for _ in range(80):
                if child.poll() is not None:
                    raise RuntimeError("Test Gateway exited before ready")
                try:
                    if get("/")[0] == 200:
                        break
                except (OSError, http.client.HTTPException):
                    pass
                time.sleep(0.2)
            else:
                raise RuntimeError("Test Gateway failed to become ready")

            # Forge a client request ID; the Gateway must replace it securely.
            first_id = verify_correlated_request(
                "/target", "public:/target",
                headers={"X-Request-ID": "forged-id-not-trustworthy"},
            )
            second_id = verify_correlated_request(
                "/internal/health", "internal:/internal/health",
                headers={"X-Forwarded-For": "203.0.113.77"},
            )
            assert first_id != second_id, "Distinct requests must get distinct random IDs"

            status, body, headers = get("/redirect")
            assert status == 307 and body == b""
            assert headers.get("Location") == "/target"
            assert re.fullmatch(r"[0-9a-f]{32}", headers.get("X-Request-ID", ""))

            status, body, headers = get("/stream")
            assert status == 200 and body == b"first second"
            assert re.fullmatch(r"[0-9a-f]{32}", headers.get("X-Request-ID", ""))

            verify_websocket_upgrade()
            print("PASS: 2 proxied routes, request-ID integrity, headers, redirect, stream and WebSocket upgrade")
        finally:
            for child in reversed(processes):
                try:
                    os.killpg(child.pid, signal.SIGTERM)
                except ProcessLookupError:
                    pass
                try:
                    child.wait(timeout=3)
                except subprocess.TimeoutExpired:
                    try:
                        os.killpg(child.pid, signal.SIGKILL)
                    except ProcessLookupError:
                        pass
            for server in servers:
                server.shutdown()
                server.server_close()
            for worker in threads:
                worker.join(timeout=2)
            log.close()


if __name__ == "__main__":
    main()

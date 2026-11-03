#!/usr/bin/env python3
"""End-to-end HTTPS/CA trust test for the local Gateway web console.

Starts only isolated loopback services and generates an ephemeral test CA.
Does not make external DNS/ACME calls, access real Caddy config or deploy.
"""
from __future__ import annotations

import json
import os
from pathlib import Path
import secrets
import signal
import socket
import subprocess
import tempfile
import time

ROOT = Path(__file__).resolve().parent.parent
HTTP = "http://127.0.0.1:8080/"
ADMIN = "http://127.0.0.1:2019/config/"
CONSOLE_HEALTH = "http://127.0.0.1:7689/healthz"
HTTPS = "https://localhost:18443"


def assert_available_ports() -> None:
    for port in (2019, 7689, 8080, 18443):
        with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as sock:
            sock.settimeout(0.25)
            if sock.connect_ex(("127.0.0.1", port)) == 0:
                raise RuntimeError("Local integration test port is occupied")


def curl(url: str, *flags: str) -> tuple[int, str, str]:
    command = [
        "curl", "--silent", "--show-error", "--noproxy", "*",
        "--max-time", "3", "--dump-header", "/dev/stderr",
        "--write-out", "\n%{http_code}", *flags, url,
    ]
    result = subprocess.run(command, text=True, capture_output=True, timeout=5)
    if result.returncode != 0:
        raise RuntimeError("Local HTTPS/HTTP test request could not complete")
    body, code = result.stdout.rsplit("\n", 1)
    return int(code), body, result.stderr


def wait_ready(url: str, process: subprocess.Popen[bytes],
               *, tls_flags: tuple[str, ...] = ()) -> None:
    for _ in range(100):
        if process.poll() is not None:
            raise RuntimeError("An isolated Gateway test child stopped unexpectedly")
        try:
            if curl(url, *tls_flags)[0] == 200:
                return
        except (RuntimeError, ValueError):
            pass
        time.sleep(0.2)
    raise RuntimeError("Isolated Gateway test service did not become ready")


def create_disposable_ca(work: Path) -> tuple[Path, Path, Path]:
    """Create a synthetic one-day CA + localhost certificate without global trust."""
    def openssl(*args: str) -> None:
        subprocess.run(
            ["openssl", *args], check=True, timeout=20,
            stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
        )

    root_cert, root_key = work / "ca.crt", work / "ca.key"
    leaf_cert, leaf_key = work / "leaf.crt", work / "leaf.key"
    openssl(
        "req", "-x509", "-newkey", "ec",
        "-pkeyopt", "ec_paramgen_curve:prime256v1",
        "-nodes", "-days", "1", "-keyout", str(root_key),
        "-out", str(root_cert), "-subj", "/CN=GoreeCloud disposable test CA",
        "-addext", "basicConstraints=critical,CA:TRUE",
        "-addext", "keyUsage=critical,keyCertSign,cRLSign",
    )
    openssl(
        "req", "-new", "-newkey", "ec",
        "-pkeyopt", "ec_paramgen_curve:prime256v1",
        "-nodes", "-keyout", str(leaf_key),
        "-out", str(work / "leaf.csr"), "-subj", "/CN=localhost",
    )
    (work / "leaf.ext").write_text(
        "subjectAltName=DNS:localhost\n"
        "extendedKeyUsage=serverAuth\n"
        "keyUsage=digitalSignature\n"
        "basicConstraints=CA:FALSE\n", encoding="utf-8",
    )
    openssl(
        "x509", "-req", "-in", str(work / "leaf.csr"),
        "-CA", str(root_cert), "-CAkey", str(root_key),
        "-CAcreateserial", "-out", str(leaf_cert),
        "-days", "1", "-extfile", str(work / "leaf.ext"),
    )
    return root_cert, leaf_cert, leaf_key


def execute() -> None:
    for filename in ("gateway", "gateway-console"):
        path = ROOT / filename
        if not path.is_file() or not os.access(path, os.X_OK):
            raise RuntimeError("Build and verify both Gateway development binaries first")
    assert_available_ports()

    with tempfile.TemporaryDirectory(prefix="gateway-private-tls-") as scratch:
        work = Path(scratch)
        children: list[subprocess.Popen[bytes]] = []
        logfiles = []

        def start(args: list[str], logname: str, env: dict[str, str]) -> subprocess.Popen[bytes]:
            log = (work / logname).open("wb")
            logfiles.append(log)
            proc = subprocess.Popen(
                args, cwd=ROOT, env=env, stdout=log,
                stderr=subprocess.STDOUT, start_new_session=True,
            )
            children.append(proc)
            return proc

        def isolated_env(tag: str) -> dict[str, str]:
            return {
                **os.environ,
                "XDG_DATA_HOME": str(work / tag / "data"),
                "XDG_CONFIG_HOME": str(work / tag / "config"),
            }

        try:
            engine = start([
                str(ROOT / "gateway"), "run",
                "--config", "configs/Gatewayfile.example",
                "--adapter", "caddyfile",
            ], "engine.log", isolated_env("engine"))
            wait_ready(HTTP, engine)
            wait_ready(ADMIN, engine)

            token = secrets.token_hex(32)
            console = start(
                [str(ROOT / "gateway-console")], "console.log",
                {**isolated_env("console"), "GOREE_GATEWAY_CONSOLE_TOKEN": token},
            )
            wait_ready(CONSOLE_HEALTH, console)

            root_cert, leaf_cert, leaf_key = create_disposable_ca(work)
            proxy_env = isolated_env("proxy")
            proxy_env["GC_TEST_TLS_CERT"] = str(leaf_cert)
            proxy_env["GC_TEST_TLS_KEY"] = str(leaf_key)
            proxy = start([
                str(ROOT / "gateway"), "run",
                "--config", "configs/PrivateConsoleTLS.example",
                "--adapter", "caddyfile",
            ], "proxy.log", proxy_env)

            for _ in range(100):
                if proxy.poll() is not None:
                    raise RuntimeError("The isolated TLS proxy stopped unexpectedly")
                if root_cert.is_file():
                    try:
                        wait_flags = ("--resolve", "localhost:18443:127.0.0.1",
                                      "--cacert", str(root_cert))
                        if curl(HTTPS + "/", *wait_flags)[0] == 200:
                            break
                    except RuntimeError:
                        pass
                time.sleep(0.2)
            else:
                raise RuntimeError("No local trusted TLS endpoint became ready")

            tls_flags = ("--resolve", "localhost:18443:127.0.0.1",
                         "--cacert", str(root_cert))
            code, _, response_headers = curl(HTTPS + "/api/v1/overview", *tls_flags)
            assert code == 401, "Anonymous HTTPS API access must be denied"

            code, body, response_headers = curl(
                HTTPS + "/api/v1/overview", *tls_flags,
                "-H", "Authorization: Bearer " + token,
            )
            assert code == 200, "Authenticated HTTPS API returned an error"
            overview = json.loads(body)
            assert overview.get("api_version") == "v1"
            assert overview.get("state") == "connected"
            assert overview.get("servers", 0) >= 1
            assert overview.get("routes", 0) >= 1
            assert "content-security-policy:" in response_headers.lower()

            code, body, response_headers = curl(HTTPS + "/", *tls_flags)
            assert code == 200 and "GoreeCloud Gateway" in body
            assert "x-frame-options: deny" in response_headers.lower()
            assert "cache-control: no-store" in response_headers.lower()
            assert not (work / "proxy" / "data" / "caddy" / "pki").exists(), "Unexpected implicit Caddy PKI state"
            print("PASS: authenticated Gateway API and web console through isolated verified local HTTPS")
        finally:
            for proc in reversed(children):
                try:
                    os.killpg(proc.pid, signal.SIGTERM)
                except ProcessLookupError:
                    pass
                try:
                    proc.wait(timeout=4)
                except subprocess.TimeoutExpired:
                    try:
                        os.killpg(proc.pid, signal.SIGKILL)
                    except ProcessLookupError:
                        pass
            for file in logfiles:
                file.close()


if __name__ == "__main__":
    execute()

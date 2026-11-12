#!/usr/bin/env python3
"""Offline Caddy/Porkbun candidate parity; never reads production config or secrets.

Requires an already-built, reviewed local Gateway executable. Only invokes
'version', 'list-modules --versions' and synthetic 'adapt --config ...'.
It DOES NOT start listeners, contact ACME, issue certificates or authorize cutover.
"""
from __future__ import annotations

import argparse
import json
import os
from pathlib import Path
import re
import selectors
import signal
import stat
import subprocess
import time

SCHEMA = "goreecloud.gateway.offline-module-parity.v1"
VERSION = re.compile(r"v[0-9]+\.[0-9]+\.[0-9]+(?:[-+.][A-Za-z0-9.-]+)?\Z")
EXPECTED_MODULES = (
    "dns.providers.porkbun",
    "http.handlers.reverse_proxy",
    "tls.issuance.acme",
    "http.handlers.goree_ip_allowlist",
    "http.handlers.goree_security_headers",
    "http.handlers.goree_request_id",
)


class CandidateError(Exception):
    pass


def pinned_version(go_mod: str, module: str) -> str:
    matched = re.findall(r"(?m)^\s*" + re.escape(module) + r"\s+(v[^\s]+)", go_mod)
    if len(matched) != 1 or not VERSION.fullmatch(matched[0]):
        raise CandidateError("unverifiable source dependency")
    return matched[0]


def parse_modules(source: str) -> dict[str, str]:
    result = {}
    for line in source.splitlines():
        fields = line.split()
        if len(fields) == 1:
            result[fields[0]] = ""
        elif len(fields) == 2 and VERSION.fullmatch(fields[1]):
            result[fields[0]] = fields[1]
    return result


def synthetic_environment() -> dict[str, str]:
    # Intentionally do NOT forward host DNS provider credentials.
    env = {k: os.environ[k] for k in ("PATH", "HOME", "LANG")
           if k in os.environ}
    env["PORKBUN_API_KEY"] = "test-only-noncredential"
    env["PORKBUN_API_SECRET_KEY"] = "test-only-noncredential"
    return env


MAX_CANDIDATE_OUTPUT = 1_048_576
MAX_CANDIDATE_RUNTIME_SECONDS = 15


def run_candidate(binary: Path, args: list[str], *, env: dict[str, str]) -> str:
    """Read candidate stdout incrementally; never buffer unbounded output.

    No shell, captured stderr, live listener, credentials or network target.
    A fresh child process group lets timeouts stop descendant stdout writers.
    """
    process = None
    finished = False
    try:
        process = subprocess.Popen(
            [os.path.abspath(binary), *args],
            stdout=subprocess.PIPE, stderr=subprocess.DEVNULL,
            env=env, start_new_session=True,
        )
        output = bytearray()
        deadline = time.monotonic() + MAX_CANDIDATE_RUNTIME_SECONDS
        with selectors.DefaultSelector() as selector:
            selector.register(process.stdout, selectors.EVENT_READ)
            while selector.get_map():
                left = deadline - time.monotonic()
                if left <= 0:
                    raise CandidateError("candidate execution unavailable")
                for key, _ in selector.select(timeout=left):
                    chunk = os.read(
                        key.fileobj.fileno(),
                        min(65536, MAX_CANDIDATE_OUTPUT + 1 - len(output)),
                    )
                    if not chunk:
                        selector.unregister(key.fileobj)
                    else:
                        output.extend(chunk)
                        if len(output) > MAX_CANDIDATE_OUTPUT:
                            raise CandidateError("candidate verification failed")
                if time.monotonic() >= deadline and selector.get_map():
                    raise CandidateError("candidate execution unavailable")
        left = deadline - time.monotonic()
        if left <= 0 or process.wait(timeout=left) != 0:
            raise CandidateError("candidate verification failed")
        finished = True
        return output.decode("utf-8", errors="strict")
    except (OSError, UnicodeError, subprocess.TimeoutExpired):
        raise CandidateError("candidate execution unavailable") from None
    finally:
        if process is not None:
            if not finished:
                try:
                    os.killpg(process.pid, signal.SIGKILL)
                except ProcessLookupError:
                    pass
            try:
                process.wait(timeout=2)
            except subprocess.TimeoutExpired:
                process.kill()
                process.wait(timeout=2)
            if process.stdout is not None:
                process.stdout.close()


def inspect_candidate(binary: Path, source_root: Path, *, runner=run_candidate) -> dict:
    try:
        info = binary.lstat()
        if (not stat.S_ISREG(info.st_mode) or info.st_uid != os.geteuid()
                or info.st_mode & 0o022 or not os.access(binary, os.X_OK)
                or info.st_size < 1024):
            raise CandidateError("unsafe candidate executable")
        go_mod_path = source_root / "go.mod"
        if not go_mod_path.is_file() or go_mod_path.stat().st_size > 256_000:
            raise CandidateError("source module manifest unavailable")
        go_mod = go_mod_path.read_text(encoding="utf-8")
        expected_caddy = pinned_version(go_mod, "github.com/caddyserver/caddy/v2")
        expected_porkbun = pinned_version(go_mod, "github.com/caddy-dns/porkbun")
        env = synthetic_environment()
        version = runner(binary, ["version"], env=env).split(" ", 1)[0].strip()
        modules = parse_modules(runner(binary, ["list-modules", "--versions"], env=env))
        complete = all(module in modules for module in EXPECTED_MODULES)
        version_match = version == expected_caddy
        porkbun_match = modules.get("dns.providers.porkbun") == expected_porkbun
        engine_module_match = all(modules.get(m) == expected_caddy
                                  for m in ("http.handlers.reverse_proxy",
                                            "tls.issuance.acme"))
        adapted = {}
        for fixture in ("Porkbunfile.example", "RuntimeParity.example"):
            config = source_root / "configs" / fixture
            if not config.is_file() or config.stat().st_size > 100_000:
                raise CandidateError("synthetic source fixture unavailable")
            output = runner(binary, [
                "adapt", "--config", str(config), "--adapter", "caddyfile"
            ], env=env)
            try:
                adapted[fixture] = json.loads(output)
            except (ValueError, TypeError):
                raise CandidateError("unparseable synthetic adaptation") from None
        porkbun_shape = '"porkbun"' in json.dumps(adapted["Porkbunfile.example"])
        proxy_shape = '"reverse_proxy"' in json.dumps(adapted["RuntimeParity.example"])
        passed = all((complete, version_match, porkbun_match,
                      engine_module_match, porkbun_shape, proxy_shape))
        return {
            "schema": SCHEMA,
            "inspection": "isolated_prebuilt_binary_with_synthetic_configs",
            "candidate_engine_matches_source": version_match,
            "porkbun_module_version_matches_source": porkbun_match,
            "required_module_names_present": complete,
            "core_module_versions_match_source": engine_module_match,
            "synthetic_porkbun_adapts": porkbun_shape,
            "synthetic_proxy_adapts": proxy_shape,
            "candidate_parity_checks_passed": passed,
            "live_porkbun_credentials_verified": False,
            "acme_renewal_verified": False,
            "private_site_parity_verified": False,
            "backup_restoration_verified": False,
            "rollback_verified": False,
            "production_changed": False,
            "production_cutover_authorized": False,
        }
    except (OSError, UnicodeError, ValueError) as exc:
        raise CandidateError("candidate or source validation unavailable") from None


def main(argv=None) -> int:
    parser = argparse.ArgumentParser(description="Offline-only source/candidate Caddy parity")
    parser.add_argument("--binary", type=Path, required=True)
    parser.add_argument("--source-root", type=Path,
                        default=Path(__file__).resolve().parents[1])
    args = parser.parse_args(argv)
    try:
        result = inspect_candidate(args.binary, args.source_root)
    except CandidateError:
        print(json.dumps({
            "schema": SCHEMA, "candidate_parity_checks_passed": False,
            "production_changed": False, "production_cutover_authorized": False,
            "error": "offline_preflight_failed",
        }, sort_keys=True))
        return 2
    print(json.dumps(result, sort_keys=True))
    return 0 if result["candidate_parity_checks_passed"] else 1


if __name__ == "__main__":
    raise SystemExit(main())

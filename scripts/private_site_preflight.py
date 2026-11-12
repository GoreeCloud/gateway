#!/usr/bin/env python3
"""Redacted read-only Caddyfile site-header preflight; not TLS or route proof.

Only literal, one-line root site headers are examined. Imports, Caddy snippets,
runtime API overrides, and live certificate issuance are not resolved.
No hostnames, config text, secrets or exception details are printed.
"""
import argparse
import json
import re
from pathlib import Path

from caddyfile_inventory import read_bounded

HOST_LABEL = re.compile(r"[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\Z")
SITE_ADDRESS = re.compile(r"(?:(https?)://)?(\*\.)?([a-z0-9.-]+)(?::([0-9]{1,5}))?\Z")


def valid_target(value: str) -> str | None:
    if not isinstance(value, str) or len(value) > 253 or not value.isascii():
        return None
    host = value.lower()
    parts = host.split(".")
    if len(parts) < 2 or any(not HOST_LABEL.fullmatch(x) for x in parts):
        return None
    return host


def strip_comments_and_quoted_text(line: str) -> str:
    """Keep unquoted structural braces while ignoring quoted values/comments."""
    output = []
    quote = None
    escaped = False
    for ch in line:
        if quote is not None:
            output.append(" ")
            if escaped:
                escaped = False
            elif ch == "\\":
                escaped = True
            elif ch == quote:
                quote = None
        elif ch == "#":
            break
        elif ch in ("'", '"', chr(96)):
            quote = ch
            output.append(" ")
        else:
            output.append(ch)
    return "".join(output)


def classify_address(address: str, target: str) -> str | None:
    match = SITE_ADDRESS.fullmatch(address.strip().lower())
    if match is None:
        return None
    scheme, wildcard, host, raw_port = match.groups()
    labels = host.split(".")
    if len(labels) < 2 or any(not HOST_LABEL.fullmatch(s) for s in labels):
        return None
    port = int(raw_port) if raw_port else None
    if port is not None and not 1 <= port <= 65535:
        return None
    if wildcard:
        # A DNS wildcard covers exactly one label; not the zone apex or grandchildren.
        if target.count(".") != host.count(".") + 1 or not target.endswith("." + host):
            return None
    elif host != target:
        return None
    if scheme == "http" or (scheme is None and port == 80):
        return "http_only"
    if wildcard:
        return "wildcard_candidate"
    if scheme == "https":
        return "explicit_https_candidate"
    return "implicit_https_candidate"


def assess(source: str, target: str) -> dict:
    """Static indicators only. Never return site names or raw config."""
    if valid_target(target) is None:
        raise ValueError("invalid target")
    target = target.lower()
    found = set()
    depth = 0
    unbalanced = False
    imports = False
    for raw in source.splitlines():
        line = strip_comments_and_quoted_text(raw).strip()
        if not line:
            continue
        if re.match(r"^import(?:\s|$)", line):
            imports = True
        if depth == 0 and line.endswith("{"):
            header = line[:-1].strip()
            # Root global options and named snippets are not site addresses.
            if header and not header.startswith("("):
                for address in header.split(","):
                    if kind := classify_address(address, target):
                        found.add(kind)
        depth += line.count("{") - line.count("}")
        if depth < 0:
            unbalanced = True
            depth = 0
    unbalanced = unbalanced or depth != 0
    priority = (
        "explicit_https_candidate", "implicit_https_candidate",
        "wildcard_candidate", "http_only"
    )
    selected = next((status for status in priority if status in found), None)
    # Imported Caddyfiles can add or override effective site declarations.
    # Never issue a positive candidate verdict from an incomplete source tree.
    if unbalanced or imports:
        selected = "indeterminate"
    if selected is None:
        selected = "absent_in_visible_root_headers"
    return {
        "schema_version": 1,
        "analysis": "static_root_site_header_candidates_not_tls_or_runtime_validation",
        "site_declaration": selected,
        "unresolved_imports": imports,
        "balanced_braces_textually": not unbalanced,
    }


def main() -> int:
    parser = argparse.ArgumentParser(description="Read-only, hostname-redacted Caddyfile site preflight")
    parser.add_argument("config", type=Path)
    parser.add_argument("hostname")
    args = parser.parse_args()
    try:
        if valid_target(args.hostname) is None:
            raise ValueError("invalid target")
        result = assess(read_bounded(args.config), args.hostname)
    except (OSError, UnicodeError, ValueError):
        print(json.dumps({"schema_version": 1, "error": "cannot_assess_safe_input"}))
        return 2
    print(json.dumps(result, sort_keys=True))
    if result["site_declaration"] == "indeterminate":
        return 2
    return 0 if result["site_declaration"].endswith("_candidate") else 1


if __name__ == "__main__":
    raise SystemExit(main())

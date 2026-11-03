#!/usr/bin/env python3
"""Count Caddyfile migration features without printing confidential configuration.

This is a static text inventory, NOT a config parser or runtime compatibility test.
It cannot resolve imports, distinguish nested routing policies, or prove TLS health.
It neither writes files nor contacts networks.
"""

import argparse
import json
import re
from pathlib import Path

MAX_SIZE = 1_048_576
DIRECTIVES = (
    "reverse_proxy",
    "tls",
    "handle",
    "handle_path",
    "header",
    "encode",
    "basic_auth",
    "basicauth",
    "respond",
    "import",
    "route",
    "redir",
    "file_server",
    "forward_auth",
    "log",
    "goree_request_id",
    "goree_security_headers",
    "goree_ip_allowlist",
)
DIRECTIVE_STARTS = {
    directive: re.compile(r"^\s*" + re.escape(directive) + r"(?=\s|\{|$)")
    for directive in DIRECTIVES
}
PORKBUN = re.compile(r"^\s*dns\s+porkbun(?=\s|\{|$)", re.IGNORECASE)
DNS = re.compile(r"^\s*dns(?=\s|\{|$)", re.IGNORECASE)
PLACEHOLDER = re.compile(r"\{\$[A-Za-z_][A-Za-z0-9_]*\}|\{env\.[A-Za-z_][A-Za-z0-9_]*\}")
PORKBUN_SECRET = re.compile(r"^\s*(api_key|api_secret_key)\s+(.+?)\s*$", re.IGNORECASE)
SAFE_PLACEHOLDER = re.compile(
    r"^\{(?:env\.[A-Za-z_][A-Za-z0-9_]*|\$[A-Za-z_][A-Za-z0-9_]*)\}$"
)


def count_features(source: str) -> dict:
    counts = {key: 0 for key in DIRECTIVES}
    count_dns = count_porkbun = exposed_provider_secrets = active = 0
    for line in source.splitlines():
        # Caddyfile is a context-sensitive language; counts are deliberately
        # approximate. Only whole leading directive words are accepted.
        if not line.strip() or line.lstrip().startswith("#"):
            continue
        active += 1
        for directive, start in DIRECTIVE_STARTS.items():
            if start.match(line):
                counts[directive] += 1
        if DNS.match(line):
            count_dns += 1
        if PORKBUN.match(line):
            count_porkbun += 1
        m = PORKBUN_SECRET.match(line)
        if m and not SAFE_PLACEHOLDER.fullmatch(m.group(2)):
            exposed_provider_secrets += 1
    return {
        "schema_version": 1,
        "analysis": "text_counts_only_not_config_validation",
        "total_lines": len(source.splitlines()),
        "active_lines": active,
        "directives": counts,
        "dns_provider_directives": count_dns,
        "porkbun_dns_directives": count_porkbun,
        "env_placeholder_occurrences": len(PLACEHOLDER.findall(source)),
        "inline_provider_credentials_suspected": exposed_provider_secrets > 0,
        "unresolved_import_directives": counts["import"] > 0,
        "braces_balanced_textually": source.count("{") == source.count("}"),
    }


def read_bounded(path: Path) -> str:
    # Do not print the path, file contents, errors or exception strings.
    with path.open("rb") as handle:
        data = handle.read(MAX_SIZE + 1)
    if len(data) > MAX_SIZE:
        raise ValueError("oversized")
    return data.decode("utf-8", errors="strict")


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Read-only, redacted Caddyfile migration feature inventory."
    )
    parser.add_argument("config", type=Path, help="Authorized local Caddyfile path")
    args = parser.parse_args()
    try:
        content = read_bounded(args.config)
    except (OSError, UnicodeError, ValueError):
        print(json.dumps({"schema_version": 1, "error": "cannot_read_safe_input"}))
        return 2
    print(json.dumps(count_features(content), sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

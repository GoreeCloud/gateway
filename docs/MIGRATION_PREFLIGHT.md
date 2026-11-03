# Migration preflight: redacted Caddyfile inventory

**Status:** Read-only static analysis tool and synthetic adapter test. This is not a config parser, a backup, an ACME test or proof of deployment parity.

## Scope

GoreeCloud Gateway can inspect the **counts** of selected migration directives without exporting the real Caddyfile, domain names, upstream hostnames, credentials or request headers. The tool reads an explicitly provided local file (maximum 1 MiB), emits a bounded JSON object containing only counts and status flags, and never writes configuration, launches containers, opens ports or connects to external services.

Usage on an authorized machine:

```sh
python3 scripts/caddyfile_inventory.py /path/to/authorized/Caddyfile
```

Its counts distinguish `reverse_proxy`, `tls`, `dns porkbun`, route handlers, headers, `basic_auth`, encodings, and certain other known directives. It only counts leading text tokens and cannot handle all Caddyfile syntax, quoted strings, imports, snippets, directives nested through other files, runtime placeholders, or route semantics. If imports are detected, review all referenced files under access controls; do not treat the displayed counts as exhaustive. The `inline_provider_credentials_suspected` flag is only a heuristic and may report false positives or negatives.

## Synthetic compatibility test

`configs/LegacyMigration.example` contains a completely synthetic, non-live representative configuration with Porkbun DNS-01, response headers, gzip/zstd encoding, an HTTP health handler, HTTP Basic Auth, and two reverse-proxy upstreams. The CI workflow generates an ephemeral **test-only** bcrypt hash and calls `gateway adapt` on this example. No DNS-01 challenge or certificate operation occurs during adaptation.

**Never copy a production Caddyfile, tokens, internal hostnames, or certificate material into this public repository, a CI run, or client-side diagnostics.** Move from static counts to staged config adaptation only inside an approved confidential environment, with an independently confirmed backup, configured runtime secrets, and known rollback.

See [CUTOVER.md](CUTOVER.md), [PORKBUN.md](PORKBUN.md) and [INGRESS_RECOVERY.md](INGRESS_RECOVERY.md).

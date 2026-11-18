# Security model and GoreeCloud platform responsibilities

## Implemented

Caddy v2.11.9 is the upstream proxy/TLS engine. The optional upstream Porkbun DNS-01 v0.3.1 plugin registers only for Caddyfile/CLI compatibility, not remote key management.

The managed HTTP console binds only literal loopback IP addresses, rejects non-loopback/DNS-rebound Host headers, and uses a process-configured bearer secret (minimum 24 characters). The React application holds the secret only in browser-tab memory. Duplicate Authorization headers fail closed. Ten failed authentications from the **direct TCP peer IP (excluding ephemeral TCP source ports)** within one minute trigger a temporary 429 response; forwarded-address headers are ignored, and no identifying history is persisted. The HTTP response sets restrictive CSP, no-store caching, clickjacking prevention, cross-origin and browser permission headers.

Management route writes now enforce a strict JSON media type and an actual 8 KiB request-size ceiling, rejecting duplicate Content-Type values, unsupported charsets, and extra trailing bytes. Managed configuration is rendered through the actual Caddyfile adapter, applied to the native engine, and persisted atomically as mode 0600. A failed persistence attempts to reload the previous engine configuration. The engine's own admin interface is disabled for managed routes. Its autosave and certificate state are isolated under a private 0700 Gateway state directory rather than default user Caddy directories. The operator can select a persistent `--state-dir`.

## Mandatory integrated platform systems

| MIPS owner | Domain | Implementation versus outstanding acceptance |
| --- | --- | --- |
| GoreeCloud UI | Verified design language and accessibility | React uses current alpha semantic tokens; browser assistive-technology acceptance remains open |
| GoreeCloud Mesh | Authorized interoperability and synchronization | No live Mesh worker or service registration |
| GoreeCloud Identity | Authentication and authorization | Local single-token only; no multi-user roles, OIDC or remote session acceptance |
| GoreeShield | Privacy, consent and retention | No tracking/analytics or browser token persistence; formal privacy acceptance pending |
| Wardveil | Cybersecurity defense and incident response | Local validation and dependency checks; no deployed Wardveil monitoring integration |

## Residual risk

No application-level persistent audit log, certificate inventory, active upstream checks, account recovery, tenant permissions, signed release update channel, DNS-01 integration test, or production-approved backup/restore. A loopback-only HTTP management console is intended for local access or a separately secured SSH tunnel. **Never expose the console directly to the public Internet or expose the Caddy admin API.** A passing source test is not a live TLS or production security certification.

## Go dependency vulnerability maintenance

The patched source pins `github.com/klauspost/compress` v1.20.2 to remediate [GO-2026-6865](https://pkg.go.dev/vuln/GO-2026-6865), `golang.org/x/net` v0.62.0 for GO-2026-6860 and GO-2026-6866, and `golang.org/x/crypto` v0.59.0 for GO-2026-5932. The protected CI `validate` check includes the official `govulncheck@v1.10.0` source-reachability scan; module metadata is submitted to the Go vulnerability database and Go telemetry is disabled. Passing it does not prove the absence of all vulnerabilities, protect a deployed legacy Caddy version, or provide live recovery.

# Security model and GoreeCloud platform responsibilities

## Implemented

Caddy v2.11.9 is the upstream proxy/TLS engine. The optional upstream Porkbun DNS-01 v0.3.1 plugin registers only for Caddyfile/CLI compatibility, not remote key management.

The managed HTTP console binds only literal loopback IP addresses, rejects non-loopback/DNS-rebound Host headers, and uses a process-configured bearer secret (minimum 24 characters). The React application holds the secret only in browser-tab memory. Duplicate Authorization headers fail closed. Ten failed authentications from the **direct TCP peer IP (excluding ephemeral TCP source ports)** within one minute trigger a temporary 429 response; forwarded-address headers are ignored, and no identifying history is persisted. The volatile per-peer limiter has a hard 256-entry capacity: expired records are reclaimed, then the oldest entry may be evicted if locally saturated. This bounds in-memory tracking but is not a protection against distributed attack or a hostile local administrator. The HTTP response sets restrictive CSP, no-store caching, clickjacking prevention, cross-origin and browser permission headers.

Management route writes now enforce a strict JSON media type and an actual 8 KiB request-size ceiling, rejecting duplicate Content-Type values, unsupported charsets, and extra trailing bytes. Managed configuration is rendered through the actual Caddyfile adapter, applied to the native engine, and persisted atomically as mode 0600. Gateway refuses saved-site file paths that traverse symlinked parent directories at startup, before runtime apply, and before atomic persistence; this is a point-in-time check that does not eliminate attacks by a hostile writer with permission to replace parent directories. A failed persistence attempts to reload the previous engine configuration. The engine's own admin interface is disabled for managed routes. Its autosave and certificate state are isolated under a private 0700 Gateway state directory rather than default user Caddy directories. The storage path is checked for existing symlink components **before** directory creation and again after; this reduces accidental redirected directory creation but remains a point-in-time check, not a defense against hostile concurrent path replacement. The operator can select a persistent `--state-dir`.

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

The patched source pins `github.com/klauspost/compress` v1.20.2 to remediate [GO-2026-6865](https://pkg.go.dev/vuln/GO-2026-6865), `golang.org/x/net` v0.62.0 for GO-2026-6860 and GO-2026-6866, and `golang.org/x/crypto` v0.59.0 for GO-2026-5932. The protected CI uses patched Go 1.27.4 and `validate` includes the official `govulncheck@v1.10.0` source-reachability scan; module metadata is submitted to the Go vulnerability database and Go telemetry is disabled. Passing it does not prove the absence of all vulnerabilities, protect a deployed legacy Caddy version, or provide live recovery.

## Additional dependency advisory review

GitHub Dependabot flagged the OpenTelemetry
[GHSA-q7gw-24fr-5p69](https://github.com/advisories/GHSA-q7gw-24fr-5p69)
dependency vulnerability. This source upgrades the affected `otel/trace`,
`otel/sdk` and `otel/sdk/log` modules to v1.47.0, with compatible related
modules. Go tests, module verification and the official reachable-code scanner
must pass for the exact source commit. The runtime is still a development alpha.

GitHub also lists **three separate unpatched** `go-jose/go-jose/v3` advisories
([GHSA-mgcc-cww5-fp2q](https://github.com/advisories/GHSA-mgcc-cww5-fp2q),
[GHSA-4w3g-jfj2-cw2q](https://github.com/advisories/GHSA-4w3g-jfj2-cw2q),
and [GHSA-vvm8-gf78-28g4](https://github.com/advisories/GHSA-vvm8-gf78-28g4)).
Caddy's standard PKI imports `go.step.sm/crypto/jose`, which imports affected
`go-jose/v3` v3.0.5. Upstream has not published a fixed v3 release as of this
review. Do **not** mark these alerts dismissed, downgrade the advisory, assert
runtime exploitability, remove Caddy modules without a compatibility review, or
equate a zero-reachable-result source scan with resolution of these alerts.
Independent review and a verified upstream/compatible patch remain open.

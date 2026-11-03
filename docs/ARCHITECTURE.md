# GoreeCloud Gateway — Architecture and Security Boundary

| Field | Value |
| --- | --- |
| Owner | GoreeCloud founder |
| Status | Development foundation; no production acceptance |
| Updated | 2026-11-03 |
| Canonical path | `GoreeCloud/gateway/docs/ARCHITECTURE.md` |

## Ownership

I am building GoreeCloud Gateway as an original distribution that embeds a version-pinned Caddy upstream engine. Core TLS, proxy, HTTP protocols, config adapters, and default CLI remain upstream technologies. GoreeCloud owns the application integration and the custom `goree_*` modules, not Caddy's upstream authorship.

## Feature boundaries

1. `goree_request_id` generates a new unpredictable 128-bit ID and propagates it to the proxied request and response, intentionally discarding inbound IDs.
2. `goree_security_headers` is opt-in. It uses safe headers but the `DENY` framing default may break legitimate embed workflows. No global CSP/HSTS policy is imposed.
3. `goree_ip_allowlist` is an opt-in CIDR gate evaluated using the directly connected socket address. It does **not** trust unverified `X-Forwarded-For` or expose all private networks by default.
4. Default Caddy functionality remains available through the vendored-by-reference, versioned Go module and standard module imports.

## Security model and limitations

- Do not expose the Caddy admin API publicly. Default is local-only; if a reverse proxy/load balancer fronts Gateway, its trust configuration must be reviewed.
- Preserve existing HTTPS keys/certificate state with restrictive Unix ownership. Never put cert keys or live config with credentials into the public GitHub repo.
- Per-app hardening, body limits, WAF, authentication, rate limits, metrics, and audits must be verified separately before any claims of coverage.
- Admin UI is not implemented. Prefer no dashboard over an unauthenticated production control plane.
- Test the composed configuration with `gateway adapt`, `gateway validate`, live HTTP/TLS probes, and application integration tests.

## Compatibility

Caddy upstream changes may require new tests or migrations. Upstream version is pinned but transitive dependency checks, SBOM, signing, and CVE review remain release gates. The minimum supported toolchain comes from the pinned Caddy build requirement, not local developer availability.

## Next roadmap candidates

- Authenticated, loopback-bound or Zero-Trust-only control plane with GC UI components and audited edits
- Declarative service registry and config checks with dry-run and reversible commits
- Structured telemetry, request tracing, configurable retention, health checks, and safe alerting
- Security policy management, rate limiting, WAF integrations, mTLS, and SSO
- Verified packaging, hardened systemd service, canary deployment, rollback rehearsals

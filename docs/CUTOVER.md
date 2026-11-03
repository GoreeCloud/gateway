# GoreeCloud Gateway — VPS Cutover and Rollback Checklist

| Field | Value |
| --- | --- |
| Owner | GoreeCloud founder |
| Status | Unexecuted runbook / production cutover NOT authorized by this file |
| Updated | 2026-11-03 |
| Canonical path | `GoreeCloud/gateway/docs/CUTOVER.md` |

This is a **procedure**, not evidence of completion or a license to switch active infrastructure.

## 1. Discover and freeze baseline

- Verify the precise VPS hosting public ingress through SSH, service units, process list, TLS endpoint checks, listener ownership on TCP 80/443 and UDP 443, and DNS. For existing containerized Caddy, follow [read-only recovery diagnostics](INGRESS_RECOVERY.md) including the fixed-field JSON evidence collector; outputs never replace privileged backup/restore verification. Empty current listeners alone do not prove a previous Docker port/network conflict is resolved.
- Inventory every currently served hostname, internal upstream address, application dependency, WebSocket/HTTP3 behavior, redirects, middleware, certificate issuer and renewal state. The [read-only Caddyfile counter](MIGRATION_PREFLIGHT.md) can help identify a bounded, secret-redacted directive inventory, but is not a route parser or an acceptance result. Keep secrets out of public repositories and logs.
- Confirm backup health and independent access (console/SSH) in case proxy-related remote access disappears.
- Establish an outage window and explicit production cutover approval. Preserve current Caddy executable, unit, environment, complete config, its data directory, access permissions, TLS storage, and hashes; secure backup with root-only access and verify restoration.
- Capture before-state HTTP response status/headers, TLS certificates/issuer/expiration, and uptime/latency baselines for a representative set of hosts. Check whether VPN and NetBird control-plane traffic relies on this proxy.

## 2. Stage without stealing live ports

- Build from a reviewed, CI-accepted commit pinned to upstream version. Produce repeatable, integrity-checked binary and SBOM; scan upstream dependencies.
- Run `go test -race ./...`, `go vet ./...`, `gateway list-modules`, `gateway adapt`, `gateway validate`, and feature integration tests. Check GC UI acceptance before adding any administrative UI.
- Validate the translated **sanitized** config in an isolated staging container/netns, using a nonconflicting loopback test listener. Do not accidentally request production ACME certs from staging.
- Match service identity, groups, file capabilities, sandboxing, environment and filesystem paths before sharing existing certificate stores.
- Verify requests to upstreams, forwarded headers, redirects, streaming, WebSockets, request IDs, allowlists, rate limiting controls if enabled, and the ACME lifecycle. The [synthetic runtime proxy test](RUNTIME_PROXY_TEST.md) now covers local masked WebSocket text-frame round trips but does not replace application-specific long-running WebSocket, HTTP/3, or production route acceptance.

## 3. Canary and production replacement

- Plan live listener handoff with an independently tested rollback and known service-manager unit names. Avoid running two daemons on 80/443 simultaneously.
- Record operator, date, commit/hash, backup location, health baseline, and rollback trigger in restricted operations records.
- Switch only after passing the preceding gates and obtaining the necessary authorization.
- Immediately test domains, HTTP-to-HTTPS, TLS, health, log redaction, authentication, expected routes, VPN-dependent endpoints, and relevant application workflows. Monitor 4xx/5xx, latency and certificate renewals.
- Do not remove old Caddy binary/data or revoke existing access until a burn-in period and rollback rehearsal have completed.

## 4. Rollback triggers and execution

- Triggers: certificate failure, unexpected route errors, broken login/SSO, significant 5xx rise, loss of remote control or VPN, or failure to serve mandatory hostnames.
- Restore the saved service unit, binary, config, environment, data directory/certificate paths and ownership using the independently verified console route; restart the original service only when 80/443 is clear.
- Re-run the same smoke probes. Preserve diagnostic logs safely and document exact affected systems, evidence and remaining remediation items.
- Never delete the known-good Caddy installation until Gateways's stable acceptance has been demonstrated in production.

## Acceptance evidence required

Passing GitHub CI alone is insufficient. A real end-to-end staging test, signed/reproducible package, dependency security review, tested backup/restore, monitored canary, GC UI visual tests for any control interface, and service-by-service parity are required before declaring GoreeCloud Gateway Stable.

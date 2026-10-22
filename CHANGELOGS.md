# GoreeCloud Gateway Changelogs

This repository-local record is the authoritative Git-native chronological change history for GoreeCloud Gateway source and repository governance. Production cutover, deployment, and lifecycle promotion remain separately evidence-gated.

## Unreleased — Development

### 2026-10-22 — Opt-in bounded native gzip/zstd for static responses

- Introduced a per-static-route response-encoding policy with `gzip`/`zstd` preference, a bounded minimum-length override, and fail-closed validation that forbids applying it to reverse-proxy streams.
- Encoded eligible bounded GET/HEAD literal bodies with quality-aware negotiation, cache variance, safe framing/validator handling, and no-transform/range guards. Added compression round-trip, negative-path, route integration, and parity-identity tests.
- Kept Caddy as the production authority. Native proxy/SSE/WebSocket compression, exact Caddy behavior parity, target acceptance, and production cutover are outside this change.

### 2026-10-22 — Sanitized Caddy encode-policy evidence gate

- Added a redacted, route-indexed `encode` inventory to the exact-source read-only Caddy collector: supported codecs, preference order, explicit minimum length or unspecified setting, response matcher presence, and subroute depth, with no raw matcher values or compressor options exposed.
- Required complete, non-contradictory, bounded per-encode fields and a matching handler count in migration-source report validation; added collector self-tests and negative verifier regressions.
- This evidence does not implement native gzip/zstd, attest exact Caddy response semantics, or authorize production migration; previously captured reports must be refreshed using the reviewed collector before being accepted by the tightened verifier.

### 2026-10-22 — Sanitized Caddy route-inventory completeness gate

- Strengthened the read-only migration verifier to reject missing, empty, contradictory, duplicated, or incomplete sanitized Caddy route records even when the source Caddyfile checksum matches.
- Bounded the number of accepted route records and validated each route's mandatory server, matcher fields, handler, and upstream entry presence, including intentionally empty matcher/upstream lists.
- Preserved the existing report format and kept independent source review and Caddy production authority mandatory. No target-VPS changes.

### 2026-10-22 — GC UI governance and migration-evidence schema v2

- Reconciled the nine-system platform contract, capability declarations, and active documentation to GoreeCloud UI (GC UI), without claiming a production-ready administration UI.
- Replaced the superseded design-system acceptance field with `gc_ui_validated` under a new `goreecloud-gateway-migration-evidence/v2` schema; old schema evidence is rejected, and the missing integration gate remains fail-closed.
- Migrated the Gateway declaration to versioned Platform Contract 2.1, with its nine-system `gc_ui` key, scoped Foundations target, Forge lifecycle metadata, and immutable reusable-workflow evaluator pin.
- Corrected the development repository identity and stale unmerged-runtime blocker. Caddy retains production authority; no deployment or cutover occurred.

### 2026-10-22 — Bounded native static-response routes

- Added explicit `static_response` route action supporting literal HTTP 200–599 statuses and up to 16 KiB response bodies, independent of upstream services and health probing.
- Preserved legacy reverse-proxy routes, required fail-closed validation of ambiguous actions, and included action/status/body semantics in deterministic parity evidence.
- Added isolated routing, HTTP GET/HEAD, configuration, and reload regressions. Caddy handler inventory still requires exact route-level review before any migration rehearsal or production cutover.

### 2026-10-22 — Bounded response-header set policy

- Added optional native route response-header set operations, with fail-closed control-character, duplicate-field, framing-header, and size validation.
- Applied route-specific headers to received proxied responses and bound their semantics into deterministic configuration-parity fingerprints.
- Added focused runtime, validation, and parity regressions; the limited set operation does not claim full Caddy handler parity or production acceptance.

### 2026-10-22 — Ambiguous Caddy container discovery fails closed

- Prevented the VPS read-only collector from silently selecting the first Caddy image when several Caddy containers are running, including independent staging ingress services.
- Required an independently verified explicit `CADDY_CONTAINER` selection in ambiguous environments; added single/ambiguous/missing-container self-tests and updated operator guidance.
- Discovered this through a non-mutating VPS preflight mismatch; separately rechecked the production `caddy` container source identity. No production Caddy or network changes.

### 2026-10-22 — Fail-closed read-only Caddy source-report binding

- Required the local migration verifier to compare the reviewed v2 Caddy source SHA-256 against a bounded, structurally complete read-only preflight report before emitting minimized parity evidence.
- Rejected absent, stale, ambiguous, oversized, malformed, or unsafe-status preflight evidence with synthetic regression cases; kept infrastructure detail out of minimized output.
- This source check cannot authenticate the report, replace protected source review, or authorize live cutover. No production mutation.

### 2026-10-21 — Explicit Caddy migration-source identity

- Advanced the reviewed migration-source manifest contract to v2, requiring separate immutable SHA-256 identities for the source Caddy configuration and the expected translated Gateway parity fingerprint.
- Bound the source configuration and retained review-evidence hashes into minimized parity evidence; added fail-closed tests for missing, conflated, malformed, and legacy identities.
- Kept Caddy authoritative and made no production deployment, certificate, firewall, DNS, or listener changes.

### 2026-10-21 — Configuration-parity trusted-proxy boundary

- Included canonical trusted-ingress-proxy prefixes in the deterministic Gateway migration-parity fingerprint; regression tests reject changes to the trust boundary while treating equivalent CIDR sets consistently.
- Clarified that the reviewed expected Gateway parity fingerprint is distinct from the raw source Caddyfile SHA-256, and that a reviewed translation must bind both identities.
- No production Caddy, listener, DNS, firewall, certificate, or routing changes.

### 2026-10-21 — Read-only Caddy migration evidence hardening

- Merged PR #31 to enrich the Caddy/VPS discovery collector with image ID, available repository digests, and bounded OCI labels; escape untrusted metadata; and report inaccessible host certificate directories and Compose checksums explicitly.
- Validated the exact collector source with Bash syntax checks, self-test, five green GitHub CI workflows, and read-only execution on `goreecloud-vps-01` against the running Caddy container.
- Retained Caddy production authority. This evidence does not substitute for independently reviewed configuration parity, credential/certificate acceptance, recovery/rollback, or cutover authorization.

### 2026-10-21 — Backend health-check hardening and toolchain security

- Integrated PR #18 into Development `main`: health probes no longer follow redirects, HTTP 2xx is required for readiness, and regression coverage guards against contacting redirect targets.
- Raised the Go toolchain floor to 1.27.2 with PR #18. A subsequent exact merged-main vulnerability scan detected reachable standard-library advisory `GO-2026-6809`; the follow-up merged PR #30 advanced `go.mod` to the documented fixed patch level, 1.27.3, with successful exact-head CI.
- Preserved Caddy production authority. No production listeners, DNS, firewall, certificates, proxy configuration, deployment, or cutover were changed by these source revisions.


### 2026-09-24 — PR #9 integrated Development Gateway foundation

- Added and iterated the first-party Gateway runtime/control-plane foundation, deterministic routing/backend selection, health-aware failover, streaming/upgraded-connection handling, trusted-proxy handling, and recovery/rollback primitives.
- Added migration-source/parity evidence, privacy-minimized status/publication preflight, isolated runtime, sustained-load/backpressure, and vulnerability-reachability gates.
- Added provider-neutral DNS-01 handling, a bounded Porkbun adapter, RFC 8555 DNS-01 issuance, encrypted ACME account-state handling, explicit registration/TOS controls, protected wrapping-key loading, rollover preparation/execution, read-only authority probing, explicit recovery, and guarded operator tooling.
- Added a read-only Caddy/VPS preflight collector for target evidence without mutating Caddy, Docker, firewall, DNS, certificates, or listeners.
- Integrated the validated PR #9 Development source into `main` as `0e0172d4b7766b6e497e9182546277ff7ab537a7`, including runtime/control-plane, recovery/migration evidence, Infrastructure Status/publication preflight, and certificate/ACME foundations.
- Recorded the historical design-system assertion, which has since been superseded by the current GC UI authority; the Gateway administration UI remains unaccepted.
- Migrated repository feature/change governance from retired `FEATURE-ROADMAP.md` / Drive-roadmap synchronization to `IMPLEMENTED-FEATURES.md`, `PLANNED-FEATURES.md`, and `CHANGELOGS.md`.
- Preserved Caddy as production-authoritative. No source or documentation change in this record authorizes listener transfer, production deployment, Release Candidate, Production Acceptance, or Stable status.

Exact candidate, merge, and merged-main workflow evidence is authoritative in GitHub rather than duplicated here as moving validation identifiers.

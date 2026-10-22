# GoreeCloud Gateway Changelogs

This repository-local record is the authoritative Git-native chronological change history for GoreeCloud Gateway source and repository governance. Production cutover, deployment, and lifecycle promotion remain separately evidence-gated.

## Unreleased — Development

### 2026-10-21 — Backend health-check hardening and toolchain security

- Integrated PR #18 into Development `main`: health probes no longer follow redirects, HTTP 2xx is required for readiness, and regression coverage guards against contacting redirect targets.
- Raised the Go toolchain floor to 1.27.2 with PR #18. A subsequent exact merged-main vulnerability scan detected reachable standard-library advisory `GO-2026-6809`; the follow-up Development source advances `go.mod` to the documented fixed patch level, 1.27.3, subject to fresh exact-head CI.
- Preserved Caddy production authority. No production listeners, DNS, firewall, certificates, proxy configuration, deployment, or cutover were changed by these source revisions.


### 2026-09-24 — PR #9 integrated Development Gateway foundation

- Added and iterated the first-party Gateway runtime/control-plane foundation, deterministic routing/backend selection, health-aware failover, streaming/upgraded-connection handling, trusted-proxy handling, and recovery/rollback primitives.
- Added migration-source/parity evidence, privacy-minimized status/publication preflight, isolated runtime, sustained-load/backpressure, and vulnerability-reachability gates.
- Added provider-neutral DNS-01 handling, a bounded Porkbun adapter, RFC 8555 DNS-01 issuance, encrypted ACME account-state handling, explicit registration/TOS controls, protected wrapping-key loading, rollover preparation/execution, read-only authority probing, explicit recovery, and guarded operator tooling.
- Added a read-only Caddy/VPS preflight collector for target evidence without mutating Caddy, Docker, firewall, DNS, certificates, or listeners.
- Integrated the validated PR #9 Development source into `main` as `0e0172d4b7766b6e497e9182546277ff7ab537a7`, including runtime/control-plane, recovery/migration evidence, Infrastructure Status/publication preflight, and certificate/ACME foundations.
- Reconciled the integrated source to current GLAZE UI V1.6 / 1.6.0 authority while keeping the future administration UI unaccepted.
- Migrated repository feature/change governance from retired `FEATURE-ROADMAP.md` / Drive-roadmap synchronization to `IMPLEMENTED-FEATURES.md`, `PLANNED-FEATURES.md`, and `CHANGELOGS.md`.
- Preserved Caddy as production-authoritative. No source or documentation change in this record authorizes listener transfer, production deployment, Release Candidate, Production Acceptance, or Stable status.

Exact candidate, merge, and merged-main workflow evidence is authoritative in GitHub rather than duplicated here as moving validation identifiers.

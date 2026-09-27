# GoreeCloud Gateway Changelogs

This repository-local record is the authoritative Git-native chronological change history for GoreeCloud Gateway source and repository governance. Production cutover, deployment, and lifecycle promotion remain separately evidence-gated.

## Unreleased — Development

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

## 2026-09-27

- Strengthened the read-only Caddy/VPS preflight to capture the running image ID, available repository digests, and bounded OCI revision/version/created/source labels for migration provenance without querying provider credentials or mutating production state.

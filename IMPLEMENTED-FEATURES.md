# GoreeCloud Gateway Implemented Features

**Lifecycle:** Development  
**Authority:** Git-native implemented-feature record for GoreeCloud Gateway  
**Production gateway authority:** Caddy remains authoritative until separately accepted cutover

This file records capabilities that are implemented in accepted `main` or in a specifically identified Development candidate. It does not convert Development source, CI, isolated runtime evidence, or migration rehearsal evidence into production acceptance.

## Accepted main foundation

- Canonical GoreeCloud Gateway product identity and branding-consumer contract.
- GNU AGPL v3 GoreeCloud-owned source licensing material.
- Explicit Caddy-authoritative migration boundary: Gateway does not own production TCP 80/443 publication merely because source exists.
- Repository governance, product specifications, feature summary, benefits, competitive objectives, and branding records.

## Integrated Development source foundation

Accepted `main` implements the PR #9 foundation at Development source/test level:

- first-party Go Gateway runtime and control-plane foundations;
- deterministic route and backend selection;
- health-aware failover;
- WebSocket/streaming and upgraded-connection handling;
- trusted-proxy and forwarding-identity enforcement;
- configuration validation, recovery, rollback, parity fingerprints, and migration-source identity contracts;
- isolated runtime and sustained-load/backpressure acceptance harnesses;
- privacy-minimized Infrastructure Status and publication-preflight contracts;
- a read-only target-Caddy/VPS preflight collector that does not mutate Caddy, Docker, firewall, DNS, certificates, or listeners, including image ID, repository-digest, and bounded OCI provenance-label evidence;
- provider-neutral DNS-01 challenge boundaries and a bounded Porkbun TXT adapter;
- RFC 8555 order-based DNS-01 renewal issuance with exact propagation/cleanup handling, fresh certificate-key/CSR generation, and independent staging/activation boundaries;
- encrypted ACME account-key persistence, explicit registration/TOS controls, protected wrapping-key loading, offline envelope restore, transactional rollover preparation/execution, read-only old/new-key authority probing, and explicit recovery;
- operator CLI wiring for account registration, rollover, probe, and recovery with explicit confirmations;
- source-level integration/evidence contracts for the nine Integral Platform Systems without representing those contracts as target-runtime acceptance.

PR #9 exact-head validation, merge evidence, and merged-main workflow results are authoritative in GitHub. Repository records must not duplicate moving candidate SHAs as long-lived feature authority.

## Explicitly not established

The integrated Development source does **not** establish:

- live target-VPS Caddy route/listener/TLS/firewall/Docker parity;
- production ACME/DNS credentials or live CA/Porkbun issuance/renewal acceptance;
- production-representative SLO/load acceptance on the target;
- target-environment backup/restore/recovery/rollback;
- accepted runtime integration with Manager, Privacy Shield, Wardveil Security, Everkeep, Glaze UI, Mesh, Identity, Policy, or Observability;
- production listener ownership, Caddy retirement, Release Candidate, Production Acceptance, or Stable status.

Implementation status must remain narrower than production and lifecycle status.

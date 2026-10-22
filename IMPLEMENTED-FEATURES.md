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
- configuration validation, recovery, rollback, trusted-proxy-aware parity fingerprints, and explicit v2 Caddy source/Gateway expected-parity identity contracts, and fail-closed read-only preflight source-checksum binding;
- isolated runtime and sustained-load/backpressure acceptance harnesses;
- privacy-minimized Infrastructure Status and publication-preflight contracts;
- a read-only target-Caddy/VPS preflight collector that does not mutate Caddy, Docker, firewall, DNS, certificates, or listeners, including image ID, available repository digests, bounded OCI provenance labels, metadata line-injection protection, and explicit reporting of inaccessible host-side evidence;
- provider-neutral DNS-01 challenge boundaries and a bounded Porkbun TXT adapter;
- RFC 8555 order-based DNS-01 renewal issuance with exact propagation/cleanup handling, fresh certificate-key/CSR generation, and independent staging/activation boundaries;
- encrypted ACME account-key persistence, explicit registration/TOS controls, protected wrapping-key loading, offline envelope restore, transactional rollover preparation/execution, read-only old/new-key authority probing, and explicit recovery;
- operator CLI wiring for account registration, rollover, probe, and recovery with explicit confirmations;
- source-level integration/evidence contracts for the nine Integral Platform Systems without representing those contracts as target-runtime acceptance.

PR #9 exact-head validation, merge evidence, and merged-main workflow results are authoritative in GitHub. Repository records must not duplicate moving candidate SHAs as long-lived feature authority.

## Accepted Development source — health-probe redirect boundary

PR #18 integrated a bounded backend health checker into accepted `main`: health probes do not follow HTTP redirects, only 2xx responses count as healthy, and focused tests assert that redirected targets are never contacted. These are Development source and isolated-test capabilities, not production target acceptance.

The Go toolchain patch floor is defined by `go.mod`. The vulnerability workflow checks the exact candidate or merged-main revision against the current Go vulnerability database; previously passing scans do not satisfy newly disclosed advisories. The October 21 merged-main scan identified `GO-2026-6809` in Go 1.27.2, with a fix in 1.27.3. New exact-head checks remain required for each subsequent toolchain update.

## Explicitly not established

The integrated Development source does **not** establish:

- live target-VPS Caddy route/listener/TLS/firewall/Docker parity;
- production ACME/DNS credentials or live CA/Porkbun issuance/renewal acceptance;
- production-representative SLO/load acceptance on the target;
- target-environment backup/restore/recovery/rollback;
- accepted runtime integration with Manager, Privacy Shield, Wardveil Security, Everkeep, Glaze UI, Mesh, Identity, Policy, or Observability;
- production listener ownership, Caddy retirement, Release Candidate, Production Acceptance, or Stable status.

Implementation status must remain narrower than production and lifecycle status.

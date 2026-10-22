# GoreeCloud Gateway Planned Features

**Lifecycle:** Development  
**Authority:** Git-native planned/deferred feature record for GoreeCloud Gateway

This file records work that remains planned, blocked by prerequisite, or not yet accepted. Completed Development source belongs in `IMPLEMENTED-FEATURES.md`; chronological change history belongs in `CHANGELOGS.md`.

## Migration and target-runtime acceptance

- Execute and independently review the sanitized read-only Caddy/VPS preflight on `goreecloud-vps-01`.
- Build a reviewed migration-source manifest from actual target Caddy state.
- Prove route, listener, backend, TLS/certificate, Docker-network, WebSocket/streaming, and applicable HTTP/2/gRPC parity.
- Run a non-conflicting alternate-port rehearsal on the target VPS.
- Establish production-representative latency, error-rate, sustained-load, backpressure, and recovery evidence.
- Capture and rehearse target backup/restore, known-good recovery, and reversible return to Caddy.
- Complete listener-ownership governance and explicit production-cutover authorization.

## Certificate and ACME acceptance

- Verify the target's current certificate issuance method.
- Complete approved operator registration/TOS review and execution.
- Provision and escrow the wrapping key through an approved secret source.
- Rehearse rollover/recovery outcomes on the target, preserving fail-closed handling for both-recognized, neither-recognized, and inconclusive states.
- Rehearse staging-CA/Porkbun issuance and renewal, public DNS propagation, retry/rate-limit/provider-outage behavior, scheduling, and alerting.
- Prove certificate activation/rollback and renewal behavior in the target environment.

## Administration and product completeness

- Build Services, Routes, Backends, Certificates, Discovery, Access, Traffic, Logs, Security, Health, Configuration, and Settings administration.
- Add complete staged configuration transactions and last-known-good activation behavior.
- Add approved Docker discovery and proposed-publication workflows.
- Expand route matching, middleware/policy chains, rate limiting, redirects/header transforms, compression, and load balancing.
- Publish a documented first-party API and CLI.
- Consider HTTP/3 only after separate dependency, security, performance, and runtime acceptance.

## Platform integration and release

- Complete evidence-backed target-environment integration with GoreeCloud Manager, Privacy Shield, Wardveil Security, Everkeep, GoreeCloud UI, GoreeCloud Mesh, GoreeCloud Identity, GoreeCloud Policy, and GoreeCloud Observability.
- Complete release provenance, production signing/distribution where applicable, deployment approval, Production Acceptance, and Stable qualification.
- Retire Caddy only after Gateway is the authorized and verified public gateway and rollback remains proven.

No planned item is considered implemented or production-accepted merely because it is listed here.

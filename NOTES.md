# GoreeCloud Gateway — Development Notes

## Current stabilization context

- Lifecycle remains Development / nonconformant. Caddy remains the production-authoritative public HTTP/HTTPS gateway; no listener ownership, DNS, firewall, certificate, Docker, or production publication authority has moved to GoreeCloud Gateway.
- PR #9 is integrated into `main` as Development source. The integrated source contains the native routing/control-plane foundation, trusted-proxy and forwarding-identity enforcement, bounded recovery/rollback primitives, certificate/ACME development boundaries, and the read-only target-Caddy preflight collector.
- The integrated Development source is declared against versioned Platform Contract 2.1 and all nine Integral Platform Systems. GoreeCloud UI (GC UI) is the sole active design-system authority; Foundations 1.0.1 has limited scope, the broader workspace is experimental/alpha, and no Gateway administration UI has been accepted.
- Repository CI, isolated runtime tests, load tests, vulnerability reachability, and source recovery tests are Development evidence only. They do not establish target-VPS parity, production deployment, Release Candidate, Production Acceptance, or Stable status.

## Active Caddy-replacement gates

- Execute and review the sanitized read-only Caddy/VPS preflight against the authorized target before proposing any mutation.
- Establish route, listener, TLS/certificate, Docker-network, WebSocket/streaming, and applicable HTTP/2/gRPC parity for the exact Gateway candidate.
- Provision production ACME/DNS authority through approved protected credential handling only after the applicable warning/approval process.
- Complete an alternate-port target-VPS rehearsal that cannot conflict with Caddy's current TCP 80/443 authority.
- Prove rollback/recovery on the target and retain Caddy as rollback authority until post-cutover acceptance is complete.
- Obtain accepted Manager, Privacy Shield, Wardveil Security, Everkeep, GoreeCloud UI, Mesh, Identity, Policy, and Observability integration evidence where applicable.
- Reconcile the Caddy/NetBird port-separation governance before any listener-transfer proposal.
- Complete release provenance, deployment approval, Production Acceptance, and Stable qualification before representing Gateway as the active public gateway.

## Safety and evidence boundary

The read-only preflight collector must remain non-mutating and privacy-minimized. Do not store reusable DNS/API credentials, ACME private keys, certificate private keys, account secrets, or other reusable credentials in repository notes or ordinary migration evidence. Planned or source-implemented behavior is not target-runtime evidence.

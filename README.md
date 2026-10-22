# GoreeCloud Gateway

GoreeCloud Gateway is the planned first-party GoreeCloud reverse proxy, HTTPS gateway, ingress controller, certificate manager, routing/load-balancing system, and controlled web-service publication platform.

**Lifecycle:** Active Development

## Current accepted-main boundary

The accepted `main` branch now contains the integrated Development Gateway runtime/control-plane foundation together with repository governance, licensing, and branding. It does **not** own production HTTP/HTTPS listener authority.

Caddy remains production-authoritative for GoreeCloud web publication until Gateway completes migration-source parity, production-representative runtime, recovery/rollback, platform-integration, listener-transfer, and explicit production-acceptance gates.

PR #9 integrated the validated native foundation, Infrastructure Status/publication-preflight work, recovery/migration evidence, and certificate/ACME Development source into `main`. That source integration remains Development evidence only and must not be represented as target-VPS parity, deployment, production acceptance, or Stable behavior.

## Product role

Gateway is intended to provide:

- HTTP/HTTPS reverse proxying and TLS termination;
- automatic certificate lifecycle management;
- host, path, header, method, and approved protocol-aware routing;
- backend service definitions, health checks, load balancing, and failover;
- WebSocket and streaming proxying;
- staged configuration validation, preview, activation, history, and rollback;
- safe Docker-oriented service discovery and publication proposals;
- explicit Internal, Private, Restricted Public, and Public exposure classifications;
- privacy-minimized operational metrics and events;
- a planned GoreeCloud UI (GC UI) administrative application, API, and CLI (the UI is not yet accepted).

Gateway does not replace GoreeCloud DNS, Network, Identity, firewall policy, or application authentication. Those systems remain authoritative for their own domains.

## Production migration principle

Discovery or configuration must never publish a backend automatically. Production cutover from Caddy must be reversible and evidence-bound. A source build that can proxy traffic is not sufficient for production authority.

Required migration evidence includes configuration/route parity, TLS and renewal behavior, upgraded/streaming connections, production-representative load/backpressure, backup/restore, rollback, listener ownership, observability, required platform-system acceptance, and explicit cutover approval.

## GoreeCloud platform requirements

Stable qualification requires substantive, current accepted evaluation and integration with all nine Integral Platform Systems:

- GoreeCloud Manager for bounded administration, lifecycle, operational visibility, and approved control-plane workflows;
- Privacy Shield for minimal logging, redaction, retention, sensitive-header protection, client-information minimization, and privacy-safe metrics;
- Wardveil Security for exposure, listener, route, TLS, certificate, backend, and configuration-integrity security state;
- Everkeep for configuration snapshots, export/import, known-good retention, backup/restore, rollback, and disaster-recovery evidence;
- GoreeCloud UI for the administrative application and adaptive/accessibility contract, with integration acceptance still required;
- GoreeCloud Mesh for governed service coordination where applicable;
- GoreeCloud Identity for approved administrative identity/authentication without making Gateway the platform identity provider;
- GoreeCloud Policy for shared policy decisions and enforcement coordination while preserving Gateway-owned routing/publication rule authority; and
- GoreeCloud Observability for health, metrics, diagnostics, performance, dependency state, freshness, provenance, and operational evidence.

GoreeCloud governance separately controls publication and production-cutover authorization. Decorative identities, labels, or metadata do not satisfy these integration gates.

## Canonical identity

Branding authority is `GoreeCloud/goreecloud-branding-assets`. The canonical Gateway product artwork is `products/gateway/app-icon.svg`. Local artwork is a synchronized consumer derivative only and does not establish implementation or network authority.

See [BRANDING.md](BRANDING.md).

## Repository governance

This repository maintains the required root records:

- `README.md`
- `SPECIFICATIONS.md`
- `FEATURES.md`
- `BENEFITS.md`
- `COMPETITIVE-OBJECTIVES.md`
- `BRANDING.md`
- `IMPLEMENTED-FEATURES.md`
- `PLANNED-FEATURES.md`
- `CHANGELOGS.md`

GitHub is authoritative for source, pull requests, exact revisions, workflow evidence, and repository history. `IMPLEMENTED-FEATURES.md` and `PLANNED-FEATURES.md` are the Git-native feature-state authorities, and `CHANGELOGS.md` is the repository-local chronological change record. The retired `FEATURE-ROADMAP.md` / Drive roadmap synchronization model must not be recreated. GoreeCloud Tasks Management remains authoritative for durable cross-repository operational obligations.

## License

Unless otherwise noted, GoreeCloud-owned repository source is licensed under the GNU Affero General Public License version 3. Third-party dependencies and protocol/cryptographic libraries retain their own applicable licenses and notices.

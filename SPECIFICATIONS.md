# GoreeCloud Gateway Specifications

## 1. Authority and lifecycle

GoreeCloud Gateway is in **Active Development**. The authoritative project specification is `GoreeCloud/Projects/Project Specification — Gateway`; this repository record is the code-adjacent governance and implementation companion.

Accepted `main` now contains product identity, AGPL-3.0 license material, repository governance, and the integrated PR #9 native Gateway Development runtime/control-plane foundation. Caddy remains production-authoritative; source integration does not transfer production listeners or publication authority.

## 2. Purpose

Gateway is the GoreeCloud-owned reverse proxy, HTTPS gateway, ingress controller, certificate manager, routing/load-balancing platform, and controlled web-service publication system intended eventually to replace Caddy's application-level production gateway role.

Gateway must remain original GoreeCloud software. Narrow protocol, cryptographic, standards, and other foundational dependencies may be used when rebuilding them would create greater risk, but a complete third-party reverse-proxy product must not become the application foundation.

## 3. Product boundaries

Gateway owns approved HTTP/HTTPS publication and related configuration/routing/certificate state. It does not become the authoritative:

- DNS server;
- VPN/overlay network;
- firewall;
- identity provider;
- application authentication system;
- general-purpose network-management platform.

GoreeCloud DNS, Network, Identity, Wardveil Security, Privacy Shield, Mesh, Everkeep, and application-specific systems remain authoritative for their respective domains.

## 4. Control-plane and data-plane separation

Gateway is both an infrastructure application and an infrastructure service.

The control plane manages canonical configuration, services/routes/backends, certificate orchestration, validation, discovery proposals, history, rollback, audit state, status, and administrative workflows.

The data plane owns approved HTTP/HTTPS listeners, TLS termination, routing, reverse proxying, backend selection, connection management, streaming/upgraded connections, policy enforcement, and runtime operational evidence.

A temporary control-plane failure should not unnecessarily terminate traffic already using the last known-good data-plane configuration unless a security condition requires fail-closed behavior.

## 5. Configuration objects and lifecycle

Gateway uses first-party **Service**, **Route**, and **Backend** objects. A backend may exist without publication; a service may exist without a route; a route may remain draft/disabled.

Candidate configuration follows:

`Draft → Validate → Preview → Activate → Observe → Retain or Roll Back`

Validation must reject unsafe or ambiguous state, including duplicate/conflicting routes, invalid backends, listener conflicts, invalid TLS/access policy, missing objects, unsafe public exposure, and other configuration that cannot safely become active.

An invalid candidate must never replace the known-good configuration.

## 6. Publication classification

Every publication requires an explicit classification:

- Internal
- Private
- Restricted Public
- Public

Private operation is the default. Discovery alone must not create an active publication. Route activation requires approved hostname/destination/TLS/access/listener/backend/conflict validation.

## 7. Proxy and TLS objectives

The product direction includes HTTP/1.1 and HTTP/2 reverse proxying, WebSocket/streaming support, timeouts, health checks, connection/backpressure controls, load balancing, redirects/header transforms, graceful configuration reloads, and optional later HTTP/3 only after separate acceptance.

Automatic HTTPS must include safe ACME lifecycle management, DNS-01, permitted HTTP-01/TLS-ALPN-01 paths, wildcard workflows, certificate inventory/expiry/renewal/failure state, protected certificate storage, and safe failure behavior. The Development candidate now includes a provider-neutral DNS-01 boundary; a bounded Porkbun TXT-record adapter that can create only challenge records inside its configured zone and remove only the exact record ID it created; and an RFC 8555 order-based renewal issuer that requires an already registered ACME account, accepts only DNS-01 pending authorizations, waits for exact TXT propagation, cleans up the exact challenge record, generates a fresh certificate key and CSR, finalizes the order, and hands the returned material to the independent renewal validation/staging boundary. Separate Development source also provides explicit two-phase ACME account registration, encrypted account-key storage, transactional account-key rollover preparation/execution, read-only authority probing, and explicit recovery controls. These source/test capabilities do not establish live CA/Porkbun issuance, protected production key provisioning, target-VPS certificate parity, production renewal acceptance, or cutover authority. A certificate failure must not silently downgrade a service to unencrypted publication.

## 8. Discovery

Initial discovery focuses on Docker. Discovery may identify eligible workloads, networks, candidate ports, approved publication metadata, and conflicts, but it produces proposed state. Activation requires approval unless a separately governed automation policy explicitly authorizes a narrow workflow.

## 9. Caddy migration boundary

Caddy remains production-authoritative until Gateway passes the complete migration and production-acceptance process.

Required evidence includes, as applicable:

- independently reviewed migration-source identity;
- exact configuration/route/backend/TLS intent parity;
- certificate issuance and renewal rehearsal;
- WebSocket/streaming behavior;
- production-representative sustained load, latency, error rate, and backpressure;
- backup/restore and known-good recovery;
- rollback and reversible cutover;
- listener ownership and conflict validation;
- monitoring/observability;
- private/public access parity;
- target-environment platform integrations;
- exact artifact/deployment identity;
- explicit production-cutover authorization.

Source validation and isolated runtime tests may grant migration-rehearsal eligibility only. They do not authorize production cutover.

## 10. GoreeCloud platform requirements

Stable qualification requires substantive accepted evaluation and integration with all nine Integral Platform Systems:

- **GoreeCloud Manager** — bounded administration, lifecycle, operational visibility, approvals, remediation, and management-plane presentation.
- **Privacy Shield** — minimal logging, redaction, sensitive-header protection, retention controls, client-information minimization, privacy-safe metrics, and truthful privacy evidence.
- **Wardveil Security** — exposure risk, route/listener conflicts, TLS/certificate/backend findings, security headers, configuration integrity, and evidence-backed protection state.
- **Everkeep** — configuration snapshots, route/configuration history, export/import, backup/restore, known-good retention, rollback, disaster recovery, and migration portability.
- **GoreeCloud UI (GC UI)** — administrative presentation, interaction, accessibility, responsive/adaptive behavior, and state semantics; Gateway UI integration is not yet accepted.
- **GoreeCloud Mesh** — governed cross-service coordination where required.
- **GoreeCloud Identity** — approved administrative identity/authentication; Gateway must not become a competing identity authority.
- **GoreeCloud Policy** — policy representation, decisions, version/freshness, enforcement coordination, explanation, and evidence while Gateway retains authority for Gateway-owned routing/publication rules.
- **GoreeCloud Observability** — operational health, metrics, logs/events, diagnostics, performance, availability, dependency health, freshness, provenance, and operational evidence.

GoreeCloud governance separately controls publication and production-cutover approval boundaries. No platform name, badge, metadata field, or documentation-only reference counts as implemented integration evidence by itself.

## 11. Security and privacy requirements

Gateway is private by default. Administrative write operations must be authenticated, authorized, auditable, bounded, and designed to prevent accidental broad exposure.

Routine views and logs must not expose reusable credentials, cookies, authentication material, secrets, private request bodies, or sensitive headers. Operational events should retain only the minimum information required to diagnose route, certificate, backend, configuration, and traffic failures.

## 12. Observability and recovery

Gateway must provide privacy-safe evidence for request/response distributions, backend/route health, connection state, certificate events, activation/validation failures, proxy errors, latency where justified, security findings, and recovery state.

The product must preserve sufficient configuration history and known-good state to recover from route, backend, TLS, redirect/header, WebSocket, health-check, exposure, or administrative-lockout regressions.

## 13. Current source-evidence boundary

The PR #9 native implementation foundation is integrated into accepted `main` as Development source and includes source, isolated-runtime, sustained-load, migration-evidence, configuration-parity, recovery, certificate/ACME, and Infrastructure Status/publication-preflight contracts. Future increments remain separately governed through their own pull requests and acceptance gates.

Repository documentation must always distinguish accepted main, validated development candidate, migration-rehearsal evidence, production migration acceptance, and final Stable qualification.

## 14. Production qualification

Gateway must remain Development and Caddy must remain production-authoritative until all applicable routing, TLS, certificate-renewal, publication, upgraded/streaming connection, backend health, configuration rollback, logging/observability, Wardveil, Privacy Shield, Everkeep, Mesh, Identity, GoreeCloud UI, backup/restore, migration rollback, listener ownership, and explicit production approval gates pass together.

# GoreeCloud Gateway Competitive Objectives

## Objective

GoreeCloud Gateway should combine the strongest useful qualities associated with modern reverse proxies, ingress controllers, and visual proxy managers while remaining original GoreeCloud software with a safer publication lifecycle and stronger GoreeCloud-native governance.

Caddy, Traefik, Nginx, and Nginx Proxy Manager are capability references only. Gateway must not inherit a complete third-party product as its codebase or present compatibility with another product as first-party implementation.

## Capability objectives

### Simple automatic HTTPS

Gateway should make correctly secured HTTPS publication straightforward while preserving explicit certificate inventory, renewal state, failure evidence, challenge/provider abstraction, and rollback-safe behavior.

### Powerful routing and discovery

The product should support expressive routing, health-aware backend selection, upgraded/streaming connections, dynamic discovery, and future scalable backends without turning service discovery into automatic exposure.

### Approachable administration

A GoreeCloud UI (GC UI) administrative application should make services, routes, backends, certificates, discovery proposals, traffic, security, health, logs, configuration history, and rollback understandable without requiring a configuration file to be the primary user interface. This is planned, not yet accepted.

### Automation without hidden authority

The API/CLI should support safe automated management while maintaining authentication, authorization, auditability, staged activation, bounded permissions, and explicit publication policy.

## GoreeCloud differentiation

- private-by-default publication classifications;
- source/artifact/evidence-bound migration readiness;
- known-good configuration retention and explicit rollback;
- Privacy Shield-minimized operational logging;
- Wardveil Security-backed exposure/TLS/configuration security evidence;
- Everkeep-backed configuration portability and recovery;
- Mesh/Identity/governance integration rather than isolated proxy administration;
- controlled Caddy migration with independent parity and listener-ownership evidence;
- first-party design and lifecycle semantics optimized for GoreeCloud rather than generic multi-tenant hosting.

## Quality objectives

- deterministic configuration validation;
- no invalid-candidate activation;
- graceful reloads and bounded connection behavior;
- correct WebSocket/streaming semantics;
- safe certificate failure behavior with no silent cleartext downgrade;
- meaningful backpressure and production-representative load testing before cutover;
- clear distinction between candidate, isolated accepted, migration-rehearsal eligible, production-approved, and Stable states;
- privacy-safe evidence sufficient to diagnose failures without routine collection of sensitive request data.

## Migration objective

Gateway wins only if it can replace Caddy without reducing safety, availability, certificate reliability, privacy, observability, or recoverability. Production cutover must therefore be treated as a migration program with independently reviewed source configuration, parity, rehearsal, rollback, and explicit approval—not as a feature toggle.

## Current evidence boundary

Accepted `main` is still the governance/branding/license baseline. Native executable work remains in draft development pull requests and Caddy remains the active production gateway. Competitive objectives do not authorize production listener transfer or Stable qualification.

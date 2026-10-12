# GoreeCloud Gateway Architecture

## Request path

`listener -> route matcher -> request policy -> upstream selector -> reverse proxy -> response policy -> client`

TLS termination and certificate selection occur at the listener boundary. Observability surrounds the request path without becoming authoritative for routing decisions.

The canonical route model now carries an explicit TLS policy into the Go runtime instead of discarding the schema-defined `tls` object. Enabled routes must declare either `required` or `disabled`. A `required` route must identify a certificate profile, while a `disabled` route cannot carry an unused certificate profile.

Request routing also fails closed for transport mismatch: a route with `tls.mode=required` is not eligible for a plaintext request. This prevents a route from being served over HTTP merely because certificate/listener lifecycle work is still being developed. Certificate-profile resolution and full listener lifecycle remain separate unfinished Milestone 1 work.

## Core capability families

### Gateway Routes
Deterministic host, path, header, method, scheme, port, and protocol matching; explicit priority; redirects; rewrites; reusable route templates; conflict detection.

### Gateway Proxy
HTTP/1.1, HTTP/2, WebSocket, streaming, request cancellation, forwarding-header policy, timeout policy, body limits, backend transport configuration, and safe error handling.

### Gateway Upstreams
Named pools, weighted and least-connection strategies, active/passive health, retry budgets, circuit-state inputs, draining, maintenance state, failover, and connection lifecycle controls.

### Gateway TLS
SNI certificate selection, TLS policy, ACME account/certificate lifecycle, DNS-01 provider adapters, renewal, revocation, OCSP/status handling where applicable, certificate inventory, expiry alerts, and controlled manual-certificate support.

The current source slice implements typed route TLS policy validation and request-time enforcement of the `required` transport boundary. It does not yet represent complete certificate-profile resolution, automatic certificate acquisition, renewal, SNI selection, or production listener management.

### Gateway Policy
Composable request/response policy chains for headers, compression, redirects, authentication handoff, IP/network constraints, rate controls, request-size controls, CORS where explicitly configured, and security headers.

### Gateway Publication
Private/public classification, service-to-route relationships, intended exposure, Conduit dependency state, Beacon hostname state, certificate state, backend health, and publication validation.

### Gateway Console and API
Glaze UI administration and a versioned local-first API for routes, upstreams, certificates, policies, diagnostics, validation, export/import, and runtime status.

### Gateway Migration
Parsers and import plans for approved Caddyfile, Traefik dynamic/static configuration, and Nginx Proxy Manager exports. Migration must generate a reviewable GoreeCloud Gateway plan; it must never silently cut over production.

## Security and privacy

- Administration binds privately by default.
- Public listeners expose only explicitly published routes.
- Configuration validation fails closed on ambiguous or unsafe route state.
- TLS-required routes are not matched for plaintext requests.
- Secrets are referenced through protected runtime configuration and are never committed to source.
- Access logging is minimized and configurable under Privacy Shield contracts.
- Wardveil Security receives evidence-backed gateway state and events rather than unsupported protection claims.
- Sensitive headers and credentials are redacted from diagnostics.

## Resilience

Everkeep integration covers configuration export, versioned snapshots, certificate metadata required for recovery, restore validation, migration packages, rollback state, and documented recovery boundaries. Private keys and reusable credentials remain protected sensitive material and are not embedded in ordinary exports.

## GoreeCloud Coordinated Networking Platform

**Owner designation (2026-10-12):** GoreeCloud Gateway, GoreeCloud VPN, and GoreeCloud DNS together form the **GoreeCloud Coordinated Networking Platform** for GoreeCloud.

This is a coordinated family of independently maintained and independently authoritative products:

- **GoreeCloud Gateway** owns application-level HTTP/HTTPS ingress, reverse proxying, TLS termination, routing, and approved service publication.
- **GoreeCloud VPN** owns encrypted private connectivity, approved remote access, device-to-device/network reachability, and VPN access policy.
- **GoreeCloud DNS** owns DNS resolution, filtering, DNS policies, and DNS-based name and service discovery.

The products should cooperate through documented interfaces and explicit policy and ownership boundaries, including private-service discovery, intended exposure classification, access context, hostname publication, and operational signals. Coordination must not silently make a private service public, override access policy, transfer authority between products, or require one product to take over another's responsibility. GoreeCloud Network/Conduit and other platform-system responsibilities remain distinct unless their authoritative specifications establish otherwise.

**Lifecycle integrity:** This designation is an owner-defined product-architecture relationship, not evidence that these cross-product integrations are fully implemented, deployed, production-qualified, or Stable. All current Gateway migration and Caddy production-authority restrictions still apply.

## Cross-product integration

- **Beacon / GoreeCloud DNS:** hostname and DNS publication state; DNS-01 adapter contracts.
- **Conduit / GoreeCloud Network:** private reachability and network-policy context; Gateway does not replace Conduit access enforcement.
- **Monitor:** service and endpoint health signals.
- **Notify:** certificate, route, health, and operational notifications.
- **Manager:** minimized administrative status and deep links.

## Production migration rule

The current Caddy deployment remains production authority until Gateway has feature-specific implementation evidence, automated tests, isolated runtime acceptance, migration and rollback validation, certificate recovery validation, security/privacy review, and explicit production approval.

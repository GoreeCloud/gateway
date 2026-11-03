# GoreeCloud Gateway — Optional Identity OIDC API authorization

**Status: Development-only, explicitly opt-in. NOT deployed and not a completed browser SSO integration.**

## Verified base contract

The Gateway read-only management sidecar (Go package `cmd/console`) accepts its original 32+-character static bearer token for `GET /api/v1/overview`. The admin API stays on `127.0.0.1:2019`; the console remains restricted to a numeric loopback listener by `validateListen`. Existing browser and Android clients can continue to use the original static-token API without modification.

The optional `cmd/console/oidcauth.go` adapter reuses the repository's pinned `github.com/coreos/go-oidc/v3` signature-verification library, already a transitive Go dependency, rather than reimplementing RSA/JWT cryptography. Authentication is disabled by default.

## Development-only activation contract

To enable an *additional* OAuth bearer-token authentication path, an authorized operator must set **all** of these environment variables at process startup:

| Environment variable | Required value |
| --- | --- |
| `GOREE_GATEWAY_OIDC_ENABLED` | Exactly `1`; absent = disabled. Any other value fails startup |
| `GOREE_GATEWAY_OIDC_ISSUER` | Trusted exact HTTPS Keycloak realm, e.g. `https://identity.example.test/realms/goreecloud` |
| `GOREE_GATEWAY_OIDC_AUDIENCE` | OAuth resource audience assigned specifically to Gateway console |
| `GOREE_GATEWAY_OIDC_CLIENT_ID` | Exact allowed `azp` OAuth client identifier |
| `GOREE_GATEWAY_OIDC_TENANT` | Exact trusted tenant identifier expected in a signed custom `goreecloud_tenant` claim |

An OIDC access token must carry a verified **RS256 signature**, matching issuer and audience, `typ=Bearer`, matching authorized OAuth client (`azp`), nonempty subject, a short lifetime (at most one hour), required scope `gateway:read`, the role `gateway-reader` in `realm_access.roles`, and the expected `goreecloud_tenant` claim. A signed token from the wrong audience/client/tenant, a missing scope/role, an ID token, a token with an unsupported algorithm, or an expired token is rejected. The custom tenant claim and resource audience require explicit Keycloak mapper and client configuration; they are **not** present in the current default realm fixture.

The API does not accept identity headers supplied by browsers or upstream proxies as proof of user identity. It rejects duplicate Authorization headers. Static token authentication still works when OIDC is explicitly enabled; this enables controlled rollback. Both paths remain limited to the existing read-only overview endpoint.

## Signing-key trust and operational boundaries

JWKS is retrieved **only** from the pinned issuer's `/protocol/openid-connect/certs` HTTPS URL, without following redirects or inheriting process proxy settings; the HTTP client verifies server certificates, caps response size at 256 KiB and times out after five seconds. A separate network fetch cooldown reduces repeated unknown-key retrieval attempts. Signature, issuer, audience, expiration and required API authorization checks are enforced through the existing vetted Go verification library plus locally verified claim restrictions.

This does **not** provide real-time revocation of already-issued bearer tokens, OAuth client registration, session cookies, OIDC authorization-code/PKCE login pages, mobile browser sign-in, logout, approval workflows or audit dashboards. Token revocation and role changes may not be recognized until expiration unless a future introspection and event-driven mechanism is integrated. Live issuer rotation, multi-node caching, provider outages and actual JWT entitlements require additional acceptance checks.

## Deployment and acceptance restrictions

**Do not enable on a VPS or expose the management port to the internet** under this development milestone. A production cutover requires separate approval, authenticated TLS ingress, a trusted proxy/identity boundary, strict end-to-end authorization, owner-approved permissions, an actual registered OIDC client, audited role and tenant mappers, operational recovery and rollback, and Android/web client acceptance.

The existing web UI is **not** an OIDC login portal; it still prompts for the original static token. GoreeCloud UI visual acceptance remains independent and unverified.

Tests include synthetic RSA signed tokens from a test-only HTTPS signing-key issuer, invalid claim and signature cases, OIDC disabled-by-default, static-token backward compatibility, duplicate-header rejection, strict key endpoint, network refresh throttling, and an oversized signing-key response. These tests do not establish a live Keycloak-to-Gateway integration.

**Related:** [GoreeCloud Gateway issue #14](https://github.com/GoreeCloud/gateway/issues/14) and [GoreeCloud Identity issue #2](https://github.com/GoreeCloud/identity/issues/2).

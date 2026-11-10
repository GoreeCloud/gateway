# GoreeCloud Gateway — Web Console, Management API, and Android App

| Property | Value |
| --- | --- |
| Owner | GoreeCloud founder |
| Status | Development source, not production accepted |
| Updated | 2026-11-03 |
| Canonical location | `GoreeCloud/gateway/docs/CONSOLE.md` |
| Upstream integration | Caddy admin API, loopback-only read access |

## Product direction

I am building original GoreeCloud Gateway clients for browsers and Android, backed by my self-hosted, Caddy-compatible infrastructure. The Caddy engine remains upstream software; the management clients, read-only API and user experience are GoreeCloud-owned source. Both interfaces follow the [GC UI design language](https://github.com/GoreeCloud/gc-ui/blob/main/docs/DESIGN_LANGUAGE.md).

The initial milestone is deliberately read-only. I can see the current engine connection, HTTP server count, top-level route count and host matchers. I cannot edit live routes, restart the gateway, change access policies or deploy infrastructure from these apps yet.

## Architecture

- **Gateway engine:** `cmd/gateway`, existing Caddy-compatible Go entrypoint; current CLI and TLS/proxy behavior remain upstream.
- **Management sidecar:** `cmd/console`, a separate Go process exposing a limited `/api/v1/overview` endpoint. It reads the local Caddy API at `127.0.0.1:2019` and constructs a minimized response. It does not return Caddy's raw configuration, upstream credentials, certificate material or private keys.
- **Web application:** static assets embedded in `cmd/console/web`. It uses same-origin API calls, keeps the bearer token in JavaScript memory, supports session lock, responsive sections, offline states, user guidance and a light/dark appearance preference.
- **Android application:** actual Kotlin/Jetpack Compose Android source under `android/`. It uses a typed HTTPS client to query the same read-only API. Its access token remains only in process memory, with no persistence, backups or WebView; the native app rejects cleartext HTTP.
- **Other services:** Caddy, DNS, NetBird, TLS termination and any Zero Trust access proxy are independent integrations. The Android application does not imply that a VPN tunnel, DNS record or a public console endpoint already exists.

## Optional GoreeCloud Identity bearer-token verification (development only)

The Go management sidecar now has an **opt-in, disabled-by-default** OIDC JWT-verification adapter for the existing read-only overview API. It requires an exact trusted HTTPS Keycloak realm, OAuth resource audience, client ID, custom tenant claim, `gateway:read` scope and `gateway-reader` role; the original static console token remains accepted without change. See the [Identity OIDC integration contract](IDENTITY_OIDC.md) for required settings, trust boundaries and deployment restrictions. **The browser and Android clients do not yet implement an OIDC interactive login flow; this is not production SSO.**

## Development build

Requirements: Go version from root `go.mod`, plus Android SDK 36, JDK 17 and Gradle 8.13 for Android.

```sh
go test ./cmd/console
go build -o gateway-console ./cmd/console
```

To run locally, provision a unique random 32+-character access token in a protected host environment variable named `GOREE_GATEWAY_CONSOLE_TOKEN`, then launch `./gateway-console`. The process refuses to start without this variable. It listens on `127.0.0.1:7689` by default and refuses nonloopback addresses. The `-listen` argument also requires an **explicit canonical numeric TCP port from 1 to 65535**; ephemeral `:0`, service names, padded spellings and out-of-range ports are rejected before the management server is created. This protects local listener configuration only, not ingress-route or certificate readiness. A containerized engine has its own loopback network namespace. The management sidecar must run in that same trusted namespace (for example, a reviewed `network_mode: service:<gateway-engine>` arrangement) to reach `127.0.0.1:2019`. Do not publish the admin port to the host or Internet to work around this restriction. The Caddy admin API must remain on local loopback (unless a separately reviewed safe integration is implemented).

Open `http://127.0.0.1:7689` **only on the machine running the console**, enter the provisioned token, and observe the local engine. A browser on a remote device requires a separately secured HTTPS ingress endpoint or a trusted, authenticated private tunnel. Do not expose the management listener or Caddy's admin API directly on public interfaces.

For Android development:

```sh
cd android
gradle :app:testDebugUnitTest
gradle :app:assembleDebug
```

A dedicated Android CI emulator job runs native Compose instrumentation tests on API 35, checking locked clean launch and native address/token input behavior **without making network requests**. Separate JVM tests verify rejection of HTTP, credential-bearing and path-containing console URLs, and the Android client also validates HTTPS at the transport layer. The emulator tests do not yet prove a visible insecure-URL error message, a real private HTTPS connection, signed production APK, update lifecycle or a physical device. Open the project in Android Studio for additional device acceptance. Provide a trusted HTTPS URL for the management sidecar through an approved private ingress path. The Android application cannot connect to the VPS through its own device's `127.0.0.1`. CI may retain a 7-day development/debug APK workflow artifact with a SHA-256 manifest for controlled device testing. Debug signing is not production release signing. No APK is considered delivered until a separately validated standalone APK is placed directly in `GoreeCloud/Artifacts` in the approved Google Drive account and its stored bytes are verified.

## API contract (v1)

`GET /api/v1/overview` requires `Authorization: Bearer <console-token>`. It returns `api_version`, `engine`, `state`, `hosts`, `checked_at`, and when a local engine is reachable, `servers` and `routes`. The state is `connected` or `unreachable`. Counts are omitted when unavailable. The optional `host_count` reports the **complete number** of distinct top-level HTTP host matchers, while `hosts` contains an alphabetically sorted, bounded preview of at most 100 hostnames. Offline responses omit counts rather than inventing zeros. Hosts represent matchers from **top-level HTTP routes only** and are not proof of DNS propagation, public availability or application health.

`GET /healthz` returns a minimal sidecar-process health result on the loopback listener without exposing operational details. No route-modifying endpoints are registered.

## Synthetic private HTTPS test (development)

The [isolated HTTPS verification script](PRIVATE_HTTPS_TEST.md) starts a separate loopback-only TLS proxy with its admin listener disabled, synthetic one-day OpenSSL certificates, and strict certificate-chain verification via an explicit temporary CA. It exercises the web application and protected read-only API without changing global trust stores or touching DNS/ACME. This is not evidence of a real remote HTTPS service, Android device-to-API connectivity, or production identity policy acceptance.

## Security restrictions and future acceptance

1. Keep the management sidecar loopback-only; remotely access it only through a reviewed HTTPS / VPN / Zero Trust boundary with trustworthy TLS and authorization. The console token is not a substitute for service authentication, privilege separation or external rate limiting.
2. Never copy a real bearer token, TLS certificate, Caddy configuration, account credential, network secret or private hostname inventory into this public repository, GitHub Actions logs, issues, or application source.
3. The sidecar throttles invalid tokens after eight failed attempts per direct TCP peer within one minute (429 with Retry-After). It ignores spoofable forwarded-client headers, keeps bounded in-memory counters, and never rejects a valid token due to an attacker's failed attempts. When the console sits behind a reverse proxy, many remote users may share its immediate peer address; this is **not** a substitute for trusted-edge per-client rate limits or a privileged identity service. Multiple Authorization headers are rejected.
4. The sidecar fetches the fixed local admin endpoint; response size is limited and data is minimized. The web app applies a restrictive Content Security Policy and `Cache-Control: no-store`. This is not proof of a complete security audit.
5. Browser and Android implementations require actual responsive/visual acceptance, Android installation and signing evidence, usability verification, integration and recovery tests before Stable.
6. The Caddy replacement still requires inventory of live routes and certificates, backup/restore verification, staged parity tests, a rollback rehearsal, explicit production acceptance, and verified VPS connectivity. Existing Caddy must not be removed based solely on these apps passing CI.

**Not implemented yet:** authenticated multiuser roles, write approvals, audit history, route editing, config rollback UI, live request metrics, alerts, rate limiting, MFA, push notifications, Android background monitoring or production deployment. These are improvement candidates, not claims of existing features.

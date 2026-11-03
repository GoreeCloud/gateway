# GoreeCloud Gateway

**Status: early development / not production-ready.**

GoreeCloud Gateway is a GoreeCloud-branded, privacy-first ingress and reverse-proxy distribution **powered by the upstream Caddy engine**. It is a separate application entrypoint with opt-in GoreeCloud HTTP middleware; it is **not** a source-code fork and it does **not** replace or rename Caddy upstream.

## What works in source

- Caddy's built-in reverse proxy, automatic HTTPS, HTTP/1.1, HTTP/2, HTTP/3, Caddyfile, JSON configuration, and standard modules (provided by the pinned upstream dependency).
- `goree_request_id`: cryptographically generated per-request correlation IDs; overwrite untrusted client IDs and propagate to upstream handlers.
- `goree_security_headers`: opt-in secure defaults for content sniffing, referrer handling and framing.
- `goree_ip_allowlist`: opt-in source-address CIDR restrictions for private administrative routes; deliberately ignores spoofable forwarded-IP headers.
- Automated Go build/test/vet workflows and an explicit non-destructive migration playbook.
- **Porkbun DNS-01 provider** compiled in as an opt-in upstream Caddy module, supporting existing Porkbun-based ACME configuration syntax without storing secrets in source.
- **GoreeCloud Gateway Web:** responsive local-first management interface with session unlock, overview, hostname discovery, security guidance, settings and light/dark appearance; currently read-only.
- **GoreeCloud Gateway for Android:** native Kotlin/Jetpack Compose app with an HTTPS-only connection flow, read-only status and hostname display, and in-memory-only access token.
- **Management API:** a separate loopback-only Go sidecar, guarded by a long random access token, which minimizes Caddy admin configuration into a versioned, read-only API.

**Not yet implemented or verified:** production-grade dashboard acceptance, end-to-end Android installation, multi-user identity, write administration, external SSO, WAF, rate limiting, production telemetry, high availability, production deployment, or full parity with existing VPS ingress. Source/CI results and release acceptance must be reported separately. A configuration directive alone is not proof a feature operates on a server.

## Build

Requirements: Go 1.26+ and network access for Go modules.

```sh
go mod tidy
go test ./...
go vet ./...
go build -trimpath -o gateway ./cmd/gateway
./gateway version
./gateway validate --config configs/Gatewayfile.example --adapter caddyfile
```

The binary intentionally preserves the Caddy CLI commands and its configuration semantics for migration compatibility. `goree_*` directives work only in this custom build.

## DNS challenge compatibility

The binary registers `dns.providers.porkbun` using the pinned MIT-licensed upstream provider. See [Porkbun integration and migration requirements](docs/PORKBUN.md). Module registration and offline Caddyfile adaptation are not live certificate issuance or renewal verification.

## Web and Android management apps

The browser UI and a **read-only**, token-protected management API are in `cmd/console/`; the native Android application is in `android/`. These are independently built frontends of the same self-hosted engine, not public access to the privileged Caddy admin API.

```sh
go build -o gateway-console ./cmd/console
# Configure GOREE_GATEWAY_CONSOLE_TOKEN securely in the host environment before starting.
./gateway-console
```

The console listens only on `127.0.0.1:7689`; Android requires a reviewed private HTTPS gateway to reach it. **Do not publish the Caddy admin endpoint or console listener.** See [web and Android documentation](docs/CONSOLE.md) for authentication, build, and release details. No Android APK is yet delivered.

## Example

See [configs/Gatewayfile.example](configs/Gatewayfile.example). Never use example addresses, upstream ports, or access lists as real production configuration. Caddy's admin API must remain loopback-only unless secured behind separate authentication and network restrictions.

## Production safety

Do not change the running `caddy` service, port 80/443 ownership, systemd units, firewall, DNS, or certificate data until the [cutover checklist](docs/CUTOVER.md) is satisfied, the existing ingress owner's identity is verified, and a tested rollback is ready. Any actual production deployment requires its own acceptance and authorization.

## Code and attribution

GoreeCloud-specific source is licensed under Apache-2.0 (see [LICENSE](LICENSE)). Caddy is independently maintained by the Caddy project under Apache-2.0, with its own attribution and trademark. See [NOTICE.md](NOTICE.md). This repository imports Caddy as a versioned Go dependency rather than copying and rebadging its whole upstream codebase.

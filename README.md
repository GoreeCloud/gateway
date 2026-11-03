# GoreeCloud Gateway

**Status: early development / not production-ready.**

GoreeCloud Gateway is a GoreeCloud-branded, privacy-first ingress and reverse-proxy distribution **powered by the upstream Caddy engine**. It is a separate application entrypoint with opt-in GoreeCloud HTTP middleware; it is **not** a source-code fork and it does **not** replace or rename Caddy upstream.

## What works in source

- Caddy's built-in reverse proxy, automatic HTTPS, HTTP/1.1, HTTP/2, HTTP/3, Caddyfile, JSON configuration, and standard modules (provided by the pinned upstream dependency).
- `goree_request_id`: cryptographically generated per-request correlation IDs; overwrite untrusted client IDs and propagate to upstream handlers.
- `goree_security_headers`: opt-in secure defaults for content sniffing, referrer handling and framing.
- `goree_ip_allowlist`: opt-in source-address CIDR restrictions for private administrative routes; deliberately ignores spoofable forwarded-IP headers.
- Automated Go build/test/vet workflows and an explicit non-destructive migration playbook.

**Not yet implemented or verified:** browser management dashboard, visual GC UI compliance, external SSO, WAF, rate limiting, production telemetry, high availability, CI acceptance, VPS deployment, or production parity. A configuration directive alone is not proof a feature operates on a server.

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

## Example

See [configs/Gatewayfile.example](configs/Gatewayfile.example). Never use example addresses, upstream ports, or access lists as real production configuration. Caddy's admin API must remain loopback-only unless secured behind separate authentication and network restrictions.

## Production safety

Do not change the running `caddy` service, port 80/443 ownership, systemd units, firewall, DNS, or certificate data until the [cutover checklist](docs/CUTOVER.md) is satisfied, the existing ingress owner's identity is verified, and a tested rollback is ready. Any actual production deployment requires its own acceptance and authorization.

## Code and attribution

GoreeCloud-specific source is licensed under Apache-2.0 (see [LICENSE](LICENSE)). Caddy is independently maintained by the Caddy project under Apache-2.0, with its own attribution and trademark. See [NOTICE.md](NOTICE.md). This repository imports Caddy as a versioned Go dependency rather than copying and rebadging its whole upstream codebase.

# GoreeCloud Gateway

**A native, self-hosted ingress service and web console built on Caddy's real Go engine.** New GoreeCloud product, not a fork claiming independent authorship of Caddy.

GoreeCloud Gateway packages **Caddy v2.11.9** as an upstream Go dependency and adds a first-party React management app and a small, loopback-only Go control plane. Caddy remains responsible for HTTP routing, reverse-proxy behavior, TLS automation and protocol handling. The native binary also registers the optional upstream Porkbun DNS-01 module v0.3.1 for Caddyfile migration compatibility; the web editor does not manage DNS credentials or DNS-01 policies. The managed console is intentionally scoped to simple hostname-to-upstream routes; advanced Caddyfile configuration is supported by the native CLI, not rewritten by this console.

## Capabilities

- Native Go executable with upstream Caddy commands (`run`, `adapt`, `validate`, etc.) and a new `serve` mode.
- Managed reverse-proxy routes: add, edit, delete and list hostnames and HTTP(S) upstreams. An authenticated route-validation API previews a candidate without applying it or persisting it, even when write mode is disabled.
- Caddyfile-to-JSON adaptation with Caddy itself; live configuration load with refusal on error.
- Atomic JSON state persistence (0600 permissions), strict rejection of ambiguous or malformed saved site arrays on reopen, rollback when persistence fails, no direct writes to unrelated Caddyfiles. The managed engine uses separate, private Caddy autosave and certificate storage (0700).
- React browser app with local login, dashboard, route table, search, edit dialogs, dark/light palettes and responsive navigation.
- Browser control plane defaults to read-only and **only binds a loopback IP**; all private API endpoints require a bearer token.
- No tracking or cloud account requirement. No persistent browser token storage. Management requests reject DNS-rebound Host headers, rate-limit repeated direct-peer authentication failures, and use stricter browser security headers.
- Route mutations require exactly one application/json Content-Type (with optional UTF-8 charset); ambiguous duplicate JSON properties are rejected, including case-insensitive field-name collisions. JSON request bodies over 8 KiB return HTTP 413, including otherwise-valid JSON followed by excessive whitespace.
- Offline tests exercise representative legacy Caddyfile route semantics but do not import or certify the existing production Caddyfile.

## Build requirements

Go 1.26+ (current go.mod minimum) and Node.js 22+ with npm (esbuild bundler). For dependency-locked builds, use the committed npm lockfile and Go sums. Matching dependencies are not, by themselves, proof of byte-for-byte reproducibility.

```sh
cd web
npm ci
npm run build
cd ..
go mod download
go test ./...
go build -trimpath -o gateway .
```

**Tagged release builds:** the API defaults to `0.1.0-dev` for source builds. Release packagers must set the public version through the Go linker, for example `CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X github.com/GoreeCloud/gateway/internal/gateway.Version=0.1.0-alpha.6" -o gateway .`, and independently verify the resulting runtime status.

## Verify published development artifacts

The [GitHub releases page](https://github.com/GoreeCloud/gateway/releases) publishes **development pre-releases**, not production-approved binaries. [v0.1.0-alpha.6](https://github.com/GoreeCloud/gateway/releases/tag/v0.1.0-alpha.6) contains separate native Linux **AMD64** and cross-compiled **ARM64** executables, built from protected source commit `d78a27c299ba50387749ebb9e134bd97d1f40f3d`. GitHub-hosted archives were downloaded back and independently matched their SHA-256 checksums and source provenance. No release has undergone public TLS/DNS-01 or real ARM64-device acceptance.

Download both archives and `SHA256SUMS.txt` into a private directory before verifying them:

```sh
sha256sum -c SHA256SUMS.txt
tar -tzf goreecloud-gateway-v0.1.0-alpha.6-linux-amd64.tar.gz | head -25
tar -xzf goreecloud-gateway-v0.1.0-alpha.6-linux-amd64.tar.gz
cat BUILDINFO.txt
go version -m ./gateway
```

For ARM64, substitute `linux-arm64` in the archive filename; **ARM64 has not yet been run on physical ARM64 hardware**. Release archives include upstream license notices, source/build metadata, the React console bundled in the executable, synthetic staging examples, and redacted preflight scripts. Matching checksums alone do not replace a release signature, binary provenance review or independent security audit.

Earlier alpha.1–alpha.3 include affected `klauspost/compress v1.20.1` and carry release-page warnings. Alpha.5 embeds OpenTelemetry modules affected by [GHSA-q7gw-24fr-5p69](https://github.com/advisories/GHSA-q7gw-24fr-5p69); alpha.6 updates those modules to **1.47.0** through [PR #25](https://github.com/GoreeCloud/gateway/pull/25). The separate inherited `go-jose/v3` alerts remain tracked in [issue #27](https://github.com/GoreeCloud/gateway/issues/27). Do not deploy any development alpha to production.

### Linux ARM64 build verification

Protected CI cross-compiles an additional static **Linux ARM64** executable and checks its ELF architecture plus embedded upstream Caddy/Porkbun module metadata. To reproduce the cross-compile on a trusted machine with Go installed:

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -o gateway-linux-arm64 .
file gateway-linux-arm64
go version -m gateway-linux-arm64
```

The alpha.6 release includes a Linux ARM64 archive as well as AMD64. Cross-compilation and architecture/module checks do **not** establish real ARM64 runtime behavior, production network support or readiness.

A separate GitHub Actions `arm64-runtime` job runs the Go/Caddy tests and an authenticated, read-only management smoke natively on a hosted ARM64 Linux VM. The smoke also verifies the native `/api/validate-site` preview, rejection of unauthenticated or ambiguous requests, and no managed-route persistence. It is independent of the older cross-build and does not count as a test on a physical GoreeCloud ARM64 device or production network.

## Source-pinned local Linux release builds

Maintainers can use the [reviewed dual-platform packaging script](scripts/package_linux_release.sh)
and [release build guide](docs/RELEASE_BUILD.md) from a clean checkout to
build static Linux AMD64/ARM64 development archives, verify their source,
module metadata, attribution and checksums. This does **not** publish a release,
add signatures, deploy anything or replace the separate strict GitHub CI gates.

## Optional hardened Linux systemd service

An **opt-in example** [systemd unit](examples/systemd/goreecloud-gateway.service)
and [deployment safety guide](docs/LINUX_SERVICE.md) use a dedicated static
Linux account, private persistent state, root-owned environment file, loopback
management and restrictive service privileges. This template is **not
automatically installed**, tested against real production ingress, or approved
for cutover; persisted Caddy routes can still bind public listeners even while
the web console is read-only.

## Local development: management console

Choose a secure random token, preserve it in your password manager, and export it into your session without adding it to shell history:

```sh
read -rsp 'Gateway admin token: ' GATEWAY_ADMIN_TOKEN; echo
export GATEWAY_ADMIN_TOKEN
./gateway serve --listen 127.0.0.1:8787 --sites ./gateway-sites.json --allow-write
```

Open **http://127.0.0.1:8787** and enter the same token. `GATEWAY_ADMIN_TOKEN` must contain at least 24 characters. Without `--allow-write`, the console is read-only. Never use a sample token in a real environment.

**Important:** Adding a public domain may cause Caddy to bind its normal HTTP/HTTPS ports and request TLS certificates if DNS points to the host. For first-run testing without DNS or certificates, add `localhost` with an HTTP upstream on a machine where the needed local ports are free. Before operating on a server, plan listener ports, firewall rules and certificates, and confirm the current ingress service is not using them. This repository does not migrate existing ingress automatically.

## Native Caddy CLI: advanced configurations

The bundled binary passes all commands other than `serve` to the upstream Caddy CLI:

```sh
./gateway validate --config Caddyfile --adapter caddyfile
./gateway run --config Caddyfile --adapter caddyfile
```

The `serve` console **does not import, inspect or modify** an unrelated Caddyfile or another Caddy process. By default it keeps its Caddy autosave and certificate data in `.gateway-state/` alongside the selected sites file; use `--state-dir /absolute/private/path` for persistent service storage. The runtime refuses group/world-readable managed-state directories and paths traversing symlinked ancestors. Use a stable, operator-controlled path without directory aliases for `--state-dir`; path validation does not defend against an attacker who can concurrently replace writable parent directories. Use one managed `--sites` file per Gateway instance.

## API

All paths other than `GET /api/health` require `Authorization: Bearer <your-token>`.

| Endpoint | Purpose |
| --- | --- |
| `GET /api/health` | Minimal process liveness, no sensitive data |
| `GET /api/status` | Console mode, Caddy state and route count |
| `GET /api/sites` | List managed site mappings |
| `POST /api/validate-site` | Authenticated, non-mutating route validation and Caddy adaptation (works in read-only mode) |
| `POST /api/sites` | Add a route (when writes enabled) |
| `PUT /api/sites/{host}` | Replace a route (when writes enabled) |
| `DELETE /api/sites/{host}` | Remove a route (when writes enabled) |

Each site is a JSON object such as `{"host":"app.example.com","upstream":"http://127.0.0.1:3000"}`. Hostnames must be DNS-compatible. Upstream URLs must use HTTP or HTTPS and may not include credentials, a non-root path, query or fragment.

## Redacted legacy ingress preflight

The optional Python 3 script `scripts/ingress_preflight.py` is **read-only** and does nothing but show help unless explicitly given `--live`. On an authorized Docker host, `python3 scripts/ingress_preflight.py --live --container caddy` checks selected Caddy/Docker metadata and emits a redacted JSON gate summary. It never prints configuration bodies, token values, host addresses, mounted source paths, certificate data or raw Docker errors. Its local synthetic fixtures run in CI and do not contact a Docker daemon.

**Important:** The result always reports backups, live TLS health, actual route parity, rollback testing, and production-cutover readiness as unverified. It is **not** a backup tool, healthcheck, upgrade executor or substitute for independent recovery testing. See [staging acceptance](docs/STAGING_ACCEPTANCE.md).

For legacy Caddy patching, independent backup restoration and production recovery, follow the [risk-gated security upgrade plan](docs/LEGACY_CADDY_PATCH_GATES.md). This document is a checklist, **not** an upgrade executor.

## Keyboard and browser accessibility regression

The local-first browser console has a skip-to-content link, modal keyboard focus trapping, Escape dismissal, focus restoration after save/close, and hidden offscreen mobile navigation. To run automated tests against **synthetic, loopback-only API fixtures** (not real Caddy listeners, certificates, DNS or stored production sites):

```sh
npm ci --prefix web --ignore-scripts
npm run build --prefix web
npm exec --prefix web -- playwright install chromium
npm run test:e2e --prefix web -- --project=chromium
```

Firefox and WebKit projects are also available when their Playwright browser binaries are installed. The pinned, development-only axe-core runner additionally scans synthetic login, overview, routes, modal, settings, security, mobile, and dark/light states for WCAG A/AA violations. Automated checks and browser keyboard tests **do not** constitute a complete screen-reader, user, or production accessibility certification.

## Dependency security gate

The required `validate` CI uses patched Go 1.27.4 (while `go.mod` retains the compatible minimum Go 1.26 API version) and runs the official `govulncheck@v1.10.0` tool in source mode against the Go vulnerability database. It rejects **reachable** affected Go symbols, while keeping imported-but-unreachable advisories visible for follow-up. Official Go tooling sends module/version metadata to its public vulnerability database, not source files; local tool telemetry is disabled for the CI step. This is a development check, not a runtime security certification or proof of safe production upgrade.

The dependency security baseline corrects [GO-2026-6865](https://pkg.go.dev/vuln/GO-2026-6865) by pinning `github.com/klauspost/compress` v1.20.2, and updates `golang.org/x/net` to v0.62.0 and `golang.org/x/crypto` to v0.59.0 for their corresponding advisories. Alpha.1–alpha.3 used the affected compression version and carry release-page warnings; alpha.4 and later pin patched v1.20.2.

## Security and limitations

Please report suspected vulnerabilities through
[private security reporting](.github/SECURITY.md), not public issues.
Dependabot dependency updates are proposed as reviewable pull requests with
required CI and no automatic merging.

See [staging and production acceptance](docs/STAGING_ACCEPTANCE.md) and [security/MIPS boundaries](docs/SECURITY_AND_MIPS.md) for precise tests and outstanding gates.

This is an **early development build**, not a production-accepted release. The pre-release binary on GitHub can lag behind the protected main branch; check the source commit and version before testing. It has no external authentication integration, multi-user roles, certificate inventory, active upstream health checks, request-log viewer, DNS-provider UI, backup scheduler or audit-log retention. The API is bearer-token protected and same-origin, but the local console serves HTTP on loopback only. Access it remotely through an authenticated SSH tunnel; do not publish TCP 8787 directly or expose Caddy's admin interface (typically port 2019).

Managed routes may use automatic HTTPS under Caddy's normal certificate and domain eligibility rules. The UI reports configuration state, **not** certificate issuance, DNS reachability, upstream health or internet availability.

Before any production cutover, validate route-by-route parity, TLS, redirects, authentication, WebSockets, streaming, backup/restore, controlled rollback, ownership, and firewall impact. No production migration is performed by a build or by running tests.

## Design-system status

The console uses the current GoreeCloud UI **alpha** semantic colors, Iris accent, spacing and shape values from [GoreeCloud/gc-ui](https://github.com/GoreeCloud/gc-ui). These tokens are provisional and do not establish Stable design acceptance. GoreeShield privacy, Wardveil cybersecurity, GoreeCloud Mesh integration and GoreeCloud Identity remain distinct ecosystem responsibilities; deeper integrations are not yet implemented here.

## Source, license and attribution

- Original Gateway Go control plane, React console, tests and documentation: Apache License 2.0, see [LICENSE](LICENSE).
- **Caddy v2**: [caddyserver/caddy](https://github.com/caddyserver/caddy), Apache License 2.0. Caddy engine, modules and internal behavior are upstream work, not GoreeCloud originals.
- GoreeCloud UI tokens: [GoreeCloud/gc-ui](https://github.com/GoreeCloud/gc-ui), MIT. Used as an alpha design reference.
- React, Lucide and build tools remain independently licensed dependencies; see their package notices.

This product is independent of and not endorsed by the upstream Caddy maintainers.

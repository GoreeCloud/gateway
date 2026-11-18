# Third-party notices

GoreeCloud Gateway is an original control plane and console built on upstream Caddy, not a reimplementation of its proxy or TLS internals.

**Caddy** — github.com/caddyserver/caddy/v2, pinned to v2.11.9. Apache License 2.0. Copyright Caddy contributors and respective upstream rightsholders. See https://github.com/caddyserver/caddy/blob/v2.11.9/LICENSE

**GoreeCloud UI reference tokens** — GoreeCloud/gc-ui tokens/gc.tokens.json, MIT. Copyright GoreeCloud contributors. https://github.com/GoreeCloud/gc-ui

**React / ReactDOM** — MIT, copyright Meta Platforms and contributors.

**Lucide icons** — ISC, copyright Lucide contributors.

**esbuild, TypeScript and associated development dependencies** — see package-level license metadata and the checked-in package lockfile.

All original Gateway files are licensed under Apache-2.0 (LICENSE). Dependency licenses remain independently effective. This project does not claim ownership or exclusive authorship of Caddy.

**Porkbun Caddy DNS-01 provider** — github.com/caddy-dns/porkbun v0.3.1, MIT, copyright Niall Fitzpatrick. Uses github.com/libdns/porkbun v1.0.1 (MIT). DNS API credentials are not embedded, fetched, or logged by the managed web console. https://github.com/caddy-dns/porkbun

**Playwright Test** — Microsoft, Apache License 2.0, https://playwright.dev/ — pinned development-only browser testing dependency. Playwright browser binaries are not bundled in the GoreeCloud Gateway release archive.

**axe-core / @axe-core/playwright** — Deque Systems, Mozilla Public License 2.0 (MPL-2.0), https://www.deque.com/axe/axe-core/ — pinned development-only accessibility test dependencies. Neither the axe engine nor test runner is bundled in the native Gateway release binary.

**Go dependency security maintenance (November 18, 2026)** — github.com/klauspost/compress v1.20.2 (BSD-3-Clause; preserved license in [third_party_licenses/klauspost-compress-LICENSE](third_party_licenses/klauspost-compress-LICENSE)); golang.org/x/net v0.62.0 and golang.org/x/crypto v0.59.0 (BSD-3-Clause). Their upstream copyrights and license obligations remain effective. This project does not claim ownership of these packages.

**OpenTelemetry Go indirect dependencies** — `go.opentelemetry.io/otel`, `otel/sdk`,
`otel/sdk/log`, `otel/trace` and related log/metric modules pinned to
v1.47.0. Apache License 2.0, copyright OpenTelemetry contributors. See
[third_party_licenses/opentelemetry-go-LICENSE](third_party_licenses/opentelemetry-go-LICENSE)
and https://github.com/open-telemetry/opentelemetry-go . These version upgrades
address [GHSA-q7gw-24fr-5p69](https://github.com/advisories/GHSA-q7gw-24fr-5p69)
in the pinned dependency graph. All upstream rights remain effective.

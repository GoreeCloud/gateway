# Third-party Notices and Provenance

GoreeCloud Gateway imports the upstream **Caddy** Go web server framework as a pinned dependency; it does not copy or claim ownership over Caddy's source or trademarks.

- Upstream: https://github.com/caddyserver/caddy
- Official documentation: https://caddyserver.com/docs/
- License: Apache License 2.0
- Trademark: Caddy is a registered trademark of its respective owner.
- Required Go module: `github.com/caddyserver/caddy/v2`, version declared in `go.mod`

Upstream copyright and license notices remain with the upstream modules. Distribution of compiled binaries must carry legally required notices for the full dependency tree. This file is not a substitute for those notices.

## Porkbun DNS integration

- Module: `github.com/caddy-dns/porkbun` v0.3.1, backed by `github.com/libdns/porkbun` v1.0.1.
- Both are MIT-licensed upstream projects. See [third_party/porkbun-LICENSE](third_party/porkbun-LICENSE).
- GoreeCloud does not claim ownership of their code or trademarks.

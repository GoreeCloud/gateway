# Gateway staging and production acceptance

**Development evidence only. No production cutover or DNS/certificate acceptance is implied.**

## CI and safe local verification

Run these in a disposable staging workspace on Linux:

```sh
npm ci --prefix web --ignore-scripts
npm run build --prefix web
npm audit --prefix web --audit-level=high
go test -count=1 ./...
go vet ./...
CGO_ENABLED=0 go build -trimpath -o gateway .
./gateway list-modules | grep -E 'http.handlers.reverse_proxy|dns.providers.porkbun'
```

Automated Go tests now exercise a real Caddy engine behind ephemeral loopback HTTP listeners: upstream response and headers, streaming chunks, and a complete WebSocket text-frame echo. They also check API auth/Host enforcement, private storage isolation, rollback, and *offline* Porkbun Caddyfile adaptation. No public DNS, DNS API key, ACME issuance, TLS port 443, or real user routes are involved.

## Outstanding production gates

- [ ] Inventory full legacy Caddyfile configuration, routing, redirects, headers, `handle`, basic auth, DNS and TLS without disclosing secrets.
- [ ] Verify Caddyfile adaptation for each legacy rule, including host/path precedence. The simplified browser route editor does not support the full Caddyfile grammar; advanced configurations must use the Caddy-compatible CLI mode.
- [ ] Demonstrate HTTP routing, redirects, WebSocket, streaming, reverse-proxy headers and authentication parity on isolated staging.
- [ ] Test authorized Porkbun DNS-01 with least-privilege staging credentials loaded from a secret store, then validate certificate issuance and renewal with real HTTPS clients.
- [ ] Validate DNS, security policies, backups, restore integrity, independent access, port ownership, downtime window and a rehearsed rollback path *before* altering any production listener.
- [ ] Independently validate remote Identity, GoreeShield privacy controls, Wardveil security review, GoreeCloud Mesh integration contracts and GoreeCloud UI responsive/keyboard/screen-reader acceptance.
- [ ] Confirm health of the existing ingress at runtime, not just Docker container state. Do not replace, stop or restart existing ingress to demonstrate an alpha release.

The upstream `github.com/caddy-dns/porkbun@v0.3.1` provider is MIT licensed, configured by `dns.providers.porkbun` and registers Caddyfile `api_key` and `api_secret_key` fields. The offline test uses inert environment placeholders only; the web editor neither collects nor persists credentials.

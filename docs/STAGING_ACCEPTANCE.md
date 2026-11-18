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

Automated Go tests exercise a real Caddy engine behind ephemeral loopback listeners: upstream HTTP response and headers, streaming chunks, a WebSocket text-frame echo **plus an upstream HTTP 101 hop-by-hop header leak regression for [CVE-2026-55458](https://github.com/caddyserver/caddy/security/advisories/GHSA-8qx7-j2v2-x6q5)**, and a locally trusted HTTPS fixture. A separate synthetic legacy-style Caddyfile regression exercises `handle` routing, path-prefix stripping, upstream request/response headers, Basic Auth deny/allow, an explicit-matcher redirect, and unmatched-route fallback. They also check API auth/Host enforcement, private storage isolation, *injected-loader* engine rollback on failed persistence, reopening saved 0600 sites state, a real Caddy loopback listener that closes on the same empty configuration produced by `Render(nil)` and serves requests again after reloading the local fixture, and *offline* Porkbun Caddyfile adaptation. These managed-state and Caddy listener lifecycle recovery tests do not establish production process-crash recovery, restoration of existing certificate volumes, protected backups, or any actual VPS rollback. No public DNS, DNS API key, ACME issuance, TLS port 443, or real user routes are involved.

## Redacted, non-mutating legacy ingress preflight

Use the independently reviewed [preflight script](../scripts/ingress_preflight.py) only on a host where Docker inspection is authorized. Default invocation displays help. The live flag executes five fixed, bounded read-only commands (`docker inspect`, `docker port`, and `docker exec caddy version/list-modules`), never inspects credentials or certificate files, and never restarts or reconfigures the container.

```sh
python3 scripts/ingress_preflight.py
python3 scripts/ingress_preflight.py --live --container caddy
```

Output is a metadata-only JSON summary: running state, healthcheck presence, port mapping *presence* without IPs, expected mount *destination* presence without host sources, Caddy binary version, Porkbun registration, and explicitly **unverified** backup/TLS/route/recovery gates. If Docker access fails, the script emits a generic error code without raw stderr. Treat even redacted operational metadata as private unless its disclosure is authorized. Do **not** publish source Caddyfiles, service hostnames, Docker labels, backup locations, volume paths or certificates. Seven synthetic Python unit tests run in CI without Docker.

This preflight alone cannot verify that a backup exists or can be restored, that certificates will renew, that any external route works, or that a Caddy security upgrade is safe. A confirmed legacy binary below Caddy 2.11.5 remains a priority for a separately authorized, rollback-ready patch.

For evaluating privately verified legacy Caddy versions against upstream advisories and planning rollback, use the [risk-gated patch and recovery runbook](LEGACY_CADDY_PATCH_GATES.md). Keep actual host/version/security posture in private operational records; public source tests and Docker metadata are not recovery proof.

## Outstanding production gates

- [ ] Inventory full legacy Caddyfile configuration, routing, redirects, headers, `handle`, basic auth, DNS and TLS without disclosing secrets.
- [ ] Verify Caddyfile adaptation for each legacy rule, including host/path precedence. The simplified browser route editor does not support the full Caddyfile grammar; advanced configurations must use the Caddy-compatible CLI mode.
- [ ] Demonstrate HTTP routing, redirects, WebSocket, streaming, reverse-proxy headers and authentication parity on isolated staging.
- [ ] Test authorized Porkbun DNS-01 with least-privilege staging credentials loaded from a secret store, then validate certificate issuance and renewal with real HTTPS clients.
- [ ] Validate DNS, security policies, backups, restore integrity, independent access, port ownership, downtime window and a rehearsed rollback path *before* altering any production listener.
- [ ] Independently validate remote Identity, GoreeShield privacy controls, Wardveil security review, GoreeCloud Mesh integration contracts and GoreeCloud UI responsive/keyboard/screen-reader acceptance.
- [ ] Confirm health of the existing ingress at runtime, not just Docker container state. Do not replace, stop or restart existing ingress to demonstrate an alpha release.

The upstream `github.com/caddy-dns/porkbun@v0.3.1` provider is MIT licensed, configured by `dns.providers.porkbun` and registers Caddyfile `api_key` and `api_secret_key` fields. The offline test uses inert environment placeholders only; the web editor neither collects nor persists credentials.

# GoreeCloud Gateway — Porkbun DNS-01 Compatibility

**Status:** Development integration only; no real Porkbun API calls or certificate issuance have been performed.

## Purpose

The distribution imports upstream `github.com/caddy-dns/porkbun` v0.3.1, registering `dns.providers.porkbun`. This enables the same `tls { dns porkbun { ... } }` syntax as existing Caddy installations. Upstream Caddy remains the underlying engine, pinned to v2.11.7; GoreeCloud does not claim authorship of either upstream project.

## Offline validation

```sh
./gateway list-modules | grep 'dns.providers.porkbun'
./gateway adapt --config configs/Porkbunfile.example --adapter caddyfile
```

The example uses documentation domain `example.test` and **environment-variable names only**, never real API keys. Never run it against production or request actual certificates.

## Secrets and networking

Provision `PORKBUN_API_KEY` and `PORKBUN_API_SECRET_KEY` through an authorized restricted runtime secret source. Never commit or upload credential values to GitHub, workflows, logs, container layers or user-facing docs. Do not give the web/Android control console direct access to the provider or ACME credentials.

The official plugin depends on `github.com/libdns/porkbun v1.0.1`. During third-party review, the provider library's HTTP requests were observed to lack a dedicated timeout and some errors include upstream response bodies; evaluate this risk in staging and do not expose raw API errors.

## Release gate

Porkbun module registration and Caddyfile adaptation do not demonstrate successful DNS-01 challenges. Before any production replacement, verify Caddyfile route parity, protected credential loading, DNS zone permissions, ACME staging behavior, existing certificate storage, backup and rollback, issuance and renewal checks, and explicit change authorization. Keep the original ingress and its data intact until safe cutover.

See [CUTOVER.md](CUTOVER.md) for the full deployment process.

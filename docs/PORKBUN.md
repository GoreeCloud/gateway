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


## November 9, 2026 — isolated candidate module and synthetic-route parity

Build the reviewed GoreeCloud Gateway source into a **local candidate executable**, then use the nondeploying preflight to compare its reported Caddy engine and Porkbun module versions with the committed Go dependency manifest. This check is not applied to the running legacy production container:

```bash
umask 022
GOWORK=off CGO_ENABLED=0 go build -trimpath -o ./gateway-candidate ./cmd/gateway
chmod go-w ./gateway-candidate
python3 scripts/candidate_module_parity.py --binary ./gateway-candidate
```

The tool invokes only `version`, `list-modules --versions`, and `adapt` against the repository's **synthetic** Porkbun and proxy examples. It strips any inherited Porkbun environment variables, supplies fixed noncredentials to the isolated adapter, refuses group/world-writable or symlinked test executables, and prints **only redacted booleans**. The mandatory module set includes the current GoreeCloud security handlers, Caddy ACME and reverse proxy, and Porkbun v0.3.1. A source manifest or binary version mismatch fails the gate. CI checks the same preflight against its newly built Linux candidate.

Passing this test confirms only module presence/version and synthetic configuration adaptation. It does **not** verify production image digest, real Porkbun credentials, ACME DNS-01 issuance or renewal, migration imports and live routes, private NetBird reachability, off-host certificate backup restoration or rollback. No service is started, no certificate is requested, no DNS records are changed, and no cutover is authorized.

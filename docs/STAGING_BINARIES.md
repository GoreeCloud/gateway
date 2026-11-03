# Linux AMD64 Gateway staging binaries

**Status:** Development-only CI output. Not a signed release, production deployment, or proof of compatibility with legacy VPS ingress.

## What CI builds

The Go/Web workflow compiles two Linux/AMD64 executables with `CGO_ENABLED=0` and `-trimpath` after tests, vetting, and configuration smoke checks:

- `gateway`: Caddy v2.11.7-based GoreeCloud gateway with Porkbun DNS-01 provider and opt-in middleware.
- `gateway-console`: token-protected, loopback-bound, read-only administrative status API and embedded web interface.

At the end of the workflow (only when the prior checks succeed), it uploads a short-lived GitHub artifact containing both binaries, a SHA-256 manifest and the `go version -m` module/build provenance for both. The workflow verifies the binary ELF class and AMD64 target without executing the artifact after upload. Artifacts are retained for seven days, not published as releases.

## Verifying an authorized staging copy

Extract the downloaded **GitHub Actions artifact ZIP** in a controlled environment. Preserve its unmodified filenames and manifest; verify the source commit and completed workflow run before trusting the archive.

```sh
cd linux-amd64
sha256sum -c SHA256SUMS.txt
go version -m ./gateway
go version -m ./gateway-console
```

The target host must be a compatible Linux AMD64 system with appropriate certificates/trust roots and writable, access-controlled Caddy storage if any TLS workflow is ever authorized. Binary checksums and module provenance do not establish TLS, Caddyfile parity, secrets handling, backup integrity, network isolation or workload permissions.

Do **not** execute unknown downloaded binaries or enable listeners on a VPS merely because their checksums match an unreviewed artifact; review the exact commit, CI provenance, dependencies and executable behavior first.

## Strict staging boundaries

- Test with synthetic nonproduction Caddyfiles and loopback/nonconflicting ports.
- Keep Porkbun and other credentials out of GitHub, stdout, build files and binaries.
- Treat Caddy's protected certificate and config mounts as authoritative production state, not development inputs.
- Never point the development console at an exposed Caddy admin API; it requires a trusted local namespace and separate private authenticated HTTPS ingress for remote clients.
- Perform approved staging DNS-01 and renewal rehearsal only with independently authorized credentials and nonproduction zones.
- Preserve all existing Caddy files, containers, data, network attachments and certificate state. Production 80/443 cutover requires a separately reviewed rollback and acceptance path.

See [INGRESS_RECOVERY.md](INGRESS_RECOVERY.md), [PORKBUN.md](PORKBUN.md), and [CUTOVER.md](CUTOVER.md).

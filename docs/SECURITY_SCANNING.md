# Gateway Go dependency vulnerability audits

**Purpose:** Detect publicly known vulnerabilities affecting invoked Go code, independently of compilation and unit-test success. This is a development security control, not a guarantee that a Gateway deployment is secure.

## CI execution

[Gateway Go Vulnerability Audit](../.github/workflows/security.yml) executes on pull requests, pushes to `main`, a weekly Monday schedule, and authorized manual dispatches. It runs on an isolated GitHub Linux runner with the exact Go version declared in `go.mod`. The workflow checks module checksums and invokes the official Go vulnerability scanner `govulncheck`, installed from a **pinned** `golang.org/x/vuln/cmd/govulncheck@v1.8.0` release.

The scanner queries Go's public vulnerability database, examines packages matched by `./...`, and identifies known vulnerabilities in reachable code paths. It does not update Gateway's `go.mod` or `go.sum`, install software on the VPS, read protected Caddy storage, or require production secrets. The tool is downloaded and executed **only inside CI**, not automatically on a connected laptop or production server.

A failing scanner exit code fails the security workflow. CI retains up to seven days of textual output as a GitHub Actions artifact. Read and classify actual findings before modifying dependencies. Separate download/database outages or tool incompatibilities from reported vulnerable calls; never silence or ignore findings without a documented, reviewed rationale.

## Triage and remediation

When a known reachable issue is reported, check the affected module, advisory, Go toolchain version, the invoked symbol and availability of patched upstream releases. Prefer a minimal reviewed upgrade to Caddy, Porkbun, or an affected dependency; examine upstream change logs and integration compatibility, update the module lockfile, then repeat Go tests, Caddyfile adaptation, HTTP/HTTPS proxy staging, Android compatibility and the vulnerability audit before proposing a release.

The local command on a trusted development machine, after reviewing the pinned scanner source, is:

```sh
go install golang.org/x/vuln/cmd/govulncheck@v1.8.0
"$(go env GOPATH)/bin/govulncheck" ./...
```

## Boundaries

This is a **source-level Go reachability analysis**, not a complete SBOM, container image scan, active network penetration test, JavaScript package audit, Android Gradle/Kotlin dependency audit, secret scan, production TLS test, or proof that Caddy's configured dynamic handlers are invoked in exactly the same manner as the scanner's static analysis. CVEs can exist without a corresponding database record or reachable static call path. A clean report does **not** mean there are no vulnerabilities.

Keep the original Caddy ingress untouched until certificate/backup recovery, real Porkbun ACME staging renewal, route parity, operational rollback and production acceptance gates are independently verified. See [CUTOVER.md](CUTOVER.md) and [INGRESS_RECOVERY.md](INGRESS_RECOVERY.md).

# Gateway dependency review cadence

**Status:** Automated *review proposals* only. No dependency update is automatically merged, released or deployed.

The checked-in [Dependabot configuration](../.github/dependabot.yml) asks GitHub to check for version updates weekly and open a limited number of pull requests for:

| Ecosystem | Manifest directory | Review window | Maximum open version-update PRs |
| --- | --- | --- | --- |
| Go modules | repository root (`/`) | Tuesday | 2 |
| Android Gradle/Kotlin | `/android` | Wednesday | 2 |
| GitHub Actions | `/` | Thursday | 2 |

Schedules use America/Chicago timezone. The limits are per package ecosystem, apply to version-update PRs, and do not suppress GitHub's separately handled security-update PRs. These checks are subject to repository-level Dependabot settings and GitHub's available service capacity; merely committing the configuration does not prove a check has run.

For each proposed update, inspect the exact source, upstream project/security advisory, license, permissions, dependency graph, and breaking-change risk. Run the relevant Go race/vet/configuration/HTTP/HTTPS/WebSocket and Android unit/emulator CI checks, plus the official [Go vulnerability audit](SECURITY_SCANNING.md) where applicable, before any approved merge. Treat Caddy and Porkbun as critical components whose version changes require confidential route and ACME staging validation. Do not assume a passing build verifies certificates, service connectivity or user acceptance.

The configuration contains no credentials or external registry authentication. Dependabot cannot change VPS services, open public admin ports, update production secrets, or replace the stopped Caddy ingress. Production deployment remains separately gated by [CUTOVER.md](CUTOVER.md) and [INGRESS_RECOVERY.md](INGRESS_RECOVERY.md).

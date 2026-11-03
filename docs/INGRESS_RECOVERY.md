# GoreeCloud Gateway — Read-Only Legacy Ingress Recovery

| Attribute | Value |
| --- | --- |
| Source of truth | Live Docker and network state, not this document |
| Scope | Containerized legacy Caddy or equivalent reverse proxy |
| Status | Diagnostic procedure; no restart, cutover, or recovery performed by this document |
| Access | Authorized host administrator through a verified management channel |

## Inspect without modifying services

Run `bash scripts/inspect_legacy_ingress.sh [container-name]` on the authorized host. For an additional machine-readable summary, run `python3 scripts/legacy_recovery_audit.py [container-name]`. Its JSON report contains only fixed-category Docker status, error class, TCP/UDP web-listener counts, network endpoint count and whether the three expected mount roots can be stated to exist. It never prints mount sources, network names, hostnames, Docker environment values, certificate data, private keys, or raw daemon logs. Both tools are read-only; neither proves backup restoration or readiness to restart.

Run `bash scripts/inspect_legacy_ingress.sh [container-name]` only in an approved administrative environment if more human-readable output is useful. The script only queries container metadata and listening sockets. It does not read raw configs, container environments, mounted source paths, TLS secrets, credential values, or private hostnames. It never starts/stops containers, recreates networks, or changes ports. A nonzero exit code means the container is not confirmed running, is missing, or inspection could not complete; this does **not** diagnose a root cause or imply a safe restart.

Independently check Docker daemon health, network state, TLS endpoint behavior, backup restoration readiness, and the actual production ingress owner. Keep detailed diagnostics in restricted operations records rather than in this public repository.

## Address-in-use errors

A Docker error such as `failed to set up container networking: Address already in use` can originate from port conflicts, network endpoint allocation, or a transient daemon/network namespace condition. **Zero currently bound ports does not prove a prior conflict is cleared.** Review TCP 80/443 and UDP 443, IPv4/IPv6, container endpoint ownership, restart ordering, relevant Docker daemon logs, and reverse-proxy dependency chains.

Do not speculatively remove bridges, networks, other containers, firewall rules or volumes. A temporary working restart can obscure the root cause; preserve evidence and rollback.

## Recovery gates

Before an authorized live recovery, verify the exact Caddy container and image digest, a trustworthy administrative access route, complete sensitive configuration and certificate backups with restore verification, original Compose/runtime settings, secret injection, DNS and ACME provider modules, listener ownership, and required upstream dependencies.

Prepare health/rollback checks covering HTTP-to-HTTPS, TLS issuer and expiry, authenticated sites, streaming and WebSocket behavior, VPN control-plane dependencies, and certificate renewal. Record operator authorization and a narrow rollback window in restricted task records.

Restoring the legacy Caddy service is separate from deploying GoreeCloud Gateway. The new distribution still requires [staged route parity and rollback testing](CUTOVER.md), including [Porkbun DNS-01 integration](PORKBUN.md). Passing CI, downloading an APK, or creating this runbook is not production acceptance.

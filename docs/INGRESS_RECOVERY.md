# GoreeCloud Gateway — Read-Only Legacy Ingress Recovery

| Attribute | Value |
| --- | --- |
| Source of truth | Live Docker and network state, not this document |
| Scope | Containerized legacy Caddy or equivalent reverse proxy |
| Status | Diagnostic procedure; no restart, cutover, or recovery performed by this document |
| Access | Authorized host administrator through a verified management channel |

## Inspect without modifying services

Run `bash scripts/inspect_legacy_ingress.sh [container-name]` on the authorized host. For an additional machine-readable summary, run `python3 scripts/legacy_recovery_audit.py [container-name]`. Its JSON report contains only fixed-category Docker status, error class, TCP/UDP web-listener counts, network endpoint count and whether the three expected mount roots can be stated to exist. It never prints mount sources, network names, hostnames, Docker environment values, certificate data, private keys, or raw daemon logs. Both tools are read-only; neither proves backup restoration or readiness to restart. The shell inspection additionally reports only a fixed Docker health category (`healthy`, `unhealthy`, `starting`, `none`, or `unknown`) and a bounded restart count. It exits nonzero when the running state is absent or the declared Docker health is unhealthy, starting, or indeterminate. A container with **no configured Docker healthcheck** is reported as `none`, never `healthy`; exit zero still does not establish real route/TLS or dependency health.

The JSON audit exits with status `1` when the container is not running, but still prints its sanitized facts. An exit status of `0` only confirms the observed container state was `running`; it is **not** evidence of recovered ingress, healthy certificates, a valid backup, or accepted production readiness. The script only queries container metadata and listening sockets. It does not read raw configs, container environments, mounted source paths, TLS secrets, credential values, or private hostnames. It never starts/stops containers, recreates networks, or changes ports. A nonzero exit code means the container is not confirmed running, is missing, or inspection could not complete; this does **not** diagnose a root cause or imply a safe restart.

Independently check Docker daemon health, network state, TLS endpoint behavior, backup restoration readiness, and the actual production ingress owner. Keep detailed diagnostics in restricted operations records rather than in this public repository.

## Address-in-use errors

A Docker error such as `failed to set up container networking: Address already in use` can originate from port conflicts, network endpoint allocation, or a transient daemon/network namespace condition. **Zero currently bound ports does not prove a prior conflict is cleared.** Review TCP 80/443 and UDP 443, IPv4/IPv6, container endpoint ownership, restart ordering, relevant Docker daemon logs, and reverse-proxy dependency chains.

Do not speculatively remove bridges, networks, other containers, firewall rules or volumes. A temporary working restart can obscure the root cause; preserve evidence and rollback.

## Recovery gates

Before an authorized live recovery, verify the exact Caddy container and image digest, a trustworthy administrative access route, complete sensitive configuration and certificate backups with restore verification, original Compose/runtime settings, secret injection, DNS and ACME provider modules, listener ownership, and required upstream dependencies.

Prepare health/rollback checks covering HTTP-to-HTTPS, TLS issuer and expiry, authenticated sites, streaming and WebSocket behavior, VPN control-plane dependencies, and certificate renewal. Record operator authorization and a narrow rollback window in restricted task records.

Restoring the legacy Caddy service is separate from deploying GoreeCloud Gateway. The new distribution still requires [staged route parity and rollback testing](CUTOVER.md), including [Porkbun DNS-01 integration](PORKBUN.md). Passing CI, downloading an APK, or creating this runbook is not production acceptance.


## Offline backup digest preflight — November 8, 2026

The repository includes `scripts/ingress_backup_integrity.py` to check **two already-created, authorized offline backup archives** (configuration and ACME/certificate data) against operator-recorded SHA-256 hashes. The script reads only local regular files that are not symlinks or hard links, are owned by the verifying process's effective user ID, require owner-only permissions, and are at most 8 GiB each. Readability by an administrator is not sufficient evidence that the input is the expected owner's archive. It streams SHA-256 without decompressing or logging file content and returns a fixed redacted JSON classification without printing supplied filenames, host paths, credentials, certificate contents or exception text.

For a trusted *offline* backup copy, record expected SHA-256 values in a protected channel and run:

```bash
python3 scripts/ingress_backup_integrity.py \
  --config-archive /path/to/offline/config.tar --config-sha256 EXPECTED_CONFIG_SHA256 \
  --acme-archive /path/to/offline/acme.tar --acme-sha256 EXPECTED_ACME_SHA256
```

**November 10, 2026 evidence hardening:** The verifier now also requires the two *opened and hashed* archive file identities to be distinct. Supplying the same regular file twice—even through two path spellings, with two matching expected digests—fails closed with `distinct_archive_sources_verified:false` and `both_hashes_match:false`. Direct helper calls without a checked pair also default to *unverified* source independence. This rejects accidental double-counting of a single archive. Two distinct files with matching hashes are still not proof that either contains the correct Caddy configuration or ACME data; backup origin, content validity and isolated restoration remain independently required.

This is an **integrity-only check**, not a backup creation or restore tool. Two matching hashes **do not** establish an isolated restore, certificate issuance/renewal, Porkbun DNS-01 parity, secure backup key handling, host route behavior, private NetBird reachability, rollback, or production cutover readiness. The script cannot authorize any change to the running legacy Caddy; a protected off-host backup and independently tested restore remain mandatory before an upgrade.


## November 9, 2026 — offline tar header safety gate (nonextracting)

The optional `scripts/offline_archive_structure.py` checks the **headers of an existing private offline tar/tar.gz archive** for basic unsafe structure before attempting a separately authorized and isolated restore. It never extracts, mutates, loads a production volume, reads Caddy runtime configuration, opens a socket, requests a certificate, or logs member names/contents. An operator may run this only against a previously copied, trusted, owner-owned, owner-only archive:

```bash
python3 scripts/offline_archive_structure.py /path/to/owner-protected/offline-copy.tar
```

It refuses symlinks/hardlinks/FIFOs/devices, unsafe absolute or parent-traversal member paths, duplicate/case-folding names, file-as-directory hierarchy conflicts in either archive order, unsupported sparse/PAX metadata, malformed or empty archives, archives with more than 20,000 entries, members over 4 GiB or aggregate declared regular-member sizes over 8 GiB. The archive itself must be a regular owner-controlled non-hardlinked file with owner-only permissions and size at most 8 GiB. The inspector now also requires an owner-controlled, owner-private **immediate parent directory**, opened without following a symlink, and opens the archive relative to the held directory descriptor. This rejects a symlinked or world-readable immediate parent but does not independently verify every ancestor directory or backup provenance; operators must supply trusted offline-copy paths. A bounded fixed-shape JSON report omits member paths and content. No extraction occurs. This is a conservative *structural* check; perfectly valid archives may be rejected when they contain links or specialized metadata.

**Separate evidence gates are mandatory:** independent SHA-256/strong integrity check of an authenticated existing backup, trusted provenance, encrypted off-host storage, safe secret lifecycle, **full disposable isolated extraction and Caddy/ACME data restore**, DNS-01 renewal acceptance, live route/TLS/private NetBird parity and tested rollback. A successful tar scan does not prove all bytes are restorable, that archived files are semantically valid, that they match production, or that cutover can proceed. The tool deliberately reports `production_cutover_authorized: false` even on success. It is a test helper, **not** a replacement for the legacy Caddy backup/recovery process.

## November 10, 2026 — private parent directory descriptor gate

`scripts/ingress_backup_integrity.py` now opens the candidate archive's immediate parent directory using Linux `O_DIRECTORY|O_NOFOLLOW` before opening the archive **relative to that held directory descriptor**. Both the parent directory and the archive must be owned by the effective user, and the parent must not grant group/other access. This rejects world-readable or symlinked parent directories even if the archive itself is 0600 and its expected hash matches; it also prevents a post-parent-check path substitution from redirecting the subsequent archive open to a different directory. Output remains limited to fixed nonsecret error categories (`unsafe_parent_directory` or `unavailable_or_unsafe`), with no filesystem paths or certificate data.

This is an integrity preflight for existing trusted **offline copies only**. It does not verify every ancestor directory, guarantee identity against a compromised same-user process or malicious kernel, establish backup origin or role, extract or restore Caddy/ACME data, or grant live migration approval. Continue to require encrypted off-host backups, independently verified isolated restore, Porkbun renewal testing, route/TLS parity and rollback evidence.

## November 10, 2026 — raw archive descriptor ownership on failure

The offline Caddy configuration/ACME backup verifier now retains responsibility for the opened archive file descriptor until `os.fdopen()` successfully transfers it into a context-managed file object. If the initial `fstat()` or file-object wrapping fails, the raw descriptor is closed by a `finally` block; after transfer, the file object always owns closure. Synthetic regressions inject stat and fdopen failures and verify that the specific opened descriptor has been closed without exposing the archive path or contents. This is resource-safety hardening for trusted **offline** archives only. A passing checksum remains insufficient evidence of correct archive provenance, isolated restoration, Porkbun renewal, route/TLS parity or approved production cutover.

## November 10, 2026 — archive inspection descriptor cleanup

The read-only `scripts/offline_archive_structure.py` now retains ownership of an already-open raw archive descriptor until `fdopen()` succeeds and enters its context manager. On an injected wrapper failure it closes the raw descriptor and reports only the existing fixed `offline_archive_rejected` classification. The local tests verify the FD is closed and normal tar-header inspection still works; member paths and content are not emitted. **This remains a structural header-only check.** A matching checksum or successful tar scan does not mean the Caddy configuration and ACME state can be restored, that certificates renew through Porkbun DNS-01, or that production cutover and rollback are authorized.

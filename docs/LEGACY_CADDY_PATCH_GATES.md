# Legacy Caddy ingress: security-upgrade and recovery gates

**Status: P1 planning document — not an approved production change or verified restore.**
This public, general-purpose runbook **does not publish any existing deployment's**
hostname, current binary version, module inventory, service status, ports,
certificate condition or backup findings. Keep those observations in authorized
**private** operational records. Upstream's
[CVE-2026-55458 advisory](https://github.com/caddyserver/caddy/security/advisories/GHSA-8qx7-j2v2-x6q5)
documents an HTTP-101/WebSocket hop-by-hop response-header issue in Caddy
versions before **2.11.5**. Assess any specific installation's affected version,
upgrade needs and patch readiness privately; do **not** infer its exposure,
compromise or recovery state from this repository.

The development Gateway binary uses upstream Caddy 2.11.9, but **its simple
web route editor is not a lossless import for the existing complex Caddyfile**.
A new Gateway alpha build or passing synthetic tests are **not** sufficient
reasons to replace legacy ingress.

## 1. Identify owners and establish independent access — OPEN

- Confirm the target is the actually serving legacy ingress host, not a similarly
  named secondary machine. Identify current container/process ownership, source
  Docker Compose file, immutable image digest, reverse-proxy listener ports and
  other services sharing the host. Do not infer service health from Docker's
  `running` flag or image tag alone.
- Verify out-of-band administrative access that will continue to work if the
  web gateway, NetBird/VPN, DNS or container networking fails. Confirm the
  ability to access an independent console before any planned interruption.
- On the **authorized** legacy host, the checked-in preflight
  [script](../scripts/ingress_preflight.py) is an optional metadata-only read:
  `python3 scripts/ingress_preflight.py --live --container caddy`.
  Review what it reads first. Its JSON intentionally cannot prove route health,
  certificate validity, backup integrity or rollback readiness. Keep even
  redacted operational output private.

## 2. Preserve a *restorable* recovery point — OPEN

- Record separately, in protected operational storage, the current immutable
  image ID/digest, Compose definition, Caddyfile, plugin identity, mounted
  certificate/configuration/state roots, service-account permissions and the
  exact version of each required runtime dependency. Never put paths, private
  hostnames, passwords, DNS API values or certificate private keys into
  public issues, CI logs, or source control.
- Capture encrypted, access-controlled copies using an authorized and inspected
  backup method. A backup *filename* or a Docker mount *existing* does not
  establish recovery. Handle ACME state and the private keys as secrets.
- Restore the **actual backup** to an isolated staging machine, with no public
  DNS, no production ingress ports, no unauthorized certificate issuance, and
  no production credentials exposed. Verify checksums, file modes, ability to
  start an independently controlled old image, and a complete return to its
  previous tested state. Preserve rollback evidence and the original image.
- If private storage cannot be inspected safely or an independently tested
  restore is missing, mark this gate **BLOCKED**, not complete.

## 3. Build and validate the patched candidate off-host — OPEN

- Choose an inspected upstream Caddy version at least 2.11.5; this project's
  pinned 2.11.9 source includes a separately licensed MIT Porkbun DNS-01
  module. Check all original image build flags, plugin versions, dependencies,
  license/notice requirements and compatible configuration semantics.
- Build candidate artifacts on a disposable staging host. Verify the **embedded**
  Caddy version using `caddy version`, and the exact module identifier
  `dns.providers.porkbun` using `caddy list-modules`. Image tags do not prove
  binary identity. Check release provenance and vulnerability findings.
- Using a safely redacted or secret-substituted copy, adapt the confidential
  legacy Caddyfile with the candidate Caddy parser. Preserve all semantics:
  matcher and `handle` precedence, `rewrite`, redirects, reverse-proxy
  host/path behavior, Basic Auth, allow/deny rules, header manipulation, gzip/
  streaming/WebSocket behavior, upstream HTTPS trust, and failure behavior.
  File adaptation alone is not runtime parity. Keep full original configuration
  and credential values entirely out of CI and public repositories.
- Run isolated HTTP, HTTP/2, HTTP/3 and WebSocket regression tests with
  appropriate local trust and synthetic upstreams. Confirm upstream `Host`,
  `X-Forwarded-*` and response-header behavior against known-good results,
  including the HTTP-101 upgrade scenario in the advisory when applicable.
- Do not bind staging to the production TCP 80/443 or UDP 443 listeners.

## 4. Prove DNS-01 and certificate lifecycle in *authorized* staging — OPEN

- Use a separately authorized staging DNS zone, least-privilege short-lived
  Porkbun credentials and isolated ACME state. Do not use live production DNS
  or leak an environment file, webhook URL or API response.
- Verify issuance, trusted HTTPS clients, renewal, key storage permissions,
  DNS propagation/cleanup and credential rotation. Observe rate limits and
  test recovery after process restart. The existing offline Porkbun adapter
  test does not satisfy any of these checks.
- Obtain independent certificate expiration/issuer inventory for every actual
  production route only through authorized private read-only tooling.

## 5. Production change approval and rollback triggers — NOT GRANTED

Only after the preceding gates have documented evidence should an operator
obtain a production-specific authorization, defined maintenance window,
working independent access, named rollback owner, full hostname/security
acceptance matrix and a rehearsed recovery sequence. Compare old and new
route responses with trusted clients. Define measurable triggers to restore
the verified old image and backup state if TLS renewal, auth, traffic,
upstream routing or host access regresses. Verify rollback from an actually
restored copy; simply restarting a stopped container is insufficient.

Any future execution must be a **separately authorized** controlled change,
not an automatic action of this document, CI or a Gateway release.
Do not stop/restart/remove existing production ingress processes or containers,
change NetBird/VPN, modify firewall/DNS records, mount protected state, or
overwrite live certificate directories during these planning steps.

## Ownership and evidence

Gateway issue [#2](https://github.com/GoreeCloud/gateway/issues/2) tracks
source and staging acceptance. The founder's relevant P1 Todoist migration
and Gateway tasks track host recovery and cutover decisions. GoreeShield
owns privacy/retention, Wardveil owns threat assessment and incident review,
Mesh owns permitted integrations, Identity owns authentication, and
GoreeCloud UI owns interface/accessibility standards.

**Unverified as of this document:** production route parity, backup integrity,
independent restoration, DNS-01 issuance/renewal, authenticated remote
management, effective CVE exploitability, final cutover authorization and
recovery from an actual failed upgrade.

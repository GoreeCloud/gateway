# Read-only private hostname/TLS preflight

**Status:** Development tool for static troubleshooting. **Not a route parser, certificate validator, service test, or authorization to modify production.**

## Purpose

When a mesh-VPN-connected client resolves a private hostname to an expected peer but fails its HTTPS handshake, first distinguish the network from the host's TLS/SNI virtual-host configuration. Successful TLS for another hostname on the same IP:443 does **not** prove that the failing hostname has a certificate, virtual-host route, or running upstream. Avoid blindly changing VPN routing, certificate storage, Caddy listeners, Docker bridges or DNS.

The read-only script checks only one-line **root site declaration candidates** from a specifically authorized local Caddyfile. It never makes network requests and never emits the target name, config lines, credentials, upstreams, private addresses or exception messages. Its JSON report contains fixed enum/boolean fields.

Example from a trusted shell with access to a reviewed local configuration file:

    python3 scripts/private_site_preflight.py /path/to/authorized/Caddyfile private.example.test

Exit status 0 = literal site declaration candidate seen. Status 1 = no visible HTTPS candidate or HTTP-only candidate. Status 2 = input unreadable, unsupported/indeterminate due to imports, or structurally unbalanced source. **None means the TLS endpoint actually works.** A declaration match is not evidence of a valid certificate or a deployed upstream.

The classifier distinguishes literal HTTPS-prefixed hostnames, implicit Caddy HTTPS site addresses, one-label wildcard candidate addresses, and explicit HTTP-only declarations. It only considers root block headers; it does not treat comments, nested matchers, quoted response bodies or similar-looking hostnames as virtual hosts. This is intentionally conservative. Snippets, Caddyfile imports, address-list formats outside the supported subset, placeholders, runtime Caddy admin API overrides, and effective certificate stores are not resolved. Imports and unbalanced braces prohibit a reliable negative declaration conclusion.

## Follow-up validation without changes

1. On an authorized NetBird client, verify the management/signal state, permitted peer path and FQDN resolution. Use real resolver results, not an assumed VPN address.
2. Observe HTTPS/TLS from that client, without disabling certificate validation. Compare a **separately authorized** known-working SNI hostname on the same destination, if applicable. The TLS alert occurs before any HTTP status; do not interpret it as an upstream application 4xx or 5xx response.
3. On the authorized ingress host, confirm the *running* container's Caddyfile bind mount and read-only process status. The file named in this script must match that mount. Verify whether any live Caddy admin/API configuration differs from disk before concluding site absence.
4. Review private site declaration, SNI match, certificate issuer/storage/expiry, private DNS policy, expected upstream, and Caddy authorization rules in a restricted operational context. Do not dump certificate keys, config contents, environment variables or personal data to public CI logs.
5. Before changes to live ingress, confirm complete protected backup and tested restore, independent administrative access, certificate issuance/rate limits, production impact approval, and a rollback plan. Test a nonconflicting staging listener, then require end-to-end authorized client and unauthorized-client checks.

Passing the static preflight plus CI tests cannot replace certificate validation, secure NetBird policy verification, private upstream health, cross-device behavior, or an approved production rollout. Keep host-specific evidence in authorized private operations tasks, not in this public repo.

See [legacy ingress recovery](INGRESS_RECOVERY.md) and [cutover safeguards](CUTOVER.md).

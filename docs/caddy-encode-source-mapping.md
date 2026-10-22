# Production Caddy encode handler — sanitized read-only mapping

**Date:** 2026-10-22  
**Status:** Partial source evidence. Not a source-to-target route acceptance record.  
**Authority:** Production Caddy remains live; native Gateway has not taken ownership of listeners.

The read-only VPS investigation selected the running production Caddy container by exact identity. Without emitting raw adapted configuration, secrets, hostnames, or upstream addresses, it independently verified the configured Caddyfile source checksum before and after an in-memory adaptation:

`sha256:2fa08f33aee6299b3c0d4b5de4e1ccd49a10f426961e89565400f3bb31343c4d`

| Evidence field | Read-only result |
| --- | --- |
| Caddy version | 2.11.4 |
| Adapted HTTP servers | 1 |
| Adapted HTTP handlers | 33: 15 reverse-proxy, 12 respond, 5 header, 1 encode |
| Encode handler | Sanitized record 25, nested route `9.0.0.0` (depth 1) |
| Next handler in same sequence | Reverse proxy, sanitized record 26 at `9.0.0.1` |
| Request-matcher category | host (values intentionally omitted) |
| Codecs and preference | zstd preferred, then gzip |
| Explicit `minimum_length` | Not specified in adapted source |
| Explicit response matcher | None in adapted source |

The existing saved preflight report was **not** collected from the production Caddy container. It describes a separate application ingress with only two handlers and uses a superseded collector. It **must not** be used to prove production migration parity. A fresh report from the production container, generated with the exact reviewed current-main collector and kept in a restricted local location, is required. This record does not reproduce confidential source or replace a protected source review.

## Unresolved checks

1. Verify whether Caddy's in-memory serving configuration matches its startup Caddyfile after any administrative reload.
2. Bind the protected production source identity, sanitized report, and reviewed native route mapping to one verified migration manifest.
3. Caddy documents a 512-byte default threshold for absent `minimum_length` and special streaming treatment for SSE/Incremental responses. Verify effective behavior on the installed version against synthetic staging equivalents rather than inferring it from the adapted field alone.
4. Test host/route precedence, TLS, headers, transport, error and retry behavior, SSE and normal unknown-length stream semantics with isolated synthetic endpoints.
5. Demonstrate rollback, certificates, DNS, firewall and listener ownership in a reversible authorized target rehearsal before any production promotion.

No action in this evidence-gathering step changed Caddy, Docker, Gateway, certificates, DNS, firewall or network listeners. See [migration issue #44](https://github.com/GoreeCloud/gateway/issues/44).

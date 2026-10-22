# Production Caddy encode handler — sanitized read-only mapping

**Date:** 2026-10-22  
**Status:** Correctly targeted read-only production inventory collected; source-to-target route parity not accepted.  
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

The previously saved `caddy-read-only-preflight.txt` was **not** collected from the production Caddy container. It describes a separate application ingress with only two handlers and uses a superseded collector. It **must not** be used to prove production migration parity.

On 2026-10-22, the operator's VPS ran the exact collector fetched from Gateway `main` revision `b85d98dac2696be8ca780e2c6d24548de5b82230`, validating Git blob `98a5a743dd0d21f17739d2b0c02b7bb8e39f7dff` and passing its self-test. The collection explicitly set `CADDY_CONTAINER=caddy`, wrote an access-restricted mode-`600` report into the operator's existing private Gateway review directory, and validated the required read-only fields. The sanitized counts were **one HTTP server, 33 handler records, one encode handler**, matching the independent inspection. The configured source SHA-256 was checked again after collection and remained unchanged.

The protected report's SHA-256 is `cf992558066f2b972954d595ba4edb8d6b36e456d9ce3928b4aecdd68a36a96b`. This digest is an identity anchor, **not** proof that the report represents in-memory Caddy state or that routes have been mapped or accepted. Neither the report's protected contents nor source hostnames, upstream addresses, certificates, or credentials are published here.

An additional read-only check fetched Caddy's active configuration through the container-local admin API (`127.0.0.1:2019/config/`) and independently adapted the unchanged startup Caddyfile. Canonical JSON (sorted keys, compact serialization) yielded **the same SHA-256** for both views: `5eb785b9318488a04c9f2ab77a1ab81aee09285929257a56c6aa9acef218a0f9`. The full handler counts agreed as well. This establishes a point-in-time in-memory/startup configuration match without exposing the JSON; it does **not** establish native Gateway parity, TLS behavior, or immunity to subsequent Caddy reloads.

## Unresolved checks

1. Reconfirm the point-in-time in-memory/startup configuration match at migration freeze, particularly after any administrative reload.
2. Bind the now-collected protected production source identity and report to an independently reviewed native route mapping and verified migration manifest.
3. Caddy documents a 512-byte default threshold for absent `minimum_length` and special streaming treatment for SSE/Incremental responses. Verify effective behavior on the installed version against synthetic staging equivalents rather than inferring it from the adapted field alone.
4. Test host/route precedence, TLS, headers, transport, error and retry behavior, SSE and normal unknown-length stream semantics with isolated synthetic endpoints.
5. Demonstrate rollback, certificates, DNS, firewall and listener ownership in a reversible authorized target rehearsal before any production promotion.

No action in this evidence-gathering step changed Caddy, Docker, Gateway, certificates, DNS, firewall or network listeners. See [migration issue #44](https://github.com/GoreeCloud/gateway/issues/44).

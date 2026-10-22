# Bounded native static-response encoding (Development)

This native `gzip`/`zstd` implementation is deliberately **opt-in** and applies only to literal `static_response` routes. A separate [proxy encoding feature](proxy-response-encoding.md), with a strictly experimental unknown-length option, is available at Development source level; neither feature establishes Caddy `encode` parity, target-environment acceptance, or a production cutover signal. Caddy remains authoritative.

## Configuration

An otherwise valid static-response route may declare:

```json
{
  "action": "static_response",
  "static_response": {"status_code": 200, "body": "A sufficiently long literal response..."},
  "response_encoding": {"formats": ["zstd", "gzip"], "minimum_length": 512}
}
```

The snippet shows the action-specific fields, not a complete route. Supported formats are exactly `zstd` and `gzip` with no duplicates; their order sets the tie-breaking preference. The default `minimum_length` is 512 bytes, and an explicit threshold must be between 0 and 65,536 inclusive. A configured policy is rejected for any proxy route; older configurations without the field remain unchanged.

## Isolated behavior

- Compresses bounded, eligible literal `GET` and `HEAD` responses only when the request explicitly accepts a configured coding with positive quality. Explicit `q=0` wins over a wildcard. An absent or malformed `Accept-Encoding` never turns on compression.
- Uses Caddy's documented default **media-type allowlist** as the starting point, skips media outside that allowlist, and avoids encoding partial responses, range requests, responses with `Content-Encoding` or `Content-Range`, `Cache-Control: no-transform`, and bodyless statuses.
- Applies a compatible `Vary: Accept-Encoding` cache marker even when a specific response stays uncompressed. Encoded responses set the coding and exact encoded length, remove representation-specific validators/digests, and suppress the body for `HEAD` while retaining the length of the corresponding `GET`.
- Literal bodies remain bounded by the existing 16 KiB `static_response` validation. The policy and preference ordering are included in the deterministic configuration-parity fingerprint. Invalid values fail validation and direct runtime activation; rejected reloads keep the last good state.

## Not yet covered

The live Caddy source contains an `encode` handler (`zstd` then `gzip`) in a nested route. The separate, opt-in known-length proxy encoder is still narrowly scoped; unknown upstream lengths, SSE incremental flushes, WebSocket upgrades, upstream pre-encoded bodies, response matcher placement, complete status/content type edge cases, exact Caddy compression defaults, and production-equivalent performance remain **unverified**. Do not infer full source parity from either development feature or deploy the candidate into production.

The next stage requires independently reviewed exact Caddy source semantics, unknown-length/SSE compression parity, a recovery/rollback rehearsal, and the gated migration verification described in [issue #44](https://github.com/GoreeCloud/gateway/issues/44).

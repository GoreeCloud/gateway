# Opt-in known-length proxy response encoding (Development)

This incremental native Gateway feature compresses **eligible, known-length HTTP 200 GET responses** from reverse-proxy routes using `zstd` or `gzip` with streaming backpressure. Caddy remains the live production authority. It does **not** establish full Caddy `encode` parity or authorize any production migration.

## Configuration

Add the following optional field to an otherwise valid reverse-proxy route:

```json
"proxy_response_encoding": {"formats": ["zstd", "gzip"], "minimum_length": 512}
```

`formats` specifies one or two distinct supported codings, in tie-breaking preference order. The default `minimum_length` is 512 bytes; an explicit value must be between 0 and 65,536 inclusive. This setting is **not** interchangeable with the `response_encoding` field reserved for static routes. Both route action boundaries are rejected by configuration validation and direct runtime activation. Unconfigured routes remain unchanged.

## Response behavior

- Negotiates explicit `Accept-Encoding` qualities; never compresses when no supported positive-quality coding was accepted.
- Compresses eligible, known-length bodies as they are consumed, without buffering the complete upstream response. Output length is unknown until streaming completes; native chunked framing is delegated to the Go HTTP server.
- Adds `Vary: Accept-Encoding` for opt-in routes, retains other `Vary` dimensions, and removes original byte-identity validators, `Content-Length`, `Content-MD5`/digest fields and `Accept-Ranges` when compression changes the representation.
- Leaves upstream bodies untouched for unknown lengths, SSE, incremental-flush hints, protocol upgrades, `HEAD`/non-GET, non-200/partial responses, `Range` requests, request/response `Cache-Control: no-transform`, upstream trailers, already-compressed or Go-transport-uncompressed responses, noncompressible media, and bodies below the configured threshold.
- Preserves upstream errors during pipe compression; closing the downstream response closes the pipe and upstream body to avoid indefinite producer blocking.

## Validation scope and limitations

Isolated unit and HTTP reverse-proxy integration tests cover encoding/decoding, content negotiation, per-route policy validation, parity fingerprinting, and excluded cases. A known-length upstream HTTP response is a prerequisite; this does not turn an unknown-length source into a buffered response.

The reviewed live Caddy inventory shows one nested `encode` handler ahead of a reverse proxy with zstd/gzip preference. That behavior must be mapped to an exact route and reviewed against live source policy before a staging rehearsal. **Not yet equivalent**: unknown-length dynamic compression, SSE flush behavior, broad Caddy response matchers, upstream error timing, byte-for-byte quality/performance parity, live routing/TLS/DNS parity, or target-VPS acceptance.

Full migration gates and remaining work are tracked in [issue #44](https://github.com/GoreeCloud/gateway/issues/44). Keep Caddy in production; no listener, firewall, certificate, DNS, Docker, or gateway service changes are authorized by this source feature.

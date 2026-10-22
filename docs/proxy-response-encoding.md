# Opt-in proxy response encoding (Development)

This native Gateway feature compresses **eligible HTTP 200 GET proxy responses** with `zstd` or `gzip` and streaming backpressure. Known-length bodies are eligible by default when the policy is enabled. Unknown-length, non-incremental streams are eligible **only with a second explicit opt-in** and a zero minimum length. Caddy remains the live production authority. Neither mode establishes full Caddy `encode` parity or authorizes a production migration.

## Configuration

Add the following optional field to an otherwise valid reverse-proxy route:

```json
"proxy_response_encoding": {"formats": ["zstd", "gzip"], "minimum_length": 512}
```

`formats` specifies one or two distinct supported codings, in tie-breaking preference order. The default `minimum_length` is 512 bytes; an explicit value must be between 0 and 65,536 inclusive. This setting is **not** interchangeable with the `response_encoding` field reserved for static routes. Both route action boundaries are rejected by configuration validation and direct runtime activation. Unconfigured routes remain unchanged.

To opt in to unknown-length bodies, the route must explicitly select the different semantics:

```json
"proxy_response_encoding": {"formats": ["zstd", "gzip"], "minimum_length": 0, "allow_unknown_length": true}
```

The explicit zero threshold is mandatory: an unknown-length source has no reliable declared size to compare with a positive threshold before sending response headers. This option may encode small and even empty unknown-length representations; it is not a faithful translation of Caddy's 512-byte default or its response-start behavior. This setting is rejected on static routes and must not be enabled as a production migration shortcut.

## Response behavior

- Negotiates explicit `Accept-Encoding` qualities; never compresses when no supported positive-quality coding was accepted.
- Compresses bodies as they are consumed without buffering the complete upstream response. For explicitly enabled unknown-length streams, each upstream read chunk is flushed through the compressor to preserve incremental delivery and downstream backpressure. Output length is unknown until streaming completes; native transfer framing is delegated to Go.
- Adds `Vary: Accept-Encoding` for opt-in routes, retains other `Vary` dimensions, and removes original byte-identity validators, `Content-Length`, `Content-MD5`/digest fields and `Accept-Ranges` when compression changes the representation.
- Leaves bodies untouched for unknown length **without the explicit override**, SSE, incremental-flush hints, protocol upgrades, `HEAD`/non-GET, non-200/partial responses, `Range` requests, request/response `Cache-Control: no-transform`, upstream trailers, already-compressed or Go-transport-uncompressed responses, noncompressible media, and known-length bodies below the configured threshold.
- Preserves upstream errors during pipe compression; closing the downstream response closes the pipe and upstream body to avoid indefinite producer blocking.

## Validation scope and limitations

Isolated unit and HTTP reverse-proxy integration tests cover encoding/decoding, content negotiation, per-route policy validation, parity fingerprinting, explicitly enabled unknown-length delivery, and exclusion cases. Tests require evidence that the first compressed segment can be read **before** the upstream source produces its next segment. An unknown-length source is never converted into a fully buffered response.

The production Caddy inventory shows one nested `encode` handler ahead of a reverse proxy with zstd/gzip preference. It does not specify an explicit minimum or response matcher. Caddy's documented default is a positive 512-byte threshold and its SSE/`Incremental` behavior differs from the native conservative pass-through policy. The experimental zero-threshold unknown-length switch **does not resolve that parity gap**. Remaining gaps: normal unknown-length 512-byte selection, SSE/Incremental coding and early-header behavior, exact live response matchers, upstream error timing, byte-for-byte/performance parity, routing/TLS/DNS parity, and target-VPS acceptance.

Full migration gates and remaining work are tracked in [issue #44](https://github.com/GoreeCloud/gateway/issues/44). Keep Caddy in production; no listener, firewall, certificate, DNS, Docker, or gateway service changes are authorized by this source feature.

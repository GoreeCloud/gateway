# Synthetic reverse-proxy runtime parity test

**Status:** Development-only local runtime test. It does not authorize or establish production ingress replacement.

The test `scripts/test_runtime_proxy_parity.py` launches the pinned GoreeCloud Gateway binary using `configs/RuntimeParity.example`, plus **two synthetic Python HTTP backends**. Every listener is explicitly bound to `127.0.0.1` on unprivileged test ports. The Gateway configuration disables the admin API and automatic HTTPS/ACME issuance.

The test verifies:

- Distinct `/internal/*` and fallback routes select the expected local upstreams, including a directly connected loopback allowlist that ignores a forged `X-Forwarded-For` header.
- Each request receives a new cryptographically random 32-character hexadecimal `X-Request-ID`. A client-supplied forged request ID is replaced, and the header forwarded upstream equals the response header.
- Opt-in `X-Frame-Options` and `X-Content-Type-Options` are present.
- A synthetic HTTP 307 redirect preserves its target path.
- A small chunked streaming response passes through correctly.
- The reverse proxy forwards an HTTP/1.1 WebSocket Upgrade handshake with the correct `Sec-WebSocket-Accept` header, then relays **two masked client text frames** and the corresponding unmasked upstream echo frames end-to-end. This verifies basic bidirectional data flow, not multi-hour session durability.

Run only with a reviewed development binary:

```sh
CGO_ENABLED=0 go build -trimpath -o gateway ./cmd/gateway
python3 scripts/test_runtime_proxy_parity.py
```

This test does **not** prove long-lived WebSocket session reliability under load and reconnects, binary/fragmented/control WebSocket frames, long-running streams, HTTP/2, HTTP/3, full production hostname/rule parity, trust-boundary equivalence, certificate issuance or renewal, protected storage restoration, IPv6, external-network exposure, or service-level availability. These require separate acceptance evidence.

The script checks for pre-existing test port listeners, starts only temporary local processes, creates no certificates, injects no production credentials, and terminates its children. Never use it as a production Caddyfile or copy production domains/secrets into the test.

See [MIGRATION_PREFLIGHT.md](MIGRATION_PREFLIGHT.md), [INTEGRATION_TEST.md](INTEGRATION_TEST.md), and [CUTOVER.md](CUTOVER.md).

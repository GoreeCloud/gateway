# Isolated Gateway and console integration test

**Status:** Development-only smoke test. Not a production deployment or cutover acceptance.

The script `scripts/test_connected_console.sh` starts two compiled executables on the same trusted Linux network namespace:

- `gateway` with the explicitly loopback-bound, synthetic HTTP-only `configs/Gatewayfile.example` at `127.0.0.1:8080`, using Caddy's default loopback admin API at `127.0.0.1:2019`.
- `gateway-console` on `127.0.0.1:7689`, using an ephemeral random bearer token kept out of logs and source.

Run it only after reviewing and compiling the exact source:

```sh
bash scripts/test_connected_console.sh
```

The smoke test confirms that the actual connected Caddy admin API produces a reduced, authenticated `/api/v1/overview` response, without exposing raw admin configuration. It checks rejected anonymous/write/cross-site requests, throttling invalid credentials despite forged forwarded addresses, continued access using the correct token, and embedded web response-security headers.

The script aborts if its loopback test ports appear occupied. It terminates only child processes it started, removes temporary logs and uses no public domains, privileged web ports, ACME certificates, real upstreams, DNS-provider secrets or production configuration.

Passing this test does **not** demonstrate TLS certificate issuance, a configured private HTTPS remote endpoint, production Docker network namespace parity, complete migrated reverse-proxy routes, WebSocket/HTTP3 operation, recoverable certificate storage, signed release provenance or safe replacement of the stopped legacy Caddy container.

See [CUTOVER.md](CUTOVER.md), [CONSOLE.md](CONSOLE.md), and [INGRESS_RECOVERY.md](INGRESS_RECOVERY.md).

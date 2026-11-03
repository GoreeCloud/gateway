# Verified local HTTPS console staging test

**Status:** Synthetic local integration test. It does not create a production HTTPS endpoint or prove live certificate renewal.

The CI script `scripts/test_private_console_tls.py` verifies GoreeCloud Gateway Web and the read-only management API through an additional local Caddy TLS reverse proxy. All three services run on an isolated GitHub runner, using no real sites, production credentials or VPS storage.

The test starts the Gateway engine on `127.0.0.1:8080`, its Caddy admin API on `127.0.0.1:2019`, and the protected web console on `127.0.0.1:7689`. It then starts a **separate** Caddy process with `configs/PrivateConsoleTLS.example`: its admin API is disabled, automatic HTTP redirects are disabled, and the HTTPS listener is explicitly bound to loopback `127.0.0.1:18443`.

The proxy issues a short-lived **internal Caddy test certificate** for `localhost`, storing the private test CA under an ephemeral isolated XDG directory. The test connects with `curl --cacert` using the generated root certificate and `--resolve` mapping to `127.0.0.1`. It does **not** disable TLS verification with `-k`. It verifies that anonymous API calls return 401, valid bearer-token calls return the connected engine overview, and the web response retains its security headers.

To run on a reviewed local Linux development system after compiling both binaries:

```sh
python3 scripts/test_private_console_tls.py
```

The script checks for occupied local ports, generates a fresh synthetic token, destroys its temporary trust data, and terminates only processes it started. It is intentionally neither a Docker configuration template nor a public ingress configuration. Do not import its test CA into global trust stores or distribute its certificates.

**Still unverified:** Android runtime connection to a *real* private HTTPS endpoint, user identity and SSO, network isolation across production Docker namespaces, public domain TLS chain, Porkbun DNS challenge, renewal, authorized secrets, service parity, and VPS rollback. A successful local HTTPS test does not grant permission to restart or replace the stopped Caddy container.

See [CONSOLE.md](CONSOLE.md), [INTEGRATION_TEST.md](INTEGRATION_TEST.md), [PORKBUN.md](PORKBUN.md), and [CUTOVER.md](CUTOVER.md).

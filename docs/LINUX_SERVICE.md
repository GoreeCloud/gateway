# Optional Linux systemd service (development only)

This example runs the native Caddy-powered GoreeCloud Gateway as an unprivileged dedicated Linux account, with private persistent state and a loopback-only web management console. It is **not installed, enabled or deployed** by this repository. The current Gateway development alpha has not passed production DNS/TLS, route/auth parity, backup restoration or rollback acceptance.

## Review before using

- The service uses a **static dedicated account** named `goreecloud-gateway`. Do not use `DynamicUser=true`: on some systemd systems its private `StateDirectory` symlink can be rejected by Gateway's symlink-parent storage protection.
- The console binds to `127.0.0.1:8787` and starts **read-only**: there is no `--allow-write` flag. However, **read-only does not disable Caddy listeners for persisted routes**. If an existing `sites.json` contains public hosts, the process can bind ports 80/443 and request certificates. **Use a new, empty state directory in an isolated staging VM first**; never point this example at an existing production Caddy data directory.
- The environment file is read by systemd to supply `GATEWAY_ADMIN_TOKEN`. Protect it as a secret: root-owned, mode 0600, dedicated to Gateway. The token also exists in the running Gateway process environment; same-user/root access remains a risk. Do not include it in command lines, source, tickets, screenshots, shell history or logs.
- `CAP_NET_BIND_SERVICE` permits low-port binding for an explicitly approved Caddy ingress. It does not itself configure public routing or TLS. `ProtectSystem=strict` and `ReadWritePaths` restrict persistent writes to Gateway's state directory.
- This unit does **not** confer GoreeCloud Identity, GoreeShield consent enforcement, Wardveil monitoring, Mesh registration or recovery guarantees.

## Optional isolated-VM preparation

After independently verifying the downloaded binary, the source commit, and its SHA-256 checksum, create a dedicated system account if it does not already exist:

```sh
sudo useradd --system --home-dir /var/lib/goreecloud-gateway \
  --shell /usr/sbin/nologin --no-create-home goreecloud-gateway
sudo install -m 0755 ./gateway /usr/local/bin/gateway
sudo install -d -m 0700 /etc/goreecloud-gateway
sudo sh -c 'umask 077; printf "GATEWAY_ADMIN_TOKEN=%s\n" "$(openssl rand -hex 32)" > /etc/goreecloud-gateway/gateway.env'
sudo chown root:root /etc/goreecloud-gateway/gateway.env
sudo chmod 0600 /etc/goreecloud-gateway/gateway.env
sudo install -m 0644 examples/systemd/goreecloud-gateway.service \
  /etc/systemd/system/goreecloud-gateway.service
sudo systemd-analyze verify /etc/systemd/system/goreecloud-gateway.service
```

These commands are examples for a fresh **test VM only**, not instructions to run on an existing VPS. The installation step changes system paths and requires an operator to review and execute it manually. Protect the generated token in your approved local secret manager before starting a service; never share the value in chat. systemd manages `/var/lib/goreecloud-gateway` using `StateDirectoryMode=0700` when the service starts.

Only after separate staging authorization would the operator run:

```sh
sudo systemctl daemon-reload
sudo systemctl enable --now goreecloud-gateway.service
sudo systemctl --no-pager status goreecloud-gateway.service
curl --noproxy '*' --fail http://127.0.0.1:8787/api/health
```

The status route requires the bearer token; the health endpoint does not. Access the management UI locally or through a separately authenticated SSH tunnel, **never by exposing port 8787**. Do not add `--allow-write` or actual public host routes before approval.

## Stop, recovery and limitations

On an isolated staging VM, a planned shutdown uses `sudo systemctl stop goreecloud-gateway.service`. **Do not stop a live public ingress without a verified independent rollback and backup restore plan.** The service's private directory is intentionally separate from legacy Caddy state, and the unit includes no migration or automatic certificate import.

This example has not been tested under a complete real-device systemd service lifecycle, public ACME issuance, provider DNS, actual certificates or host firewalls. Review `docs/STAGING_ACCEPTANCE.md` and `docs/LEGACY_CADDY_PATCH_GATES.md` before any production proposal.

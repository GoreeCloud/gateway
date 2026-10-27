# Security policy

I require GoreeCloud Gateway to keep Caddy's privileged admin API on loopback, require authentication before returning configuration, and keep live configuration changes disabled by default. This initial implementation is an **alpha**, not a public-admin service security certification.

- Run Gateway as a dedicated least-privilege account and isolate its backup directory (`0700` directory, `0600` files).
- Keep the access token in protected process environment or a secret manager, never in `.env` committed to Git, browser storage, source code, logs, or screenshots.
- Configure `GATEWAY_PUBLIC_ORIGIN` with the exact HTTPS origin behind a trusted authenticated proxy. Do not bind Gateway or Caddy's admin API to wildcard interfaces.
- Use a private/VPN-connected administrative route where possible. Set ingress request-size limits, TLS and auth, and restrict firewall access.
- Validate routing policy, origin controls, cookie behavior, shutdown/restart behavior, recoverability and TLS in a staging environment before production.
- Configuration snapshots may include sensitive module settings. Guard, rotate and securely purge these with a dedicated retention policy.
- The change-review endpoint shows structural paths and node types, not configuration values. A review is bound to the authenticated session, exact draft fingerprint and live base revision. It cannot replace staging validation.
- The current pre-apply read and /load are two separate operations; another independent Caddy writer can race between them. Avoid concurrent writers until a tested conditional API update workflow is introduced.
- An absent or malformed upstream-status response is a telemetry gap, not a healthy upstream. Do not treat missing counters as zeros when making operational decisions.
- Gateway's activity log is only a process-local convenience, not a compliance audit log.

Report vulnerabilities privately to GoreeCloud's administrator rather than publishing exploits or credentials in repository issues.

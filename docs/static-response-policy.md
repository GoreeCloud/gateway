# Bounded native static-response policy (Development)

Gateway's optional route action `static_response` writes a literal HTTP response without selecting, connecting to, or health-checking a backend. This is a migration parity building block, **not** a claim that a live Caddy `respond` handler has been translated or accepted.

Routes without `action` retain the existing `reverse_proxy` behavior and configuration fingerprint. An explicit `reverse_proxy` action is equivalent. Static-response routes require an explicit action and `static_response.status_code`, and must omit `service_id`. They can use the existing bounded `response_headers` set policy, including `Content-Type`, to set response headers.

```json
{
  "id": "maintenance",
  "action": "static_response",
  "hostname": "maintenance.example.test",
  "path_prefix": "/",
  "exposure": "private",
  "enabled": true,
  "tls": {"mode": "disabled"},
  "response_headers": {"Cache-Control": "no-store"},
  "static_response": {"status_code": 503, "body": "Temporarily unavailable\n"}
}
```

The example uses TLS-disabled transport for isolated local tests; production ingress requirements and certificate profiles still apply. A status from 200 through 599 is accepted. Response bodies are capped at 16 KiB; statuses 204, 205, and 304 must have no body. HEAD returns the corresponding status and headers without a body. A nonempty body defaults to `text/plain; charset=utf-8` unless `Content-Type` is explicitly set. The action never interpolates request values, expands templates, evaluates expressions, or implements advanced Caddy responders.

Unknown actions, a static response combined with a service, a static-response definition on a proxy route, invalid status, and oversized or forbidden-status bodies are rejected even when directly activating runtime state. Changes to the action, status, body, and policy headers are included in the validated configuration fingerprint. The entire route must still pass hostname, exposure, TLS, and overlap validation before loading as a configuration.

Migration operators must review the exact source Caddy `static_response` handler and the ordering of adjacent matchers before drafting its Gateway route. The current preflight inventory reports handler counts, not proof of semantic equivalence. No live Caddy, DNS, firewall, certificate, listener, or Gateway production deployment is changed by this source capability.

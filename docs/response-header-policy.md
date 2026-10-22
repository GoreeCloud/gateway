# Bounded response-header set policy (Development)

The optional `routes[].response_headers` object sets a small number of fixed
HTTP response headers **after** an upstream response is received. It is a
partial native representation of Caddy response-header handling, not complete
Caddy handler parity.

Example route property:

```json
"response_headers": {
  "X-Content-Type-Options": "nosniff",
  "Referrer-Policy": "strict-origin-when-cross-origin"
}
```

- Header names must be valid HTTP field names. They are matched
  case-insensitively, and conflicting spellings are rejected.
- Values must be fixed strings without unsafe control characters. At most 64
  headers, 4,096 bytes per value, and 16,384 total name/value bytes are
  permitted.
- Connection/framing or content-coding headers cannot be set through this
  mechanism; body compression is not performed by this policy.
- The configured value **replaces** the same-named upstream header. It does
  not support append, remove, regex substitution, deferred writes, or dynamic
  placeholders.
- Headers apply to successfully received proxied responses. Gateway-generated
  errors, including no-route and unavailable-upstream responses, are not
  covered by this setting.
- Configuration parity fingerprints include both header names and values;
  HTTP field-name casing alone does not change the fingerprint.

Migration plans must explicitly classify Caddy header rules unsupported by
this subset. A successful response-header test does not establish full route,
TLS, certificate, policy, or production cutover parity. Production Caddy
remains authoritative until separate acceptance gates are satisfied.

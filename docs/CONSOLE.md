# GoreeCloud Gateway — Web and Android Management Clients

Status: in development; no VPS deployment.

Both clients use a first-party, read-only console API with token authentication. The web client is served by a loopback-bound companion service; Android uses a native interface and HTTPS connectivity through trusted private access. Direct public exposure of Caddy's administration API is prohibited.

This initial phase includes live status, configured hostname discovery, connection diagnostics, and per-platform onboarding. Write operations and production cutover require separate hardening, authorization, backup, and rollback testing.

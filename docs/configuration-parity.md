# GoreeCloud Gateway configuration parity evidence

GoreeCloud Gateway must not replace Caddy merely because the native proxy can start or pass isolated traffic tests. Before a migration rehearsal, the intended Gateway configuration must be compared with an independently reviewed migration-candidate identity.

`internal/config/parity.go` provides a deterministic SHA-256 fingerprint for the complete validated Gateway configuration. Services, routes, backends, certificate profiles, backend membership, route methods, and the trusted-ingress-proxy CIDR set are normalized before hashing. Trusted proxy addresses and prefixes are canonicalized and sorted; changing the trusted-proxy trust boundary must change the fingerprint. The emitted evidence contains only aggregate counts and immutable fingerprints; it does not expose route hostnames, backend URLs, certificate paths, credentials, or request data.

The expected fingerprint must come from a separately reviewed migration preparation step. The parity function does not inspect the live Caddy runtime, translate a Caddyfile, discover production routes, or claim that the expected fingerprint is trustworthy merely because an operator supplied it. Target-environment migration tooling must derive and retain that reviewed source identity using the approved GoreeCloud deployment process.

For Caddy migration, the v2 reviewed manifest separately identifies the source Caddyfile checksum and the expected Gateway fingerprint. The source and review hashes are preserved in the minimized migration evidence; they cannot replace the actual Gateway fingerprint check. `BuildConfigParityEvidence` binds the comparison to an exact GoreeCloud Gateway source revision. `ValidateConfigParityEvidence` fails closed unless the expected and actual SHA-256 fingerprints match exactly and the evidence keeps `production_cutover_authorized=false`.

Configuration parity is only one migration gate. It does not prove listener ownership, certificate readiness, TLS renewal, sustained production capacity, observability, backup/restore, rollback, Privacy Shield, Wardveil Security, Everkeep, Glaze UI, GoreeCloud Mesh, GoreeCloud Identity, governance acceptance, or production safety.

Caddy remains production-authoritative until the full retained migration-evidence set is complete and an explicit production cutover is separately approved.
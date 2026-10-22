# Gateway Migration-Source Manifest

GoreeCloud Gateway uses `goreecloud-gateway-migration-source-manifest/v1` to bind configuration-parity evaluation to a retained, independently reviewed identity for the migration source.

For the current migration, `source_system` is fixed to `caddy`. A different source requires a separately reviewed contract rather than silently reusing this one.

The manifest contains only:

- recording time;
- migration-source system and environment labels;
- exact reviewed configuration SHA-256;
- aggregate service, route, backend, and certificate-profile counts;
- SHA-256 identity of the independent review evidence; and
- `production_cutover_authorized=false`.

It deliberately excludes hostnames, backend URLs, certificate paths, credentials, request data, route contents, certificate material, and private runtime diagnostics.

**Do not confuse the two digests:** the preflight collector's `config_sha256` is the SHA-256 of the source Caddy configuration file. In this manifest, `config_sha256` is the *reviewed expected Gateway configuration-parity fingerprint* produced from the translated Gateway candidate's validated semantics (including the trusted-proxy boundary). They are different identities and are not interchangeable. The independently retained review evidence must associate the exact Caddy source revision/configuration-file hash with the reviewed translation and expected Gateway fingerprint; `review_evidence_sha256` identifies that evidence. Neither digest alone proves faithful translation.

Gateway strictly decodes the manifest, rejects unknown or trailing JSON, requires valid immutable identities, verifies the aggregate counts against the candidate Gateway configuration, and then evaluates the candidate configuration fingerprint against the reviewed migration-source fingerprint.

## Local parity verifier

`cmd/gateway-migration-verify` provides a fail-closed local verification path for a candidate Gateway configuration and an independently reviewed migration-source manifest. It requires all three inputs explicitly:

```sh
go run ./cmd/gateway-migration-verify \
  -config /path/to/gateway-candidate.json \
  -manifest /path/to/reviewed-caddy-source.json \
  -source-revision 0123456789abcdef0123456789abcdef01234567
```

The command:

1. validates the candidate Gateway configuration;
2. strictly loads and validates the reviewed Caddy migration-source manifest;
3. verifies aggregate counts and the deterministic configuration fingerprint;
4. validates the resulting parity evidence; and
5. emits only the minimized `goreecloud-gateway-config-parity-evidence/v1` JSON on success.

A mismatch, malformed input, invalid source revision, or cutover-authorizing source manifest exits nonzero and emits no acceptance evidence. The verifier does not discover, connect to, read, reload, or mutate Caddy. It operates only on the explicit reviewed files supplied to it and cannot authorize production cutover.

A valid manifest does not prove that it was correctly derived from live Caddy state. The target-environment migration-preparation process must independently produce, review, retain, and govern the source configuration identity and its review evidence. Only then can the resulting parity evidence support a migration gate.

This contract cannot authorize production cutover. Caddy remains production-authoritative until all remaining migration gates and explicit production approval pass.


## Target-VPS read-only preflight

`deploy/vps/caddy-read-only-preflight.sh` provides a bounded live-state discovery step before a migration-source manifest is created. It reports Caddy container/config identity, safe route/upstream/TLS structure, listeners, relevant firewall matches, Docker-network membership, and public certificate metadata without printing raw Caddy configuration, Docker environment values, provider credentials, or private-key material.

The preflight report is **not** a migration-source manifest and is **not** parity evidence. Its purpose is to identify and retain the live inputs that must then be independently reviewed through the approved migration-preparation process. See `docs/vps-caddy-read-only-preflight.md`.

# Gateway Migration-Source Manifest

GoreeCloud Gateway uses `goreecloud-gateway-migration-source-manifest/v2` to bind configuration-parity evaluation to a retained, independently reviewed identity for the migration source.

For the current migration, `source_system` is fixed to `caddy`. A different source requires a separately reviewed contract rather than silently reusing this one.

The manifest contains only:

- recording time;
- migration-source system and environment labels;
- SHA-256 of the exact reviewed source Caddy configuration file (`source_config_sha256`);
- separately reviewed canonical Gateway configuration-parity fingerprint (`expected_gateway_config_sha256`);
- aggregate service, route, backend, and certificate-profile counts;
- SHA-256 identity of the independent review evidence; and
- `production_cutover_authorized=false`.

It deliberately excludes hostnames, backend URLs, certificate paths, credentials, request data, route contents, certificate material, and private runtime diagnostics.

**Distinct identities are mandatory:** the preflight collector's `config_sha256` is the SHA-256 of the source Caddy configuration file and belongs in the manifest's `source_config_sha256`. The manifest's `expected_gateway_config_sha256` is instead the *reviewed expected Gateway semantic fingerprint* of the translated, validated candidate (including trusted proxies). These identities must both be present and must differ; a legacy v1 manifest or an identical value in both fields fails closed. The independently retained review evidence must bind the exact Caddy source/configuration-file hash to the reviewed translation and expected Gateway fingerprint; `review_evidence_sha256` identifies that material. The verifier does not independently inspect live Caddy, so the review process still establishes source authenticity.

Gateway strictly decodes the manifest, rejects unknown or trailing JSON, requires valid immutable identities, verifies the aggregate counts against the candidate Gateway configuration, and then evaluates the candidate configuration fingerprint against the reviewed migration-source fingerprint.

## Local parity verifier

`cmd/gateway-migration-verify` provides a fail-closed local verification path for a candidate Gateway configuration and an independently reviewed migration-source manifest. It requires all four inputs explicitly, including the bounded read-only Caddy preflight report. The report must also contain a nonempty, count-consistent sanitized route inventory: every enumerated handler record needs its server, handler, and explicit matcher/upstream fields (which can be empty when inapplicable). Missing, duplicate, out-of-range, or incomplete records fail closed; the verifier does not assert that their content has been independently verified:

```sh
go run ./cmd/gateway-migration-verify \
  -config /path/to/gateway-candidate.json \
  -manifest /path/to/reviewed-caddy-source.json \
  -preflight-report /protected/path/to/caddy-read-only-preflight.txt \
  -source-revision 0123456789abcdef0123456789abcdef01234567
```

The command accepts only the v2 manifest contract. It additionally requires a report from the exact reviewed read-only collector and fails closed if its Caddy source checksum differs from the manifest, its adapted configuration failed to parse, its safety markers are incomplete/ambiguous, or its report is oversized. The original report must stay in an approved protected location; it may contain internal infrastructure identifiers and is never copied into minimized output. The verifier emits only source and review SHA-256 identities, without configuration values.

The command:

1. validates the candidate Gateway configuration;
2. strictly loads and validates the reviewed Caddy migration-source manifest;
3. bounds and checks the read-only collector report's status and source Caddy configuration hash against the manifest;
4. verifies aggregate counts and the deterministic Gateway configuration fingerprint;
5. validates the resulting parity evidence; and
6. emits only the minimized `goreecloud-gateway-config-parity-evidence/v1` JSON on success.

A missing preflight report, stale/mismatched Caddy source, malformed input, invalid source revision, or cutover-authorizing source manifest exits nonzero and emits no acceptance evidence. The verifier does not discover, connect to, read, reload, or mutate Caddy. It operates only on the explicit reviewed files supplied to it and cannot authorize production cutover.

A matching report and manifest still do not prove that the report came from the current live Caddy container, that the reviewed source was faithfully translated, or that the host-side Compose and certificate evidence is complete. The target-environment migration-preparation process must independently produce, review, retain, and govern the source configuration identity and its review evidence. Only then can the resulting parity evidence support a migration gate.

This contract cannot authorize production cutover. Caddy remains production-authoritative until all remaining migration gates and explicit production approval pass.


## Target-VPS read-only preflight

`deploy/vps/caddy-read-only-preflight.sh` provides a bounded live-state discovery step before a migration-source manifest is created. It reports Caddy container/config identity, safe route/upstream/TLS structure, listeners, relevant firewall matches, Docker-network membership, and public certificate metadata without printing raw Caddy configuration, Docker environment values, provider credentials, or private-key material.

The preflight report is **not** a migration-source manifest and is **not** parity evidence. Its purpose is to identify and retain the live inputs that must then be independently reviewed through the approved migration-preparation process. See [`docs/CADDY-VPS-READ-ONLY-PREFLIGHT.md`](./CADDY-VPS-READ-ONLY-PREFLIGHT.md).

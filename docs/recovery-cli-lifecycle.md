# Gateway recovery CLI — isolated configuration lifecycle

**Status:** Development. Configuration-file operations only; no automatic runtime reload or live cutover.

`cmd/gateway-recovery` exposes four explicit actions: `snapshot`, `stage`, `activate`, and `restore`. Each emits a machine-readable JSON record. Staging does not mutate the active configuration. Activation requires the expected SHA-256 of the active configuration and creates a recovery snapshot before replacing its file. Restoration likewise requires the expected active SHA-256, so a stale rollback instruction fails closed.

## Isolated operator sequence

Use a dedicated, access-restricted test lifecycle root containing a synthetic Gateway configuration. Replace the placeholders with verified **isolated** paths and exact source revisions; do not run these examples against production without separately established service, backup, and rollback acceptance.

```bash
go run ./cmd/gateway-recovery \
  -action stage \
  -root /path/to/isolated/lifecycle \
  -candidate /path/to/candidate.json \
  -source-revision 89abcdef0123456789abcdef0123456789abcdef
```

Read the returned `stage_dir`, then calculate the SHA-256 of the independently verified active configuration before activation.

```bash
go run ./cmd/gateway-recovery \
  -action activate \
  -root /path/to/isolated/lifecycle \
  -config /path/to/isolated/lifecycle/active/gateway.json \
  -staged /path/from/stage_dir \
  -expected-current-sha256 ACTUAL_ACTIVE_SHA256 \
  -current-source-revision 0123456789abcdef0123456789abcdef01234567
```

The activation receipt contains an `activated_config_sha256` and a lifecycle-root-relative `recovery_snapshot`. To reverse the config-file activation:

```bash
go run ./cmd/gateway-recovery \
  -action restore \
  -root /path/to/isolated/lifecycle \
  -config /path/to/isolated/lifecycle/active/gateway.json \
  -snapshot /path/to/isolated/lifecycle/SNAPSHOT_FROM_RECEIPT \
  -expected-current-sha256 ACTIVATED_CONFIG_SHA256
```

These commands are intentionally not a deployment guide. The returned `production_cutover_authorized` values remain false. The CLI does not reload a Gateway process, publish ports, preserve or restore TLS material, handle DNS/firewall state, or demonstrate a working upstream/application health check. Those steps require a separately permitted target rehearsal under Everkeep recovery authority and the [Gateway migration gates](https://github.com/GoreeCloud/gateway/issues/44).

The isolated CLI round-trip, malformed candidate, changed-active-config CAS, and unsupported-action regressions are in `cmd/gateway-recovery/main_test.go`. They prove configuration lifecycle behavior only.

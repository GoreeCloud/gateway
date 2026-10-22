# Gateway isolated configuration recovery rehearsal — 2026-10-22

**Result:** PASS for isolated configuration-file round trip; **NOT ACCEPTED** as operational VPS or production rollback evidence.

This record distinguishes a deterministic, synthetic configuration-recovery test from actual reversal of network listeners, services, or certificate ownership. Production Caddy remained authoritative and unchanged throughout the rehearsal.

## Reproducible isolated test

| Field | Verified value |
| --- | --- |
| Test host | Authorized laptop `personal-laptop-ideapad3-01` |
| Checkout | Isolated temporary checkout; no production paths used |
| Baseline accepted main revision | `3da6d4b77aea2ffd067a6310367f83fd183758c5` |
| Race-enabled command | `go test -race -count=1 ./internal/config -run 'Test(Recovery\|StageRevision\|ActivateStagedRevision)' -v` |
| Result | **9/9 tests PASS** |

The tests exercise staging a candidate B without mutating the active configuration A, activating B with an expected digest and a recovery snapshot, and restoring A with the expected active B digest. After restoration, the original hostname is verified. Additional cases reject unexpected active-configuration changes, tampered or invalid candidates, path escapes, and symbolic-link snapshots.

The attempted extra command-line fixture exercise was interrupted and has **no accepted result**. No production VPS service or Gateway deployment was staged, activated, or restored during this test.

## Production gates deliberately left open

1. Stage, activate, and reverse a **native Gateway service** on an authorized isolated target, exercising actual processes, listener ownership, health checks, and the Caddy fallback path.
2. Validate TLS/certificate, secrets, Docker/service state, DNS, and firewall restoration through approved Everkeep recovery procedures; collect operator-observed evidence.
3. Make the stage/activate operator path reproducible. The current `cmd/gateway-recovery` CLI exposes snapshot and restore, not stage and activate.
4. Assess interprocess locking around digest comparison and rename, plus parent-directory synchronization for stronger crash-consistency guarantees.
5. Verify host-specific route policies against the protected live Caddy source and require a valid migration-source manifest before promotion.

For target/production acceptance, keep `backup_restore_proven=false` and `rollback_rehearsed=false`. Passing config unit tests is not a waiver of those controls. Track migration completion in [issue #44](https://github.com/GoreeCloud/gateway/issues/44).

package config

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const recoveryTestRevision = "0123456789abcdef0123456789abcdef01234567"

func TestRecoverySnapshotRestoreRoundTrip(t *testing.T) {
	root := t.TempDir()
	active := filepath.Join(root, "active", "gateway.json")
	writeRecoveryTestConfig(t, active, "one.acceptance.test")
	original, err := os.ReadFile(active)
	if err != nil {
		t.Fatal(err)
	}
	originalDigest := recoveryTestSHA(original)

	snapshotDir, snapshot, err := CreateRecoverySnapshot(
		root,
		active,
		recoveryTestRevision,
		time.Date(2026, time.August, 29, 12, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Schema != RecoverySnapshotSchemaV1 || snapshot.ConfigSHA256 != originalDigest {
		t.Fatalf("unexpected snapshot: %+v", snapshot)
	}
	if snapshot.ProductionCutoverAuthorized {
		t.Fatal("snapshot authorized production cutover")
	}

	writeRecoveryTestConfig(t, active, "two.acceptance.test")
	candidate, err := os.ReadFile(active)
	if err != nil {
		t.Fatal(err)
	}
	candidateDigest := recoveryTestSHA(candidate)
	if candidateDigest == originalDigest {
		t.Fatal("candidate configuration did not change")
	}

	receipt, err := RestoreRecoverySnapshot(
		root,
		active,
		snapshotDir,
		candidateDigest,
		time.Date(2026, time.August, 29, 12, 1, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Schema != RecoveryReceiptSchemaV1 || !receipt.ConfigValidated || !receipt.CompareAndSwapValidated {
		t.Fatalf("unexpected restore receipt: %+v", receipt)
	}
	if receipt.PreviousConfigSHA256 != candidateDigest || receipt.RestoredConfigSHA256 != originalDigest {
		t.Fatalf("unexpected restore digests: %+v", receipt)
	}
	if receipt.ProductionCutoverAuthorized {
		t.Fatal("restore receipt authorized production cutover")
	}
	restored, err := os.ReadFile(active)
	if err != nil {
		t.Fatal(err)
	}
	if recoveryTestSHA(restored) != originalDigest {
		t.Fatal("active configuration was not restored to snapshot")
	}
}

func TestRecoveryRestoreRejectsChangedActiveConfiguration(t *testing.T) {
	root := t.TempDir()
	active := filepath.Join(root, "active", "gateway.json")
	writeRecoveryTestConfig(t, active, "one.acceptance.test")
	snapshotDir, _, err := CreateRecoverySnapshot(root, active, recoveryTestRevision, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	writeRecoveryTestConfig(t, active, "two.acceptance.test")
	planned, err := os.ReadFile(active)
	if err != nil {
		t.Fatal(err)
	}
	plannedDigest := recoveryTestSHA(planned)

	writeRecoveryTestConfig(t, active, "newer.acceptance.test")
	if _, err := RestoreRecoverySnapshot(root, active, snapshotDir, plannedDigest, time.Now().UTC()); err == nil {
		t.Fatal("restore accepted a configuration that changed after rollback planning")
	}
	current, err := Load(active)
	if err != nil {
		t.Fatal(err)
	}
	if current.Routes[0].Hostname != "newer.acceptance.test" {
		t.Fatalf("changed configuration was overwritten: %+v", current.Routes[0])
	}
}

func TestRecoverySnapshotRejectsPathEscape(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "gateway.json")
	writeRecoveryTestConfig(t, outside, "outside.acceptance.test")
	if _, _, err := CreateRecoverySnapshot(root, outside, recoveryTestRevision, time.Now().UTC()); err == nil {
		t.Fatal("snapshot accepted configuration outside recovery root")
	}
}

func TestRecoverySnapshotRejectsSymlink(t *testing.T) {
	root := t.TempDir()
	realPath := filepath.Join(root, "real.json")
	writeRecoveryTestConfig(t, realPath, "real.acceptance.test")
	linkPath := filepath.Join(root, "linked.json")
	if err := os.Symlink(realPath, linkPath); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, _, err := CreateRecoverySnapshot(root, linkPath, recoveryTestRevision, time.Now().UTC()); err == nil {
		t.Fatal("snapshot accepted symbolic-link configuration")
	}
}

func writeRecoveryTestConfig(t *testing.T, path, hostname string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	data := []byte(`{
  "schema": "goreecloud-gateway-config/v1",
  "services": [{"id":"svc","name":"Service","backend_ids":["backend"]}],
  "routes": [{"id":"route","service_id":"svc","hostname":"` + hostname + `","path_prefix":"/","methods":["GET"],"exposure":"private","allowed_client_cidrs":["10.0.0.0/8"],"enabled":true,"tls":{"mode":"disabled"}}],
  "backends": [{"id":"backend","url":"http://127.0.0.1:18081","enabled":true,"health_path":"/health"}],
  "certificate_profiles": []
}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func recoveryTestSHA(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

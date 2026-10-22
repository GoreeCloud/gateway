package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoreeCloud/goreecloud-gateway/internal/config"
)

const (
	cliPreviousRevision  = "0123456789abcdef0123456789abcdef01234567"
	cliCandidateRevision = "89abcdef0123456789abcdef0123456789abcdef"
)

func TestCLIStageActivateRestoreRoundTrip(t *testing.T) {
	root := t.TempDir()
	active := filepath.Join(root, "active", "gateway.json")
	candidate := filepath.Join(root, "candidate.json")
	writeCLIConfig(t, active, "original.gateway.test")
	writeCLIConfig(t, candidate, "staged.gateway.test")
	original, err := os.ReadFile(active)
	if err != nil {
		t.Fatal(err)
	}
	originalSHA := cliSHA(original)

	var stagedOut bytes.Buffer
	if err := run([]string{
		"-action", "stage", "-root", root, "-candidate", candidate,
		"-source-revision", cliCandidateRevision,
	}, &stagedOut); err != nil {
		t.Fatal(err)
	}
	var staged stageOutput
	if err := json.Unmarshal(stagedOut.Bytes(), &staged); err != nil {
		t.Fatal(err)
	}
	if staged.StageDir == "" || staged.Stage.ProductionCutoverAuthorized {
		t.Fatalf("invalid staged evidence: %+v", staged)
	}
	checkCLIHostname(t, active, "original.gateway.test")

	var activatedOut bytes.Buffer
	if err := run([]string{
		"-action", "activate", "-root", root, "-config", active,
		"-staged", staged.StageDir, "-expected-current-sha256", originalSHA,
		"-current-source-revision", cliPreviousRevision,
	}, &activatedOut); err != nil {
		t.Fatal(err)
	}
	var activation config.ActivationReceipt
	if err := json.Unmarshal(activatedOut.Bytes(), &activation); err != nil {
		t.Fatal(err)
	}
	if !activation.CompareAndSwapValidated || !activation.ConfigValidated ||
		activation.ProductionCutoverAuthorized || activation.RecoverySnapshot == "" {
		t.Fatalf("invalid activation receipt: %+v", activation)
	}
	checkCLIHostname(t, active, "staged.gateway.test")

	var restoredOut bytes.Buffer
	if err := run([]string{
		"-action", "restore", "-root", root, "-config", active,
		"-snapshot", filepath.Join(root, filepath.FromSlash(activation.RecoverySnapshot)),
		"-expected-current-sha256", activation.ActivatedConfigSHA256,
	}, &restoredOut); err != nil {
		t.Fatal(err)
	}
	var recovery config.RecoveryReceipt
	if err := json.Unmarshal(restoredOut.Bytes(), &recovery); err != nil {
		t.Fatal(err)
	}
	if recovery.ProductionCutoverAuthorized || !recovery.CompareAndSwapValidated ||
		recovery.RestoredConfigSHA256 != originalSHA {
		t.Fatalf("invalid recovery receipt: %+v", recovery)
	}
	checkCLIHostname(t, active, "original.gateway.test")
}

func TestCLIActivationRejectsChangedActiveConfig(t *testing.T) {
	root := t.TempDir()
	active := filepath.Join(root, "active.json")
	candidate := filepath.Join(root, "candidate.json")
	writeCLIConfig(t, active, "original.gateway.test")
	writeCLIConfig(t, candidate, "staged.gateway.test")
	original, err := os.ReadFile(active)
	if err != nil {
		t.Fatal(err)
	}
	var stagedOut bytes.Buffer
	if err := run([]string{
		"-action", "stage", "-root", root, "-candidate", candidate,
		"-source-revision", cliCandidateRevision,
	}, &stagedOut); err != nil {
		t.Fatal(err)
	}
	var staged stageOutput
	if err := json.Unmarshal(stagedOut.Bytes(), &staged); err != nil {
		t.Fatal(err)
	}

	writeCLIConfig(t, active, "changed.gateway.test")
	err = run([]string{
		"-action", "activate", "-root", root, "-config", active,
		"-staged", staged.StageDir, "-expected-current-sha256", cliSHA(original),
		"-current-source-revision", cliPreviousRevision,
	}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "changed") {
		t.Fatalf("expected fail-closed CAS check, got %v", err)
	}
	checkCLIHostname(t, active, "changed.gateway.test")
}

func TestCLIStageRejectsInvalidOrMissingCandidate(t *testing.T) {
	root := t.TempDir()
	if err := run([]string{"-action", "stage", "-root", root, "-source-revision", cliCandidateRevision}, &bytes.Buffer{}); err == nil {
		t.Fatal("accepted missing candidate")
	}
	invalid := filepath.Join(root, "invalid.json")
	if err := os.WriteFile(invalid, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{
		"-action", "stage", "-root", root, "-candidate", invalid,
		"-source-revision", cliCandidateRevision,
	}, &bytes.Buffer{}); err == nil {
		t.Fatal("accepted invalid candidate configuration")
	}
	if err := run([]string{"-action", "unexpected"}, &bytes.Buffer{}); err == nil {
		t.Fatal("accepted unsupported action")
	}
	if err := run([]string{"-action", "snapshot", "extra"}, &bytes.Buffer{}); err == nil {
		t.Fatal("accepted positional arguments")
	}
}

func writeCLIConfig(t *testing.T, path, hostname string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	data := []byte(`{
  "schema": "goreecloud-gateway-config/v1",
  "services": [{"id":"svc","name":"Service","backend_ids":["backend"]}],
  "routes": [{"id":"route","service_id":"svc","hostname":"` + hostname + `","path_prefix":"/","methods":["GET"],"exposure":"private","enabled":true,"tls":{"mode":"disabled"}}],
  "backends": [{"id":"backend","url":"http://127.0.0.1:18081","enabled":true,"health_path":"/health"}],
  "certificate_profiles": []
}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func checkCLIHostname(t *testing.T, path, want string) {
	t.Helper()
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Routes[0].Hostname; got != want {
		t.Fatalf("hostname = %q, want %q", got, want)
	}
}

func cliSHA(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

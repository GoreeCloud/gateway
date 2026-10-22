package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/GoreeCloud/goreecloud-gateway/internal/config"
)

func TestRunEmitsValidatedMinimizedParityEvidence(t *testing.T) {
	cfg := migrationVerifierTestConfig()
	fingerprint, err := config.ConfigParityFingerprint(cfg)
	if err != nil {
		t.Fatal(err)
	}
	configPath := writeJSONFile(t, "gateway.json", cfg)
	manifestPath := writeJSONFile(t, "migration-source.json", migrationVerifierManifest(cfg, fingerprint))
	preflightPath := writePreflightFile(t, strings.Repeat("d", 64))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{
		"-config", configPath,
		"-manifest", manifestPath,
		"-preflight-report", preflightPath,
		"-source-revision", strings.Repeat("a", 40),
	}, &stdout, &stderr, func() time.Time {
		return time.Date(2026, 9, 4, 16, 0, 0, 0, time.UTC)
	})
	if code != 0 {
		t.Fatalf("run code=%d stderr=%q", code, stderr.String())
	}
	if strings.Contains(stdout.String(), "gateway.internal") || strings.Contains(stdout.String(), "127.0.0.1") {
		t.Fatalf("minimized parity evidence leaked configuration details: %s", stdout.String())
	}
	var evidence config.ConfigParityEvidence
	if err := json.Unmarshal(stdout.Bytes(), &evidence); err != nil {
		t.Fatal(err)
	}
	if err := config.ValidateConfigParityEvidence(evidence); err != nil {
		t.Fatalf("emitted evidence failed validation: %v", err)
	}
	if evidence.ProductionCutoverAuthorized {
		t.Fatal("migration verifier unexpectedly authorized production cutover")
	}
	if evidence.MigrationSourceConfigSHA256 != strings.Repeat("d", 64) || evidence.MigrationReviewEvidenceSHA256 != strings.Repeat("c", 64) {
		t.Fatal("minimized migration evidence did not bind source and review identities")
	}
}

func TestRunFailsClosedOnParityMismatch(t *testing.T) {
	cfg := migrationVerifierTestConfig()
	configPath := writeJSONFile(t, "gateway.json", cfg)
	manifest := migrationVerifierManifest(cfg, strings.Repeat("f", 64))
	manifestPath := writeJSONFile(t, "migration-source.json", manifest)
	preflightPath := writePreflightFile(t, manifest.SourceConfigSHA256)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{
		"-config", configPath,
		"-manifest", manifestPath,
		"-preflight-report", preflightPath,
		"-source-revision", strings.Repeat("b", 40),
	}, &stdout, &stderr, func() time.Time { return time.Now().UTC() })
	if code != 1 {
		t.Fatalf("run code=%d, want 1", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("parity mismatch unexpectedly emitted acceptance evidence: %s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "parity rejected") {
		t.Fatalf("stderr=%q", stderr.String())
	}
}

func TestRunRejectsPreflightSourceDrift(t *testing.T) {
	cfg := migrationVerifierTestConfig()
	fingerprint, err := config.ConfigParityFingerprint(cfg)
	if err != nil {
		t.Fatal(err)
	}
	configPath := writeJSONFile(t, "gateway.json", cfg)
	manifestPath := writeJSONFile(t, "migration-source.json", migrationVerifierManifest(cfg, fingerprint))
	preflightPath := writePreflightFile(t, strings.Repeat("e", 64))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{
		"-config", configPath,
		"-manifest", manifestPath,
		"-preflight-report", preflightPath,
		"-source-revision", strings.Repeat("a", 40),
	}, &stdout, &stderr, time.Now)
	if code != 1 || stdout.Len() != 0 {
		t.Fatalf("stale Caddy identity accepted: code=%d stdout=%q", code, stdout.String())
	}
	if !strings.Contains(stderr.String(), "preflight source rejected") {
		t.Fatalf("stderr=%q", stderr.String())
	}
}

func TestRunRequiresExplicitInputs(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := run(nil, &stdout, &stderr, time.Now); code != 2 {
		t.Fatalf("run code=%d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("missing inputs unexpectedly emitted output: %s", stdout.String())
	}
}

func migrationVerifierTestConfig() *config.Config {
	return &config.Config{
		Schema: "goreecloud-gateway-config/v1",
		Services: []config.Service{
			{ID: "svc", Name: "service", BackendIDs: []string{"backend"}},
		},
		Routes: []config.Route{
			{
				ID:         "route",
				ServiceID:  "svc",
				Hostname:   "gateway.internal",
				PathPrefix: "/",
				Methods:    []string{"GET"},
				Exposure:   "private",
				Enabled:    true,
				TLS:        config.RouteTLS{Mode: "disabled"},
			},
		},
		Backends: []config.Backend{
			{ID: "backend", URL: "http://127.0.0.1:19090", Enabled: true, HealthPath: "/healthz"},
		},
	}
}

func migrationVerifierManifest(cfg *config.Config, fingerprint string) config.MigrationSourceManifest {
	return config.MigrationSourceManifest{
		Schema:                      config.MigrationSourceManifestSchemaV2,
		RecordedAt:                  "2026-09-04T15:55:00Z",
		SourceSystem:                "caddy",
		Environment:                 "target-review",
		SourceConfigSHA256:          strings.Repeat("d", 64),
		ExpectedGatewayConfigSHA256: fingerprint,
		ServiceCount:                len(cfg.Services),
		RouteCount:                  len(cfg.Routes),
		BackendCount:                len(cfg.Backends),
		CertificateProfileCount:     len(cfg.CertificateProfiles),
		ReviewEvidenceSHA256:        strings.Repeat("c", 64),
		ProductionCutoverAuthorized: false,
	}
}

func writeJSONFile(t *testing.T, name string, value any) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func writePreflightFile(t *testing.T, sourceSHA string) string {
	t.Helper()
	report := "GoreeCloud Gateway Caddy VPS read-only preflight\n" +
		"mode=read-only\nmutation_permitted=no\nsecret_values_printed=no\nlive_provider_query_performed=no\n" +
		"config_sha256=" + sourceSHA + "\nadapted_config_status=parsed\n" +
		"=== Safety Summary ===\n" +
		"mode=read-only\nmutation_permitted=no\nsecret_values_printed=no\nlive_provider_query_performed=no\n" +
		"caddy_production_authority_unchanged=yes\n"
	path := filepath.Join(t.TempDir(), "read-only-caddy-preflight.txt")
	if err := os.WriteFile(path, []byte(report), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

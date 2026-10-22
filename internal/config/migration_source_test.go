package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBuildConfigParityEvidenceFromMigrationSource(t *testing.T) {
	cfg := testParityConfig()
	fingerprint, err := ConfigParityFingerprint(cfg)
	if err != nil {
		t.Fatal(err)
	}
	manifest := testMigrationSourceManifest(cfg, fingerprint)
	evidence, err := BuildConfigParityEvidenceFromMigrationSource(
		cfg,
		strings.Repeat("a", 40),
		manifest,
		time.Date(2026, 8, 30, 13, 45, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !evidence.ConfigurationParityMatched {
		t.Fatal("reviewed migration source did not produce parity match")
	}
	if evidence.ProductionCutoverAuthorized {
		t.Fatal("migration-source parity unexpectedly authorized production cutover")
	}
	if evidence.MigrationSourceConfigSHA256 != manifest.SourceConfigSHA256 || evidence.MigrationReviewEvidenceSHA256 != manifest.ReviewEvidenceSHA256 {
		t.Fatal("migration evidence lost reviewed source or review identity")
	}
}

func TestBuildConfigParityEvidenceFromMigrationSourceRejectsCountMismatch(t *testing.T) {
	cfg := testParityConfig()
	fingerprint, err := ConfigParityFingerprint(cfg)
	if err != nil {
		t.Fatal(err)
	}
	manifest := testMigrationSourceManifest(cfg, fingerprint)
	manifest.RouteCount++
	if _, err := BuildConfigParityEvidenceFromMigrationSource(cfg, strings.Repeat("b", 40), manifest, time.Now().UTC()); err == nil {
		t.Fatal("migration-source aggregate mismatch unexpectedly accepted")
	}
}

func TestValidateMigrationSourceManifestRejectsWrongSystemAndCutover(t *testing.T) {
	cfg := testParityConfig()
	fingerprint, err := ConfigParityFingerprint(cfg)
	if err != nil {
		t.Fatal(err)
	}
	manifest := testMigrationSourceManifest(cfg, fingerprint)
	manifest.SourceSystem = "other-proxy"
	if err := ValidateMigrationSourceManifest(manifest); err == nil {
		t.Fatal("unexpected migration source system accepted")
	}
	manifest.SourceSystem = "caddy"
	manifest.ProductionCutoverAuthorized = true
	if err := ValidateMigrationSourceManifest(manifest); err == nil {
		t.Fatal("cutover-authorizing migration source manifest accepted")
	}
}

func TestValidateMigrationSourceManifestRejectsMissingOrConflatedSourceIdentity(t *testing.T) {
	cfg := testParityConfig()
	fingerprint, err := ConfigParityFingerprint(cfg)
	if err != nil {
		t.Fatal(err)
	}
	manifest := testMigrationSourceManifest(cfg, fingerprint)
	manifest.SourceConfigSHA256 = ""
	if err := ValidateMigrationSourceManifest(manifest); err == nil {
		t.Fatal("missing Caddy source identity accepted")
	}
	manifest.SourceConfigSHA256 = fingerprint
	if err := ValidateMigrationSourceManifest(manifest); err == nil {
		t.Fatal("source Caddy identity conflated with expected Gateway fingerprint")
	}
	manifest.SourceConfigSHA256 = strings.Repeat("d", 64)
	manifest.ExpectedGatewayConfigSHA256 = "invalid"
	if err := ValidateMigrationSourceManifest(manifest); err == nil {
		t.Fatal("invalid expected Gateway fingerprint accepted")
	}
	manifest.ExpectedGatewayConfigSHA256 = fingerprint
	manifest.Schema = "goreecloud-gateway-migration-source-manifest/v1"
	if err := ValidateMigrationSourceManifest(manifest); err == nil {
		t.Fatal("legacy source manifest without an explicit Caddy/Gateway identity distinction accepted")
	}
}

func TestLoadMigrationSourceManifestStrictJSON(t *testing.T) {
	cfg := testParityConfig()
	fingerprint, err := ConfigParityFingerprint(cfg)
	if err != nil {
		t.Fatal(err)
	}
	manifest := testMigrationSourceManifest(cfg, fingerprint)
	encoded, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "migration-source.json")
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadMigrationSourceManifest(path); err != nil {
		t.Fatal(err)
	}

	unknown := append(encoded[:len(encoded)-1], []byte(`,"unexpected":true}`)...)
	if err := os.WriteFile(path, unknown, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadMigrationSourceManifest(path); err == nil {
		t.Fatal("unknown migration-source field unexpectedly accepted")
	}
}

func testMigrationSourceManifest(cfg *Config, fingerprint string) MigrationSourceManifest {
	return MigrationSourceManifest{
		Schema:                      MigrationSourceManifestSchemaV2,
		RecordedAt:                  "2026-08-30T13:40:00Z",
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

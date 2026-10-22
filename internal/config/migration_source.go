package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

const MigrationSourceManifestSchemaV2 = "goreecloud-gateway-migration-source-manifest/v2"

// MigrationSourceManifest is a minimized, independently reviewed identity for
// the configuration that Gateway is expected to replace. It intentionally
// excludes hostnames, backend URLs, certificate paths, credentials, and route
// contents; those remain in the reviewed migration-preparation material rather
// than crossing into retained acceptance evidence.
type MigrationSourceManifest struct {
	Schema                      string `json:"schema"`
	RecordedAt                  string `json:"recorded_at"`
	SourceSystem                string `json:"source_system"`
	Environment                 string `json:"environment"`
	SourceConfigSHA256          string `json:"source_config_sha256"`
	ExpectedGatewayConfigSHA256 string `json:"expected_gateway_config_sha256"`
	ServiceCount                int    `json:"service_count"`
	RouteCount                  int    `json:"route_count"`
	BackendCount                int    `json:"backend_count"`
	CertificateProfileCount     int    `json:"certificate_profile_count"`
	ReviewEvidenceSHA256        string `json:"review_evidence_sha256"`
	ProductionCutoverAuthorized bool   `json:"production_cutover_authorized"`
}

// LoadMigrationSourceManifest strictly decodes a retained migration-source
// manifest. Unknown fields and trailing JSON fail closed so reviewers know
// exactly which evidence contract was evaluated.
func LoadMigrationSourceManifest(path string) (MigrationSourceManifest, error) {
	var manifest MigrationSourceManifest
	data, err := os.ReadFile(strings.TrimSpace(path))
	if err != nil {
		return manifest, fmt.Errorf("gateway migration source: read manifest: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return MigrationSourceManifest{}, fmt.Errorf("gateway migration source: decode manifest: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return MigrationSourceManifest{}, errors.New("gateway migration source: manifest contains trailing JSON data")
		}
		return MigrationSourceManifest{}, fmt.Errorf("gateway migration source: decode trailing manifest data: %w", err)
	}
	if err := ValidateMigrationSourceManifest(manifest); err != nil {
		return MigrationSourceManifest{}, err
	}
	return manifest, nil
}

// ValidateMigrationSourceManifest verifies the minimized identity contract. The
// source system is deliberately fixed to Caddy for the current migration; a
// future source requires a new reviewed contract rather than silent reuse.
func ValidateMigrationSourceManifest(manifest MigrationSourceManifest) error {
	if manifest.Schema != MigrationSourceManifestSchemaV2 {
		return errors.New("gateway migration source: unsupported manifest schema")
	}
	if _, err := time.Parse(time.RFC3339Nano, manifest.RecordedAt); err != nil {
		return errors.New("gateway migration source: recorded_at is invalid")
	}
	if strings.ToLower(strings.TrimSpace(manifest.SourceSystem)) != "caddy" {
		return errors.New("gateway migration source: source_system must be caddy for this migration")
	}
	if strings.TrimSpace(manifest.Environment) == "" {
		return errors.New("gateway migration source: environment is required")
	}
	if !validSHA256(strings.ToLower(strings.TrimSpace(manifest.SourceConfigSHA256))) {
		return errors.New("gateway migration source: source Caddy configuration SHA-256 is invalid")
	}
	if !validSHA256(strings.ToLower(strings.TrimSpace(manifest.ExpectedGatewayConfigSHA256))) {
		return errors.New("gateway migration source: expected Gateway parity SHA-256 is invalid")
	}
	if strings.EqualFold(strings.TrimSpace(manifest.SourceConfigSHA256), strings.TrimSpace(manifest.ExpectedGatewayConfigSHA256)) {
		return errors.New("gateway migration source: Caddy source and expected Gateway fingerprints must be independently identified")
	}
	if !validSHA256(strings.ToLower(strings.TrimSpace(manifest.ReviewEvidenceSHA256))) {
		return errors.New("gateway migration source: review evidence SHA-256 is invalid")
	}
	if manifest.ServiceCount < 0 || manifest.RouteCount < 0 || manifest.BackendCount < 0 || manifest.CertificateProfileCount < 0 {
		return errors.New("gateway migration source: aggregate counts are invalid")
	}
	if manifest.ProductionCutoverAuthorized {
		return errors.New("gateway migration source: manifest cannot authorize production cutover")
	}
	return nil
}

// BuildConfigParityEvidenceFromMigrationSource binds Gateway parity evaluation
// to a validated, independently reviewed migration-source manifest. Aggregate
// counts must match as an additional fail-closed guard before fingerprint
// equality is evaluated. This function cannot authorize production cutover.
func BuildConfigParityEvidenceFromMigrationSource(
	cfg *Config,
	sourceRevision string,
	manifest MigrationSourceManifest,
	now time.Time,
) (ConfigParityEvidence, error) {
	if err := ValidateMigrationSourceManifest(manifest); err != nil {
		return ConfigParityEvidence{}, err
	}
	if cfg == nil {
		return ConfigParityEvidence{}, errors.New("gateway migration source: Gateway configuration is required")
	}
	if err := cfg.Validate(); err != nil {
		return ConfigParityEvidence{}, err
	}
	if len(cfg.Services) != manifest.ServiceCount ||
		len(cfg.Routes) != manifest.RouteCount ||
		len(cfg.Backends) != manifest.BackendCount ||
		len(cfg.CertificateProfiles) != manifest.CertificateProfileCount {
		return ConfigParityEvidence{}, errors.New("gateway migration source: aggregate configuration counts do not match reviewed source")
	}
	evidence, err := BuildConfigParityEvidence(cfg, sourceRevision, manifest.ExpectedGatewayConfigSHA256, now)
	if err != nil {
		return ConfigParityEvidence{}, err
	}
	evidence.MigrationSourceConfigSHA256 = strings.ToLower(strings.TrimSpace(manifest.SourceConfigSHA256))
	evidence.MigrationReviewEvidenceSHA256 = strings.ToLower(strings.TrimSpace(manifest.ReviewEvidenceSHA256))
	return evidence, nil
}

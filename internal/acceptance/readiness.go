// Package acceptance defines fail-closed migration-readiness contracts for
// GoreeCloud Gateway. It evaluates evidence only and never changes listeners,
// routes, certificates, authority, or production state.
package acceptance

import (
	"errors"
	"strings"
	"time"
)

// V2 replaces the retired administration-design gate with the current
// GoreeCloud UI integration gate. V1 evidence is deliberately not accepted.
const MigrationEvidenceSchemaV2 = "goreecloud-gateway-migration-evidence/v2"

// MigrationEvidence records bounded acceptance results for one exact Gateway
// source revision and immutable runtime artifact. It intentionally carries no
// routes, hostnames, backend URLs, certificate bytes, keys, credentials,
// request data, client identifiers, or raw diagnostics.
type MigrationEvidence struct {
	Schema                         string `json:"schema"`
	RecordedAt                     string `json:"recorded_at"`
	SourceRevision                 string `json:"source_revision"`
	RuntimeArtifactSHA256          string `json:"runtime_artifact_sha256"`
	ConfigurationValidated         bool   `json:"configuration_validated"`
	RouteParityValidated           bool   `json:"route_parity_validated"`
	TLSRenewalRehearsalPassed      bool   `json:"tls_renewal_rehearsal_passed"`
	StreamingUpgradeValidated      bool   `json:"streaming_upgrade_validated"`
	SustainedLoadValidated         bool   `json:"sustained_load_validated"`
	BackpressureValidated          bool   `json:"backpressure_validated"`
	BackupRestoreProven            bool   `json:"backup_restore_proven"`
	RollbackRehearsed              bool   `json:"rollback_rehearsed"`
	ListenerOwnershipValidated     bool   `json:"listener_ownership_validated"`
	ObservabilityValidated         bool   `json:"observability_validated"`
	ManagerIntegrationValidated    bool   `json:"manager_integration_validated"`
	PrivacyShieldValidated         bool   `json:"privacy_shield_validated"`
	WardveilSecurityValidated      bool   `json:"wardveil_security_validated"`
	EverkeepValidated              bool   `json:"everkeep_validated"`
	GoreeCloudUIValidated          bool   `json:"goreecloud_ui_validated"`
	MeshCoordinationValidated      bool   `json:"mesh_coordination_validated"`
	IdentityIntegrationValidated   bool   `json:"identity_integration_validated"`
	GovernanceIntegrationValidated bool   `json:"governance_integration_validated"`
	ProductionCutoverAuthorized    bool   `json:"production_cutover_authorized"`
}

// MigrationDecision reports whether evidence is complete enough to enter an
// explicitly approved production migration rehearsal. It never authorizes the
// production cutover itself.
type MigrationDecision struct {
	EligibleForMigrationRehearsal bool     `json:"eligible_for_migration_rehearsal"`
	MissingGates                  []string `json:"missing_gates,omitempty"`
	ProductionCutoverAuthorized   bool     `json:"production_cutover_authorized"`
}

// EvaluateMigrationReadiness validates the evidence identity and enumerates
// every incomplete migration gate. ProductionCutoverAuthorized is always false.
func EvaluateMigrationReadiness(evidence MigrationEvidence) (MigrationDecision, error) {
	decision := MigrationDecision{ProductionCutoverAuthorized: false}
	if evidence.Schema != MigrationEvidenceSchemaV2 {
		return decision, errors.New("gateway acceptance: unsupported migration evidence schema")
	}
	if _, err := time.Parse(time.RFC3339Nano, evidence.RecordedAt); err != nil {
		return decision, errors.New("gateway acceptance: recorded_at is invalid")
	}
	if !validHexIdentity(evidence.SourceRevision, 40, 64) {
		return decision, errors.New("gateway acceptance: source revision is invalid")
	}
	if !validHexIdentity(evidence.RuntimeArtifactSHA256, 64) {
		return decision, errors.New("gateway acceptance: runtime artifact digest is invalid")
	}
	if evidence.ProductionCutoverAuthorized {
		return decision, errors.New("gateway acceptance: evidence cannot authorize production cutover")
	}

	gates := []struct {
		name string
		pass bool
	}{
		{"configuration_validated", evidence.ConfigurationValidated},
		{"route_parity_validated", evidence.RouteParityValidated},
		{"tls_renewal_rehearsal_passed", evidence.TLSRenewalRehearsalPassed},
		{"streaming_upgrade_validated", evidence.StreamingUpgradeValidated},
		{"sustained_load_validated", evidence.SustainedLoadValidated},
		{"backpressure_validated", evidence.BackpressureValidated},
		{"backup_restore_proven", evidence.BackupRestoreProven},
		{"rollback_rehearsed", evidence.RollbackRehearsed},
		{"listener_ownership_validated", evidence.ListenerOwnershipValidated},
		{"observability_validated", evidence.ObservabilityValidated},
		{"manager_integration_validated", evidence.ManagerIntegrationValidated},
		{"privacy_shield_validated", evidence.PrivacyShieldValidated},
		{"wardveil_security_validated", evidence.WardveilSecurityValidated},
		{"everkeep_validated", evidence.EverkeepValidated},
		{"goreecloud_ui_validated", evidence.GoreeCloudUIValidated},
		{"mesh_coordination_validated", evidence.MeshCoordinationValidated},
		{"identity_integration_validated", evidence.IdentityIntegrationValidated},
		{"governance_integration_validated", evidence.GovernanceIntegrationValidated},
	}
	for _, gate := range gates {
		if !gate.pass {
			decision.MissingGates = append(decision.MissingGates, gate.name)
		}
	}
	decision.EligibleForMigrationRehearsal = len(decision.MissingGates) == 0
	return decision, nil
}

func validHexIdentity(value string, lengths ...int) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	for _, r := range value {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	for _, length := range lengths {
		if len(value) == length {
			return true
		}
	}
	return false
}

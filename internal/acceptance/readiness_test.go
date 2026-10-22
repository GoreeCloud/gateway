package acceptance

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestEvaluateMigrationReadinessRequiresEveryGate(t *testing.T) {
	evidence := completeMigrationEvidence()
	evidence.BackupRestoreProven = false
	evidence.ManagerIntegrationValidated = false
	evidence.WardveilSecurityValidated = false
	evidence.MeshCoordinationValidated = false
	evidence.IdentityIntegrationValidated = false
	evidence.GovernanceIntegrationValidated = false

	decision, err := EvaluateMigrationReadiness(evidence)
	if err != nil {
		t.Fatal(err)
	}
	if decision.EligibleForMigrationRehearsal {
		t.Fatal("incomplete evidence unexpectedly eligible for migration rehearsal")
	}
	if decision.ProductionCutoverAuthorized {
		t.Fatal("readiness decision unexpectedly authorized production cutover")
	}
	want := []string{
		"backup_restore_proven",
		"manager_integration_validated",
		"wardveil_security_validated",
		"mesh_coordination_validated",
		"identity_integration_validated",
		"governance_integration_validated",
	}
	if len(decision.MissingGates) != len(want) {
		t.Fatalf("missing gates = %v, want %v", decision.MissingGates, want)
	}
	for i := range want {
		if decision.MissingGates[i] != want[i] {
			t.Fatalf("missing gates = %v, want %v", decision.MissingGates, want)
		}
	}
}

func TestEvaluateMigrationReadinessAcceptsCompleteBoundedEvidence(t *testing.T) {
	decision, err := EvaluateMigrationReadiness(completeMigrationEvidence())
	if err != nil {
		t.Fatal(err)
	}
	if !decision.EligibleForMigrationRehearsal || len(decision.MissingGates) != 0 {
		t.Fatalf("unexpected decision: %+v", decision)
	}
	if decision.ProductionCutoverAuthorized {
		t.Fatal("complete source evidence unexpectedly authorized production cutover")
	}
}

func TestEvaluateMigrationReadinessRejectsCutoverClaim(t *testing.T) {
	evidence := completeMigrationEvidence()
	evidence.ProductionCutoverAuthorized = true
	if _, err := EvaluateMigrationReadiness(evidence); err == nil {
		t.Fatal("cutover-authorizing evidence unexpectedly accepted")
	}
}

func TestEvaluateMigrationReadinessRejectsInvalidIdentity(t *testing.T) {
	evidence := completeMigrationEvidence()
	evidence.RuntimeArtifactSHA256 = strings.Repeat("z", 64)
	if _, err := EvaluateMigrationReadiness(evidence); err == nil {
		t.Fatal("invalid runtime artifact digest unexpectedly accepted")
	}
}

func TestEvaluateMigrationReadinessRejectsPreviousEvidenceSchema(t *testing.T) {
	evidence := completeMigrationEvidence()
	evidence.Schema = "goreecloud-gateway-migration-evidence/v1"
	if _, err := EvaluateMigrationReadiness(evidence); err == nil {
		t.Fatal("superseded evidence schema unexpectedly accepted")
	}
}

func TestEvaluateMigrationReadinessRequiresGoreeCloudUIAcceptance(t *testing.T) {
	evidence := completeMigrationEvidence()
	evidence.GCUIValidated = false
	decision, err := EvaluateMigrationReadiness(evidence)
	if err != nil {
		t.Fatal(err)
	}
	if decision.EligibleForMigrationRehearsal || len(decision.MissingGates) != 1 || decision.MissingGates[0] != "gc_ui_validated" {
		t.Fatalf("incomplete UI integration unexpectedly accepted: %+v", decision)
	}
}

func TestEvaluateMigrationReadinessDoesNotCountUnknownJSONFields(t *testing.T) {
	source := completeMigrationEvidence()
	source.GCUIValidated = false
	encoded, err := json.Marshal(source)
	if err != nil {
		t.Fatal(err)
	}
	encoded = append(encoded[:len(encoded)-1], []byte(`,"obsolete_ui_acceptance":true}`)...)
	var evidence MigrationEvidence
	if err := json.Unmarshal(encoded, &evidence); err != nil {
		t.Fatal(err)
	}
	// The current gate must still be proved explicitly, even if another
	// input field asserts an unrelated UI acceptance state.
	decision, err := EvaluateMigrationReadiness(evidence)
	if err != nil || decision.EligibleForMigrationRehearsal || len(decision.MissingGates) != 1 || decision.MissingGates[0] != "gc_ui_validated" {
		t.Fatalf("missing UI integration gate unexpectedly accepted: decision=%+v err=%v", decision, err)
	}
}

func completeMigrationEvidence() MigrationEvidence {
	return MigrationEvidence{
		Schema:                         MigrationEvidenceSchemaV2,
		RecordedAt:                     time.Date(2026, 8, 29, 4, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
		SourceRevision:                 strings.Repeat("a", 40),
		RuntimeArtifactSHA256:          strings.Repeat("b", 64),
		ConfigurationValidated:         true,
		RouteParityValidated:          true,
		TLSRenewalRehearsalPassed:     true,
		StreamingUpgradeValidated:     true,
		SustainedLoadValidated:        true,
		BackpressureValidated:         true,
		BackupRestoreProven:           true,
		RollbackRehearsed:             true,
		ListenerOwnershipValidated:    true,
		ObservabilityValidated:        true,
		ManagerIntegrationValidated:   true,
		PrivacyShieldValidated:        true,
		WardveilSecurityValidated:     true,
		EverkeepValidated:             true,
		GCUIValidated:                 true,
		MeshCoordinationValidated:     true,
		IdentityIntegrationValidated:  true,
		GovernanceIntegrationValidated: true,
		ProductionCutoverAuthorized:   false,
	}
}

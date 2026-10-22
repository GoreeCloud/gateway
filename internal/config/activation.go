package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	StagedRevisionSchemaV1    = "goreecloud-gateway-config-staged-revision/v1"
	ActivationReceiptSchemaV1 = "goreecloud-gateway-config-activation-receipt/v1"
)

type StagedRevision struct {
	Schema                      string `json:"schema"`
	StagedAt                    string `json:"staged_at"`
	SourceRevision              string `json:"source_revision"`
	ConfigSHA256                string `json:"config_sha256"`
	ConfigFile                  string `json:"config_file"`
	ProductionCutoverAuthorized bool   `json:"production_cutover_authorized"`
}

type ActivationReceipt struct {
	Schema                      string `json:"schema"`
	ActivatedAt                 string `json:"activated_at"`
	SourceRevision              string `json:"source_revision"`
	PreviousSourceRevision      string `json:"previous_source_revision"`
	PreviousConfigSHA256        string `json:"previous_config_sha256"`
	ActivatedConfigSHA256       string `json:"activated_config_sha256"`
	RecoverySnapshot            string `json:"recovery_snapshot"`
	ConfigValidated             bool   `json:"config_validated"`
	CompareAndSwapValidated     bool   `json:"compare_and_swap_validated"`
	ProductionCutoverAuthorized bool   `json:"production_cutover_authorized"`
}

// StageRevision validates a complete Gateway configuration before persisting an
// immutable, digest-bound staged revision below lifecycleRoot. Staging does not
// alter the active configuration and never authorizes production cutover.
func StageRevision(lifecycleRoot string, candidate []byte, sourceRevision string, now time.Time) (string, StagedRevision, error) {
	var staged StagedRevision
	root, err := validateLifecycleRoot(lifecycleRoot)
	if err != nil {
		return "", staged, err
	}
	sourceRevision = strings.ToLower(strings.TrimSpace(sourceRevision))
	if !recoverySourceRevision.MatchString(sourceRevision) {
		return "", staged, errors.New("gateway activation: exact 40-character lowercase source revision is required")
	}
	if now.IsZero() {
		return "", staged, errors.New("gateway activation: staging time is required")
	}
	if err := validateConfigBytes(candidate); err != nil {
		return "", staged, fmt.Errorf("gateway activation: candidate configuration is not valid: %w", err)
	}

	digest := sha256Hex(candidate)
	stagingRoot := filepath.Join(root, "staged")
	if err := os.MkdirAll(stagingRoot, 0o700); err != nil {
		return "", staged, fmt.Errorf("gateway activation: create staging directory: %w", err)
	}
	if err := syncDirectory(root); err != nil {
		return "", staged, err
	}
	if err := rejectSymlinkPath(root, stagingRoot); err != nil {
		return "", staged, err
	}
	stageDir := filepath.Join(stagingRoot, now.UTC().Format("20060102T150405.000000000Z")+"-"+digest[:12])
	if err := os.Mkdir(stageDir, 0o700); err != nil {
		return "", staged, fmt.Errorf("gateway activation: create staged revision directory: %w", err)
	}
	if err := syncDirectory(stagingRoot); err != nil {
		return "", staged, err
	}

	const configName = "gateway.json"
	if err := atomicWrite(filepath.Join(stageDir, configName), candidate, 0o600); err != nil {
		return "", staged, err
	}
	staged = StagedRevision{
		Schema:                      StagedRevisionSchemaV1,
		StagedAt:                    now.UTC().Format(time.RFC3339Nano),
		SourceRevision:              sourceRevision,
		ConfigSHA256:                digest,
		ConfigFile:                  configName,
		ProductionCutoverAuthorized: false,
	}
	manifest, err := json.MarshalIndent(staged, "", "  ")
	if err != nil {
		return "", StagedRevision{}, fmt.Errorf("gateway activation: encode staged revision manifest: %w", err)
	}
	if err := atomicWrite(filepath.Join(stageDir, "manifest.json"), append(manifest, '\n'), 0o600); err != nil {
		return "", StagedRevision{}, err
	}
	return stageDir, staged, nil
}

// ActivateStagedRevision atomically replaces activeConfigPath with one validated
// staged revision only if the active configuration still has the expected
// digest. A recovery snapshot of the previous known-good configuration is
// created before replacement. Runtime reload remains a separate acceptance
// boundary; this function only commits the canonical configuration file.
func ActivateStagedRevision(
	lifecycleRoot,
	activeConfigPath,
	stageDir,
	expectedCurrentSHA256,
	currentSourceRevision string,
	now time.Time,
) (ActivationReceipt, error) {
	root, _, err := validateRecoveryPaths(lifecycleRoot, activeConfigPath)
	if err != nil {
		return ActivationReceipt{}, err
	}
	var receipt ActivationReceipt
	err = withRecoveryLock(root, func() error {
		var lockedErr error
		receipt, lockedErr = activateStagedRevisionUnlocked(lifecycleRoot, activeConfigPath, stageDir, expectedCurrentSHA256, currentSourceRevision, now)
		return lockedErr
	})
	return receipt, err
}

// Called with recoveryRoot's process-shared lock held, including through
// snapshot creation, validation and the compare-and-swap replacement.
func activateStagedRevisionUnlocked(
	lifecycleRoot,
	activeConfigPath,
	stageDir,
	expectedCurrentSHA256,
	currentSourceRevision string,
	now time.Time,
) (ActivationReceipt, error) {
	var receipt ActivationReceipt
	root, active, err := validateRecoveryPaths(lifecycleRoot, activeConfigPath)
	if err != nil {
		return receipt, err
	}
	if now.IsZero() {
		return receipt, errors.New("gateway activation: activation time is required")
	}
	currentSourceRevision = strings.ToLower(strings.TrimSpace(currentSourceRevision))
	if !recoverySourceRevision.MatchString(currentSourceRevision) {
		return receipt, errors.New("gateway activation: exact previous 40-character lowercase source revision is required")
	}

	stageDir, err = filepath.Abs(strings.TrimSpace(stageDir))
	if err != nil || strings.TrimSpace(stageDir) == "" {
		return receipt, errors.New("gateway activation: staged revision path is required")
	}
	if err := requireContainedPath(root, stageDir); err != nil {
		return receipt, err
	}
	if err := rejectSymlinkPath(root, stageDir); err != nil {
		return receipt, err
	}

	manifestBytes, err := readRegularNoSymlink(filepath.Join(stageDir, "manifest.json"))
	if err != nil {
		return receipt, err
	}
	var staged StagedRevision
	if err := json.Unmarshal(manifestBytes, &staged); err != nil {
		return receipt, fmt.Errorf("gateway activation: decode staged revision manifest: %w", err)
	}
	if err := validateStagedRevision(staged); err != nil {
		return receipt, err
	}
	stagedConfigPath := filepath.Join(stageDir, staged.ConfigFile)
	if err := requireContainedPath(stageDir, stagedConfigPath); err != nil {
		return receipt, err
	}
	stagedBytes, err := readRegularNoSymlink(stagedConfigPath)
	if err != nil {
		return receipt, err
	}
	if actual := sha256Hex(stagedBytes); actual != staged.ConfigSHA256 {
		return receipt, errors.New("gateway activation: staged configuration digest mismatch")
	}
	if err := validateConfigBytes(stagedBytes); err != nil {
		return receipt, fmt.Errorf("gateway activation: staged configuration is not valid: %w", err)
	}

	currentBytes, err := readRegularNoSymlink(active)
	if err != nil {
		return receipt, err
	}
	if err := validateConfigBytes(currentBytes); err != nil {
		return receipt, fmt.Errorf("gateway activation: active configuration is not valid: %w", err)
	}
	currentDigest := sha256Hex(currentBytes)
	expectedCurrentSHA256 = strings.ToLower(strings.TrimSpace(expectedCurrentSHA256))
	if !validSHA256(expectedCurrentSHA256) {
		return receipt, errors.New("gateway activation: expected current configuration SHA-256 is required")
	}
	if currentDigest != expectedCurrentSHA256 {
		return receipt, errors.New("gateway activation: active configuration changed after activation was planned")
	}
	if currentDigest == staged.ConfigSHA256 {
		return receipt, errors.New("gateway activation: staged configuration is already active")
	}

	snapshotDir, _, err := createRecoverySnapshotUnlocked(root, active, currentSourceRevision, now)
	if err != nil {
		return receipt, fmt.Errorf("gateway activation: create previous-known-good snapshot: %w", err)
	}
	if err := atomicWrite(active, stagedBytes, 0o600); err != nil {
		return receipt, err
	}
	if _, err := Load(active); err != nil {
		_ = atomicWrite(active, currentBytes, 0o600)
		return receipt, fmt.Errorf("gateway activation: activated configuration failed validation: %w", err)
	}
	activatedBytes, err := readRegularNoSymlink(active)
	if err != nil {
		_ = atomicWrite(active, currentBytes, 0o600)
		return receipt, err
	}
	activatedDigest := sha256Hex(activatedBytes)
	if activatedDigest != staged.ConfigSHA256 {
		_ = atomicWrite(active, currentBytes, 0o600)
		return receipt, errors.New("gateway activation: activated configuration digest mismatch")
	}

	relativeSnapshot, err := filepath.Rel(root, snapshotDir)
	if err != nil {
		_ = atomicWrite(active, currentBytes, 0o600)
		return receipt, fmt.Errorf("gateway activation: resolve recovery snapshot receipt path: %w", err)
	}
	receipt = ActivationReceipt{
		Schema:                      ActivationReceiptSchemaV1,
		ActivatedAt:                 now.UTC().Format(time.RFC3339Nano),
		SourceRevision:              staged.SourceRevision,
		PreviousSourceRevision:      currentSourceRevision,
		PreviousConfigSHA256:        currentDigest,
		ActivatedConfigSHA256:       activatedDigest,
		RecoverySnapshot:            filepath.ToSlash(relativeSnapshot),
		ConfigValidated:             true,
		CompareAndSwapValidated:     true,
		ProductionCutoverAuthorized: false,
	}
	return receipt, nil
}

func validateStagedRevision(staged StagedRevision) error {
	if staged.Schema != StagedRevisionSchemaV1 {
		return errors.New("gateway activation: unsupported staged revision schema")
	}
	if _, err := time.Parse(time.RFC3339Nano, staged.StagedAt); err != nil {
		return errors.New("gateway activation: staged revision timestamp is invalid")
	}
	if !recoverySourceRevision.MatchString(staged.SourceRevision) {
		return errors.New("gateway activation: staged source revision is invalid")
	}
	if !validSHA256(staged.ConfigSHA256) {
		return errors.New("gateway activation: staged configuration SHA-256 is invalid")
	}
	if staged.ConfigFile != "gateway.json" {
		return errors.New("gateway activation: staged configuration filename is invalid")
	}
	if staged.ProductionCutoverAuthorized {
		return errors.New("gateway activation: staged revision cannot authorize production cutover")
	}
	return nil
}

func validateConfigBytes(data []byte) error {
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return err
	}
	return cfg.Validate()
}

func validateLifecycleRoot(lifecycleRoot string) (string, error) {
	root, err := filepath.Abs(strings.TrimSpace(lifecycleRoot))
	if err != nil || strings.TrimSpace(lifecycleRoot) == "" {
		return "", errors.New("gateway activation: lifecycle root is required")
	}
	info, err := os.Lstat(root)
	if err != nil {
		return "", fmt.Errorf("gateway activation: inspect lifecycle root: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("gateway activation: lifecycle root must be a real directory")
	}
	return root, nil
}

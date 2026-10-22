package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	RecoverySnapshotSchemaV1 = "goreecloud-gateway-config-recovery-snapshot/v1"
	RecoveryReceiptSchemaV1  = "goreecloud-gateway-config-recovery-receipt/v1"
)

var recoverySourceRevision = regexp.MustCompile(`^[0-9a-f]{40}$`)

type RecoverySnapshot struct {
	Schema                      string `json:"schema"`
	CreatedAt                   string `json:"created_at"`
	SourceRevision              string `json:"source_revision"`
	ConfigSHA256                string `json:"config_sha256"`
	ConfigFile                  string `json:"config_file"`
	ProductionCutoverAuthorized bool   `json:"production_cutover_authorized"`
}

type RecoveryReceipt struct {
	Schema                      string `json:"schema"`
	RestoredAt                  string `json:"restored_at"`
	SourceRevision              string `json:"source_revision"`
	PreviousConfigSHA256        string `json:"previous_config_sha256"`
	RestoredConfigSHA256        string `json:"restored_config_sha256"`
	SnapshotConfigSHA256        string `json:"snapshot_config_sha256"`
	ConfigValidated             bool   `json:"config_validated"`
	CompareAndSwapValidated     bool   `json:"compare_and_swap_validated"`
	ProductionCutoverAuthorized bool   `json:"production_cutover_authorized"`
}

// CreateRecoverySnapshot copies one validated Gateway configuration into an
// owner-only recovery snapshot below recoveryRoot. Both the active configuration
// and the snapshot remain path-bounded to recoveryRoot so a rehearsal cannot
// accidentally capture arbitrary host files.
func CreateRecoverySnapshot(recoveryRoot, activeConfigPath, sourceRevision string, now time.Time) (string, RecoverySnapshot, error) {
	root, _, err := validateRecoveryPaths(recoveryRoot, activeConfigPath)
	if err != nil {
		return "", RecoverySnapshot{}, err
	}
	var dir string
	var snapshot RecoverySnapshot
	err = withRecoveryLock(root, func() error {
		var lockedErr error
		dir, snapshot, lockedErr = createRecoverySnapshotUnlocked(recoveryRoot, activeConfigPath, sourceRevision, now)
		return lockedErr
	})
	return dir, snapshot, err
}

// Call only while holding recoveryRoot's process-shared recovery lock. The
// activation path must not recursively acquire the same advisory lock.
func createRecoverySnapshotUnlocked(recoveryRoot, activeConfigPath, sourceRevision string, now time.Time) (string, RecoverySnapshot, error) {
	var snapshot RecoverySnapshot
	root, active, err := validateRecoveryPaths(recoveryRoot, activeConfigPath)
	if err != nil {
		return "", snapshot, err
	}
	sourceRevision = strings.ToLower(strings.TrimSpace(sourceRevision))
	if !recoverySourceRevision.MatchString(sourceRevision) {
		return "", snapshot, errors.New("gateway recovery: exact 40-character lowercase source revision is required")
	}
	if now.IsZero() {
		return "", snapshot, errors.New("gateway recovery: snapshot time is required")
	}
	if _, err := Load(active); err != nil {
		return "", snapshot, fmt.Errorf("gateway recovery: active configuration is not valid: %w", err)
	}
	activeBytes, err := readRegularNoSymlink(active)
	if err != nil {
		return "", snapshot, err
	}
	configDigest := sha256Hex(activeBytes)

	snapshotsRoot := filepath.Join(root, "snapshots")
	if err := os.MkdirAll(snapshotsRoot, 0o700); err != nil {
		return "", snapshot, fmt.Errorf("gateway recovery: create snapshots directory: %w", err)
	}
	if err := syncDirectory(root); err != nil {
		return "", snapshot, err
	}
	if err := rejectSymlinkPath(root, snapshotsRoot); err != nil {
		return "", snapshot, err
	}
	snapshotDir := filepath.Join(
		snapshotsRoot,
		now.UTC().Format("20060102T150405.000000000Z")+"-"+configDigest[:12],
	)
	if err := os.Mkdir(snapshotDir, 0o700); err != nil {
		return "", snapshot, fmt.Errorf("gateway recovery: create snapshot directory: %w", err)
	}
	if err := syncDirectory(snapshotsRoot); err != nil {
		return "", snapshot, err
	}

	configName := "gateway.json"
	if err := atomicWrite(filepath.Join(snapshotDir, configName), activeBytes, 0o600); err != nil {
		return "", snapshot, err
	}
	snapshot = RecoverySnapshot{
		Schema:                      RecoverySnapshotSchemaV1,
		CreatedAt:                   now.UTC().Format(time.RFC3339Nano),
		SourceRevision:              sourceRevision,
		ConfigSHA256:                configDigest,
		ConfigFile:                  configName,
		ProductionCutoverAuthorized: false,
	}
	manifest, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return "", RecoverySnapshot{}, fmt.Errorf("gateway recovery: encode snapshot manifest: %w", err)
	}
	if err := atomicWrite(filepath.Join(snapshotDir, "manifest.json"), append(manifest, '\n'), 0o600); err != nil {
		return "", RecoverySnapshot{}, err
	}
	return snapshotDir, snapshot, nil
}

// RestoreRecoverySnapshot restores a validated configuration snapshot only if
// the active configuration still matches expectedCurrentSHA256. This
// compare-and-swap boundary prevents rollback from overwriting a newer state.
func RestoreRecoverySnapshot(
	recoveryRoot,
	activeConfigPath,
	snapshotDir,
	expectedCurrentSHA256 string,
	now time.Time,
) (RecoveryReceipt, error) {
	root, _, err := validateRecoveryPaths(recoveryRoot, activeConfigPath)
	if err != nil {
		return RecoveryReceipt{}, err
	}
	var receipt RecoveryReceipt
	err = withRecoveryLock(root, func() error {
		var lockedErr error
		receipt, lockedErr = restoreRecoverySnapshotUnlocked(recoveryRoot, activeConfigPath, snapshotDir, expectedCurrentSHA256, now)
		return lockedErr
	})
	return receipt, err
}

// This digest check and replacement must share the same recovery lock with
// activation and snapshot creation to serialize cooperating processes.
func restoreRecoverySnapshotUnlocked(
	recoveryRoot,
	activeConfigPath,
	snapshotDir,
	expectedCurrentSHA256 string,
	now time.Time,
) (RecoveryReceipt, error) {
	var receipt RecoveryReceipt
	root, active, err := validateRecoveryPaths(recoveryRoot, activeConfigPath)
	if err != nil {
		return receipt, err
	}
	if now.IsZero() {
		return receipt, errors.New("gateway recovery: restore time is required")
	}
	snapshotDir, err = filepath.Abs(strings.TrimSpace(snapshotDir))
	if err != nil {
		return receipt, fmt.Errorf("gateway recovery: resolve snapshot path: %w", err)
	}
	if err := requireContainedPath(root, snapshotDir); err != nil {
		return receipt, err
	}
	if err := rejectSymlinkPath(root, snapshotDir); err != nil {
		return receipt, err
	}

	manifestBytes, err := readRegularNoSymlink(filepath.Join(snapshotDir, "manifest.json"))
	if err != nil {
		return receipt, err
	}
	var snapshot RecoverySnapshot
	if err := json.Unmarshal(manifestBytes, &snapshot); err != nil {
		return receipt, fmt.Errorf("gateway recovery: decode snapshot manifest: %w", err)
	}
	if err := validateRecoverySnapshot(snapshot); err != nil {
		return receipt, err
	}
	snapshotConfigPath := filepath.Join(snapshotDir, snapshot.ConfigFile)
	if err := requireContainedPath(snapshotDir, snapshotConfigPath); err != nil {
		return receipt, err
	}
	if _, err := Load(snapshotConfigPath); err != nil {
		return receipt, fmt.Errorf("gateway recovery: snapshot configuration is not valid: %w", err)
	}
	snapshotBytes, err := readRegularNoSymlink(snapshotConfigPath)
	if err != nil {
		return receipt, err
	}
	if actual := sha256Hex(snapshotBytes); actual != snapshot.ConfigSHA256 {
		return receipt, errors.New("gateway recovery: snapshot configuration digest mismatch")
	}

	currentBytes, err := readRegularNoSymlink(active)
	if err != nil {
		return receipt, err
	}
	currentDigest := sha256Hex(currentBytes)
	expectedCurrentSHA256 = strings.ToLower(strings.TrimSpace(expectedCurrentSHA256))
	if !validSHA256(expectedCurrentSHA256) {
		return receipt, errors.New("gateway recovery: expected current configuration SHA-256 is required")
	}
	if currentDigest != expectedCurrentSHA256 {
		return receipt, errors.New("gateway recovery: active configuration changed after rollback was planned")
	}

	if err := atomicWrite(active, snapshotBytes, 0o600); err != nil {
		return receipt, err
	}
	if _, err := Load(active); err != nil {
		_ = atomicWrite(active, currentBytes, 0o600)
		return receipt, fmt.Errorf("gateway recovery: restored configuration failed validation: %w", err)
	}
	restoredBytes, err := readRegularNoSymlink(active)
	if err != nil {
		_ = atomicWrite(active, currentBytes, 0o600)
		return receipt, err
	}
	restoredDigest := sha256Hex(restoredBytes)
	if restoredDigest != snapshot.ConfigSHA256 {
		_ = atomicWrite(active, currentBytes, 0o600)
		return receipt, errors.New("gateway recovery: restored configuration digest mismatch")
	}

	receipt = RecoveryReceipt{
		Schema:                      RecoveryReceiptSchemaV1,
		RestoredAt:                  now.UTC().Format(time.RFC3339Nano),
		SourceRevision:              snapshot.SourceRevision,
		PreviousConfigSHA256:        currentDigest,
		RestoredConfigSHA256:        restoredDigest,
		SnapshotConfigSHA256:        snapshot.ConfigSHA256,
		ConfigValidated:             true,
		CompareAndSwapValidated:     true,
		ProductionCutoverAuthorized: false,
	}
	return receipt, nil
}

func validateRecoverySnapshot(snapshot RecoverySnapshot) error {
	if snapshot.Schema != RecoverySnapshotSchemaV1 {
		return errors.New("gateway recovery: unsupported snapshot schema")
	}
	if _, err := time.Parse(time.RFC3339Nano, snapshot.CreatedAt); err != nil {
		return errors.New("gateway recovery: snapshot timestamp is invalid")
	}
	if !recoverySourceRevision.MatchString(snapshot.SourceRevision) {
		return errors.New("gateway recovery: snapshot source revision is invalid")
	}
	if !validSHA256(snapshot.ConfigSHA256) {
		return errors.New("gateway recovery: snapshot configuration SHA-256 is invalid")
	}
	if snapshot.ConfigFile != "gateway.json" {
		return errors.New("gateway recovery: snapshot configuration filename is invalid")
	}
	if snapshot.ProductionCutoverAuthorized {
		return errors.New("gateway recovery: snapshot cannot authorize production cutover")
	}
	return nil
}

func validateRecoveryPaths(recoveryRoot, activeConfigPath string) (string, string, error) {
	root, err := filepath.Abs(strings.TrimSpace(recoveryRoot))
	if err != nil || strings.TrimSpace(recoveryRoot) == "" {
		return "", "", errors.New("gateway recovery: recovery root is required")
	}
	info, err := os.Lstat(root)
	if err != nil {
		return "", "", fmt.Errorf("gateway recovery: inspect recovery root: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", "", errors.New("gateway recovery: recovery root must be a real directory")
	}
	active, err := filepath.Abs(strings.TrimSpace(activeConfigPath))
	if err != nil || strings.TrimSpace(activeConfigPath) == "" {
		return "", "", errors.New("gateway recovery: active configuration path is required")
	}
	if err := requireContainedPath(root, active); err != nil {
		return "", "", err
	}
	if err := rejectSymlinkPath(root, active); err != nil {
		return "", "", err
	}
	return root, active, nil
}

func requireContainedPath(root, candidate string) error {
	relative, err := filepath.Rel(root, candidate)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return errors.New("gateway recovery: path escapes recovery root")
	}
	return nil
}

func rejectSymlinkPath(root, candidate string) error {
	relative, err := filepath.Rel(root, candidate)
	if err != nil {
		return fmt.Errorf("gateway recovery: resolve bounded path: %w", err)
	}
	current := root
	for _, part := range strings.Split(relative, string(filepath.Separator)) {
		if part == "." || part == "" {
			continue
		}
		current = filepath.Join(current, part)
		entry, statErr := os.Lstat(current)
		if errors.Is(statErr, os.ErrNotExist) {
			break
		}
		if statErr != nil {
			return fmt.Errorf("gateway recovery: inspect bounded path: %w", statErr)
		}
		if entry.Mode()&os.ModeSymlink != 0 {
			return errors.New("gateway recovery: path contains symbolic link")
		}
	}
	return nil
}

func readRegularNoSymlink(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("gateway recovery: inspect file: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("gateway recovery: expected a regular non-symlink file")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("gateway recovery: read file: %w", err)
	}
	return data, nil
}

func atomicWrite(path string, data []byte, mode os.FileMode) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("gateway recovery: create parent directory: %w", err)
	}
	temporary, err := os.CreateTemp(directory, ".goreecloud-gateway-recovery-*")
	if err != nil {
		return fmt.Errorf("gateway recovery: create temporary file: %w", err)
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(mode); err != nil {
		temporary.Close()
		return fmt.Errorf("gateway recovery: protect temporary file: %w", err)
	}
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return fmt.Errorf("gateway recovery: write temporary file: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return fmt.Errorf("gateway recovery: sync temporary file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("gateway recovery: close temporary file: %w", err)
	}
	if err := os.Rename(temporaryName, path); err != nil {
		return fmt.Errorf("gateway recovery: atomically replace file: %w", err)
	}
	return syncDirectory(directory)
}

// Persist the directory entry created by rename (and newly-created snapshot
// directories). A file fsync alone cannot make the replacement crash durable.
func syncDirectory(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("gateway recovery: open directory for sync: %w", err)
	}
	defer dir.Close()
	if err := dir.Sync(); err != nil {
		return fmt.Errorf("gateway recovery: sync parent directory; target may already have changed: %w", err)
	}
	return nil
}

func sha256Hex(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func validSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil && value == strings.ToLower(value)
}

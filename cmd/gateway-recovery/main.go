package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/GoreeCloud/goreecloud-gateway/internal/config"
)

type snapshotOutput struct {
	SnapshotDir string                  `json:"snapshot_dir"`
	Snapshot    config.RecoverySnapshot `json:"snapshot"`
}

type stageOutput struct {
	StageDir string                `json:"stage_dir"`
	Stage    config.StagedRevision `json:"stage"`
}

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// run exposes the config-only recovery lifecycle to operators. Neither this
// CLI nor its receipts authorizes a runtime reload, listener transfer, or
// production cutover; those require separate target acceptance.
func run(args []string, output io.Writer) error {
	flags := flag.NewFlagSet("gateway-recovery", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	action := flags.String("action", "", "recovery action: snapshot, stage, activate or restore")
	root := flags.String("root", "", "bounded Gateway recovery root")
	configPath := flags.String("config", "", "active Gateway configuration path")
	sourceRevision := flags.String("source-revision", "", "exact source revision for snapshot or staged candidate")
	currentSourceRevision := flags.String("current-source-revision", "", "exact previous active source revision for activation")
	candidatePath := flags.String("candidate", "", "candidate Gateway configuration file to stage")
	stageDir := flags.String("staged", "", "staged revision directory to activate")
	snapshotDir := flags.String("snapshot", "", "snapshot directory to restore")
	expectedCurrent := flags.String("expected-current-sha256", "", "expected active configuration SHA-256 before activate or restore")
	if err := flags.Parse(args); err != nil {
		return fmt.Errorf("gateway recovery: parse flags: %w", err)
	}
	if flags.NArg() > 0 {
		return fmt.Errorf("gateway recovery: unexpected positional arguments")
	}

	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")

	switch *action {
	case "snapshot":
		directory, snapshot, err := config.CreateRecoverySnapshot(
			*root,
			*configPath,
			*sourceRevision,
			time.Now().UTC(),
		)
		if err != nil {
			return err
		}
		return encoder.Encode(snapshotOutput{SnapshotDir: directory, Snapshot: snapshot})
	case "stage":
		if strings.TrimSpace(*candidatePath) == "" {
			return fmt.Errorf("gateway recovery: -candidate is required for staging")
		}
		candidate, err := os.ReadFile(strings.TrimSpace(*candidatePath))
		if err != nil {
			return fmt.Errorf("gateway recovery: read candidate: %w", err)
		}
		directory, staged, err := config.StageRevision(*root, candidate, *sourceRevision, time.Now().UTC())
		if err != nil {
			return err
		}
		return encoder.Encode(stageOutput{StageDir: directory, Stage: staged})
	case "activate":
		receipt, err := config.ActivateStagedRevision(
			*root, *configPath, *stageDir, *expectedCurrent, *currentSourceRevision, time.Now().UTC(),
		)
		if err != nil {
			return err
		}
		return encoder.Encode(receipt)
	case "restore":
		receipt, err := config.RestoreRecoverySnapshot(
			*root,
			*configPath,
			*snapshotDir,
			*expectedCurrent,
			time.Now().UTC(),
		)
		if err != nil {
			return err
		}
		return encoder.Encode(receipt)
	default:
		return fmt.Errorf("gateway recovery: -action must be snapshot, stage, activate or restore")
	}
}

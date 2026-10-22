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

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, time.Now))
}

func run(args []string, stdout, stderr io.Writer, now func() time.Time) int {
	flags := flag.NewFlagSet("gateway-migration-verify", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "", "validated Gateway candidate configuration path")
	manifestPath := flags.String("manifest", "", "independently reviewed Caddy migration-source manifest path")
	preflightPath := flags.String("preflight-report", "", "protected read-only Caddy collector report path")
	sourceRevision := flags.String("source-revision", "", "exact 40-character lowercase Gateway source revision being evaluated")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "gateway migration verify: positional arguments are not supported")
		return 2
	}
	if strings.TrimSpace(*configPath) == "" || strings.TrimSpace(*manifestPath) == "" || strings.TrimSpace(*preflightPath) == "" || strings.TrimSpace(*sourceRevision) == "" {
		fmt.Fprintln(stderr, "gateway migration verify: -config, -manifest, -preflight-report, and -source-revision are required")
		return 2
	}
	if now == nil {
		fmt.Fprintln(stderr, "gateway migration verify: evidence clock is required")
		return 1
	}

	cfg, err := config.Load(strings.TrimSpace(*configPath))
	if err != nil {
		fmt.Fprintf(stderr, "gateway migration verify: load candidate configuration: %v\n", err)
		return 1
	}
	manifest, err := config.LoadMigrationSourceManifest(strings.TrimSpace(*manifestPath))
	if err != nil {
		fmt.Fprintf(stderr, "gateway migration verify: load reviewed migration source: %v\n", err)
		return 1
	}
	report, err := os.Open(strings.TrimSpace(*preflightPath))
	if err != nil {
		fmt.Fprintf(stderr, "gateway migration verify: open Caddy preflight report: %v\n", err)
		return 1
	}
	defer report.Close()
	if err := config.ValidateCaddyPreflightSourceIdentity(report, manifest); err != nil {
		fmt.Fprintf(stderr, "gateway migration verify: preflight source rejected: %v\n", err)
		return 1
	}

	evidence, err := config.BuildConfigParityEvidenceFromMigrationSource(
		cfg,
		strings.TrimSpace(*sourceRevision),
		manifest,
		now().UTC(),
	)
	if err != nil {
		fmt.Fprintf(stderr, "gateway migration verify: build parity evidence: %v\n", err)
		return 1
	}
	if err := config.ValidateConfigParityEvidence(evidence); err != nil {
		fmt.Fprintf(stderr, "gateway migration verify: parity rejected: %v\n", err)
		return 1
	}

	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(evidence); err != nil {
		fmt.Fprintf(stderr, "gateway migration verify: encode minimized evidence: %v\n", err)
		return 1
	}
	return 0
}

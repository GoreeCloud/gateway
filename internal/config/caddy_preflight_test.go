package config

import (
	"strings"
	"testing"
)

func TestValidateCaddyPreflightSourceIdentityAcceptsMatchingReadOnlyReport(t *testing.T) {
	cfg := testParityConfig()
	fingerprint, err := ConfigParityFingerprint(cfg)
	if err != nil {
		t.Fatal(err)
	}
	manifest := testMigrationSourceManifest(cfg, fingerprint)
	report := syntheticCaddyPreflightReport(manifest.SourceConfigSHA256)
	if err := ValidateCaddyPreflightSourceIdentity(strings.NewReader(report), manifest); err != nil {
		t.Fatal(err)
	}
}

func TestValidateCaddyPreflightSourceIdentityFailsClosed(t *testing.T) {
	cfg := testParityConfig()
	fingerprint, err := ConfigParityFingerprint(cfg)
	if err != nil {
		t.Fatal(err)
	}
	manifest := testMigrationSourceManifest(cfg, fingerprint)
	valid := syntheticCaddyPreflightReport(manifest.SourceConfigSHA256)
	for _, tc := range []struct {
		name   string
		report string
	}{
		{name: "source drift", report: syntheticCaddyPreflightReport(strings.Repeat("e", 64))},
		{name: "unavailable checksum", report: syntheticCaddyPreflightReport("unavailable")},
		{name: "missing source checksum", report: strings.Replace(valid, "config_sha256="+manifest.SourceConfigSHA256+"\n", "", 1)},
		{name: "duplicate checksum", report: valid + "config_sha256=" + manifest.SourceConfigSHA256 + "\n"},
		{name: "missing footer", report: strings.TrimSuffix(valid, "caddy_production_authority_unchanged=yes\n")},
		{name: "incomplete header", report: strings.Replace(valid, "mode=read-only\n", "", 1)},
		{name: "unsafe mutation", report: strings.Replace(valid, "mutation_permitted=no", "mutation_permitted=yes", 1)},
		{name: "adaptation failed", report: strings.Replace(valid, "adapted_config_status=parsed", "adapted_config_status=parse-failed", 1)},
		{name: "fatal collector status", report: valid + "fatal=mock-failure\n"},
		{name: "conflicting footer", report: strings.Replace(valid, "live_provider_query_performed=no", "live_provider_query_performed=yes", 1)},
		{name: "oversized report", report: valid + strings.Repeat("ignored=synthetic\n", 300000)},
		{name: "oversized line", report: valid + strings.Repeat("x", 140000)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateCaddyPreflightSourceIdentity(strings.NewReader(tc.report), manifest); err == nil {
				t.Fatal("invalid Caddy preflight report accepted")
			}
		})
	}
	if err := ValidateCaddyPreflightSourceIdentity(nil, manifest); err == nil {
		t.Fatal("nil Caddy report accepted")
	}
}

func syntheticCaddyPreflightReport(sourceSHA string) string {
	return "GoreeCloud Gateway Caddy VPS read-only preflight\n" +
		"mode=read-only\n" +
		"mutation_permitted=no\n" +
		"secret_values_printed=no\n" +
		"live_provider_query_performed=no\n" +
		"config_sha256=" + sourceSHA + "\n" +
		"=== Sanitized Adapted Caddy Structure ===\n" +
		"adapted_config_status=parsed\n" +
		"sanitized_route_record_count=1\n" +
		"route_record[001].hosts=synthetic.invalid\n" +
		"=== Safety Summary ===\n" +
		"mode=read-only\n" +
		"mutation_permitted=no\n" +
		"secret_values_printed=no\n" +
		"live_provider_query_performed=no\n" +
		"caddy_production_authority_unchanged=yes\n"
}

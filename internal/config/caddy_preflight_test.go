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
		{name: "missing route count", report: strings.Replace(valid, "sanitized_route_record_count=1\n", "", 1)},
		{name: "missing encode count", report: strings.Replace(valid, "encode_handler_count=0\n", "", 1)},
		{name: "duplicate encode count", report: valid + "encode_handler_count=0\n"},
		{name: "encode count mismatch", report: strings.Replace(valid, "encode_handler_count=0", "encode_handler_count=1", 1)},
		{name: "encode fields on proxy", report: valid + "route_record[001].encode_codecs=gzip\n"},
		{name: "zero routes", report: strings.Replace(valid, "sanitized_route_record_count=1", "sanitized_route_record_count=0", 1)},
		{name: "route count mismatch", report: strings.Replace(valid, "sanitized_route_record_count=1", "sanitized_route_record_count=2", 1)},
		{name: "duplicate route count", report: valid + "sanitized_route_record_count=1\n"},
		{name: "missing route handler", report: strings.Replace(valid, "route_record[001].handler=reverse_proxy\n", "", 1)},
		{name: "missing route server", report: strings.Replace(valid, "route_record[001].server=srv0\n", "", 1)},
		{name: "missing optional-empty route field", report: strings.Replace(valid, "route_record[001].paths=\n", "", 1)},
		{name: "duplicate route handler", report: valid + "route_record[001].handler=reverse_proxy\n"},
		{name: "invalid route handler", report: strings.Replace(valid, "route_record[001].handler=reverse_proxy", "route_record[001].handler=", 1)},
		{name: "out of range route", report: strings.Replace(valid, "route_record[001].handler=reverse_proxy", "route_record[002].handler=reverse_proxy", 1)},
		{name: "invalid row key", report: strings.Replace(valid, "route_record[001].handler=reverse_proxy", "route_record[foo].handler=reverse_proxy", 1)},
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

func TestValidateCaddyPreflightSourceIdentityEncodeInventory(t *testing.T) {
	cfg := testParityConfig()
	fingerprint, err := ConfigParityFingerprint(cfg)
	if err != nil {
		t.Fatal(err)
	}
	manifest := testMigrationSourceManifest(cfg, fingerprint)
	valid := strings.Replace(syntheticCaddyPreflightReport(manifest.SourceConfigSHA256), "encode_handler_count=0", "encode_handler_count=1", 1)
	valid = strings.Replace(valid, "route_record[001].handler=reverse_proxy", "route_record[001].handler=encode", 1)
	valid = strings.Replace(valid, "route_record[001].upstreams=backend:8080\n", "route_record[001].upstreams=\n"+
		"route_record[001].encode_codecs=gzip;zstd\n"+
		"route_record[001].encode_prefer=zstd;gzip\n"+
		"route_record[001].encode_minimum_length=unspecified\n"+
		"route_record[001].encode_match=implicit\n"+
		"route_record[001].encode_depth=1\n", 1)
	if err := ValidateCaddyPreflightSourceIdentity(strings.NewReader(valid), manifest); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		report string
	}{
		{"missing codecs", strings.Replace(valid, "route_record[001].encode_codecs=gzip;zstd\n", "", 1)},
		{"unsupported codec", strings.Replace(valid, "encode_codecs=gzip;zstd", "encode_codecs=br", 1)},
		{"duplicate codec", strings.Replace(valid, "encode_codecs=gzip;zstd", "encode_codecs=gzip;gzip", 1)},
		{"codec order not canonical", strings.Replace(valid, "encode_codecs=gzip;zstd", "encode_codecs=zstd;gzip", 1)},
		{"unsupported preference", strings.Replace(valid, "encode_prefer=zstd;gzip", "encode_prefer=br", 1)},
		{"duplicate preference", strings.Replace(valid, "encode_prefer=zstd;gzip", "encode_prefer=zstd;zstd", 1)},
		{"invalid threshold", strings.Replace(valid, "encode_minimum_length=unspecified", "encode_minimum_length=-1", 1)},
		{"invalid match shape", strings.Replace(valid, "encode_match=implicit", "encode_match=unknown", 1)},
		{"invalid depth", strings.Replace(valid, "encode_depth=1", "encode_depth=999", 1)},
		{"duplicated field", valid + "route_record[001].encode_match=implicit\n"},
		{"count mismatch", strings.Replace(valid, "encode_handler_count=1", "encode_handler_count=0", 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateCaddyPreflightSourceIdentity(strings.NewReader(tc.report), manifest); err == nil {
				t.Fatal("invalid Caddy encode policy inventory accepted")
			}
		})
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
		"encode_handler_count=0\n" +
		"route_record[001].server=srv0\n" +
		"route_record[001].hosts=synthetic.invalid\n" +
		"route_record[001].paths=\n" +
		"route_record[001].methods=\n" +
		"route_record[001].protocols=\n" +
		"route_record[001].handler=reverse_proxy\n" +
		"route_record[001].upstreams=backend:8080\n" +
		"=== Safety Summary ===\n" +
		"mode=read-only\n" +
		"mutation_permitted=no\n" +
		"secret_values_printed=no\n" +
		"live_provider_query_performed=no\n" +
		"caddy_production_authority_unchanged=yes\n"
}

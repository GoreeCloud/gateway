package config

import (
	"strings"
	"testing"
	"time"
)

func TestConfigParityFingerprintIsOrderIndependent(t *testing.T) {
	cfg := testParityConfig()
	first, err := ConfigParityFingerprint(cfg)
	if err != nil {
		t.Fatal(err)
	}

	cfg.Services[0].BackendIDs[0], cfg.Services[0].BackendIDs[1] = cfg.Services[0].BackendIDs[1], cfg.Services[0].BackendIDs[0]
	cfg.Backends[0], cfg.Backends[1] = cfg.Backends[1], cfg.Backends[0]
	cfg.Routes[0].Methods[0], cfg.Routes[0].Methods[1] = cfg.Routes[0].Methods[1], cfg.Routes[0].Methods[0]
	second, err := ConfigParityFingerprint(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("semantic reorder changed parity fingerprint: %s != %s", first, second)
	}
}

func TestConfigParityFingerprintChangesWithRouteSemantics(t *testing.T) {
	cfg := testParityConfig()
	first, err := ConfigParityFingerprint(cfg)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Routes[0].PathPrefix = "/changed"
	second, err := ConfigParityFingerprint(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("route semantic change did not change parity fingerprint")
	}
}

func TestStaticResponseParityBindsBodyAndStatusWithoutChangingLegacyProxyIdentity(t *testing.T) {
	cfg := validConfig()
	legacy, err := ConfigParityFingerprint(cfg)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Routes[0].Action = "reverse_proxy"
	explicitProxy, err := ConfigParityFingerprint(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if legacy != explicitProxy {
		t.Fatal("explicit reverse_proxy action must preserve legacy parity identity")
	}
	cfg.Routes[0].Action = "static_response"
	cfg.Routes[0].ServiceID = ""
	cfg.Routes[0].StaticResponse = &StaticResponse{StatusCode: 503, Body: "maintenance"}
	staticFirst, err := ConfigParityFingerprint(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if staticFirst == legacy {
		t.Fatal("changing from proxy to static response must change parity identity")
	}
	cfg.Routes[0].StaticResponse.StatusCode = 200
	staticStatus, err := ConfigParityFingerprint(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if staticStatus == staticFirst {
		t.Fatal("static response status change did not change parity identity")
	}
	cfg.Routes[0].StaticResponse.Body = "updated"
	staticBody, err := ConfigParityFingerprint(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if staticBody == staticStatus {
		t.Fatal("static response body change did not change parity identity")
	}
}

func TestStaticResponseEncodingParityBindsPreferenceAndThreshold(t *testing.T) {
	cfg := validConfig()
	cfg.Routes[0].Action = "static_response"
	cfg.Routes[0].ServiceID = ""
	cfg.Routes[0].StaticResponse = &StaticResponse{StatusCode: 200, Body: strings.Repeat("a", 1024)}
	baseline, err := ConfigParityFingerprint(cfg)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Routes[0].ResponseEncoding = &ResponseEncoding{Formats: []string{"zstd", "gzip"}}
	first, err := ConfigParityFingerprint(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if first == baseline {
		t.Fatal("opt-in encoding must change configuration parity")
	}
	cfg.Routes[0].ResponseEncoding.Formats = []string{"gzip", "zstd"}
	reordered, err := ConfigParityFingerprint(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if first == reordered {
		t.Fatal("encoding preference order must be fingerprinted")
	}
	minimum := 1024
	cfg.Routes[0].ResponseEncoding.MinimumLength = &minimum
	threshold, err := ConfigParityFingerprint(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if reordered == threshold {
		t.Fatal("encoding minimum length must be fingerprinted")
	}
}

func TestProxyResponseEncodingParityBindsPreferenceAndThreshold(t *testing.T) {
	cfg := validConfig()
	baseline, err := ConfigParityFingerprint(cfg)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Routes[0].ProxyResponseEncoding = &ResponseEncoding{Formats: []string{"zstd", "gzip"}}
	first, err := ConfigParityFingerprint(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if baseline == first {
		t.Fatal("opt-in proxy encoding must change parity identity")
	}
	cfg.Routes[0].ProxyResponseEncoding.Formats = []string{"gzip", "zstd"}
	reordered, err := ConfigParityFingerprint(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if first == reordered {
		t.Fatal("proxy encoding preference order must change parity identity")
	}
	minimum := 2048
	cfg.Routes[0].ProxyResponseEncoding.MinimumLength = &minimum
	threshold, err := ConfigParityFingerprint(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if reordered == threshold {
		t.Fatal("proxy encoding threshold must change parity identity")
	}
}

func TestConfigParityFingerprintBindsResponseHeaders(t *testing.T) {
	cfg := testParityConfig()
	baseline, err := ConfigParityFingerprint(cfg)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Routes[0].ResponseHeaders = map[string]string{"X-Gateway-Policy": "one"}
	first, err := ConfigParityFingerprint(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if baseline == first {
		t.Fatal("response-header policy did not change parity fingerprint")
	}
	cfg.Routes[0].ResponseHeaders = map[string]string{"x-gateway-policy": "one"}
	canonical, err := ConfigParityFingerprint(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if canonical != first {
		t.Fatal("header casing changed semantic parity fingerprint")
	}
	cfg.Routes[0].ResponseHeaders["x-gateway-policy"] = "two"
	changed, err := ConfigParityFingerprint(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if changed == first {
		t.Fatal("response-header value change did not change parity fingerprint")
	}
}

func TestConfigParityFingerprintTracksTrustedProxyTrustBoundary(t *testing.T) {
	cfg := testParityConfig()
	without, err := ConfigParityFingerprint(cfg)
	if err != nil {
		t.Fatal(err)
	}
	cfg.TrustedProxies = []string{"192.0.2.0/24"}
	withFirst, err := ConfigParityFingerprint(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if without == withFirst {
		t.Fatal("adding a trusted ingress proxy did not change parity fingerprint")
	}
	cfg.TrustedProxies = []string{"198.51.100.0/24"}
	withSecond, err := ConfigParityFingerprint(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if withFirst == withSecond {
		t.Fatal("changing the trusted proxy network did not change parity fingerprint")
	}
}

func TestConfigParityFingerprintNormalizesTrustedProxyEntries(t *testing.T) {
	cfg := testParityConfig()
	cfg.TrustedProxies = []string{"198.51.100.9", "192.0.2.37/24", "192.0.2.0/24"}
	first, err := ConfigParityFingerprint(cfg)
	if err != nil {
		t.Fatal(err)
	}
	cfg.TrustedProxies = []string{"192.0.2.0/24", "198.51.100.9/32"}
	second, err := ConfigParityFingerprint(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("equivalent trusted-proxy sets changed parity fingerprint: %s != %s", first, second)
	}
}

func TestValidateConfigParityEvidenceRejectsIncompleteMigrationIdentity(t *testing.T) {
	cfg := testParityConfig()
	expected, err := ConfigParityFingerprint(cfg)
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := BuildConfigParityEvidence(cfg, strings.Repeat("a", 40), expected, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	evidence.MigrationSourceConfigSHA256 = strings.Repeat("d", 64)
	if err := ValidateConfigParityEvidence(evidence); err == nil {
		t.Fatal("source identity without reviewed evidence was accepted")
	}
	evidence.MigrationReviewEvidenceSHA256 = strings.Repeat("c", 64)
	if err := ValidateConfigParityEvidence(evidence); err != nil {
		t.Fatalf("fully bound migration identity rejected: %v", err)
	}
	evidence.MigrationSourceConfigSHA256 = expected
	if err := ValidateConfigParityEvidence(evidence); err == nil {
		t.Fatal("Gateway fingerprint reused as Caddy source identity was accepted")
	}
}

func TestBuildAndValidateConfigParityEvidence(t *testing.T) {
	cfg := testParityConfig()
	expected, err := ConfigParityFingerprint(cfg)
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := BuildConfigParityEvidence(
		cfg,
		strings.Repeat("a", 40),
		expected,
		time.Date(2026, 8, 30, 3, 10, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !evidence.ConfigurationParityMatched {
		t.Fatal("matching reviewed configuration was not marked as parity matched")
	}
	if evidence.ProductionCutoverAuthorized {
		t.Fatal("parity evidence unexpectedly authorized production cutover")
	}
	if err := ValidateConfigParityEvidence(evidence); err != nil {
		t.Fatal(err)
	}
}

func TestValidateConfigParityEvidenceRejectsMismatchAndCutover(t *testing.T) {
	cfg := testParityConfig()
	evidence, err := BuildConfigParityEvidence(
		cfg,
		strings.Repeat("b", 40),
		strings.Repeat("0", 64),
		time.Date(2026, 8, 30, 3, 10, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatal(err)
	}
	if evidence.ConfigurationParityMatched {
		t.Fatal("mismatching fingerprint unexpectedly matched")
	}
	if err := ValidateConfigParityEvidence(evidence); err == nil {
		t.Fatal("mismatching parity evidence unexpectedly validated")
	}

	actual, err := ConfigParityFingerprint(cfg)
	if err != nil {
		t.Fatal(err)
	}
	evidence, err = BuildConfigParityEvidence(
		cfg,
		strings.Repeat("b", 40),
		actual,
		time.Date(2026, 8, 30, 3, 10, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatal(err)
	}
	evidence.ProductionCutoverAuthorized = true
	if err := ValidateConfigParityEvidence(evidence); err == nil {
		t.Fatal("cutover-authorizing parity evidence unexpectedly validated")
	}
}

func testParityConfig() *Config {
	return &Config{
		Schema: "goreecloud-gateway-config/v1",
		Backends: []Backend{
			{ID: "backend-a", URL: "http://127.0.0.1:18081", Enabled: true, HealthPath: "/health"},
			{ID: "backend-b", URL: "http://127.0.0.1:18082", Enabled: true, HealthPath: "/health"},
		},
		Services: []Service{
			{ID: "service-a", Name: "Service A", BackendIDs: []string{"backend-a", "backend-b"}},
		},
		CertificateProfiles: []CertificateProfile{
			{ID: "tls-a", CertificateFile: "/run/goreecloud/cert.pem", PrivateKeyFile: "/run/goreecloud/key.pem", Enabled: true},
		},
		Routes: []Route{
			{
				ID: "route-a", ServiceID: "service-a", Hostname: "Example.GoreeCloud.Invalid.", PathPrefix: "/api",
				Methods: []string{"post", "GET"}, Exposure: "restricted-public", Enabled: true,
				TLS: RouteTLS{Mode: "required", CertificateProfile: "tls-a"},
			},
		},
	}
}

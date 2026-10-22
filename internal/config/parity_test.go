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

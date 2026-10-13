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

func TestConfigParityFingerprintChangesWithNetworkAccess(t *testing.T) {
	cfg := testParityConfig()
	initial, err := ConfigParityFingerprint(cfg)
	if err != nil { t.Fatal(err) }
	cfg.Routes[0].AllowedClientCIDRs = []string{"198.51.100.0/24"}
	changed, err := ConfigParityFingerprint(cfg)
	if err != nil { t.Fatal(err) }
	if changed == initial { t.Fatal("access policy change did not change migration fingerprint") }

	cfg.Routes[0].AllowedClientCIDRs = []string{"203.0.113.0/24", "198.51.100.0/24"}
	first, err := ConfigParityFingerprint(cfg)
	if err != nil { t.Fatal(err) }
	cfg.Routes[0].AllowedClientCIDRs[0], cfg.Routes[0].AllowedClientCIDRs[1] =
		cfg.Routes[0].AllowedClientCIDRs[1], cfg.Routes[0].AllowedClientCIDRs[0]
	second, err := ConfigParityFingerprint(cfg)
	if err != nil { t.Fatal(err) }
	if first != second { t.Fatal("CIDR list ordering changed migration fingerprint") }
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
				Methods: []string{"post", "GET"}, Exposure: "restricted-public", AllowedClientCIDRs: []string{"203.0.113.0/24"}, Enabled: true,
				TLS: RouteTLS{Mode: "required", CertificateProfile: "tls-a"},
			},
		},
	}
}

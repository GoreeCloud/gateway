package config

import (
	"strings"
	"testing"
)

func TestResponseHeaderSetPolicyValidates(t *testing.T) {
	cfg := validConfig()
	cfg.Routes[0].ResponseHeaders = map[string]string{
		"Strict-Transport-Security": "max-age=31536000",
		"X-Frame-Options": "DENY",
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("bounded response headers rejected: %v", err)
	}
}

func TestResponseHeaderSetPolicyRejectsUnsafeValues(t *testing.T) {
	tests := []struct {
		name    string
		headers map[string]string
	}{
		{"injected line break", map[string]string{"X-Policy": "safe\r\nInjected: yes"}},
		{"control character", map[string]string{"X-Policy": "a\x00b"}},
		{"framing", map[string]string{"Content-Length": "8"}},
		{"content coding", map[string]string{"Content-Encoding": "gzip"}},
		{"hop by hop", map[string]string{"Connection": "keep-alive"}},
		{"invalid name", map[string]string{"Bad Header": "value"}},
		{"case collision", map[string]string{"X-Policy": "a", "x-policy": "b"}},
		{"oversized value", map[string]string{"X-Policy": strings.Repeat("a", 4097)}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.Routes[0].ResponseHeaders = tc.headers
			if err := cfg.Validate(); err == nil {
				t.Fatal("unsafe response-header set operation unexpectedly validated")
			}
		})
	}
}

func TestEnabledRouteRequiresExplicitTLSPolicy(t *testing.T) {
	cfg := validConfig()
	cfg.Routes[0].TLS = RouteTLS{}
	if err := cfg.Validate(); err == nil {
		t.Fatal("enabled route without TLS policy unexpectedly validated")
	}
}

func TestRequiredTLSRequiresCertificateProfile(t *testing.T) {
	cfg := validConfig()
	cfg.Routes[0].TLS = RouteTLS{Mode: "required"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("TLS-required route without certificate profile unexpectedly validated")
	}
}

func TestRequiredTLSRejectsMissingOrDisabledCertificateProfile(t *testing.T) {
	cfg := validConfig()
	cfg.Routes[0].TLS.CertificateProfile = "missing"
	if err := cfg.Validate(); err == nil {
		t.Fatal("missing certificate profile unexpectedly validated")
	}

	cfg = validConfig()
	cfg.CertificateProfiles[0].Enabled = false
	if err := cfg.Validate(); err == nil {
		t.Fatal("disabled certificate profile unexpectedly validated for required TLS route")
	}
}

func TestEnabledCertificateProfileRequiresFileReferences(t *testing.T) {
	cfg := validConfig()
	cfg.CertificateProfiles[0].PrivateKeyFile = ""
	if err := cfg.Validate(); err == nil {
		t.Fatal("enabled certificate profile without private-key file unexpectedly validated")
	}
}

func TestDisabledTLSRejectsCertificateProfile(t *testing.T) {
	cfg := validConfig()
	cfg.Routes[0].TLS = RouteTLS{Mode: "disabled", CertificateProfile: "should-not-be-used"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("TLS-disabled route with certificate profile unexpectedly validated")
	}
}

func TestExplicitTLSPoliciesValidate(t *testing.T) {
	for _, tls := range []RouteTLS{
		{Mode: "required", CertificateProfile: "default-private"},
		{Mode: "disabled"},
	} {
		cfg := validConfig()
		cfg.Routes[0].TLS = tls
		if err := cfg.Validate(); err != nil {
			t.Fatalf("TLS policy %+v rejected: %v", tls, err)
		}
	}
}

func validConfig() *Config {
	return &Config{
		Schema: "goreecloud-gateway-config/v1",
		Services: []Service{{
			ID:         "svc",
			Name:       "Service",
			BackendIDs: []string{"backend"},
		}},
		Routes: []Route{{
			ID:         "route",
			ServiceID:  "svc",
			Hostname:   "service.goreecloud.com",
			PathPrefix: "/",
			Exposure:   "private",
			Enabled:    true,
			TLS: RouteTLS{
				Mode:               "required",
				CertificateProfile: "default-private",
			},
		}},
		Backends: []Backend{{
			ID:      "backend",
			URL:     "http://127.0.0.1:8080",
			Enabled: true,
		}},
		CertificateProfiles: []CertificateProfile{{
			ID:              "default-private",
			CertificateFile: "/run/goreecloud-gateway/tls/default-private.crt",
			PrivateKeyFile:  "/run/goreecloud-gateway/tls/default-private.key",
			Enabled:         true,
		}},
	}
}

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

func TestStaticResponseRouteRequiresNoServiceOrBackend(t *testing.T) {
	cfg := &Config{
		Schema: "goreecloud-gateway-config/v1",
		Routes: []Route{{
			ID: "maintenance", Action: "static_response", Hostname: "maintenance.example.test",
			PathPrefix: "/", Exposure: "public", Enabled: true,
			TLS: RouteTLS{Mode: "disabled"},
			StaticResponse: &StaticResponse{StatusCode: 503, Body: "Temporarily unavailable\n"},
		}},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("standalone static response should validate: %v", err)
	}
}

func TestRouteActionsRejectAmbiguousOrUnsafeStaticResponses(t *testing.T) {
	tests := []struct {
		name string
		edit func(*Route)
	}{
		{"unknown action", func(r *Route) { r.Action = "script" }},
		{"proxy with static response", func(r *Route) { r.StaticResponse = &StaticResponse{StatusCode: 200} }},
		{"static with service", func(r *Route) { r.Action = "static_response" }},
		{"static missing response", func(r *Route) { r.Action = "static_response"; r.ServiceID = "" }},
		{"informational status", func(r *Route) { r.Action = "static_response"; r.ServiceID = ""; r.StaticResponse = &StaticResponse{StatusCode: 101} }},
		{"invalid status", func(r *Route) { r.Action = "static_response"; r.ServiceID = ""; r.StaticResponse = &StaticResponse{StatusCode: 600} }},
		{"oversized body", func(r *Route) { r.Action = "static_response"; r.ServiceID = ""; r.StaticResponse = &StaticResponse{StatusCode: 200, Body: strings.Repeat("a", 16*1024+1)} }},
		{"no-content body", func(r *Route) { r.Action = "static_response"; r.ServiceID = ""; r.StaticResponse = &StaticResponse{StatusCode: 204, Body: "no"} }},
		{"reset-content body", func(r *Route) { r.Action = "static_response"; r.ServiceID = ""; r.StaticResponse = &StaticResponse{StatusCode: 205, Body: "no"} }},
		{"not-modified body", func(r *Route) { r.Action = "static_response"; r.ServiceID = ""; r.StaticResponse = &StaticResponse{StatusCode: 304, Body: "no"} }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validConfig()
			tc.edit(&cfg.Routes[0])
			if err := cfg.Validate(); err == nil {
				t.Fatal("invalid route action unexpectedly passed validation")
			}
			cfg.Routes[0].Enabled = false
			if err := cfg.ValidateRouteActions(); err == nil {
				t.Fatal("invalid disabled route action unexpectedly passed activation validation")
			}
		})
	}
}

func TestLegacyReverseProxyRouteActionIsCompatible(t *testing.T) {
	cfg := validConfig()
	for _, action := range []string{"", "reverse_proxy"} {
		cfg.Routes[0].Action = action
		if err := cfg.Validate(); err != nil {
			t.Fatalf("route action %q rejected: %v", action, err)
		}
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

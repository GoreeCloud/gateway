package config

import "testing"

func TestBackendEndpointValidationAcceptsHTTPAndHTTPS(t *testing.T) {
	for _, endpoint := range []Backend{
		{ID: "backend", URL: "http://127.0.0.1:8080", HealthPath: "/healthz", Enabled: true},
		{ID: "backend", URL: "https://service.internal.example/base", HealthPath: "/ready", Enabled: true},
	} {
		cfg := validConfig()
		cfg.Backends[0] = endpoint
		if err := cfg.Validate(); err != nil {
			t.Fatalf("valid backend %+v rejected: %v", endpoint, err)
		}
	}
}

func TestBackendEndpointValidationRejectsUnsafeURLs(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{name: "relative", url: "/local/service"},
		{name: "unsupported scheme", url: "file:///tmp/backend.sock"},
		{name: "credentials", url: "https://user:secret@service.internal.example"},
		{name: "fragment", url: "https://service.internal.example/#admin"},
		{name: "query", url: "https://service.internal.example/?token=secret"},
		{name: "empty query delimiter", url: "https://service.internal.example/?"},
		{name: "encoded query value", url: "https://service.internal.example/base?next=%2Fprivate"},

		{name: "missing host", url: "https:///service"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.Backends[0].URL = tc.url
			if err := cfg.Validate(); err == nil {
				t.Fatalf("unsafe backend URL %q unexpectedly validated", tc.url)
			}
		})
	}
}

func TestBackendEndpointValidationRejectsNonPathHealthTarget(t *testing.T) {
	for _, healthPath := range []string{"healthz", "/healthz?token=value", "/healthz#fragment"} {
		cfg := validConfig()
		cfg.Backends[0].HealthPath = healthPath
		if err := cfg.Validate(); err == nil {
			t.Fatalf("unsafe health path %q unexpectedly validated", healthPath)
		}
	}
}

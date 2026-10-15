package config

import "testing"

// HealthChecker only accepts canonical lower-case "http" and "https" scheme
// names. Configuration must not allow a spelling it cannot probe or route.
func TestGoreeCloudBackendSchemeMatchesHealthTransport(t *testing.T) {
	for _, tc := range []struct {
		endpoint string
		accepted bool
	}{
		{endpoint: "http://127.0.0.1:8080", accepted: true},
		{endpoint: "https://service.internal.example", accepted: true},
		{endpoint: "HTTP://127.0.0.1:8080", accepted: false},
		{endpoint: "Https://service.internal.example", accepted: false},
	} {
		cfg := validConfig()
		cfg.Backends[0].URL = tc.endpoint
		err := cfg.Validate()
		if (err == nil) != tc.accepted {
			t.Errorf("URL %q accepted=%v, want=%v, error=%v", tc.endpoint, err == nil, tc.accepted, err)
		}
	}
}

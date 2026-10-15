package config

import "testing"

// The Go URL parser normalizes scheme spellings. Configuration should
// accept those endpoints consistently with the HTTP health probe.
func TestGoreeCloudBackendSchemeMatchesHealthTransport(t *testing.T) {
	for _, tc := range []struct {
		endpoint string
		accepted bool
	}{
		{endpoint: "http://127.0.0.1:8080", accepted: true},
		{endpoint: "https://service.internal.example", accepted: true},
		{endpoint: "HTTP://127.0.0.1:8080", accepted: true},
		{endpoint: "Https://service.internal.example", accepted: true},
	} {
		cfg := validConfig()
		cfg.Backends[0].URL = tc.endpoint
		err := cfg.Validate()
		if (err == nil) != tc.accepted {
			t.Errorf("URL %q accepted=%v, want=%v, error=%v", tc.endpoint, err == nil, tc.accepted, err)
		}
	}
}

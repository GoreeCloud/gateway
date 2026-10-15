package config

import "testing"

// HealthChecker reads the untrimmed URL and refuses network-path references.
// Configuration must reject these values before an unusable backend is live.
func TestGoreeCloudBackendValidationMatchesHealthChecker(t *testing.T) {
	for _, tc := range []struct {
		name string
		url string
		healthPath string
	}{
		{name: "leading URL whitespace", url: " http://127.0.0.1:8080", healthPath: "/ready"},
		{name: "trailing URL whitespace", url: "http://127.0.0.1:8080 ", healthPath: "/ready"},
		{name: "network path health target", url: "http://127.0.0.1:8080", healthPath: "//outside.example"},
		{name: "leading health path whitespace", url: "http://127.0.0.1:8080", healthPath: " /ready"},
		{name: "trailing health path whitespace", url: "http://127.0.0.1:8080", healthPath: "/ready "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.Backends[0].URL = tc.url
			cfg.Backends[0].HealthPath = tc.healthPath
			if err := cfg.Validate(); err == nil {
				t.Fatal("unsafe or unusable backend endpoint unexpectedly accepted")
			}
		})
	}
}

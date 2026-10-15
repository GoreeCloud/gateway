package config

import "testing"

func TestGoreeCloudEmptyFragmentDelimitersRejected(t *testing.T) {
	for _, endpoint := range []string{
		"http://127.0.0.1:8080/#",
		"https://service.internal.example#",
	} {
		cfg := validConfig()
		cfg.Backends[0].URL = endpoint
		if err := cfg.Validate(); err == nil {
			t.Fatalf("fragment-delimited backend %q accepted", endpoint)
		}
	}
}

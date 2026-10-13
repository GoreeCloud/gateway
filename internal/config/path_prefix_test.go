package config

import "testing"

func TestRoutePathValidation(t *testing.T) {
	tests := []struct {
		path string
		valid bool
	}{
		{"/", true},
		{"/account", true},
		{"/account/settings", true},
		{"account", false},
		{"/account//settings", false},
		{"/account/../public", false},
	}
	for _, tc := range tests {
		if got := validateRoutePathPrefix(tc.path) == nil; got != tc.valid {
			t.Errorf("path %q validity %t expected %t", tc.path, got, tc.valid)
		}
	}
}

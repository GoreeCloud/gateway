package config

import "testing"

func TestExposureValidationDeniesUnclassifiedAndUnsafeRoutes(t *testing.T) {
	cases := []struct {
		name     string
		exposure string
		allowed  []string
		tlsMode  string
		wantOK   bool
	}{
		{"private explicit subnet", "private", []string{"10.0.0.0/8"}, "required", true},
		{"private no subnet", "private", nil, "required", false},
		{"private public wildcard", "private", []string{"0.0.0.0/0"}, "required", false},
		{"private invalid prefix", "private", []string{"not-a-prefix"}, "required", false},
		{"private duplicate subnet", "private", []string{"10.1.0.0/16", "10.1.2.3/16"}, "required", false},
		{"private ipv6", "private", []string{"fd00::/8"}, "required", true},
		{"private mapped ipv6", "private", []string{"::ffff:10.0.0.0/104"}, "required", false},
		{"restricted requires network", "restricted-public", nil, "required", false},
		{"restricted requires tls", "restricted-public", []string{"203.0.113.7/32"}, "disabled", false},
		{"restricted with tls", "restricted-public", []string{"203.0.113.7/32"}, "required", true},
		{"public requires tls", "public", nil, "disabled", false},
		{"public requires no list", "public", []string{"10.0.0.0/8"}, "required", false},
		{"public explicit", "public", nil, "required", true},
		{"internal loopback", "internal", nil, "disabled", true},
		{"internal rejects list", "internal", []string{"127.0.0.0/8"}, "disabled", false},
		{"missing", "", nil, "required", false},
		{"unknown", "partner-network", nil, "required", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.Routes[0].Exposure = tc.exposure
			cfg.Routes[0].AllowedClientCIDRs = tc.allowed
			cfg.Routes[0].TLS.Mode = tc.tlsMode
			if tc.tlsMode == "disabled" {
				cfg.Routes[0].TLS.CertificateProfile = ""
			}
			err := cfg.Validate()
			if (err == nil) != tc.wantOK {
				t.Fatalf("Validate() error = %v; want success = %v", err, tc.wantOK)
			}
		})
	}
}

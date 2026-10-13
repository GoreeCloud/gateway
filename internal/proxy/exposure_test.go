package proxy

import (
	"net/netip"
	"testing"

	"github.com/GoreeCloud/goreecloud-gateway/internal/config"
)

func TestRouteClientExposureMatrix(t *testing.T) {
	cases := []struct {
		name     string
		exposure string
		allowed  []string
		client   string
		want     bool
	}{
		{"internal loopback", "internal", nil, "127.0.0.1", true},
		{"internal IPv6", "internal", nil, "::1", true},
		{"internal public client", "internal", nil, "203.0.113.5", false},
		{"private allowed", "private", []string{"100.64.0.0/10"}, "100.88.3.2", true},
		{"private denied", "private", []string{"100.64.0.0/10"}, "203.0.113.5", false},
		{"private denied no allowlist", "private", nil, "10.0.0.5", false},
		{"private denied wildcard", "private", []string{"0.0.0.0/0"}, "198.51.100.5", false},
		{"restricted allowed", "restricted-public", []string{"203.0.113.0/24"}, "203.0.113.5", true},
		{"restricted denied", "restricted-public", []string{"203.0.113.0/24"}, "198.51.100.5", false},
		{"public", "public", nil, "198.51.100.5", true},
		{"unspecified exposure", "", nil, "127.0.0.1", false},
		{"unknown exposure", "unknown", nil, "127.0.0.1", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			route := config.Route{Exposure: tc.exposure, AllowedClientCIDRs: tc.allowed}
			if got := permitsRouteClient(route, netip.MustParseAddr(tc.client)); got != tc.want {
				t.Fatalf("client=%s exposure=%s allowed=%v got=%v want=%v", tc.client, tc.exposure, tc.allowed, got, tc.want)
			}
		})
	}
}

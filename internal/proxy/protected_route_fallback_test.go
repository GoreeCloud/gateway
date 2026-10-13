package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/GoreeCloud/goreecloud-gateway/internal/config"
)

// Verify a public catch-all cannot answer for a protected path, even when
// an incoming method does not match the protected route's declared methods.
func TestProtectedRouteMethodFallback(t *testing.T) {
	upstreamCalls := 0
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		upstreamCalls++
		w.WriteHeader(http.StatusNoContent)
	}))
	defer backend.Close()

	cfg := testConfig([]config.Backend{{ID: "backend", URL: backend.URL, Enabled: true}}, []string{"backend"})
	cfg.Routes = append(cfg.Routes, config.Route{
		ID: "restricted", ServiceID: "svc", Hostname: "app.goreecloud.com",
		PathPrefix: "/team", Methods: []string{"POST"}, Enabled: true,
		Exposure: "private", AllowedClientCIDRs: []string{"192.0.2.0/24"},
	})
	h := New(cfg)
	defer h.Close()
	h.checker = &fakeChecker{healthy: map[string]bool{"backend": true}}

	tests := []struct {
		name, method, path, peer string
		want int
	}{
		{"unrelated path", "GET", "/news", "198.51.100.4:6000", http.StatusNoContent},
		{"exact path denied", "POST", "/team", "198.51.100.4:6000", http.StatusNotFound},
		{"method mismatch denied", "GET", "/team", "198.51.100.4:6000", http.StatusNotFound},
		{"nested path denied", "GET", "/team/preferences", "198.51.100.4:6000", http.StatusNotFound},
		{"authorized client wrong method", "GET", "/team", "192.0.2.4:6000", http.StatusNotFound},
		{"authorized client allowed", "POST", "/team", "192.0.2.4:6000", http.StatusNoContent},
		{"segment boundary public", "GET", "/teammate", "198.51.100.4:6000", http.StatusNoContent},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "http://app.goreecloud.com"+tc.path, nil)
			req.RemoteAddr = tc.peer
			response := httptest.NewRecorder()
			h.ServeHTTP(response, req)
			if response.Code != tc.want {
				t.Errorf("status=%d want=%d for %s %s", response.Code, tc.want, tc.method, tc.path)
			}
		})
	}
	if upstreamCalls != 3 {
		t.Fatalf("upstream requests=%d, want 3", upstreamCalls)
	}
}

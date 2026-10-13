package proxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/GoreeCloud/goreecloud-gateway/internal/config"
)

func TestPrivateGatewayRouteRejectsUnauthorizedClientsBeforeProbing(t *testing.T) {
	hits := 0
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		_, _ = io.WriteString(w, "ok")
	}))
	defer backend.Close()

	cfg := testConfig([]config.Backend{{ID: "backend", URL: backend.URL, Enabled: true}}, []string{"backend"})
	cfg.Routes[0].Exposure = "private"
	cfg.Routes[0].AllowedClientCIDRs = []string{"100.64.0.0/10"}
	handler := New(cfg)
	defer handler.Close()
	checker := &fakeChecker{healthy: map[string]bool{"backend": true}, checks: map[string]int{}}
	handler.checker = checker

	for _, tc := range []struct {
		name     string
		remote   string
		spoofed  string
		wantCode int
	}{
		{"allowed VPN client", "100.88.2.3:5000", "", http.StatusOK},
		{"external client", "203.0.113.9:5000", "", http.StatusNotFound},
		{"spoofed VPN address", "203.0.113.9:5000", "100.88.2.3", http.StatusNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "http://app.goreecloud.com/", nil)
			req.RemoteAddr = tc.remote
			if tc.spoofed != "" {
				req.Header.Set("X-Forwarded-For", tc.spoofed)
			}
			rw := httptest.NewRecorder()
			handler.ServeHTTP(rw, req)
			if rw.Code != tc.wantCode {
				t.Fatalf("status=%d want=%d body=%q", rw.Code, tc.wantCode, rw.Body.String())
			}
		})
	}
	if hits != 1 || checker.count("backend") != 1 {
		t.Fatalf("unauthorized request reached upstream or health probing: hits=%d checks=%d", hits, checker.count("backend"))
	}
}

func TestGatewayRouteChecksResolvedTrustedProxyClient(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "should not be reached")
	}))
	defer backend.Close()
	cfg := testConfig([]config.Backend{{ID: "backend", URL: backend.URL, Enabled: true}}, []string{"backend"})
	cfg.Routes[0].Exposure = "private"
	cfg.Routes[0].AllowedClientCIDRs = []string{"100.64.0.0/10"}
	cfg.TrustedProxies = []string{"10.0.0.0/8"}
	handler := New(cfg)
	defer handler.Close()
	req := httptest.NewRequest(http.MethodGet, "http://app.goreecloud.com/", nil)
	req.RemoteAddr = "10.0.0.3:5000"
	req.Header.Set("X-Forwarded-For", "203.0.113.8")
	rw := httptest.NewRecorder()
	handler.ServeHTTP(rw, req)
	if rw.Code != http.StatusNotFound || !strings.Contains(rw.Body.String(), "404") {
		t.Fatalf("untrusted client through trusted hop should be denied: status=%d body=%q", rw.Code, rw.Body.String())
	}
}

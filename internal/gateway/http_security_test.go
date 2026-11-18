package gateway

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestManagementHostAndAuthorizationHardening(t *testing.T) {
	manager, err := Open(t.TempDir()+"/sites.json", func([]byte, bool) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Start(); err != nil {
		t.Fatal(err)
	}
	token := strings.Repeat("s", 32)
	handler := Handler(manager, fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("<html></html>")},
	}, token, false)
	tryRequest := func(host string, authorization []string, forwarded string) int {
		req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
		req.Host = host
		req.RemoteAddr = "127.0.0.1:56789"
		for _, value := range authorization {
			req.Header.Add("Authorization", value)
		}
		if forwarded != "" {
			req.Header.Set("X-Forwarded-For", forwarded)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec.Code
	}
	if code := tryRequest("malicious.example", []string{"Bearer " + token}, ""); code != http.StatusForbidden {
		t.Fatalf("DNS rebinding Host bypassed: %d", code)
	}
	if code := tryRequest("127.0.0.1:8787", []string{"Bearer " + token, "Bearer " + token}, ""); code != http.StatusUnauthorized {
		t.Fatalf("duplicate Authorization was accepted: %d", code)
	}
	if code := tryRequest("localhost:8787", []string{"Bearer " + token}, ""); code != http.StatusOK {
		t.Fatalf("legitimate loopback session rejected: %d", code)
	}
	for i := 0; i < 10; i++ {
		if code := tryRequest("127.0.0.1:8787", []string{"Bearer invalid"}, "192.0.2.7"); code != http.StatusUnauthorized {
			t.Fatalf("failed attempt %d: %d", i, code)
		}
	}
	if code := tryRequest("127.0.0.1:8787", []string{"Bearer " + token}, "203.0.113.7"); code != http.StatusTooManyRequests {
		t.Fatalf("direct-peer rate limit bypassed via X-Forwarded-For: %d", code)
	}
}

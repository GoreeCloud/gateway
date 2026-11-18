package gateway

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"
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
	attempt := 0
	tryRequest := func(host string, authorization []string, forwarded string) int {
		attempt++
		req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
		req.Host = host
		req.RemoteAddr = fmt.Sprintf("127.0.0.1:%d", 55000+attempt)
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

// Every 127/8 IP is loopback, so a local process can rotate direct TCP peers.
// Rate limiting must not allocate unlimited authentication-failure records.
func TestAuthFailureRegistryBoundedAcrossLoopbackPeers(t *testing.T) {
	a := &api{failures: make(map[string]authFailure)}
	for i := 0; i < maxAuthFailurePeers*4; i++ {
		peer := fmt.Sprintf("127.0.%d.%d:41000", i/256, i%256)
		a.noteFailure(peer)
		if got := len(a.failures); got > maxAuthFailurePeers {
			t.Fatalf("authentication failures grew beyond %d entries: %d", maxAuthFailurePeers, got)
		}
	}
	last := fmt.Sprintf("127.0.%d.%d:41000", 3, 255)
	if _, ok := a.failures[authPeerKey(last)]; !ok {
		t.Fatal("latest authenticated peer was incorrectly evicted")
	}
	for i := 0; i < 9; i++ {
		a.noteFailure(last)
	}
	if !a.tooManyFailures(last) {
		t.Fatal("direct peer did not trip the normal per-IP failure threshold")
	}
}

func TestAuthFailureRegistryPrunesExpiredRecords(t *testing.T) {
	a := &api{failures: make(map[string]authFailure)}
	for i := 0; i < maxAuthFailurePeers; i++ {
		peer := fmt.Sprintf("127.0.%d.%d:42000", i/256, i%256)
		a.failures[authPeerKey(peer)] = authFailure{
			count: 10,
			until: time.Now().Add(-time.Minute),
		}
	}
	a.noteFailure("127.0.3.1:42001")
	if got := len(a.failures); got != 1 {
		t.Fatalf("expired failure records not reclaimed: %d", got)
	}
}

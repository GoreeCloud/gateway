package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPeerKeyIgnoresForwardedIdentity(t *testing.T) {
	cases := map[string]string{
		"127.0.0.1:1234":       "127.0.0.1",
		"[::1]:1234":           "::1",
		"[::ffff:192.0.2.1]:1": "192.0.2.1",
		"invalid":              "unknown",
	}
	for source, expected := range cases {
		if got := peerKey(source); got != expected {
			t.Errorf("source %s: got %s, want %s", source, got, expected)
		}
	}
}

func TestFailedAuthLimiterWindowAndMemory(t *testing.T) {
	now := time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)
	limiter := newFailedAuthLimiter(func() time.Time { return now })
	for i := 0; i < authFailureLimit; i++ {
		if !limiter.allowFailure("192.0.2.1:1900") {
			t.Fatalf("premature throttle at attempt %d", i)
		}
	}
	if limiter.allowFailure("192.0.2.1:1900") {
		t.Fatal("did not throttle bad credentials")
	}
	if !limiter.allowFailure("192.0.2.2:1900") {
		t.Fatal("independent direct peer unexpectedly throttled")
	}
	now = now.Add(authWindow)
	if !limiter.allowFailure("192.0.2.1:1900") {
		t.Fatal("failure window did not reset")
	}
	for i := 1; i <= 300; i++ {
		address := fmt.Sprintf("198.51.%d.%d:1900", i/250, i%250)
		limiter.allowFailure(address)
	}
	if got := len(limiter.entries); got > maxAuthPeers+1 {
		t.Fatalf("unbounded failure tracking: %d", got)
	}
}

func TestFailedAuthThrottlingAndValidAccess(t *testing.T) {
	token := strings.Repeat("T", 40)
	admin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("{\"apps\":{\"http\":{\"servers\":{}}}}"))
	}))
	defer admin.Close()
	handler, err := newHandler(token, admin.URL)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i <= authFailureLimit; i++ {
		request := httptest.NewRequest("GET", "/api/v1/overview", nil)
		request.RemoteAddr = "198.51.100.50:5432"
		request.Header.Set("Authorization", "Bearer incorrect")
		request.Header.Set("X-Forwarded-For", fmt.Sprintf("203.0.113.%d", i+1))
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		expected := http.StatusUnauthorized
		if i == authFailureLimit {
			expected = http.StatusTooManyRequests
		}
		if recorder.Code != expected {
			t.Fatalf("attempt %d: got %d, expected %d", i+1, recorder.Code, expected)
		}
		if i == authFailureLimit && recorder.Header().Get("Retry-After") == "" {
			t.Fatal("throttled response missing Retry-After")
		}
	}
	valid := httptest.NewRequest("GET", "/api/v1/overview", nil)
	valid.RemoteAddr = "198.51.100.50:5432"
	valid.Header.Set("Authorization", "Bearer "+token)
	success := httptest.NewRecorder()
	handler.ServeHTTP(success, valid)
	if success.Code != http.StatusOK {
		t.Fatalf("throttle blocked authorized session: %d", success.Code)
	}
	valid.Header.Add("Authorization", "Bearer "+token)
	duplicate := httptest.NewRecorder()
	handler.ServeHTTP(duplicate, valid)
	if duplicate.Code != http.StatusTooManyRequests {
		t.Fatalf("duplicate token header was not rejected: %d", duplicate.Code)
	}
}

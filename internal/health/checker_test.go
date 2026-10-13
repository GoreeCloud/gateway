package health

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/GoreeCloud/goreecloud-gateway/internal/config"
)

func TestHealthCheckerTreatsRedirectAsUnhealthyWithoutFollowing(t *testing.T) {
	var redirectedRequests atomic.Int32
	external := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirectedRequests.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer external.Close()

	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, external.URL+"/unexpected", http.StatusFound)
	}))
	defer source.Close()

	client := &http.Client{Timeout: time.Second}
	checker := &Checker{Client: client}
	result := checker.Check(context.Background(), config.Backend{
		ID: "primary", URL: source.URL, HealthPath: "/health",
	})
	if result.Healthy || result.StatusCode != http.StatusFound || result.Err == nil {
		t.Fatalf("redirect must not certify health: %+v", result)
	}
	if redirectedRequests.Load() != 0 {
		t.Fatal("health checker followed a redirect to another destination")
	}
	if client.CheckRedirect != nil {
		t.Fatal("the shared client must not be mutated")
	}
}

func TestHealthCheckerAccepts2xxOnly(t *testing.T) {
	for _, status := range []int{http.StatusNoContent, http.StatusMultipleChoices, http.StatusUnauthorized, http.StatusServiceUnavailable} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
			}))
			defer server.Close()
			result := New(time.Second).Check(context.Background(), config.Backend{
				ID: "backend", URL: server.URL, HealthPath: "/ready",
			})
			wantHealthy := status >= http.StatusOK && status < http.StatusMultipleChoices
			if result.Healthy != wantHealthy {
				t.Fatalf("status %d: got healthy=%v, expected %v", status, result.Healthy, wantHealthy)
			}
			if (result.Err == nil) != wantHealthy {
				t.Fatalf("status %d: result error inconsistent with health: %v", status, result.Err)
			}
		})
	}
}

func TestHealthCheckerRejectsUnsafeDirectInputs(t *testing.T) {
	for _, target := range []config.Backend{
		{ID: "unsafe-query", URL: "https://service.example/?token=unsafe"},
		{ID: "unsafe-user", URL: "https://user:pass@service.example/"},
		{ID: "unsafe-scheme", URL: "file:///tmp/socket"},
		{ID: "unsafe-health-path", URL: "https://service.example", HealthPath: "//outside.example"},
	} {
		result := New(time.Second).Check(context.Background(), target)
		if result.Err == nil || result.Healthy {
			t.Fatalf("unsafe health-check configuration unexpectedly accepted: %+v", result)
		}
	}
	result := (&Checker{}).Check(context.Background(), config.Backend{ID: "no-client", URL: "https://service.example"})
	if result.Err == nil || result.Healthy {
		t.Fatal("unconfigured health-check client must fail closed")
	}
}

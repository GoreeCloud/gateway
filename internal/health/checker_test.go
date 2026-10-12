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

func TestCheckerRejectsRedirectAndDoesNotContactRedirectTarget(t *testing.T) {
	var requests atomic.Int32
	redirectTarget := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer redirectTarget.Close()

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ready":
			w.WriteHeader(http.StatusNoContent)
		case "/redirect":
			http.Redirect(w, r, redirectTarget.URL+"/ready", http.StatusFound)
		default:
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}))
	defer backend.Close()

	checker := New(time.Second)
	for _, tc := range []struct {
		path    string
		status  int
		healthy bool
	}{
		{"/ready", http.StatusNoContent, true},
		{"/redirect", http.StatusFound, false},
		{"/offline", http.StatusServiceUnavailable, false},
	} {
		t.Run(tc.path, func(t *testing.T) {
			result := checker.Check(context.Background(), config.Backend{
				ID: "probe", URL: backend.URL, HealthPath: tc.path,
			})
			if result.StatusCode != tc.status || result.Healthy != tc.healthy {
				t.Fatalf("unexpected health result: %+v", result)
			}
			if (result.Err == nil) == (!tc.healthy) {
				t.Fatalf("unexpected health error: %+v", result)
			}
		})
	}
	if requests.Load() != 0 {
		t.Fatalf("health probe followed redirect %d times", requests.Load())
	}
}

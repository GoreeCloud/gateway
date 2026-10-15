package health

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/GoreeCloud/goreecloud-gateway/internal/config"
)

// Go's URL parser normalizes URI scheme casing. A supported backend must
// remain reachable by the health checker after config validation.
func TestGoreeCloudNormalizedSchemeHealthCheck(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.URL.Path != "/ready" {
			t.Errorf("health check path = %q", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	target := "HTTP" + strings.TrimPrefix(server.URL, "http")
	cfg := &config.Config{
		Schema:   "goreecloud-gateway-config/v1",
		Backends: []config.Backend{{ID: "healthy", URL: target, HealthPath: "/ready", Enabled: true}},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("valid normalized HTTP endpoint rejected: %v", err)
	}
	result := New(time.Second).Check(context.Background(), cfg.Backends[0])
	if result.Err != nil || !result.Healthy || result.StatusCode != http.StatusNoContent {
		t.Fatalf("normalized scheme probe failed: %+v", result)
	}
	if n := requests.Load(); n != 1 {
		t.Fatalf("received %d health probes, want 1", n)
	}
}

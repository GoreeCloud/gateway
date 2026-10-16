package proxy

import (
	"context"
	"testing"
	"time"

	"github.com/GoreeCloud/goreecloud-gateway/internal/config"
	"github.com/GoreeCloud/goreecloud-gateway/internal/health"
)

// A previously healthy backend cannot be trusted forever if periodic
// background probes have stopped or fallen behind.
func TestGoreeCloudStaleBackendHealthIsProbedAgain(t *testing.T) {
	backend := config.Backend{ID: "backend", URL: "http://127.0.0.1:8111", Enabled: true}
	h := New(testConfig([]config.Backend{backend}, []string{"backend"}))
	defer h.Close()
	checker := &fakeChecker{healthy: map[string]bool{"backend": false}, checks: map[string]int{}}
	h.checker = checker

	h.healthMu.Lock()
	h.healthState["backend"] = cachedHealth{
		result:    health.Result{BackendID: "backend", Healthy: true},
		checkedAt: time.Now().Add(-10 * healthRefreshInterval),
	}
	h.healthMu.Unlock()

	chosen := h.healthyCandidates(context.Background(), []config.Backend{backend})
	if len(chosen) != 0 {
		t.Fatal("backend with stale healthy status was eligible for new routing")
	}
	if got := checker.count("backend"); got != 1 {
		t.Fatalf("stale backend should have been rechecked, got %d checks", got)
	}
}

func TestGoreeCloudRecentBackendHealthIsReused(t *testing.T) {
	backend := config.Backend{ID: "backend", URL: "http://127.0.0.1:8111", Enabled: true}
	h := New(testConfig([]config.Backend{backend}, []string{"backend"}))
	defer h.Close()
	checker := &fakeChecker{healthy: map[string]bool{"backend": false}, checks: map[string]int{}}
	h.checker = checker

	h.healthMu.Lock()
	h.healthState["backend"] = cachedHealth{
		result:    health.Result{BackendID: "backend", Healthy: true},
		checkedAt: time.Now(),
	}
	h.healthMu.Unlock()

	chosen := h.healthyCandidates(context.Background(), []config.Backend{backend})
	if len(chosen) != 1 || checker.count("backend") != 0 {
		t.Fatal("fresh healthy status was not reused")
	}
}

package proxy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/GoreeCloud/goreecloud-gateway/internal/config"
	"github.com/GoreeCloud/goreecloud-gateway/internal/health"
)

func TestCachedHealthFreshnessBoundary(t *testing.T) {
	now := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		at time.Time
		fresh bool
	}{
		{"current", now, true},
		{"before expiry", now.Add(-healthEvidenceTTL + time.Millisecond), true},
		{"expired", now.Add(-healthEvidenceTTL), false},
		{"future", now.Add(time.Second), false},
		{"missing", time.Time{}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := (cachedHealth{checkedAt: tc.at}).freshAt(now); got != tc.fresh {
				t.Fatalf("freshAt = %v, want %v", got, tc.fresh)
			}
		})
	}
}

func TestStaleHealthyBackendIsReprobedAndExcluded(t *testing.T) {
	var probes atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		probes.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	backend := config.Backend{ID: "stale", URL: server.URL, Enabled: true}
	h := New(&config.Config{Backends: []config.Backend{backend}})
	defer h.Close()
	h.healthMu.Lock()
	h.healthState[backend.ID] = cachedHealth{
		result: health.Result{BackendID: backend.ID, Healthy: true},
		checkedAt: time.Now().Add(-2 * healthEvidenceTTL),
	}
	h.healthMu.Unlock()
	got := h.healthyCandidates(context.Background(), []config.Backend{backend})
	if len(got) != 0 || probes.Load() != 1 {
		t.Fatalf("stale backend not reprobed and rejected: candidates=%d probes=%d", len(got), probes.Load())
	}
}

func TestStatusSnapshotStaleHealthIsUnknown(t *testing.T) {
	now := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	h := New(&config.Config{
		Routes: []config.Route{{ID: "route", Enabled: true}},
		Backends: []config.Backend{{ID: "backend", Enabled: true}},
	})
	defer h.Close()
	h.healthMu.Lock()
	h.healthState["backend"] = cachedHealth{
		result: health.Result{BackendID: "backend", Healthy: true},
		checkedAt: now.Add(-healthEvidenceTTL),
	}
	h.healthMu.Unlock()
	status := h.StatusSnapshot(now)
	if status.Availability != AvailabilityUnknown || status.Unknown != 1 || status.HealthyBackends != 0 {
		t.Fatalf("stale health reported as available: %+v", status)
	}
}

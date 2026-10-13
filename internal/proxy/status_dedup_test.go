package proxy

import (
	"testing"
	"time"

	"github.com/GoreeCloud/goreecloud-gateway/internal/config"
	"github.com/GoreeCloud/goreecloud-gateway/internal/health"
)

// Two public/private route references must not inflate backend availability.
func TestGoreeCloudStatusCountsSharedBackendOnce(t *testing.T) {
	now := time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC)
	handler := New(&config.Config{
		Services: []config.Service{{ID: "shared", BackendIDs: []string{"b"}}},
		Routes: []config.Route{
			{ID: "first", ServiceID: "shared", Enabled: true},
			{ID: "second", ServiceID: "shared", Enabled: true},
		},
		Backends: []config.Backend{{ID: "b", Enabled: true}},
	})
	defer handler.Close()
	handler.healthMu.Lock()
	handler.healthState["b"] = cachedHealth{result: health.Result{Healthy: true}, checkedAt: now}
	handler.healthMu.Unlock()

	got := handler.StatusSnapshot(now)
	if got.Routes != 2 || got.Backends != 1 || got.HealthyBackends != 1 ||
		got.Availability != AvailabilityAvailable {
		t.Fatalf("shared backend counted more than once: %+v", got)
	}
}

func TestGoreeCloudDisabledRoutesCannotInflateAvailability(t *testing.T) {
	now := time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC)
	handler := New(&config.Config{
		Services: []config.Service{{ID: "disabled", BackendIDs: []string{"b"}}},
		Routes:   []config.Route{{ID: "disabled", ServiceID: "disabled", Enabled: false}},
		Backends: []config.Backend{{ID: "b", Enabled: true}},
	})
	defer handler.Close()
	handler.healthMu.Lock()
	handler.healthState["b"] = cachedHealth{result: health.Result{Healthy: true}, checkedAt: now}
	handler.healthMu.Unlock()

	got := handler.StatusSnapshot(now)
	if got.Routes != 0 || got.Backends != 0 || got.HealthyBackends != 0 ||
		got.Availability != AvailabilityInactive {
		t.Fatalf("disabled route influenced status: %+v", got)
	}
}

package proxy

import (
	"testing"
	"time"

	"github.com/GoreeCloud/goreecloud-gateway/internal/config"
	"github.com/GoreeCloud/goreecloud-gateway/internal/health"
)

func TestGoreeCloudActiveUnservedRouteCannotReportAllAvailable(t *testing.T) {
	now := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
	handler := New(&config.Config{
		Services: []config.Service{
			{ID: "ready", BackendIDs: []string{"live"}},
			{ID: "empty"},
			{ID: "disabled-only", BackendIDs: []string{"offline"}},
		},
		Routes: []config.Route{
			{ID: "one", ServiceID: "ready", Enabled: true},
			{ID: "two", ServiceID: "empty", Enabled: true},
			{ID: "three", ServiceID: "disabled-only", Enabled: true},
		},
		Backends: []config.Backend{
			{ID: "live", Enabled: true},
			{ID: "offline", Enabled: false},
		},
	})
	defer handler.Close()
	handler.healthMu.Lock()
	handler.healthState["live"] = cachedHealth{result: health.Result{Healthy: true}, checkedAt: now}
	handler.healthMu.Unlock()
	got := handler.StatusSnapshot(now)
	if got.Routes != 3 || got.UnservedRoutes != 2 || got.Backends != 1 ||
		got.HealthyBackends != 1 || got.Availability != AvailabilityDegraded ||
		got.AvailabilityReason != AvailabilityReasonUnservedRoutes {
		t.Fatalf("active but unserved routes were reported available: %+v", got)
	}
}

func TestGoreeCloudUnservedRouteWithoutUsableBackendsHasSpecificReason(t *testing.T) {
	now := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
	handler := New(&config.Config{
		Services: []config.Service{{ID: "disabled", BackendIDs: []string{"offline"}}},
		Routes:   []config.Route{{ID: "active", ServiceID: "disabled", Enabled: true}},
		Backends: []config.Backend{{ID: "offline", Enabled: false}},
	})
	defer handler.Close()
	got := handler.StatusSnapshot(now)
	if got.Routes != 1 || got.UnservedRoutes != 1 || got.Backends != 0 ||
		got.Availability != AvailabilityUnavailable ||
		got.AvailabilityReason != AvailabilityReasonUnservedRoutes {
		t.Fatalf("incorrect unserved route status: %+v", got)
	}
}

func TestGoreeCloudUnservedRouteRemainsVisibleWhileOtherHealthUnknown(t *testing.T) {
	status := RuntimeStatus{Routes: 2, UnservedRoutes: 1, Backends: 1, Unknown: 1}
	state, reason := deriveAvailability(status)
	if state != AvailabilityUnknown || reason != AvailabilityReasonUnservedRoutes {
		t.Fatalf("known unserved route hidden by unknown backend health: (%q, %q)", state, reason)
	}
}

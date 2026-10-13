package proxy

import (
	"testing"
	"time"

	"github.com/GoreeCloud/goreecloud-gateway/internal/config"
	"github.com/GoreeCloud/goreecloud-gateway/internal/health"
)

func TestStatusSnapshotIsAggregateAndPrivacySafe(t *testing.T) {
	now := time.Date(2026, time.August, 25, 12, 0, 0, 0, time.UTC)
	handler := New(&config.Config{
		Schema:   "goreecloud-gateway-config/v1",
		Services: []config.Service{{ID: "svc", BackendIDs: []string{"healthy", "unhealthy", "unknown"}}},
		Routes: []config.Route{
			{ID: "enabled", ServiceID: "svc", Hostname: "private.example", Enabled: true},
			{ID: "disabled", ServiceID: "svc", Hostname: "disabled.example", Enabled: false},
		},
		Backends: []config.Backend{
			{ID: "healthy", URL: "http://10.0.0.1", Enabled: true},
			{ID: "unhealthy", URL: "http://10.0.0.2", Enabled: true},
			{ID: "unknown", URL: "http://10.0.0.3", Enabled: true},
			{ID: "disabled", URL: "http://10.0.0.4", Enabled: false},
		},
	})
	defer handler.Close()

	handler.healthMu.Lock()
	handler.healthState["healthy"] = cachedHealth{result: health.Result{BackendID: "healthy", Healthy: true}, checkedAt: now}
	handler.healthState["unhealthy"] = cachedHealth{result: health.Result{BackendID: "unhealthy", Healthy: false}, checkedAt: now}
	handler.healthMu.Unlock()

	got := handler.StatusSnapshot(now)
	if got.Schema != RuntimeStatusSchemaV1 || !got.GeneratedAt.Equal(now) {
		t.Fatalf("unexpected status identity: %+v", got)
	}
	if got.Services != 1 || got.Routes != 1 || got.Backends != 3 {
		t.Fatalf("unexpected aggregate counts: %+v", got)
	}
	if got.HealthyBackends != 1 || got.Unhealthy != 1 || got.Unknown != 1 {
		t.Fatalf("unexpected health counts: %+v", got)
	}
	if got.Availability != AvailabilityDegraded || got.AvailabilityReason != AvailabilityReasonPartialBackendHealth {
		t.Fatalf("unexpected semantic availability: %+v", got)
	}
}

func TestDeriveAvailability(t *testing.T) {
	tests := []struct {
		name       string
		status     RuntimeStatus
		wantState  string
		wantReason string
	}{
		{
			name:       "inactive without enabled routes",
			status:     RuntimeStatus{Routes: 0, Backends: 2, HealthyBackends: 2},
			wantState:  AvailabilityInactive,
			wantReason: AvailabilityReasonNoActiveRoutes,
		},
		{
			name:       "unavailable without enabled backends",
			status:     RuntimeStatus{Routes: 1},
			wantState:  AvailabilityUnavailable,
			wantReason: AvailabilityReasonNoActiveBackends,
		},
		{
			name:       "unknown before health is observed",
			status:     RuntimeStatus{Routes: 1, Backends: 2, Unknown: 2},
			wantState:  AvailabilityUnknown,
			wantReason: AvailabilityReasonHealthNotObserved,
		},
		{
			name:       "available when every backend is healthy",
			status:     RuntimeStatus{Routes: 1, Backends: 2, HealthyBackends: 2},
			wantState:  AvailabilityAvailable,
			wantReason: AvailabilityReasonAllBackendsHealthy,
		},
		{
			name:       "degraded with partial health",
			status:     RuntimeStatus{Routes: 1, Backends: 3, HealthyBackends: 1, Unhealthy: 1, Unknown: 1},
			wantState:  AvailabilityDegraded,
			wantReason: AvailabilityReasonPartialBackendHealth,
		},
		{
			name:       "unknown when some backends have not been observed",
			status:     RuntimeStatus{Routes: 1, Backends: 3, Unhealthy: 2, Unknown: 1},
			wantState:  AvailabilityUnknown,
			wantReason: AvailabilityReasonHealthNotObserved,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotState, gotReason := deriveAvailability(tt.status)
			if gotState != tt.wantState || gotReason != tt.wantReason {
				t.Fatalf("deriveAvailability() = (%q, %q), want (%q, %q)", gotState, gotReason, tt.wantState, tt.wantReason)
			}
		})
	}
}

func TestActiveRouteStatusExcludesUnattachedBackends(t *testing.T) {
	now := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	handler := New(&config.Config{
		Services: []config.Service{
			{ID: "active", BackendIDs: []string{"offline"}},
			{ID: "unused", BackendIDs: []string{"online"}},
		},
		Routes: []config.Route{
			{ID: "active", ServiceID: "active", Enabled: true},
			{ID: "disabled", ServiceID: "unused", Enabled: false},
		},
		Backends: []config.Backend{
			{ID: "offline", Enabled: true},
			{ID: "online", Enabled: true},
		},
	})
	defer handler.Close()
	handler.healthMu.Lock()
	handler.healthState["offline"] = cachedHealth{result: health.Result{Healthy: false}, checkedAt: now}
	handler.healthState["online"] = cachedHealth{result: health.Result{Healthy: true}, checkedAt: now}
	handler.healthMu.Unlock()
	got := handler.StatusSnapshot(now)
	if got.Backends != 1 || got.HealthyBackends != 0 || got.Unhealthy != 1 || got.Availability != AvailabilityUnavailable {
		t.Fatalf("unrouted backend influenced availability: %+v", got)
	}
}

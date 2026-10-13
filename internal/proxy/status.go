package proxy

import "time"

const RuntimeStatusSchemaV1 = "goreecloud-gateway-runtime-status/v1"

const (
	AvailabilityUnknown     = "unknown"
	AvailabilityInactive    = "inactive"
	AvailabilityAvailable   = "available"
	AvailabilityDegraded    = "degraded"
	AvailabilityUnavailable = "unavailable"
)

const (
	AvailabilityReasonConfigurationUnavailable = "configuration_unavailable"
	AvailabilityReasonNoActiveRoutes           = "no_active_routes"
	AvailabilityReasonNoActiveBackends         = "no_active_backends"
	AvailabilityReasonHealthNotObserved        = "health_not_observed"
	AvailabilityReasonAllBackendsHealthy       = "all_backends_healthy"
	AvailabilityReasonPartialBackendHealth     = "partial_backend_health"
	AvailabilityReasonNoHealthyBackends        = "no_healthy_backends"
)

// RuntimeStatus is a privacy-safe aggregate snapshot of Gateway data-plane
// state. It intentionally excludes hostnames, backend URLs, request data,
// headers, client identifiers, credentials, and other sensitive material.
// Availability describes only Gateway service availability. It does not make
// claims about connectivity, privacy, security, or continuity.
type RuntimeStatus struct {
	Schema             string    `json:"schema"`
	GeneratedAt        time.Time `json:"generated_at"`
	Services           int       `json:"services"`
	Routes             int       `json:"routes"`
	Backends           int       `json:"backends"`
	HealthyBackends    int       `json:"healthy_backends"`
	Unhealthy          int       `json:"unhealthy_backends"`
	Unknown            int       `json:"unknown_backends"`
	Availability       string    `json:"availability"`
	AvailabilityReason string    `json:"availability_reason"`
}

// StatusSnapshot returns aggregate runtime evidence suitable for later
// Wardveil Security, Privacy Shield, Observability, and Manager adapters
// without exposing request content or infrastructure identifiers.
func (h *Handler) StatusSnapshot(now time.Time) RuntimeStatus {
	status := RuntimeStatus{
		Schema:             RuntimeStatusSchemaV1,
		GeneratedAt:        now,
		Availability:       AvailabilityUnknown,
		AvailabilityReason: AvailabilityReasonConfigurationUnavailable,
	}
	runtime := h.state.Load()
	if runtime == nil || runtime.cfg == nil {
		return status
	}
	cfg := runtime.cfg

	status.Services = len(cfg.Services)
	for _, route := range cfg.Routes {
		if route.Enabled {
			status.Routes++
		}
	}
	for _, backend := range cfg.Backends {
		if !backend.Enabled {
			continue
		}
		status.Backends++
		h.healthMu.RLock()
		healthState, ok := h.healthState[backend.ID]
		h.healthMu.RUnlock()
		if !ok || !healthState.freshAt(now) {
			status.Unknown++
			continue
		}
		if healthState.result.Healthy {
			status.HealthyBackends++
		} else {
			status.Unhealthy++
		}
	}
	status.Availability, status.AvailabilityReason = deriveAvailability(status)
	return status
}

func deriveAvailability(status RuntimeStatus) (string, string) {
	if status.Routes == 0 {
		return AvailabilityInactive, AvailabilityReasonNoActiveRoutes
	}
	if status.Backends == 0 {
		return AvailabilityUnavailable, AvailabilityReasonNoActiveBackends
	}
	if status.Unknown == status.Backends {
		return AvailabilityUnknown, AvailabilityReasonHealthNotObserved
	}
	if status.HealthyBackends == status.Backends {
		return AvailabilityAvailable, AvailabilityReasonAllBackendsHealthy
	}
	if status.HealthyBackends > 0 {
		return AvailabilityDegraded, AvailabilityReasonPartialBackendHealth
	}
	return AvailabilityUnavailable, AvailabilityReasonNoHealthyBackends
}

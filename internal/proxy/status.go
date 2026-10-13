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
	AvailabilityReasonUnservedRoutes           = "unserved_routes"
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
	UnservedRoutes     int       `json:"unserved_routes,omitempty"`
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
	activeServices := make(map[string]struct{}, len(cfg.Routes))
	for _, route := range cfg.Routes {
		if route.Enabled {
			status.Routes++
			activeServices[route.ServiceID] = struct{}{}
		}
	}
	enabledBackends := make(map[string]struct{}, len(cfg.Backends))
	for _, backend := range cfg.Backends {
		if backend.Enabled {
			enabledBackends[backend.ID] = struct{}{}
		}
	}
	serviceHasBackend := make(map[string]bool, len(activeServices))
	servingBackends := make(map[string]struct{})
	for _, service := range cfg.Services {
		if _, active := activeServices[service.ID]; !active {
			continue
		}
		for _, backendID := range service.BackendIDs {
			if _, enabled := enabledBackends[backendID]; enabled {
				serviceHasBackend[service.ID] = true
				servingBackends[backendID] = struct{}{}
			}
		}
	}
	for _, route := range cfg.Routes {
		if route.Enabled && !serviceHasBackend[route.ServiceID] {
			status.UnservedRoutes++
		}
	}
	for _, backend := range cfg.Backends {
		_, serving := servingBackends[backend.ID]
		if !backend.Enabled || !serving {
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
		if status.UnservedRoutes > 0 {
			return AvailabilityUnavailable, AvailabilityReasonUnservedRoutes
		}
		return AvailabilityUnavailable, AvailabilityReasonNoActiveBackends
	}
	// An unobserved backend might be healthy. With no known-healthy backend,
	// report uncertainty rather than claiming all routes are unavailable.
	if status.HealthyBackends == 0 && status.Unknown > 0 {
		return AvailabilityUnknown, AvailabilityReasonHealthNotObserved
	}
	if status.UnservedRoutes > 0 && status.HealthyBackends > 0 {
		return AvailabilityDegraded, AvailabilityReasonUnservedRoutes
	}
	if status.HealthyBackends == status.Backends {
		return AvailabilityAvailable, AvailabilityReasonAllBackendsHealthy
	}
	if status.HealthyBackends > 0 {
		return AvailabilityDegraded, AvailabilityReasonPartialBackendHealth
	}
	return AvailabilityUnavailable, AvailabilityReasonNoHealthyBackends
}

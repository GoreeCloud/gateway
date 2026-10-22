package routing

import (
	"net/http"
	"sort"
	"strings"

	"github.com/GoreeCloud/goreecloud-gateway/internal/config"
)

type Match struct {
	Route   config.Route
	Service config.Service
	Backend config.Backend
}

type CandidateMatch struct {
	Route    config.Route
	Service  config.Service
	Backends []config.Backend
}

func ResolveCandidates(cfg *config.Config, req *http.Request) (CandidateMatch, bool) {
	host := req.Host
	if h, _, found := strings.Cut(host, ":"); found {
		host = h
	}

	var routes []config.Route
	for _, route := range cfg.Routes {
		if route.Enabled {
			routes = append(routes, route)
		}
	}
	sort.SliceStable(routes, func(i, j int) bool {
		return len(routes[i].PathPrefix) > len(routes[j].PathPrefix)
	})

	for _, route := range routes {
		if !strings.EqualFold(host, route.Hostname) ||
			!strings.HasPrefix(req.URL.Path, normalizedPrefix(route.PathPrefix)) ||
			!methodAllowed(route.Methods, req.Method) ||
			!transportAllowed(route, req) {
			continue
		}
		if route.IsStaticResponse() {
			return CandidateMatch{Route: route}, true
		}

		service, ok := cfg.Service(route.ServiceID)
		if !ok {
			continue
		}

		candidates := make([]config.Backend, 0, len(service.BackendIDs))
		for _, id := range service.BackendIDs {
			backend, found := cfg.Backend(id)
			if found && backend.Enabled {
				candidates = append(candidates, backend)
			}
		}
		if len(candidates) > 0 {
			return CandidateMatch{Route: route, Service: service, Backends: candidates}, true
		}
	}

	return CandidateMatch{}, false
}

func Resolve(cfg *config.Config, req *http.Request) (Match, bool) {
	match, ok := ResolveCandidates(cfg, req)
	if !ok {
		return Match{}, false
	}
	if match.Route.IsStaticResponse() {
		return Match{Route: match.Route}, true
	}
	return Match{Route: match.Route, Service: match.Service, Backend: match.Backends[0]}, true
}

func normalizedPrefix(value string) string {
	if value == "" {
		return "/"
	}
	return value
}

func methodAllowed(methods []string, method string) bool {
	if len(methods) == 0 {
		return true
	}
	for _, allowed := range methods {
		if strings.EqualFold(allowed, method) {
			return true
		}
	}
	return false
}

func transportAllowed(route config.Route, req *http.Request) bool {
	if route.TLS.Mode != "required" {
		return true
	}
	return req.TLS != nil
}

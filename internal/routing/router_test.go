package routing

import (
	"net/http/httptest"
	"testing"

	"github.com/GoreeCloud/goreecloud-gateway/internal/config"
)

func TestResolvePrefersLongestPath(t *testing.T) {
	cfg := &config.Config{Schema:"goreecloud-gateway-config/v1", Services:[]config.Service{{ID:"svc", BackendIDs:[]string{"b"}}}, Backends:[]config.Backend{{ID:"b", URL:"http://127.0.0.1:9000", Enabled:true}}, Routes:[]config.Route{{ID:"root", ServiceID:"svc", Hostname:"app.goreecloud.com", PathPrefix:"/", Enabled:true},{ID:"api", ServiceID:"svc", Hostname:"app.goreecloud.com", PathPrefix:"/api", Methods:[]string{"GET"}, Enabled:true}}}
	r := httptest.NewRequest("GET", "http://app.goreecloud.com/api/health", nil)
	m, ok := Resolve(cfg, r); if !ok { t.Fatal("expected route") }; if m.Route.ID != "api" { t.Fatalf("got %s", m.Route.ID) }
}

func TestResolveRejectsWrongMethod(t *testing.T) {
	cfg := &config.Config{Schema:"goreecloud-gateway-config/v1", Services:[]config.Service{{ID:"svc", BackendIDs:[]string{"b"}}}, Backends:[]config.Backend{{ID:"b", URL:"http://127.0.0.1:9000", Enabled:true}}, Routes:[]config.Route{{ID:"api", ServiceID:"svc", Hostname:"app.goreecloud.com", PathPrefix:"/api", Methods:[]string{"GET"}, Enabled:true}}}
	r := httptest.NewRequest("POST", "http://app.goreecloud.com/api", nil)
	if _, ok := Resolve(cfg, r); ok { t.Fatal("unexpected route") }
}

func TestResolveStaticResponseWithoutBackendPrefersLongestPath(t *testing.T) {
	cfg := &config.Config{
		Schema: "goreecloud-gateway-config/v1",
		Services: []config.Service{{ID: "svc", BackendIDs: []string{"b"}}},
		Backends: []config.Backend{{ID: "b", URL: "http://127.0.0.1:9000", Enabled: true}},
		Routes: []config.Route{
			{ID: "root", ServiceID: "svc", Hostname: "app.goreecloud.com", PathPrefix: "/", Enabled: true},
			{ID: "maintenance", Action: "static_response", Hostname: "app.goreecloud.com", PathPrefix: "/maintenance", Enabled: true,
				StaticResponse: &config.StaticResponse{StatusCode: 503, Body: "unavailable"}},
		},
	}
	req := httptest.NewRequest("GET", "http://app.goreecloud.com/maintenance", nil)
	match, ok := ResolveCandidates(cfg, req)
	if !ok || match.Route.ID != "maintenance" || len(match.Backends) != 0 {
		t.Fatalf("static route was not selected independently: %+v matched=%v", match, ok)
	}
	resolved, ok := Resolve(cfg, req)
	if !ok || resolved.Route.ID != "maintenance" {
		t.Fatalf("Resolve dropped static route: %+v matched=%v", resolved, ok)
	}
}

package proxy

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/GoreeCloud/goreecloud-gateway/internal/config"
	"github.com/GoreeCloud/goreecloud-gateway/internal/health"
)

// Stale health must be rechecked only after the verified client has passed
// route authorization. Failure to re-probe must never revive a stale backend.
func TestProtectedRouteStaleHealthAuthorizationOrder(t *testing.T) {
	var upstreamHits atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		upstreamHits.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()

	backend := config.Backend{ID: "backend", URL: upstream.URL, Enabled: true}
	cfg := testConfig([]config.Backend{backend}, []string{"backend"})
	cfg.Routes = append(cfg.Routes, config.Route{
		ID: "team", ServiceID: "svc", Hostname: "app.goreecloud.com",
		PathPrefix: "/team", Methods: []string{"POST"}, Enabled: true,
		Exposure: "private", AllowedClientCIDRs: []string{"192.0.2.0/24"},
	})
	h := New(cfg)
	defer h.Close()
	checker := &fakeChecker{
		healthy: map[string]bool{"backend": false},
		checks:  map[string]int{},
	}
	h.checker = checker

	setStale := func() {
		h.healthMu.Lock()
		h.healthState[backend.ID] = cachedHealth{
			result:    health.Result{BackendID: backend.ID, Healthy: true},
			checkedAt: time.Now().Add(-2 * healthEvidenceTTL),
		}
		h.healthMu.Unlock()
	}
	request := func(method, peer string) int {
		t.Helper()
		req := httptest.NewRequest(method, "http://app.goreecloud.com/team", nil)
		req.RemoteAddr = peer
		response := httptest.NewRecorder()
		h.ServeHTTP(response, req)
		return response.Code
	}

	setStale()
	if code := request(http.MethodPost, "198.51.100.4:6000"); code != http.StatusNotFound {
		t.Fatalf("unauthorized client received %d, want 404", code)
	}
	if code := request(http.MethodGet, "192.0.2.4:6000"); code != http.StatusNotFound {
		t.Fatalf("authorized client, disallowed method received %d, want 404", code)
	}
	if got := checker.count(backend.ID); got != 0 {
		t.Fatalf("denied requests triggered %d health probes", got)
	}
	if got := upstreamHits.Load(); got != 0 {
		t.Fatalf("denied requests reached upstream %d times", got)
	}

	if code := request(http.MethodPost, "192.0.2.4:6000"); code != http.StatusServiceUnavailable {
		t.Fatalf("stale healthy backend was served: response %d, want 503", code)
	}
	if got := checker.count(backend.ID); got != 1 {
		t.Fatalf("expected one fresh health probe, got %d", got)
	}
	if upstreamHits.Load() != 0 {
		t.Fatal("unhealthy backend received proxied traffic")
	}

	checker.healthy["backend"] = true
	setStale()
	if code := request(http.MethodPost, "192.0.2.4:6000"); code != http.StatusNoContent {
		t.Fatalf("freshly approved backend response %d, want 204", code)
	}
	if got := checker.count(backend.ID); got != 2 {
		t.Fatalf("expected second fresh probe, got %d", got)
	}
	if got := upstreamHits.Load(); got != 1 {
		t.Fatalf("approved traffic reached upstream %d times, want 1", got)
	}
}

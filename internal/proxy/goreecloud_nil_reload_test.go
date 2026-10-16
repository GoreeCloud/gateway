package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/GoreeCloud/goreecloud-gateway/internal/config"
)

func TestGoreeCloudNilReloadRetainsLastGoodState(t *testing.T) {
	cfg := &config.Config{Schema: "goreecloud-gateway-config/v1"}
	h := New(cfg)
	defer h.Close()
	before := h.state.Load()
	if before == nil || before.cfg != cfg {
		t.Fatal("setup did not retain the provided config")
	}
	h.Reload(nil)
	after := h.state.Load()
	if after != before {
		t.Fatal("nil reload replaced the last known-good config")
	}
}

func TestGoreeCloudNilInitialConfigurationRespondsUnavailable(t *testing.T) {
	h := New(nil)
	defer h.Close()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://gateway.invalid/", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("nil gateway config returned %d; want 503", rr.Code)
	}
}

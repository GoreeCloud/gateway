package proxy

import (
    "context"
    "testing"

    "github.com/GoreeCloud/goreecloud-gateway/internal/config"
)

func TestGoreeCloudOldRequestCannotPublishHealthAfterReload(t *testing.T) {
    oldBackend := config.Backend{ID: "shared", URL: "http://127.0.0.1:8111", Enabled: true}
    newBackend := config.Backend{ID: "shared", URL: "http://127.0.0.1:8222", Enabled: true}
    h := New(testConfig([]config.Backend{oldBackend}, []string{"shared"}))
    defer h.Close()
    h.checker = &fakeChecker{healthy: map[string]bool{"shared": true}}

    h.Reload(testConfig([]config.Backend{newBackend}, []string{"shared"}))
    h.refreshBackends(context.Background(), []config.Backend{oldBackend})
    h.healthMu.RLock()
    _, present := h.healthState["shared"]
    h.healthMu.RUnlock()
    if present {
        t.Fatal("old request published health for backend ID reused by new config")
    }

    h.refreshBackends(context.Background(), []config.Backend{newBackend})
    h.healthMu.RLock()
    fresh, present := h.healthState["shared"]
    h.healthMu.RUnlock()
    if !present || !fresh.result.Healthy {
        t.Fatal("new backend health was not available")
    }
}

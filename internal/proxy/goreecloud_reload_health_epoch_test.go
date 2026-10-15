package proxy

import (
	"context"
	"testing"
	"time"

	"github.com/GoreeCloud/goreecloud-gateway/internal/config"
	"github.com/GoreeCloud/goreecloud-gateway/internal/health"
)

type goreecloudDelayedChecker struct {
	started chan struct{}
	unblock chan struct{}
}

func (c *goreecloudDelayedChecker) Check(_ context.Context, b config.Backend) health.Result {
	close(c.started)
	<-c.unblock
	return health.Result{BackendID: b.ID, Healthy: true}
}

func TestGoreeCloudStaleHealthProbeCannotRepublishAfterReload(t *testing.T) {
	old := testConfig([]config.Backend{{ID: "shared", URL: "http://127.0.0.1:8111", Enabled: true}}, []string{"shared"})
	h := New(old)
	defer h.Close()
	checker := &goreecloudDelayedChecker{started: make(chan struct{}), unblock: make(chan struct{})}
	h.checker = checker
	done := make(chan struct{})
	go func() {
		h.refreshBackends(context.Background(), old.Backends)
		close(done)
	}()
	select {
	case <-checker.started:
	case <-time.After(3 * time.Second):
		t.Fatal("health probe did not start")
	}
	next := testConfig([]config.Backend{{ID: "shared", URL: "http://127.0.0.1:8222", Enabled: true}}, []string{"shared"})
	h.Reload(next)
	close(checker.unblock)
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("health probe remained blocked")
	}
	h.healthMu.RLock()
	_, present := h.healthState["shared"]
	h.healthMu.RUnlock()
	if present {
		t.Fatal("health result from old configuration was published after reload")
	}

	// New probes must remain operational after discarding the stale result.
	h.checker = &fakeChecker{healthy: map[string]bool{"shared": true}}
	h.refreshBackends(context.Background(), next.Backends)
	h.healthMu.RLock()
	fresh, ok := h.healthState["shared"]
	h.healthMu.RUnlock()
	if !ok || !fresh.result.Healthy {
		t.Fatal("new configuration was unable to establish fresh health")
	}
}

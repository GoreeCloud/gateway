package proxy

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/GoreeCloud/goreecloud-gateway/internal/config"
	"github.com/GoreeCloud/goreecloud-gateway/internal/health"
)

type fakeChecker struct {
	healthy map[string]bool
	mu      sync.Mutex
	checks  map[string]int
}

func (f *fakeChecker) Check(_ context.Context, backend config.Backend) health.Result {
	f.mu.Lock()
	if f.checks != nil {
		f.checks[backend.ID]++
	}
	f.mu.Unlock()
	return health.Result{BackendID: backend.ID, Healthy: f.healthy[backend.ID]}
}

func (f *fakeChecker) count(backendID string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.checks[backendID]
}

func TestProxyRoutesAndStreams(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = io.WriteString(w, "proxied:"+r.URL.Path)
	}))
	defer upstream.Close()

	cfg := testConfig([]config.Backend{{ID: "b", URL: upstream.URL, Enabled: true, HealthPath: "/"}}, []string{"b"})
	handler := New(cfg)
	defer handler.Close()
	handler.checker = &fakeChecker{healthy: map[string]bool{"b": true}}

	req := httptest.NewRequest(http.MethodGet, "http://app.goreecloud.com/stream", nil)
	rw := httptest.NewRecorder()
	handler.ServeHTTP(rw, req)
	if rw.Code != http.StatusOK {
		t.Fatalf("status=%d", rw.Code)
	}
	if got := strings.TrimSpace(rw.Body.String()); got != "proxied:/stream" {
		t.Fatalf("body=%q", got)
	}
}

func TestHealthAwareSelectionSkipsUnhealthyBackend(t *testing.T) {
	unhealthy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("unhealthy backend received routed request")
	}))
	defer unhealthy.Close()
	ready := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "ready")
	}))
	defer ready.Close()

	cfg := testConfig([]config.Backend{
		{ID: "one", URL: unhealthy.URL, Enabled: true},
		{ID: "two", URL: ready.URL, Enabled: true},
	}, []string{"one", "two"})
	handler := New(cfg)
	defer handler.Close()
	handler.checker = &fakeChecker{healthy: map[string]bool{"one": false, "two": true}}

	rw := httptest.NewRecorder()
	handler.ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "http://app.goreecloud.com/", nil))
	if rw.Code != http.StatusOK || strings.TrimSpace(rw.Body.String()) != "ready" {
		t.Fatalf("status=%d body=%q", rw.Code, rw.Body.String())
	}
}

func TestHealthStateIsReusedAcrossRequests(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "ok")
	}))
	defer upstream.Close()

	handler := New(testConfig([]config.Backend{{ID: "one", URL: upstream.URL, Enabled: true}}, []string{"one"}))
	defer handler.Close()
	checker := &fakeChecker{healthy: map[string]bool{"one": true}, checks: map[string]int{}}
	handler.checker = checker

	for i := 0; i < 2; i++ {
		rw := httptest.NewRecorder()
		handler.ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "http://app.goreecloud.com/", nil))
		if rw.Code != http.StatusOK {
			t.Fatalf("request %d status=%d", i+1, rw.Code)
		}
	}
	if got := checker.count("one"); got != 1 {
		t.Fatalf("health checks=%d want=1", got)
	}
}

func TestRoundRobinDistributesHealthyPrimaries(t *testing.T) {
	var mu sync.Mutex
	hits := map[string]int{}
	makeBackend := func(id string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			hits[id]++
			mu.Unlock()
			_, _ = io.WriteString(w, id)
		}))
	}
	one := makeBackend("one")
	defer one.Close()
	two := makeBackend("two")
	defer two.Close()

	handler := New(testConfig([]config.Backend{
		{ID: "one", URL: one.URL, Enabled: true},
		{ID: "two", URL: two.URL, Enabled: true},
	}, []string{"one", "two"}))
	defer handler.Close()
	handler.checker = &fakeChecker{healthy: map[string]bool{"one": true, "two": true}}

	for i := 0; i < 4; i++ {
		rw := httptest.NewRecorder()
		handler.ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "http://app.goreecloud.com/", nil))
		if rw.Code != http.StatusOK {
			t.Fatalf("request %d status=%d", i+1, rw.Code)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if hits["one"] != 2 || hits["two"] != 2 {
		t.Fatalf("hits=%v", hits)
	}
}

func TestRetryableRequestFailsOverOnServiceUnavailable(t *testing.T) {
	firstHits := 0
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		firstHits++
		http.Error(w, "not ready", http.StatusServiceUnavailable)
	}))
	defer first.Close()
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "second")
	}))
	defer second.Close()

	cfg := testConfig([]config.Backend{
		{ID: "one", URL: first.URL, Enabled: true},
		{ID: "two", URL: second.URL, Enabled: true},
	}, []string{"one", "two"})
	handler := New(cfg)
	defer handler.Close()
	handler.checker = &fakeChecker{healthy: map[string]bool{"one": true, "two": true}}

	rw := httptest.NewRecorder()
	handler.ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "http://app.goreecloud.com/", nil))
	if rw.Code != http.StatusOK || strings.TrimSpace(rw.Body.String()) != "second" {
		t.Fatalf("status=%d body=%q", rw.Code, rw.Body.String())
	}
	if firstHits != 1 {
		t.Fatalf("first backend hits=%d", firstHits)
	}
}

func TestNonRetryableRequestDoesNotReplay(t *testing.T) {
	firstHits := 0
	secondHits := 0
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		firstHits++
		http.Error(w, "not ready", http.StatusServiceUnavailable)
	}))
	defer first.Close()
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secondHits++
		w.WriteHeader(http.StatusOK)
	}))
	defer second.Close()

	cfg := testConfig([]config.Backend{
		{ID: "one", URL: first.URL, Enabled: true},
		{ID: "two", URL: second.URL, Enabled: true},
	}, []string{"one", "two"})
	handler := New(cfg)
	defer handler.Close()
	handler.checker = &fakeChecker{healthy: map[string]bool{"one": true, "two": true}}

	rw := httptest.NewRecorder()
	handler.ServeHTTP(rw, httptest.NewRequest(http.MethodPost, "http://app.goreecloud.com/", strings.NewReader("payload")))
	if rw.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d", rw.Code)
	}
	if firstHits != 1 || secondHits != 0 {
		t.Fatalf("first=%d second=%d", firstHits, secondHits)
	}
}

func TestNoHealthyBackendsFailsClosed(t *testing.T) {
	cfg := testConfig([]config.Backend{{ID: "one", URL: "http://127.0.0.1:1", Enabled: true}}, []string{"one"})
	handler := New(cfg)
	defer handler.Close()
	handler.checker = &fakeChecker{healthy: map[string]bool{"one": false}}

	rw := httptest.NewRecorder()
	handler.ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "http://app.goreecloud.com/", nil))
	if rw.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d", rw.Code)
	}
}

func TestStreamingFlushesBeforeUpstreamCompletes(t *testing.T) {
	release := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("upstream response writer cannot flush")
		}
		_, _ = io.WriteString(w, "first\n")
		flusher.Flush()
		<-release
		_, _ = io.WriteString(w, "second\n")
	}))
	defer upstream.Close()

	handler := New(testConfig([]config.Backend{{ID: "one", URL: upstream.URL, Enabled: true}}, []string{"one"}))
	defer handler.Close()
	handler.checker = &fakeChecker{healthy: map[string]bool{"one": true}}
	gateway := httptest.NewServer(handler)
	defer gateway.Close()

	req, _ := http.NewRequest(http.MethodGet, gateway.URL+"/stream", nil)
	req.Host = "app.goreecloud.com"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		close(release)
		t.Fatal(err)
	}
	defer resp.Body.Close()

	lineCh := make(chan string, 1)
	go func() {
		line, _ := bufio.NewReader(resp.Body).ReadString('\n')
		lineCh <- line
	}()
	select {
	case line := <-lineCh:
		if line != "first\n" {
			close(release)
			t.Fatalf("first chunk=%q", line)
		}
	case <-time.After(2 * time.Second):
		close(release)
		t.Fatal("streaming chunk was buffered until upstream completion")
	}
	close(release)
}

func TestUpgradeConnectionIsTunneled(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.EqualFold(r.Header.Get("Upgrade"), "goreecloud-test") {
			http.Error(w, "missing upgrade", http.StatusBadRequest)
			return
		}
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			t.Fatal("upstream response writer cannot hijack")
		}
		conn, rw, err := hijacker.Hijack()
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		_, _ = rw.WriteString("HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: goreecloud-test\r\n\r\n")
		_ = rw.Flush()
		line, err := rw.ReadString('\n')
		if err != nil {
			return
		}
		_, _ = rw.WriteString("echo:" + line)
		_ = rw.Flush()
	}))
	defer upstream.Close()

	handler := New(testConfig([]config.Backend{{ID: "one", URL: upstream.URL, Enabled: true}}, []string{"one"}))
	defer handler.Close()
	handler.checker = &fakeChecker{healthy: map[string]bool{"one": true}}
	gateway := httptest.NewServer(handler)
	defer gateway.Close()

	address := strings.TrimPrefix(gateway.URL, "http://")
	conn, err := net.DialTimeout("tcp", address, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))

	_, _ = io.WriteString(conn, "GET /socket HTTP/1.1\r\nHost: app.goreecloud.com\r\nConnection: Upgrade\r\nUpgrade: goreecloud-test\r\n\r\n")
	reader := bufio.NewReader(conn)
	status, err := reader.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(status, "101") {
		t.Fatalf("upgrade status=%q", status)
	}
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if line == "\r\n" {
			break
		}
	}
	_, _ = io.WriteString(conn, "hello\n")
	echo, err := reader.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if echo != "echo:hello\n" {
		t.Fatalf("echo=%q", echo)
	}
}

func TestReloadKeepsServingNewValidatedConfig(t *testing.T) {
	one := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "one") }))
	defer one.Close()
	two := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "two") }))
	defer two.Close()

	handler := New(testConfig([]config.Backend{{ID: "b", URL: one.URL, Enabled: true}}, []string{"b"}))
	defer handler.Close()
	handler.checker = &fakeChecker{healthy: map[string]bool{"b": true}}
	handler.Reload(testConfig([]config.Backend{{ID: "b", URL: two.URL, Enabled: true}}, []string{"b"}))

	rw := httptest.NewRecorder()
	handler.ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "http://app.goreecloud.com/", nil))
	if strings.TrimSpace(rw.Body.String()) != "two" {
		t.Fatalf("body=%q", rw.Body.String())
	}
}

func testConfig(backends []config.Backend, backendIDs []string) *config.Config {
	return &config.Config{
		Schema:   "goreecloud-gateway-config/v1",
		Services: []config.Service{{ID: "svc", BackendIDs: backendIDs}},
		Backends: backends,
		Routes:   []config.Route{{ID: "r", ServiceID: "svc", Hostname: "app.goreecloud.com", PathPrefix: "/", Exposure: "public", Enabled: true}},
	}
}

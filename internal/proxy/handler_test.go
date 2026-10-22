package proxy

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/GoreeCloud/goreecloud-gateway/internal/config"
	"github.com/GoreeCloud/goreecloud-gateway/internal/health"
	"github.com/klauspost/compress/zstd"
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

func TestProxyKnownLengthOptInEncodingAndUnknownLengthPassthrough(t *testing.T) {
	body := []byte(strings.Repeat("GoreeCloud reverse proxy compression integration. ", 128))
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("ETag", "identity-representation")
		if r.URL.Path == "/unknown" {
			w.(http.Flusher).Flush() // force chunked upstream framing
		} else {
			w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		}
		_, _ = w.Write(body)
	}))
	defer upstream.Close()
	cfg := testConfig([]config.Backend{{ID: "b", URL: upstream.URL, Enabled: true}}, []string{"b"})
	cfg.Routes[0].TLS = config.RouteTLS{Mode: "disabled"}
	cfg.Routes[0].ProxyResponseEncoding = &config.ResponseEncoding{Formats: []string{"zstd", "gzip"}}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	handler := New(cfg)
	defer handler.Close()
	handler.checker = &fakeChecker{healthy: map[string]bool{"b": true}}
	for _, tc := range []struct {
		name, path, accept, want string
	}{
		{"zstd", "/known", "zstd, gzip", "zstd"},
		{"gzip", "/known", "gzip;q=1, zstd;q=0", "gzip"},
		{"unknown-length", "/unknown", "zstd, gzip", ""},
		{"client-declined", "/known", "zstd;q=0, gzip;q=0", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "http://app.goreecloud.com"+tc.path, nil)
			req.Header.Set("Accept-Encoding", tc.accept)
			rw := httptest.NewRecorder()
			handler.ServeHTTP(rw, req)
			if rw.Code != http.StatusOK || rw.Header().Get("Content-Encoding") != tc.want || rw.Header().Get("ETag") != map[bool]string{true: "identity-representation", false: ""}[tc.want == ""] {
				t.Fatalf("unexpected %s result: status=%d headers=%v", tc.name, rw.Code, rw.Header())
			}
			if !strings.Contains(rw.Header().Get("Vary"), "Accept-Encoding") {
				t.Fatalf("missing Vary: %v", rw.Header())
			}
			var decoded []byte
			var err error
			switch tc.want {
			case "gzip":
				reader, initErr := gzip.NewReader(bytes.NewReader(rw.Body.Bytes()))
				if initErr != nil { t.Fatal(initErr) }
				decoded, err = io.ReadAll(reader)
				_ = reader.Close()
			case "zstd":
				reader, initErr := zstd.NewReader(nil)
				if initErr != nil { t.Fatal(initErr) }
				decoded, err = reader.DecodeAll(rw.Body.Bytes(), nil)
				reader.Close()
			default:
				decoded = rw.Body.Bytes()
			}
			if err != nil || !bytes.Equal(decoded, body) {
				t.Fatalf("%s body mismatch: error=%v", tc.name, err)
			}
		})
	}
}

func TestProxyUnknownLengthExplicitStreamingEncoding(t *testing.T) {
	body := []byte(strings.Repeat("native unknown-length streaming. ", 120))
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.(http.Flusher).Flush() // force unknown Content-Length
		_, _ = w.Write(body)
	}))
	defer upstream.Close()
	zero := 0
	cfg := testConfig([]config.Backend{{ID: "b", URL: upstream.URL, Enabled: true}}, []string{"b"})
	cfg.Routes[0].TLS = config.RouteTLS{Mode: "disabled"}
	cfg.Routes[0].ProxyResponseEncoding = &config.ResponseEncoding{
		Formats: []string{"gzip"}, MinimumLength: &zero, AllowUnknownLength: true,
	}
	if err := cfg.Validate(); err != nil { t.Fatal(err) }
	handler := New(cfg)
	defer handler.Close()
	handler.checker = &fakeChecker{healthy: map[string]bool{"b": true}}
	req := httptest.NewRequest(http.MethodGet, "http://app.goreecloud.com/stream", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rw := httptest.NewRecorder()
	handler.ServeHTTP(rw, req)
	if rw.Code != 200 || rw.Header().Get("Content-Encoding") != "gzip" { t.Fatalf("status=%d headers=%v", rw.Code, rw.Header()) }
	reader, err := gzip.NewReader(bytes.NewReader(rw.Body.Bytes()))
	if err != nil { t.Fatal(err) }
	decoded, err := io.ReadAll(reader)
	_ = reader.Close()
	if err != nil || !bytes.Equal(decoded, body) { t.Fatalf("unknown stream did not round-trip: %v", err) }
}

func TestProxySSEHeadersAndEventsStreamWithoutEncoding(t *testing.T) {
	release := make(chan struct{})
	var releaseOnce sync.Once
	defer releaseOnce.Do(func() { close(release) })
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		_, _ = io.WriteString(w, "data: first\n\n")
		w.(http.Flusher).Flush()
		select {
		case <-release:
			_, _ = io.WriteString(w, "data: second\n\n")
		case <-r.Context().Done():
		}
	}))
	defer upstream.Close()
	zero := 0
	cfg := testConfig([]config.Backend{{ID: "b", URL: upstream.URL, Enabled: true}}, []string{"b"})
	cfg.Routes[0].TLS = config.RouteTLS{Mode: "disabled"}
	cfg.Routes[0].ProxyResponseEncoding = &config.ResponseEncoding{
		Formats: []string{"gzip"}, MinimumLength: &zero, AllowUnknownLength: true,
	}
	handler := New(cfg)
	defer handler.Close()
	handler.checker = &fakeChecker{healthy: map[string]bool{"b": true}}
	gateway := httptest.NewServer(handler)
	defer gateway.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, gateway.URL+"/events", nil)
	if err != nil { t.Fatal(err) }
	req.Host = "app.goreecloud.com"
	req.Header.Set("Accept-Encoding", "gzip")
	client := &http.Client{Transport: &http.Transport{DisableCompression: true}}
	resp, err := client.Do(req)
	if err != nil { t.Fatalf("SSE headers stalled: %v", err) }
	defer resp.Body.Close()
	if resp.StatusCode != 200 || resp.Header.Get("Content-Encoding") != "" { t.Fatalf("SSE transformed: %d %v", resp.StatusCode, resp.Header) }
	line, err := bufio.NewReader(resp.Body).ReadString('\n')
	if err != nil || line != "data: first\n" { t.Fatalf("SSE first event stalled: %q %v", line, err) }
	releaseOnce.Do(func() { close(release) })
}

func TestStaticResponseSkipsBackendAndHandlesGETAndHEAD(t *testing.T) {
	cfg := &config.Config{
		Schema: "goreecloud-gateway-config/v1",
		Routes: []config.Route{{
			ID: "maintenance", Action: "static_response", Hostname: "app.goreecloud.com",
			PathPrefix: "/", Enabled: true, TLS: config.RouteTLS{Mode: "disabled"},
			StaticResponse: &config.StaticResponse{StatusCode: 503, Body: "Maintenance\n"},
			ResponseHeaders: map[string]string{"Cache-Control": "no-store", "Content-Type": "text/plain; charset=utf-8"},
		}},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	handler := New(cfg)
	defer handler.Close()
	checker := &fakeChecker{healthy: map[string]bool{}, checks: map[string]int{}}
	handler.checker = checker

	for _, method := range []string{http.MethodGet, http.MethodHead} {
		rw := httptest.NewRecorder()
		handler.ServeHTTP(rw, httptest.NewRequest(method, "http://app.goreecloud.com/", nil))
		if rw.Code != 503 || rw.Header().Get("Cache-Control") != "no-store" || rw.Header().Get("Content-Length") != "12" {
			t.Fatalf("%s response code=%d headers=%v", method, rw.Code, rw.Header())
		}
		wantBody := "Maintenance\n"
		if method == http.MethodHead {
			wantBody = ""
		}
		if rw.Body.String() != wantBody {
			t.Fatalf("%s body=%q want=%q", method, rw.Body.String(), wantBody)
		}
	}
	if len(checker.checks) != 0 {
		t.Fatalf("static responses must never probe an upstream: %v", checker.checks)
	}
}

func TestStaticResponseOptInGzipGETAndHEAD(t *testing.T) {
	body := strings.Repeat("GoreeCloud static maintenance response. ", 40)
	cfg := &config.Config{Schema: "goreecloud-gateway-config/v1", Routes: []config.Route{{
		ID: "maintenance", Action: "static_response", Hostname: "app.goreecloud.com", PathPrefix: "/", Enabled: true,
		TLS: config.RouteTLS{Mode: "disabled"}, StaticResponse: &config.StaticResponse{StatusCode: 503, Body: body},
		ResponseEncoding: &config.ResponseEncoding{Formats: []string{"gzip"}},
	}}}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	handler := New(cfg)
	defer handler.Close()
	var contentLength string
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		req := httptest.NewRequest(method, "http://app.goreecloud.com/", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		rw := httptest.NewRecorder()
		handler.ServeHTTP(rw, req)
		if rw.Code != 503 || rw.Header().Get("Content-Encoding") != "gzip" || rw.Header().Get("Vary") != "Accept-Encoding" {
			t.Fatalf("%s status=%d headers=%v", method, rw.Code, rw.Header())
		}
		if method == http.MethodGet {
			contentLength = rw.Header().Get("Content-Length")
			reader, err := gzip.NewReader(bytes.NewReader(rw.Body.Bytes()))
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := io.ReadAll(reader)
			_ = reader.Close()
			if err != nil || string(decoded) != body {
				t.Fatalf("GET failed to decode: %v", err)
			}
		} else if rw.Body.Len() != 0 || rw.Header().Get("Content-Length") != contentLength {
			t.Fatalf("HEAD body or length mismatched GET: headers=%v", rw.Header())
		}
	}
}

func TestStaticResponseNoContentAndMalformedDirectActivation(t *testing.T) {
	cfg := &config.Config{Schema: "goreecloud-gateway-config/v1", Routes: []config.Route{{
		ID: "empty", Action: "static_response", Hostname: "app.goreecloud.com", PathPrefix: "/", Enabled: true,
		TLS: config.RouteTLS{Mode: "disabled"}, StaticResponse: &config.StaticResponse{StatusCode: 204},
	}}}
	handler := New(cfg)
	defer handler.Close()
	rw := httptest.NewRecorder()
	handler.ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "http://app.goreecloud.com/", nil))
	if rw.Code != 204 || rw.Body.Len() != 0 {
		t.Fatalf("expected an empty 204 response, got %d %q", rw.Code, rw.Body.String())
	}

	invalid := &config.Config{Schema: "goreecloud-gateway-config/v1", Routes: []config.Route{{
		ID: "bad", Action: "static_response", Hostname: "app.goreecloud.com", PathPrefix: "/", Enabled: true,
		StaticResponse: &config.StaticResponse{StatusCode: 503, Body: strings.Repeat("x", 16*1024+1)},
	}}}
	handler.Reload(invalid)
	rw = httptest.NewRecorder()
	handler.ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "http://app.goreecloud.com/", nil))
	if rw.Code != 204 {
		t.Fatalf("unsafe reload replaced the last good static response: %d", rw.Code)
	}

	unsafeHandler := New(invalid)
	defer unsafeHandler.Close()
	rw = httptest.NewRecorder()
	unsafeHandler.ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "http://app.goreecloud.com/", nil))
	if rw.Code != 503 || rw.Body.String() == strings.Repeat("x", 16*1024+1) {
		t.Fatal("malformed direct activation did not fail closed")
	}
}

func TestProxyAppliesBoundedResponseHeaderSetPolicy(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Gateway-Policy", "upstream")
		_, _ = io.WriteString(w, "original body")
	}))
	defer upstream.Close()

	cfg := testConfig([]config.Backend{{ID: "b", URL: upstream.URL, Enabled: true}}, []string{"b"})
	cfg.Routes[0].TLS = config.RouteTLS{Mode: "disabled"}
	cfg.Routes[0].ResponseHeaders = map[string]string{"X-Gateway-Policy": "overridden", "X-Content-Type-Options": "nosniff"}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	handler := New(cfg)
	defer handler.Close()
	handler.checker = &fakeChecker{healthy: map[string]bool{"b": true}}

	rw := httptest.NewRecorder()
	handler.ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "http://app.goreecloud.com/", nil))
	if rw.Code != http.StatusOK || rw.Body.String() != "original body" {
		t.Fatalf("status=%d body=%q", rw.Code, rw.Body.String())
	}
	if got := rw.Header().Get("X-Gateway-Policy"); got != "overridden" {
		t.Fatalf("response policy header=%q", got)
	}
	if got := rw.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("security header=%q", got)
	}
}

func TestProxyRejectsUnsafeResponsePolicyFromDirectCaller(t *testing.T) {
	cfg := testConfig(nil, nil)
	cfg.Routes[0].ResponseHeaders = map[string]string{"X-Unsafe": "ok\r\nInjected: yes"}
	handler := New(cfg)
	defer handler.Close()

	rw := httptest.NewRecorder()
	handler.ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "http://app.goreecloud.com/", nil))
	if rw.Code != http.StatusServiceUnavailable || rw.Header().Get("X-Unsafe") != "" {
		t.Fatalf("unsafe direct-call policy not rejected: status=%d headers=%v", rw.Code, rw.Header())
	}
}

func TestReloadRetainsKnownGoodPolicyAfterInvalidResponseHeaders(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "healthy")
	}))
	defer upstream.Close()
	cfg := testConfig([]config.Backend{{ID: "b", URL: upstream.URL, Enabled: true}}, []string{"b"})
	cfg.Routes[0].ResponseHeaders = map[string]string{"X-Policy": "original"}
	handler := New(cfg)
	defer handler.Close()
	handler.checker = &fakeChecker{healthy: map[string]bool{"b": true}}

	unsafe := testConfig([]config.Backend{{ID: "b", URL: upstream.URL, Enabled: true}}, []string{"b"})
	unsafe.Routes[0].ResponseHeaders = map[string]string{"X-Policy": "invalid\nheader"}
	handler.Reload(unsafe)
	rw := httptest.NewRecorder()
	handler.ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "http://app.goreecloud.com/", nil))
	if rw.Code != http.StatusOK || rw.Header().Get("X-Policy") != "original" {
		t.Fatalf("reload replaced known-good response policy: status=%d headers=%v", rw.Code, rw.Header())
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
		Routes:   []config.Route{{ID: "r", ServiceID: "svc", Hostname: "app.goreecloud.com", PathPrefix: "/", Enabled: true}},
	}
}

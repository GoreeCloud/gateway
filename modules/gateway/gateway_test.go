package gateway

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
)

type testHandler struct {
	fn func(http.ResponseWriter, *http.Request) error
}

func (h testHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) error {
	return h.fn(w, r)
}

func TestRequestIDReplacesUntrustedHeader(t *testing.T) {
	next := testHandler{fn: func(w http.ResponseWriter, r *http.Request) error {
		w.Header().Set("X-Upstream-ID", r.Header.Get(requestIDHeader))
		w.WriteHeader(http.StatusNoContent)
		return nil
	}}
	req := httptest.NewRequest("GET", "http://example.test/", nil)
	req.Header.Set(requestIDHeader, "forged")
	w := httptest.NewRecorder()
	if err := (RequestID{}).ServeHTTP(w, req, caddyhttp.Handler(next)); err != nil {
		t.Fatal(err)
	}
	id := w.Header().Get(requestIDHeader)
	if ok, _ := regexp.MatchString("^[0-9a-f]{32}$", id); !ok {
		t.Fatalf("bad ID %q", id)
	}
	if got := w.Header().Get("X-Upstream-ID"); got != id {
		t.Fatalf("upstream saw %q, response contained %q", got, id)
	}
	if got := req.Header.Get(requestIDHeader); got != "forged" {
		t.Fatalf("mutated original request: %q", got)
	}
}

func TestSecurityHeaders(t *testing.T) {
	req := httptest.NewRequest("GET", "http://example.test/", nil)
	w := httptest.NewRecorder()
	next := testHandler{fn: func(w http.ResponseWriter, _ *http.Request) error {
		w.WriteHeader(http.StatusNoContent)
		return nil
	}}
	if err := (SecurityHeaders{}).ServeHTTP(w, req, caddyhttp.Handler(next)); err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
	} {
		if got := w.Header().Get(name); got != value {
			t.Errorf("%s: got %q want %q", name, got, value)
		}
	}
	if got := w.Header().Get("Strict-Transport-Security"); got != "" {
		t.Fatalf("unexpected HSTS %q", got)
	}
}

func TestAllowlistIgnoresSpoofedForwardingHeader(t *testing.T) {
	m := &IPAllowlist{CIDRs: []string{"127.0.0.0/8", "2001:db8::/32"}}
	prefixes, err := compilePrefixes(m.CIDRs)
	if err != nil {
		t.Fatal(err)
	}
	m.networks = prefixes
	called := false
	next := testHandler{fn: func(w http.ResponseWriter, _ *http.Request) error {
		called = true
		w.WriteHeader(http.StatusNoContent)
		return nil
	}}
	req := httptest.NewRequest("GET", "http://example.test/admin", nil)
	req.RemoteAddr = "192.0.2.7:54321"
	req.Header.Set("X-Forwarded-For", "127.0.0.1")
	w := httptest.NewRecorder()
	if err := m.ServeHTTP(w, req, caddyhttp.Handler(next)); err != nil {
		t.Fatal(err)
	}
	if called || w.Code != http.StatusForbidden {
		t.Fatalf("forged header bypass: called=%v status=%d", called, w.Code)
	}

	called = false
	req.RemoteAddr = "127.0.0.1:1234"
	w = httptest.NewRecorder()
	if err := m.ServeHTTP(w, req, caddyhttp.Handler(next)); err != nil {
		t.Fatal(err)
	}
	if !called || w.Code != http.StatusNoContent {
		t.Fatalf("allowed request rejected: called=%v status=%d", called, w.Code)
	}
}

func TestCIDRValidation(t *testing.T) {
	for _, cidrs := range [][]string{nil, {}, {"not-a-cidr"}, {"0.0.0.0/33"}} {
		if _, err := compilePrefixes(cidrs); err == nil {
			t.Fatalf("should reject %+v", cidrs)
		}
	}
}

package gateway

import (
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/caddyserver/caddy/v2/modules/caddyhttp"
)

func TestMappedIPv4CIDRMatchesBothPeerForms(t *testing.T) {
    prefixes, err := compilePrefixes([]string{"::ffff:192.0.2.0/120"})
    if err != nil {
        t.Fatal(err)
    }
    m := &IPAllowlist{networks: prefixes}
    next := testHandler{fn: func(w http.ResponseWriter, _ *http.Request) error {
        w.WriteHeader(http.StatusNoContent)
        return nil
    }}
    for _, tc := range []struct {
        peer string
        want int
    }{
        {"192.0.2.12:1234", http.StatusNoContent},
        {"[::ffff:192.0.2.12]:1234", http.StatusNoContent},
        {"192.0.3.12:1234", http.StatusForbidden},
        {"[::1]:1234", http.StatusForbidden},
    } {
        req := httptest.NewRequest(http.MethodGet, "http://example.test/", nil)
        req.RemoteAddr = tc.peer
        req.Header.Set("X-Forwarded-For", "192.0.2.12")
        response := httptest.NewRecorder()
        if err := m.ServeHTTP(response, req, caddyhttp.Handler(next)); err != nil {
            t.Fatal(err)
        }
        if response.Code != tc.want {
            t.Errorf("peer %s: got %d, want %d", tc.peer, response.Code, tc.want)
        }
    }
}

func TestMappedIPv4CIDRDoesNotBroadenPrefix(t *testing.T) {
    prefixes, err := compilePrefixes([]string{"::ffff:192.0.2.128/121"})
    if err != nil {
        t.Fatal(err)
    }
    if !prefixes[0].Contains(prefixes[0].Addr()) {
        t.Fatal("normalized prefix does not contain network address")
    }
    if prefixes[0].Bits() != 25 || !prefixes[0].Addr().Is4() {
        t.Fatalf("unexpected mapped prefix normalization: %v", prefixes[0])
    }
}

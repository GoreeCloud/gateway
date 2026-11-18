package gateway

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/certmagic"
	"golang.org/x/crypto/bcrypt"
)

// The synthetic Caddyfile is intentionally unrelated to real tenant routing.
func TestSyntheticLegacyCaddyfileRouteSemantics(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(dir, "data"))
	oldAutosave, oldStorage := caddy.ConfigAutosavePath, caddy.DefaultStorage
	caddy.ConfigAutosavePath = filepath.Join(dir, "config", "caddy", "autosave.json")
	caddy.DefaultStorage = &certmagic.FileStorage{Path: filepath.Join(dir, "data", "caddy")}
	defer func() {
		caddy.Stop()
		caddy.ConfigAutosavePath = oldAutosave
		caddy.DefaultStorage = oldStorage
	}()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/hello" || r.URL.Query().Get("mode") != "one" {
			http.Error(w, "upstream path or query mismatch", http.StatusBadRequest)
			return
		}
		if r.Header.Get("X-Fixture-Trace") != "trace-verified" {
			http.Error(w, "upstream header missing", http.StatusBadRequest)
			return
		}
		w.Header().Set("X-Fixture-Upstream", "reached")
		_, _ = io.WriteString(w, "upstream-ok")
	}))
	defer upstream.Close()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	_ = listener.Close()

	// Fixture-only password; never interacts with production credentials.
	hash, err := bcrypt.GenerateFromPassword([]byte("fixture-only-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	caddyfileText := fmt.Sprintf(`{
 admin off
 auto_https off
}
http://%s {
 @protected path /private/*
 handle @protected {
  basic_auth {
   synthetic %s
  }
  header X-Fixture-Route private
  respond "private-ok" 200
 }
 handle /redirect {
  redir * /new-location 307
 }
 handle /app/* {
  uri strip_prefix /app
  reverse_proxy %s {
   header_up X-Fixture-Trace trace-verified
  }
 }
 handle {
  respond "fallback" 404
 }
}
`, address, string(hash), upstream.URL)
	adapter := caddyconfig.GetAdapter("caddyfile")
	if adapter == nil {
		t.Fatal("Caddyfile adapter missing")
	}
	cfg, warnings, err := adapter.Adapt(caddyfile.Format([]byte(caddyfileText)), nil)
	if err != nil || len(warnings) != 0 {
		t.Fatalf("synthetic route adaptation: %v warnings=%v", err, warnings)
	}
	if err := caddy.Load(cfg, true); err != nil {
		t.Fatalf("start local fixture Caddy: %v", err)
	}

	client := &http.Client{Timeout: 4 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	request := func(path string, authenticated bool) *http.Response {
		t.Helper()
		req, err := http.NewRequest(http.MethodGet, "http://"+address+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		if authenticated {
			req.SetBasicAuth("synthetic", "fixture-only-password")
		}
		res, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	confirm := func(path string, auth bool, code int, bodyContains string) *http.Response {
		t.Helper()
		res := request(path, auth)
		body, err := io.ReadAll(io.LimitReader(res.Body, 256))
		_ = res.Body.Close()
		if err != nil || res.StatusCode != code || !strings.Contains(string(body), bodyContains) {
			t.Fatalf("route %q: status=%d body=%q err=%v", path, res.StatusCode, body, err)
		}
		return res
	}
	proxied := confirm("/app/hello?mode=one", false, 200, "upstream-ok")
	if proxied.Header.Get("X-Fixture-Upstream") != "reached" {
		t.Fatal("proxy response headers missing")
	}
	denied := confirm("/private/data", false, 401, "")
	if !strings.HasPrefix(denied.Header.Get("WWW-Authenticate"), "Basic ") {
		t.Fatal("Basic Auth challenge missing")
	}
	accepted := confirm("/private/data", true, 200, "private-ok")
	if accepted.Header.Get("X-Fixture-Route") != "private" {
		t.Fatal("protected route response header missing")
	}
	redirect := confirm("/redirect", false, 307, "")
	if redirect.Header.Get("Location") != "/new-location" {
		t.Fatalf("redirect Location %q", redirect.Header.Get("Location"))
	}
	confirm("/unknown", false, 404, "fallback")
}

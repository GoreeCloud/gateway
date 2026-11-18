package gateway

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/certmagic"
)

// A real local Caddy engine must stop its listener after the final site is
// removed and be able to apply a new configuration in the same process.
// This fixture never binds production ports, requests public DNS or ACME,
// or reads existing Caddy configurations/certificates.
func TestIsolatedCaddyUnloadAndRecover(t *testing.T) {
	isolated := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(isolated, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(isolated, "data"))
	oldAutosave, oldStorage := caddy.ConfigAutosavePath, caddy.DefaultStorage
	caddy.ConfigAutosavePath = filepath.Join(isolated, "config", "caddy", "autosave.json")
	caddy.DefaultStorage = &certmagic.FileStorage{Path: filepath.Join(isolated, "data", "caddy")}
	defer func() {
		caddy.Stop()
		caddy.ConfigAutosavePath, caddy.DefaultStorage = oldAutosave, oldStorage
	}()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Reload-Fixture", "ok")
		_, _ = io.WriteString(w, "fixture-online")
	}))
	defer upstream.Close()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatal(err)
	}
	// The Caddyfile is limited to loopback and has no automatic HTTPS.
	source := "{\n admin off\n auto_https off\n}\nhttp://" + address +
		" {\n bind 127.0.0.1\n reverse_proxy " + upstream.URL + "\n}\n"
	adapter := caddyconfig.GetAdapter("caddyfile")
	if adapter == nil {
		t.Fatal("Caddyfile adapter unavailable")
	}
	config, warnings, err := adapter.Adapt(caddyfile.Format([]byte(source)), nil)
	if err != nil || len(warnings) != 0 {
		t.Fatalf("adapt synthetic loopback Caddyfile: %v warnings=%v", err, warnings)
	}
	load := func(cfg []byte) {
		t.Helper()
		if err := caddy.Load(cfg, true); err != nil {
			t.Fatalf("load synthetic Caddy config: %v", err)
		}
	}
	client := &http.Client{
		Timeout:   2 * time.Second,
		Transport: &http.Transport{Proxy: nil},
	}
	assertWorking := func() {
		t.Helper()
		res, err := client.Get("http://" + address + "/recovered")
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		body, err := io.ReadAll(io.LimitReader(res.Body, 256))
		if err != nil || res.StatusCode != http.StatusOK ||
			string(body) != "fixture-online" || res.Header.Get("X-Reload-Fixture") != "ok" {
			t.Fatalf("proxy did not recover: status=%d body=%q error=%v", res.StatusCode, body, err)
		}
	}

	load(config)
	assertWorking()

	// Render(nil) is the same no-sites engine configuration Manager applies
	// when the last managed site is removed. It must release the old listener.
	emptyConfig, err := Render(nil)
	if err != nil {
		t.Fatal(err)
	}
	load(emptyConfig)
	deadline := time.Now().Add(3 * time.Second)
	for {
		conn, dialErr := net.DialTimeout("tcp", address, 300*time.Millisecond)
		if dialErr != nil {
			break
		}
		_ = conn.Close()
		if time.Now().After(deadline) {
			t.Fatalf("Caddy still accepts traffic after empty config applied at %s", address)
		}
		time.Sleep(30 * time.Millisecond)
	}

	// This reapply simulates recovery from an empty configuration, not a
	// complete process crash, disk backup restoration, DNS or certificate flow.
	load(config)
	assertWorking()
}

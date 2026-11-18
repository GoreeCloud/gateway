package gateway

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidationRejectsInjection(t *testing.T) {
	tests := []Site{
		{Host: "example.com\nadmin off", Upstream: "http://127.0.0.1:8000"},
		{Host: "-bad.example", Upstream: "http://127.0.0.1:8000"},
		{Host: "example.com", Upstream: "file:///etc/passwd"},
		{Host: "example.com", Upstream: "http://user:pass@127.0.0.1:8000"},
		{Host: "example.com", Upstream: "http://127.0.0.1:8000/secret"},
		{Host: "example.com", Upstream: "http://127.0.0.1:99999"},
	}
	for _, site := range tests {
		if _, err := ValidateSite(site); err == nil {
			t.Errorf("accepted unsafe site %#v", site)
		}
	}
}
func TestCaddyRender(t *testing.T) {
	cfg, err := Render([]Site{{Host: "api.example.com", Upstream: "http://127.0.0.1:3000"}})
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(cfg) || !bytes.Contains(cfg, []byte("reverse_proxy")) {
		t.Fatalf("bad Caddy JSON: %s", cfg)
	}
}
func TestManagerAtomicCommitAndRollback(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sites.json")
	apply := func(b []byte, _ bool) error {
		if bytes.Contains(b, []byte("reject.example.com")) {
			return errors.New("simulated engine rejection")
		}
		return nil
	}
	m, err := Open(path, apply)
	if err != nil {
		t.Fatal(err)
	}
	if err = m.Start(); err != nil {
		t.Fatal(err)
	}
	if err = m.Add(Site{Host: "ok.example.com", Upstream: "http://localhost:3000"}); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0600 {
		t.Fatalf("unsafe file mode: %v", st.Mode())
	}
	if err = m.Add(Site{Host: "reject.example.com", Upstream: "http://localhost:3000"}); err == nil {
		t.Fatal("engine failure was ignored")
	}
	sites, _ := m.Snapshot()
	if len(sites) != 1 || sites[0].Host != "ok.example.com" {
		t.Fatalf("unexpected persisted sites: %#v", sites)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "reject.example.com") {
		t.Fatal("failed config was persisted")
	}
	if err = m.Delete("ok.example.com"); err != nil {
		t.Fatal(err)
	}
}

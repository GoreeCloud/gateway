package gateway

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPersistedSitesRejectAmbiguousFieldsBeforeLoader(t *testing.T) {
	cases := []string{
		`[{"host":"one.example","host":"two.example","upstream":"http://127.0.0.1:3333"}]`,
		`[{"Host":"one.example","host":"two.example","upstream":"http://127.0.0.1:3333"}]`,
		`[{"host":"one.example","UpStream":"http://127.0.0.1:3333","upstream":"http://127.0.0.1:4444"}]`,
		`[{"host":"one.example","upstream":"http://127.0.0.1:3333"}, {"host":"a.example","HOST":"b.example","upstream":"http://127.0.0.1:4444"}]`,
		`[{"host":"one.example","upstream":"http://127.0.0.1:3333"}] trailing`,
		`null`,
	}
	for _, input := range cases {
		t.Run(strings.ReplaceAll(input[:min(len(input), 35)], "/", "_"), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "sites.json")
			if err := os.WriteFile(path, []byte(input), 0600); err != nil {
				t.Fatal(err)
			}
			loaded := false
			manager, err := Open(path, func([]byte, bool) error {
				loaded = true
				return nil
			})
			if err == nil || manager != nil {
				t.Fatalf("ambiguous managed site state accepted: %s", input)
			}
			if loaded {
				t.Fatal("unsafe state reached live engine loader")
			}
			if strings.Contains(err.Error(), "127.0.0.1") || strings.Contains(err.Error(), "one.example") {
				t.Fatal("rejected managed state leaked route details in error")
			}
		})
	}
}

func TestPersistedSitesStillOpenCanonicalPrivateArray(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sites.json")
	valid := `[{"host":"Example.Test","upstream":"http://127.0.0.1:3333"}]`
	if err := os.WriteFile(path, []byte(valid), 0600); err != nil {
		t.Fatal(err)
	}
	loaderCount := 0
	manager, err := Open(path, func([]byte, bool) error {
		loaderCount++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sites, running := manager.Snapshot()
	if len(sites) != 1 || sites[0].Host != "example.test" || running {
		t.Fatalf("valid state misread: %+v running=%v", sites, running)
	}
	if err := manager.Start(); err != nil {
		t.Fatal(err)
	}
	if loaderCount != 1 {
		t.Fatalf("valid state expected one load, got %d", loaderCount)
	}
}

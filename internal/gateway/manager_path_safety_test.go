package gateway

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// All test data is disposable, and loaders are injected rather than calling
// Caddy. No sockets, live certificates, DNS, or production state are touched.
func TestOpenRejectsSitesFileUnderSymlinkedParent(t *testing.T) {
	root := t.TempDir()
	actual := filepath.Join(root, "actual")
	if err := os.Mkdir(actual, 0700); err != nil {
		t.Fatal(err)
	}
	siteFile := filepath.Join(actual, "sites.json")
	if err := os.WriteFile(siteFile, []byte("[]\n"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "alias")
	if err := os.Symlink(actual, link); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(filepath.Join(link, "sites.json"), func([]byte, bool) error { return nil }); err == nil || !strings.Contains(err.Error(), "symbolic links") {
		t.Fatalf("symlinked saved-sites parent accepted: %v", err)
	}
	if _, err := Open(siteFile, func([]byte, bool) error { return nil }); err != nil {
		t.Fatalf("legitimate saved-sites path refused: %v", err)
	}
}

func TestManagerRejectsRedirectedSitesParentBeforeApplying(t *testing.T) {
	root := t.TempDir()
	active, remote := filepath.Join(root, "active"), filepath.Join(root, "remote")
	for _, dir := range []string{active, remote} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	applied := 0
	m, err := Open(filepath.Join(active, "sites.json"), func([]byte, bool) error {
		applied++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Start(); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(active, filepath.Join(root, "moved")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(remote, active); err != nil {
		t.Fatal(err)
	}
	if err := m.Add(Site{Host: "fixture.example.test", Upstream: "http://127.0.0.1:3030"}); err == nil || !strings.Contains(err.Error(), "symbolic links") {
		t.Fatalf("redirected save path accepted: %v", err)
	}
	if applied != 0 {
		t.Fatalf("Caddy loader ran despite unsafe persistence path: calls=%d", applied)
	}
	if _, err := os.Lstat(filepath.Join(remote, "sites.json")); !os.IsNotExist(err) {
		t.Fatalf("unsafe destination was written: %v", err)
	}
	if sites, running := m.Snapshot(); len(sites) != 0 || running {
		t.Fatalf("manager changed despite refusal: sites=%v running=%v", sites, running)
	}
}

func TestManagerRollsBackIfSitesParentRedirectedDuringLoad(t *testing.T) {
	root := t.TempDir()
	active, remote := filepath.Join(root, "active"), filepath.Join(root, "remote")
	for _, dir := range []string{active, remote} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	var applied [][]byte
	loader := func(config []byte, _ bool) error {
		applied = append(applied, bytes.Clone(config))
		if len(applied) == 1 {
			if err := os.Rename(active, filepath.Join(root, "moved")); err != nil {
				return err
			}
			if err := os.Symlink(remote, active); err != nil {
				return err
			}
		}
		return nil
	}
	m, err := Open(filepath.Join(active, "sites.json"), loader)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Start(); err != nil {
		t.Fatal(err)
	}
	before, err := Render(nil)
	if err != nil {
		t.Fatal(err)
	}
	err = m.Add(Site{Host: "fixture.example.test", Upstream: "http://127.0.0.1:3030"})
	if err == nil || !strings.Contains(err.Error(), "engine rolled back") {
		t.Fatalf("expected managed config rollback after persistence path substitution: %v", err)
	}
	if len(applied) != 2 || bytes.Equal(applied[0], before) || !bytes.Equal(applied[1], before) {
		t.Fatalf("expected apply + rollback to original configuration; loader calls=%d", len(applied))
	}
	if _, err := os.Lstat(filepath.Join(remote, "sites.json")); !os.IsNotExist(err) {
		t.Fatalf("symlink destination unexpectedly modified: %v", err)
	}
	if sites, running := m.Snapshot(); len(sites) != 0 || running {
		t.Fatalf("manager changed despite rollback: sites=%v running=%v", sites, running)
	}
}

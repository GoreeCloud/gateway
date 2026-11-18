package gateway

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These recovery tests use an injected loader and disposable files. They do
// not bind 80/443, contact ACME/DNS, or read any existing Caddy state.
func TestManagerFailedPersistenceRestoresAppliedConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sites.json")
	var applied [][]byte
	loader := func(cfg []byte, _ bool) error {
		applied = append(applied, bytes.Clone(cfg))
		return nil
	}
	manager, err := Open(path, loader)
	if err != nil {
		t.Fatal(err)
	}
	if err = manager.Start(); err != nil {
		t.Fatal(err)
	}
	original, err := Render(nil)
	if err != nil {
		t.Fatal(err)
	}
	// Linux CI runs without root. Denying parent directory write permissions
	// forces saveAtomic failure *after* the simulated engine accepts the route.
	if err = os.Chmod(dir, 0500); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(dir, 0700)

	err = manager.Add(Site{Host: "fixture.example.com", Upstream: "http://127.0.0.1:3030"})
	if err == nil || !strings.Contains(err.Error(), "rolled back") {
		t.Fatalf("expected persistence rejection with rollback, got: %v", err)
	}
	if len(applied) != 2 || bytes.Equal(applied[0], original) || !bytes.Equal(applied[1], original) {
		t.Fatalf("live configuration not reverted to original (calls=%d)", len(applied))
	}
	if sites, running := manager.Snapshot(); running || len(sites) != 0 {
		t.Fatalf("failed save changed in-memory state: running=%t sites=%v", running, sites)
	}
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed save unexpectedly produced sites file: %v", err)
	}
}

func TestManagerPersistedStateCanBeReopenedWithoutSecrets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sites.json")
	loader := func([]byte, bool) error { return nil }
	first, err := Open(path, loader)
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Start(); err != nil {
		t.Fatal(err)
	}
	if err := first.Add(Site{Host: "fixture.example.com", Upstream: "https://127.0.0.1:8443"}); err != nil {
		t.Fatal(err)
	}
	stat, err := os.Stat(path)
	if err != nil || stat.Mode().Perm() != 0600 {
		t.Fatalf("saved state must be private, stat error=%v", err)
	}
	second, err := Open(path, loader)
	if err != nil {
		t.Fatal(err)
	}
	reopened, wasRunning := second.Snapshot()
	if wasRunning || len(reopened) != 1 || reopened[0].Host != "fixture.example.com" {
		t.Fatalf("unexpected pre-start imported state: running=%t sites=%v", wasRunning, reopened)
	}
	if err := second.Start(); err != nil {
		t.Fatal(err)
	}
	active, nowRunning := second.Snapshot()
	if !nowRunning || len(active) != 1 {
		t.Fatalf("reopening existing managed state failed: running=%t sites=%v", nowRunning, active)
	}
}

func TestManagerRejectsNonPrivateAndSymlinkedState(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.json")
	raw := []byte("[{\"host\":\"fixture.example.com\",\"upstream\":\"http://127.0.0.1:3030\"}]\n")
	if err := os.WriteFile(target, raw, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(target, func([]byte, bool) error { return nil }); err == nil {
		t.Fatal("accepted group/world readable site state")
	}
	if err := os.Chmod(target, 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.json")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(link, func([]byte, bool) error { return nil }); err == nil {
		t.Fatal("accepted symlinked site state")
	}
}

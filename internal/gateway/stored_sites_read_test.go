package gateway

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAnchoredSavedSiteReadRejectsReplacementAndGrowth(t *testing.T) {
	dir := t.TempDir()
	original := filepath.Join(dir, "sites.json")
	external := filepath.Join(t.TempDir(), "outside.json")
	secret := []byte("private-data-must-not-be-returned")
	if err := os.WriteFile(original, []byte("[]"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(external, secret, 0600); err != nil {
		t.Fatal(err)
	}
	r, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	checked, err := r.Lstat("sites.json")
	if err != nil {
		t.Fatal(err)
	}
	// Hold the old inode open so the filesystem cannot recycle it when
	// constructing the replacement, keeping the identity check deterministic.
	held, err := os.Open(original)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()
	if err := os.Remove(original); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, original); err != nil {
		t.Fatal(err)
	}
	content, err := readAnchoredStoredSites(r, "sites.json", checked)
	if err == nil || bytes.Contains(content, secret) {
		t.Fatal("replaced leaf symlink escaped anchored directory")
	}
	if err := os.Remove(original); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(original, secret, 0600); err != nil {
		t.Fatal(err)
	}
	content, err = readAnchoredStoredSites(r, "sites.json", checked)
	if err == nil || bytes.Contains(content, secret) {
		t.Fatal("replaced saved state inode accepted")
	}
	if err := os.WriteFile(original, bytes.Repeat([]byte("x"), maxStoredSitesBytes+1), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := readPrivateStoredSites(original); err == nil {
		t.Fatal("oversized persisted file accepted")
	}
	if err := os.WriteFile(original, []byte("[]"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(original, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := readPrivateStoredSites(original); err == nil {
		t.Fatal("public persisted file accepted")
	}
}

func TestAnchoredSavedSiteReadRejectsInPlaceSizeChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sites.json")
	if err := os.WriteFile(path, []byte("[]"), 0600); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	info, err := root.Lstat("sites.json")
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write([]byte(strings.Repeat(" ", maxStoredSitesBytes))); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := readAnchoredStoredSites(root, "sites.json", info); err == nil {
		t.Fatal("file enlarged after validation accepted")
	}
}

func TestPrivateSavedSiteAnchoredReadRemainsCompatible(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sites.json")
	content := []byte(`[{"host":"example.test","upstream":"http://127.0.0.1:3000"}]`)
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatal(err)
	}
	read, err := readPrivateStoredSites(path)
	if err != nil || !bytes.Equal(read, content) {
		t.Fatalf("valid private state rejected: %v", err)
	}
}

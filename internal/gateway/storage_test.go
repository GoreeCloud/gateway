package gateway

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/caddyserver/caddy/v2"
)

func TestPrepareManagedStorageIsPrivateAndSeparated(t *testing.T) {
	old := caddy.ConfigAutosavePath
	oldStorage := caddy.DefaultStorage
	defer func() { caddy.ConfigAutosavePath = old; caddy.DefaultStorage = oldStorage }()
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	dir := filepath.Join(t.TempDir(), "gateway")
	if err := PrepareManagedStorage(dir); err != nil {
		t.Fatal(err)
	}
	if caddy.ConfigAutosavePath != filepath.Join(dir, "autosave.json") {
		t.Fatalf("autosave points to unexpected path: %s", caddy.ConfigAutosavePath)
	}
	if os.Getenv("XDG_DATA_HOME") != filepath.Join(dir, "data") {
		t.Fatal("certificate state not isolated")
	}
	if caddy.DefaultStorage.Path != filepath.Join(dir, "data", "caddy") {
		t.Fatal("Caddy default storage not isolated")
	}
	if err := validatePrivateDirectory(dir); err != nil {
		t.Fatal(err)
	}
	if err := validatePrivateDirectory(filepath.Join(dir, "data")); err != nil {
		t.Fatal(err)
	}
}

func TestPrepareManagedStorageRejectsSharedDirectoryAndSymlink(t *testing.T) {
	parent := t.TempDir()
	shared := filepath.Join(parent, "shared")
	if err := os.Mkdir(shared, 0755); err != nil {
		t.Fatal(err)
	}
	if err := PrepareManagedStorage(shared); err == nil {
		t.Fatal("accepted group/world readable state directory")
	}
	link := filepath.Join(parent, "link")
	if err := os.Symlink(shared, link); err != nil {
		t.Fatal(err)
	}
	if err := PrepareManagedStorage(link); err == nil {
		t.Fatal("accepted symlink state directory")
	}
}

// Reject paths with a symlinked ancestor, not just symlinks in the final
// directory. Caddy's autosave and certificate settings must remain unchanged
// after validation rejects a path.
func TestPrepareManagedStorageRejectsSymlinkedAncestorWithoutGlobalChanges(t *testing.T) {
	parent := t.TempDir()
	actual := filepath.Join(parent, "actual")
	if err := os.Mkdir(actual, 0700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(parent, "alias")
	if err := os.Symlink(actual, alias); err != nil {
		t.Fatal(err)
	}
	oldAutosave, oldStorage := caddy.ConfigAutosavePath, caddy.DefaultStorage
	t.Setenv("XDG_DATA_HOME", "fixture-original-data")
	t.Setenv("XDG_CONFIG_HOME", "fixture-original-config")
	err := PrepareManagedStorage(filepath.Join(alias, "managed"))
	if err == nil {
		t.Fatal("accepted a symlinked parent in the private certificate state path")
	}
	// The early check must not create anything behind the link, even if
	// later validation would reject the redirected path.
	if _, err := os.Lstat(filepath.Join(actual, "managed")); !os.IsNotExist(err) {
		t.Fatalf("unsafe symlink ancestor caused a filesystem mutation: %v", err)
	}
	if caddy.ConfigAutosavePath != oldAutosave || caddy.DefaultStorage != oldStorage {
		t.Fatal("rejected state path mutated the Caddy globals")
	}
	if os.Getenv("XDG_DATA_HOME") != "fixture-original-data" ||
		os.Getenv("XDG_CONFIG_HOME") != "fixture-original-config" {
		t.Fatal("rejected state path mutated XDG state env")
	}
}

// Reject a symlinked *leaf* before MkdirAll creates subdirectories or alters
// inherited Caddy certificate storage settings.
func TestPrepareManagedStorageRejectsSymlinkedLeafBeforeMutation(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	if err := os.Mkdir(target, 0700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if err := PrepareManagedStorage(filepath.Join(link, "nested", "data")); err == nil {
		t.Fatal("accepted symlinked ancestor")
	}
	if _, err := os.Lstat(filepath.Join(target, "nested")); !os.IsNotExist(err) {
		t.Fatalf("created directory in redirected certificate target: %v", err)
	}
}

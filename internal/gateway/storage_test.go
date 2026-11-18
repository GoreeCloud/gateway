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

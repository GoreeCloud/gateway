package gateway

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/certmagic"
)

// PrepareManagedStorage segregates Gateway's autosaved configuration and
// certificate storage from an unrelated installation of upstream Caddy.
// Call this before caddy.Load, only from the Gateway-managed serve mode.
func PrepareManagedStorage(directory string) error {
	if directory == "" {
		return errors.New("managed state directory is required")
	}
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(absolute, 0700); err != nil {
		return fmt.Errorf("prepare Gateway state directory: %w", err)
	}
	if err := validatePrivateDirectory(absolute); err != nil {
		return err
	}
	data := filepath.Join(absolute, "data")
	if err := os.MkdirAll(data, 0700); err != nil {
		return fmt.Errorf("prepare Gateway certificate data: %w", err)
	}
	if err := validatePrivateDirectory(data); err != nil {
		return err
	}
	if err := os.Setenv("XDG_DATA_HOME", data); err != nil {
		return err
	}
	if err := os.Setenv("XDG_CONFIG_HOME", filepath.Join(absolute, "config")); err != nil {
		return err
	}
	caddy.ConfigAutosavePath = filepath.Join(absolute, "autosave.json")
	caddy.DefaultStorage = &certmagic.FileStorage{Path: filepath.Join(data, "caddy")}
	return nil
}

func validatePrivateDirectory(directory string) error {
	info, err := os.Lstat(directory)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return fmt.Errorf("Gateway state directory %q must be a private non-symlink directory (0700)", directory)
	}
	return nil
}

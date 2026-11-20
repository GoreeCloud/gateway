package gateway

import (
	"errors"
	"io"
	"os"
	"path/filepath"
)

// maxStoredSitesBytes bounds startup state reads, including if a local file
// grows after its initial stat. The caller still performs strict JSON and
// semantic validation before loading anything into Caddy.
const maxStoredSitesBytes = 256 << 10

// readPrivateStoredSites anchors the final file lookup to the inspected
// directory. Stable, trusted parent ownership remains required: this is not
// an immutable filesystem snapshot or a defense against a hostile same-UID
// process modifying in-place bytes while they are being read.
func readPrivateStoredSites(path string) ([]byte, error) {
	dir, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return nil, errors.New("cannot open private sites directory")
	}
	defer dir.Close()
	name := filepath.Base(path)
	info, err := dir.Lstat(name)
	if err != nil {
		return nil, err
	}
	return readAnchoredStoredSites(dir, name, info)
}

// readAnchoredStoredSites revalidates the opened inode identity, permissions,
// and bounded content, instead of trusting a stat followed by os.ReadFile.
// It never follows a symlink outside the anchored directory.
func readAnchoredStoredSites(dir *os.Root, name string, expected os.FileInfo) ([]byte, error) {
	if expected == nil || !expected.Mode().IsRegular() {
		return nil, errors.New("sites file must be a regular, non-symlink file")
	}
	if expected.Mode().Perm()&0077 != 0 {
		return nil, errors.New("sites file must not be accessible by group or others; chmod 600")
	}
	if expected.Size() < 0 || expected.Size() > maxStoredSitesBytes {
		return nil, errors.New("sites file too large")
	}
	f, err := dir.Open(name)
	if err != nil {
		return nil, errors.New("sites file changed during open")
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(expected, opened) ||
		opened.Mode().Perm()&0077 != 0 || opened.Size() < 0 || opened.Size() > maxStoredSitesBytes {
		return nil, errors.New("sites file changed or became unsafe during open")
	}
	data, err := io.ReadAll(io.LimitReader(f, maxStoredSitesBytes+1))
	if err != nil || len(data) > maxStoredSitesBytes {
		return nil, errors.New("sites file unreadable or too large")
	}
	after, err := f.Stat()
	if err != nil || !os.SameFile(expected, after) || after.Size() != int64(len(data)) ||
		after.Mode().Perm()&0077 != 0 {
		return nil, errors.New("sites file changed during read")
	}
	return data, nil
}

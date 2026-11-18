package gateway

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/caddyserver/caddy/v2"
)

type Loader func([]byte, bool) error

type Manager struct {
	mu       sync.Mutex
	path     string
	sites    []Site
	previous []byte
	running  bool
	load     Loader
}

func Open(path string, loader Loader) (*Manager, error) {
	if loader == nil {
		loader = caddy.Load
	}
	if path == "" {
		return nil, errors.New("site state path is required")
	}
	m := &Manager{path: path, load: loader}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return m, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("sites file must be a regular, non-symlink file")
	}
	if info.Mode().Perm()&0077 != 0 {
		return nil, errors.New("sites file must not be accessible by group or others; chmod 600")
	}
	if info.Size() > 256<<10 {
		return nil, errors.New("sites file too large")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var sites []Site
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&sites); err != nil {
		return nil, fmt.Errorf("invalid sites file: %w", err)
	}
	if err = decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, errors.New("unexpected trailing data in sites file")
	}
	m.sites, err = Normalize(sites)
	return m, err
}

func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cfg, err := Render(m.sites)
	if err != nil {
		return err
	}
	if len(m.sites) > 0 {
		if err = m.load(cfg, true); err != nil {
			return err
		}
		m.running = true
	}
	m.previous = cfg
	return nil
}

func (m *Manager) Snapshot() ([]Site, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := append([]Site(nil), m.sites...)
	return out, m.running
}

func (m *Manager) Add(site Site) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	next := append(append([]Site(nil), m.sites...), site)
	return m.change(next)
}

func (m *Manager) Update(oldHost string, site Site) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	next := append([]Site(nil), m.sites...)
	found := false
	for i := range next {
		if next[i].Host == oldHost {
			next[i] = site
			found = true
			break
		}
	}
	if !found {
		return ErrNotFound
	}
	return m.change(next)
}

func (m *Manager) Delete(host string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	next := make([]Site, 0, len(m.sites))
	found := false
	for _, s := range m.sites {
		if s.Host == host {
			found = true
			continue
		}
		next = append(next, s)
	}
	if !found {
		return ErrNotFound
	}
	return m.change(next)
}

var ErrNotFound = errors.New("site not found")

func (m *Manager) change(next []Site) error {
	clean, err := Normalize(next)
	if err != nil {
		return err
	}
	cfg, err := Render(clean)
	if err != nil {
		return err
	}
	if err := m.load(cfg, true); err != nil {
		return fmt.Errorf("engine refused configuration; no changes saved: %w", err)
	}
	if err := saveAtomic(m.path, clean); err != nil {
		// Revert the live process if persistence failed.
		rollback := m.load(m.previous, true)
		if rollback != nil {
			return fmt.Errorf("failed to save (%v) and engine rollback failed (%v)", err, rollback)
		}
		return fmt.Errorf("failed to save; engine rolled back: %w", err)
	}
	m.previous = cfg
	m.sites = clean
	m.running = len(clean) > 0
	return nil
}

func saveAtomic(path string, sites []Site) error {
	dir := filepath.Dir(path)
	data, err := json.MarshalIndent(sites, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	f, err := os.CreateTemp(dir, ".gateway-sites-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if err = f.Chmod(0600); err != nil {
		return err
	}
	if _, err = f.Write(data); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Rename(f.Name(), path); err != nil {
		return err
	}
	// Flush the directory entry on platforms that support directory sync.
	if d, e := os.Open(dir); e == nil {
		_ = d.Sync()
		_ = d.Close()
	}
	return nil
}

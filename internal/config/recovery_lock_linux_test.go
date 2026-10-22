//go:build linux

package config

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestConcurrentRecoveryRestoreSerializesExpectedDigest(t *testing.T) {
	root := t.TempDir()
	active := filepath.Join(root, "active", "gateway.json")
	writeRecoveryTestConfig(t, active, "first.acceptance.test")
	snapshotDir, _, err := CreateRecoverySnapshot(root, active, recoveryTestRevision, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	writeRecoveryTestConfig(t, active, "second.acceptance.test")
	before, err := os.ReadFile(active)
	if err != nil {
		t.Fatal(err)
	}
	expected := recoveryTestSHA(before)

	const attempts = 12
	start := make(chan struct{})
	results := make(chan error, attempts)
	var workers sync.WaitGroup
	for i := 0; i < attempts; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			_, restoreErr := RestoreRecoverySnapshot(root, active, snapshotDir, expected, time.Now().UTC())
			results <- restoreErr
		}()
	}
	close(start)
	workers.Wait()
	close(results)

	succeeded := 0
	for result := range results {
		if result == nil {
			succeeded++
		}
	}
	if succeeded != 1 {
		t.Fatalf("expected exactly one restore to pass the active-config CAS, got %d", succeeded)
	}
	final, err := Load(active)
	if err != nil {
		t.Fatal(err)
	}
	if got := final.Routes[0].Hostname; got != "first.acceptance.test" {
		t.Fatalf("restore ended at incorrect hostname %q", got)
	}
}

func TestRecoveryLockRejectsSymlink(t *testing.T) {
	root := t.TempDir()
	active := filepath.Join(root, "active", "gateway.json")
	writeRecoveryTestConfig(t, active, "first.acceptance.test")
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("unaltered"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, ".goreecloud-gateway-recovery.lock")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, _, err := CreateRecoverySnapshot(root, active, recoveryTestRevision, time.Now().UTC()); err == nil {
		t.Fatal("snapshot accepted a symlink process-lock path")
	}
	actual, err := os.ReadFile(outside)
	if err != nil {
		t.Fatal(err)
	}
	if string(actual) != "unaltered" {
		t.Fatal("symlink target was modified")
	}
}

func TestAtomicWriteFsyncAndPermissions(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "active", "gateway.json")
	if err := atomicWrite(path, []byte("one"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := atomicWrite(path, []byte("two"), 0o600); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "two" {
		t.Fatalf("atomic replacement was not observable: %q", data)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("atomic replacement lost private mode: %v", info.Mode())
	}
	matches, err := filepath.Glob(filepath.Join(root, "active", ".goreecloud-gateway-recovery-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("atomicWrite leaked %d temporary files", len(matches))
	}
}

//go:build linux

package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// withRecoveryLock serializes cooperative Gateway configuration lifecycle
// operations across processes on the same Linux filesystem. The operating
// system releases the advisory lock on process termination; unlike an O_EXCL
// sentinel, an interrupted process cannot strand a stale lock indefinitely.
// Other processes editing the active file outside these APIs are not covered.
func withRecoveryLock(root string, run func() error) error {
	lockPath := filepath.Join(root, ".goreecloud-gateway-recovery.lock")
	fd, err := syscall.Open(lockPath, syscall.O_CREAT|syscall.O_RDWR|syscall.O_CLOEXEC|syscall.O_NOFOLLOW, 0o600)
	if err != nil {
		return fmt.Errorf("gateway recovery: open process lock: %w", err)
	}
	lockFile := os.NewFile(uintptr(fd), lockPath)
	defer lockFile.Close()
	info, err := lockFile.Stat()
	if err != nil {
		return fmt.Errorf("gateway recovery: inspect process lock: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 {
		return errors.New("gateway recovery: process lock must be an owner-only regular file")
	}
	for {
		err = syscall.Flock(fd, syscall.LOCK_EX)
		if !errors.Is(err, syscall.EINTR) {
			break
		}
	}
	if err != nil {
		return fmt.Errorf("gateway recovery: acquire process lock: %w", err)
	}
	defer syscall.Flock(fd, syscall.LOCK_UN)
	return run()
}

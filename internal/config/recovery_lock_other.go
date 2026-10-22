//go:build !linux

package config

import "errors"

// The current server recovery protocol is qualified only for Linux. Failing
// closed avoids silently downgrading compare-and-swap safety on other systems.
func withRecoveryLock(_ string, _ func() error) error {
	return errors.New("gateway recovery: process-shared locking is qualified only for Linux")
}

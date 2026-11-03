package main

import (
	"net"
	"net/netip"
	"sync"
	"time"
)

const (
	authFailureLimit = 8
	authWindow       = time.Minute
	maxAuthPeers     = 256
)

type failureWindow struct {
	start time.Time
	count int
}

// failedAuthLimiter bounds invalid-token attempts by the actual TCP peer.
// Forwarded headers are intentionally never used as authentication evidence.
type failedAuthLimiter struct {
	mu      sync.Mutex
	entries map[string]failureWindow
	now     func() time.Time
}

func newFailedAuthLimiter(clock func() time.Time) *failedAuthLimiter {
	return &failedAuthLimiter{
		entries: make(map[string]failureWindow),
		now:     clock,
	}
}

func peerKey(remote string) string {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		return "unknown"
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return "unknown"
	}
	return ip.Unmap().String()
}

// allowFailure records one failed authentication and reports whether the
// caller is still below the rejection threshold. Valid tokens bypass this
// limiter so spoofing callers cannot deny service to an authorized operator.
func (l *failedAuthLimiter) allowFailure(remote string) bool {
	key := peerKey(remote)
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	record, exists := l.entries[key]
	if exists && now.Sub(record.start) >= authWindow {
		delete(l.entries, key)
		exists = false
	}
	if !exists {
		if len(l.entries) >= maxAuthPeers {
			for peer, value := range l.entries {
				if now.Sub(value.start) >= authWindow {
					delete(l.entries, peer)
				}
			}
		}
		if len(l.entries) >= maxAuthPeers {
			key = "overflow"
		}
		record = l.entries[key]
		if record.start.IsZero() || now.Sub(record.start) >= authWindow {
			record = failureWindow{start: now}
		}
	}
	record.count++
	l.entries[key] = record
	return record.count <= authFailureLimit
}

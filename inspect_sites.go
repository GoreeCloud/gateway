package main

import (
	"encoding/json"
	"errors"
	"io"
	"os"

	"github.com/GoreeCloud/gateway/internal/gateway"
)

// inspectSites verifies private saved route state without starting Caddy,
// binding a listener, contacting an upstream or writing the original file.
// The output intentionally excludes hostnames, upstreams, tokens and paths.
// Offline validation cannot demonstrate restoration or live TLS behavior.
func inspectSites(args []string, out io.Writer) error {
	if len(args) != 2 || args[0] != "--file" || args[1] == "" {
		return errors.New("usage: gateway inspect-sites --file PRIVATE_SITES_JSON")
	}
	// Startup allows a missing file for first boot; recovery inspection must
	// instead require an existing private artifact to avoid a false success.
	if _, err := os.Lstat(args[1]); err != nil {
		return errors.New("saved-site recovery candidate missing or inaccessible")
	}
	manager, err := gateway.Open(args[1], func([]byte, bool) error {
		return errors.New("offline inspection must not load Caddy")
	})
	if err != nil {
		return errors.New("private saved-site recovery candidate rejected")
	}
	sites, running := manager.Snapshot()
	if running {
		return errors.New("unexpected running state during offline inspection")
	}
	if _, err := gateway.Render(sites); err != nil {
		return errors.New("saved-site recovery candidate rejected by Caddy adapter")
	}
	return json.NewEncoder(out).Encode(struct {
		ReadOnly        bool   `json:"readOnly"`
		Valid           bool   `json:"valid"`
		CaddyAdapted    bool   `json:"caddyAdapted"`
		SiteCount       int    `json:"siteCount"`
		Applied         bool   `json:"applied"`
		RestoreTested   bool   `json:"restoreTested"`
		ProductionReady bool   `json:"productionReady"`
		Reason          string `json:"reason"`
	}{true, true, true, len(sites), false, false, false, "offline_source_only"})
}

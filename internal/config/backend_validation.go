package config

import (
	"errors"
	"net/url"
	"strconv"
	"strings"
)

// validateBackendEndpoint keeps Gateway's runtime transport boundary explicit.
// Backend endpoints are configuration, not request input, but invalid or
// credential-bearing URLs must still fail before routing or health checks run.
func validateBackendEndpoint(backend Backend) error {
	rawURL := backend.URL
	if rawURL != strings.TrimSpace(rawURL) {
		return errors.New("backend URL must not contain surrounding whitespace")
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return errors.New("backend URL is invalid")
	}
	if parsed.Opaque != "" || parsed.Host == "" || parsed.Hostname() == "" {
		return errors.New("backend URL must be an absolute network URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("backend URL scheme must be http or https")
	}
	if parsed.User != nil {
		return errors.New("backend URL must not contain embedded credentials")
	}
	if strings.HasSuffix(parsed.Host, ":") {
		return errors.New("backend URL must not contain an empty explicit port")
	}
	if portText := parsed.Port(); portText != "" {
		port, portErr := strconv.Atoi(portText)
		if portErr != nil || port < 1 || port > 65535 {
			return errors.New("backend URL port must be between 1 and 65535")
		}
	}
	// An explicit trailing '#' is a fragment delimiter even though
	// net/url reports an empty Fragment value for that syntax.
	if parsed.Fragment != "" || strings.Contains(rawURL, "#") {
		return errors.New("backend URL must not contain a fragment")
	}
	if parsed.RawQuery != "" || parsed.ForceQuery {
		return errors.New("backend URL must not contain a query")
	}

	healthPath := backend.HealthPath
	if healthPath == "" {
		return nil
	}
	if healthPath != strings.TrimSpace(healthPath) || !strings.HasPrefix(healthPath, "/") ||
		strings.HasPrefix(healthPath, "//") || strings.ContainsAny(healthPath, "?#") {
		return errors.New("backend health path must be an absolute path without query or fragment")
	}
	return nil
}

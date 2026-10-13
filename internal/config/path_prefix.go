package config

import (
	"fmt"
	"strings"
)

// validateRoutePathPrefix requires a single, unambiguous URL path boundary.
// Empty is retained as a legacy alias for "/"; the router normalizes it.
func validateRoutePathPrefix(prefix string) error {
	if prefix == "" || prefix == "/" {
		return nil
	}
	if !strings.HasPrefix(prefix, "/") || strings.Contains(prefix, "//") || strings.ContainsAny(prefix, "\\?#%") {
		return fmt.Errorf("path prefix must be an absolute, unescaped, segment-normalized URL path")
	}
	for _, segment := range strings.Split(strings.Trim(prefix, "/"), "/") {
		if segment == "." || segment == ".." {
			return fmt.Errorf("path prefix must not contain dot segments")
		}
	}
	for _, r := range prefix {
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("path prefix must not contain control characters")
		}
	}
	return nil
}

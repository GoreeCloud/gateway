package gateway

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/caddyserver/caddy/v2/caddyconfig"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	_ "github.com/caddyserver/caddy/v2/modules/standard"
)

type Site struct {
	Host     string `json:"host"`
	Upstream string `json:"upstream"`
}

var dnsLabel = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)

func ValidateSite(s Site) (Site, error) {
	s.Host = strings.ToLower(strings.TrimSpace(s.Host))
	if len(s.Host) < 1 || len(s.Host) > 253 || strings.Contains(s.Host, "..") {
		return Site{}, errors.New("host must be a valid DNS name")
	}
	labels := strings.Split(s.Host, ".")
	if len(labels) < 2 && s.Host != "localhost" {
		return Site{}, errors.New("host must be a fully qualified domain name or localhost")
	}
	for _, label := range labels {
		if !dnsLabel.MatchString(label) {
			return Site{}, errors.New("host contains an invalid DNS label")
		}
	}
	u, err := url.Parse(s.Upstream)
	if err != nil || u == nil || (u.Scheme != "http" && u.Scheme != "https") ||
		u.Opaque != "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" ||
		(u.Path != "" && u.Path != "/") || u.RawPath != "" {
		return Site{}, errors.New("upstream must be an http(s) URL without credentials, path, query or fragment")
	}
	if u.Hostname() == "" || strings.ContainsAny(u.Hostname(), " \t\n\r") {
		return Site{}, errors.New("upstream hostname is invalid")
	}
	if net.ParseIP(u.Hostname()) == nil {
		hostname := strings.ToLower(u.Hostname())
		for _, l := range strings.Split(hostname, ".") {
			if !dnsLabel.MatchString(l) {
				return Site{}, errors.New("upstream hostname is invalid")
			}
		}
	}
	port := u.Port()
	if port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return Site{}, errors.New("upstream port is invalid")
		}
	}
	s.Upstream = u.Scheme + "://" + u.Host
	return s, nil
}

func Normalize(sites []Site) ([]Site, error) {
	seen := map[string]bool{}
	out := make([]Site, 0, len(sites))
	if len(sites) > 1000 {
		return nil, errors.New("maximum 1000 sites")
	}
	for _, s := range sites {
		clean, err := ValidateSite(s)
		if err != nil {
			return nil, fmt.Errorf("site %q: %w", s.Host, err)
		}
		if seen[clean.Host] {
			return nil, fmt.Errorf("duplicate site: %s", clean.Host)
		}
		seen[clean.Host] = true
		out = append(out, clean)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Host < out[j].Host })
	return out, nil
}

// Render uses Caddy's real Caddyfile adapter, not an independent proxy implementation.
// All interpolated data has passed strict single-line validation.
func Render(sites []Site) ([]byte, error) {
	clean, err := Normalize(sites)
	if err != nil {
		return nil, err
	}
	if len(clean) == 0 {
		return []byte(`{"admin":{"disabled":true}}`), nil
	}
	var b strings.Builder
	b.WriteString("{\n  admin off\n}\n\n")
	for _, site := range clean {
		fmt.Fprintf(&b, "%s {\n  encode zstd gzip\n  reverse_proxy %s\n}\n\n", site.Host, site.Upstream)
	}
	adapter := caddyconfig.GetAdapter("caddyfile")
	if adapter == nil {
		return nil, errors.New("Caddyfile adapter not registered")
	}
	jsonConfig, warnings, err := adapter.Adapt(caddyfile.Format([]byte(b.String())), nil)
	if err != nil {
		return nil, fmt.Errorf("Caddy could not adapt configuration: %w", err)
	}
	if len(warnings) > 0 {
		return nil, fmt.Errorf("Caddy adaptation warning: %v", warnings)
	}
	if !json.Valid(jsonConfig) {
		return nil, errors.New("adapter returned invalid JSON")
	}
	return jsonConfig, nil
}

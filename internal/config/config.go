package config

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
)

type Config struct {
	Schema              string               `json:"schema"`
	TrustedProxies      []string             `json:"trusted_proxies,omitempty"`
	Services            []Service            `json:"services"`
	Routes              []Route              `json:"routes"`
	Backends            []Backend            `json:"backends"`
	CertificateProfiles []CertificateProfile `json:"certificate_profiles,omitempty"`
}

type Service struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	BackendIDs []string `json:"backend_ids"`
}

type Route struct {
	ID              string            `json:"id"`
	ServiceID       string            `json:"service_id"`
	Hostname        string            `json:"hostname"`
	PathPrefix      string            `json:"path_prefix"`
	Methods         []string          `json:"methods"`
	Exposure        string            `json:"exposure"`
	Enabled         bool              `json:"enabled"`
	TLS             RouteTLS          `json:"tls"`
	ResponseHeaders map[string]string `json:"response_headers,omitempty"`
}

type RouteTLS struct {
	Mode               string `json:"mode"`
	CertificateProfile string `json:"certificate_profile,omitempty"`
}

type CertificateProfile struct {
	ID              string `json:"id"`
	CertificateFile string `json:"certificate_file"`
	PrivateKeyFile  string `json:"private_key_file"`
	Enabled         bool   `json:"enabled"`
}

type Backend struct {
	ID         string `json:"id"`
	URL        string `json:"url"`
	Enabled    bool   `json:"enabled"`
	HealthPath string `json:"health_path"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) Validate() error {
	if c.Schema != "goreecloud-gateway-config/v1" {
		return fmt.Errorf("unsupported schema %q", c.Schema)
	}
	if _, err := ParseTrustedProxyPrefixes(c.TrustedProxies); err != nil {
		return fmt.Errorf("trusted proxy configuration: %w", err)
	}

	services := map[string]Service{}
	backends := map[string]Backend{}
	profiles := map[string]CertificateProfile{}

	for _, profile := range c.CertificateProfiles {
		if strings.TrimSpace(profile.ID) == "" {
			return fmt.Errorf("certificate profile id is required")
		}
		if _, ok := profiles[profile.ID]; ok {
			return fmt.Errorf("duplicate certificate profile %q", profile.ID)
		}
		if profile.Enabled {
			if strings.TrimSpace(profile.CertificateFile) == "" || strings.TrimSpace(profile.PrivateKeyFile) == "" {
				return fmt.Errorf("enabled certificate profile %q requires certificate and private-key file references", profile.ID)
			}
		}
		profiles[profile.ID] = profile
	}

	for _, b := range c.Backends {
		if strings.TrimSpace(b.ID) == "" || strings.TrimSpace(b.URL) == "" {
			return fmt.Errorf("backend id and url are required")
		}
		if _, ok := backends[b.ID]; ok {
			return fmt.Errorf("duplicate backend %q", b.ID)
		}
		if err := validateBackendEndpoint(b); err != nil {
			return fmt.Errorf("backend %q: %w", b.ID, err)
		}
		backends[b.ID] = b
	}
	for _, s := range c.Services {
		if s.ID == "" {
			return fmt.Errorf("service id is required")
		}
		if _, ok := services[s.ID]; ok {
			return fmt.Errorf("duplicate service %q", s.ID)
		}
		for _, id := range s.BackendIDs {
			if _, ok := backends[id]; !ok {
				return fmt.Errorf("service %q references missing backend %q", s.ID, id)
			}
		}
		services[s.ID] = s
	}
	if err := c.ValidateResponseHeaderPolicies(); err != nil {
		return err
	}
	seen := map[string]string{}
	for _, r := range c.Routes {
		if !r.Enabled {
			continue
		}
		if _, ok := services[r.ServiceID]; !ok {
			return fmt.Errorf("route %q references missing service %q", r.ID, r.ServiceID)
		}
		if r.Hostname == "" {
			return fmt.Errorf("route %q hostname is required", r.ID)
		}
		if r.PathPrefix == "" {
			r.PathPrefix = "/"
		}
		switch r.TLS.Mode {
		case "required":
			profileID := strings.TrimSpace(r.TLS.CertificateProfile)
			if profileID == "" {
				return fmt.Errorf("route %q requires a TLS certificate profile", r.ID)
			}
			profile, ok := profiles[profileID]
			if !ok {
				return fmt.Errorf("route %q references missing certificate profile %q", r.ID, profileID)
			}
			if !profile.Enabled {
				return fmt.Errorf("route %q references disabled certificate profile %q", r.ID, profileID)
			}
		case "disabled":
			if strings.TrimSpace(r.TLS.CertificateProfile) != "" {
				return fmt.Errorf("route %q cannot set a TLS certificate profile when TLS is disabled", r.ID)
			}
		default:
			return fmt.Errorf("route %q has unsupported TLS mode %q", r.ID, r.TLS.Mode)
		}
		key := strings.ToLower(r.Hostname) + "\x00" + r.PathPrefix + "\x00" + strings.Join(r.Methods, ",")
		if prior, ok := seen[key]; ok {
			return fmt.Errorf("route %q conflicts with %q", r.ID, prior)
		}
		seen[key] = r.ID
	}
	return nil
}

// ValidateResponseHeaderPolicies is separately enforced at runtime-state
// activation, including for callers that bypass Config.Load.
func (c *Config) ValidateResponseHeaderPolicies() error {
	if c == nil {
		return fmt.Errorf("gateway configuration is required")
	}
	for _, route := range c.Routes {
		if err := validateResponseHeaders(route.ResponseHeaders); err != nil {
			return fmt.Errorf("route %q response headers: %w", route.ID, err)
		}
	}
	return nil
}

// validateResponseHeaders bounds operator-owned *set* operations. Hop-by-hop
// and framing fields belong to net/http rather than route configuration.
func validateResponseHeaders(headers map[string]string) error {
	if len(headers) > 64 {
		return fmt.Errorf("at most 64 response headers are permitted")
	}
	seen := make(map[string]bool, len(headers))
	total := 0
	for name, value := range headers {
		if len(name) == 0 || len(name) > 128 {
			return fmt.Errorf("invalid response header name length")
		}
		for i := 0; i < len(name); i++ {
			b := name[i]
			if !((b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') ||
				(b >= '0' && b <= '9') || strings.ContainsRune("!#$%&'*+-.^_`|~", rune(b))) {
				return fmt.Errorf("invalid response header name")
			}
		}
		canonical := http.CanonicalHeaderKey(name)
		if seen[canonical] {
			return fmt.Errorf("duplicate response header %q", canonical)
		}
		seen[canonical] = true
		switch canonical {
		case "Connection", "Keep-Alive", "Proxy-Authenticate", "Proxy-Authorization", "Te", "Trailer", "Transfer-Encoding", "Upgrade", "Content-Length", "Content-Encoding":
			return fmt.Errorf("response header %q controls connection framing or content coding", canonical)
		}
		if len(value) > 4096 {
			return fmt.Errorf("response header %q exceeds value limit", canonical)
		}
		for i := 0; i < len(value); i++ {
			if value[i] == 0x7f || (value[i] < 0x20 && value[i] != '\t') {
				return fmt.Errorf("response header %q contains control characters", canonical)
			}
		}
		total += len(name) + len(value)
		if total > 16384 {
			return fmt.Errorf("response headers exceed total size limit")
		}
	}
	return nil
}

func (c *Config) Service(id string) (Service, bool) {
	for _, s := range c.Services {
		if s.ID == id {
			return s, true
		}
	}
	return Service{}, false
}

func (c *Config) Backend(id string) (Backend, bool) {
	for _, b := range c.Backends {
		if b.ID == id {
			return b, true
		}
	}
	return Backend{}, false
}

func (c *Config) CertificateProfile(id string) (CertificateProfile, bool) {
	for _, profile := range c.CertificateProfiles {
		if profile.ID == id {
			return profile, true
		}
	}
	return CertificateProfile{}, false
}

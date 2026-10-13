package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"net/netip"
	"strings"
	"time"
)

const ConfigParityEvidenceSchemaV1 = "goreecloud-gateway-config-parity-evidence/v1"

// ConfigParityEvidence records whether one exact validated Gateway
// configuration matches an operator-reviewed migration-candidate fingerprint.
// It deliberately contains only aggregate counts and immutable identities; it
// does not expose hostnames, backend URLs, certificate paths, credentials, or
// route contents.
type ConfigParityEvidence struct {
	Schema                      string `json:"schema"`
	RecordedAt                  string `json:"recorded_at"`
	SourceRevision              string `json:"source_revision"`
	ExpectedConfigSHA256        string `json:"expected_config_sha256"`
	ActualConfigSHA256          string `json:"actual_config_sha256"`
	ServiceCount                int    `json:"service_count"`
	RouteCount                  int    `json:"route_count"`
	BackendCount                int    `json:"backend_count"`
	CertificateProfileCount     int    `json:"certificate_profile_count"`
	ConfigurationParityMatched  bool   `json:"configuration_parity_matched"`
	ProductionCutoverAuthorized bool   `json:"production_cutover_authorized"`
}

type parityService struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	BackendIDs []string `json:"backend_ids"`
}

type parityRoute struct {
	ID         string   `json:"id"`
	ServiceID  string   `json:"service_id"`
	Hostname   string   `json:"hostname"`
	PathPrefix string   `json:"path_prefix"`
	Methods    []string `json:"methods"`
	Exposure   string   `json:"exposure"`
	AllowedClientCIDRs []string `json:"allowed_client_cidrs,omitempty"`
	Enabled    bool     `json:"enabled"`
	TLSMode    string   `json:"tls_mode"`
	TLSProfile string   `json:"tls_profile,omitempty"`
}

type parityBackend struct {
	ID         string `json:"id"`
	URL        string `json:"url"`
	Enabled    bool   `json:"enabled"`
	HealthPath string `json:"health_path"`
}

type parityCertificateProfile struct {
	ID              string `json:"id"`
	CertificateFile string `json:"certificate_file"`
	PrivateKeyFile  string `json:"private_key_file"`
	Enabled         bool   `json:"enabled"`
}

type parityConfig struct {
	Schema              string                     `json:"schema"`
	Services            []parityService            `json:"services"`
	Routes              []parityRoute              `json:"routes"`
	Backends            []parityBackend            `json:"backends"`
	CertificateProfiles []parityCertificateProfile `json:"certificate_profiles"`
}

// ConfigParityFingerprint returns a deterministic SHA-256 identity for the
// complete validated Gateway configuration. Collection ordering is normalized
// so the fingerprint represents configuration semantics rather than JSON array
// order.
func ConfigParityFingerprint(cfg *Config) (string, error) {
	if cfg == nil {
		return "", errors.New("gateway parity: configuration is required")
	}
	if err := cfg.Validate(); err != nil {
		return "", err
	}

	canonical := parityConfig{Schema: cfg.Schema}
	for _, service := range cfg.Services {
		backendIDs := append([]string(nil), service.BackendIDs...)
		sort.Strings(backendIDs)
		canonical.Services = append(canonical.Services, parityService{
			ID:         strings.TrimSpace(service.ID),
			Name:       strings.TrimSpace(service.Name),
			BackendIDs: backendIDs,
		})
	}
	sort.Slice(canonical.Services, func(i, j int) bool { return canonical.Services[i].ID < canonical.Services[j].ID })

	for _, route := range cfg.Routes {
		methods := make([]string, 0, len(route.Methods))
		for _, method := range route.Methods {
			methods = append(methods, strings.ToUpper(strings.TrimSpace(method)))
		}
		sort.Strings(methods)
		allowedClientCIDRs := make([]string, 0, len(route.AllowedClientCIDRs))
		for _, raw := range route.AllowedClientCIDRs {
			prefix, _ := netip.ParsePrefix(raw) // Already checked by cfg.Validate.
			allowedClientCIDRs = append(allowedClientCIDRs, prefix.Masked().String())
		}
		sort.Strings(allowedClientCIDRs)
		pathPrefix := strings.TrimSpace(route.PathPrefix)
		if pathPrefix == "" {
			pathPrefix = "/"
		}
		canonical.Routes = append(canonical.Routes, parityRoute{
			ID:         strings.TrimSpace(route.ID),
			ServiceID:  strings.TrimSpace(route.ServiceID),
			Hostname:   strings.ToLower(strings.TrimSuffix(strings.TrimSpace(route.Hostname), ".")),
			PathPrefix: pathPrefix,
			Methods:    methods,
			Exposure:   strings.TrimSpace(route.Exposure),
			AllowedClientCIDRs: allowedClientCIDRs,
			Enabled:    route.Enabled,
			TLSMode:    strings.TrimSpace(route.TLS.Mode),
			TLSProfile: strings.TrimSpace(route.TLS.CertificateProfile),
		})
	}
	sort.Slice(canonical.Routes, func(i, j int) bool { return canonical.Routes[i].ID < canonical.Routes[j].ID })

	for _, backend := range cfg.Backends {
		canonical.Backends = append(canonical.Backends, parityBackend{
			ID:         strings.TrimSpace(backend.ID),
			URL:        strings.TrimSpace(backend.URL),
			Enabled:    backend.Enabled,
			HealthPath: strings.TrimSpace(backend.HealthPath),
		})
	}
	sort.Slice(canonical.Backends, func(i, j int) bool { return canonical.Backends[i].ID < canonical.Backends[j].ID })

	for _, profile := range cfg.CertificateProfiles {
		canonical.CertificateProfiles = append(canonical.CertificateProfiles, parityCertificateProfile{
			ID:              strings.TrimSpace(profile.ID),
			CertificateFile: strings.TrimSpace(profile.CertificateFile),
			PrivateKeyFile:  strings.TrimSpace(profile.PrivateKeyFile),
			Enabled:         profile.Enabled,
		})
	}
	sort.Slice(canonical.CertificateProfiles, func(i, j int) bool {
		return canonical.CertificateProfiles[i].ID < canonical.CertificateProfiles[j].ID
	})

	encoded, err := json.Marshal(canonical)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

// BuildConfigParityEvidence compares a validated Gateway configuration against
// an independently reviewed expected configuration fingerprint. A mismatch is
// represented in the evidence and must fail validation; this function never
// authorizes production cutover.
func BuildConfigParityEvidence(cfg *Config, sourceRevision, expectedConfigSHA256 string, now time.Time) (ConfigParityEvidence, error) {
	var evidence ConfigParityEvidence
	sourceRevision = strings.ToLower(strings.TrimSpace(sourceRevision))
	if !recoverySourceRevision.MatchString(sourceRevision) {
		return evidence, errors.New("gateway parity: exact 40-character lowercase source revision is required")
	}
	if now.IsZero() {
		return evidence, errors.New("gateway parity: evidence time is required")
	}
	expectedConfigSHA256 = strings.ToLower(strings.TrimSpace(expectedConfigSHA256))
	if !validSHA256(expectedConfigSHA256) {
		return evidence, errors.New("gateway parity: expected configuration SHA-256 is invalid")
	}
	actual, err := ConfigParityFingerprint(cfg)
	if err != nil {
		return evidence, err
	}
	evidence = ConfigParityEvidence{
		Schema:                      ConfigParityEvidenceSchemaV1,
		RecordedAt:                  now.UTC().Format(time.RFC3339Nano),
		SourceRevision:              sourceRevision,
		ExpectedConfigSHA256:        expectedConfigSHA256,
		ActualConfigSHA256:          actual,
		ServiceCount:                len(cfg.Services),
		RouteCount:                  len(cfg.Routes),
		BackendCount:                len(cfg.Backends),
		CertificateProfileCount:     len(cfg.CertificateProfiles),
		ConfigurationParityMatched:  actual == expectedConfigSHA256,
		ProductionCutoverAuthorized: false,
	}
	return evidence, nil
}

// ValidateConfigParityEvidence fails closed unless the evidence is internally
// valid and the reviewed and actual configuration fingerprints match exactly.
func ValidateConfigParityEvidence(evidence ConfigParityEvidence) error {
	if evidence.Schema != ConfigParityEvidenceSchemaV1 {
		return errors.New("gateway parity: unsupported evidence schema")
	}
	if _, err := time.Parse(time.RFC3339Nano, evidence.RecordedAt); err != nil {
		return errors.New("gateway parity: recorded_at is invalid")
	}
	if !recoverySourceRevision.MatchString(evidence.SourceRevision) {
		return errors.New("gateway parity: source revision is invalid")
	}
	if !validSHA256(evidence.ExpectedConfigSHA256) || !validSHA256(evidence.ActualConfigSHA256) {
		return errors.New("gateway parity: configuration fingerprint is invalid")
	}
	if evidence.ServiceCount < 0 || evidence.RouteCount < 0 || evidence.BackendCount < 0 || evidence.CertificateProfileCount < 0 {
		return errors.New("gateway parity: aggregate counts are invalid")
	}
	if evidence.ProductionCutoverAuthorized {
		return errors.New("gateway parity: evidence cannot authorize production cutover")
	}
	if !evidence.ConfigurationParityMatched || evidence.ExpectedConfigSHA256 != evidence.ActualConfigSHA256 {
		return errors.New("gateway parity: reviewed configuration does not match the Gateway candidate")
	}
	return nil
}

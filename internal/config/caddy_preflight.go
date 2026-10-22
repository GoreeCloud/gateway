package config

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// ValidateCaddyPreflightSourceIdentity checks that an explicitly supplied
// read-only collector report identifies the same Caddy configuration as the
// independently reviewed v2 manifest. It does not attest to the report's
// provenance, independent review, or production readiness.
func ValidateCaddyPreflightSourceIdentity(report io.Reader, manifest MigrationSourceManifest) error {
	if err := ValidateMigrationSourceManifest(manifest); err != nil {
		return err
	}
	if report == nil {
		return errors.New("gateway preflight: report is required")
	}

	const maxReportBytes = 4 << 20
	limited := &io.LimitedReader{R: report, N: maxReportBytes + 1}
	scanner := bufio.NewScanner(limited)
	scanner.Buffer(make([]byte, 64<<10), 128<<10)

	required := map[string]struct {
		value string
		count int
	}{
		"mode":                                   {value: "read-only", count: 2},
		"mutation_permitted":                     {value: "no", count: 2},
		"secret_values_printed":                  {value: "no", count: 2},
		"live_provider_query_performed":         {value: "no", count: 2},
		"config_sha256":                          {count: 1},
		"adapted_config_status":                  {value: "parsed", count: 1},
		"caddy_production_authority_unchanged": {value: "yes", count: 1},
	}
	counts := make(map[string]int, len(required))
	var sourceSHA string
	const maxRouteRecords = 8192
	var declaredRouteCount int
	var routeCountSeen bool
	routeRecords := make(map[int]map[string]bool)
	requiredRouteFields := []string{"server", "hosts", "paths", "methods", "protocols", "handler", "upstreams"}
	for scanner.Scan() {
		line := strings.TrimSuffix(scanner.Text(), "\r")
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		if key == "fatal" {
			return errors.New("gateway preflight: collector reported a fatal error")
		}
		if key == "sanitized_route_record_count" {
			if routeCountSeen {
				return errors.New("gateway preflight: duplicate sanitized route count")
			}
			routeCountSeen = true
			parsed, err := strconv.Atoi(value)
			if err != nil || parsed < 1 || parsed > maxRouteRecords {
				return errors.New("gateway preflight: sanitized route count is invalid or empty")
			}
			declaredRouteCount = parsed
			continue
		}
		if strings.HasPrefix(key, "route_record[") {
			indexText, field, found := strings.Cut(strings.TrimPrefix(key, "route_record["), "].")
			index, err := strconv.Atoi(indexText)
			if !found || err != nil || index < 1 || index > maxRouteRecords || fmt.Sprintf("%03d", index) != indexText || !routeCountSeen || index > declaredRouteCount {
				return errors.New("gateway preflight: invalid or out-of-range sanitized route record")
			}
			switch field {
			case "server", "hosts", "paths", "methods", "protocols", "handler", "upstreams":
			default:
				return errors.New("gateway preflight: unsupported sanitized route field")
			}
			if (field == "server" || field == "handler") && value == "" {
				return errors.New("gateway preflight: sanitized route has no server or handler")
			}
			if routeRecords[index] == nil {
				routeRecords[index] = make(map[string]bool, len(requiredRouteFields))
			}
			if routeRecords[index][field] {
				return errors.New("gateway preflight: duplicate sanitized route field")
			}
			routeRecords[index][field] = true
			continue
		}
		spec, tracked := required[key]
		if !tracked {
			continue
		}
		counts[key]++
		if counts[key] > spec.count {
			return errors.New("gateway preflight: duplicate or ambiguous report field")
		}
		if key == "config_sha256" {
			sourceSHA = value
		} else if value != spec.value {
			return errors.New("gateway preflight: unsafe or incomplete collector status")
		}
	}
	if err := scanner.Err(); err != nil {
		return errors.New("gateway preflight: report could not be scanned safely")
	}
	if limited.N == 0 {
		return errors.New("gateway preflight: report exceeds the size limit")
	}
	for key, spec := range required {
		if counts[key] != spec.count {
			return errors.New("gateway preflight: required collector fields are missing or incomplete")
		}
	}
	if !routeCountSeen || len(routeRecords) != declaredRouteCount {
		return errors.New("gateway preflight: sanitized route inventory is incomplete")
	}
	for index := 1; index <= declaredRouteCount; index++ {
		fields := routeRecords[index]
		for _, field := range requiredRouteFields {
			if !fields[field] {
				return errors.New("gateway preflight: sanitized route record is incomplete")
			}
		}
	}
	if !validSHA256(strings.ToLower(sourceSHA)) {
		return errors.New("gateway preflight: source configuration SHA-256 is invalid")
	}
	if !strings.EqualFold(sourceSHA, strings.TrimSpace(manifest.SourceConfigSHA256)) {
		return errors.New("gateway preflight: Caddy source configuration differs from reviewed manifest")
	}
	return nil
}

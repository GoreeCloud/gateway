package responsecoding

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"github.com/GoreeCloud/goreecloud-gateway/internal/config"
	"github.com/klauspost/compress/zstd"
)

const DefaultMinimumLength = 512

// EncodeStatic conditionally encodes a validated literal response. It never
// handles upstream streams, protocol upgrades, or unbounded bodies. The
// response headers are mutated only when a policy is explicitly configured.
func EncodeStatic(req *http.Request, headers http.Header, status int, body []byte, policy *config.ResponseEncoding) ([]byte, error) {
	if policy == nil {
		return body, nil
	}
	addVary(headers, "Accept-Encoding")
	if req.Method != http.MethodGet && req.Method != http.MethodHead ||
		status == http.StatusNoContent || status == http.StatusResetContent || status == http.StatusNotModified ||
		status == http.StatusPartialContent || len(body) == 0 || req.Header.Get("Range") != "" ||
		headers.Get("Content-Encoding") != "" || headers.Get("Content-Range") != "" ||
		hasDirective(headers.Get("Cache-Control"), "no-transform") || !compressibleType(headers.Get("Content-Type")) {
		return body, nil
	}
	minimumLength := DefaultMinimumLength
	if policy.MinimumLength != nil {
		minimumLength = *policy.MinimumLength
	}
	if len(body) < minimumLength {
		return body, nil
	}
	format := negotiate(req.Header.Values("Accept-Encoding"), policy.Formats)
	if format == "" {
		return body, nil
	}

	var encoded []byte
	switch format {
	case "gzip":
		var out bytes.Buffer
		writer := gzip.NewWriter(&out)
		if _, err := writer.Write(body); err != nil {
			return nil, fmt.Errorf("gateway gzip response encoding: %w", err)
		}
		if err := writer.Close(); err != nil {
			return nil, fmt.Errorf("gateway gzip finalize: %w", err)
		}
		encoded = out.Bytes()
	case "zstd":
		writer, err := zstd.NewWriter(nil)
		if err != nil {
			return nil, fmt.Errorf("gateway zstd response encoding: %w", err)
		}
		encoded = writer.EncodeAll(body, nil)
		if err := writer.Close(); err != nil {
			return nil, fmt.Errorf("gateway zstd finalize: %w", err)
		}
	default:
		return nil, fmt.Errorf("gateway unsupported response encoding %q", format)
	}
	headers.Set("Content-Encoding", format)
	headers.Set("Content-Length", strconv.Itoa(len(encoded)))
	// Validators/digests of the identity representation must not be presented
	// as byte-for-byte validators of the newly encoded representation.
	headers.Del("ETag")
	headers.Del("Content-Md5")
	headers.Del("Digest")
	return encoded, nil
}

func compressibleType(value string) bool {
	mediaType, _, err := mime.ParseMediaType(value)
	if err != nil {
		return false
	}
	mediaType = strings.ToLower(mediaType)
	if strings.HasPrefix(mediaType, "text/") || strings.HasPrefix(mediaType, "font/") {
		return true
	}
	// The supported media prefixes are an explicit subset of Caddy's documented
	// default response matcher. Unknown and already-compressed media stay raw.
	for _, prefix := range []string{
		"application/atom+xml", "application/eot", "application/font", "application/geo+json",
		"application/graphql+json", "application/javascript", "application/json", "application/ld+json",
		"application/manifest+json", "application/opentype", "application/otf", "application/rss+xml",
		"application/truetype", "application/ttf", "application/vnd.api+json", "application/vnd.ms-fontobject",
		"application/wasm", "application/x-httpd-cgi", "application/x-javascript", "application/x-opentype",
		"application/x-otf", "application/x-perl", "application/x-protobuf", "application/x-ttf",
		"application/xhtml+xml", "application/xml", "image/svg+xml", "image/vnd.microsoft.icon",
		"image/x-icon", "multipart/bag", "multipart/mixed",
	} {
		if strings.HasPrefix(mediaType, prefix) {
			return true
		}
	}
	return false
}

func hasDirective(value, directive string) bool {
	for _, part := range strings.Split(value, ",") {
		if strings.EqualFold(strings.TrimSpace(part), directive) {
			return true
		}
	}
	return false
}

func addVary(headers http.Header, field string) {
	for _, value := range headers.Values("Vary") {
		for _, part := range strings.Split(value, ",") {
			if strings.EqualFold(strings.TrimSpace(part), field) || strings.TrimSpace(part) == "*" {
				return
			}
		}
	}
	headers.Add("Vary", field)
}

// negotiate honors explicit q-values, explicit exclusions over wildcards,
// and the configured preference order for ties. Invalid/ambiguous tokens are
// treated as unacceptable rather than silently enabling an encoding.
func negotiate(values, formats []string) string {
	if len(values) == 0 {
		return ""
	}
	quality := make(map[string]float64)
	seen := make(map[string]bool)
	for _, raw := range strings.Split(strings.Join(values, ","), ",") {
		parts := strings.Split(raw, ";")
		name := strings.ToLower(strings.TrimSpace(parts[0]))
		if name == "" {
			continue
		}
		q := 1.0
		if len(parts) > 2 {
			q = 0
		} else if len(parts) == 2 {
			parameter := strings.TrimSpace(parts[1])
			key, value, ok := strings.Cut(parameter, "=")
			parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
			if !ok || !strings.EqualFold(strings.TrimSpace(key), "q") || err != nil || parsed < 0 || parsed > 1 {
				q = 0
			} else {
				q = parsed
			}
		}
		if seen[name] && q > quality[name] {
			q = quality[name]
		}
		quality[name] = q
		seen[name] = true
	}
	best, bestQ := "", 0.0
	for _, format := range formats {
		q, ok := quality[format]
		if !ok {
			q = quality["*"]
		}
		if q > bestQ {
			best, bestQ = format, q
		}
	}
	return best
}

package responsecoding

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/GoreeCloud/goreecloud-gateway/internal/config"
	"github.com/klauspost/compress/zstd"
)

func TestEncodeStaticRoundTripAndHeaders(t *testing.T) {
	body := bytes.Repeat([]byte("GoreeCloud content with compressible whitespace. "), 35)
	policy := &config.ResponseEncoding{Formats: []string{"zstd", "gzip"}}
	for _, tc := range []struct {
		name   string
		accept string
		want   string
	}{
		{"preferred zstd", "gzip, zstd", "zstd"},
		{"higher gzip quality", "zstd;q=0.2, gzip;q=0.8", "gzip"},
		{"explicit exclusion", "zstd;q=0, gzip;q=1", "gzip"},
		{"wildcard", "*;q=0.8", "zstd"},
		{"explicit overrides wildcard", "zstd;q=0, *;q=0.8", "gzip"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest(http.MethodGet, "https://gateway.invalid/test", nil)
			req.Header.Set("Accept-Encoding", tc.accept)
			headers := make(http.Header)
			headers.Set("Content-Type", "text/plain; charset=utf-8")
			headers.Set("ETag", "\"identity-etag\"")
			headers.Set("Vary", "Origin")
			encoded, err := EncodeStatic(req, headers, http.StatusOK, body, policy)
			if err != nil {
				t.Fatal(err)
			}
			if headers.Get("Content-Encoding") != tc.want || headers.Get("Content-Length") != strconv.Itoa(len(encoded)) || headers.Get("ETag") != "" {
				t.Fatalf("incorrect encoded headers: %v", headers)
			}
			if !strings.Contains(strings.Join(headers.Values("Vary"), ","), "Accept-Encoding") || !strings.Contains(strings.Join(headers.Values("Vary"), ","), "Origin") {
				t.Fatalf("cache variance was not preserved: %v", headers)
			}
			var decoded []byte
			switch tc.want {
			case "gzip":
				reader, err := gzip.NewReader(bytes.NewReader(encoded))
				if err != nil {
					t.Fatal(err)
				}
				decoded, err = io.ReadAll(reader)
				_ = reader.Close()
				if err != nil {
					t.Fatal(err)
				}
			case "zstd":
				decoder, err := zstd.NewReader(nil)
				if err != nil {
					t.Fatal(err)
				}
				decoded, err = decoder.DecodeAll(encoded, nil)
				decoder.Close()
				if err != nil {
					t.Fatal(err)
				}
			}
			if !bytes.Equal(decoded, body) {
				t.Fatal("encoded response did not round-trip")
			}
		})
	}
}

func TestEncodeStaticFailClosedAndIdentityCases(t *testing.T) {
	body := bytes.Repeat([]byte("static response"), 80)
	policy := &config.ResponseEncoding{Formats: []string{"zstd", "gzip"}}
	for _, tc := range []struct {
		name    string
		method  string
		accept  string
		status  int
		typeVal string
		setup   func(*http.Request, http.Header)
	}{
		{"no accept header", http.MethodGet, "", http.StatusOK, "text/plain", nil},
		{"q zero", http.MethodGet, "zstd;q=0,gzip;q=0", http.StatusOK, "text/plain", nil},
		{"malformed q", http.MethodGet, "gzip;q=1.5", http.StatusOK, "text/plain", nil},
		{"duplicate exclusion", http.MethodGet, "gzip;q=1,gzip;q=0", http.StatusOK, "text/plain", nil},
		{"wrong content type", http.MethodGet, "zstd", http.StatusOK, "image/png", nil},
		{"status 206", http.MethodGet, "zstd", http.StatusPartialContent, "text/plain", nil},
		{"post response", http.MethodPost, "zstd", http.StatusOK, "text/plain", nil},
		{"range request", http.MethodGet, "zstd", http.StatusOK, "text/plain", func(r *http.Request, _ http.Header) { r.Header.Set("Range", "bytes=0-10") }},
		{"already encoded", http.MethodGet, "zstd", http.StatusOK, "text/plain", func(_ *http.Request, h http.Header) { h.Set("Content-Encoding", "br") }},
		{"no transform", http.MethodGet, "zstd", http.StatusOK, "text/plain", func(_ *http.Request, h http.Header) { h.Set("Cache-Control", "public, no-transform") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest(tc.method, "https://gateway.invalid/", nil)
			if tc.accept != "" {
				req.Header.Set("Accept-Encoding", tc.accept)
			}
			headers := make(http.Header)
			headers.Set("Content-Type", tc.typeVal)
			if tc.setup != nil {
				tc.setup(req, headers)
			}
			result, err := EncodeStatic(req, headers, tc.status, body, policy)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(result, body) || (headers.Get("Content-Encoding") != "" && headers.Get("Content-Encoding") != "br") || headers.Get("Vary") != "Accept-Encoding" {
				t.Fatalf("identity response changed unexpectedly: %v", headers)
			}
		})
	}
	minimum := 2048
	req, _ := http.NewRequest(http.MethodGet, "https://gateway.invalid/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	headers := http.Header{"Content-Type": {"text/plain"}}
	short, err := EncodeStatic(req, headers, http.StatusOK, body, &config.ResponseEncoding{Formats: []string{"gzip"}, MinimumLength: &minimum})
	if err != nil || !bytes.Equal(short, body) || headers.Get("Content-Encoding") != "" {
		t.Fatalf("minimum length bypassed: err=%v headers=%v", err, headers)
	}
}

func TestEncodeStaticNoPolicyLeavesHeadersUntouched(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "https://gateway.invalid/", nil)
	headers := http.Header{"Content-Type": {"text/plain"}}
	body := []byte("identity")
	actual, err := EncodeStatic(req, headers, http.StatusOK, body, nil)
	if err != nil || !bytes.Equal(actual, body) || headers.Get("Vary") != "" {
		t.Fatalf("absent policy changed response: %v %v", err, headers)
	}
}

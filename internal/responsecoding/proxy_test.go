package responsecoding

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/GoreeCloud/goreecloud-gateway/internal/config"
	"github.com/klauspost/compress/zstd"
)

func proxyFixture(body []byte, contentType string) *http.Response {
	return &http.Response{
		StatusCode:    http.StatusOK,
		ContentLength: int64(len(body)),
		Header:        http.Header{"Content-Type": {contentType}, "Content-Length": {strconv.Itoa(len(body))}},
		Body:          io.NopCloser(bytes.NewReader(body)),
	}
}

func TestEncodeKnownLengthProxyRoundTrip(t *testing.T) {
	body := bytes.Repeat([]byte("GoreeCloud native streaming proxy payload. "), 120)
	policy := &config.ResponseEncoding{Formats: []string{"zstd", "gzip"}}
	for _, tc := range []struct{ name, accept, want string }{
		{"preferred zstd", "gzip, zstd", "zstd"},
		{"quality gzip", "gzip;q=1, zstd;q=0.3", "gzip"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest(http.MethodGet, "https://gateway.invalid/", nil)
			req.Header.Set("Accept-Encoding", tc.accept)
			resp := proxyFixture(body, "application/json")
			resp.Header.Set("ETag", "identity")
			resp.Header.Set("Vary", "Origin")
			if err := EncodeProxy(req, resp, policy); err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.Header.Get("Content-Encoding") != tc.want || resp.ContentLength != -1 || resp.Header.Get("Content-Length") != "" || resp.Header.Get("ETag") != "" {
				t.Fatalf("encoded proxy framing/validator mismatch: %+v", resp)
			}
			if !strings.Contains(strings.Join(resp.Header.Values("Vary"), ","), "Accept-Encoding") || !strings.Contains(strings.Join(resp.Header.Values("Vary"), ","), "Origin") {
				t.Fatal("Vary not preserved")
			}
			compressed, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatal(err)
			}
			var decoded []byte
			switch tc.want {
			case "gzip":
				reader, err := gzip.NewReader(bytes.NewReader(compressed))
				if err != nil {
					t.Fatal(err)
				}
				decoded, err = io.ReadAll(reader)
				_ = reader.Close()
				if err != nil {
					t.Fatal(err)
				}
			case "zstd":
				reader, err := zstd.NewReader(nil)
				if err != nil {
					t.Fatal(err)
				}
				decoded, err = reader.DecodeAll(compressed, nil)
				reader.Close()
				if err != nil {
					t.Fatal(err)
				}
			}
			if !bytes.Equal(decoded, body) {
				t.Fatal("compressed proxy body did not round-trip")
			}
		})
	}
}

func TestEncodeKnownLengthProxyFailClosed(t *testing.T) {
	body := bytes.Repeat([]byte("GoreeCloud proxy payload. "), 64)
	policy := &config.ResponseEncoding{Formats: []string{"zstd", "gzip"}}
	for _, tc := range []struct {
		name      string
		configure func(*http.Request, *http.Response)
	}{
		{"unknown length", func(_ *http.Request, r *http.Response) { r.ContentLength = -1 }},
		{"sse with charset", func(_ *http.Request, r *http.Response) {
			r.Header.Set("Content-Type", "text/event-stream; charset=utf-8")
		}},
		{"incremental", func(_ *http.Request, r *http.Response) { r.Header.Set("Incremental", "?1") }},
		{"upgrade", func(q *http.Request, _ *http.Response) { q.Header.Set("Upgrade", "websocket") }},
		{"partial", func(_ *http.Request, r *http.Response) { r.StatusCode = http.StatusPartialContent }},
		{"head", func(q *http.Request, _ *http.Response) { q.Method = http.MethodHead }},
		{"range", func(q *http.Request, _ *http.Response) { q.Header.Set("Range", "bytes=0-20") }},
		{"already coded", func(_ *http.Request, r *http.Response) { r.Header.Set("Content-Encoding", "br") }},
		{"no transform", func(_ *http.Request, r *http.Response) {
			r.Header.Add("Cache-Control", "public")
			r.Header.Add("Cache-Control", "no-transform")
		}},
		{"request no transform", func(q *http.Request, _ *http.Response) { q.Header.Set("Cache-Control", "no-transform") }},
		{"transport decompressed", func(_ *http.Request, r *http.Response) { r.Uncompressed = true }},
		{"status no content", func(_ *http.Request, r *http.Response) { r.StatusCode = http.StatusNoContent }},
		{"upstream content range", func(_ *http.Request, r *http.Response) { r.Header.Set("Content-Range", "bytes 0-100/1000") }},
		{"upstream trailer declaration", func(_ *http.Request, r *http.Response) { r.Header.Set("Trailer", "Digest") }},
		{"binary", func(_ *http.Request, r *http.Response) { r.Header.Set("Content-Type", "image/png") }},
		{"response trailer", func(_ *http.Request, r *http.Response) { r.Trailer = http.Header{"Digest": {"pending"}} }},
		{"client q zero", func(q *http.Request, _ *http.Response) { q.Header.Set("Accept-Encoding", "gzip;q=0, zstd;q=0") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest(http.MethodGet, "https://gateway.invalid/", nil)
			req.Header.Set("Accept-Encoding", "gzip, zstd")
			resp := proxyFixture(body, "text/plain; charset=utf-8")
			tc.configure(req, resp)
			original := resp.Body
			if err := EncodeProxy(req, resp, policy); err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.Body != original || (resp.Header.Get("Content-Encoding") != "" && resp.Header.Get("Content-Encoding") != "br") {
				t.Fatal("unsupported proxy response was unexpectedly encoded")
			}
			if resp.Header.Get("Vary") != "Accept-Encoding" {
				t.Fatal("missing variation header")
			}
		})
	}
}

func TestEncodeKnownLengthProxyThreshold(t *testing.T) {
	minimum := 2048
	policy := &config.ResponseEncoding{Formats: []string{"gzip"}, MinimumLength: &minimum}
	req, _ := http.NewRequest(http.MethodGet, "https://gateway.invalid/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	resp := proxyFixture(bytes.Repeat([]byte("x"), 1024), "text/plain")
	before := resp.Body
	if err := EncodeProxy(req, resp, policy); err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.Body != before || resp.Header.Get("Content-Encoding") != "" {
		t.Fatal("below-threshold response compressed")
	}
}

type blockingProxySource struct {
	done chan struct{}
	once sync.Once
}

func (s *blockingProxySource) Read(_ []byte) (int, error) {
	<-s.done
	return 0, io.EOF
}

func (s *blockingProxySource) Close() error {
	s.once.Do(func() { close(s.done) })
	return nil
}

func TestEncodeKnownLengthProxyClientAbortClosesUpstream(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "https://gateway.invalid/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	source := &blockingProxySource{done: make(chan struct{})}
	resp := &http.Response{
		StatusCode: http.StatusOK, ContentLength: 1024,
		Header: http.Header{"Content-Type": {"text/plain"}}, Body: source,
	}
	if err := EncodeProxy(req, resp, &config.ResponseEncoding{Formats: []string{"gzip"}}); err != nil {
		t.Fatal(err)
	}
	if err := resp.Body.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-source.done:
	case <-time.After(2 * time.Second):
		t.Fatal("abandoned downstream kept upstream body open")
	}
}

func TestEncodeKnownLengthProxyPolicyAbsent(t *testing.T) {
	body := bytes.Repeat([]byte("x"), 1024)
	req, _ := http.NewRequest(http.MethodGet, "https://gateway.invalid/", nil)
	resp := proxyFixture(body, "text/plain")
	if err := EncodeProxy(req, resp, nil); err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.Header.Get("Vary") != "" || resp.Header.Get("Content-Encoding") != "" {
		t.Fatal("legacy response mutated")
	}
}

func TestEncodeUnknownLengthProxyExplicitOptIn(t *testing.T) {
	zero := 0
	for _, tc := range []struct{ format string }{{"gzip"}, {"zstd"}} {
		t.Run(tc.format, func(t *testing.T) {
			first := bytes.Repeat([]byte("first segment. "), 80)
			second := bytes.Repeat([]byte("second segment. "), 80)
			upstreamReader, upstreamWriter := io.Pipe()
			req, _ := http.NewRequest(http.MethodGet, "https://gateway.invalid/stream", nil)
			req.Header.Set("Accept-Encoding", tc.format)
			resp := &http.Response{
				StatusCode: http.StatusOK, ContentLength: -1,
				Header: http.Header{"Content-Type": {"text/plain"}}, Body: upstreamReader,
			}
			policy := &config.ResponseEncoding{Formats: []string{tc.format}, MinimumLength: &zero, AllowUnknownLength: true}
			if err := EncodeProxy(req, resp, policy); err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.Header.Get("Content-Encoding") != tc.format || resp.ContentLength != -1 {
				t.Fatalf("unknown-length opt-in missing stream headers: %v", resp.Header)
			}
			firstDecoded := make(chan struct{})
			producerDone := make(chan error, 1)
			go func() {
				if _, err := upstreamWriter.Write(first); err != nil {
					producerDone <- err
					return
				}
				<-firstDecoded
				_, err := upstreamWriter.Write(second)
				_ = upstreamWriter.Close()
				producerDone <- err
			}()
			decodedFirst := make(chan []byte, 1)
			readErr := make(chan error, 1)
			go func() {
				var decoder io.ReadCloser
				if tc.format == "gzip" {
					reader, err := gzip.NewReader(resp.Body)
					if err != nil {
						readErr <- err
						return
					}
					decoder = reader
				} else {
					reader, err := zstd.NewReader(resp.Body)
					if err != nil {
						readErr <- err
						return
					}
					decoder = reader.IOReadCloser()
				}
				defer decoder.Close()
				prefix := make([]byte, len(first))
				if _, err := io.ReadFull(decoder, prefix); err != nil {
					readErr <- err
					return
				}
				decodedFirst <- prefix
				<-firstDecoded
				rest, err := io.ReadAll(decoder)
				if err != nil {
					readErr <- err
					return
				}
				if !bytes.Equal(rest, second) {
					readErr <- io.ErrUnexpectedEOF
					return
				}
				readErr <- nil
			}()
			select {
			case got := <-decodedFirst:
				if !bytes.Equal(got, first) {
					t.Fatal("streaming first chunk mismatch")
				}
				close(firstDecoded)
			case err := <-readErr:
				close(firstDecoded)
				t.Fatalf("failed before streaming first chunk: %v", err)
			case <-time.After(3 * time.Second):
				close(firstDecoded)
				t.Fatal("first compressed chunk buffered until upstream EOF")
			}
			if err := <-readErr; err != nil {
				t.Fatalf("second chunk: %v", err)
			}
			if err := <-producerDone; err != nil {
				t.Fatalf("upstream write: %v", err)
			}
		})
	}
}

func TestEncodeUnknownLengthProxyProtectsSSEAndIncremental(t *testing.T) {
	zero := 0
	policy := &config.ResponseEncoding{Formats: []string{"gzip"}, MinimumLength: &zero, AllowUnknownLength: true}
	for _, tc := range []struct {
		name    string
		headers http.Header
	}{
		{"sse", http.Header{"Content-Type": {"text/event-stream; charset=utf-8"}}},
		{"incremental", http.Header{"Content-Type": {"application/json"}, "Incremental": {"?1"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest(http.MethodGet, "https://gateway.invalid/", nil)
			req.Header.Set("Accept-Encoding", "gzip")
			original := io.NopCloser(strings.NewReader("data: ready\n\n"))
			resp := &http.Response{StatusCode: 200, ContentLength: -1, Header: tc.headers, Body: original}
			if err := EncodeProxy(req, resp, policy); err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.Body != original || resp.Header.Get("Content-Encoding") != "" {
				t.Fatal("SSE/incremental stream was encoded despite bypass")
			}
		})
	}
}

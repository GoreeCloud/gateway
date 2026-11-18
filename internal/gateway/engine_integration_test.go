package gateway

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/certmagic"
)

func TestLoopbackEngineHTTPStreamingAndWebSocket(t *testing.T) {
	// Caddy autosave defaults to the user's home. Never touch a real user's
	// Caddy state from a synthetic integration test.
	isolated := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(isolated, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(isolated, "data"))
	previousAutosave := caddy.ConfigAutosavePath
	previousStorage := caddy.DefaultStorage
	caddy.DefaultStorage = &certmagic.FileStorage{Path: filepath.Join(isolated, "data", "caddy")}
	defer func() { caddy.DefaultStorage = previousStorage }()
	caddy.ConfigAutosavePath = filepath.Join(isolated, "config", "caddy", "autosave.json")
	defer func() { caddy.ConfigAutosavePath = previousAutosave }()
	// All listeners are ephemeral, loopback-only and intentionally do not use
	// port 80/443, public DNS, the production Caddyfile or any certificate.
	streamRelease := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ping":
			w.Header().Set("X-Gateway-Test", "upstream")
			_, _ = io.WriteString(w, "caddy-upstream-ok")
		case "/stream":
			w.Header().Set("Content-Type", "text/plain")
			_, _ = io.WriteString(w, "first")
			w.(http.Flusher).Flush()
			select {
			case <-streamRelease:
			case <-time.After(3 * time.Second):
			}
			_, _ = io.WriteString(w, "-second")
		case "/ws":
			conn, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Errorf("hijack: %v", err)
				return
			}
			defer conn.Close()
			key := r.Header.Get("Sec-WebSocket-Key")
			if key == "" {
				t.Error("missing websocket key")
				return
			}
			sum := sha1.Sum([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
			_, _ = fmt.Fprintf(conn, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: %s\r\n\r\n", base64.StdEncoding.EncodeToString(sum[:]))
			header := make([]byte, 2)
			if _, err := io.ReadFull(conn, header); err != nil {
				t.Errorf("read websocket header: %v", err)
				return
			}
			if header[0] != 0x81 || header[1]&0x80 == 0 {
				t.Errorf("unexpected websocket client frame: %x", header)
				return
			}
			n := int(header[1] & 0x7f)
			if n > 125 {
				t.Error("unexpected websocket frame length")
				return
			}
			mask := make([]byte, 4)
			payload := make([]byte, n)
			if _, err := io.ReadFull(conn, mask); err != nil {
				t.Error(err)
				return
			}
			if _, err := io.ReadFull(conn, payload); err != nil {
				t.Error(err)
				return
			}
			for i := range payload {
				payload[i] ^= mask[i%4]
			}
			_, _ = conn.Write(append([]byte{0x81, byte(n)}, payload...))
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	src := fmt.Sprintf("{\n  admin off\n}\nhttp://%s {\n  reverse_proxy %s\n}\n", addr, upstream.URL)
	adapter := caddyconfig.GetAdapter("caddyfile")
	if adapter == nil {
		t.Fatal("Caddyfile adapter unavailable")
	}
	adapted, warnings, err := adapter.Adapt(caddyfile.Format([]byte(src)), nil)
	if err != nil || len(warnings) != 0 {
		t.Fatalf("adapt: %v warnings=%v", err, warnings)
	}
	if err := caddy.Load(adapted, true); err != nil {
		t.Fatalf("start isolated Caddy: %v", err)
	}
	defer caddy.Stop()
	client := &http.Client{Timeout: 4 * time.Second}
	res, err := client.Get("http://" + addr + "/ping")
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(res.Body)
	_ = res.Body.Close()
	if err != nil || res.StatusCode != http.StatusOK || string(body) != "caddy-upstream-ok" || res.Header.Get("X-Gateway-Test") != "upstream" {
		t.Fatalf("reverse proxy HTTP: status=%d body=%q error=%v", res.StatusCode, body, err)
	}

	stream, err := client.Get("http://" + addr + "/stream")
	if err != nil {
		t.Fatal(err)
	}
	first := make([]byte, 5)
	if _, err := io.ReadFull(stream.Body, first); err != nil {
		_ = stream.Body.Close()
		t.Fatal(err)
	}
	if string(first) != "first" {
		t.Fatalf("stream first chunk: %q", first)
	}
	close(streamRelease)
	remaining, err := io.ReadAll(stream.Body)
	_ = stream.Body.Close()
	if err != nil || string(remaining) != "-second" {
		t.Fatalf("stream continuation: %q error=%v", remaining, err)
	}

	conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(4 * time.Second))
	wsKey := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef"))
	_, _ = fmt.Fprintf(conn, "GET /ws HTTP/1.1\r\nHost: %s\r\nConnection: Upgrade\r\nUpgrade: websocket\r\nSec-WebSocket-Version: 13\r\nSec-WebSocket-Key: %s\r\n\r\n", addr, wsKey)
	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil || !strings.Contains(line, "101 Switching Protocols") {
		t.Fatalf("websocket handshake: %q error=%v", line, err)
	}
	for {
		h, err := reader.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if h == "\r\n" {
			break
		}
	}
	_, _ = conn.Write([]byte{0x81, 0x84, 1, 2, 3, 4, 'p' ^ 1, 'o' ^ 2, 'n' ^ 3, 'g' ^ 4})
	echo := make([]byte, 6)
	if _, err := io.ReadFull(reader, echo); err != nil {
		t.Fatal(err)
	}
	if string(echo) != string([]byte{0x81, 0x04, 'p', 'o', 'n', 'g'}) {
		t.Fatalf("websocket echoed frame mismatch: %x", echo)
	}
}

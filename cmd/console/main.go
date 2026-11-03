package main

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/netip"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"
)

//go:embed web/*
var assets embed.FS

const maxConfig = 2 << 20

type overview struct {
	APIVersion string   `json:"api_version"`
	Engine     string   `json:"engine"`
	State      string   `json:"state"`
	Servers    *int     `json:"servers,omitempty"`
	Routes     *int     `json:"routes,omitempty"`
	Hosts      []string `json:"hosts"`
	CheckedAt  string   `json:"checked_at"`
}

type adminConfig struct {
	Apps struct {
		HTTP struct {
			Servers map[string]struct {
				Routes []struct {
					Match []struct {
						Host []string
					}
				}
			}
		}
	}
}

func summarize(r io.Reader) (overview, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxConfig+1))
	if err != nil || len(data) > maxConfig {
		return overview{}, errors.New("oversized response")
	}
	var cfg adminConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return overview{}, errors.New("invalid response")
	}
	routes := 0
	hosts := map[string]struct{}{}
	for _, server := range cfg.Apps.HTTP.Servers {
		routes += len(server.Routes)
		for _, route := range server.Routes {
			for _, match := range route.Match {
				for _, host := range match.Host {
					if len(host) > 0 && len(host) <= 253 {
						hosts[host] = struct{}{}
					}
				}
			}
		}
	}
	names := make([]string, 0, len(hosts))
	for name := range hosts {
		names = append(names, name)
	}
	sort.Strings(names)
	if len(names) > 100 {
		names = names[:100]
	}
	servers := len(cfg.Apps.HTTP.Servers)
	return overview{
		APIVersion: "v1", Engine: "Caddy-compatible", State: "connected",
		Servers: &servers, Routes: &routes, Hosts: names,
	}, nil
}

func newHandler(token, adminURL string) (http.Handler, error) {
	if len(token) < 32 || strings.TrimSpace(token) != token {
		return nil, errors.New("console token must have 32 or more characters without surrounding whitespace")
	}
	approved := sha256.Sum256([]byte(token))
	static, err := fs.Sub(assets, "web")
	if err != nil {
		return nil, err
	}
	client := &http.Client{
		Timeout: 2500 * time.Millisecond,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, map[string]bool{"ok": true})
	})
	mux.HandleFunc("/api/v1/overview", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		attempt := sha256.Sum256([]byte(strings.TrimPrefix(auth, "Bearer ")))
		if subtle.ConstantTimeCompare(attempt[:], approved[:]) != 1 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		state := overview{APIVersion: "v1", Engine: "Caddy-compatible", State: "unreachable", Hosts: []string{}}
		req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, adminURL+"/config/", nil)
		if err == nil {
			resp, err := client.Do(req)
			if err == nil {
				defer resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					if parsed, err := summarize(resp.Body); err == nil {
						state = parsed
					}
				}
			}
		}
		state.CheckedAt = time.Now().UTC().Format(time.RFC3339)
		writeJSON(w, state)
	})
	mux.Handle("/", http.FileServer(http.FS(static)))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headers := w.Header()
		headers.Set("Cache-Control", "no-store")
		headers.Set("X-Content-Type-Options", "nosniff")
		headers.Set("X-Frame-Options", "DENY")
		headers.Set("Referrer-Policy", "no-referrer")
		headers.Set("Content-Security-Policy", "default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'")
		if strings.HasPrefix(r.URL.Path, "/api/") && r.Header.Get("Sec-Fetch-Site") == "cross-site" {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		mux.ServeHTTP(w, r)
	}), nil
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(value)
}

func validateListen(value string) error {
	host, port, err := net.SplitHostPort(value)
	if err != nil || port == "" {
		return fmt.Errorf("invalid console listener")
	}
	addr, err := netip.ParseAddr(host)
	if err != nil || !addr.IsLoopback() {
		return fmt.Errorf("console must bind to numeric loopback")
	}
	return nil
}

func main() {
	listen := flag.String("listen", "127.0.0.1:7689", "numeric loopback listen address")
	flag.Parse()
	if err := validateListen(*listen); err != nil {
		log.Fatal(err)
	}
	handler, err := newHandler(os.Getenv("GOREE_GATEWAY_CONSOLE_TOKEN"), "http://127.0.0.1:2019")
	if err != nil {
		log.Fatal(err)
	}
	server := &http.Server{
		Addr: *listen, Handler: handler,
		ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 8192,
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	go func() {
		<-ctx.Done()
		_ = server.Close()
	}()
	log.Printf("GoreeCloud console available at loopback %s", *listen)
	if err = server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

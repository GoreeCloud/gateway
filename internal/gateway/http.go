package gateway

import (
	"bytes"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Version is overwritten by -ldflags for tagged release builds.
var Version = "0.1.0-dev"

type api struct {
	manager    *Manager
	token      []byte
	write      bool
	public     http.Handler
	index      []byte
	failuresMu sync.Mutex
	failures   map[string]authFailure
}

type authFailure struct {
	count int
	until time.Time
}

// The loopback-only management console still accepts connections from many
// different 127/8 source addresses. Bound per-peer failure history so a local
// untrusted process cannot grow its in-memory rate-limit table without limit.
const maxAuthFailurePeers = 256

func Handler(m *Manager, assets fs.FS, token string, allowWrite bool) http.Handler {
	index, err := fs.ReadFile(assets, "index.html")
	if err != nil {
		panic("web build missing index.html: " + err.Error())
	}
	a := &api{manager: m, token: []byte(token), write: allowWrite, public: http.FileServer(http.FS(assets)), index: index, failures: map[string]authFailure{}}
	return http.HandlerFunc(a.serve)
}

func (a *api) serve(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
	w.Header().Set("Cross-Origin-Opener-Policy", "same-origin")
	w.Header().Set("Permissions-Policy", "geolocation=(), camera=(), microphone=()")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; connect-src 'self'; img-src 'self' data:; style-src 'self'; script-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'")
	// Reject DNS-rebinding and spoofed Host headers. The listener itself is
	// also restricted to a loopback IP address by main.serve.
	hostname := r.Host
	if h, _, err := net.SplitHostPort(r.Host); err == nil {
		hostname = h
	}
	address := net.ParseIP(hostname)
	if hostname != "localhost" && (address == nil || !address.IsLoopback()) {
		http.Error(w, "Invalid management Host", http.StatusForbidden)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/") {
		a.routeAPI(w, r)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "Method not allowed", 405)
		return
	}
	if r.URL.Path == "/" || (!strings.Contains(r.URL.Path, ".") && !strings.HasPrefix(r.URL.Path, "/assets/")) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(a.index)
		return
	}
	a.public.ServeHTTP(w, r)
}

func (a *api) authorized(r *http.Request) bool {
	headers := r.Header.Values("Authorization")
	if len(headers) != 1 {
		return false
	}
	if !strings.HasPrefix(headers[0], "Bearer ") {
		return false
	}
	got := []byte(strings.TrimPrefix(headers[0], "Bearer "))
	return len(got) == len(a.token) && subtle.ConstantTimeCompare(got, a.token) == 1
}

func send(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}
func fail(w http.ResponseWriter, code int, message string) {
	send(w, code, map[string]string{"error": message})
}

func (a *api) routeAPI(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/api/health" && r.Method == http.MethodGet {
		send(w, 200, map[string]bool{"ok": true})
		return
	}
	if a.tooManyFailures(r.RemoteAddr) {
		fail(w, 429, "Too many authentication failures")
		return
	}
	if !a.authorized(r) {
		a.noteFailure(r.RemoteAddr)
		fail(w, 401, "Unauthorized")
		return
	}
	a.clearFailures(r.RemoteAddr)
	switch r.URL.Path {
	case "/api/status":
		if r.Method != http.MethodGet {
			fail(w, 405, "Method not allowed")
			return
		}
		sites, running := a.manager.Snapshot()
		send(w, 200, map[string]any{"name": "GoreeCloud Gateway", "engine": "Caddy v2", "version": Version, "running": running, "siteCount": len(sites), "writable": a.write, "management": "loopback"})
		return
	case "/api/validate-site":
		if r.Method != http.MethodPost {
			fail(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}
		// This endpoint is available even when edits are disabled. It performs
		// validation and Caddyfile adaptation without calling the live loader,
		// touching the managed site file, or contacting any upstream.
		site, err := readSite(w, r)
		if err != nil {
			failSiteInput(w, err)
			return
		}
		if _, err := Render([]Site{site}); err != nil {
			fail(w, http.StatusBadRequest, "Site cannot be adapted by Caddy")
			return
		}
		send(w, http.StatusOK, map[string]any{"valid": true, "dryRun": true, "applied": false, "site": site})
		return
	case "/api/sites":
		if r.Method == http.MethodGet {
			sites, _ := a.manager.Snapshot()
			if sites == nil {
				sites = []Site{}
			}
			send(w, 200, map[string]any{"sites": sites})
			return
		}
		if r.Method == http.MethodPost {
			if !a.write {
				fail(w, 403, "Read-only console; use --allow-write")
				return
			}
			site, err := readSite(w, r)
			if err != nil {
				failSiteInput(w, err)
				return
			}
			if err = a.manager.Add(site); err != nil {
				fail(w, 400, err.Error())
				return
			}
			send(w, 201, site)
			return
		}
		fail(w, 405, "Method not allowed")
		return
	default:
		if !strings.HasPrefix(r.URL.Path, "/api/sites/") {
			fail(w, 404, "Not found")
			return
		}
		if !a.write {
			fail(w, 403, "Read-only console; use --allow-write")
			return
		}
		host, err := url.PathUnescape(strings.TrimPrefix(r.URL.EscapedPath(), "/api/sites/"))
		if err != nil || host == "" || strings.Contains(host, "/") {
			fail(w, 400, "Invalid hostname")
			return
		}
		switch r.Method {
		case http.MethodDelete:
			err = a.manager.Delete(host)
			if errors.Is(err, ErrNotFound) {
				fail(w, 404, "Site not found")
				return
			}
			if err != nil {
				fail(w, 500, err.Error())
				return
			}
			w.WriteHeader(http.StatusNoContent)
		case http.MethodPut:
			site, e := readSite(w, r)
			if e != nil {
				failSiteInput(w, e)
				return
			}
			err = a.manager.Update(host, site)
			if errors.Is(err, ErrNotFound) {
				fail(w, 404, "Site not found")
				return
			}
			if err != nil {
				fail(w, 400, err.Error())
				return
			}
			send(w, 200, site)
		default:
			fail(w, 405, "Method not allowed")
		}
	}
}

var errSiteBodyTooLarge = errors.New("site JSON exceeds 8 KiB limit")

func failSiteInput(w http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	if errors.Is(err, errSiteBodyTooLarge) {
		status = http.StatusRequestEntityTooLarge
	}
	fail(w, status, err.Error())
}

func siteJSONError(err error) error {
	var sizeErr *http.MaxBytesError
	if errors.As(err, &sizeErr) {
		return errSiteBodyTooLarge
	}
	return errors.New("Invalid site JSON")
}

func readSite(w http.ResponseWriter, r *http.Request) (Site, error) {
	var site Site
	values := r.Header.Values("Content-Type")
	if len(values) != 1 {
		return site, errors.New("Content-Type must be application/json")
	}
	mediaType, params, err := mime.ParseMediaType(values[0])
	if err != nil || mediaType != "application/json" ||
		(len(params) > 0 && (len(params) != 1 || !strings.EqualFold(params["charset"], "utf-8"))) {
		return site, errors.New("Content-Type must be application/json")
	}
	// Unlike io.LimitReader, MaxBytesReader reports oversized input even if
	// extra bytes follow otherwise valid JSON or consist only of whitespace.
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 8<<10))
	if err != nil {
		return site, siteJSONError(err)
	}
	if err := rejectDuplicateSiteFields(raw); err != nil {
		return site, err
	}
	reader := json.NewDecoder(bytes.NewReader(raw))
	reader.DisallowUnknownFields()
	if err := reader.Decode(&site); err != nil {
		return site, siteJSONError(err)
	}
	if err := reader.Decode(&struct{}{}); err != io.EOF {
		return site, siteJSONError(err)
	}
	return ValidateSite(site)
}

// Failed authentication is counted using only the direct TCP peer, not any
// untrusted Forwarded or X-Forwarded-For headers. State is volatile and never logged.
// authPeerKey strips the ephemeral TCP source port and canonicalizes the
// literal direct-peer IP; never use X-Forwarded-For for this decision.
func authPeerKey(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.String()
	}
	return host
}

func (a *api) tooManyFailures(peer string) bool {
	peer = authPeerKey(peer)
	a.failuresMu.Lock()
	defer a.failuresMu.Unlock()
	f := a.failures[peer]
	if time.Now().After(f.until) {
		delete(a.failures, peer)
		return false
	}
	return f.count >= 10
}

func (a *api) noteFailure(peer string) {
	peer = authPeerKey(peer)
	a.failuresMu.Lock()
	defer a.failuresMu.Unlock()
	now := time.Now()
	f, known := a.failures[peer]
	if !known && len(a.failures) >= maxAuthFailurePeers {
		// Reclaim expired entries before evicting a currently tracked peer.
		for key, entry := range a.failures {
			if !now.Before(entry.until) {
				delete(a.failures, key)
			}
		}
		if len(a.failures) >= maxAuthFailurePeers {
			// If many direct loopback peers fail at once, discard the
			// oldest record rather than allocating unbounded state.
			// This is local-process abuse resistance, not a distributed
			// authentication defense.
			var oldestKey string
			var oldestDeadline time.Time
			for key, entry := range a.failures {
				if oldestKey == "" || entry.until.Before(oldestDeadline) {
					oldestKey, oldestDeadline = key, entry.until
				}
			}
			delete(a.failures, oldestKey)
		}
	}
	if !now.Before(f.until) {
		f = authFailure{until: now.Add(time.Minute)}
	}
	f.count++
	a.failures[peer] = f
}

func (a *api) clearFailures(peer string) {
	peer = authPeerKey(peer)
	a.failuresMu.Lock()
	defer a.failuresMu.Unlock()
	delete(a.failures, peer)
}

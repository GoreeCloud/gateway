package gateway

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"strings"
)

type api struct {
	manager *Manager
	token   []byte
	write   bool
	public  http.Handler
	index   []byte
}

func Handler(m *Manager, assets fs.FS, token string, allowWrite bool) http.Handler {
	index, err := fs.ReadFile(assets, "index.html")
	if err != nil {
		panic("web build missing index.html: " + err.Error())
	}
	a := &api{manager: m, token: []byte(token), write: allowWrite, public: http.FileServer(http.FS(assets)), index: index}
	return http.HandlerFunc(a.serve)
}

func (a *api) serve(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; connect-src 'self'; img-src 'self' data:; style-src 'self'; script-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'")
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
	if !a.authorized(r) {
		fail(w, 401, "Unauthorized")
		return
	}
	switch r.URL.Path {
	case "/api/status":
		if r.Method != http.MethodGet {
			fail(w, 405, "Method not allowed")
			return
		}
		sites, running := a.manager.Snapshot()
		send(w, 200, map[string]any{"name": "GoreeCloud Gateway", "engine": "Caddy v2", "version": "0.1.0-dev", "running": running, "siteCount": len(sites), "writable": a.write, "management": "loopback"})
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
			site, err := readSite(r)
			if err != nil {
				fail(w, 400, err.Error())
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
			site, e := readSite(r)
			if e != nil {
				fail(w, 400, e.Error())
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

func readSite(r *http.Request) (Site, error) {
	var s Site
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		return s, errors.New("Content-Type must be application/json")
	}
	reader := json.NewDecoder(io.LimitReader(r.Body, 8<<10))
	reader.DisallowUnknownFields()
	if err := reader.Decode(&s); err != nil {
		return s, errors.New("Invalid site JSON")
	}
	if err := reader.Decode(&struct{}{}); err != io.EOF {
		return s, errors.New("Unexpected trailing JSON")
	}
	return ValidateSite(s)
}

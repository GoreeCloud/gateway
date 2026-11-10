package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type consoleReadiness struct {
	AdminConfig           string `json:"admin_config"`
	IngressRoutesVerified bool   `json:"ingress_routes_verified"`
}

func TestConsoleAdminReadinessAuthenticationAndProjection(t *testing.T) {
	configRequests := 0
	admin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		configRequests++
		if r.URL.Path != "/config/" || r.Method != http.MethodGet {
			t.Errorf("unexpected admin request: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"apps":{"http":{"servers":{"private":{"routes":[{"match":[{"host":["private.internal.test"]}]}]}}}}}`))
	}))
	defer admin.Close()
	secret := strings.Repeat("r", 48)
	handler, err := newHandler(secret, admin.URL)
	if err != nil {
		t.Fatal(err)
	}
	send := func(method string, token string, extra bool) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, "/api/v1/readiness", nil)
		if token != "" {
			req.Header.Add("Authorization", "Bearer "+token)
		}
		if extra {
			req.Header.Add("Authorization", "Bearer "+secret)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}
	for _, tc := range []struct {
		name, method, token string
		extra               bool
		want                int
	}{
		{"no_auth", http.MethodGet, "", false, http.StatusUnauthorized},
		{"wrong_token", http.MethodGet, "incorrect", false, http.StatusUnauthorized},
		{"duplicate_bearer", http.MethodGet, secret, true, http.StatusUnauthorized},
		{"read_only", http.MethodPost, secret, false, http.StatusMethodNotAllowed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := send(tc.method, tc.token, tc.extra)
			if rec.Code != tc.want {
				t.Fatalf("got %d, wanted %d", rec.Code, tc.want)
			}
		})
	}
	if configRequests != 0 {
		t.Fatal("unauthorized or non-GET call probed admin")
	}
	rec := send(http.MethodGet, secret, false)
	if rec.Code != http.StatusOK {
		t.Fatalf("authorized GET: %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "private.internal.test") ||
		strings.Contains(rec.Body.String(), "\"routes\":") {
		t.Fatal("readiness response disclosed admin route/host metadata")
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("unexpected content type: %q", got)
	}
	var state consoleReadiness
	if err := json.Unmarshal(rec.Body.Bytes(), &state); err != nil {
		t.Fatal(err)
	}
	if state.AdminConfig != "queryable" || state.IngressRoutesVerified {
		t.Fatalf("readiness falsely claims verified ingress: %+v", state)
	}
	if configRequests != 1 {
		t.Fatalf("expected exactly one local admin config fetch, got %d", configRequests)
	}
}

func TestConsoleAdminReadinessFailsClosed(t *testing.T) {
	tests := []struct {
		name, body string
		status     int
	}{
		{"upstream_503", `{"message":"private-debug"}`, http.StatusServiceUnavailable},
		{"malformed_json", "{not-valid-json", http.StatusOK},
		{"oversized", strings.Repeat("x", maxConfig+1), http.StatusOK},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			admin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer admin.Close()
			secret := strings.Repeat("q", 48)
			handler, err := newHandler(secret, admin.URL)
			if err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequest(http.MethodGet, "/api/v1/readiness", nil)
			req.Header.Set("Authorization", "Bearer "+secret)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusServiceUnavailable {
				t.Fatalf("got %d, expected unavailable", rec.Code)
			}
			if strings.Contains(rec.Body.String(), "private-debug") {
				t.Fatal("readiness exposed upstream error message")
			}
			var state consoleReadiness
			if err := json.Unmarshal(rec.Body.Bytes(), &state); err != nil {
				t.Fatal(err)
			}
			if state.AdminConfig != "unavailable" || state.IngressRoutesVerified {
				t.Fatalf("failed readiness must deny ingress claim: %+v", state)
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(rec.Body.Bytes(), &fields); err != nil || len(fields) != 2 {
				t.Fatal("failed readiness shape is not constrained")
			}
		})
	}
}

func TestConsoleAdminReadinessOffline(t *testing.T) {
	secret := strings.Repeat("s", 48)
	handler, err := newHandler(secret, "http://127.0.0.1:1")
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/readiness", nil)
	req.Header.Set("Authorization", "Bearer "+secret)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("offline admin incorrectly ready: %d", rec.Code)
	}
}

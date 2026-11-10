package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestConsoleBindSafety(t *testing.T) {
	for _, value := range []string{
		"0.0.0.0:7689", ":7689", "localhost:7689",
		"127.0.0.1:0", "[::1]:0", "127.0.0.1:00000",
		"127.0.0.1:65536", "[::1]:99999", "127.0.0.1:9999999999",
		"127.0.0.1:-1", "127.0.0.1:admin", "127.0.0.1:00080",
		"127.0.0.1:", "127.0.0.1",
	} {
		if validateListen(value) == nil {
			t.Errorf("public, ephemeral, noncanonical or invalid listener allowed: %s", value)
		}
	}
	for _, value := range []string{
		"127.0.0.1:7689", "[::1]:7689", "127.0.0.1:1", "[::1]:65535",
	} {
		if err := validateListen(value); err != nil {
			t.Errorf("canonical loopback listener %s rejected: %v", value, err)
		}
	}
}

func TestReadOnlyOverview(t *testing.T) {
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("{\"apps\":{\"http\":{\"servers\":{\"sample\":{\"routes\":[{\"match\":[{\"host\":[\"test.local\"]}]}]}}}}}"))
	}))
	defer local.Close()
	key := strings.Repeat("k", 40)
	handler, err := newHandler(key, local.URL)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/overview", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("request without authorization returned %d", recorder.Code)
	}
	request.Header.Set("Authorization", "Bearer "+key)
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("authorized request returned %d", recorder.Code)
	}
	var result overview
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.State != "connected" || result.Servers == nil || result.Routes == nil || *result.Routes != 1 || len(result.Hosts) != 1 {
		t.Fatalf("wrong minimized response: %+v", result)
	}
	request = httptest.NewRequest(http.MethodPost, "/api/v1/overview", nil)
	request.Header.Set("Authorization", "Bearer "+key)
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("unexpected write behavior: %d", recorder.Code)
	}
}

func TestOfflineState(t *testing.T) {
	handler, err := newHandler(strings.Repeat("k", 40), "http://127.0.0.1:1")
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/overview", nil)
	req.Header.Set("Authorization", "Bearer "+strings.Repeat("k", 40))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	var result overview
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.State != "unreachable" || result.Routes != nil || result.Servers != nil {
		t.Fatalf("offline metrics should be unknown: %+v", result)
	}
}

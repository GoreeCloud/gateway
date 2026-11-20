package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func TestValidateSiteDryRunIsAuthenticatedAndNeverPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sites.json")
	loadCalls := 0
	manager, err := Open(path, func([]byte, bool) error {
		loadCalls++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Start(); err != nil {
		t.Fatal(err)
	}
	token := strings.Repeat("d", 32)
	handler := Handler(manager, fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("ok")}}, token, false)
	request := func(method, bearer, payload string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, "/api/validate-site", strings.NewReader(payload))
		req.Host = "127.0.0.1:8787"
		if bearer != "" {
			req.Header.Set("Authorization", "Bearer "+bearer)
		}
		if payload != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}
	good := `{"host":"App.Example","upstream":"http://127.0.0.1:3300"}`
	if got := request(http.MethodPost, "", good); got.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated site validation was allowed: %d", got.Code)
	}
	if got := request(http.MethodGet, token, ""); got.Code != http.StatusMethodNotAllowed {
		t.Fatalf("unexpected method accepted: %d", got.Code)
	}
	if got := request(http.MethodPost, token, `{"host":"a.example","host":"b.example","upstream":"http://127.0.0.1:3000"}`); got.Code != http.StatusBadRequest {
		t.Fatalf("ambiguous site accepted: %d", got.Code)
	}
	if got := request(http.MethodPost, token, `{"host":"a.example","upstream":"http://user:password@127.0.0.1:3000"}`); got.Code != http.StatusBadRequest {
		t.Fatalf("credential-bearing upstream accepted: %d", got.Code)
	}
	resp := request(http.MethodPost, token, good)
	if resp.Code != http.StatusOK {
		t.Fatalf("read-only dry run rejected: %d %s", resp.Code, resp.Body.String())
	}
	var result struct {
		Valid   bool `json:"valid"`
		DryRun  bool `json:"dryRun"`
		Applied bool `json:"applied"`
		Site    Site `json:"site"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if !result.Valid || !result.DryRun || result.Applied || result.Site.Host != "app.example" || result.Site.Upstream != "http://127.0.0.1:3300" {
		t.Fatalf("invalid preview: %+v", result)
	}
	if loadCalls != 0 {
		t.Fatalf("dry-run invoked live Caddy loader %d times", loadCalls)
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("dry-run persisted managed state: %v", err)
	}
	if sites, running := manager.Snapshot(); len(sites) != 0 || running {
		t.Fatalf("dry-run mutated manager state: %v, running=%v", sites, running)
	}
}

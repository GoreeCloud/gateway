package gateway

import (
	"encoding/json"
	"io/fs"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestAPIAuthenticationAndWriteGates(t *testing.T) {
	m, err := Open(t.TempDir()+"/sites.json", func([]byte, bool) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if err = m.Start(); err != nil {
		t.Fatal(err)
	}
	static := fs.FS(fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("<html>Ready</html>")}})
	h := Handler(m, static, strings.Repeat("x", 32), false)
	request := func(method, path, token, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	if x := request("GET", "/api/health", "", ""); x.Code != 200 {
		t.Fatalf("health: %d", x.Code)
	}
	if x := request("GET", "/api/status", "", ""); x.Code != 401 {
		t.Fatalf("missing auth: %d", x.Code)
	}
	if x := request("POST", "/api/sites", strings.Repeat("x", 32), `{"host":"app.example.com","upstream":"http://localhost:3000"}`); x.Code != 403 {
		t.Fatalf("write gate: %d", x.Code)
	}
	h = Handler(m, static, strings.Repeat("x", 32), true)
	r := request("POST", "/api/sites", strings.Repeat("x", 32), `{"host":"app.example.com","upstream":"http://localhost:3000"}`)
	if r.Code != 201 {
		t.Fatalf("add: %d %s", r.Code, r.Body.String())
	}
	l := request("GET", "/api/sites", strings.Repeat("x", 32), "")
	var out struct {
		Sites []Site `json:"sites"`
	}
	if err = json.Unmarshal(l.Body.Bytes(), &out); err != nil || len(out.Sites) != 1 {
		t.Fatalf("list: %s", l.Body.String())
	}
	if x := request("GET", "/", "", ""); x.Code != 200 {
		t.Fatalf("html: %d", x.Code)
	}
	if x := request("POST", "/api/sites", strings.Repeat("x", 32), `{"host":"evil.com","upstream":"http://localhost:3000","other":1}`); x.Code != 400 {
		t.Fatalf("unknown field: %d", x.Code)
	}
}

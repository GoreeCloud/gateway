package gateway

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestSiteJSONMediaTypeAndBoundedRequestBody(t *testing.T) {
	manager, err := Open(t.TempDir()+"/sites.json", func([]byte, bool) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Start(); err != nil {
		t.Fatal(err)
	}
	token := strings.Repeat("t", 32)
	handler := Handler(manager, fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("<html></html>")}}, token, true)
	body := func(host string) string {
		return fmt.Sprintf("{\"host\":%q,\"upstream\":\"http://127.0.0.1:3000\"}", host)
	}
	request := func(contents string, mediaTypes ...string) int {
		t.Helper()
		r := httptest.NewRequest(http.MethodPost, "/api/sites", strings.NewReader(contents))
		r.Host = "127.0.0.1:8787"
		r.Header.Set("Authorization", "Bearer "+token)
		for _, ct := range mediaTypes {
			r.Header.Add("Content-Type", ct)
		}
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, r)
		return recorder.Code
	}
	good := body("a.example.com")
	if code := request(good, "application/json"); code != 201 {
		t.Fatalf("ordinary JSON rejected: %d", code)
	}
	if code := request(body("b.example.com"), "application/json; charset=UTF-8"); code != 201 {
		t.Fatalf("charset UTF-8 rejected: %d", code)
	}
	exact := body("c.example.com")
	exact += strings.Repeat(" ", (8<<10)-len(exact))
	if code := request(exact, "application/json"); code != 201 {
		t.Fatalf("JSON exactly 8 KiB rejected: %d", code)
	}
	tooLarge := body("d.example.com")
	tooLarge += strings.Repeat(" ", (8<<10)+1-len(tooLarge))
	if code := request(tooLarge, "application/json"); code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized valid JSON plus whitespace accepted (%d)", code)
	}
	if code := request(body("e.example.com"), "application/json-malicious"); code != 400 {
		t.Fatalf("wrong media type accepted: %d", code)
	}
	if code := request(body("f.example.com"), "application/json", "application/json"); code != 400 {
		t.Fatalf("duplicate Content-Type accepted: %d", code)
	}
	if code := request(body("g.example.com"), "application/json; charset=iso-8859-1"); code != 400 {
		t.Fatalf("non-UTF-8 JSON accepted: %d", code)
	}
	if code := request(body("h.example.com"), "application/json; boundary=abc"); code != 400 {
		t.Fatalf("unexpected JSON parameter accepted: %d", code)
	}
	if code := request(body("i.example.com")+" {}", "application/json"); code != 400 {
		t.Fatalf("trailing JSON accepted: %d", code)
	}
	if code := request(body("j.example.com")); code != 400 {
		t.Fatalf("missing Content-Type accepted: %d", code)
	}
	sites, _ := manager.Snapshot()
	if len(sites) != 3 {
		t.Fatalf("invalid input mutated stored sites: %d", len(sites))
	}
}

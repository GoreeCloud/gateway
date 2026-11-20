package gateway

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReadSiteRejectsAmbiguousDuplicateFields(t *testing.T) {
	inputs := []string{
		`{"host":"one.example","host":"two.example","upstream":"http://127.0.0.1:3000"}`,
		`{"host":"one.example","upstream":"http://127.0.0.1:3000","upstream":"http://127.0.0.1:3001"}`,
		`{"Host":"one.example","host":"two.example","upstream":"http://127.0.0.1:3000"}`,
		`{"host":"one.example","UpStream":"http://127.0.0.1:3000","upstream":"http://127.0.0.1:3001"}`,
	}
	for _, body := range inputs {
		request := httptest.NewRequest(http.MethodPost, "/api/sites", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		if _, err := readSite(httptest.NewRecorder(), request); err == nil {
			t.Fatalf("duplicate field accepted: %s", body)
		}
	}
}

func TestReadSiteStrictFieldsPreserveBoundedSemantics(t *testing.T) {
	good := `{"host":"A.EXAMPLE","upstream":"http://127.0.0.1:3000"}`
	request := httptest.NewRequest(http.MethodPost, "/api/sites", strings.NewReader(good))
	request.Header.Set("Content-Type", "application/json")
	site, err := readSite(httptest.NewRecorder(), request)
	if err != nil || site.Host != "a.example" {
		t.Fatalf("valid route rejected or not canonicalized: %+v, %v", site, err)
	}

	request = httptest.NewRequest(http.MethodPost, "/api/sites", strings.NewReader(good+strings.Repeat(" ", 8192)))
	request.Header.Set("Content-Type", "application/json")
	_, err = readSite(httptest.NewRecorder(), request)
	if !errors.Is(err, errSiteBodyTooLarge) {
		t.Fatalf("oversized otherwise-valid document should fail with 413: %v", err)
	}

	request = httptest.NewRequest(http.MethodPost, "/api/sites", strings.NewReader(`{"host":"a.example","upstream":"http://127.0.0.1:3000","extra":true}`))
	request.Header.Set("Content-Type", "application/json")
	if _, err = readSite(httptest.NewRecorder(), request); err == nil {
		t.Fatal("unknown route field accepted")
	}
}

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOfflineInspectSitesIsStrictAndNonMutating(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sites.json")
	state := []byte(`[{"host":"example.test","upstream":"http://127.0.0.1:3444"}]`)
	if err := os.WriteFile(path, state, 0600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := inspectSites([]string{"--file", path}, &out); err != nil {
		t.Fatal(err)
	}
	var report struct {
		ReadOnly        bool `json:"readOnly"`
		Valid           bool `json:"valid"`
		CaddyAdapted    bool `json:"caddyAdapted"`
		SiteCount       int  `json:"siteCount"`
		Applied         bool `json:"applied"`
		RestoreTested   bool `json:"restoreTested"`
		ProductionReady bool `json:"productionReady"`
	}
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if !report.ReadOnly || !report.Valid || !report.CaddyAdapted || report.SiteCount != 1 ||
		report.Applied || report.RestoreTested || report.ProductionReady {
		t.Fatalf("misleading offline report: %+v", report)
	}
	if bytes.Contains(out.Bytes(), []byte("example.test")) || bytes.Contains(out.Bytes(), []byte("127.0.0.1")) ||
		bytes.Contains(out.Bytes(), []byte(dir)) {
		t.Fatal("private route details leaked into offline report")
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(after, state) {
		t.Fatal("offline inspection changed saved-state artifact")
	}
	if _, err := os.Stat(filepath.Join(dir, ".gateway-state")); !os.IsNotExist(err) {
		t.Fatal("offline inspection created a Caddy state directory")
	}
}

func TestOfflineInspectSitesFailsClosedAndRedacts(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "sites.json")
	for _, state := range []string{
		`[{"host":"x.example","host":"y.example","upstream":"http://127.0.0.1:3000"}]`,
		`[{"host":"x.example","upstream":"http://user:password@127.0.0.1:3000"}]`,
		`[] trailing`,
	} {
		if err := os.WriteFile(target, []byte(state), 0600); err != nil {
			t.Fatal(err)
		}
		var out bytes.Buffer
		err := inspectSites([]string{"--file", target}, &out)
		if err == nil {
			t.Fatalf("unsafe recovery candidate accepted: %s", state)
		}
		if out.Len() != 0 || strings.Contains(err.Error(), dir) || strings.Contains(err.Error(), "x.example") ||
			strings.Contains(err.Error(), "password") {
			t.Fatal("sensitive failed recovery candidate leaked")
		}
	}
	if err := inspectSites([]string{"--file", filepath.Join(dir, "missing.json")}, &bytes.Buffer{}); err == nil {
		t.Fatal("missing backup was misreported valid")
	}
	for _, args := range [][]string{{}, {"--file"}, {"--file", target, "--extra"}, {"--unknown", target}} {
		if err := inspectSites(args, &bytes.Buffer{}); err == nil {
			t.Fatalf("invalid CLI accepted: %v", args)
		}
	}
}

package gateway

import (
	"bytes"
	"testing"

	_ "github.com/caddy-dns/porkbun"
	"github.com/caddyserver/caddy/v2/caddyconfig"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
)

// Offline-only adaptation: no API key, external DNS, ACME or network access.
func TestLegacyPorkbunDNSModuleOfflineAdaptation(t *testing.T) {
	source := []byte("{\n acme_dns porkbun {\n api_key {env.PORKBUN_API_KEY}\n api_secret_key {env.PORKBUN_API_SECRET_KEY}\n }\n}\napp.example.invalid {\n reverse_proxy 127.0.0.1:18765\n}\n")
	adapter := caddyconfig.GetAdapter("caddyfile")
	if adapter == nil {
		t.Fatal("missing Caddyfile adapter")
	}
	adapted, warnings, err := adapter.Adapt(caddyfile.Format(source), nil)
	if err != nil || len(warnings) > 0 {
		t.Fatalf("adapt Porkbun: %v warnings=%v", err, warnings)
	}
	if !bytes.Contains(adapted, []byte("\"name\":\"porkbun\"")) {
		t.Fatalf("Porkbun module absent")
	}
}

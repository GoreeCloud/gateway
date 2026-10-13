package tlsconfig

import (
	"testing"

	"github.com/GoreeCloud/goreecloud-gateway/internal/config"
)

func tlsInventoryConfig() *config.Config {
	return &config.Config{
		Schema: "goreecloud-gateway-config/v1",
		Services: []config.Service{{ID: "svc", Name: "Service", BackendIDs: []string{"backend"}}},
		Backends: []config.Backend{{ID: "backend", URL: "http://127.0.0.1:8080", Enabled: true}},
		CertificateProfiles: []config.CertificateProfile{{
			ID: "private-cert", Enabled: true, CertificateFile: "/run/goreecloud/certs/private.pem", PrivateKeyFile: "/run/goreecloud/certs/private-key.pem",
		}},
		Routes: []config.Route{{
			ID: "route", ServiceID: "svc", Hostname: "App.GoreeCloud.com", PathPrefix: "/", Exposure: "internal", Enabled: true,
			TLS: config.RouteTLS{Mode: "required", CertificateProfile: "private-cert"},
		}},
	}
}

func TestInventorySelectsCertificateProfileByNormalizedServerName(t *testing.T) {
	inventory, err := NewInventory(tlsInventoryConfig())
	if err != nil {
		t.Fatal(err)
	}
	profile, ok := inventory.ProfileForServerName("app.goreecloud.com.")
	if !ok || profile.ID != "private-cert" {
		t.Fatalf("profile=%+v ok=%v", profile, ok)
	}
}

func TestInventoryRejectsConflictingHostnameProfiles(t *testing.T) {
	cfg := tlsInventoryConfig()
	cfg.CertificateProfiles = append(cfg.CertificateProfiles, config.CertificateProfile{
		ID: "other-cert", Enabled: true, CertificateFile: "/run/goreecloud/certs/other.pem", PrivateKeyFile: "/run/goreecloud/certs/other-key.pem",
	})
	cfg.Routes = append(cfg.Routes, config.Route{
		ID: "route-two", ServiceID: "svc", Hostname: "app.goreecloud.com", PathPrefix: "/other", Enabled: true,
		TLS: config.RouteTLS{Mode: "required", CertificateProfile: "other-cert"},
	})
	if _, err := NewInventory(cfg); err == nil {
		t.Fatal("conflicting hostname certificate profiles unexpectedly accepted")
	}
}

func TestInventoryDoesNotSelectTLSDisabledRoute(t *testing.T) {
	cfg := tlsInventoryConfig()
	cfg.Routes[0].TLS = config.RouteTLS{Mode: "disabled"}
	inventory, err := NewInventory(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := inventory.ProfileForServerName("app.goreecloud.com"); ok {
		t.Fatal("TLS-disabled route unexpectedly populated SNI inventory")
	}
}

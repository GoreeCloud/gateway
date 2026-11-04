//go:build keycloak_integration

package main

// A real issuer acceptance test, compiled only under -tags=keycloak_integration.
// The Identity GitHub workflow supplies short-lived tokens from a disposable
// HTTPS Keycloak container and a one-day self-signed localhost certificate.
// This never runs against a real user, production issuer, or deployed Gateway.

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

const (
	testKeycloakIssuer = "https://localhost:8443/realms/goreecloud"
	testFixtureDir     = "/tmp/goreecloud-keycloak-gateway-ci"
)

func readPrivateFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := testFixtureDir + "/" + name
	st, err := os.Lstat(path)
	if err != nil || !st.Mode().IsRegular() || st.Mode().Perm()&0077 != 0 ||
		st.Size() == 0 || st.Size() > 262144 {
		t.Fatalf("disposable %s fixture unavailable or has unsafe permissions", name)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read disposable %s fixture", name)
	}
	return data
}

func TestRealKeycloakGatewayHTTPSAccessToken(t *testing.T) {
	switch os.Getenv("GOREE_TEST_KEYCLOAK_HTTPS_GATEWAY") {
	case "":
		t.Skip("HTTPS Keycloak integration is disabled")
	case "1":
		if os.Getenv("GITHUB_ACTIONS") != "true" ||
			os.Getenv("GITHUB_REPOSITORY") != "GoreeCloud/identity" ||
			os.Getenv("CI") != "true" {
			t.Fatal("enabled integration requires isolated GoreeCloud Identity CI")
		}
	default:
		t.Fatal("invalid explicit HTTPS Keycloak integration test flag")
	}

	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(readPrivateFixture(t, "tls.crt")) {
		t.Fatal("invalid disposable localhost CA")
	}
	client := &http.Client{
		Timeout: 6 * time.Second,
		Transport: &http.Transport{
			Proxy: nil,
			TLSClientConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
				RootCAs:    roots,
			},
		},
	}
	settings := gatewayOIDCSettings{
		Issuer: testKeycloakIssuer, Audience: "goreecloud-gateway-console",
		ClientID: "goreecloud-ci-public", Tenant: "family",
	}
	if err := settings.validate(); err != nil {
		t.Fatal("real HTTPS issuer unexpectedly invalid")
	}
	verifier, err := newGatewayOIDCAuth(settings, client)
	if err != nil {
		t.Fatal("cannot instantiate pinned HTTPS Keycloak verifier")
	}
	access := strings.TrimSpace(string(readPrivateFixture(t, "access.jwt")))
	id := strings.TrimSpace(string(readPrivateFixture(t, "id.jwt")))
	if !verifier.authorize(context.Background(), access) {
		t.Fatal("real HTTPS Keycloak Gateway token rejected")
	}

	// Test the *real* read-only console handler, with legacy static token and
	// OIDC accepted through the same Authorization header path.
	admin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"apps":{"http":{"servers":{}}}}`))
	}))
	defer admin.Close()
	static := strings.Repeat("S", 48)
	handler, err := newHandlerWithOIDC(static, admin.URL, verifier)
	if err != nil {
		t.Fatal("cannot instantiate read-only Gateway console")
	}
	check := func(label, raw string, want int) {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:7689/api/v1/overview", nil)
		if raw != "" {
			req.Header.Set("Authorization", "Bearer "+raw)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Errorf("%s: unexpected access decision status %d", label, rec.Code)
		}
	}
	check("signed_keycloak", access, http.StatusOK)
	check("legacy_static", static, http.StatusOK)
	check("id_token", id, http.StatusUnauthorized)
	check("unknown_token", "invalid", http.StatusUnauthorized)
	check("no_token", "", http.StatusUnauthorized)

	parts := strings.Split(access, ".")
	if len(parts) != 3 || len(parts[2]) < 8 {
		t.Fatal("unexpected signed JWT layout")
	}
	tampered := parts[2]
	if tampered[0] == 'A' {
		tampered = "B" + tampered[1:]
	} else {
		tampered = "A" + tampered[1:]
	}
	check("modified_signature", strings.Join([]string{parts[0], parts[1], tampered}, "."), http.StatusUnauthorized)

	// A *valid* Keycloak signature is not enough to grant another service,
	// OAuth client or tenant access to the same read-only Gateway API.
	for _, variant := range []struct {
		name string
		edit func(*gatewayOIDCSettings)
	}{
		{"wrong_audience", func(s *gatewayOIDCSettings) { s.Audience = "unrelated-service" }},
		{"wrong_client", func(s *gatewayOIDCSettings) { s.ClientID = "unrelated-client" }},
		{"wrong_tenant", func(s *gatewayOIDCSettings) { s.Tenant = "other-tenant" }},
	} {
		t.Run(variant.name, func(t *testing.T) {
			other := settings
			variant.edit(&other)
			a, err := newGatewayOIDCAuth(other, client)
			if err != nil {
				t.Fatal("negative-test verifier setup failed")
			}
			if a.authorize(context.Background(), access) {
				t.Fatal("real signed Keycloak token accepted for wrong trust configuration")
			}
		})
	}
}

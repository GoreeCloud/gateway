package main

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type oidcFixture struct {
	key    *rsa.PrivateKey
	issuer string
	config gatewayOIDCSettings
	server *httptest.Server
	calls  atomic.Int32
}

func newOIDCFixture(t *testing.T) *oidcFixture {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	f := &oidcFixture{key: key}
	f.server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.calls.Add(1)
		if r.URL.Path != "/realms/goreecloud/protocol/openid-connect/certs" {
			http.NotFound(w, r)
			return
		}
		pub := key.PublicKey
		jwks := map[string]any{"keys": []any{map[string]any{
			"kty": "RSA", "use": "sig", "alg": "RS256", "kid": "test-issuer-key",
			"n": base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
			"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
		}}}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jwks)
	}))
	t.Cleanup(f.server.Close)
	f.issuer = f.server.URL + "/realms/goreecloud"
	f.config = gatewayOIDCSettings{
		Issuer: f.issuer, Audience: "goreecloud-gateway-console",
		ClientID: "goreecloud-cli-test", Tenant: "family",
	}
	return f
}

func (f *oidcFixture) token(t *testing.T, overrides map[string]any) string {
	t.Helper()
	now := time.Now().Unix()
	claims := map[string]any{
		"iss": f.issuer, "aud": f.config.Audience, "sub": "synthetic-user",
		"iat": now - 3, "exp": now + 250, "nbf": now - 3,
		"typ": "Bearer", "azp": f.config.ClientID,
		"scope": "openid gateway:read", "goreecloud_tenant": "family",
		"realm_access": map[string]any{"roles": []string{"gateway-reader"}},
	}
	for k, v := range overrides {
		claims[k] = v
	}
	hdr, _ := json.Marshal(map[string]any{"alg": "RS256", "kid": "test-issuer-key", "typ": "JWT"})
	payload, _ := json.Marshal(claims)
	data := base64.RawURLEncoding.EncodeToString(hdr) + "." + base64.RawURLEncoding.EncodeToString(payload)
	digest := sha256.Sum256([]byte(data))
	signature, err := rsa.SignPKCS1v15(rand.Reader, f.key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	return data + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func (f *oidcFixture) auth(t *testing.T) *verifiedGatewayBearer {
	t.Helper()
	a, err := newGatewayOIDCAuth(f.config, f.server.Client())
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestGatewayOIDCVerifiedBearer(t *testing.T) {
	f := newOIDCFixture(t)
	a := f.auth(t)
	good := f.token(t, nil)
	if !a.authorize(context.Background(), good) {
		t.Fatal("valid scoped, role-bound token denied")
	}
	if !a.authorize(context.Background(), good) {
		t.Fatal("valid cached token denied")
	}
	if f.calls.Load() != 1 {
		t.Fatalf("expected one pinned signing-key fetch, got %d", f.calls.Load())
	}
}

func TestGatewayOIDCRejectsUntrustedClaims(t *testing.T) {
	f := newOIDCFixture(t)
	a := f.auth(t)
	now := time.Now().Unix()
	tests := []struct {
		name string
		v    map[string]any
	}{
		{"issuer", map[string]any{"iss": "https://evil.example/realms/goreecloud"}},
		{"audience", map[string]any{"aud": "other-resource"}},
		{"client", map[string]any{"azp": "other-client"}},
		{"missing_scope", map[string]any{"scope": "openid"}},
		{"scope_substring", map[string]any{"scope": "openid gateway:reader"}},
		{"tenant", map[string]any{"goreecloud_tenant": "business"}},
		{"role", map[string]any{"realm_access": map[string]any{"roles": []string{"visitor"}}}},
		{"type", map[string]any{"typ": "ID"}},
		{"subject", map[string]any{"sub": ""}},
		{"expired", map[string]any{"exp": now - 300}},
		{"far_future", map[string]any{"exp": now + 9000}},
		{"iat_future", map[string]any{"iat": now + 900}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if a.authorize(context.Background(), f.token(t, tt.v)) {
				t.Fatal("untrusted access-token claim accepted")
			}
		})
	}
}

func TestGatewayOIDCRejectsTampering(t *testing.T) {
	f := newOIDCFixture(t)
	a := f.auth(t)
	raw := f.token(t, nil)
	parts := strings.Split(raw, ".")
	parts[1] = base64.RawURLEncoding.EncodeToString([]byte(`{"iss":"https://evil.example","sub":"attacker"}`))
	if a.authorize(context.Background(), strings.Join(parts, ".")) {
		t.Fatal("tampered signature accepted")
	}
	if a.authorize(context.Background(), "opaque-static-token") {
		t.Fatal("opaque token accepted")
	}
	if a.authorize(context.Background(), strings.Repeat("a", 17000)) {
		t.Fatal("oversized token accepted")
	}
}

func TestGatewayOIDCConfigurationFailClosed(t *testing.T) {
	f := newOIDCFixture(t)
	for _, issuer := range []string{
		"http://example.test/realms/demo",
		"https://user:password@example.test/realms/demo",
		"https://example.test/realms/demo?redirect=https://evil.test",
		"https://example.test/realms/demo/extra",
		"https://example.test/realms/%2e%2e",
		"file:///etc/passwd",
	} {
		t.Run(issuer, func(t *testing.T) {
			c := f.config
			c.Issuer = issuer
			if _, err := newGatewayOIDCAuth(c, nil); err == nil {
				t.Fatal("unsafe issuer accepted")
			}
		})
	}
}

func TestGatewayOIDCIssuerPortBounds(t *testing.T) {
	settings := gatewayOIDCSettings{
		Audience: "goreecloud-gateway-console",
		ClientID: "synthetic-client", Tenant: "family",
	}
	for _, port := range []string{"", "0", "00000", "65536", "99999", "99999999999999999999"} {
		t.Run("reject_"+port, func(t *testing.T) {
			settings.Issuer = "https://issuer.example:" + port + "/realms/goreecloud"
			if err := settings.validate(); err == nil {
				t.Fatal("issuer with invalid explicit HTTPS port accepted")
			}
		})
	}
	for _, port := range []string{"443", "8443", "65535"} {
		t.Run("accept_"+port, func(t *testing.T) {
			settings.Issuer = "https://issuer.example:" + port + "/realms/goreecloud"
			if err := settings.validate(); err != nil {
				t.Fatalf("valid private HTTPS issuer port rejected: %v", err)
			}
		})
	}
}

func TestGatewayOIDCOptInDefaultOff(t *testing.T) {
	t.Setenv("GOREE_GATEWAY_OIDC_ENABLED", "")
	a, err := optInGatewayOIDC()
	if err != nil || a != nil {
		t.Fatalf("default must be disabled, got %v", err)
	}
	t.Setenv("GOREE_GATEWAY_OIDC_ENABLED", "true")
	if _, err := optInGatewayOIDC(); err == nil {
		t.Fatal("nonexplicit enable accepted")
	}
	t.Setenv("GOREE_GATEWAY_OIDC_ENABLED", "1")
	for _, v := range []string{"GOREE_GATEWAY_OIDC_ISSUER", "GOREE_GATEWAY_OIDC_AUDIENCE", "GOREE_GATEWAY_OIDC_CLIENT_ID", "GOREE_GATEWAY_OIDC_TENANT"} {
		t.Setenv(v, "")
	}
	if _, err := optInGatewayOIDC(); err == nil {
		t.Fatal("incomplete opt-in accepted")
	}
}

func TestGatewayOIDCReadOnlyConsoleCompatibility(t *testing.T) {
	f := newOIDCFixture(t)
	a := f.auth(t)
	admin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"apps":{"http":{"servers":{}}}}`))
	}))
	defer admin.Close()
	staticSecret := strings.Repeat("S", 48)
	handler, err := newHandlerWithOIDC(staticSecret, admin.URL, a)
	if err != nil {
		t.Fatal(err)
	}
	testRequest := func(label, token string, expected int) {
		t.Helper()
		r := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:7689/api/v1/overview", nil)
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, r)
		if rec.Code != expected {
			t.Errorf("%s: got %d want %d", label, rec.Code, expected)
		}
	}
	testRequest("static_legacy", staticSecret, http.StatusOK)
	testRequest("valid_oidc", f.token(t, nil), http.StatusOK)
	testRequest("oidc_missing_role", f.token(t, map[string]any{"realm_access": map[string]any{"roles": []string{"visitor"}}}), http.StatusUnauthorized)
	testRequest("missing", "", http.StatusUnauthorized)
	testRequest("wrong", "invalid", http.StatusUnauthorized)
	r := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:7689/api/v1/overview", nil)
	r.Header.Add("Authorization", "Bearer "+f.token(t, nil))
	r.Header.Add("Authorization", "Bearer "+staticSecret)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, r)
	if rec.Code != http.StatusUnauthorized {
		t.Fatal("duplicate Authorization headers accepted")
	}
}

func TestGatewayOIDCDefaultHandlerStillStaticOnly(t *testing.T) {
	h, err := newHandler(strings.Repeat("Q", 48), "http://127.0.0.1:2019")
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:7689/api/v1/overview", nil)
	r.Header.Set("Authorization", "Bearer a.fake.jwt")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	if rec.Code != http.StatusUnauthorized {
		t.Fatal("default handler unexpectedly accepted OIDC")
	}
}

func TestGatewayOIDCRejectsJWKSRedirect(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	_ = key
	redirect := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://elsewhere.example/certs", http.StatusFound)
	}))
	defer redirect.Close()
	cfg := gatewayOIDCSettings{Issuer: redirect.URL + "/realms/goreecloud", Audience: "res", ClientID: "client", Tenant: "family"}
	a, err := newGatewayOIDCAuth(cfg, redirect.Client())
	if err != nil {
		t.Fatal(err)
	}
	validJWT := fTokenForRedirect(t, cfg, key)
	if a.authorize(context.Background(), validJWT) {
		t.Fatal("unknown jwt accepted")
	}
}

func fTokenForRedirect(t *testing.T, cfg gatewayOIDCSettings, key *rsa.PrivateKey) string {
	t.Helper()
	now := time.Now().Unix()
	claims := map[string]any{
		"iss": cfg.Issuer, "aud": cfg.Audience, "sub": "synthetic-user",
		"azp": cfg.ClientID, "iat": now - 2, "exp": now + 180, "typ": "Bearer",
		"scope": "gateway:read", "goreecloud_tenant": cfg.Tenant,
		"realm_access": map[string]any{"roles": []string{"gateway-reader"}},
	}
	hdr, _ := json.Marshal(map[string]any{"alg": "RS256", "kid": "test-issuer-key"})
	payload, _ := json.Marshal(claims)
	signed := base64.RawURLEncoding.EncodeToString(hdr) + "." + base64.RawURLEncoding.EncodeToString(payload)
	digest := sha256.Sum256([]byte(signed))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	return signed + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func TestGatewayOIDCValidateOnlyHTTPS(t *testing.T) {
	u, _ := url.Parse("https://example.test/realms/tenant")
	if u.Scheme != "https" {
		t.Fatal("invalid test issuer")
	}
	c := gatewayOIDCSettings{Issuer: u.String(), Audience: "res", ClientID: "cli", Tenant: "family"}
	if err := c.validate(); err != nil {
		t.Fatal(err)
	}
}

// Keep TLS trust verification to the vetted net/http TLS stack. Test fixture
// clients carry a local certificate trust root; production never skips TLS.

package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
)

// gatewayOIDCAuth is deliberately optional. Its bearer-token checks are
// additive: the pre-existing static console token remains accepted unchanged.
// This API is read-only; auth-code browser sessions and public ingress are NOT
// implemented by this component.
type gatewayOIDCAuth interface {
	authorize(context.Context, string) bool
}

type gatewayOIDCSettings struct {
	Issuer   string
	Audience string
	ClientID string
	Tenant   string
}

const (
	requiredGatewayScope = "gateway:read"
	requiredGatewayRole  = "gateway-reader"
	maxGatewayJWTLength  = 16384
)

var realmName = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

func (c gatewayOIDCSettings) validate() error {
	if c.Audience == "" || c.ClientID == "" || c.Tenant == "" ||
		len(c.Audience) > 256 || len(c.ClientID) > 256 || len(c.Tenant) > 256 ||
		strings.ContainsAny(c.Audience+c.ClientID+c.Tenant, " \r\n\t") {
		return errors.New("incomplete trusted OIDC settings")
	}
	u, err := url.Parse(c.Issuer)
	if err != nil || u == nil || u.Scheme != "https" || u.Hostname() == "" ||
		u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" ||
		u.RawPath != "" || strings.HasSuffix(c.Issuer, "/") || len(c.Issuer) > 2048 {
		return errors.New("OIDC issuer must be an exact HTTPS realm URL")
	}
	parts := strings.Split(u.EscapedPath(), "/")
	if len(parts) != 3 || parts[0] != "" || parts[1] != "realms" ||
		!realmName.MatchString(parts[2]) {
		return errors.New("OIDC issuer must be a canonical Keycloak realm")
	}
	if u.Port() != "" {
		// HTTPS over explicit non-default ports is valid for private deployments.
		if u.Port() == "0" {
			return errors.New("invalid issuer port")
		}
	}
	return nil
}

// jwksThrottledTransport bounds network GETs triggered by attacker-supplied
// unknown JWT key IDs. Rejections fail closed; valid cached keys remain usable.
type jwksThrottledTransport struct {
	base     http.RoundTripper
	mu       sync.Mutex
	last     time.Time
	now      func() time.Time
	cooldown time.Duration
	// full URL is pinned; do not permit arbitrary discovery or token URLs.
	approvedURL string
}

func (t *jwksThrottledTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.Method != http.MethodGet || r.URL.String() != t.approvedURL {
		return nil, errors.New("unexpected signing-key HTTP request")
	}
	t.mu.Lock()
	now := t.now()
	if !t.last.IsZero() && now.Sub(t.last) < t.cooldown {
		t.mu.Unlock()
		return nil, errors.New("signing-key refresh is rate-limited")
	}
	t.last = now
	t.mu.Unlock()
	response, err := t.base.RoundTrip(r)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		return response, nil
	}
	// Enforce an independent 256 KiB cap; never allow oversized issuer data
	// even when the remote library uses a permissive response reader.
	body, err := io.ReadAll(io.LimitReader(response.Body, 262145))
	_ = response.Body.Close()
	if err != nil || len(body) > 262144 {
		return nil, errors.New("oversized or unreadable signing-key response")
	}
	response.Body = io.NopCloser(bytes.NewReader(body))
	response.ContentLength = int64(len(body))
	return response, nil
}

type verifiedGatewayBearer struct {
	settings gatewayOIDCSettings
	verifier *oidc.IDTokenVerifier
}

func newGatewayOIDCAuth(c gatewayOIDCSettings, client *http.Client) (*verifiedGatewayBearer, error) {
	if err := c.validate(); err != nil {
		return nil, err
	}
	jwksURL := c.Issuer + "/protocol/openid-connect/certs"
	if client == nil {
		client = &http.Client{
			Transport: &http.Transport{
				Proxy:           nil,
				TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
			},
		}
	}
	base := client.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	guard := &jwksThrottledTransport{
		base: base, now: time.Now, cooldown: 10 * time.Second,
		approvedURL: jwksURL,
	}
	pinnedClient := &http.Client{
		Timeout: 5 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Transport: guard,
	}
	ctx := oidc.ClientContext(context.Background(), pinnedClient)
	keys := oidc.NewRemoteKeySet(ctx, jwksURL)
	verifier := oidc.NewVerifier(c.Issuer, keys, &oidc.Config{
		ClientID:             c.Audience,
		SupportedSigningAlgs: []string{"RS256"},
	})
	return &verifiedGatewayBearer{settings: c, verifier: verifier}, nil
}

type gatewayBearerClaims struct {
	Type        string `json:"typ"`
	Client      string `json:"azp"`
	Subject     string `json:"sub"`
	Scope       string `json:"scope"`
	Tenant      string `json:"goreecloud_tenant"`
	IssuedAt    int64  `json:"iat"`
	NotBefore   int64  `json:"nbf"`
	ExpiresAt   int64  `json:"exp"`
	RealmAccess struct {
		Roles []string `json:"roles"`
	} `json:"realm_access"`
}

func (a *verifiedGatewayBearer) authorize(ctx context.Context, raw string) bool {
	if raw == "" || len(raw) > maxGatewayJWTLength || strings.Count(raw, ".") != 2 {
		return false
	}
	token, err := a.verifier.Verify(ctx, raw)
	if err != nil {
		return false
	}
	var claims gatewayBearerClaims
	if err := token.Claims(&claims); err != nil {
		return false
	}
	now := time.Now().Unix()
	if claims.Type != "Bearer" || claims.Client != a.settings.ClientID ||
		claims.Subject == "" || len(claims.Subject) > 256 ||
		claims.Tenant != a.settings.Tenant ||
		claims.IssuedAt <= 0 || claims.ExpiresAt <= claims.IssuedAt ||
		claims.ExpiresAt-claims.IssuedAt > 3600 ||
		claims.IssuedAt > now+30 || claims.NotBefore > now+30 || claims.ExpiresAt <= now {
		return false
	}
	scoped := false
	for _, scope := range strings.Fields(claims.Scope) {
		if scope == requiredGatewayScope {
			scoped = true
		}
	}
	if !scoped || len(claims.RealmAccess.Roles) > 256 {
		return false
	}
	for _, role := range claims.RealmAccess.Roles {
		if role == requiredGatewayRole {
			return true
		}
	}
	return false
}

// optInGatewayOIDC refuses incomplete configuration and preserves the historic
// static-token behavior unless the operator explicitly enables OIDC.
func optInGatewayOIDC() (gatewayOIDCAuth, error) {
	switch os.Getenv("GOREE_GATEWAY_OIDC_ENABLED") {
	case "":
		return nil, nil
	case "1":
		settings := gatewayOIDCSettings{
			Issuer:   os.Getenv("GOREE_GATEWAY_OIDC_ISSUER"),
			Audience: os.Getenv("GOREE_GATEWAY_OIDC_AUDIENCE"),
			ClientID: os.Getenv("GOREE_GATEWAY_OIDC_CLIENT_ID"),
			Tenant:   os.Getenv("GOREE_GATEWAY_OIDC_TENANT"),
		}
		return newGatewayOIDCAuth(settings, nil)
	default:
		return nil, fmt.Errorf("invalid GOREE_GATEWAY_OIDC_ENABLED value")
	}
}

// Verify compilation-time acceptance of only authenticated claim sets. It is
// deliberately not exported as a policy/header-mapping bypass.
var _ gatewayOIDCAuth = (*verifiedGatewayBearer)(nil)

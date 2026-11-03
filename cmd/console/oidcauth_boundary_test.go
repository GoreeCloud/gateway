package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSigningKeyTransportRateLimitedAndPinned(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/realms/goreecloud/protocol/openid-connect/certs" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"keys":[]}`))
	}))
	defer server.Close()
	url := server.URL + "/realms/goreecloud/protocol/openid-connect/certs"
	now := time.Unix(500, 0)
	guard := &jwksThrottledTransport{
		base: server.Client().Transport, now: func() time.Time { return now },
		cooldown: 10 * time.Second, approvedURL: url,
	}
	request, _ := http.NewRequest(http.MethodGet, url, nil)
	first, err := guard.RoundTrip(request)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, first.Body)
	_ = first.Body.Close()
	if _, err := guard.RoundTrip(request); err == nil {
		t.Fatal("unknown signing kid could trigger repeated network fetch")
	}
	now = now.Add(11 * time.Second)
	second, err := guard.RoundTrip(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = second.Body.Close()
	bad, _ := http.NewRequest(http.MethodGet, server.URL+"/other", nil)
	if _, err := guard.RoundTrip(bad); err == nil {
		t.Fatal("unexpected signing-key endpoint permitted")
	}
	post, _ := http.NewRequest(http.MethodPost, url, nil)
	if _, err := guard.RoundTrip(post); err == nil {
		t.Fatal("unexpected key-fetch method permitted")
	}
}

func TestSigningKeyTransportRejectsOversizedResponse(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(w, bytes.NewReader(bytes.Repeat([]byte("x"), 262146)))
	}))
	defer server.Close()
	url := server.URL + "/realms/goreecloud/protocol/openid-connect/certs"
	guard := &jwksThrottledTransport{
		base: server.Client().Transport, now: time.Now, cooldown: 10 * time.Second,
		approvedURL: url,
	}
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	resp, err := guard.RoundTrip(req)
	if err == nil || resp != nil {
		t.Fatal("oversized signing key response not rejected")
	}
}

func TestOIDCUnavailableIssuerFailsClosed(t *testing.T) {
	f := newOIDCFixture(t)
	auth := f.auth(t)
	signed := f.token(t, nil)
	f.server.Close()
	if auth.authorize(context.Background(), signed) {
		t.Fatal("signed bearer accepted without trustworthy issuer signing key")
	}
}

func TestBadGatewayBearerNeverAccepted(t *testing.T) {
	f := newOIDCFixture(t)
	a := f.auth(t)
	token := f.token(t, map[string]any{"goreecloud_tenant": "other"})
	if a.authorize(context.Background(), token) {
		t.Fatal("mismatched tenancy accepted")
	}
	// A fake identity-bearing string in an Authorization header is not an
	// authenticated Keycloak token, even when the prefix is 'Bearer '.
	if a.authorize(context.Background(), strings.Repeat("x", 1024)) {
		t.Fatal("string substitution accepted")
	}
}

package gateway

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/certmagic"
)

// TestIsolatedTrustedLocalHTTPS checks frontend TLS termination using an
// ephemeral, per-test certificate. No public DNS, ACME traffic, production
// ports, system trust-store edits, or real certificate state are involved.
func TestIsolatedTrustedLocalHTTPS(t *testing.T) {
	isolated := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(isolated, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(isolated, "data"))
	oldAutosave, oldStorage := caddy.ConfigAutosavePath, caddy.DefaultStorage
	caddy.ConfigAutosavePath = filepath.Join(isolated, "config", "caddy", "autosave.json")
	caddy.DefaultStorage = &certmagic.FileStorage{Path: filepath.Join(isolated, "data", "caddy")}
	defer func() { caddy.Stop(); caddy.ConfigAutosavePath = oldAutosave; caddy.DefaultStorage = oldStorage }()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: big.NewInt(101),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    now.Add(-time.Minute), NotAfter: now.Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:              []string{"localhost"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(cert)
	certFile, keyFile := filepath.Join(isolated, "fixture-cert.pem"), filepath.Join(isolated, "fixture-key.pem")
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0600); err != nil {
		t.Fatal(err)
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Gateway-Fixture", "trusted-https")
		_, _ = io.WriteString(w, "hello-over-tls")
	}))
	defer upstream.Close()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	source := fmt.Sprintf("{\n admin off\n auto_https disable_redirects\n}\nhttps://%s {\n bind 127.0.0.1\n tls %q %q\n reverse_proxy %s\n}\n", addr, certFile, keyFile, upstream.URL)
	adapter := caddyconfig.GetAdapter("caddyfile")
	if adapter == nil {
		t.Fatal("Caddyfile adapter unavailable")
	}
	cfg, warnings, err := adapter.Adapt(caddyfile.Format([]byte(source)), nil)
	if err != nil || len(warnings) != 0 {
		t.Fatalf("adapt local TLS: %v, warnings=%v", err, warnings)
	}
	if err = caddy.Load(cfg, true); err != nil {
		t.Fatalf("load local TLS: %v", err)
	}

	client := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}}}
	res, err := client.Get("https://" + addr + "/")
	if err != nil {
		t.Fatalf("trusted local TLS: %v", err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 128))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK || string(body) != "hello-over-tls" || res.Header.Get("X-Gateway-Fixture") != "trusted-https" {
		t.Fatalf("TLS proxy response: status=%d body=%q", res.StatusCode, body)
	}
	if res.TLS == nil || !res.TLS.HandshakeComplete || res.TLS.Version < tls.VersionTLS12 {
		t.Fatal("TLS handshake missing or insufficient")
	}
	// The same local TLS server must negotiate real HTTP/2 with a trusted
	// client. This checks h2 frontend termination, h1 upstream forwarding and
	// response integrity; it uses no public domain, DNS or ACME credentials.
	h2Client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig:   &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12},
			ForceAttemptHTTP2: true,
		},
	}
	h2Response, err := h2Client.Get("https://" + addr + "/h2-check")
	if err != nil {
		t.Fatalf("trusted local HTTP/2: %v", err)
	}
	h2Body, err := io.ReadAll(io.LimitReader(h2Response.Body, 128))
	_ = h2Response.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if h2Response.ProtoMajor != 2 || h2Response.StatusCode != http.StatusOK ||
		string(h2Body) != "hello-over-tls" || h2Response.Header.Get("X-Gateway-Fixture") != "trusted-https" {
		t.Fatalf("HTTP/2 TLS proxy mismatch: protocol=%s status=%d body=%q", h2Response.Proto, h2Response.StatusCode, h2Body)
	}
	// A client without this isolated fixture CA must not trust the endpoint.
	untrusted := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}}}
	rejected, err := untrusted.Get("https://" + addr + "/")
	if err == nil {
		_ = rejected.Body.Close()
		t.Fatal("untrusted fixture certificate unexpectedly accepted")
	}
}

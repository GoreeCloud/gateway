package tlsconfig

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/GoreeCloud/goreecloud-gateway/internal/config"
)

func writeTestCertificate(t *testing.T, dir, prefix string) (string, string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil { t.Fatal(err) }
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(time.Now().UnixNano()), Subject: pkix.Name{CommonName: "gateway.example.test"}, DNSNames: []string{"gateway.example.test"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil { t.Fatal(err) }
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil { t.Fatal(err) }
	certPath, keyPath := filepath.Join(dir, prefix+"-cert.pem"), filepath.Join(dir, prefix+"-key.pem")
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil { t.Fatal(err) }
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0o600); err != nil { t.Fatal(err) }
	return certPath, keyPath
}

func reloadConfig(certPath, keyPath string) *config.Config {
	return &config.Config{
		Schema: "goreecloud-gateway-config/v1",
		Services: []config.Service{{ID: "svc", BackendIDs: []string{"backend"}}},
		Backends: []config.Backend{{ID: "backend", URL: "http://127.0.0.1:8080", Enabled: true}},
		CertificateProfiles: []config.CertificateProfile{{ID: "primary", CertificateFile: certPath, PrivateKeyFile: keyPath, Enabled: true}},
		Routes: []config.Route{{ID: "route", ServiceID: "svc", Hostname: "gateway.example.test", PathPrefix: "/", Exposure: "internal", Enabled: true, TLS: config.RouteTLS{Mode: "required", CertificateProfile: "primary"}}},
	}
}

func TestReloaderRejectsInvalidReplacementAndRetainsRuntime(t *testing.T) {
	dir := t.TempDir()
	certPath, keyPath := writeTestCertificate(t, dir, "first")
	reloader, err := NewReloader(reloadConfig(certPath, keyPath))
	if err != nil { t.Fatal(err) }
	before, err := reloader.TLSConfig().GetCertificate(nil)
	if err == nil || before != nil { t.Fatal("missing SNI unexpectedly selected a certificate") }

	bad := reloadConfig(certPath, filepath.Join(dir, "missing-key.pem"))
	if err := reloader.Reload(bad); err == nil { t.Fatal("invalid replacement unexpectedly accepted") }
	if _, err := reloader.TLSConfig().GetCertificate(nil); err == nil { t.Fatal("failed reload changed fail-closed SNI behavior") }
}

func TestReloaderPublishesValidatedReplacement(t *testing.T) {
	dir := t.TempDir()
	cert1, key1 := writeTestCertificate(t, dir, "first")
	cert2, key2 := writeTestCertificate(t, dir, "second")
	reloader, err := NewReloader(reloadConfig(cert1, key1))
	if err != nil { t.Fatal(err) }
	before := reloader.runtime
	if err := reloader.Reload(reloadConfig(cert2, key2)); err != nil { t.Fatal(err) }
	if before == reloader.runtime { t.Fatal("validated replacement was not published") }
}

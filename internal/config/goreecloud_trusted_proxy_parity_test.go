package config

import "testing"

func TestGoreeCloudTrustedProxyBoundaryChangesParityFingerprint(t *testing.T) {
    cfg := testParityConfig()
    baseline, err := ConfigParityFingerprint(cfg)
    if err != nil { t.Fatal(err) }
    cfg.TrustedProxies = []string{"192.0.2.0/24"}
    withTrusted, err := ConfigParityFingerprint(cfg)
    if err != nil { t.Fatal(err) }
    if baseline == withTrusted { t.Fatal("trusted proxy boundary omitted from parity identity") }
    cfg.TrustedProxies = []string{"192.0.2.0/24", "198.51.100.0/24"}
    first, err := ConfigParityFingerprint(cfg)
    if err != nil { t.Fatal(err) }
    cfg.TrustedProxies[0], cfg.TrustedProxies[1] = cfg.TrustedProxies[1], cfg.TrustedProxies[0]
    second, err := ConfigParityFingerprint(cfg)
    if err != nil { t.Fatal(err) }
    if first != second { t.Fatal("ordering changed trusted proxy semantic parity") }
}

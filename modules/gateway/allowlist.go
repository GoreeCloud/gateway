package gateway

import (
	"fmt"
	"net"
	"net/http"
	"net/netip"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
)

func init() {
	caddy.RegisterModule(IPAllowlist{})
	httpcaddyfile.RegisterHandlerDirective("goree_ip_allowlist", parseIPAllowlist)
}

// IPAllowlist guards an explicitly selected route by the directly connected
// source IP. X-Forwarded-For and other client-controlled headers are ignored.
type IPAllowlist struct {
	CIDRs    []string       `json:"cidrs,omitempty"`
	networks []netip.Prefix `json:"-"`
}

func (IPAllowlist) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID: "http.handlers.goree_ip_allowlist",
		New: func() caddy.Module { return new(IPAllowlist) },
	}
}

func (m *IPAllowlist) Provision(_ caddy.Context) error {
	networks, err := compilePrefixes(m.CIDRs)
	if err != nil {
		return err
	}
	m.networks = networks
	return nil
}

func compilePrefixes(cidrs []string) ([]netip.Prefix, error) {
	if len(cidrs) == 0 {
		return nil, fmt.Errorf("goree_ip_allowlist: at least one CIDR is required")
	}
	out := make([]netip.Prefix, 0, len(cidrs))
	for _, cidr := range cidrs {
		prefix, err := netip.ParsePrefix(cidr)
		if err != nil {
			return nil, fmt.Errorf("goree_ip_allowlist: invalid CIDR %q: %w", cidr, err)
		}
		out = append(out, prefix.Masked())
	}
	return out, nil
}

func (m *IPAllowlist) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return nil
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return nil
	}
	addr = addr.Unmap()
	for _, prefix := range m.networks {
		if prefix.Contains(addr) {
			return next.ServeHTTP(w, r)
		}
	}
	http.Error(w, "Forbidden", http.StatusForbidden)
	return nil
}

func parseIPAllowlist(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	m := new(IPAllowlist)
	for h.Next() {
		args := h.RemainingArgs()
		if len(args) == 0 {
			return nil, h.ArgErr()
		}
		m.CIDRs = append(m.CIDRs, args...)
		if h.NextBlock(0) {
			return nil, h.Err("goree_ip_allowlist accepts no block")
		}
	}
	if _, err := compilePrefixes(m.CIDRs); err != nil {
		return nil, err
	}
	return m, nil
}

var _ caddy.Provisioner = (*IPAllowlist)(nil)
var _ caddyhttp.MiddlewareHandler = (*IPAllowlist)(nil)

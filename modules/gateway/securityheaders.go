package gateway

import (
	"net/http"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
)

func init() {
	caddy.RegisterModule(SecurityHeaders{})
	httpcaddyfile.RegisterHandlerDirective("goree_security_headers", parseSecurityHeaders)
}

// SecurityHeaders adds conservative opt-in headers to routed responses.
// It does not set HSTS or CSP because those require application-specific policies.
type SecurityHeaders struct{}

func (SecurityHeaders) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID: "http.handlers.goree_security_headers",
		New: func() caddy.Module { return new(SecurityHeaders) },
	}
}

func (SecurityHeaders) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	h := w.Header()
	if h.Get("X-Content-Type-Options") == "" {
		h.Set("X-Content-Type-Options", "nosniff")
	}
	if h.Get("Referrer-Policy") == "" {
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
	}
	if h.Get("X-Frame-Options") == "" {
		h.Set("X-Frame-Options", "DENY")
	}
	return next.ServeHTTP(w, r)
}

func parseSecurityHeaders(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	for h.Next() {
		if h.NextArg() {
			return nil, h.ArgErr()
		}
		if h.NextBlock(0) {
			return nil, h.Err("goree_security_headers accepts no block")
		}
	}
	return new(SecurityHeaders), nil
}

var _ caddyhttp.MiddlewareHandler = (*SecurityHeaders)(nil)

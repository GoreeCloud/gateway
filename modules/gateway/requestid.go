package gateway

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
)

const requestIDHeader = "X-Request-ID"

func init() {
	caddy.RegisterModule(RequestID{})
	httpcaddyfile.RegisterHandlerDirective("goree_request_id", parseRequestID)
	httpcaddyfile.RegisterDirectiveOrder("goree_request_id", httpcaddyfile.Before, "handle")
}

// RequestID assigns a new cryptographically random ID to every request.
// Client-provided IDs are discarded to prevent forged correlation values.
type RequestID struct{}

func (RequestID) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.goree_request_id",
		New: func() caddy.Module { return new(RequestID) },
	}
}

func (RequestID) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return fmt.Errorf("goree_request_id: generate secure ID: %w", err)
	}
	id := hex.EncodeToString(buf[:])
	w.Header().Set(requestIDHeader, id)
	// Clone to avoid mutation of shared request state in other handlers.
	upstream := r.Clone(r.Context())
	upstream.Header.Set(requestIDHeader, id)
	return next.ServeHTTP(w, upstream)
}

func parseRequestID(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	for h.Next() {
		if h.NextArg() {
			return nil, h.ArgErr()
		}
		if h.NextBlock(0) {
			return nil, h.Err("goree_request_id accepts no block")
		}
	}
	return new(RequestID), nil
}

var _ caddyhttp.MiddlewareHandler = (*RequestID)(nil)

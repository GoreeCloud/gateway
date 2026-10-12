package proxy

import (
	"net/netip"

	"github.com/GoreeCloud/goreecloud-gateway/internal/config"
)

// permitsRouteClient is a network gate, not a replacement for application
// authorization. It evaluates the already verified client identity returned by
// TrustedProxyPolicy, never user-controlled forwarding headers.
func permitsRouteClient(route config.Route, client netip.Addr) bool {
	if !client.IsValid() {
		return false
	}
	client = client.Unmap()

	switch route.Exposure {
	case "public":
		return true
	case "internal":
		return client.IsLoopback()
	case "private", "restricted-public":
		for _, raw := range route.AllowedClientCIDRs {
			prefix, err := netip.ParsePrefix(raw)
			if err != nil || !prefix.IsValid() || prefix.Bits() == 0 {
				continue // fail closed when a caller bypasses config validation
			}
			if prefix.Contains(client) {
				return true
			}
		}
	}
	return false // unknown or omitted exposure has no implicit public access
}

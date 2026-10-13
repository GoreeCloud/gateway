package config

import (
	"fmt"
	"net/netip"
)

// validateRouteExposure keeps the route's claimed audience consistent with
// an explicit network policy. An unclassified enabled route must not be served.
func validateRouteExposure(route Route) error {
	switch route.Exposure {
	case "internal", "public":
		if len(route.AllowedClientCIDRs) != 0 {
			return fmt.Errorf("%s routes cannot declare network allowlists", route.Exposure)
		}
	case "private", "restricted-public":
		if len(route.AllowedClientCIDRs) == 0 {
			return fmt.Errorf("%s routes require approved client CIDRs", route.Exposure)
		}
	default:
		return fmt.Errorf("unsupported or missing exposure classification %q", route.Exposure)
	}

	if (route.Exposure == "public" || route.Exposure == "restricted-public") && route.TLS.Mode != "required" {
		return fmt.Errorf("%s routes require TLS", route.Exposure)
	}
	seen := make(map[netip.Prefix]struct{}, len(route.AllowedClientCIDRs))
	for _, raw := range route.AllowedClientCIDRs {
		prefix, err := netip.ParsePrefix(raw)
		if err != nil || !prefix.IsValid() || prefix.Bits() == 0 {
			return fmt.Errorf("invalid or unrestricted client CIDR %q", raw)
		}
		if prefix.Addr().Zone() != "" || prefix.Addr().Is4In6() {
			return fmt.Errorf("client CIDR %q must use an unzoned, canonical address family", raw)
		}
		prefix = prefix.Masked()
		if _, exists := seen[prefix]; exists {
			return fmt.Errorf("duplicate client CIDR %q", raw)
		}
		seen[prefix] = struct{}{}
	}
	return nil
}

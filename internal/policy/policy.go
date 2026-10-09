// Package policy is the one decision on whether ConnectDoctor may connect to
// an address (.claude/rules/security.md § 1, docs/DESIGN.md § 4).
//
// ⚠ There is exactly one implementation. Every other package calls Allowed;
// none keeps its own list, and tests call Allowed rather than re-listing ranges.
//
// ⚠ It fails closed. An address is allowed only when it is global unicast and
// outside every range the IANA IPv4 and IPv6 Special-Purpose Address
// Registries mark "Globally Reachable: False" (RFC 6890 § 2.2.2). For IPv6,
// only 2000::/3 (IANA "Global Unicast") is considered at all, so a range the
// registry adds later outside it is refused without a change here.
package policy

import "net/netip"

// denied lists the special-purpose ranges that net/netip's predicates do not
// already exclude. Grounds per line; "registry" means the IANA special-purpose
// registry for that family.
var denied = mustPrefixes(
	// IPv4
	"0.0.0.0/8",          // "this network", RFC 791
	"100.64.0.0/10",      // shared address space (CGNAT), RFC 6598
	"192.0.0.0/24",       // IETF protocol assignments, RFC 6890
	"192.0.2.0/24",       // TEST-NET-1, RFC 5737
	"192.88.99.0/24",     // 6to4 relay anycast, deprecated by RFC 7526
	"198.18.0.0/15",      // benchmarking, RFC 2544
	"198.51.100.0/24",    // TEST-NET-2, RFC 5737
	"203.0.113.0/24",     // TEST-NET-3, RFC 5737
	"240.0.0.0/4",        // reserved, RFC 1112
	"255.255.255.255/32", // limited broadcast, RFC 919
	// IPv6, inside 2000::/3
	"2001::/23",     // IETF protocol assignments incl. Teredo 2001::/32, RFC 2928, RFC 4380
	"2001:db8::/32", // documentation, RFC 3849
	"2002::/16",     // 6to4, embeds an IPv4 address, RFC 3056
	"3fff::/20",     // documentation, RFC 9637
)

var globalUnicast6 = netip.MustParsePrefix("2000::/3")

// Allowed reports whether ConnectDoctor may connect to a.
func Allowed(a netip.Addr) bool {
	if !a.IsValid() || a.Zone() != "" {
		return false
	}
	// ⚠ ::ffff:127.0.0.1 is loopback. Decide on the IPv4 address.
	a = a.Unmap()
	if !a.IsGlobalUnicast() || a.IsPrivate() {
		// Excludes unspecified, loopback, multicast, link-local (incl.
		// 169.254.169.254 and fe80::/10), RFC 1918 and fc00::/7.
		return false
	}
	if a.Is6() && !globalUnicast6.Contains(a) {
		// Excludes ::/96, 64:ff9b::/96 (NAT64), 100::/64, 5f00::/16 and
		// anything else outside the global unicast block.
		return false
	}
	for _, p := range denied {
		if p.Contains(a) {
			return false
		}
	}
	return true
}

func mustPrefixes(ss ...string) []netip.Prefix {
	out := make([]netip.Prefix, len(ss))
	for i, s := range ss {
		out[i] = netip.MustParsePrefix(s)
	}
	return out
}

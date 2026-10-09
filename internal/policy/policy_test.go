package policy

import (
	"net/netip"
	"testing"
)

func TestAllowed(t *testing.T) {
	refused := []string{
		// unspecified, loopback
		"0.0.0.0", "0.1.2.3", "127.0.0.1", "127.255.255.254", "::", "::1",
		// RFC 1918, ULA
		"10.0.0.1", "172.16.0.1", "172.31.255.255", "192.168.1.1", "fd00::1", "fc00::1",
		// link-local, cloud metadata
		"169.254.169.254", "169.254.0.1", "fe80::1", "fd00:ec2::254",
		// CGNAT
		"100.64.0.1", "100.127.255.254",
		// IETF, documentation, benchmarking, reserved, broadcast, multicast
		"192.0.0.8", "192.0.2.1", "198.51.100.7", "203.0.113.9", "198.18.0.1", "198.19.255.1",
		"192.88.99.1", "240.0.0.1", "255.255.255.255", "224.0.0.1", "239.1.1.1", "ff02::1", "ff0e::1",
		// IPv4 hidden in IPv6
		"::ffff:127.0.0.1", "::ffff:10.0.0.1", "::ffff:169.254.169.254", "::127.0.0.1",
		"64:ff9b::7f00:1", "64:ff9b:1::1", "2002:7f00:1::1", "2001:0:4136:e378::1",
		// IPv6 documentation, benchmarking, discard, SRv6
		"2001:db8::1", "3fff::1", "2001:2::1", "100::1", "5f00::1",
		// zoned
		"fe80::1%eth0",
	}
	for _, s := range refused {
		if Allowed(netip.MustParseAddr(s)) {
			t.Errorf("Allowed(%s) = true, want false", s)
		}
	}

	allowed := []string{"93.184.215.14", "1.1.1.1", "8.8.8.8", "2606:4700:4700::1111", "2001:4860:4860::8888", "::ffff:93.184.215.14"}
	for _, s := range allowed {
		if !Allowed(netip.MustParseAddr(s)) {
			t.Errorf("Allowed(%s) = false, want true", s)
		}
	}

	if Allowed(netip.Addr{}) {
		t.Error("Allowed(zero Addr) = true, want false")
	}
}

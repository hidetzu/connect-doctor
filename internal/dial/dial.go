// Package dial is the only way product code opens a connection to a target
// (.claude/rules/security.md § 1, docs/adr/0003).
//
// ⚠ It takes a netip.Addr, never a host name, and its Control hook calls
// internal/policy on the address the kernel is about to connect to. One
// policy, two call sites: the DNS step decides first, this hook decides again
// at the socket, so a mistake in the first cannot become a connection.
package dial

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"syscall"

	"github.com/hidetzu/connect-doctor/internal/policy"
)

// ErrRefused is returned when the policy refuses the address at the socket.
var ErrRefused = errors.New("dial: address refused by policy")

// Control is the net.Dialer hook. It is exported so the check that it
// refuses can call it directly.
func Control(_, address string, _ syscall.RawConn) error {
	ap, err := netip.ParseAddrPort(address)
	if err != nil || !policy.Allowed(ap.Addr()) {
		return ErrRefused
	}
	return nil
}

// TCP connects to addr:port. The caller's context bounds the attempt.
func TCP(ctx context.Context, addr netip.Addr, port uint16) (net.Conn, error) {
	d := net.Dialer{Control: Control}
	return d.DialContext(ctx, "tcp", netip.AddrPortFrom(addr, port).String())
}

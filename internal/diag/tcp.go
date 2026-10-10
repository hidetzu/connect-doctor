package diag

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"strconv"
	"syscall"
	"time"

	"github.com/hidetzu/connect-doctor/internal/dial"
	"github.com/hidetzu/connect-doctor/internal/limits"
)

// Dialer opens a TCP connection to a validated address. Production uses
// dial.TCP; the fast tier passes a fake. ⚠ The fake never replaces the policy.
type Dialer func(ctx context.Context, addr netip.Addr, port uint16) (net.Conn, error)

// tcpStep tries addrs in order (the DNS step already put IPv4 first) until
// one connects. It returns the step and, when ok, the open connection.
func tcpStep(ctx context.Context, d Dialer, addrs []netip.Addr, portStr string, now func() time.Time) (Step, net.Conn) {
	port64, _ := strconv.ParseUint(portStr, 10, 16) // target.Parse allows only 80 and 443
	port := uint16(port64)

	ctx, cancel := context.WithTimeout(ctx, limits.TCPTotal)
	defer cancel()

	var attempts []Attempt
	var codes []string
	for _, a := range addrs {
		actx, acancel := context.WithTimeout(ctx, limits.TCPAttempt)
		start := now()
		conn, err := d(actx, a, port)
		acancel()
		at := Attempt{Address: netip.AddrPortFrom(a, port).String(), DurationMS: now().Sub(start).Milliseconds()}
		if err == nil {
			at.Outcome = "ok"
			attempts = append(attempts, at)
			return Step{Step: StepTCP, Status: StatusOK, Detail: &Detail{Address: at.Address, Attempts: attempts}}, conn
		}
		code, raw := classifyTCPError(err)
		at.Outcome = publicCode(code)
		at.Error = raw
		if code == "tcp.refused_address" {
			// ⚠ A refused address is never shown (.claude/rules/security.md § 5).
			at.Address = ""
		}
		attempts = append(attempts, at)
		codes = append(codes, code)
		if ctx.Err() != nil {
			break // the step's whole budget is spent
		}
	}

	s := Step{Step: StepTCP, Status: StatusFailed, Detail: &Detail{Attempts: attempts}}
	if len(codes) == 0 {
		// No address to try: the DNS step never hands over an empty list.
		s.Code = "tcp.failed"
		return s, nil
	}
	// ⚠ Owner decision (hidetzu/connect-doctor#2, 2026-10-10): the first
	// attempted address's outcome concludes. Every attempt stays in detail.
	s.Code = publicCode(codes[0])
	if s.Code == "tcp.refused_address" {
		s.Status = StatusRefused
	}
	if allNoIPv6Route(addrs[:len(codes)], codes) {
		// ⚠ Our side has no IPv6 route: our gap, worded as ours.
		s.Code = "tcp.no_route_family"
	}
	return s, nil
}

// noRoute is the internal marker for ENETUNREACH before the family rule applies.
const noRoute = "tcp.unreachable(no-route)"

func allNoIPv6Route(addrs []netip.Addr, codes []string) bool {
	if len(addrs) == 0 || len(addrs) != len(codes) {
		return false
	}
	for i, a := range addrs {
		if !a.Is6() || codes[i] != noRoute {
			return false
		}
	}
	return true
}

// classifyTCPError maps a dial error to one outcome (.claude/rules/go.md).
// It returns the code and, for an unrecognised error, the raw text.
func classifyTCPError(err error) (code, raw string) {
	var ne net.Error
	switch {
	case errors.Is(err, dial.ErrRefused):
		return "tcp.refused_address", ""
	case errors.Is(err, syscall.ECONNREFUSED):
		return "tcp.refused", ""
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &ne) && ne.Timeout():
		return "tcp.timeout", ""
	case errors.Is(err, syscall.ENETUNREACH):
		return noRoute, ""
	case errors.Is(err, syscall.EHOSTUNREACH):
		return "tcp.unreachable", ""
	}
	return "tcp.failed", err.Error()
}

// publicCode turns the internal no-route marker back into the public code.
func publicCode(code string) string {
	if code == noRoute {
		return "tcp.unreachable"
	}
	return code
}

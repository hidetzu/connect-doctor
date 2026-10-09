package diag

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"slices"

	"github.com/hidetzu/connect-doctor/internal/limits"
	"github.com/hidetzu/connect-doctor/internal/policy"
)

// Resolver is the part of *net.Resolver the DNS step uses, so the fast tier
// can run the step against a fake. ⚠ The fake never replaces the policy.
type Resolver interface {
	LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error)
}

// dnsStep resolves host and applies the policy to every address.
// It returns the step and, when it is ok, the addresses in dialling order.
func dnsStep(ctx context.Context, r Resolver, host string) (Step, []netip.Addr) {
	ctx, cancel := context.WithTimeout(ctx, limits.DNS)
	defer cancel()

	// ⚠ Absolute name: without the trailing dot the resolver also tries our
	// own search domains (docs/DESIGN.md § 2, DNS).
	addrs, err := r.LookupNetIP(ctx, "ip", host+".")
	if err != nil {
		return classifyDNSError(err), nil
	}
	if len(addrs) == 0 {
		// The standard library reports NODATA as an error; an empty success
		// is not expected, and is worded the same as NODATA.
		return Step{Step: StepDNS, Status: StatusFailed, Code: "dns.not_found"}, nil
	}

	out := make([]netip.Addr, 0, len(addrs))
	for _, a := range addrs {
		a = a.Unmap()
		// ⚠ Any refused address refuses the whole name, and none of the
		// addresses is shown (docs/adr/0004).
		if !policy.Allowed(a) {
			return Step{Step: StepDNS, Status: StatusRefused, Code: "dns.refused_address"}, nil
		}
		if !slices.Contains(out, a) {
			out = append(out, a)
		}
	}
	// IPv4 first (docs/DESIGN.md § 2, TCP). Stable, so the resolver's order
	// is kept within each family.
	slices.SortStableFunc(out, func(a, b netip.Addr) int {
		switch {
		case a.Is4() && !b.Is4():
			return -1
		case !a.Is4() && b.Is4():
			return 1
		}
		return 0
	})
	shown := make([]string, len(out))
	for i, a := range out {
		shown[i] = a.String()
	}
	return Step{Step: StepDNS, Status: StatusOK, Detail: &Detail{Addresses: shown}}, out
}

// classifyDNSError maps a resolver error to one outcome (.claude/rules/go.md).
func classifyDNSError(err error) Step {
	s := Step{Step: StepDNS, Status: StatusFailed}
	var de *net.DNSError
	switch {
	case errors.As(err, &de) && de.IsNotFound:
		// ⚠ NXDOMAIN and NODATA are indistinguishable here (measured,
		// docs/DESIGN.md § 2).
		s.Code = "dns.not_found"
	case errors.As(err, &de) && de.IsTimeout, errors.Is(err, context.DeadlineExceeded):
		s.Code = "dns.timeout"
	case errors.As(err, &de):
		s.Code = "dns.server_failure"
		// ⚠ de.Err only: de.Server names our own resolver.
		s.Detail = &Detail{Error: de.Err}
	default:
		s.Code = "dns.server_failure"
		s.Detail = &Detail{Error: err.Error()}
	}
	return s
}

// Package diag runs the steps and builds the one Result that the page and
// the API both render (docs/DESIGN.md § 5, § 6).
package diag

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"net/url"
	"time"

	"github.com/hidetzu/connect-doctor/internal/limits"
	"github.com/hidetzu/connect-doctor/internal/target"
)

// Checker runs checks.
type Checker struct {
	Resolver Resolver
	// Dial is required. ⚠ There is no default: a Checker built without one
	// panics rather than quietly reaching the real network from a test.
	Dial Dialer
	Now  func() time.Time
}

// Check diagnoses raw. It never returns an error: every outcome, including
// refusing the input, is a Result.
func (c *Checker) Check(ctx context.Context, raw string) Result {
	now := time.Now
	if c.Now != nil {
		now = c.Now
	}
	start := now()
	ctx, cancel := context.WithTimeout(ctx, limits.Check)
	defer cancel()

	res := Result{ObservedFrom: "server", ObservedFromNote: ObservedFromNote, CheckedAt: start.UTC()}

	t, err := target.Parse(raw)
	if err != nil {
		var r *target.Refusal
		code := target.CodeMalformed
		if errors.As(err, &r) {
			code = r.Code
		}
		// ⚠ Nothing was attempted: every step is skipped, and the input is
		// not echoed (it may carry credentials).
		res.Hops = []Hop{{Steps: skippedFrom(0)}}
		res.Conclusion = Conclusion{Status: ConclusionRefused, Code: code, Summary: summaryRefusedInput(code)}
		res.DurationMS = now().Sub(start).Milliseconds()
		return res
	}
	res.URL = t.URL

	// ⚠ Every hop is a new URL that goes through target, DNS, the policy,
	// TCP, TLS and HTTP from the start; net/http's redirect follower is never
	// used (docs/adr/0003, .claude/rules/security.md § 3). One context bounds
	// all of them (limits.Check).
	for {
		hop, next := c.runHop(ctx, t, now)
		res.Hops = append(res.Hops, hop)
		if next == "" {
			res.Conclusion = conclude(res.Hops)
			break
		}
		if len(res.Hops) > limits.RedirectHops {
			res.Conclusion = Conclusion{Status: ConclusionFailed, Code: "http.too_many_redirects", Summary: summaryTooManyRedirects(limits.RedirectHops)}
			break
		}
		nt, err := target.Parse(next)
		if err != nil {
			var r *target.Refusal
			code := target.CodeMalformed
			if errors.As(err, &r) {
				code = r.Code
			}
			refused := Hop{Code: code, Steps: skippedFrom(0)}
			if code != target.CodeCredentials && code != target.CodeMalformed {
				// ⚠ Never echo credentials, and never a string that did not parse.
				refused.URL = next
			}
			res.Hops = append(res.Hops, refused)
			res.Conclusion = conclude(res.Hops)
			break
		}
		t = nt
	}
	res.DurationMS = now().Sub(start).Milliseconds()
	return res
}

// followed are the statuses whose Location is followed (RFC 9110 § 15.4).
// 300 and 304 are answers, not redirects to follow.
var followed = map[int]bool{301: true, 302: true, 303: true, 307: true, 308: true}

// runHop runs every step for one URL. It returns the hop and, when the
// response is a redirect to follow, the next URL resolved against this one.
func (c *Checker) runHop(ctx context.Context, t target.Target, now func() time.Time) (Hop, string) {
	hop := Hop{URL: t.URL}
	var dns Step
	var addrs []netip.Addr
	if t.IsLiteral() {
		// target.Parse already applied the policy to a literal.
		dns = Step{Step: StepDNS, Status: StatusNotApplicable, Detail: &Detail{Addresses: []string{t.Addr.String()}}}
		addrs = []netip.Addr{t.Addr}
	} else {
		s := now()
		dns, addrs = dnsStep(ctx, c.Resolver, t.Host)
		ms := now().Sub(s).Milliseconds()
		dns.DurationMS = &ms
	}
	dns.Message = Message(dns.Code)
	hop.Steps = append(hop.Steps, dns)

	tls := Step{Step: StepTLS, Status: StatusSkipped}
	if t.Scheme == "http" {
		tls.Status = StatusNotApplicable
	}
	http := Step{Step: StepHTTP, Status: StatusSkipped}

	if dns.Status == StatusOK || dns.Status == StatusNotApplicable {
		if c.Dial == nil {
			panic("diag: Checker.Dial is nil")
		}
		s := now()
		tcp, conn := tcpStep(ctx, c.Dial, addrs, t.Port, now)
		ms := now().Sub(s).Milliseconds()
		tcp.DurationMS = &ms
		tcp.Message = Message(tcp.Code)
		hop.Steps = append(hop.Steps, tcp)
		if tcp.Status == StatusOK {
			if t.Scheme == "https" {
				s := now()
				var tc net.Conn
				tls, tc = tlsHandshake(ctx, conn, t.Host)
				ms := now().Sub(s).Milliseconds()
				tls.DurationMS = &ms
				tls.Message = Message(tls.Code)
				if tc != nil {
					conn = tc
				}
			}
			if t.Scheme == "http" || tls.Status == StatusOK {
				s := now()
				http = httpStep(ctx, conn, t.URL)
				ms := now().Sub(s).Milliseconds()
				http.DurationMS = &ms
				http.Message = Message(http.Code)
			}
		}
		if conn != nil {
			conn.Close()
		}
	} else {
		hop.Steps = append(hop.Steps, Step{Step: StepTCP, Status: StatusSkipped})
	}
	hop.Steps = append(hop.Steps, tls, http)

	if http.Status != StatusOK || !followed[http.Detail.StatusCode] {
		return hop, ""
	}
	loc := http.Detail.Headers["Location"]
	if loc == "" {
		return hop, ""
	}
	base, err := url.Parse(t.URL)
	ref, err2 := url.Parse(loc)
	if err != nil || err2 != nil {
		// target.Parse refuses it as malformed on the next turn.
		return hop, loc
	}
	return hop, base.ResolveReference(ref).String()
}

func skippedFrom(i int) []Step {
	var out []Step
	for _, s := range Steps[i:] {
		out = append(out, Step{Step: s, Status: StatusSkipped})
	}
	return out
}

// conclude finds the first failed or refused step across all hops.
func conclude(hops []Hop) Conclusion {
	lastOK := ""
	var gaps []string
	for i, h := range hops {
		if h.Code != "" {
			// A redirect target refused before any step ran (only hops ≥ 2).
			return Conclusion{Status: ConclusionRefused, Code: "http.redirect_refused", Summary: summaryRedirectRefused(i+1, h.Code)}
		}
		for _, s := range h.Steps {
			switch s.Status {
			case StatusFailed:
				return Conclusion{Status: ConclusionFailed, FailedStep: s.Step, Code: s.Code, Summary: summaryFailed(s.Step, s.Code) + summaryAfterRedirects(i)}
			case StatusRefused:
				if i > 0 {
					// ⚠ Owner decision (hidetzu/connect-doctor#5): a refused
					// redirect target is http.redirect_refused, naming the hop.
					return Conclusion{Status: ConclusionRefused, Code: "http.redirect_refused", Summary: summaryRedirectRefused(i+1, s.Code)}
				}
				return Conclusion{Status: ConclusionRefused, FailedStep: s.Step, Code: s.Code, Summary: summaryRefusedStep(s.Code)}
			case StatusOK:
				lastOK = s.Step
			case StatusNotImplemented:
				gaps = append(gaps, s.Step)
			}
		}
	}
	if len(gaps) > 0 {
		// ⚠ Never ok while a layer was not checked (docs/DESIGN.md § 6).
		return Conclusion{Status: ConclusionIncomplete, Summary: summaryIncomplete(lastOK, gaps)}
	}
	return Conclusion{Status: ConclusionOK, Summary: summaryOK(statusCode(hops), httpsHop(hops)) + summaryAfterRedirects(len(hops)-1)}
}

// statusCode is the HTTP status of the last hop.
func statusCode(hops []Hop) int {
	last := hops[len(hops)-1]
	for _, s := range last.Steps {
		if s.Step == StepHTTP && s.Detail != nil {
			return s.Detail.StatusCode
		}
	}
	return 0
}

func httpsHop(hops []Hop) bool {
	for _, s := range hops[len(hops)-1].Steps {
		if s.Step == StepTLS {
			return s.Status != StatusNotApplicable
		}
	}
	return false
}

// tlsHandshake adapts tlsStep's *tls.Conn to net.Conn without a typed nil.
func tlsHandshake(ctx context.Context, conn net.Conn, host string) (Step, net.Conn) {
	st, tc := tlsStep(ctx, conn, host)
	if tc == nil {
		return st, nil
	}
	return st, tc
}

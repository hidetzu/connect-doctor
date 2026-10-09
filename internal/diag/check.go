// Package diag runs the steps and builds the one Result that the page and
// the API both render (docs/DESIGN.md § 5, § 6).
package diag

import (
	"context"
	"errors"
	"time"

	"github.com/hidetzu/connect-doctor/internal/limits"
	"github.com/hidetzu/connect-doctor/internal/target"
)

// Checker runs checks.
type Checker struct {
	Resolver Resolver
	Now      func() time.Time
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

	hop := Hop{URL: t.URL}
	var dns Step
	if t.IsLiteral() {
		// target.Parse already applied the policy to a literal.
		dns = Step{Step: StepDNS, Status: StatusNotApplicable, Detail: &Detail{Addresses: []string{t.Addr.String()}}}
	} else {
		s := now()
		dns, _ = dnsStep(ctx, c.Resolver, t.Host)
		ms := now().Sub(s).Milliseconds()
		dns.DurationMS = &ms
	}
	dns.Message = Message(dns.Code)
	hop.Steps = append(hop.Steps, dns)

	if dns.Status == StatusOK || dns.Status == StatusNotApplicable {
		hop.Steps = append(hop.Steps, notImplemented(StepTCP))
		if t.Scheme == "https" {
			hop.Steps = append(hop.Steps, notImplemented(StepTLS))
		} else {
			hop.Steps = append(hop.Steps, Step{Step: StepTLS, Status: StatusNotApplicable})
		}
		hop.Steps = append(hop.Steps, notImplemented(StepHTTP))
	} else {
		hop.Steps = append(hop.Steps, skippedFrom(1)...)
	}
	res.Hops = []Hop{hop}
	res.Conclusion = conclude(res.Hops)
	res.DurationMS = now().Sub(start).Milliseconds()
	return res
}

func notImplemented(step string) Step {
	return Step{Step: step, Status: StatusNotImplemented, Message: notImplementedMessage}
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
	for _, h := range hops {
		for _, s := range h.Steps {
			switch s.Status {
			case StatusFailed:
				return Conclusion{Status: ConclusionFailed, FailedStep: s.Step, Code: s.Code, Summary: summaryFailed(s.Step, s.Code)}
			case StatusRefused:
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
	// Unreachable until every step is implemented; its sentence arrives with
	// the HTTP step (hidetzu/connect-doctor#4).
	return Conclusion{Status: ConclusionOK}
}

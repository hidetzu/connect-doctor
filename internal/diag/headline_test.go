package diag

import (
	"strings"
	"testing"
)

// hidetzu/connect-doctor#25: every conclusion has a headline, a cause and a
// state; refusals are neutral, never red or yellow.
func TestHeadlineCauseState(t *testing.T) {
	httpStep := func(code int) Step { return Step{Step: StepHTTP, Status: StatusOK, Detail: &Detail{StatusCode: code}} }
	cases := []struct {
		name  string
		r     Result
		state string
		head  string
	}{
		{"ok", Result{Conclusion: Conclusion{Status: ConclusionOK}, Hops: []Hop{{Steps: []Step{httpStep(200)}}}}, "ok", "すべての層を通りました（200）"},
		{"503", Result{Conclusion: Conclusion{Status: ConclusionOK}, Hops: []Hop{{Steps: []Step{httpStep(503)}}}}, "warn", "サーバは 503 を返しました"},
		{"tls", Result{Conclusion: Conclusion{Status: ConclusionFailed, FailedStep: StepTLS, Code: "tls.cert_expired"}, Hops: []Hop{{}}}, "fail", "TLS で止まっています"},
		{"input", Result{Conclusion: Conclusion{Status: ConclusionRefused, Code: "input.local_name"}, Hops: []Hop{{}}}, "neutral", "このURLは診断しませんでした"},
		{"dns refused", Result{Conclusion: Conclusion{Status: ConclusionRefused, FailedStep: StepDNS, Code: "dns.refused_address"}, Hops: []Hop{{}}}, "neutral", "DNS で止めました"},
		{"redirect refused", Result{Conclusion: Conclusion{Status: ConclusionRefused, Code: "http.redirect_refused"}, Hops: []Hop{{}, {Code: "input.refused_address"}}}, "neutral", "リダイレクト先で止めました"},
		{"target", Result{Conclusion: Conclusion{Status: ConclusionRefused, Code: CodeTargetLimited}, Hops: []Hop{{}}}, "neutral", "接続を控えました"},
		{"loop", Result{Conclusion: Conclusion{Status: ConclusionFailed, Code: "http.too_many_redirects"}, Hops: []Hop{{}}}, "fail", "リダイレクトが終わりません"},
	}
	for _, c := range cases {
		if got := State(c.r); got != c.state {
			t.Errorf("%s: state %s, want %s", c.name, got, c.state)
		}
		if got := Headline(c.r); !strings.Contains(got, c.head) {
			t.Errorf("%s: headline %q, want it to contain %q", c.name, got, c.head)
		}
		if Cause(c.r) == "" {
			t.Errorf("%s: no cause", c.name)
		}
	}
	// A refused redirect explains the refused hop's own reason.
	r := cases[5].r
	if Cause(r) != Message("input.refused_address") {
		t.Errorf("redirect refused cause = %q", Cause(r))
	}
}

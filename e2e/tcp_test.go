//go:build e2e

package e2e

import (
	"strings"
	"testing"
	"time"

	"github.com/hidetzu/connect-doctor/internal/diag"
	"github.com/hidetzu/connect-doctor/internal/limits"
)

// hidetzu/connect-doctor#2, AC 2, 3, 5.
func TestTCPOutcomesThroughTheBinary(t *testing.T) {
	in := start(t)
	cases := []struct {
		url, ladder, code string
	}{
		{"https://ok.test/", "dns=ok tcp=ok tls=ok http=ok", ""},
		{"http://ok.test/", "dns=ok tcp=ok tls=not_applicable http=ok", ""},
		{"https://closed.test/", "dns=ok tcp=failed tls=skipped http=skipped", "tcp.refused"},
		{"http://closed.test/", "dns=ok tcp=failed tls=not_applicable http=skipped", "tcp.refused"},
		{"https://unreach.test/", "dns=ok tcp=failed tls=skipped http=skipped", "tcp.unreachable"},
		{"https://v6only.test/", "dns=ok tcp=failed tls=skipped http=skipped", "tcp.no_route_family"},
		{"https://" + addrClosed + "/", "dns=not_applicable tcp=failed tls=skipped http=skipped", "tcp.refused"},
	}
	for _, c := range cases {
		_, res, _ := in.api(t, c.url)
		if got := ladder(res); got != c.ladder {
			t.Errorf("%s: ladder %s, want %s", c.url, got, c.ladder)
		}
		if c.code != "" && (res.Conclusion.Code != c.code || res.Conclusion.FailedStep != "tcp") {
			t.Errorf("%s: conclusion %+v, want tcp/%s", c.url, res.Conclusion, c.code)
		}
		if c.code == "" && res.Conclusion.Status != diag.ConclusionOK {
			t.Errorf("%s: conclusion %+v, want ok", c.url, res.Conclusion)
		}
	}
}

func TestTCPTimeoutIsBounded(t *testing.T) {
	in := start(t)
	begin := time.Now()
	_, res, _ := in.api(t, "https://drop.test/")
	took := time.Since(begin)
	if res.Conclusion.Code != "tcp.timeout" {
		t.Errorf("drop.test: %+v, want tcp.timeout", res.Conclusion)
	}
	// ⚠ Bounded by limits.TCPAttempt; slack for scheduling, never the whole-check ceiling.
	if took < limits.TCPAttempt-time.Second || took > limits.TCPAttempt+3*time.Second {
		t.Errorf("drop.test took %s, want about %s", took, limits.TCPAttempt)
	}
}

// The first address is silent, the second answers: ok, and the detail says
// what happened to each.
func TestTCPFallsBackToTheNextAddress(t *testing.T) {
	in := start(t)
	_, res, body := in.api(t, "https://fallback.test/")
	tcp := res.Hops[0].Steps[1]
	if tcp.Status != diag.StatusOK || tcp.Detail.Address != addrListen+":443" {
		t.Fatalf("fallback.test: %+v", tcp)
	}
	a := tcp.Detail.Attempts
	if len(a) != 2 || a[0].Address != addrDrop+":443" || a[0].Outcome != "tcp.timeout" || a[1].Outcome != "ok" {
		t.Errorf("attempts = %+v", a)
	}
	if !strings.Contains(body, `"attempts"`) {
		t.Error("attempts missing from JSON")
	}
}

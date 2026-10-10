package diag

import (
	"context"
	"encoding/json"
	"net/netip"
	"strings"
	"testing"
	"time"
)

// fakeGate refuses the hostnames and destinations it is told to.
type fakeGate struct {
	hosts map[string]bool
	dests map[string]bool
}

func (g fakeGate) AllowHost(h string) (bool, time.Duration) {
	if g.hosts[h] {
		return false, 7 * time.Second
	}
	return true, 0
}

func (g fakeGate) AllowDial(d netip.AddrPort) (bool, time.Duration) {
	if g.dests[d.String()] {
		return false, 3 * time.Second
	}
	return true, 0
}

func TestHostLimitStopsBeforeAnythingLeaves(t *testing.T) {
	r := &fakeResolver{addrs: []string{"93.184.215.14"}}
	d := &scriptedDial{}
	res := (&Checker{Resolver: r, Dial: d.dial, Targets: fakeGate{hosts: map[string]bool{"example.com": true}}}).Check(context.Background(), "https://example.com/")
	if res.Conclusion.Status != ConclusionRefused || res.Conclusion.Code != CodeTargetLimited || res.Conclusion.FailedStep != "" {
		t.Errorf("conclusion = %+v", res.Conclusion)
	}
	if res.Hops[0].Code != CodeTargetLimited || len(r.asked) != 0 || len(d.tried) != 0 {
		t.Errorf("hop %+v, resolved %v, dialled %v — want nothing resolved or dialled", res.Hops[0].Code, r.asked, d.tried)
	}
	if res.RetryAfter != 7*time.Second {
		t.Errorf("RetryAfter = %s", res.RetryAfter)
	}
	if b, _ := json.Marshal(res); strings.Contains(string(b), "retry") {
		t.Errorf("RetryAfter leaked into the JSON: %s", b)
	}
}

func TestDestinationLimitIsPerAttempt(t *testing.T) {
	v4, v4b := "93.184.215.14", "93.184.215.15"
	r := &fakeResolver{addrs: []string{v4, v4b}}
	d := &scriptedDial{}
	gate := fakeGate{dests: map[string]bool{v4 + ":443": true}}
	res := (&Checker{Resolver: r, Dial: d.dial, Targets: gate}).Check(context.Background(), "https://example.com/")
	tcp := res.Hops[0].Steps[1]
	if tcp.Status != StatusOK || len(tcp.Detail.Attempts) != 2 || tcp.Detail.Attempts[0].Outcome != CodeTargetLimited {
		t.Fatalf("tcp = %+v %+v", tcp, tcp.Detail)
	}
	if strings.Join(d.tried, ",") != v4b {
		t.Errorf("dialled %v, want only the address under budget", d.tried)
	}

	gate.dests[v4b+":443"] = true
	d2 := &scriptedDial{}
	res = (&Checker{Resolver: r, Dial: d2.dial, Targets: gate}).Check(context.Background(), "https://example.com/")
	tcp = res.Hops[0].Steps[1]
	if tcp.Status != StatusRefused || tcp.Code != CodeTargetLimited || res.Conclusion.Code != CodeTargetLimited || len(d2.tried) != 0 {
		t.Errorf("all refused: tcp %+v, conclusion %+v, dialled %v", tcp, res.Conclusion, d2.tried)
	}
}

func TestRedirectIntoALimitedHostname(t *testing.T) {
	r := &mapResolver{names: map[string][]string{"a.example": {"93.184.215.1"}, "b.example": {"93.184.215.2"}}}
	s := &sites{responses: map[string]string{"93.184.215.1": redirectTo("302 Found", "https://b.example/"), "93.184.215.2": ok200}}
	res := (&Checker{Resolver: r, Dial: s.dial, Targets: fakeGate{hosts: map[string]bool{"b.example": true}}}).Check(context.Background(), "https://a.example/")
	if len(res.Hops) != 2 || res.Hops[1].Code != CodeTargetLimited || res.Conclusion.Code != CodeTargetLimited || !strings.Contains(res.Conclusion.Summary, "2番目") {
		t.Errorf("hops %d, conclusion %+v", len(res.Hops), res.Conclusion)
	}
	if len(s.dialled) != 1 {
		t.Errorf("dialled %v, want only the first hop", s.dialled)
	}
}

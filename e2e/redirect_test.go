//go:build e2e

package e2e

import (
	"strings"
	"testing"
	"time"

	"github.com/hidetzu/connect-doctor/internal/diag"
	"github.com/hidetzu/connect-doctor/internal/limits"
)

// hidetzu/connect-doctor#5 AC 1 and 2, as a pair: the permitted redirect
// reaches its listener, the refused one reaches nothing.
func TestRedirectFollowedAndRefused(t *testing.T) {
	in := start(t)

	before := publicAccepts.Load()
	_, res, _ := in.api(t, "https://redir.test/")
	if len(res.Hops) != 2 || res.Hops[1].URL != "https://ok.test/landed" || res.Conclusion.Status != diag.ConclusionOK {
		t.Fatalf("redir.test: hops %d, conclusion %+v", len(res.Hops), res.Conclusion)
	}
	if ladder := hopLadder(res, 1); ladder != "dns=ok tcp=ok tls=ok http=ok" {
		t.Errorf("hop 2 ladder %s", ladder)
	}
	if publicAccepts.Load()-before < 2 {
		t.Fatal("control: the permitted chain did not reach the listener twice — the refusal below would prove nothing")
	}

	loopBefore := loopbackAccepts.Load()
	_, res, _ = in.api(t, "https://toloop.test/")
	if res.Conclusion.Code != "http.redirect_refused" || res.Hops[1].Code != "input.refused_address" || !strings.Contains(res.Conclusion.Summary, "2番目") {
		t.Errorf("toloop.test: conclusion %+v, hop 2 %+v", res.Conclusion, res.Hops[1])
	}
	time.Sleep(200 * time.Millisecond)
	if n := loopbackAccepts.Load() - loopBefore; n != 0 {
		t.Errorf("the redirect to 127.0.0.1 reached the loopback listener %d times, want 0", n)
	}
}

// AC 3: refused before resolving.
func TestRedirectRefusedBeforeResolving(t *testing.T) {
	in := start(t)
	for u, code := range map[string]string{"https://tolocal.test/": "input.local_name", "https://toport.test/": "input.unsupported_port"} {
		okBefore := in.dns.Queries("ok.test")
		_, res, _ := in.api(t, u)
		if res.Conclusion.Code != "http.redirect_refused" || len(res.Hops) != 2 || res.Hops[1].Code != code {
			t.Errorf("%s: %+v, hops %+v", u, res.Conclusion, res.Hops)
		}
		if in.dns.Queries("localhost") != 0 || in.dns.Queries("ok.test") != okBefore {
			t.Errorf("%s: the refused hop was resolved", u)
		}
	}
}

// AC 4 and 5.
func TestRedirectLoopAndRelative(t *testing.T) {
	in := start(t)
	_, res, _ := in.api(t, "https://spin.test/")
	if res.Conclusion.Code != "http.too_many_redirects" || len(res.Hops) != limits.RedirectHops+1 {
		t.Errorf("spin.test: %+v, %d hops", res.Conclusion, len(res.Hops))
	}
	_, res, _ = in.api(t, "https://rel.test/")
	if len(res.Hops) != 2 || res.Hops[1].URL != "https://rel.test/after" || res.Conclusion.Status != diag.ConclusionOK {
		t.Errorf("rel.test: %+v, hops %+v", res.Conclusion, res.Hops)
	}
}

// AC 6: a chain of slow hops is cut by the whole-check ceiling.
func TestRedirectChainRespectsTheCeiling(t *testing.T) {
	in := start(t)
	begin := time.Now()
	_, res, _ := in.api(t, "https://slowhop.test/")
	took := time.Since(begin)
	if took > limits.Check+2*time.Second {
		t.Errorf("the check took %s, over the %s ceiling", took, limits.Check)
	}
	if res.Conclusion.Status != diag.ConclusionFailed || !strings.HasSuffix(res.Conclusion.Code, ".timeout") {
		t.Errorf("slowhop.test: %+v after %d hops", res.Conclusion, len(res.Hops))
	}
}

// One ladder per hop on the page (owner decision on #5).
func TestPageShowsEveryHop(t *testing.T) {
	in := start(t)
	page := in.page(t, "/?url=https://redir.test/")
	if strings.Count(page, `<table class="ladder"`) != 2 || !strings.Contains(page, "1. <code>https://redir.test/</code>") || !strings.Contains(page, "2. <code>https://ok.test/landed</code>") {
		t.Error("the page does not show one labelled ladder per hop")
	}
}

func hopLadder(res diag.Result, i int) string {
	var s []string
	for _, st := range res.Hops[i].Steps {
		s = append(s, st.Step+"="+string(st.Status))
	}
	return strings.Join(s, " ")
}

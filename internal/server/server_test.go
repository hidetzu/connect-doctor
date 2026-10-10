package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/hidetzu/connect-doctor/internal/diag"
	"github.com/hidetzu/connect-doctor/internal/limits"
	"github.com/hidetzu/connect-doctor/internal/tlstest"
)

type staticResolver struct{ addrs []netip.Addr }

func (r staticResolver) LookupNetIP(context.Context, string, string) ([]netip.Addr, error) {
	return r.addrs, nil
}

// blockingResolver holds every lookup until release is closed.
type blockingResolver struct {
	entered chan struct{}
	release chan struct{}
}

func (r blockingResolver) LookupNetIP(ctx context.Context, _, _ string) ([]netip.Addr, error) {
	r.entered <- struct{}{}
	<-r.release
	return []netip.Addr{netip.MustParseAddr("93.184.215.14")}, nil
}

func get(t *testing.T, h http.Handler, target string) (*http.Response, string) {
	return getFrom(t, h, target, "")
}

// getFrom sends the request with an X-Forwarded-For header (when xff is set).
func getFrom(t *testing.T, h http.Handler, target, xff string) (*http.Response, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	if xff != "" {
		req.Header.Set("X-Forwarded-For", xff)
	}
	h.ServeHTTP(rec, req)
	b, _ := io.ReadAll(rec.Result().Body)
	return rec.Result(), string(b)
}

func pipeDial(context.Context, netip.Addr, uint16) (net.Conn, error) {
	return tlstest.Serve(nil, tlstest.OK200), nil
}

func newTestServer(r diag.Resolver, logs io.Writer, slots int) *Server {
	return newWithSlots(&diag.Checker{Resolver: r, Dial: pipeDial}, log.New(logs, "", 0), slots)
}

func TestAPIAndPageRenderTheSameResult(t *testing.T) {
	s := newTestServer(staticResolver{[]netip.Addr{netip.MustParseAddr("93.184.215.14")}}, io.Discard, 4)
	h := s.Handler()

	resp, body := get(t, h, "/api/check?url="+url.QueryEscape("http://ok.test/"))
	if resp.StatusCode != 200 || !strings.HasPrefix(resp.Header.Get("Content-Type"), "application/json") {
		t.Fatalf("api: %d %s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	var res diag.Result
	if err := json.Unmarshal([]byte(body), &res); err != nil {
		t.Fatal(err)
	}
	if res.Conclusion.Status != diag.ConclusionOK || res.ObservedFrom != "server" {
		t.Fatalf("api result: %+v", res)
	}

	resp, page := get(t, h, "/?url="+url.QueryEscape("http://ok.test/"))
	if resp.StatusCode != 200 {
		t.Fatalf("page: %d", resp.StatusCode)
	}
	for _, want := range []string{
		diag.Headline(res), diag.Cause(res), // the page shows the API's result, in its own words
		`class="verdict ok"`,
		"あなたのPCからの接続結果ではありません", // the vantage, on every result (docs/adr/0001)
		"93.184.215.14",
		`<ol class="rail"`, "✓",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("page does not contain %q", want)
		}
	}
}

func TestPageWithoutURLShowsFormAndVantage(t *testing.T) {
	_, page := get(t, newTestServer(staticResolver{}, io.Discard, 1).Handler(), "/")
	for _, want := range []string{`<form method="get" action="/">`, "あなたのPCからの接続結果ではありません", `<div class="strip"`} {
		if !strings.Contains(page, want) {
			t.Errorf("page does not contain %q", want)
		}
	}
	if strings.Contains(page, "結論") {
		t.Error("page without a URL shows a conclusion")
	}
}

func TestAPIInputRefusalIs400(t *testing.T) {
	h := newTestServer(staticResolver{}, io.Discard, 1).Handler()
	for _, u := range []string{"", "http://127.0.0.1/", "ftp://x.test/"} {
		resp, body := get(t, h, "/api/check?url="+url.QueryEscape(u))
		if resp.StatusCode != 400 || !strings.Contains(body, `"status": "refused"`) {
			t.Errorf("%q: %d %s", u, resp.StatusCode, body)
		}
	}
}

// AC 8: the bound is answered as busy, not as a DNS failure.
func TestBusyIsNotATargetFailure(t *testing.T) {
	r := blockingResolver{entered: make(chan struct{}), release: make(chan struct{})}
	s := newTestServer(r, io.Discard, 1)
	h := s.Handler()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		get(t, h, "/api/check?url=http://ok.test/")
	}()
	<-r.entered // the only slot is now held

	resp, body := get(t, h, "/api/check?url=http://ok.test/")
	if resp.StatusCode != http.StatusServiceUnavailable || !strings.Contains(body, `"server.busy"`) {
		t.Errorf("api over the bound: %d %s", resp.StatusCode, body)
	}
	if strings.Contains(body, "dns.") {
		t.Errorf("busy answered as a DNS outcome: %s", body)
	}
	resp, page := get(t, h, "/?url=http://ok.test/")
	if resp.StatusCode != http.StatusServiceUnavailable || !strings.Contains(page, diag.Message("server.busy")) {
		t.Errorf("page over the bound: %d", resp.StatusCode)
	}
	if s.Busy() != 2 {
		t.Errorf("busy counted %d, want 2", s.Busy())
	}
	close(r.release)
	wg.Wait()
}

// AC 9: the URL being checked, and so its query string, never reaches a log.
func TestLogsCarryNoQueryString(t *testing.T) {
	var logs bytes.Buffer
	h := newTestServer(staticResolver{[]netip.Addr{netip.MustParseAddr("93.184.215.14")}}, &logs, 1).Handler()
	secret := "SECRET123"
	get(t, h, "/api/check?url="+url.QueryEscape("http://x.test/?token="+secret))
	get(t, h, "/?url="+url.QueryEscape("http://x.test/?token="+secret))
	if strings.Contains(logs.String(), secret) || strings.Contains(logs.String(), "x.test") {
		t.Errorf("log leaks the checked URL:\n%s", logs.String())
	}
	// ⚠ Control: the log did receive the requests.
	if n := strings.Count(logs.String(), "\n"); n != 2 {
		t.Errorf("log has %d lines, want 2:\n%s", n, logs.String())
	}
}

// hidetzu/connect-doctor#26 AC 3: the production bound, not a test-sized one.
func TestProductionConcurrencyBound(t *testing.T) {
	r := blockingResolver{entered: make(chan struct{}), release: make(chan struct{})}
	s := New(&diag.Checker{Resolver: r, Dial: pipeDial}, log.New(io.Discard, "", 0), Options{TrustXFF: true})
	h := s.Handler()
	var wg sync.WaitGroup
	for i := 0; i < limits.ConcurrentChecks; i++ {
		wg.Add(1)
		// Distinct clients and hostnames: the per-client and per-target limits are not under test here.
		xff, u := fmt.Sprintf("198.51.100.%d", i+1), fmt.Sprintf("/api/check?url=http://ok%d.test/", i)
		go func() { defer wg.Done(); getFrom(t, h, u, xff) }()
		<-r.entered
	}
	resp, body := getFrom(t, h, "/api/check?url=http://ok-extra.test/", "198.51.100.200")
	if resp.StatusCode != http.StatusServiceUnavailable || !strings.Contains(body, "server.busy") {
		t.Errorf("check %d: %d %s, want 503 server.busy", limits.ConcurrentChecks+1, resp.StatusCode, body)
	}
	close(r.release)
	wg.Wait()
}

func limitedServer(logs io.Writer, trustXFF bool) http.Handler {
	r := staticResolver{[]netip.Addr{netip.MustParseAddr("93.184.215.14")}}
	return New(&diag.Checker{Resolver: r, Dial: pipeDial}, log.New(logs, "", 0), Options{TrustXFF: trustXFF}).Handler()
}

// hidetzu/connect-doctor#6: the burst, then 429 with Retry-After, and nothing
// about the request beyond hostname and client prefix in the log.
func TestClientLimitAnswers429(t *testing.T) {
	var logs bytes.Buffer
	h := limitedServer(&logs, true)
	// A different hostname each time: the per-target limit (hidetzu/connect-doctor#27) is not under test here.
	u := func(i int) string {
		return "/api/check?url=" + url.QueryEscape(fmt.Sprintf("http://ok%d.test/secret/path?token=SECRET123", i))
	}
	for i := 0; i < limits.ClientBurst; i++ {
		if resp, body := getFrom(t, h, u(i), "203.0.113.7"); resp.StatusCode != 200 {
			t.Fatalf("check %d: %d %s", i+1, resp.StatusCode, body)
		}
	}
	resp, body := getFrom(t, h, u(limits.ClientBurst), "203.0.113.7")
	if resp.StatusCode != http.StatusTooManyRequests || !strings.Contains(body, `"server.rate_limited"`) || resp.Header.Get("Retry-After") == "" {
		t.Fatalf("over the burst: %d %q Retry-After=%q", resp.StatusCode, body, resp.Header.Get("Retry-After"))
	}
	line := ""
	for _, l := range strings.Split(logs.String(), "\n") {
		if strings.HasPrefix(l, "limit ") {
			line = l
		}
	}
	if !strings.Contains(line, "refused=burst") || !strings.Contains(line, "target=ok3.test") || !strings.Contains(line, "client=203.0.113.0/24") {
		t.Errorf("refusal log line: %q", line)
	}
	for _, leak := range []string{"SECRET123", "secret/path", "203.0.113.7"} {
		if strings.Contains(logs.String(), leak) {
			t.Errorf("log contains %q:\n%s", leak, logs.String())
		}
	}
	// The page is limited too.
	if resp, page := getFrom(t, h, "/?url=http://ok.test/", "203.0.113.7"); resp.StatusCode != http.StatusTooManyRequests || !strings.Contains(page, diag.Message("server.rate_limited")) {
		t.Errorf("page over the limit: %d", resp.StatusCode)
	}
	// ⚠ Control: another client is unaffected, and the plain page is never limited.
	if resp, _ := getFrom(t, h, u(9), "203.0.113.8"); resp.StatusCode != 200 {
		t.Errorf("another client: %d", resp.StatusCode)
	}
	if resp, _ := getFrom(t, h, "/", "203.0.113.7"); resp.StatusCode != 200 {
		t.Errorf("the form without a URL was limited: %d", resp.StatusCode)
	}
}

// ⚠ Only the last X-Forwarded-For entry counts: forged leading entries do not
// buy a fresh budget.
func TestForgedForwardedForDoesNotResetTheLimit(t *testing.T) {
	h := limitedServer(io.Discard, true)
	for i := 0; i < limits.ClientBurst; i++ {
		getFrom(t, h, fmt.Sprintf("/api/check?url=http://f%d.test/", i), fmt.Sprintf("10.9.9.%d, 203.0.113.9", i))
	}
	if resp, _ := getFrom(t, h, "/api/check?url=http://f9.test/", "192.0.2.77, 203.0.113.9"); resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("a forged first entry reset the limit: %d", resp.StatusCode)
	}
}

// ⚠ Without -trust-xff the header is ignored entirely: the TCP peer is the client.
func TestForwardedForIgnoredUnlessTrusted(t *testing.T) {
	h := limitedServer(io.Discard, false)
	for i := 0; i < limits.ClientBurst; i++ {
		getFrom(t, h, fmt.Sprintf("/api/check?url=http://g%d.test/", i), fmt.Sprintf("203.0.113.%d", i+1))
	}
	if resp, _ := getFrom(t, h, "/api/check?url=http://g9.test/", "203.0.113.99"); resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("an untrusted X-Forwarded-For changed the client: %d", resp.StatusCode)
	}
}

// hidetzu/connect-doctor#27: a production server always carries the target
// limiter, and a target-limited check answers 429 with Retry-After and one
// log line holding the hostname and client prefix only.
func TestTargetLimitAnswers429(t *testing.T) {
	var logs bytes.Buffer
	r := staticResolver{[]netip.Addr{netip.MustParseAddr("93.184.215.14")}}
	c := &diag.Checker{Resolver: r, Dial: pipeDial}
	h := New(c, log.New(&logs, "", 0), Options{TrustXFF: true}).Handler()
	if c.Targets == nil {
		t.Fatal("server.New left the Checker without a target limiter")
	}
	// fresh=1 (再診断): the cache must not answer, and the target limit must still hold.
	u := "/api/check?fresh=1&url=" + url.QueryEscape("http://victim.test/p?token=SECRET9")
	for i := 0; i < limits.TargetHostBurst; i++ {
		if resp, body := getFrom(t, h, u, fmt.Sprintf("198.51.100.%d", i+1)); resp.StatusCode != 200 {
			t.Fatalf("check %d: %d %s", i+1, resp.StatusCode, body)
		}
	}
	// ⚠ A different client each time: the hostname budget holds whoever asks.
	resp, body := getFrom(t, h, u, "198.51.100.99")
	if resp.StatusCode != http.StatusTooManyRequests || !strings.Contains(body, `"server.target_rate_limited"`) || resp.Header.Get("Retry-After") == "" {
		t.Fatalf("over the hostname budget: %d Retry-After=%q %s", resp.StatusCode, resp.Header.Get("Retry-After"), body)
	}
	if !strings.Contains(logs.String(), "limit refused=target target=victim.test client=198.51.100.0/24") {
		t.Errorf("log: %s", logs.String())
	}
	for _, leak := range []string{"SECRET9", "/p?", "198.51.100.99"} {
		if strings.Contains(logs.String(), leak) {
			t.Errorf("log contains %q", leak)
		}
	}
}

// countingDial counts connections.
type countingDial struct{ n int }

func (c *countingDial) dial(ctx context.Context, a netip.Addr, p uint16) (net.Conn, error) {
	c.n++
	return pipeDial(ctx, a, p)
}

// hidetzu/connect-doctor#28: a repeated check is answered from the cache
// without connecting; 再診断 connects again.
func TestRepeatedCheckIsAnsweredFromTheCache(t *testing.T) {
	d := &countingDial{}
	r := staticResolver{[]netip.Addr{netip.MustParseAddr("93.184.215.14")}}
	h := New(&diag.Checker{Resolver: r, Dial: d.dial}, log.New(io.Discard, "", 0), Options{TrustXFF: true}).Handler()

	_, first := getFrom(t, h, "/api/check?url=http://cache.test/", "198.51.100.1")
	_, second := getFrom(t, h, "/api/check?url=HTTP://Cache.TEST/", "198.51.100.2")
	if d.n != 1 {
		t.Fatalf("connections after a repeat: %d, want 1", d.n)
	}
	var a, b diag.Result
	_ = json.Unmarshal([]byte(first), &a)
	_ = json.Unmarshal([]byte(second), &b)
	if a.Cached || !b.Cached || !a.CheckedAt.Equal(b.CheckedAt) {
		t.Errorf("first cached=%v, second cached=%v, checked_at %s vs %s", a.Cached, b.Cached, a.CheckedAt, b.CheckedAt)
	}
	_, page := getFrom(t, h, "/?url=http://cache.test/", "198.51.100.3")
	if d.n != 1 || !strings.Contains(page, "秒前の診断結果です") || !strings.Contains(page, "fresh=1") {
		t.Errorf("page from the cache: connections %d, age line or 再診断 link missing", d.n)
	}
	getFrom(t, h, "/api/check?fresh=1&url=http://cache.test/", "198.51.100.4")
	if d.n != 2 {
		t.Errorf("再診断 made %d connections in total, want 2", d.n)
	}
}

// hidetzu/connect-doctor#25 AC 3: colour encodes state, never a layer. The
// stylesheet must name no step, and a step's class must be its status.
func TestColourIsStateNotLayer(t *testing.T) {
	b, err := templates.ReadFile("templates/page.html")
	if err != nil {
		t.Fatal(err)
	}
	css := string(b)[strings.Index(string(b), "<style>"):strings.Index(string(b), "</style>")]
	for _, layer := range []string{".dns", ".tcp", ".tls", ".http"} {
		if strings.Contains(strings.ToLower(css), layer) {
			t.Errorf("the stylesheet has a rule for %s", layer)
		}
	}
	if got := stepClass(diag.Step{Step: diag.StepTLS, Status: diag.StatusFailed}); got != "failed" {
		t.Errorf("a failed TLS step has class %q, want its status only", got)
	}
	if got := stepClass(diag.Step{Step: diag.StepHTTP, Status: diag.StatusOK, Detail: &diag.Detail{StatusCode: 503}}); got != "ok warn" {
		t.Errorf("an HTTP 503 step has class %q, want \"ok warn\"", got)
	}
}

func TestVantageChip(t *testing.T) {
	r := staticResolver{}
	_, page := get(t, New(&diag.Checker{Resolver: r, Dial: pipeDial}, log.New(io.Discard, "", 0), Options{Vantage: "Tokyo, Japan"}).Handler(), "/")
	if !strings.Contains(page, "Tokyo, Japan から観測") {
		t.Error("the chip does not name the vantage given by the operator")
	}
	_, page = get(t, New(&diag.Checker{Resolver: r, Dial: pipeDial}, log.New(io.Discard, "", 0), Options{}).Handler(), "/")
	if strings.Contains(page, "Tokyo") || !strings.Contains(page, "ConnectDoctorのサーバから観測") {
		t.Error("without -vantage the chip must name no place")
	}
}

// hidetzu/connect-doctor#29: the icon is served as the favicon and inlined in
// the header; the page still loads nothing from another host.
func TestServiceIcon(t *testing.T) {
	h := newTestServer(staticResolver{}, io.Discard, 1).Handler()
	resp, body := get(t, h, "/favicon.svg")
	if resp.StatusCode != 200 || resp.Header.Get("Content-Type") != "image/svg+xml" || !strings.HasPrefix(body, "<svg") {
		t.Fatalf("/favicon.svg: %d %q %.20q", resp.StatusCode, resp.Header.Get("Content-Type"), body)
	}
	resp, page := get(t, h, "/")
	if !strings.Contains(page, `<link rel="icon" href="/favicon.svg" type="image/svg+xml">`) {
		t.Error("the page does not link the favicon")
	}
	if !strings.Contains(page, `<span class="brand"><span aria-hidden="true"><svg`) {
		t.Error("the header does not show the icon before the name")
	}
	csp := resp.Header.Get("Content-Security-Policy")
	for _, want := range []string{"default-src 'none'", "img-src 'self'"} {
		if !strings.Contains(csp, want) {
			t.Errorf("CSP %q lacks %q", csp, want)
		}
	}
	if strings.Contains(csp, "script-src") || strings.Contains(csp, "*") {
		t.Errorf("CSP widened beyond the favicon: %q", csp)
	}
	// ⚠ What the page LOADS: src attributes and <link> elements. A plain <a href>
	// to the source (hidetzu/connect-doctor#41) is navigation, not a load.
	if strings.Contains(page, `src="http`) || regexp.MustCompile(`<link [^>]*href="http`).MatchString(page) {
		t.Error("the page loads something from another host")
	}
}

// hidetzu/connect-doctor#41: every page says what ConnectDoctor does, what
// happens to the URL, and where the source is.
func TestTaglineAndFooter(t *testing.T) {
	h := newTestServer(staticResolver{[]netip.Addr{netip.MustParseAddr("93.184.215.14")}}, io.Discard, 1).Handler()
	for _, path := range []string{"/", "/?url=http://ok.test/", "/?url=http://127.0.0.1/"} {
		_, page := get(t, h, path)
		for _, want := range []string{diag.Tagline, diag.PrivacyNote(), `<a href="` + diag.SourceURL + `">`} {
			if !strings.Contains(page, want) {
				t.Errorf("%s does not contain %q", path, want)
			}
		}
	}
	// ⚠ The sentence states the cache's real duration.
	if !strings.Contains(diag.PrivacyNote(), strconv.Itoa(int(limits.CacheTTL.Seconds()))+"秒") {
		t.Errorf("privacy note %q does not match limits.CacheTTL", diag.PrivacyNote())
	}
}

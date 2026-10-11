package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"image/png"
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
	"sync/atomic"
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

	resp, page := get(t, h, "/diagnose?url="+url.QueryEscape("http://ok.test/"))
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
	h := newTestServer(staticResolver{}, io.Discard, 1).Handler()
	// ⚠ Both pages carry the one form, and it submits to /diagnose (hidetzu/connect-doctor#51).
	for _, path := range []string{"/", "/diagnose"} {
		_, page := get(t, h, path)
		for _, want := range []string{`<form method="get" action="/diagnose">`, "あなたのPCからの接続結果ではありません", `<div class="strip"`} {
			if !strings.Contains(page, want) {
				t.Errorf("%s does not contain %q", path, want)
			}
		}
		if strings.Contains(page, `class="verdict`) {
			t.Errorf("%s without a URL shows a conclusion card", path)
		}
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
	resp, page := get(t, h, "/diagnose?url=http://ok.test/")
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
	get(t, h, "/diagnose?url="+url.QueryEscape("http://x.test/?token="+secret))
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
	if resp.StatusCode != http.StatusTooManyRequests || !strings.Contains(body, `"server.rate_limited"`) || resp.Header.Get("Retry-After") == "" || !strings.Contains(body, "あなたの診断回数") {
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
	// The page is limited too, and says it is the visitor's limit and how long
	// to wait (hidetzu/connect-doctor#43).
	resp, page := getFrom(t, h, "/diagnose?url=http://okp.test/", "203.0.113.7")
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("page over the limit: %d", resp.StatusCode)
	}
	secs, _ := strconv.Atoi(resp.Header.Get("Retry-After"))
	if want := diag.RateLimited("burst", secs); secs < 1 || !strings.Contains(page, want) {
		t.Errorf("page does not say %q (Retry-After %q)", want, resp.Header.Get("Retry-After"))
	}
	if !strings.Contains(page, "あなたの診断回数") || !strings.Contains(page, "3回まで") {
		t.Errorf("the page does not name the visitor's limit")
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
	_, page := getFrom(t, h, "/diagnose?url=http://cache.test/", "198.51.100.3")
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
	// ⚠ Every <style> in every template: the shared sheet and each page's own.
	files, err := templates.ReadDir("templates")
	if err != nil {
		t.Fatal(err)
	}
	styles := 0
	for _, f := range files {
		b, err := templates.ReadFile("templates/" + f.Name())
		if err != nil {
			t.Fatal(err)
		}
		for _, css := range regexp.MustCompile(`(?s)<style>.*?</style>`).FindAllString(string(b), -1) {
			styles++
			for _, layer := range []string{".dns", ".tcp", ".tls", ".http"} {
				if strings.Contains(strings.ToLower(css), layer) {
					t.Errorf("%s: the stylesheet has a rule for %s", f.Name(), layer)
				}
			}
		}
	}
	if styles == 0 {
		t.Fatal("no <style> found in any template")
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
	if !strings.Contains(page, `<a class="brand" href="/"><span aria-hidden="true"><svg`) {
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

// hidetzu/connect-doctor#41: every page says what ConnectDoctor does and what
// happens to the URL. ⚠ The link to the source was removed (owner decision
// 2026-10-11): no page links the repository.
func TestTaglineAndFooter(t *testing.T) {
	h := newTestServer(staticResolver{[]netip.Addr{netip.MustParseAddr("93.184.215.14")}}, io.Discard, 1).Handler()
	for _, path := range []string{"/", "/diagnose", "/diagnose?url=http://ok.test/", "/diagnose?url=http://127.0.0.1/"} {
		_, page := get(t, h, path)
		for _, want := range []string{diag.PrivacyNote()} {
			if !strings.Contains(page, want) {
				t.Errorf("%s does not contain %q", path, want)
			}
		}
		if strings.Contains(page, "github.com") || strings.Contains(page, "ソースコード") {
			t.Errorf("%s links the source", path)
		}
	}
	if _, page := get(t, h, "/"); !strings.Contains(page, diag.Tagline) {
		t.Error("the index does not show the tagline")
	}
	// ⚠ The sentence states the cache's real duration.
	// The request log keeps the URL (owner decision 2026-10-11), so the note
	// must not promise that nothing is stored.
	if strings.Contains(diag.PrivacyNote(), "保存しません") || !strings.Contains(diag.PrivacyNote(), "アクセスログにしばらく残ります") {
		t.Errorf("privacy note %q does not say the access log keeps the URL", diag.PrivacyNote())
	}
	// The cache's duration is not given in seconds (owner decision 2026-10-11).
	if strings.ContainsAny(diag.PrivacyNote(), "0123456789") {
		t.Errorf("privacy note %q gives a number", diag.PrivacyNote())
	}
}

func TestRateLimitedSentences(t *testing.T) {
	for reason, want := range map[string]string{
		"burst": "続けて診断できるのは3回まで", "hour": "1時間に30回まで", "day": "1日に100回まで", "concurrent": "前の診断がまだ終わっていません",
	} {
		if got := diag.RateLimited(reason, 9); !strings.Contains(got, want) {
			t.Errorf("%s: %q", reason, got)
		}
	}
	if got := diag.RateLimited("burst", 9); !strings.Contains(got, "あと 9 秒") {
		t.Errorf("seconds missing: %q", got)
	}
}

// hidetzu/connect-doctor#44: description and Open Graph tags; the absolute
// ones only when the operator gives the public URL.
func TestOpenGraph(t *testing.T) {
	r := staticResolver{}
	with := New(&diag.Checker{Resolver: r, Dial: pipeDial}, log.New(io.Discard, "", 0), Options{PublicURL: "https://cd.example/"}).Handler()
	_, page := get(t, with, "/")
	for _, want := range []string{
		`<meta name="description" content="` + diag.Tagline + `">`,
		`<meta property="og:title" content="ConnectDoctor">`,
		`<meta property="og:url" content="https://cd.example/">`,
		`<meta property="og:image" content="https://cd.example/og.png">`,
		`<meta name="twitter:card" content="summary_large_image">`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("page lacks %s", want)
		}
	}
	if _, diagnose := get(t, with, "/diagnose"); !strings.Contains(diagnose, `<meta property="og:url" content="https://cd.example/diagnose">`) {
		t.Error("/diagnose's og:url is not its own path")
	}
	without := New(&diag.Checker{Resolver: r, Dial: pipeDial}, log.New(io.Discard, "", 0), Options{}).Handler()
	_, page = get(t, without, "/")
	if strings.Contains(page, "og:image") || strings.Contains(page, "og:url") || !strings.Contains(page, "og:description") {
		t.Error("without -public-url, og:image/og:url must be omitted and the rest kept")
	}
	resp, body := get(t, with, "/og.png")
	if resp.StatusCode != 200 || resp.Header.Get("Content-Type") != "image/png" {
		t.Fatalf("/og.png: %d %q", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	cfg, err := png.DecodeConfig(strings.NewReader(body))
	if err != nil || cfg.Width != 1200 || cfg.Height != 630 {
		t.Errorf("/og.png is %dx%d (%v), want 1200x630", cfg.Width, cfg.Height, err)
	}
}

// countingResolver counts lookups, to prove a path resolved nothing.
type countingResolver struct {
	staticResolver
	n atomic.Int32
}

func (r *countingResolver) LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error) {
	r.n.Add(1)
	return r.staticResolver.LookupNetIP(ctx, network, host)
}

// The landing page (hidetzu/connect-doctor#51, candidate C, settled by
// looking): every sentence diag composes is on it, the numbers are
// internal/limits', it starts no check and spends no limit.
func TestLandingPage(t *testing.T) {
	r := &countingResolver{staticResolver: staticResolver{[]netip.Addr{netip.MustParseAddr("93.184.215.14")}}}
	var logs bytes.Buffer
	h := New(&diag.Checker{Resolver: r, Dial: pipeDial}, log.New(&logs, "", 0), Options{PublicURL: "https://cd.example/", Vantage: "Tokyo, Japan"}).Handler()
	resp, page := get(t, h, "/")
	if resp.StatusCode != 200 || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/html") || resp.Header.Get("Content-Security-Policy") != pageCSP {
		t.Fatalf("/: %d %q CSP=%q", resp.StatusCode, resp.Header.Get("Content-Type"), resp.Header.Get("Content-Security-Policy"))
	}
	wants := []string{diag.HeroLede, diag.LayersTitle, diag.SymptomsTitle, "Tokyo, Japan から観測", diag.PrivacyNote(),
		`<form method="get" action="/diagnose">`, `curl &#39;https://cd.example/api/check?url=https://example.com&#39;`}
	for _, ph := range diag.HeroHeadline {
		wants = append(wants, "<span>"+ph+"</span>")
	}
	wants = append(wants, diag.Trust...)
	for _, l := range diag.Layers {
		wants = append(wants, "<b>"+strings.ToUpper(l.Name)+"</b>", l.Short)
	}
	wants = append(wants, diag.Symptoms...)
	for _, sec := range diag.Sections("https://cd.example") {
		wants = append(wants, sec.Title)
		wants = append(wants, sec.Items...)
	}
	for _, want := range wants {
		if !strings.Contains(page, template.HTMLEscapeString(want)) && !strings.Contains(page, want) {
			t.Errorf("/ does not contain %q", want)
		}
	}
	if strings.Contains(page, `class="verdict`) {
		t.Error("the landing page shows a conclusion card")
	}
	// ⚠ The numbers are the limits', never typed in.
	for _, want := range []string{
		"続けて" + strconv.Itoa(limits.ClientBurst) + "回まで", "1時間に" + strconv.Itoa(limits.ClientPerHour) + "回まで",
		"1日に" + strconv.Itoa(limits.ClientPerDay) + "回まで", "たどるのは" + strconv.Itoa(limits.RedirectHops) + "回まで",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("/ does not say %q", want)
		}
	}
	// What the page loads: nothing from another host (hidetzu/connect-doctor#29).
	if strings.Contains(page, `src="http`) || regexp.MustCompile(`<link [^>]*href="http`).MatchString(page) {
		t.Error("/ loads something from another host")
	}
	// Without -public-url the example is a relative path; without -vantage no place is named.
	_, plain := get(t, New(&diag.Checker{Resolver: r, Dial: pipeDial}, log.New(io.Discard, "", 0), Options{}).Handler(), "/")
	if !strings.Contains(plain, `curl &#39;/api/check?url=https://example.com&#39;`) || !strings.Contains(plain, "ConnectDoctorのサーバから観測") {
		t.Error("/ without options: the example is not relative, or a place is named")
	}
	// The landing page starts no check and spends no limit: the burst is still whole afterwards.
	for i := 0; i < limits.ClientBurst+2; i++ {
		get(t, h, "/")
	}
	if n := r.n.Load(); n != 0 {
		t.Errorf("the landing page resolved %d names", n)
	}
	for i := 0; i < limits.ClientBurst; i++ {
		if resp, _ := get(t, h, "/api/check?url=http://ok"+strconv.Itoa(i)+".test/"); resp.StatusCode != 200 {
			t.Fatalf("check %d after visiting /: %d", i, resp.StatusCode)
		}
	}
	if strings.Contains(logs.String(), "limit refused") {
		t.Errorf("visiting / spent a limit: %s", logs.String())
	}
	// ⚠ The sentence about the marks restates glyph() and the cache line
	// restates the diagnose template; this is the mechanical cross-check that
	// keeps the copies together (CLAUDE.md § 3).
	prose := strings.Join(func() (all []string) {
		for _, sec := range diag.Sections("") {
			all = append(all, sec.Items...)
		}
		return
	}(), "\n")
	for _, st := range []diag.Status{diag.StatusOK, diag.StatusFailed, diag.StatusRefused, diag.StatusSkipped} {
		if !strings.Contains(prose, glyph(st)+" は") {
			t.Errorf("/ does not explain the mark %q for %s", glyph(st), st)
		}
	}
	if tmpl, _ := templates.ReadFile("templates/diagnose.html"); !strings.Contains(prose, "秒前の診断結果です") || !strings.Contains(string(tmpl), "秒前の診断結果です") {
		t.Error("the cache sentence on / and the diagnose template no longer say the same thing")
	}
	// The cards name the four steps in the order the check runs, by the steps' own names.
	if got := []string{diag.Layers[0].Name, diag.Layers[1].Name, diag.Layers[2].Name, diag.Layers[3].Name}; strings.Join(got, " ") != "dns tcp tls http" {
		t.Errorf("cards are %v", got)
	}
	// ⚠ Never open with what does not work (CLAUDE.md § 4-1): what is declined comes after how to read a result.
	if secs := diag.Sections(""); secs[0].Title != "結果の読み方" {
		t.Errorf("the first prose section is %q", secs[0].Title)
	}
}

// Before hidetzu/connect-doctor#51 results lived at /?url=…; those links and
// /about are sent on with a 301. ⚠ The redirect resolves nothing and spends
// no limit.
func TestOldURLsRedirect(t *testing.T) {
	r := &countingResolver{staticResolver: staticResolver{[]netip.Addr{netip.MustParseAddr("93.184.215.14")}}}
	var logs bytes.Buffer
	h := New(&diag.Checker{Resolver: r, Dial: pipeDial}, log.New(&logs, "", 0), Options{}).Handler()
	for path, want := range map[string]string{
		"/?url=https%3A%2F%2Fexample.com%2Fa%3Fb%3Dc&fresh=1": "/diagnose?url=https%3A%2F%2Fexample.com%2Fa%3Fb%3Dc&fresh=1",
		"/?url=": "/diagnose?url=",
		"/about": "/",
	} {
		for i := 0; i < limits.ClientBurst+2; i++ {
			resp, _ := get(t, h, path)
			if resp.StatusCode != http.StatusMovedPermanently || resp.Header.Get("Location") != want {
				t.Fatalf("%s: %d Location=%q, want 301 %q", path, resp.StatusCode, resp.Header.Get("Location"), want)
			}
		}
	}
	if n := r.n.Load(); n != 0 {
		t.Errorf("redirects resolved %d names", n)
	}
	if resp, _ := get(t, h, "/api/check?url=http://ok.test/"); resp.StatusCode != 200 {
		t.Errorf("after the redirects the first check is %d: a redirect spent the limit", resp.StatusCode)
	}
	if strings.Contains(logs.String(), "limit refused") {
		t.Errorf("a redirect spent a limit: %s", logs.String())
	}
}

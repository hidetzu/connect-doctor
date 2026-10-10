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
		res.Conclusion.Summary, // the page shows the API's conclusion, verbatim
		diag.ObservedFromNote,
		"93.184.215.14",
		"✅", "ステータス200",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("page does not contain %q", want)
		}
	}
}

func TestPageWithoutURLShowsFormAndVantage(t *testing.T) {
	_, page := get(t, newTestServer(staticResolver{}, io.Discard, 1).Handler(), "/")
	for _, want := range []string{`<form method="get" action="/">`, diag.ObservedFromNote} {
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
	u := "/api/check?url=" + url.QueryEscape("http://victim.test/p?token=SECRET9")
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

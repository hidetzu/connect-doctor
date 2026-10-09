package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/hidetzu/connect-doctor/internal/diag"
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
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	b, _ := io.ReadAll(rec.Result().Body)
	return rec.Result(), string(b)
}

func newTestServer(r diag.Resolver, logs io.Writer, slots int) *Server {
	return newWithSlots(&diag.Checker{Resolver: r}, log.New(logs, "", 0), slots)
}

func TestAPIAndPageRenderTheSameResult(t *testing.T) {
	s := newTestServer(staticResolver{[]netip.Addr{netip.MustParseAddr("93.184.215.14")}}, io.Discard, 4)
	h := s.Handler()

	resp, body := get(t, h, "/api/check?url="+url.QueryEscape("https://ok.test/"))
	if resp.StatusCode != 200 || !strings.HasPrefix(resp.Header.Get("Content-Type"), "application/json") {
		t.Fatalf("api: %d %s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	var res diag.Result
	if err := json.Unmarshal([]byte(body), &res); err != nil {
		t.Fatal(err)
	}
	if res.Conclusion.Status != diag.ConclusionIncomplete || res.ObservedFrom != "server" {
		t.Fatalf("api result: %+v", res)
	}

	resp, page := get(t, h, "/?url="+url.QueryEscape("https://ok.test/"))
	if resp.StatusCode != 200 {
		t.Fatalf("page: %d", resp.StatusCode)
	}
	for _, want := range []string{
		res.Conclusion.Summary, // the page shows the API's conclusion, verbatim
		diag.ObservedFromNote,
		"93.184.215.14",
		"✅", "🚧", diag.StatusLabel(diag.StatusNotImplemented),
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
		get(t, h, "/api/check?url=https://ok.test/")
	}()
	<-r.entered // the only slot is now held

	resp, body := get(t, h, "/api/check?url=https://ok.test/")
	if resp.StatusCode != http.StatusServiceUnavailable || !strings.Contains(body, `"server.busy"`) {
		t.Errorf("api over the bound: %d %s", resp.StatusCode, body)
	}
	if strings.Contains(body, "dns.") {
		t.Errorf("busy answered as a DNS outcome: %s", body)
	}
	resp, page := get(t, h, "/?url=https://ok.test/")
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
	get(t, h, "/api/check?url="+url.QueryEscape("https://x.test/?token="+secret))
	get(t, h, "/?url="+url.QueryEscape("https://x.test/?token="+secret))
	if strings.Contains(logs.String(), secret) || strings.Contains(logs.String(), "x.test") {
		t.Errorf("log leaks the checked URL:\n%s", logs.String())
	}
	// ⚠ Control: the log did receive the requests.
	if n := strings.Count(logs.String(), "\n"); n != 2 {
		t.Errorf("log has %d lines, want 2:\n%s", n, logs.String())
	}
}

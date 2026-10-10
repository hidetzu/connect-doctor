//go:build e2e

// Package e2e is the final gate: the real binary, built now, over real HTTP,
// resolving through a DNS server we run on loopback, connecting to listeners
// inside an empty network namespace (main_test.go).
//
// ⚠ No policy is widened for this: the binary is the one that ships, and
// only its resolver is pointed at internal/dnstest (an operator setting).
package e2e

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hidetzu/connect-doctor/internal/diag"
	"github.com/hidetzu/connect-doctor/internal/dnstest"
)

type instance struct {
	base string
	dns  *dnstest.Server
}

// start builds the binary from this tree and runs it against a fresh fake DNS
// server. ⚠ Building here, every run, is how the gate knows it measures the
// code just written and not a stale artefact.
func start(t *testing.T) *instance {
	t.Helper()
	a := netip.MustParseAddr
	dns, err := dnstest.Start(map[string]dnstest.Answer{
		"ok.test":         {Addrs: []netip.Addr{a(addrListen), a(addrV6)}},
		"closed.test":     {Addrs: []netip.Addr{a(addrClosed)}},
		"drop.test":       {Addrs: []netip.Addr{a(addrDrop)}},
		"unreach.test":    {Addrs: []netip.Addr{a(addrUnreachable)}},
		"v6only.test":     {Addrs: []netip.Addr{a(addrV6)}},
		"fallback.test":   {Addrs: []netip.Addr{a(addrDrop), a(addrListen)}},
		"expired.test":    {Addrs: []netip.Addr{a(addrExpired)}},
		"untrusted.test":  {Addrs: []netip.Addr{a(addrUntrusted)}},
		"mismatch.test":   {Addrs: []netip.Addr{a(addrMismatch)}},
		"plain.test":      {Addrs: []netip.Addr{a(addrPlain)}},
		"silent.test":     {Addrs: []netip.Addr{a(addrSilent)}},
		"status503.test":  {Addrs: []netip.Addr{a(addrListen)}},
		"bigbody.test":    {Addrs: []netip.Addr{a(addrListen)}},
		"bigheader.test":  {Addrs: []netip.Addr{a(addrListen)}},
		"httpsilent.test": {Addrs: []netip.Addr{a(addrHTTPSilent)}},
		"httpclose.test":  {Addrs: []netip.Addr{a(addrHTTPClose)}},
		"redir.test":      {Addrs: []netip.Addr{a(addrListen)}},
		"toloop.test":     {Addrs: []netip.Addr{a(addrListen)}},
		"tolocal.test":    {Addrs: []netip.Addr{a(addrListen)}},
		"toport.test":     {Addrs: []netip.Addr{a(addrListen)}},
		"spin.test":       {Addrs: []netip.Addr{a(addrListen)}},
		"rel.test":        {Addrs: []netip.Addr{a(addrListen)}},
		"slowhop.test":    {Addrs: []netip.Addr{a(addrListen)}},
		"private.test":    {Addrs: []netip.Addr{a("10.0.0.7")}},
		"loop.test":       {Addrs: []netip.Addr{a("127.0.0.1")}},
		"slow.test":       {Drop: true},
		"broken.test":     {Rcode: dnstest.RcodeServFail},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { dns.Close() })

	bin := filepath.Join(t.TempDir(), "connect-doctor")
	build := exec.Command("go", "build", "-o", bin, "../cmd/connect-doctor")
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		t.Fatalf("build: %v", err)
	}

	// -trust-xff is the operator setting Cloud Run runs with; it lets each
	// request below be its own client, so the per-client limits only bite in
	// the test written for them (ratelimit_test.go). ⚠ The limits stay on.
	cmd := exec.Command(bin, "-addr", "127.0.0.1:0", "-dns-server", dns.Addr(), "-trust-xff")
	cmd.Env = append(os.Environ(), trustEnv...)
	stdout, _ := cmd.StdoutPipe()
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })

	lines := make(chan string, 1)
	go func() {
		sc := bufio.NewScanner(stdout)
		if sc.Scan() {
			lines <- sc.Text()
		}
	}()
	select {
	case line := <-lines:
		base, ok := strings.CutPrefix(line, "connect-doctor: listening on ")
		if !ok {
			t.Fatalf("unexpected first line: %q", line)
		}
		return &instance{base: base, dns: dns}
	case <-time.After(10 * time.Second):
		t.Fatal("binary did not announce its address")
	}
	return nil
}

// clientSeq gives every request a distinct client address unless a test
// asks for a specific one.
var clientSeq atomic.Int64

func nextClient() string {
	n := clientSeq.Add(1)
	return fmt.Sprintf("198.18.%d.%d", (n>>8)&0xff, n&0xff)
}

// fetch sends GET path with X-Forwarded-For set to client.
func (in *instance) fetch(t *testing.T, path, client string) (*http.Response, string) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, in.base+path, nil)
	req.Header.Set("X-Forwarded-For", client)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp, string(b)
}

func (in *instance) api(t *testing.T, u string) (int, diag.Result, string) {
	t.Helper()
	resp, body := in.fetch(t, "/api/check?url="+url.QueryEscape(u), nextClient())
	var res diag.Result
	if err := json.Unmarshal([]byte(body), &res); err != nil {
		t.Fatalf("%s: not JSON: %s", u, body)
	}
	return resp.StatusCode, res, body
}

func (in *instance) page(t *testing.T, path string) string {
	t.Helper()
	_, body := in.fetch(t, path, nextClient())
	return body
}

func ladder(res diag.Result) string {
	var s []string
	for _, st := range res.Hops[0].Steps {
		s = append(s, fmt.Sprintf("%s=%s", st.Step, st.Status))
	}
	return strings.Join(s, " ")
}

// AC 5 and AC 6, together: the control reaches the DNS server, the refused
// URLs do not.
func TestRefusedURLsReachNothing(t *testing.T) {
	in := start(t)

	// ⚠ Control first: a permitted URL causes queries for exactly its name.
	code, res, _ := in.api(t, "https://ok.test/")
	if code != 200 || res.Conclusion.Status != diag.ConclusionOK || res.ObservedFrom != "server" {
		t.Fatalf("ok.test: %d %+v", code, res.Conclusion)
	}
	if got := ladder(res); got != "dns=ok tcp=ok tls=ok http=ok" {
		t.Errorf("ok.test ladder: %s", got)
	}
	if got := res.Hops[0].Steps[0].Detail.Addresses; strings.Join(got, ",") != "93.184.215.14,2606:4700:4700::1111" {
		t.Errorf("ok.test addresses: %v", got)
	}
	// The pure-Go resolver asks A and AAAA: two queries, one name.
	if n := in.dns.Queries("ok.test"); n == 0 {
		t.Fatalf("control: the DNS server saw no query for ok.test — the gate cannot prove anything")
	}
	baseline := in.dns.TotalQueries()
	if baseline != in.dns.Queries("ok.test") {
		t.Errorf("queries for names other than ok.test: total %d", baseline)
	}

	loopBefore := loopbackAccepts.Load()
	for _, u := range []string{
		"http://127.0.0.1/",
		"http://[::1]/",
		"http://169.254.169.254/latest/meta-data/",
		"http://localhost/",
		"http://ok.test:8080/",
		"http://2130706433/",
		"file:///etc/passwd",
	} {
		code, res, _ := in.api(t, u)
		if code != 400 || res.Conclusion.Status != diag.ConclusionRefused {
			t.Errorf("%s: %d %+v, want 400 refused", u, code, res.Conclusion)
		}
		if got := ladder(res); got != "dns=skipped tcp=skipped tls=skipped http=skipped" {
			t.Errorf("%s ladder: %s", u, got)
		}
	}
	if n := in.dns.TotalQueries(); n != baseline {
		t.Errorf("refused URLs caused %d DNS queries, want 0", n-baseline)
	}
	// A name that resolves to loopback is resolved (that is how the address
	// is learned) and then refused before any dial.
	if _, res, _ := in.api(t, "https://loop.test/"); res.Conclusion.Code != "dns.refused_address" {
		t.Errorf("loop.test: %+v, want dns.refused_address", res.Conclusion)
	}
	// ⚠ And nothing was dialled: the loopback listeners saw no connection,
	// while the control above did reach the public listener.
	if n := loopbackAccepts.Load() - loopBefore; n != 0 {
		t.Errorf("refused URLs reached the loopback listener %d times, want 0", n)
	}
	if publicAccepts.Load() == 0 {
		t.Error("control: the public listener saw no connection — the gate cannot prove the refusal")
	}
}

func TestDNSOutcomesThroughTheBinary(t *testing.T) {
	in := start(t)
	cases := map[string]struct{ status, code string }{
		"https://missing.test/": {diag.ConclusionFailed, "dns.not_found"},
		"https://broken.test/":  {diag.ConclusionFailed, "dns.server_failure"},
		"https://slow.test/":    {diag.ConclusionFailed, "dns.timeout"},
		"https://private.test/": {diag.ConclusionRefused, "dns.refused_address"},
	}
	cases["https://loop.test/"] = struct{ status, code string }{diag.ConclusionRefused, "dns.refused_address"}
	for u, want := range cases {
		code, res, body := in.api(t, u)
		if code != 200 || res.Conclusion.Status != want.status || res.Conclusion.Code != want.code || res.Conclusion.FailedStep != "dns" {
			t.Errorf("%s: %d %+v, want %s/%s", u, code, res.Conclusion, want.status, want.code)
		}
		if strings.Contains(body, "10.0.0.7") || strings.Contains(body, "127.0.0.1") {
			t.Errorf("%s: response shows the refused address", u)
		}
	}
}

// AC 7.
func TestPage(t *testing.T) {
	in := start(t)
	page := in.page(t, "/")
	for _, want := range []string{`<form method="get" action="/">`, diag.ObservedFromNote} {
		if !strings.Contains(page, want) {
			t.Errorf("/ does not contain %q", want)
		}
	}
	page = in.page(t, "/?url="+url.QueryEscape("https://ok.test/"))
	_, res, _ := in.api(t, "https://ok.test/")
	for _, want := range []string{"✅", "ステータス200", res.Conclusion.Summary, diag.ObservedFromNote, " ms</span>"} {
		if !strings.Contains(page, want) {
			t.Errorf("/?url= does not contain %q", want)
		}
	}
}

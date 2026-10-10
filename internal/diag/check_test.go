package diag

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/netip"
	"strings"
	"testing"

	"github.com/hidetzu/connect-doctor/internal/target"
	"github.com/hidetzu/connect-doctor/internal/tlstest"
)

type fakeResolver struct {
	addrs []string
	err   error
	asked []string
}

func (f *fakeResolver) LookupNetIP(_ context.Context, _, host string) ([]netip.Addr, error) {
	f.asked = append(f.asked, host)
	var out []netip.Addr
	for _, a := range f.addrs {
		out = append(out, netip.MustParseAddr(a))
	}
	return out, f.err
}

// okDial "connects" to anything, over an in-memory pipe whose other end is a
// TLS server with a certificate the test CA issued for example.com.
func okDial(context.Context, netip.Addr, uint16) (net.Conn, error) {
	return tlstest.Pipe(&validCert, nil), nil
}

func check(r Resolver, url string) Result {
	return (&Checker{Resolver: r, Dial: okDial}).Check(context.Background(), url)
}

func statuses(h Hop) string {
	var s []string
	for _, st := range h.Steps {
		s = append(s, st.Step+"="+string(st.Status))
	}
	return strings.Join(s, " ")
}

func TestDNSOutcomes(t *testing.T) {
	cases := []struct {
		name   string
		r      *fakeResolver
		status Status
		code   string
	}{
		{"nxdomain", &fakeResolver{err: &net.DNSError{Err: "no such host", IsNotFound: true}}, StatusFailed, "dns.not_found"},
		{"empty success", &fakeResolver{}, StatusFailed, "dns.not_found"},
		{"timeout", &fakeResolver{err: &net.DNSError{Err: "i/o timeout", IsTimeout: true}}, StatusFailed, "dns.timeout"},
		{"deadline", &fakeResolver{err: context.DeadlineExceeded}, StatusFailed, "dns.timeout"},
		{"servfail", &fakeResolver{err: &net.DNSError{Err: "server misbehaving", IsTemporary: true}}, StatusFailed, "dns.server_failure"},
		{"other", &fakeResolver{err: errors.New("boom")}, StatusFailed, "dns.server_failure"},
		{"private", &fakeResolver{addrs: []string{"10.0.0.5"}}, StatusRefused, "dns.refused_address"},
		{"mixed", &fakeResolver{addrs: []string{"93.184.215.14", "127.0.0.1"}}, StatusRefused, "dns.refused_address"},
		{"mapped loopback", &fakeResolver{addrs: []string{"::ffff:127.0.0.1"}}, StatusRefused, "dns.refused_address"},
	}
	for _, c := range cases {
		res := check(c.r, "https://example.com/")
		dns := res.Hops[0].Steps[0]
		if dns.Status != c.status || dns.Code != c.code {
			t.Errorf("%s: dns = %s/%s, want %s/%s", c.name, dns.Status, dns.Code, c.status, c.code)
		}
		if res.Conclusion.FailedStep != StepDNS || res.Conclusion.Code != c.code {
			t.Errorf("%s: conclusion = %+v", c.name, res.Conclusion)
		}
		if got := statuses(res.Hops[0]); !strings.HasSuffix(got, "tcp=skipped tls=skipped http=skipped") {
			t.Errorf("%s: steps = %s", c.name, got)
		}
		if dns.Message == "" || res.Conclusion.Summary == "" {
			t.Errorf("%s: missing words: %+v", c.name, dns)
		}
	}
}

// ⚠ A refused address never appears anywhere in the result (docs/adr/0004).
func TestRefusedAddressNeverShown(t *testing.T) {
	for _, addrs := range [][]string{{"10.1.2.3"}, {"93.184.215.14", "10.1.2.3"}, {"fd12:3456::9"}} {
		res := check(&fakeResolver{addrs: addrs}, "https://example.com/")
		b, _ := json.Marshal(res)
		for _, a := range addrs {
			if strings.Contains(string(b), a) {
				t.Errorf("refused result for %v contains %s: %s", addrs, a, b)
			}
		}
		if res.Conclusion.Status != ConclusionRefused {
			t.Errorf("%v: conclusion %s, want refused", addrs, res.Conclusion.Status)
		}
	}
}

func TestDNSOkIsIncomplete(t *testing.T) {
	r := &fakeResolver{addrs: []string{"2606:4700:4700::1111", "93.184.215.14", "93.184.215.14"}}
	res := check(r, "https://Example.com/a")
	if len(r.asked) != 1 || r.asked[0] != "example.com." {
		t.Fatalf("resolver asked %v, want [example.com.] (absolute name)", r.asked)
	}
	if res.Conclusion.Status != ConclusionIncomplete || res.Conclusion.FailedStep != "" {
		t.Errorf("conclusion = %+v, want incomplete", res.Conclusion)
	}
	if got := statuses(res.Hops[0]); got != "dns=ok tcp=ok tls=ok http=not_implemented" {
		t.Errorf("steps = %s", got)
	}
	if got := res.Hops[0].Steps[0].Detail.Addresses; strings.Join(got, ",") != "93.184.215.14,2606:4700:4700::1111" {
		t.Errorf("addresses = %v, want IPv4 first, deduplicated", got)
	}
	if res.ObservedFrom != "server" || res.ObservedFromNote == "" {
		t.Errorf("vantage missing: %+v", res)
	}
	if res.URL != "https://example.com/a" {
		t.Errorf("url = %s", res.URL)
	}
	// http:// has no TLS step at all.
	res = check(r, "http://example.com/")
	if got := statuses(res.Hops[0]); got != "dns=ok tcp=ok tls=not_applicable http=not_implemented" {
		t.Errorf("http steps = %s", got)
	}
}

func TestInputRefusalResolvesNothing(t *testing.T) {
	for _, u := range []string{"http://127.0.0.1/", "http://localhost/", "http://example.com:8080/", "https://u:p@example.com/", ""} {
		r := &fakeResolver{addrs: []string{"93.184.215.14"}}
		res := check(r, u)
		if len(r.asked) != 0 {
			t.Errorf("%q: resolver asked %v, want nothing", u, r.asked)
		}
		if res.Conclusion.Status != ConclusionRefused || !strings.HasPrefix(res.Conclusion.Code, "input.") {
			t.Errorf("%q: conclusion = %+v", u, res.Conclusion)
		}
		if res.URL != "" {
			t.Errorf("%q: url echoed as %q", u, res.URL)
		}
		if got := statuses(res.Hops[0]); got != "dns=skipped tcp=skipped tls=skipped http=skipped" {
			t.Errorf("%q: steps = %s", u, got)
		}
	}
	// ⚠ Control: the same resolver is asked for a permitted URL.
	r := &fakeResolver{addrs: []string{"93.184.215.14"}}
	check(r, "http://example.com/")
	if len(r.asked) != 1 {
		t.Errorf("control: resolver asked %v, want one query", r.asked)
	}
}

func TestLiteralAddressSkipsDNS(t *testing.T) {
	r := &fakeResolver{}
	res := check(r, "https://93.184.215.14/")
	if len(r.asked) != 0 {
		t.Errorf("resolver asked %v", r.asked)
	}
	if got := statuses(res.Hops[0]); got != "dns=not_applicable tcp=ok tls=ok http=not_implemented" {
		t.Errorf("steps = %s", got)
	}
}

func TestEveryCodeHasWords(t *testing.T) {
	codes := []string{
		target.CodeMissing, target.CodeMalformed, target.CodeUnsupportedScheme, target.CodeUnsupportedPort,
		target.CodeCredentials, target.CodeLocalName, target.CodeRefusedAddress, target.CodeIDNNotImplemented,
		"dns.not_found", "dns.timeout", "dns.server_failure", "dns.refused_address", "server.busy",
		"tls.cert_expired", "tls.cert_untrusted", "tls.cert_name_mismatch", "tls.handshake_failed", "tls.timeout", "tls.not_tls",
		"tcp.refused", "tcp.timeout", "tcp.unreachable", "tcp.no_route_family", "tcp.refused_address", "tcp.failed",
	}
	for _, c := range codes {
		if Message(c) == "" {
			t.Errorf("no message for %s", c)
		}
	}
	for _, s := range []Status{StatusOK, StatusFailed, StatusRefused, StatusSkipped, StatusNotApplicable, StatusNotImplemented} {
		if StatusLabel(s) == "" {
			t.Errorf("no label for %s", s)
		}
	}
}

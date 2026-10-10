package diag

import (
	"context"
	"encoding/json"
	"net"
	"net/netip"
	"strings"
	"testing"

	"github.com/hidetzu/connect-doctor/internal/limits"
	"github.com/hidetzu/connect-doctor/internal/tlstest"
)

// mapResolver answers per name and records what it was asked.
type mapResolver struct {
	names map[string][]string
	asked []string
}

func (m *mapResolver) LookupNetIP(_ context.Context, _, host string) ([]netip.Addr, error) {
	host = strings.TrimSuffix(host, ".")
	m.asked = append(m.asked, host)
	var out []netip.Addr
	for _, a := range m.names[host] {
		out = append(out, netip.MustParseAddr(a))
	}
	if len(out) == 0 {
		return nil, &net.DNSError{Err: "no such host", IsNotFound: true}
	}
	return out, nil
}

// site is one fake server: every connection to addr answers with response.
type sites struct {
	responses map[string]string
	dialled   []string
}

func (s *sites) dial(_ context.Context, a netip.Addr, _ uint16) (net.Conn, error) {
	s.dialled = append(s.dialled, a.String())
	cert, _ := testCA.Valid("a.example", "b.example")
	return tlstest.Serve(&cert, []byte(s.responses[a.String()])), nil
}

func redirectTo(status, loc string) string {
	return "HTTP/1.1 " + status + "\r\nLocation: " + loc + "\r\nContent-Length: 0\r\n\r\n"
}

const ok200 = "HTTP/1.1 200 OK\r\nContent-Length: 0\r\n\r\n"

func runRedirect(t *testing.T, aResponse string, names map[string][]string) (Result, *mapResolver, *sites) {
	t.Helper()
	if names == nil {
		names = map[string][]string{}
	}
	names["a.example"] = []string{"93.184.215.1"}
	names["b.example"] = []string{"93.184.215.2"}
	r := &mapResolver{names: names}
	s := &sites{responses: map[string]string{"93.184.215.1": aResponse, "93.184.215.2": ok200}}
	return (&Checker{Resolver: r, Dial: s.dial}).Check(context.Background(), "https://a.example/"), r, s
}

func TestRedirectIsFollowedAsANewHop(t *testing.T) {
	res, r, _ := runRedirect(t, redirectTo("301 Moved Permanently", "https://b.example/x"), nil)
	if len(res.Hops) != 2 || res.Hops[1].URL != "https://b.example/x" {
		t.Fatalf("hops = %+v", res.Hops)
	}
	if strings.Join(r.asked, ",") != "a.example,b.example" {
		t.Errorf("resolved %v, want each hop resolved anew", r.asked)
	}
	if res.Conclusion.Status != ConclusionOK || !strings.Contains(res.Conclusion.Summary, "1回たどった") {
		t.Errorf("conclusion = %+v", res.Conclusion)
	}
	if res.URL != "https://a.example/" {
		t.Errorf("url = %s, want the URL that was asked about", res.URL)
	}
}

func TestRelativeLocation(t *testing.T) {
	res, _, _ := runRedirect(t, redirectTo("302 Found", "/next?x=1"), nil)
	if len(res.Hops) < 2 || res.Hops[1].URL != "https://a.example/next?x=1" {
		t.Errorf("hops = %+v", res.Hops)
	}
}

func TestRedirectToRefusedTargets(t *testing.T) {
	cases := []struct {
		name, location, hopCode string
	}{
		{"loopback literal", "http://127.0.0.1/", "input.refused_address"},
		{"metadata", "http://169.254.169.254/latest/", "input.refused_address"},
		{"local name", "http://localhost/", "input.local_name"},
		{"other port", "http://b.example:8080/", "input.unsupported_port"},
		{"credentials", "https://u:p@b.example/", "input.credentials"},
		{"scheme", "file:///etc/passwd", "input.unsupported_scheme"},
	}
	for _, c := range cases {
		res, r, s := runRedirect(t, redirectTo("302 Found", c.location), nil)
		if len(res.Hops) != 2 || res.Hops[1].Code != c.hopCode {
			t.Errorf("%s: hops = %+v", c.name, res.Hops)
			continue
		}
		if res.Conclusion.Status != ConclusionRefused || res.Conclusion.Code != "http.redirect_refused" || res.Conclusion.FailedStep != "" || !strings.Contains(res.Conclusion.Summary, "2番目") {
			t.Errorf("%s: conclusion = %+v", c.name, res.Conclusion)
		}
		// ⚠ Nothing was resolved or dialled for the refused hop.
		if len(r.asked) != 1 || len(s.dialled) != 1 {
			t.Errorf("%s: resolved %v, dialled %v — want only the first hop", c.name, r.asked, s.dialled)
		}
		if c.hopCode == "input.credentials" && strings.Contains(res.Hops[1].URL, "u:p") {
			t.Errorf("credentials echoed: %s", res.Hops[1].URL)
		}
	}
}

func TestRedirectToAPrivateName(t *testing.T) {
	res, _, s := runRedirect(t, redirectTo("307 Temporary Redirect", "https://inside.example/"), map[string][]string{"inside.example": {"10.1.2.3"}})
	if res.Conclusion.Code != "http.redirect_refused" || res.Hops[1].Steps[0].Code != "dns.refused_address" {
		t.Errorf("conclusion = %+v, hop 2 dns = %+v", res.Conclusion, res.Hops[1].Steps[0])
	}
	if len(s.dialled) != 1 {
		t.Errorf("dialled %v, want only the first hop", s.dialled)
	}
	if b, _ := json.Marshal(res); strings.Contains(string(b), "10.1.2.3") {
		t.Error("the refused address is shown")
	}
}

func TestRedirectLoopStops(t *testing.T) {
	res, _, _ := runRedirect(t, redirectTo("302 Found", "https://a.example/"), nil)
	if res.Conclusion.Code != "http.too_many_redirects" || res.Conclusion.Status != ConclusionFailed {
		t.Errorf("conclusion = %+v", res.Conclusion)
	}
	if len(res.Hops) != limits.RedirectHops+1 {
		t.Errorf("%d hops, want %d (the first plus %d redirects)", len(res.Hops), limits.RedirectHops+1, limits.RedirectHops)
	}
}

func TestNotEveryThreeHundredIsFollowed(t *testing.T) {
	for _, resp := range []string{
		"HTTP/1.1 304 Not Modified\r\nLocation: https://b.example/\r\n\r\n",
		"HTTP/1.1 301 Moved Permanently\r\nContent-Length: 0\r\n\r\n", // no Location
	} {
		res, _, _ := runRedirect(t, resp, nil)
		if len(res.Hops) != 1 || res.Conclusion.Status != ConclusionOK {
			t.Errorf("%q: %d hops, %+v", resp[:24], len(res.Hops), res.Conclusion)
		}
	}
}

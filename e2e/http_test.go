//go:build e2e

package e2e

import (
	"strings"
	"testing"
	"time"

	"github.com/hidetzu/connect-doctor/internal/diag"
	"github.com/hidetzu/connect-doctor/internal/limits"
)

// hidetzu/connect-doctor#4, AC 1 and 3.
func TestHTTPOutcomesThroughTheBinary(t *testing.T) {
	in := start(t)
	cases := []struct{ url, ladder, conclusion, code, summary string }{
		{"https://ok.test/", "dns=ok tcp=ok tls=ok http=ok", diag.ConclusionOK, "", "ステータス200"},
		{"http://ok.test/", "dns=ok tcp=ok tls=not_applicable http=ok", diag.ConclusionOK, "", "DNS・TCP・HTTP"},
		// ⚠ Owner decision (#4): any status is a successful connection.
		{"https://status503.test/", "dns=ok tcp=ok tls=ok http=ok", diag.ConclusionOK, "", "ステータス503"},
		{"https://bigheader.test/", "dns=ok tcp=ok tls=ok http=failed", diag.ConclusionFailed, "http.malformed_response", "HTTP"},
		{"http://httpclose.test/", "dns=ok tcp=ok tls=not_applicable http=failed", diag.ConclusionFailed, "http.no_response", "何も返さず"},
	}
	for _, c := range cases {
		_, res, body := in.api(t, c.url)
		if got := ladder(res); got != c.ladder {
			t.Errorf("%s: ladder %s, want %s", c.url, got, c.ladder)
		}
		if res.Conclusion.Status != c.conclusion || res.Conclusion.Code != c.code || !strings.Contains(res.Conclusion.Summary, c.summary) {
			t.Errorf("%s: conclusion %+v", c.url, res.Conclusion)
		}
		// AC 3.
		if strings.Contains(body, "not_implemented") {
			t.Errorf("%s: a step is still not_implemented", c.url)
		}
	}
}

func TestHTTPBodyIsCapped(t *testing.T) {
	in := start(t)
	if page := in.page(t, "/?url=https://bigbody.test/"); !strings.Contains(page, "200 OK") || !strings.Contains(page, "上限で打ち切り") {
		t.Error("the page does not show the status and the body cap")
	}
	_, res, _ := in.api(t, "https://bigbody.test/")
	d := res.Hops[0].Steps[3].Detail
	if res.Conclusion.Status != diag.ConclusionOK || d == nil || d.BodyBytesRead != limits.ResponseBodyBytes || !d.BodyTruncated {
		t.Errorf("bigbody.test: %+v / %+v, want ok with %d bytes read and truncated", res.Conclusion, d, limits.ResponseBodyBytes)
	}
}

// AC 2, observed at the server.
func TestHTTPRequestCarriesOurName(t *testing.T) {
	in := start(t)
	in.api(t, "https://ok.test/")
	r := lastRequest.Load()
	if r == nil {
		t.Fatal("the server saw no request")
	}
	if r.Host != "ok.test" || !strings.Contains(r.UserAgent(), "ConnectDoctor") || !strings.Contains(r.UserAgent(), "github.com/hidetzu/connect-doctor") || !r.Close {
		t.Errorf("request: host=%s ua=%q close=%v", r.Host, r.UserAgent(), r.Close)
	}
}

func TestHTTPTimeoutIsBounded(t *testing.T) {
	in := start(t)
	begin := time.Now()
	_, res, _ := in.api(t, "http://httpsilent.test/")
	took := time.Since(begin)
	if res.Conclusion.Code != "http.timeout" {
		t.Errorf("httpsilent.test: %+v", res.Conclusion)
	}
	// limits.HTTP is 8 s.
	if took < 7*time.Second || took > 12*time.Second {
		t.Errorf("httpsilent.test took %s, want about 8 s", took)
	}
}

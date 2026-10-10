//go:build e2e

package e2e

import (
	"strings"
	"testing"
	"time"

	"github.com/hidetzu/connect-doctor/internal/diag"
)

// hidetzu/connect-doctor#3, AC 1–3.
func TestTLSOutcomesThroughTheBinary(t *testing.T) {
	in := start(t)
	cases := []struct{ url, ladder, code string }{
		{"https://ok.test/", "dns=ok tcp=ok tls=ok http=ok", ""},
		{"https://" + addrListen + "/", "dns=not_applicable tcp=ok tls=ok http=ok", ""},
		{"https://expired.test/", "dns=ok tcp=ok tls=failed http=skipped", "tls.cert_expired"},
		{"https://untrusted.test/", "dns=ok tcp=ok tls=failed http=skipped", "tls.cert_untrusted"},
		{"https://mismatch.test/", "dns=ok tcp=ok tls=failed http=skipped", "tls.cert_name_mismatch"},
		{"https://plain.test/", "dns=ok tcp=ok tls=failed http=skipped", "tls.not_tls"},
		// AC 3: never skipped for http://, even when TCP succeeded.
		{"http://ok.test/", "dns=ok tcp=ok tls=not_applicable http=ok", ""},
	}
	for _, c := range cases {
		_, res, _ := in.api(t, c.url)
		if got := ladder(res); got != c.ladder {
			t.Errorf("%s: ladder %s, want %s", c.url, got, c.ladder)
		}
		if c.code == "" {
			continue
		}
		// AC 2: the conclusion names TLS and the cause in one sentence.
		if res.Conclusion.FailedStep != "tls" || res.Conclusion.Code != c.code || !strings.Contains(res.Conclusion.Summary, "TLSハンドシェイク") || res.Conclusion.Summary == "" {
			t.Errorf("%s: conclusion %+v, want tls/%s", c.url, res.Conclusion, c.code)
		}
	}
}

func TestTLSDetailThroughTheBinary(t *testing.T) {
	in := start(t)
	_, res, _ := in.api(t, "https://ok.test/")
	d := res.Hops[0].Steps[2].Detail
	if d == nil || d.ALPN != "http/1.1" || !strings.HasPrefix(d.TLSVersion, "TLS 1.") || d.Certificate == nil {
		t.Fatalf("tls detail = %+v", d)
	}
	_, res, _ = in.api(t, "https://expired.test/")
	c := res.Hops[0].Steps[2].Detail
	if c == nil || c.Certificate == nil || c.Certificate.NotAfter == "" {
		t.Fatalf("expired.test: the certificate's dates are missing: %+v", c)
	}
	// ⚠ The page shows the same dates the API returns, not just a sentence about them.
	page := in.page(t, "/?url=https://expired.test/")
	if !strings.Contains(page, c.Certificate.NotAfter) || !strings.Contains(page, "<code>expired.test</code>") {
		t.Errorf("the page does not show the expired certificate's name and not_after %s", c.Certificate.NotAfter)
	}
}

func TestTLSTimeoutIsBounded(t *testing.T) {
	in := start(t)
	begin := time.Now()
	_, res, _ := in.api(t, "https://silent.test/")
	took := time.Since(begin)
	if res.Conclusion.Code != "tls.timeout" || res.Hops[0].Steps[1].Status != diag.StatusOK {
		t.Errorf("silent.test: %+v", res.Conclusion)
	}
	// limits.TLS is 5 s.
	if took < 4*time.Second || took > 9*time.Second {
		t.Errorf("silent.test took %s, want about 5 s", took)
	}
}

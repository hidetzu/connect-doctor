//go:build e2e

package e2e

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/hidetzu/connect-doctor/internal/diag"
)

// hidetzu/connect-doctor#25 AC 1, 2, 4: every kind of result shows the
// headline card, one row per hop, the stopped step marked, and the chip.
func TestPageVisualLanguage(t *testing.T) {
	in := start(t, "-vantage", "Tokyo, Japan")
	cases := []struct {
		url, state, headline, stopped string
		rows                          int
	}{
		{"https://ok.test/", "ok", "すべての層を通りました（200）", "", 1},
		{"https://missing.test/", "fail", "DNS で止まっています", "failed", 1},
		{"https://closed.test/", "fail", "TCP で止まっています", "failed", 1},
		{"https://expired.test/", "fail", "TLS で止まっています", "failed", 1},
		{"https://status503.test/", "warn", "サーバは 503 を返しました", "ok warn", 1},
		{"http://127.0.0.1/", "neutral", "このURLは診断しませんでした", "", 1},
		{"https://toloop.test/", "neutral", "リダイレクト先で止めました", "", 2},
		{"https://redir.test/", "ok", "すべての層を通りました（200）", "", 2},
	}
	for _, c := range cases {
		page := in.page(t, "/diagnose?url="+url.QueryEscape(c.url))
		if !strings.Contains(page, `class="verdict `+c.state+`"`) {
			t.Errorf("%s: no %s headline card", c.url, c.state)
		}
		if !strings.Contains(page, c.headline) {
			t.Errorf("%s: headline %q missing", c.url, c.headline)
		}
		if n := strings.Count(page, `<ol class="rail"`); n != c.rows {
			t.Errorf("%s: %d rows, want %d", c.url, n, c.rows)
		}
		if c.stopped != "" && !strings.Contains(page, `<li class="`+c.stopped+`">`) {
			t.Errorf("%s: no step marked %q", c.url, c.stopped)
		}
		if !strings.Contains(page, diag.Tagline) || !strings.Contains(page, diag.PrivacyNote()) || strings.Contains(page, "ソースコード") {
			t.Errorf("%s: tagline or privacy note missing, or the source link is back", c.url)
		}
		if !strings.Contains(page, "Tokyo, Japan から観測") {
			t.Errorf("%s: the chip does not name the vantage", c.url)
		}
		if c.state != "neutral" && !strings.Contains(page, "ms</span>") {
			t.Errorf("%s: no durations", c.url)
		}
	}
}

// The landing page through the binary (hidetzu/connect-doctor#51): served at
// /, named after the vantage, its form submits to /diagnose, the API example
// uses -public-url; /?url= and /about are sent on with a 301.
func TestLandingThroughTheBinary(t *testing.T) {
	in := start(t, "-vantage", "Tokyo, Japan", "-public-url", "https://cd.example")
	resp, page := in.fetch(t, "/", nextClient())
	if resp.StatusCode != 200 || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/html") {
		t.Fatalf("/: %d %q", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	for _, want := range []string{diag.HeroLede, diag.LayersTitle, diag.Symptoms[0], "Tokyo, Japan から観測", `<form method="get" action="/diagnose">`,
		"https://cd.example/api/check?url=https://example.com", diag.Sections("")[0].Items[0], diag.PrivacyNote()} {
		if !strings.Contains(page, want) {
			t.Errorf("/ does not contain %q", want)
		}
	}
	// ⚠ A client that does not follow redirects, so the 301 itself is what is read.
	noFollow := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	for path, want := range map[string]string{"/?url=https%3A%2F%2Fok.test%2F&fresh=1": "/diagnose?url=https%3A%2F%2Fok.test%2F&fresh=1", "/about": "/"} {
		req, _ := http.NewRequest(http.MethodGet, in.base+path, nil)
		req.Header.Set("X-Forwarded-For", nextClient())
		resp, err := noFollow.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusMovedPermanently || resp.Header.Get("Location") != want {
			t.Errorf("%s: %d Location=%q, want 301 %q", path, resp.StatusCode, resp.Header.Get("Location"), want)
		}
	}
	// The redirect ran no check: the fake DNS server saw no query for it.
	before := in.dns.TotalQueries()
	req, _ := http.NewRequest(http.MethodGet, in.base+"/?url=https%3A%2F%2Fok.test%2F", nil)
	req.Header.Set("X-Forwarded-For", nextClient())
	if resp, err := noFollow.Do(req); err == nil {
		resp.Body.Close()
	}
	if after := in.dns.TotalQueries(); after != before {
		t.Errorf("the redirect caused %d DNS queries", after-before)
	}
}

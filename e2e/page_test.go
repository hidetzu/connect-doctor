//go:build e2e

package e2e

import (
	"net/url"
	"strings"
	"testing"
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
		page := in.page(t, "/?url="+url.QueryEscape(c.url))
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
		if !strings.Contains(page, "Tokyo, Japan から観測") {
			t.Errorf("%s: the chip does not name the vantage", c.url)
		}
		if c.state != "neutral" && !strings.Contains(page, "ms</span>") {
			t.Errorf("%s: no durations", c.url)
		}
	}
}

//go:build e2e

package e2e

import (
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/hidetzu/connect-doctor/internal/limits"
)

// hidetzu/connect-doctor#6 through the binary: one client gets the burst, then
// 429 with Retry-After; a forged leading X-Forwarded-For entry changes nothing;
// another client is unaffected.
func TestClientRateLimitThroughTheBinary(t *testing.T) {
	in := start(t)
	// A different (non-existent) hostname each time: the per-target limits are
	// not under test here, and nothing is dialled.
	path := func(i int) string { return "/api/check?url=https%3A%2F%2Fnx" + strconv.Itoa(i) + ".test%2F" }
	for i := 0; i < limits.ClientBurst; i++ {
		if resp, body := in.fetch(t, path(i), "203.0.113.50"); resp.StatusCode != http.StatusOK {
			t.Fatalf("check %d: %d %s", i+1, resp.StatusCode, body)
		}
	}
	resp, body := in.fetch(t, path(limits.ClientBurst), "10.1.1.1, 203.0.113.50") // forged first entry, same real client
	if resp.StatusCode != http.StatusTooManyRequests || !strings.Contains(body, "server.rate_limited") {
		t.Fatalf("over the burst: %d %s", resp.StatusCode, body)
	}
	if ra, err := strconv.Atoi(resp.Header.Get("Retry-After")); err != nil || ra < 1 || ra > int(limits.ClientRefill.Seconds()) {
		t.Errorf("Retry-After = %q", resp.Header.Get("Retry-After"))
	}
	// hidetzu/connect-doctor#43: the page names the visitor's limit and the wait.
	_, page := in.fetch(t, "/?url=nxpage.test", "203.0.113.50")
	if !strings.Contains(page, "あなたの診断回数が上限に達しました") || !strings.Contains(page, " 秒で、もう一度診断できます") {
		t.Error("the page does not say it is the visitor's limit and how long to wait")
	}
	// ⚠ Control: a different client is served.
	if resp, _ := in.fetch(t, path(9), "203.0.113.51"); resp.StatusCode != http.StatusOK {
		t.Errorf("another client: %d", resp.StatusCode)
	}
}

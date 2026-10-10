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
	path := "/api/check?url=https%3A%2F%2Fok.test%2F"
	for i := 0; i < limits.ClientBurst; i++ {
		if resp, body := in.fetch(t, path, "203.0.113.50"); resp.StatusCode != http.StatusOK {
			t.Fatalf("check %d: %d %s", i+1, resp.StatusCode, body)
		}
	}
	resp, body := in.fetch(t, path, "10.1.1.1, 203.0.113.50") // forged first entry, same real client
	if resp.StatusCode != http.StatusTooManyRequests || !strings.Contains(body, "server.rate_limited") {
		t.Fatalf("over the burst: %d %s", resp.StatusCode, body)
	}
	if ra, err := strconv.Atoi(resp.Header.Get("Retry-After")); err != nil || ra < 1 || ra > int(limits.ClientRefill.Seconds()) {
		t.Errorf("Retry-After = %q", resp.Header.Get("Retry-After"))
	}
	// ⚠ Control: a different client is served.
	if resp, _ := in.fetch(t, path, "203.0.113.51"); resp.StatusCode != http.StatusOK {
		t.Errorf("another client: %d", resp.StatusCode)
	}
}

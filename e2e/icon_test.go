//go:build e2e

package e2e

import (
	"strings"
	"testing"
)

// hidetzu/connect-doctor#29 through the binary.
func TestServiceIconThroughTheBinary(t *testing.T) {
	in := start(t)
	resp, body := in.fetch(t, "/favicon.svg", nextClient())
	if resp.StatusCode != 200 || resp.Header.Get("Content-Type") != "image/svg+xml" || !strings.HasPrefix(body, "<svg") {
		t.Fatalf("/favicon.svg: %d %q", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	resp, page := in.fetch(t, "/", nextClient())
	if !strings.Contains(page, `rel="icon" href="/favicon.svg"`) || !strings.Contains(page, `<span aria-hidden="true"><svg`) {
		t.Error("favicon link or header icon missing")
	}
	if !strings.Contains(resp.Header.Get("Content-Security-Policy"), "img-src 'self'") {
		t.Errorf("CSP %q", resp.Header.Get("Content-Security-Policy"))
	}
}

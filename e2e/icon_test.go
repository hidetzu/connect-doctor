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

// hidetzu/connect-doctor#44 through the binary, with -public-url.
func TestOpenGraphThroughTheBinary(t *testing.T) {
	in := start(t, "-public-url", "https://cd.example")
	_, page := in.fetch(t, "/", nextClient())
	if !strings.Contains(page, `<meta property="og:image" content="https://cd.example/og.png">`) || !strings.Contains(page, `name="description"`) {
		t.Error("Open Graph tags missing")
	}
	resp, _ := in.fetch(t, "/og.png", nextClient())
	if resp.StatusCode != 200 || resp.Header.Get("Content-Type") != "image/png" {
		t.Errorf("/og.png: %d %q", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
}

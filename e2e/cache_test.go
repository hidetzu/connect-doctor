//go:build e2e

package e2e

import (
	"net/url"
	"strings"
	"testing"
)

// hidetzu/connect-doctor#28 through the binary: the second check of a URL is
// answered from the cache without connecting; 再診断 connects again.
func TestCacheThroughTheBinary(t *testing.T) {
	in := start(t)
	before := cachedAccepts.Load()
	_, first, _ := in.api(t, "https://cached.test/")
	settle()
	if n := cachedAccepts.Load() - before; n != 1 || first.Cached {
		t.Fatalf("first check: %d connections, cached=%v", n, first.Cached)
	}
	_, second, _ := in.api(t, "HTTPS://Cached.TEST/") // same once normalised
	settle()
	if n := cachedAccepts.Load() - before; n != 1 || !second.Cached || !second.CheckedAt.Equal(first.CheckedAt) {
		t.Errorf("repeat: %d connections, cached=%v, checked_at %s vs %s", n, second.Cached, second.CheckedAt, first.CheckedAt)
	}
	page := in.page(t, "/?url="+url.QueryEscape("https://cached.test/"))
	if !strings.Contains(page, "秒前の診断結果です") || !strings.Contains(page, "fresh=1") {
		t.Error("the page does not show the age line and the 再診断 link")
	}
	in.apiFresh(t, "https://cached.test/")
	settle()
	if n := cachedAccepts.Load() - before; n != 2 {
		t.Errorf("再診断: %d connections in total, want 2", n)
	}
}

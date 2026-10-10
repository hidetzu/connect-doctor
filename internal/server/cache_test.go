package server

import (
	"fmt"
	"testing"
	"time"

	"github.com/hidetzu/connect-doctor/internal/diag"
	"github.com/hidetzu/connect-doctor/internal/limits"
)

type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time { return c.t }

func TestCacheTTL(t *testing.T) {
	c := &fakeClock{t: time.Unix(0, 0)}
	rc := newResultCache(c.now)
	k := cacheKey("https://example.com/")
	rc.put(k, diag.Result{URL: "https://example.com/"})
	if res, ok := rc.get(k); !ok || !res.Cached {
		t.Fatalf("fresh entry: ok=%v cached=%v", ok, res.Cached)
	}
	c.t = c.t.Add(limits.CacheTTL)
	if _, ok := rc.get(k); ok {
		t.Error("entry served at the TTL")
	}
}

func TestCacheKeyIsTheNormalisedURL(t *testing.T) {
	if cacheKey("HTTPS://Example.COM/a#frag") != cacheKey("https://example.com/a") {
		t.Error("URLs target.Parse normalises to one do not share a key")
	}
	for _, other := range []string{"https://example.com/b", "http://example.com/a", "https://example.com/a?x=1"} {
		if cacheKey(other) == cacheKey("https://example.com/a") {
			t.Errorf("%s shares the key", other)
		}
	}
	if cacheKey("http://127.0.0.1/") != "" {
		t.Error("a refused input has a cache key")
	}
}

func TestWhatIsNotCached(t *testing.T) {
	rc := newResultCache(nil)
	for _, code := range []string{diag.CodeTargetLimited, "input.local_name"} {
		k := cacheKey("https://" + code[:5] + ".example/")
		rc.put(k, diag.Result{Conclusion: diag.Conclusion{Code: code}})
		if _, ok := rc.get(k); ok {
			t.Errorf("%s was cached", code)
		}
	}
	k := cacheKey("https://down.example/")
	rc.put(k, diag.Result{Conclusion: diag.Conclusion{Status: "failed", Code: "tcp.timeout"}})
	if _, ok := rc.get(k); !ok {
		t.Error("a failure was not cached (owner decision: failures are cached too)")
	}
}

func TestCacheIsBounded(t *testing.T) {
	rc := newResultCache(nil)
	for i := 0; i < limits.CacheEntries+50; i++ {
		rc.put(cacheKey(fmt.Sprintf("https://h%d.example/", i)), diag.Result{})
	}
	if n := rc.len(); n > limits.CacheEntries {
		t.Errorf("%d entries, bound %d", n, limits.CacheEntries)
	}
}

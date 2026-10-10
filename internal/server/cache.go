package server

import (
	"container/list"
	"sync"
	"time"

	"github.com/hidetzu/connect-doctor/internal/diag"
	"github.com/hidetzu/connect-doctor/internal/limits"
	"github.com/hidetzu/connect-doctor/internal/target"
)

// resultCache answers repeated checks of the same normalised URL for
// limits.CacheTTL without connecting again (hidetzu/connect-doctor#28).
//
// ⚠ One cache of the whole result, not one per layer: two caches answering
// one question drift apart (docs/adr/0010). ⚠ Bounded (limits.CacheEntries).
type resultCache struct {
	mu  sync.Mutex
	now func() time.Time
	m   map[string]*list.Element
	lru *list.List
}

type cacheEntry struct {
	key string
	res diag.Result
	at  time.Time
}

func newResultCache(now func() time.Time) *resultCache {
	if now == nil {
		now = time.Now
	}
	return &resultCache{now: now, m: map[string]*list.Element{}, lru: list.New()}
}

// cacheKey is the normalised URL target.Parse produces, or "" for input that
// is refused before any check (nothing to cache).
func cacheKey(raw string) string {
	t, err := target.Parse(raw)
	if err != nil {
		return ""
	}
	return t.URL
}

// cacheable reports whether a result may answer later requests: a target
// limit's refusal is not a diagnosis, and an input refusal never connected.
func cacheable(res diag.Result) bool {
	c := res.Conclusion.Code
	return c != diag.CodeTargetLimited && (len(c) < 6 || c[:6] != "input.")
}

// get returns a fresh entry, marked cached.
func (c *resultCache) get(key string) (diag.Result, bool) {
	if key == "" {
		return diag.Result{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.m[key]
	if !ok {
		return diag.Result{}, false
	}
	ent := e.Value.(*cacheEntry)
	if c.now().Sub(ent.at) >= limits.CacheTTL {
		c.lru.Remove(e)
		delete(c.m, key)
		return diag.Result{}, false
	}
	res := ent.res
	res.Cached = true
	return res, true
}

func (c *resultCache) put(key string, res diag.Result) {
	if key == "" || !cacheable(res) {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.m[key]; ok {
		c.lru.Remove(e)
		delete(c.m, key)
	}
	for len(c.m) >= limits.CacheEntries {
		old := c.lru.Back()
		c.lru.Remove(old)
		delete(c.m, old.Value.(*cacheEntry).key)
	}
	res.Cached = false
	c.m[key] = c.lru.PushFront(&cacheEntry{key: key, res: res, at: c.now()})
}

func (c *resultCache) len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.m)
}

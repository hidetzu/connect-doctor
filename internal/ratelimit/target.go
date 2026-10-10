package ratelimit

import (
	"container/list"
	"math"
	"net/netip"
	"sync"
	"time"

	"github.com/hidetzu/connect-doctor/internal/limits"
)

// buckets is a bounded table of token buckets.
type buckets struct {
	burst  float64
	refill time.Duration
	m      map[string]*list.Element
	lru    *list.List
}

type bucket struct {
	key     string
	tokens  float64
	updated time.Time
}

func newBuckets(burst int, refill time.Duration) *buckets {
	return &buckets{burst: float64(burst), refill: refill, m: map[string]*list.Element{}, lru: list.New()}
}

// take spends one token for key, or says how long until one is available.
func (b *buckets) take(key string, now time.Time) (bool, time.Duration) {
	var k *bucket
	if e, ok := b.m[key]; ok {
		b.lru.MoveToFront(e)
		k = e.Value.(*bucket)
		k.tokens = math.Min(b.burst, k.tokens+now.Sub(k.updated).Seconds()/b.refill.Seconds())
		k.updated = now
	} else {
		for len(b.m) >= limits.TrackedTargets {
			old := b.lru.Back()
			b.lru.Remove(old)
			delete(b.m, old.Value.(*bucket).key)
		}
		k = &bucket{key: key, tokens: b.burst, updated: now}
		b.m[key] = b.lru.PushFront(k)
	}
	if k.tokens < 1 {
		return false, max(time.Duration((1-k.tokens)*float64(b.refill)), time.Second)
	}
	k.tokens--
	return true, 0
}

// Targets limits how often ConnectDoctor connects to one hostname and to one
// destination, whoever asks (hidetzu/connect-doctor#27, docs/adr/0010).
// ⚠ It protects the sites being checked; the per-client limits protect us.
type Targets struct {
	mu   sync.Mutex
	now  func() time.Time
	host *buckets
	dest *buckets
}

// NewTargets returns the target limiter. now may be nil (time.Now).
func NewTargets(now func() time.Time) *Targets {
	if now == nil {
		now = time.Now
	}
	return &Targets{now: now,
		host: newBuckets(limits.TargetHostBurst, limits.TargetHostRefill),
		dest: newBuckets(limits.TargetDestBurst, limits.TargetDestRefill)}
}

// AllowHost spends one unit of the hostname's budget; call it when a hop
// starts, for every hop.
func (t *Targets) AllowHost(host string) (bool, time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.host.take(host, t.now())
}

// AllowDial spends one unit of the destination's budget; call it for every
// TCP attempt.
func (t *Targets) AllowDial(dest netip.AddrPort) (bool, time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.dest.take(dest.String(), t.now())
}

// Tracked returns how many hostnames and destinations are held.
func (t *Targets) Tracked() (int, int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.host.m), len(t.dest.m)
}

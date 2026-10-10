// Package ratelimit decides whether a client may start a check now
// (hidetzu/connect-doctor#6, docs/adr/0010).
//
// ⚠ Everything lives in this process's memory and is bounded
// (limits.TrackedClients). A replaced instance starts from zero, so the
// daily limit is best effort; that is stated wherever the limit is described.
package ratelimit

import (
	"container/list"
	"math"
	"net/netip"
	"sync"
	"time"

	"github.com/hidetzu/connect-doctor/internal/limits"
)

// Reason names which limit refused a request.
type Reason string

const (
	ReasonNone       Reason = ""
	ReasonBurst      Reason = "burst"      // the token bucket is empty
	ReasonHour       Reason = "hour"       // limits.ClientPerHour reached
	ReasonDay        Reason = "day"        // limits.ClientPerDay reached
	ReasonConcurrent Reason = "concurrent" // limits.ClientConcurrent checks already running
	ReasonGlobal     Reason = "global"     // limits.GlobalPerHour reached for everyone
)

// Decision is the answer for one request.
type Decision struct {
	Reason     Reason
	RetryAfter time.Duration // set when refused
	release    func()
}

// Allowed reports whether the check may start.
func (d Decision) Allowed() bool { return d.Reason == ReasonNone }

// Done must be called when an allowed check finishes (it frees the
// concurrency slot). Calling it on a refusal does nothing.
func (d Decision) Done() {
	if d.release != nil {
		d.release()
	}
}

type client struct {
	key      string
	tokens   float64
	updated  time.Time
	hourFrom time.Time
	hourN    int
	dayFrom  time.Time
	dayN     int
	running  int
	elem     *list.Element
}

// Limiter holds per-client state and the global breaker.
type Limiter struct {
	mu      sync.Mutex
	now     func() time.Time
	clients map[string]*client
	lru     *list.List // front = most recently seen
	gFrom   time.Time
	gN      int
	refused map[Reason]int64
}

// New returns a Limiter. now may be nil (time.Now).
func New(now func() time.Time) *Limiter {
	if now == nil {
		now = time.Now
	}
	return &Limiter{now: now, clients: map[string]*client{}, lru: list.New(), refused: map[Reason]int64{}}
}

// Key turns a client address into the unit that is limited: an IPv4
// address, or an IPv6 /64 (so changing the interface identifier does not
// buy a fresh budget).
func Key(a netip.Addr) string {
	a = a.Unmap()
	if a.Is6() {
		p, _ := a.Prefix(64)
		return p.String()
	}
	return a.String()
}

// LogPrefix is what a log line may say about a client: IPv4 /24, IPv6 /48.
// ⚠ Never the full address (docs/adr/0010).
func LogPrefix(a netip.Addr) string {
	a = a.Unmap()
	bits := 24
	if a.Is6() {
		bits = 48
	}
	p, _ := a.Prefix(bits)
	return p.String()
}

// Refused returns how many requests each limit has refused so far.
func (l *Limiter) Refused() map[Reason]int64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make(map[Reason]int64, len(l.refused))
	for k, v := range l.refused {
		out[k] = v
	}
	return out
}

// Take decides for one request from the client with key.
func (l *Limiter) Take(key string) Decision {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()

	c := l.get(key, now)
	// Refill the bucket.
	c.tokens = math.Min(limits.ClientBurst, c.tokens+now.Sub(c.updated).Seconds()/limits.ClientRefill.Seconds())
	c.updated = now
	if now.Sub(c.hourFrom) >= time.Hour {
		c.hourFrom, c.hourN = now, 0
	}
	if now.Sub(c.dayFrom) >= 24*time.Hour {
		c.dayFrom, c.dayN = now, 0
	}
	if now.Sub(l.gFrom) >= time.Hour {
		l.gFrom, l.gN = now, 0
	}

	refuse := func(r Reason, after time.Duration) Decision {
		l.refused[r]++
		return Decision{Reason: r, RetryAfter: max(after, time.Second)}
	}
	switch {
	case c.running >= limits.ClientConcurrent:
		return refuse(ReasonConcurrent, time.Second)
	case c.dayN >= limits.ClientPerDay:
		return refuse(ReasonDay, c.dayFrom.Add(24*time.Hour).Sub(now))
	case c.hourN >= limits.ClientPerHour:
		return refuse(ReasonHour, c.hourFrom.Add(time.Hour).Sub(now))
	case c.tokens < 1:
		return refuse(ReasonBurst, time.Duration((1-c.tokens)*float64(limits.ClientRefill)))
	case l.gN >= limits.GlobalPerHour:
		return refuse(ReasonGlobal, l.gFrom.Add(time.Hour).Sub(now))
	}
	c.tokens--
	c.hourN++
	c.dayN++
	c.running++
	l.gN++
	released := false
	return Decision{release: func() {
		l.mu.Lock()
		defer l.mu.Unlock()
		if !released {
			released = true
			c.running--
		}
	}}
}

// get returns the client's state, creating it with a full bucket, and
// forgets the least recently seen client when the table is full.
func (l *Limiter) get(key string, now time.Time) *client {
	if c, ok := l.clients[key]; ok {
		l.lru.MoveToFront(c.elem)
		return c
	}
	for len(l.clients) >= limits.TrackedClients {
		old := l.lru.Back()
		oc := old.Value.(*client)
		if oc.running > 0 {
			// ⚠ Never forget a client with a check in flight; its slot must be freed.
			l.lru.MoveToFront(old)
			if l.lru.Back() == old {
				break
			}
			continue
		}
		l.lru.Remove(old)
		delete(l.clients, oc.key)
	}
	c := &client{key: key, tokens: limits.ClientBurst, updated: now, hourFrom: now, dayFrom: now}
	c.elem = l.lru.PushFront(c)
	l.clients[key] = c
	return c
}

// Tracked returns how many clients are held (for tests of the bound).
func (l *Limiter) Tracked() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.clients)
}

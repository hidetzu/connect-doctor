package ratelimit

import (
	"fmt"
	"net/netip"
	"testing"
	"time"

	"github.com/hidetzu/connect-doctor/internal/limits"
)

type clock struct{ t time.Time }

func (c *clock) now() time.Time            { return c.t }
func (c *clock) add(d time.Duration)       { c.t = c.t.Add(d) }
func take(l *Limiter, key string) Decision { d := l.Take(key); d.Done(); return d }

func TestBurstThenRefill(t *testing.T) {
	c := &clock{t: time.Unix(0, 0)}
	l := New(c.now)
	for i := 0; i < limits.ClientBurst; i++ {
		if d := take(l, "a"); !d.Allowed() {
			t.Fatalf("check %d refused: %s", i+1, d.Reason)
		}
	}
	d := take(l, "a")
	if d.Reason != ReasonBurst || d.RetryAfter <= 0 || d.RetryAfter > limits.ClientRefill {
		t.Fatalf("after the burst: %+v, want burst with Retry-After ≤ %s", d, limits.ClientRefill)
	}
	if d := take(l, "b"); !d.Allowed() {
		t.Errorf("another client was refused: %s", d.Reason)
	}
	c.add(limits.ClientRefill)
	if d := take(l, "a"); !d.Allowed() {
		t.Errorf("after one refill interval: %s", d.Reason)
	}
	if d := take(l, "a"); d.Allowed() {
		t.Error("two checks allowed after one refill interval")
	}
}

func TestHourAndDay(t *testing.T) {
	c := &clock{t: time.Unix(0, 0)}
	l := New(c.now)
	n := 0
	for ; n < 1000; n++ {
		c.add(limits.ClientRefill) // never short of tokens
		if d := take(l, "a"); !d.Allowed() {
			if d.Reason != ReasonHour {
				t.Fatalf("refused for %s, want hour", d.Reason)
			}
			break
		}
	}
	if n != limits.ClientPerHour {
		t.Fatalf("allowed %d in the hour, want %d", n, limits.ClientPerHour)
	}
	allowed := n
	for h := 0; h < 30 && allowed < 1000; h++ {
		c.add(time.Hour)
		for i := 0; i < limits.ClientPerHour; i++ {
			c.add(limits.ClientRefill)
			d := take(l, "a")
			if !d.Allowed() {
				if d.Reason == ReasonDay {
					goto day
				}
				break
			}
			allowed++
		}
	}
day:
	if allowed != limits.ClientPerDay {
		t.Errorf("allowed %d within a day, want %d", allowed, limits.ClientPerDay)
	}
}

func TestOneAtATime(t *testing.T) {
	l := New(nil)
	d := l.Take("a")
	if !d.Allowed() {
		t.Fatal(d.Reason)
	}
	if d2 := l.Take("a"); d2.Reason != ReasonConcurrent {
		t.Errorf("second concurrent check: %s, want concurrent", d2.Reason)
	}
	d.Done()
	d.Done() // idempotent
	if d3 := take(l, "a"); !d3.Allowed() {
		t.Errorf("after Done: %s", d3.Reason)
	}
}

func TestGlobalBreaker(t *testing.T) {
	c := &clock{t: time.Unix(0, 0)}
	l := New(c.now)
	for i := 0; i < limits.GlobalPerHour; i++ {
		if d := take(l, fmt.Sprintf("c%d", i)); !d.Allowed() {
			t.Fatalf("client %d refused: %s", i, d.Reason)
		}
	}
	if d := take(l, "fresh"); d.Reason != ReasonGlobal {
		t.Errorf("a fresh client past the breaker: %s, want global", d.Reason)
	}
	c.add(time.Hour)
	if d := take(l, "fresh2"); !d.Allowed() {
		t.Errorf("after an hour: %s", d.Reason)
	}
	if l.Refused()[ReasonGlobal] != 1 {
		t.Errorf("refusals counted: %v", l.Refused())
	}
}

func TestKeysAndLogPrefixes(t *testing.T) {
	a := netip.MustParseAddr
	if Key(a("2001:db8:1:2:aaaa::1")) != Key(a("2001:db8:1:2:bbbb::9")) {
		t.Error("two addresses in one IPv6 /64 have different keys")
	}
	if Key(a("2001:db8:1:2::1")) == Key(a("2001:db8:1:3::1")) {
		t.Error("two IPv6 /64s share a key")
	}
	if Key(a("203.0.113.7")) == Key(a("203.0.113.8")) {
		t.Error("two IPv4 addresses share a key")
	}
	if Key(a("::ffff:203.0.113.7")) != Key(a("203.0.113.7")) {
		t.Error("IPv4-mapped IPv6 is not keyed as IPv4")
	}
	if got := LogPrefix(a("203.0.113.7")); got != "203.0.113.0/24" {
		t.Errorf("IPv4 log prefix %s", got)
	}
	if got := LogPrefix(a("2001:db8:1:2::1")); got != "2001:db8:1::/48" {
		t.Errorf("IPv6 log prefix %s", got)
	}
}

func TestMemoryIsBounded(t *testing.T) {
	l := New(nil)
	for i := 0; i < limits.TrackedClients+500; i++ {
		take(l, fmt.Sprintf("c%d", i))
	}
	if n := l.Tracked(); n > limits.TrackedClients {
		t.Errorf("tracking %d clients, bound is %d", n, limits.TrackedClients)
	}
}

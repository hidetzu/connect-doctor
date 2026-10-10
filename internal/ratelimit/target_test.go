package ratelimit

import (
	"fmt"
	"net/netip"
	"testing"

	"github.com/hidetzu/connect-doctor/internal/limits"
)

func TestHostBudget(t *testing.T) {
	c := &clock{}
	tg := NewTargets(c.now)
	for i := 0; i < limits.TargetHostBurst; i++ {
		if ok, _ := tg.AllowHost("victim.example"); !ok {
			t.Fatalf("hop %d refused", i+1)
		}
	}
	ok, after := tg.AllowHost("victim.example")
	if ok || after <= 0 || after > limits.TargetHostRefill {
		t.Fatalf("after the burst: ok=%v after=%s", ok, after)
	}
	if ok, _ := tg.AllowHost("other.example"); !ok {
		t.Error("another hostname was refused")
	}
	c.add(limits.TargetHostRefill)
	if ok, _ := tg.AllowHost("victim.example"); !ok {
		t.Error("not refilled after one interval")
	}
}

func TestDestinationBudgetIsSharedAcrossHostnames(t *testing.T) {
	c := &clock{}
	tg := NewTargets(c.now)
	dest := netip.MustParseAddrPort("203.0.113.10:443")
	for i := 0; i < limits.TargetDestBurst; i++ {
		// Each attempt comes from a different hostname: the hostname budget is not the one spent.
		tg.AllowHost(fmt.Sprintf("n%d.attacker.example", i))
		if ok, _ := tg.AllowDial(dest); !ok {
			t.Fatalf("attempt %d refused", i+1)
		}
	}
	if ok, _ := tg.AllowDial(dest); ok {
		t.Error("the destination budget was not shared")
	}
	if ok, _ := tg.AllowDial(netip.MustParseAddrPort("203.0.113.10:80")); !ok {
		t.Error("another port shares the budget")
	}
}

func TestTargetMemoryIsBounded(t *testing.T) {
	tg := NewTargets(nil)
	for i := 0; i < limits.TrackedTargets+100; i++ {
		tg.AllowHost(fmt.Sprintf("h%d.example", i))
		tg.AllowDial(netip.AddrPortFrom(netip.AddrFrom4([4]byte{198, 51, byte(i >> 8), byte(i)}), 443))
	}
	if h, d := tg.Tracked(); h > limits.TrackedTargets || d > limits.TrackedTargets {
		t.Errorf("tracking %d hostnames, %d destinations; bound %d", h, d, limits.TrackedTargets)
	}
}

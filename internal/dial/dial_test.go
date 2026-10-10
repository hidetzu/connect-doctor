package dial

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"
)

func TestControlRefusesWhatPolicyRefuses(t *testing.T) {
	for _, a := range []string{"127.0.0.1:443", "[::1]:80", "10.0.0.1:443", "169.254.169.254:80", "[::ffff:127.0.0.1]:443", "nonsense"} {
		if err := Control("tcp", a, nil); !errors.Is(err, ErrRefused) {
			t.Errorf("Control(%s) = %v, want ErrRefused", a, err)
		}
	}
	if err := Control("tcp", "93.184.215.14:443", nil); err != nil {
		t.Errorf("Control(public) = %v, want nil", err)
	}
}

// ⚠ The refusal is proven by a listener the dial would have reached, and did
// not (.claude/rules/security.md § 6). The paired control is in the final
// gate, where a permitted address can carry a listener.
func TestTCPToLoopbackReachesNoListener(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	var accepted atomic.Int32
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			accepted.Add(1)
			c.Close()
		}
	}()
	ap := netip.MustParseAddrPort(ln.Addr().String())

	// Control check: an ordinary dialer reaches this listener.
	var d net.Dialer
	c, err := d.Dial("tcp", ap.String())
	if err != nil {
		t.Fatalf("control dial failed, so the refusal below would prove nothing: %v", err)
	}
	c.Close()
	deadline := time.Now().Add(2 * time.Second)
	for accepted.Load() != 1 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if accepted.Load() != 1 {
		t.Fatalf("control: listener accepted %d, want 1", accepted.Load())
	}

	_, err = TCP(context.Background(), ap.Addr(), ap.Port())
	if !errors.Is(err, ErrRefused) {
		t.Fatalf("TCP(127.0.0.1) err = %v, want ErrRefused", err)
	}
	time.Sleep(100 * time.Millisecond)
	if n := accepted.Load(); n != 1 {
		t.Errorf("listener accepted %d after the refused dial, want still 1", n)
	}
}

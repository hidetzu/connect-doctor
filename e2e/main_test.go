//go:build e2e

package e2e

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// Addresses the harness gives meaning to, inside the namespace. ⚠ They are
// globally-routable so the policy permits them; documentation ranges would be
// refused and prove nothing. ⚠ No packet can leave: the namespace has no route
// out (checked before anything runs).
const (
	addrListen      = "93.184.215.14"        // on lo, listeners on :80 and :443
	addrClosed      = "93.184.215.15"        // on lo, nothing listening -> RST
	addrDrop        = "8.8.4.4"              // routed into a dummy interface -> silence
	addrUnreachable = "9.9.9.9"              // unreachable route -> EHOSTUNREACH
	addrV6          = "2606:4700:4700::1111" // no IPv6 route at all -> ENETUNREACH
)

var (
	publicAccepts   atomic.Int32 // connections accepted on addrListen
	loopbackAccepts atomic.Int32 // connections accepted on 127.0.0.1 — must stay 0
)

// TestMain refuses to run anywhere but an empty network namespace, then
// builds the world the final gate needs.
//
// ⚠ An exercise must not change the world (.claude/rules/verification.md):
// the binary under test makes real TCP connections, so the only place it may
// run is a namespace with no way out. scripts/verify.sh final provides one
// (unshare -rn).
func TestMain(m *testing.M) {
	if err := setup(); err != nil {
		fmt.Fprintf(os.Stderr, "e2e: %v\n", err)
		fmt.Fprintln(os.Stderr, "e2e: ⚠ run the final gate with scripts/verify.sh final — it runs these tests inside an empty network namespace")
		os.Exit(2)
	}
	os.Exit(m.Run())
}

func ip(args ...string) (string, error) {
	out, err := exec.Command("ip", args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("ip %s: %v: %s", strings.Join(args, " "), err, out)
	}
	return string(out), nil
}

func setup() error {
	links, err := ip("-o", "link")
	if err != nil {
		return err
	}
	for _, l := range strings.Split(strings.TrimSpace(links), "\n") {
		if f := strings.Fields(l); len(f) > 1 && strings.TrimSuffix(f[1], ":") != "lo" {
			return fmt.Errorf("refusing to run: interface %s exists, so this is not an empty network namespace", f[1])
		}
	}
	for _, fam := range []string{"-4", "-6"} {
		if r, _ := ip(fam, "route", "show", "default"); strings.TrimSpace(r) != "" {
			return fmt.Errorf("refusing to run: a default route exists (%s)", strings.TrimSpace(r))
		}
	}
	for _, c := range [][]string{
		{"link", "set", "lo", "up"},
		{"addr", "add", addrListen + "/32", "dev", "lo"},
		{"addr", "add", addrClosed + "/32", "dev", "lo"},
		{"link", "add", "d0", "type", "dummy"},
		{"link", "set", "d0", "up"},
		{"route", "add", addrDrop + "/32", "dev", "d0"},
		{"route", "add", "unreachable", addrUnreachable + "/32"},
	} {
		if _, err := ip(c...); err != nil {
			return err
		}
	}
	for _, a := range []string{addrListen + ":80", addrListen + ":443"} {
		if err := listen(a, &publicAccepts); err != nil {
			return err
		}
	}
	for _, a := range []string{"127.0.0.1:80", "127.0.0.1:443"} {
		if err := listen(a, &loopbackAccepts); err != nil {
			return err
		}
	}
	return nil
}

// listen accepts and counts connections, holding each briefly so the
// handshake is complete before it closes.
func listen(addr string, n *atomic.Int32) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			n.Add(1)
			go func() { time.Sleep(200 * time.Millisecond); c.Close() }()
		}
	}()
	return nil
}

func TestHarnessIsIsolated(t *testing.T) {
	// ⚠ Control for the whole gate: an outward address is unreachable here.
	c, err := net.DialTimeout("tcp", "1.1.1.1:443", time.Second)
	if err == nil {
		c.Close()
		t.Fatal("1.1.1.1:443 connected: the namespace has a way out")
	}
}

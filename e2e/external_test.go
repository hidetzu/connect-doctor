//go:build external

// The external tier: the other end is a real recursive resolver, real zones
// and real servers (a TCP handshake, no bytes sent) we did not write (.claude/rules/verification.md).
//
// ⚠ It never asserts what the other side will return. It records what our
// DNS step returned beside what this machine's own resolver (getent) returned
// for the same name, and fails only when our step could not run at all.
// ⚠ It depends on third parties' uptime, so it never runs on a PR.
package e2e

import (
	"context"
	"net"
	"os/exec"
	"slices"
	"strings"
	"testing"

	"github.com/hidetzu/connect-doctor/internal/diag"
	"github.com/hidetzu/connect-doctor/internal/dial"
)

func TestExternalDNSAgainstSystemResolver(t *testing.T) {
	if _, err := exec.LookPath("getent"); err != nil {
		t.Skip("getent not available: the cross-check cannot run here (NOT-VERIFIED, not passed)")
	}
	c := &diag.Checker{Resolver: &net.Resolver{PreferGo: true}, Dial: dial.TCP}
	for _, name := range []string{"example.com", "www.cloudflare.com", "github.com", "does-not-exist.example.com"} {
		res := c.Check(context.Background(), "https://"+name+"/")
		dns := res.Hops[0].Steps[0]
		var ours []string
		if dns.Detail != nil {
			ours = slices.Clone(dns.Detail.Addresses)
		}
		out, _ := exec.Command("getent", "ahosts", name).Output()
		var theirs []string
		for _, line := range strings.Split(string(out), "\n") {
			if f := strings.Fields(line); len(f) > 0 && !slices.Contains(theirs, f[0]) {
				theirs = append(theirs, f[0])
			}
		}
		slices.Sort(ours)
		slices.Sort(theirs)
		tcp := res.Hops[0].Steps[1]
		t.Logf("%-28s ours=%s/%s %v | getent %v | same set: %v | tcp=%s/%s", name, dns.Status, dns.Code, ours, theirs, slices.Equal(ours, theirs), tcp.Status, tcp.Code)
		if dns.Status == "" {
			t.Errorf("%s: our DNS step produced no status", name)
		}
	}
}

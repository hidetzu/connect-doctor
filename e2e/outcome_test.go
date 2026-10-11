//go:build e2e

package e2e

import (
	"strings"
	"testing"
)

// hidetzu/connect-doctor#55 through the binary: the outcome line for a fresh
// check, a cached one, a TLS failure and a two-hop redirect, read from the
// binary's own stderr, with no hostname in any of them.
func TestOutcomeLineThroughTheBinary(t *testing.T) {
	in := start(t)
	for _, u := range []string{"https://ok.test/", "https://ok.test/", "https://expired.test/", "https://redir.test/"} {
		in.api(t, u)
	}
	logs := in.stderr.String()
	for _, want := range []string{
		"check code=ok stop=none v4=ok v6=untried cached=false hops=1",
		"check code=ok stop=none v4=ok v6=untried cached=true hops=1",
		"check code=tls.cert_expired stop=tls v4=ok v6=none cached=false hops=1",
		"check code=ok stop=none v4=ok v6=untried cached=false hops=2",
	} {
		if !strings.Contains(logs, want) {
			t.Errorf("the binary's log lacks %q\n%s", want, logs)
		}
	}
	for _, l := range strings.Split(logs, "\n") {
		if strings.Contains(l, "check ") && (strings.Contains(l, ".test") || strings.Contains(l, "93.184.") || strings.Contains(l, "http")) {
			t.Errorf("a check line names the target: %s", l)
		}
	}
}

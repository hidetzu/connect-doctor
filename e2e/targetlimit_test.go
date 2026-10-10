//go:build e2e

package e2e

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/hidetzu/connect-doctor/internal/diag"
	"github.com/hidetzu/connect-doctor/internal/limits"
)

func settle() { time.Sleep(200 * time.Millisecond) }

// These use fresh=1 (再診断) so the cache (hidetzu/connect-doctor#28) does not
// answer: the target limits must hold for fresh checks.

// hidetzu/connect-doctor#27 AC 1: past the hostname budget, whoever asks,
// nothing more reaches the target.
func TestHostnameBudgetThroughTheBinary(t *testing.T) {
	in := start(t)
	before := limitedAccepts.Load()
	for i := 0; i < limits.TargetHostBurst; i++ {
		in.apiFresh(t, "https://limited.test/")
	}
	settle()
	if n := limitedAccepts.Load() - before; n != limits.TargetHostBurst {
		t.Fatalf("control: the target saw %d connections, want %d", n, limits.TargetHostBurst)
	}
	code, res, _ := in.apiFresh(t, "https://limited.test/")
	if code != http.StatusTooManyRequests || res.Conclusion.Code != diag.CodeTargetLimited {
		t.Errorf("past the budget: %d %+v", code, res.Conclusion)
	}
	settle()
	if n := limitedAccepts.Load() - before; n != limits.TargetHostBurst {
		t.Errorf("the refused check reached the target: %d connections, want %d", n, limits.TargetHostBurst)
	}
}

// AC 2: hostnames sharing one address share its budget.
func TestDestinationBudgetThroughTheBinary(t *testing.T) {
	in := start(t)
	before := sharedAccepts.Load()
	for i := 1; i <= limits.TargetDestBurst; i++ {
		in.apiFresh(t, fmt.Sprintf("https://share%d.test/", i))
	}
	settle()
	if n := sharedAccepts.Load() - before; n != limits.TargetDestBurst {
		t.Fatalf("control: %d connections, want %d", n, limits.TargetDestBurst)
	}
	code, res, _ := in.api(t, fmt.Sprintf("https://share%d.test/", limits.TargetDestBurst+1))
	tcp := res.Hops[0].Steps[1]
	if code != http.StatusTooManyRequests || tcp.Status != diag.StatusRefused || tcp.Code != diag.CodeTargetLimited {
		t.Errorf("a fresh hostname on a spent address: %d, tcp %+v", code, tcp)
	}
	settle()
	if n := sharedAccepts.Load() - before; n != limits.TargetDestBurst {
		t.Errorf("the refused attempt reached the target: %d connections", n)
	}
}

// AC 3: a redirect into a spent hostname stops at that hop with the same code.
func TestRedirectIntoASpentHostname(t *testing.T) {
	in := start(t)
	for i := 0; i < limits.TargetHostBurst; i++ {
		in.apiFresh(t, "https://limited2.test/")
	}
	settle()
	before := limited2Accepts.Load()
	code, res, _ := in.api(t, "https://tolimited.test/")
	if code != http.StatusTooManyRequests || len(res.Hops) != 2 || res.Hops[1].Code != diag.CodeTargetLimited || !strings.Contains(res.Conclusion.Summary, "2番目") {
		t.Errorf("redirect into a spent hostname: %d %+v (%d hops)", code, res.Conclusion, len(res.Hops))
	}
	settle()
	if n := limited2Accepts.Load() - before; n != 0 {
		t.Errorf("the refused hop reached its target %d times", n)
	}
}

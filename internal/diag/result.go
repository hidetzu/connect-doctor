package diag

import "time"

// Status is the vocabulary of docs/DESIGN.md § 1. ⚠ A new value is a spec change.
type Status string

const (
	StatusOK             Status = "ok"
	StatusFailed         Status = "failed"
	StatusRefused        Status = "refused"
	StatusSkipped        Status = "skipped"
	StatusNotApplicable  Status = "not_applicable"
	StatusNotImplemented Status = "not_implemented"
)

// Conclusion statuses (docs/DESIGN.md § 6).
const (
	ConclusionOK         = "ok"
	ConclusionFailed     = "failed"
	ConclusionRefused    = "refused"
	ConclusionIncomplete = "incomplete"
)

// Step names, in the order they run.
const (
	StepDNS  = "dns"
	StepTCP  = "tcp"
	StepTLS  = "tls"
	StepHTTP = "http"
)

// Steps is the order of the ladder.
var Steps = []string{StepDNS, StepTCP, StepTLS, StepHTTP}

// Result is the JSON API, and what the page renders (docs/adr/0006).
// ⚠ Field names are a contract once the API ships.
type Result struct {
	URL              string     `json:"url,omitempty"`
	ObservedFrom     string     `json:"observed_from"`
	ObservedFromNote string     `json:"observed_from_note"`
	CheckedAt        time.Time  `json:"checked_at"`
	DurationMS       int64      `json:"duration_ms"`
	Conclusion       Conclusion `json:"conclusion"`
	Hops             []Hop      `json:"hops"`
}

// Conclusion is the one answer.
type Conclusion struct {
	Status     string `json:"status"`
	FailedStep string `json:"failed_step,omitempty"`
	Code       string `json:"code,omitempty"`
	Summary    string `json:"summary"`
}

// Hop is one URL in a redirect chain.
type Hop struct {
	URL   string `json:"url,omitempty"`
	Steps []Step `json:"steps"`
}

// Step is one layer's result.
type Step struct {
	Step       string  `json:"step"`
	Status     Status  `json:"status"`
	DurationMS *int64  `json:"duration_ms,omitempty"`
	Code       string  `json:"code,omitempty"`
	Message    string  `json:"message,omitempty"`
	Detail     *Detail `json:"detail,omitempty"`
}

// Detail is what a step observed. ⚠ A refused address never appears here
// (.claude/rules/security.md § 5).
type Detail struct {
	Addresses []string  `json:"addresses,omitempty"` // DNS
	Address   string    `json:"address,omitempty"`   // TCP: the address that connected
	Attempts  []Attempt `json:"attempts,omitempty"`  // TCP: every address tried, in order
	Error     string    `json:"error,omitempty"`
}

// Attempt is one TCP connection attempt.
type Attempt struct {
	Address    string `json:"address,omitempty"` // ⚠ absent when the policy refused it
	Outcome    string `json:"outcome"`           // "ok" or a tcp.* code
	DurationMS int64  `json:"duration_ms"`
	Error      string `json:"error,omitempty"` // only for tcp.failed
}

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
	URL              string    `json:"url,omitempty"`
	ObservedFrom     string    `json:"observed_from"`
	ObservedFromNote string    `json:"observed_from_note"`
	CheckedAt        time.Time `json:"checked_at"`
	// Cached is true when this result was answered from the cache without
	// connecting again; CheckedAt is then the original check's time.
	Cached     bool       `json:"cached,omitempty"`
	DurationMS int64      `json:"duration_ms"`
	Conclusion Conclusion `json:"conclusion"`
	Hops       []Hop      `json:"hops"`

	// RetryAfter is set when a target limit stopped the check (not in the JSON;
	// the server turns it into a Retry-After header).
	RetryAfter time.Duration `json:"-"`
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
	URL string `json:"url,omitempty"`
	// Code is set only on a redirect target refused before any step ran
	// (an input.* code); its steps are all skipped.
	Code  string `json:"code,omitempty"`
	Steps []Step `json:"steps"`

	retryAfter time.Duration
}

// Step is one layer's result.
type Step struct {
	Step       string  `json:"step"`
	Status     Status  `json:"status"`
	DurationMS *int64  `json:"duration_ms,omitempty"`
	Code       string  `json:"code,omitempty"`
	Message    string  `json:"message,omitempty"`
	Detail     *Detail `json:"detail,omitempty"`

	retryAfter time.Duration
}

// Detail is what a step observed. ⚠ A refused address never appears here
// (.claude/rules/security.md § 5).
type Detail struct {
	Addresses []string  `json:"addresses,omitempty"` // DNS
	Address   string    `json:"address,omitempty"`   // TCP: the address that connected
	Attempts  []Attempt `json:"attempts,omitempty"`  // TCP: every address tried, in order

	TLSVersion  string      `json:"tls_version,omitempty"`  // TLS
	CipherSuite string      `json:"cipher_suite,omitempty"` // TLS
	ALPN        string      `json:"alpn,omitempty"`         // TLS
	Certificate *CertDetail `json:"certificate,omitempty"`  // TLS: the leaf, verified or not

	StatusCode    int               `json:"status_code,omitempty"`     // HTTP
	Status        string            `json:"status,omitempty"`          // HTTP: the status line's text
	Protocol      string            `json:"protocol,omitempty"`        // HTTP
	Headers       map[string]string `json:"headers,omitempty"`         // HTTP: a short subset
	BodyBytesRead int64             `json:"body_bytes_read,omitempty"` // HTTP: at most limits.ResponseBodyBytes
	BodyTruncated bool              `json:"body_truncated,omitempty"`  // HTTP: more body existed than was read

	Error string `json:"error,omitempty"`
}

// CertDetail is what the leaf certificate says about itself. ⚠ Shown, not graded.
type CertDetail struct {
	Subject   string   `json:"subject"`
	Names     []string `json:"names,omitempty"` // DNS and IP SANs
	Issuer    string   `json:"issuer"`
	NotBefore string   `json:"not_before"`
	NotAfter  string   `json:"not_after"`
}

// Attempt is one TCP connection attempt.
type Attempt struct {
	Address    string `json:"address,omitempty"` // ⚠ absent when the policy refused it
	Outcome    string `json:"outcome"`           // "ok" or a tcp.* code
	DurationMS int64  `json:"duration_ms"`
	Error      string `json:"error,omitempty"` // only for tcp.failed
}

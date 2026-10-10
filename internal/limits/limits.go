// Package limits holds every timeout and cap in ConnectDoctor, in one place
// (.claude/rules/security.md § 4, docs/DESIGN.md § 3).
//
// ⚠ These are budgets we chose, not measurements. Tightened on 2026-10-10 as part of
// the abuse limits (docs/adr/0010, hidetzu/connect-doctor#26). A number written inline
// anywhere else is a second implementation of the same question.
package limits

import "time"

const (
	// DNS bounds one resolution, including the pure-Go resolver's retries.
	DNS = 2 * time.Second

	// TCPAttempt and TCPTotal bound one connect attempt and all of them.
	TCPAttempt = 3 * time.Second
	TCPTotal   = 4 * time.Second

	// TLS bounds the handshake.
	TLS = 3 * time.Second

	// HTTP bounds writing the request and reading the response headers.
	HTTP = 4 * time.Second

	// Check is the hard ceiling for the whole check, every hop included,
	// enforced by one context.
	Check = 10 * time.Second

	// RedirectHops is the most hops one check follows.
	RedirectHops = 3

	// ResponseHeaderBytes and ResponseBodyBytes cap what is read.
	ResponseHeaderBytes = 64 << 10
	ResponseBodyBytes   = 64 << 10

	// URLBytes caps the length of the URL we accept.
	URLBytes = 2048

	// ConcurrentChecks bounds what one process can be made to do at once.
	ConcurrentChecks = 8

	// Per-client limits (docs/adr/0010, hidetzu/connect-doctor#6). ⚠ In memory, per instance:
	// a replaced instance starts from zero, so ClientPerDay is best effort.
	ClientBurst      = 3
	ClientRefill     = 12 * time.Second // one more check per interval after the burst: 5/min
	ClientPerHour    = 30
	ClientPerDay     = 100
	ClientConcurrent = 1

	// GlobalPerHour is the cost breaker: checks per hour per instance, whoever asks.
	GlobalPerHour = 3000

	// TrackedClients bounds the limiter's memory; the least recently seen client is
	// forgotten first.
	TrackedClients = 10000

	// Server-side timeouts for our own HTTP listener.
	ServerReadHeader = 5 * time.Second
	ServerWrite      = Check + 10*time.Second
)

// Package limits holds every timeout and cap in ConnectDoctor, in one place
// (.claude/rules/security.md § 4, docs/DESIGN.md § 3).
//
// ⚠ These are budgets we chose, not measurements. A number written inline
// anywhere else is a second implementation of the same question.
package limits

import "time"

const (
	// DNS bounds one resolution, including the pure-Go resolver's retries.
	DNS = 3 * time.Second

	// TCPAttempt and TCPTotal bound one connect attempt and all of them.
	TCPAttempt = 4 * time.Second
	TCPTotal   = 6 * time.Second

	// TLS bounds the handshake.
	TLS = 5 * time.Second

	// HTTP bounds writing the request and reading the response headers.
	HTTP = 8 * time.Second

	// Check is the hard ceiling for the whole check, every hop included,
	// enforced by one context.
	Check = 20 * time.Second

	// RedirectHops is the most hops one check follows.
	RedirectHops = 5

	// ResponseHeaderBytes and ResponseBodyBytes cap what is read.
	ResponseHeaderBytes = 64 << 10
	ResponseBodyBytes   = 64 << 10

	// URLBytes caps the length of the URL we accept.
	URLBytes = 2048

	// ConcurrentChecks bounds what one process can be made to do at once.
	ConcurrentChecks = 16

	// Server-side timeouts for our own HTTP listener.
	ServerReadHeader = 5 * time.Second
	ServerWrite      = Check + 10*time.Second
)

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

	// Server-side timeouts for our own HTTP listener.
	ServerReadHeader = 5 * time.Second
	ServerWrite      = Check + 10*time.Second
)

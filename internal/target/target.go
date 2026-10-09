// Package target turns one string into one checkable URL, or refuses it
// before anything leaves the process (.claude/rules/security.md § 2,
// docs/DESIGN.md § 4 T2, T3, T8, T9, T13).
//
// ⚠ Refusing a name here is an early answer, not the defence: every resolved
// address still goes through internal/policy.
package target

import (
	"net/netip"
	"net/url"
	"strings"

	"github.com/hidetzu/connect-doctor/internal/limits"
	"github.com/hidetzu/connect-doctor/internal/policy"
)

// Refusal codes. The vocabulary is docs/DESIGN.md § 1.
const (
	CodeMissing           = "input.missing"
	CodeMalformed         = "input.malformed"
	CodeUnsupportedScheme = "input.unsupported_scheme"
	CodeUnsupportedPort   = "input.unsupported_port"
	CodeCredentials       = "input.credentials"
	CodeLocalName         = "input.local_name"
	CodeRefusedAddress    = "input.refused_address"
	CodeIDNNotImplemented = "input.idn_not_implemented"
)

// Refusal is why a URL is not checked. It is an error so callers can return
// it, but it is an outcome, not a fault.
type Refusal struct{ Code string }

func (r *Refusal) Error() string { return r.Code }

// Target is one URL we will check.
type Target struct {
	URL    string     // normalised: lower-case scheme and host, no fragment
	Scheme string     // "http" or "https"
	Host   string     // lower-case DNS name without trailing dot, or the literal address
	Port   string     // "80" or "443"
	Addr   netip.Addr // valid only when Host is an IP literal
}

// IsLiteral reports whether the host was an IP address rather than a name.
func (t Target) IsLiteral() bool { return t.Addr.IsValid() }

// localSuffixes are names that only mean something inside one network.
// Grounds: RFC 6761 § 6.3 (localhost), RFC 6762 (local), RFC 8375
// (home.arpa), ICANN's 2024 reservation of "internal" for private use.
var localSuffixes = []string{"localhost", "local", "home.arpa", "internal"}

// Parse parses raw and refuses what ConnectDoctor will not check.
func Parse(raw string) (Target, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Target{}, &Refusal{CodeMissing}
	}
	if len(raw) > limits.URLBytes {
		return Target{}, &Refusal{CodeMalformed}
	}
	u, err := url.Parse(raw)
	if err != nil {
		return Target{}, &Refusal{CodeMalformed}
	}
	scheme := strings.ToLower(u.Scheme)
	switch scheme {
	case "http", "https":
	case "":
		// "example.com" parses as a path. ⚠ We do not guess a scheme.
		return Target{}, &Refusal{CodeMalformed}
	default:
		return Target{}, &Refusal{CodeUnsupportedScheme}
	}
	if u.Opaque != "" {
		return Target{}, &Refusal{CodeMalformed}
	}
	if u.User != nil {
		return Target{}, &Refusal{CodeCredentials}
	}
	host := u.Hostname()
	if host == "" {
		return Target{}, &Refusal{CodeMalformed}
	}

	port := u.Port()
	if port == "" {
		port = map[string]string{"http": "80", "https": "443"}[scheme]
	}
	// ⚠ Exactly "80" or "443": "0080" is refused rather than normalised.
	if port != "80" && port != "443" {
		return Target{}, &Refusal{CodeUnsupportedPort}
	}

	t := Target{Scheme: scheme, Port: port}
	if strings.HasPrefix(u.Host, "[") {
		a, err := netip.ParseAddr(host)
		if err != nil || !a.Is6() || a.Zone() != "" {
			return Target{}, &Refusal{CodeMalformed}
		}
		t.Addr, t.Host = a, a.String()
	} else if a, err := netip.ParseAddr(host); err == nil && a.Is4() {
		// netip accepts only canonical dotted-quad: "0177.0.0.1" and
		// "127.1" fail here and are refused below as numeric.
		t.Addr, t.Host = a, a.String()
	} else {
		name, code := checkName(host)
		if code != "" {
			return Target{}, &Refusal{code}
		}
		t.Host = name
	}
	if t.IsLiteral() && !policy.Allowed(t.Addr) {
		return Target{}, &Refusal{CodeRefusedAddress}
	}

	n := *u
	n.Scheme = scheme
	n.Fragment, n.RawFragment = "", ""
	n.Host = t.Host
	if t.Addr.Is6() {
		n.Host = "[" + t.Host + "]"
	}
	if u.Port() != "" {
		n.Host += ":" + port
	}
	if n.Path == "" {
		n.Path = "/"
	}
	t.URL = n.String()
	return t, nil
}

// checkName validates a DNS host name and returns it lower-case without a
// trailing dot, or a refusal code.
func checkName(host string) (string, string) {
	name := strings.TrimSuffix(strings.ToLower(host), ".")
	if name == "" || len(name) > 253 {
		return "", CodeMalformed
	}
	labels := strings.Split(name, ".")
	for _, l := range labels {
		if l == "" || len(l) > 63 {
			return "", CodeMalformed
		}
		for _, r := range l {
			switch {
			case r > 0x7f:
				// ⚠ Our gap, not a bad URL: punycode needs a module we do
				// not take (docs/adr/0006).
				return "", CodeIDNNotImplemented
			case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
			default:
				return "", CodeMalformed
			}
		}
	}
	// ⚠ WHATWG URL § host parsing: a final label that is a number makes the
	// host an IPv4 address in some notation ("2130706433", "0x7f.1",
	// "0177.0.0.1", "127.1"). Resolvers disagree on these; refuse them all.
	if numeric(labels[len(labels)-1]) {
		return "", CodeMalformed
	}
	if len(labels) == 1 {
		return "", CodeLocalName
	}
	for _, s := range localSuffixes {
		if name == s || strings.HasSuffix(name, "."+s) {
			return "", CodeLocalName
		}
	}
	return name, ""
}

func numeric(label string) bool {
	if strings.HasPrefix(label, "0x") {
		label = label[2:]
		if label == "" {
			return true
		}
		for _, r := range label {
			if !strings.ContainsRune("0123456789abcdef", r) {
				return false
			}
		}
		return true
	}
	for _, r := range label {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

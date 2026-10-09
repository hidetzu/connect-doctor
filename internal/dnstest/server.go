// Package dnstest is a minimal DNS server on loopback UDP, for tests only.
//
// ⚠ It answers A and AAAA queries from a fixed table and counts every query
// by name, so a test can assert that a refused URL caused zero queries
// (.claude/rules/security.md § 6: prove nothing left, paired with a control
// that did).
//
// It is written by hand because the standard library has no DNS message
// builder and the project takes no third-party modules (docs/adr/0006).
// It implements only what the pure-Go resolver sends: one question, class IN,
// over UDP. Anything else is answered with FORMERR.
package dnstest

import (
	"encoding/binary"
	"errors"
	"net"
	"net/netip"
	"strings"
	"sync"
)

// RCODE values, RFC 1035 § 4.1.1.
const (
	RcodeSuccess  = 0
	RcodeFormErr  = 1
	RcodeServFail = 2
	RcodeNXDomain = 3
)

const (
	typeA    = 1
	typeAAAA = 28
	classIN  = 1
)

// Answer is how the server responds for one name.
type Answer struct {
	Rcode int          // RcodeSuccess with no Addrs is NODATA.
	Addrs []netip.Addr // A for IPv4, AAAA for IPv6.
	Drop  bool         // Never answer (the client times out).
}

// Server is a running fake DNS server.
type Server struct {
	conn    net.PacketConn
	mu      sync.Mutex
	answers map[string]Answer
	queries map[string]int
	done    chan struct{}
}

// Start listens on 127.0.0.1 on a free UDP port. Names not in answers get
// NXDOMAIN. Names are compared lower-case and without the trailing dot.
func Start(answers map[string]Answer) (*Server, error) {
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	s := &Server{conn: conn, answers: map[string]Answer{}, queries: map[string]int{}, done: make(chan struct{})}
	for k, v := range answers {
		s.answers[canonical(k)] = v
	}
	go s.serve()
	return s, nil
}

// Addr is the host:port the server listens on.
func (s *Server) Addr() string { return s.conn.LocalAddr().String() }

// Queries returns how many queries (of any type) arrived for name.
func (s *Server) Queries(name string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.queries[canonical(name)]
}

// TotalQueries returns how many queries arrived for any name.
func (s *Server) TotalQueries() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, v := range s.queries {
		n += v
	}
	return n
}

// Close stops the server.
func (s *Server) Close() error {
	err := s.conn.Close()
	<-s.done
	return err
}

func canonical(name string) string { return strings.TrimSuffix(strings.ToLower(name), ".") }

func (s *Server) serve() {
	defer close(s.done)
	buf := make([]byte, 1500)
	for {
		n, from, err := s.conn.ReadFrom(buf)
		if err != nil {
			return
		}
		if resp := s.handle(buf[:n]); resp != nil {
			_, _ = s.conn.WriteTo(resp, from)
		}
	}
}

func (s *Server) handle(q []byte) []byte {
	if len(q) < 12 {
		return nil
	}
	name, qtype, qend, err := parseQuestion(q)
	if err != nil {
		return header(q, RcodeFormErr, 0, 0)
	}
	s.mu.Lock()
	s.queries[name]++
	ans, ok := s.answers[name]
	s.mu.Unlock()
	if !ok {
		ans = Answer{Rcode: RcodeNXDomain}
	}
	if ans.Drop {
		return nil
	}
	var rrs [][]byte
	if ans.Rcode == RcodeSuccess {
		for _, a := range ans.Addrs {
			switch {
			case a.Is4() && qtype == typeA:
				b := a.As4()
				rrs = append(rrs, rr(typeA, b[:]))
			case a.Is6() && !a.Is4In6() && qtype == typeAAAA:
				b := a.As16()
				rrs = append(rrs, rr(typeAAAA, b[:]))
			}
		}
	}
	out := header(q, ans.Rcode, 1, len(rrs))
	out = append(out, q[12:qend]...) // the question, echoed
	for _, r := range rrs {
		out = append(out, r...)
	}
	return out
}

func header(q []byte, rcode, qd, an int) []byte {
	h := make([]byte, 12)
	copy(h[0:2], q[0:2]) // ID
	rd := q[2] & 0x01
	h[2] = 0x80 | rd // QR=1, opcode 0, RD copied
	h[3] = 0x80 | byte(rcode&0x0f)
	binary.BigEndian.PutUint16(h[4:], uint16(qd))
	binary.BigEndian.PutUint16(h[6:], uint16(an))
	return h
}

// rr builds one answer record whose name is a pointer to the question name
// at offset 12.
func rr(typ uint16, data []byte) []byte {
	b := []byte{0xc0, 0x0c}
	b = binary.BigEndian.AppendUint16(b, typ)
	b = binary.BigEndian.AppendUint16(b, classIN)
	b = binary.BigEndian.AppendUint32(b, 60)
	b = binary.BigEndian.AppendUint16(b, uint16(len(data)))
	return append(b, data...)
}

func parseQuestion(q []byte) (name string, qtype uint16, end int, err error) {
	if binary.BigEndian.Uint16(q[4:]) != 1 {
		return "", 0, 0, errors.New("want exactly one question")
	}
	i := 12
	var labels []string
	for {
		if i >= len(q) {
			return "", 0, 0, errors.New("truncated name")
		}
		l := int(q[i])
		i++
		if l == 0 {
			break
		}
		if l > 63 || i+l > len(q) {
			return "", 0, 0, errors.New("bad label")
		}
		labels = append(labels, string(q[i:i+l]))
		i += l
	}
	if i+4 > len(q) {
		return "", 0, 0, errors.New("truncated question")
	}
	qtype = binary.BigEndian.Uint16(q[i:])
	if binary.BigEndian.Uint16(q[i+2:]) != classIN {
		return "", 0, 0, errors.New("class not IN")
	}
	return canonical(strings.Join(labels, ".")), qtype, i + 4, nil
}

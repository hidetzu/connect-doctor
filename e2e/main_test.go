//go:build e2e

package e2e

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hidetzu/connect-doctor/internal/tlstest"
)

// Addresses the harness gives meaning to, inside the namespace. ⚠ They are
// globally-routable so the policy permits them; documentation ranges would be
// refused and prove nothing. ⚠ No packet can leave: the namespace has no route
// out (checked before anything runs).
const (
	addrListen      = "93.184.215.14"        // on lo, listeners on :80 and :443
	addrClosed      = "93.184.215.15"        // on lo, nothing listening -> RST
	addrDrop        = "8.8.4.4"              // routed into a dummy interface -> silence
	addrUnreachable = "9.9.9.9"              // unreachable route -> EHOSTUNREACH
	addrV6          = "2606:4700:4700::1111" // no IPv6 route at all -> ENETUNREACH

	// TLS on :443, one address per certificate problem.
	addrExpired   = "93.184.215.21" // expired certificate
	addrUntrusted = "93.184.215.22" // issued by a CA nobody trusts
	addrMismatch  = "93.184.215.23" // valid, for another name
	addrPlain     = "93.184.215.24" // plain HTTP on 443
	addrSilent    = "93.184.215.25" // accepts, never speaks

	// HTTP on :80.
	addrHTTPSilent = "93.184.215.26" // reads the request, never answers
	addrHTTPClose  = "93.184.215.27" // reads the request, closes without a byte

	// Redirect chains that revisit hosts get their own addresses, so the
	// per-target limits (hidetzu/connect-doctor#27) are not what they test.
	addrSpinA          = "93.184.215.37"
	addrSpinB          = "93.184.215.38"
	addrSlowA          = "93.184.215.40"
	addrSlowB          = "93.184.215.41"
	addrRedirToLimited = "93.184.215.46"

	// hidetzu/connect-doctor#27: TLS listeners that count what reaches them.
	addrLimited  = "93.184.215.44" // limited.test
	addrShared   = "93.184.215.45" // share1..share6.test, one address
	addrLimited2 = "93.184.215.47" // limited2.test, the redirect target

	// hidetzu/connect-doctor#28: counts what a cached answer must not cause.
	addrCached = "93.184.215.48" // cached.test
)

var (
	limitedAccepts  atomic.Int32
	sharedAccepts   atomic.Int32
	limited2Accepts atomic.Int32
	cachedAccepts   atomic.Int32
)

// lastRequest is what the most recent request to addrListen carried
// (hidetzu/connect-doctor#4 AC 2).
var lastRequest atomic.Pointer[http.Request]

// trustEnv makes the binary under test trust the test CA and nothing else,
// through the system root store (internal/tlstest). ⚠ No product flag.
var trustEnv []string

var (
	publicAccepts   atomic.Int32 // connections accepted on addrListen
	loopbackAccepts atomic.Int32 // connections accepted on 127.0.0.1 — must stay 0
)

// TestMain refuses to run anywhere but an empty network namespace, then
// builds the world the final gate needs.
//
// ⚠ An exercise must not change the world (.claude/rules/verification.md):
// the binary under test makes real TCP connections, so the only place it may
// run is a namespace with no way out. scripts/verify.sh final provides one
// (unshare -rn).
func TestMain(m *testing.M) {
	if err := setup(); err != nil {
		fmt.Fprintf(os.Stderr, "e2e: %v\n", err)
		fmt.Fprintln(os.Stderr, "e2e: ⚠ run the final gate with scripts/verify.sh final — it runs these tests inside an empty network namespace")
		os.Exit(2)
	}
	os.Exit(m.Run())
}

func ip(args ...string) (string, error) {
	out, err := exec.Command("ip", args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("ip %s: %v: %s", strings.Join(args, " "), err, out)
	}
	return string(out), nil
}

func setup() error {
	links, err := ip("-o", "link")
	if err != nil {
		return err
	}
	for _, l := range strings.Split(strings.TrimSpace(links), "\n") {
		if f := strings.Fields(l); len(f) > 1 && strings.TrimSuffix(f[1], ":") != "lo" {
			return fmt.Errorf("refusing to run: interface %s exists, so this is not an empty network namespace", f[1])
		}
	}
	for _, fam := range []string{"-4", "-6"} {
		if r, _ := ip(fam, "route", "show", "default"); strings.TrimSpace(r) != "" {
			return fmt.Errorf("refusing to run: a default route exists (%s)", strings.TrimSpace(r))
		}
	}
	for _, c := range [][]string{
		{"link", "set", "lo", "up"},
		{"addr", "add", addrListen + "/32", "dev", "lo"},
		{"addr", "add", addrClosed + "/32", "dev", "lo"},
		{"addr", "add", addrExpired + "/32", "dev", "lo"},
		{"addr", "add", addrUntrusted + "/32", "dev", "lo"},
		{"addr", "add", addrMismatch + "/32", "dev", "lo"},
		{"addr", "add", addrPlain + "/32", "dev", "lo"},
		{"addr", "add", addrSilent + "/32", "dev", "lo"},
		{"addr", "add", addrHTTPSilent + "/32", "dev", "lo"},
		{"addr", "add", addrHTTPClose + "/32", "dev", "lo"},
		{"addr", "add", addrSpinA + "/32", "dev", "lo"},
		{"addr", "add", addrSpinB + "/32", "dev", "lo"},
		{"addr", "add", addrSlowA + "/32", "dev", "lo"},
		{"addr", "add", addrSlowB + "/32", "dev", "lo"},
		{"addr", "add", addrRedirToLimited + "/32", "dev", "lo"},
		{"addr", "add", addrLimited + "/32", "dev", "lo"},
		{"addr", "add", addrShared + "/32", "dev", "lo"},
		{"addr", "add", addrLimited2 + "/32", "dev", "lo"},
		{"addr", "add", addrCached + "/32", "dev", "lo"},
		{"link", "add", "d0", "type", "dummy"},
		{"link", "set", "d0", "up"},
		{"route", "add", addrDrop + "/32", "dev", "d0"},
		{"route", "add", "unreachable", addrUnreachable + "/32"},
	} {
		if _, err := ip(c...); err != nil {
			return err
		}
	}
	ca, err := tlstest.NewCA("ConnectDoctor e2e CA")
	if err != nil {
		return err
	}
	stranger, err := tlstest.NewCA("a CA nobody trusts")
	if err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "e2e-roots-")
	if err != nil {
		return err
	}
	if trustEnv, err = tlstest.TrustOnly(ca, dir); err != nil {
		return err
	}
	now := time.Now()
	good, _ := ca.Valid("ok.test", "fallback.test", "status503.test", "bigbody.test", "bigheader.test",
		"redir.test", "toloop.test", "tolocal.test", "toport.test", "spin-a.test", "spin-b.test", "rel.test",
		"slowhop-a.test", "slowhop-b.test", "tolimited.test", "limited.test", "limited2.test", "cached.test",
		"share1.test", "share2.test", "share3.test", "share4.test", "share5.test", "share6.test", addrListen)
	expired, _ := ca.Leaf([]string{"expired.test"}, now.Add(-48*time.Hour), now.Add(-24*time.Hour))
	untrusted, _ := stranger.Valid("untrusted.test")
	mismatch, _ := ca.Valid("other.test")

	for _, a := range []string{addrListen, addrSpinA, addrSpinB, addrSlowA, addrSlowB, addrRedirToLimited} {
		if err := serveHTTP(a, &good); err != nil {
			return err
		}
	}
	for a, n := range map[string]*atomic.Int32{addrLimited: &limitedAccepts, addrShared: &sharedAccepts, addrLimited2: &limited2Accepts, addrCached: &cachedAccepts} {
		if err := listen(a+":443", n, &good); err != nil {
			return err
		}
	}
	for addr, cert := range map[string]*tls.Certificate{
		addrExpired: &expired, addrUntrusted: &untrusted, addrMismatch: &mismatch,
	} {
		if err := listen(addr+":443", &publicAccepts, cert); err != nil {
			return err
		}
	}
	if err := listenRaw(addrPlain+":443", []byte("HTTP/1.1 400 Bad Request\r\nContent-Length: 0\r\n\r\n")); err != nil {
		return err
	}
	if err := listenRaw(addrSilent+":443", nil); err != nil {
		return err
	}
	if err := listenRaw(addrHTTPSilent+":80", nil); err != nil {
		return err
	}
	if err := listenClose(addrHTTPClose + ":80"); err != nil {
		return err
	}
	for _, a := range []string{"127.0.0.1:80", "127.0.0.1:443"} {
		if err := listen(a, &loopbackAccepts, nil); err != nil {
			return err
		}
	}
	return nil
}

// listen accepts and counts connections; with cert, it then completes a TLS
// handshake. Each connection is held briefly so the client sees it complete.
func listen(addr string, n *atomic.Int32, cert *tls.Certificate) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			n.Add(1)
			go func() {
				defer c.Close()
				if cert != nil {
					s := tls.Server(c, &tls.Config{Certificates: []tls.Certificate{*cert}, NextProtos: []string{"http/1.1"}})
					_ = s.SetDeadline(time.Now().Add(10 * time.Second))
					if s.Handshake() == nil {
						buf := make([]byte, 1)
						_, _ = s.Read(buf)
					}
					return
				}
				time.Sleep(200 * time.Millisecond)
			}()
		}
	}()
	return nil
}

// listenRaw answers whatever arrives with raw (or, for nil, with silence
// until the client gives up).
func listenRaw(addr string, raw []byte) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer c.Close()
				_ = c.SetDeadline(time.Now().Add(15 * time.Second))
				buf := make([]byte, 4096)
				_, _ = c.Read(buf)
				if raw != nil {
					_, _ = c.Write(raw)
				}
				_, _ = c.Read(buf)
			}()
		}
	}()
	return nil
}

// serveHTTP runs a real HTTP server on addr:80 and, with cert, on addr:443,
// answering by Host. ⚠ HTTP/1.1 only, as ConnectDoctor offers (docs/adr/0005).
func serveHTTP(addr string, cert *tls.Certificate) error {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastRequest.Store(r)
		host, _, _ := strings.Cut(r.Host, ":")
		switch host {
		case "status503.test":
			w.WriteHeader(http.StatusServiceUnavailable)
		case "bigbody.test":
			w.Header().Set("Content-Length", strconv.Itoa(10<<20))
			_, _ = w.Write([]byte(strings.Repeat("x", 10<<20)))
		case "bigheader.test":
			w.Header().Set("X-Big", strings.Repeat("a", 70<<10))
			_, _ = w.Write([]byte("ok"))
		// hidetzu/connect-doctor#5
		case "redir.test":
			http.Redirect(w, r, "https://ok.test/landed", http.StatusMovedPermanently)
		case "toloop.test":
			http.Redirect(w, r, "http://127.0.0.1/", http.StatusFound)
		case "tolocal.test":
			http.Redirect(w, r, "http://localhost/", http.StatusFound)
		case "toport.test":
			http.Redirect(w, r, "http://ok.test:8080/", http.StatusFound)
		case "spin-a.test":
			http.Redirect(w, r, "https://spin-b.test/", http.StatusFound)
		case "spin-b.test":
			http.Redirect(w, r, "https://spin-a.test/", http.StatusFound)
		case "tolimited.test":
			http.Redirect(w, r, "https://limited2.test/", http.StatusFound)
		case "rel.test":
			if r.URL.Path == "/after" {
				_, _ = w.Write([]byte("ok"))
				return
			}
			w.Header().Set("Location", "/after")
			w.WriteHeader(http.StatusFound)
		case "slowhop-a.test", "slowhop-b.test":
			// Each hop answers just inside limits.HTTP, so only the whole-check
			// ceiling (limits.Check) can stop the chain. Two hosts alternate so
			// the per-target limits are not what stops it.
			time.Sleep(3 * time.Second)
			next := "https://slowhop-b.test/"
			if host == "slowhop-b.test" {
				next = "https://slowhop-a.test/"
			}
			w.Header().Set("Location", next+strconv.Itoa(len(r.URL.Path)))
			w.WriteHeader(http.StatusFound)
		default:
			_, _ = w.Write([]byte("ok"))
		}
	})
	plain, err := net.Listen("tcp", addr+":80")
	if err != nil {
		return err
	}
	secure, err := net.Listen("tcp", addr+":443")
	if err != nil {
		return err
	}
	go (&http.Server{Handler: h}).Serve(counting{plain, &publicAccepts})
	tl := tls.NewListener(counting{secure, &publicAccepts}, &tls.Config{Certificates: []tls.Certificate{*cert}, NextProtos: []string{"http/1.1"}})
	go (&http.Server{Handler: h}).Serve(tl)
	return nil
}

// counting counts accepted connections.
type counting struct {
	net.Listener
	n *atomic.Int32
}

func (c counting) Accept() (net.Conn, error) {
	conn, err := c.Listener.Accept()
	if err == nil {
		c.n.Add(1)
	}
	return conn, err
}

// listenClose reads the request and closes without answering.
func listenClose(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				buf := make([]byte, 4096)
				_, _ = c.Read(buf)
				c.Close()
			}()
		}
	}()
	return nil
}

func TestHarnessIsIsolated(t *testing.T) {
	// ⚠ Control for the whole gate: an outward address is unreachable here.
	c, err := net.DialTimeout("tcp", "1.1.1.1:443", time.Second)
	if err == nil {
		c.Close()
		t.Fatal("1.1.1.1:443 connected: the namespace has a way out")
	}
}

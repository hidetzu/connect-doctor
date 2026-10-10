// Package tlstest makes certificates at test time, for tests only.
//
// ⚠ Trust is given to its CA the way any deployment gives trust: through the
// system root store, by pointing SSL_CERT_FILE at the CA and SSL_CERT_DIR at
// an empty directory (crypto/x509 loadSystemRoots). ⚠ Nothing in product
// code can be told to trust anything else, and InsecureSkipVerify stays
// forbidden (internal/conformance).
package tlstest

import (
	"bufio"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

// CA is a certificate authority that can issue leaves.
type CA struct {
	Cert *x509.Certificate
	Key  *ecdsa.PrivateKey
	PEM  []byte
}

// NewCA creates a self-signed CA named name.
func NewCA(name string) (*CA, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: name},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, err
	}
	cert, _ := x509.ParseCertificate(der)
	return &CA{Cert: cert, Key: key, PEM: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})}, nil
}

// Leaf issues a server certificate for names (DNS names or IP addresses),
// valid between notBefore and notAfter.
func (ca *CA) Leaf(names []string, notBefore, notAfter time.Time) (tls.Certificate, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}
	serial, _ := rand.Int(rand.Reader, big.NewInt(1<<62))
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: names[0]},
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	for _, n := range names {
		if ip := net.ParseIP(n); ip != nil {
			tmpl.IPAddresses = append(tmpl.IPAddresses, ip)
		} else {
			tmpl.DNSNames = append(tmpl.DNSNames, n)
		}
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca.Cert, &key.PublicKey, ca.Key)
	if err != nil {
		return tls.Certificate{}, err
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}, nil
}

// Valid issues a leaf valid from an hour ago for a day.
func (ca *CA) Valid(names ...string) (tls.Certificate, error) {
	return ca.Leaf(names, time.Now().Add(-time.Hour), time.Now().Add(24*time.Hour))
}

// TrustOnly writes ca's PEM under dir and returns the environment entries
// that make crypto/x509 trust that CA and nothing else.
func TrustOnly(ca *CA, dir string) ([]string, error) {
	file := filepath.Join(dir, "ca.pem")
	empty := filepath.Join(dir, "empty-cert-dir")
	if err := os.WriteFile(file, ca.PEM, 0o600); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(empty, 0o700); err != nil {
		return nil, err
	}
	return []string{"SSL_CERT_FILE=" + file, "SSL_CERT_DIR=" + empty}, nil
}

// Pipe returns the client end of a loopback TCP connection whose server end
// completes a TLS handshake with cert (when cert is non-nil), or answers with
// raw bytes (when raw is non-nil), then waits until the client closes.
//
// ⚠ A real socket, not net.Pipe: net.Pipe has no buffer, and a TLS client
// that sends an alert while the server is still writing its flight
// deadlocks on it (measured: every failure case timed out).
func Pipe(cert *tls.Certificate, raw []byte) net.Conn {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	go func() {
		defer ln.Close()
		server, err := ln.Accept()
		if err != nil {
			return
		}
		defer server.Close()
		switch {
		case cert != nil:
			s := tls.Server(server, &tls.Config{Certificates: []tls.Certificate{*cert}, NextProtos: []string{"http/1.1"}})
			if s.Handshake() != nil {
				return
			}
			buf := make([]byte, 1)
			_, _ = s.Read(buf)
		case raw != nil:
			buf := make([]byte, 4096)
			_, _ = server.Read(buf) // the ClientHello
			_, _ = server.Write(raw)
			_, _ = server.Read(buf)
		}
	}()
	client, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		panic(err)
	}
	return client
}

// OK200 is a minimal complete HTTP/1.1 response.
var OK200 = []byte("HTTP/1.1 200 OK\r\nContent-Length: 2\r\nConnection: close\r\n\r\nok")

// Serve returns the client end of a loopback TCP connection whose server end
// speaks TLS with cert if the client starts a TLS handshake, or plain text
// otherwise, and then answers the first request with response — or, for a
// nil response, closes without answering.
func Serve(cert *tls.Certificate, response []byte) net.Conn {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	go func() {
		defer ln.Close()
		raw, err := ln.Accept()
		if err != nil {
			return
		}
		defer raw.Close()
		br := bufio.NewReader(raw)
		first, err := br.Peek(1)
		if err != nil {
			return
		}
		var c net.Conn = peeked{raw, br}
		if first[0] == 0x16 { // TLS handshake record
			s := tls.Server(c, &tls.Config{Certificates: []tls.Certificate{*cert}, NextProtos: []string{"http/1.1"}})
			if s.Handshake() != nil {
				return
			}
			c = s
		}
		buf := make([]byte, 4096)
		_, _ = c.Read(buf) // the request
		if response == nil {
			return
		}
		_, _ = c.Write(response)
		_, _ = c.Read(buf)
	}()
	client, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		panic(err)
	}
	return client
}

// peeked is a net.Conn that reads through a bufio.Reader that has peeked.
type peeked struct {
	net.Conn
	r *bufio.Reader
}

func (p peeked) Read(b []byte) (int, error) { return p.r.Read(b) }

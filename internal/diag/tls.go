package diag

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"time"

	"github.com/hidetzu/connect-doctor/internal/limits"
)

// tlsStep handshakes over the TCP step's connection and verifies the chain
// for serverName against the system root store (docs/adr/0005).
//
// ⚠ There is no way to tell this step to trust anything else, and no
// InsecureSkipVerify (internal/conformance): what it reports is what a
// standard client would conclude.
func tlsStep(ctx context.Context, conn net.Conn, serverName string) (Step, *tls.Conn) {
	ctx, cancel := context.WithTimeout(ctx, limits.TLS)
	defer cancel()

	tc := tls.Client(conn, &tls.Config{
		ServerName: serverName,
		// ⚠ HTTP/1.1 only (docs/adr/0005).
		NextProtos: []string{"http/1.1"},
	})
	err := tc.HandshakeContext(ctx)
	if err == nil {
		st := tc.ConnectionState()
		d := &Detail{
			TLSVersion:  tls.VersionName(st.Version),
			CipherSuite: tls.CipherSuiteName(st.CipherSuite),
			ALPN:        st.NegotiatedProtocol,
		}
		if len(st.PeerCertificates) > 0 {
			d.Certificate = certDetail(st.PeerCertificates[0])
		}
		return Step{Step: StepTLS, Status: StatusOK, Detail: d}, tc
	}
	return classifyTLSError(err), nil
}

// classifyTLSError maps a handshake error to one outcome (.claude/rules/go.md).
func classifyTLSError(err error) Step {
	s := Step{Step: StepTLS, Status: StatusFailed, Detail: &Detail{}}

	// The certificate the server presented, even though it did not verify:
	// "expired on <date>" is the useful part of an expiry.
	var cve *tls.CertificateVerificationError
	if errors.As(err, &cve) && len(cve.UnverifiedCertificates) > 0 {
		s.Detail.Certificate = certDetail(cve.UnverifiedCertificates[0])
	}

	var (
		invalid  x509.CertificateInvalidError
		unknown  x509.UnknownAuthorityError
		hostname x509.HostnameError
		record   tls.RecordHeaderError
		alert    tls.AlertError
		ne       net.Error
	)
	switch {
	case errors.As(err, &invalid) && invalid.Reason == x509.Expired:
		// crypto/x509 uses Expired for "not yet valid" too; the dates in
		// detail say which.
		s.Code = "tls.cert_expired"
	case errors.As(err, &unknown):
		s.Code = "tls.cert_untrusted"
	case errors.As(err, &hostname):
		s.Code = "tls.cert_name_mismatch"
	case errors.As(err, &record):
		s.Code = "tls.not_tls"
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &ne) && ne.Timeout():
		s.Code = "tls.timeout"
	case errors.As(err, &alert):
		s.Code = "tls.handshake_failed"
		s.Detail.Error = err.Error()
	default:
		// ⚠ Not mapped to the nearest known outcome: the generic code, with
		// the raw text.
		s.Code = "tls.handshake_failed"
		s.Detail.Error = err.Error()
	}
	if s.Detail.Certificate == nil && s.Detail.Error == "" {
		s.Detail = nil
	}
	return s
}

func certDetail(c *x509.Certificate) *CertDetail {
	d := &CertDetail{
		Subject:   c.Subject.String(),
		Issuer:    c.Issuer.String(),
		NotBefore: c.NotBefore.UTC().Format(time.RFC3339),
		NotAfter:  c.NotAfter.UTC().Format(time.RFC3339),
		Names:     append([]string{}, c.DNSNames...),
	}
	for _, ip := range c.IPAddresses {
		d.Names = append(d.Names, ip.String())
	}
	return d
}

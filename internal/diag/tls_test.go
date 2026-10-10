package diag

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/hidetzu/connect-doctor/internal/tlstest"
)

func handshake(t *testing.T, conn net.Conn, host string) Step {
	t.Helper()
	st, tc := tlsStep(context.Background(), conn, host)
	if tc != nil {
		tc.Close()
	}
	return st
}

func TestTLSOutcomes(t *testing.T) {
	other, err := tlstest.NewCA("someone else")
	if err != nil {
		t.Fatal(err)
	}
	expired, _ := testCA.Leaf([]string{"example.com"}, time.Now().Add(-48*time.Hour), time.Now().Add(-24*time.Hour))
	notYet, _ := testCA.Leaf([]string{"example.com"}, time.Now().Add(24*time.Hour), time.Now().Add(48*time.Hour))
	untrusted, _ := other.Valid("example.com")
	wrongName, _ := testCA.Valid("other.example")

	cases := []struct {
		name   string
		conn   net.Conn
		status Status
		code   string
	}{
		{"valid", tlstest.Pipe(&validCert, nil), StatusOK, ""},
		{"expired", tlstest.Pipe(&expired, nil), StatusFailed, "tls.cert_expired"},
		{"not yet valid", tlstest.Pipe(&notYet, nil), StatusFailed, "tls.cert_expired"},
		{"unknown CA", tlstest.Pipe(&untrusted, nil), StatusFailed, "tls.cert_untrusted"},
		{"wrong name", tlstest.Pipe(&wrongName, nil), StatusFailed, "tls.cert_name_mismatch"},
		{"plain HTTP", tlstest.Pipe(nil, []byte("HTTP/1.1 400 Bad Request\r\nContent-Length: 0\r\n\r\n")), StatusFailed, "tls.not_tls"},
	}
	for _, c := range cases {
		st := handshake(t, c.conn, "example.com")
		if st.Status != c.status || st.Code != c.code {
			t.Errorf("%s: %s/%s, want %s/%s (detail %+v)", c.name, st.Status, st.Code, c.status, c.code, st.Detail)
		}
	}
}

func TestTLSDetail(t *testing.T) {
	st := handshake(t, tlstest.Pipe(&validCert, nil), "example.com")
	d := st.Detail
	if d == nil || !strings.HasPrefix(d.TLSVersion, "TLS 1.") || d.CipherSuite == "" || d.ALPN != "http/1.1" {
		t.Fatalf("detail = %+v", d)
	}
	if d.Certificate == nil || !strings.Contains(strings.Join(d.Certificate.Names, ","), "example.com") || !strings.Contains(d.Certificate.Issuer, "ConnectDoctor test CA") {
		t.Errorf("certificate = %+v", d.Certificate)
	}

	// ⚠ The expiry date is shown for an expired certificate: that is the useful part.
	expired, _ := testCA.Leaf([]string{"example.com"}, time.Now().Add(-48*time.Hour), time.Now().Add(-24*time.Hour))
	st = handshake(t, tlstest.Pipe(&expired, nil), "example.com")
	if st.Detail == nil || st.Detail.Certificate == nil || st.Detail.Certificate.NotAfter == "" {
		t.Errorf("expired: detail = %+v, want the certificate's dates", st.Detail)
	}
}

func TestTLSErrorClassification(t *testing.T) {
	cases := []struct {
		err  error
		code string
	}{
		{context.DeadlineExceeded, "tls.timeout"},
		{&net.OpError{Op: "read", Err: timeoutErr{}}, "tls.timeout"},
		{tls.AlertError(40), "tls.handshake_failed"},
		{x509.CertificateInvalidError{Reason: x509.Expired}, "tls.cert_expired"},
		{x509.CertificateInvalidError{Reason: x509.NotAuthorizedToSign}, "tls.handshake_failed"},
	}
	for _, c := range cases {
		if st := classifyTLSError(c.err); st.Code != c.code {
			t.Errorf("%v: %s, want %s", c.err, st.Code, c.code)
		}
	}
	// ⚠ An unrecognised error keeps its raw text.
	if st := classifyTLSError(x509.CertificateInvalidError{Reason: x509.NotAuthorizedToSign}); st.Detail == nil || st.Detail.Error == "" {
		t.Errorf("unclassified error lost its text: %+v", st.Detail)
	}
}

func TestTLSFailureConcludes(t *testing.T) {
	expired, _ := testCA.Leaf([]string{"example.com"}, time.Now().Add(-48*time.Hour), time.Now().Add(-24*time.Hour))
	d := func(context.Context, netip.Addr, uint16) (net.Conn, error) { return tlstest.Pipe(&expired, nil), nil }
	res := (&Checker{Resolver: &fakeResolver{addrs: []string{"93.184.215.14"}}, Dial: d}).Check(context.Background(), "https://example.com/")
	if got := statuses(res.Hops[0]); got != "dns=ok tcp=ok tls=failed http=skipped" {
		t.Errorf("steps = %s", got)
	}
	if res.Conclusion.FailedStep != StepTLS || res.Conclusion.Code != "tls.cert_expired" || !strings.Contains(res.Conclusion.Summary, "TLSハンドシェイク") {
		t.Errorf("conclusion = %+v", res.Conclusion)
	}
}

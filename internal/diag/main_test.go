package diag

import (
	"crypto/tls"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/hidetzu/connect-doctor/internal/tlstest"
)

// testCA is the only CA this package's tests trust, through the system root
// store (SSL_CERT_FILE / SSL_CERT_DIR), set before anything loads the roots.
var (
	testCA    *tlstest.CA
	validCert tls.Certificate
)

func TestMain(m *testing.M) {
	var err error
	if testCA, err = tlstest.NewCA("ConnectDoctor test CA"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	validCert, _ = testCA.Valid("example.com", "93.184.215.14")
	dir, _ := os.MkdirTemp("", "diag-roots-")
	env, err := tlstest.TrustOnly(testCA, dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	for _, kv := range env {
		k, v, _ := strings.Cut(kv, "=")
		os.Setenv(k, v)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

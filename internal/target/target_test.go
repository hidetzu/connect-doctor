package target

import (
	"errors"
	"strings"
	"testing"
)

func TestParseRefuses(t *testing.T) {
	cases := map[string]string{
		"":            CodeMissing,
		"   ":         CodeMissing,
		"not a url":   CodeMalformed,
		"example.com": CodeMalformed,
		"https://":    CodeMalformed,
		"https://" + strings.Repeat("a", 2100) + ".com/": CodeMalformed,
		"file:///etc/passwd":                             CodeUnsupportedScheme,
		"ftp://example.com/":                             CodeUnsupportedScheme,
		"gopher://example.com/":                          CodeUnsupportedScheme,
		"mailto:a@example.com":                           CodeUnsupportedScheme,
		"http://example.com:8080/":                       CodeUnsupportedPort,
		"https://example.com:22/":                        CodeUnsupportedPort,
		"http://example.com:0080/":                       CodeUnsupportedPort,
		"https://user:pass@example.com/":                 CodeCredentials,
		"https://user@example.com/":                      CodeCredentials,
		"http://localhost/":                              CodeLocalName,
		"http://LOCALHOST./":                             CodeLocalName,
		"http://foo.localhost/":                          CodeLocalName,
		"http://printer.local/":                          CodeLocalName,
		"http://router.home.arpa/":                       CodeLocalName,
		"http://metadata.google.internal/":               CodeLocalName,
		"http://intranet/":                               CodeLocalName,
		"http://2130706433/":                             CodeMalformed,
		"http://0x7f.1/":                                 CodeMalformed,
		"http://0x7f000001/":                             CodeMalformed,
		"http://0177.0.0.1/":                             CodeMalformed,
		"http://127.1/":                                  CodeMalformed,
		"http://exa mple.com/":                           CodeMalformed,
		"http://[fe80::1%25eth0]/":                       CodeMalformed,
		"http://127.0.0.1/":                              CodeRefusedAddress,
		"http://[::1]/":                                  CodeRefusedAddress,
		"http://[::ffff:127.0.0.1]/":                     CodeRefusedAddress,
		"http://169.254.169.254/latest/":                 CodeRefusedAddress,
		"http://10.0.0.1/":                               CodeRefusedAddress,
		"https://日本語.jp/":                                CodeIDNNotImplemented,
	}
	for in, want := range cases {
		_, err := Parse(in)
		var r *Refusal
		if !errors.As(err, &r) {
			t.Errorf("Parse(%q) err = %v, want refusal %s", in, err, want)
			continue
		}
		if r.Code != want {
			t.Errorf("Parse(%q) = %s, want %s", in, r.Code, want)
		}
	}
}

func TestParseAccepts(t *testing.T) {
	cases := []struct{ in, url, host, port string }{
		{"https://example.com", "https://example.com/", "example.com", "443"},
		{"  HTTPS://Example.COM/a?b=c#frag ", "https://example.com/a?b=c", "example.com", "443"},
		{"http://example.com:80/x?y", "http://example.com:80/x?y", "example.com", "80"},
		{"https://example.com:443", "https://example.com:443/", "example.com", "443"},
		{"http://example.com:443/", "http://example.com:443/", "example.com", "443"},
		{"https://example.com./", "https://example.com/", "example.com", "443"},
		{"https://93.184.215.14/", "https://93.184.215.14/", "93.184.215.14", "443"},
		{"https://[2606:4700:4700::1111]/", "https://[2606:4700:4700::1111]/", "2606:4700:4700::1111", "443"},
		{"https://ok.test/", "https://ok.test/", "ok.test", "443"},
	}
	for _, c := range cases {
		got, err := Parse(c.in)
		if err != nil {
			t.Errorf("Parse(%q) err = %v", c.in, err)
			continue
		}
		if got.URL != c.url || got.Host != c.host || got.Port != c.port {
			t.Errorf("Parse(%q) = {%s %s %s}, want {%s %s %s}", c.in, got.URL, got.Host, got.Port, c.url, c.host, c.port)
		}
	}
}

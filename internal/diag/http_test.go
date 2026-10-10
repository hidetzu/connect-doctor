package diag

import (
	"bufio"
	"context"
	"net"
	"net/http"
	"strings"
	"testing"

	"github.com/hidetzu/connect-doctor/internal/limits"
	"github.com/hidetzu/connect-doctor/internal/tlstest"
)

func get(t *testing.T, response []byte) Step {
	t.Helper()
	conn := tlstest.Serve(nil, response)
	defer conn.Close()
	return httpStep(context.Background(), conn, "http://example.com/path?q=1")
}

func TestHTTPOutcomes(t *testing.T) {
	big := "HTTP/1.1 200 OK\r\nX-Big: " + strings.Repeat("a", limits.ResponseHeaderBytes+10) + "\r\n\r\n"
	cases := []struct {
		name     string
		response []byte
		status   Status
		code     string
	}{
		{"200", tlstest.OK200, StatusOK, ""},
		{"503 is ok at this layer", []byte("HTTP/1.1 503 Service Unavailable\r\nContent-Length: 0\r\n\r\n"), StatusOK, ""},
		{"404 is ok at this layer", []byte("HTTP/1.1 404 Not Found\r\nContent-Length: 0\r\n\r\n"), StatusOK, ""},
		{"headers over the cap", []byte(big), StatusFailed, "http.malformed_response"},
		{"not HTTP", []byte("SSH-2.0-OpenSSH_9.6\r\n\r\n"), StatusFailed, "http.malformed_response"},
		// ⚠ Owner decision (#4): nothing arrived is not malformed.
		{"closed without a response", nil, StatusFailed, "http.no_response"},
	}
	for _, c := range cases {
		st := get(t, c.response)
		if st.Status != c.status || st.Code != c.code {
			t.Errorf("%s: %s/%s, want %s/%s (%+v)", c.name, st.Status, st.Code, c.status, c.code, st.Detail)
		}
	}
}

func TestHTTPDetailAndBodyCap(t *testing.T) {
	st := get(t, []byte("HTTP/1.1 503 Service Unavailable\r\nServer: test\r\nContent-Type: text/plain\r\nX-Secret: no\r\nContent-Length: 2\r\n\r\nhi"))
	d := st.Detail
	if d.StatusCode != 503 || d.Protocol != "HTTP/1.1" || d.Headers["Server"] != "test" || d.BodyBytesRead != 2 || d.BodyTruncated {
		t.Errorf("detail = %+v", d)
	}
	if _, ok := d.Headers["X-Secret"]; ok {
		t.Error("a header outside the shown subset was kept")
	}

	// ⚠ The claim is about what is read from the wire, not what the result
	// says: count the bytes the step actually took off the connection.
	body := strings.Repeat("x", 10<<20) // 10 MiB
	conn := &countingConn{Conn: tlstest.Serve(nil, []byte("HTTP/1.1 200 OK\r\nContent-Length: 10485760\r\n\r\n"+body))}
	defer conn.Close()
	st = httpStep(context.Background(), conn, "http://example.com/")
	if st.Status != StatusOK || st.Detail.BodyBytesRead != limits.ResponseBodyBytes || !st.Detail.BodyTruncated {
		t.Errorf("10 MiB body: %+v, want ok, %d bytes read, truncated", st.Detail, limits.ResponseBodyBytes)
	}
	if max := int64(limits.ResponseHeaderBytes + limits.ResponseBodyBytes + 1); conn.read > max {
		t.Errorf("read %d bytes off the connection, want at most %d", conn.read, max)
	}
}

// AC 2: the request names us and asks for the connection to close.
func TestHTTPRequestHeaders(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	got := make(chan *http.Request, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		req, err := http.ReadRequest(bufio.NewReader(c))
		if err == nil {
			got <- req
		}
		_, _ = c.Write(tlstest.OK200)
	}()
	conn, _ := net.Dial("tcp", ln.Addr().String())
	defer conn.Close()
	httpStep(context.Background(), conn, "https://example.com/path?q=1")
	req := <-got
	if req.Method != "GET" || req.Host != "example.com" || req.RequestURI != "/path?q=1" {
		t.Errorf("request line/host: %s %s host=%s", req.Method, req.RequestURI, req.Host)
	}
	if ua := req.UserAgent(); !strings.Contains(ua, "ConnectDoctor") || !strings.Contains(ua, "github.com/hidetzu/connect-doctor") {
		t.Errorf("User-Agent = %q", ua)
	}
	if !req.Close {
		t.Error("Connection: close was not sent")
	}
}

func TestHTTPErrorClassification(t *testing.T) {
	if st := classifyHTTPError(context.DeadlineExceeded, 0, false); st.Code != "http.timeout" {
		t.Errorf("deadline: %s", st.Code)
	}
	if st := classifyHTTPError(&net.OpError{Op: "read", Err: timeoutErr{}}, 10, false); st.Code != "http.timeout" {
		t.Errorf("net timeout: %s", st.Code)
	}
}

func TestStatusWordsInConclusion(t *testing.T) {
	for code, want := range map[int]string{200: "ステータス200を返しました", 503: "サーバ側（アプリケーション）", 404: "URLのパス", 301: "リダイレクト"} {
		if s := summaryOK(code, true); !strings.Contains(s, want) || !strings.Contains(s, "DNS・TCP・TLS・HTTP") {
			t.Errorf("%d: %s", code, s)
		}
	}
	if s := summaryOK(200, false); !strings.Contains(s, "DNS・TCP・HTTP") {
		t.Errorf("http:// summary: %s", s)
	}
}

type countingConn struct {
	net.Conn
	read int64
}

func (c *countingConn) Read(b []byte) (int, error) {
	n, err := c.Conn.Read(b)
	c.read += int64(n)
	return n, err
}

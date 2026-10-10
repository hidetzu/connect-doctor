package diag

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"syscall"

	"github.com/hidetzu/connect-doctor/internal/limits"
)

// UserAgent names ConnectDoctor and where to read about it, so a target's
// operator can tell what connected (hidetzu/connect-doctor#4 AC 2).
const UserAgent = "ConnectDoctor/0.1 (+https://github.com/hidetzu/connect-doctor)"

// headersShown are the response headers worth showing; the rest are not
// read into the result.
var headersShown = []string{"Server", "Content-Type", "Content-Length", "Location"}

// httpStep writes one GET on conn and reads the status line and headers,
// then at most limits.ResponseBodyBytes of body (docs/adr/0005).
//
// ⚠ No http.Client, no Transport: the request goes on the connection the
// earlier steps opened, so a failure here belongs to this step alone.
func httpStep(ctx context.Context, conn net.Conn, rawURL string) Step {
	ctx, cancel := context.WithTimeout(ctx, limits.HTTP)
	defer cancel()
	if dl, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(dl)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		// target.Parse produced rawURL; this cannot happen with its output.
		return Step{Step: StepHTTP, Status: StatusFailed, Code: "http.malformed_response", Detail: &Detail{Error: err.Error()}}
	}
	req.Header.Set("User-Agent", UserAgent)
	req.Close = true // Connection: close

	// ⚠ The header budget is enforced by the reader itself, so an endless
	// header cannot be read past the cap.
	lr := &io.LimitedReader{R: conn, N: limits.ResponseHeaderBytes}
	br := bufio.NewReader(lr)

	if err := req.Write(conn); err != nil {
		return classifyHTTPError(err, 0, false)
	}
	resp, err := http.ReadResponse(br, req)
	if err != nil {
		// Bytes that arrived from the server, parsed or still buffered.
		return classifyHTTPError(err, limits.ResponseHeaderBytes-lr.N, lr.N == 0)
	}
	defer resp.Body.Close()

	// Now the body budget, on the same reader: one byte more than the cap,
	// to know whether it was cut. ⚠ This reader is the only cap; nothing
	// above it limits again (CLAUDE.md § 3).
	lr.N = limits.ResponseBodyBytes + 1
	n, _ := io.Copy(io.Discard, resp.Body)

	d := &Detail{
		StatusCode:    resp.StatusCode,
		Status:        resp.Status,
		Protocol:      resp.Proto,
		BodyBytesRead: min(n, limits.ResponseBodyBytes),
		BodyTruncated: n > limits.ResponseBodyBytes,
	}
	for _, h := range headersShown {
		if v := resp.Header.Get(h); v != "" {
			if d.Headers == nil {
				d.Headers = map[string]string{}
			}
			d.Headers[h] = v
		}
	}
	return Step{Step: StepHTTP, Status: StatusOK, Detail: d}
}

// classifyHTTPError maps a failure to one outcome (.claude/rules/go.md).
// read is how many response bytes arrived; capped is whether the header
// budget ran out.
func classifyHTTPError(err error, read int64, capped bool) Step {
	s := Step{Step: StepHTTP, Status: StatusFailed}
	var ne net.Error
	switch {
	case capped:
		s.Code = "http.malformed_response"
		s.Detail = &Detail{Error: "response headers exceeded the read limit"}
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &ne) && ne.Timeout():
		s.Code = "http.timeout"
	case read == 0 && (errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) ||
		errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.EPIPE) || errors.Is(err, net.ErrClosed)):
		// ⚠ Owner decision (hidetzu/connect-doctor#4): nothing arrived is not
		// malformed (.claude/rules/evidence.md).
		s.Code = "http.no_response"
	default:
		s.Code = "http.malformed_response"
		s.Detail = &Detail{Error: err.Error()}
	}
	return s
}

package diag

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/netip"
	"os"
	"strings"
	"syscall"
	"testing"

	"github.com/hidetzu/connect-doctor/internal/dial"
)

// opErr wraps an errno the way net.Dialer does.
func opErr(errno syscall.Errno) error {
	return &net.OpError{Op: "dial", Net: "tcp", Err: os.NewSyscallError("connect", errno)}
}

type timeoutErr struct{}

func (timeoutErr) Error() string   { return "i/o timeout" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return true }

// scriptedDial answers per address; an address not in the script connects.
type scriptedDial struct {
	errs  map[string]error
	tried []string
}

func (d *scriptedDial) dial(_ context.Context, a netip.Addr, _ uint16) (net.Conn, error) {
	d.tried = append(d.tried, a.String())
	if err, ok := d.errs[a.String()]; ok {
		return nil, err
	}
	c, _ := net.Pipe()
	return c, nil
}

func checkTCP(addrs []string, errs map[string]error) (Result, *scriptedDial) {
	d := &scriptedDial{errs: errs}
	r := &fakeResolver{addrs: addrs}
	return (&Checker{Resolver: r, Dial: d.dial}).Check(context.Background(), "https://example.com/"), d
}

func TestTCPOutcomes(t *testing.T) {
	v4, v4b, v6 := "93.184.215.14", "93.184.215.15", "2606:4700:4700::1111"
	cases := []struct {
		name   string
		addrs  []string
		errs   map[string]error
		status Status
		code   string
	}{
		{"refused", []string{v4}, map[string]error{v4: opErr(syscall.ECONNREFUSED)}, StatusFailed, "tcp.refused"},
		{"timeout", []string{v4}, map[string]error{v4: &net.OpError{Op: "dial", Err: timeoutErr{}}}, StatusFailed, "tcp.timeout"},
		{"deadline", []string{v4}, map[string]error{v4: context.DeadlineExceeded}, StatusFailed, "tcp.timeout"},
		{"host unreachable", []string{v4}, map[string]error{v4: opErr(syscall.EHOSTUNREACH)}, StatusFailed, "tcp.unreachable"},
		{"v4 net unreachable", []string{v4}, map[string]error{v4: opErr(syscall.ENETUNREACH)}, StatusFailed, "tcp.unreachable"},
		{"v6 only, no route", []string{v6}, map[string]error{v6: opErr(syscall.ENETUNREACH)}, StatusFailed, "tcp.no_route_family"},
		{"policy at the socket", []string{v4}, map[string]error{v4: &net.OpError{Op: "dial", Err: dial.ErrRefused}}, StatusRefused, "tcp.refused_address"},
		{"unknown", []string{v4}, map[string]error{v4: errors.New("weird")}, StatusFailed, "tcp.failed"},
		// ⚠ Owner decision (#2): the first attempted address concludes.
		{"first wins: v4 refused, v6 timeout", []string{v4, v6}, map[string]error{v4: opErr(syscall.ECONNREFUSED), v6: context.DeadlineExceeded}, StatusFailed, "tcp.refused"},
		{"first wins: v4 timeout, v4b refused", []string{v4, v4b}, map[string]error{v4: context.DeadlineExceeded, v4b: opErr(syscall.ECONNREFUSED)}, StatusFailed, "tcp.timeout"},
		{"v4 no route + v6 no route is not a family gap", []string{v4, v6}, map[string]error{v4: opErr(syscall.ENETUNREACH), v6: opErr(syscall.ENETUNREACH)}, StatusFailed, "tcp.unreachable"},
	}
	for _, c := range cases {
		res, _ := checkTCP(c.addrs, c.errs)
		tcp := res.Hops[0].Steps[1]
		if tcp.Status != c.status || tcp.Code != c.code {
			t.Errorf("%s: tcp = %s/%s, want %s/%s", c.name, tcp.Status, tcp.Code, c.status, c.code)
		}
		if res.Conclusion.FailedStep != StepTCP || res.Conclusion.Code != c.code || tcp.Message == "" {
			t.Errorf("%s: conclusion = %+v", c.name, res.Conclusion)
		}
		if got := statuses(res.Hops[0]); !strings.HasSuffix(got, "tls=skipped http=skipped") {
			t.Errorf("%s: steps = %s", c.name, got)
		}
		if len(tcp.Detail.Attempts) != len(c.addrs) {
			t.Errorf("%s: %d attempts recorded, want %d", c.name, len(tcp.Detail.Attempts), len(c.addrs))
		}
	}
}

func TestTCPSecondAddressConnects(t *testing.T) {
	v4, v6 := "93.184.215.14", "2606:4700:4700::1111"
	res, d := checkTCP([]string{v6, v4}, map[string]error{v4: context.DeadlineExceeded})
	tcp := res.Hops[0].Steps[1]
	if strings.Join(d.tried, ",") != v4+","+v6 {
		t.Errorf("tried %v, want IPv4 first", d.tried)
	}
	if tcp.Status != StatusOK || tcp.Detail.Address != "["+v6+"]:443" {
		t.Fatalf("tcp = %+v", tcp)
	}
	a := tcp.Detail.Attempts
	if len(a) != 2 || a[0].Outcome != "tcp.timeout" || a[1].Outcome != "ok" {
		t.Errorf("attempts = %+v", a)
	}
	if res.Conclusion.Status != ConclusionIncomplete {
		t.Errorf("conclusion = %+v", res.Conclusion)
	}
}

func TestUnknownErrorKeepsItsText(t *testing.T) {
	res, _ := checkTCP([]string{"93.184.215.14"}, map[string]error{"93.184.215.14": errors.New("weird thing")})
	if a := res.Hops[0].Steps[1].Detail.Attempts[0]; a.Error != "weird thing" {
		t.Errorf("attempt = %+v, want the raw error kept", a)
	}
}

func TestNilDialPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("Check with no Dialer did not panic")
		}
	}()
	(&Checker{Resolver: &fakeResolver{addrs: []string{"93.184.215.14"}}}).Check(context.Background(), "https://example.com/")
}

// ⚠ An address refused at the socket is never shown, like one refused by DNS.
func TestRefusedAtSocketIsNotShown(t *testing.T) {
	v4 := "93.184.215.14"
	res, _ := checkTCP([]string{v4}, map[string]error{v4: &net.OpError{Op: "dial", Err: dial.ErrRefused}})
	tcp := res.Hops[0].Steps[1]
	if tcp.Code != "tcp.refused_address" {
		t.Fatalf("tcp = %+v", tcp)
	}
	b, _ := json.Marshal(tcp)
	if strings.Contains(string(b), v4) {
		t.Errorf("refused TCP step shows the address: %s", b)
	}
}

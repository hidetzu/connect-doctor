package main

import "testing"

func TestListenAddr(t *testing.T) {
	cases := []struct {
		given      bool
		addr, port string
		want       string
	}{
		{false, "127.0.0.1:8080", "", "127.0.0.1:8080"},
		{false, "127.0.0.1:8080", "9000", "0.0.0.0:9000"},
		// ⚠ An explicit -addr wins over PORT: the final gate relies on it.
		{true, "127.0.0.1:0", "9000", "127.0.0.1:0"},
	}
	for _, c := range cases {
		if got := listenAddr(c.given, c.addr, c.port); got != c.want {
			t.Errorf("listenAddr(%v, %q, %q) = %q, want %q", c.given, c.addr, c.port, got, c.want)
		}
	}
}

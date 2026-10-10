// Command connect-doctor serves the ConnectDoctor page and JSON API.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/hidetzu/connect-doctor/internal/diag"
	"github.com/hidetzu/connect-doctor/internal/dial"
	"github.com/hidetzu/connect-doctor/internal/limits"
	"github.com/hidetzu/connect-doctor/internal/server"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "listen address (port 0 picks a free one)")
	dnsServer := flag.String("dns-server", "", "resolver to use, host:port (default: the system's resolvers)")
	flag.Parse()

	logger := log.New(os.Stderr, "connect-doctor: ", log.LstdFlags)

	// The pure-Go resolver: the same code path on every platform, and no
	// cgo getaddrinfo, whose parsing of numeric host names differs.
	resolver := &net.Resolver{PreferGo: true}
	if *dnsServer != "" {
		// ⚠ The resolver address is the operator's configuration, not user
		// input, so it is dialled directly rather than through the policy:
		// a resolver on loopback is normal. User-chosen targets never reach
		// this dialer.
		server := *dnsServer
		resolver.Dial = func(ctx context.Context, network, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, network, server)
		}
	}

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		logger.Fatal(err)
	}
	// ⚠ The final gate reads this line to find the port.
	fmt.Printf("connect-doctor: listening on http://%s\n", ln.Addr())

	srv := &http.Server{
		Handler:           server.New(&diag.Checker{Resolver: resolver, Dial: dial.TCP}, logger).Handler(),
		ReadHeaderTimeout: limits.ServerReadHeader,
		WriteTimeout:      limits.ServerWrite,
		ErrorLog:          logger,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		_ = srv.Shutdown(context.Background())
	}()
	if err := srv.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
		logger.Fatal(err)
	}
}

// graphlayer-investigate serves the offline, loopback-only ASA investigation UI.
package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/mattpainter701/graphlayer/platform/investigation"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	listen := flag.String("listen", "127.0.0.1:8088", "explicit numeric loopback address and port")
	dataDir := flag.String("data-dir", "", "directory for sanitized local snapshots and investigations")
	flag.Parse()
	if flag.NArg() != 0 {
		return fmt.Errorf("unexpected arguments")
	}
	if err := investigation.ValidateListenAddress(*listen); err != nil {
		return err
	}
	if *dataDir == "" {
		base, err := os.UserConfigDir()
		if err != nil {
			return fmt.Errorf("set --data-dir to a private workspace directory")
		}
		*dataDir = filepath.Join(base, "GraphLayer", "investigations")
	}
	store, err := investigation.NewStore(*dataDir)
	if err != nil {
		return fmt.Errorf("cannot open local workspace: %w", err)
	}
	defer store.Close()
	listener, err := net.Listen("tcp", *listen)
	if err != nil {
		return fmt.Errorf("cannot listen on the requested loopback address")
	}
	defer listener.Close()
	app, err := investigation.NewHandler(store, listener.Addr().String())
	if err != nil {
		return err
	}
	server := investigation.HTTPServer(listener.Addr().String(), app)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	completed := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = server.Shutdown(shutdown)
		case <-completed:
		}
	}()
	fmt.Printf("Listening on http://%s\n", listener.Addr().String())
	fmt.Println("Local single-user workspace. ACL policy only; routing, NAT and return paths are not evaluated.")
	err = server.Serve(listener)
	close(completed)
	if err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("local investigation server stopped unexpectedly")
	}
	return nil
}

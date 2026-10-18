package main

import (
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/mattpainter701/graphlayer/platform/investigation"
)

func TestCLIUsesListenerAddressAndBoundedHTTPServer(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	address := listener.Addr().String()
	if err := investigation.ValidateListenAddress(address); err != nil {
		t.Fatalf("actual listener address rejected: %v", err)
	}
	server := investigation.HTTPServer(address, http.NotFoundHandler())
	if server.Addr != address {
		t.Fatalf("server address = %q, want actual bound address %q", server.Addr, address)
	}
	if server.ReadHeaderTimeout <= 0 || server.ReadTimeout <= 0 || server.WriteTimeout <= 0 || server.IdleTimeout <= 0 || server.MaxHeaderBytes <= 0 {
		t.Fatalf("CLI server has unbounded HTTP settings: %#v", server)
	}
	if server.ReadHeaderTimeout > 10*time.Second || server.ReadTimeout > 30*time.Second || server.WriteTimeout > 60*time.Second {
		t.Fatalf("unexpectedly loose HTTP timeout bounds: %#v", server)
	}
}

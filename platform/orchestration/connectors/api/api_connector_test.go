package api

import (
	"bytes"
	"context"
	"encoding/pem"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mattpainter701/graphlayer/platform/orchestration/connectors/base"
	"github.com/mattpainter701/graphlayer/platform/orchestration/types"
)

func TestAPIConnectorConnectSetsConnectedState(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	config, cleanup := newTLSConfigForHandler(t, handler, 3)

	connector, err := NewAPIConnector(config, types.DeviceTypeGenericAPI)
	if err != nil {
		cleanup()
		t.Fatalf("failed to create connector: %v", err)
	}
	defer cleanup()
	defer connector.Disconnect(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := connector.Connect(ctx); err != nil {
		t.Fatalf("connect returned error: %v", err)
	}

	if !connector.IsConnected() {
		t.Fatalf("expected connector to be connected")
	}

	if state := connector.GetConnectionState(); state != base.ConnectionStateConnected {
		t.Fatalf("unexpected connection state: %s", state)
	}
}

func TestAPIConnectorRetryClonesRequestBody(t *testing.T) {
	t.Parallel()

	var (
		mu     sync.Mutex
		bodies []string
	)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}

		mu.Lock()
		bodies = append(bodies, string(data))
		count := len(bodies)
		mu.Unlock()

		if count == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	})

	config, cleanup := newTLSConfigForHandler(t, handler, 2)

	connector, err := NewAPIConnector(config, types.DeviceTypeGenericAPI)
	if err != nil {
		cleanup()
		t.Fatalf("failed to create connector: %v", err)
	}
	defer cleanup()
	defer connector.Disconnect(context.Background())

	resp, err := connector.doRequest(context.Background(), http.MethodPost, "/payload", bytes.NewBufferString("payload-data"))
	if err != nil {
		t.Fatalf("doRequest failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status code: %d", resp.StatusCode)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(bodies) != 2 {
		t.Fatalf("expected 2 attempts, got %d", len(bodies))
	}

	for i, body := range bodies {
		if body != "payload-data" {
			t.Fatalf("attempt %d body mismatch: %q", i+1, body)
		}
	}
}

func newTLSConfigForHandler(t *testing.T, handler http.Handler, retryCount int) (*base.ConnectionConfig, func()) {
	t.Helper()

	server := httptest.NewTLSServer(handler)

	host, port := extractHostPort(t, server.URL)

	certDER := server.TLS.Certificates[0].Certificate[0]
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	certPath := filepath.Join(t.TempDir(), "server-cert.pem")
	if err := os.WriteFile(certPath, certPEM, 0600); err != nil {
		t.Fatalf("failed to write certificate: %v", err)
	}

	config := &base.ConnectionConfig{
		Host:       host,
		Port:       port,
		Timeout:    5 * time.Second,
		RetryCount: retryCount,
		RetryDelay: 10 * time.Millisecond,
		CACert:     certPath,
	}

	cleanup := func() {
		server.Close()
	}

	return config, cleanup
}

func extractHostPort(t *testing.T, rawURL string) (string, int) {
	t.Helper()

	trimmed := strings.TrimPrefix(rawURL, "https://")
	host, portStr, err := net.SplitHostPort(trimmed)
	if err != nil {
		t.Fatalf("failed to parse test server URL: %v", err)
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("failed to convert port: %v", err)
	}

	return host, port
}

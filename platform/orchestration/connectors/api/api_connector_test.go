package api

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
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
		if r.URL.Path != "/payload" {
			w.WriteHeader(http.StatusOK)
			return
		}
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
		if r.URL.Path != "/payload" {
			w.WriteHeader(http.StatusOK)
			return
		}
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
	connectCtx, connectCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer connectCancel()
	if err := connector.Connect(connectCtx); err != nil {
		t.Fatalf("connect returned error: %v", err)
	}

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

func TestReadRequestBodyEnforcesLimit(t *testing.T) {
	withinLimit := bytes.Repeat([]byte("x"), int(maxAPIRequestBodyBytes))
	got, err := readRequestBody(bytes.NewReader(withinLimit))
	if err != nil || !bytes.Equal(got, withinLimit) {
		t.Fatalf("body at size limit was rejected: got %d bytes, err %v", len(got), err)
	}

	tooLarge := bytes.Repeat([]byte("x"), int(maxAPIRequestBodyBytes+1))
	_, err = readRequestBody(bytes.NewReader(tooLarge))
	if err == nil || !strings.Contains(err.Error(), "size limit") {
		t.Fatalf("oversized body was not rejected: %v", err)
	}
}

type failingBodyReader struct{}

func (failingBodyReader) Read([]byte) (int, error) { return 0, fmt.Errorf("secret body detail") }

func TestReadRequestBodyDoesNotExposeReaderError(t *testing.T) {
	_, err := readRequestBody(failingBodyReader{})
	if err == nil || strings.Contains(err.Error(), "secret body detail") {
		t.Fatalf("reader error leaked input details: %v", err)
	}
}

func newTLSConfigForHandler(t *testing.T, handler http.Handler, retryCount int) (*base.ConnectionConfig, func()) {
	t.Helper()

	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate test CA key")
	}
	now := time.Now()
	caTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "GraphLayer test CA"},
		NotBefore:    now.Add(-time.Minute), NotAfter: now.Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("failed to create test CA certificate")
	}
	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate test server key")
	}
	leafTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "127.0.0.1"},
		NotBefore:    now.Add(-time.Minute), NotAfter: now.Add(time.Hour),
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTemplate, caTemplate, &leafKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("failed to create test server certificate")
	}
	leafCert, err := x509.ParseCertificate(leafDER)
	if err != nil {
		t.Fatalf("failed to parse test server certificate")
	}
	serverKey, err := tls.X509KeyPair(
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER}),
		mustMarshalECPrivateKey(t, leafKey),
	)
	if err != nil {
		t.Fatalf("failed to load test server certificate")
	}
	server := httptest.NewUnstartedServer(handler)
	server.TLS = &tls.Config{Certificates: []tls.Certificate{serverKey}, MinVersion: tls.VersionTLS12}
	server.StartTLS()

	host, port := extractHostPort(t, server.URL)
	if host != "127.0.0.1" || leafCert.IsCA {
		t.Fatalf("invalid non-CA loopback test certificate")
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})

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

func mustMarshalECPrivateKey(t *testing.T, key *ecdsa.PrivateKey) []byte {
	t.Helper()
	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("failed to marshal test server key")
	}
	return pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der})
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

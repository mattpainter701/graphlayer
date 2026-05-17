package ssh

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	gossh "golang.org/x/crypto/ssh"

	"github.com/mattpainter701/graphlayer/platform/orchestration/connectors/base"
	"github.com/mattpainter701/graphlayer/platform/orchestration/types"
)

func TestSSHConnectorConnectSetsConnectedState(t *testing.T) {
	t.Parallel()

	const (
		username = "tester"
		password = "secret"
	)

	listener, serverConfig := newTestSSHServer(t, username, password)
	defer listener.Close()

	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		acceptTestSSHConnections(listener, serverConfig, ctx.Done())
	}()
	defer func() {
		cancel()
		wg.Wait()
	}()

	host, portStr, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatalf("failed to parse listener address: %v", err)
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("failed to parse port: %v", err)
	}

	config := &base.ConnectionConfig{
		Host:       host,
		Port:       port,
		Username:   username,
		Password:   password,
		Timeout:    5 * time.Second,
		RetryCount: 1,
		RetryDelay: 100 * time.Millisecond,
	}

	connector, err := NewSSHConnector(config, types.DeviceTypeGenericSSH)
	if err != nil {
		t.Fatalf("failed to create connector: %v", err)
	}

	connectCtx, connectCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer connectCancel()

	if err := connector.Connect(connectCtx); err != nil {
		t.Fatalf("connect returned error: %v", err)
	}
	defer connector.Disconnect(context.Background())

	if !connector.IsConnected() {
		t.Fatalf("expected connector to be connected")
	}

	if state := connector.GetConnectionState(); state != base.ConnectionStateConnected {
		t.Fatalf("unexpected connection state: %s", state)
	}
}

func newTestSSHServer(t *testing.T, username, password string) (net.Listener, *gossh.ServerConfig) {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate host key: %v", err)
	}

	signer, err := gossh.NewSignerFromKey(privateKey)
	if err != nil {
		t.Fatalf("failed to create host key signer: %v", err)
	}

	config := &gossh.ServerConfig{
		PasswordCallback: func(conn gossh.ConnMetadata, pass []byte) (*gossh.Permissions, error) {
			if conn.User() == username && string(pass) == password {
				return nil, nil
			}
			return nil, fmt.Errorf("password rejected for %s", conn.User())
		},
	}
	config.AddHostKey(signer)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen for test ssh server: %v", err)
	}

	return listener, config
}

func acceptTestSSHConnections(listener net.Listener, config *gossh.ServerConfig, done <-chan struct{}) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-done:
				return
			default:
				continue
			}
		}

		go func(c net.Conn) {
			defer c.Close()

			sshConn, chans, reqs, err := gossh.NewServerConn(c, config)
			if err != nil {
				return
			}
			defer sshConn.Close()

			go func() {
				for req := range reqs {
					if req.Type == "keepalive@openssh.com" {
						req.Reply(true, nil)
					} else {
						req.Reply(false, nil)
					}
				}
			}()

			go func() {
				for ch := range chans {
					ch.Reject(gossh.UnknownChannelType, "channel not supported")
				}
			}()
		}(conn)
	}
}

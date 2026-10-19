package ssh

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

	listener, serverConfig, hostKey := newTestSSHServer(t, username, password)
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
		_ = listener.Close()
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

	knownHosts := filepath.Join(t.TempDir(), "known_hosts")
	knownHostLine := fmt.Sprintf("[%s]:%d %s", host, port, gossh.MarshalAuthorizedKey(hostKey))
	if err := os.WriteFile(knownHosts, []byte(knownHostLine), 0600); err != nil {
		t.Fatalf("failed to write fixture known_hosts: %v", err)
	}

	config := &base.ConnectionConfig{
		Host:       host,
		Port:       port,
		Username:   username,
		Password:   password,
		Timeout:    5 * time.Second,
		RetryCount: 1,
		RetryDelay: 100 * time.Millisecond,
		Options:    map[string]interface{}{"known_hosts_path": knownHosts},
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

	result, err := connector.ExecuteCommand(ctx, "show version", 2*time.Second)
	if err != nil {
		t.Fatalf("ExecuteCommand returned error: %v", err)
	}
	if result == nil || !strings.Contains(result.Output, "fixture output") {
		t.Fatalf("expected parsed SSH fixture output, got %#v", result)
	}
}

func newTestSSHServer(t *testing.T, username, password string) (net.Listener, *gossh.ServerConfig, gossh.PublicKey) {
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

	return listener, config, signer.PublicKey()
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
					if ch.ChannelType() != "session" {
						ch.Reject(gossh.UnknownChannelType, "channel not supported")
						continue
					}
					channel, requests, err := ch.Accept()
					if err != nil {
						continue
					}
					go func() {
						defer channel.Close()
						for request := range requests {
							if request.Type != "exec" {
								request.Reply(false, nil)
								continue
							}
							request.Reply(true, nil)
							_, _ = channel.Write([]byte("fixture output\n"))
							_, _ = channel.SendRequest("exit-status", false, []byte{0, 0, 0, 0})
							return
						}
					}()
				}
			}()
		}(conn)
	}
}

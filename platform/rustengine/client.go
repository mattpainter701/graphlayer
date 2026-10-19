// Package rustengine is the bounded process transport to the required native
// Rust worker. It contains no parser, connector, or investigation decisions.
package rustengine

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

const Protocol = "graphlayer-engine-v1"
const MaxInput = 160 << 20
const MaxOutput = 8 << 20

var ErrUnavailable = errors.New("required Rust analysis engine is unavailable")

// Set only by same-package protocol tests; never populated from configuration.
var workerArguments []string

type request struct {
	ProtocolVersion string `json:"protocol_version"`
	Operation       string `json:"operation"`
	Input           any    `json:"input"`
}
type response struct {
	ProtocolVersion string          `json:"protocol_version"`
	Operation       string          `json:"operation"`
	Result          json.RawMessage `json:"result,omitempty"`
	Error           string          `json:"error,omitempty"`
	ErrorKind       string          `json:"error_kind,omitempty"`
}

func unavailable(reason string) error { return fmt.Errorf("%w: %s", ErrUnavailable, reason) }

// Path resolves only an explicitly configured absolute executable or the
// packaged sibling. It never searches PATH or invokes a shell.
func Path() (string, error) {
	path := os.Getenv("GRAPHLAYER_ENGINE")
	if path != "" && !filepath.IsAbs(path) {
		return "", unavailable("worker path must be absolute")
	}
	if path == "" {
		executable, err := os.Executable()
		if err != nil {
			return "", unavailable("worker executable is unavailable")
		}
		name := "graphlayer-engine"
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		path = filepath.Join(filepath.Dir(executable), name)
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || (runtime.GOOS != "windows" && info.Mode().Perm()&0111 == 0) {
		return "", unavailable("worker executable is unavailable")
	}
	return path, nil
}

// Evaluate runs a one-shot domain request with a bounded execution deadline.
func Evaluate(operation string, input, out any) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return Call(ctx, operation, input, out)
}

// Validate requires an explicit positive acknowledgement from a validator.
// An empty object or a false result is a broken protocol, not validation success.
func Validate(operation string, input any) error {
	var result struct {
		Valid bool `json:"valid"`
	}
	if err := Evaluate(operation, input, &result); err != nil {
		return err
	}
	if !result.Valid {
		return unavailable("worker returned an incomplete validation result")
	}
	return nil
}

func encode(operation string, input any) ([]byte, error) {
	data, err := json.Marshal(request{Protocol, operation, input})
	if err != nil || len(data)+1 > MaxInput {
		return nil, unavailable("worker request exceeds protocol bounds")
	}
	return append(data, '\n'), nil
}

func Call(ctx context.Context, operation string, input, out any) error {
	path, err := Path()
	if err != nil {
		return err
	}
	data, err := encode(operation, input)
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, path, workerArguments...)
	cmd.WaitDelay = time.Second
	cmd.Stdin = bytes.NewReader(data)
	stdout := &outputBuffer{}
	cmd.Stdout = stdout
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		if stdout.overflow {
			return unavailable("worker output exceeds protocol bounds")
		}
		if ctx.Err() != nil {
			return unavailable("worker deadline exceeded")
		}
		return unavailable("worker process failed")
	}
	if stdout.overflow {
		return unavailable("worker output exceeds protocol bounds")
	}
	return decode(stdout.buffer.Bytes(), operation, out)
}

func decode(data []byte, operation string, out any) error {
	var result response
	if err := DecodeStrict(data, &result); err != nil {
		return unavailable("worker returned malformed output")
	}
	if result.ProtocolVersion != Protocol || result.Operation != operation {
		return unavailable("worker protocol mismatch")
	}
	if result.Error != "" {
		if len(result.Result) != 0 || len(result.Error) > 4096 {
			return unavailable("worker returned an invalid error")
		}
		if result.ErrorKind == "domain" {
			return errors.New(result.Error)
		}
		return unavailable("worker rejected the request")
	}
	if result.ErrorKind != "" || len(result.Result) == 0 || bytes.Equal(result.Result, []byte("null")) {
		return unavailable("worker returned an incomplete result")
	}
	if out != nil {
		if err := DecodeStrict(result.Result, out); err != nil {
			return unavailable("worker returned an invalid result")
		}
	}
	return nil
}

type outputBuffer struct {
	buffer   bytes.Buffer
	overflow bool
}

func (b *outputBuffer) Write(p []byte) (int, error) {
	remaining := MaxOutput - b.buffer.Len()
	if len(p) > remaining {
		_, _ = b.buffer.Write(p[:remaining])
		b.overflow = true
		return remaining, errors.New("output limit")
	}
	return b.buffer.Write(p)
}

// Session holds an explicitly scoped Rust connector worker. Credentials are
// sent through stdin, and all network sessions and parsing remain in Rust.
type Session struct {
	mu      sync.Mutex
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	scanner *bufio.Scanner
	closed  bool
}

func NewSession() (*Session, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}
	args := append(append([]string{}, workerArguments...), "--session")
	cmd := exec.Command(path, args...)
	cmd.WaitDelay = time.Second
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, unavailable("cannot open worker input")
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, unavailable("cannot open worker output")
	}
	cmd.Stderr = io.Discard
	if err = cmd.Start(); err != nil {
		_ = stdin.Close()
		return nil, unavailable("cannot start worker")
	}
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64<<10), MaxOutput+1)
	return &Session{cmd: cmd, stdin: stdin, scanner: scanner}, nil
}

func (s *Session) Call(ctx context.Context, operation string, input, out any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return unavailable("worker session is closed")
	}
	data, err := encode(operation, input)
	if err != nil {
		return err
	}
	finished := make(chan error, 1)
	go func() {
		if _, err := s.stdin.Write(data); err != nil {
			finished <- unavailable("worker input failed")
			return
		}
		if !s.scanner.Scan() {
			finished <- unavailable("worker session ended")
			return
		}
		finished <- decode(s.scanner.Bytes(), operation, out)
	}()
	select {
	case err := <-finished:
		if errors.Is(err, ErrUnavailable) {
			s.closeLocked()
		}
		return err
	case <-ctx.Done():
		s.closeLocked()
		<-finished
		return unavailable("worker deadline exceeded")
	}
}
func (s *Session) closeLocked() {
	if s.closed {
		return
	}
	s.closed = true
	_ = s.stdin.Close()
	_ = s.cmd.Process.Kill()
	_ = s.cmd.Wait()
}
func (s *Session) Close() error { s.mu.Lock(); defer s.mu.Unlock(); s.closeLocked(); return nil }

// DecodeStrict rejects ambiguous envelopes, duplicate fields and trailing data.
// It deliberately accepts scalar/array results as well as objects.
func DecodeStrict(data []byte, target any) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	first, err := d.Token()
	if err != nil {
		return err
	}
	if err = consume(d, first, 0); err != nil {
		return err
	}
	if _, err = d.Token(); err != io.EOF {
		return errors.New("trailing JSON")
	}
	d = json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err = d.Decode(target); err != nil {
		return err
	}
	return nil
}
func consume(d *json.Decoder, value json.Token, depth int) error {
	if depth > 128 {
		return errors.New("JSON nesting limit")
	}
	delimiter, ok := value.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		seen := map[string]bool{}
		for d.More() {
			key, err := d.Token()
			if err != nil {
				return err
			}
			name, ok := key.(string)
			if !ok || seen[name] {
				return errors.New("duplicate JSON field")
			}
			seen[name] = true
			value, err := d.Token()
			if err != nil {
				return err
			}
			if err = consume(d, value, depth+1); err != nil {
				return err
			}
		}
		closing, err := d.Token()
		if err != nil || closing != json.Delim('}') {
			return errors.New("unclosed JSON object")
		}
	case '[':
		for d.More() {
			value, err := d.Token()
			if err != nil {
				return err
			}
			if err = consume(d, value, depth+1); err != nil {
				return err
			}
		}
		closing, err := d.Token()
		if err != nil || closing != json.Delim(']') {
			return errors.New("unclosed JSON array")
		}
	default:
		return errors.New("invalid JSON delimiter")
	}
	return nil
}

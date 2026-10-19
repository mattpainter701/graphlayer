package rustengine

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

func TestWorkerProcess(t *testing.T) {
	if os.Getenv("GRAPHLAYER_PROTOCOL_TEST") != "1" {
		return
	}
	mode := os.Getenv("GRAPHLAYER_PROTOCOL_MODE")
	if mode == "timeout" {
		time.Sleep(time.Minute)
		os.Exit(0)
	}
	if mode == "overflow" || mode == "overflow_hang" {
		_, _ = io.Copy(os.Stdout, strings.NewReader(strings.Repeat("x", MaxOutput+1)))
		if mode == "overflow_hang" {
			time.Sleep(time.Minute)
		}
		os.Exit(0)
	}
	if mode == "failed" {
		_, _ = os.Stderr.WriteString("credential-canary")
		os.Exit(1)
	}
	decoder := json.NewDecoder(os.Stdin)
	for {
		var input request
		if decoder.Decode(&input) != nil {
			os.Exit(0)
		}
		switch mode {
		case "duplicate":
			_, _ = os.Stdout.WriteString(`{"protocol_version":"graphlayer-engine-v1","protocol_version":"wrong","operation":"probe","result":{}}` + "\n")
		case "trailing":
			_, _ = os.Stdout.WriteString(`{"protocol_version":"graphlayer-engine-v1","operation":"probe","result":{}} {}` + "\n")
		case "domain":
			_ = json.NewEncoder(os.Stdout).Encode(response{ProtocolVersion: Protocol, Operation: input.Operation, Error: "invalid flow source address", ErrorKind: "domain"})
		case "wrong_operation":
			_ = json.NewEncoder(os.Stdout).Encode(response{ProtocolVersion: Protocol, Operation: "wrong", Result: json.RawMessage(`{}`)})
		case "missing_validation":
			_ = json.NewEncoder(os.Stdout).Encode(response{ProtocolVersion: Protocol, Operation: input.Operation, Result: json.RawMessage(`{}`)})
		case "false_validation":
			_ = json.NewEncoder(os.Stdout).Encode(response{ProtocolVersion: Protocol, Operation: input.Operation, Result: json.RawMessage(`{"valid":false}`)})
		default:
			_ = json.NewEncoder(os.Stdout).Encode(response{ProtocolVersion: Protocol, Operation: input.Operation, Result: json.RawMessage(`{"valid":true}`)})
		}
	}
}

func TestValidationRequiresExplicitPositiveResult(t *testing.T) {
	for _, mode := range []string{"missing_validation", "false_validation", "valid"} {
		t.Run(mode, func(t *testing.T) {
			fakeWorker(t, mode)
			err := Validate("validate_scope", map[string]any{})
			if mode == "valid" && err != nil {
				t.Fatal(err)
			}
			if mode != "valid" && !errors.Is(err, ErrUnavailable) {
				t.Fatalf("incomplete validation result was accepted: %v", err)
			}
		})
	}
}

func fakeWorker(t *testing.T, mode string) {
	t.Helper()
	path, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GRAPHLAYER_ENGINE", path)
	t.Setenv("GRAPHLAYER_PROTOCOL_TEST", "1")
	t.Setenv("GRAPHLAYER_PROTOCOL_MODE", mode)
	workerArguments = []string{"-test.run=^TestWorkerProcess$", "--"}
	t.Cleanup(func() { workerArguments = nil })
}
func TestOneShotRejectsBrokenOrAmbiguousWorkerOutput(t *testing.T) {
	for _, mode := range []string{"timeout", "overflow", "failed", "duplicate", "trailing", "wrong_operation"} {
		t.Run(mode, func(t *testing.T) {
			fakeWorker(t, mode)
			deadline := 10 * time.Second
			if mode == "timeout" {
				deadline = 150 * time.Millisecond
			}
			ctx, cancel := context.WithTimeout(context.Background(), deadline)
			defer cancel()
			var output map[string]any
			err := Call(ctx, "probe", map[string]any{"credential": "never-log-this"}, &output)
			if !errors.Is(err, ErrUnavailable) || strings.Contains(err.Error(), "canary") || strings.Contains(err.Error(), "never-log-this") {
				t.Fatalf("worker failure escaped its boundary: %v", err)
			}
			expected := "malformed output"
			switch mode {
			case "timeout":
				expected = "deadline exceeded"
			case "overflow":
				expected = "output exceeds protocol bounds"
			case "failed":
				expected = "process failed"
			case "wrong_operation":
				expected = "protocol mismatch"
			}
			if !strings.Contains(err.Error(), expected) {
				t.Fatalf("wrong failure for %s: %v", mode, err)
			}
		})
	}
}
func TestDomainErrorsRemainDistinctFromWorkerFailures(t *testing.T) {
	fakeWorker(t, "domain")
	var output map[string]any
	err := Call(context.Background(), "probe", map[string]any{}, &output)
	if err == nil || errors.Is(err, ErrUnavailable) || err.Error() != "invalid flow source address" {
		t.Fatalf("domain error was not preserved: %v", err)
	}
}

func TestStdoutOverflowKillsWorkerWithoutCallerDeadline(t *testing.T) {
	fakeWorker(t, "overflow_hang")
	started := time.Now()
	err := Call(context.Background(), "probe", nil, nil)
	if !errors.Is(err, ErrUnavailable) || !strings.Contains(err.Error(), "output exceeds protocol bounds") {
		t.Fatalf("wrong overflow failure: %v", err)
	}
	if time.Since(started) > 5*time.Second {
		t.Fatal("overflow left the worker running until its deadline")
	}
}

func TestEnvelopeKeysAreExactAndResultErrorAreExclusive(t *testing.T) {
	for _, fields := range []string{
		`"error":"bad","Error":"","result":{}`,
		`"result":null,"Result":{}`,
		`"Result":{}`,
		`"result":{},"error":""`,
		`"result":{},"error_kind":""`,
		`"error":""`,
	} {
		data := []byte(`{"protocol_version":"graphlayer-engine-v1","operation":"probe",` + fields + `}`)
		if err := decode(data, "probe", nil); !errors.Is(err, ErrUnavailable) {
			t.Fatalf("ambiguous envelope was accepted: %s", fields)
		}
	}
	var result map[string]any
	if err := decode([]byte(`{"protocol_version":"graphlayer-engine-v1","operation":"probe","result":{"Case":1,"case":2}}`), "probe", &result); err != nil {
		t.Fatalf("case-sensitive data map was changed: %v", err)
	}
}
func TestSessionProcessesMultipleFramesAndCloses(t *testing.T) {
	fakeWorker(t, "valid")
	session, err := NewSession()
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	for i := 0; i < 2; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		var output struct {
			Valid bool `json:"valid"`
		}
		err = session.Call(ctx, "probe", map[string]any{}, &output)
		cancel()
		if err != nil || !output.Valid {
			t.Fatalf("frame %d: %v", i, err)
		}
	}
	if err = session.Close(); err != nil {
		t.Fatal(err)
	}
	if err = session.Call(context.Background(), "probe", nil, nil); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("closed session accepted a frame: %v", err)
	}
}
func TestStdoutCapCannotBeBypassedByIOCopy(t *testing.T) {
	buffer := &outputBuffer{}
	_, err := io.Copy(buffer, io.LimitReader(strings.NewReader(strings.Repeat("x", MaxOutput+1)), MaxOutput+1))
	if err == nil || !buffer.overflow || buffer.buffer.Len() != MaxOutput {
		t.Fatal("stdout exceeded its fixed cap")
	}
}
func TestStrictJSONRejectsNestedDuplicateFields(t *testing.T) {
	for _, input := range []string{`{"result":{"x":1,"x":2}}`, `{"x":1} {"y":2}`, `{"x":[{"z":1,"z":2}]}`} {
		var value any
		if DecodeStrict([]byte(input), &value) == nil {
			t.Fatal("ambiguous JSON was accepted")
		}
	}
}

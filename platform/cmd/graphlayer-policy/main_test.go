package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mattpainter701/graphlayer/platform/policy"
)

func TestReadStrict(t *testing.T) {
	tests := []struct {
		name, data string
		wantErr    bool
	}{
		{"valid", `{"snapshot_id":"s","device_id":"d","context":"default"}`, false},
		{"unknown field", `{"snapshot_id":"s","device_id":"d","context":"default","secret":"redact-me"}`, true},
		{"trailing JSON", `{"snapshot_id":"s","device_id":"d","context":"default"} {}`, true},
		{"malformed", `{"snapshot_id":`, true},
		{"null", `null`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "input.json")
			if e := os.WriteFile(p, []byte(tt.data), 0600); e != nil {
				t.Fatal(e)
			}
			var out struct {
				SnapshotID string `json:"snapshot_id"`
				DeviceID   string `json:"device_id"`
				Context    string `json:"context"`
			}
			err := readStrict(p, &out)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error=%v wantErr=%v", err, tt.wantErr)
			}
			if tt.name == "unknown field" && strings.Contains(err.Error(), "redact-me") {
				t.Fatal("error disclosed input contents")
			}
		})
	}
	t.Run("size limit", func(t *testing.T) {
		p := filepath.Join(t.TempDir(), "large.json")
		f, e := os.Create(p)
		if e != nil {
			t.Fatal(e)
		}
		chunk := make([]byte, 1<<20)
		for i := 0; i < 9; i++ {
			if _, e = f.Write(chunk); e != nil {
				t.Fatal(e)
			}
		}
		f.Close()
		var out struct{}
		if e = readStrict(p, &out); e == nil {
			t.Fatal("expected size limit error")
		}
	})
}

func TestReadStrictBundleRequiredFields(t *testing.T) {
	valid := `{"scope":{"snapshot_id":"s","device_id":"d","context":"default"},"complete":true,"default_action":"deny","rules":[{"id":"r","position":0,"enabled":false,"action":"deny","source":"any","destination":"any","protocol":"tcp"}]}`
	tests := []struct {
		name, data string
		wantErr    bool
	}{
		{"missing complete", strings.Replace(valid, `"complete":true,`, "", 1), true},
		{"null complete", strings.Replace(valid, `"complete":true`, `"complete":null`, 1), true},
		{"missing rules", strings.Replace(valid, `"rules":[{"id":"r","position":0,"enabled":false,"action":"deny","source":"any","destination":"any","protocol":"tcp"}]`, "", 1), true},
		{"null rules", strings.Replace(valid, `"rules":[{"id":"r","position":0,"enabled":false,"action":"deny","source":"any","destination":"any","protocol":"tcp"}]`, `"rules":null`, 1), true},
		{"missing enabled", strings.Replace(valid, `"enabled":false,`, "", 1), true},
		{"null enabled", strings.Replace(valid, `"enabled":false`, `"enabled":null`, 1), true},
		{"missing position", strings.Replace(valid, `"position":0,`, "", 1), true},
		{"null position", strings.Replace(valid, `"position":0`, `"position":null`, 1), true},
		{"explicit false and zero accepted", valid, false},
		{"explicit empty rules accepted", strings.Replace(valid, `"rules":[{"id":"r","position":0,"enabled":false,"action":"deny","source":"any","destination":"any","protocol":"tcp"}]`, `"rules":[]`, 1), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "bundle.json")
			if err := os.WriteFile(p, []byte(tt.data), 0600); err != nil {
				t.Fatal(err)
			}
			var bundle policy.Bundle
			err := readStrict(p, &bundle)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error=%v wantErr=%v", err, tt.wantErr)
			}
		})
	}
}

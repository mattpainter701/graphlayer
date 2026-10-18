package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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

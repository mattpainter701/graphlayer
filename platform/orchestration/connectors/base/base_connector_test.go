package base

import (
	"testing"
	"time"
)

func TestCreateConfigResultHashesWithPrefix(t *testing.T) {
	config := &ConnectionConfig{
		Host:       "127.0.0.1",
		Port:       22,
		Timeout:    time.Second,
		RetryCount: 1,
		RetryDelay: time.Second,
	}

	bc := NewBaseConnector(config)

	result := bc.createConfigResult("device-1", "running", "sample-config", time.Second)
	if result.Hash == "" {
		t.Fatalf("expected hash to be populated")
	}

	const prefix = "sha256:"
	if len(result.Hash) <= len(prefix) || result.Hash[:len(prefix)] != prefix {
		t.Fatalf("expected hash to be prefixed with %q, got %q", prefix, result.Hash)
	}

	other := bc.createConfigResult("device-1", "running", "sample-config", time.Second)
	if result.Hash != other.Hash {
		t.Fatalf("expected deterministic hashing, got %q and %q", result.Hash, other.Hash)
	}
}

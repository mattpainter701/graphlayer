package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/mattpainter701/graphlayer/platform/platform/orchestration/orchestration"
)

func TestRunHealth_AllHealthy(t *testing.T) {
	now := time.Now()
	report := orchestration.HealthReport{
		Timestamp:       now,
		TotalConnectors: 2,
		HealthyCount:    2,
		DegradedCount:   0,
		FailedCount:     0,
		OverallSuccess:  1.0,
		Summary:         "HEALTHY: all connectors operating normally",
		Connectors: map[string]*orchestration.ConnectorHealth{
			"guid-1": {
				ConnectorGUID: "guid-1",
				DeviceType:    "fortigate",
				DeviceID:      "10.0.0.1",
				TotalRuns:     10,
				SuccessRate:   1.0,
				LastSuccessAt: &now,
			},
			"guid-2": {
				ConnectorGUID: "guid-2",
				DeviceType:    "paloalto",
				DeviceID:      "10.0.0.2",
				TotalRuns:     10,
				SuccessRate:   1.0,
				LastSuccessAt: &now,
			},
		},
	}

	tmpFile, err := os.CreateTemp("", "health-test-*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	data, _ := json.Marshal(report)
	tmpFile.Write(data)
	tmpFile.Close()

	// Reset global state
	healthInput = tmpFile.Name()

	var buf bytes.Buffer
	cmd := healthCmd
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--input", tmpFile.Name()})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !bytes.Contains([]byte(output), []byte("HEALTHY")) {
		t.Errorf("expected HEALTHY in output, got:\n%s", output)
	}
	if !bytes.Contains([]byte(output), []byte("fortigate")) {
		t.Errorf("expected fortigate in output, got:\n%s", output)
	}
	if !bytes.Contains([]byte(output), []byte("100.0%")) {
		t.Errorf("expected 100.0%% in output, got:\n%s", output)
	}
}

func TestRunHealth_WithFailures(t *testing.T) {
	now := time.Now()
	past := now.Add(-1 * time.Hour)
	report := orchestration.HealthReport{
		Timestamp:       now,
		TotalConnectors: 3,
		HealthyCount:    1,
		DegradedCount:   1,
		FailedCount:     1,
		OverallSuccess:  0.5,
		Summary:         "CRITICAL: connectors failing",
		Connectors: map[string]*orchestration.ConnectorHealth{
			"guid-1": {
				ConnectorGUID: "guid-1",
				DeviceType:    "cisco",
				DeviceID:      "10.0.0.3",
				TotalRuns:     10,
				SuccessRate:   1.0,
				LastSuccessAt: &now,
			},
			"guid-2": {
				ConnectorGUID: "guid-2",
				DeviceType:    "fortigate",
				DeviceID:      "10.0.0.1",
				TotalRuns:     10,
				SuccessRate:   0.7,
				LastSuccessAt: &past,
			},
			"guid-3": {
				ConnectorGUID: "guid-3",
				DeviceType:    "paloalto",
				DeviceID:      "10.0.0.2",
				TotalRuns:     10,
				SuccessRate:   0.1,
				LastSuccessAt: &past,
				LastError:     "connection timeout after 30s",
			},
		},
	}

	tmpFile, err := os.CreateTemp("", "health-test-*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	data, _ := json.Marshal(report)
	tmpFile.Write(data)
	tmpFile.Close()

	var buf bytes.Buffer
	cmd := healthCmd
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--input", tmpFile.Name()})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !bytes.Contains([]byte(output), []byte("CRITICAL")) {
		t.Errorf("expected CRITICAL in output, got:\n%s", output)
	}
	if !bytes.Contains([]byte(output), []byte("connection timeout")) {
		t.Errorf("expected error message in output, got:\n%s", output)
	}
}

func TestRunHealth_EmptyReport(t *testing.T) {
	now := time.Now()
	report := orchestration.HealthReport{
		Timestamp:       now,
		TotalConnectors: 0,
		Summary:         "UNKNOWN: no connector data",
		Connectors:      map[string]*orchestration.ConnectorHealth{},
	}

	tmpFile, err := os.CreateTemp("", "health-test-*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	data, _ := json.Marshal(report)
	tmpFile.Write(data)
	tmpFile.Close()

	var buf bytes.Buffer
	cmd := healthCmd
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--input", tmpFile.Name()})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !bytes.Contains([]byte(output), []byte("UNKNOWN")) {
		t.Errorf("expected UNKNOWN in output, got:\n%s", output)
	}
}

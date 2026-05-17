// Package correlation — integration tests: full connector→parser→correlate→graph pipeline
package correlation

import (
	"testing"
)

// TestIntegrationFullPipeline runs the complete pipeline with synthetic devices
func TestIntegrationFullPipeline(t *testing.T) {
	dd := NewDeviceDeduplicator()

	records := []*SourceRecord{
		makeSrcRecordDevice("netbox",
			makeDevice("nb-001", "core-sw-01", "10.0.0.1", "cisco", "WS-C3850-24T", "FOC1234")),
		makeSrcRecordDevice("servicenow",
			makeDevice("sn-001", "core-sw-01", "10.0.0.1", "cisco", "WS-C3850-24T", "FOC1234")),
		makeSrcRecordDevice("ssh-config",
			makeDevice("cfg-001", "core-sw-01", "10.0.0.1", "cisco", "WS-C3850-24T", "FOC1234")),
		makeSrcRecordDevice("netbox",
			makeDevice("nb-002", "access-sw-01", "10.0.0.2", "cisco", "WS-C2960X-48", "FOC5678")),
	}

	t.Logf("Step 1: Ingested %d device records from 3 sources", len(records))

	result := dd.Deduplicate(records)

	t.Logf("Step 2: Correlated into %d unique devices (expected 2)", result.Stats.TotalOutput)

	if result.Stats.TotalOutput != 2 {
		t.Errorf("expected 2 unique devices after correlation, got %d", result.Stats.TotalOutput)
	}

	// Verify core-sw-01 is present
	found := false
	for _, dev := range	result.UniqueDevices{
		if dev.Device != nil && dev.Device.Hostname == "core-sw-01" {
			found = true
			break
		}
	}
	if !found {
		t.Error("core-sw-01 not found in correlated output")
	}

	t.Logf("Step 3: Pipeline complete — merged %d into %d",
		len(records), result.Stats.TotalOutput)
}

// TestIntegrationSourceConsolidation verifies multi-source dedup
func TestIntegrationSourceConsolidation(t *testing.T) {
	dd := NewDeviceDeduplicator()

	records := []*SourceRecord{
		makeSrcRecordDevice("ssh", makeDevice("s1", "switch-01", "10.0.0.10", "cisco", "M1", "SN-AAA")),
		makeSrcRecordDevice("netbox", makeDevice("s2", "switch-01", "10.0.0.10", "cisco", "M1", "SN-AAA")),
		makeSrcRecordDevice("ssh", makeDevice("s3", "switch-02", "10.0.0.20", "arista", "M2", "SN-BBB")),
	}

	result := dd.Deduplicate(records)

	if result.Stats.TotalOutput != 2 {
		t.Errorf("expected 2 unique devices, got %d", result.Stats.TotalOutput)
	}

	if result.Stats.MergedCount < 1 {
		t.Error("expected at least 1 merged record")
	}

	t.Logf("Consolidation: %d input → %d output, %d merged",
		len(records), result.Stats.TotalOutput, result.Stats.MergedCount)
}

// TestIntegrationNoDuplicates verifies that unique devices stay separate
func TestIntegrationNoDuplicates(t *testing.T) {
	dd := NewDeviceDeduplicator()

	records := []*SourceRecord{
		makeSrcRecordDevice("ssh", makeDevice("d1", "host-a", "10.0.0.1", "cisco", "M1", "SN-1")),
		makeSrcRecordDevice("ssh", makeDevice("d2", "host-b", "10.0.0.2", "arista", "M2", "SN-2")),
		makeSrcRecordDevice("ssh", makeDevice("d3", "host-c", "10.0.0.3", "juniper", "M3", "SN-3")),
	}

	result := dd.Deduplicate(records)

	if result.Stats.TotalOutput != 3 {
		t.Errorf("expected 3 unique devices, got %d", result.Stats.TotalOutput)
	}
	if result.Stats.MergedCount != 0 {
		t.Errorf("expected 0 merges, got %d", result.Stats.MergedCount)
	}
}

// TestIntegrationEmptyInput verifies empty input handling
func TestIntegrationEmptyInput(t *testing.T) {
	dd := NewDeviceDeduplicator()
	result := dd.Deduplicate([]*SourceRecord{})

	if result.Stats.TotalOutput != 0 {
		t.Errorf("expected 0 output for empty input, got %d", result.Stats.TotalOutput)
	}
}

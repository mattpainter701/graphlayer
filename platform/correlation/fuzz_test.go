// Package correlation — fuzz tests
package correlation

import (
	"testing"
)

// FuzzDedupBySerial feeds random serial numbers to the dedup engine
func FuzzDedupBySerial(f *testing.F) {
	f.Add("SN12345", "SN12345")
	f.Add("SN12345", "SN67890")
	f.Add("", "")

	f.Fuzz(func(t *testing.T, serialA, serialB string) {
		if len(serialA) > 256 || len(serialB) > 256 {
			return
		}

		records := []*SourceRecord{
			makeSrcRecordAsset("netbox",
				makeAsset("a1", "10.0.0.1", "", "sw1", serialA, "cisco")),
			makeSrcRecordAsset("servicenow",
				makeAsset("a2", "10.0.0.2", "", "sw2", serialB, "cisco")),
		}

		dd := NewDeviceDeduplicator()
		result := dd.Deduplicate(records)

		// Should produce 1 or 2 unique devices
		if result.Stats.TotalOutput < 1 || result.Stats.TotalOutput > 2 {
			t.Errorf("unexpected output count: %d", result.Stats.TotalOutput)
		}
	})
}

// FuzzDedupEmptyInput feeds varied empty/small inputs
func FuzzDedupEmptyInput(f *testing.F) {
	f.Add(0)
	f.Add(1)
	f.Add(5)

	f.Fuzz(func(t *testing.T, count int) {
		if count < 0 || count > 100 {
			return
		}

		records := make([]*SourceRecord, count)
		for i := 0; i < count; i++ {
			records[i] = makeSrcRecordDevice("src",
				makeDevice("id", "host", "10.0.0.1", "cisco", "M", "SN"))
		}

		dd := NewDeviceDeduplicator()
		result := dd.Deduplicate(records)

		if result.Stats.TotalOutput < 1 && count > 0 {
			t.Errorf("expected at least 1 output for %d inputs, got %d",
				count, result.Stats.TotalOutput)
		}
	})
}

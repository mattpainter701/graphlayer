// Package correlation — benchmark tests
package correlation

import (
	"fmt"
	"testing"
)

// BenchmarkDedupBySerial measures serial-based dedup throughput at scale
func BenchmarkDedupBySerial(b *testing.B) {
	sizes := []int{10, 50, 100}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			records := make([]*SourceRecord, size*2) // pairs of duplicates
			for i := 0; i < size; i++ {
				records[i*2] = makeSrcRecordAsset("netbox",
					makeAsset(fmt.Sprintf("a%d", i*2), "10.0.0.1", "",
						fmt.Sprintf("sw-%d", i), fmt.Sprintf("SN-%d", i), "cisco"))
				records[i*2+1] = makeSrcRecordAsset("servicenow",
					makeAsset(fmt.Sprintf("a%d", i*2+1), "10.0.0.2", "",
						fmt.Sprintf("sw-%d", i), fmt.Sprintf("SN-%d", i), "cisco"))
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				dd := NewDeviceDeduplicator()
				dd.Deduplicate(records)
			}
		})
	}
}

// BenchmarkDedupSmallBatch measures throughput for small batch sizes
func BenchmarkDedupSmallBatch(b *testing.B) {
	records := []*SourceRecord{
		makeSrcRecordDevice("ssh", makeDevice("d1", "sw-01", "10.0.0.1", "cisco", "M1", "SN1")),
		makeSrcRecordDevice("netbox", makeDevice("d2", "sw-01", "10.0.0.1", "cisco", "M1", "SN1")),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dd := NewDeviceDeduplicator()
		dd.Deduplicate(records)
	}
}

// BenchmarkMakeSourceRecord measures record creation cost
func BenchmarkMakeSourceRecord(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		makeSrcRecordDevice("source",
			makeDevice("id", "host", "10.0.0.1", "cisco", "model", "serial"))
	}
}

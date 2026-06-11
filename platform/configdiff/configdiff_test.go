package configdiff

import (
	"testing"
	"time"
)

func baseTime() time.Time {
	return time.Date(2026, 6, 11, 10, 0, 0, 0, time.UTC)
}

func TestDiffIdenticalSnapshots(t *testing.T) {
	snap := &ConfigSnapshot{
		RoundID:   "round-1",
		Timestamp: baseTime(),
		Devices: map[string]DeviceConfigSnapshot{
			"sw1": {
				DeviceID: "sw1", Hostname: "switch1", Vendor: "cisco",
				ConfigHash: "abc123", OSName: "IOS", OSVersion: "15.2",
			},
		},
	}
	result := Diff(snap, snap)
	if result.Summary.TotalChanges != 0 {
		t.Errorf("expected 0 changes for identical snapshots, got %d", result.Summary.TotalChanges)
	}
}

func TestDiffAddedDevice(t *testing.T) {
	old := &ConfigSnapshot{RoundID: "r1", Timestamp: baseTime(), Devices: map[string]DeviceConfigSnapshot{}}
	new := &ConfigSnapshot{RoundID: "r2", Timestamp: baseTime().Add(time.Hour),
		Devices: map[string]DeviceConfigSnapshot{
			"sw1": {DeviceID: "sw1", Hostname: "switch1"},
		},
	}
	result := Diff(old, new)
	if result.Summary.DevicesAdded != 1 {
		t.Errorf("expected 1 device added, got %d", result.Summary.DevicesAdded)
	}
	if len(result.Changes) != 1 || result.Changes[0].Type != ChangeTypeAdded {
		t.Errorf("expected Added change, got %v", result.Changes)
	}
}

func TestDiffRemovedDevice(t *testing.T) {
	old := &ConfigSnapshot{RoundID: "r1", Timestamp: baseTime(),
		Devices: map[string]DeviceConfigSnapshot{
			"sw1": {DeviceID: "sw1", Hostname: "switch1"},
		},
	}
	new := &ConfigSnapshot{RoundID: "r2", Timestamp: baseTime().Add(time.Hour), Devices: map[string]DeviceConfigSnapshot{}}
	result := Diff(old, new)
	if result.Summary.DevicesRemoved != 1 {
		t.Errorf("expected 1 device removed, got %d", result.Summary.DevicesRemoved)
	}
}

func TestDiffConfigHashChange(t *testing.T) {
	old := &ConfigSnapshot{RoundID: "r1", Timestamp: baseTime(),
		Devices: map[string]DeviceConfigSnapshot{
			"sw1": {DeviceID: "sw1", Hostname: "sw1", ConfigHash: "aaa"},
		},
	}
	new := &ConfigSnapshot{RoundID: "r2", Timestamp: baseTime().Add(time.Hour),
		Devices: map[string]DeviceConfigSnapshot{
			"sw1": {DeviceID: "sw1", Hostname: "sw1", ConfigHash: "bbb"},
		},
	}
	result := Diff(old, new)
	if result.Summary.TotalChanges != 1 || result.Changes[0].Type != ChangeTypeHashChange {
		t.Errorf("expected 1 hash change, got %d changes: %v", result.Summary.TotalChanges, result.Changes)
	}
}

func TestDiffVLANChange(t *testing.T) {
	old := &ConfigSnapshot{RoundID: "r1", Timestamp: baseTime(),
		Devices: map[string]DeviceConfigSnapshot{
			"sw1": {DeviceID: "sw1", Hostname: "sw1", ConfigHash: "aaa",
				VLANS: map[int]VLANDiff{10: {ID: 10, Name: "data", State: "active"}}},
		},
	}
	new := &ConfigSnapshot{RoundID: "r2", Timestamp: baseTime().Add(time.Hour),
		Devices: map[string]DeviceConfigSnapshot{
			"sw1": {DeviceID: "sw1", Hostname: "sw1", ConfigHash: "aaa",
				VLANS: map[int]VLANDiff{10: {ID: 10, Name: "data", State: "active"}, 20: {ID: 20, Name: "voice", State: "active"}}},
		},
	}
	result := Diff(old, new)
	if result.Summary.TotalChanges != 1 || result.Changes[0].Type != ChangeTypeVLANChange {
		t.Errorf("expected 1 vlan change, got %d: %v", result.Summary.TotalChanges, result.Changes)
	}
}

func TestDiffInterfaceChange(t *testing.T) {
	old := &ConfigSnapshot{RoundID: "r1", Timestamp: baseTime(),
		Devices: map[string]DeviceConfigSnapshot{
			"sw1": {DeviceID: "sw1", Hostname: "sw1", ConfigHash: "aaa",
				Interfaces: []InterfaceDiff{{Name: "Gi0/1", AdminUp: true, Mode: "access"}}},
		},
	}
	new := &ConfigSnapshot{RoundID: "r2", Timestamp: baseTime().Add(time.Hour),
		Devices: map[string]DeviceConfigSnapshot{
			"sw1": {DeviceID: "sw1", Hostname: "sw1", ConfigHash: "aaa",
				Interfaces: []InterfaceDiff{{Name: "Gi0/1", AdminUp: false, Mode: "access"}}},
		},
	}
	result := Diff(old, new)
	if result.Summary.TotalChanges != 1 || result.Changes[0].Type != ChangeTypeIFChange {
		t.Errorf("expected 1 interface change, got %d: %v", result.Summary.TotalChanges, result.Changes)
	}
}

func TestComputeConfigHash(t *testing.T) {
	h1 := ComputeConfigHash("hostname switch1")
	h2 := ComputeConfigHash("hostname switch2")
	if h1 == h2 {
		t.Error("different inputs should produce different hashes")
	}
	if len(h1) != 64 {
		t.Errorf("expected 64-char hex hash, got %d chars", len(h1))
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	snap := &ConfigSnapshot{RoundID: "r1", Timestamp: baseTime(),
		Devices: map[string]DeviceConfigSnapshot{
			"sw1": {DeviceID: "sw1", Hostname: "sw1", ConfigHash: "aaa"},
		},
	}
	data, err := SaveSnapshot(snap)
	if err != nil {
		t.Fatalf("save failed: %v", err)
	}
	loaded, err := LoadSnapshot(data)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if loaded.RoundID != "r1" || loaded.Devices["sw1"].Hostname != "sw1" {
		t.Errorf("round-trip mismatch: %+v", loaded)
	}
}

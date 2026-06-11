package configdiff

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

// DeviceConfigSnapshot captures a device configuration at a point in time.
type DeviceConfigSnapshot struct {
	DeviceID   string                   `json:"device_id"`
	Hostname   string                   `json:"hostname"`
	Vendor     string                   `json:"vendor"`
	OSName     string                   `json:"os_name"`
	OSVersion  string                   `json:"os_version"`
	ConfigHash string                   `json:"config_hash"`
	CollectedAt time.Time               `json:"collected_at"`
	VLANS      map[int]VLANDiff         `json:"vlans,omitempty"`
	Interfaces []InterfaceDiff          `json:"interfaces,omitempty"`
	RawConfig  string                   `json:"raw_config,omitempty"`
}

// VLANDiff captures VLAN state for comparison.
type VLANDiff struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	State string `json:"state"`
}

// InterfaceDiff captures interface state for comparison.
type InterfaceDiff struct {
	Name       string `json:"name"`
	AdminUp    bool   `json:"admin_up"`
	Mode       string `json:"mode"`
	AccessVLAN *int   `json:"access_vlan,omitempty"`
}

// ConfigSnapshot holds snapshots from a single collection round.
type ConfigSnapshot struct {
	RoundID   string                         `json:"round_id"`
	Timestamp time.Time                      `json:"timestamp"`
	Devices   map[string]DeviceConfigSnapshot `json:"devices"`
}

// ChangeType describes what changed.
type ChangeType string

const (
	ChangeTypeAdded      ChangeType = "added"
	ChangeTypeRemoved    ChangeType = "removed"
	ChangeTypeModified   ChangeType = "modified"
	ChangeTypeVLANChange ChangeType = "vlan_change"
	ChangeTypeIFChange   ChangeType = "interface_change"
	ChangeTypeHashChange ChangeType = "config_hash_change"
)

// ConfigChange represents a single detected change.
type ConfigChange struct {
	DeviceID  string     `json:"device_id"`
	Hostname  string     `json:"hostname"`
	Type      ChangeType `json:"type"`
	Field     string     `json:"field"`
	OldValue  string     `json:"old_value,omitempty"`
	NewValue  string     `json:"new_value,omitempty"`
	Timestamp time.Time  `json:"timestamp"`
}

// DiffResult holds the complete diff between two snapshots.
type DiffResult struct {
	OldRoundID string         `json:"old_round_id"`
	NewRoundID string         `json:"new_round_id"`
	OldTime    time.Time      `json:"old_time"`
	NewTime    time.Time      `json:"new_time"`
	Changes    []ConfigChange `json:"changes"`
	Summary    DiffSummary    `json:"summary"`
}

// DiffSummary provides aggregate counts.
type DiffSummary struct {
	DevicesAdded    int `json:"devices_added"`
	DevicesRemoved  int `json:"devices_removed"`
	DevicesModified int `json:"devices_modified"`
	TotalChanges    int `json:"total_changes"`
}

// Diff compares two config snapshots and returns all detected changes.
func Diff(old, new *ConfigSnapshot) *DiffResult {
	result := &DiffResult{
		OldRoundID: old.RoundID,
		NewRoundID: new.RoundID,
		OldTime:    old.Timestamp,
		NewTime:    new.Timestamp,
	}

	for id, newDev := range new.Devices {
		oldDev, exists := old.Devices[id]
		if !exists {
			result.Changes = append(result.Changes, ConfigChange{
				DeviceID:  id,
				Hostname:  newDev.Hostname,
				Type:      ChangeTypeAdded,
				Timestamp: new.Timestamp,
			})
			result.Summary.DevicesAdded++
			continue
		}
		changes := diffDevice(&oldDev, &newDev, new.Timestamp)
		if len(changes) > 0 {
			result.Changes = append(result.Changes, changes...)
			result.Summary.DevicesModified++
		}
	}

	for id, oldDev := range old.Devices {
		if _, exists := new.Devices[id]; !exists {
			result.Changes = append(result.Changes, ConfigChange{
				DeviceID:  id,
				Hostname:  oldDev.Hostname,
				Type:      ChangeTypeRemoved,
				Timestamp: new.Timestamp,
			})
			result.Summary.DevicesRemoved++
		}
	}

	result.Summary.TotalChanges = len(result.Changes)

	sort.Slice(result.Changes, func(i, j int) bool {
		if result.Changes[i].DeviceID != result.Changes[j].DeviceID {
			return result.Changes[i].DeviceID < result.Changes[j].DeviceID
		}
		return string(result.Changes[i].Type) < string(result.Changes[j].Type)
	})

	return result
}

func diffDevice(old, new *DeviceConfigSnapshot, ts time.Time) []ConfigChange {
	var changes []ConfigChange

	if old.ConfigHash != new.ConfigHash {
		changes = append(changes, ConfigChange{
			DeviceID:  new.DeviceID,
			Hostname:  new.Hostname,
			Type:      ChangeTypeHashChange,
			Field:     "config_hash",
			OldValue:  old.ConfigHash,
			NewValue:  new.ConfigHash,
			Timestamp: ts,
		})
	}

	if old.Hostname != new.Hostname {
		changes = append(changes, ConfigChange{
			DeviceID:  new.DeviceID,
			Hostname:  new.Hostname,
			Type:      ChangeTypeModified,
			Field:     "hostname",
			OldValue:  old.Hostname,
			NewValue:  new.Hostname,
			Timestamp: ts,
		})
	}

	changes = append(changes, diffVLANs(old, new, ts)...)
	changes = append(changes, diffInterfaces(old, new, ts)...)

	return changes
}

func diffVLANs(old, new *DeviceConfigSnapshot, ts time.Time) []ConfigChange {
	var changes []ConfigChange
	oldVLANs := make(map[int]VLANDiff)
	for _, v := range old.VLANS {
		oldVLANs[v.ID] = v
	}
	for _, newV := range new.VLANS {
		oldV, exists := oldVLANs[newV.ID]
		if !exists {
			changes = append(changes, ConfigChange{
				DeviceID:  new.DeviceID, Hostname: new.Hostname,
				Type: ChangeTypeVLANChange, Field: fmt.Sprintf("vlan_%d", newV.ID),
				NewValue: fmt.Sprintf("added: %s (%s)", newV.Name, newV.State), Timestamp: ts,
			})
			continue
		}
		if oldV.Name != newV.Name || oldV.State != newV.State {
			changes = append(changes, ConfigChange{
				DeviceID:  new.DeviceID, Hostname: new.Hostname,
				Type: ChangeTypeVLANChange, Field: fmt.Sprintf("vlan_%d", newV.ID),
				OldValue: fmt.Sprintf("%s (%s)", oldV.Name, oldV.State),
				NewValue: fmt.Sprintf("%s (%s)", newV.Name, newV.State), Timestamp: ts,
			})
		}
	}
	for _, oldV := range old.VLANS {
		if _, exists := new.VLANS[oldV.ID]; !exists {
			changes = append(changes, ConfigChange{
				DeviceID:  new.DeviceID, Hostname: new.Hostname,
				Type: ChangeTypeVLANChange, Field: fmt.Sprintf("vlan_%d", oldV.ID),
				OldValue: fmt.Sprintf("removed: %s (%s)", oldV.Name, oldV.State), Timestamp: ts,
			})
		}
	}
	return changes
}

func diffInterfaces(old, new *DeviceConfigSnapshot, ts time.Time) []ConfigChange {
	var changes []ConfigChange
	oldIFs := make(map[string]InterfaceDiff)
	for _, i := range old.Interfaces {
		oldIFs[i.Name] = i
	}
	for _, newIF := range new.Interfaces {
		oldIF, exists := oldIFs[newIF.Name]
		if !exists {
			changes = append(changes, ConfigChange{
				DeviceID:  new.DeviceID, Hostname: new.Hostname,
				Type: ChangeTypeIFChange, Field: fmt.Sprintf("interface_%s", newIF.Name),
				NewValue: fmt.Sprintf("added: admin=%v mode=%s", newIF.AdminUp, newIF.Mode), Timestamp: ts,
			})
			continue
		}
		if oldIF.AdminUp != newIF.AdminUp || oldIF.Mode != newIF.Mode {
			changes = append(changes, ConfigChange{
				DeviceID:  new.DeviceID, Hostname: new.Hostname,
				Type: ChangeTypeIFChange, Field: fmt.Sprintf("interface_%s", newIF.Name),
				OldValue: fmt.Sprintf("admin=%v mode=%s", oldIF.AdminUp, oldIF.Mode),
				NewValue: fmt.Sprintf("admin=%v mode=%s", newIF.AdminUp, newIF.Mode), Timestamp: ts,
			})
		}
	}
	for _, oldIF := range old.Interfaces {
		found := false
		for _, n := range new.Interfaces {
			if n.Name == oldIF.Name {
				found = true
				break
			}
		}
		if !found {
			changes = append(changes, ConfigChange{
				DeviceID:  new.DeviceID, Hostname: new.Hostname,
				Type: ChangeTypeIFChange, Field: fmt.Sprintf("interface_%s", oldIF.Name),
				OldValue: fmt.Sprintf("removed: admin=%v mode=%s", oldIF.AdminUp, oldIF.Mode), Timestamp: ts,
			})
		}
	}
	return changes
}

// ComputeConfigHash returns the SHA-256 hex digest of the given config text.
func ComputeConfigHash(config string) string {
	h := sha256.Sum256([]byte(config))
	return hex.EncodeToString(h[:])
}

// LoadSnapshot reads a JSON config snapshot from bytes.
func LoadSnapshot(data []byte) (*ConfigSnapshot, error) {
	var snap ConfigSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, fmt.Errorf("failed to parse snapshot: %w", err)
	}
	return &snap, nil
}

// SaveSnapshot writes a config snapshot as indented JSON.
func SaveSnapshot(snap *ConfigSnapshot) ([]byte, error) {
	return json.MarshalIndent(snap, "", "  ")
}

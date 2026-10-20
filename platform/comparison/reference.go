// Package comparison
// Sprint 10: Reference topology management — known-good topologies
// for reproducible comparison testing.
package comparison

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// ====================================================================
// Reference Topology Management
// ====================================================================

// ReferenceTopology is a known-good topology used as ground truth
// for comparison testing.
type ReferenceTopology struct {
	// Name is a human-readable label for this reference topology.
	Name string `json:"name"`

	// Description explains what this topology represents.
	Description string `json:"description"`

	// Topology is the known-good reference-format topology.
	Topology ReferenceExportTopology `json:"topology"`

	// ExpectedMetrics are the expected comparison results.
	// Useful for regression testing.
	ExpectedMetrics *ComparisonMetrics `json:"expected_metrics,omitempty"`
}

// LoadReferenceTopology loads a reference topology from a JSON file.
func LoadReferenceTopology(path string) (*ReferenceTopology, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var ref ReferenceTopology
	if err := json.Unmarshal(data, &ref); err != nil {
		return nil, err
	}
	return &ref, nil
}

// SaveReferenceTopology writes a reference topology to a JSON file.
func SaveReferenceTopology(path string, ref *ReferenceTopology) error {
	data, err := json.MarshalIndent(ref, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// ====================================================================
// Factory Functions — create known-good test topologies
// ====================================================================

// NewReferenceTopology creates a new reference topology with metadata.
func NewReferenceTopology(name, description string, devices []ReferenceDevice, links []ReferenceLink) *ReferenceTopology {
	return &ReferenceTopology{
		Name:        name,
		Description: description,
		Topology: ReferenceExportTopology{
			Metadata: ReferenceMetadata{
				Source:      "reference",
				Version:     "1.0",
				GeneratedAt: time.Now(),
				TotalDevices: len(devices),
				TotalLinks:  len(links),
			},
			Devices: devices,
			Links:   links,
		},
	}
}

// ====================================================================
// Small 3-device reference topology for basic testing
// ====================================================================

// NewSmallReferenceTopology creates a minimal 3-device reference topology:
//   - switch-01 (Cisco IOS L2 switch)
//   - switch-02 (Arista EOS L2 switch)
//   - firewall-01 (Palo Alto PA-220)
// With one L2 link between the switches.
func NewSmallReferenceTopology() *ReferenceTopology {
	devices := []ReferenceDevice{
		{
			ID:              "hostname:switch-01",
			Hostname:        "switch-01",
			Vendor:          "cisco",
			Family:          "IOS",
			Model:           "WS-C2960X-48FPD-L",
			Serial:          "FOC1234ABCD",
			ManagementIP:    "10.0.0.1",
			DeviceType:      "switch",
			OSVersion:       "15.2(7)E7",
			InterfaceCount:  48,
			HasConfig:       true,
			HasOperational:  true,
			EvidenceCount:   2,
			EvidenceSources: []string{"config", "operational"},
			Confidence:      0.90,
		},
		{
			ID:              "hostname:switch-02",
			Hostname:        "switch-02",
			Vendor:          "arista",
			Family:          "EOS",
			Model:           "DCS-7050SX-64",
			Serial:          "JPE1234WXYZ",
			ManagementIP:    "10.0.0.2",
			DeviceType:      "switch",
			OSVersion:       "4.28.3M",
			InterfaceCount:  64,
			HasConfig:       true,
			HasOperational:  true,
			EvidenceCount:   2,
			EvidenceSources: []string{"config", "operational"},
			Confidence:      0.90,
		},
		{
			ID:              "hostname:firewall-01",
			Hostname:        "firewall-01",
			Vendor:          "paloalto",
			Family:          "PAN-OS",
			Model:           "PA-220",
			Serial:          "PA220-001234",
			ManagementIP:    "10.0.0.254",
			DeviceType:      "firewall",
			OSVersion:       "10.1.5",
			InterfaceCount:  8,
			HasConfig:       true,
			HasOperational:  true,
			EvidenceCount:   2,
			EvidenceSources: []string{"config", "operational"},
			Confidence:      0.90,
		},
	}

	links := []ReferenceLink{
		{
			ID:            "l2link:switch-01|Gi1/0/1|switch-02|Et1",
			FromDevice:    "hostname:switch-01",
			FromPort:      "Gi1/0/1",
			ToDevice:      "hostname:switch-02",
			ToPort:        "Et1",
			LinkType:      "l2",
			Protocol:      "lldp",
			EvidenceCount: 2,
			Confidence:    0.90,
		},
	}

	return NewReferenceTopology(
		"small-3-device",
		"Minimal 3-device reference: two L2 switches + one firewall, one L2 link",
		devices, links,
	)
}

// ====================================================================
// Multi-vendor reference topology
// ====================================================================

// NewMultiVendorReferenceTopology creates a 7-device, multi-vendor topology
// with Cisco, Arista, Dell, Palo Alto, Fortinet, and Juniper devices.
func NewMultiVendorReferenceTopology() *ReferenceTopology {
	devices := []ReferenceDevice{
		{
			ID:              "hostname:core-sw-01",
			Hostname:        "core-sw-01",
			Vendor:          "cisco",
			Family:          "NX-OS",
			Model:           "N9K-C93180YC-FX",
			Serial:          "FOC9876ABCD",
			ManagementIP:    "10.1.0.1",
			DeviceType:      "switch",
			OSVersion:       "9.3(7)",
			InterfaceCount:  48,
			HasConfig:       true,
			HasOperational:  true,
			EvidenceCount:   3,
			EvidenceSources: []string{"config", "operational", "netbox"},
			Confidence:      0.95,
		},
		{
			ID:              "hostname:dist-sw-01",
			Hostname:        "dist-sw-01",
			Vendor:          "arista",
			Family:          "EOS",
			Model:           "DCS-7280CR2-60",
			Serial:          "JPE9876LMNO",
			ManagementIP:    "10.1.0.2",
			DeviceType:      "switch",
			OSVersion:       "4.29.2F",
			InterfaceCount:  60,
			HasConfig:       true,
			HasOperational:  true,
			EvidenceCount:   2,
			EvidenceSources: []string{"config", "operational"},
			Confidence:      0.90,
		},
		{
			ID:              "hostname:access-sw-01",
			Hostname:        "access-sw-01",
			Vendor:          "dell",
			Family:          "OS10",
			Model:           "S4148F-ON",
			Serial:          "DELL1234567",
			ManagementIP:    "10.1.0.10",
			DeviceType:      "switch",
			OSVersion:       "10.5.4.0",
			InterfaceCount:  48,
			HasConfig:       true,
			HasOperational:  true,
			EvidenceCount:   2,
			EvidenceSources: []string{"config", "operational"},
			Confidence:      0.85,
		},
		{
			ID:              "hostname:edge-fw-01",
			Hostname:        "edge-fw-01",
			Vendor:          "paloalto",
			Family:          "PAN-OS",
			Model:           "PA-5250",
			Serial:          "PA5250-9876AB",
			ManagementIP:    "10.1.0.254",
			DeviceType:      "firewall",
			OSVersion:       "11.0.3",
			InterfaceCount:  24,
			HasConfig:       true,
			HasOperational:  true,
			EvidenceCount:   2,
			EvidenceSources: []string{"config", "operational"},
			Confidence:      0.90,
		},
		{
			ID:              "hostname:branch-fw-01",
			Hostname:        "branch-fw-01",
			Vendor:          "fortinet",
			Family:          "FortiGate",
			Model:           "FG-60F",
			Serial:          "FGT60FTK987654",
			ManagementIP:    "10.2.0.254",
			DeviceType:      "firewall",
			OSVersion:       "7.4.1",
			InterfaceCount:  10,
			HasConfig:       true,
			HasOperational:  true,
			EvidenceCount:   2,
			EvidenceSources: []string{"config", "operational"},
			Confidence:      0.85,
		},
		{
			ID:              "hostname:core-rt-01",
			Hostname:        "core-rt-01",
			Vendor:          "juniper",
			Family:          "Junos",
			Model:           "MX204",
			Serial:          "JNPR-MX204-001",
			ManagementIP:    "10.1.0.100",
			DeviceType:      "router",
			OSVersion:       "21.4R3-S2",
			InterfaceCount:  8,
			HasConfig:       true,
			HasOperational:  true,
			EvidenceCount:   2,
			EvidenceSources: []string{"config", "operational"},
			Confidence:      0.90,
		},
		{
			ID:              "hostname:spine-sw-01",
			Hostname:        "spine-sw-01",
			Vendor:          "aruba",
			Family:          "AOS-CX",
			Model:           "CX 8325-48Y8C",
			Serial:          "ARUBA-CX-8325-001",
			ManagementIP:    "10.1.0.200",
			DeviceType:      "switch",
			OSVersion:       "10.10.1000",
			InterfaceCount:  56,
			HasConfig:       true,
			HasOperational:  true,
			EvidenceCount:   2,
			EvidenceSources: []string{"config", "operational"},
			Confidence:      0.90,
		},
	}

	links := []ReferenceLink{
		{
			ID: "l2link:core-sw|Eth1/1|dist-sw|Eth1",
			FromDevice: "hostname:core-sw-01", FromPort: "Eth1/1",
			ToDevice: "hostname:dist-sw-01", ToPort: "Eth1",
			LinkType: "l2", Protocol: "lldp",
			EvidenceCount: 2, Confidence: 0.90,
		},
		{
			ID: "l2link:dist-sw|Eth48|access-sw|Eth1/1",
			FromDevice: "hostname:dist-sw-01", FromPort: "Eth48",
			ToDevice: "hostname:access-sw-01", ToPort: "Eth1/1",
			LinkType: "l2", Protocol: "lldp",
			EvidenceCount: 2, Confidence: 0.85,
		},
		{
			ID: "l2link:dist-sw|Eth24|edge-fw|eth1/1",
			FromDevice: "hostname:dist-sw-01", FromPort: "Eth24",
			ToDevice: "hostname:edge-fw-01", ToPort: "eth1/1",
			LinkType: "l2", Protocol: "arp",
			EvidenceCount: 1, Confidence: 0.70,
		},
		{
			ID: "l2link:core-sw|Eth1/5|core-rt|ge-0/0/0",
			FromDevice: "hostname:core-sw-01", FromPort: "Eth1/5",
			ToDevice: "hostname:core-rt-01", ToPort: "ge-0/0/0",
			LinkType: "l3", Protocol: "bgp",
			EvidenceCount: 2, Confidence: 0.90,
		},
		{
			ID: "l2link:core-sw|Eth1/6|spine-sw|1/1/1",
			FromDevice: "hostname:core-sw-01", FromPort: "Eth1/6",
			ToDevice: "hostname:spine-sw-01", ToPort: "1/1/1",
			LinkType: "l2", Protocol: "lldp",
			EvidenceCount: 2, Confidence: 0.90,
		},
	}

	return NewReferenceTopology(
		"multi-vendor-7-device",
		"7-device topology covering Cisco, Arista, Dell, Palo Alto, Fortinet, Juniper, Aruba with 5 links",
		devices, links,
	)
}

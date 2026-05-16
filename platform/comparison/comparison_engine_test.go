// Package comparison
// Sprint 10: Comparison test suite — known-good reference topologies
// and metric validation.
package comparison

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/mattpainter701/graphlayer/platform/models"
)

// ====================================================================
// Test: reference format round-trip
// ====================================================================

func TestreferenceFormatRoundTrip(t *testing.T) {
	ref := NewSmallReferenceTopology()

	// Marshal to JSON
	data, err := json.MarshalIndent(ref.Topology, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal topology: %v", err)
	}

	// Unmarshal back
	var roundtrip referenceTopology
	if err := json.Unmarshal(data, &roundtrip); err != nil {
		t.Fatalf("failed to unmarshal topology: %v", err)
	}

	// Verify
	if len(roundtrip.Devices) != 3 {
		t.Errorf("expected 3 devices, got %d", len(roundtrip.Devices))
	}
	if len(roundtrip.Links) != 1 {
		t.Errorf("expected 1 link, got %d", len(roundtrip.Links))
	}
}

// ====================================================================
// Test: Device comparison — identical topologies
// ====================================================================

func TestCompareIdenticalTopologies(t *testing.T) {
	ref := NewSmallReferenceTopology()

	// Compare against itself
	metrics := CompareTopologies(&ref.Topology, &ref.Topology)

	if metrics.DeviceMetrics.MatchedDevices != 3 {
		t.Errorf("expected 3 matched devices, got %d", metrics.DeviceMetrics.MatchedDevices)
	}
	if metrics.DeviceMetrics.ExtraDevices != 0 {
		t.Errorf("expected 0 extra devices, got %d", metrics.DeviceMetrics.ExtraDevices)
	}
	if metrics.DeviceMetrics.MissingDevices != 0 {
		t.Errorf("expected 0 missing devices, got %d", metrics.DeviceMetrics.MissingDevices)
	}
	if metrics.DeviceMetrics.DeviceAccuracy != 1.0 {
		t.Errorf("expected device accuracy 1.0, got %.4f", metrics.DeviceMetrics.DeviceAccuracy)
	}
	if metrics.DeviceMetrics.F1Score != 1.0 {
		t.Errorf("expected F1 1.0, got %.4f", metrics.DeviceMetrics.F1Score)
	}
	if metrics.LinkMetrics.MatchedLinks != 1 {
		t.Errorf("expected 1 matched link, got %d", metrics.LinkMetrics.MatchedLinks)
	}
}

// ====================================================================
// Test: Device comparison — GraphLayer has extra device
// ====================================================================

func TestCompareExtraDevice(t *testing.T) {
	ref := NewSmallReferenceTopology()

	// GraphLayer has an extra device reference doesn't
	gl := ref.Topology
	gl.Devices = append(gl.Devices, referenceDevice{
		ID:              "hostname:extra-switch-03",
		Hostname:        "extra-switch-03",
		Vendor:          "cisco",
		ManagementIP:    "10.0.0.3",
		DeviceType:      "switch",
		InterfaceCount:  24,
		HasConfig:       true,
		EvidenceCount:   1,
		EvidenceSources: []string{"config"},
		Confidence:      0.70,
	})

	np := ref.Topology // reference has only 3

	metrics := CompareTopologies(&gl, &np)

	if metrics.DeviceMetrics.GraphLayerCount != 4 {
		t.Errorf("expected gl count 4, got %d", metrics.DeviceMetrics.GraphLayerCount)
	}
	if metrics.DeviceMetrics.referenceCount != 3 {
		t.Errorf("expected np count 3, got %d", metrics.DeviceMetrics.referenceCount)
	}
	if metrics.DeviceMetrics.ExtraDevices != 1 {
		t.Errorf("expected 1 extra device, got %d", metrics.DeviceMetrics.ExtraDevices)
	}
	if metrics.DeviceMetrics.MissingDevices != 0 {
		t.Errorf("expected 0 missing devices, got %d", metrics.DeviceMetrics.MissingDevices)
	}
	if metrics.DeviceMetrics.DevicePrecision != 0.75 {
		t.Errorf("expected precision 0.75, got %.4f", metrics.DeviceMetrics.DevicePrecision)
	}
	if metrics.DeviceMetrics.DeviceRecall != 1.0 {
		t.Errorf("expected recall 1.0, got %.4f", metrics.DeviceMetrics.DeviceRecall)
	}
}

// ====================================================================
// Test: Device comparison — reference has device GraphLayer missed
// ====================================================================

func TestCompareMissingDevice(t *testing.T) {
	ref := NewSmallReferenceTopology()

	// GraphLayer missed a device
	gl := referenceTopology{
		Metadata: ref.Topology.Metadata,
		Devices:  ref.Topology.Devices[:2], // only 2 devices
		Links:    ref.Topology.Links[:0],   // no links for now
	}

	np := ref.Topology // reference has 3

	metrics := CompareTopologies(&gl, &np)

	if metrics.DeviceMetrics.GraphLayerCount != 2 {
		t.Errorf("expected gl count 2, got %d", metrics.DeviceMetrics.GraphLayerCount)
	}
	if metrics.DeviceMetrics.MissingDevices < 1 {
		t.Errorf("expected at least 1 missing device, got %d", metrics.DeviceMetrics.MissingDevices)
	}
	if metrics.DeviceMetrics.DeviceRecall < 1.0 {
		t.Logf("recall: %.4f (expected < 1.0 since we missed a device)", metrics.DeviceMetrics.DeviceRecall)
	}
}

// ====================================================================
// Test: Link comparison — fuzzy matching
// ====================================================================

func TestLinkFuzzyMatching(t *testing.T) {
	ref := NewSmallReferenceTopology()

	// Slightly different port name (should still match via fuzzy)
	gl := ref.Topology
	np := referenceTopology{
		Metadata: ref.Topology.Metadata,
		Devices:  ref.Topology.Devices,
		Links: []referenceLink{
			{
				ID: "l2link:reference-version",
				FromDevice: "hostname:switch-01",
				FromPort:   "GigabitEthernet1/0/1", // different name format
				ToDevice:   "hostname:switch-02",
				ToPort:     "Ethernet1", // different name format
				LinkType:   "l2",
				Confidence: 0.85,
			},
		},
	}

	metrics := CompareTopologies(&gl, &np)

	// Fuzzy matching should find this link
	if metrics.LinkMetrics.MatchedLinks < 1 {
		t.Errorf("fuzzy matching failed: expected at least 1 matched link, got %d", metrics.LinkMetrics.MatchedLinks)
	}
}

// ====================================================================
// Test: Vendor coverage
// ====================================================================

func TestVendorCoverage(t *testing.T) {
	ref := NewMultiVendorReferenceTopology()

	// reference only supports 4 vendors
	npDevices := make([]referenceDevice, 0)
	for _, d := range ref.Topology.Devices {
		if d.Vendor == "cisco" || d.Vendor == "arista" || d.Vendor == "paloalto" || d.Vendor == "juniper" {
			npDevices = append(npDevices, d)
		}
	}
	np := referenceTopology{
		Metadata: ref.Topology.Metadata,
		Devices:  npDevices,
		Links:    ref.Topology.Links,
	}

	metrics := CompareTopologies(&ref.Topology, &np)

	if metrics.VendorCoverage.GraphLayerVendorCount <= 4 {
		t.Errorf("expected GraphLayer to cover more than 4 vendors, got %d", metrics.VendorCoverage.GraphLayerVendorCount)
	}
	if len(metrics.VendorCoverage.ExtraVendors) == 0 {
		t.Errorf("expected extra vendors in GraphLayer, got none")
	}
}

// ====================================================================
// Test: Evidence quality
// ====================================================================

func TestEvidenceQuality(t *testing.T) {
	ref := NewMultiVendorReferenceTopology()

	metrics := CompareTopologies(&ref.Topology, &ref.Topology)

	if metrics.EvidenceQuality.GraphLayerAvgEvidence < 1.0 {
		t.Errorf("expected avg evidence >= 1.0, got %.2f", metrics.EvidenceQuality.GraphLayerAvgEvidence)
	}
	if metrics.EvidenceQuality.GraphLayerEvidenceRatio != 1.0 {
		t.Errorf("expected evidence ratio 1.0, got %.4f", metrics.EvidenceQuality.GraphLayerEvidenceRatio)
	}
	if metrics.EvidenceQuality.GraphLayerAvgConfidence < 0.5 {
		t.Errorf("expected avg confidence >= 0.5, got %.4f", metrics.EvidenceQuality.GraphLayerAvgConfidence)
	}
}

// ====================================================================
// Test: False positive analysis
// ====================================================================

func TestFalsePositiveAnalysis(t *testing.T) {
	ref := NewSmallReferenceTopology()

	// GraphLayer double-counts two devices and adds a spurious link
	gl := ref.Topology
	gl.Devices = append(gl.Devices, referenceDevice{
		ID:              "hostname:spurious-device",
		Hostname:        "spurious-device",
		Vendor:          "unknown",
		ManagementIP:    "10.99.99.99",
		DeviceType:      "unknown",
		InterfaceCount:  1,
		EvidenceCount:   0,
		Confidence:      0.10,
	})
	gl.Links = append(gl.Links, referenceLink{
		ID:         "l2link:spurious-link",
		FromDevice: "hostname:spurious-device",
		FromPort:   "eth0",
		ToDevice:   "hostname:switch-01",
		ToPort:     "Gi1/0/48",
		LinkType:   "l2",
		Confidence: 0.05,
	})

	np := ref.Topology

	metrics := CompareTopologies(&gl, &np)

	if metrics.FalsePositives.GraphLayerFalsePositiveDevices < 1 {
		t.Errorf("expected >= 1 FP devices, got %d", metrics.FalsePositives.GraphLayerFalsePositiveDevices)
	}
	if metrics.FalsePositives.GraphLayerFPRate <= 0 {
		t.Errorf("expected positive FP rate, got %.4f", metrics.FalsePositives.GraphLayerFPRate)
	}
	if metrics.FalsePositives.GraphLayerLowConfidenceUnmatched < 1 {
		t.Errorf("expected >= 1 low-confidence unmatched, got %d", metrics.FalsePositives.GraphLayerLowConfidenceUnmatched)
	}
}

// ====================================================================
// Test: Overall score
// ====================================================================

func TestOverallScore(t *testing.T) {
	ref := NewMultiVendorReferenceTopology()

	// Perfect comparison
	metrics := CompareTopologies(&ref.Topology, &ref.Topology)

	if metrics.OverallScore < 0.95 {
		t.Errorf("expected overall score >= 0.95 for perfect match, got %.4f", metrics.OverallScore)
	}
}

// ====================================================================
// Test: Report generation
// ====================================================================

func TestReportGeneration(t *testing.T) {
	ref := NewMultiVendorReferenceTopology()

	metrics := CompareTopologies(&ref.Topology, &ref.Topology)

	report := GenerateReport(
		"GraphLayer v2.0 vs reference Comparison",
		"v2.0.0",
		"reference-v4.3",
		metrics,
	)

	md := report.RenderMarkdown()
	if len(md) < 100 {
		t.Errorf("report markdown too short: %d chars", len(md))
	}

	// Verify key sections are present
	for _, section := range []string{
		"Device Accuracy",
		"Link Accuracy",
		"Vendor Coverage",
		"Evidence Quality",
		"False Positive",
		"Methodology",
	} {
		if !contains(md, section) {
			t.Errorf("report missing section: %s", section)
		}
	}

	// Summary should not be empty
	if report.Summary == "" {
		t.Error("report summary is empty")
	}

	// Recommendations for perfect match should be nil or minimal
	t.Logf("Recommendations: %v", report.Recommendations)
}

// ====================================================================
// Test: Per-vendor breakdown
// ====================================================================

func TestPerVendorBreakdown(t *testing.T) {
	ref := NewMultiVendorReferenceTopology()

	metrics := CompareTopologies(&ref.Topology, &ref.Topology)

	if len(metrics.PerVendor) < 6 {
		t.Errorf("expected at least 6 vendor breakdowns, got %d", len(metrics.PerVendor))
	}

	for vendor, vb := range metrics.PerVendor {
		if vb.GraphLayerDevices != vb.referenceDevices {
			t.Errorf("vendor %s: device count mismatch (GL=%d, np=%d)",
				vendor, vb.GraphLayerDevices, vb.referenceDevices)
		}
		if vb.DeviceAccuracy != 1.0 {
			t.Errorf("vendor %s: expected device accuracy 1.0, got %.4f", vendor, vb.DeviceAccuracy)
		}
	}
}

// ====================================================================
// Test: Reference topology save/load round-trip
// ====================================================================

func TestReferenceTopologySaveLoad(t *testing.T) {
	ref := NewSmallReferenceTopology()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "ref-topo.json")

	if err := SaveReferenceTopology(path, ref); err != nil {
		t.Fatalf("failed to save reference: %v", err)
	}

	loaded, err := LoadReferenceTopology(path)
	if err != nil {
		t.Fatalf("failed to load reference: %v", err)
	}

	if loaded.Name != ref.Name {
		t.Errorf("name mismatch: got %q, want %q", loaded.Name, ref.Name)
	}
	if len(loaded.Topology.Devices) != len(ref.Topology.Devices) {
		t.Errorf("device count mismatch: got %d, want %d", len(loaded.Topology.Devices), len(ref.Topology.Devices))
	}
}

// ====================================================================
// Test: Device ID generation
// ====================================================================

func TestDeviceIDGeneration(t *testing.T) {
	tests := []struct {
		name     string
		asset    models.AssetNode
		wantPrefix string
	}{
		{"serial", models.AssetNode{Serial: "ABC123"}, "serial:"},
		{"ip", models.AssetNode{IP: "10.0.0.1"}, "ip:"},
		{"hostname", models.AssetNode{Hostname: "switch01"}, "hostname:"},
		{"fallback", models.AssetNode{Vendor: "cisco", Family: "IOS"}, "hash:"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := deviceID(tt.asset)
			if !hasPrefix(id, tt.wantPrefix) {
				t.Errorf("deviceID() = %q, want prefix %q", id, tt.wantPrefix)
			}
		})
	}
}

// ====================================================================
// Test: GraphLayer → reference translation
// ====================================================================

func TestTranslateAssetsToreference(t *testing.T) {
	// Create mock assets
	assets := []models.AssetNode{
		{
			Hostname:               "test-switch-01",
			Vendor:                 "cisco",
			Family:                 "IOS",
			Model:                  "WS-C2960X",
			Serial:                 "FOC12345",
			IP:                     "10.0.0.1",
			DeviceType:             "switch",
			OSVersion:              "15.2(7)E7",
			ConfigurationParsed:    true,
			Integration:            "ssh",
			PhysicalNetworkInterfaces: []models.PhysicalInterface{
				{Name: "Gi1/0/1"},
				{Name: "Gi1/0/2"},
			},
		},
	}

	devices := TranslateAssetsToreference(assets)
	if len(devices) != 1 {
		t.Fatalf("expected 1 device, got %d", len(devices))
	}

	d := devices[0]
	if d.Hostname != "test-switch-01" {
		t.Errorf("hostname mismatch: %q", d.Hostname)
	}
	if d.Vendor != "cisco" {
		t.Errorf("vendor mismatch: %q", d.Vendor)
	}
	if d.InterfaceCount != 2 {
		t.Errorf("interface count mismatch: %d", d.InterfaceCount)
	}
	if !d.HasConfig {
		t.Error("expected HasConfig=true")
	}
	if d.EvidenceCount < 1 {
		t.Error("expected EvidenceCount >= 1")
	}
}

// ====================================================================
// Test: Empty topologies
// ====================================================================

func TestCompareEmptyTopologies(t *testing.T) {
	empty := &referenceTopology{
		Metadata: referenceMetadata{
			Source:       "test",
			TotalDevices: 0,
			TotalLinks:   0,
		},
	}

	metrics := CompareTopologies(empty, empty)

	if metrics.DeviceMetrics.GraphLayerCount != 0 {
		t.Errorf("expected 0 devices, got %d", metrics.DeviceMetrics.GraphLayerCount)
	}
	if metrics.DeviceMetrics.F1Score != 0 {
		t.Errorf("expected F1=0 for empty, got %.4f", metrics.DeviceMetrics.F1Score)
	}
}

// ====================================================================
// Test: Device fuzzy matching
// ====================================================================

func TestFuzzyDeviceMatchingByIP(t *testing.T) {
	glDevices := []referenceDevice{
		{ID: "hostname:switch-01", Hostname: "switch-01", ManagementIP: "10.0.0.1", Vendor: "cisco"},
	}
	npDevices := []referenceDevice{
		{ID: "serial:DIFFERENT-ID", Hostname: "switch-old-name", ManagementIP: "10.0.0.1", Vendor: "cisco"},
	}

	dm := compareDevices(glDevices, npDevices)

	if dm.MatchedDevices != 1 {
		t.Errorf("fuzzy IP matching failed: expected 1 matched, got %d", dm.MatchedDevices)
	}
}

func TestFuzzyDeviceMatchingByHostname(t *testing.T) {
	glDevices := []referenceDevice{
		{ID: "ip:10.0.0.1", Hostname: "core-switch-01", Vendor: "cisco"},
	}
	npDevices := []referenceDevice{
		{ID: "ip:192.168.1.1", Hostname: "core-switch-01", Vendor: "cisco"},
	}

	dm := compareDevices(glDevices, npDevices)

	if dm.MatchedDevices != 1 {
		t.Errorf("fuzzy hostname matching failed: expected 1 matched, got %d", dm.MatchedDevices)
	}
}

// ====================================================================
// Helpers
// ====================================================================

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

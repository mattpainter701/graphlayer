// Package correlation
// Sprint 8: Integration tests for cross-source deduplication & correlation engine.
package correlation

import (
	"testing"
	"time"

	"github.com/mattpainter701/graphlayer/platform/models"
)

// ========================================================================
// Helper: Create Test Source Records
// ========================================================================

func makeDevice(id, hostname, mgmtIP, vendor, model, serial string) *models.Device {
	return &models.Device{
		ID:           id,
		Hostname:     hostname,
		ManagementIP: mgmtIP,
		Vendor:       vendor,
		Model:        model,
	}
}

func makeAsset(id, ip, mac, hostname, serial, vendor string) *models.AssetNode {
	return &models.AssetNode{
		Id:       id,
		IP:       ip,
		Mac:      mac,
		Hostname: hostname,
		Serial:   serial,
		Vendor:   vendor,
	}
}

func makeSrcRecordDevice(name string, dev *models.Device) *SourceRecord {
	return &SourceRecord{
		SourceName:  name,
		SourceType:  models.ConfidenceSourceConfig,
		Device:      dev,
		CollectedAt: time.Now(),
		Metadata:    make(map[string]string),
	}
}

func makeSrcRecordAsset(name string, asset *models.AssetNode) *SourceRecord {
	return &SourceRecord{
		SourceName:  name,
		SourceType:  models.ConfidenceSourceConnector,
		Asset:       asset,
		CollectedAt: time.Now(),
		Metadata:    make(map[string]string),
	}
}

// ========================================================================
// Tests: Serial Number Dedup
// ========================================================================

func TestDedupBySerial(t *testing.T) {
	dd := NewDeviceDeduplicator()

	records := []*SourceRecord{
		makeSrcRecordAsset("netbox", makeAsset("a1", "10.0.0.1", "", "sw-core-01", "SN12345", "cisco")),
		makeSrcRecordAsset("servicenow", makeAsset("a2", "10.0.0.2", "", "core-switch", "SN12345", "cisco")),
	}

	result := dd.Deduplicate(records)

	if result.Stats.TotalOutput != 1 {
		t.Errorf("expected 1 unique device, got %d", result.Stats.TotalOutput)
	}
	if result.Stats.MergedCount != 1 {
		t.Errorf("expected 1 merged record, got %d", result.Stats.MergedCount)
	}
	if result.Stats.MatchesFound < 1 {
		t.Errorf("expected at least 1 match, got %d", result.Stats.MatchesFound)
	}
}

// ========================================================================
// Tests: Management IP Dedup
// ========================================================================

func TestDedupByManagementIP(t *testing.T) {
	dd := NewDeviceDeduplicator()

	records := []*SourceRecord{
		makeSrcRecordDevice("ssh-config", makeDevice("d1", "sw-01", "10.1.1.1", "cisco", "C9300", "")),
		makeSrcRecordDevice("netbox", makeDevice("d2", "switch-01", "10.1.1.1", "cisco", "Catalyst 9300", "")),
	}

	result := dd.Deduplicate(records)

	if result.Stats.TotalOutput != 1 {
		t.Errorf("expected 1 unique device, got %d", result.Stats.TotalOutput)
	}
	if result.Stats.MergedCount != 1 {
		t.Errorf("expected 1 merged record, got %d", result.Stats.MergedCount)
	}

	// Verify merged device has info from both
	if len(result.UniqueDevices) < 1 {
		t.Fatal("no unique devices")
	}
	canonical := result.UniqueDevices[0]
	if canonical.Device.Hostname != "sw-01" {
		t.Errorf("expected hostname 'sw-01', got '%s'", canonical.Device.Hostname)
	}
	if canonical.Device.Model != "C9300" {
		t.Errorf("expected model 'C9300', got '%s'", canonical.Device.Model)
	}
}

// ========================================================================
// Tests: MAC Address Dedup
// ========================================================================

func TestDedupByMAC(t *testing.T) {
	dd := NewDeviceDeduplicator()

	records := []*SourceRecord{
		makeSrcRecordAsset("runzero", makeAsset("a1", "10.0.0.10", "aa:bb:cc:dd:ee:ff", "host-a", "", "vmware")),
		makeSrcRecordAsset("claroty", makeAsset("a2", "10.0.0.11", "aa:bb:cc:dd:ee:ff", "host-a-ot", "", "vmware")),
	}

	result := dd.Deduplicate(records)

	if result.Stats.TotalOutput != 1 {
		t.Errorf("expected 1 unique device, got %d", result.Stats.TotalOutput)
	}

	// Verify canonical has merged info
	if len(result.UniqueDevices) < 1 {
		t.Fatal("no unique devices")
	}
	canonical := result.UniqueDevices[0]
	// The canonical should have IP from one and hostname from canon source
	if canonical.Asset.Mac == "" {
		t.Error("expected MAC to be populated")
	}
}

// ========================================================================
// Tests: Hostname Dedup
// ========================================================================

func TestDedupByHostname(t *testing.T) {
	dd := NewDeviceDeduplicator()

	records := []*SourceRecord{
		makeSrcRecordDevice("ssh-config", makeDevice("d1", "core-switch-01", "10.0.0.1", "cisco", "C9500", "")),
		makeSrcRecordDevice("netbox", makeDevice("d2", "core-switch-01", "10.0.0.2", "cisco", "", "")),
	}

	result := dd.Deduplicate(records)

	if result.Stats.TotalOutput != 1 {
		t.Errorf("expected 1 unique device, got %d", result.Stats.TotalOutput)
	}
	if result.Stats.MergedCount != 1 {
		t.Errorf("expected 1 merged, got %d", result.Stats.MergedCount)
	}
}

// ========================================================================
// Tests: No Match — Different Devices
// ========================================================================

func TestDedupNoMatch(t *testing.T) {
	dd := NewDeviceDeduplicator()

	records := []*SourceRecord{
		makeSrcRecordDevice("ssh-config", makeDevice("d1", "sw-01", "10.0.0.1", "cisco", "C9300", "SN111")),
		makeSrcRecordDevice("ssh-config", makeDevice("d2", "sw-02", "10.0.0.2", "arista", "DCS-7050", "SN222")),
	}

	result := dd.Deduplicate(records)

	if result.Stats.TotalOutput != 2 {
		t.Errorf("expected 2 unique devices, got %d", result.Stats.TotalOutput)
	}
	if result.Stats.MergedCount != 0 {
		t.Errorf("expected 0 merged, got %d", result.Stats.MergedCount)
	}
	if result.Stats.MatchesFound != 0 {
		t.Errorf("expected 0 matches, got %d", result.Stats.MatchesFound)
	}
}

// ========================================================================
// Tests: Empty Input
// ========================================================================

func TestDedupEmptyInput(t *testing.T) {
	dd := NewDeviceDeduplicator()

	result := dd.Deduplicate([]*SourceRecord{})

	if result.Stats.TotalInput != 0 {
		t.Errorf("expected 0 total input, got %d", result.Stats.TotalInput)
	}
	if result.Stats.TotalOutput != 0 {
		t.Errorf("expected 0 total output, got %d", result.Stats.TotalOutput)
	}
}

// ========================================================================
// Tests: Single Record
// ========================================================================

func TestDedupSingleRecord(t *testing.T) {
	dd := NewDeviceDeduplicator()

	records := []*SourceRecord{
		makeSrcRecordDevice("ssh-config", makeDevice("d1", "sw-01", "10.0.0.1", "cisco", "C9300", "SN111")),
	}

	result := dd.Deduplicate(records)

	if result.Stats.TotalOutput != 1 {
		t.Errorf("expected 1 unique device, got %d", result.Stats.TotalOutput)
	}
	if result.Stats.MergedCount != 0 {
		t.Errorf("expected 0 merged, got %d", result.Stats.MergedCount)
	}
}

// ========================================================================
// Tests: Multi-Source Scenario (3 sources → 1 device)
// ========================================================================

func TestDedupThreeSourceMerge(t *testing.T) {
	dd := NewDeviceDeduplicator()

	records := []*SourceRecord{
		makeSrcRecordDevice("ssh-config",
			makeDevice("d1", "core-01", "10.0.0.1", "cisco", "C9500", "SERIAL001")),
		makeSrcRecordDevice("netbox",
			makeDevice("d2", "core-01.example.com", "10.0.0.1", "", "", "SERIAL001")),
		makeSrcRecordAsset("servicenow",
			makeAsset("a3", "10.0.0.1", "00:11:22:33:44:55", "core-01", "SERIAL001", "cisco")),
	}

	result := dd.Deduplicate(records)

	if result.Stats.TotalOutput != 1 {
		t.Errorf("expected 1 unique device, got %d", result.Stats.TotalOutput)
	}
	if result.Stats.MergedCount != 2 {
		t.Errorf("expected 2 merged records, got %d", result.Stats.MergedCount)
	}
	if result.Stats.MatchesFound < 2 {
		t.Errorf("expected at least 2 matches, got %d", result.Stats.MatchesFound)
	}

	// Verify merged result carries forward good data
	canonical := result.UniqueDevices[0]
	if canonical.Device == nil {
		t.Fatal("expected canonical to have a Device")
	}
	if canonical.Device.Hostname != "core-01" {
		t.Errorf("expected hostname 'core-01', got '%s'", canonical.Device.Hostname)
	}
	if canonical.Device.ManagementIP != "10.0.0.1" {
		t.Errorf("expected management IP '10.0.0.1', got '%s'", canonical.Device.ManagementIP)
	}
}

// ========================================================================
// Tests: Link Correlation
// ========================================================================

func TestLinkCorrelationSimple(t *testing.T) {
	lc := NewLinkCorrelator()

	sourceLinks := []SourceLink{
		{
			SourceName: "ssh-config",
			Links: []models.Link{
				{
					Id:      "l1",
					FromID:  "d1",
					ToID:    "d2",
					Source:  "sw-01",
					Destination: "sw-02",
					Interface: models.PhysicalInterface{Name: "Gi1/0/1"},
					DestInterface: models.PhysicalInterface{Name: "Gi1/0/1"},
				},
			},
		},
		{
			SourceName: "netbox",
			Links: []models.Link{
				{
					Id:      "l2",
					FromID:  "d1",
					ToID:    "d2",
					Source:  "sw-01",
					Destination: "sw-02",
					Interface: models.PhysicalInterface{Name: "Gi1/0/1"},
					DestInterface: models.PhysicalInterface{Name: "Gi1/0/1"},
				},
			},
		},
	}

	result := lc.CorrelateLinks(sourceLinks)

	if result.Stats.TotalInput != 2 {
		t.Errorf("expected 2 total input, got %d", result.Stats.TotalInput)
	}
	if result.Stats.CorrelatedSets != 1 {
		t.Errorf("expected 1 correlated set, got %d", result.Stats.CorrelatedSets)
	}
	if len(result.CorrelatedLinks) != 1 {
		t.Errorf("expected 1 correlated link, got %d", len(result.CorrelatedLinks))
	}
	if result.CorrelatedLinks[0].MatchConfidence < 0.5 {
		t.Errorf("expected match confidence >= 0.5, got %.2f", result.CorrelatedLinks[0].MatchConfidence)
	}
}

func TestLinkCorrelationUnmatched(t *testing.T) {
	lc := NewLinkCorrelator()

	sourceLinks := []SourceLink{
		{
			SourceName: "ssh-config",
			Links: []models.Link{
				{Id: "l1", FromID: "d1", ToID: "d2", Source: "sw-01", Destination: "sw-02"},
			},
		},
		{
			SourceName: "netbox",
			Links: []models.Link{
				{Id: "l2", FromID: "d3", ToID: "d4", Source: "sw-03", Destination: "sw-04"},
			},
		},
	}

	result := lc.CorrelateLinks(sourceLinks)

	if result.Stats.CorrelatedSets != 0 {
		t.Errorf("expected 0 correlated sets, got %d", result.Stats.CorrelatedSets)
	}
	if result.Stats.Unmatched != 2 {
		t.Errorf("expected 2 unmatched links, got %d", result.Stats.Unmatched)
	}
}

// ========================================================================
// Tests: Conflict Detection
// ========================================================================

func TestConflictDetection(t *testing.T) {
	cd := NewConflictDetector()

	canonical := makeSrcRecordDevice("merged", makeDevice("d1", "sw-01", "10.0.0.1", "cisco", "C9300-48P", "SN123"))
	sources := []*SourceRecord{
		makeSrcRecordDevice("netbox", makeDevice("d2", "sw-01", "10.0.0.1", "cisco", "C9300-24P", "SN123")),
	}

	conflicts := cd.DetectConflicts(canonical, sources)

	if len(conflicts) == 0 {
		t.Error("expected conflicts for differing model field")
	}
	if len(conflicts) > 0 {
		conflict := conflicts[0]
		if conflict.Field != "model" {
			t.Errorf("expected conflict on 'model', got '%s'", conflict.Field)
		}
		if conflict.Severity != "critical" {
			t.Errorf("expected severity 'critical', got '%s'", conflict.Severity)
		}
		if conflict.Resolution == "unresolved" {
			t.Error("expected conflict to be resolved")
		}
	}
}

func TestConflictDetectionNoConflict(t *testing.T) {
	cd := NewConflictDetector()

	canonical := makeSrcRecordDevice("merged", makeDevice("d1", "sw-01", "10.0.0.1", "cisco", "C9300", "SN123"))
	sources := []*SourceRecord{
		makeSrcRecordDevice("netbox", makeDevice("d2", "sw-01", "10.0.0.1", "cisco", "C9300", "SN123")),
	}

	conflicts := cd.DetectConflicts(canonical, sources)

	if len(conflicts) != 0 {
		t.Errorf("expected 0 conflicts for identical records, got %d", len(conflicts))
	}
}

func TestConflictSeverity(t *testing.T) {
	tests := []struct {
		field    string
		expected string
	}{
		{"hostname", "critical"},
		{"vendor", "critical"},
		{"model", "critical"},
		{"management_ip", "critical"},
		{"serial", "critical"},
		{"mac", "critical"},
		{"os_name", "warning"},
		{"os_version", "warning"},
		{"site", "warning"},
		{"role", "info"},
		{"unknown_field", "info"},
	}

	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			got := SeverityForField(tt.field)
			if got != tt.expected {
				t.Errorf("SeverityForField(%q) = %q, want %q", tt.field, got, tt.expected)
			}
		})
	}
}

// ========================================================================
// Tests: Manual Override
// ========================================================================

func TestManualOverride(t *testing.T) {
	cd := NewConflictDetector()

	override := OverrideRecord{
		ID:               "override-1",
		Field:            "model",
		DeviceIdentifier: "sw-01",
		Source:           "manual",
		ManualValue:      "C9300-48UXM",
		Reason:           "Physical inspection confirmed model",
		AppliedBy:        "admin",
		AppliedAt:        time.Now(),
	}
	cd.AddOverride(override)

	canonical := makeSrcRecordDevice("merged", makeDevice("d1", "sw-01", "10.0.0.1", "cisco", "C9300-48P", ""))
	sources := []*SourceRecord{
		makeSrcRecordDevice("netbox", makeDevice("d2", "sw-01", "10.0.0.1", "cisco", "C9300-24P", "")),
	}

	conflicts := cd.DetectConflicts(canonical, sources)

	if len(conflicts) == 0 {
		t.Fatal("expected conflicts")
	}

	conflict := conflicts[0]
	if conflict.Resolution != "manual-override" {
		t.Errorf("expected manual-override resolution, got '%s'", conflict.Resolution)
	}
	if conflict.WinningValue != "C9300-48UXM" {
		t.Errorf("expected winning value 'C9300-48UXM', got '%s'", conflict.WinningValue)
	}
}

func TestOverrideExpiration(t *testing.T) {
	expired := time.Now().Add(-1 * time.Hour)
	active := time.Now().Add(1 * time.Hour)

	overrideExpired := OverrideRecord{
		ID:        "exp-1",
		ExpiresAt: &expired,
	}
	overrideActive := OverrideRecord{
		ID:        "act-1",
		ExpiresAt: &active,
	}

	if overrideExpired.Active() {
		t.Error("expired override should not be active")
	}
	if !overrideActive.Active() {
		t.Error("active override should be active")
	}
}

// ========================================================================
// Tests: Rule Engine
// ========================================================================

func TestRuleEngineDefaults(t *testing.T) {
	re := NewRuleEngine()

	active := re.ActiveRules()
	if len(active) != 4 {
		t.Errorf("expected 4 default rules, got %d", len(active))
	}

	// Verify priority ordering
	for i := 1; i < len(active); i++ {
		if active[i].Priority < active[i-1].Priority {
			t.Errorf("rules out of order: rule %d priority %d before rule %d priority %d",
				i-1, active[i-1].Priority, i, active[i].Priority)
		}
	}
}

func TestRuleEngineAddRemove(t *testing.T) {
	re := NewRuleEngine()

	rule := CorrelationRule{
		ID:       "test-rule",
		Name:     "Test Rule",
		Priority: 0,
		Enabled:  true,
	}

	re.AddRule(rule)

	if re.GetRule("test-rule") == nil {
		t.Error("expected to find added rule")
	}

	if !re.RemoveRule("test-rule") {
		t.Error("expected remove to succeed")
	}

	if re.GetRule("test-rule") != nil {
		t.Error("expected rule to be gone after removal")
	}
}

func TestRuleEngineDisable(t *testing.T) {
	re := NewRuleEngine()

	initialActive := len(re.ActiveRules())

	// Disable first rule
	re.Rules[0].Enabled = false

	afterDisable := len(re.ActiveRules())
	if afterDisable != initialActive-1 {
		t.Errorf("expected %d active rules after disable, got %d", initialActive-1, afterDisable)
	}

	// Re-enable
	re.Rules[0].Enabled = true
	if len(re.ActiveRules()) != initialActive {
		t.Errorf("expected %d active rules after re-enable, got %d", initialActive, len(re.ActiveRules()))
	}
}

// ========================================================================
// Tests: Full Correlation Pipeline
// ========================================================================

func TestFullPipeline(t *testing.T) {
	engine := NewCorrelationEngine()

	records := []*SourceRecord{
		// SSH config source: C9500 core switch
		makeSrcRecordDevice("ssh-cisco-config",
			makeDevice("core-01", "core-sw-01", "10.0.1.1", "cisco", "C9500-24Y4C", "FOC1234ABCD")),
		// NetBox source: same device with different model name
		makeSrcRecordDevice("netbox",
			makeDevice("nb-core-01", "core-sw-01", "10.0.1.1", "cisco", "Catalyst 9500", "FOC1234ABCD")),
		// RunZero asset scan: same device via MAC
		makeSrcRecordAsset("runzero",
			makeAsset("rz-001", "10.0.1.1", "00:1a:2b:3c:4d:5e", "core-sw-01", "FOC1234ABCD", "cisco")),
	}

	report := engine.CorrelateInput(records)

	// Verify dedup
	if report.DeviceDedup.Stats.TotalOutput != 1 {
		t.Errorf("expected 1 unique device after dedup, got %d", report.DeviceDedup.Stats.TotalOutput)
	}

	// Verify sources identified
	if len(report.InputSources) != 3 {
		t.Errorf("expected 3 input sources, got %d", len(report.InputSources))
	}

	// Verify report generated
	if report.Summary == "" {
		t.Error("expected non-empty summary")
	}
	if report.GeneratedAt.IsZero() {
		t.Error("expected non-zero generated timestamp")
	}
}

// ========================================================================
// Tests: Mixed Device and Asset Records
// ========================================================================

func TestMixedDeviceAsset(t *testing.T) {
	dd := NewDeviceDeduplicator()

	// Device record (from config parsing) and Asset record (from scanning tool)
	records := []*SourceRecord{
		makeSrcRecordDevice("ssh-config",
			makeDevice("d1", "access-sw-12", "192.168.1.12", "arista", "DCS-7050SX", "JPE1234")),
		makeSrcRecordAsset("tenable",
			makeAsset("a1", "192.168.1.12", "00:0c:29:ab:cd:ef", "ACCESS-SW-12", "JPE1234", "arista")),
	}

	result := dd.Deduplicate(records)

	if result.Stats.TotalOutput != 1 {
		t.Errorf("expected 1 unique device (mixed Device+Asset), got %d", result.Stats.TotalOutput)
	}
}

// ========================================================================
// Tests: MAC Normalization
// ========================================================================

func TestMACNormalization(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"aa:bb:cc:dd:ee:ff", "aabbccddeeff"},
		{"AA:BB:CC:DD:EE:FF", "aabbccddeeff"},
		{"aa-bb-cc-dd-ee-ff", "aabbccddeeff"},
		{"aabb.ccdd.eeff", "aabbccddeeff"},
		{"AABBCCDDEEFF", "aabbccddeeff"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeMAC(tt.input)
			if got != tt.expected {
				t.Errorf("normalizeMAC(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// ========================================================================
// Tests: Hostname Normalization
// ========================================================================

func TestHostnameNormalization(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"CORE-SW-01", "core-sw-01"},
		{"core-sw-01.example.com", "core-sw-01"},
		{"  Access-SW-12  ", "access-sw-12"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeHostname(tt.input)
			if got != tt.expected {
				t.Errorf("normalizeHostname(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// ========================================================================
// Tests: Confidence-Weighted Merge
// ========================================================================

func TestConfidenceWeightedMerge(t *testing.T) {
	dd := NewDeviceDeduplicator()

	// Two records: one from config (high confidence) and one from connector (medium confidence)
	configDevice := makeDevice("d1", "core-01", "10.0.0.1", "cisco", "C9500-24Y4C", "SERIAL-A")
	connectorDevice := makeDevice("d2", "core-01.example.com", "10.0.0.1", "", "", "SERIAL-A")

	records := []*SourceRecord{
		makeSrcRecordDevice("ssh-config", configDevice),
		makeSrcRecordDevice("netbox", connectorDevice),
	}

	result := dd.Deduplicate(records)

	if result.Stats.TotalOutput != 1 {
		t.Fatal("expected 1 merged device")
	}

	canonical := result.UniqueDevices[0]
	// Config source is first and has higher priority — its values should be preferred
	if canonical.Device.Hostname != "core-01" {
		t.Errorf("expected canonical hostname from config source, got '%s'", canonical.Device.Hostname)
	}
	if canonical.Device.ManagementIP != "10.0.0.1" {
		t.Errorf("expected management IP '10.0.0.1', got '%s'", canonical.Device.ManagementIP)
	}
}

// ========================================================================
// Tests: Conflict Summary
// ========================================================================

func TestConflictSummary(t *testing.T) {
	conflicts := []ConflictEntry{
		{Field: "hostname", Severity: "critical", Resolution: "higher-confidence"},
		{Field: "model", Severity: "critical", Resolution: "manual-override"},
		{Field: "os_version", Severity: "warning", Resolution: "higher-confidence"},
		{Field: "role", Severity: "info", Resolution: "unresolved"},
	}

	summary := SummarizeConflicts(conflicts)

	if summary.TotalConflicts != 4 {
		t.Errorf("expected 4 total conflicts, got %d", summary.TotalConflicts)
	}
	if summary.BySeverity["critical"] != 2 {
		t.Errorf("expected 2 critical, got %d", summary.BySeverity["critical"])
	}
	if summary.BySeverity["warning"] != 1 {
		t.Errorf("expected 1 warning, got %d", summary.BySeverity["warning"])
	}
	if summary.BySeverity["info"] != 1 {
		t.Errorf("expected 1 info, got %d", summary.BySeverity["info"])
	}
	if summary.ResolvedCount != 3 {
		t.Errorf("expected 3 resolved, got %d", summary.ResolvedCount)
	}
	if summary.UnresolvedCount != 1 {
		t.Errorf("expected 1 unresolved, got %d", summary.UnresolvedCount)
	}
	if summary.OverrideCount != 1 {
		t.Errorf("expected 1 override, got %d", summary.OverrideCount)
	}
}

// ========================================================================
// Tests: Quick Correlate
// ========================================================================

func TestQuickCorrelate(t *testing.T) {
	records := []*SourceRecord{
		makeSrcRecordDevice("ssh", makeDevice("d1", "sw-01", "10.0.0.1", "cisco", "C9300", "SN1")),
		makeSrcRecordDevice("netbox", makeDevice("d2", "sw-01", "10.0.0.1", "cisco", "", "SN1")),
	}

	report := QuickCorrelate(records)

	if report == nil {
		t.Fatal("expected non-nil report")
	}
	if report.EngineVersion != "correlation-v1" {
		t.Errorf("expected version 'correlation-v1', got '%s'", report.EngineVersion)
	}
}

// ========================================================================
// Tests: Engine Stats
// ========================================================================

func TestEngineStats(t *testing.T) {
	engine := NewCorrelationEngine()
	stats := engine.EngineStats()

	if stats["version"] != "correlation-v1" {
		t.Errorf("expected version 'correlation-v1', got '%v'", stats["version"])
	}
	if stats["dedup_min_confidence"] != 0.5 {
		t.Errorf("expected dedup_min_confidence 0.5, got %v", stats["dedup_min_confidence"])
	}
}

// ========================================================================
// Tests: Source Record Primary Identifier
// ========================================================================

func TestSourceRecordPrimaryIdentifier(t *testing.T) {
	// Device record
	dev := makeSrcRecordDevice("ssh", makeDevice("d1", "sw-01", "10.1.1.1", "cisco", "C9300", ""))
	if dev.PrimaryIdentifier() != "10.1.1.1" {
		t.Errorf("expected management IP '10.1.1.1', got '%s'", dev.PrimaryIdentifier())
	}

	// Device without management IP
	devNoIP := makeSrcRecordDevice("ssh", makeDevice("d2", "sw-02", "", "cisco", "C9300", ""))
	if devNoIP.PrimaryIdentifier() != "sw-02" {
		t.Errorf("expected hostname 'sw-02', got '%s'", devNoIP.PrimaryIdentifier())
	}

	// Asset record
	asset := makeSrcRecordAsset("runzero", makeAsset("a1", "10.1.1.10", "aa:bb:cc:dd:ee:ff", "host-a", "SN1", ""))
	if asset.PrimaryIdentifier() != "10.1.1.10" {
		t.Errorf("expected IP '10.1.1.10', got '%s'", asset.PrimaryIdentifier())
	}
}

package models

import (
	"encoding/json"
	"testing"
)

func TestTopologyRecordMetaV2Defaults(t *testing.T) {
	evidence := []EvidenceRefV2{{
		SourceType:   ConfidenceSourceConfig,
		SourceSystem: "ios-parser",
		Fields:       []string{"hostname", "vendor"},
		Quality:      0.95,
	}}

	meta := NewTopologyRecordMetaV2(TopologyRecordKindDevice, "device:sw1", evidence, nil)
	if meta.SchemaVersion != TopologySchemaVersionV2 {
		t.Fatalf("unexpected schema version: %s", meta.SchemaVersion)
	}
	if meta.RecordKind != TopologyRecordKindDevice {
		t.Fatalf("unexpected record kind: %s", meta.RecordKind)
	}
	if meta.Confidence == nil || meta.Confidence.Overall <= 0 {
		t.Fatalf("expected confidence derived from evidence, got %#v", meta.Confidence)
	}
	if len(meta.Evidence) != 1 || meta.Evidence[0].ObservedAt.IsZero() {
		t.Fatalf("expected evidence timestamp/defaults, got %#v", meta.Evidence)
	}
	if issues := ValidateTopologyRecordMetaV2(meta, TopologyRecordKindDevice); len(issues) != 0 {
		t.Fatalf("unexpected validation issues: %#v", issues)
	}
}

func TestTopologyRecordV2ImplementationsExposeMetadata(t *testing.T) {
	records := []TopologyRecordV2{
		&DeviceV2{Meta: NewTopologyRecordMetaV2(TopologyRecordKindDevice, "d1", nil, nil), Hostname: "sw1"},
		&InterfaceV2{Meta: NewTopologyRecordMetaV2(TopologyRecordKindInterface, "i1", nil, nil), DeviceID: "d1", Name: "Gi1/0/1"},
		&LinkV2{Meta: NewTopologyRecordMetaV2(TopologyRecordKindLink, "l1", nil, nil)},
		&VLANV2{Meta: NewTopologyRecordMetaV2(TopologyRecordKindVLAN, "vlan:10", nil, nil), VLANID: 10},
		&RouteV2{Meta: NewTopologyRecordMetaV2(TopologyRecordKindRoute, "r1", nil, nil), DestinationPrefix: "10.0.0.0/24"},
		&SecurityPolicyV2{Meta: NewTopologyRecordMetaV2(TopologyRecordKindSecurityPolicy, "p1", nil, nil), Name: "allow-web", Action: "allow"},
		&NATRuleV2{Meta: NewTopologyRecordMetaV2(TopologyRecordKindNATRule, "n1", nil, nil), Type: "source", Enabled: true},
		&VPNV2{Meta: NewTopologyRecordMetaV2(TopologyRecordKindVPN, "vpn1", nil, nil), Name: "site-a", Type: "ipsec"},
	}

	for _, record := range records {
		meta := record.TopologyMetadata()
		if meta == nil {
			t.Fatalf("%T returned nil metadata", record)
		}
		if meta.SchemaVersion != TopologySchemaVersionV2 {
			t.Fatalf("%T missing schema version", record)
		}
		if record.TopologyKind() != meta.RecordKind {
			t.Fatalf("%T kind mismatch: method=%s meta=%s", record, record.TopologyKind(), meta.RecordKind)
		}
		if meta.Evidence == nil {
			t.Fatalf("%T must expose evidence array even when empty", record)
		}
	}
}

func TestConfidenceProfileFromEvidenceV2ScoresFields(t *testing.T) {
	evidence := []EvidenceRefV2{
		{SourceType: ConfidenceSourceConfig, SourceSystem: "parser", Fields: []string{"hostname"}, Quality: 1.0},
		{SourceType: ConfidenceSourceConnector, SourceSystem: "netbox", Fields: []string{"hostname"}, Quality: 0.8},
		{SourceType: ConfidenceSourceInferred, SourceSystem: "heuristic", Fields: []string{"role"}, Quality: 0.5},
	}

	profile := ConfidenceProfileFromEvidenceV2(evidence, []string{"hostname", "role"})
	if profile.Overall <= 0 {
		t.Fatalf("expected positive overall score, got %.2f", profile.Overall)
	}
	if profile.PerAttribute["hostname"] <= profile.PerAttribute["role"] {
		t.Fatalf("expected stronger hostname evidence than role evidence: %#v", profile.PerAttribute)
	}
	if profile.Tier == ConfidenceTierNone {
		t.Fatalf("expected non-empty tier for evidence-backed profile")
	}
}

func TestLegacyAdaptersPopulateV2MetadataAndFields(t *testing.T) {
	device := LegacyDeviceToV2(Device{
		ID:              "dev-1",
		Hostname:        "core1",
		Vendor:          "cisco",
		Model:           "C9300",
		Version:         "17.9",
		VirtualContexts: []string{"default"},
		ManagementIP:    "192.0.2.10",
		Site:            "lab",
		Role:            "core",
		Interfaces:      []string{"Gi1/0/1"},
	}, nil)
	if device.Meta.RecordKind != TopologyRecordKindDevice || device.Meta.DeviceID != "dev-1" {
		t.Fatalf("bad device adapter metadata: %#v", device.Meta)
	}
	if len(device.ManagementIPs) != 1 || device.ManagementIPs[0] != "192.0.2.10" {
		t.Fatalf("management ip not mapped: %#v", device.ManagementIPs)
	}

	iface := LegacyInterfaceToV2(Interface{
		Name:            "Gi1/0/1",
		Addresses:       []string{"10.0.0.1/24"},
		VlanID:          10,
		Type:            "physical",
		Status:          "up",
		LinkStatus:      "up",
		VirtualFirewall: "default",
	}, "dev-1", nil)
	if iface.Meta.RecordKind != TopologyRecordKindInterface || iface.Meta.DeviceID != "dev-1" {
		t.Fatalf("bad interface adapter metadata: %#v", iface.Meta)
	}
	if len(iface.IPAddresses) != 1 || iface.IPAddresses[0].AddressFamily != "ipv4" {
		t.Fatalf("ip assignment not mapped: %#v", iface.IPAddresses)
	}
	if len(iface.VLANIDs) != 1 || iface.VLANIDs[0] != 10 {
		t.Fatalf("vlan id not mapped: %#v", iface.VLANIDs)
	}

	route := LegacyRoutingEntryToV2(RoutingEntry{Destination: "2001:db8::/32", NextHop: "fe80::1", Interface: "Gi1/0/2", Protocol: "static"}, "dev-1", nil)
	if route.AddressFamily != "ipv6" || len(route.NextHops) != 1 {
		t.Fatalf("route not mapped: %#v", route)
	}
}

func TestTopologyRecordV2JSONIncludesEvidenceAndConfidence(t *testing.T) {
	record := DeviceV2{
		Meta:     NewTopologyRecordMetaV2(TopologyRecordKindDevice, "device:sw1", nil, nil),
		Hostname: "sw1",
		Vendor:   "cisco",
	}
	payload, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	meta, ok := decoded["meta"].(map[string]any)
	if !ok {
		t.Fatalf("missing meta object in %s", payload)
	}
	if _, ok := meta["evidence"]; !ok {
		t.Fatalf("missing evidence in %s", payload)
	}
	if _, ok := meta["confidence"]; !ok {
		t.Fatalf("missing confidence in %s", payload)
	}
}

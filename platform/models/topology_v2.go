package models

import (
	"fmt"
	"time"
)

// TopologySchemaVersionV2 is the canonical schema identifier emitted by the
// normalized topology record model. It is intentionally a string, not a plain
// integer, so records can be routed across future incompatible families.
const TopologySchemaVersionV2 = "topology.record.v2"

// TopologyRecordKindV2 identifies the universal record kinds supported by v2.
type TopologyRecordKindV2 string

const (
	TopologyRecordKindDevice         TopologyRecordKindV2 = "device"
	TopologyRecordKindInterface      TopologyRecordKindV2 = "interface"
	TopologyRecordKindLink           TopologyRecordKindV2 = "link"
	TopologyRecordKindVLAN           TopologyRecordKindV2 = "vlan"
	TopologyRecordKindRoute          TopologyRecordKindV2 = "route"
	TopologyRecordKindSecurityPolicy TopologyRecordKindV2 = "security_policy"
	TopologyRecordKindNATRule        TopologyRecordKindV2 = "nat_rule"
	TopologyRecordKindVPN            TopologyRecordKindV2 = "vpn"
)

// TopologyRecordV2 is implemented by every normalized topology v2 record.
type TopologyRecordV2 interface {
	TopologyKind() TopologyRecordKindV2
	TopologyMetadata() *TopologyRecordMetaV2
}

// TopologyRecordMetaV2 is embedded in every topology v2 record via the Meta
// field. Evidence and confidence live here so consumers can uniformly inspect
// provenance and trust without knowing the concrete record type.
type TopologyRecordMetaV2 struct {
	SchemaVersion  string               `json:"schema_version" bson:"schema_version"`
	RecordKind     TopologyRecordKindV2 `json:"record_kind" bson:"record_kind"`
	RecordID       string               `json:"record_id" bson:"record_id"`
	TenantID       string               `json:"tenant_id,omitempty" bson:"tenant_id,omitempty"`
	DeviceID       string               `json:"device_id,omitempty" bson:"device_id,omitempty"`
	VirtualContext string               `json:"virtual_context,omitempty" bson:"virtual_context,omitempty"`
	SourceVendor   string               `json:"source_vendor,omitempty" bson:"source_vendor,omitempty"`
	SourcePlatform string               `json:"source_platform,omitempty" bson:"source_platform,omitempty"`
	Evidence       []EvidenceRefV2      `json:"evidence" bson:"evidence"`
	Confidence     *ConfidenceProfile   `json:"confidence" bson:"confidence"`
	Tags           map[string]string    `json:"tags,omitempty" bson:"tags,omitempty"`
	CreatedAt      time.Time            `json:"created_at" bson:"created_at"`
	UpdatedAt      time.Time            `json:"updated_at" bson:"updated_at"`
}

// EvidenceRefV2 captures source-level provenance for a record or attribute.
// A single topology record may contain multiple references when parsers,
// operational commands, connectors, or inference engines corroborate a fact.
type EvidenceRefV2 struct {
	ID                string           `json:"id,omitempty" bson:"id,omitempty"`
	SourceType        ConfidenceSource `json:"source_type" bson:"source_type"`
	SourceSystem      string           `json:"source_system" bson:"source_system"`
	Collector         string           `json:"collector,omitempty" bson:"collector,omitempty"`
	Parser            string           `json:"parser,omitempty" bson:"parser,omitempty"`
	Vendor            string           `json:"vendor,omitempty" bson:"vendor,omitempty"`
	Platform          string           `json:"platform,omitempty" bson:"platform,omitempty"`
	DeviceName        string           `json:"device_name,omitempty" bson:"device_name,omitempty"`
	Command           string           `json:"command,omitempty" bson:"command,omitempty"`
	FilePath          string           `json:"file_path,omitempty" bson:"file_path,omitempty"`
	SourceBlock       string           `json:"source_block,omitempty" bson:"source_block,omitempty"`
	SourceSnippetHash string           `json:"source_snippet_hash,omitempty" bson:"source_snippet_hash,omitempty"`
	LineStart         int              `json:"line_start,omitempty" bson:"line_start,omitempty"`
	LineEnd           int              `json:"line_end,omitempty" bson:"line_end,omitempty"`
	ObservedAt        time.Time        `json:"observed_at" bson:"observed_at"`
	Fields            []string         `json:"fields,omitempty" bson:"fields,omitempty"`
	Quality           float64          `json:"quality" bson:"quality"`
	Notes             []string         `json:"notes,omitempty" bson:"notes,omitempty"`
}

// TopologyValidationIssueV2 is returned by ValidateTopologyRecordMetaV2 and
// tests/helpers without pulling in a validation framework.
type TopologyValidationIssueV2 struct {
	Field   string `json:"field" bson:"field"`
	Message string `json:"message" bson:"message"`
}

// NewTopologyRecordMetaV2 returns metadata with schema version, evidence slice,
// confidence profile, and timestamps initialized consistently.
func NewTopologyRecordMetaV2(kind TopologyRecordKindV2, recordID string, evidence []EvidenceRefV2, confidence *ConfidenceProfile) TopologyRecordMetaV2 {
	now := time.Now().UTC()
	if evidence == nil {
		evidence = []EvidenceRefV2{}
	}
	NormalizeEvidenceRefsV2(evidence)
	if confidence == nil {
		confidence = ConfidenceProfileFromEvidenceV2(evidence, nil)
	}
	return TopologyRecordMetaV2{
		SchemaVersion: TopologySchemaVersionV2,
		RecordKind:    kind,
		RecordID:      recordID,
		Evidence:      evidence,
		Confidence:    confidence,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

// NormalizeEvidenceRefsV2 fills defaults and clamps evidence quality in-place.
func NormalizeEvidenceRefsV2(evidence []EvidenceRefV2) {
	now := time.Now().UTC()
	for i := range evidence {
		if evidence[i].SourceType == "" {
			evidence[i].SourceType = ConfidenceSourceConfig
		}
		if evidence[i].SourceSystem == "" {
			evidence[i].SourceSystem = string(evidence[i].SourceType)
		}
		if evidence[i].Quality == 0 {
			evidence[i].Quality = 1.0
		}
		evidence[i].Quality = clampScore(evidence[i].Quality)
		if evidence[i].ObservedAt.IsZero() {
			evidence[i].ObservedAt = now
		}
	}
}

// ConfidenceProfileFromEvidenceV2 builds a simple profile from evidence refs.
// When fields is empty it scores a synthetic "record" attribute; otherwise each
// requested field gets the evidence-weighted average across matching refs.
func ConfidenceProfileFromEvidenceV2(evidence []EvidenceRefV2, fields []string) *ConfidenceProfile {
	profile := NewConfidenceProfile()
	NormalizeEvidenceRefsV2(evidence)
	useAllEvidence := len(fields) == 0
	if useAllEvidence {
		fields = []string{"record"}
	}
	for _, field := range fields {
		var total float64
		var count int
		var source ConfidenceSource
		for _, ev := range evidence {
			if !useAllEvidence && len(ev.Fields) > 0 && !containsStringV2(ev.Fields, field) {
				continue
			}
			weight, ok := SourceWeights[ev.SourceType]
			if !ok || weight == 0 {
				weight = SourceWeights[ConfidenceSourceConfig]
			}
			total += clampScore(weight * ev.Quality)
			count++
			if source == "" {
				source = ev.SourceType
			}
		}
		if count == 0 {
			profile.AddAttribute(field, 0, ConfidenceSourceInferred, "no evidence references matched field")
			continue
		}
		profile.AddAttribute(field, total/float64(count), source, fmt.Sprintf("derived from %d topology v2 evidence reference(s)", count))
	}
	profile.SetOverall()
	return profile
}

// ValidateTopologyRecordMetaV2 checks universal invariants shared by every v2
// record. Concrete validators can append to this list.
func ValidateTopologyRecordMetaV2(meta TopologyRecordMetaV2, expectedKind TopologyRecordKindV2) []TopologyValidationIssueV2 {
	var issues []TopologyValidationIssueV2
	if meta.SchemaVersion != TopologySchemaVersionV2 {
		issues = append(issues, TopologyValidationIssueV2{"meta.schema_version", "must equal " + TopologySchemaVersionV2})
	}
	if meta.RecordKind != expectedKind {
		issues = append(issues, TopologyValidationIssueV2{"meta.record_kind", "must equal " + string(expectedKind)})
	}
	if meta.RecordID == "" {
		issues = append(issues, TopologyValidationIssueV2{"meta.record_id", "is required"})
	}
	if meta.Evidence == nil {
		issues = append(issues, TopologyValidationIssueV2{"meta.evidence", "must be present, use an empty array when provenance is unavailable"})
	}
	if meta.Confidence == nil {
		issues = append(issues, TopologyValidationIssueV2{"meta.confidence", "is required"})
	}
	return issues
}

// DeviceV2 is the normalized representation of network device identity and role.
type DeviceV2 struct {
	Meta            TopologyRecordMetaV2 `json:"meta" bson:"meta"`
	Hostname        string               `json:"hostname" bson:"hostname"`
	Vendor          string               `json:"vendor,omitempty" bson:"vendor,omitempty"`
	Model           string               `json:"model,omitempty" bson:"model,omitempty"`
	OSName          string               `json:"os_name,omitempty" bson:"os_name,omitempty"`
	OSVersion       string               `json:"os_version,omitempty" bson:"os_version,omitempty"`
	SerialNumber    string               `json:"serial_number,omitempty" bson:"serial_number,omitempty"`
	ManagementIPs   []string             `json:"management_ips,omitempty" bson:"management_ips,omitempty"`
	Site            string               `json:"site,omitempty" bson:"site,omitempty"`
	Role            string               `json:"role,omitempty" bson:"role,omitempty"`
	VirtualContexts []string             `json:"virtual_contexts,omitempty" bson:"virtual_contexts,omitempty"`
	InterfaceIDs    []string             `json:"interface_ids,omitempty" bson:"interface_ids,omitempty"`
	Attributes      map[string]string    `json:"attributes,omitempty" bson:"attributes,omitempty"`
}

func (r *DeviceV2) TopologyKind() TopologyRecordKindV2      { return TopologyRecordKindDevice }
func (r *DeviceV2) TopologyMetadata() *TopologyRecordMetaV2 { return &r.Meta }

// InterfaceV2 models a physical, virtual, routed, switched, aggregate, or tunnel interface.
type InterfaceV2 struct {
	Meta              TopologyRecordMetaV2 `json:"meta" bson:"meta"`
	DeviceID          string               `json:"device_id" bson:"device_id"`
	Name              string               `json:"name" bson:"name"`
	Type              string               `json:"type,omitempty" bson:"type,omitempty"`
	AdminStatus       string               `json:"admin_status,omitempty" bson:"admin_status,omitempty"`
	OperStatus        string               `json:"oper_status,omitempty" bson:"oper_status,omitempty"`
	MACAddress        string               `json:"mac_address,omitempty" bson:"mac_address,omitempty"`
	MTU               int                  `json:"mtu,omitempty" bson:"mtu,omitempty"`
	SpeedMbps         int                  `json:"speed_mbps,omitempty" bson:"speed_mbps,omitempty"`
	IPAddresses       []IPAssignmentV2     `json:"ip_addresses,omitempty" bson:"ip_addresses,omitempty"`
	VLANIDs           []int                `json:"vlan_ids,omitempty" bson:"vlan_ids,omitempty"`
	NativeVLANID      *int                 `json:"native_vlan_id,omitempty" bson:"native_vlan_id,omitempty"`
	ParentInterfaceID string               `json:"parent_interface_id,omitempty" bson:"parent_interface_id,omitempty"`
	AggregateID       string               `json:"aggregate_id,omitempty" bson:"aggregate_id,omitempty"`
	ZoneIDs           []string             `json:"zone_ids,omitempty" bson:"zone_ids,omitempty"`
	Description       string               `json:"description,omitempty" bson:"description,omitempty"`
}

func (r *InterfaceV2) TopologyKind() TopologyRecordKindV2      { return TopologyRecordKindInterface }
func (r *InterfaceV2) TopologyMetadata() *TopologyRecordMetaV2 { return &r.Meta }

type IPAssignmentV2 struct {
	Address       string `json:"address" bson:"address"`
	PrefixLength  int    `json:"prefix_length,omitempty" bson:"prefix_length,omitempty"`
	AddressFamily string `json:"address_family,omitempty" bson:"address_family,omitempty"`
	Scope         string `json:"scope,omitempty" bson:"scope,omitempty"`
	Primary       bool   `json:"primary,omitempty" bson:"primary,omitempty"`
}

// LinkV2 connects two interfaces/devices with optional discovery protocol data.
type LinkV2 struct {
	Meta          TopologyRecordMetaV2 `json:"meta" bson:"meta"`
	EndpointA     LinkEndpointV2       `json:"endpoint_a" bson:"endpoint_a"`
	EndpointB     LinkEndpointV2       `json:"endpoint_b" bson:"endpoint_b"`
	Type          string               `json:"type,omitempty" bson:"type,omitempty"`
	Protocol      string               `json:"protocol,omitempty" bson:"protocol,omitempty"`
	State         string               `json:"state,omitempty" bson:"state,omitempty"`
	BandwidthMbps int                  `json:"bandwidth_mbps,omitempty" bson:"bandwidth_mbps,omitempty"`
	LatencyMs     *float64             `json:"latency_ms,omitempty" bson:"latency_ms,omitempty"`
}

func (r *LinkV2) TopologyKind() TopologyRecordKindV2      { return TopologyRecordKindLink }
func (r *LinkV2) TopologyMetadata() *TopologyRecordMetaV2 { return &r.Meta }

type LinkEndpointV2 struct {
	DeviceID    string `json:"device_id" bson:"device_id"`
	InterfaceID string `json:"interface_id,omitempty" bson:"interface_id,omitempty"`
	Name        string `json:"name,omitempty" bson:"name,omitempty"`
	IPAddress   string `json:"ip_address,omitempty" bson:"ip_address,omitempty"`
	MACAddress  string `json:"mac_address,omitempty" bson:"mac_address,omitempty"`
}

// VLANV2 models layer-2 broadcast domains and optional overlay identifiers.
type VLANV2 struct {
	Meta         TopologyRecordMetaV2 `json:"meta" bson:"meta"`
	VLANID       int                  `json:"vlan_id" bson:"vlan_id"`
	Name         string               `json:"name,omitempty" bson:"name,omitempty"`
	DeviceIDs    []string             `json:"device_ids,omitempty" bson:"device_ids,omitempty"`
	InterfaceIDs []string             `json:"interface_ids,omitempty" bson:"interface_ids,omitempty"`
	VNI          *int                 `json:"vni,omitempty" bson:"vni,omitempty"`
	State        string               `json:"state,omitempty" bson:"state,omitempty"`
}

func (r *VLANV2) TopologyKind() TopologyRecordKindV2      { return TopologyRecordKindVLAN }
func (r *VLANV2) TopologyMetadata() *TopologyRecordMetaV2 { return &r.Meta }

// RouteV2 models routing table entries from static, connected, or dynamic protocols.
type RouteV2 struct {
	Meta              TopologyRecordMetaV2 `json:"meta" bson:"meta"`
	DestinationPrefix string               `json:"destination_prefix" bson:"destination_prefix"`
	AddressFamily     string               `json:"address_family,omitempty" bson:"address_family,omitempty"`
	VRF               string               `json:"vrf,omitempty" bson:"vrf,omitempty"`
	Protocol          string               `json:"protocol,omitempty" bson:"protocol,omitempty"`
	Type              string               `json:"type,omitempty" bson:"type,omitempty"`
	NextHops          []RouteNextHopV2     `json:"next_hops,omitempty" bson:"next_hops,omitempty"`
	Metric            int                  `json:"metric,omitempty" bson:"metric,omitempty"`
	AdminDistance     int                  `json:"admin_distance,omitempty" bson:"admin_distance,omitempty"`
	Tags              []string             `json:"tags,omitempty" bson:"tags,omitempty"`
}

func (r *RouteV2) TopologyKind() TopologyRecordKindV2      { return TopologyRecordKindRoute }
func (r *RouteV2) TopologyMetadata() *TopologyRecordMetaV2 { return &r.Meta }

type RouteNextHopV2 struct {
	IPAddress   string `json:"ip_address,omitempty" bson:"ip_address,omitempty"`
	InterfaceID string `json:"interface_id,omitempty" bson:"interface_id,omitempty"`
	Interface   string `json:"interface,omitempty" bson:"interface,omitempty"`
	Weight      int    `json:"weight,omitempty" bson:"weight,omitempty"`
}

// SecurityPolicyV2 models ACL/firewall policy intent across vendors.
type SecurityPolicyV2 struct {
	Meta                 TopologyRecordMetaV2 `json:"meta" bson:"meta"`
	Name                 string               `json:"name" bson:"name"`
	Enabled              bool                 `json:"enabled" bson:"enabled"`
	Action               string               `json:"action" bson:"action"`
	Direction            string               `json:"direction,omitempty" bson:"direction,omitempty"`
	Order                int                  `json:"order,omitempty" bson:"order,omitempty"`
	SourceZones          []string             `json:"source_zones,omitempty" bson:"source_zones,omitempty"`
	DestinationZones     []string             `json:"destination_zones,omitempty" bson:"destination_zones,omitempty"`
	SourceAddresses      []string             `json:"source_addresses,omitempty" bson:"source_addresses,omitempty"`
	DestinationAddresses []string             `json:"destination_addresses,omitempty" bson:"destination_addresses,omitempty"`
	Services             []string             `json:"services,omitempty" bson:"services,omitempty"`
	Applications         []string             `json:"applications,omitempty" bson:"applications,omitempty"`
	Log                  bool                 `json:"log" bson:"log"`
	NATRuleIDs           []string             `json:"nat_rule_ids,omitempty" bson:"nat_rule_ids,omitempty"`
	Description          string               `json:"description,omitempty" bson:"description,omitempty"`
}

func (r *SecurityPolicyV2) TopologyKind() TopologyRecordKindV2 {
	return TopologyRecordKindSecurityPolicy
}
func (r *SecurityPolicyV2) TopologyMetadata() *TopologyRecordMetaV2 { return &r.Meta }

// NATRuleV2 normalizes source/destination/static/dynamic NAT variants.
type NATRuleV2 struct {
	Meta                  TopologyRecordMetaV2 `json:"meta" bson:"meta"`
	Name                  string               `json:"name,omitempty" bson:"name,omitempty"`
	Type                  string               `json:"type" bson:"type"`
	Direction             string               `json:"direction,omitempty" bson:"direction,omitempty"`
	OriginalSource        []string             `json:"original_source,omitempty" bson:"original_source,omitempty"`
	OriginalDestination   []string             `json:"original_destination,omitempty" bson:"original_destination,omitempty"`
	TranslatedSource      []string             `json:"translated_source,omitempty" bson:"translated_source,omitempty"`
	TranslatedDestination []string             `json:"translated_destination,omitempty" bson:"translated_destination,omitempty"`
	OriginalService       []string             `json:"original_service,omitempty" bson:"original_service,omitempty"`
	TranslatedService     []string             `json:"translated_service,omitempty" bson:"translated_service,omitempty"`
	InboundInterfaceID    string               `json:"inbound_interface_id,omitempty" bson:"inbound_interface_id,omitempty"`
	OutboundInterfaceID   string               `json:"outbound_interface_id,omitempty" bson:"outbound_interface_id,omitempty"`
	Bidirectional         bool                 `json:"bidirectional" bson:"bidirectional"`
	Enabled               bool                 `json:"enabled" bson:"enabled"`
}

func (r *NATRuleV2) TopologyKind() TopologyRecordKindV2      { return TopologyRecordKindNATRule }
func (r *NATRuleV2) TopologyMetadata() *TopologyRecordMetaV2 { return &r.Meta }

// VPNV2 models site-to-site, remote-access, GRE, and other tunnel overlays.
type VPNV2 struct {
	Meta              TopologyRecordMetaV2 `json:"meta" bson:"meta"`
	Name              string               `json:"name" bson:"name"`
	Type              string               `json:"type" bson:"type"`
	Mode              string               `json:"mode,omitempty" bson:"mode,omitempty"`
	LocalEndpoint     string               `json:"local_endpoint" bson:"local_endpoint"`
	RemoteEndpoint    string               `json:"remote_endpoint" bson:"remote_endpoint"`
	LocalNetworks     []string             `json:"local_networks,omitempty" bson:"local_networks,omitempty"`
	RemoteNetworks    []string             `json:"remote_networks,omitempty" bson:"remote_networks,omitempty"`
	EncryptionProfile string               `json:"encryption_profile,omitempty" bson:"encryption_profile,omitempty"`
	Routing           string               `json:"routing,omitempty" bson:"routing,omitempty"`
	State             string               `json:"state,omitempty" bson:"state,omitempty"`
	FailoverGroup     string               `json:"failover_group,omitempty" bson:"failover_group,omitempty"`
}

func (r *VPNV2) TopologyKind() TopologyRecordKindV2      { return TopologyRecordKindVPN }
func (r *VPNV2) TopologyMetadata() *TopologyRecordMetaV2 { return &r.Meta }

func containsStringV2(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

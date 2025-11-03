package models

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Network merge status constants
const (
	NetworkMergeStatusSingle = "single" // Single firewall network
	NetworkMergeStatusMerged = "merged" // Merged network from multiple firewalls
	NetworkMergeStatusSplit  = "split"  // Previously merged network that was split
)

type Relationship struct {
	Key         string `json:"_key,omitempty"`
	Source      string `json:"_from,omitempty"`
	Destination string `json:"_to,omitempty"`
}

// Links
type Link struct {
	Id                     string            `json:"_id" bson:"_id"`
	FromID                 string            `json:"_from" bson:"_from"`
	ToID                   string            `json:"_to" bson:"_to"`
	Source                 string            `json:"source" bson:"source"`
	Destination            string            `json:"destination" bson:"destination"`
	EdgeName               string            `json:"edge_name" bson:"edge_name"`
	Port                   string            `json:"port" bson:"port"`
	Protocol               string            `json:"protocol" bson:"protocol"`
	IsOTProtocol           bool              `json:"is_ot_protocol" bson:"is_ot_protocol"`
	IsRemoteProtocol       bool              `json:"is_remote_protocol" bson:"is_remote_protocol"`
	IsFileTransferProtocol bool              `json:"is_file_transfer_protocol" bson:"is_file_transfer_protocol"`
	IsDirected             bool              `json:"is_directed" bson:"is_directed"`
	IsRelationship         bool              `json:"is_relationship" bson:"is_relationship"`
	Interface              PhysicalInterface `json:"interface,omitempty" bson:"interface,omitempty"`           // source interface
	DestInterface          PhysicalInterface `json:"dest_interface,omitempty" bson:"dest_interface,omitempty"` // dest interface
	PortMap                []LinkPortMap     `json:"port_map,omitempty" bson:"port_map,omitempty"`
	FirewallNames          []string          `json:"firewall_names,omitempty" bson:"firewall_names,omitempty"`
	UpdatedAt              string            `json:"updated_at,omitempty" bson:"updated_at,omitempty"`
	SourcePorts            []string          `json:"source_ports,omitempty" bson:"source_ports,omitempty"`
	DestinationPorts       []string          `json:"destination_ports,omitempty" bson:"destination_ports,omitempty"`
	ServicePorts           []LinkPortMap     `json:"service_ports,omitempty" bson:"service_ports,omitempty"`
	ApplicationPorts       []LinkPortMap     `json:"application_ports,omitempty" bson:"application_ports,omitempty"`
	RuleHit                string            `json:"rule_hit,omitempty" bson:"rule_hit,omitempty"`
	SourceRule             string            `json:"source_rule,omitempty" bson:"source_rule,omitempty"`
	DestinationRule        string            `json:"destination_rule,omitempty" bson:"destination_rule,omitempty"`

	// Lifecycle
	Lifecycle RecordLifecycle `json:"lifecycle" bson:"lifecycle"`
}

type LinkPortMap struct {
	Port     string `json:"port" bson:"port"`
	Protocol string `json:"protocol" bson:"protocol"`
}

// AssetNetworkLink
type AssetNetworkLink struct {
	Asset   AssetNode   `json:"asset"`
	Network NetworkNode `json:"network"`
}

// AssetNode
type AssetNode struct {
	Id              string `json:"_id,omitempty" bson:"_id,omitempty"`
	Key             string `json:"_key,omitempty" bson:"_key,omitempty"`
	IP              string `json:"ip" bson:"ip" faker:"ipv4"`
	Subnet          string `json:"subnet" bson:"subnet" faker:"ipv4"`
	Mac             string `json:"mac" bson:"mac" faker:"mac_address"`
	Hostname        string `json:"hostname" bson:"hostname" faker:"domain_name"`
	ImpactLevel     string `json:"impact_level" bson:"impact_level" faker:"oneof: Low Medium High"`
	NetworkName     string `json:"network_name" bson:"network_name" faker:"url"`
	RiskLevel       string `json:"risk_level" bson:"risk_level" faker:"oneof: Low Medium High"`
	FirstSeen       string `json:"first_seen" bson:"first_seen" faker:"date"`
	LastSeen        string `json:"last_seen" bson:"last_seen" faker:"date"`
	Vendor          string `json:"vendor" bson:"vendor" faker:"name"`
	Family          string `json:"family" bson:"family" faker:"name"`
	Model           string `json:"model" bson:"model" faker:"name"`
	CatalogNumber   string `json:"catalog_number" bson:"catalog_number"`                        // Can be empty
	Serial          string `json:"serial" bson:"serial"`                                        // Can be empty
	FirmwareVersion string `json:"firmware_version" bson:"firmware_version" faker:"uuid_digit"` // Always empty (might be invalid)
	HardwareVersion string `json:"hardware_version" bson:"hardware_version" faker:"uuid_digit"`
	OSName          string `json:"os_name" bson:"os_name" faker:"oneof: Linxu Windows iOS"`
	OSVersion       string `json:"os_version" bson:"os_version" faker:"uuid_digit"`
	Role            string `json:"role" bson:"role" faker:"name"`
	ClientProtocols string `json:"client_protocols" bson:"client_protocols"`
	ServerProtocols string `json:"server_protocols" bson:"server_protocols"`

	// NetworkInterfaces is a list of network interfaces
	// that link to the NetworkNode
	PhysicalNetworkInterfaces []PhysicalInterface  `json:"physical_network_interfaces,omitempty" bson:"physical_network_interfaces,omitempty"` // Can be empty
	CompensatingControls      []string             `json:"compensating_controls,omitempty" bson:"compensating_controls,omitempty"`             // Can be empty
	ExposureScore             float64              `json:"exposure_score" bson:"exposure_score"`
	Metadata                  NodeMetadata         `json:"metadata,omitempty" bson:"metadata,omitempty"`   // Can be empty
	DeviceType                string               `json:"device_type" bson:"device_type"`                 // Can be empty
	Services                  []string             `json:"services" bson:"services"`                       // Always empty (might be invalid)
	VulnerabilityCount        int32                `json:"vulnerability_count" bson:"vulnerability_count"` // looks duplicated with network_device_metrics.vulnerability_count, but has different values
	UpdatedBy                 string               `json:"updated_by" bson:"updated_by"`
	Decay                     float64              `json:"decay" bson:"decay"`                     // Always empty (might be invalid), inactivity score (0.0-1.0)
	DecayLevel                string               `json:"decay_level" bson:"decay_level"`         // Always empty (might be invalid), "active", "stale", "decayed"
	DecayUpdated              string               `json:"decay_updated" bson:"decay_updated"`     // Always empty (might be invalid), when decay was last calculated
	Integration               string               `json:"integration" bson:"integration"`         // integration that created/updated this record
	DataFreshness             string               `json:"data_freshness" bson:"data_freshness"`   // TODO: Missing in addFields stage
	InAttackPaths             bool                 `json:"in_attack_paths" bson:"in_attack_paths"` // TODO: need to check if this is correct (looks the same as used_in_attack_paths)
	IsRecentlyDiscovered      bool                 `json:"is_recently_discovered" bson:"is_recently_discovered"`
	FromOptica                bool                 `json:"from_optica" bson:"from_optica"`
	BusinessUnit              string               `json:"business_unit" bson:"business_unit"`
	Site                      string               `json:"site" bson:"site"`
	IsNetworkDevice           bool                 `json:"is_network_device" bson:"is_network_device"`
	NetworkDeviceMetrics      NetworkDeviceMetrics `json:"network_device_metrics,omitempty" bson:"network_device_metrics,omitempty"`

	// Analysis and risk
	UsedInAttackPaths     bool    `json:"used_in_attack_paths" bson:"used_in_attack_paths"`
	IsCrownJewel          bool    `json:"is_crown_jewel" bson:"is_crown_jewel"`
	RuleAnalysisSupported bool    `json:"rule_analysis_supported" bson:"rule_analysis_supported"`
	RoutingSupported      bool    `json:"routing_supported" bson:"routing_supported"`
	ConfigurationParsed   bool    `json:"configuration_parsed" bson:"configuration_parsed"`
	LastConfigUpdate      *string `json:"last_config_update,omitempty" bson:"last_config_update"` // Can be empty

	// Timestamps
	CreatedAt string `json:"created_at" bson:"created_at"`
	UpdatedAt string `json:"updated_at" bson:"updated_at"`

	// Lifecycle
	Lifecycle RecordLifecycle `json:"lifecycle" bson:"lifecycle"`

	// Endpoint Integration specific fields
	EndpointData EndpointData `json:"endpoint_data,omitempty" bson:"endpoint_data,omitempty"`

	// Layer-2 enrichment (optional)
	L2Placement *L2AssetPlacement `json:"l2_placement,omitempty" bson:"l2_placement,omitempty"`
}

type NetworkDeviceMetrics struct {
	RulesCount              int64    `json:"rules_count" bson:"rules_count"`
	SegmentationIssuesCount int64    `json:"segmentation_issues_count" bson:"segmentation_issues_count"`
	ServerProtocols         []string `json:"server_protocols" bson:"server_protocols"`
	VulnerabilityCount      int64    `json:"vulnerability_count" bson:"vulnerability_count"`
}

type AssetNodeOmitEmpty struct {
	Id              string `json:"_id,omitempty" bson:"_id,omitempty"`
	Key             string `json:"_key,omitempty" bson:"_key,omitempty"`
	IP              string `json:"ip" bson:"ip" faker:"ipv4"`
	Subnet          string `json:"subnet" faker:"ipv4"`
	Mac             string `json:"mac,omitempty" faker:"mac_address"`
	Hostname        string `json:"hostname,omitempty" faker:"domain_name"`
	ImpactLevel     string `json:"impact_level,omitempty" faker:"oneof: Low Medium High"`
	NetworkName     string `json:"network_name" faker:"url"`
	RiskLevel       string `json:"risk_level,omitempty" faker:"oneof: Low Medium High"`
	FirstSeen       string `json:"first_seen,omitempty" faker:"date"`
	LastSeen        string `json:"last_seen,omitempty" faker:"date"`
	Vendor          string `json:"vendor,omitempty" faker:"name"`
	Family          string `json:"family,omitempty" faker:"name"`
	Model           string `json:"model,omitempty" faker:"name"`
	CatalogNumber   string `json:"catalog_number,omitempty"`
	Serial          string `json:"serial,omitempty"`
	FirmwareVersion string `json:"firmware_version,omitempty" faker:"uuid_digit"`
	HardwareVersion string `json:"hardware_version,omitempty" faker:"uuid_digit"`
	OSName          string `json:"os_name,omitempty" faker:"oneof: Linxu Windows iOS"`
	OSVersion       string `json:"os_version,omitempty" faker:"uuid_digit"`
	Role            string `json:"role,omitempty" faker:"name"`
	ClientProtocols string `json:"client_protocols,omitempty"`
	ServerProtocols string `json:"server_protocols,omitempty"`
}

// Vulnerability Node
type VulnerabilityNode struct {
	Id string `json:"_id,omitempty" bson:"_id,omitempty"`
	//Key is CVEID
	Key                      string       `json:"_key,omitempty" bson:"_key,omitempty"`
	CVEID                    string       `json:"cve_id" bson:"cve_id"`
	AddtionalID              string       `json:"additional_id" bson:"additional_id"`
	Tags                     []string     `json:"tags" bson:"tags"`
	Summary                  string       `json:"summary" bson:"summary"`
	CVSS2                    string       `json:"cvss2" bson:"cvss2"`
	PublicExploit            string       `json:"public_exploit" bson:"public_exploit"`
	Protocols                []string     `json:"protocols" bson:"protocols"`
	Ports                    []string     `json:"ports" bson:"ports"`
	Name                     string       `json:"name" bson:"name"`
	Type                     string       `json:"type" bson:"type"`
	Impact                   string       `json:"impact" bson:"impact"`
	VendorID                 string       `json:"vendor_id" bson:"vendor_id"`
	CVSS3                    string       `json:"cvss3" bson:"cvss3"`
	OriginalRelease          string       `json:"original_release" bson:"original_release"`
	LastRevised              string       `json:"last_revised" bson:"last_revised"`
	CVESource                string       `json:"cve_source" bson:"cve_source"`
	VendorAdvisory           string       `json:"vendor_advisory" bson:"vendor_advisory"`
	AdditionalSources        []string     `json:"additional_sources" bson:"additional_sources"`
	InsertedTime             string       `json:"inserted_time" bson:"inserted_time"`
	LastModified             string       `json:"last_modified" bson:"last_modified"`
	Title                    string       `json:"title" bson:"title"`
	EPSSInfo                 EPSSDataItem `json:"epss_info" bson:"epss_info"`
	PCIComplianceStatus      string       `json:"pci_compliance_status" bson:"pci_compliance_status"`
	VulnerableSince          string       `json:"vulnerable_since" bson:"vulnerable_since"`
	Metadata                 NodeMetadata `json:"metadata,omitempty" bson:"metadata,omitempty"`
	IsKEV                    bool         `json:"is_kev" bson:"is_kev"`
	RansomwareCampaignStatus string       `json:"ransomware_campaign_status" bson:"ransomware_campaign_status"`

	// Enhanced fields from vuln-GraphLayer API
	VulnGraphLayerData     *VulnGraphLayerDetailedCatalog `json:"vuln_GraphLayer_data,omitempty" bson:"vuln_GraphLayer_data,omitempty"`
	EnhancedSummary    string                     `json:"enhanced_summary,omitempty" bson:"enhanced_summary,omitempty"`
	AttackComplexity   string                     `json:"attack_complexity,omitempty" bson:"attack_complexity,omitempty"`
	AttackVector       string                     `json:"attack_vector,omitempty" bson:"attack_vector,omitempty"`
	PrivilegesRequired string                     `json:"privileges_required,omitempty" bson:"privileges_required,omitempty"`
	UserInteraction    string                     `json:"user_interaction,omitempty" bson:"user_interaction,omitempty"`
	Scope              string                     `json:"scope,omitempty" bson:"scope,omitempty"`
	ExploitMaturity    string                     `json:"exploit_maturity,omitempty" bson:"exploit_maturity,omitempty"`
	DataSource         string                     `json:"data_source,omitempty" bson:"data_source,omitempty"`
	LastEnhanced       string                     `json:"last_enhanced,omitempty" bson:"last_enhanced,omitempty"`

	// NEW: Digital Twin specific fields to be added
	GraphLayerScore          float64 `json:"GraphLayer_score" bson:"GraphLayer_score"` // 0.0-1.0 percentage
	CVECount             int64   `json:"cve_count" bson:"cve_count"`
	AssetsAffected       int64   `json:"assets_affected" bson:"assets_affected"`
	DataFreshness        string  `json:"data_freshness" bson:"data_freshness"`
	DataFreshnessColor   string  `json:"data_freshness_color" bson:"data_freshness_color"`
	DaysSinceLastSeen    int32   `json:"days_since_last_seen" bson:"days_since_last_seen"`
	UsedInAttackPaths    bool    `json:"used_in_attack_paths" bson:"used_in_attack_paths"`
	ExposedToInternet    bool    `json:"exposed_to_internet" bson:"exposed_to_internet"`
	AffectsCrownJewels   bool    `json:"affects_crown_jewels" bson:"affects_crown_jewels"`
	IsRecentlyDiscovered bool    `json:"is_recently_discovered" bson:"is_recently_discovered"`
	FromOptica           bool    `json:"from_optica" bson:"from_optica"`
	Integration          string  `json:"integration" bson:"integration"`

	// Lifecycle
	Lifecycle RecordLifecycle `json:"lifecycle" bson:"lifecycle"`

	// Layer-2 metadata
	VLANID      *int   `json:"vlan_id,omitempty" bson:"vlan_id,omitempty"`
	VLANName    string `json:"vlan_name,omitempty" bson:"vlan_name,omitempty"`
	L2SegmentID string `json:"l2_segment_id,omitempty" bson:"l2_segment_id,omitempty"`
}

// L2AssetPlacement enriches an asset with MAC-to-port context and control posture.
type L2AssetPlacement struct {
	PrimaryMAC          string   `json:"primary_mac" bson:"primary_mac"`
	AdditionalMACs      []string `json:"additional_macs,omitempty" bson:"additional_macs,omitempty"`
	LastSeenSwitchID    string   `json:"last_seen_switch_id" bson:"last_seen_switch_id"`
	LastSeenPort        string   `json:"last_seen_port" bson:"last_seen_port"`
	LastSeenVLAN        int      `json:"last_seen_vlan" bson:"last_seen_vlan"`
	LastSeenTS          string   `json:"last_seen_ts" bson:"last_seen_ts"`
	PlacementConfidence float64  `json:"placement_confidence" bson:"placement_confidence"`
	PlacementFreshness  string   `json:"placement_freshness" bson:"placement_freshness"`
	EvidenceSource      string   `json:"evidence_source" bson:"evidence_source"`
	PortSecurityEnabled bool     `json:"port_security_enabled" bson:"port_security_enabled"`
	DHCPSnoopingActive  bool     `json:"dhcp_snooping_active" bson:"dhcp_snooping_active"`
	ARPInspectionActive bool     `json:"arp_inspection_active" bson:"arp_inspection_active"`
	Dot1xRequired       bool     `json:"dot1x_required" bson:"dot1x_required"`
}

// L2Link represents a Layer-2 adjacency between switchports or aggregates.
type L2Link struct {
	ID           string          `json:"id" bson:"_id,omitempty"`
	Key          string          `json:"key" bson:"key"`
	FromDevice   string          `json:"from_device" bson:"from_device"`
	FromPort     string          `json:"from_port" bson:"from_port"`
	ToDevice     string          `json:"to_device" bson:"to_device"`
	ToPort       string          `json:"to_port" bson:"to_port"`
	LinkType     string          `json:"link_type" bson:"link_type"`
	AllowedVLANs []int           `json:"allowed_vlans,omitempty" bson:"allowed_vlans,omitempty"`
	NativeVLAN   *int            `json:"native_vlan,omitempty" bson:"native_vlan,omitempty"`
	LAGMemberOf  *int            `json:"lag_member_of,omitempty" bson:"lag_member_of,omitempty"`
	UpdatedAt    string          `json:"updated_at" bson:"updated_at"`
	Lifecycle    RecordLifecycle `json:"lifecycle" bson:"lifecycle"`
}

// NetworkNode
type NetworkNode struct {
	Id                   string              `json:"_id,omitempty" bson:"_id,omitempty"`
	Key                  string              `json:"_key,omitempty" bson:"_key,omitempty"`
	NetworkName          string              `json:"network_name" bson:"network_name"`
	AddressGroups        []string            `json:"address_groups" bson:"address_groups"`
	Subnet               string              `json:"subnet" bson:"subnet"`
	InsertedTime         string              `json:"inserted_time" bson:"inserted_time"`
	LastModified         string              `json:"last_modified" bson:"last_modified"`
	FirewallNames        []string            `json:"firewall_names" bson:"firewall_names"` // An array of all firewalls associated with this node (replaces firewall_name)
	MergeStatus          string              `json:"merge_status" bson:"merge_status"`     // Use NetworkMergeStatus* constants
	SplitSources         []string            `json:"split_sources" bson:"split_sources"`   // An array of firewall names that were previously split from this node
	MergeKey             string              `json:"merge_key" bson:"merge_key"`           // A unique SHA-1 hash of subnet for lookup
	BusinessUnit         string              `json:"business_unit" bson:"business_unit"`
	Site                 string              `json:"site" bson:"site"`
	StartingPoint        bool                `json:"starting_point" bson:"starting_point"`
	Criticality          string              `json:"criticality" bson:"criticality"`
	ActionNeeded         bool                `json:"action_needed" bson:"action_needed"`
	AutoClassified       bool                `json:"auto_classified" bson:"auto_classified"`
	PhysicalInterfaces   []PhysicalInterface `json:"physical_interfaces,omitempty" bson:"physical_interfaces,omitempty"`
	CompensatingControls []string            `json:"compensating_controls,omitempty" bson:"compensating_controls,omitempty"`
	ExposureScore        float64             `json:"exposure_score" bson:"exposure_score"`
	FirewallContext      string              `json:"firewall_context,omitempty" bson:"firewall_context,omitempty"`
	Metadata             NodeMetadata        `json:"metadata,omitempty" bson:"metadata,omitempty"`

	// NEW: Digital Twin specific fields to be added
	DataFreshness      string `json:"data_freshness" bson:"data_freshness"`
	DataFreshnessColor string `json:"data_freshness_color" bson:"data_freshness_color"`
	DaysSinceLastSeen  int32  `json:"days_since_last_seen" bson:"days_since_last_seen"`
	InAttackPaths      bool   `json:"in_attack_paths" bson:"in_attack_paths"`
	IsCrownJewel       bool   `json:"is_crown_jewel" bson:"is_crown_jewel"`
	UpdatedBy          string `json:"updated_by" bson:"updated_by"`
	Status             string `json:"status" bson:"status"`

	// Lifecycle
	Lifecycle RecordLifecycle `json:"lifecycle" bson:"lifecycle"`

	// Layer-2 VLAN metadata (optional)
	VLANID      *int   `json:"vlan_id,omitempty" bson:"vlan_id,omitempty"`
	VLANName    string `json:"vlan_name,omitempty" bson:"vlan_name,omitempty"`
	L2SegmentID string `json:"l2_segment_id,omitempty" bson:"l2_segment_id,omitempty"`
}

// ids references to the actual objects
type SimulationAttackResponseDB struct {
	CompensatingControls []string `json:"compensating_controls,omitempty" bson:"compensating_controls,omitempty"`
	VerticesIds          []string `json:"vertices_ids" bson:"network_nodes_ids"`
	EdgesIds             []string `json:"edges_ids" bson:"links_ids"`
	VulnsIds             []string `json:"vulns_ids" bson:"vulns_ids"`
	MitigationsIds       []string `json:"mitigations_ids,omitempty" bson:"mitigations_ids,omitempty"`
	AssetsIds            []string `json:"assets_ids" bson:"asset_nodes_ids"`
}

// actual objects
type SimulationAttackResponse struct {
	CompensatingControls []string            `json:"compensating_controls,omitempty" bson:"compensating_controls,omitempty"`
	Vertices             []NetworkNode       `json:"vertices" bson:"network_nodes"`
	Edges                []Link              `json:"edges" bson:"links"`
	Vulns                []VulnerabilityNode `json:"vulnerabilities,omitempty" bson:"vulns,omitempty"`
	Mitigations          []MitigationNode    `json:"mitigations,omitempty"`
	Assets               []AssetNode         `json:"assets,omitempty" bson:"asset_nodes"`
	VerticesIds          []string            `json:"vertices_ids" bson:"network_nodes_ids"`
	EdgesIds             []string            `json:"edges_ids" bson:"links_ids"`
	VulnsIds             []string            `json:"vulns_ids" bson:"vulns_ids"`
	MitigationsIds       []string            `json:"mitigations_ids,omitempty" bson:"mitigations_ids,omitempty"`
	AssetsIds            []string            `json:"assets_ids" bson:"asset_nodes_ids"`
}

type SimlulationAttackVulns struct {
	Asset         AssetNode         `json:"asset"`
	Vulnerability VulnerabilityNode `json:"vulnerability"`
}

type MitigationNode struct {
	Id          string `json:"_id,omitempty"`
	Key         string `json:"_key,omitempty"`
	DefenseType string `json:"defense_type"`
}

type GetAssetsResponse struct {
	Assets []AssetNode `json:"assets"`
	Total  int32       `json:"total"`
}

// PhysicalInterface represents a network interface
type PhysicalInterface struct {
	Name       string   `json:"name" bson:"name"`               // e.g., "eth0", "eth1"
	Addresses  []string `json:"address" bson:"address"`         // IP addresses assigned to this interface
	NetworkIDs []string `json:"network_ids" bson:"network_ids"` // IDs of networks this interface connects to
}

type NodeMetadata struct {
	Parser      string `json:"parser,omitempty" bson:"parser,omitempty"`
	Version     string `json:"version,omitempty" bson:"version,omitempty"`
	Description string `json:"description,omitempty" bson:"description,omitempty"`
	SourceType  string `json:"source_type,omitempty" bson:"source_type,omitempty"`
}

// AssetFilters defines filtering options for asset queries
type AssetFilters struct {
	NetworkIDs         []string `json:"network_ids,omitempty" bson:"network_ids,omitempty"`
	NetworkGroups      []string `json:"network_groups,omitempty" bson:"network_groups,omitempty"`
	DeviceTypes        []string `json:"device_types,omitempty" bson:"device_types,omitempty"`
	Vendors            []string `json:"vendors,omitempty" bson:"vendors,omitempty"`
	OSNames            []string `json:"os_names,omitempty" bson:"os_names,omitempty"`
	Integrations       []string `json:"integrations,omitempty" bson:"integrations,omitempty"`
	HasVulnerabilities bool     `json:"has_vulnerabilities" bson:"has_vulnerabilities"` // true = with vulns, false = without vulns
	FilterByVulns      bool     `json:"filter_by_vulns" bson:"filter_by_vulns"`         // whether to apply vuln filter
	RiskLevels         []string `json:"risk_levels,omitempty" bson:"risk_levels,omitempty"`
	FirstSeenAfter     string   `json:"first_seen_after,omitempty" bson:"first_seen_after,omitempty"`
	FirstSeenBefore    string   `json:"first_seen_before,omitempty" bson:"first_seen_before,omitempty"`
	LastSeenAfter      string   `json:"last_seen_after,omitempty" bson:"last_seen_after,omitempty"`
	LastSeenBefore     string   `json:"last_seen_before,omitempty" bson:"last_seen_before,omitempty"`
	SearchTerm         string   `json:"search_term,omitempty" bson:"search_term,omitempty"`
	Subnets            []string `json:"subnets,omitempty" bson:"subnets,omitempty"`
	MinDecay           float64  `json:"min_decay" bson:"min_decay"`             // 0.0 = no filter
	MaxDecay           float64  `json:"max_decay" bson:"max_decay"`             // 0.0 = no filter
	FilterByDecay      bool     `json:"filter_by_decay" bson:"filter_by_decay"` // whether to apply decay filter
	Models             []string `json:"models,omitempty" bson:"models,omitempty"`
	Roles              []string `json:"roles,omitempty" bson:"roles,omitempty"`
	Families           []string `json:"families,omitempty" bson:"families,omitempty"`
}

// AssetStats provides statistical information about assets
type AssetStats struct {
	TotalAssets          int32            `json:"total_assets" bson:"total_assets"`
	AssetsWithVulns      int32            `json:"assets_with_vulnerabilities" bson:"assets_with_vulnerabilities"`
	AssetsByRiskLevel    map[string]int32 `json:"assets_by_risk_level" bson:"assets_by_risk_level"`
	AssetsByDeviceType   map[string]int32 `json:"assets_by_device_type" bson:"assets_by_device_type"`
	AssetsByIntegration  map[string]int32 `json:"assets_by_integration" bson:"assets_by_integration"`
	AssetsByVendor       map[string]int32 `json:"assets_by_vendor" bson:"assets_by_vendor"`
	AssetsByOSName       map[string]int32 `json:"assets_by_os_name" bson:"assets_by_os_name"`
	AssetsByDecayLevel   map[string]int32 `json:"assets_by_decay_level" bson:"assets_by_decay_level"`
	TotalVulnerabilities int32            `json:"total_vulnerabilities" bson:"total_vulnerabilities"`
	AvgVulnsPerAsset     float64          `json:"avg_vulnerabilities_per_asset" bson:"avg_vulnerabilities_per_asset"`
	AvgDecayScore        float64          `json:"avg_decay_score" bson:"avg_decay_score"`
	LastUpdated          string           `json:"last_updated" bson:"last_updated"`
	ActiveAssets         int32            `json:"active_assets" bson:"active_assets"`   // decay < 0.3
	StaleAssets          int32            `json:"stale_assets" bson:"stale_assets"`     // 0.3 <= decay < 0.7
	DecayedAssets        int32            `json:"decayed_assets" bson:"decayed_assets"` // decay >= 0.7
}

// UnmarshalJSON implements custom JSON unmarshaling for Sorting
func (asp *SortParams) UnmarshalJSON(data []byte) error {
	// Create a temporary struct to unmarshal the JSON
	type Alias SortParams
	aux := &struct {
		Direction string `json:"direction"`
		*Alias
	}{
		Alias: (*Alias)(asp),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// Convert string direction to enum
	if aux.Direction != "" {
		if direction, ok := ConvertSortDirectionFromString(aux.Direction); ok {
			asp.Direction = direction
		} else {
			return fmt.Errorf("invalid sort direction: %s", aux.Direction)
		}
	}

	return nil
}

// AssetSearchRequest defines the request body for advanced asset search
type AssetSearchRequest struct {
	Filters    AssetFilters     `json:"filters" bson:"filters"`
	Pagination PaginationParams `json:"pagination" bson:"pagination"`
	Sort       SortParams       `json:"sort" bson:"sort"`
}

// AssetExploreRequest defines the request body for asset explore functionality
type AssetExploreRequest struct {
	Filters    AssetFilters     `json:"filters" bson:"filters"`
	Pagination PaginationParams `json:"pagination" bson:"pagination"`
	Sort       SortParams       `json:"sort" bson:"sort"`
}

// AssetHealthSummary provides health status information about assets
type AssetHealthSummary struct {
	TotalAssets           int32                        `json:"total_assets" bson:"total_assets"`
	ActiveAssets          int32                        `json:"active_assets" bson:"active_assets"`   // seen recently
	StaleAssets           int32                        `json:"stale_assets" bson:"stale_assets"`     // somewhat old
	DecayedAssets         int32                        `json:"decayed_assets" bson:"decayed_assets"` // very old
	CriticalAssetsDecayed int32                        `json:"critical_assets_decayed" bson:"critical_assets_decayed"`
	DecayDistribution     map[string]int32             `json:"decay_distribution" bson:"decay_distribution"` // 0.0-0.2, 0.2-0.4, etc.
	IntegrationHealth     map[string]IntegrationHealth `json:"integration_health" bson:"integration_health"`
	LastCalculated        string                       `json:"last_calculated" bson:"last_calculated"`
}

// IntegrationHealth tracks health per integration source
type IntegrationHealth struct {
	TotalAssets   int32   `json:"total_assets" bson:"total_assets"`
	ActiveAssets  int32   `json:"active_assets" bson:"active_assets"`
	DecayedAssets int32   `json:"decayed_assets" bson:"decayed_assets"`
	AvgDecayScore float64 `json:"avg_decay_score" bson:"avg_decay_score"`
	LastIngestion string  `json:"last_ingestion" bson:"last_ingestion"`
	HealthScore   float64 `json:"health_score" bson:"health_score"` // 0.0-1.0, 1.0 = healthy
}

// AssetRiskAssessment provides risk analysis for an asset
type AssetRiskAssessment struct {
	OverallRisk        string   `json:"overall_risk" bson:"overall_risk"` // "Critical", "High", "Medium", "Low"
	VulnerabilityRisk  string   `json:"vulnerability_risk" bson:"vulnerability_risk"`
	ExposureRisk       string   `json:"exposure_risk" bson:"exposure_risk"`
	DecayRisk          string   `json:"decay_risk" bson:"decay_risk"`
	CriticalVulnCount  int32    `json:"critical_vuln_count" bson:"critical_vuln_count"`
	HighVulnCount      int32    `json:"high_vuln_count" bson:"high_vuln_count"`
	PublicExploitCount int32    `json:"public_exploit_count" bson:"public_exploit_count"`
	KEVVulnCount       int32    `json:"kev_vuln_count" bson:"kev_vuln_count"` // Known Exploited Vulnerabilities
	LastSecurityScan   string   `json:"last_security_scan" bson:"last_security_scan"`
	RecommendedActions []string `json:"recommended_actions" bson:"recommended_actions"`
	RiskScore          float64  `json:"risk_score" bson:"risk_score"` // 0.0-10.0
}

type GetExpandedAssetsResponse struct {
	Assets []AssetNode `json:"assets"`
	Total  int32       `json:"total"`
	Page   int32       `json:"page"`
	Limit  int32       `json:"limit"`
	Stats  *AssetStats `json:"stats,omitempty"` // optional stats for current filtered set
}

// AssetDetailedResponse provides comprehensive asset information for detail views
type AssetDetailedResponse struct {
	Asset              AssetNode           `json:"asset" bson:"asset"`
	Vulnerabilities    []VulnerabilityNode `json:"vulnerabilities,omitempty" bson:"vulnerabilities,omitempty"`
	VulnerabilityCount int32               `json:"vulnerability_count" bson:"vulnerability_count"`
	Networks           []NetworkNode       `json:"networks,omitempty" bson:"networks,omitempty"`
	Links              []Link              `json:"links,omitempty" bson:"links,omitempty"`
	LastSeenDaysAgo    int32               `json:"last_seen_days_ago" bson:"last_seen_days_ago"`
	FirstSeenDaysAgo   int32               `json:"first_seen_days_ago" bson:"first_seen_days_ago"`
	RelatedAssets      []AssetNode         `json:"related_assets,omitempty" bson:"related_assets,omitempty"` // assets in same network
	SecurityScore      float64             `json:"security_score" bson:"security_score"`                     // calculated security score
	RiskAssessment     AssetRiskAssessment `json:"risk_assessment" bson:"risk_assessment"`
}

// DuplicateAssetsResponse represents groups of duplicate assets
type DuplicateAssetsResponse struct {
	DuplicateGroups [][]AssetNode `json:"duplicate_groups" bson:"duplicate_groups"`
	TotalGroups     int32         `json:"total_groups" bson:"total_groups"`
	TotalDuplicates int32         `json:"total_duplicates" bson:"total_duplicates"`
}

// AssetValidationError represents a single asset with validation errors
type AssetValidationError struct {
	Asset  AssetNode `json:"asset" bson:"asset"`
	Errors []string  `json:"errors" bson:"errors"`
}

// AssetValidationResponse represents the response for assets with validation errors
type AssetValidationResponse struct {
	ValidationErrors []AssetValidationError `json:"validation_errors" bson:"validation_errors"`
	TotalAssets      int32                  `json:"total_assets" bson:"total_assets"`
	TotalErrors      int32                  `json:"total_errors" bson:"total_errors"`
}

type NetworkDeviceNode struct {
	ID              string `json:"id" bson:"id"`
	Hostname        string `json:"hostname" bson:"hostname"`
	IPAddress       string `json:"ip_address" bson:"ip_address"`
	DeviceType      string `json:"device_type" bson:"device_type"` // firewall, router, switch, ids, sensor
	Model           string `json:"model" bson:"model"`
	OSName          string `json:"os_name" bson:"os_name"`
	FirmwareVersion string `json:"firmware_version" bson:"firmware_version"`

	// Device-specific metrics
	Metrics NetworkDeviceMetrics `json:"metrics" bson:"metrics"`

	// Data freshness and tracking
	DataFreshness string `json:"data_freshness" bson:"data_freshness"`
	FirstSeen     string `json:"first_seen" bson:"first_seen"`
	LastSeen      string `json:"last_seen" bson:"last_seen"`

	// Security and compliance
	Integration string `json:"integration" bson:"integration"`
}

type NetworkDeviceNodeWithDetails struct {
	ID              string `json:"id" bson:"_id"`
	Hostname        string `json:"hostname" bson:"hostname"`
	IPAddress       string `json:"ip_address" bson:"ip_address"`
	DeviceType      string `json:"device_type" bson:"device_type"` // firewall, router, switch, ids, sensor
	Subnet          string `json:"subnet" bson:"subnet"`
	MacAddress      string `json:"mac_address" bson:"mac"`
	Model           string `json:"model" bson:"model"`
	Vendor          string `json:"vendor" bson:"vendor"`
	HardwareVersion string `json:"hardware_version" bson:"hardware_version"`
	OSName          string `json:"os_name" bson:"os_name"`
	OSVersion       string `json:"os_version" bson:"os_version"`
	FirmwareVersion string `json:"firmware_version" bson:"firmware_version"`

	// Device-specific metrics
	Metrics NetworkDeviceMetrics `json:"metrics" bson:"metrics"`

	// Data freshness and tracking
	DataFreshness string `json:"data_freshness" bson:"data_freshness"`
	FirstSeen     string `json:"first_seen" bson:"first_seen"`
	LastSeen      string `json:"last_seen" bson:"last_seen"`

	// Security and compliance
	CompensatingControls []string `json:"compensating_controls" bson:"compensating_controls"`
	BusinessUnit         string   `json:"business_unit" bson:"business_unit"`
	Site                 string   `json:"site" bson:"site"`
	Integration          string   `json:"integration" bson:"integration"`
	UpdatedBy            string   `json:"updated_by" bson:"updated_by"`

	// Analysis and risk
	UsedInAttackPaths     bool   `json:"used_in_attack_paths" bson:"used_in_attack_paths"`
	IsCrownJewel          bool   `json:"is_crown_jewel" bson:"is_crown_jewel"`
	RuleAnalysisSupported bool   `json:"rule_analysis_supported" bson:"rule_analysis_supported"`
	RoutingSupported      bool   `json:"routing_supported" bson:"routing_supported"`
	ConfigurationParsed   bool   `json:"configuration_parsed" bson:"configuration_parsed"`
	LastConfigUpdate      string `json:"last_config_update" bson:"last_config_update"`

	// Metadata
	Metadata NodeMetadata `json:"metadata" bson:"metadata"`

	// Timestamps
	CreatedAt string `json:"created_at" bson:"created_at"`
	UpdatedAt string `json:"updated_at" bson:"updated_at"`
}

func (n *NetworkNode) GetAssumeBreach() bool {
	return n.StartingPoint
}

func (n *NetworkNode) SetAssumeBreach(value bool) {
	n.StartingPoint = value
}

func (n *NetworkNode) GetBusinessImpact() string {
	return n.Criticality
}

func (n *NetworkNode) SetBusinessImpact(value string) {
	n.Criticality = value
}

func (n *NetworkNode) GetBusinessImpactColor() string {
	switch strings.ToLower(n.Criticality) {
	case "critical":
		return "red"
	case "high":
		return "orange"
	case "medium":
		return "yellow"
	case "low":
		return "blue"
	default:
		return "gray"
	}
}

func (n *NetworkNode) GetNetworkGroups() []string {
	return n.AddressGroups
}

func (n *NetworkNode) SetNetworkGroups(groups []string) {
	n.AddressGroups = groups
}

func (v *VulnerabilityNode) GetGraphLayerScoreBadge() string {
	switch {
	case v.GraphLayerScore >= 0.9:
		return "critical"
	case v.GraphLayerScore >= 0.7:
		return "high"
	case v.GraphLayerScore >= 0.4:
		return "medium"
	default:
		return "low"
	}
}

func (v *VulnerabilityNode) GetCVSSBadge() string {
	cvss3, err := strconv.ParseFloat(v.CVSS3, 64)
	if err != nil {
		return "low"
	}

	switch {
	case cvss3 >= 9.0:
		return "critical"
	case cvss3 >= 7.0:
		return "high"
	case cvss3 >= 4.0:
		return "medium"
	default:
		return "low"
	}
}

func (v *VulnerabilityNode) GetPublicExploitIcon() string {
	if v.PublicExploit == "Known" {
		return "exploit-available"
	}
	return "no-exploit"
}

func (v *VulnerabilityNode) GetEPSSScore() float64 {
	if v.EPSSInfo.Epss != "" {
		if score, err := strconv.ParseFloat(v.EPSSInfo.Epss, 64); err == nil {
			return score
		}
	}
	return 0.0
}

func (v *VulnerabilityNode) GetEPSSPercentile() float64 {
	if v.EPSSInfo.Percentile != "" {
		if percentile, err := strconv.ParseFloat(v.EPSSInfo.Percentile, 64); err == nil {
			return percentile
		}
	}
	return 0.0
}

// Package intelligence provides topology inference and graph intelligence for GraphLayer.
// Sprint 11: Fill in gaps — infer what isn't explicitly configured.
//
// Features:
//   - Topology inference from routing tables (next-hop → links)
//   - L2 topology inference from MAC + LLDP/CDP
//   - Subnet discovery and gateway inference
//   - Path inference (A→B through routing tables)
//   - Implicit link detection (matching subnets on interfaces)
//   - Network boundary detection
//   - NAT boundary inference
//   - Rule engine with priority and conflict resolution
package intelligence

import (
	"time"

	"github.com/mattpainter701/graphlayer/platform/models"
)

// ========================================================================
// Core Inference Types
// ========================================================================

// InferredLink represents a logical or physical link derived from inference.
// Distinguished from directly-parsed L2Link by its EvidenceSource and lower confidence.
type InferredLink struct {
	// Unique identifier for this inferred link
	ID string `json:"id"`

	// Endpoint identifiers
	FromDevice string `json:"from_device"`
	FromPort   string `json:"from_port,omitempty"`
	ToDevice   string `json:"to_device"`
	ToPort     string `json:"to_port,omitempty"`

	// Link characterization
	LinkType        string `json:"link_type"`         // physical, logical, inferred-l3, inferred-l2, implicit
	Protocol        string `json:"protocol,omitempty"` // ospf, bgp, static, lldp, mac-table
	InferenceMethod string `json:"inference_method"`  // routing-next-hop, mac-correlation, lldp-match, subnet-match

	// Evidence and confidence
	Confidence     float64                    `json:"confidence"`     // 0.0–1.0
	EvidenceSource models.ConfidenceSource   `json:"evidence_source"` // always "inferred"
	EvidenceDetail string                     `json:"evidence_detail"` // Human-readable explanation
	SupportingData map[string]interface{}     `json:"supporting_data,omitempty"` // Routes, MACs, LLDP entries that support this link

	// Lifecycle
	InferredAt time.Time `json:"inferred_at"`
}

// SubnetGroup represents a group of devices sharing a common IP prefix.
type SubnetGroup struct {
	// Subnet CIDR (e.g., "10.1.1.0/24")
	Subnet string `json:"subnet"`

	// Devices in this subnet
	Devices []string `json:"devices"` // Device hostnames or IDs

	// Inferred gateway device (if found)
	GatewayDevice string `json:"gateway_device,omitempty"`
	GatewayIP     string `json:"gateway_ip,omitempty"`

	// Interfaces that participate in this subnet
	Interfaces []SubnetInterface `json:"interfaces"`

	// VLAN association (if known)
	VLANID   int    `json:"vlan_id,omitempty"`
	VLANName string `json:"vlan_name,omitempty"`

	// Metadata
	InferredAt      time.Time                `json:"inferred_at"`
	Confidence      float64                  `json:"confidence"`
	EvidenceSources []models.ConfidenceSource `json:"evidence_sources"`
}

// SubnetInterface represents an interface participating in a subnet.
type SubnetInterface struct {
	Device    string `json:"device"`
	Interface string `json:"interface"`
	IP        string `json:"ip"`
}

// PathResult represents a computed routing path between two endpoints.
type PathResult struct {
	Source      string   `json:"source"`      // Source device
	Destination string   `json:"destination"` // Destination IP or device
	Hops        []L3Hop  `json:"hops"`        // Ordered list of hops
	Complete    bool     `json:"complete"`    // Did we reach the destination?
	PartialPath bool     `json:"partial_path"` // Path is incomplete but useful
	PathType    string   `json:"path_type"`    // direct, routed, unknown, loop
	Confidence  float64  `json:"confidence"`
	ComputedAt  time.Time `json:"computed_at"`
}

// L3Hop represents a single hop in a computed L3 path.
type L3Hop struct {
	Device         string `json:"device"`
	NextHop        string `json:"next_hop"`
	EgressInterface string `json:"egress_interface,omitempty"`
	Protocol       string `json:"protocol"` // static, ospf, bgp, connected
	Metric         int    `json:"metric"`
	AdminDistance  int    `json:"admin_distance"`
}

// NATBoundary represents an inferred NAT translation boundary.
type NATBoundary struct {
	// Private side
	PrivateIP      string `json:"private_ip"`
	PrivateSubnet  string `json:"private_subnet,omitempty"`

	// Public side
	PublicIP       string `json:"public_ip"`
	PublicDevice   string `json:"public_device,omitempty"`

	// Translation details
	NATType        string `json:"nat_type"`   // source, destination, static, dynamic, pat
	FirewallDevice string `json:"firewall_device,omitempty"`

	// Evidence
	Confidence     float64 `json:"confidence"`
	EvidenceSource string  `json:"evidence_source"` // nat-rule, routing-analysis, ip-correlation
	InferredAt     time.Time `json:"inferred_at"`
}

// NetworkBoundary represents where the managed network ends.
type NetworkBoundary struct {
	// Boundary characterization
	BoundaryType string `json:"boundary_type"` // internet-edge, wan-edge, vpn-endpoint, dmz-edge, nat-boundary

	// The device/interface at the boundary
	Device      string `json:"device"`
	Interface   string `json:"interface,omitempty"`

	// Inside vs outside characterization
	InsideNetworks  []string `json:"inside_networks"`  // Private IP ranges
	OutsideNetworks []string `json:"outside_networks"` // Public IPs, unknown ranges

	// Evidence
	Confidence     float64 `json:"confidence"`
	EvidenceDetail string  `json:"evidence_detail"`
	InferredAt     time.Time `json:"inferred_at"`
}

// ========================================================================
// Inference Rule Engine Types
// ========================================================================

// RulePriority defines the priority of an inference rule.
// Higher priority rules run first and their results take precedence.
type RulePriority int

const (
	PriorityHighest  RulePriority = 100
	PriorityHigh     RulePriority = 75
	PriorityNormal   RulePriority = 50
	PriorityLow      RulePriority = 25
	PriorityLowest   RulePriority = 0
)

// InferenceRule is a single rule in the inference engine.
// Rules produce inferred links, boundaries, or subnet groupings.
type InferenceRule struct {
	// Identity
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Priority    RulePriority `json:"priority"`
	Enabled     bool         `json:"enabled"`

	// What this rule produces
	RuleType string `json:"rule_type"` // l3-link, l2-link, subnet-group, nat-boundary, network-boundary, implicit-link

	// Execution condition — if empty, always runs
	Condition string `json:"condition,omitempty"` // Go expression evaluated at runtime

	// Conflict resolution
	ResolveConflict string `json:"resolve_conflict"` // prefer, discard, merge, warn

	// Metadata
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// RuleResult captures the output of a single inference rule execution.
type RuleResult struct {
	RuleName     string                 `json:"rule_name"`
	RuleType     string                 `json:"rule_type"`
	Priority     RulePriority           `json:"priority"`
	Success      bool                   `json:"success"`
	Outputs      []interface{}          `json:"outputs"` // InferredLink, SubnetGroup, etc.
	Warnings     []string               `json:"warnings,omitempty"`
	Conflicts    []Conflict             `json:"conflicts,omitempty"`
	ExecutedAt   time.Time              `json:"executed_at"`
	DurationMs   int64                  `json:"duration_ms"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// Conflict represents a disagreement between two inference results.
type Conflict struct {
	// What conflicted
	ConflictType string `json:"conflict_type"` // link, boundary, subnet, path

	// The competing claims
	ClaimA ConflictClaim `json:"claim_a"`
	ClaimB ConflictClaim `json:"claim_b"`

	// Resolution
	Resolution       string `json:"resolution"` // prefer-a, prefer-b, merge, discard-both, unresolved
	ResolutionReason string `json:"resolution_reason"`
}

// ConflictClaim represents one side of a conflict.
type ConflictClaim struct {
	RuleName string      `json:"rule_name"`
	Priority RulePriority `json:"priority"`
	Value    interface{} `json:"value"`
}

// ========================================================================
// Inference Engine State
// ========================================================================

// InferenceResult holds all outputs from a full inference run.
type InferenceResult struct {
	// Inferred links (L2 and L3)
	L2Links []InferredLink `json:"l2_links"`
	L3Links []InferredLink `json:"l3_links"`

	// Subnet groups
	SubnetGroups []SubnetGroup `json:"subnet_groups"`

	// Boundaries
	NATBoundaries    []NATBoundary    `json:"nat_boundaries"`
	NetworkBoundaries []NetworkBoundary `json:"network_boundaries"`

	// Paths (if requested)
	Paths []PathResult `json:"paths,omitempty"`

	// Implicit links
	ImplicitLinks []InferredLink `json:"implicit_links"`

	// Rule execution results
	RuleResults []RuleResult `json:"rule_results"`

	// Conflicts found and resolved
	Conflicts []Conflict `json:"conflicts"`

	// Summary statistics
	Stats InferenceStats `json:"stats"`

	// Metadata
	GeneratedAt time.Time `json:"generated_at"`
	EngineVersion string  `json:"engine_version"`
}

// InferenceStats provides summary statistics about an inference run.
type InferenceStats struct {
	TotalLinksInferred     int     `json:"total_links_inferred"`
	L2LinksInferred        int     `json:"l2_links_inferred"`
	L3LinksInferred        int     `json:"l3_links_inferred"`
	ImplicitLinksInferred  int     `json:"implicit_links_inferred"`
	SubnetGroupsFound      int     `json:"subnet_groups_found"`
	NATBoundariesInferred  int     `json:"nat_boundaries_inferred"`
	NetworkBoundariesFound int     `json:"network_boundaries_found"`
	PathsComputed          int     `json:"paths_computed"`
	ConflictsDetected      int     `json:"conflicts_detected"`
	ConflictsResolved      int     `json:"conflicts_resolved"`
	RulesExecuted          int     `json:"rules_executed"`
	RulesFailed            int     `json:"rules_failed"`
	AverageConfidence      float64 `json:"average_confidence"`
	TotalDurationMs        int64   `json:"total_duration_ms"`
}

// ========================================================================
// Helper types
// ========================================================================

// InferenceContext bundles all input data needed for inference.
type InferenceContext struct {
	// Devices indexed by hostname
	Devices map[string]*models.Device `json:"devices"`

	// Interfaces indexed by device:ifname
	Interfaces map[string]*models.Interface `json:"interfaces"`

	// L2 topology data
	L2Links    []models.L2Link    `json:"l2_links"`
	FDBEntries []models.FDBEntry  `json:"fdb_entries"`
	L2Switches []models.L2Switch  `json:"l2_switches"`

	// L3 routing data
	RoutingTables []models.L3RoutingTable `json:"routing_tables"`
	RoutingEntries []models.RoutingEntry  `json:"routing_entries"`
	OSPFProcesses  []models.OSPFProcess   `json:"ospf_processes"`
	BGPProcesses   []models.BGPProcess    `json:"bgp_processes"`

	// NAT data
	NATRules []models.NAT `json:"nat_rules"`
	VIPs     []models.VIP `json:"vips"`

	// Subnet/network data
	NetworkNodes []models.NetworkNode `json:"network_nodes"`

	// LLDP/CDP neighbor data — stored as maps of device -> neighbors
	LLDPNeighbors map[string][]LLDPNeighborData `json:"lldp_neighbors"`

	// Tunnels
	Tunnels []models.Tunnel `json:"tunnels"`

	// Metadata
	CollectionTime time.Time `json:"collection_time"`
}

// LLDPNeighborData is a vendor-agnostic LLDP/CDP neighbor entry used for inference.
type LLDPNeighborData struct {
	LocalInterface  string `json:"local_interface"`
	RemoteSystemName string `json:"remote_system_name"`
	RemoteInterface string `json:"remote_interface"`
	RemoteChassisID string `json:"remote_chassis_id"`
	RemoteMgmtIP    string `json:"remote_mgmt_ip,omitempty"`
	Protocol        string `json:"protocol"` // lldp, cdp
	Capabilities    []string `json:"capabilities,omitempty"`
}

// ========================================================================
// Default rules
// ========================================================================

// DefaultInferenceRules returns the built-in set of inference rules.
func DefaultInferenceRules() []InferenceRule {
	return []InferenceRule{
		{
			Name:            "l3-routing-next-hop-links",
			Description:     "Infer L3 links from routing table next-hop relationships",
			Priority:        PriorityHigh,
			Enabled:         true,
			RuleType:        "l3-link",
			ResolveConflict: "prefer",
		},
		{
			Name:            "l2-lldp-cdp-links",
			Description:     "Infer L2 links from LLDP/CDP neighbor data",
			Priority:        PriorityHighest,
			Enabled:         true,
			RuleType:        "l2-link",
			ResolveConflict: "prefer",
		},
		{
			Name:            "l2-mac-table-links",
			Description:     "Infer L2 links from MAC address table correlation",
			Priority:        PriorityNormal,
			Enabled:         true,
			RuleType:        "l2-link",
			ResolveConflict: "merge",
		},
		{
			Name:            "subnet-discovery",
			Description:     "Group devices by common IP prefixes and infer gateway devices",
			Priority:        PriorityNormal,
			Enabled:         true,
			RuleType:        "subnet-group",
			ResolveConflict: "merge",
		},
		{
			Name:            "implicit-link-detection",
			Description:     "Detect implicit links from matching subnets on interfaces",
			Priority:        PriorityLow,
			Enabled:         true,
			RuleType:        "implicit-link",
			ResolveConflict: "discard",
		},
		{
			Name:            "network-boundary-detection",
			Description:     "Detect where the managed network ends",
			Priority:        PriorityNormal,
			Enabled:         true,
			RuleType:        "network-boundary",
			ResolveConflict: "warn",
		},
		{
			Name:            "nat-boundary-inference",
			Description:     "Infer NAT boundaries from private IPs behind public IPs",
			Priority:        PriorityNormal,
			Enabled:         true,
			RuleType:        "nat-boundary",
			ResolveConflict: "warn",
		},
	}
}

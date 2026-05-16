package models

import "time"

// ========================================================================
// Phase 2: L3 Routing Foundation Models
// ========================================================================
//
// This file defines comprehensive L3 routing models for static routes,
// OSPF, and BGP routing protocols. These models support multi-vendor
// parsing and enable L3 reachability analysis for the network topology
// platform.
//
// Key Features:
// - Static route modeling with administrative distance
// - OSPF topology extraction (neighbors, LSA, areas)
// - BGP routing table analysis (peers, prefixes, AS paths)
// - Multi-protocol route selection (best path determination)
// - Evidence-based confidence scoring
// - VRF/VirtualRouter support
//
// ========================================================================

// L3RoutingTable represents the complete routing table for a device/VRF
type L3RoutingTable struct {
	ID             string              `json:"id" bson:"_id,omitempty"`
	DeviceID       string              `json:"device_id" bson:"device_id"`
	Hostname       string              `json:"hostname" bson:"hostname"`
	VirtualContext string              `json:"virtual_context" bson:"virtual_context"` // VRF/VDOM name
	VirtualRouter  string              `json:"virtual_router" bson:"virtual_router"`   // Virtual router name
	Routes         []L3Route           `json:"routes" bson:"routes"`
	DefaultRoute   *L3Route            `json:"default_route,omitempty" bson:"default_route,omitempty"`
	ConfigHash     string              `json:"config_hash" bson:"config_hash"`       // SHA-256 hash of routing config
	InsertedTime   time.Time           `json:"inserted_time" bson:"inserted_time"`
	LastModified   time.Time           `json:"last_modified" bson:"last_modified"`
	Active         bool                `json:"active" bson:"active"`
}

// L3Route represents a single routing table entry
type L3Route struct {
	Prefix            string    `json:"prefix" bson:"prefix"`                                       // Destination network (CIDR)
	PrefixLength      int       `json:"prefix_length" bson:"prefix_length"`                         // Subnet mask length
	NextHop           string    `json:"next_hop" bson:"next_hop"`                                   // Next hop IP
	Interface         string    `json:"interface" bson:"interface"`                                 // Egress interface
	Protocol          string    `json:"protocol" bson:"protocol"`                                   // static, connected, ospf, bgp, rip, eigrp
	Metric            int       `json:"metric" bson:"metric"`                                       // Route metric
	AdminDistance     int       `json:"admin_distance" bson:"admin_distance"`                       // Administrative distance
	Age               int       `json:"age" bson:"age"`                                             // Route age in seconds
	IsActive          bool      `json:"is_active" bson:"is_active"`                                 // Active in FIB
	IsBest            bool      `json:"is_best" bson:"is_best"`                                     // Best route for prefix
	Tag               int       `json:"tag,omitempty" bson:"tag,omitempty"`                         // Route tag
	RouteName         string    `json:"route_name,omitempty" bson:"route_name,omitempty"`           // Named static route
	Description       string    `json:"description,omitempty" bson:"description,omitempty"`         // Route description
	SourceBlock       string    `json:"source_block" bson:"source_block"`                           // Config snippet
	SourceSnippetHash string    `json:"source_snippet_hash" bson:"source_snippet_hash"`             // SHA-256 hash
	ConfidenceScore   int       `json:"confidence_score" bson:"confidence_score"`                   // 0-100 confidence
	EvidenceSources   []string  `json:"evidence_sources" bson:"evidence_sources"`                   // Evidence types
	LastSeen          time.Time `json:"last_seen" bson:"last_seen"`

	// Protocol-specific metadata
	OSPFMetadata *OSPFRouteMetadata `json:"ospf_metadata,omitempty" bson:"ospf_metadata,omitempty"`
	BGPMetadata  *BGPRouteMetadata  `json:"bgp_metadata,omitempty" bson:"bgp_metadata,omitempty"`
}

// OSPFRouteMetadata stores OSPF-specific route attributes
type OSPFRouteMetadata struct {
	RouteType       string `json:"route_type" bson:"route_type"`             // intra-area, inter-area, external-1, external-2
	AreaID          string `json:"area_id" bson:"area_id"`                   // OSPF area (e.g., "0.0.0.0")
	Cost            int    `json:"cost" bson:"cost"`                         // OSPF cost
	ForwardingAddr  string `json:"forwarding_addr,omitempty" bson:"forwarding_addr,omitempty"` // Forwarding address
	ExternalRouteTag int   `json:"external_route_tag,omitempty" bson:"external_route_tag,omitempty"`
	AdvertisingRouter string `json:"advertising_router,omitempty" bson:"advertising_router,omitempty"`
}

// BGPRouteMetadata stores BGP-specific route attributes
type BGPRouteMetadata struct {
	ASPath          []int    `json:"as_path" bson:"as_path"`                         // AS path
	NextHopAS       int      `json:"next_hop_as,omitempty" bson:"next_hop_as,omitempty"` // Next hop AS number
	LocalPref       int      `json:"local_pref" bson:"local_pref"`                   // Local preference
	MED             int      `json:"med" bson:"med"`                                 // Multi-exit discriminator
	Origin          string   `json:"origin" bson:"origin"`                           // IGP, EGP, incomplete
	Communities     []string `json:"communities,omitempty" bson:"communities,omitempty"` // BGP communities
	Weight          int      `json:"weight,omitempty" bson:"weight,omitempty"`       // Cisco weight
	PathLength      int      `json:"path_length" bson:"path_length"`                 // AS path length
	OriginatorID    string   `json:"originator_id,omitempty" bson:"originator_id,omitempty"`
	ClusterList     []string `json:"cluster_list,omitempty" bson:"cluster_list,omitempty"`
	ReceivedFrom    string   `json:"received_from" bson:"received_from"`             // Peer IP
}

// ========================================================================
// OSPF Models
// ========================================================================

// OSPFProcess represents an OSPF routing process
type OSPFProcess struct {
	ID             string             `json:"id" bson:"_id,omitempty"`
	DeviceID       string             `json:"device_id" bson:"device_id"`
	ProcessID      int                `json:"process_id" bson:"process_id"`     // OSPF process ID
	RouterID       string             `json:"router_id" bson:"router_id"`       // Router ID (IP format)
	VirtualContext string             `json:"virtual_context" bson:"virtual_context"`
	VirtualRouter  string             `json:"virtual_router" bson:"virtual_router"`
	Areas          []OSPFArea         `json:"areas" bson:"areas"`
	Neighbors      []OSPFNeighbor     `json:"neighbors" bson:"neighbors"`
	Networks       []OSPFNetwork      `json:"networks" bson:"networks"`         // Networks advertised
	Interfaces     []OSPFInterface    `json:"interfaces" bson:"interfaces"`
	PassiveInterfaces []string        `json:"passive_interfaces,omitempty" bson:"passive_interfaces,omitempty"`
	RedistributedProtocols []string   `json:"redistributed_protocols,omitempty" bson:"redistributed_protocols,omitempty"`
	DefaultOriginate bool             `json:"default_originate" bson:"default_originate"`
	ReferenceBandwidth int            `json:"reference_bandwidth,omitempty" bson:"reference_bandwidth,omitempty"` // Mbps
	ConfigHash     string             `json:"config_hash" bson:"config_hash"`
	InsertedTime   time.Time          `json:"inserted_time" bson:"inserted_time"`
	LastModified   time.Time          `json:"last_modified" bson:"last_modified"`
	Active         bool               `json:"active" bson:"active"`
}

// OSPFArea represents an OSPF area
type OSPFArea struct {
	AreaID          string   `json:"area_id" bson:"area_id"`                   // e.g., "0.0.0.0" or "10"
	AreaType        string   `json:"area_type" bson:"area_type"`               // normal, stub, nssa, totally-stub
	AuthType        string   `json:"auth_type,omitempty" bson:"auth_type,omitempty"` // none, simple, md5
	NetworkCount    int      `json:"network_count" bson:"network_count"`       // Number of networks
	RouterCount     int      `json:"router_count" bson:"router_count"`         // Number of routers
	LSACount        int      `json:"lsa_count" bson:"lsa_count"`               // Number of LSAs
	StubNoSummary   bool     `json:"stub_no_summary" bson:"stub_no_summary"`   // Totally stubby area
	NSSANoRedist    bool     `json:"nssa_no_redist" bson:"nssa_no_redist"`     // NSSA no redistribution
	DefaultCost     int      `json:"default_cost,omitempty" bson:"default_cost,omitempty"`
}

// OSPFNeighbor represents an OSPF adjacency
type OSPFNeighbor struct {
	NeighborID      string    `json:"neighbor_id" bson:"neighbor_id"`         // Neighbor router ID
	IPAddress       string    `json:"ip_address" bson:"ip_address"`           // Neighbor IP
	Interface       string    `json:"interface" bson:"interface"`             // Local interface
	State           string    `json:"state" bson:"state"`                     // FULL, 2WAY, EXSTART, etc.
	Priority        int       `json:"priority" bson:"priority"`               // DR election priority
	DeadTime        int       `json:"dead_time" bson:"dead_time"`             // Seconds until dead
	AreaID          string    `json:"area_id" bson:"area_id"`                 // OSPF area
	Role            string    `json:"role,omitempty" bson:"role,omitempty"`   // DR, BDR, DROTHER
	LastChange      time.Time `json:"last_change" bson:"last_change"`
	LSARetransmit   int       `json:"lsa_retransmit" bson:"lsa_retransmit"`   // Retransmit queue length
	DBSummary       int       `json:"db_summary" bson:"db_summary"`           // Database summary queue length
}

// OSPFNetwork represents a network advertised into OSPF
type OSPFNetwork struct {
	Network   string `json:"network" bson:"network"`     // Network CIDR
	AreaID    string `json:"area_id" bson:"area_id"`     // Area this network belongs to
	Wildcard  string `json:"wildcard,omitempty" bson:"wildcard,omitempty"` // Wildcard mask (Cisco)
}

// OSPFInterface represents an OSPF-enabled interface
type OSPFInterface struct {
	InterfaceName   string `json:"interface_name" bson:"interface_name"`
	IPAddress       string `json:"ip_address" bson:"ip_address"`
	AreaID          string `json:"area_id" bson:"area_id"`
	NetworkType     string `json:"network_type" bson:"network_type"`     // broadcast, p2p, nbma, p2mp
	Cost            int    `json:"cost" bson:"cost"`
	State           string `json:"state" bson:"state"`                   // DR, BDR, DROTHER, P2P
	Priority        int    `json:"priority" bson:"priority"`
	HelloInterval   int    `json:"hello_interval" bson:"hello_interval"` // Seconds
	DeadInterval    int    `json:"dead_interval" bson:"dead_interval"`   // Seconds
	RetransmitTime  int    `json:"retransmit_time" bson:"retransmit_time"`
	NeighborCount   int    `json:"neighbor_count" bson:"neighbor_count"`
	Passive         bool   `json:"passive" bson:"passive"`
	AuthType        string `json:"auth_type,omitempty" bson:"auth_type,omitempty"`
}

// OSPFLSA represents an OSPF Link State Advertisement
type OSPFLSA struct {
	ID             string    `json:"id" bson:"_id,omitempty"`
	DeviceID       string    `json:"device_id" bson:"device_id"`
	ProcessID      int       `json:"process_id" bson:"process_id"`
	LSAType        string    `json:"lsa_type" bson:"lsa_type"`           // router, network, summary, asbr-summary, external
	LSAID          string    `json:"lsa_id" bson:"lsa_id"`               // LSA identifier
	AdvertisingRouter string `json:"advertising_router" bson:"advertising_router"`
	AreaID         string    `json:"area_id" bson:"area_id"`
	SequenceNumber string    `json:"sequence_number" bson:"sequence_number"`
	Age            int       `json:"age" bson:"age"`                     // Seconds
	Checksum       string    `json:"checksum" bson:"checksum"`
	Length         int       `json:"length" bson:"length"`               // Bytes
	Options        string    `json:"options,omitempty" bson:"options,omitempty"`
	Links          []OSPFLink `json:"links,omitempty" bson:"links,omitempty"` // For router LSAs
	NetworkMask    string    `json:"network_mask,omitempty" bson:"network_mask,omitempty"`
	Metric         int       `json:"metric,omitempty" bson:"metric,omitempty"`
	ForwardingAddr string    `json:"forwarding_addr,omitempty" bson:"forwarding_addr,omitempty"`
	ExternalTag    int       `json:"external_tag,omitempty" bson:"external_tag,omitempty"`
	LastModified   time.Time `json:"last_modified" bson:"last_modified"`
}

// OSPFLink represents a link in an OSPF router LSA
type OSPFLink struct {
	LinkID       string `json:"link_id" bson:"link_id"`
	LinkData     string `json:"link_data" bson:"link_data"`
	LinkType     string `json:"link_type" bson:"link_type"`     // p2p, transit, stub, virtual
	Metric       int    `json:"metric" bson:"metric"`
	TOSCount     int    `json:"tos_count" bson:"tos_count"`
}

// ========================================================================
// BGP Models
// ========================================================================

// BGPProcess represents a BGP routing process
type BGPProcess struct {
	ID              string            `json:"id" bson:"_id,omitempty"`
	DeviceID        string            `json:"device_id" bson:"device_id"`
	LocalAS         int               `json:"local_as" bson:"local_as"`
	RouterID        string            `json:"router_id" bson:"router_id"`
	VirtualContext  string            `json:"virtual_context" bson:"virtual_context"`
	VirtualRouter   string            `json:"virtual_router" bson:"virtual_router"`
	Neighbors       []BGPNeighbor     `json:"neighbors" bson:"neighbors"`
	Networks        []BGPNetwork      `json:"networks" bson:"networks"`     // Networks originated
	AggregateAddrs  []BGPAggregate    `json:"aggregate_addrs,omitempty" bson:"aggregate_addrs,omitempty"`
	RedistributedProtocols []string   `json:"redistributed_protocols,omitempty" bson:"redistributed_protocols,omitempty"`
	DefaultOriginate bool             `json:"default_originate" bson:"default_originate"`
	Confederation   *BGPConfederation `json:"confederation,omitempty" bson:"confederation,omitempty"`
	RouteReflector  bool              `json:"route_reflector" bson:"route_reflector"`
	ClusterID       string            `json:"cluster_id,omitempty" bson:"cluster_id,omitempty"`
	ConfigHash      string            `json:"config_hash" bson:"config_hash"`
	InsertedTime    time.Time         `json:"inserted_time" bson:"inserted_time"`
	LastModified    time.Time         `json:"last_modified" bson:"last_modified"`
	Active          bool              `json:"active" bson:"active"`
}

// BGPNeighbor represents a BGP peer relationship
type BGPNeighbor struct {
	IPAddress           string    `json:"ip_address" bson:"ip_address"`
	RemoteAS            int       `json:"remote_as" bson:"remote_as"`
	Description         string    `json:"description,omitempty" bson:"description,omitempty"`
	State               string    `json:"state" bson:"state"`                 // Established, Idle, Active, Connect
	UptimeSeconds       int       `json:"uptime_seconds" bson:"uptime_seconds"`
	PrefixesReceived    int       `json:"prefixes_received" bson:"prefixes_received"`
	PrefixesSent        int       `json:"prefixes_sent" bson:"prefixes_sent"`
	MsgSent             int       `json:"msg_sent" bson:"msg_sent"`
	MsgReceived         int       `json:"msg_received" bson:"msg_received"`
	UpdateSource        string    `json:"update_source,omitempty" bson:"update_source,omitempty"`
	NextHopSelf         bool      `json:"next_hop_self" bson:"next_hop_self"`
	RouteReflectorClient bool     `json:"route_reflector_client" bson:"route_reflector_client"`
	PeerGroup           string    `json:"peer_group,omitempty" bson:"peer_group,omitempty"`
	EBGP                bool      `json:"ebgp" bson:"ebgp"`                   // External BGP peer
	Multihop            int       `json:"multihop,omitempty" bson:"multihop,omitempty"` // EBGP multihop TTL
	LocalIPAddress      string    `json:"local_ip_address,omitempty" bson:"local_ip_address,omitempty"`
	LocalPort           int       `json:"local_port,omitempty" bson:"local_port,omitempty"`
	RemotePort          int       `json:"remote_port,omitempty" bson:"remote_port,omitempty"`
	PasswordConfigured  bool      `json:"password_configured" bson:"password_configured"`
	HoldTime            int       `json:"hold_time" bson:"hold_time"`         // Seconds
	KeepAlive           int       `json:"keep_alive" bson:"keep_alive"`       // Seconds
	LastRead            time.Time `json:"last_read" bson:"last_read"`
	LastWrite           time.Time `json:"last_write" bson:"last_write"`
	Capabilities        []string  `json:"capabilities,omitempty" bson:"capabilities,omitempty"`
}

// BGPNetwork represents a network originated into BGP
type BGPNetwork struct {
	Network     string `json:"network" bson:"network"`     // Network CIDR
	Mask        string `json:"mask,omitempty" bson:"mask,omitempty"`
	RouteMap    string `json:"route_map,omitempty" bson:"route_map,omitempty"`
	Backdoor    bool   `json:"backdoor" bson:"backdoor"`
}

// BGPAggregate represents a BGP aggregate address
type BGPAggregate struct {
	Network        string `json:"network" bson:"network"`
	Mask           string `json:"mask,omitempty" bson:"mask,omitempty"`
	SummaryOnly    bool   `json:"summary_only" bson:"summary_only"`
	AsSet          bool   `json:"as_set" bson:"as_set"`
	AttributeMap   string `json:"attribute_map,omitempty" bson:"attribute_map,omitempty"`
	SuppressMap    string `json:"suppress_map,omitempty" bson:"suppress_map,omitempty"`
}

// BGPConfederation represents BGP confederation configuration
type BGPConfederation struct {
	ConfederationID int   `json:"confederation_id" bson:"confederation_id"`
	Peers           []int `json:"peers" bson:"peers"` // Confederation peer AS numbers
}

// BGPRIBEntry represents an entry in the BGP Routing Information Base
type BGPRIBEntry struct {
	ID              string    `json:"id" bson:"_id,omitempty"`
	DeviceID        string    `json:"device_id" bson:"device_id"`
	Prefix          string    `json:"prefix" bson:"prefix"`
	PrefixLength    int       `json:"prefix_length" bson:"prefix_length"`
	NextHop         string    `json:"next_hop" bson:"next_hop"`
	Path            []int     `json:"path" bson:"path"`               // AS path
	Origin          string    `json:"origin" bson:"origin"`           // IGP, EGP, incomplete
	LocalPref       int       `json:"local_pref" bson:"local_pref"`
	MED             int       `json:"med" bson:"med"`
	Weight          int       `json:"weight,omitempty" bson:"weight,omitempty"` // Cisco weight
	Communities     []string  `json:"communities,omitempty" bson:"communities,omitempty"`
	ReceivedFrom    string    `json:"received_from" bson:"received_from"` // Peer IP
	IsBest          bool      `json:"is_best" bson:"is_best"`
	IsValid         bool      `json:"is_valid" bson:"is_valid"`
	Status          string    `json:"status" bson:"status"`           // valid, invalid, best
	LastModified    time.Time `json:"last_modified" bson:"last_modified"`
}

// ========================================================================
// L3 Reachability Decision Models
// ========================================================================

// L3ReachabilityQuery represents a query for L3 reachability
type L3ReachabilityQuery struct {
	SourceIP       string `json:"source_ip" bson:"source_ip"`
	DestIP         string `json:"dest_ip" bson:"dest_ip"`
	Protocol       string `json:"protocol,omitempty" bson:"protocol,omitempty"` // tcp, udp, icmp, any
	Port           int    `json:"port,omitempty" bson:"port,omitempty"`
	SourceDevice   string `json:"source_device,omitempty" bson:"source_device,omitempty"`
	VirtualContext string `json:"virtual_context,omitempty" bson:"virtual_context,omitempty"`
	VirtualRouter  string `json:"virtual_router,omitempty" bson:"virtual_router,omitempty"`
}

// L3ReachabilityResult represents the result of an L3 reachability query
type L3ReachabilityResult struct {
	QueryID         string              `json:"query_id" bson:"query_id"`
	Query           L3ReachabilityQuery `json:"query" bson:"query"`
	Outcome         string              `json:"outcome" bson:"outcome"`         // Allowed, Blocked
	ConfidenceScore int                 `json:"confidence_score" bson:"confidence_score"` // 0-100
	ReasonCode      string              `json:"reason_code" bson:"reason_code"`
	ReasonDetail    string              `json:"reason_detail" bson:"reason_detail"`
	RoutingPath     []L3Hop             `json:"routing_path,omitempty" bson:"routing_path,omitempty"`
	NextHopDevice   string              `json:"next_hop_device,omitempty" bson:"next_hop_device,omitempty"`
	EgressInterface string              `json:"egress_interface,omitempty" bson:"egress_interface,omitempty"`
	RequiresFirewall bool               `json:"requires_firewall" bson:"requires_firewall"` // Handoff to firewall policy
	Evidence        L3ReachabilityEvidence `json:"evidence" bson:"evidence"`
	DecisionTime    time.Duration       `json:"decision_time" bson:"decision_time"`
	Timestamp       time.Time           `json:"timestamp" bson:"timestamp"`
	TTLExpires      time.Time           `json:"ttl_expires" bson:"ttl_expires"`
}

// L3Hop represents a single hop in the routing path
type L3Hop struct {
	DeviceID        string `json:"device_id" bson:"device_id"`
	Hostname        string `json:"hostname" bson:"hostname"`
	IngressInterface string `json:"ingress_interface,omitempty" bson:"ingress_interface,omitempty"`
	EgressInterface string `json:"egress_interface" bson:"egress_interface"`
	NextHop         string `json:"next_hop" bson:"next_hop"`
	Protocol        string `json:"protocol" bson:"protocol"`     // Route protocol
	Metric          int    `json:"metric" bson:"metric"`
	AdminDistance   int    `json:"admin_distance" bson:"admin_distance"`
}

// L3ReachabilityEvidence stores evidence used for L3 decision
type L3ReachabilityEvidence struct {
	RouteFound         bool     `json:"route_found" bson:"route_found"`
	RouteProtocol      string   `json:"route_protocol,omitempty" bson:"route_protocol,omitempty"`
	RouteMetric        int      `json:"route_metric,omitempty" bson:"route_metric,omitempty"`
	RouteAdminDistance int      `json:"route_admin_distance,omitempty" bson:"route_admin_distance,omitempty"`
	NextHop            string   `json:"next_hop,omitempty" bson:"next_hop,omitempty"`
	EgressInterface    string   `json:"egress_interface,omitempty" bson:"egress_interface,omitempty"`
	RoutingLoopDetected bool    `json:"routing_loop_detected" bson:"routing_loop_detected"`
	BlackHole          bool     `json:"black_hole" bson:"black_hole"`
	NullRoute          bool     `json:"null_route" bson:"null_route"`
	EvidenceSources    []string `json:"evidence_sources" bson:"evidence_sources"`
	ConfidenceFactors  map[string]int `json:"confidence_factors" bson:"confidence_factors"`
}

// L3DecisionCache represents a cached L3 decision
type L3DecisionCache struct {
	ID             string               `json:"id" bson:"_id,omitempty"`
	QueryHash      string               `json:"query_hash" bson:"query_hash"` // Hash of query params
	Result         L3ReachabilityResult `json:"result" bson:"result"`
	CreatedAt      time.Time            `json:"created_at" bson:"created_at"`
	ExpiresAt      time.Time            `json:"expires_at" bson:"expires_at"`
}

// ========================================================================
// Route Selection Models
// ========================================================================

// RouteCandidate represents a candidate route for best path selection
type RouteCandidate struct {
	Route           L3Route `json:"route" bson:"route"`
	SelectionScore  int     `json:"selection_score" bson:"selection_score"`   // Higher is better
	IsPreferred     bool    `json:"is_preferred" bson:"is_preferred"`
	SelectionReason string  `json:"selection_reason" bson:"selection_reason"` // Why this route was chosen
}

// RouteSelectionCriteria defines criteria for route selection
type RouteSelectionCriteria struct {
	PreferLongestMatch bool              `json:"prefer_longest_match" bson:"prefer_longest_match"` // Default: true
	AdminDistanceMap   map[string]int    `json:"admin_distance_map" bson:"admin_distance_map"`     // Protocol -> AD
	PreferProtocol     string            `json:"prefer_protocol,omitempty" bson:"prefer_protocol,omitempty"`
	MaxMetric          int               `json:"max_metric,omitempty" bson:"max_metric,omitempty"`
	RequireActiveRoute bool              `json:"require_active_route" bson:"require_active_route"` // Default: true
}

// ========================================================================
// Helper Types
// ========================================================================

// L3NodeMetadata stores routing-specific metadata for L3 elements.
// Replaces the deprecated duplicate NodeMetadata definition.
type L3NodeMetadata struct {
	Vendor      string            `json:"vendor,omitempty" bson:"vendor,omitempty"`
	Model       string            `json:"model,omitempty" bson:"model,omitempty"`
	Version     string            `json:"version,omitempty" bson:"version,omitempty"`
	Site        string            `json:"site,omitempty" bson:"site,omitempty"`
	Region      string            `json:"region,omitempty" bson:"region,omitempty"`
	Tags        []string          `json:"tags,omitempty" bson:"tags,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty" bson:"annotations,omitempty"`
}

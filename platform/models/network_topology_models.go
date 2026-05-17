package models

// Service defines transport-layer protocol and port mappings
type Service struct {
	ID             string    `json:"id" bson:"_id,omitempty"`
	Name           string    `json:"name" bson:"name"`                                     // Service name
	VirtualContext string    `json:"virtual_context" bson:"virtual_context"`               // Context this service belongs to
	SourcePorts    []PortMap `json:"source_ports,omitempty" bson:"source_ports,omitempty"` // Optional source port(s)
	DestPorts      []PortMap `json:"dest_ports,omitempty" bson:"dest_ports,omitempty"`     // Optional destination port(s)
	ICMPType       *int      `json:"icmp_type,omitempty" bson:"icmp_type,omitempty"`       // ICMP type (if applicable)
	ICMPCode       *int      `json:"icmp_code,omitempty" bson:"icmp_code,omitempty"`       // ICMP code (if applicable)
	Group          []string  `json:"group,omitempty" bson:"group,omitempty"`               // Group(s) this service belongs to
	Description    string    `json:"description,omitempty" bson:"description,omitempty"`   // Optional service description
}

type PortMap struct {
	Protocol string   `json:"protocol" bson:"protocol"`
	Ports    []string `json:"port" bson:"port"`
}

// ServiceGroup defines a logical grouping of multiple service objects
type ServiceGroup struct {
	ID             string   `json:"id" bson:"_id,omitempty"`                // Unique identifier
	Name           string   `json:"name" bson:"name"`                       // Group name
	VirtualContext string   `json:"virtual_context" bson:"virtual_context"` // VRF/VDOM context the group belongs to
	Members        []string `json:"members" bson:"members"`                 // References to ServiceObject IDs or nested group IDs
	Description    string   `json:"description" bson:"description"`         // Notes or group purpose
}

// Application defines application-layer protocols or services
type Application struct {
	ID             string   `json:"id" bson:"_id,omitempty"`                                // Unique identifier
	Name           string   `json:"name" bson:"name"`                                       // Application name
	VirtualContext string   `json:"virtual_context" bson:"virtual_context"`                 // Context this application belongs to
	Protocol       string   `json:"protocol" bson:"protocol"`                               // Layer 4 protocol associated (if any)
	DefaultPorts   []string `json:"default_ports,omitempty" bson:"default_ports,omitempty"` // Common ports used
	Group          []string `json:"group,omitempty" bson:"group,omitempty"`                 // Group(s) this app belongs to
	Priority       *int     `json:"priority,omitempty" bson:"priority,omitempty"`           // Evaluation priority
	Category       string   `json:"category,omitempty" bson:"category,omitempty"`           // App category
	Subcategory    string   `json:"subcategory,omitempty" bson:"subcategory,omitempty"`     // App subcategory
	Risk           *int     `json:"risk,omitempty" bson:"risk,omitempty"`                   // Risk score from 1–5
	Description    string   `json:"description,omitempty" bson:"description,omitempty"`     // App description or purpose
}

// ApplicationGroup defines a logical grouping of multiple application objects
type ApplicationGroup struct {
	ID             string   `json:"id" bson:"_id,omitempty"`
	Name           string   `json:"name" bson:"name"`                       // Group name
	VirtualContext string   `json:"virtual_context" bson:"virtual_context"` // VRF/VDOM context the group belongs to
	Members        []string `json:"members" bson:"members"`                 // Embedded application objects
	Description    string   `json:"description" bson:"description"`         // Notes or group purpose
}

// RoutingEntry controls packet forwarding based on destination IP
type RoutingEntry struct {
	ID             string `json:"id" bson:"_id,omitempty"`                // Unique identifier
	VirtualContext string `json:"virtual_context" bson:"virtual_context"` // VRF/VDOM/context for the route
	VirtualRouter  string `json:"virtual_router" bson:"virtual_router"`   // Virtual router name
	Destination    string `json:"destination" bson:"destination"`         // CIDR destination network
	NextHop        string `json:"next_hop" bson:"next_hop"`               // IP of the next hop
	Interface      string `json:"interface" bson:"interface"`             // Egress interface
	Type           string `json:"type" bson:"type"`                       // static, connected, dynamic
	Protocol       string `json:"protocol" bson:"protocol"`               // ospf, bgp, etc.
	Metric         int    `json:"metric" bson:"metric"`                   // Route metric
	AdminDistance  int    `json:"admin_distance" bson:"admin_distance"`   // Administrative distance
	Name           string `json:"name" bson:"name"`                       // Name of the route
}

// NAT modifies source or destination during transit
type NAT struct {
	ID                    string `json:"id" bson:"_id,omitempty"`                                                  // Unique identifier
	Name                  string `json:"name,omitempty" bson:"name,omitempty"`                                     // Optional rule name
	VirtualContext        string `json:"virtual_context" bson:"virtual_context"`                                   // Context this NAT belongs to
	Type                  string `json:"type" bson:"type"`                                                         // source, destination, static, dynamic, auto
	OriginalSource        string `json:"original_source" bson:"original_source"`                                   // Original source address
	OriginalDestination   string `json:"original_destination,omitempty" bson:"original_destination,omitempty"`     // Original destination address
	TranslatedSource      string `json:"translated_source,omitempty" bson:"translated_source,omitempty"`           // Translated source address
	TranslatedDestination string `json:"translated_destination,omitempty" bson:"translated_destination,omitempty"` // Translated destination address
	OriginalService       string `json:"original_service,omitempty" bson:"original_service,omitempty"`             // Original service
	TranslatedService     string `json:"translated_service,omitempty" bson:"translated_service,omitempty"`         // Translated service
	InboundInterface      string `json:"inbound_interface,omitempty" bson:"inbound_interface,omitempty"`           // Inbound interface
	OutboundInterface     string `json:"outbound_interface,omitempty" bson:"outbound_interface,omitempty"`         // Outbound interface
	Bidirectional         bool   `json:"bidirectional" bson:"bidirectional"`                                       // Whether NAT is symmetric
}

// VIP defines a public-facing NAT object mapping to internal IP
type VIP struct {
	ID             string `json:"id" bson:"_id,omitempty"`                                // Unique identifier
	Name           string `json:"name" bson:"name"`                                       // VIP rule name
	VirtualContext string `json:"virtual_context" bson:"virtual_context"`                 // Context this VIP belongs to
	Interface      string `json:"interface" bson:"interface"`                             // Interface on which VIP is applied
	ExternalIP     string `json:"external_ip" bson:"external_ip"`                         // Public-facing IP
	MappedIP       string `json:"mapped_ip" bson:"mapped_ip"`                             // Internal destination IP
	Type           string `json:"type" bson:"type"`                                       // static, dynamic
	Protocol       string `json:"protocol" bson:"protocol"`                               // tcp, udp, icmp, etc.
	ExternalPort   string `json:"external_port,omitempty" bson:"external_port,omitempty"` // Optional public port
	MappedPort     string `json:"mapped_port,omitempty" bson:"mapped_port,omitempty"`     // Optional internal port
	Description    string `json:"description,omitempty" bson:"description,omitempty"`     // Optional description of the VIP rule
}

// LogConfig represents logging configuration for a firewall policy
type LogConfig struct {
	AtStart bool `json:"at_start" bson:"at_start"` // Log at session start
	AtEnd   bool `json:"at_end" bson:"at_end"`     // Log at session end
}

// FirewallPolicy defines access between segments based on zones, addresses, and services
type FirewallPolicy struct {
	ID                       string    `json:"id" bson:"_id,omitempty"`                                      // Rule ID
	Name                     string    `json:"name" bson:"name"`                                             // Human-readable rule name
	Firewall                 string    `json:"firewall_name" bson:"firewall_name"`                           // Firewall name
	VirtualContext           string    `json:"virtual_context" bson:"virtual_context"`                       // VRF/VDOM context the rule belongs to
	Enabled                  bool      `json:"enabled" bson:"enabled"`                                       // Whether the rule is active
	SourceZones              []string  `json:"source_zones" bson:"source_zones"`                             // Source zones
	DestinationZones         []string  `json:"destination_zones" bson:"destination_zones"`                   // Destination zones
	SourceAddresses          []string  `json:"source_addresses" bson:"source_addresses"`                     // Source addresses
	DestinationAddresses     []string  `json:"destination_addresses" bson:"destination_addresses"`           // Destination addresses
	SourceAddressGroups      []string  `json:"source_address_groups" bson:"source_address_groups"`           // Source address groups
	DestinationAddressGroups []string  `json:"destination_address_groups" bson:"destination_address_groups"` // Destination address groups
	ServiceGroups            []string  `json:"service_groups" bson:"service_groups"`                         // Service groups
	Services                 []string  `json:"services" bson:"services"`                                     // Services
	Applications             []string  `json:"applications" bson:"applications"`                             // Applications
	Action                   string    `json:"action" bson:"action"`                                         // allow, deny, reject
	Log                      LogConfig `json:"log" bson:"log"`                                               // Logging configuration
	NAT                      *NAT      `json:"nat,omitempty" bson:"nat,omitempty"`                           // Optional NAT configuration
	Description              string    `json:"description" bson:"description"`                               // Notes on rule purpose
	RuleOrder                int       `json:"rule_order" bson:"rule_order"`                                 // Rule priority/order
	Direction                string    `json:"direction" bson:"direction"`                                   // inbound, outbound, inter-zone
	ImplicitDeny             *bool     `json:"implicit_deny,omitempty" bson:"implicit_deny,omitempty"`       // Whether default deny applies if no match
	SourcePorts              []string  `json:"source_ports" bson:"source_ports"`                             // Source ports
	DestinationPorts         []string  `json:"destination_ports" bson:"destination_ports"`                   // Destination ports
	AnySource                bool      `json:"any_source" bson:"any_source"`                                 // Whether the rule is an any source rule
	AnyDestination           bool      `json:"any_destination" bson:"any_destination"`                       // Whether the rule is an any destination rule
}

// Tunnel models logical tunnels such as IPsec or GRE
type Tunnel struct {
	ID                string   `json:"id" bson:"_id,omitempty"`                                          // Unique identifier
	Name              string   `json:"name" bson:"name"`                                                 // Tunnel name
	VirtualContext    string   `json:"virtual_context" bson:"virtual_context"`                           // VRF/VDOM context
	Type              string   `json:"type" bson:"type"`                                                 // ipsec, gre, ssl, remote-access
	LocalEndpoint     string   `json:"local_endpoint" bson:"local_endpoint"`                             // Interface or IP of the local end
	RemoteEndpoint    string   `json:"remote_endpoint" bson:"remote_endpoint"`                           // IP of the remote end
	EncryptionProfile string   `json:"encryption_profile,omitempty" bson:"encryption_profile,omitempty"` // Optional VPN profile name
	LocalNetworks     []string `json:"local_networks" bson:"local_networks"`                             // List of protected local network references
	RemoteNetworks    []string `json:"remote_networks" bson:"remote_networks"`                           // List of protected remote network references
	RouteInjection    bool     `json:"route_injection" bson:"route_injection"`                           // Whether the tunnel injects routes
	FailoverGroup     string   `json:"failover_group,omitempty" bson:"failover_group,omitempty"`         // Optional tunnel failover group
	Weight            *int     `json:"weight,omitempty" bson:"weight,omitempty"`                         // Tunnel preference for routing decisions
}

// Device represents network devices/elements and their placement
type Device struct {
	ID              string   `json:"id" bson:"_id,omitempty"`                  // Unique identifier
	Hostname        string   `json:"hostname" bson:"hostname"`                 // Device name
	Vendor          string   `json:"vendor" bson:"vendor"`                     // e.g., cisco, fortinet, paloalto
	Model           string   `json:"model" bson:"model"`                       // Hardware or virtual model
	Version         string   `json:"version" bson:"version"`                   // Firmware or software version
	VirtualContexts []string `json:"virtual_contexts" bson:"virtual_contexts"` // List of VDOMs/contexts assigned to this device
	ManagementIP    string   `json:"management_ip" bson:"management_ip"`       // Management IP address
	Site            string   `json:"site" bson:"site"`                         // Location or datacenter name
	Role            string   `json:"role" bson:"role"`                         // edge, core, perimeter, etc.
	Interfaces      []string `json:"interfaces" bson:"interfaces"`             // Interfaces assigned to the device
	PolicyGroups    []string `json:"policy_groups" bson:"policy_groups"`       // Reference to policy group(s), e.g., Panorama device groups

	// Layer-2 device fingerprint
	OSName     string `json:"os_name,omitempty" bson:"os_name,omitempty"`
	OSVersion  string `json:"os_version,omitempty" bson:"os_version,omitempty"`
	ConfigHash string `json:"config_hash,omitempty" bson:"config_hash,omitempty"`
}

// L2Switch captures the vendor-agnostic Layer-2 snapshot for a device.
type L2Switch struct {
	ID               string          `json:"id" bson:"_id,omitempty"`
	DeviceID         string          `json:"device_id" bson:"device_id"`
	Vendor           string          `json:"vendor" bson:"vendor"`
	OSName           string          `json:"os_name" bson:"os_name"`
	OSVersion        string          `json:"os_version" bson:"os_version"`
	ConfigHash       string          `json:"config_hash" bson:"config_hash"`
	Site             string          `json:"site,omitempty" bson:"site,omitempty"`
	BusinessUnit     string          `json:"business_unit,omitempty" bson:"business_unit,omitempty"`
	Role             string          `json:"role,omitempty" bson:"role,omitempty"`
	VLANs            []L2VLAN        `json:"vlans" bson:"vlans"`
	Interfaces       []L2SwitchPort  `json:"interfaces" bson:"interfaces"`
	PortChannels     []L2PortChannel `json:"port_channels" bson:"port_channels"`
	GlobalL2Controls L2GlobalControls `json:"global_l2_controls" bson:"global_l2_controls"`
	SVIs             []L3HandoffMarker `json:"svis,omitempty" bson:"svis,omitempty"`
	Metadata         NodeMetadata    `json:"metadata,omitempty" bson:"metadata,omitempty"`
	InsertedTime     string          `json:"inserted_time" bson:"inserted_time"`
	LastModified     string          `json:"last_modified" bson:"last_modified"`
	Active           bool            `json:"active" bson:"active"`
}

// L2VLAN defines VLAN metadata plus evidence references.
type L2VLAN struct {
	VLANID            int     `json:"vlan_id" bson:"vlan_id"`
	VLANName          string  `json:"vlan_name" bson:"vlan_name"`
	State             string  `json:"state" bson:"state"`
	VACLName          *string `json:"vacl_name,omitempty" bson:"vacl_name,omitempty"`
	SourceBlock       string  `json:"source_block" bson:"source_block"`
	SourceSnippetHash string  `json:"source_snippet_hash" bson:"source_snippet_hash"`
}

// L2SwitchPort stores switchport posture and security controls.
type L2SwitchPort struct {
	InterfaceName        string  `json:"interface_name" bson:"interface_name"`
	AdminState           string  `json:"admin_state" bson:"admin_state"`
	Mode                 string  `json:"mode" bson:"mode"`
	AccessVLAN           *int    `json:"access_vlan,omitempty" bson:"access_vlan,omitempty"`
	TrunkAllowedVLANs    []int   `json:"trunk_allowed_vlans,omitempty" bson:"trunk_allowed_vlans,omitempty"`
	TrunkNativeVLAN      *int    `json:"trunk_native_vlan,omitempty" bson:"trunk_native_vlan,omitempty"`
	PortChannelID        *int    `json:"port_channel_id,omitempty" bson:"port_channel_id,omitempty"`
	DHCPSnoopingTrust    bool    `json:"dhcp_snooping_trust" bson:"dhcp_snooping_trust"`
	ARPInspectionTrust   bool    `json:"arp_inspection_trust" bson:"arp_inspection_trust"`
	IPSourceGuard        bool    `json:"ip_source_guard" bson:"ip_source_guard"`
	Dot1xRequired        bool    `json:"dot1x_required" bson:"dot1x_required"`
	MABEnabled           bool    `json:"mab_enabled" bson:"mab_enabled"`
	UnauthVLAN           *int    `json:"unauth_vlan,omitempty" bson:"unauth_vlan,omitempty"`
	PVLANRole            *string `json:"pvlan_role,omitempty" bson:"pvlan_role,omitempty"`
	PVLANPrimaryVLAN     *int    `json:"pvlan_primary_vlan,omitempty" bson:"pvlan_primary_vlan,omitempty"`
	PVLANSecondaryVLAN   *int    `json:"pvlan_secondary_vlan,omitempty" bson:"pvlan_secondary_vlan,omitempty"`
	Description          string  `json:"description,omitempty" bson:"description,omitempty"`
	SourceBlock          string  `json:"source_block" bson:"source_block"`
	SourceSnippetHash    string  `json:"source_snippet_hash" bson:"source_snippet_hash"`
}

// L2PortChannel represents link-aggregation metadata for Layer-2 analysis.
type L2PortChannel struct {
	PortChannelID        int      `json:"port_channel_id" bson:"port_channel_id"`
	Name                 string   `json:"name" bson:"name"`
	MemberInterfaces     []string `json:"member_interfaces" bson:"member_interfaces"`
	Mode                 string   `json:"mode" bson:"mode"`
	AccessVLAN           *int     `json:"access_vlan,omitempty" bson:"access_vlan,omitempty"`
	TrunkAllowedVLANs    []int    `json:"trunk_allowed_vlans,omitempty" bson:"trunk_allowed_vlans,omitempty"`
	TrunkNativeVLAN      *int     `json:"trunk_native_vlan,omitempty" bson:"trunk_native_vlan,omitempty"`
	SourceBlock          string   `json:"source_block" bson:"source_block"`
	SourceSnippetHash    string   `json:"source_snippet_hash" bson:"source_snippet_hash"`
}

// L2GlobalControls holds device-wide security toggles.
type L2GlobalControls struct {
	SnoopingEnabled        bool   `json:"snooping_enabled" bson:"snooping_enabled"`
	SnoopingVLANs          []int  `json:"snooping_vlans,omitempty" bson:"snooping_vlans,omitempty"`
	DAIEnabledVLANs        []int  `json:"dai_enabled_vlans,omitempty" bson:"dai_enabled_vlans,omitempty"`
	IPSGGlobal             bool   `json:"ipsg_global" bson:"ipsg_global"`
	Dot1xSystemAuthControl bool   `json:"dot1x_system_auth_control" bson:"dot1x_system_auth_control"`
	MABGlobal              bool   `json:"mab_global" bson:"mab_global"`
	SourceBlock            string `json:"source_block" bson:"source_block"`
	SourceSnippetHash      string `json:"source_snippet_hash" bson:"source_snippet_hash"`
}

// L3HandoffMarker flags VLANs that terminate into Layer-3 interfaces.
type L3HandoffMarker struct {
	SVIID             int    `json:"svi_id" bson:"svi_id"`
	InterfaceName     string `json:"interface_name" bson:"interface_name"`
	IPAddress         string `json:"ip_address" bson:"ip_address"`
	Subnet            string `json:"subnet" bson:"subnet"`
	Description       string `json:"description,omitempty" bson:"description,omitempty"`
	SourceBlock       string `json:"source_block" bson:"source_block"`
	SourceSnippetHash string `json:"source_snippet_hash" bson:"source_snippet_hash"`
}

// FDBEntry stores MAC placement with confidence/freshness metadata.
type FDBEntry struct {
	ID             string  `json:"id" bson:"_id,omitempty"`
	DeviceID       string  `json:"device_id" bson:"device_id"`
	MAC            string  `json:"mac" bson:"mac"`
	LastSeenPort   string  `json:"last_seen_port" bson:"last_seen_port"`
	VLANID         int     `json:"vlan_id" bson:"vlan_id"`
	LastSeenTS     string  `json:"last_seen_ts" bson:"last_seen_ts"`
	EvidenceSource string  `json:"evidence_source" bson:"evidence_source"`
	Confidence     float64 `json:"confidence" bson:"confidence"`
	Freshness      string  `json:"freshness" bson:"freshness"`
}

// L2Decision represents deterministic reachability outcomes (phase 2).
type L2Decision struct {
	ID          string   `json:"id" bson:"_id,omitempty"`
	SourceMAC   string   `json:"source_mac" bson:"source_mac"`
	DestMAC     string   `json:"dest_mac" bson:"dest_mac"`
	VLANID      int      `json:"vlan_id" bson:"vlan_id"`
	Outcome     string   `json:"outcome" bson:"outcome"`
	ReasonCodes []string `json:"reason_codes,omitempty" bson:"reason_codes,omitempty"`
	HandoffToL3 bool     `json:"handoff_to_l3" bson:"handoff_to_l3"`
	EvaluatedAt string   `json:"evaluated_at" bson:"evaluated_at"`
}

// ManagementPolicy controls access to device management interfaces (e.g., SSH, SNMP)
type ManagementPolicy struct {
	ID              string   `json:"id" bson:"_id,omitempty"`                  // Unique identifier
	Name            string   `json:"name" bson:"name"`                         // Human-readable name
	Device          string   `json:"device" bson:"device"`                     // Hostname or ID of the device
	VirtualContext  string   `json:"virtual_context" bson:"virtual_context"`   // VRF/VDOM context
	SourceAddresses []string `json:"source_addresses" bson:"source_addresses"` // Who is allowed to connect
	Services        []string `json:"services" bson:"services"`                 // Services allowed for access (e.g., ssh, https)
	DestInterfaces  []string `json:"dest_interfaces" bson:"dest_interfaces"`   // Interfaces where access is allowed
	Action          string   `json:"action" bson:"action"`                     // allow, deny
	Log             bool     `json:"log" bson:"log"`                           // Whether to log connection attempts
	Description     string   `json:"description" bson:"description"`           // Notes on policy intent
}

// ParsedNetworkTopology holds all mapped objects from a PaloAltoProcessorV2
// Each slice corresponds to a type in network-topology.models.go
// All IDs are left empty, and VirtualContext is set to vsys name or "shared" as appropriate
// ParentDevice is set to the PaloAltoProcessorV2 hostname
// This function maps all possible elements: VirtualContexts, Interfaces, Zones, Addresses, AddressGroups, Services, ServiceGroups, Applications, RoutingEntries, NATs, FirewallPolicies, Tunnels, Devices
// If a mapping is not possible, a comment is added
type ParsedNetworkTopology struct {
	ID                string             `json:"id" bson:"_id,omitempty"`
	FirewallName      string             `json:"firewall_name" bson:"firewall_name"`
	VirtualContexts   []VirtualContext   `json:"virtual_contexts" bson:"virtual_contexts"`
	Interfaces        []Interface        `json:"interfaces" bson:"interfaces"`
	Zones             []Zone             `json:"zones" bson:"zones"`
	Addresses         []Address          `json:"addresses" bson:"addresses"`
	AddressGroups     []AddressGroup     `json:"address_groups" bson:"address_groups"`
	Services          []Service          `json:"services" bson:"services"`
	ServiceGroups     []ServiceGroup     `json:"service_groups" bson:"service_groups"`
	Applications      []Application      `json:"applications" bson:"applications"`
	ApplicationGroups []ApplicationGroup `json:"application_groups" bson:"application_groups"`
	RoutingEntries    []RoutingEntry     `json:"routing_entries" bson:"routing_entries"`
	NATs              []NAT              `json:"nats" bson:"nats"`
	FirewallPolicies  []FirewallPolicy   `json:"firewall_policies" bson:"firewall_policies"`
	Tunnels           []Tunnel           `json:"tunnels" bson:"tunnels"`
	Devices           []Device           `json:"devices" bson:"devices"`
	OwnedAddresses    []Address          `json:"owned_addresses" bson:"owned_addresses"` // Addresses owned/managed by this firewall topology
	Metadata          NodeMetadata       `json:"metadata" bson:"metadata"`
	InsertedTime      string             `json:"inserted_time" bson:"inserted_time"`
	LastModified      string             `json:"last_modified" bson:"last_modified"`
	Active            bool               `json:"active" bson:"active"`
}

// ParsedNetworkTopologyMetadata contains the metadata and core information for a parsed network topology
// This is the main record that references all the split collections
type ParsedNetworkTopologyMetadata struct {
	ID           string       `json:"id" bson:"_id,omitempty"`
	FirewallName string       `json:"firewall_name" bson:"firewall_name"`
	Metadata     NodeMetadata `json:"metadata" bson:"metadata"`
	InsertedTime string       `json:"inserted_time" bson:"inserted_time"`
	LastModified string       `json:"last_modified" bson:"last_modified"`
	Active       bool         `json:"active" bson:"active"`
}

// ParsedNetworkTopologyVirtualContexts contains virtual contexts for a specific firewall
type ParsedNetworkTopologyVirtualContexts struct {
	ID                 string         `json:"id" bson:"_id,omitempty"`
	TopologyID         string         `json:"topology_id" bson:"topology_id"`
	FirewallName       string         `json:"firewall_name" bson:"firewall_name"`
	VirtualContext     string         `json:"virtual_context" bson:"virtual_context"` // The specific virtual context this belongs to
	VirtualContextData VirtualContext `json:"virtual_context_data" bson:"virtual_context_data"`
}

// ParsedNetworkTopologyInterfaces contains interfaces for a specific firewall
type ParsedNetworkTopologyInterfaces struct {
	ID             string    `json:"id" bson:"_id,omitempty"`
	TopologyID     string    `json:"topology_id" bson:"topology_id"`
	FirewallName   string    `json:"firewall_name" bson:"firewall_name"`
	VirtualContext string    `json:"virtual_context" bson:"virtual_context"` // The specific virtual context this belongs to
	Interface      Interface `json:"interface_data" bson:"interface_data"`
}

// ParsedNetworkTopologyZones contains zones for a specific firewall
type ParsedNetworkTopologyZones struct {
	ID             string `json:"id" bson:"_id,omitempty"`
	TopologyID     string `json:"topology_id" bson:"topology_id"`
	FirewallName   string `json:"firewall_name" bson:"firewall_name"`
	VirtualContext string `json:"virtual_context" bson:"virtual_context"` // The specific virtual context this belongs to
	Zone           Zone   `json:"zone_data" bson:"zone_data"`
}

// ParsedNetworkTopologyAddresses contains addresses for a specific firewall
type ParsedNetworkTopologyAddresses struct {
	ID             string  `json:"id" bson:"_id,omitempty"`
	TopologyID     string  `json:"topology_id" bson:"topology_id"`
	FirewallName   string  `json:"firewall_name" bson:"firewall_name"`
	VirtualContext string  `json:"virtual_context" bson:"virtual_context"` // The specific virtual context this belongs to
	Address        Address `json:"address_data" bson:"address_data"`
}

// ParsedNetworkTopologyAddressGroups contains address groups for a specific firewall
type ParsedNetworkTopologyAddressGroups struct {
	ID             string       `json:"id" bson:"_id,omitempty"`
	TopologyID     string       `json:"topology_id" bson:"topology_id"`
	FirewallName   string       `json:"firewall_name" bson:"firewall_name"`
	VirtualContext string       `json:"virtual_context" bson:"virtual_context"` // The specific virtual context this belongs to
	AddressGroup   AddressGroup `json:"address_group_data" bson:"address_group_data"`
}

// ParsedNetworkTopologyServices contains services for a specific firewall
type ParsedNetworkTopologyServices struct {
	ID             string  `json:"id" bson:"_id,omitempty"`
	TopologyID     string  `json:"topology_id" bson:"topology_id"`
	FirewallName   string  `json:"firewall_name" bson:"firewall_name"`
	VirtualContext string  `json:"virtual_context" bson:"virtual_context"` // The specific virtual context this belongs to
	Service        Service `json:"service_data" bson:"service_data"`
}

// ParsedNetworkTopologyServiceGroups contains service groups for a specific firewall
type ParsedNetworkTopologyServiceGroups struct {
	ID             string       `json:"id" bson:"_id,omitempty"`
	TopologyID     string       `json:"topology_id" bson:"topology_id"`
	FirewallName   string       `json:"firewall_name" bson:"firewall_name"`
	VirtualContext string       `json:"virtual_context" bson:"virtual_context"` // The specific virtual context this belongs to
	ServiceGroup   ServiceGroup `json:"service_group_data" bson:"service_group_data"`
}

// ParsedNetworkTopologyApplications contains applications for a specific firewall
type ParsedNetworkTopologyApplications struct {
	ID             string      `json:"id" bson:"_id,omitempty"`
	TopologyID     string      `json:"topology_id" bson:"topology_id"`
	FirewallName   string      `json:"firewall_name" bson:"firewall_name"`
	VirtualContext string      `json:"virtual_context" bson:"virtual_context"` // The specific virtual context this belongs to
	Application    Application `json:"application_data" bson:"application_data"`
}

// ParsedNetworkTopologyApplicationGroups contains application groups for a specific firewall
type ParsedNetworkTopologyApplicationGroups struct {
	ID               string           `json:"id" bson:"_id,omitempty"`
	TopologyID       string           `json:"topology_id" bson:"topology_id"`
	FirewallName     string           `json:"firewall_name" bson:"firewall_name"`
	VirtualContext   string           `json:"virtual_context" bson:"virtual_context"` // The specific virtual context this belongs to
	ApplicationGroup ApplicationGroup `json:"application_group_data" bson:"application_group_data"`
}

// ParsedNetworkTopologyRoutingEntries contains routing entries for a specific firewall
type ParsedNetworkTopologyRoutingEntries struct {
	ID             string       `json:"id" bson:"_id,omitempty"`
	TopologyID     string       `json:"topology_id" bson:"topology_id"`
	FirewallName   string       `json:"firewall_name" bson:"firewall_name"`
	VirtualContext string       `json:"virtual_context" bson:"virtual_context"` // The specific virtual context this belongs to
	RoutingEntry   RoutingEntry `json:"routing_entry_data" bson:"routing_entry_data"`
}

// ParsedNetworkTopologyNATs contains NATs for a specific firewall
type ParsedNetworkTopologyNATs struct {
	ID             string `json:"id" bson:"_id,omitempty"`
	TopologyID     string `json:"topology_id" bson:"topology_id"`
	FirewallName   string `json:"firewall_name" bson:"firewall_name"`
	VirtualContext string `json:"virtual_context" bson:"virtual_context"` // The specific virtual context this belongs to
	NAT            NAT    `json:"nat_data" bson:"nat_data"`
}

// ParsedNetworkTopologyFirewallPolicies contains firewall policies for a specific firewall
type ParsedNetworkTopologyFirewallPolicies struct {
	ID             string         `json:"id" bson:"_id,omitempty"`
	TopologyID     string         `json:"topology_id" bson:"topology_id"`
	FirewallName   string         `json:"firewall_name" bson:"firewall_name"`
	VirtualContext string         `json:"virtual_context" bson:"virtual_context"` // The specific virtual context this belongs to
	FirewallPolicy FirewallPolicy `json:"firewall_policy_data" bson:"firewall_policy_data"`
}

// ParsedNetworkTopologyTunnels contains tunnels for a specific firewall
type ParsedNetworkTopologyTunnels struct {
	ID             string `json:"id" bson:"_id,omitempty"`
	TopologyID     string `json:"topology_id" bson:"topology_id"`
	FirewallName   string `json:"firewall_name" bson:"firewall_name"`
	VirtualContext string `json:"virtual_context" bson:"virtual_context"` // The specific virtual context this belongs to
	Tunnel         Tunnel `json:"tunnel_data" bson:"tunnel_data"`
}

// ParsedNetworkTopologyDevices contains devices for a specific firewall
type ParsedNetworkTopologyDevices struct {
	ID             string `json:"id" bson:"_id,omitempty"`
	TopologyID     string `json:"topology_id" bson:"topology_id"`
	FirewallName   string `json:"firewall_name" bson:"firewall_name"`
	VirtualContext string `json:"virtual_context" bson:"virtual_context"` // The specific virtual context this belongs to
	Device         Device `json:"device_data" bson:"device_data"`
}

// ParsedNetworkTopologyOwnedAddresses contains owned addresses for a specific firewall
type ParsedNetworkTopologyOwnedAddresses struct {
	ID             string  `json:"id" bson:"_id,omitempty"`
	TopologyID     string  `json:"topology_id" bson:"topology_id"`
	FirewallName   string  `json:"firewall_name" bson:"firewall_name"`
	VirtualContext string  `json:"virtual_context" bson:"virtual_context"` // The specific virtual context this belongs to
	OwnedAddress   Address `json:"owned_address_data" bson:"owned_address_data"`
}

// DeviceGroup represents a logical grouping of devices (e.g., Panorama device groups)
type DeviceGroup struct {
	ID          string            `json:"id" bson:"_id,omitempty"`
	Name        string            `json:"name" bson:"name"`
	Description string            `json:"description,omitempty" bson:"description,omitempty"`
	Members     []string          `json:"members" bson:"members"` // Device IDs in this group
	Metadata    map[string]string `json:"metadata,omitempty" bson:"metadata,omitempty"`
}

// IntegrationMessage represents a message passed between integration components
type IntegrationMessage struct {
	ID        string                 `json:"id" bson:"_id,omitempty"`
	Source    string                 `json:"source" bson:"source"`
	Type      string                 `json:"type" bson:"type"`
	Payload   map[string]interface{} `json:"payload" bson:"payload"`
	Timestamp int64                  `json:"timestamp" bson:"timestamp"`
}



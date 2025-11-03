package models

import "fmt"

// ASA
// ASAConfig represents the parsed Cisco ASA configuration
type ASAConfig struct {
	// Device Identity
	Hostname                 string
	Version                  string
	
	// Configuration Data (from config parsing)
	Interfaces               []CiscoInterface
	NamedNetworkObjects      []CiscoNetworkObject
	Rules                    []CiscoRule
	Routes                   []CiscoRoute
	AccessGroups             []CiscoAccessGroup
	ServiceObjectGroups      []CiscoServiceObjectGroup
	StandaloneServiceObjects []CiscoStandaloneServiceObject
	NetworkObjectGroups      []CiscoNetworkObjectGroup
	ProtocolObjectGroups     []CiscoProtocolObjectGroup
	NatRules                 []CiscoNatRule
	RawConfig                string
	
	// Operational Data (from show commands) - Phase 1.2.4
	OperationalInterfaces    []ASAOperationalInterface   // show interface
	OperationalRoutes        []ASAOperationalRoute       // show route
	ARPEntries               []ASAARPEntry               // show arp
	XlateEntries             []ASAXlateEntry             // show xlate
	NATRules                 []ASANATRuleOperational     // show nat
	ActiveConnections        []ASAConnection             // show conn
	LocalHosts               []ASALocalHost              // show local-host
	SecurityZones            []ASAZone                   // show nameif
	ACLEntries               []ASAACLEntry               // show access-list
	ServicePolicies          []ASAServicePolicy          // show service-policy
	FailoverState            *ASAFailoverState           // show failover
	VPNSessions              []ASAVPNSession             // show vpn-sessiondb
	ObjectGroups             []ASAObjectGroup            // show object-group
	
	// Collection Metadata
	CollectedAt              string
}

type CiscoNatRule struct {
	SourceInterface      string
	DestinationInterface string
	Type                 string // "source" or "destination"
	NatType              string // "static", "dynamic", etc.
	RealObject           string
	MappedObject         string
}

// Interface represents a network interface configuration
type CiscoInterface struct {
	Name          string
	SecurityLevel int
	BridgeGroup   *int
	IPAddress     string
	Netmask       string
	Subnet        string // CIDR notation of the subnet
	Description   string
	VLAN          int
	Shutdown      bool
	Enabled       bool
	Members       []string
}

// ValidateInterface checks if an interface has the minimum required configuration
func (i *CiscoInterface) ValidateInterface() error {
	if i.Name == "" {
		return fmt.Errorf("interface has no name")
	}

	// Check if interface is shutdown
	if i.Shutdown {
		return nil // No need to validate other fields if interface is shutdown
	}

	// For enabled interfaces, check required fields
	if i.IPAddress == "" {
		return fmt.Errorf("interface %s has no IP address", i.Name)
	}

	if i.Netmask == "" {
		return fmt.Errorf("interface %s has no netmask", i.Name)
	}

	if i.SecurityLevel == 0 {
		return fmt.Errorf("interface %s has no security level", i.Name)
	}

	return nil
}

// NetworkObject represents a named network object
// https://www.cisco.com/c/en/us/td/docs/security/asa/asa-cli-reference/I-R/asa-command-ref-I-R/n-commands.html#wp7864662520
type CiscoNetworkObject struct {
	Name        string
	IPAddress   string
	Description string
}

// Rule represents an access control rule
// https://www.cisco.com/c/en/us/td/docs/security/asa/asa-cli-reference/A-H/asa-command-ref-A-H/aa-ac-commands.html#wp3258414970
type CiscoRule struct {
	Order       int
	Access      string
	Action      string // "permit" or "deny"
	Protocol    string
	Source      string
	SourcePorts []string // Slice of service names or port numbers
	Destination string
	DestPorts   []string // Slice of service names or port numbers
	Service     string
	Description string
	Log         bool
	Inactive    bool
	Debugging   bool
	Type        string
	// Map of protocol to its allowed ports
	ProtocolPorts map[string][]string
	Raw           string
}

// Route represents a static route
// https://www.cisco.com/c/en/us/td/docs/security/asa/asa-cli-reference/I-R/asa-command-ref-I-R/ret-rz-commands.html#wp1781820814
type CiscoRoute struct {
	Interface string
	Network   string
	Netmask   string
	NextHop   string // Next hop IP address
	Metric    int    // Administrative distance
}

// https://www.cisco.com/c/en/us/td/docs/security/asa/asa-cli-reference/A-H/asa-command-ref-A-H/aa-ac-commands.html#wp3192264241
type CiscoAccessGroup struct {
	Name       string // ACL name
	Type       string // "in" or "out" -
	Interfaces string
}

type CiscoServiceObject struct {
	Protocol string
	Operator string // e.g., "eq", "range", etc.
	Value    string // e.g., "snmp", "telnet", "1500", etc.
	Raw      string // the full original line for reference
}

type CiscoServiceObjectGroup struct {
	Name        string
	Protocol    string
	Description string
	Ports       []string
	Services    []CiscoServiceObject
}

func (g CiscoServiceObjectGroup) IsServiceObjectGroupEmpty() bool {
	return g.Name == "" || (len(g.Services) == 0 && len(g.Ports) == 0)
}

type CiscoNetworkObjectGroup struct {
	Name        string
	Description string
	Members     []string
}

// ProtocolObjectGroup represents a protocol object group
type CiscoProtocolObjectGroup struct {
	Name        string
	Description string
	Protocols   []string
}

type CiscoStandaloneServiceObject struct {
	Name        string
	Description string
	Services    []CiscoServiceObject
}

// IOS
// IOSConfig represents the parsed Cisco IOS configuration
type IOSConfig struct {
	Hostname            string
	Version             string
	Interfaces          []CiscoIOSInterface
	StandardACLs        []CiscoStandardACL
	ExtendedACLs        []CiscoExtendedACL
	NamedACLs           []CiscoNamedACL
	Routes              []CiscoIOSRoute
	VLANs               []CiscoVLAN
	NATRules            []CiscoIOSNATRule
	AccessGroupBindings []CiscoIOSAccessGroup
	RawConfig           string
}

// CiscoIOSInterface represents a Cisco IOS interface configuration
type CiscoIOSInterface struct {
	Name         string
	Type         string // FastEthernet, GigabitEthernet, Serial, Loopback, etc.
	IPAddress    string
	Netmask      string
	Subnet       string // CIDR notation of the subnet
	Description  string
	VLAN         int
	Shutdown     bool
	Enabled      bool
	AccessVLAN   int                   // for switchport access
	TrunkVLANs   []int                 // for switchport trunk
	AccessGroups []CiscoIOSAccessGroup // ACLs bound to this interface
}

// ValidateIOSInterface checks if an IOS interface has the minimum required configuration
func (i *CiscoIOSInterface) ValidateIOSInterface() error {
	if i.Name == "" {
		return fmt.Errorf("interface has no name")
	}

	// Check if interface is shutdown
	if i.Shutdown {
		return nil // No need to validate other fields if interface is shutdown
	}

	// For routed interfaces, check IP configuration
	if i.Type != "switchport" && i.IPAddress == "" {
		return fmt.Errorf("routed interface %s has no IP address", i.Name)
	}

	return nil
}

// CiscoIOSAccessGroup represents ACL binding to an interface
type CiscoIOSAccessGroup struct {
	ACLName   string // Name or number of the ACL
	Direction string // "in" or "out"
	Interface string // Interface this ACL is applied to
	ACLType   string // "standard", "extended", "named"
}

// CiscoStandardACL represents a standard numbered ACL (1-99, 1300-1999)
type CiscoStandardACL struct {
	Number      int
	Rules       []CiscoStandardACLRule
	Description string
	Raw         string
}

// CiscoStandardACLRule represents a single rule in a standard ACL
type CiscoStandardACLRule struct {
	Order    int
	Action   string // "permit" or "deny"
	Source   string // Source IP/network
	Wildcard string // Wildcard mask
	Log      bool
	Raw      string
}

// CiscoExtendedACL represents an extended numbered ACL (100-199, 2000-2699)
type CiscoExtendedACL struct {
	Number      int
	Rules       []CiscoExtendedACLRule
	Description string
	Raw         string
}

// CiscoExtendedACLRule represents a single rule in an extended ACL
type CiscoExtendedACLRule struct {
	Order          int
	Action         string   // "permit" or "deny"
	Protocol       string   // tcp, udp, icmp, ip, etc.
	Source         string   // Source IP/network
	SourceWildcard string   // Source wildcard mask
	SourcePorts    []string // Source ports/port ranges
	Destination    string   // Destination IP/network
	DestWildcard   string   // Destination wildcard mask
	DestPorts      []string // Destination ports/port ranges
	Established    bool     // TCP established flag
	Log            bool
	Raw            string
}

// CiscoNamedACL represents a named ACL
type CiscoNamedACL struct {
	Name        string
	Type        string                 // "standard" or "extended"
	Rules       []CiscoExtendedACLRule // Use extended rule format for both types
	Description string
	Raw         string
}

// CiscoIOSRoute represents a static route in IOS
type CiscoIOSRoute struct {
	Network       string // Destination network
	Netmask       string // Network mask
	NextHop       string // Next hop IP address
	Interface     string // Exit interface (optional)
	AdminDistance int    // Administrative distance
	Permanent     bool   // Permanent route flag
	Raw           string
}

// CiscoVLAN represents a VLAN configuration
type CiscoVLAN struct {
	ID          int
	Name        string
	Description string
	State       string // active, suspend, etc.
	Raw         string
}

// CiscoIOSNATRule represents a NAT rule in IOS
type CiscoIOSNATRule struct {
	Type             string // "static", "dynamic", "overload"
	InsideLocal      string // Inside local address/network
	InsideGlobal     string // Inside global address/network
	OutsideLocal     string // Outside local address/network (for bidirectional)
	OutsideGlobal    string // Outside global address/network (for bidirectional)
	InsideInterface  string // Inside interface
	OutsideInterface string // Outside interface
	ACL              string // Access list for dynamic NAT (optional)
	Pool             string // NAT pool name (optional)
	Overload         bool   // PAT/overload flag
	Raw              string
}

// Layer-2 Cisco structures (New Features for L2) ----------------------------------------------------

// CiscoL2Config carries Layer-2 centric data extracted from Cisco devices.
// This extends traditional L3 parsing with segmentation, security posture,
// and evidence metadata required for Layer-2 visibility.
type CiscoL2Config struct {
	// Device Identity
	Hostname            string
	Version             string
	
	// Configuration Data (from config parsing)
	VLANs               []CiscoL2VLAN
	Interfaces          []CiscoL2Interface
	PortChannels        []CiscoPortChannel
	GlobalL2Controls    CiscoGlobalL2Controls
	SVIs                []CiscoSVI
	RawConfig           string
	ConfigHash          string
	SourceSnippetHashes map[string]string
	
	// Operational Data (from show commands) - Phase 1.0.5
	FDBEntries               []CiscoFDBEntry                // show mac address-table dynamic
	DHCPSnoopingBindings     []CiscoDHCPSnoopingBinding     // show ip dhcp snooping binding
	AuthenticationSessions   []CiscoAuthenticationSession   // show authentication sessions
	PortSecurityStatus       []CiscoPortSecurityStatus      // show port-security
	ARPInspectionStats       []CiscoARPInspectionStats      // show ip arp inspection statistics
	IPSGBindings             []CiscoIPSGBinding             // show ip verify source
	PVLANMappings            []CiscoPVLANMapping            // show vlan private-vlan
	
	// Collection Metadata
	CollectedAt              string
}

// CiscoL2VLAN records VLAN metadata and its evidence markers.
type CiscoL2VLAN struct {
	ID                int
	Name              string
	State             string
	VACLName          *string
	SourceBlock       string
	SourceSnippetHash string
}

// CiscoL2Interface stores per-port switching posture and security controls.
type CiscoL2Interface struct {
	Name               string
	AdminState         string
	Mode               string
	AccessVLAN         *int
	TrunkAllowedVLANs  []int
	TrunkNativeVLAN    *int
	PortChannelID      *int
	DHCPSnoopingTrust  bool
	ARPInspectionTrust bool
	IPSourceGuard      bool
	Dot1xRequired      bool
	MABEnabled         bool
	UnauthVLAN         *int
	PACLInName         *string
	PACLOutName        *string
	PVLANRole          *string
	PVLANPrimaryVLAN   *int
	PVLANSecondaryVLAN *int
	Description        string
	SourceBlock        string
	SourceSnippetHash  string
}

// CiscoPortChannel models aggregated Layer-2 interfaces and their VLAN posture.
type CiscoPortChannel struct {
	ID                int
	Name              string
	MemberInterfaces  []string
	Mode              string
	AccessVLAN        *int
	TrunkAllowedVLANs []int
	TrunkNativeVLAN   *int
	SourceBlock       string
	SourceSnippetHash string
}

// CiscoGlobalL2Controls aggregates switch-wide integrity toggles.
type CiscoGlobalL2Controls struct {
	SnoopingEnabled        bool
	SnoopingVLANs          []int
	DAIEnabledVLANs        []int
	DAIACLByVLAN           map[int]string
	IPSGGlobal             bool
	Dot1xSystemAuthControl bool
	MABGlobal              bool
	SourceBlock            string
	SourceSnippetHash      string
	LastUpdated            string
}

// CiscoSVI marks VLAN interfaces that provide L3 handoff points.
type CiscoSVI struct {
	VLANID            int
	InterfaceName     string
	IPAddress         string
	Subnet            string
	Description       string
	SourceBlock       string
	SourceSnippetHash string
}

// CiscoFDBEntry represents MAC placement evidence gathered from the device or
// supporting telemetry sources.
type CiscoFDBEntry struct {
	MAC            string
	LastSeenPort   string
	VLANID         int
	LastSeenTS     string
	EvidenceSource string
	Confidence     float64
}

// ===== Operational Data Structures (Phase 1.0.5) =====

// CiscoDHCPSnoopingBinding represents a DHCP snooping binding entry
// This is the PRIMARY source for host-to-port confidence scoring
type CiscoDHCPSnoopingBinding struct {
	MACAddress   string `json:"mac_address"`
	IPAddress    string `json:"ip_address"`
	LeaseSeconds int    `json:"lease_seconds"`
	Type         string `json:"type"`      // "dhcp-snooping" | "static"
	VLANId       int    `json:"vlan_id"`
	Interface    string `json:"interface"`
	CollectedAt  string `json:"collected_at"`
}

// CiscoARPInspectionStats represents DAI statistics for a VLAN
type CiscoARPInspectionStats struct {
	VLANId      int    `json:"vlan_id"`
	Forwarded   int    `json:"forwarded"`
	Dropped     int    `json:"dropped"`
	DHCPDrops   int    `json:"dhcp_drops"`
	ACLDrops    int    `json:"acl_drops"`
	CollectedAt string `json:"collected_at"`
}

// CiscoAuthenticationSession represents an 802.1X or MAB session
// This is DECISION CRITICAL - determines Allowed vs. Conditional/Blocked
type CiscoAuthenticationSession struct {
	Interface   string `json:"interface"`
	MACAddress  string `json:"mac_address"`
	Method      string `json:"method"`         // "dot1x" | "mab" | "webauth"
	Domain      string `json:"domain"`         // "DATA" | "VOICE"
	Status      string `json:"status"`         // "Authz Success" | "Authz Failed" | "Running"
	SessionID   string `json:"session_id"`
	VLANId      int    `json:"vlan_id,omitempty"`
	Username    string `json:"username,omitempty"`
	SessionTime string `json:"session_time,omitempty"`
	CollectedAt string `json:"collected_at"`
}

// CiscoPortSecurityStatus represents port security status for an interface
type CiscoPortSecurityStatus struct {
	Interface         string `json:"interface"`
	MaxMACCount       int    `json:"max_mac_count"`
	CurrentMACCount   int    `json:"current_mac_count"`
	SecurityViolation string `json:"security_violation"` // "Shutdown" | "Restrict" | "Protect"
	Status            string `json:"status"`             // "Secure-up" | "Secure-down" | "Secure-shutdown"
	ViolationCount    int    `json:"violation_count"`
	LastSource        string `json:"last_source,omitempty"`
	CollectedAt       string `json:"collected_at"`
}

// CiscoIPSGBinding represents an IP Source Guard binding
type CiscoIPSGBinding struct {
	Interface   string `json:"interface"`
	FilterType  string `json:"filter_type"`  // "ip" | "ip-mac"
	FilterMode  string `json:"filter_mode"`  // "active" | "inactive"
	IPAddress   string `json:"ip_address"`
	MACAddress  string `json:"mac_address,omitempty"`
	VLANId      int    `json:"vlan_id"`
	CollectedAt string `json:"collected_at"`
}

// CiscoPVLANMapping represents a Private VLAN mapping
type CiscoPVLANMapping struct {
	PrimaryVLAN      int      `json:"primary_vlan"`
	SecondaryVLAN    int      `json:"secondary_vlan"`
	Type             string   `json:"type"` // "isolated" | "community"
	PromiscuousPorts []string `json:"promiscuous_ports,omitempty"`
	HostPorts        []string `json:"host_ports,omitempty"`
	CollectedAt      string   `json:"collected_at"`
}

// ===== ASA Operational Data Structures (Phase 1.2.4) =====

// ASAOperationalInterface represents an ASA interface with operational state
type ASAOperationalInterface struct {
	Name          string `json:"name"`
	Nameif        string `json:"nameif"`         // Security zone name
	SecurityLevel int    `json:"security_level"`
	IPAddress     string `json:"ip_address"`
	SubnetMask    string `json:"subnet_mask"`
	MACAddress    string `json:"mac_address"`
	MTU           int    `json:"mtu"`
	AdminState    string `json:"admin_state"`    // "up" | "down"
	LineProtocol  string `json:"line_protocol"`  // "up" | "down"
	Bandwidth     int    `json:"bandwidth"`      // Mbps
	Description   string `json:"description,omitempty"`
	CollectedAt   string `json:"collected_at"`
}

// ASAOperationalRoute represents a routing table entry
type ASAOperationalRoute struct {
	Network       string `json:"network"`
	Netmask       string `json:"netmask"`
	NextHop       string `json:"next_hop,omitempty"`
	Interface     string `json:"interface"`
	RouteType     string `json:"route_type"`    // "C" (connected), "S" (static), etc.
	AdminDistance int    `json:"admin_distance,omitempty"`
	Metric        int    `json:"metric,omitempty"`
	CollectedAt   string `json:"collected_at"`
}

// ASAARPEntry represents an ARP table entry
type ASAARPEntry struct {
	Interface   string `json:"interface"`
	IPAddress   string `json:"ip_address"`
	MACAddress  string `json:"mac_address"`
	Age         int    `json:"age"`        // seconds
	CollectedAt string `json:"collected_at"`
}

// ASAXlateEntry represents an active NAT translation
type ASAXlateEntry struct {
	Flags         string `json:"flags"`          // "D" (DNS), "i" (dynamic), "s" (static), etc.
	SourceZone    string `json:"source_zone"`    // e.g., "inside"
	SourceIP      string `json:"source_ip"`
	SourcePort    int    `json:"source_port,omitempty"`
	DestZone      string `json:"dest_zone"`      // e.g., "outside"
	DestIP        string `json:"dest_ip"`
	DestPort      int    `json:"dest_port,omitempty"`
	Protocol      string `json:"protocol,omitempty"`  // "TCP", "UDP", "ICMP"
	IdleTime      string `json:"idle_time"`
	Timeout       string `json:"timeout"`
	CollectedAt   string `json:"collected_at"`
}

// ASANATRuleOperational represents a NAT policy rule with hit counts
type ASANATRuleOperational struct {
	Section          string `json:"section"`           // "Manual NAT" or "Auto NAT"
	RuleNumber       int    `json:"rule_number"`
	SourceZone       string `json:"source_zone"`
	DestZone         string `json:"dest_zone"`
	SourceObject     string `json:"source_object"`
	DestObject       string `json:"dest_object,omitempty"`
	NATType          string `json:"nat_type"`          // "static", "dynamic"
	TranslateHits    int    `json:"translate_hits"`
	UntranslateHits  int    `json:"untranslate_hits"`
	CollectedAt      string `json:"collected_at"`
}

// ASAConnection represents an active firewall connection
type ASAConnection struct {
	Protocol     string `json:"protocol"`       // "TCP", "UDP", "ICMP"
	SourceZone   string `json:"source_zone"`
	SourceIP     string `json:"source_ip"`
	SourcePort   int    `json:"source_port,omitempty"`
	DestZone     string `json:"dest_zone"`
	DestIP       string `json:"dest_ip"`
	DestPort     int    `json:"dest_port,omitempty"`
	IdleTime     string `json:"idle_time"`
	Bytes        int    `json:"bytes"`
	Flags        string `json:"flags"`
	CollectedAt  string `json:"collected_at"`
}

// ASALocalHost represents a local host entry
type ASALocalHost struct {
	Interface        string `json:"interface"`
	IPAddress        string `json:"ip_address"`
	TCPFlowCount     int    `json:"tcp_flow_count"`
	TCPFlowLimit     string `json:"tcp_flow_limit"`      // "unlimited" or number
	UDPFlowCount     int    `json:"udp_flow_count"`
	UDPFlowLimit     string `json:"udp_flow_limit"`
	TCPEmbryonicCount int   `json:"tcp_embryonic_count"`
	CollectedAt      string `json:"collected_at"`
}

// ASAZone represents a security zone (nameif)
type ASAZone struct {
	Interface     string `json:"interface"`
	ZoneName      string `json:"zone_name"`
	SecurityLevel int    `json:"security_level"`
	CollectedAt   string `json:"collected_at"`
}

// ASAACLEntry represents an access control list entry
type ASAACLEntry struct {
	ACLName     string `json:"acl_name"`
	LineNumber  int    `json:"line_number"`
	RuleType    string `json:"rule_type"`      // "extended", "standard"
	Action      string `json:"action"`         // "permit", "deny"
	Protocol    string `json:"protocol"`       // "tcp", "udp", "ip", "icmp"
	Source      string `json:"source"`
	SourcePort  string `json:"source_port,omitempty"`
	Dest        string `json:"dest"`
	DestPort    string `json:"dest_port,omitempty"`
	HitCount    int    `json:"hit_count"`
	HashID      string `json:"hash_id,omitempty"`
	CollectedAt string `json:"collected_at"`
}

// ASAServicePolicy represents a service policy
type ASAServicePolicy struct {
	PolicyName  string `json:"policy_name"`
	ClassMap    string `json:"class_map"`
	Inspection  string `json:"inspection,omitempty"`
	Packets     int    `json:"packets"`
	Drops       int    `json:"drops"`
	ResetDrops  int    `json:"reset_drops,omitempty"`
	CollectedAt string `json:"collected_at"`
}

// ASAFailoverState represents failover/HA state
type ASAFailoverState struct {
	FailoverEnabled bool   `json:"failover_enabled"`
	ThisUnit        string `json:"this_unit"`          // "Primary" | "Secondary"
	ThisState       string `json:"this_state"`         // "Active" | "Standby Ready" | "Failed"
	OtherUnit       string `json:"other_unit"`
	OtherState      string `json:"other_state"`
	ActiveTime      int    `json:"active_time"`        // seconds
	StandbyTime     int    `json:"standby_time"`       // seconds
	LANInterface    string `json:"lan_interface"`
	LANStatus       string `json:"lan_status"`         // "up" | "down"
	CollectedAt     string `json:"collected_at"`
}

// ASAVPNSession represents a VPN session
type ASAVPNSession struct {
	SessionType string `json:"session_type"`   // "IPsec", "SSL", "IKEv2"
	Username    string `json:"username"`
	PublicIP    string `json:"public_ip"`
	Protocol    string `json:"protocol"`
	SessionID   int    `json:"session_id"`
	Duration    string `json:"duration"`
	BytesTx     int    `json:"bytes_tx"`
	BytesRx     int    `json:"bytes_rx"`
	CollectedAt string `json:"collected_at"`
}

// ASAObjectGroup represents an object group
type ASAObjectGroup struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`           // "network", "service", "protocol"
	Members     []string `json:"members"`
	CollectedAt string   `json:"collected_at"`
}

// ===== NXOS Data Structures (Phase 2.1.4) =====

// NXOSConfig represents the parsed Cisco NXOS configuration
type NXOSConfig struct {
	// Device Identity
	Hostname string
	Version  string

	// Configuration Data (from config parsing)
	Interfaces          []CiscoInterface
	Routes              []CiscoRoute
	VLANs               []CiscoVLAN
	RawConfig           string

	// Operational Data (from show commands) - Phase 2.1.2 & 2.1.3
	MACTable            []NXOSMACEntry              // show mac address-table
	VLANOperational     []NXOSVLANOperational       // show vlan
	InterfaceStatus     []NXOSInterfaceStatus       // show interface status
	InterfaceSwitchport []NXOSInterfaceSwitchport   // show interface switchport (if needed)
	TrunkInfo           []NXOSTrunk                 // show interface trunk
	PortChannels        []NXOSPortChannel           // show port-channel summary
	STPSummary          *NXOSSTPSummary             // show spanning-tree summary
	LLDPNeighbors       []NXOSLLDPNeighbor          // show lldp neighbors detail
	CDPNeighbors        []NXOSCDPNeighbor           // show cdp neighbors detail
	ARPEntries          []NXOSARPEntry              // show ip arp

	// VPC Data
	VPCStatus           *NXOSVPCStatus              // show vpc
	VPCKeepalive        *NXOSVPCKeepalive           // show vpc peer-keepalive
	VPCConsistency      []NXOSVPCConsistencyParam   // show vpc consistency-parameters

	// VDC & FabricPath
	VDCs                []NXOSVDC                   // show vdc
	FabricPathTopology  *NXOSFabricPathTopology     // show fabricpath topology

	// Collection Metadata
	CollectedAt string
}

// NXOSMACEntry represents a MAC address table entry
type NXOSMACEntry struct {
	VLAN          int    `json:"vlan"`
	MACAddress    string `json:"mac_address"`
	Type          string `json:"type"`         // "dynamic", "static"
	Age           int    `json:"age"`          // seconds, -1 for static
	Secure        bool   `json:"secure"`
	NTFY          bool   `json:"ntfy"`         // Notification
	Port          string `json:"port"`
	IsPrimary     bool   `json:"is_primary"`   // "*" prefix
	IsVPCPeerLink bool   `json:"is_vpc_peer"`  // "+" suffix
	CollectedAt   string `json:"collected_at"`
}

// NXOSVLANOperational represents operational VLAN state
type NXOSVLANOperational struct {
	ID          int      `json:"id"`
	Name        string   `json:"name"`
	Status      string   `json:"status"`      // "active", "suspended"
	Ports       []string `json:"ports"`
	CollectedAt string   `json:"collected_at"`
}

// NXOSInterfaceStatus represents interface status
type NXOSInterfaceStatus struct {
	Port        string `json:"port"`
	Name        string `json:"name"`
	Status      string `json:"status"`       // "connected", "notconnect", "disabled"
	VLAN        string `json:"vlan"`         // VLAN ID or "trunk"
	Duplex      string `json:"duplex"`       // "full", "half", "auto"
	Speed       string `json:"speed"`        // "10G", "1000", "auto"
	Type        string `json:"type"`
	CollectedAt string `json:"collected_at"`
}

// NXOSInterfaceSwitchport represents switchport configuration
type NXOSInterfaceSwitchport struct {
	Interface          string `json:"interface"`
	Mode               string `json:"mode"`                // "access", "trunk"
	AccessVLAN         int    `json:"access_vlan"`
	TrunkNativeVLAN    int    `json:"trunk_native_vlan"`
	TrunkAllowedVLANs  string `json:"trunk_allowed_vlans"` // "1-4094" or "10,20,30"
	AdminPrivateVLAN   string `json:"admin_private_vlan"`
	OperationalMode    string `json:"operational_mode"`
	CollectedAt        string `json:"collected_at"`
}

// NXOSTrunk represents a trunk interface
type NXOSTrunk struct {
	Port           string `json:"port"`
	NativeVLAN     int    `json:"native_vlan"`
	Status         string `json:"status"`          // "trunking", "not-trunking"
	PortChannel    string `json:"port_channel"`
	VLANsAllowed   string `json:"vlans_allowed"`   // "1,10,20" or "1-4094"
	CollectedAt    string `json:"collected_at"`
}

// NXOSPortChannel represents a port-channel (LAG)
type NXOSPortChannel struct {
	GroupID      int               `json:"group_id"`
	Name         string            `json:"name"`
	State        string            `json:"state"`        // "SU" = Switched, Up
	Type         string            `json:"type"`         // "Eth", "Fcoe"
	Protocol     string            `json:"protocol"`     // "LACP", "NONE", "STATIC"
	Members      []string          `json:"members"`
	MemberStates map[string]string `json:"member_states"` // Port -> State (P, D, I, etc.)
	CollectedAt  string            `json:"collected_at"`
}

// NXOSSTPSummary represents spanning tree summary
type NXOSSTPSummary struct {
	Mode                 string `json:"mode"`                    // "rapid-pvst", "mst"
	RootBridgeVLANs      []int  `json:"root_bridge_vlans"`
	ExtendedSystemID     bool   `json:"extended_system_id"`
	PortfastDefault      bool   `json:"portfast_default"`
	BPDUGuardDefault     bool   `json:"bpdu_guard_default"`
	LoopguardDefault     bool   `json:"loopguard_default"`
	EtherChannelMisconfig bool  `json:"etherchannel_misconfig_guard"`
	CollectedAt          string `json:"collected_at"`
}

// NXOSLLDPNeighbor represents an LLDP neighbor
type NXOSLLDPNeighbor struct {
	LocalPort           string `json:"local_port"`
	ChassisID           string `json:"chassis_id"`
	PortID              string `json:"port_id"`
	PortDescription     string `json:"port_description"`
	SystemName          string `json:"system_name"`
	SystemDescription   string `json:"system_description"`
	SystemCapabilities  string `json:"system_capabilities"`
	EnabledCapabilities string `json:"enabled_capabilities"`
	ManagementAddress   string `json:"management_address"`
	VLANID              int    `json:"vlan_id"`
	TimeRemaining       int    `json:"time_remaining"`      // seconds
	CollectedAt         string `json:"collected_at"`
}

// NXOSCDPNeighbor represents a CDP neighbor
type NXOSCDPNeighbor struct {
	DeviceID         string `json:"device_id"`
	LocalInterface   string `json:"local_interface"`
	RemotePort       string `json:"remote_port"`
	Platform         string `json:"platform"`
	Capabilities     string `json:"capabilities"`
	IPAddress        string `json:"ip_address"`
	Version          string `json:"version"`
	NativeVLAN       int    `json:"native_vlan"`
	Duplex           string `json:"duplex"`
	ManagementIP     string `json:"management_ip"`
	Holdtime         int    `json:"holdtime"`
	CollectedAt      string `json:"collected_at"`
}

// NXOSARPEntry represents an ARP table entry
type NXOSARPEntry struct {
	IPAddress   string `json:"ip_address"`
	Age         string `json:"age"`         // "00:05:12" format
	MACAddress  string `json:"mac_address"`
	Interface   string `json:"interface"`
	CollectedAt string `json:"collected_at"`
}

// ===== VPC Data Structures =====

// NXOSVPCStatus represents VPC domain status
type NXOSVPCStatus struct {
	DomainID                    int    `json:"domain_id"`
	PeerStatus                  string `json:"peer_status"`
	KeepaliveStatus             string `json:"keepalive_status"`
	ConfigConsistencyStatus     string `json:"config_consistency_status"`
	PerVLANConsistencyStatus    string `json:"per_vlan_consistency_status"`
	Type2ConsistencyStatus      string `json:"type2_consistency_status"`
	VPCRole                     string `json:"vpc_role"`               // "primary", "secondary"
	NumberOfVPCs                int    `json:"number_of_vpcs"`
	PeerGateway                 bool   `json:"peer_gateway"`
	DualActiveExcludedVLANs     string `json:"dual_active_excluded_vlans"`
	GracefulConsistencyCheck    bool   `json:"graceful_consistency_check"`
	AutoRecoveryEnabled         bool   `json:"auto_recovery_enabled"`
	DelayRestoreEnabled         bool   `json:"delay_restore_enabled"`
	OperationalLayer3PeerRouter bool   `json:"operational_layer3_peer_router"`
	PeerLinkID                  int    `json:"peer_link_id"`
	PeerLinkPort                string `json:"peer_link_port"`
	PeerLinkStatus              string `json:"peer_link_status"`
	PeerLinkActiveVLANs         string `json:"peer_link_active_vlans"`
	CollectedAt                 string `json:"collected_at"`
}

// NXOSVPCKeepalive represents VPC keepalive status
type NXOSVPCKeepalive struct {
	Status          string `json:"status"`           // "peer is alive"
	PeerAliveFor    string `json:"peer_alive_for"`   // "(166978) seconds"
	SendStatus      string `json:"send_status"`      // "Success"
	LastSendAt      string `json:"last_send_at"`
	SentOnInterface string `json:"sent_on_interface"`
	ReceiveStatus   string `json:"receive_status"`
	LastReceiveAt   string `json:"last_receive_at"`
	ReceivedOnIntf  string `json:"received_on_interface"`
	Destination     string `json:"destination"`      // IP address
	Interval        string `json:"interval"`         // "1000 msec"
	Timeout         string `json:"timeout"`          // "5 seconds"
	VRF             string `json:"vrf"`
	UDPPort         int    `json:"udp_port"`
	CollectedAt     string `json:"collected_at"`
}

// NXOSVPCConsistencyParam represents a VPC consistency parameter
type NXOSVPCConsistencyParam struct {
	Name        string `json:"name"`
	Type        string `json:"type"`         // "1" = suspend if mismatch
	LocalValue  string `json:"local_value"`
	PeerValue   string `json:"peer_value"`
	Consistent  bool   `json:"consistent"`
	CollectedAt string `json:"collected_at"`
}

// ===== VDC & FabricPath Data Structures =====

// NXOSVDC represents a Virtual Device Context
type NXOSVDC struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	State       string `json:"state"`        // "active", "suspended"
	MACAddress  string `json:"mac_address"`
	Type        string `json:"type"`         // "Ethernet", "Storage"
	LineCard    string `json:"line_card"`
	CollectedAt string `json:"collected_at"`
}

// NXOSFabricPathTopology represents FabricPath IS-IS topology
type NXOSFabricPathTopology struct {
	Domain       string                     `json:"domain"`
	Tag          int                        `json:"tag"`
	SystemID     string                     `json:"system_id"`
	ISType       string                     `json:"is_type"`       // "L1L2"
	SAP          int                        `json:"sap"`
	MaxLSPMTU    int                        `json:"max_lsp_mtu"`
	TopologyID   int                        `json:"topology_id"`
	LocalSwID    int                        `json:"local_switch_id"`
	Adjacencies  []NXOSFabricPathAdjacency  `json:"adjacencies"`
	CollectedAt  string                     `json:"collected_at"`
}

// NXOSFabricPathAdjacency represents a FabricPath neighbor
type NXOSFabricPathAdjacency struct {
	SwitchID   int    `json:"switch_id"`
	SystemID   string `json:"system_id"`
	Name       string `json:"name"`
	Port       string `json:"port"`
	UpTime     string `json:"up_time"`
	Bandwidth  string `json:"bandwidth"`
	IsLocal    bool   `json:"is_local"`
}

// ===========================
// L2 Security Control Models
// ===========================

// CiscoGlobalL2Controls represents global L2 security controls configuration
type CiscoGlobalL2Controls struct {
	DHCPSnoopingEnabled     bool     `json:"dhcp_snooping_enabled"`
	DHCPSnoopingVLANs       []int    `json:"dhcp_snooping_vlans"`        // VLANs with DHCP snooping enabled
	ARPInspectionEnabled    bool     `json:"arp_inspection_enabled"`     // DAI globally enabled
	ARPInspectionVLANs      []int    `json:"arp_inspection_vlans"`       // VLANs with DAI enabled
	IPSGEnabled             bool     `json:"ipsg_enabled"`               // IP Source Guard globally enabled
	Dot1XEnabled            bool     `json:"dot1x_enabled"`              // 802.1X globally enabled
	MABEnabled              bool     `json:"mab_enabled"`                // MAC Authentication Bypass enabled
	PortSecurityEnabled     bool     `json:"port_security_enabled"`      // Port security feature enabled
	PrivateVLANsConfigured  bool     `json:"private_vlans_configured"`   // PVLANs configured
	CollectedAt             string   `json:"collected_at"`
}

// CiscoL2InterfaceSecurity represents per-interface L2 security state
// This extends the basic interface model with security-specific fields
type CiscoL2InterfaceSecurity struct {
	Interface                  string `json:"interface"`

	// DHCP Snooping
	DHCPSnoopingTrusted        bool   `json:"dhcp_snooping_trusted"`
	DHCPSnoopingRateLimit      int    `json:"dhcp_snooping_rate_limit"`     // pkts/sec
	DHCPSnoopingBindingCount   int    `json:"dhcp_snooping_binding_count"`

	// Dynamic ARP Inspection
	ARPInspectionTrusted       bool   `json:"arp_inspection_trusted"`
	ARPInspectionRateLimit     int    `json:"arp_inspection_rate_limit"`

	// IP Source Guard
	IPSGEnabled                bool   `json:"ipsg_enabled"`
	IPSGFilterType             string `json:"ipsg_filter_type"`             // ip, ip-mac
	IPSGBindingCount           int    `json:"ipsg_binding_count"`

	// 802.1X / MAB
	Dot1XEnabled               bool   `json:"dot1x_enabled"`
	Dot1XPortControl           string `json:"dot1x_port_control"`           // auto, force-authorized, force-unauthorized
	Dot1XHostMode              string `json:"dot1x_host_mode"`              // single-host, multi-host, multi-auth, multi-domain
	MABEnabled                 bool   `json:"mab_enabled"`
	AuthenticationStatus       string `json:"authentication_status"`        // Authc Success, Authc Failed, Running, etc.
	AuthenticatedMAC           string `json:"authenticated_mac"`
	AuthenticationMethod       string `json:"authentication_method"`        // dot1x, mab

	// Port Security
	PortSecurityEnabled        bool   `json:"port_security_enabled"`
	PortSecurityMaxMAC         int    `json:"port_security_max_mac"`
	PortSecurityCurrentMAC     int    `json:"port_security_current_mac"`
	PortSecurityViolationMode  string `json:"port_security_violation_mode"` // shutdown, restrict, protect
	PortSecurityViolationCount int    `json:"port_security_violation_count"`
	PortSecuritySticky         bool   `json:"port_security_sticky"`

	// STP Security
	STPBPDUGuard               bool   `json:"stp_bpdu_guard"`
	STPRootGuard               bool   `json:"stp_root_guard"`
	STPLoopGuard               bool   `json:"stp_loop_guard"`
	STPPortFast                bool   `json:"stp_portfast"`

	// PVLAN
	PrivateVLANPrimary         int    `json:"private_vlan_primary"`
	PrivateVLANSecondary       int    `json:"private_vlan_secondary"`
	PrivateVLANType            string `json:"private_vlan_type"`            // host, promiscuous, isolated, community

	// Security Posture Summary
	SecurityScore              int    `json:"security_score"`               // 0-100 calculated score
	SecurityLevel              string `json:"security_level"`               // high, medium, low, none

	CollectedAt                string `json:"collected_at"`
}

// CiscoFDBEntry represents a MAC address table entry with L2 security metadata
// This extends the basic FDBEntry with confidence scoring and evidence tracking
type CiscoFDBEntry struct {
	DeviceID         string            `json:"device_id"`
	MAC              string            `json:"mac"`
	VLAN             int               `json:"vlan"`
	Interface        string            `json:"interface"`
	Type             string            `json:"type"`              // dynamic, static, secure, self
	Age              int               `json:"age"`               // seconds

	// Evidence & Confidence Scoring
	EvidenceSources  []string          `json:"evidence_sources"`  // mac-table, dhcp-snooping, arp, dot1x, etc.
	Confidence       string            `json:"confidence"`        // high, medium, low
	ConfidenceScore  int               `json:"confidence_score"`  // 0-100
	Freshness        string            `json:"freshness"`         // fresh, stale, old

	// L2 Security Context
	DHCPSnoopingBound bool             `json:"dhcp_snooping_bound"` // Has DHCP snooping binding
	Authenticated     bool             `json:"authenticated"`       // Has 802.1X/MAB auth
	PortSecurityLearned bool           `json:"port_security_learned"` // Learned via port security
	IPSGBound         bool             `json:"ipsg_bound"`          // Has IPSG binding

	// Associated Data
	IPAddress         string            `json:"ip_address"`          // From DHCP/ARP
	DHCPLeaseTime     int               `json:"dhcp_lease_time"`     // Seconds remaining
	AuthMethod        string            `json:"auth_method"`         // dot1x, mab, none

	// Metadata
	Metadata          map[string]string `json:"metadata"`            // Additional context
	LastSeen          string            `json:"last_seen"`           // RFC3339 timestamp
	CollectedAt       string            `json:"collected_at"`        // RFC3339 timestamp
}

// L2DecisionCacheEntry represents a cached L2 decision for an endpoint
// Used by the L2 decision engine to store Allowed/Conditional/Blocked decisions
type L2DecisionCacheEntry struct {
	DeviceID            string            `json:"device_id"`
	MAC                 string            `json:"mac"`
	IPAddress           string            `json:"ip_address"`
	VLAN                int               `json:"vlan"`
	Interface           string            `json:"interface"`

	// Decision
	Decision            string            `json:"decision"`              // Allowed, Conditional, Blocked
	DecisionReason      string            `json:"decision_reason"`       // Why this decision was made
	DecisionConfidence  int               `json:"decision_confidence"`   // 0-100

	// Security Controls State
	Authenticated       bool              `json:"authenticated"`
	DHCPSnoopingBound   bool              `json:"dhcp_snooping_bound"`
	ARPInspectionValid  bool              `json:"arp_inspection_valid"`
	IPSGValid           bool              `json:"ipsg_valid"`
	PortSecurityValid   bool              `json:"port_security_valid"`

	// Evidence
	EvidenceSources     []string          `json:"evidence_sources"`
	SecurityScore       int               `json:"security_score"`        // 0-100

	// Cache Management
	CachedAt            string            `json:"cached_at"`             // RFC3339 timestamp
	ExpiresAt           string            `json:"expires_at"`            // RFC3339 timestamp
	TTLSeconds          int               `json:"ttl_seconds"`

	// Metadata
	Metadata            map[string]string `json:"metadata"`
}

// L2SwitchTopology represents the complete L2 switch topology with security state
// This is the top-level model for L2 network visibility
type L2SwitchTopology struct {
	DeviceID            string                        `json:"device_id"`
	Hostname            string                        `json:"hostname"`
	Model               string                        `json:"model"`
	Version             string                        `json:"version"`

	// Global L2 Controls
	GlobalControls      *CiscoGlobalL2Controls        `json:"global_controls"`

	// Topology
	Interfaces          map[string]*CiscoInterface    `json:"interfaces"`           // Interface name -> Interface
	VLANs               map[int]*CiscoVLAN            `json:"vlans"`                // VLAN ID -> VLAN
	PortChannels        []CiscoPortChannel            `json:"port_channels"`

	// Security State
	InterfaceSecurity   map[string]*CiscoL2InterfaceSecurity `json:"interface_security"` // Interface name -> Security state
	FDBEntries          []*CiscoFDBEntry              `json:"fdb_entries"`

	// L2 Security Data
	DHCPSnoopingBindings   []DHCPSnoopingBinding      `json:"dhcp_snooping_bindings"`
	AuthSessions           []AuthenticationSession    `json:"auth_sessions"`
	PortSecurityStatus     []PortSecurityStatus       `json:"port_security_status"`
	IPSGBindings           []IPSGBinding              `json:"ipsg_bindings"`
	ARPInspectionStats     *ARPInspectionStatistics   `json:"arp_inspection_stats"`
	PrivateVLANs           []PrivateVLAN              `json:"private_vlans"`

	// Decisions (cached)
	L2Decisions         []*L2DecisionCacheEntry       `json:"l2_decisions"`

	// Metadata
	CollectedAt         string                        `json:"collected_at"`
	ParsedAt            string                        `json:"parsed_at"`
}

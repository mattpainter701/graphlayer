package models

import (
	"fmt"
	"net"
)

// ========================================================================
// Dell OS10 / FTOS Data Models
// ========================================================================
// This file defines Dell OS10-specific configuration and operational data
// structures used by the OS10/FTOS parsers and mappers.
//
// OS10 is the current Dell EMC SmartFabric OS (formerly FTOS / Force10 OS).
// VLT (Virtual Link Trunking) is OS10's multi-chassis LAG equivalent of MLAG.

// OS10Config represents a parsed Dell OS10 configuration
type OS10Config struct {
	// Device Identity
	Hostname string
	Version  string
	Model    string
	Serial   string

	// Configuration Data
	Interfaces          []OS10Interface
	VLANs               []OS10VLAN
	PortChannels        []OS10PortChannel
	VLTConfig           *OS10VLTConfig
	StaticRoutes        []OS10StaticRoute
	OSPFRouter          *OS10OSPFRouter
	BGPConfig           *OS10BGPConfig
	VRFConfigs          []OS10VRF
	StandardACLs        []OS10StandardACL
	ExtendedACLs        []OS10ExtendedACL
	NamedACLs           []OS10NamedACL
	AccessGroupBindings []OS10AccessGroup
	RawConfig           string
}

// OS10Interface represents a Dell OS10 interface configuration
type OS10Interface struct {
	Name          string
	Description   string
	IPAddress     string
	Netmask       string
	Subnet        string
	Shutdown      bool
	VLAN          int      // VLAN ID for subinterfaces / SVI
	AccessVLAN    int      // For switchport access
	TrunkVLANs    []int    // For switchport trunk
	TrunkNative   int      // Native VLAN for trunk
	Switchport    bool     // Is this a switchport (L2)
	SwitchportMode string  // access, trunk, routed
	LACPTimeout   string   // fast / slow
	LAGGroup      int      // Port-channel group (0 = none)
	LAGLACP       bool     // LACP mode active/passive
	Flowcontrol   string   // send / receive settings
	MTU           int
	LLDPTransmit  bool
	LLDPReceive   bool
	StormControl  string   // broadcast, multicast, unknown-unicast
}

// ValidateOS10Interface checks interface validity
func (i *OS10Interface) ValidateOS10Interface() error {
	if i.Name == "" {
		return fmt.Errorf("interface has no name")
	}
	return nil
}

// OS10VLAN represents a VLAN definition in OS10
type OS10VLAN struct {
	ID          int
	Name        string
	State       string // active / suspend
	TrunkGroups []string
	Raw         string
}

// OS10PortChannel represents a port-channel / LAG configuration
type OS10PortChannel struct {
	ID               int
	Name             string
	MemberInterfaces []string
	Mode             string // access / trunk / routed
	LACPTimeout      string
	MinimumLinks     int
	LoadBalance      string
	Description      string
}

// OS10VLTConfig represents a VLT (Virtual Link Trunking) configuration
// VLT is Dell's multi-chassis link aggregation technology, equivalent to MLAG
type OS10VLTConfig struct {
	DomainID             string
	DiscoveryInterface   string   // VLTi discovery interface
	PeerLink             string   // VLT peer-link port-channel
	BackupDestination    string   // VLT backup destination IP
	VLTMACAddress        string   // Virtual MAC for VLT
	DelayRestore         int      // Delay restore timer (seconds)
	PeerRouting          bool     // Peer-routing enabled
	PeerRoutingTimeout   int      // Peer-routing timeout
	BackupHeartbeatInterval int  // Backup heartbeat interval
	Enabled              bool
	VLANs                []int    // VLANs participating in VLT
	VLTIDs               map[int]int // interface -> VLT ID
}

// OS10StaticRoute represents a static route in OS10
type OS10StaticRoute struct {
	Prefix    string
	PrefixLen int
	NextHop   string
	Interface string
	Distance  int
	Tag       int
	Name      string
	VRF       string
	Raw       string
}

// OS10VRF represents a VRF configuration
type OS10VRF struct {
	Name              string
	RD                string
	RouteTargetImport []string
	RouteTargetExport []string
}

// OS10AccessGroup represents ACL binding to an interface
type OS10AccessGroup struct {
	ACLName   string
	Direction string // in / out
	Interface string
	ACLType   string // standard / extended
}

// OS10StandardACL represents a standard numbered ACL
type OS10StandardACL struct {
	Name  string
	Rules []OS10StandardACLRule
}

// OS10StandardACLRule is a single rule in a standard ACL
type OS10StandardACLRule struct {
	Order    int
	Action   string // permit / deny
	Source   string
	Wildcard string
	Log      bool
	Raw      string
}

// OS10ExtendedACL represents an extended ACL
type OS10ExtendedACL struct {
	Name  string
	Rules []OS10ExtendedACLRule
}

// OS10ExtendedACLRule is a single rule in an extended ACL
type OS10ExtendedACLRule struct {
	Order          int
	Action         string
	Protocol       string
	Source         string
	SourceWildcard string
	SourcePorts    []string
	Dest           string
	DestWildcard   string
	DestPorts      []string
	Established    bool
	Log            bool
	Raw            string
}

// OS10NamedACL represents a named ACL
type OS10NamedACL struct {
	Name        string
	Rules       []OS10ExtendedACLRule
	Description string
}

// OSPF structures for OS10
type OS10OSPFRouter struct {
	RouterID  string
	ProcessID int
	Networks  []OS10OSPFNetwork
	Areas     map[string]OS10OSPFArea
}

// OS10OSPFNetwork represents an OSPF network statement
type OS10OSPFNetwork struct {
	Network  string
	Wildcard string
	Area     string
}

// OS10OSPFArea represents an OSPF area
type OS10OSPFArea struct {
	ID       string
	Type     string // normal / stub / nssa
	Networks []string
}

// BGP structures for OS10
type OS10BGPConfig struct {
	ASN          int
	RouterID     string
	Peers        []OS10BGPPeer
	Networks     []string
	VRFs         []OS10BGPVRF
	MaximumPaths int
}

// OS10BGPPeer represents a BGP neighbor
type OS10BGPPeer struct {
	IP           string
	ASN          int
	Remote       string
	Multihop     int
	UpdateSource string
	PeerGroup    string
	Description  string
}

// OS10BGPVRF represents a BGP VRF address-family
type OS10BGPVRF struct {
	Name     string
	RD       string
	Peers    []OS10BGPPeer
	Networks []string
}

// ========================================================================
// Operational Data Structures
// ========================================================================

// OS10LLDPNeighbor represents an LLDP neighbor entry
type OS10LLDPNeighbor struct {
	LocalInterface string
	ChassisID      string
	PortID         string
	SystemName     string
	SystemDesc     string
	Capabilities   []string
	ManagementAddr string
	TTL            int
}

// OS10MACEntry represents a MAC address table entry
type OS10MACEntry struct {
	MAC          string
	VLANID       int
	Type         string // dynamic / static / secure / other
	Port         string
	LastMove     string
	CollectionTS string
}

// OS10ARPEntry represents an ARP table entry
type OS10ARPEntry struct {
	IP           string
	MAC          string
	Interface    string
	Type         string // dynamic / static
	AgingSeconds int
	VLAN         int
	CollectionTS string
}

// OS10RouteEntry represents a routing table entry from show ip route
type OS10RouteEntry struct {
	Prefix       string
	PrefixLen    int
	Protocol     string
	NextHop      string
	Interface    string
	Metric       int
	Distance     int
	Tag          string
	Age          string
	IsActive     bool
	IsBest       bool
	CollectionTS string
}

// OS10InterfaceStatus represents operational interface status
type OS10InterfaceStatus struct {
	Name        string
	Status      string // up / down / admin-down
	VLAN        string
	Duplex      string // full / half / auto
	Speed       string
	Type        string
	ErrorState  string
	Description string
}

// OS10VLTOperational represents VLT operational state
type OS10VLTOperational struct {
	DomainID          string
	State             string // active / inactive
	PeerStatus        string // connected / disconnected
	PeerLinkStatus    string // up / down
	Role              string // primary / secondary
	SystemMAC         string
	VLTMAC            string
	VLTUnitID         int
	Version           string
	LocalSystemMAC    string
	VLTiLinkStatus    string
	DelayRestore      int
	PeerRouting       bool
	LastConfigChange  string
}

// ========================================================================
// OS10 L2 Structures (matching CiscoL2Config pattern)
// ========================================================================

// OS10L2VLAN records VLAN metadata
type OS10L2VLAN struct {
	ID    int
	Name  string
	State string
}

// OS10L2Interface stores per-port switching posture
type OS10L2Interface struct {
	Name              string
	AdminState        string
	Mode              string
	AccessVLAN        *int
	TrunkAllowedVLANs []int
	TrunkNativeVLAN   *int
	PortChannelID     *int
	Description       string
}

// OS10PortChannelSummary summarizes a port-channel
type OS10PortChannelSummary struct {
	ID   int
	Name string
	Mode string
}

// OS10SVI marks VLAN interfaces for L3 handoff
type OS10SVI struct {
	VLANID        int
	InterfaceName string
	IPAddress     string
	Description   string
}

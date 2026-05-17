package models

import (
	"fmt"
)

// ========================================================================
// Arista EOS Data Models
// ========================================================================
// This file defines Arista EOS-specific configuration and operational data
// structures used by the EOS parsers and mappers.

// EOSConfig represents a parsed Arista EOS configuration
type EOSConfig struct {
	// Device Identity
	Hostname string
	Version  string
	Model    string
	Serial   string

	// Configuration Data
	Interfaces               []EOSInterface
	VLANs                    []EOSVLAN
	PortChannels             []EOSPortChannel
	MLAGConfig               *EOSMLAGConfig
	VXLANConfig              *EOSVXLANConfig
	StaticRoutes             []EOSStaticRoute
	OSPFRouter               *EOSOSPFRouter
	BGPConfig                *EOSBGPConfig
	VRFConfigs               []EOSVRF
	StandardACLs             []EOSStandardACL
	ExtendedACLs             []EOSExtendedACL
	NamedACLs                []EOSNamedACL
	AccessGroupBindings      []EOSAccessGroup
	RawConfig                string
}

// EOSInterface represents an Arista EOS interface configuration
type EOSInterface struct {
	Name         string
	Description  string
	IPAddress    string
	Netmask      string
	Subnet       string
	Shutdown     bool
	VLAN         int           // VLAN ID for subinterfaces / SVI
	AccessVLAN   int           // For switchport access
	TrunkVLANs   []int         // For switchport trunk
	TrunkNative  int           // Native VLAN for trunk
	Switchport   bool          // Is this a switchport (L2)
	SwitchportMode string      // access, trunk, routed
	LACPTimeout  string        // fast / slow
	LAGGroup     int           // Port-channel group (0 = none)
	LAGLACP      bool          // LACP mode active/passive
	Flowcontrol  string        // send / receive settings
	MTU          int
	LLDPTransmit bool
	LLDPReceive  bool
	StormControl string        // broadcast, multicast, unknown-unicast
}

// ValidateEOSInterface checks interface validity
func (i *EOSInterface) ValidateEOSInterface() error {
	if i.Name == "" {
		return fmt.Errorf("interface has no name")
	}
	return nil
}

// EOSVLAN represents a VLAN definition in EOS
type EOSVLAN struct {
	ID          int
	Name        string
	State       string // active / suspend
	TrunkGroups []string
	Raw         string
}

// EOSPortChannel represents a port-channel / LAG configuration
type EOSPortChannel struct {
	ID               int
	Name             string
	MemberInterfaces []string
	Mode             string // access / trunk / routed
	LACPTimeout      string
	MinimumLinks     int
	LoadBalance      string
	Description      string
}

// EOSMLAGConfig represents an MLAG (Multi-chassis Link Aggregation) configuration
type EOSMLAGConfig struct {
	DomainID         string
	LocalInterface   string   // MLAG keepalive local interface
	PeerAddress      string   // MLAG peer IP address
	PeerLink         string   // MLAG peer-link port-channel
	L3PeerLink       bool     // L3 peer-link
	L3SourceAddress  string
	BackupActive     bool
	AutoRecovery     bool
	ReloadDelay      int
	ConfigSanity     string   // check consistency
	HeartbeatTimeout int
	Enabled          bool
	VLANs            []int    // VLANs participating in MLAG
	MLAGIDs          map[int]int // interface -> MLAG ID
}

// EOSVXLANConfig represents VXLAN configuration
type EOSVXLANConfig struct {
	Enabled             bool
	SourceInterface     string
	SourceAddress       string
	FloodList           []string
	VNItoVLAN           map[int]int   // VNI -> VLAN mapping
	VLANtoVNI           map[int]int   // VLAN -> VNI mapping
	VNItoGroup          map[int]string // VNI -> multicast group
	VTEPAddress         string
	MACVRF              string
	LearningMode        string // data-plane / control-plane
	RD                  string
	RouteTargetImport   string
	RouteTargetExport   string
}

// EOSStaticRoute represents a static route in EOS
type EOSStaticRoute struct {
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

// EOSVRF represents a VRF configuration
type EOSVRF struct {
	Name              string
	RD                string
	RouteTargetImport []string
	RouteTargetExport []string
}

// EOSAccessGroup represents ACL binding to an interface
type EOSAccessGroup struct {
	ACLName   string
	Direction string // in / out
	Interface string
	ACLType   string // standard / extended
}

// EOSStandardACL represents a standard numbered ACL
type EOSStandardACL struct {
	Name  string
	Rules []EOSStandardACLRule
}

type EOSStandardACLRule struct {
	Order    int
	Action   string // permit / deny
	Source   string
	Wildcard string
	Log      bool
	Raw      string
}

// EOSExtendedACL represents an extended ACL
type EOSExtendedACL struct {
	Name  string
	Rules []EOSExtendedACLRule
}

type EOSExtendedACLRule struct {
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

// EOSNamedACL represents a named ACL (EOS uses names for most ACLs)
type EOSNamedACL struct {
	Name        string
	Rules       []EOSExtendedACLRule
	Description string
}

// OSPF structures for EOS
type EOSOSPFRouter struct {
	RouterID  string
	ProcessID int
	Networks  []EOSOSPFNetwork
	Areas     map[string]EOSOSPFArea
}

type EOSOSPFNetwork struct {
	Network   string
	Wildcard  string
	Area      string
}

type EOSOSPFArea struct {
	ID       string
	Type     string // normal / stub / nssa
	Networks []string
}

// BGP structures for EOS
type EOSBGPConfig struct {
	ASN          int
	RouterID     string
	Peers        []EOSBGPPeer
	Networks     []string
	VRFs         []EOSBGPVRF
	MaximumPaths int
}

type EOSBGPPeer struct {
	IP      string
	ASN     int
	Remote  string
	Multihop int
	UpdateSource string
	PeerGroup    string
	Description  string
}

type EOSBGPVRF struct {
	Name     string
	RD       string
	Peers    []EOSBGPPeer
	Networks []string
}

// ========================================================================
// Operational Data Structures
// ========================================================================

// EOSLLDPNeighbor represents an LLDP neighbor entry
type EOSLLDPNeighbor struct {
	LocalInterface string
	ChassisID      string
	PortID         string
	SystemName     string
	SystemDesc     string
	Capabilities   []string
	ManagementAddr string
	TTL            int
}

// EOSMACEntry represents a MAC address table entry
type EOSMACEntry struct {
	MAC          string
	VLANID       int
	Type         string // dynamic / static / secure / other
	Port         string
	LastMove     string
	CollectionTS string
}

// EOSARPEntry represents an ARP table entry
type EOSARPEntry struct {
	IP           string
	MAC          string
	Interface    string
	Type         string // dynamic / static
	AgingSeconds int
	VLAN         int
	CollectionTS string
}

// EOSRouteEntry represents a routing table entry from show ip route
type EOSRouteEntry struct {
	Prefix      string
	PrefixLen   int
	Protocol    string
	NextHop     string
	Interface   string
	Metric      int
	Distance    int
	Tag         string
	Age         string
	IsActive    bool
	IsBest      bool
	CollectionTS string
}

// EOSInterfaceStatus represents operational interface status
type EOSInterfaceStatus struct {
	Name       string
	Status     string // connected / notconnect / disabled / errdisable
	VLAN       string
	Duplex     string // a-full / a-half / full / half
	Speed      string
	Type       string
	ErrorState string
	Description string
}

// EOSMLAGOperational represents MLAG operational state
type EOSMLAGOperational struct {
	DomainID          string
	State             string // active / inactive
	PeerStatus        string // connected / disconnected
	PeerLinkStatus    string // up / down
	PeerPrimaryState  string
	PeerSecondaryState string
	MLAGPortCount     int
	InactivePorts     int
	HedaAlternatePort string
	LastConfigChange  string
}

// EOSVXLANOperational represents VXLAN operational state
type EOSVXLANOperational struct {
	VTEPAddress     string
	VNI             int
	SourceInterface string
	FloodList       int
	VLANtoVNIMap    map[int]int
	OperationalVNI  int
	MACRouteCount   int
}

// ========================================================================
// EOS L2 Structures (matching CiscoL2Config pattern)
// ========================================================================

// EOSL2VLAN records VLAN metadata
type EOSL2VLAN struct {
	ID    int
	Name  string
	State string
}

// EOSL2Interface stores per-port switching posture
type EOSL2Interface struct {
	Name              string
	AdminState        string
	Mode              string
	AccessVLAN        *int
	TrunkAllowedVLANs []int
	TrunkNativeVLAN   *int
	PortChannelID     *int
	Description       string
}

// EOSPortChannelSummary summarizes a port-channel
type EOSPortChannelSummary struct {
	ID   int
	Name string
	Mode string
}

// EOSSVI marks VLAN interfaces for L3 handoff
type EOSSVI struct {
	VLANID        int
	InterfaceName string
	IPAddress     string
	Description   string
}

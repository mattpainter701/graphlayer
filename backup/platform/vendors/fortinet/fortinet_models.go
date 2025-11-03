package models

import "time"

// ========================================
// FortiGate Device Model
// ========================================

// FortiGateConfig represents the complete FortiGate device configuration
type FortiGateConfig struct {
	// Device Identity
	Hostname     string
	Version      string
	Build        int
	SerialNumber string
	Model        string

	// System Configuration
	VDOMMode      string // "split" or "multi-vdom"
	OperationMode string // "nat" or "transparent"
	HAMode        string // "standalone", "active-passive", "active-active"
	HAGroupID     int
	HAGroupName   string
	AdminUsers    int

	// Virtual Domains (multi-VDOM support)
	VDOMs         []FortiGateVDOMConfig

	// Collection Metadata
	CollectedAt   string
}

// FortiGateVDOMConfig represents configuration for a single VDOM
type FortiGateVDOMConfig struct {
	// VDOM Identity
	VDOMName       string
	VDOMEnabled    bool
	VClusterID     int
	VDOMComment    string

	// Network Configuration
	Interfaces     []FortiGateInterfaceConfig
	Zones          []FortiGateZoneConfig
	StaticRoutes   []FortiGateStaticRouteConfig
	BGPConfig      *FortiGateBGPConfiguration

	// Security Policies
	IPv4Policies   []FortiGateSecurityPolicy
	IPv6Policies   []FortiGateIPv6SecurityPolicy
	CentralSNAT    []FortiGateCentralSNATPolicy
	VIPs           []FortiGateVIPConfig        // DNAT
	IPPools        []FortiGateIPPoolConfig

	// Objects
	Addresses      []FortiGateAddressObject
	AddressGroups  []FortiGateAddressGroupObject
	Services       []FortiGateServiceObject
	ServiceGroups  []FortiGateServiceGroupObject

	// SD-WAN Configuration
	SDWANConfig    *FortiGateSDWANConfig

	// VPN Configuration
	IPsecPhase1    []FortiGateIPsecPhase1Config
	IPsecPhase2    []FortiGateIPsecPhase2Config

	// Operational Data
	InterfaceStatus []FortiGateInterfaceOperationalStatus
	RoutingTable    []FortiGateRouteEntry
	BGPNeighbors    []FortiGateBGPNeighborOperational
	VPNStatus       []FortiGateVPNOperationalStatus
	SDWANHealth     []FortiGateSDWANHealthStatus
}

// ========================================
// Network Configuration Models
// ========================================

// FortiGateInterfaceConfig represents an interface configuration
type FortiGateInterfaceConfig struct {
	Name            string
	Type            string // "physical", "vlan", "loopback", "tunnel", "aggregate"
	VDOM            string
	IPAddress       string
	Netmask         string
	Subnet          string // CIDR notation
	Alias           string
	Description     string
	Status          string // "up" or "down"
	Role            string // "lan", "wan", "dmz", "undefined"
	Mode            string // "static", "dhcp"
	AllowAccess     []string
	VlanID          int
	ParentInterface string // For VLAN interfaces
	Members         []string // For aggregate/redundant interfaces
	Speed           string
	Duplex          string
	MTU             int
	MTUOverride     bool
}

// FortiGateZoneConfig represents a security zone
type FortiGateZoneConfig struct {
	Name           string
	VDOM           string
	Interfaces     []string
	Description    string
	IntrazoneAllow bool // true = allow, false = deny
}

// FortiGateStaticRouteConfig represents a static route
type FortiGateStaticRouteConfig struct {
	SeqNum         int
	Enabled        bool
	Destination    string // CIDR notation
	Gateway        string
	Device         string
	Distance       int
	Weight         int
	Priority       int
	Comment        string
	VDOM           string
}

// FortiGateBGPConfiguration represents BGP configuration
type FortiGateBGPConfiguration struct {
	AS                    int
	RouterID              string
	VDOM                  string
	Neighbors             []FortiGateBGPNeighborConfig
	Networks              []FortiGateBGPNetworkConfig
	AggregateAddresses    []string
	RedistributeConnected bool
	RedistributeStatic    bool
	RedistributeOSPF      bool
}

// FortiGateBGPNeighborConfig represents a BGP neighbor configuration
type FortiGateBGPNeighborConfig struct {
	IP                     string
	RemoteAS               int
	Description            string
	UpdateSource           string
	NextHopSelf            bool
	RouteMapIn             string
	RouteMapOut            string
	PrefixListIn           string
	PrefixListOut          string
	MaximumPrefix          int
	MaximumPrefixThreshold int
}

// FortiGateBGPNetworkConfig represents a BGP network advertisement
type FortiGateBGPNetworkConfig struct {
	ID            int
	Prefix        string
	BackdoorRoute bool
	RouteMap      string
}

// ========================================
// Security Policy Models
// ========================================

// FortiGateSecurityPolicy represents an IPv4 firewall policy
type FortiGateSecurityPolicy struct {
	PolicyID         int
	Name             string
	UUID             string
	VDOM             string
	SourceInterfaces []string
	DestInterfaces   []string
	SourceAddresses  []string
	DestAddresses    []string
	Services         []string
	Action           string // "accept", "deny", "ipsec"
	Enabled          bool
	Schedule         string

	// NAT Configuration
	NATEnabled       bool
	NATType          string // "ippool", "source-nat"
	NATPoolNames     []string

	// Logging
	LogTraffic       string // "all", "utm", "disable"

	// Security Profiles
	AVProfile        string
	WebfilterProfile string
	IPSProfile       string
	ApplicationList  string
	VoIPProfile      string
	DNSFilterProfile string
	SSHFilterProfile string

	Comments         string
}

// FortiGateIPv6SecurityPolicy represents an IPv6 firewall policy
type FortiGateIPv6SecurityPolicy struct {
	PolicyID         int
	Name             string
	UUID             string
	VDOM             string
	SourceInterfaces []string
	DestInterfaces   []string
	SourceAddresses  []string
	DestAddresses    []string
	Services         []string
	Action           string
	Enabled          bool
	Schedule         string
	LogTraffic       string
	Comments         string
}

// FortiGateCentralSNATPolicy represents a Central SNAT policy
type FortiGateCentralSNATPolicy struct {
	PolicyID           int
	UUID               string
	VDOM               string
	SourceInterfaces   []string
	DestInterfaces     []string
	OriginalAddresses  []string
	DestAddresses      []string
	NATType            string // "ippool", "dynamic-ip-pool"
	PoolNames          []string
	NATIPPools         []string
	Enabled            bool
	Comments           string
}

// FortiGateVIPConfig represents a Virtual IP (DNAT) configuration
type FortiGateVIPConfig struct {
	Name             string
	UUID             string
	VDOM             string
	Type             string // "static-nat", "server-load-balance", "dns-translation"
	ExternalIP       string
	ExternalInterface string
	MappedIPs        []string
	Protocol         string // "tcp", "udp", "sctp", "icmp"
	ExternalPort     string
	MappedPort       string
	Comment          string
}

// FortiGateIPPoolConfig represents an IP pool for NAT
type FortiGateIPPoolConfig struct {
	Name     string
	UUID     string
	VDOM     string
	Type     string // "overload", "one-to-one", "fixed-port-range"
	StartIP  string
	EndIP    string
	Comments string
}

// ========================================
// Object Models
// ========================================

// FortiGateAddressObject represents an address object
type FortiGateAddressObject struct {
	Name               string
	UUID               string
	VDOM               string
	Type               string // "ipmask", "iprange", "fqdn", "geography", "wildcard"
	Subnet             string // For type=ipmask (CIDR)
	StartIP            string // For type=iprange
	EndIP              string // For type=iprange
	FQDN               string // For type=fqdn
	Country            string // For type=geography
	Wildcard           string // For type=wildcard
	Comment            string
	AssociatedInterface string
}

// FortiGateAddressGroupObject represents an address group
type FortiGateAddressGroupObject struct {
	Name    string
	UUID    string
	VDOM    string
	Members []string
	Comment string
}

// FortiGateServiceObject represents a service object
type FortiGateServiceObject struct {
	Name            string
	UUID            string
	VDOM            string
	Category        string // "General", "Web Access", "File Access", etc.
	Protocol        string // "TCP/UDP/SCTP", "ICMP", "ICMP6", "IP"
	TCPPortRanges   []PortRange
	UDPPortRanges   []PortRange
	SCTPPortRanges  []PortRange
	ICMPType        int
	ICMPCode        int
	ProtocolNumber  int
	Comment         string
}

// PortRange represents a source/destination port range
type PortRange struct {
	SourcePort      string
	DestinationPort string
}

// FortiGateServiceGroupObject represents a service group
type FortiGateServiceGroupObject struct {
	Name    string
	UUID    string
	VDOM    string
	Members []string
	Comment string
}

// ========================================
// SD-WAN Models
// ========================================

// FortiGateSDWANConfig represents SD-WAN configuration
type FortiGateSDWANConfig struct {
	Enabled         bool
	LoadBalanceMode string // "source-ip-based", "weight-based", "usage-based", "source-dest-ip-based"
	Members         []FortiGateSDWANMember
	Zones           []FortiGateSDWANZone
	HealthChecks    []FortiGateSDWANHealthCheck
	Services        []FortiGateSDWANService
}

// FortiGateSDWANMember represents an SD-WAN member interface
type FortiGateSDWANMember struct {
	SeqNum          int
	Interface       string
	Zone            string
	Gateway         string
	SourceIPv4      string
	GatewayIPv6     string
	SourceIPv6      string
	Cost            int
	Weight          int
	Priority        int
	Enabled         bool
	Comment         string
}

// FortiGateSDWANZone represents an SD-WAN zone
type FortiGateSDWANZone struct {
	Name string
}

// FortiGateSDWANHealthCheck represents an SD-WAN health check configuration
type FortiGateSDWANHealthCheck struct {
	Name             string
	ProbePackets     bool
	AddrMode         string // "ipv4", "ipv6"
	UseSystemDNS     bool
	Server           string
	Protocol         string // "ping", "tcp-echo", "udp-echo", "http", "https", "twamp", "dns"
	Port             int
	Interval         int
	ProbeTimeout     int
	FailTime         int
	RecoveryTime     int
	ProbeCount       int
	SecurityMode     string
	MemberSeqNums    []int
}

// FortiGateSDWANService represents an SD-WAN service rule
type FortiGateSDWANService struct {
	ID                    int
	Name                  string
	Mode                  string // "manual", "priority", "sla", "load-balance"
	MinimumSLAMeetMembers int
	DestAddresses         []string
	SourceAddresses       []string
	Users                 []string
	Groups                []string
	Protocol              int
	StartPort             int
	EndPort               int
	PriorityMembers       []int
	Enabled               bool
	Comment               string
}

// FortiGateSDWANHealthStatus represents SD-WAN health check operational status
type FortiGateSDWANHealthStatus struct {
	Name    string
	Members []FortiGateSDWANMemberHealth
}

// FortiGateSDWANMemberHealth represents health status of an SD-WAN member
type FortiGateSDWANMemberHealth struct {
	SeqNum       int
	Interface    string
	State        string // "alive", "dead"
	PacketSent   int
	PacketLost   int
	PacketRecv   int
	Latency      float64
	Jitter       float64
	PacketLoss   float64
	SLAUptime    float64
}

// ========================================
// VPN Models
// ========================================

// FortiGateIPsecPhase1Config represents IPsec Phase 1 (IKE) configuration
type FortiGateIPsecPhase1Config struct {
	Name              string
	Type              string // "static", "dynamic", "ddns"
	Interface         string
	LocalGateway      string
	RemoteGateway     string
	PeerType          string // "any", "one", "dialup", "peer", "peergrp"
	Proposals         []string
	DHGroups          []int
	AuthMethod        string // "psk", "signature"
	PSKSecret         string // Masked/hashed
	Certificates      []string
	IKEVersion        int
	Mode              string // "main", "aggressive"
	Peergroup         string
	NetDevice         bool
	AddRoute          bool
	NATTraversal      string // "enable", "disable", "forced"
	DPD               string // "disable", "on-idle", "on-demand"
	DPDRetryCount     int
	DPDRetryInterval  int
	Comments          string
	VDOM              string
}

// FortiGateIPsecPhase2Config represents IPsec Phase 2 configuration
type FortiGateIPsecPhase2Config struct {
	Name               string
	Phase1Name         string
	Proposals          []string
	DHGroups           []int
	ReplayDetection    bool
	PFS                bool
	KeylifeSeconds     int
	KeylifeKilobytes   int
	SourceAddrType     string // "subnet", "range", "ip", "name"
	DestAddrType       string // "subnet", "range", "ip", "name"
	SourceSubnets      []string
	DestSubnets        []string
	Protocol           int
	SourcePort         int
	DestPort           int
	Comments           string
	VDOM               string
}

// FortiGateVPNOperationalStatus represents IPsec VPN tunnel operational status
type FortiGateVPNOperationalStatus struct {
	Name           string
	Type           string // "dialup", "static"
	RemoteGateway  string
	LocalGateway   string
	Status         string // "up", "down"
	Established    time.Time
	TxBytes        int64
	RxBytes        int64
	TxPackets      int64
	RxPackets      int64
	IncomingSPI    string
	OutgoingSPI    string
	VDOM           string
}

// ========================================
// Operational Status Models
// ========================================

// FortiGateInterfaceOperationalStatus represents interface operational status
type FortiGateInterfaceOperationalStatus struct {
	Name          string
	Status        string // "up" or "down"
	Link          string // "up" or "down"
	Speed         int    // Mbps
	Duplex        string // "full" or "half"
	TxPackets     int64
	RxPackets     int64
	TxBytes       int64
	RxBytes       int64
	TxErrors      int64
	RxErrors      int64
	TxDropped     int64
	RxDropped     int64
	ParentInterface string
	VDOM          string
}

// FortiGateRouteEntry represents a routing table entry
type FortiGateRouteEntry struct {
	Type           string // "connected", "static", "rip", "ospf", "bgp", "isis"
	Destination    string // CIDR notation
	Gateway        string
	Distance       int
	Metric         int
	Age            int
	Interface      string
	VDOM           string
}

// FortiGateBGPNeighborOperational represents BGP neighbor operational status
type FortiGateBGPNeighborOperational struct {
	NeighborIP       string
	RemoteAS         int
	State            string // "Established", "Active", "Idle", "Connect", "OpenSent", "OpenConfirm"
	UptimeSeconds    int
	PrefixReceived   int
	PrefixAccepted   int
	PrefixAdvertised int
	LocalIP          string
	LocalAS          int
	UpdateSource     string
	VDOM             string
}

// ========================================
// FortiManager Models
// ========================================

// FortiManagerConfig represents FortiManager centralized management configuration
type FortiManagerConfig struct {
	// FortiManager Identity
	Hostname      string
	Version       string
	SerialNumber  string

	// Administrative Domains (ADOMs)
	ADOMs         []FortiManagerADOM

	// Collection Metadata
	CollectedAt   string
}

// FortiManagerADOM represents an Administrative Domain in FortiManager
type FortiManagerADOM struct {
	Name          string
	Description   string
	Mode          string // "normal", "backup"
	OSVersion     string // Minimum FortiOS version
	MRVersion     int    // Maintenance release version

	// Managed Devices
	Devices       []FortiManagerManagedDevice

	// Policy Packages
	Packages      []FortiManagerPolicyPackage

	// Global Objects (shared across ADOM)
	GlobalAddresses      []FortiGateAddressObject
	GlobalAddressGroups  []FortiGateAddressGroupObject
	GlobalServices       []FortiGateServiceObject
	GlobalServiceGroups  []FortiGateServiceGroupObject
}

// FortiManagerManagedDevice represents a FortiGate managed by FortiManager
type FortiManagerManagedDevice struct {
	Name            string
	SerialNumber    string
	IPAddress       string
	Model           string
	Version         string
	Connected       bool
	HAMode          string
	VDOMMode        string
	ADOM            string
	DeviceGroup     []string
	InstallStatus   string // "installed", "pending", "modified"
	LastConnection  time.Time
}

// FortiManagerPolicyPackage represents a policy package in FortiManager
type FortiManagerPolicyPackage struct {
	Name          string
	Type          string // "pkg", "folder"
	Description   string
	ADOM          string
	Scope         []FortiManagerScope
	Policies      []FortiGateSecurityPolicy
	InstallTargets []string // Device names
}

// FortiManagerScope represents the scope of a policy package
type FortiManagerScope struct {
	Name string
	VDOM string
}

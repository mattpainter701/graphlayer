package aruba

// Aruba Vendor Models
// Supports AOS-CX and AOS-S (ProCurve) platforms
//
// OS Version Support:
// - AOS-CX 10.04+
// - AOS-CX 10.06+
// - AOS-CX 10.08+
// - AOS-CX 10.10+
// - AOS-CX 10.12+ (latest as of 2025)
// - AOS-S 16.x (ProCurve)
//
// Platform Support:
// - AOS-CX: 6200, 6300, 6400, 8320, 8325, 8360, 9300, 10000 series
// - AOS-S: 2530, 2540, 2920, 2930F, 2930M, 3810, 5400R series

import "time"

// ArubaDeviceInfo contains platform and version information
type ArubaDeviceInfo struct {
	Hostname     string
	DeviceID     string
	Platform     string // "aos-cx", "aos-s"
	Model        string // "6300", "8360", "2930F", etc.
	OSVersion    string
	SerialNumber string
	ConfigHash   string
}

// ArubaL2Config represents the complete L2 configuration for an Aruba device
type ArubaL2Config struct {
	Device         ArubaDeviceInfo
	VLANs          []ArubaVLAN
	Interfaces     []ArubaInterface
	PortChannels   []ArubaPortChannel
	GlobalControls ArubaGlobalL2Controls
	SVIs           []ArubaSVI
	ParsedAt       time.Time
}

// ArubaVLAN represents a VLAN configuration
type ArubaVLAN struct {
	VLANID            int
	Name              string
	State             string // "active", "suspended"
	Description       string
	VACLName          string // VLAN ACL if applied
	SourceBlock       string // Config stanza identifier
	SourceSnippetHash string // SHA-256 of config block
}

// ArubaInterface represents a switchport configuration
type ArubaInterface struct {
	// Basic interface info
	InterfaceName string // AOS-CX: "1/1/1", AOS-S: "1"
	Description   string
	AdminState    string // "up", "down"
	Type          string // "ethernet", "lag"

	// Switchport configuration
	Mode              string   // "access", "trunk", "routed"
	AccessVLAN        int      // Access VLAN ID
	TrunkNativeVLAN   int      // Native VLAN for trunks
	TrunkAllowedVLANs []int    // Allowed VLANs on trunk
	VoiceVLAN         int      // Voice VLAN (if configured)

	// LAG membership
	PortChannelID int    // LAG ID (0 if not member)
	PortChannelName string // LAG name

	// L2 Security Controls (per-port)
	DHCPSnoopingTrust   bool
	ARPInspectionTrust  bool
	IPSourceGuard       bool
	PortSecurityEnabled bool
	PortSecurityMaxMAC  int
	Dot1XRequired       bool
	MABEnabled          bool
	UnauthVLAN          int
	PACLInName          string
	PACLOutName         string

	// Metadata
	SourceBlock       string
	SourceSnippetHash string
}

// ArubaPortChannel represents a LAG/port-channel
type ArubaPortChannel struct {
	PortChannelID   int
	PortChannelName string // "lag 1", "Trk1"
	MemberInterfaces []string
	Mode            string // "access", "trunk"
	AccessVLAN      int
	TrunkNativeVLAN int
	TrunkAllowedVLANs []int
	LACPMode        string // "active", "passive"
	SourceBlock     string
	SourceSnippetHash string
}

// ArubaGlobalL2Controls represents device-wide L2 security settings
type ArubaGlobalL2Controls struct {
	// DHCP Snooping
	DHCPSnoopingEnabled bool
	DHCPSnoopingVLANs   []int
	DHCPSnoopingOption82 bool

	// Dynamic ARP Inspection
	DAIEnabledVLANs []int
	DAIACLNameByVLAN map[string]string

	// IP Source Guard
	IPSGEnabled bool

	// 802.1X/MAC Auth
	Dot1XSystemAuthControl bool
	MABGlobalEnabled       bool
	RADIUSServers          []string

	// Port Security
	PortSecurityAging int // Aging time in seconds

	// Metadata
	SourceBlock       string
	SourceSnippetHash string
}

// ArubaSVI represents a Layer 3 SVI (interface vlan)
type ArubaSVI struct {
	VLANID            int
	InterfaceName     string // "vlan 10"
	IPAddress         string
	SubnetMask        string
	AdminState        string
	Description       string
	SourceBlock       string
	SourceSnippetHash string
}

// ArubaFDBEntry represents a MAC address table entry
type ArubaFDBEntry struct {
	MACAddress     string
	VLANID         int
	Port           string
	Type           string // "dynamic", "static", "secure"
	LastSeenTime   time.Time
	EvidenceSource string // "mac-address-table", "snmp"
	Confidence     float64
}

// ArubaLLDPNeighbor represents an LLDP neighbor
type ArubaLLDPNeighbor struct {
	LocalInterface   string
	RemoteDeviceID   string
	RemotePortID     string
	RemoteSystemName string
	RemoteSystemDesc string
	ChassisID        string
	Capabilities     []string
	ManagementIP     string
}

// ArubaDHCPSnoopingBinding represents a DHCP snooping binding entry
type ArubaDHCPSnoopingBinding struct {
	MACAddress string
	IPAddress  string
	VLANID     int
	Interface  string
	LeaseTime  int // Seconds
	BindingType string // "dynamic", "static"
}

// ArubaAuthSession represents an 802.1X or MAB authentication session
type ArubaAuthSession struct {
	Interface       string
	MACAddress      string
	Username        string
	AuthMethod      string // "dot1x", "mab"
	AuthState       string // "authenticated", "unauthenticated"
	VLANID          int
	SessionDuration int // Seconds
	AuthServer      string
}

// ArubaPortSecurity represents port security configuration and state
type ArubaPortSecurity struct {
	Interface       string
	Enabled         bool
	MaxMAC          int
	ViolationMode   string // "restrict", "shutdown", "protect"
	SecureMACs      []string
	ViolationCount  int
}

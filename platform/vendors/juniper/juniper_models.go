package juniper

// Juniper Vendor Models
// Supports Junos OS on EX, QFX, SRX platforms
//
// OS Version Support:
// - Junos 15.1+ (minimum supported)
// - Junos 17.3+ (LTS)
// - Junos 18.4+ (LTS)
// - Junos 19.4+ (LTS)
// - Junos 20.4+ (LTS)
// - Junos 21.2+ (LTS)
// - Junos 23.2+ (latest LTS as of 2025)
//
// Platform Support:
// - EX Series: EX2300, EX3400, EX4300, EX4400, EX4600, EX4650
// - QFX Series: QFX5100, QFX5110, QFX5120, QFX5200, QFX5210
// - SRX Series: SRX300, SRX340, SRX345, SRX550M (when used as L2)

import "time"

// JuniperDeviceInfo contains platform and version information
type JuniperDeviceInfo struct {
	Hostname     string
	DeviceID     string
	Platform     string // "junos"
	Model        string // "EX4300", "QFX5100", etc.
	OSVersion    string
	SerialNumber string
	ConfigHash   string
}

// JuniperL2Config represents the complete L2 configuration
type JuniperL2Config struct {
	Device         JuniperDeviceInfo
	VLANs          []JuniperVLAN
	Interfaces     []JuniperInterface
	AggregatedEthernets []JuniperAE // Aggregated Ethernet (LAG)
	GlobalControls JuniperGlobalL2Controls
	IRBs           []JuniperIRB // Integrated Routing and Bridging
	ParsedAt       time.Time
}

// JuniperVLAN represents a VLAN configuration
type JuniperVLAN struct {
	VLANName          string // Junos uses names primarily
	VLANID            int
	Description       string
	L3Interface       string // IRB interface if configured
	SourceBlock       string
	SourceSnippetHash string
}

// JuniperInterface represents an interface configuration
type JuniperInterface struct {
	// Basic interface info
	InterfaceName string // "ge-0/0/0", "xe-0/1/0", "ae0"
	Description   string
	Unit          int    // Logical unit number (usually 0 for L2)
	Disabled      bool   // Administrative state

	// Switchport configuration (ethernet-switching family)
	PortMode          string   // "access", "trunk"
	AccessVLAN        string   // VLAN name (Junos uses names)
	TrunkNativeVLAN   string   // Native VLAN name
	TrunkAllowedVLANs []string // VLAN names

	// AE membership
	AggregatedEthernetID string // "ae0", "ae1", etc.

	// L2 Security Controls
	DHCPSnoopingTrusted bool
	ARPInspectionTrusted bool
	IPSourceGuardEnabled bool
	Dot1XEnabled        bool
	MABEnabled          bool
	GuestVLAN           string
	PortSecurityEnabled bool
	PortSecurityMaxMAC  int

	// Metadata
	SourceBlock       string
	SourceSnippetHash string
}

// JuniperAE represents an Aggregated Ethernet (LAG) interface
type JuniperAE struct {
	AEName            string   // "ae0", "ae1"
	Description       string
	Unit              int
	MemberInterfaces  []string // Physical interfaces
	LACPMode          string   // "active", "passive"
	PortMode          string   // "access", "trunk"
	AccessVLAN        string
	TrunkNativeVLAN   string
	TrunkAllowedVLANs []string
	SourceBlock       string
	SourceSnippetHash string
}

// JuniperGlobalL2Controls represents device-wide L2 security settings
type JuniperGlobalL2Controls struct {
	// DHCP Snooping (called "dhcp-security" in Junos)
	DHCPSecurityEnabled bool
	DHCPSecurityVLANs   []string

	// Dynamic ARP Inspection (called "arp-inspection" in Junos)
	ARPInspectionEnabled bool
	ARPInspectionVLANs   []string

	// IP Source Guard
	IPSourceGuardEnabled bool

	// 802.1X
	Dot1XEnabled    bool
	RADIUSServers   []string

	// Storm Control
	StormControlEnabled bool

	// Metadata
	SourceBlock       string
	SourceSnippetHash string
}

// JuniperIRB represents an Integrated Routing and Bridging interface (L3 SVI equivalent)
type JuniperIRB struct {
	IRBName           string // "irb.0", "irb.10"
	Unit              int
	VLANID            int
	VLANName          string
	IPAddress         string
	Description       string
	Disabled          bool
	SourceBlock       string
	SourceSnippetHash string
}

// JuniperFDBEntry represents a MAC address table entry
type JuniperFDBEntry struct {
	MACAddress        string
	VLANName          string
	InterfaceName     string
	IsStatic          bool
	SourceSnippetHash string
}

// JuniperLLDPNeighbor represents an LLDP neighbor
type JuniperLLDPNeighbor struct {
	LocalInterface            string
	RemoteChassisID           string
	RemotePort                string
	RemotePortDescription     string
	RemoteSystemName          string
	RemoteSystemDescription   string
	SourceSnippetHash         string
}

// JuniperDHCPBinding represents a DHCP snooping binding
type JuniperDHCPBinding struct {
	MACAddress        string
	IPAddress         string
	VLANName          string
	InterfaceName     string
	BindingType       string
	SourceSnippetHash string
}

// JuniperDot1XSession represents an 802.1X authentication session
type JuniperDot1XSession struct {
	InterfaceName        string
	SupplicantMAC        string
	AuthenticationState  string
	SessionTimeout       int
	ReauthEnabled        bool
	ReauthPeriod         int
	SourceSnippetHash    string
}

// JuniperInterfaceStatus represents operational interface status
type JuniperInterfaceStatus struct {
	InterfaceName     string
	AdminStatus       string
	OperStatus        string
	Description       string
	SourceSnippetHash string
}

// JuniperVLANOperational represents operational VLAN state
type JuniperVLANOperational struct {
	VLANName          string
	VLANID            int
	MemberInterfaces  []string
	SourceSnippetHash string
}

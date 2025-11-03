package paloalto

import (
	"encoding/xml"
	"fmt"
	"strings"
	"time"
)

// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |P|A|L|O| |A|L|T|O| |P|A|N|-|O|S| |P|A|R|S|E|R|S|
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
/*
 * PAN-OS XML parsers for configuration and operational data
 *
 * PAN-OS uses XML API for all configuration and operational commands
 * Key concepts:
 * - Virtual Systems (vsys) for multi-tenancy
 * - Security Zones (explicit trust/untrust/DMZ)
 * - App-ID for Layer 7 application identification
 * - Policy-based NAT (source + destination in same rule)
 */

// ========================================
// SYSTEM INFORMATION PARSERS
// ========================================

// PANOSSystemInfo represents PAN-OS system information
type PANOSSystemInfo struct {
	Hostname     string `xml:"hostname"`
	IPAddress    string `xml:"ip-address"`
	Netmask      string `xml:"netmask"`
	SerialNumber string `xml:"serial"`
	Model        string `xml:"model"`
	SWVersion    string `xml:"sw-version"`
	AppVersion   string `xml:"app-version"`
	Uptime       string `xml:"uptime"`
	DeviceName   string `xml:"devicename"`
}

// PANOSSystemInfoResponse wraps the system info response
type PANOSSystemInfoResponse struct {
	XMLName xml.Name         `xml:"response"`
	Status  string           `xml:"status,attr"`
	Result  struct {
		System PANOSSystemInfo `xml:"system"`
	} `xml:"result"`
}

// ParsePANOSSystemInfo parses system information from XML
func ParsePANOSSystemInfo(xmlData string) (*PANOSSystemInfo, error) {
	var response PANOSSystemInfoResponse
	err := xml.Unmarshal([]byte(xmlData), &response)
	if err != nil {
		return nil, fmt.Errorf("failed to parse PAN-OS system info XML: %w", err)
	}

	if response.Status != "success" {
		return nil, fmt.Errorf("PAN-OS API returned non-success status: %s", response.Status)
	}

	return &response.Result.System, nil
}

// ========================================
// VIRTUAL SYSTEM PARSERS
// ========================================

// PANOSVirtualSystem represents a PAN-OS virtual system
type PANOSVirtualSystem struct {
	Name        string   `xml:"name,attr"`
	DisplayName string   `xml:"display-name"`
	Description string   `xml:"description"`
	Interfaces  []string `xml:"import>network>interface>member"`
}

// PANOSVsysConfig holds all virtual systems
type PANOSVsysConfig struct {
	XMLName xml.Name `xml:"response"`
	Status  string   `xml:"status,attr"`
	Result  struct {
		Vsys struct {
			Entry []PANOSVirtualSystem `xml:"entry"`
		} `xml:"vsys"`
	} `xml:"result"`
}

// ParsePANOSVirtualSystems parses virtual systems configuration from XML
func ParsePANOSVirtualSystems(xmlData string) ([]PANOSVirtualSystem, error) {
	var config PANOSVsysConfig
	err := xml.Unmarshal([]byte(xmlData), &config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse PAN-OS vsys XML: %w", err)
	}

	if config.Status != "success" {
		return nil, fmt.Errorf("PAN-OS API returned non-success status: %s", config.Status)
	}

	return config.Result.Vsys.Entry, nil
}

// ExtractVsysNames extracts vsys names from ParsePANOSVirtualSystems output
// Phase 3.1.2: Used for dynamic vsys command generation
func ExtractVsysNames(xmlData string) ([]string, error) {
	vsysList, err := ParsePANOSVirtualSystems(xmlData)
	if err != nil {
		return nil, err
	}

	var vsysNames []string
	for _, vsys := range vsysList {
		if vsys.Name != "" {
			vsysNames = append(vsysNames, vsys.Name)
		}
	}

	// If no vsys found, default to vsys1 (standalone firewall mode)
	if len(vsysNames) == 0 {
		vsysNames = []string{"vsys1"}
	}

	return vsysNames, nil
}

// ========================================
// SECURITY ZONE PARSERS
// ========================================

// PANOSZone represents a PAN-OS security zone
type PANOSZone struct {
	Name                      string   `xml:"name,attr"`
	Network                   struct {
		Layer3 []string `xml:"layer3>member"`
		Layer2 []string `xml:"layer2>member"`
	} `xml:"network"`
	EnableUserIdentification  bool     `xml:"enable-user-identification"`
	UserAcl                   []string `xml:"user-acl>include-list>member"`
}

// PANOSZoneConfig holds zone configuration
type PANOSZoneConfig struct {
	XMLName xml.Name `xml:"response"`
	Status  string   `xml:"status,attr"`
	Result  struct {
		Zone struct {
			Entry []PANOSZone `xml:"entry"`
		} `xml:"zone"`
	} `xml:"result"`
}

// ParsePANOSZones parses security zones from XML
func ParsePANOSZones(xmlData string) ([]PANOSZone, error) {
	var config PANOSZoneConfig
	err := xml.Unmarshal([]byte(xmlData), &config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse PAN-OS zone XML: %w", err)
	}

	if config.Status != "success" {
		return nil, fmt.Errorf("PAN-OS API returned non-success status: %s", config.Status)
	}

	return config.Result.Zone.Entry, nil
}

// ========================================
// INTERFACE PARSERS
// ========================================

// PANOSInterface represents a PAN-OS network interface
type PANOSInterface struct {
	Name          string `xml:"name,attr"`
	Type          string // ethernet, aggregate-ethernet, vlan, loopback, tunnel
	Comment       string `xml:"comment"`
	// Layer 3 config
	Layer3        *PANOSLayer3Config `xml:"layer3"`
	// Layer 2 config
	Layer2        *PANOSLayer2Config `xml:"layer2"`
}

// PANOSLayer3Config represents Layer 3 interface configuration
type PANOSLayer3Config struct {
	IP            []PANOSIPConfig `xml:"ip>entry"`
	InterfaceManagementProfile string `xml:"interface-management-profile"`
	AdjustTCPMSS  bool            `xml:"adjust-tcp-mss>enable"`
}

// PANOSLayer2Config represents Layer 2 interface configuration
type PANOSLayer2Config struct {
	NetflowProfile string `xml:"netflow-profile"`
}

// PANOSIPConfig represents IP address configuration
type PANOSIPConfig struct {
	Name    string `xml:"name,attr"` // IP address with CIDR (e.g., "10.1.1.1/24")
}

// PANOSInterfaceConfig holds interface configuration
type PANOSInterfaceConfig struct {
	XMLName xml.Name `xml:"response"`
	Status  string   `xml:"status,attr"`
	Result  struct {
		Network struct {
			Interface struct {
				Ethernet []PANOSInterface `xml:"ethernet>entry"`
				Vlan     []PANOSInterface `xml:"vlan>entry"`
				Loopback []PANOSInterface `xml:"loopback>entry"`
				Tunnel   []PANOSInterface `xml:"tunnel>entry"`
			} `xml:"interface"`
		} `xml:"network"`
	} `xml:"result"`
}

// ParsePANOSInterfaces parses interfaces from XML
func ParsePANOSInterfaces(xmlData string) ([]PANOSInterface, error) {
	var config PANOSInterfaceConfig
	err := xml.Unmarshal([]byte(xmlData), &config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse PAN-OS interface XML: %w", err)
	}

	if config.Status != "success" {
		return nil, fmt.Errorf("PAN-OS API returned non-success status: %s", config.Status)
	}

	// Collect all interfaces
	var interfaces []PANOSInterface

	// Ethernet interfaces
	for _, iface := range config.Result.Network.Interface.Ethernet {
		iface.Type = "ethernet"
		interfaces = append(interfaces, iface)
	}

	// VLAN interfaces
	for _, iface := range config.Result.Network.Interface.Vlan {
		iface.Type = "vlan"
		interfaces = append(interfaces, iface)
	}

	// Loopback interfaces
	for _, iface := range config.Result.Network.Interface.Loopback {
		iface.Type = "loopback"
		interfaces = append(interfaces, iface)
	}

	// Tunnel interfaces
	for _, iface := range config.Result.Network.Interface.Tunnel {
		iface.Type = "tunnel"
		interfaces = append(interfaces, iface)
	}

	return interfaces, nil
}

// ========================================
// ADDRESS OBJECT PARSERS
// ========================================

// PANOSAddressObject represents a PAN-OS address object
type PANOSAddressObject struct {
	Name        string   `xml:"name,attr"`
	Description string   `xml:"description"`
	Type        string   // Inferred from which field is populated
	IPNetmask   string   `xml:"ip-netmask"`
	IPRange     string   `xml:"ip-range"`
	FQDN        string   `xml:"fqdn"`
	Tags        []string `xml:"tag>member"`
}

// PANOSAddressGroup represents a PAN-OS address group
type PANOSAddressGroup struct {
	Name        string   `xml:"name,attr"`
	Description string   `xml:"description"`
	Static      []string `xml:"static>member"`
	Dynamic     struct {
		Filter string `xml:"filter"`
	} `xml:"dynamic"`
	Tags        []string `xml:"tag>member"`
}

// PANOSAddressConfig holds address object configuration
type PANOSAddressConfig struct {
	XMLName xml.Name `xml:"response"`
	Status  string   `xml:"status,attr"`
	Result  struct {
		Address struct {
			Entry []PANOSAddressObject `xml:"entry"`
		} `xml:"address"`
		AddressGroup struct {
			Entry []PANOSAddressGroup `xml:"entry"`
		} `xml:"address-group"`
	} `xml:"result"`
}

// ParsePANOSAddressObjects parses address objects and groups from XML
func ParsePANOSAddressObjects(xmlData string) ([]PANOSAddressObject, []PANOSAddressGroup, error) {
	var config PANOSAddressConfig
	err := xml.Unmarshal([]byte(xmlData), &config)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse PAN-OS address XML: %w", err)
	}

	if config.Status != "success" {
		return nil, nil, fmt.Errorf("PAN-OS API returned non-success status: %s", config.Status)
	}

	// Infer type for address objects
	for i := range config.Result.Address.Entry {
		addr := &config.Result.Address.Entry[i]
		if addr.IPNetmask != "" {
			addr.Type = "ip-netmask"
		} else if addr.IPRange != "" {
			addr.Type = "ip-range"
		} else if addr.FQDN != "" {
			addr.Type = "fqdn"
		}
	}

	return config.Result.Address.Entry, config.Result.AddressGroup.Entry, nil
}

// ========================================
// SERVICE OBJECT PARSERS
// ========================================

// PANOSServiceObject represents a PAN-OS service object
type PANOSServiceObject struct {
	Name        string `xml:"name,attr"`
	Description string `xml:"description"`
	Protocol    struct {
		TCP *PANOSProtocolConfig `xml:"tcp"`
		UDP *PANOSProtocolConfig `xml:"udp"`
	} `xml:"protocol"`
	Tags        []string `xml:"tag>member"`
}

// PANOSProtocolConfig represents TCP/UDP protocol configuration
type PANOSProtocolConfig struct {
	Port       string `xml:"port"`       // e.g., "80", "8080-8090"
	SourcePort string `xml:"source-port"`
}

// PANOSServiceGroup represents a PAN-OS service group
type PANOSServiceGroup struct {
	Name        string   `xml:"name,attr"`
	Description string   `xml:"description"`
	Members     []string `xml:"members>member"`
	Tags        []string `xml:"tag>member"`
}

// PANOSServiceConfig holds service object configuration
type PANOSServiceConfig struct {
	XMLName xml.Name `xml:"response"`
	Status  string   `xml:"status,attr"`
	Result  struct {
		Service struct {
			Entry []PANOSServiceObject `xml:"entry"`
		} `xml:"service"`
		ServiceGroup struct {
			Entry []PANOSServiceGroup `xml:"entry"`
		} `xml:"service-group"`
	} `xml:"result"`
}

// ParsePANOSServiceObjects parses service objects and groups from XML
func ParsePANOSServiceObjects(xmlData string) ([]PANOSServiceObject, []PANOSServiceGroup, error) {
	var config PANOSServiceConfig
	err := xml.Unmarshal([]byte(xmlData), &config)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse PAN-OS service XML: %w", err)
	}

	if config.Status != "success" {
		return nil, nil, fmt.Errorf("PAN-OS API returned non-success status: %s", config.Status)
	}

	return config.Result.Service.Entry, config.Result.ServiceGroup.Entry, nil
}

// ========================================
// SECURITY POLICY PARSERS
// ========================================

// PANOSSecurityPolicy represents a PAN-OS security policy rule
type PANOSSecurityPolicy struct {
	Name                string   `xml:"name,attr"`
	UUID                string   `xml:"uuid,attr"`
	FromZones           []string `xml:"from>member"`
	ToZones             []string `xml:"to>member"`
	SourceAddresses     []string `xml:"source>member"`
	SourceUsers         []string `xml:"source-user>member"`
	HIPProfiles         []string `xml:"hip-profiles>member"`
	DestinationAddresses []string `xml:"destination>member"`
	Applications        []string `xml:"application>member"`
	Services            []string `xml:"service>member"`
	Categories          []string `xml:"category>member"`
	Action              string   `xml:"action"`
	LogSetting          string   `xml:"log-setting"`
	LogStart            string   `xml:"log-start"`
	LogEnd              string   `xml:"log-end"`
	Description         string   `xml:"description"`
	Tags                []string `xml:"tag>member"`
	Disabled            string   `xml:"disabled"` // "yes" or "no"
	RuleType            string   `xml:"rule-type"`
	ProfileSetting      *PANOSProfileSetting `xml:"profile-setting"`
}

// PANOSProfileSetting represents security profiles attached to a policy
type PANOSProfileSetting struct {
	Group    string `xml:"group>member"`
	Profiles *PANOSProfiles `xml:"profiles"`
}

// PANOSProfiles holds individual security profiles
type PANOSProfiles struct {
	UrlFiltering  []string `xml:"url-filtering>member"`
	DataFiltering []string `xml:"data-filtering>member"`
	FileBlocking  []string `xml:"file-blocking>member"`
	Virus         []string `xml:"virus>member"`
	Spyware       []string `xml:"spyware>member"`
	Vulnerability []string `xml:"vulnerability>member"`
	Wildfire      []string `xml:"wildfire-analysis>member"`
}

// PANOSSecurityPolicyConfig holds security policy configuration
type PANOSSecurityPolicyConfig struct {
	XMLName xml.Name `xml:"response"`
	Status  string   `xml:"status,attr"`
	Result  struct {
		Security struct {
			Rules struct {
				Entry []PANOSSecurityPolicy `xml:"entry"`
			} `xml:"rules"`
		} `xml:"security"`
	} `xml:"result"`
}

// ParsePANOSSecurityPolicies parses security policies from XML
func ParsePANOSSecurityPolicies(xmlData string) ([]PANOSSecurityPolicy, error) {
	var config PANOSSecurityPolicyConfig
	err := xml.Unmarshal([]byte(xmlData), &config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse PAN-OS security policy XML: %w", err)
	}

	if config.Status != "success" {
		return nil, fmt.Errorf("PAN-OS API returned non-success status: %s", config.Status)
	}

	return config.Result.Security.Rules.Entry, nil
}

// ========================================
// NAT POLICY PARSERS
// ========================================

// PANOSNATPolicy represents a PAN-OS NAT policy rule
type PANOSNATPolicy struct {
	Name                     string   `xml:"name,attr"`
	UUID                     string   `xml:"uuid,attr"`
	NATType                  string   `xml:"nat-type"` // ipv4, nat64, nptv6
	FromZones                []string `xml:"from>member"`
	ToZones                  []string `xml:"to>member"`
	ToInterface              string   `xml:"to-interface"`
	Service                  string   `xml:"service"`
	SourceAddresses          []string `xml:"source>member"`
	DestinationAddresses     []string `xml:"destination>member"`
	SourceTranslation        *PANOSSourceTranslation `xml:"source-translation"`
	DestinationTranslation   *PANOSDestTranslation   `xml:"destination-translation"`
	Description              string   `xml:"description"`
	Tags                     []string `xml:"tag>member"`
	Disabled                 string   `xml:"disabled"` // "yes" or "no"
}

// PANOSSourceTranslation represents source NAT translation
type PANOSSourceTranslation struct {
	DynamicIPAndPort *struct {
		InterfaceAddress *struct {
			Interface string `xml:"interface"`
			IP        string `xml:"ip"`
		} `xml:"interface-address"`
		TranslatedAddress []string `xml:"translated-address>member"`
	} `xml:"dynamic-ip-and-port"`
	DynamicIP *struct {
		TranslatedAddress []string `xml:"translated-address>member"`
		Fallback          *struct {
			TranslatedAddress []string `xml:"translated-address>member"`
		} `xml:"fallback"`
	} `xml:"dynamic-ip"`
	StaticIP *struct {
		TranslatedAddress string `xml:"translated-address"`
		BiDirectional     string `xml:"bi-directional"` // "yes" or "no"
	} `xml:"static-ip"`
}

// PANOSDestTranslation represents destination NAT translation
type PANOSDestTranslation struct {
	TranslatedAddress string `xml:"translated-address"`
	TranslatedPort    int    `xml:"translated-port"`
}

// PANOSNATPolicyConfig holds NAT policy configuration
type PANOSNATPolicyConfig struct {
	XMLName xml.Name `xml:"response"`
	Status  string   `xml:"status,attr"`
	Result  struct {
		NAT struct {
			Rules struct {
				Entry []PANOSNATPolicy `xml:"entry"`
			} `xml:"rules"`
		} `xml:"nat"`
	} `xml:"result"`
}

// ParsePANOSNATPolicies parses NAT policies from XML
func ParsePANOSNATPolicies(xmlData string) ([]PANOSNATPolicy, error) {
	var config PANOSNATPolicyConfig
	err := xml.Unmarshal([]byte(xmlData), &config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse PAN-OS NAT policy XML: %w", err)
	}

	if config.Status != "success" {
		return nil, fmt.Errorf("PAN-OS API returned non-success status: %s", config.Status)
	}

	return config.Result.NAT.Rules.Entry, nil
}

// ========================================
// VIRTUAL ROUTER PARSERS
// ========================================

// PANOSVirtualRouter represents a PAN-OS virtual router
type PANOSVirtualRouter struct {
	Name       string   `xml:"name,attr"`
	Interfaces []string `xml:"interface>member"`
	StaticRoutes []PANOSStaticRoute `xml:"routing-table>ip>static-route>entry"`
}

// PANOSStaticRoute represents a static route
type PANOSStaticRoute struct {
	Name        string `xml:"name,attr"`
	Destination string `xml:"destination"`
	Interface   string `xml:"interface"`
	NextHop     struct {
		IPAddress string `xml:"ip-address"`
	} `xml:"nexthop"`
	Metric      int    `xml:"metric"`
	AdminDistance int  `xml:"admin-distance"`
}

// PANOSVirtualRouterConfig holds virtual router configuration
type PANOSVirtualRouterConfig struct {
	XMLName xml.Name `xml:"response"`
	Status  string   `xml:"status,attr"`
	Result  struct {
		Network struct {
			VirtualRouter struct {
				Entry []PANOSVirtualRouter `xml:"entry"`
			} `xml:"virtual-router"`
		} `xml:"network"`
	} `xml:"result"`
}

// ParsePANOSVirtualRouters parses virtual routers from XML
func ParsePANOSVirtualRouters(xmlData string) ([]PANOSVirtualRouter, error) {
	var config PANOSVirtualRouterConfig
	err := xml.Unmarshal([]byte(xmlData), &config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse PAN-OS virtual router XML: %w", err)
	}

	if config.Status != "success" {
		return nil, fmt.Errorf("PAN-OS API returned non-success status: %s", config.Status)
	}

	return config.Result.Network.VirtualRouter.Entry, nil
}

// ========================================
// OPERATIONAL DATA PARSERS
// ========================================

// PANOSInterfaceStatus represents operational interface status
type PANOSInterfaceStatus struct {
	Name     string `xml:"name"`
	State    string `xml:"state"`    // up, down
	IP       string `xml:"ip"`
	MAC      string `xml:"mac"`
	Speed    string `xml:"speed"`
	Duplex   string `xml:"duplex"`
	Zone     string `xml:"zone"`
	VRouter  string `xml:"vsys"`
}

// PANOSARPEntry represents an ARP table entry
type PANOSARPEntry struct {
	Interface string `xml:"interface"`
	IPAddress string `xml:"ip"`
	MAC       string `xml:"mac"`
	TTL       int    `xml:"ttl"`
	Status    string `xml:"status"` // c (complete), i (incomplete)
}

// PANOSRouteEntry represents a routing table entry
type PANOSRouteEntry struct {
	VirtualRouter string `xml:"virtual-router"`
	Destination   string `xml:"destination"`
	NextHop       string `xml:"nexthop"`
	Interface     string `xml:"interface"`
	Metric        int    `xml:"metric"`
	Flags         string `xml:"flags"` // A (active), C (connected), S (static), etc.
	Age           string `xml:"age"`
	RouteTable    string `xml:"route-table"` // unicast, multicast
}

// PANOSHAStatus represents High Availability status
type PANOSHAStatus struct {
	Enabled      bool   `xml:"enabled"`
	Group        struct {
		Mode         string `xml:"mode"`          // Active-Passive, Active-Active
		LocalState   string `xml:"local-state"`   // active, passive, initial
		PeerState    string `xml:"peer-state"`
		RunningSync  string `xml:"running-sync"`  // synchronized, not-synchronized
		RunningSync Enabled bool `xml:"running-sync-enabled"`
	} `xml:"group"`
	LinkMonitoring struct {
		Enabled bool `xml:"enabled"`
		Groups  []struct {
			Name    string `xml:"name"`
			Enabled bool   `xml:"enabled"`
		} `xml:"group"`
	} `xml:"link-monitoring"`
	PathMonitoring struct {
		Enabled bool `xml:"enabled"`
		Groups  []struct {
			Name    string `xml:"name"`
			Enabled bool   `xml:"enabled"`
		} `xml:"group"`
	} `xml:"path-monitoring"`
}

// Utility functions for operational parsers

// ParsePANOSInterfaceStatus parses interface operational status
func ParsePANOSInterfaceStatus(xmlData string) ([]PANOSInterfaceStatus, error) {
	// Simplified parser - actual implementation would handle full XML structure
	var result struct {
		XMLName xml.Name `xml:"response"`
		Status  string   `xml:"status,attr"`
		Result  struct {
			IfInfo struct {
				Entry []PANOSInterfaceStatus `xml:"entry"`
			} `xml:"ifinfo"`
		} `xml:"result"`
	}

	err := xml.Unmarshal([]byte(xmlData), &result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse PAN-OS interface status XML: %w", err)
	}

	if result.Status != "success" {
		return nil, fmt.Errorf("PAN-OS API returned non-success status: %s", result.Status)
	}

	return result.Result.IfInfo.Entry, nil
}

// ParsePANOSARPTable parses ARP table
func ParsePANOSARPTable(xmlData string) ([]PANOSARPEntry, error) {
	var result struct {
		XMLName xml.Name `xml:"response"`
		Status  string   `xml:"status,attr"`
		Result  struct {
			Entries struct {
				Entry []PANOSARPEntry `xml:"entry"`
			} `xml:"entries"`
		} `xml:"result"`
	}

	err := xml.Unmarshal([]byte(xmlData), &result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse PAN-OS ARP table XML: %w", err)
	}

	if result.Status != "success" {
		return nil, fmt.Errorf("PAN-OS API returned non-success status: %s", result.Status)
	}

	return result.Result.Entries.Entry, nil
}

// ParsePANOSRoutingTable parses routing table
func ParsePANOSRoutingTable(xmlData string) ([]PANOSRouteEntry, error) {
	var result struct {
		XMLName xml.Name `xml:"response"`
		Status  string   `xml:"status,attr"`
		Result  struct {
			Routing struct {
				Entry []PANOSRouteEntry `xml:"entry"`
			} `xml:"routing"`
		} `xml:"result"`
	}

	err := xml.Unmarshal([]byte(xmlData), &result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse PAN-OS routing table XML: %w", err)
	}

	if result.Status != "success" {
		return nil, fmt.Errorf("PAN-OS API returned non-success status: %s", result.Status)
	}

	return result.Result.Routing.Entry, nil
}

// ========================================
// BGP/OSPF ROUTING PROTOCOL PARSERS
// Phase 3.1.3: Dynamic Routing Protocol Support
// ========================================

// PANOSBGPSummary represents BGP summary information
type PANOSBGPSummary struct {
	VirtualRouter string `xml:"virtual-router,attr"`
	LocalAS       string `xml:"local-as"`
	RouterID      string `xml:"router-id"`
	TotalPeers    int    `xml:"total-peers"`
	PeersUp       int    `xml:"peers-up"`
	PeersDown     int    `xml:"peers-down"`
}

// PANOSBGPNeighbor represents a BGP peer/neighbor
type PANOSBGPNeighbor struct {
	VirtualRouter     string `xml:"virtual-router,attr"`
	PeerName          string `xml:"peer-name"`
	PeerAddress       string `xml:"peer-address"`
	PeerAS            string `xml:"peer-as"`
	State             string `xml:"status"`                  // "Established", "Idle", "Active", "Connect"
	PrefixesReceived  int    `xml:"prefix-count-in"`
	PrefixesSent      int    `xml:"prefix-count-out"`
	Uptime            string `xml:"status-duration"`
	ConnectionState   string `xml:"status-flap-count"`
	LocalAddress      string `xml:"local-address"`
	ConnectionsDropped int   `xml:"connect-retry-count"`
}

// PANOSBGPRoute represents a BGP route (RIB-in or RIB-out)
type PANOSBGPRoute struct {
	VirtualRouter string `xml:"virtual-router,attr"`
	Prefix        string `xml:"prefix"`
	NextHop       string `xml:"nexthop"`
	Peer          string `xml:"peer"`
	ASPath        string `xml:"as-path"`
	Origin        string `xml:"origin"`        // "IGP", "EGP", "Incomplete"
	LocalPref     int    `xml:"local-pref"`
	MED           int    `xml:"med"`           // Multi-Exit Discriminator
	Community     string `xml:"community"`
	Valid         bool   `xml:"valid"`
	BestPath      bool   `xml:"best"`
}

// PANOSOSPFNeighbor represents an OSPF neighbor adjacency
type PANOSOSPFNeighbor struct {
	VirtualRouter string `xml:"virtual-router,attr"`
	NeighborID    string `xml:"neighbor-id"`       // Router ID
	Interface     string `xml:"interface"`
	IPAddress     string `xml:"address"`
	State         string `xml:"state"`             // "Full", "2-Way", "Exstart", "Exchange", "Loading"
	Priority      int    `xml:"priority"`
	DeadTime      string `xml:"dead-time"`
	AreaID        string `xml:"area-id"`
	DRAddress     string `xml:"dr"`                // Designated Router
	BDRAddress    string `xml:"bdr"`               // Backup Designated Router
}

// PANOSOSPFRoute represents an OSPF route in the RIB
type PANOSOSPFRoute struct {
	VirtualRouter string `xml:"virtual-router,attr"`
	Prefix        string `xml:"destination"`
	NextHop       string `xml:"nexthop"`
	RouteType     string `xml:"route-type"`        // "intra-area", "inter-area", "ext-1", "ext-2"
	Cost          int    `xml:"metric"`
	AreaID        string `xml:"area"`
	RouteAge      string `xml:"age"`
	RouteTag      string `xml:"tag,omitempty"`     // For external routes
}

// ParsePANOSBGPSummary parses BGP summary information
func ParsePANOSBGPSummary(xmlData string) (*PANOSBGPSummary, error) {
	var result struct {
		XMLName xml.Name `xml:"response"`
		Status  string   `xml:"status,attr"`
		Result  struct {
			BGP struct {
				Summary PANOSBGPSummary `xml:"summary"`
			} `xml:"bgp"`
		} `xml:"result"`
	}

	err := xml.Unmarshal([]byte(xmlData), &result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse PAN-OS BGP summary XML: %w", err)
	}

	if result.Status != "success" {
		return nil, fmt.Errorf("PAN-OS API returned non-success status: %s", result.Status)
	}

	return &result.Result.BGP.Summary, nil
}

// ParsePANOSBGPNeighbors parses BGP neighbor/peer information
func ParsePANOSBGPNeighbors(xmlData string) ([]PANOSBGPNeighbor, error) {
	var result struct {
		XMLName xml.Name `xml:"response"`
		Status  string   `xml:"status,attr"`
		Result  struct {
			BGP struct {
				Peer struct {
					Entry []PANOSBGPNeighbor `xml:"entry"`
				} `xml:"peer"`
			} `xml:"bgp"`
		} `xml:"result"`
	}

	err := xml.Unmarshal([]byte(xmlData), &result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse PAN-OS BGP neighbors XML: %w", err)
	}

	if result.Status != "success" {
		return nil, fmt.Errorf("PAN-OS API returned non-success status: %s", result.Status)
	}

	return result.Result.BGP.Peer.Entry, nil
}

// ParsePANOSBGPRoutes parses BGP routes (RIB-in or RIB-out)
func ParsePANOSBGPRoutes(xmlData string) ([]PANOSBGPRoute, error) {
	var result struct {
		XMLName xml.Name `xml:"response"`
		Status  string   `xml:"status,attr"`
		Result  struct {
			BGP struct {
				RIB struct {
					Entry []PANOSBGPRoute `xml:"entry"`
				} `xml:"rib"`
			} `xml:"bgp"`
		} `xml:"result"`
	}

	err := xml.Unmarshal([]byte(xmlData), &result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse PAN-OS BGP routes XML: %w", err)
	}

	if result.Status != "success" {
		return nil, fmt.Errorf("PAN-OS API returned non-success status: %s", result.Status)
	}

	return result.Result.BGP.RIB.Entry, nil
}

// ParsePANOSOSPFNeighbors parses OSPF neighbor adjacency information
func ParsePANOSOSPFNeighbors(xmlData string) ([]PANOSOSPFNeighbor, error) {
	var result struct {
		XMLName xml.Name `xml:"response"`
		Status  string   `xml:"status,attr"`
		Result  struct {
			OSPF struct {
				Neighbor struct {
					Entry []PANOSOSPFNeighbor `xml:"entry"`
				} `xml:"neighbor"`
			} `xml:"ospf"`
		} `xml:"result"`
	}

	err := xml.Unmarshal([]byte(xmlData), &result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse PAN-OS OSPF neighbors XML: %w", err)
	}

	if result.Status != "success" {
		return nil, fmt.Errorf("PAN-OS API returned non-success status: %s", result.Status)
	}

	return result.Result.OSPF.Neighbor.Entry, nil
}

// ParsePANOSOSPFRoutes parses OSPF routes from the RIB
func ParsePANOSOSPFRoutes(xmlData string) ([]PANOSOSPFRoute, error) {
	var result struct {
		XMLName xml.Name `xml:"response"`
		Status  string   `xml:"status,attr"`
		Result  struct {
			OSPF struct {
				RIB struct {
					Entry []PANOSOSPFRoute `xml:"entry"`
				} `xml:"rib"`
			} `xml:"ospf"`
		} `xml:"result"`
	}

	err := xml.Unmarshal([]byte(xmlData), &result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse PAN-OS OSPF routes XML: %w", err)
	}

	if result.Status != "success" {
		return nil, fmt.Errorf("PAN-OS API returned non-success status: %s", result.Status)
	}

	return result.Result.OSPF.RIB.Entry, nil
}

// ========================================
// VPN TUNNEL PARSERS
// Phase 3.1.4: IPsec and SSL VPN Mapping
// ========================================

// PANOSIPsecTunnel represents an IPsec tunnel configuration
type PANOSIPsecTunnel struct {
	Name                string           `xml:"name,attr"`
	TunnelInterface     string           `xml:"tunnel-interface"`
	AntiReplay          string           `xml:"anti-replay"`
	CopyTOS             string           `xml:"copy-tos"`
	IKEGateway          []string         `xml:"auto-key>ike-gateway>entry>name"`
	IPsecCryptoProfile  string           `xml:"auto-key>ipsec-crypto-profile"`
	ProxyIDs            []PANOSProxyID   `xml:"auto-key>proxy-id>entry"`
	TunnelMonitor       PANOSTunnelMonitor `xml:"tunnel-monitor"`
	Disabled            string           `xml:"disabled"` // "yes" or "no"
}

// PANOSProxyID represents IPsec proxy-id (traffic selectors)
type PANOSProxyID struct {
	Name     string `xml:"name,attr"`
	Local    string `xml:"local"`
	Remote   string `xml:"remote"`
	Protocol struct {
		Any    string `xml:"any,omitempty"`
		Number string `xml:"number,omitempty"`
		TCP    struct {
			LocalPort  string `xml:"local-port"`
			RemotePort string `xml:"remote-port"`
		} `xml:"tcp,omitempty"`
		UDP struct {
			LocalPort  string `xml:"local-port"`
			RemotePort string `xml:"remote-port"`
		} `xml:"udp,omitempty"`
	} `xml:"protocol"`
}

// PANOSTunnelMonitor represents tunnel monitoring configuration
type PANOSTunnelMonitor struct {
	Enable         string `xml:"enable"`
	DestinationIP  string `xml:"destination-ip"`
	ProxyID        string `xml:"proxy-id"`
}

// PANOSIKEGateway represents an IKE gateway configuration
type PANOSIKEGateway struct {
	Name             string `xml:"name,attr"`
	Version          string `xml:"protocol>version"` // "ikev1", "ikev2", "ikev2-preferred"
	Interface        string `xml:"local-address>interface"`
	LocalIPAddress   string `xml:"local-address>ip"`
	PeerIPAddress    string `xml:"peer-address>ip"`
	PeerFQDN         string `xml:"peer-address>fqdn,omitempty"`
	PreSharedKey     string `xml:"authentication>pre-shared-key>key"` // Should be redacted in logs
	LocalID          string `xml:"local-id>id"`
	PeerID           string `xml:"peer-id>id"`
	IKECryptoProfile string `xml:"protocol-common>ike-crypto-profile"`
	NAT              struct {
		Enable string `xml:"enable"`
	} `xml:"protocol-common>nat-traversal"`
	Fragmentation struct {
		Enable string `xml:"enable"`
	} `xml:"protocol-common>fragmentation"`
}

// PANOSIPsecTunnelStatus represents operational status of IPsec tunnel
type PANOSIPsecTunnelStatus struct {
	Name            string `xml:"name"`
	State           string `xml:"state"`           // "active", "inactive", "init"
	TunnelID        string `xml:"id"`
	LocalIP         string `xml:"localip"`
	PeerIP          string `xml:"peerip"`
	InnerInterface  string `xml:"inner-if"`
	OuterInterface  string `xml:"outer-if"`
	Gateway         string `xml:"gwid"`            // IKE gateway name
	BytesIn         string `xml:"bytes-in"`
	BytesOut        string `xml:"bytes-out"`
	PacketsIn       string `xml:"packets-in"`
	PacketsOut      string `xml:"packets-out"`
	TunnelUptime    string `xml:"tunnel-up-time,omitempty"`
	ESPSPIIn        string `xml:"esp-spi-in"`
	ESPSPIOut       string `xml:"esp-spi-out"`
}

// PANOSIKESAStatus represents operational status of IKE SA (Phase 1)
type PANOSIKESAStatus struct {
	Name          string `xml:"name"`
	Gateway       string `xml:"gateway-name"`
	LocalIP       string `xml:"localip"`
	PeerIP        string `xml:"peerip"`
	State         string `xml:"state"`           // "mature", "init"
	Version       string `xml:"ver"`             // "v1", "v2"
	LocalID       string `xml:"local-id"`
	PeerID        string `xml:"peer-id"`
	InitiatorSPI  string `xml:"initiator-spi"`
	ResponderSPI  string `xml:"responder-spi"`
	NATDetected   string `xml:"nat-detected"`    // "yes", "no"
	Lifetime      string `xml:"lifetime"`
}

// PANOSGlobalProtectUser represents an active SSL VPN user
type PANOSGlobalProtectUser struct {
	Username      string `xml:"username"`
	Domain        string `xml:"domain,omitempty"`
	Computer      string `xml:"computer"`
	ClientIP      string `xml:"virtual-ip"`
	PublicIP      string `xml:"public-ip"`
	LoginTime     string `xml:"login-time"`
	Gateway       string `xml:"gateway"`
	Region        string `xml:"region,omitempty"`
	AppVersion    string `xml:"app-version"`
	BytesIn       string `xml:"bytes-in"`
	BytesOut      string `xml:"bytes-out"`
}

// ParsePANOSIPsecTunnels parses IPsec tunnel configuration
func ParsePANOSIPsecTunnels(xmlData string) ([]PANOSIPsecTunnel, error) {
	var result struct {
		XMLName xml.Name `xml:"response"`
		Status  string   `xml:"status,attr"`
		Result  struct {
			IPsec struct {
				Entry []PANOSIPsecTunnel `xml:"entry"`
			} `xml:"ipsec"`
		} `xml:"result"`
	}

	err := xml.Unmarshal([]byte(xmlData), &result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse PAN-OS IPsec tunnel XML: %w", err)
	}

	if result.Status != "success" {
		return nil, fmt.Errorf("PAN-OS API returned non-success status: %s", result.Status)
	}

	return result.Result.IPsec.Entry, nil
}

// ParsePANOSIKEGateways parses IKE gateway configuration
func ParsePANOSIKEGateways(xmlData string) ([]PANOSIKEGateway, error) {
	var result struct {
		XMLName xml.Name `xml:"response"`
		Status  string   `xml:"status,attr"`
		Result  struct {
			Gateway struct {
				Entry []PANOSIKEGateway `xml:"entry"`
			} `xml:"gateway"`
		} `xml:"result"`
	}

	err := xml.Unmarshal([]byte(xmlData), &result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse PAN-OS IKE gateway XML: %w", err)
	}

	if result.Status != "success" {
		return nil, fmt.Errorf("PAN-OS API returned non-success status: %s", result.Status)
	}

	return result.Result.Gateway.Entry, nil
}

// ParsePANOSIPsecTunnelStatus parses operational status of IPsec tunnels
func ParsePANOSIPsecTunnelStatus(xmlData string) ([]PANOSIPsecTunnelStatus, error) {
	var result struct {
		XMLName xml.Name `xml:"response"`
		Status  string   `xml:"status,attr"`
		Result  struct {
			IPsec struct {
				Entry []PANOSIPsecTunnelStatus `xml:"entry"`
			} `xml:"ipsec"`
		} `xml:"result"`
	}

	err := xml.Unmarshal([]byte(xmlData), &result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse PAN-OS IPsec tunnel status XML: %w", err)
	}

	if result.Status != "success" {
		return nil, fmt.Errorf("PAN-OS API returned non-success status: %s", result.Status)
	}

	return result.Result.IPsec.Entry, nil
}

// ParsePANOSIKESAStatus parses operational status of IKE SAs
func ParsePANOSIKESAStatus(xmlData string) ([]PANOSIKESAStatus, error) {
	var result struct {
		XMLName xml.Name `xml:"response"`
		Status  string   `xml:"status,attr"`
		Result  struct {
			Entry []PANOSIKESAStatus `xml:"entry"`
		} `xml:"result"`
	}

	err := xml.Unmarshal([]byte(xmlData), &result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse PAN-OS IKE SA status XML: %w", err)
	}

	if result.Status != "success" {
		return nil, fmt.Errorf("PAN-OS API returned non-success status: %s", result.Status)
	}

	return result.Result.Entry, nil
}

// ParsePANOSGlobalProtectUsers parses active GlobalProtect SSL VPN users
func ParsePANOSGlobalProtectUsers(xmlData string) ([]PANOSGlobalProtectUser, error) {
	var result struct {
		XMLName xml.Name `xml:"response"`
		Status  string   `xml:"status,attr"`
		Result  struct {
			Entry []PANOSGlobalProtectUser `xml:"entry"`
		} `xml:"result"`
	}

	err := xml.Unmarshal([]byte(xmlData), &result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse PAN-OS GlobalProtect users XML: %w", err)
	}

	if result.Status != "success" {
		return nil, fmt.Errorf("PAN-OS API returned non-success status: %s", result.Status)
	}

	return result.Result.Entry, nil
}

// ========================================
// SECURITY PROFILE PARSERS
// Phase 3.1.5: Security Profile Mapping
// ========================================

// PANOSURLFilteringProfile represents a URL filtering profile
type PANOSURLFilteringProfile struct {
	Name        string   `xml:"name,attr"`
	Description string   `xml:"description,omitempty"`
	Block       []string `xml:"block>member"`
	Alert       []string `xml:"alert>member"`
	Allow       []string `xml:"allow>member"`
	Continue    []string `xml:"continue>member"`
	Override    []string `xml:"override>member,omitempty"`
}

// PANOSAntivirusProfile represents an antivirus profile
type PANOSAntivirusProfile struct {
	Name        string                  `xml:"name,attr"`
	Description string                  `xml:"description,omitempty"`
	Decoders    []PANOSAntivirusDecoder `xml:"decoder>entry"`
}

// PANOSAntivirusDecoder represents protocol decoder action for antivirus
type PANOSAntivirusDecoder struct {
	Name   string `xml:"name,attr"`      // http, smtp, ftp, smb, pop3, imap
	Action string `xml:"action"`         // default, allow, alert, drop, reset-both, reset-client, reset-server
}

// PANOSAntiSpywareProfile represents an anti-spyware profile
type PANOSAntiSpywareProfile struct {
	Name        string                   `xml:"name,attr"`
	Description string                   `xml:"description,omitempty"`
	Rules       []PANOSAntiSpywareRule   `xml:"rules>entry"`
}

// PANOSAntiSpywareRule represents anti-spyware rule
type PANOSAntiSpywareRule struct {
	Name     string   `xml:"name,attr"`
	Severity []string `xml:"severity>member"` // critical, high, medium, low, informational
	Category string   `xml:"category"`
	Action   struct {
		Default    string `xml:"default,omitempty"`
		Alert      string `xml:"alert,omitempty"`
		Drop       string `xml:"drop,omitempty"`
		ResetBoth  string `xml:"reset-both,omitempty"`
		ResetClient string `xml:"reset-client,omitempty"`
		ResetServer string `xml:"reset-server,omitempty"`
	} `xml:"action"`
	PacketCapture string `xml:"packet-capture,omitempty"` // disable, single-packet, extended-capture
}

// PANOSVulnerabilityProfile represents a vulnerability protection profile
type PANOSVulnerabilityProfile struct {
	Name        string                      `xml:"name,attr"`
	Description string                      `xml:"description,omitempty"`
	Rules       []PANOSVulnerabilityRule    `xml:"rules>entry"`
}

// PANOSVulnerabilityRule represents vulnerability protection rule
type PANOSVulnerabilityRule struct {
	Name       string   `xml:"name,attr"`
	Severity   []string `xml:"severity>member"`   // critical, high, medium, low, informational
	CVE        []string `xml:"cve>member,omitempty"`
	Vendor     []string `xml:"vendor-id>member,omitempty"`
	Category   string   `xml:"category"`
	HostType   string   `xml:"host"`              // client, server, any
	Action     struct {
		Default    string `xml:"default,omitempty"`
		Alert      string `xml:"alert,omitempty"`
		Drop       string `xml:"drop,omitempty"`
		ResetBoth  string `xml:"reset-both,omitempty"`
		ResetClient string `xml:"reset-client,omitempty"`
		ResetServer string `xml:"reset-server,omitempty"`
		BlockIP    struct {
			TrackBy  string `xml:"track-by"`      // source, source-and-destination
			Duration string `xml:"duration"`      // seconds
		} `xml:"block-ip,omitempty"`
	} `xml:"action"`
	PacketCapture string `xml:"packet-capture,omitempty"`
}

// PANOSFileBlockingProfile represents a file blocking profile
type PANOSFileBlockingProfile struct {
	Name        string                    `xml:"name,attr"`
	Description string                    `xml:"description,omitempty"`
	Rules       []PANOSFileBlockingRule   `xml:"rules>entry"`
}

// PANOSFileBlockingRule represents file blocking rule
type PANOSFileBlockingRule struct {
	Name         string   `xml:"name,attr"`
	Applications []string `xml:"application>member"`  // protocols: smtp, http, ftp, etc.
	FileTypes    []string `xml:"file-type>member"`    // PE, PDF, MS-Office, Flash, Java, etc.
	Direction    string   `xml:"direction"`           // upload, download, both
	Action       string   `xml:"action"`              // alert, block, continue
}

// ParsePANOSURLFilteringProfiles parses URL filtering profiles
func ParsePANOSURLFilteringProfiles(xmlData string) ([]PANOSURLFilteringProfile, error) {
	var result struct {
		XMLName xml.Name `xml:"response"`
		Status  string   `xml:"status,attr"`
		Result  struct {
			URLFiltering struct {
				Entry []PANOSURLFilteringProfile `xml:"entry"`
			} `xml:"url-filtering"`
		} `xml:"result"`
	}

	err := xml.Unmarshal([]byte(xmlData), &result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse PAN-OS URL filtering profiles XML: %w", err)
	}

	if result.Status != "success" {
		return nil, fmt.Errorf("PAN-OS API returned non-success status: %s", result.Status)
	}

	return result.Result.URLFiltering.Entry, nil
}

// ParsePANOSAntivirusProfiles parses antivirus profiles
func ParsePANOSAntivirusProfiles(xmlData string) ([]PANOSAntivirusProfile, error) {
	var result struct {
		XMLName xml.Name `xml:"response"`
		Status  string   `xml:"status,attr"`
		Result  struct {
			Virus struct {
				Entry []PANOSAntivirusProfile `xml:"entry"`
			} `xml:"virus"`
		} `xml:"result"`
	}

	err := xml.Unmarshal([]byte(xmlData), &result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse PAN-OS antivirus profiles XML: %w", err)
	}

	if result.Status != "success" {
		return nil, fmt.Errorf("PAN-OS API returned non-success status: %s", result.Status)
	}

	return result.Result.Virus.Entry, nil
}

// ParsePANOSAntiSpywareProfiles parses anti-spyware profiles
func ParsePANOSAntiSpywareProfiles(xmlData string) ([]PANOSAntiSpywareProfile, error) {
	var result struct {
		XMLName xml.Name `xml:"response"`
		Status  string   `xml:"status,attr"`
		Result  struct {
			Spyware struct {
				Entry []PANOSAntiSpywareProfile `xml:"entry"`
			} `xml:"spyware"`
		} `xml:"result"`
	}

	err := xml.Unmarshal([]byte(xmlData), &result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse PAN-OS anti-spyware profiles XML: %w", err)
	}

	if result.Status != "success" {
		return nil, fmt.Errorf("PAN-OS API returned non-success status: %s", result.Status)
	}

	return result.Result.Spyware.Entry, nil
}

// ParsePANOSVulnerabilityProfiles parses vulnerability protection profiles
func ParsePANOSVulnerabilityProfiles(xmlData string) ([]PANOSVulnerabilityProfile, error) {
	var result struct {
		XMLName xml.Name `xml:"response"`
		Status  string   `xml:"status,attr"`
		Result  struct {
			Vulnerability struct {
				Entry []PANOSVulnerabilityProfile `xml:"entry"`
			} `xml:"vulnerability"`
		} `xml:"result"`
	}

	err := xml.Unmarshal([]byte(xmlData), &result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse PAN-OS vulnerability profiles XML: %w", err)
	}

	if result.Status != "success" {
		return nil, fmt.Errorf("PAN-OS API returned non-success status: %s", result.Status)
	}

	return result.Result.Vulnerability.Entry, nil
}

// ParsePANOSFileBlockingProfiles parses file blocking profiles
func ParsePANOSFileBlockingProfiles(xmlData string) ([]PANOSFileBlockingProfile, error) {
	var result struct {
		XMLName xml.Name `xml:"response"`
		Status  string   `xml:"status,attr"`
		Result  struct {
			FileBlocking struct {
				Entry []PANOSFileBlockingProfile `xml:"entry"`
			} `xml:"file-blocking"`
		} `xml:"result"`
	}

	err := xml.Unmarshal([]byte(xmlData), &result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse PAN-OS file blocking profiles XML: %w", err)
	}

	if result.Status != "success" {
		return nil, fmt.Errorf("PAN-OS API returned non-success status: %s", result.Status)
	}

	return result.Result.FileBlocking.Entry, nil
}

// ParsePANOSHAStatus parses High Availability status
func ParsePANOSHAStatus(xmlData string) (*PANOSHAStatus, error) {
	var result struct{
		XMLName xml.Name `xml:"response"`
		Status  string   `xml:"status,attr"`
		Result  struct {
			HA PANOSHAStatus `xml:"ha"`
		} `xml:"result"`
	}

	err := xml.Unmarshal([]byte(xmlData), &result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse PAN-OS HA status XML: %w", err)
	}

	if result.Status != "success" {
		return nil, fmt.Errorf("PAN-OS API returned non-success status: %s", result.Status)
	}

	return &result.Result.HA, nil
}

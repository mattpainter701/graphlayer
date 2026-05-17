package cisco

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mattpainter701/graphlayer/platform/models"
)

// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |O|p|e|r|a|t|i|o|n|a|l| |C|o|m|m|a|n|d| |P|a|r|s|e|r|s|
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
/*
 * The following functions parse operational command outputs from Cisco IOS devices.
 * These supplement config-based parsing with runtime state for L2 topology visibility.
 */

// ===========================
// MAC Address Table Parsing
// ===========================

// ParseMACAddressTable parses "show mac address-table dynamic" output
// Format: Vlan    Mac Address       Type        Ports
//         10      0004.4d41.4e31    DYNAMIC     Gi1/0/1
func ParseMACAddressTable(output string, collectionTime time.Time) []models.CiscoFDBEntry {
	var entries []models.CiscoFDBEntry
	
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		// Skip headers and separators
		if strings.Contains(line, "Mac Address Table") ||
			strings.Contains(line, "---") ||
			strings.Contains(line, "Vlan") && strings.Contains(line, "Mac Address") ||
			strings.HasPrefix(line, "Total") ||
			line == "" {
			continue
		}
		
		// Parse: Vlan, MAC, Type, Ports
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		
		vlanStr := fields[0]
		macAddr := fields[1]
		entryType := strings.ToUpper(fields[2])
		port := fields[3]
		
		// Only process DYNAMIC entries
		if entryType != "DYNAMIC" {
			continue
		}
		
		vlanID, err := strconv.Atoi(vlanStr)
		if err != nil {
			continue
		}
		
		// Normalize MAC address (cisco format: 0004.4d41.4e31 -> 00:04:4D:41:4E:31)
		normalizedMAC := normalizeMACAddress(macAddr)
		
		entry := models.CiscoFDBEntry{
			MAC:            normalizedMAC,
			LastSeenPort:   port,
			VLANID:         vlanID,
			LastSeenTS:     collectionTime.Format(time.RFC3339),
			EvidenceSource: "fdb",
			Confidence:     1.0, // High confidence from switch FDB
		}
		
		entries = append(entries, entry)
	}
	
	return entries
}

// normalizeMACAddress converts Cisco MAC format to standard format
// 0004.4d41.4e31 -> 00:04:4D:41:4E:31
func normalizeMACAddress(ciscoMAC string) string {
	// Remove dots
	clean := strings.ReplaceAll(ciscoMAC, ".", "")
	
	// Add colons every 2 characters
	if len(clean) == 12 {
		return strings.ToUpper(
			clean[0:2] + ":" + clean[2:4] + ":" + clean[4:6] + ":" +
				clean[6:8] + ":" + clean[8:10] + ":" + clean[10:12],
		)
	}
	
	return strings.ToUpper(ciscoMAC)
}

// ===========================
// LLDP Neighbor Parsing
// ===========================

// LLDPNeighbor represents an LLDP neighbor discovery entry
type LLDPNeighbor struct {
	LocalInterface string
	ChassisID      string
	PortID         string
	SystemName     string
	SystemDesc     string
	Capabilities   []string
	ManagementAddr string
}

// ParseLLDPNeighbors parses "show lldp neighbors detail" output
func ParseLLDPNeighbors(output string) []LLDPNeighbor {
	var neighbors []LLDPNeighbor
	
	// Split by neighbor entries (look for "Local Intf:" or "Local Interface:")
	lines := strings.Split(output, "\n")
	
	var current *LLDPNeighbor
	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		if strings.HasPrefix(line, "Local Intf:") || strings.HasPrefix(line, "Local Interface:") {
			// Save previous neighbor
			if current != nil {
				neighbors = append(neighbors, *current)
			}
			
			// Start new neighbor
			current = &LLDPNeighbor{}
			
			// Extract local interface
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				current.LocalInterface = strings.TrimSpace(parts[1])
			}
		} else if current != nil {
			// Parse neighbor details
			if strings.HasPrefix(line, "Chassis id:") {
				current.ChassisID = extractValue(line, ":")
			} else if strings.HasPrefix(line, "Port id:") {
				current.PortID = extractValue(line, ":")
			} else if strings.HasPrefix(line, "System Name:") {
				current.SystemName = extractValue(line, ":")
			} else if strings.HasPrefix(line, "System Description:") {
				current.SystemDesc = extractValue(line, ":")
			} else if strings.HasPrefix(line, "System Capabilities:") {
				caps := extractValue(line, ":")
				current.Capabilities = strings.Split(caps, ",")
			} else if strings.HasPrefix(line, "Management Addresses:") {
				// Next line usually contains the IP
				current.ManagementAddr = extractValue(line, ":")
			}
		}
	}
	
	// Add last neighbor
	if current != nil {
		neighbors = append(neighbors, *current)
	}
	
	return neighbors
}

// ===========================
// CDP Neighbor Parsing
// ===========================

// CDPNeighbor represents a CDP neighbor discovery entry
type CDPNeighbor struct {
	DeviceID       string
	LocalInterface string
	Platform       string
	Capabilities   []string
	RemotePort     string
	IPAddress      string
}

// ParseCDPNeighbors parses "show cdp neighbors detail" output
func ParseCDPNeighbors(output string) []CDPNeighbor {
	var neighbors []CDPNeighbor
	
	// Split by device separator
	deviceSections := strings.Split(output, "-------------------------")
	
	for _, section := range deviceSections {
		if strings.TrimSpace(section) == "" {
			continue
		}
		
		neighbor := CDPNeighbor{}
		lines := strings.Split(section, "\n")
		
		for _, line := range lines {
			line = strings.TrimSpace(line)
			
			if strings.HasPrefix(line, "Device ID:") {
				neighbor.DeviceID = extractValue(line, ":")
			} else if strings.Contains(line, "Interface:") {
				// Format: "Interface: GigabitEthernet1/0/1,  Port ID (outgoing port): GigabitEthernet1/0/2"
				parts := strings.Split(line, ",")
				if len(parts) >= 1 {
					neighbor.LocalInterface = strings.TrimSpace(strings.TrimPrefix(parts[0], "Interface:"))
				}
				if len(parts) >= 2 && strings.Contains(parts[1], "Port ID") {
					portParts := strings.Split(parts[1], ":")
					if len(portParts) >= 2 {
						neighbor.RemotePort = strings.TrimSpace(portParts[1])
					}
				}
			} else if strings.HasPrefix(line, "Platform:") {
				neighbor.Platform = extractValue(line, ":")
			} else if strings.HasPrefix(line, "Capabilities:") {
				caps := extractValue(line, ":")
				neighbor.Capabilities = strings.Fields(caps)
			} else if strings.Contains(line, "IP address:") {
				neighbor.IPAddress = extractValue(line, ":")
			}
		}
		
		if neighbor.DeviceID != "" {
			neighbors = append(neighbors, neighbor)
		}
	}
	
	return neighbors
}

// ===========================
// Spanning Tree Parsing
// ===========================

// SpanningTreeVLAN represents STP state for a VLAN
type SpanningTreeVLAN struct {
	VLANID       int
	RootBridge   string
	RootPriority int
	RootCost     int
	LocalBridge  string
	Interfaces   []SpanningTreeInterface
}

// SpanningTreeInterface represents STP state for an interface
type SpanningTreeInterface struct {
	Name     string
	Role     string // Root, Designated, Alternate, Backup
	State    string // Forwarding, Blocking, Listening, Learning
	Cost     int
	Priority int
	PortType string // P2p, Shr, Edge
}

// ParseSpanningTree parses "show spanning-tree vlan X detail" output
func ParseSpanningTree(output string) []SpanningTreeVLAN {
	var vlans []SpanningTreeVLAN
	
	// Split by VLAN sections
	vlanSections := regexp.MustCompile(`VLAN\d+`).Split(output, -1)
	vlanHeaders := regexp.MustCompile(`VLAN\d+`).FindAllString(output, -1)
	
	for i, section := range vlanSections {
		if i == 0 || strings.TrimSpace(section) == "" {
			continue
		}
		
		vlan := SpanningTreeVLAN{}
		
		// Extract VLAN ID from header
		if i-1 < len(vlanHeaders) {
			vlanStr := strings.TrimPrefix(vlanHeaders[i-1], "VLAN")
			if vid, err := strconv.Atoi(vlanStr); err == nil {
				vlan.VLANID = vid
			}
		}
		
		lines := strings.Split(section, "\n")
		var currentInterface *SpanningTreeInterface
		
		for _, line := range lines {
			line = strings.TrimSpace(line)
			
			// Root bridge info
			if strings.Contains(line, "Root ID") {
				// Look for priority and address in following lines
			} else if strings.Contains(line, "Priority") && strings.Contains(line, "address") {
				// Extract root priority and address
				parts := strings.Fields(line)
				for i, part := range parts {
					if part == "Priority" && i+1 < len(parts) {
						if priority, err := strconv.Atoi(parts[i+1]); err == nil {
							vlan.RootPriority = priority
						}
					}
				}
			} else if strings.Contains(line, "Cost") && !strings.Contains(line, "Port") {
				// Root cost
				parts := strings.Fields(line)
				for i, part := range parts {
					if part == "Cost" && i+1 < len(parts) {
						if cost, err := strconv.Atoi(parts[i+1]); err == nil {
							vlan.RootCost = cost
						}
					}
				}
			} else if strings.HasPrefix(line, "Port") && strings.Contains(line, "(") {
				// Interface line: Port 257 (GigabitEthernet1/0/1) of VLAN0010 is forwarding
				if currentInterface != nil {
					vlan.Interfaces = append(vlan.Interfaces, *currentInterface)
				}
				
				currentInterface = &SpanningTreeInterface{}
				
				// Extract interface name
				re := regexp.MustCompile(`\(([^)]+)\)`)
				matches := re.FindStringSubmatch(line)
				if len(matches) >= 2 {
					currentInterface.Name = matches[1]
				}
				
				// Extract state
				if strings.Contains(line, "forwarding") {
					currentInterface.State = "Forwarding"
				} else if strings.Contains(line, "blocking") {
					currentInterface.State = "Blocking"
				} else if strings.Contains(line, "listening") {
					currentInterface.State = "Listening"
				} else if strings.Contains(line, "learning") {
					currentInterface.State = "Learning"
				}
			} else if currentInterface != nil && strings.Contains(line, "Port path cost") {
				// Extract cost
				parts := strings.Fields(line)
				for i, part := range parts {
					if part == "cost" && i+1 < len(parts) {
						cost := strings.TrimSuffix(parts[i+1], ",")
						if c, err := strconv.Atoi(cost); err == nil {
							currentInterface.Cost = c
						}
					}
				}
			} else if currentInterface != nil && (strings.Contains(line, "Port role") || strings.Contains(line, "role:")) {
				// Extract role
				if strings.Contains(strings.ToLower(line), "root") {
					currentInterface.Role = "Root"
				} else if strings.Contains(strings.ToLower(line), "designated") {
					currentInterface.Role = "Designated"
				} else if strings.Contains(strings.ToLower(line), "alternate") {
					currentInterface.Role = "Alternate"
				} else if strings.Contains(strings.ToLower(line), "backup") {
					currentInterface.Role = "Backup"
				}
			}
		}
		
		// Add last interface
		if currentInterface != nil && currentInterface.Name != "" {
			vlan.Interfaces = append(vlan.Interfaces, *currentInterface)
		}
		
		if vlan.VLANID > 0 {
			vlans = append(vlans, vlan)
		}
	}
	
	return vlans
}

// ===========================
// Interface Status Parsing
// ===========================

// InterfaceStatus represents operational status of an interface
type InterfaceStatus struct {
	Name        string
	Description string
	Status      string // connected, notconnect, disabled
	VLAN        string // VLAN ID or "trunk"
	Duplex      string
	Speed       string
	Type        string
}

// ParseInterfaceStatus parses "show interfaces status" output
func ParseInterfaceStatus(output string) []InterfaceStatus {
	var interfaces []InterfaceStatus
	
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		// Skip headers
		if strings.Contains(line, "Port") && strings.Contains(line, "Name") {
			continue
		}
		if line == "" || strings.Contains(line, "---") {
			continue
		}
		
		// Parse fixed-width format
		// Port      Name               Status       Vlan       Duplex  Speed Type
		// Gi1/0/1   RTU_MAIN_CTRL      connected    10         a-full  a-1000 10/100/1000BaseTX
		
		// Extract fields by position (approximate column widths)
		if len(line) < 30 {
			continue
		}
		
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}
		
		status := InterfaceStatus{
			Name:   fields[0],
			Status: fields[len(fields)-5],
			VLAN:   fields[len(fields)-4],
			Duplex: fields[len(fields)-3],
			Speed:  fields[len(fields)-2],
			Type:   fields[len(fields)-1],
		}
		
		// Description is everything between name and status
		if len(fields) > 6 {
			status.Description = strings.Join(fields[1:len(fields)-5], " ")
		}
		
		interfaces = append(interfaces, status)
	}
	
	return interfaces
}

// ===========================
// Interface Trunk Parsing
// ===========================

// TrunkInterface represents trunk configuration
type TrunkInterface struct {
	Port           string
	Mode           string
	Encapsulation  string
	Status         string
	NativeVLAN     int
	AllowedVLANs   []int
	ActiveVLANs    []int
	ForwardingVLANs []int
}

// ParseInterfacesTrunk parses "show interfaces trunk" output
func ParseInterfacesTrunk(output string) []TrunkInterface {
	var trunks []TrunkInterface
	trunkMap := make(map[string]*TrunkInterface)
	
	lines := strings.Split(output, "\n")
	section := ""
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		if line == "" {
			continue
		}
		
		// Detect sections
		if strings.Contains(line, "Port") && strings.Contains(line, "Mode") {
			section = "mode"
			continue
		} else if strings.Contains(line, "Vlans allowed on trunk") {
			section = "allowed"
			continue
		} else if strings.Contains(line, "Vlans allowed and active") {
			section = "active"
			continue
		} else if strings.Contains(line, "Vlans in spanning tree forwarding") {
			section = "forwarding"
			continue
		}
		
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		
		port := fields[0]
		
		// Create trunk entry if doesn't exist
		if _, exists := trunkMap[port]; !exists {
			trunkMap[port] = &TrunkInterface{Port: port}
		}
		
		trunk := trunkMap[port]
		
		switch section {
		case "mode":
			if len(fields) >= 4 {
				trunk.Mode = fields[1]
				trunk.Encapsulation = fields[2]
				trunk.Status = fields[3]
				if len(fields) >= 5 {
					if nv, err := strconv.Atoi(fields[4]); err == nil {
						trunk.NativeVLAN = nv
					}
				}
			}
		case "allowed":
			if len(fields) >= 2 {
				vlanStr := fields[1]
				// Handle "none" or VLAN list
				if vlanStr != "none" {
					trunk.AllowedVLANs = parseVLANList(vlanStr)
				}
			}
		case "active":
			if len(fields) >= 2 {
				vlanStr := fields[1]
				if vlanStr != "none" {
					trunk.ActiveVLANs = parseVLANList(vlanStr)
				}
			}
		case "forwarding":
			if len(fields) >= 2 {
				vlanStr := fields[1]
				if vlanStr != "none" {
					trunk.ForwardingVLANs = parseVLANList(vlanStr)
				}
			}
		}
	}
	
	// Convert map to slice
	for _, trunk := range trunkMap {
		trunks = append(trunks, *trunk)
	}
	
	return trunks
}

// parseVLANList parses VLAN list like "10,20,30,40,50,100,200,999"
func parseVLANList(vlanStr string) []int {
	var vlans []int
	
	parts := strings.Split(vlanStr, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		
		// Handle ranges like "10-20"
		if strings.Contains(part, "-") {
			rangeParts := strings.Split(part, "-")
			if len(rangeParts) == 2 {
				start, err1 := strconv.Atoi(strings.TrimSpace(rangeParts[0]))
				end, err2 := strconv.Atoi(strings.TrimSpace(rangeParts[1]))
				if err1 == nil && err2 == nil {
					for v := start; v <= end; v++ {
						vlans = append(vlans, v)
					}
				}
			}
		} else {
			// Single VLAN
			if vlan, err := strconv.Atoi(part); err == nil {
				vlans = append(vlans, vlan)
			}
		}
	}
	
	return vlans
}

// ===========================
// EtherChannel Parsing
// ===========================

// EtherChannel represents a port-channel
type EtherChannel struct {
	GroupNumber int
	PortChannel string
	Protocol    string
	Ports       []string
	Flags       string
}

// ParseEtherChannelSummary parses "show etherchannel summary" output
func ParseEtherChannelSummary(output string) []EtherChannel {
	var channels []EtherChannel
	
	lines := strings.Split(output, "\n")
	inTable := false
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		// Look for table header
		if strings.Contains(line, "Group") && strings.Contains(line, "Port-channel") {
			inTable = true
			continue
		}
		
		if !inTable || line == "" {
			continue
		}
		
		// Skip separator lines
		if strings.Contains(line, "---") || strings.Contains(line, "+++") {
			continue
		}
		
		// Parse: Group  Port-channel  Protocol    Ports
		//        10     Po10(SU)      LACP        Gi1/0/21(P)    Gi1/0/22(P)
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		
		channel := EtherChannel{}
		
		// Group number
		if group, err := strconv.Atoi(fields[0]); err == nil {
			channel.GroupNumber = group
		} else {
			continue
		}
		
		// Port-channel with flags: Po10(SU)
		poField := fields[1]
		if idx := strings.Index(poField, "("); idx > 0 {
			channel.PortChannel = poField[:idx]
			channel.Flags = strings.Trim(poField[idx:], "()")
		} else {
			channel.PortChannel = poField
		}
		
		// Protocol
		channel.Protocol = fields[2]
		
		// Member ports (rest of fields)
		for i := 3; i < len(fields); i++ {
			portField := fields[i]
			// Remove port flags: Gi1/0/21(P) -> Gi1/0/21
			if idx := strings.Index(portField, "("); idx > 0 {
				portField = portField[:idx]
			}
			channel.Ports = append(channel.Ports, portField)
		}
		
		channels = append(channels, channel)
	}
	
	return channels
}

// ===========================
// DHCP Snooping Binding Parsing
// ===========================

// DHCPSnoopingBinding represents a DHCP snooping binding entry
type DHCPSnoopingBinding struct {
	MACAddress string
	IPAddress  string
	Lease      int
	Type       string
	VLAN       int
	Interface  string
}

// ParseDHCPSnoopingBinding parses "show ip dhcp snooping binding" output
func ParseDHCPSnoopingBinding(output string) []DHCPSnoopingBinding {
	var bindings []DHCPSnoopingBinding
	
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		// Skip headers and separators
		if strings.Contains(line, "MacAddress") || strings.Contains(line, "---") || line == "" {
			continue
		}
		if strings.HasPrefix(line, "Total") {
			continue
		}
		
		// Parse: MacAddress          IpAddress        Lease(sec)  Type           VLAN  Interface
		//        00:04:4D:41:4E:31   10.10.10.15      86312       dhcp-snooping   10   GigabitEthernet1/0/1
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}
		
		binding := DHCPSnoopingBinding{
			MACAddress: fields[0],
			IPAddress:  fields[1],
			Type:       fields[3],
		}
		
		if lease, err := strconv.Atoi(fields[2]); err == nil {
			binding.Lease = lease
		}
		
		if vlan, err := strconv.Atoi(fields[4]); err == nil {
			binding.VLAN = vlan
		}
		
		binding.Interface = fields[5]
		
		bindings = append(bindings, binding)
	}
	
	return bindings
}

// ===========================
// ARP Inspection Parsing
// ===========================

// ARPInspectionConfig represents DAI configuration
type ARPInspectionConfig struct {
	SourceMACValidation bool
	DestMACValidation   bool
	IPValidation        bool
	VLANConfigs         []ARPInspectionVLAN
}

// ARPInspectionVLAN represents DAI config for a VLAN
type ARPInspectionVLAN struct {
	VLAN      int
	Config    string // Enabled/Disabled
	Operation string // Active/Inactive
	ACLMatch  string
	StaticACL string
}

// ParseARPInspection parses "show ip arp inspection" output
func ParseARPInspection(output string) ARPInspectionConfig {
	config := ARPInspectionConfig{}
	
	lines := strings.Split(output, "\n")
	section := ""
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		if line == "" {
			continue
		}
		
		// Global validation settings
		if strings.Contains(line, "Source Mac Validation") {
			config.SourceMACValidation = strings.Contains(line, "Enabled")
		} else if strings.Contains(line, "Destination Mac Validation") {
			config.DestMACValidation = strings.Contains(line, "Enabled")
		} else if strings.Contains(line, "IP Address Validation") {
			config.IPValidation = strings.Contains(line, "Enabled")
		}
		
		// Detect VLAN table section
		if strings.Contains(line, "Vlan") && strings.Contains(line, "Configuration") {
			section = "vlan_config"
			continue
		}
		
		// Parse VLAN entries
		if section == "vlan_config" {
			fields := strings.Fields(line)
			if len(fields) >= 4 {
				// Try to parse VLAN ID from first field
				if vlan, err := strconv.Atoi(fields[0]); err == nil {
					vlanConfig := ARPInspectionVLAN{
						VLAN:      vlan,
						Config:    fields[1],
						Operation: fields[2],
						ACLMatch:  fields[3],
					}
					if len(fields) >= 5 {
						vlanConfig.StaticACL = fields[4]
					}
					config.VLANConfigs = append(config.VLANConfigs, vlanConfig)
				}
			}
		}
	}
	
	return config
}

// ===========================
// Switchport Configuration Parsing
// ===========================

// SwitchportConfig represents switchport configuration
type SwitchportConfig struct {
	Name                  string
	Mode                  string // access, trunk, dynamic
	AdminMode             string
	OperationalMode       string
	AccessVLAN            int
	NativeVLAN            int
	TrunkingVLANs         []int
	VoiceVLAN             int
	EncapsulationType     string
	NegotiationOfTrunking string
}

// ParseInterfacesSwitchport parses "show interfaces switchport" output
func ParseInterfacesSwitchport(output string) []SwitchportConfig {
	var configs []SwitchportConfig
	
	lines := strings.Split(output, "\n")
	var current *SwitchportConfig
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		if line == "" {
			continue
		}
		
		// New interface section starts with "Name:"
		if strings.HasPrefix(line, "Name:") {
			// Save previous config
			if current != nil && current.Name != "" {
				configs = append(configs, *current)
			}
			
			// Start new config
			current = &SwitchportConfig{}
			current.Name = strings.TrimSpace(strings.TrimPrefix(line, "Name:"))
			
		} else if current != nil {
			// Parse configuration lines
			if strings.Contains(line, "Administrative Mode:") {
				current.AdminMode = extractValue(line, ":")
				if current.Mode == "" {
					current.Mode = current.AdminMode
				}
			} else if strings.Contains(line, "Operational Mode:") {
				current.OperationalMode = extractValue(line, ":")
			} else if strings.Contains(line, "Administrative Trunking Encapsulation:") {
				current.EncapsulationType = extractValue(line, ":")
			} else if strings.Contains(line, "Negotiation of Trunking:") {
				current.NegotiationOfTrunking = extractValue(line, ":")
			} else if strings.Contains(line, "Access Mode VLAN:") {
				vlanStr := extractValue(line, ":")
				// Extract number from "100 (VLAN0100)" format
				fields := strings.Fields(vlanStr)
				if len(fields) > 0 {
					if vlan, err := strconv.Atoi(fields[0]); err == nil {
						current.AccessVLAN = vlan
					}
				}
			} else if strings.Contains(line, "Trunking Native Mode VLAN:") {
				vlanStr := extractValue(line, ":")
				fields := strings.Fields(vlanStr)
				if len(fields) > 0 {
					if vlan, err := strconv.Atoi(fields[0]); err == nil {
						current.NativeVLAN = vlan
					}
				}
			} else if strings.Contains(line, "Trunking VLANs Enabled:") {
				vlanStr := extractValue(line, ":")
				if vlanStr != "ALL" && vlanStr != "" {
					current.TrunkingVLANs = parseVLANList(vlanStr)
				}
			} else if strings.Contains(line, "Voice VLAN:") {
				vlanStr := extractValue(line, ":")
				if vlanStr != "none" {
					fields := strings.Fields(vlanStr)
					if len(fields) > 0 {
						if vlan, err := strconv.Atoi(fields[0]); err == nil {
							current.VoiceVLAN = vlan
						}
					}
				}
			}
		}
	}
	
	// Add last config
	if current != nil && current.Name != "" {
		configs = append(configs, *current)
	}
	
	return configs
}

// ===========================
// Port Security Parsing
// ===========================

// PortSecurityEntry represents port security configuration
type PortSecurityEntry struct {
	Interface        string
	MaxSecureAddr    int
	CurrentAddr      int
	ViolationCount   int
	SecurityAction   string
}

// ParsePortSecurity parses "show port-security" output
func ParsePortSecurity(output string) []PortSecurityEntry {
	var entries []PortSecurityEntry
	
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		// Skip headers and separators
		if strings.Contains(line, "Secure Port") || strings.Contains(line, "---") || 
		   strings.Contains(line, "Total") || line == "" {
			continue
		}
		
		// Parse: Secure Port  MaxSecureAddr  CurrentAddr  SecurityViolation  Security Action
		//        Gi1/0/1              2            1                  0         Restrict
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		
		entry := PortSecurityEntry{
			Interface:      fields[0],
			SecurityAction: fields[len(fields)-1],
		}
		
		if max, err := strconv.Atoi(fields[1]); err == nil {
			entry.MaxSecureAddr = max
		}
		
		if curr, err := strconv.Atoi(fields[2]); err == nil {
			entry.CurrentAddr = curr
		}
		
		if viol, err := strconv.Atoi(fields[3]); err == nil {
			entry.ViolationCount = viol
		}
		
		entries = append(entries, entry)
	}
	
	return entries
}

// ===========================
// VLAN Parsing
// ===========================

// VLANEntry represents a VLAN catalog entry
type VLANEntry struct {
	ID     int
	Name   string
	Status string // active, suspended
	Ports  []string
}

// ParseVLAN parses "show vlan" output
func ParseVLAN(output string) []VLANEntry {
	var vlans []VLANEntry

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip headers and separators
		if strings.Contains(line, "VLAN Name") || strings.Contains(line, "---") ||
		   strings.Contains(line, "VLAN Type") || line == "" {
			continue
		}

		// Parse: VLAN   Name                             Status    Ports
		//        10     Engineering                      active    Gi1/0/1, Gi1/0/2
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		vlanID, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}

		vlan := VLANEntry{
			ID:     vlanID,
			Name:   fields[1],
			Status: fields[2],
		}

		// Parse ports (rest of the fields, comma-separated)
		if len(fields) > 3 {
			portsStr := strings.Join(fields[3:], " ")
			ports := strings.Split(portsStr, ",")
			for _, port := range ports {
				vlan.Ports = append(vlan.Ports, strings.TrimSpace(port))
			}
		}

		vlans = append(vlans, vlan)
	}

	return vlans
}

// ===========================
// IP Interface Brief Parsing
// ===========================

// IPInterfaceBrief represents an IP interface summary
type IPInterfaceBrief struct {
	Interface  string
	IPAddress  string
	OK         string // YES/NO
	Method     string // manual, DHCP, etc.
	Status     string // up, down, administratively down
	Protocol   string // up, down
}

// ParseIPInterfaceBrief parses "show ip interface brief" output
func ParseIPInterfaceBrief(output string) []IPInterfaceBrief {
	var interfaces []IPInterfaceBrief

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip headers and separators
		if strings.Contains(line, "Interface") && strings.Contains(line, "IP-Address") ||
		   line == "" {
			continue
		}

		// Parse: Interface              IP-Address      OK? Method Status                Protocol
		//        GigabitEthernet1/0/1   10.1.1.1        YES manual up                    up
		//        Vlan10                 10.0.10.1       YES manual up                    up
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}

		iface := IPInterfaceBrief{
			Interface: fields[0],
			IPAddress: fields[1],
			OK:        fields[2],
			Method:    fields[3],
			Status:    fields[4],
			Protocol:  fields[5],
		}

		interfaces = append(interfaces, iface)
	}

	return interfaces
}

// ===========================
// ARP Table Parsing
// ===========================

// ARPEntry represents an ARP table entry
type ARPEntry struct {
	Protocol  string
	IPAddress string
	Age       int // minutes, -1 for incomplete
	MACAddress string
	Type      string // ARPA, SNAP
	Interface string
}

// ParseIPARP parses "show ip arp" output
func ParseIPARP(output string) []ARPEntry {
	var entries []ARPEntry

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip headers and separators
		if strings.Contains(line, "Protocol") && strings.Contains(line, "Address") ||
		   line == "" {
			continue
		}

		// Parse: Protocol  Address          Age (min)  Hardware Addr   Type   Interface
		//        Internet  10.0.10.2             5   0050.56bb.0001  ARPA   Vlan10
		//        Internet  10.0.10.3             -   Incomplete      ARPA   Vlan10
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}

		entry := ARPEntry{
			Protocol:  fields[0],
			IPAddress: fields[1],
		}

		// Parse age
		ageStr := fields[2]
		if age, err := strconv.Atoi(ageStr); err == nil {
			entry.Age = age
		} else {
			entry.Age = -1 // Incomplete or other
		}

		// Handle "Incomplete" entries
		if fields[3] == "Incomplete" {
			entry.MACAddress = "Incomplete"
			entry.Type = fields[4]
			if len(fields) > 5 {
				entry.Interface = fields[5]
			}
		} else {
			entry.MACAddress = fields[3]
			entry.Type = fields[4]
			if len(fields) > 5 {
				entry.Interface = fields[5]
			}
		}

		entries = append(entries, entry)
	}

	return entries
}

// ===========================
// IGMP Snooping Groups Parsing
// ===========================

// IGMPSnoopingGroup represents an IGMP snooping multicast group
type IGMPSnoopingGroup struct {
	VLAN       int
	GroupAddress string
	Type       string // igmp, static
	Version    string // v1, v2, v3
	Port       string
}

// ParseIGMPSnoopingGroups parses "show ip igmp snooping groups" output
func ParseIGMPSnoopingGroups(output string) []IGMPSnoopingGroup {
	var groups []IGMPSnoopingGroup

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip headers and separators
		if strings.Contains(line, "Vlan") && strings.Contains(line, "Group") ||
		   strings.Contains(line, "---") || line == "" {
			continue
		}

		// Parse: Vlan      Group                    Type    Version Port
		//        10        239.1.1.1                igmp    v2      Gi1/0/1
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}

		vlanID, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}

		group := IGMPSnoopingGroup{
			VLAN:         vlanID,
			GroupAddress: fields[1],
			Type:         fields[2],
			Version:      fields[3],
			Port:         fields[4],
		}

		groups = append(groups, group)
	}

	return groups
}

// ===========================
// Version Parsing
// ===========================

// VersionInfo represents system version and hardware information
type VersionInfo struct {
	Version         string
	ImageFile       string
	Uptime          string
	SystemImage     string
	Model           string
	SerialNumber    string
	ProcessorType   string
	MemorySize      string
	ConfigRegister  string
}

// ParseVersion parses "show version" output
func ParseVersion(output string) *VersionInfo {
	info := &VersionInfo{}

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Parse version line
		if strings.Contains(line, "Cisco IOS Software") || strings.Contains(line, "IOS") && strings.Contains(line, "Version") {
			// Extract version from line like: "Cisco IOS Software, C3750 Software (C3750-IPSERVICESK9-M), Version 15.0(2)SE11"
			if idx := strings.Index(line, "Version "); idx != -1 {
				versionStr := line[idx+8:]
				if commaIdx := strings.Index(versionStr, ","); commaIdx != -1 {
					info.Version = versionStr[:commaIdx]
				} else {
					info.Version = strings.Fields(versionStr)[0]
				}
			}
		}

		// Parse uptime
		if strings.Contains(line, "uptime is") {
			if idx := strings.Index(line, "uptime is "); idx != -1 {
				info.Uptime = strings.TrimSpace(line[idx+10:])
			}
		}

		// Parse system image
		if strings.Contains(line, "System image file is") {
			if idx := strings.Index(line, "System image file is "); idx != -1 {
				info.SystemImage = strings.Trim(strings.TrimSpace(line[idx+21:]), "\"")
			}
		}

		// Parse model/processor
		if strings.Contains(line, "cisco") && (strings.Contains(line, "processor") || strings.Contains(line, "Processor")) {
			// Example: "cisco WS-C3750G-24TS (PowerPC405) processor (revision M0) with 131072K bytes"
			fields := strings.Fields(line)
			if len(fields) > 1 {
				info.Model = fields[1]
			}
			if len(fields) > 2 && strings.HasPrefix(fields[2], "(") {
				info.ProcessorType = strings.Trim(fields[2], "()")
			}
		}

		// Parse memory
		if strings.Contains(line, "bytes of memory") {
			fields := strings.Fields(line)
			for i, field := range fields {
				if strings.HasSuffix(field, "K") || strings.HasSuffix(field, "M") {
					info.MemorySize = field
					break
				}
				if field == "with" && i+1 < len(fields) {
					info.MemorySize = fields[i+1]
					break
				}
			}
		}

		// Parse serial number
		if strings.Contains(line, "Processor board ID") {
			if idx := strings.Index(line, "Processor board ID "); idx != -1 {
				info.SerialNumber = strings.TrimSpace(line[idx+19:])
			}
		} else if strings.Contains(line, "System serial number") {
			parts := strings.Split(line, ":")
			if len(parts) > 1 {
				info.SerialNumber = strings.TrimSpace(parts[1])
			}
		}

		// Parse config register
		if strings.Contains(line, "Configuration register is") {
			fields := strings.Fields(line)
			if len(fields) > 0 {
				info.ConfigRegister = fields[len(fields)-1]
			}
		}
	}

	return info
}

// ===========================
// L2 Security Control Parsers
// ===========================

// DHCPSnoopingBinding represents a DHCP snooping binding entry
type DHCPSnoopingBinding struct {
	MACAddress   string `json:"mac_address"`
	IPAddress    string `json:"ip_address"`
	LeaseSeconds int    `json:"lease_seconds"`
	Type         string `json:"type"` // dhcp-snooping, static
	VLAN         int    `json:"vlan"`
	Interface    string `json:"interface"`
}

// ParseDHCPSnoopingBinding parses "show ip dhcp snooping binding" output
// Sample output:
// MacAddress          IpAddress        Lease(sec)  Type           VLAN  Interface
// ------------------  ---------------  ----------  -------------  ----  --------------------
// 00:50:56:BB:00:01   10.0.10.2        86395       dhcp-snooping  10    GigabitEthernet1/0/1
// 00:50:56:BB:00:02   10.0.10.3        86395       dhcp-snooping  10    GigabitEthernet1/0/2
func ParseDHCPSnoopingBinding(output string) []DHCPSnoopingBinding {
	var bindings []DHCPSnoopingBinding
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "MacAddress") || strings.HasPrefix(trimmed, "--") {
			continue
		}

		fields := strings.Fields(trimmed)
		if len(fields) < 6 {
			continue
		}

		binding := DHCPSnoopingBinding{
			MACAddress: fields[0],
			IPAddress:  fields[1],
			Type:       fields[3],
			Interface:  fields[5],
		}

		// Parse lease time (seconds)
		if lease, err := strconv.Atoi(fields[2]); err == nil {
			binding.LeaseSeconds = lease
		}

		// Parse VLAN ID
		if vlan, err := strconv.Atoi(fields[4]); err == nil {
			binding.VLAN = vlan
		}

		bindings = append(bindings, binding)
	}

	return bindings
}

// ARPInspectionStatistics represents DAI statistics
type ARPInspectionStatistics struct {
	VLANConfig          map[int]string `json:"vlan_config"`          // VLAN -> enabled/disabled
	PacketsForwarded    int            `json:"packets_forwarded"`    // Total packets forwarded
	PacketsDropped      int            `json:"packets_dropped"`      // Total packets dropped
	InvalidSourceMAC    int            `json:"invalid_source_mac"`   // Dropped: invalid source MAC
	InvalidDestMAC      int            `json:"invalid_dest_mac"`     // Dropped: invalid dest MAC
	InvalidIP           int            `json:"invalid_ip"`           // Dropped: invalid IP
	DHCPSnoopingDenied  int            `json:"dhcp_snooping_denied"` // Dropped: DHCP snooping check failed
	ACLDenied           int            `json:"acl_denied"`           // Dropped: ACL denied
}

// ParseARPInspectionStatistics parses "show ip arp inspection statistics" output
// Sample output:
//  Vlan      Forwarded        Dropped     DHCP Drops      ACL Drops
//  ----      ---------        -------     ----------      ---------
//    10           1234            56            30              26
//    20           5678            12             8               4
func ParseARPInspectionStatistics(output string) *ARPInspectionStatistics {
	stats := &ARPInspectionStatistics{
		VLANConfig: make(map[int]string),
	}

	lines := strings.Split(output, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "Vlan") || strings.HasPrefix(trimmed, "--") {
			continue
		}

		fields := strings.Fields(trimmed)
		if len(fields) >= 5 {
			// Parse VLAN statistics
			vlan, err := strconv.Atoi(fields[0])
			if err != nil {
				continue
			}

			forwarded, _ := strconv.Atoi(fields[1])
			dropped, _ := strconv.Atoi(fields[2])
			dhcpDrops, _ := strconv.Atoi(fields[3])
			aclDrops, _ := strconv.Atoi(fields[4])

			stats.PacketsForwarded += forwarded
			stats.PacketsDropped += dropped
			stats.DHCPSnoopingDenied += dhcpDrops
			stats.ACLDenied += aclDrops
			stats.VLANConfig[vlan] = "enabled"
		}
	}

	return stats
}

// AuthenticationSession represents an 802.1X or MAB session
type AuthenticationSession struct {
	Interface      string `json:"interface"`
	MACAddress     string `json:"mac_address"`
	IPv4Address    string `json:"ipv4_address"`
	IPv6Address    string `json:"ipv6_address"`
	Status         string `json:"status"`   // Authc Success, Authc Failed, Running, etc.
	Domain         string `json:"domain"`   // DATA, VOICE
	Method         string `json:"method"`   // dot1x, mab
	SessionTimeout int    `json:"session_timeout"`
	VLAN           int    `json:"vlan"`
}

// ParseAuthenticationSessions parses "show authentication sessions" output
// Sample output:
// Interface  MAC Address     IP Address      Status         Domain  Method
// ---------- -------------- --------------- -------------- ------- -------
// Gi1/0/1    0050.56bb.0001  10.0.10.2       Authc Success  DATA    dot1x
// Gi1/0/2    0050.56bb.0002  10.0.20.3       Authc Success  DATA    mab
func ParseAuthenticationSessions(output string) []AuthenticationSession {
	var sessions []AuthenticationSession
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "Interface") || strings.HasPrefix(trimmed, "--") {
			continue
		}

		fields := strings.Fields(trimmed)
		if len(fields) < 6 {
			continue
		}

		session := AuthenticationSession{
			Interface:   fields[0],
			MACAddress:  fields[1],
			IPv4Address: fields[2],
			Domain:      fields[4],
			Method:      fields[5],
		}

		// Parse status (may be multi-word like "Authc Success")
		if len(fields) >= 7 {
			session.Status = fields[3] + " " + fields[4]
			session.Domain = fields[5]
			session.Method = fields[6]
		} else {
			session.Status = fields[3]
		}

		sessions = append(sessions, session)
	}

	return sessions
}

// PortSecurityStatus represents port security status
type PortSecurityStatus struct {
	Interface           string `json:"interface"`
	MaxMAC              int    `json:"max_mac"`
	CurrentMAC          int    `json:"current_mac"`
	SecurityViolation   string `json:"security_violation"` // Count, Mode (Protect/Restrict/Shutdown)
	ViolationCount      int    `json:"violation_count"`
	LastSourceAddress   string `json:"last_source_address"`
	LastSourceVLAN      int    `json:"last_source_vlan"`
	Status              string `json:"status"` // Enabled, Disabled
	ViolationMode       string `json:"violation_mode"` // Protect, Restrict, Shutdown
}

// ParsePortSecurity parses "show port-security" output
// Sample output:
// Secure Port  MaxSecureAddr  CurrentAddr  SecurityViolation  Security Action
//                 (Count)       (Count)          (Count)
// ----------- --------------- ------------ ------------------- ---------------
// Gi1/0/1              1            1                0           Shutdown
// Gi1/0/2              2            2                5           Restrict
func ParsePortSecurity(output string) []PortSecurityStatus {
	var statuses []PortSecurityStatus
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.Contains(trimmed, "Secure Port") || strings.HasPrefix(trimmed, "--") ||
			strings.Contains(trimmed, "(Count)") {
			continue
		}

		fields := strings.Fields(trimmed)
		if len(fields) < 5 {
			continue
		}

		status := PortSecurityStatus{
			Interface:     fields[0],
			ViolationMode: fields[4],
			Status:        "Enabled",
		}

		// Parse max MAC
		if maxMAC, err := strconv.Atoi(fields[1]); err == nil {
			status.MaxMAC = maxMAC
		}

		// Parse current MAC
		if currentMAC, err := strconv.Atoi(fields[2]); err == nil {
			status.CurrentMAC = currentMAC
		}

		// Parse violation count
		if violations, err := strconv.Atoi(fields[3]); err == nil {
			status.ViolationCount = violations
		}

		statuses = append(statuses, status)
	}

	return statuses
}

// IPSGBinding represents an IP Source Guard binding
type IPSGBinding struct {
	Interface  string `json:"interface"`
	IPAddress  string `json:"ip_address"`
	MACAddress string `json:"mac_address"`
	VLAN       int    `json:"vlan"`
	Type       string `json:"type"` // static, dhcp-snooping
}

// ParseIPVerifySource parses "show ip verify source" output
// Sample output:
// Interface  Filter-type  Filter-mode  IP-address       Mac-address        Vlan
// ---------  -----------  -----------  ---------------  -----------------  ----
// Gi1/0/1    ip-mac       active       10.0.10.2        0050.56bb.0001     10
// Gi1/0/2    ip           active       10.0.20.3        -                  20
func ParseIPVerifySource(output string) []IPSGBinding {
	var bindings []IPSGBinding
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "Interface") || strings.HasPrefix(trimmed, "--") {
			continue
		}

		fields := strings.Fields(trimmed)
		if len(fields) < 6 {
			continue
		}

		binding := IPSGBinding{
			Interface:  fields[0],
			IPAddress:  fields[3],
			MACAddress: fields[4],
			Type:       "dhcp-snooping", // Default type
		}

		// Parse VLAN
		if vlan, err := strconv.Atoi(fields[5]); err == nil {
			binding.VLAN = vlan
		}

		bindings = append(bindings, binding)
	}

	return bindings
}

// PrivateVLAN represents a Private VLAN mapping
type PrivateVLAN struct {
	PrimaryVLAN     int      `json:"primary_vlan"`
	SecondaryVLAN   int      `json:"secondary_vlan"`
	SecondaryType   string   `json:"secondary_type"` // isolated, community
	Ports           []string `json:"ports"`
}

// ParsePrivateVLAN parses "show vlan private-vlan" output
// Sample output:
// Primary Secondary Type              Ports
// ------- --------- ----------------- ------------------------------------------
// 100     101       isolated          Gi1/0/1, Gi1/0/2
// 100     102       community         Gi1/0/3, Gi1/0/4
func ParsePrivateVLAN(output string) []PrivateVLAN {
	var pvlans []PrivateVLAN
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "Primary") || strings.HasPrefix(trimmed, "--") {
			continue
		}

		fields := strings.Fields(trimmed)
		if len(fields) < 3 {
			continue
		}

		pvlan := PrivateVLAN{
			SecondaryType: fields[2],
		}

		// Parse primary VLAN
		if primary, err := strconv.Atoi(fields[0]); err == nil {
			pvlan.PrimaryVLAN = primary
		}

		// Parse secondary VLAN
		if secondary, err := strconv.Atoi(fields[1]); err == nil {
			pvlan.SecondaryVLAN = secondary
		}

		// Parse ports (fields 3+)
		if len(fields) > 3 {
			ports := strings.Join(fields[3:], " ")
			pvlan.Ports = strings.Split(ports, ",")
			// Trim whitespace from port names
			for i := range pvlan.Ports {
				pvlan.Ports[i] = strings.TrimSpace(pvlan.Ports[i])
			}
		}

		pvlans = append(pvlans, pvlan)
	}

	return pvlans
}

// ===========================
// Helper Functions
// ===========================

// extractValue extracts value after a separator (usually ":")
func extractValue(line, separator string) string {
	parts := strings.Split(line, separator)
	if len(parts) >= 2 {
		return strings.TrimSpace(strings.Join(parts[1:], separator))
	}
	return ""
}


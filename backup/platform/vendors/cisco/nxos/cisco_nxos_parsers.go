package cisco

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Cisco NXOS Operational Parsers - Phase 2.1.2 and 2.1.3
// These parsers enable L2 topology mapping, VPC detection, and FabricPath discovery

// ===== L2 BASE PARSERS (9 parsers) =====

// ===== 1. MAC Address Table Parser (P0) =====

// NXOSMACEntry represents a MAC address table entry
type NXOSMACEntry struct {
	VLAN          int       `json:"vlan"`
	MACAddress    string    `json:"mac_address"`
	Type          string    `json:"type"`         // "dynamic", "static"
	Age           int       `json:"age"`          // seconds, -1 for static
	Secure        bool      `json:"secure"`
	NTFY          bool      `json:"ntfy"`         // Notification
	Port          string    `json:"port"`
	IsPrimary     bool      `json:"is_primary"`   // "*" prefix
	IsVPCPeerLink bool      `json:"is_vpc_peer"`  // "+" suffix
	CollectedAt   time.Time `json:"collected_at"`
}

// ParseNXOSMACTable parses "show mac address-table" output
func ParseNXOSMACTable(output string) ([]NXOSMACEntry, error) {
	var entries []NXOSMACEntry
	lines := strings.Split(output, "\n")
	
	// *  10     0050.56bb.0001     dynamic  300        F      F    Eth1/1
	// *  10     0050.56bb.0002     dynamic  150        F      F    Po10+
	macRe := regexp.MustCompile(`^(\*?)\s+(\d+)\s+([0-9a-f.]+)\s+(dynamic|static)\s+([\d-]+)\s+([TF])\s+([TF])\s+(\S+)(\+?)`)
	
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		if match := macRe.FindStringSubmatch(trimmed); match != nil {
			vlan, _ := strconv.Atoi(match[2])
			age, _ := strconv.Atoi(match[5])
			
			entry := NXOSMACEntry{
				VLAN:          vlan,
				MACAddress:    normalizeMACAddress(match[3]),
				Type:          match[4],
				Age:           age,
				Secure:        match[6] == "T",
				NTFY:          match[7] == "T",
				Port:          match[8],
				IsPrimary:     match[1] == "*",
				IsVPCPeerLink: match[9] == "+",
				CollectedAt:   time.Now().UTC(),
			}
			entries = append(entries, entry)
		}
	}
	
	return entries, nil
}

// ===== 2. VLAN Parser (P0) =====

// NXOSVLAN represents a VLAN
type NXOSVLAN struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Status      string    `json:"status"`      // "active", "suspended"
	Ports       []string  `json:"ports"`
	CollectedAt time.Time `json:"collected_at"`
}

// ParseNXOSVLAN parses "show vlan" output
func ParseNXOSVLAN(output string) ([]NXOSVLAN, error) {
	var vlans []NXOSVLAN
	lines := strings.Split(output, "\n")
	
	// 1    default                          active    Eth1/10, Eth1/11, Eth1/12
	// 10   Engineering                      active    Eth1/1, Eth1/2, Po10
	vlanRe := regexp.MustCompile(`^(\d+)\s+(\S+)\s+(active|suspended)\s+(.*)`)
	
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		// Skip header lines
		if strings.HasPrefix(trimmed, "VLAN") || strings.HasPrefix(trimmed, "----") || trimmed == "" {
			continue
		}
		
		if match := vlanRe.FindStringSubmatch(trimmed); match != nil {
			id, _ := strconv.Atoi(match[1])
			
			// Parse ports (comma-separated)
			var ports []string
			if match[4] != "" && match[4] != "-" {
				portList := strings.Split(match[4], ",")
				for _, port := range portList {
					ports = append(ports, strings.TrimSpace(port))
				}
			}
			
			vlans = append(vlans, NXOSVLAN{
				ID:          id,
				Name:        match[2],
				Status:      match[3],
				Ports:       ports,
				CollectedAt: time.Now().UTC(),
			})
		}
	}
	
	return vlans, nil
}

// ===== 3. Interface Status Parser (P0) =====

// NXOSInterfaceStatus represents interface status
type NXOSInterfaceStatus struct {
	Port        string    `json:"port"`
	Name        string    `json:"name"`
	Status      string    `json:"status"`       // "connected", "notconnect", "disabled"
	VLAN        string    `json:"vlan"`         // VLAN ID or "trunk"
	Duplex      string    `json:"duplex"`       // "full", "half", "auto"
	Speed       string    `json:"speed"`        // "10G", "1000", "auto"
	Type        string    `json:"type"`
	CollectedAt time.Time `json:"collected_at"`
}

// ParseNXOSInterfaceStatus parses "show interface status" output
func ParseNXOSInterfaceStatus(output string) ([]NXOSInterfaceStatus, error) {
	var interfaces []NXOSInterfaceStatus
	lines := strings.Split(output, "\n")
	
	// Eth1/1        uplink-to-core     connected 10        full    10G     10Gbase-SR
	// Po10          VPC-Peer-Link      connected trunk     full    20G     --
	intfRe := regexp.MustCompile(`^(\S+)\s+(\S+)\s+(connected|notconnect|disabled|xcvrAbsen)\s+(\S+)\s+(\S+)\s+(\S+)\s+(.*)`)
	
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		// Skip header
		if strings.HasPrefix(trimmed, "Port") || strings.HasPrefix(trimmed, "----") || trimmed == "" {
			continue
		}
		
		if match := intfRe.FindStringSubmatch(trimmed); match != nil {
			interfaces = append(interfaces, NXOSInterfaceStatus{
				Port:        match[1],
				Name:        match[2],
				Status:      match[3],
				VLAN:        match[4],
				Duplex:      match[5],
				Speed:       match[6],
				Type:        strings.TrimSpace(match[7]),
				CollectedAt: time.Now().UTC(),
			})
		}
	}
	
	return interfaces, nil
}

// ===== 4. Trunk Parser (P0) =====

// NXOSTrunk represents a trunk interface
type NXOSTrunk struct {
	Port           string    `json:"port"`
	NativeVLAN     int       `json:"native_vlan"`
	Status         string    `json:"status"`          // "trunking", "not-trunking"
	PortChannel    string    `json:"port_channel"`
	VLANsAllowed   string    `json:"vlans_allowed"`   // "1,10,20" or "1-4094"
	CollectedAt    time.Time `json:"collected_at"`
}

// ParseNXOSTrunk parses "show interface trunk" output
func ParseNXOSTrunk(output string) ([]NXOSTrunk, error) {
	var trunks []NXOSTrunk
	lines := strings.Split(output, "\n")
	
	trunkMap := make(map[string]*NXOSTrunk)
	
	// Eth1/2        1       trunking      --
	headerRe := regexp.MustCompile(`^(\S+)\s+(\d+)\s+(trunking|not-trunking)\s+(\S+)`)
	// Eth1/2        1,10,20,30,40,50
	vlanRe := regexp.MustCompile(`^(\S+)\s+([\d,\-]+)`)
	
	inVLANSection := false
	
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		// Detect VLAN section
		if strings.Contains(trimmed, "Vlans Allowed on Trunk") {
			inVLANSection = true
			continue
		}
		
		if !inVLANSection {
			// Parse trunk header
			if match := headerRe.FindStringSubmatch(trimmed); match != nil {
				nativeVLAN, _ := strconv.Atoi(match[2])
				trunk := &NXOSTrunk{
					Port:        match[1],
					NativeVLAN:  nativeVLAN,
					Status:      match[3],
					PortChannel: match[4],
					CollectedAt: time.Now().UTC(),
				}
				trunkMap[match[1]] = trunk
			}
		} else {
			// Parse VLAN allowed list
			if match := vlanRe.FindStringSubmatch(trimmed); match != nil {
				if trunk, exists := trunkMap[match[1]]; exists {
					trunk.VLANsAllowed = match[2]
				}
			}
		}
	}
	
	// Convert map to slice
	for _, trunk := range trunkMap {
		trunks = append(trunks, *trunk)
	}
	
	return trunks, nil
}

// ===== 5. Port-Channel Parser (P0 - VPC CRITICAL) =====

// NXOSPortChannel represents a port-channel (LAG)
type NXOSPortChannel struct {
	GroupID     int       `json:"group_id"`
	Name        string    `json:"name"`
	State       string    `json:"state"`        // "SU" = Switched, Up
	Type        string    `json:"type"`         // "Eth", "Fcoe"
	Protocol    string    `json:"protocol"`     // "LACP", "NONE", "STATIC"
	Members     []string  `json:"members"`
	MemberStates map[string]string `json:"member_states"` // Port -> State (P, D, I, etc.)
	CollectedAt time.Time `json:"collected_at"`
}

// ParseNXOSPortChannel parses "show port-channel summary" output
func ParseNXOSPortChannel(output string) ([]NXOSPortChannel, error) {
	var portChannels []NXOSPortChannel
	lines := strings.Split(output, "\n")
	
	// 10    Po10(SU)    Eth      LACP      Eth1/3(P)    Eth1/4(P)
	pcRe := regexp.MustCompile(`^(\d+)\s+Po\d+\(([A-Z]+)\)\s+(\S+)\s+(\S+)\s+(.+)`)
	
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		// Skip headers
		if strings.Contains(trimmed, "Group Port") || strings.Contains(trimmed, "----") || trimmed == "" {
			continue
		}
		
		if match := pcRe.FindStringSubmatch(trimmed); match != nil {
			groupID, _ := strconv.Atoi(match[1])
			
			pc := NXOSPortChannel{
				GroupID:      groupID,
				Name:         fmt.Sprintf("Po%d", groupID),
				State:        match[2],
				Type:         match[3],
				Protocol:     match[4],
				Members:      []string{},
				MemberStates: make(map[string]string),
				CollectedAt:  time.Now().UTC(),
			}
			
			// Parse member interfaces: Eth1/3(P)    Eth1/4(P)
			memberStr := match[5]
			memberRe := regexp.MustCompile(`(\S+)\(([A-Z])\)`)
			memberMatches := memberRe.FindAllStringSubmatch(memberStr, -1)
			
			for _, m := range memberMatches {
				pc.Members = append(pc.Members, m[1])
				pc.MemberStates[m[1]] = m[2]
			}
			
			portChannels = append(portChannels, pc)
		}
	}
	
	return portChannels, nil
}

// ===== 6. STP Summary Parser (P0) =====

// NXOSSTPSummary represents spanning tree summary
type NXOSSTPSummary struct {
	Mode                 string    `json:"mode"`                    // "rapid-pvst", "mst"
	RootBridgeVLANs      []int     `json:"root_bridge_vlans"`
	ExtendedSystemID     bool      `json:"extended_system_id"`
	PortfastDefault      bool      `json:"portfast_default"`
	BPDUGuardDefault     bool      `json:"bpdu_guard_default"`
	LoopguardDefault     bool      `json:"loopguard_default"`
	EtherChannelMisconfig bool     `json:"etherchannel_misconfig_guard"`
	CollectedAt          time.Time `json:"collected_at"`
}

// ParseNXOSSTP parses "show spanning-tree summary" output
func ParseNXOSSTP(output string) (*NXOSSTPSummary, error) {
	summary := &NXOSSTPSummary{
		RootBridgeVLANs: []int{},
		CollectedAt:     time.Now().UTC(),
	}
	
	lines := strings.Split(output, "\n")
	
	modeRe := regexp.MustCompile(`Switch is in (\S+) mode`)
	rootRe := regexp.MustCompile(`Root bridge for:\s+(.+)`)
	vlanRe := regexp.MustCompile(`VLAN(\d+)`)
	
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		// Mode
		if match := modeRe.FindStringSubmatch(trimmed); match != nil {
			summary.Mode = match[1]
		}
		
		// Root bridge VLANs
		if match := rootRe.FindStringSubmatch(trimmed); match != nil {
			vlanMatches := vlanRe.FindAllStringSubmatch(match[1], -1)
			for _, vm := range vlanMatches {
				vlan, _ := strconv.Atoi(vm[1])
				summary.RootBridgeVLANs = append(summary.RootBridgeVLANs, vlan)
			}
		}
		
		// Boolean flags
		if strings.Contains(trimmed, "Extended system ID") && strings.Contains(trimmed, "enabled") {
			summary.ExtendedSystemID = true
		}
		if strings.Contains(trimmed, "Portfast Default") && strings.Contains(trimmed, "enabled") {
			summary.PortfastDefault = true
		}
		if strings.Contains(trimmed, "BPDU Guard Default") && strings.Contains(trimmed, "enabled") {
			summary.BPDUGuardDefault = true
		}
		if strings.Contains(trimmed, "Loopguard Default") && strings.Contains(trimmed, "enabled") {
			summary.LoopguardDefault = true
		}
		if strings.Contains(trimmed, "EtherChannel misconfig guard") && strings.Contains(trimmed, "enabled") {
			summary.EtherChannelMisconfig = true
		}
	}
	
	return summary, nil
}

// ===== 7. LLDP Neighbors Parser (P0 - TOPOLOGY CRITICAL) =====

// NXOSLLDPNeighbor represents an LLDP neighbor
type NXOSLLDPNeighbor struct {
	LocalPort           string    `json:"local_port"`
	ChassisID           string    `json:"chassis_id"`
	PortID              string    `json:"port_id"`
	PortDescription     string    `json:"port_description"`
	SystemName          string    `json:"system_name"`
	SystemDescription   string    `json:"system_description"`
	SystemCapabilities  string    `json:"system_capabilities"`
	EnabledCapabilities string    `json:"enabled_capabilities"`
	ManagementAddress   string    `json:"management_address"`
	VLANID              int       `json:"vlan_id"`
	TimeRemaining       int       `json:"time_remaining"`      // seconds
	CollectedAt         time.Time `json:"collected_at"`
}

// ParseNXOSLLDP parses "show lldp neighbors detail" output
func ParseNXOSLLDP(output string) ([]NXOSLLDPNeighbor, error) {
	var neighbors []NXOSLLDPNeighbor
	lines := strings.Split(output, "\n")
	
	var currentNeighbor *NXOSLLDPNeighbor
	
	chassisRe := regexp.MustCompile(`^Chassis id:\s+(.+)`)
	portIDRe := regexp.MustCompile(`^Port id:\s+(.+)`)
	localPortRe := regexp.MustCompile(`^Local Port id:\s+(.+)`)
	portDescRe := regexp.MustCompile(`^Port Description:\s+(.+)`)
	sysNameRe := regexp.MustCompile(`^System Name:\s+(.+)`)
	sysDescRe := regexp.MustCompile(`^System Description:\s+(.+)`)
	timeRe := regexp.MustCompile(`^Time remaining:\s+(\d+)\s+seconds`)
	sysCapRe := regexp.MustCompile(`^System Capabilities:\s+(.+)`)
	enabledCapRe := regexp.MustCompile(`^Enabled Capabilities:\s+(.+)`)
	mgmtAddrRe := regexp.MustCompile(`^Management Address:\s+(.+)`)
	vlanRe := regexp.MustCompile(`^Vlan ID:\s+(\d+)`)
	
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		// Chassis ID starts a new neighbor
		if match := chassisRe.FindStringSubmatch(trimmed); match != nil {
			if currentNeighbor != nil {
				neighbors = append(neighbors, *currentNeighbor)
			}
			currentNeighbor = &NXOSLLDPNeighbor{
				ChassisID:   match[1],
				CollectedAt: time.Now().UTC(),
			}
		} else if currentNeighbor != nil {
			if match := portIDRe.FindStringSubmatch(trimmed); match != nil {
				currentNeighbor.PortID = match[1]
			} else if match := localPortRe.FindStringSubmatch(trimmed); match != nil {
				currentNeighbor.LocalPort = match[1]
			} else if match := portDescRe.FindStringSubmatch(trimmed); match != nil {
				currentNeighbor.PortDescription = match[1]
			} else if match := sysNameRe.FindStringSubmatch(trimmed); match != nil {
				currentNeighbor.SystemName = match[1]
			} else if match := sysDescRe.FindStringSubmatch(trimmed); match != nil {
				currentNeighbor.SystemDescription = match[1]
			} else if match := timeRe.FindStringSubmatch(trimmed); match != nil {
				currentNeighbor.TimeRemaining, _ = strconv.Atoi(match[1])
			} else if match := sysCapRe.FindStringSubmatch(trimmed); match != nil {
				currentNeighbor.SystemCapabilities = match[1]
			} else if match := enabledCapRe.FindStringSubmatch(trimmed); match != nil {
				currentNeighbor.EnabledCapabilities = match[1]
			} else if match := mgmtAddrRe.FindStringSubmatch(trimmed); match != nil {
				currentNeighbor.ManagementAddress = match[1]
			} else if match := vlanRe.FindStringSubmatch(trimmed); match != nil {
				currentNeighbor.VLANID, _ = strconv.Atoi(match[1])
			}
		}
	}
	
	// Save last neighbor
	if currentNeighbor != nil {
		neighbors = append(neighbors, *currentNeighbor)
	}
	
	return neighbors, nil
}

// ===== 8. CDP Neighbors Parser (P0) =====

// NXOSCDPNeighbor represents a CDP neighbor
type NXOSCDPNeighbor struct {
	DeviceID         string    `json:"device_id"`
	LocalInterface   string    `json:"local_interface"`
	RemotePort       string    `json:"remote_port"`
	Platform         string    `json:"platform"`
	Capabilities     string    `json:"capabilities"`
	IPAddress        string    `json:"ip_address"`
	Version          string    `json:"version"`
	NativeVLAN       int       `json:"native_vlan"`
	Duplex           string    `json:"duplex"`
	ManagementIP     string    `json:"management_ip"`
	Holdtime         int       `json:"holdtime"`
	CollectedAt      time.Time `json:"collected_at"`
}

// ParseNXOSCDP parses "show cdp neighbors detail" output
func ParseNXOSCDP(output string) ([]NXOSCDPNeighbor, error) {
	var neighbors []NXOSCDPNeighbor
	lines := strings.Split(output, "\n")
	
	var currentNeighbor *NXOSCDPNeighbor
	
	deviceIDRe := regexp.MustCompile(`^Device ID:\s*(.+)`)
	interfaceRe := regexp.MustCompile(`^Interface:\s+(\S+),\s+Port ID \(outgoing port\):\s+(.+)`)
	platformRe := regexp.MustCompile(`^Platform:\s+(.+?),\s+Capabilities:\s+(.+)`)
	ipRe := regexp.MustCompile(`IPv4 Address:\s+(\d+\.\d+\.\d+\.\d+)`)
	holdtimeRe := regexp.MustCompile(`^Holdtime:\s+(\d+)\s+sec`)
	nativeVLANRe := regexp.MustCompile(`^Native VLAN:\s+(\d+)`)
	duplexRe := regexp.MustCompile(`^Duplex:\s+(\S+)`)
	mgmtIPRe := regexp.MustCompile(`Management address\(es\):.*IPv4 Address:\s+(\d+\.\d+\.\d+\.\d+)`)
	
	inVersionSection := false
	versionLines := []string{}
	
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		// Device ID starts new neighbor
		if match := deviceIDRe.FindStringSubmatch(trimmed); match != nil {
			if currentNeighbor != nil {
				if len(versionLines) > 0 {
					currentNeighbor.Version = strings.Join(versionLines, " ")
				}
				neighbors = append(neighbors, *currentNeighbor)
			}
			currentNeighbor = &NXOSCDPNeighbor{
				DeviceID:    strings.TrimSpace(match[1]),
				CollectedAt: time.Now().UTC(),
			}
			inVersionSection = false
			versionLines = []string{}
		} else if currentNeighbor != nil {
			if match := interfaceRe.FindStringSubmatch(trimmed); match != nil {
				currentNeighbor.LocalInterface = match[1]
				currentNeighbor.RemotePort = match[2]
			} else if match := platformRe.FindStringSubmatch(trimmed); match != nil {
				currentNeighbor.Platform = strings.TrimSpace(match[1])
				currentNeighbor.Capabilities = strings.TrimSpace(match[2])
			} else if match := ipRe.FindStringSubmatch(trimmed); match != nil {
				if currentNeighbor.IPAddress == "" {
					currentNeighbor.IPAddress = match[1]
				}
			} else if match := holdtimeRe.FindStringSubmatch(trimmed); match != nil {
				currentNeighbor.Holdtime, _ = strconv.Atoi(match[1])
			} else if match := nativeVLANRe.FindStringSubmatch(trimmed); match != nil {
				currentNeighbor.NativeVLAN, _ = strconv.Atoi(match[1])
			} else if match := duplexRe.FindStringSubmatch(trimmed); match != nil {
				currentNeighbor.Duplex = match[1]
			} else if match := mgmtIPRe.FindStringSubmatch(trimmed); match != nil {
				currentNeighbor.ManagementIP = match[1]
			} else if strings.HasPrefix(trimmed, "Version:") {
				inVersionSection = true
			} else if inVersionSection && trimmed != "" && !strings.HasPrefix(trimmed, "Native VLAN") {
				versionLines = append(versionLines, trimmed)
			}
		}
	}
	
	// Save last neighbor
	if currentNeighbor != nil {
		if len(versionLines) > 0 {
			currentNeighbor.Version = strings.Join(versionLines, " ")
		}
		neighbors = append(neighbors, *currentNeighbor)
	}
	
	return neighbors, nil
}

// ===== 9. ARP Parser (P1) =====

// NXOSARPEntry represents an ARP table entry
type NXOSARPEntry struct {
	IPAddress   string    `json:"ip_address"`
	Age         string    `json:"age"`         // "00:05:12" format
	MACAddress  string    `json:"mac_address"`
	Interface   string    `json:"interface"`
	CollectedAt time.Time `json:"collected_at"`
}

// ParseNXOSARP parses "show ip arp" output
func ParseNXOSARP(output string) ([]NXOSARPEntry, error) {
	var entries []NXOSARPEntry
	lines := strings.Split(output, "\n")
	
	// 10.0.10.2       00:05:12  0050.56bb.0001  Vlan10
	arpRe := regexp.MustCompile(`^(\d+\.\d+\.\d+\.\d+)\s+([\d:]+)\s+([0-9a-f.]+)\s+(\S+)`)
	
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		if match := arpRe.FindStringSubmatch(trimmed); match != nil {
			entries = append(entries, NXOSARPEntry{
				IPAddress:   match[1],
				Age:         match[2],
				MACAddress:  normalizeMACAddress(match[3]),
				Interface:   match[4],
				CollectedAt: time.Now().UTC(),
			})
		}
	}
	
	return entries, nil
}

// ===== VPC/FABRIC PARSERS (5 parsers) =====

// ===== 10. VPC Status Parser (P0 - VPC CRITICAL) =====

// NXOSVPCStatus represents VPC domain status
type NXOSVPCStatus struct {
	DomainID                    int       `json:"domain_id"`
	PeerStatus                  string    `json:"peer_status"`
	KeepaliveStatus             string    `json:"keepalive_status"`
	ConfigConsistencyStatus     string    `json:"config_consistency_status"`
	PerVLANConsistencyStatus    string    `json:"per_vlan_consistency_status"`
	Type2ConsistencyStatus      string    `json:"type2_consistency_status"`
	VPCRole                     string    `json:"vpc_role"`               // "primary", "secondary"
	NumberOfVPCs                int       `json:"number_of_vpcs"`
	PeerGateway                 bool      `json:"peer_gateway"`
	DualActiveExcludedVLANs     string    `json:"dual_active_excluded_vlans"`
	GracefulConsistencyCheck    bool      `json:"graceful_consistency_check"`
	AutoRecoveryEnabled         bool      `json:"auto_recovery_enabled"`
	DelayRestoreEnabled         bool      `json:"delay_restore_enabled"`
	OperationalLayer3PeerRouter bool      `json:"operational_layer3_peer_router"`
	PeerLinkID                  int       `json:"peer_link_id"`
	PeerLinkPort                string    `json:"peer_link_port"`
	PeerLinkStatus              string    `json:"peer_link_status"`
	PeerLinkActiveVLANs         string    `json:"peer_link_active_vlans"`
	CollectedAt                 time.Time `json:"collected_at"`
}

// ParseNXOSVPC parses "show vpc" output
func ParseNXOSVPC(output string) (*NXOSVPCStatus, error) {
	vpc := &NXOSVPCStatus{
		CollectedAt: time.Now().UTC(),
	}
	
	lines := strings.Split(output, "\n")
	
	domainRe := regexp.MustCompile(`^vPC domain id\s*:\s*(\d+)`)
	peerStatusRe := regexp.MustCompile(`^Peer status\s*:\s*(.+)`)
	keepaliveRe := regexp.MustCompile(`^vPC keep-alive status\s*:\s*(.+)`)
	configConsistRe := regexp.MustCompile(`^Configuration consistency status\s*:\s*(\S+)`)
	perVlanConsistRe := regexp.MustCompile(`^Per-vlan consistency status\s*:\s*(\S+)`)
	type2ConsistRe := regexp.MustCompile(`^Type-2 consistency status\s*:\s*(\S+)`)
	roleRe := regexp.MustCompile(`^vPC role\s*:\s*(\S+)`)
	numVPCsRe := regexp.MustCompile(`^Number of vPCs configured\s*:\s*(\d+)`)
	peerLinkRe := regexp.MustCompile(`^(\d+)\s+(Po\d+)\s+(up|down)\s+(.+)`)
	
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		if match := domainRe.FindStringSubmatch(trimmed); match != nil {
			vpc.DomainID, _ = strconv.Atoi(match[1])
		} else if match := peerStatusRe.FindStringSubmatch(trimmed); match != nil {
			vpc.PeerStatus = strings.TrimSpace(match[1])
		} else if match := keepaliveRe.FindStringSubmatch(trimmed); match != nil {
			vpc.KeepaliveStatus = strings.TrimSpace(match[1])
		} else if match := configConsistRe.FindStringSubmatch(trimmed); match != nil {
			vpc.ConfigConsistencyStatus = match[1]
		} else if match := perVlanConsistRe.FindStringSubmatch(trimmed); match != nil {
			vpc.PerVLANConsistencyStatus = match[1]
		} else if match := type2ConsistRe.FindStringSubmatch(trimmed); match != nil {
			vpc.Type2ConsistencyStatus = match[1]
		} else if match := roleRe.FindStringSubmatch(trimmed); match != nil {
			vpc.VPCRole = match[1]
		} else if match := numVPCsRe.FindStringSubmatch(trimmed); match != nil {
			vpc.NumberOfVPCs, _ = strconv.Atoi(match[1])
		} else if strings.Contains(trimmed, "Peer Gateway") && strings.Contains(trimmed, "Enabled") {
			vpc.PeerGateway = true
		} else if strings.Contains(trimmed, "Graceful Consistency Check") && strings.Contains(trimmed, "Enabled") {
			vpc.GracefulConsistencyCheck = true
		} else if strings.Contains(trimmed, "Auto-recovery status") && strings.Contains(trimmed, "Enabled") {
			vpc.AutoRecoveryEnabled = true
		} else if strings.Contains(trimmed, "Operational Layer3 Peer-router") && strings.Contains(trimmed, "Enabled") {
			vpc.OperationalLayer3PeerRouter = true
		} else if match := peerLinkRe.FindStringSubmatch(trimmed); match != nil {
			vpc.PeerLinkID, _ = strconv.Atoi(match[1])
			vpc.PeerLinkPort = match[2]
			vpc.PeerLinkStatus = match[3]
			vpc.PeerLinkActiveVLANs = match[4]
		}
	}
	
	return vpc, nil
}

// ===== 11. VPC Peer-Keepalive Parser (P0) =====

// NXOSVPCKeepalive represents VPC keepalive status
type NXOSVPCKeepalive struct {
	Status          string    `json:"status"`           // "peer is alive"
	PeerAliveFor    string    `json:"peer_alive_for"`   // "(166978) seconds"
	SendStatus      string    `json:"send_status"`      // "Success"
	LastSendAt      string    `json:"last_send_at"`
	SentOnInterface string    `json:"sent_on_interface"`
	ReceiveStatus   string    `json:"receive_status"`
	LastReceiveAt   string    `json:"last_receive_at"`
	ReceivedOnIntf  string    `json:"received_on_interface"`
	Destination     string    `json:"destination"`      // IP address
	Interval        string    `json:"interval"`         // "1000 msec"
	Timeout         string    `json:"timeout"`          // "5 seconds"
	VRF             string    `json:"vrf"`
	UDPPort         int       `json:"udp_port"`
	CollectedAt     time.Time `json:"collected_at"`
}

// ParseNXOSVPCKeepalive parses "show vpc peer-keepalive" output
func ParseNXOSVPCKeepalive(output string) (*NXOSVPCKeepalive, error) {
	ka := &NXOSVPCKeepalive{
		CollectedAt: time.Now().UTC(),
	}
	
	lines := strings.Split(output, "\n")
	
	statusRe := regexp.MustCompile(`^vPC keep-alive status\s*:\s*(.+)`)
	peerAliveRe := regexp.MustCompile(`^--Peer is alive for\s*:\s*(.+)`)
	sendStatusRe := regexp.MustCompile(`^--Send status\s*:\s*(\S+)`)
	lastSendRe := regexp.MustCompile(`^--Last send at\s*:\s*(.+)`)
	sentOnRe := regexp.MustCompile(`^--Sent on interface\s*:\s*(\S+)`)
	recvStatusRe := regexp.MustCompile(`^--Receive status\s*:\s*(\S+)`)
	lastRecvRe := regexp.MustCompile(`^--Last receive at\s*:\s*(.+)`)
	recvOnRe := regexp.MustCompile(`^--Received on interface\s*:\s*(\S+)`)
	destRe := regexp.MustCompile(`^--Destination\s*:\s*(\S+)`)
	intervalRe := regexp.MustCompile(`^--Keepalive interval\s*:\s*(.+)`)
	timeoutRe := regexp.MustCompile(`^--Keepalive timeout\s*:\s*(.+)`)
	vrfRe := regexp.MustCompile(`^--Keepalive vrf\s*:\s*(\S+)`)
	portRe := regexp.MustCompile(`^--Keepalive udp port\s*:\s*(\d+)`)
	
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		if match := statusRe.FindStringSubmatch(trimmed); match != nil {
			ka.Status = strings.TrimSpace(match[1])
		} else if match := peerAliveRe.FindStringSubmatch(trimmed); match != nil {
			ka.PeerAliveFor = strings.TrimSpace(match[1])
		} else if match := sendStatusRe.FindStringSubmatch(trimmed); match != nil {
			ka.SendStatus = match[1]
		} else if match := lastSendRe.FindStringSubmatch(trimmed); match != nil {
			ka.LastSendAt = strings.TrimSpace(match[1])
		} else if match := sentOnRe.FindStringSubmatch(trimmed); match != nil {
			ka.SentOnInterface = match[1]
		} else if match := recvStatusRe.FindStringSubmatch(trimmed); match != nil {
			ka.ReceiveStatus = match[1]
		} else if match := lastRecvRe.FindStringSubmatch(trimmed); match != nil {
			ka.LastReceiveAt = strings.TrimSpace(match[1])
		} else if match := recvOnRe.FindStringSubmatch(trimmed); match != nil {
			ka.ReceivedOnIntf = match[1]
		} else if match := destRe.FindStringSubmatch(trimmed); match != nil {
			ka.Destination = match[1]
		} else if match := intervalRe.FindStringSubmatch(trimmed); match != nil {
			ka.Interval = strings.TrimSpace(match[1])
		} else if match := timeoutRe.FindStringSubmatch(trimmed); match != nil {
			ka.Timeout = strings.TrimSpace(match[1])
		} else if match := vrfRe.FindStringSubmatch(trimmed); match != nil {
			ka.VRF = match[1]
		} else if match := portRe.FindStringSubmatch(trimmed); match != nil {
			ka.UDPPort, _ = strconv.Atoi(match[1])
		}
	}
	
	return ka, nil
}

// ===== 12. VPC Consistency Parameters Parser (P1) =====

// NXOSVPCConsistencyParam represents a VPC consistency parameter
type NXOSVPCConsistencyParam struct {
	Name        string    `json:"name"`
	Type        string    `json:"type"`         // "1" = suspend if mismatch
	LocalValue  string    `json:"local_value"`
	PeerValue   string    `json:"peer_value"`
	Consistent  bool      `json:"consistent"`
	CollectedAt time.Time `json:"collected_at"`
}

// ParseNXOSVPCConsistency parses "show vpc consistency-parameters global" output
func ParseNXOSVPCConsistency(output string) ([]NXOSVPCConsistencyParam, error) {
	var params []NXOSVPCConsistencyParam
	lines := strings.Split(output, "\n")
	
	// STP MST Simulate PVST       1     Enabled                Enabled
	paramRe := regexp.MustCompile(`^(.+?)\s+(\d+)\s+(\S+.*?)\s{2,}(\S+.*)`)
	
	for _, line := range lines {
		// Skip headers
		if strings.Contains(line, "Name") && strings.Contains(line, "Type") {
			continue
		}
		if strings.Contains(line, "----") || strings.TrimSpace(line) == "" {
			continue
		}
		if strings.Contains(line, "Legend:") || strings.Contains(line, "Type 1") {
			continue
		}
		
		if match := paramRe.FindStringSubmatch(line); match != nil {
			param := NXOSVPCConsistencyParam{
				Name:        strings.TrimSpace(match[1]),
				Type:        match[2],
				LocalValue:  strings.TrimSpace(match[3]),
				PeerValue:   strings.TrimSpace(match[4]),
				Consistent:  strings.TrimSpace(match[3]) == strings.TrimSpace(match[4]),
				CollectedAt: time.Now().UTC(),
			}
			params = append(params, param)
		}
	}
	
	return params, nil
}

// ===== 13. VDC Parser (P1) =====

// NXOSVDC represents a Virtual Device Context
type NXOSVDC struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	State       string    `json:"state"`        // "active", "suspended"
	MACAddress  string    `json:"mac_address"`
	Type        string    `json:"type"`         // "Ethernet", "Storage"
	LineCard    string    `json:"line_card"`
	CollectedAt time.Time `json:"collected_at"`
}

// ParseNXOSVDC parses "show vdc" output
func ParseNXOSVDC(output string) ([]NXOSVDC, error) {
	var vdcs []NXOSVDC
	lines := strings.Split(output, "\n")
	
	// 1       PROD-VDC                          active              00:0c:29:f2:f8:2c   Ethernet    f3-line-card
	vdcRe := regexp.MustCompile(`^(\d+)\s+(\S+)\s+(active|suspended)\s+([0-9a-f:]+)\s+(\S+)\s+(.+)`)
	
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		// Skip headers
		if strings.Contains(trimmed, "vdc_id") || strings.Contains(trimmed, "----") || trimmed == "" {
			continue
		}
		if strings.Contains(trimmed, "Switchwide mode:") {
			continue
		}
		
		if match := vdcRe.FindStringSubmatch(trimmed); match != nil {
			id, _ := strconv.Atoi(match[1])
			vdcs = append(vdcs, NXOSVDC{
				ID:          id,
				Name:        match[2],
				State:       match[3],
				MACAddress:  match[4],
				Type:        match[5],
				LineCard:    strings.TrimSpace(match[6]),
				CollectedAt: time.Now().UTC(),
			})
		}
	}
	
	return vdcs, nil
}

// ===== 14. FabricPath Topology Parser (P1 - FABRICPATH CRITICAL) =====

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
	CollectedAt  time.Time                  `json:"collected_at"`
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

// ParseNXOSFabricPath parses "show fabricpath topology" output
func ParseNXOSFabricPath(output string) (*NXOSFabricPathTopology, error) {
	topo := &NXOSFabricPathTopology{
		Adjacencies: []NXOSFabricPathAdjacency{},
		CollectedAt: time.Now().UTC(),
	}
	
	lines := strings.Split(output, "\n")
	
	domainRe := regexp.MustCompile(`^Fabricpath IS-IS domain:\s+(\S+)`)
	tagRe := regexp.MustCompile(`^Fabricpath IS-IS tag:\s+(\d+)`)
	sysIDRe := regexp.MustCompile(`^System ID:\s+([0-9a-f.]+)\s+IS-Type:\s+(\S+)`)
	sapRe := regexp.MustCompile(`^SAP:\s+(\d+)`)
	mtuRe := regexp.MustCompile(`^Maximum LSP MTU:\s+(\d+)`)
	topoIDRe := regexp.MustCompile(`^Topology:\s+(\d+)`)
	localSwRe := regexp.MustCompile(`^Adjacency Database for Switch-ID\s+(\d+):`)
	// 1   0016.4748.ac4e     Leaf-01         *local*     2d17h              --
	// 10  0050.56bb.1234     Spine-01         Eth1/1     2d17h              10000000
	adjRe := regexp.MustCompile(`^(\d+)\s+([0-9a-f.]+)\s+(\S+)\s+(\S+)\s+(\S+)\s+(.+)`)
	
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		if match := domainRe.FindStringSubmatch(trimmed); match != nil {
			topo.Domain = match[1]
		} else if match := tagRe.FindStringSubmatch(trimmed); match != nil {
			topo.Tag, _ = strconv.Atoi(match[1])
		} else if match := sysIDRe.FindStringSubmatch(trimmed); match != nil {
			topo.SystemID = match[1]
			topo.ISType = match[2]
		} else if match := sapRe.FindStringSubmatch(trimmed); match != nil {
			topo.SAP, _ = strconv.Atoi(match[1])
		} else if match := mtuRe.FindStringSubmatch(trimmed); match != nil {
			topo.MaxLSPMTU, _ = strconv.Atoi(match[1])
		} else if match := topoIDRe.FindStringSubmatch(trimmed); match != nil {
			topo.TopologyID, _ = strconv.Atoi(match[1])
		} else if match := localSwRe.FindStringSubmatch(trimmed); match != nil {
			topo.LocalSwID, _ = strconv.Atoi(match[1])
		} else if match := adjRe.FindStringSubmatch(trimmed); match != nil {
			swID, _ := strconv.Atoi(match[1])
			adj := NXOSFabricPathAdjacency{
				SwitchID:  swID,
				SystemID:  match[2],
				Name:      match[3],
				Port:      match[4],
				UpTime:    match[5],
				Bandwidth: strings.TrimSpace(match[6]),
				IsLocal:   match[4] == "*local*",
			}
			topo.Adjacencies = append(topo.Adjacencies, adj)
		}
	}
	
	return topo, nil
}

// ===== Helper Functions =====

// normalizeMACAddress converts MAC address to lowercase with colons (reused from previous parsers)
func normalizeMACAddress(mac string) string {
	// Remove any separators
	mac = strings.ReplaceAll(mac, ":", "")
	mac = strings.ReplaceAll(mac, "-", "")
	mac = strings.ReplaceAll(mac, ".", "")
	mac = strings.ToLower(mac)
	
	// Add colons every 2 characters
	if len(mac) == 12 {
		return fmt.Sprintf("%s:%s:%s:%s:%s:%s",
			mac[0:2], mac[2:4], mac[4:6], mac[6:8], mac[8:10], mac[10:12])
	}

	return mac
}

// ===== IP Route Parser =====

// NXOSRoute represents an NXOS routing table entry
type NXOSRoute struct {
	Network          string `json:"network"`
	Netmask          string `json:"netmask"`
	NextHop          string `json:"next_hop"`
	Interface        string `json:"interface"`
	RouteType        string `json:"route_type"`        // "C" = connected, "S" = static, "O" = OSPF, "B" = BGP
	AdminDistance    int    `json:"admin_distance"`
	Metric           int    `json:"metric"`
	Protocol         string `json:"protocol"`
	Via              string `json:"via"`               // "via 10.1.1.1"
	Age              string `json:"age"`               // "2d17h"
	CollectedAt      string `json:"collected_at"`
}

// ParseNXOSIPRoute parses "show ip route" output from NXOS
func ParseNXOSIPRoute(output string) ([]NXOSRoute, error) {
	var routes []NXOSRoute

	lines := strings.Split(output, "\n")
	currentRoute := &NXOSRoute{}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip empty lines and headers
		if trimmed == "" || strings.HasPrefix(trimmed, "IP Route Table") {
			continue
		}

		// Parse route entry lines
		// Format examples:
		// 10.0.10.0/24, ubest/mbest: 1/0, attached
		//     *via 10.0.10.1, Vlan10, [0/0], 2d17h, direct
		// 0.0.0.0/0, ubest/mbest: 1/0
		//     *via 10.1.1.254, Eth1/1, [1/0], 00:05:23, static

		if !strings.HasPrefix(trimmed, "*via") && !strings.HasPrefix(trimmed, "via") {
			// This is a route header line
			if strings.Contains(trimmed, "/") {
				// Save previous route if exists
				if currentRoute.Network != "" {
					routes = append(routes, *currentRoute)
				}

				// Start new route
				currentRoute = &NXOSRoute{}

				// Parse network (e.g., "10.0.10.0/24")
				parts := strings.Split(trimmed, ",")
				if len(parts) > 0 {
					networkStr := strings.TrimSpace(parts[0])
					// Split CIDR into network and prefix
					netParts := strings.Split(networkStr, "/")
					if len(netParts) == 2 {
						currentRoute.Network = netParts[0]
						// Convert CIDR prefix to netmask (simplified)
						prefix, _ := strconv.Atoi(netParts[1])
						currentRoute.Netmask = cidrToNetmask(prefix)
					}
				}

				// Parse route type from description
				if strings.Contains(trimmed, "attached") {
					currentRoute.RouteType = "C" // Connected
					currentRoute.Protocol = "direct"
				} else if strings.Contains(trimmed, "static") {
					currentRoute.RouteType = "S" // Static
					currentRoute.Protocol = "static"
				} else if strings.Contains(trimmed, "ospf") {
					currentRoute.RouteType = "O" // OSPF
					currentRoute.Protocol = "ospf"
				} else if strings.Contains(trimmed, "bgp") {
					currentRoute.RouteType = "B" // BGP
					currentRoute.Protocol = "bgp"
				}
			}
		} else {
			// This is a "via" line (next-hop information)
			// Format: *via 10.0.10.1, Vlan10, [0/0], 2d17h, direct
			trimmed = strings.TrimPrefix(trimmed, "*")
			trimmed = strings.TrimPrefix(trimmed, "via ")

			parts := strings.Split(trimmed, ",")
			if len(parts) >= 3 {
				// Next-hop
				currentRoute.NextHop = strings.TrimSpace(parts[0])
				currentRoute.Via = "via " + currentRoute.NextHop

				// Interface
				currentRoute.Interface = strings.TrimSpace(parts[1])

				// Admin distance and metric [AD/Metric]
				if len(parts) >= 3 {
					adMetric := strings.Trim(strings.TrimSpace(parts[2]), "[]")
					adParts := strings.Split(adMetric, "/")
					if len(adParts) == 2 {
						if ad, err := strconv.Atoi(strings.TrimSpace(adParts[0])); err == nil {
							currentRoute.AdminDistance = ad
						}
						if metric, err := strconv.Atoi(strings.TrimSpace(adParts[1])); err == nil {
							currentRoute.Metric = metric
						}
					}
				}

				// Age
				if len(parts) >= 4 {
					currentRoute.Age = strings.TrimSpace(parts[3])
				}

				// Protocol (if not already set)
				if len(parts) >= 5 && currentRoute.Protocol == "" {
					currentRoute.Protocol = strings.TrimSpace(parts[4])
				}
			}
		}
	}

	// Add last route
	if currentRoute.Network != "" {
		routes = append(routes, *currentRoute)
	}

	return routes, nil
}

// cidrToNetmask converts a CIDR prefix length to a netmask string
func cidrToNetmask(prefix int) string {
	masks := map[int]string{
		8:  "255.0.0.0",
		16: "255.255.0.0",
		24: "255.255.255.0",
		25: "255.255.255.128",
		26: "255.255.255.192",
		27: "255.255.255.224",
		28: "255.255.255.240",
		29: "255.255.255.248",
		30: "255.255.255.252",
		31: "255.255.255.254",
		32: "255.255.255.255",
	}

	if mask, ok := masks[prefix]; ok {
		return mask
	}

	// For non-standard prefixes, compute the mask
	mask := uint32(0xFFFFFFFF << (32 - prefix))
	return fmt.Sprintf("%d.%d.%d.%d",
		(mask>>24)&0xFF, (mask>>16)&0xFF, (mask>>8)&0xFF, mask&0xFF)
}

// ===== HSRP Parser =====

// NXOSHSRP represents HSRP configuration and status
type NXOSHSRP struct {
	Interface       string `json:"interface"`
	Group           int    `json:"group"`
	State           string `json:"state"`           // Active, Standby, Init, Listen
	Priority        int    `json:"priority"`
	Preempt         bool   `json:"preempt"`
	VirtualIP       string `json:"virtual_ip"`
	ActiveRouter    string `json:"active_router"`
	StandbyRouter   string `json:"standby_router"`
	VirtualMAC      string `json:"virtual_mac"`
	HelloTime       int    `json:"hello_time"`      // seconds
	HoldTime        int    `json:"hold_time"`       // seconds
	Authentication  string `json:"authentication"`
	StateChanges    int    `json:"state_changes"`
	LastStateChange string `json:"last_state_change"` // "2d17h"
	CollectedAt     string `json:"collected_at"`
}

// ParseNXOSHSRP parses "show hsrp" output from NXOS
func ParseNXOSHSRP(output string) ([]NXOSHSRP, error) {
	var hsrpEntries []NXOSHSRP

	lines := strings.Split(output, "\n")
	var currentHSRP *NXOSHSRP

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip empty lines
		if trimmed == "" {
			continue
		}

		// Detect new interface section
		// Example: "Ethernet1/1 - Group 10 (HSRP-V2) (IPv4)"
		if strings.Contains(trimmed, " - Group ") && !strings.HasPrefix(trimmed, "Local") {
			// Save previous entry
			if currentHSRP != nil && currentHSRP.Interface != "" {
				hsrpEntries = append(hsrpEntries, *currentHSRP)
			}

			// Start new entry
			currentHSRP = &NXOSHSRP{}

			// Parse interface and group
			parts := strings.Split(trimmed, " - Group ")
			if len(parts) >= 2 {
				currentHSRP.Interface = strings.TrimSpace(parts[0])

				// Extract group number (e.g., "10 (HSRP-V2) (IPv4)")
				groupStr := parts[1]
				fields := strings.Fields(groupStr)
				if len(fields) > 0 {
					if group, err := strconv.Atoi(fields[0]); err == nil {
						currentHSRP.Group = group
					}
				}
			}
		} else if currentHSRP != nil {
			// Parse HSRP attributes
			if strings.HasPrefix(trimmed, "Local state is") {
				// Example: "Local state is Active, priority 110 (Cfged 110), may preempt"
				if strings.Contains(trimmed, "Local state is ") {
					stateStr := strings.TrimPrefix(trimmed, "Local state is ")
					parts := strings.Split(stateStr, ",")
					if len(parts) > 0 {
						currentHSRP.State = strings.TrimSpace(parts[0])
					}

					// Parse priority
					for _, part := range parts {
						if strings.Contains(part, "priority ") {
							prioStr := strings.TrimSpace(part)
							fields := strings.Fields(prioStr)
							for i, field := range fields {
								if field == "priority" && i+1 < len(fields) {
									if prio, err := strconv.Atoi(fields[i+1]); err == nil {
										currentHSRP.Priority = prio
									}
									break
								}
							}
						}

						if strings.Contains(part, "may preempt") || strings.Contains(part, "preempt") {
							currentHSRP.Preempt = true
						}
					}
				}
			} else if strings.Contains(trimmed, "Hellotime") {
				// Example: "Hellotime 3 sec, holdtime 10 sec"
				fields := strings.Fields(trimmed)
				for i, field := range fields {
					if field == "Hellotime" && i+1 < len(fields) {
						if hello, err := strconv.Atoi(fields[i+1]); err == nil {
							currentHSRP.HelloTime = hello
						}
					}
					if field == "holdtime" && i+1 < len(fields) {
						if hold, err := strconv.Atoi(fields[i+1]); err == nil {
							currentHSRP.HoldTime = hold
						}
					}
				}
			} else if strings.Contains(trimmed, "Virtual IP address is") {
				// Example: "Virtual IP address is 10.0.10.1 (Cfged)"
				parts := strings.Split(trimmed, "Virtual IP address is ")
				if len(parts) >= 2 {
					ipStr := parts[1]
					fields := strings.Fields(ipStr)
					if len(fields) > 0 {
						currentHSRP.VirtualIP = fields[0]
					}
				}
			} else if strings.Contains(trimmed, "Active router is") {
				// Example: "Active router is local" or "Active router is 10.0.10.2, priority 100"
				parts := strings.Split(trimmed, "Active router is ")
				if len(parts) >= 2 {
					routerStr := strings.TrimSpace(parts[1])
					if routerStr == "local" {
						currentHSRP.ActiveRouter = "local"
					} else {
						fields := strings.Split(routerStr, ",")
						if len(fields) > 0 {
							currentHSRP.ActiveRouter = strings.TrimSpace(fields[0])
						}
					}
				}
			} else if strings.Contains(trimmed, "Standby router is") {
				// Example: "Standby router is 10.0.10.3, priority 100 expires in 8.416000 sec"
				parts := strings.Split(trimmed, "Standby router is ")
				if len(parts) >= 2 {
					routerStr := parts[1]
					fields := strings.Split(routerStr, ",")
					if len(fields) > 0 {
						currentHSRP.StandbyRouter = strings.TrimSpace(fields[0])
					}
				}
			} else if strings.Contains(trimmed, "Virtual mac address is") {
				// Example: "Virtual mac address is 0000.0c9f.f00a (Default MAC)"
				parts := strings.Split(trimmed, "Virtual mac address is ")
				if len(parts) >= 2 {
					macStr := parts[1]
					fields := strings.Fields(macStr)
					if len(fields) > 0 {
						currentHSRP.VirtualMAC = fields[0]
					}
				}
			} else if strings.Contains(trimmed, "state changes") {
				// Example: "2 state changes, last state change 2d17h"
				fields := strings.Fields(trimmed)
				if len(fields) > 0 {
					if changes, err := strconv.Atoi(fields[0]); err == nil {
						currentHSRP.StateChanges = changes
					}
				}

				if strings.Contains(trimmed, "last state change ") {
					parts := strings.Split(trimmed, "last state change ")
					if len(parts) >= 2 {
						currentHSRP.LastStateChange = strings.TrimSpace(parts[1])
					}
				}
			} else if strings.Contains(trimmed, "Authentication") {
				// Example: "Authentication text \"cisco\""
				parts := strings.Split(trimmed, "Authentication ")
				if len(parts) >= 2 {
					currentHSRP.Authentication = strings.TrimSpace(parts[1])
				}
			}
		}
	}

	// Add last entry
	if currentHSRP != nil && currentHSRP.Interface != "" {
		hsrpEntries = append(hsrpEntries, *currentHSRP)
	}

	return hsrpEntries, nil
}

// ===========================
// L2 Security Control Parsers (NXOS)
// ===========================

// ParseNXOSDHCPSnoopingBinding parses "show ip dhcp snooping binding" output for NXOS
// Sample NXOS output is similar to IOS but may have slight formatting differences
func ParseNXOSDHCPSnoopingBinding(output string) []DHCPSnoopingBinding {
	// Reuse IOS parser as NXOS format is very similar
	return ParseDHCPSnoopingBinding(output)
}

// ParseNXOSARPInspectionStatistics parses "show ip arp inspection statistics" output for NXOS
func ParseNXOSARPInspectionStatistics(output string) *ARPInspectionStatistics {
	// Reuse IOS parser as NXOS format is similar
	return ParseARPInspectionStatistics(output)
}

// ParseNXOSAuthenticationSessions parses "show authentication sessions" output for NXOS
// NXOS format may have additional fields compared to IOS
func ParseNXOSAuthenticationSessions(output string) []AuthenticationSession {
	var sessions []AuthenticationSession
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "Interface") || strings.HasPrefix(trimmed, "--") ||
			strings.Contains(trimmed, "MAC Address") {
			continue
		}

		fields := strings.Fields(trimmed)
		if len(fields) < 5 {
			continue
		}

		session := AuthenticationSession{
			Interface:   fields[0],
			MACAddress:  fields[1],
			IPv4Address: fields[2],
			Status:      fields[3],
		}

		// NXOS may have different field ordering
		if len(fields) >= 6 {
			session.Method = fields[4]
			session.Domain = fields[5]
		} else if len(fields) >= 5 {
			session.Method = fields[4]
		}

		sessions = append(sessions, session)
	}

	return sessions
}

// ParseNXOSPortSecurity parses "show port-security" output for NXOS
// NXOS output format:
// Port      MaxSecAddr  CurrentAddr  SecurityViolation  Security Action
// Ethernet1/1    1          1              0                 Shutdown
func ParseNXOSPortSecurity(output string) []PortSecurityStatus {
	var statuses []PortSecurityStatus
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.Contains(trimmed, "Port") || strings.HasPrefix(trimmed, "--") ||
			strings.Contains(trimmed, "MaxSecAddr") {
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

// ParseNXOSIPVerifySource parses "show ip verify source" output for NXOS
// NXOS format is similar to IOS
func ParseNXOSIPVerifySource(output string) []IPSGBinding {
	var bindings []IPSGBinding
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "Interface") || strings.HasPrefix(trimmed, "--") {
			continue
		}

		fields := strings.Fields(trimmed)
		if len(fields) < 5 {
			continue
		}

		binding := IPSGBinding{
			Interface: fields[0],
			Type:      "dhcp-snooping",
		}

		// NXOS may have different field ordering
		// Try to parse IP address (look for valid IP pattern)
		for i := 1; i < len(fields); i++ {
			if strings.Contains(fields[i], ".") && !strings.Contains(fields[i], ":") {
				// Likely an IP address
				if len(strings.Split(fields[i], ".")) == 4 {
					binding.IPAddress = fields[i]
					break
				}
			}
		}

		// Parse MAC address (look for colon or dot notation)
		for i := 1; i < len(fields); i++ {
			if strings.Contains(fields[i], ":") || (strings.Contains(fields[i], ".") && len(fields[i]) == 14) {
				binding.MACAddress = fields[i]
				break
			}
		}

		// Try to parse VLAN from last field
		if vlan, err := strconv.Atoi(fields[len(fields)-1]); err == nil {
			binding.VLAN = vlan
		}

		bindings = append(bindings, binding)
	}

	return bindings
}

// ParseNXOSPrivateVLAN parses "show vlan private-vlan" output for NXOS
// NXOS format:
// Primary  Secondary  Type        Ports
// -------  ---------  ----------  ------------------
// 100      101        isolated    Eth1/1-2
// 100      102        community   Eth1/3-4
func ParseNXOSPrivateVLAN(output string) []PrivateVLAN {
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
		// NXOS may use ranges like "Eth1/1-2" or "Eth1/1, Eth1/2"
		if len(fields) > 3 {
			ports := strings.Join(fields[3:], " ")
			// Handle both comma-separated and range notation
			if strings.Contains(ports, ",") {
				pvlan.Ports = strings.Split(ports, ",")
			} else {
				// Single port or range
				pvlan.Ports = []string{ports}
			}
			// Trim whitespace from port names
			for i := range pvlan.Ports {
				pvlan.Ports[i] = strings.TrimSpace(pvlan.Ports[i])
			}
		}

		pvlans = append(pvlans, pvlan)
	}

	return pvlans
}


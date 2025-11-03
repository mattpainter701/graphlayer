package cisco

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Cisco ASA Operational Parsers - Phase 1.2.2 and 1.2.3
// These parsers enable L3+ topology mapping, NAT discovery, and firewall policy extraction

// ===== P0 CRITICAL PARSERS =====

// ===== 1. Interface Parser (P0) =====

// ASAInterface represents an ASA interface with operational state
type ASAInterface struct {
	Name          string    `json:"name"`
	Nameif        string    `json:"nameif"`         // Security zone name
	SecurityLevel int       `json:"security_level"`
	IPAddress     string    `json:"ip_address"`
	SubnetMask    string    `json:"subnet_mask"`
	MACAddress    string    `json:"mac_address"`
	MTU           int       `json:"mtu"`
	AdminState    string    `json:"admin_state"`    // "up" | "down"
	LineProtocol  string    `json:"line_protocol"`  // "up" | "down"
	Bandwidth     int       `json:"bandwidth"`      // Mbps
	Description   string    `json:"description,omitempty"`
	CollectedAt   time.Time `json:"collected_at"`
}

// ParseASAInterface parses "show interface" output
func ParseASAInterface(output string) ([]ASAInterface, error) {
	var interfaces []ASAInterface
	lines := strings.Split(output, "\n")
	
	var currentInterface *ASAInterface
	
	// Regex patterns
	interfaceLineRe := regexp.MustCompile(`^Interface\s+(\S+)\s+"([^"]*)",\s+is\s+(\w+),\s+line\s+protocol\s+is\s+(\w+)`)
	macRe := regexp.MustCompile(`MAC address\s+([0-9a-f.]+)`)
	ipRe := regexp.MustCompile(`IP address\s+(\d+\.\d+\.\d+\.\d+),\s+subnet mask\s+(\d+\.\d+\.\d+\.\d+)`)
	mtuRe := regexp.MustCompile(`MTU\s+(\d+)`)
	bwRe := regexp.MustCompile(`BW\s+(\d+)\s+Mbps`)
	
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		// New interface header
		if match := interfaceLineRe.FindStringSubmatch(trimmed); match != nil {
			// Save previous interface
			if currentInterface != nil {
				interfaces = append(interfaces, *currentInterface)
			}
			
			// Start new interface
			currentInterface = &ASAInterface{
				Name:          match[1],
				Nameif:        match[2],
				AdminState:    match[3],
				LineProtocol:  match[4],
				CollectedAt:   time.Now().UTC(),
			}
		} else if currentInterface != nil {
			// Parse interface details
			if match := macRe.FindStringSubmatch(trimmed); match != nil {
				currentInterface.MACAddress = normalizeMACAddress(match[1])
			} else if match := ipRe.FindStringSubmatch(trimmed); match != nil {
				currentInterface.IPAddress = match[1]
				currentInterface.SubnetMask = match[2]
			} else if match := mtuRe.FindStringSubmatch(trimmed); match != nil {
				currentInterface.MTU, _ = strconv.Atoi(match[1])
			} else if match := bwRe.FindStringSubmatch(trimmed); match != nil {
				currentInterface.Bandwidth, _ = strconv.Atoi(match[1])
			}
		}
	}
	
	// Save last interface
	if currentInterface != nil {
		interfaces = append(interfaces, *currentInterface)
	}
	
	return interfaces, nil
}

// ===== 2. Route Parser (P0) =====

// ASARoute represents a routing table entry
type ASARoute struct {
	Network       string    `json:"network"`
	Netmask       string    `json:"netmask"`
	NextHop       string    `json:"next_hop,omitempty"`
	Interface     string    `json:"interface"`
	RouteType     string    `json:"route_type"`    // "C" (connected), "S" (static), etc.
	AdminDistance int       `json:"admin_distance,omitempty"`
	Metric        int       `json:"metric,omitempty"`
	CollectedAt   time.Time `json:"collected_at"`
}

// ParseASARoute parses "show route" output
func ParseASARoute(output string) ([]ASARoute, error) {
	var routes []ASARoute
	lines := strings.Split(output, "\n")
	
	// Regex for route lines
	// C    192.168.1.0 255.255.255.0 is directly connected, outside
	// S    0.0.0.0 0.0.0.0 [1/0] via 192.168.1.254, outside
	connectedRe := regexp.MustCompile(`^([CSIRMBDO])\s+(\d+\.\d+\.\d+\.\d+)\s+(\d+\.\d+\.\d+\.\d+)\s+is directly connected,\s+(\S+)`)
	staticRe := regexp.MustCompile(`^([CSIRMBDO])\s+(\d+\.\d+\.\d+\.\d+)\s+(\d+\.\d+\.\d+\.\d+)\s+\[(\d+)/(\d+)\]\s+via\s+(\d+\.\d+\.\d+\.\d+),\s+(\S+)`)
	
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		// Skip empty lines and headers
		if trimmed == "" || strings.HasPrefix(trimmed, "Codes:") || strings.HasPrefix(trimmed, "Gateway") {
			continue
		}
		
		// Try connected route
		if match := connectedRe.FindStringSubmatch(trimmed); match != nil {
			routes = append(routes, ASARoute{
				RouteType:   match[1],
				Network:     match[2],
				Netmask:     match[3],
				Interface:   match[4],
				CollectedAt: time.Now().UTC(),
			})
		} else if match := staticRe.FindStringSubmatch(trimmed); match != nil {
			ad, _ := strconv.Atoi(match[4])
			metric, _ := strconv.Atoi(match[5])
			routes = append(routes, ASARoute{
				RouteType:     match[1],
				Network:       match[2],
				Netmask:       match[3],
				AdminDistance: ad,
				Metric:        metric,
				NextHop:       match[6],
				Interface:     match[7],
				CollectedAt:   time.Now().UTC(),
			})
		}
	}
	
	return routes, nil
}

// ===== 3. ARP Parser (P0) =====

// ASAARPEntry represents an ARP table entry
type ASAARPEntry struct {
	Interface   string    `json:"interface"`
	IPAddress   string    `json:"ip_address"`
	MACAddress  string    `json:"mac_address"`
	Age         int       `json:"age"`        // seconds
	CollectedAt time.Time `json:"collected_at"`
}

// ParseASAARP parses "show arp" output
func ParseASAARP(output string) ([]ASAARPEntry, error) {
	var entries []ASAARPEntry
	lines := strings.Split(output, "\n")
	
	// Format: outside 192.168.1.254 0050.56bb.0001 3599
	arpRe := regexp.MustCompile(`^(\S+)\s+(\d+\.\d+\.\d+\.\d+)\s+([0-9a-fA-F.:]+)\s+(\d+)`)
	
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		if match := arpRe.FindStringSubmatch(trimmed); match != nil {
			age, _ := strconv.Atoi(match[4])
			entries = append(entries, ASAARPEntry{
				Interface:   match[1],
				IPAddress:   match[2],
				MACAddress:  normalizeMACAddress(match[3]),
				Age:         age,
				CollectedAt: time.Now().UTC(),
			})
		}
	}
	
	return entries, nil
}

// ===== 4. Xlate (NAT Translation) Parser (P0 - CRITICAL) =====

// ASAXlateEntry represents an active NAT translation
type ASAXlateEntry struct {
	Flags         string    `json:"flags"`          // "D" (DNS), "i" (dynamic), "s" (static), etc.
	SourceZone    string    `json:"source_zone"`    // e.g., "inside"
	SourceIP      string    `json:"source_ip"`
	SourcePort    int       `json:"source_port,omitempty"`
	DestZone      string    `json:"dest_zone"`      // e.g., "outside"
	DestIP        string    `json:"dest_ip"`
	DestPort      int       `json:"dest_port,omitempty"`
	Protocol      string    `json:"protocol,omitempty"`  // "TCP", "UDP", "ICMP"
	IdleTime      string    `json:"idle_time"`
	Timeout       string    `json:"timeout"`
	CollectedAt   time.Time `json:"collected_at"`
}

// ParseASAXlate parses "show xlate" output
func ParseASAXlate(output string) ([]ASAXlateEntry, error) {
	var entries []ASAXlateEntry
	lines := strings.Split(output, "\n")
	
	// NAT from inside:10.0.0.10 to outside:192.168.1.100
	//     flags s idle 0:00:05 timeout 0:00:00
	// NAT from inside:10.0.0.10/443 to outside:192.168.1.100/443
	natRe := regexp.MustCompile(`^NAT from (\w+):(\d+\.\d+\.\d+\.\d+)(?:/(\d+))?\s+to\s+(\w+):(\d+\.\d+\.\d+\.\d+)(?:/(\d+))?`)
	flagsRe := regexp.MustCompile(`flags\s+([a-zA-Z]+)\s+idle\s+([\d:]+)\s+timeout\s+([\d:]+)`)
	
	var currentXlate *ASAXlateEntry
	
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		// Skip header lines
		if strings.Contains(trimmed, "in use") || strings.Contains(trimmed, "Flags:") {
			continue
		}
		
		// NAT line
		if match := natRe.FindStringSubmatch(trimmed); match != nil {
			currentXlate = &ASAXlateEntry{
				SourceZone:  match[1],
				SourceIP:    match[2],
				DestZone:    match[4],
				DestIP:      match[5],
				CollectedAt: time.Now().UTC(),
			}
			
			if match[3] != "" {
				currentXlate.SourcePort, _ = strconv.Atoi(match[3])
			}
			if match[6] != "" {
				currentXlate.DestPort, _ = strconv.Atoi(match[6])
			}
			
			entries = append(entries, *currentXlate)
		} else if currentXlate != nil && match := flagsRe.FindStringSubmatch(trimmed); match != nil {
			// Update last entry with flags
			if len(entries) > 0 {
				entries[len(entries)-1].Flags = match[1]
				entries[len(entries)-1].IdleTime = match[2]
				entries[len(entries)-1].Timeout = match[3]
			}
		}
	}
	
	return entries, nil
}

// ===== 5. NAT Policy Parser (P0) =====

// ASANATRule represents a NAT policy rule
type ASANATRule struct {
	Section          string    `json:"section"`           // "Manual NAT" or "Auto NAT"
	RuleNumber       int       `json:"rule_number"`
	SourceZone       string    `json:"source_zone"`
	DestZone         string    `json:"dest_zone"`
	SourceObject     string    `json:"source_object"`
	DestObject       string    `json:"dest_object,omitempty"`
	NATType          string    `json:"nat_type"`          // "static", "dynamic"
	TranslateHits    int       `json:"translate_hits"`
	UntranslateHits  int       `json:"untranslate_hits"`
	CollectedAt      time.Time `json:"collected_at"`
}

// ParseASANAT parses "show nat" output
func ParseASANAT(output string) ([]ASANATRule, error) {
	var rules []ASANATRule
	lines := strings.Split(output, "\n")
	
	currentSection := ""
	
	// 1 (inside) to (outside) source static obj-10.0.0.10 obj-192.168.1.100
	ruleRe := regexp.MustCompile(`^(\d+)\s+\((\w+)\)\s+to\s+\((\w+)\)\s+source\s+(static|dynamic)\s+(\S+)\s+(\S+)`)
	hitsRe := regexp.MustCompile(`translate_hits\s*=\s*(\d+)(?:,\s*untranslate_hits\s*=\s*(\d+))?`)
	
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		// Section headers
		if strings.Contains(trimmed, "Manual NAT") {
			currentSection = "Manual NAT"
			continue
		} else if strings.Contains(trimmed, "Auto NAT") {
			currentSection = "Auto NAT"
			continue
		}
		
		// Rule line
		if match := ruleRe.FindStringSubmatch(trimmed); match != nil {
			ruleNum, _ := strconv.Atoi(match[1])
			rule := ASANATRule{
				Section:      currentSection,
				RuleNumber:   ruleNum,
				SourceZone:   match[2],
				DestZone:     match[3],
				NATType:      match[4],
				SourceObject: match[5],
				DestObject:   match[6],
				CollectedAt:  time.Now().UTC(),
			}
			rules = append(rules, rule)
		} else if len(rules) > 0 && match := hitsRe.FindStringSubmatch(trimmed); match != nil {
			// Update last rule with hit counts
			translateHits, _ := strconv.Atoi(match[1])
			rules[len(rules)-1].TranslateHits = translateHits
			
			if match[2] != "" {
				untranslateHits, _ := strconv.Atoi(match[2])
				rules[len(rules)-1].UntranslateHits = untranslateHits
			}
		}
	}
	
	return rules, nil
}

// ===== 6. Connection Parser (P0) =====

// ASAConnection represents an active firewall connection
type ASAConnection struct {
	Protocol     string    `json:"protocol"`       // "TCP", "UDP", "ICMP"
	SourceZone   string    `json:"source_zone"`
	SourceIP     string    `json:"source_ip"`
	SourcePort   int       `json:"source_port,omitempty"`
	DestZone     string    `json:"dest_zone"`
	DestIP       string    `json:"dest_ip"`
	DestPort     int       `json:"dest_port,omitempty"`
	IdleTime     string    `json:"idle_time"`
	Bytes        int       `json:"bytes"`
	Flags        string    `json:"flags"`
	CollectedAt  time.Time `json:"collected_at"`
}

// ParseASAConn parses "show conn" output
func ParseASAConn(output string) ([]ASAConnection, error) {
	var connections []ASAConnection
	lines := strings.Split(output, "\n")
	
	// TCP outside 192.168.1.50:54321 inside 10.0.0.10:443, idle 0:00:05, bytes 5000, flags UIO
	connRe := regexp.MustCompile(`^(TCP|UDP|ICMP)\s+(\w+)\s+(\d+\.\d+\.\d+\.\d+):(\d+)\s+(\w+)\s+(\d+\.\d+\.\d+\.\d+):(\d+),\s+idle\s+([\d:]+),\s+bytes\s+(\d+),\s+flags\s+(\S+)`)
	
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		if match := connRe.FindStringSubmatch(trimmed); match != nil {
			srcPort, _ := strconv.Atoi(match[4])
			dstPort, _ := strconv.Atoi(match[7])
			bytes, _ := strconv.Atoi(match[9])
			
			connections = append(connections, ASAConnection{
				Protocol:    match[1],
				SourceZone:  match[2],
				SourceIP:    match[3],
				SourcePort:  srcPort,
				DestZone:    match[5],
				DestIP:      match[6],
				DestPort:    dstPort,
				IdleTime:    match[8],
				Bytes:       bytes,
				Flags:       match[10],
				CollectedAt: time.Now().UTC(),
			})
		}
	}
	
	return connections, nil
}

// ===== 7. Local Host Parser (P0) =====

// ASALocalHost represents a local host entry
type ASALocalHost struct {
	Interface        string    `json:"interface"`
	IPAddress        string    `json:"ip_address"`
	TCPFlowCount     int       `json:"tcp_flow_count"`
	TCPFlowLimit     string    `json:"tcp_flow_limit"`      // "unlimited" or number
	UDPFlowCount     int       `json:"udp_flow_count"`
	UDPFlowLimit     string    `json:"udp_flow_limit"`
	TCPEmbryonicCount int      `json:"tcp_embryonic_count"`
	CollectedAt      time.Time `json:"collected_at"`
}

// ParseASALocalHost parses "show local-host" output
func ParseASALocalHost(output string) ([]ASALocalHost, error) {
	var hosts []ASALocalHost
	lines := strings.Split(output, "\n")
	
	var currentHost *ASALocalHost
	
	// Interface inside: 50 active, 100 maximum active, 0 denied
	// local host: <10.0.0.10>,
	interfaceRe := regexp.MustCompile(`^Interface\s+(\w+):`)
	hostRe := regexp.MustCompile(`^local host:\s+<(\d+\.\d+\.\d+\.\d+)>`)
	tcpRe := regexp.MustCompile(`TCP flow count/limit\s+=\s+(\d+)/(\S+)`)
	udpRe := regexp.MustCompile(`UDP flow count/limit\s+=\s+(\d+)/(\S+)`)
	embryonicRe := regexp.MustCompile(`TCP embryonic count\s+=\s+(\d+)`)
	
	currentInterface := ""
	
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		// Interface line
		if match := interfaceRe.FindStringSubmatch(trimmed); match != nil {
			currentInterface = match[1]
		} else if match := hostRe.FindStringSubmatch(trimmed); match != nil {
			// Save previous host
			if currentHost != nil {
				hosts = append(hosts, *currentHost)
			}
			
			// New host
			currentHost = &ASALocalHost{
				Interface:   currentInterface,
				IPAddress:   match[1],
				CollectedAt: time.Now().UTC(),
			}
		} else if currentHost != nil {
			// Parse flow counts
			if match := tcpRe.FindStringSubmatch(trimmed); match != nil {
				currentHost.TCPFlowCount, _ = strconv.Atoi(match[1])
				currentHost.TCPFlowLimit = match[2]
			} else if match := udpRe.FindStringSubmatch(trimmed); match != nil {
				currentHost.UDPFlowCount, _ = strconv.Atoi(match[1])
				currentHost.UDPFlowLimit = match[2]
			} else if match := embryonicRe.FindStringSubmatch(trimmed); match != nil {
				currentHost.TCPEmbryonicCount, _ = strconv.Atoi(match[1])
			}
		}
	}
	
	// Save last host
	if currentHost != nil {
		hosts = append(hosts, *currentHost)
	}
	
	return hosts, nil
}

// ===== 8. Nameif (Zone) Parser (P0) =====

// ASAZone represents a security zone (nameif)
type ASAZone struct {
	Interface     string    `json:"interface"`
	ZoneName      string    `json:"zone_name"`
	SecurityLevel int       `json:"security_level"`
	CollectedAt   time.Time `json:"collected_at"`
}

// ParseASANameif parses "show nameif" output
func ParseASANameif(output string) ([]ASAZone, error) {
	var zones []ASAZone
	lines := strings.Split(output, "\n")
	
	// GigabitEthernet0/0       outside                  0
	nameifRe := regexp.MustCompile(`^(\S+)\s+(\S+)\s+(\d+)`)
	
	dataStarted := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		// Header line
		if strings.Contains(trimmed, "Interface") && strings.Contains(trimmed, "Name") {
			dataStarted = true
			continue
		}
		
		if dataStarted && match := nameifRe.FindStringSubmatch(trimmed); match != nil {
			secLevel, _ := strconv.Atoi(match[3])
			zones = append(zones, ASAZone{
				Interface:     match[1],
				ZoneName:      match[2],
				SecurityLevel: secLevel,
				CollectedAt:   time.Now().UTC(),
			})
		}
	}
	
	return zones, nil
}

// ===== P1 HIGH PRIORITY PARSERS =====

// ===== 9. ACL Parser (P1) =====

// ASAACLEntry represents an access control list entry
type ASAACLEntry struct {
	ACLName     string    `json:"acl_name"`
	LineNumber  int       `json:"line_number"`
	RuleType    string    `json:"rule_type"`      // "extended", "standard"
	Action      string    `json:"action"`         // "permit", "deny"
	Protocol    string    `json:"protocol"`       // "tcp", "udp", "ip", "icmp"
	Source      string    `json:"source"`
	SourcePort  string    `json:"source_port,omitempty"`
	Dest        string    `json:"dest"`
	DestPort    string    `json:"dest_port,omitempty"`
	HitCount    int       `json:"hit_count"`
	HashID      string    `json:"hash_id,omitempty"`
	CollectedAt time.Time `json:"collected_at"`
}

// ParseASAACL parses "show access-list" output
func ParseASAACL(output string) ([]ASAACLEntry, error) {
	var entries []ASAACLEntry
	lines := strings.Split(output, "\n")
	
	// access-list outside_access_in; 3 elements; name hash: 0x12345678
	// access-list outside_access_in line 1 extended permit tcp any host 192.168.1.100 eq https (hitcnt=150)
	aclHeaderRe := regexp.MustCompile(`^access-list\s+(\S+);\s+\d+\s+elements;\s+name hash:\s+(\S+)`)
	aclLineRe := regexp.MustCompile(`^access-list\s+(\S+)\s+line\s+(\d+)\s+extended\s+(permit|deny)\s+(\S+)\s+(.+?)\s+\(hitcnt=(\d+)\)`)
	
	currentACLName := ""
	currentHashID := ""
	
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		// ACL header
		if match := aclHeaderRe.FindStringSubmatch(trimmed); match != nil {
			currentACLName = match[1]
			currentHashID = match[2]
		} else if match := aclLineRe.FindStringSubmatch(trimmed); match != nil {
			lineNum, _ := strconv.Atoi(match[2])
			hitCount, _ := strconv.Atoi(match[6])
			
			// Parse remaining fields (source, dest, ports)
			remaining := match[5]
			
			entry := ASAACLEntry{
				ACLName:     currentACLName,
				LineNumber:  lineNum,
				RuleType:    "extended",
				Action:      match[3],
				Protocol:    match[4],
				HitCount:    hitCount,
				HashID:      currentHashID,
				CollectedAt: time.Now().UTC(),
			}
			
			// Simplified parsing - in production, this would be more sophisticated
			parts := strings.Fields(remaining)
			if len(parts) >= 2 {
				entry.Source = parts[0]
				entry.Dest = parts[1]
			}
			if len(parts) >= 4 && parts[2] == "eq" {
				entry.DestPort = parts[3]
			}
			
			entries = append(entries, entry)
		}
	}
	
	return entries, nil
}

// ===== 10. Service Policy Parser (P1) =====

// ASAServicePolicy represents a service policy
type ASAServicePolicy struct {
	PolicyName  string    `json:"policy_name"`
	ClassMap    string    `json:"class_map"`
	Inspection  string    `json:"inspection,omitempty"`
	Packets     int       `json:"packets"`
	Drops       int       `json:"drops"`
	ResetDrops  int       `json:"reset_drops,omitempty"`
	CollectedAt time.Time `json:"collected_at"`
}

// ParseASAServicePolicy parses "show service-policy" output
func ParseASAServicePolicy(output string) ([]ASAServicePolicy, error) {
	var policies []ASAServicePolicy
	lines := strings.Split(output, "\n")
	
	// Global policy:
	//   Service-policy: global_policy
	//     Class-map: inspection_default
	//       Inspect: dns preset_dns_map, packet 1000, drop 0, reset-drop 0
	policyRe := regexp.MustCompile(`Service-policy:\s+(\S+)`)
	classRe := regexp.MustCompile(`Class-map:\s+(\S+)`)
	inspectRe := regexp.MustCompile(`Inspect:\s+(\S+).+packet\s+(\d+),\s+drop\s+(\d+)(?:,\s+reset-drop\s+(\d+))?`)
	
	currentPolicy := ""
	currentClass := ""
	
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		if match := policyRe.FindStringSubmatch(trimmed); match != nil {
			currentPolicy = match[1]
		} else if match := classRe.FindStringSubmatch(trimmed); match != nil {
			currentClass = match[1]
		} else if match := inspectRe.FindStringSubmatch(trimmed); match != nil {
			packets, _ := strconv.Atoi(match[2])
			drops, _ := strconv.Atoi(match[3])
			
			policy := ASAServicePolicy{
				PolicyName:  currentPolicy,
				ClassMap:    currentClass,
				Inspection:  match[1],
				Packets:     packets,
				Drops:       drops,
				CollectedAt: time.Now().UTC(),
			}
			
			if match[4] != "" {
				policy.ResetDrops, _ = strconv.Atoi(match[4])
			}
			
			policies = append(policies, policy)
		}
	}
	
	return policies, nil
}

// ===== 11. Failover Parser (P1) =====

// ASAFailoverState represents failover/HA state
type ASAFailoverState struct {
	FailoverEnabled bool      `json:"failover_enabled"`
	ThisUnit        string    `json:"this_unit"`          // "Primary" | "Secondary"
	ThisState       string    `json:"this_state"`         // "Active" | "Standby Ready" | "Failed"
	OtherUnit       string    `json:"other_unit"`
	OtherState      string    `json:"other_state"`
	ActiveTime      int       `json:"active_time"`        // seconds
	StandbyTime     int       `json:"standby_time"`       // seconds
	LANInterface    string    `json:"lan_interface"`
	LANStatus       string    `json:"lan_status"`         // "up" | "down"
	CollectedAt     time.Time `json:"collected_at"`
}

// ParseASAFailover parses "show failover" output
func ParseASAFailover(output string) (*ASAFailoverState, error) {
	state := &ASAFailoverState{
		CollectedAt: time.Now().UTC(),
	}
	
	lines := strings.Split(output, "\n")
	
	// Failover On
	// Failover unit Primary
	// This host: Primary - Active
	// Other host: Secondary - Standby Ready
	failoverOnRe := regexp.MustCompile(`^Failover\s+(On|Off)`)
	unitRe := regexp.MustCompile(`^Failover unit\s+(\w+)`)
	lanRe := regexp.MustCompile(`^Failover LAN Interface:\s+\S+\s+(\S+)\s+\((\w+)\)`)
	thisHostRe := regexp.MustCompile(`^This host:\s+(\w+)\s+-\s+(.+)`)
	otherHostRe := regexp.MustCompile(`^Other host:\s+(\w+)\s+-\s+(.+)`)
	activeTimeRe := regexp.MustCompile(`Active time:\s+(\d+)\s+\(sec\)`)
	
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		if match := failoverOnRe.FindStringSubmatch(trimmed); match != nil {
			state.FailoverEnabled = (match[1] == "On")
		} else if match := unitRe.FindStringSubmatch(trimmed); match != nil {
			state.ThisUnit = match[1]
		} else if match := lanRe.FindStringSubmatch(trimmed); match != nil {
			state.LANInterface = match[1]
			state.LANStatus = match[2]
		} else if match := thisHostRe.FindStringSubmatch(trimmed); match != nil {
			state.ThisState = match[2]
		} else if match := otherHostRe.FindStringSubmatch(trimmed); match != nil {
			state.OtherUnit = match[1]
			state.OtherState = match[2]
		} else if match := activeTimeRe.FindStringSubmatch(trimmed); match != nil {
			state.ActiveTime, _ = strconv.Atoi(match[1])
		}
	}
	
	return state, nil
}

// ===== 12. VPN Session Parser (P1) =====

// ASAVPNSession represents a VPN session
type ASAVPNSession struct {
	SessionType string    `json:"session_type"`   // "IPsec", "SSL", "IKEv2"
	Username    string    `json:"username"`
	PublicIP    string    `json:"public_ip"`
	Protocol    string    `json:"protocol"`
	SessionID   int       `json:"session_id"`
	Duration    string    `json:"duration"`
	BytesTx     int       `json:"bytes_tx"`
	BytesRx     int       `json:"bytes_rx"`
	CollectedAt time.Time `json:"collected_at"`
}

// ParseASAVPN parses "show vpn-sessiondb" output
func ParseASAVPN(output string) ([]ASAVPNSession, error) {
	var sessions []ASAVPNSession
	lines := strings.Split(output, "\n")
	
	// Session Type: IPsec
	// Username     : user1                  Index    : 1
	// Public IP    : 203.0.113.50           Protocol : IKEv2 IPsec
	// Session ID   : 1000                   Duration : 0h:30m:15s
	// Bytes Tx     : 50000                  Bytes Rx : 150000
	
	var currentSession *ASAVPNSession
	
	sessionTypeRe := regexp.MustCompile(`^Session Type:\s+(\S+)`)
	usernameRe := regexp.MustCompile(`^Username\s+:\s+(\S+)`)
	publicIPRe := regexp.MustCompile(`^Public IP\s+:\s+(\S+)`)
	protocolRe := regexp.MustCompile(`^.*Protocol\s+:\s+(.+)`)
	sessionIDRe := regexp.MustCompile(`^Session ID\s+:\s+(\d+)`)
	durationRe := regexp.MustCompile(`^.*Duration\s+:\s+(\S+)`)
	bytesRe := regexp.MustCompile(`^Bytes Tx\s+:\s+(\d+)\s+Bytes Rx\s+:\s+(\d+)`)
	
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		if match := sessionTypeRe.FindStringSubmatch(trimmed); match != nil {
			// Save previous session
			if currentSession != nil {
				sessions = append(sessions, *currentSession)
			}
			
			currentSession = &ASAVPNSession{
				SessionType: match[1],
				CollectedAt: time.Now().UTC(),
			}
		} else if currentSession != nil {
			if match := usernameRe.FindStringSubmatch(trimmed); match != nil {
				currentSession.Username = match[1]
			} else if match := publicIPRe.FindStringSubmatch(trimmed); match != nil {
				currentSession.PublicIP = match[1]
			} else if match := protocolRe.FindStringSubmatch(trimmed); match != nil {
				currentSession.Protocol = strings.TrimSpace(match[1])
			} else if match := sessionIDRe.FindStringSubmatch(trimmed); match != nil {
				currentSession.SessionID, _ = strconv.Atoi(match[1])
			} else if match := durationRe.FindStringSubmatch(trimmed); match != nil {
				currentSession.Duration = match[1]
			} else if match := bytesRe.FindStringSubmatch(trimmed); match != nil {
				currentSession.BytesTx, _ = strconv.Atoi(match[1])
				currentSession.BytesRx, _ = strconv.Atoi(match[2])
			}
		}
	}
	
	// Save last session
	if currentSession != nil {
		sessions = append(sessions, *currentSession)
	}
	
	return sessions, nil
}

// ===== 13. Object Group Parser (P1) =====

// ASAObjectGroup represents an object group
type ASAObjectGroup struct {
	Name        string    `json:"name"`
	Type        string    `json:"type"`           // "network", "service", "protocol"
	Members     []string  `json:"members"`
	CollectedAt time.Time `json:"collected_at"`
}

// ParseASAObjectGroup parses "show object-group" output
func ParseASAObjectGroup(output string) ([]ASAObjectGroup, error) {
	var groups []ASAObjectGroup
	lines := strings.Split(output, "\n")
	
	// object-group network inside-networks
	//  network-object 10.0.0.0 255.255.255.0
	//  network-object 10.1.0.0 255.255.255.0
	// object-group service web-services
	//  service-object tcp eq www
	//  service-object tcp eq https
	
	groupRe := regexp.MustCompile(`^object-group\s+(network|service|protocol)\s+(\S+)`)
	memberRe := regexp.MustCompile(`^\s+(network-object|service-object|protocol-object)\s+(.+)`)
	
	var currentGroup *ASAObjectGroup
	
	for _, line := range lines {
		// object-group line
		if match := groupRe.FindStringSubmatch(line); match != nil {
			// Save previous group
			if currentGroup != nil {
				groups = append(groups, *currentGroup)
			}
			
			currentGroup = &ASAObjectGroup{
				Type:        match[1],
				Name:        match[2],
				Members:     []string{},
				CollectedAt: time.Now().UTC(),
			}
		} else if currentGroup != nil && match := memberRe.FindStringSubmatch(line); match != nil {
			// Member line
			currentGroup.Members = append(currentGroup.Members, match[2])
		}
	}
	
	// Save last group
	if currentGroup != nil {
		groups = append(groups, *currentGroup)
	}
	
	return groups, nil
}

// ===== Helper Functions =====

// normalizeMACAddress converts MAC address to lowercase with colons (reused from L2 parsers)
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


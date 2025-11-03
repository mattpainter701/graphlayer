package cisco

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// L2 Security Operational Parsers for Cisco IOS
// These parsers enable L2 decision engine to produce Allowed/Conditional/Blocked outcomes

// ===== 1. DHCP Snooping Bindings Parser =====

// DHCPSnoopingBinding represents a DHCP snooping binding entry
// This is the PRIMARY source for host-to-port confidence scoring
type DHCPSnoopingBinding struct {
	MACAddress   string    `json:"mac_address"`
	IPAddress    string    `json:"ip_address"`
	LeaseSeconds int       `json:"lease_seconds"`
	Type         string    `json:"type"`         // "dhcp-snooping" | "static"
	VLANId       int       `json:"vlan_id"`
	Interface    string    `json:"interface"`
	CollectedAt  time.Time `json:"collected_at"`
}

// ParseDHCPSnoopingBindings parses output from "show ip dhcp snooping binding"
func ParseDHCPSnoopingBindings(output string) ([]DHCPSnoopingBinding, error) {
	var bindings []DHCPSnoopingBinding
	lines := strings.Split(output, "\n")
	
	// Skip header lines
	dataStarted := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		// Skip empty lines
		if trimmed == "" {
			continue
		}
		
		// Skip header line
		if strings.Contains(trimmed, "MacAddress") && strings.Contains(trimmed, "IpAddress") {
			dataStarted = true
			continue
		}
		
		// Skip separator line
		if strings.HasPrefix(trimmed, "---") {
			continue
		}
		
		// Parse binding line
		if dataStarted {
			binding, err := parseDHCPSnoopingBindingLine(trimmed)
			if err != nil {
				// Log warning but continue parsing
				continue
			}
			bindings = append(bindings, binding)
		}
	}
	
	return bindings, nil
}

func parseDHCPSnoopingBindingLine(line string) (DHCPSnoopingBinding, error) {
	// Format: 00:11:22:33:44:55   192.168.1.10     86400       dhcp-snooping  100   GigabitEthernet1/0/12
	fields := strings.Fields(line)
	if len(fields) < 6 {
		return DHCPSnoopingBinding{}, fmt.Errorf("invalid binding line: %s", line)
	}
	
	leaseSeconds, _ := strconv.Atoi(fields[2])
	vlanId, _ := strconv.Atoi(fields[4])
	
	return DHCPSnoopingBinding{
		MACAddress:   normalizeMACAddress(fields[0]),
		IPAddress:    fields[1],
		LeaseSeconds: leaseSeconds,
		Type:         fields[3],
		VLANId:       vlanId,
		Interface:    fields[5],
		CollectedAt:  time.Now().UTC(),
	}, nil
}

// ===== 2. ARP Inspection Statistics Parser =====

// ARPInspectionStats represents DAI statistics for a VLAN
type ARPInspectionStats struct {
	VLANId        int       `json:"vlan_id"`
	Forwarded     int       `json:"forwarded"`
	Dropped       int       `json:"dropped"`
	DHCPDrops     int       `json:"dhcp_drops"`
	ACLDrops      int       `json:"acl_drops"`
	CollectedAt   time.Time `json:"collected_at"`
}

// ParseARPInspectionStats parses output from "show ip arp inspection statistics"
func ParseARPInspectionStats(output string) ([]ARPInspectionStats, error) {
	var stats []ARPInspectionStats
	lines := strings.Split(output, "\n")
	
	dataStarted := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		if trimmed == "" {
			continue
		}
		
		// Header line
		if strings.Contains(trimmed, "Vlan") && strings.Contains(trimmed, "Forwarded") {
			dataStarted = true
			continue
		}
		
		// Separator line
		if strings.HasPrefix(trimmed, "---") {
			continue
		}
		
		// Parse stats line
		if dataStarted {
			stat, err := parseARPInspectionStatsLine(trimmed)
			if err != nil {
				continue
			}
			stats = append(stats, stat)
		}
	}
	
	return stats, nil
}

func parseARPInspectionStatsLine(line string) (ARPInspectionStats, error) {
	// Format:   100           150             12              12              0
	fields := strings.Fields(line)
	if len(fields) < 5 {
		return ARPInspectionStats{}, fmt.Errorf("invalid stats line: %s", line)
	}
	
	vlanId, _ := strconv.Atoi(fields[0])
	forwarded, _ := strconv.Atoi(fields[1])
	dropped, _ := strconv.Atoi(fields[2])
	dhcpDrops, _ := strconv.Atoi(fields[3])
	aclDrops, _ := strconv.Atoi(fields[4])
	
	return ARPInspectionStats{
		VLANId:      vlanId,
		Forwarded:   forwarded,
		Dropped:     dropped,
		DHCPDrops:   dhcpDrops,
		ACLDrops:    aclDrops,
		CollectedAt: time.Now().UTC(),
	}, nil
}

// ===== 3. Authentication Sessions Parser =====

// AuthenticationSession represents an 802.1X or MAB session
// This is DECISION CRITICAL - determines Allowed vs. Conditional/Blocked
type AuthenticationSession struct {
	Interface    string    `json:"interface"`
	MACAddress   string    `json:"mac_address"`
	Method       string    `json:"method"`        // "dot1x" | "mab" | "webauth"
	Domain       string    `json:"domain"`        // "DATA" | "VOICE"
	Status       string    `json:"status"`        // "Authz Success" | "Authz Failed" | "Running"
	SessionID    string    `json:"session_id"`
	VLANId       int       `json:"vlan_id,omitempty"`
	Username     string    `json:"username,omitempty"`
	SessionTime  string    `json:"session_time,omitempty"`
	CollectedAt  time.Time `json:"collected_at"`
}

// ParseAuthenticationSessions parses output from "show authentication sessions"
func ParseAuthenticationSessions(output string) ([]AuthenticationSession, error) {
	var sessions []AuthenticationSession
	lines := strings.Split(output, "\n")
	
	dataStarted := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		if trimmed == "" {
			continue
		}
		
		// Header line
		if strings.Contains(trimmed, "Interface") && strings.Contains(trimmed, "MAC Address") {
			dataStarted = true
			continue
		}
		
		// Separator line
		if strings.HasPrefix(trimmed, "---") {
			continue
		}
		
		// Parse session line
		if dataStarted {
			session, err := parseAuthenticationSessionLine(trimmed)
			if err != nil {
				continue
			}
			sessions = append(sessions, session)
		}
	}
	
	return sessions, nil
}

func parseAuthenticationSessionLine(line string) (AuthenticationSession, error) {
	// Format: Gi1/0/12   0011.2233.4455  dot1x    DATA     Authz Success  0A1B2C3D4E5F6789
	fields := strings.Fields(line)
	if len(fields) < 5 {
		return AuthenticationSession{}, fmt.Errorf("invalid session line: %s", line)
	}
	
	session := AuthenticationSession{
		Interface:   canonicalizeInterfaceName(fields[0]),
		MACAddress:  normalizeMACAddress(fields[1]),
		Method:      strings.ToLower(fields[2]),
		Domain:      fields[3],
		Status:      strings.Join(fields[4:len(fields)-1], " "),
		CollectedAt: time.Now().UTC(),
	}
	
	// Session ID is usually the last field
	if len(fields) > 5 {
		session.SessionID = fields[len(fields)-1]
	}
	
	return session, nil
}

// ===== 4. Port Security Parser =====

// PortSecurityStatus represents port security status for an interface
type PortSecurityStatus struct {
	Interface          string    `json:"interface"`
	MaxMACCount        int       `json:"max_mac_count"`
	CurrentMACCount    int       `json:"current_mac_count"`
	SecurityViolation  string    `json:"security_violation"`  // "Shutdown" | "Restrict" | "Protect"
	Status             string    `json:"status"`              // "Secure-up" | "Secure-down" | "Secure-shutdown"
	ViolationCount     int       `json:"violation_count"`
	LastSource         string    `json:"last_source,omitempty"`
	CollectedAt        time.Time `json:"collected_at"`
}

// ParsePortSecurity parses output from "show port-security"
func ParsePortSecurity(output string) ([]PortSecurityStatus, error) {
	var statuses []PortSecurityStatus
	lines := strings.Split(output, "\n")
	
	dataStarted := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		if trimmed == "" {
			continue
		}
		
		// Header line
		if strings.Contains(trimmed, "Secure Port") && strings.Contains(trimmed, "MaxSecureAddr") {
			dataStarted = true
			continue
		}
		
		// Separator line
		if strings.HasPrefix(trimmed, "---") {
			continue
		}
		
		// Total count line (skip)
		if strings.HasPrefix(trimmed, "Total Addresses") {
			continue
		}
		
		// Parse port security line
		if dataStarted {
			status, err := parsePortSecurityLine(trimmed)
			if err != nil {
				continue
			}
			statuses = append(statuses, status)
		}
	}
	
	return statuses, nil
}

func parsePortSecurityLine(line string) (PortSecurityStatus, error) {
	// Format: Gi1/0/12     1              1            0                  Shutdown
	fields := strings.Fields(line)
	if len(fields) < 5 {
		return PortSecurityStatus{}, fmt.Errorf("invalid port security line: %s", line)
	}
	
	maxMAC, _ := strconv.Atoi(fields[1])
	currentMAC, _ := strconv.Atoi(fields[2])
	violations, _ := strconv.Atoi(fields[3])
	
	return PortSecurityStatus{
		Interface:         canonicalizeInterfaceName(fields[0]),
		MaxMACCount:       maxMAC,
		CurrentMACCount:   currentMAC,
		ViolationCount:    violations,
		SecurityViolation: fields[4],
		Status:            "Secure-up", // Default, would need more parsing for actual status
		CollectedAt:       time.Now().UTC(),
	}, nil
}

// ParsePortSecurityInterface parses detailed output from "show port-security interface {interface}"
func ParsePortSecurityInterface(output string) ([]PortSecurityStatus, error) {
	var statuses []PortSecurityStatus
	
	// This command can show multiple interfaces or one interface
	// Split by interface blocks
	interfaceBlocks := splitByInterfaceBlocks(output)
	
	for _, block := range interfaceBlocks {
		status, err := parsePortSecurityInterfaceBlock(block)
		if err != nil {
			continue
		}
		statuses = append(statuses, status)
	}
	
	return statuses, nil
}

func parsePortSecurityInterfaceBlock(block string) (PortSecurityStatus, error) {
	lines := strings.Split(block, "\n")
	
	status := PortSecurityStatus{
		CollectedAt: time.Now().UTC(),
	}
	
	// Extract interface name from first line or "Port Security" line
	interfaceRe := regexp.MustCompile(`(?i)Port Security\s+:\s+Enabled\s+on\s+(\S+)`)
	maxMACRe := regexp.MustCompile(`(?i)Maximum MAC Addresses\s+:\s+(\d+)`)
	totalMACRe := regexp.MustCompile(`(?i)Total MAC Addresses\s+:\s+(\d+)`)
	violationModeRe := regexp.MustCompile(`(?i)Violation Mode\s+:\s+(\S+)`)
	portStatusRe := regexp.MustCompile(`(?i)Port Status\s+:\s+(\S+)`)
	violationCountRe := regexp.MustCompile(`(?i)Security Violation Count\s+:\s+(\d+)`)
	lastSourceRe := regexp.MustCompile(`(?i)Last Source Address:Vlan\s+:\s+([0-9a-fA-F.:]+)`)
	
	for _, line := range lines {
		if match := interfaceRe.FindStringSubmatch(line); match != nil {
			status.Interface = canonicalizeInterfaceName(match[1])
		} else if match := maxMACRe.FindStringSubmatch(line); match != nil {
			status.MaxMACCount, _ = strconv.Atoi(match[1])
		} else if match := totalMACRe.FindStringSubmatch(line); match != nil {
			status.CurrentMACCount, _ = strconv.Atoi(match[1])
		} else if match := violationModeRe.FindStringSubmatch(line); match != nil {
			status.SecurityViolation = match[1]
		} else if match := portStatusRe.FindStringSubmatch(line); match != nil {
			status.Status = match[1]
		} else if match := violationCountRe.FindStringSubmatch(line); match != nil {
			status.ViolationCount, _ = strconv.Atoi(match[1])
		} else if match := lastSourceRe.FindStringSubmatch(line); match != nil {
			status.LastSource = match[1]
		}
	}
	
	if status.Interface == "" {
		return status, fmt.Errorf("no interface found in block")
	}
	
	return status, nil
}

// ===== 5. IP Source Guard Parser =====

// IPSGBinding represents an IP Source Guard binding
type IPSGBinding struct {
	Interface    string    `json:"interface"`
	FilterType   string    `json:"filter_type"`   // "ip" | "ip-mac"
	FilterMode   string    `json:"filter_mode"`   // "active" | "inactive"
	IPAddress    string    `json:"ip_address"`
	MACAddress   string    `json:"mac_address,omitempty"`
	VLANId       int       `json:"vlan_id"`
	CollectedAt  time.Time `json:"collected_at"`
}

// ParseIPVerifySource parses output from "show ip verify source"
func ParseIPVerifySource(output string) ([]IPSGBinding, error) {
	var bindings []IPSGBinding
	lines := strings.Split(output, "\n")
	
	dataStarted := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		if trimmed == "" {
			continue
		}
		
		// Header line
		if strings.Contains(trimmed, "Interface") && strings.Contains(trimmed, "Filter-type") {
			dataStarted = true
			continue
		}
		
		// Separator line
		if strings.HasPrefix(trimmed, "---") {
			continue
		}
		
		// Parse binding line
		if dataStarted {
			binding, err := parseIPSGBindingLine(trimmed)
			if err != nil {
				continue
			}
			bindings = append(bindings, binding)
		}
	}
	
	return bindings, nil
}

func parseIPSGBindingLine(line string) (IPSGBinding, error) {
	// Format: Gi1/0/12   ip           active       192.168.1.10     0011.2233.4455     100
	// Or:     Gi1/0/12   ip           active       192.168.1.10                        100
	fields := strings.Fields(line)
	if len(fields) < 5 {
		return IPSGBinding{}, fmt.Errorf("invalid IPSG line: %s", line)
	}
	
	binding := IPSGBinding{
		Interface:   canonicalizeInterfaceName(fields[0]),
		FilterType:  fields[1],
		FilterMode:  fields[2],
		IPAddress:   fields[3],
		CollectedAt: time.Now().UTC(),
	}
	
	// Check if MAC address and VLAN are present
	if len(fields) == 6 {
		binding.MACAddress = normalizeMACAddress(fields[4])
		binding.VLANId, _ = strconv.Atoi(fields[5])
	} else if len(fields) == 5 {
		// No MAC, last field is VLAN
		binding.VLANId, _ = strconv.Atoi(fields[4])
	}
	
	return binding, nil
}

// ===== 6. Private VLAN Parser =====

// PVLANMapping represents a Private VLAN mapping
type PVLANMapping struct {
	PrimaryVLAN      int       `json:"primary_vlan"`
	SecondaryVLAN    int       `json:"secondary_vlan"`
	Type             string    `json:"type"`              // "isolated" | "community"
	PromiscuousPorts []string  `json:"promiscuous_ports,omitempty"`
	HostPorts        []string  `json:"host_ports,omitempty"`
	CollectedAt      time.Time `json:"collected_at"`
}

// ParsePrivateVLAN parses output from "show vlan private-vlan"
func ParsePrivateVLAN(output string) ([]PVLANMapping, error) {
	var mappings []PVLANMapping
	lines := strings.Split(output, "\n")
	
	dataStarted := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		if trimmed == "" {
			continue
		}
		
		// Header line
		if strings.Contains(trimmed, "Primary") && strings.Contains(trimmed, "Secondary") {
			dataStarted = true
			continue
		}
		
		// Separator line
		if strings.HasPrefix(trimmed, "---") {
			continue
		}
		
		// Parse PVLAN line
		if dataStarted {
			mapping, err := parsePVLANMappingLine(trimmed)
			if err != nil {
				continue
			}
			mappings = append(mappings, mapping)
		}
	}
	
	return mappings, nil
}

func parsePVLANMappingLine(line string) (PVLANMapping, error) {
	// Format: 100     101       isolated          Gi1/0/1, Gi1/0/2
	// Or:     100     102       community         Gi1/0/3, Gi1/0/4
	
	// Split on whitespace, but ports might be comma-separated
	parts := strings.Fields(line)
	if len(parts) < 3 {
		return PVLANMapping{}, fmt.Errorf("invalid PVLAN line: %s", line)
	}
	
	primaryVLAN, _ := strconv.Atoi(parts[0])
	secondaryVLAN, _ := strconv.Atoi(parts[1])
	pvlanType := parts[2]
	
	mapping := PVLANMapping{
		PrimaryVLAN:   primaryVLAN,
		SecondaryVLAN: secondaryVLAN,
		Type:          pvlanType,
		CollectedAt:   time.Now().UTC(),
	}
	
	// Extract ports (everything after type field)
	if len(parts) > 3 {
		portsStr := strings.Join(parts[3:], " ")
		ports := strings.Split(portsStr, ",")
		for i, port := range ports {
			ports[i] = strings.TrimSpace(canonicalizeInterfaceName(port))
		}
		
		// Determine if promiscuous or host ports based on type
		if pvlanType == "isolated" || pvlanType == "community" {
			mapping.HostPorts = ports
		}
	}
	
	return mapping, nil
}

// ===== Helper Functions =====

// normalizeMACAddress converts MAC address to lowercase with colons
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

// canonicalizeInterfaceName converts interface name to compact form
func canonicalizeInterfaceName(ifName string) string {
	// GigabitEthernet1/0/12 -> Gi1/0/12
	// TenGigabitEthernet1/1/1 -> Te1/1/1
	// FastEthernet0/1 -> Fa0/1
	// Port-channel10 -> Po10
	
	replacements := map[string]string{
		"GigabitEthernet":    "Gi",
		"TenGigabitEthernet": "Te",
		"FastEthernet":       "Fa",
		"Port-channel":       "Po",
		"Ethernet":           "Eth",
	}
	
	result := ifName
	for old, new := range replacements {
		if strings.HasPrefix(result, old) {
			result = strings.Replace(result, old, new, 1)
			break
		}
	}
	
	return result
}

// splitByInterfaceBlocks splits output into per-interface blocks
func splitByInterfaceBlocks(output string) []string {
	// Split by common delimiters
	var blocks []string
	currentBlock := ""
	
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		// Check if this is a new interface block header
		if strings.Contains(line, "Port Security") && strings.Contains(line, "Interface") {
			if currentBlock != "" {
				blocks = append(blocks, currentBlock)
			}
			currentBlock = line + "\n"
		} else {
			currentBlock += line + "\n"
		}
	}
	
	// Add last block
	if currentBlock != "" {
		blocks = append(blocks, currentBlock)
	}
	
	// If no blocks found, return entire output as one block
	if len(blocks) == 0 {
		return []string{output}
	}
	
	return blocks
}


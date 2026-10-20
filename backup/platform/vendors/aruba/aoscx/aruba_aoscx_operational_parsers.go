package aoscx

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mattpainter701/graphlayer/platform/vendors/aruba"
)

// Aruba AOS-CX Operational Parser
//
// Parses output from show commands for runtime state
//
// Supported Commands:
// - show mac-address-table
// - show lldp neighbor-info
// - show interface <name>
// - show vlan
// - show lacp aggregates
// - show dhcp-snooping binding
// - show port-access clients
// - show port-security
//
// Command Access: SSH CLI or REST API

// ParseAOSCXMACAddressTable parses "show mac-address-table" output
//
// Example Output:
// MAC Address Table
//
// VLAN    MAC Address       Port        Type
// ----    -----------       ----        ----
// 10      00:11:22:33:44:55 1/1/1      dynamic
// 10      00:11:22:33:44:66 1/1/2      dynamic
// 20      00:aa:bb:cc:dd:ee 1/1/24     dynamic
func ParseAOSCXMACAddressTable(output string) ([]aruba.ArubaFDBEntry, error) {
	var entries []aruba.ArubaFDBEntry

	lines := strings.Split(output, "\n")
	inTable := false

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip headers
		if strings.Contains(line, "VLAN") && strings.Contains(line, "MAC Address") {
			inTable = true
			continue
		}
		if strings.Contains(line, "----") {
			continue
		}

		if !inTable || line == "" {
			continue
		}

		// Parse MAC entry line
		fields := strings.Fields(line)
		if len(fields) >= 4 {
			vlanID, err := strconv.Atoi(fields[0])
			if err != nil {
				continue
			}

			entry := aruba.ArubaFDBEntry{
				VLANID:         vlanID,
				MACAddress:     strings.ToLower(fields[1]),
				Port:           fields[2],
				Type:           fields[3],
				LastSeenTime:   time.Now(),
				EvidenceSource: "mac-address-table",
				Confidence:     1.0, // High confidence from switch FDB
			}

			entries = append(entries, entry)
		}
	}

	return entries, nil
}

// ParseAOSCXLLDPNeighbors parses "show lldp neighbor-info" output
//
// Example Output:
// LLDP Neighbor Information
//
// Local Port: 1/1/24
// Chassis ID: 00:11:22:33:44:55
// Port ID: Ethernet1
// System Name: core-switch-01
// System Description: Aruba JL635A 8360-24XF2C
// Capabilities: Bridge, Router
// Management Address: 192.168.1.1
func ParseAOSCXLLDPNeighbors(output string) ([]aruba.ArubaLLDPNeighbor, error) {
	var neighbors []aruba.ArubaLLDPNeighbor

	lines := strings.Split(output, "\n")
	var current *aruba.ArubaLLDPNeighbor

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if line == "" {
			// End of neighbor entry
			if current != nil {
				neighbors = append(neighbors, *current)
				current = nil
			}
			continue
		}

		// Parse fields
		if strings.HasPrefix(line, "Local Port:") {
			if current != nil {
				neighbors = append(neighbors, *current)
			}
			current = &aruba.ArubaLLDPNeighbor{}
			current.LocalInterface = strings.TrimSpace(strings.TrimPrefix(line, "Local Port:"))
		} else if current != nil {
			if strings.HasPrefix(line, "Chassis ID:") {
				current.ChassisID = strings.TrimSpace(strings.TrimPrefix(line, "Chassis ID:"))
			} else if strings.HasPrefix(line, "Port ID:") {
				current.RemotePortID = strings.TrimSpace(strings.TrimPrefix(line, "Port ID:"))
			} else if strings.HasPrefix(line, "System Name:") {
				current.RemoteSystemName = strings.TrimSpace(strings.TrimPrefix(line, "System Name:"))
				current.RemoteDeviceID = current.RemoteSystemName
			} else if strings.HasPrefix(line, "System Description:") {
				current.RemoteSystemDesc = strings.TrimSpace(strings.TrimPrefix(line, "System Description:"))
			} else if strings.HasPrefix(line, "Capabilities:") {
				caps := strings.TrimSpace(strings.TrimPrefix(line, "Capabilities:"))
				current.Capabilities = strings.Split(caps, ",")
				for i := range current.Capabilities {
					current.Capabilities[i] = strings.TrimSpace(current.Capabilities[i])
				}
			} else if strings.HasPrefix(line, "Management Address:") {
				current.ManagementIP = strings.TrimSpace(strings.TrimPrefix(line, "Management Address:"))
			}
		}
	}

	// Add last neighbor
	if current != nil {
		neighbors = append(neighbors, *current)
	}

	return neighbors, nil
}

// ParseAOSCXInterfaceStatus parses "show interface" output for status
//
// Example Output:
// Interface 1/1/1 is up
//  Admin state is up
//  Hardware: Ethernet, address: 00:11:22:33:44:55
//  Description: Access Port to Engineering
//  MTU 1500
//  Type: 1000-full
//  VLAN 10
func ParseAOSCXInterfaceStatus(output string) (map[string]map[string]string, error) {
	status := make(map[string]map[string]string)

	lines := strings.Split(output, "\n")
	var currentInterface string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Match interface header
		if strings.HasPrefix(trimmed, "Interface ") {
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				currentInterface = parts[1]
				status[currentInterface] = make(map[string]string)

				// Parse operational state
				if strings.Contains(trimmed, " is up") {
					status[currentInterface]["oper_state"] = "up"
				} else if strings.Contains(trimmed, " is down") {
					status[currentInterface]["oper_state"] = "down"
				}
			}
		} else if currentInterface != "" {
			// Parse interface details
			if strings.HasPrefix(trimmed, "Admin state is ") {
				adminState := strings.TrimPrefix(trimmed, "Admin state is ")
				status[currentInterface]["admin_state"] = adminState
			} else if strings.HasPrefix(trimmed, "Description: ") {
				desc := strings.TrimPrefix(trimmed, "Description: ")
				status[currentInterface]["description"] = desc
			} else if strings.HasPrefix(trimmed, "Type: ") {
				speed := strings.TrimPrefix(trimmed, "Type: ")
				status[currentInterface]["speed"] = speed
			} else if strings.HasPrefix(trimmed, "VLAN ") {
				vlan := strings.TrimPrefix(trimmed, "VLAN ")
				status[currentInterface]["vlan"] = vlan
			}
		}
	}

	return status, nil
}

// ParseAOSCXDHCPSnoopingBinding parses "show dhcp-snooping binding" output
//
// Example Output:
// DHCP Snooping Binding Table
//
// MAC Address       IP Address      VLAN  Interface  Lease Time
// ---------------   -------------   ----  ---------  ----------
// 00:11:22:33:44:55 192.168.10.100  10    1/1/1      86400
// 00:aa:bb:cc:dd:ee 192.168.10.101  10    1/1/2      86400
func ParseAOSCXDHCPSnoopingBinding(output string) ([]aruba.ArubaDHCPSnoopingBinding, error) {
	var bindings []aruba.ArubaDHCPSnoopingBinding

	lines := strings.Split(output, "\n")
	inTable := false

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip headers
		if strings.Contains(line, "MAC Address") && strings.Contains(line, "IP Address") {
			inTable = true
			continue
		}
		if strings.Contains(line, "---") {
			continue
		}

		if !inTable || line == "" {
			continue
		}

		// Parse binding entry
		fields := strings.Fields(line)
		if len(fields) >= 5 {
			vlanID, err := strconv.Atoi(fields[2])
			if err != nil {
				continue
			}

			leaseTime, _ := strconv.Atoi(fields[4])

			binding := aruba.ArubaDHCPSnoopingBinding{
				MACAddress:  strings.ToLower(fields[0]),
				IPAddress:   fields[1],
				VLANID:      vlanID,
				Interface:   fields[3],
				LeaseTime:   leaseTime,
				BindingType: "dynamic",
			}

			bindings = append(bindings, binding)
		}
	}

	return bindings, nil
}

// ParseAOSCXAuthSessions parses "show port-access clients" output
//
// Example Output:
// Port Access Client Status
//
// Interface  Client MAC        Auth Method  Auth State        VLAN
// ---------  ----------------  -----------  ----------------  ----
// 1/1/5      00:11:22:33:44:55 dot1x        authenticated     10
// 1/1/6      00:aa:bb:cc:dd:ee mac-auth     authenticated     20
// 1/1/7      00:ff:ee:dd:cc:bb dot1x        unauthenticated   999
func ParseAOSCXAuthSessions(output string) ([]aruba.ArubaAuthSession, error) {
	var sessions []aruba.ArubaAuthSession

	lines := strings.Split(output, "\n")
	inTable := false

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip headers
		if strings.Contains(line, "Interface") && strings.Contains(line, "Client MAC") {
			inTable = true
			continue
		}
		if strings.Contains(line, "---") {
			continue
		}

		if !inTable || line == "" {
			continue
		}

		// Parse session entry
		fields := strings.Fields(line)
		if len(fields) >= 5 {
			vlanID, _ := strconv.Atoi(fields[4])

			// Map auth method
			authMethod := fields[2]
			if authMethod == "dot1x" {
				authMethod = "dot1x"
			} else if authMethod == "mac-auth" {
				authMethod = "mab"
			}

			session := aruba.ArubaAuthSession{
				Interface:  fields[0],
				MACAddress: strings.ToLower(fields[1]),
				AuthMethod: authMethod,
				AuthState:  fields[3],
				VLANID:     vlanID,
			}

			sessions = append(sessions, session)
		}
	}

	return sessions, nil
}

// ParseAOSCXPortSecurity parses "show port-security" output
//
// Example Output:
// Port Security Configuration
//
// Interface  Status   Max MAC  Violation  Secure MACs
// ---------  -------  -------  ---------  -----------
// 1/1/10     enabled  2        restrict   1
// 1/1/11     enabled  1        shutdown   1
func ParseAOSCXPortSecurity(output string) ([]aruba.ArubaPortSecurity, error) {
	var entries []aruba.ArubaPortSecurity

	lines := strings.Split(output, "\n")
	inTable := false

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip headers
		if strings.Contains(line, "Interface") && strings.Contains(line, "Status") {
			inTable = true
			continue
		}
		if strings.Contains(line, "---") {
			continue
		}

		if !inTable || line == "" {
			continue
		}

		// Parse entry
		fields := strings.Fields(line)
		if len(fields) >= 4 {
			maxMAC, _ := strconv.Atoi(fields[2])

			entry := aruba.ArubaPortSecurity{
				Interface:     fields[0],
				Enabled:       fields[1] == "enabled",
				MaxMAC:        maxMAC,
				ViolationMode: fields[3],
			}

			entries = append(entries, entry)
		}
	}

	return entries, nil
}

// ParseAOSCXVLANStatus parses "show vlan" output for VLAN operational state
//
// Example Output:
// VLAN  Name                Status
// ----  ----------------    ------
// 1     DEFAULT_VLAN        up
// 10    Engineering         up
// 20    IT                  up
// 30    Guest               suspended
func ParseAOSCXVLANStatus(output string) (map[int]string, error) {
	vlanStatus := make(map[int]string)

	lines := strings.Split(output, "\n")
	inTable := false

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip headers
		if strings.Contains(line, "VLAN") && strings.Contains(line, "Name") {
			inTable = true
			continue
		}
		if strings.Contains(line, "---") {
			continue
		}

		if !inTable || line == "" {
			continue
		}

		// Parse VLAN entry
		fields := strings.Fields(line)
		if len(fields) >= 3 {
			vlanID, err := strconv.Atoi(fields[0])
			if err != nil {
				continue
			}

			status := fields[len(fields)-1] // Last field is status
			vlanStatus[vlanID] = status
		}
	}

	return vlanStatus, nil
}

// ParseAOSCXLACPAggregates parses "show lacp aggregates" output
//
// Example Output:
// Aggregate: lag1
// Aggregated-interfaces : 1/1/25 1/1/26
// Mode: active
//
// Aggregate: lag2
// Aggregated-interfaces : 1/1/27 1/1/28
// Mode: passive
func ParseAOSCXLACPAggregates(output string) (map[int][]string, error) {
	lagMembers := make(map[int][]string)

	lines := strings.Split(output, "\n")
	var currentLAG int

	lagPattern := regexp.MustCompile(`^Aggregate:\s+lag(\d+)$`)

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Match LAG header
		if matches := lagPattern.FindStringSubmatch(line); matches != nil {
			lagID, _ := strconv.Atoi(matches[1])
			currentLAG = lagID
			lagMembers[lagID] = []string{}
		} else if currentLAG > 0 && strings.HasPrefix(line, "Aggregated-interfaces") {
			// Parse member interfaces
			parts := strings.Split(line, ":")
			if len(parts) == 2 {
				members := strings.Fields(parts[1])
				lagMembers[currentLAG] = members
			}
		}
	}

	return lagMembers, nil
}

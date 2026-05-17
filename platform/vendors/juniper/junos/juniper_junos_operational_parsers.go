package junos

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strconv"
	"strings"

	"github.com/mattpainter701/graphlayer/platform/vendors/juniper"
)

// computeSnippetHash creates SHA-256 hash for evidence tracking
func computeSnippetHash(snippet string) string {
	hash := sha256.Sum256([]byte(snippet))
	return hex.EncodeToString(hash[:])
}

// ParseJunosEthernetSwitchingTable parses "show ethernet-switching table"
// Example output:
// Ethernet switching table : 3 entries, 2 learned
// Routing instance : default-switch
//   VLAN              MAC address       Type         Age Interfaces
//   Engineering       00:11:22:33:44:55 Learn          0 ge-0/0/1.0
//   Engineering       00:11:22:33:44:66 Learn        120 ge-0/0/2.0
//   Sales             00:11:22:33:44:77 Static         - ge-0/0/3.0
func ParseJunosEthernetSwitchingTable(output string) ([]juniper.JuniperFDBEntry, error) {
	var entries []juniper.JuniperFDBEntry

	lines := strings.Split(output, "\n")

	// Regex to match MAC table entries
	// Format: VLAN_NAME  MAC_ADDRESS  TYPE  AGE  INTERFACE
	entryRegex := regexp.MustCompile(`^\s*(\S+)\s+([0-9a-fA-F:]{17})\s+(\S+)\s+(\S+)\s+(\S+)`)

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip headers and empty lines
		if line == "" || strings.HasPrefix(line, "Ethernet switching") ||
		   strings.HasPrefix(line, "Routing instance") || strings.HasPrefix(line, "VLAN") {
			continue
		}

		matches := entryRegex.FindStringSubmatch(line)
		if len(matches) == 6 {
			vlanName := matches[1]
			macAddress := strings.ToUpper(matches[2])
			entryType := matches[3]
			// age := matches[4] // Not currently stored
			interfaceName := matches[5]

			// Determine if static or dynamic
			isStatic := (strings.ToLower(entryType) == "static")

			entry := juniper.JuniperFDBEntry{
				VLANName:          vlanName,
				MACAddress:        macAddress,
				InterfaceName:     interfaceName,
				IsStatic:          isStatic,
				SourceSnippetHash: computeSnippetHash(line),
			}

			entries = append(entries, entry)
		}
	}

	return entries, nil
}

// ParseJunosLLDPNeighbors parses "show lldp neighbors"
// Example output:
// Local Interface    Parent Interface    Chassis Id          Port info          System Name
// ge-0/0/1           -                   00:11:22:33:44:55   Gi0/1              switch-core-01
// ge-0/0/2           -                   00:11:22:33:44:66   Gi0/2              switch-core-02
func ParseJunosLLDPNeighbors(output string) ([]juniper.JuniperLLDPNeighbor, error) {
	var neighbors []juniper.JuniperLLDPNeighbor

	lines := strings.Split(output, "\n")

	// Regex to match LLDP neighbor entries
	neighborRegex := regexp.MustCompile(`^(\S+)\s+\S*\s+([0-9a-fA-F:]{17})\s+(\S+)\s+(.+)`)

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip headers and empty lines
		if line == "" || strings.HasPrefix(line, "Local Interface") {
			continue
		}

		matches := neighborRegex.FindStringSubmatch(line)
		if len(matches) == 5 {
			localInterface := matches[1]
			chassisID := strings.ToUpper(matches[2])
			remotePort := matches[3]
			systemName := strings.TrimSpace(matches[4])

			neighbor := juniper.JuniperLLDPNeighbor{
				LocalInterface:    localInterface,
				RemoteChassisID:   chassisID,
				RemotePort:        remotePort,
				RemoteSystemName:  systemName,
				SourceSnippetHash: computeSnippetHash(line),
			}

			neighbors = append(neighbors, neighbor)
		}
	}

	return neighbors, nil
}

// ParseJunosLLDPNeighborsDetail parses "show lldp neighbors interface <interface> detail"
// Example output:
// Local Interface: ge-0/0/1
// Parent Interface: -
// Chassis type       : Mac address
// Chassis ID         : 00:11:22:33:44:55
// Port type          : Interface name
// Port ID            : Gi0/1
// Port description   : Core Switch Port
// System name        : switch-core-01
// System description : Cisco IOS Software
func ParseJunosLLDPNeighborsDetail(output string) ([]juniper.JuniperLLDPNeighbor, error) {
	var neighbors []juniper.JuniperLLDPNeighbor

	// Split by "Local Interface:" to get individual neighbor blocks
	blocks := strings.Split(output, "Local Interface:")

	for _, block := range blocks {
		if strings.TrimSpace(block) == "" {
			continue
		}

		var neighbor juniper.JuniperLLDPNeighbor
		neighbor.SourceSnippetHash = computeSnippetHash(block)

		lines := strings.Split(block, "\n")

		for _, line := range lines {
			line = strings.TrimSpace(line)

			if line == "" {
				continue
			}

			// Parse first line as local interface
			if neighbor.LocalInterface == "" && !strings.Contains(line, ":") {
				neighbor.LocalInterface = strings.TrimSpace(line)
				continue
			}

			// Parse key-value pairs
			parts := strings.SplitN(line, ":", 2)
			if len(parts) != 2 {
				continue
			}

			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])

			switch key {
			case "Chassis ID":
				neighbor.RemoteChassisID = strings.ToUpper(value)
			case "Port ID":
				neighbor.RemotePort = value
			case "Port description":
				neighbor.RemotePortDescription = value
			case "System name":
				neighbor.RemoteSystemName = value
			case "System description":
				neighbor.RemoteSystemDescription = value
			}
		}

		if neighbor.LocalInterface != "" && neighbor.RemoteChassisID != "" {
			neighbors = append(neighbors, neighbor)
		}
	}

	return neighbors, nil
}

// ParseJunosInterfaceStatus parses "show interfaces terse | match ge-|xe-|ae"
// Example output:
// ge-0/0/0                up    up
// ge-0/0/0.0              up    up   eth-switch
// ge-0/0/1                up    up
// ge-0/0/1.0              up    up   eth-switch
// ae0                     up    up
// ae0.0                   up    up   eth-switch
func ParseJunosInterfaceStatus(output string) (map[string]juniper.JuniperInterfaceStatus, error) {
	statusMap := make(map[string]juniper.JuniperInterfaceStatus)

	lines := strings.Split(output, "\n")

	// Regex to match interface status lines
	statusRegex := regexp.MustCompile(`^(\S+)\s+(up|down)\s+(up|down)(?:\s+(.+))?`)

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if line == "" {
			continue
		}

		matches := statusRegex.FindStringSubmatch(line)
		if len(matches) >= 4 {
			interfaceName := matches[1]
			adminStatus := matches[2]
			operStatus := matches[3]
			description := ""
			if len(matches) == 5 {
				description = strings.TrimSpace(matches[4])
			}

			status := juniper.JuniperInterfaceStatus{
				InterfaceName:     interfaceName,
				AdminStatus:       adminStatus,
				OperStatus:        operStatus,
				Description:       description,
				SourceSnippetHash: computeSnippetHash(line),
			}

			statusMap[interfaceName] = status
		}
	}

	return statusMap, nil
}

// ParseJunosDHCPSecurityBinding parses "show dhcp-security binding"
// Example output:
// MAC Address        IP Address      Type     VLAN   Interface
// 00:11:22:33:44:55  192.168.10.100  DHCP     Engineering  ge-0/0/1.0
// 00:11:22:33:44:66  192.168.10.101  DHCP     Engineering  ge-0/0/2.0
// 00:11:22:33:44:77  192.168.10.10   Static   Engineering  ge-0/0/3.0
func ParseJunosDHCPSecurityBinding(output string) ([]juniper.JuniperDHCPBinding, error) {
	var bindings []juniper.JuniperDHCPBinding

	lines := strings.Split(output, "\n")

	// Regex to match DHCP binding entries
	bindingRegex := regexp.MustCompile(`^([0-9a-fA-F:]{17})\s+(\d+\.\d+\.\d+\.\d+)\s+(\S+)\s+(\S+)\s+(\S+)`)

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip headers and empty lines
		if line == "" || strings.HasPrefix(line, "MAC Address") {
			continue
		}

		matches := bindingRegex.FindStringSubmatch(line)
		if len(matches) == 6 {
			macAddress := strings.ToUpper(matches[1])
			ipAddress := matches[2]
			bindingType := matches[3]
			vlanName := matches[4]
			interfaceName := matches[5]

			binding := juniper.JuniperDHCPBinding{
				MACAddress:        macAddress,
				IPAddress:         ipAddress,
				VLANName:          vlanName,
				InterfaceName:     interfaceName,
				BindingType:       bindingType,
				SourceSnippetHash: computeSnippetHash(line),
			}

			bindings = append(bindings, binding)
		}
	}

	return bindings, nil
}

// ParseJunosDot1XSessions parses "show dot1x interface detail"
// Example output:
// Interface: ge-0/0/1.0
//   Role: Authenticator
//   Administrative state: Auto
//   Operational state: Authenticated
//   Quiet Period: 60
//   Transmit Period: 30
//   Supplicant MAC: 00:11:22:33:44:55
//   Session Timeout: 3600
//   Reauthentication Enabled: Yes
//   Reauthentication Period: 3600
func ParseJunosDot1XSessions(output string) ([]juniper.JuniperDot1XSession, error) {
	var sessions []juniper.JuniperDot1XSession

	// Split by "Interface:" to get individual session blocks
	blocks := strings.Split(output, "Interface:")

	for _, block := range blocks {
		if strings.TrimSpace(block) == "" {
			continue
		}

		var session juniper.JuniperDot1XSession
		session.SourceSnippetHash = computeSnippetHash(block)

		lines := strings.Split(block, "\n")

		// First line is the interface name
		if len(lines) > 0 {
			session.InterfaceName = strings.TrimSpace(lines[0])
		}

		for _, line := range lines[1:] {
			line = strings.TrimSpace(line)

			if line == "" {
				continue
			}

			// Parse key-value pairs
			parts := strings.SplitN(line, ":", 2)
			if len(parts) != 2 {
				continue
			}

			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])

			switch key {
			case "Operational state":
				session.AuthenticationState = value
			case "Supplicant MAC":
				session.SupplicantMAC = strings.ToUpper(value)
			case "Session Timeout":
				if timeout, err := strconv.Atoi(value); err == nil {
					session.SessionTimeout = timeout
				}
			case "Reauthentication Enabled":
				session.ReauthEnabled = (strings.ToLower(value) == "yes")
			case "Reauthentication Period":
				if period, err := strconv.Atoi(value); err == nil {
					session.ReauthPeriod = period
				}
			}
		}

		if session.InterfaceName != "" {
			sessions = append(sessions, session)
		}
	}

	return sessions, nil
}

// ParseJunosVLANSummary parses "show vlans" for operational VLAN state
// Example output:
// Name                 Tag     Interfaces
// Engineering          10      ge-0/0/1.0, ge-0/0/2.0, ae0.0
// Sales                20      ge-0/0/3.0, ge-0/0/4.0
// default              1
func ParseJunosVLANSummary(output string) (map[string]juniper.JuniperVLANOperational, error) {
	vlanMap := make(map[string]juniper.JuniperVLANOperational)

	lines := strings.Split(output, "\n")

	// Regex to match VLAN summary entries
	vlanRegex := regexp.MustCompile(`^(\S+)\s+(\d+)\s*(.*)`)

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip headers and empty lines
		if line == "" || strings.HasPrefix(line, "Name") {
			continue
		}

		matches := vlanRegex.FindStringSubmatch(line)
		if len(matches) >= 3 {
			vlanName := matches[1]
			vlanIDStr := matches[2]
			interfacesStr := ""
			if len(matches) == 4 {
				interfacesStr = matches[3]
			}

			vlanID, err := strconv.Atoi(vlanIDStr)
			if err != nil {
				continue
			}

			// Parse interfaces (comma-separated)
			var interfaces []string
			if interfacesStr != "" {
				for _, iface := range strings.Split(interfacesStr, ",") {
					iface = strings.TrimSpace(iface)
					if iface != "" {
						interfaces = append(interfaces, iface)
					}
				}
			}

			vlanOp := juniper.JuniperVLANOperational{
				VLANName:          vlanName,
				VLANID:            vlanID,
				MemberInterfaces:  interfaces,
				SourceSnippetHash: computeSnippetHash(line),
			}

			vlanMap[vlanName] = vlanOp
		}
	}

	return vlanMap, nil
}

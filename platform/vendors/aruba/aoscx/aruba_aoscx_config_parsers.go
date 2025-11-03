package aoscx

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mattpainter701/graphlayer/platform/platform/vendors/aruba"
)

// Aruba AOS-CX Configuration Parser
//
// OS Version Support:
// - AOS-CX 10.04+ (minimum supported)
// - AOS-CX 10.06+
// - AOS-CX 10.08+
// - AOS-CX 10.10+
// - AOS-CX 10.12+ (latest as of 2025)
//
// Configuration Source: running-config or startup-config
// Access Method: SSH CLI or REST API (/rest/v10.04/fullconfigs/running-config)
//
// Key Differences from Cisco IOS:
// - Interface naming: "1/1/1" instead of "GigabitEthernet1/0/1"
// - VLAN config: "vlan <id>" with indented "name <name>"
// - Port mode: "vlan access <id>" or "vlan trunk allowed <list>"
// - LAG: "interface lag <id>" with member "lag <id>"
//
// Example AOS-CX Config:
//
// vlan 10
//     name Engineering
//     description Engineering VLAN
//
// interface 1/1/1
//     no shutdown
//     description Access Port
//     vlan access 10
//
// interface 1/1/24
//     no shutdown
//     description Trunk to Core
//     vlan trunk native 1
//     vlan trunk allowed 10,20,30-40

// ParseAOSCXConfig parses a complete AOS-CX running-config
func ParseAOSCXConfig(config string, deviceID string) (*aruba.ArubaL2Config, error) {
	result := &aruba.ArubaL2Config{
		Device: aruba.ArubaDeviceInfo{
			DeviceID: deviceID,
			Platform: "aos-cx",
		},
		ParsedAt: time.Now(),
	}

	// Parse device info
	result.Device.Hostname = parseAOSCXHostname(config)
	result.Device.OSVersion = parseAOSCXVersion(config)
	result.Device.ConfigHash = computeConfigHash(config)

	// Parse VLANs
	vlans, err := ParseAOSCXVLANs(config)
	if err == nil {
		result.VLANs = vlans
	}

	// Parse interfaces
	interfaces, err := ParseAOSCXInterfaces(config)
	if err == nil {
		result.Interfaces = interfaces
	}

	// Parse port-channels (LAGs)
	portChannels, err := ParseAOSCXPortChannels(config)
	if err == nil {
		result.PortChannels = portChannels
	}

	// Parse global L2 controls
	globalControls, err := ParseAOSCXGlobalControls(config)
	if err == nil {
		result.GlobalControls = *globalControls
	}

	// Parse SVIs
	svis, err := ParseAOSCXSVIs(config)
	if err == nil {
		result.SVIs = svis
	}

	return result, nil
}

// ParseAOSCXVLANs parses VLAN configurations from AOS-CX config
//
// AOS-CX VLAN Format:
//
// vlan 10
//     name Engineering
//     description Engineering Department VLAN
//
// vlan 20
//     name IT
//     shutdown
func ParseAOSCXVLANs(config string) ([]aruba.ArubaVLAN, error) {
	var vlans []aruba.ArubaVLAN

	// Regex to match VLAN blocks
	vlanBlockPattern := regexp.MustCompile(`(?m)^vlan (\d+)\s*$`)
	lines := strings.Split(config, "\n")

	for i := 0; i < len(lines); i++ {
		line := strings.TrimRight(lines[i], "\r")

		// Match VLAN header
		if matches := vlanBlockPattern.FindStringSubmatch(line); matches != nil {
			vlanID, _ := strconv.Atoi(matches[1])

			vlan := aruba.ArubaVLAN{
				VLANID:      vlanID,
				State:       "active", // Default
				SourceBlock: fmt.Sprintf("vlan %d", vlanID),
			}

			// Parse VLAN block (indented lines following "vlan X")
			blockLines := []string{line}
			j := i + 1
			for j < len(lines) {
				nextLine := strings.TrimRight(lines[j], "\r")

				// Check if line is indented (part of VLAN block)
				if strings.HasPrefix(nextLine, "    ") || strings.HasPrefix(nextLine, "\t") {
					blockLines = append(blockLines, nextLine)

					trimmed := strings.TrimSpace(nextLine)

					// Parse VLAN name
					if strings.HasPrefix(trimmed, "name ") {
						vlan.Name = strings.TrimPrefix(trimmed, "name ")
					}

					// Parse description
					if strings.HasPrefix(trimmed, "description ") {
						vlan.Description = strings.TrimPrefix(trimmed, "description ")
					}

					// Parse state
					if trimmed == "shutdown" {
						vlan.State = "suspended"
					}

					j++
				} else {
					// End of VLAN block
					break
				}
			}

			// Compute snippet hash
			blockText := strings.Join(blockLines, "\n")
			vlan.SourceSnippetHash = computeSnippetHash(blockText)

			vlans = append(vlans, vlan)
			i = j - 1 // Continue from end of block
		}
	}

	return vlans, nil
}

// ParseAOSCXInterfaces parses interface configurations from AOS-CX config
//
// AOS-CX Interface Format:
//
// interface 1/1/1
//     no shutdown
//     description Access Port
//     vlan access 10
//     ip dhcp-snooping trust
//     port-access role employee
//
// interface 1/1/24
//     no shutdown
//     description Trunk to Core
//     vlan trunk native 1
//     vlan trunk allowed 10,20,30-40
func ParseAOSCXInterfaces(config string) ([]aruba.ArubaInterface, error) {
	var interfaces []aruba.ArubaInterface

	// Regex patterns
	interfacePattern := regexp.MustCompile(`(?m)^interface ([\d/]+|lag \d+)$`)
	lines := strings.Split(config, "\n")

	for i := 0; i < len(lines); i++ {
		line := strings.TrimRight(lines[i], "\r")

		// Match interface header
		if matches := interfacePattern.FindStringSubmatch(line); matches != nil {
			ifName := matches[1]

			iface := aruba.ArubaInterface{
				InterfaceName: ifName,
				AdminState:    "down", // Default
				Mode:          "access", // Default
				SourceBlock:   fmt.Sprintf("interface %s", ifName),
			}

			// Parse interface block
			blockLines := []string{line}
			j := i + 1
			for j < len(lines) {
				nextLine := strings.TrimRight(lines[j], "\r")

				// Check if indented
				if strings.HasPrefix(nextLine, "    ") || strings.HasPrefix(nextLine, "\t") {
					blockLines = append(blockLines, nextLine)
					trimmed := strings.TrimSpace(nextLine)

					// Parse admin state
					if trimmed == "no shutdown" {
						iface.AdminState = "up"
					} else if trimmed == "shutdown" {
						iface.AdminState = "down"
					}

					// Parse description
					if strings.HasPrefix(trimmed, "description ") {
						iface.Description = strings.TrimPrefix(trimmed, "description ")
					}

					// Parse VLAN access mode
					if strings.HasPrefix(trimmed, "vlan access ") {
						iface.Mode = "access"
						vlanStr := strings.TrimPrefix(trimmed, "vlan access ")
						if vlanID, err := strconv.Atoi(vlanStr); err == nil {
							iface.AccessVLAN = vlanID
						}
					}

					// Parse VLAN trunk native
					if strings.HasPrefix(trimmed, "vlan trunk native ") {
						iface.Mode = "trunk"
						vlanStr := strings.TrimPrefix(trimmed, "vlan trunk native ")
						if vlanID, err := strconv.Atoi(vlanStr); err == nil {
							iface.TrunkNativeVLAN = vlanID
						}
					}

					// Parse VLAN trunk allowed
					if strings.HasPrefix(trimmed, "vlan trunk allowed ") {
						iface.Mode = "trunk"
						vlanList := strings.TrimPrefix(trimmed, "vlan trunk allowed ")
						iface.TrunkAllowedVLANs = parseVLANList(vlanList)
					}

					// Parse LAG membership
					if strings.HasPrefix(trimmed, "lag ") {
						lagStr := strings.TrimPrefix(trimmed, "lag ")
						if lagID, err := strconv.Atoi(lagStr); err == nil {
							iface.PortChannelID = lagID
							iface.PortChannelName = fmt.Sprintf("lag %d", lagID)
						}
					}

					// Parse DHCP snooping trust
					if strings.Contains(trimmed, "dhcp-snooping trust") {
						iface.DHCPSnoopingTrust = true
					}

					// Parse ARP inspection trust
					if strings.Contains(trimmed, "arp-inspection trust") {
						iface.ARPInspectionTrust = true
					}

					// Parse IP source guard
					if strings.Contains(trimmed, "ip source-lockdown") || strings.Contains(trimmed, "ip verify source") {
						iface.IPSourceGuard = true
					}

					// Parse port-access (802.1X)
					if strings.Contains(trimmed, "port-access") {
						iface.Dot1XRequired = true
					}

					// Parse MAC authentication
					if strings.Contains(trimmed, "mac-authentication") {
						iface.MABEnabled = true
					}

					j++
				} else {
					break
				}
			}

			// Compute snippet hash
			blockText := strings.Join(blockLines, "\n")
			iface.SourceSnippetHash = computeSnippetHash(blockText)

			interfaces = append(interfaces, iface)
			i = j - 1
		}
	}

	return interfaces, nil
}

// ParseAOSCXPortChannels parses LAG/port-channel configurations
//
// AOS-CX LAG Format:
//
// interface lag 1
//     no shutdown
//     description Uplink LAG
//     vlan trunk native 1
//     vlan trunk allowed all
//     lacp mode active
func ParseAOSCXPortChannels(config string) ([]aruba.ArubaPortChannel, error) {
	var portChannels []aruba.ArubaPortChannel

	// Parse LAG interfaces
	lagPattern := regexp.MustCompile(`(?m)^interface lag (\d+)$`)
	lines := strings.Split(config, "\n")

	for i := 0; i < len(lines); i++ {
		line := strings.TrimRight(lines[i], "\r")

		if matches := lagPattern.FindStringSubmatch(line); matches != nil {
			lagID, _ := strconv.Atoi(matches[1])

			pc := aruba.ArubaPortChannel{
				PortChannelID:   lagID,
				PortChannelName: fmt.Sprintf("lag %d", lagID),
				Mode:            "access", // Default
				SourceBlock:     fmt.Sprintf("interface lag %d", lagID),
			}

			// Parse LAG block
			blockLines := []string{line}
			j := i + 1
			for j < len(lines) {
				nextLine := strings.TrimRight(lines[j], "\r")

				if strings.HasPrefix(nextLine, "    ") || strings.HasPrefix(nextLine, "\t") {
					blockLines = append(blockLines, nextLine)
					trimmed := strings.TrimSpace(nextLine)

					// Parse VLAN access
					if strings.HasPrefix(trimmed, "vlan access ") {
						pc.Mode = "access"
						vlanStr := strings.TrimPrefix(trimmed, "vlan access ")
						if vlanID, err := strconv.Atoi(vlanStr); err == nil {
							pc.AccessVLAN = vlanID
						}
					}

					// Parse VLAN trunk native
					if strings.HasPrefix(trimmed, "vlan trunk native ") {
						pc.Mode = "trunk"
						vlanStr := strings.TrimPrefix(trimmed, "vlan trunk native ")
						if vlanID, err := strconv.Atoi(vlanStr); err == nil {
							pc.TrunkNativeVLAN = vlanID
						}
					}

					// Parse VLAN trunk allowed
					if strings.HasPrefix(trimmed, "vlan trunk allowed ") {
						pc.Mode = "trunk"
						vlanList := strings.TrimPrefix(trimmed, "vlan trunk allowed ")
						if vlanList == "all" {
							// All VLANs allowed (1-4094)
							pc.TrunkAllowedVLANs = []int{}
						} else {
							pc.TrunkAllowedVLANs = parseVLANList(vlanList)
						}
					}

					// Parse LACP mode
					if strings.HasPrefix(trimmed, "lacp mode ") {
						pc.LACPMode = strings.TrimPrefix(trimmed, "lacp mode ")
					}

					j++
				} else {
					break
				}
			}

			// Find member interfaces (interfaces with "lag X" command)
			pc.MemberInterfaces = findLAGMembers(config, lagID)

			blockText := strings.Join(blockLines, "\n")
			pc.SourceSnippetHash = computeSnippetHash(blockText)

			portChannels = append(portChannels, pc)
			i = j - 1
		}
	}

	return portChannels, nil
}

// ParseAOSCXGlobalControls parses global L2 security controls
func ParseAOSCXGlobalControls(config string) (*aruba.ArubaGlobalL2Controls, error) {
	controls := &aruba.ArubaGlobalL2Controls{
		DAIACLNameByVLAN: make(map[string]string),
	}

	lines := strings.Split(config, "\n")
	blockLines := []string{}

	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		trimmed := strings.TrimSpace(line)

		// DHCP snooping enabled
		if strings.Contains(trimmed, "dhcp-snooping") && strings.Contains(trimmed, "enable") {
			controls.DHCPSnoopingEnabled = true
			blockLines = append(blockLines, line)
		}

		// DHCP snooping VLANs
		if strings.HasPrefix(trimmed, "dhcp-snooping vlan ") {
			vlanList := strings.TrimPrefix(trimmed, "dhcp-snooping vlan ")
			controls.DHCPSnoopingVLANs = parseVLANList(vlanList)
			blockLines = append(blockLines, line)
		}

		// ARP inspection VLANs
		if strings.HasPrefix(trimmed, "arp-inspection vlan ") {
			vlanList := strings.TrimPrefix(trimmed, "arp-inspection vlan ")
			controls.DAIEnabledVLANs = parseVLANList(vlanList)
			blockLines = append(blockLines, line)
		}

		// 802.1X system auth control
		if strings.Contains(trimmed, "aaa authentication") && strings.Contains(trimmed, "port-access") {
			controls.Dot1XSystemAuthControl = true
			blockLines = append(blockLines, line)
		}

		blockLines = append(blockLines, line)
	}

	if len(blockLines) > 0 {
		controls.SourceBlock = "global L2 controls"
		blockText := strings.Join(blockLines, "\n")
		controls.SourceSnippetHash = computeSnippetHash(blockText)
	}

	return controls, nil
}

// ParseAOSCXSVIs parses SVI (interface vlan) configurations
//
// AOS-CX SVI Format:
//
// interface vlan 10
//     ip address 192.168.10.1/24
//     description Engineering Gateway
func ParseAOSCXSVIs(config string) ([]aruba.ArubaSVI, error) {
	var svis []aruba.ArubaSVI

	sviPattern := regexp.MustCompile(`(?m)^interface vlan (\d+)$`)
	lines := strings.Split(config, "\n")

	for i := 0; i < len(lines); i++ {
		line := strings.TrimRight(lines[i], "\r")

		if matches := sviPattern.FindStringSubmatch(line); matches != nil {
			vlanID, _ := strconv.Atoi(matches[1])

			svi := aruba.ArubaSVI{
				VLANID:        vlanID,
				InterfaceName: fmt.Sprintf("vlan %d", vlanID),
				AdminState:    "down", // Default
				SourceBlock:   fmt.Sprintf("interface vlan %d", vlanID),
			}

			// Parse SVI block
			blockLines := []string{line}
			j := i + 1
			for j < len(lines) {
				nextLine := strings.TrimRight(lines[j], "\r")

				if strings.HasPrefix(nextLine, "    ") || strings.HasPrefix(nextLine, "\t") {
					blockLines = append(blockLines, nextLine)
					trimmed := strings.TrimSpace(nextLine)

					// Parse IP address
					if strings.HasPrefix(trimmed, "ip address ") {
						ipStr := strings.TrimPrefix(trimmed, "ip address ")
						// Format: "192.168.10.1/24" or "192.168.10.1 255.255.255.0"
						if strings.Contains(ipStr, "/") {
							parts := strings.Split(ipStr, "/")
							svi.IPAddress = parts[0]
							if len(parts) > 1 {
								cidrToMask(parts[1]) // Convert to mask
							}
						} else {
							parts := strings.Fields(ipStr)
							if len(parts) >= 2 {
								svi.IPAddress = parts[0]
								svi.SubnetMask = parts[1]
							}
						}
					}

					// Parse description
					if strings.HasPrefix(trimmed, "description ") {
						svi.Description = strings.TrimPrefix(trimmed, "description ")
					}

					// Parse admin state
					if trimmed == "no shutdown" {
						svi.AdminState = "up"
					}

					j++
				} else {
					break
				}
			}

			blockText := strings.Join(blockLines, "\n")
			svi.SourceSnippetHash = computeSnippetHash(blockText)

			svis = append(svis, svi)
			i = j - 1
		}
	}

	return svis, nil
}

// Helper functions

func parseAOSCXHostname(config string) string {
	lines := strings.Split(config, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "hostname ") {
			return strings.TrimPrefix(trimmed, "hostname ")
		}
	}
	return ""
}

func parseAOSCXVersion(config string) string {
	// Version usually appears in comments at top of config
	lines := strings.Split(config, "\n")
	for _, line := range lines {
		if strings.Contains(line, "ArubaOS-CX") {
			return strings.TrimSpace(line)
		}
	}
	return ""
}

func parseVLANList(vlanList string) []int {
	var vlans []int
	vlanList = strings.TrimSpace(vlanList)

	// Handle "all" keyword
	if vlanList == "all" {
		return []int{} // Empty slice means all VLANs
	}

	// Split by comma
	parts := strings.Split(vlanList, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)

		// Check for range (e.g., "10-20")
		if strings.Contains(part, "-") {
			rangeParts := strings.Split(part, "-")
			if len(rangeParts) == 2 {
				start, err1 := strconv.Atoi(strings.TrimSpace(rangeParts[0]))
				end, err2 := strconv.Atoi(strings.TrimSpace(rangeParts[1]))
				if err1 == nil && err2 == nil {
					for i := start; i <= end; i++ {
						vlans = append(vlans, i)
					}
				}
			}
		} else {
			// Single VLAN
			if vlanID, err := strconv.Atoi(part); err == nil {
				vlans = append(vlans, vlanID)
			}
		}
	}

	return vlans
}

func findLAGMembers(config string, lagID int) []string {
	var members []string
	searchStr := fmt.Sprintf("lag %d", lagID)

	lines := strings.Split(config, "\n")
	inInterface := false
	currentInterface := ""

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Track current interface
		if strings.HasPrefix(line, "interface ") && !strings.HasPrefix(trimmed, "interface lag") {
			inInterface = true
			currentInterface = strings.TrimPrefix(trimmed, "interface ")
		} else if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			inInterface = false
		}

		// Check for LAG membership
		if inInterface && trimmed == searchStr {
			members = append(members, currentInterface)
		}
	}

	return members
}

func computeConfigHash(config string) string {
	hash := sha256.Sum256([]byte(config))
	return fmt.Sprintf("sha256:%x", hash)
}

func computeSnippetHash(snippet string) string {
	// Normalize whitespace
	normalized := strings.TrimSpace(snippet)
	hash := sha256.Sum256([]byte(normalized))
	return fmt.Sprintf("sha256:%x", hash)
}

func cidrToMask(cidr string) string {
	// Convert CIDR notation to subnet mask
	cidrInt, _ := strconv.Atoi(cidr)
	mask := (0xFFFFFFFF << (32 - cidrInt)) & 0xFFFFFFFF
	return fmt.Sprintf("%d.%d.%d.%d",
		(mask>>24)&0xFF,
		(mask>>16)&0xFF,
		(mask>>8)&0xFF,
		mask&0xFF,
	)
}

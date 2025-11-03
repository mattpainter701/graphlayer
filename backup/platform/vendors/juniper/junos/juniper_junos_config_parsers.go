package junos

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mattpainter701/graphlayer/platform/platform/vendors/juniper"
)

// Juniper Junos Configuration Parser
//
// OS Version Support:
// - Junos 15.1+ (minimum)
// - Junos 17.3+ through 23.2+ (LTS versions)
//
// Configuration Source: "show configuration | display set" or hierarchical format
// Access Method: SSH CLI or NETCONF
//
// Key Differences from Cisco/Aruba:
// - Hierarchical brace-based structure: `vlans { vlan-name { ... } }`
// - Uses VLAN names primarily, not IDs
// - Interface naming: "ge-0/0/0" (slot/pic/port)
// - Aggregated Ethernet (AE) instead of Port-channel
// - "ethernet-switching" family for L2 config
//
// Example Junos Config:
//
// vlans {
//     Engineering {
//         vlan-id 10;
//         description "Engineering VLAN";
//     }
//     IT {
//         vlan-id 20;
//         l3-interface irb.20;
//     }
// }
//
// interfaces {
//     ge-0/0/0 {
//         description "Access Port";
//         unit 0 {
//             family ethernet-switching {
//                 port-mode access;
//                 vlan {
//                     members Engineering;
//                 }
//             }
//         }
//     }
// }

// ParseJunosConfig parses a complete Junos configuration
func ParseJunosConfig(config string, deviceID string) (*juniper.JuniperL2Config, error) {
	result := &juniper.JuniperL2Config{
		Device: juniper.JuniperDeviceInfo{
			DeviceID: deviceID,
			Platform: "junos",
		},
		ParsedAt: time.Now(),
	}

	// Parse device info
	result.Device.Hostname = parseJunosHostname(config)
	result.Device.OSVersion = parseJunosVersion(config)
	result.Device.ConfigHash = computeConfigHash(config)

	// Parse VLANs
	vlans, err := ParseJunosVLANs(config)
	if err == nil {
		result.VLANs = vlans
	}

	// Parse interfaces
	interfaces, err := ParseJunosInterfaces(config)
	if err == nil {
		result.Interfaces = interfaces
	}

	// Parse aggregated ethernet (AE)
	aes, err := ParseJunosAggregatedEthernet(config)
	if err == nil {
		result.AggregatedEthernets = aes
	}

	// Parse global L2 controls
	controls, err := ParseJunosGlobalControls(config)
	if err == nil {
		result.GlobalControls = *controls
	}

	// Parse IRBs (L3 interfaces)
	irbs, err := ParseJunosIRBs(config)
	if err == nil {
		result.IRBs = irbs
	}

	return result, nil
}

// ParseJunosVLANs parses VLAN configurations from Junos config
//
// Junos VLAN Format:
//
// vlans {
//     Engineering {
//         vlan-id 10;
//         description "Engineering Department";
//     }
//     IT {
//         vlan-id 20;
//         l3-interface irb.20;
//     }
// }
func ParseJunosVLANs(config string) ([]juniper.JuniperVLAN, error) {
	var vlans []juniper.JuniperVLAN

	// Find vlans block
	vlansBlock := extractJunosBlock(config, "vlans")
	if vlansBlock == "" {
		return vlans, nil
	}

	// Parse each VLAN within the vlans block
	vlanPattern := regexp.MustCompile(`(\S+)\s*\{([^}]+)\}`)
	matches := vlanPattern.FindAllStringSubmatch(vlansBlock, -1)

	for _, match := range matches {
		if len(match) < 3 {
			continue
		}

		vlanName := match[1]
		vlanBody := match[2]

		vlan := juniper.JuniperVLAN{
			VLANName:    vlanName,
			SourceBlock: fmt.Sprintf("vlans %s", vlanName),
		}

		// Parse VLAN ID
		if vlanIDMatch := regexp.MustCompile(`vlan-id\s+(\d+);`).FindStringSubmatch(vlanBody); vlanIDMatch != nil {
			vlanID, _ := strconv.Atoi(vlanIDMatch[1])
			vlan.VLANID = vlanID
		}

		// Parse description
		if descMatch := regexp.MustCompile(`description\s+"([^"]+)";`).FindStringSubmatch(vlanBody); descMatch != nil {
			vlan.Description = descMatch[1]
		} else if descMatch := regexp.MustCompile(`description\s+(\S+);`).FindStringSubmatch(vlanBody); descMatch != nil {
			vlan.Description = descMatch[1]
		}

		// Parse L3 interface
		if l3Match := regexp.MustCompile(`l3-interface\s+(\S+);`).FindStringSubmatch(vlanBody); l3Match != nil {
			vlan.L3Interface = l3Match[1]
		}

		// Compute hash
		blockText := fmt.Sprintf("%s {\n%s\n}", vlanName, vlanBody)
		vlan.SourceSnippetHash = computeSnippetHash(blockText)

		vlans = append(vlans, vlan)
	}

	return vlans, nil
}

// ParseJunosInterfaces parses interface configurations
//
// Junos Interface Format:
//
// interfaces {
//     ge-0/0/0 {
//         description "Access Port";
//         unit 0 {
//             family ethernet-switching {
//                 port-mode access;
//                 vlan {
//                     members Engineering;
//                 }
//             }
//         }
//     }
// }
func ParseJunosInterfaces(config string) ([]juniper.JuniperInterface, error) {
	var interfaces []juniper.JuniperInterface

	// Find interfaces block
	interfacesBlock := extractJunosBlock(config, "interfaces")
	if interfacesBlock == "" {
		return interfaces, nil
	}

	// Match physical interfaces (ge-, xe-, et-, ae-)
	ifacePattern := regexp.MustCompile(`(ge-[\d/]+|xe-[\d/]+|et-[\d/]+|ae\d+)\s*\{`)
	lines := strings.Split(interfacesBlock, "\n")

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		if matches := ifacePattern.FindStringSubmatch(line); matches != nil {
			ifaceName := matches[1]

			// Extract interface block (handle nested braces)
			ifaceBlock, endIdx := extractNestedBlock(lines, i)
			i = endIdx

			iface := parseJunosInterface(ifaceName, ifaceBlock)
			interfaces = append(interfaces, iface)
		}
	}

	return interfaces, nil
}

// parseJunosInterface parses a single interface block
func parseJunosInterface(ifaceName string, ifaceBlock string) juniper.JuniperInterface {
	iface := juniper.JuniperInterface{
		InterfaceName: ifaceName,
		Unit:          0, // Default
		SourceBlock:   fmt.Sprintf("interfaces %s", ifaceName),
	}

	// Parse description
	if descMatch := regexp.MustCompile(`description\s+"([^"]+)";`).FindStringSubmatch(ifaceBlock); descMatch != nil {
		iface.Description = descMatch[1]
	}

	// Parse disable
	if strings.Contains(ifaceBlock, "disable;") {
		iface.Disabled = true
	}

	// Parse AE membership (ether-options)
	if aeMatch := regexp.MustCompile(`802\.3ad\s+(\S+);`).FindStringSubmatch(ifaceBlock); aeMatch != nil {
		iface.AggregatedEthernetID = aeMatch[1]
	}

	// Parse ethernet-switching config within unit 0
	if strings.Contains(ifaceBlock, "family ethernet-switching") {
		switchingBlock := extractSubBlock(ifaceBlock, "family ethernet-switching")

		// Parse port mode
		if strings.Contains(switchingBlock, "port-mode access") {
			iface.PortMode = "access"
		} else if strings.Contains(switchingBlock, "port-mode trunk") {
			iface.PortMode = "trunk"
		}

		// Parse VLAN members (access or trunk)
		if vlanBlock := extractSubBlock(switchingBlock, "vlan"); vlanBlock != "" {
			// Access mode
			if membersMatch := regexp.MustCompile(`members\s+(\S+);`).FindStringSubmatch(vlanBlock); membersMatch != nil {
				iface.AccessVLAN = membersMatch[1]
			}
			// Trunk mode - multiple members
			if membersMatch := regexp.MustCompile(`members\s+\[([\s\S]+?)\];`).FindStringSubmatch(vlanBlock); membersMatch != nil {
				members := strings.Fields(membersMatch[1])
				iface.TrunkAllowedVLANs = members
			}
		}

		// Parse native VLAN
		if nativeMatch := regexp.MustCompile(`native-vlan-id\s+(\S+);`).FindStringSubmatch(switchingBlock); nativeMatch != nil {
			iface.TrunkNativeVLAN = nativeMatch[1]
		}

		// Parse DHCP security (trust)
		if strings.Contains(switchingBlock, "dhcp-trusted") {
			iface.DHCPSnoopingTrusted = true
		}

		// Parse ARP inspection (trust)
		if strings.Contains(switchingBlock, "arp-inspection") && strings.Contains(switchingBlock, "trusted") {
			iface.ARPInspectionTrusted = true
		}

		// Parse IP source guard
		if strings.Contains(switchingBlock, "ip-source-guard") {
			iface.IPSourceGuardEnabled = true
		}
	}

	// Parse 802.1X
	if strings.Contains(ifaceBlock, "dot1x") || strings.Contains(ifaceBlock, "802.1x") {
		iface.Dot1XEnabled = true
	}

	// Parse MAC authentication (MAB)
	if strings.Contains(ifaceBlock, "mac-radius") {
		iface.MABEnabled = true
	}

	// Compute hash
	iface.SourceSnippetHash = computeSnippetHash(ifaceBlock)

	return iface
}

// ParseJunosAggregatedEthernet parses AE (LAG) configurations
func ParseJunosAggregatedEthernet(config string) ([]juniper.JuniperAE, error) {
	var aes []juniper.JuniperAE

	// Find interfaces block and extract AE interfaces
	interfacesBlock := extractJunosBlock(config, "interfaces")
	if interfacesBlock == "" {
		return aes, nil
	}

	// Match AE interfaces
	aePattern := regexp.MustCompile(`(ae\d+)\s*\{`)
	lines := strings.Split(interfacesBlock, "\n")

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		if matches := aePattern.FindStringSubmatch(line); matches != nil {
			aeName := matches[1]

			// Extract AE block
			aeBlock, endIdx := extractNestedBlock(lines, i)
			i = endIdx

			ae := parseJunosAE(aeName, aeBlock, config)
			aes = append(aes, ae)
		}
	}

	return aes, nil
}

// parseJunosAE parses a single AE interface block
func parseJunosAE(aeName string, aeBlock string, fullConfig string) juniper.JuniperAE {
	ae := juniper.JuniperAE{
		AEName:      aeName,
		Unit:        0,
		SourceBlock: fmt.Sprintf("interfaces %s", aeName),
	}

	// Parse description
	if descMatch := regexp.MustCompile(`description\s+"([^"]+)";`).FindStringSubmatch(aeBlock); descMatch != nil {
		ae.Description = descMatch[1]
	}

	// Parse LACP mode
	if strings.Contains(aeBlock, "lacp") {
		if strings.Contains(aeBlock, "active") {
			ae.LACPMode = "active"
		} else if strings.Contains(aeBlock, "passive") {
			ae.LACPMode = "passive"
		}
	}

	// Parse ethernet-switching config
	if strings.Contains(aeBlock, "family ethernet-switching") {
		switchingBlock := extractSubBlock(aeBlock, "family ethernet-switching")

		if strings.Contains(switchingBlock, "port-mode access") {
			ae.PortMode = "access"
		} else if strings.Contains(switchingBlock, "port-mode trunk") {
			ae.PortMode = "trunk"
		}

		// Parse VLAN members
		if vlanBlock := extractSubBlock(switchingBlock, "vlan"); vlanBlock != "" {
			if membersMatch := regexp.MustCompile(`members\s+(\S+);`).FindStringSubmatch(vlanBlock); membersMatch != nil {
				ae.AccessVLAN = membersMatch[1]
			}
			if membersMatch := regexp.MustCompile(`members\s+\[([\s\S]+?)\];`).FindStringSubmatch(vlanBlock); membersMatch != nil {
				members := strings.Fields(membersMatch[1])
				ae.TrunkAllowedVLANs = members
			}
		}
	}

	// Find member interfaces
	ae.MemberInterfaces = findAEMembers(fullConfig, aeName)

	ae.SourceSnippetHash = computeSnippetHash(aeBlock)

	return ae
}

// ParseJunosGlobalControls parses global L2 security controls
func ParseJunosGlobalControls(config string) (*juniper.JuniperGlobalL2Controls, error) {
	controls := &juniper.JuniperGlobalL2Controls{}

	// Parse DHCP security
	if dhcpBlock := extractJunosBlock(config, "ethernet-switching-options dhcp-security"); dhcpBlock != "" {
		controls.DHCPSecurityEnabled = true
	}

	// Parse ARP inspection
	if arpBlock := extractJunosBlock(config, "ethernet-switching-options arp-inspection"); arpBlock != "" {
		controls.ARPInspectionEnabled = true
	}

	// Parse 802.1X
	if strings.Contains(config, "access profile") || strings.Contains(config, "dot1x") {
		controls.Dot1XEnabled = true
	}

	controls.SourceBlock = "global L2 controls"
	controls.SourceSnippetHash = computeSnippetHash("global controls")

	return controls, nil
}

// ParseJunosIRBs parses IRB (Integrated Routing and Bridging) interfaces
func ParseJunosIRBs(config string) ([]juniper.JuniperIRB, error) {
	var irbs []juniper.JuniperIRB

	// Find IRB interfaces
	interfacesBlock := extractJunosBlock(config, "interfaces")
	if interfacesBlock == "" {
		return irbs, nil
	}

	irbPattern := regexp.MustCompile(`irb\s*\{`)
	if !irbPattern.MatchString(interfacesBlock) {
		return irbs, nil
	}

	// Extract IRB block
	irbBlock := extractSubBlock(interfacesBlock, "irb")

	// Parse unit blocks
	unitPattern := regexp.MustCompile(`unit\s+(\d+)\s*\{`)
	lines := strings.Split(irbBlock, "\n")

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		if matches := unitPattern.FindStringSubmatch(line); matches != nil {
			unitNum, _ := strconv.Atoi(matches[1])

			unitBlock, endIdx := extractNestedBlock(lines, i)
			i = endIdx

			irb := juniper.JuniperIRB{
				IRBName:     fmt.Sprintf("irb.%d", unitNum),
				Unit:        unitNum,
				VLANID:      unitNum, // Usually matches VLAN ID
				SourceBlock: fmt.Sprintf("interfaces irb unit %d", unitNum),
			}

			// Parse description
			if descMatch := regexp.MustCompile(`description\s+"([^"]+)";`).FindStringSubmatch(unitBlock); descMatch != nil {
				irb.Description = descMatch[1]
			}

			// Parse IP address
			if ipMatch := regexp.MustCompile(`address\s+([\d.]+/\d+);`).FindStringSubmatch(unitBlock); ipMatch != nil {
				irb.IPAddress = ipMatch[1]
			}

			irb.SourceSnippetHash = computeSnippetHash(unitBlock)

			irbs = append(irbs, irb)
		}
	}

	return irbs, nil
}

// Helper functions

func parseJunosHostname(config string) string {
	if match := regexp.MustCompile(`set system host-name\s+(\S+)`).FindStringSubmatch(config); match != nil {
		return match[1]
	}
	if match := regexp.MustCompile(`host-name\s+(\S+);`).FindStringSubmatch(config); match != nil {
		return match[1]
	}
	return ""
}

func parseJunosVersion(config string) string {
	// Version usually in comments
	lines := strings.Split(config, "\n")
	for _, line := range lines {
		if strings.Contains(line, "JUNOS") || strings.Contains(line, "Junos:") {
			return strings.TrimSpace(line)
		}
	}
	return ""
}

func extractJunosBlock(config, blockName string) string {
	pattern := regexp.MustCompile(regexp.QuoteMeta(blockName) + `\s*\{`)
	loc := pattern.FindStringIndex(config)
	if loc == nil {
		return ""
	}

	start := loc[1]
	braceCount := 1
	i := start

	for i < len(config) && braceCount > 0 {
		if config[i] == '{' {
			braceCount++
		} else if config[i] == '}' {
			braceCount--
		}
		i++
	}

	return config[start:i-1]
}

func extractSubBlock(text, blockName string) string {
	pattern := regexp.MustCompile(regexp.QuoteMeta(blockName) + `\s*\{`)
	loc := pattern.FindStringIndex(text)
	if loc == nil {
		return ""
	}

	start := loc[1]
	braceCount := 1
	i := start

	for i < len(text) && braceCount > 0 {
		if text[i] == '{' {
			braceCount++
		} else if text[i] == '}' {
			braceCount--
		}
		i++
	}

	return text[start:i-1]
}

func extractNestedBlock(lines []string, startIdx int) (string, int) {
	var blockLines []string
	braceCount := 0
	i := startIdx

	for i < len(lines) {
		line := lines[i]
		blockLines = append(blockLines, line)

		braceCount += strings.Count(line, "{")
		braceCount -= strings.Count(line, "}")

		i++
		if braceCount == 0 {
			break
		}
	}

	return strings.Join(blockLines, "\n"), i
}

func findAEMembers(config, aeName string) []string {
	var members []string
	pattern := regexp.MustCompile(`(ge-[\d/]+|xe-[\d/]+|et-[\d/]+)[\s\S]+?802\.3ad\s+` + regexp.QuoteMeta(aeName))

	matches := pattern.FindAllStringSubmatch(config, -1)
	for _, match := range matches {
		if len(match) > 1 {
			members = append(members, match[1])
		}
	}

	return members
}

func computeConfigHash(config string) string {
	hash := sha256.Sum256([]byte(config))
	return fmt.Sprintf("sha256:%x", hash)
}

func computeSnippetHash(snippet string) string {
	normalized := strings.TrimSpace(snippet)
	hash := sha256.Sum256([]byte(normalized))
	return fmt.Sprintf("sha256:%x", hash)
}

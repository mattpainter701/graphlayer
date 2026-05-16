package eos

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mattpainter701/graphlayer/platform/platform/models"
)

// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |O|p|e|r|a|t|i|o|n|a|l| |P|a|r|s|e|r|s|
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
/*
 * The following functions parse operational command outputs from Arista EOS devices.
 * These supplement config-based parsing with runtime state for L2/L3 topology visibility.
 */

// ===========================
// LLDP Neighbor Parsing
// ===========================

// ParseEOSLLDPNeighbors parses "show lldp neighbors detail" output
func ParseEOSLLDPNeighbors(output string) []models.EOSLLDPNeighbor {
	var neighbors []models.EOSLLDPNeighbor
	lines := strings.Split(output, "\n")

	var current *models.EOSLLDPNeighbor
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.Contains(line, "Interface:") && strings.Contains(line, "Port description:") {
			// "Interface: Ethernet1/1, Port description: <desc>"
			parts := strings.Split(line, ",")
			if strings.HasPrefix(parts[0], "Interface:") {
				if current != nil {
					neighbors = append(neighbors, *current)
				}
				current = &models.EOSLLDPNeighbor{}
				current.LocalInterface = strings.TrimSpace(strings.TrimPrefix(parts[0], "Interface:"))
			}
		} else if strings.HasPrefix(line, "Local Interface:") {
			if current != nil {
				neighbors = append(neighbors, *current)
			}
			current = &models.EOSLLDPNeighbor{}
			current.LocalInterface = extractValue(line, ":")
		} else if current != nil {
			if strings.HasPrefix(line, "Chassis ID:") {
				current.ChassisID = extractValue(line, ":")
			} else if strings.HasPrefix(line, "Port ID:") {
				current.PortID = extractValue(line, ":")
			} else if strings.HasPrefix(line, "System Name:") || strings.HasPrefix(line, "System name:") {
				current.SystemName = extractValue(line, ":")
			} else if strings.HasPrefix(line, "System Description:") || strings.HasPrefix(line, "System description:") {
				current.SystemDesc = extractValue(line, ":")
			} else if strings.HasPrefix(line, "System Capabilities:") || strings.HasPrefix(line, "Capabilities:") {
				caps := extractValue(line, ":")
				current.Capabilities = strings.Split(caps, ",")
			} else if strings.HasPrefix(line, "Management Address:") || strings.HasPrefix(line, "Management address:") {
				current.ManagementAddr = extractValue(line, ":")
			} else if strings.HasPrefix(line, "TTL:") {
				t := strings.TrimSpace(strings.TrimPrefix(line, "TTL:"))
				if v, err := strconv.Atoi(t); err == nil {
					current.TTL = v
				}
			}
		}
	}

	if current != nil {
		neighbors = append(neighbors, *current)
	}
	return neighbors
}

// extractValue returns the value after the last colon separator
func extractValue(line, sep string) string {
	parts := strings.SplitN(line, sep, 2)
	if len(parts) >= 2 {
		return strings.TrimSpace(parts[1])
	}
	return ""
}

// ===========================
// MAC Address Table Parsing
// ===========================

// ParseEOSMACAddressTable parses "show mac address-table" output
// EOS Format:
//   Vlan    Mac Address       Type        Ports      Moves   Last Move
//   10      0004.4d41.4e31    DYNAMIC     Et1/1      1       0:01:23 ago
func ParseEOSMACAddressTable(output string, collectionTime time.Time) []models.EOSMACEntry {
	var entries []models.EOSMACEntry
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Skip headers, separators, and totals
		if line == "" || strings.HasPrefix(line, "Mac Address Table") ||
			strings.HasPrefix(line, "---") || strings.Contains(line, "Vlan") && strings.Contains(line, "Mac Address") ||
			strings.HasPrefix(line, "Total") || strings.HasPrefix(line, "Multicast") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		vlanStr := fields[0]
		macAddr := fields[1]
		entryType := strings.ToUpper(fields[2])
		port := fields[3]

		vlanID, err := strconv.Atoi(vlanStr)
		if err != nil {
			continue
		}

		// Normalize MAC (EOS uses Cisco-style format)
		normalizedMAC := normalizeEOSMAC(macAddr)

		entry := models.EOSMACEntry{
			MAC:          normalizedMAC,
			VLANID:       vlanID,
			Type:         entryType,
			Port:         port,
			CollectionTS: collectionTime.Format(time.RFC3339),
		}

		if len(fields) > 4 {
			entry.LastMove = strings.Join(fields[4:], " ")
		}
		entries = append(entries, entry)
	}
	return entries
}

// normalizeEOSMAC converts EOS MAC format (0004.4d41.4e31) to standard (00:04:4D:41:4E:31)
func normalizeEOSMAC(eosMAC string) string {
	clean := strings.ReplaceAll(eosMAC, ".", "")
	if strings.Contains(clean, ":") {
		return strings.ToUpper(clean)
	}
	if len(clean) == 12 {
		return strings.ToUpper(
			clean[0:2] + ":" + clean[2:4] + ":" + clean[4:6] + ":" +
				clean[6:8] + ":" + clean[8:10] + ":" + clean[10:12],
		)
	}
	return strings.ToUpper(eosMAC)
}

// ===========================
// ARP Table Parsing
// ===========================

// ParseEOSARPTable parses "show ip arp" output
// EOS Format:
//   Address         Age (min)  Hardware Addr      Interface      VLAN
//   10.0.0.1         0         0004.4d41.4e31     Vlan10         10
func ParseEOSARPTable(output string, collectionTime time.Time) []models.EOSARPEntry {
	var entries []models.EOSARPEntry
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Address") || strings.HasPrefix(line, "---") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		entry := models.EOSARPEntry{
			IP:           fields[0],
			MAC:          normalizeEOSMAC(fields[2]),
			Interface:    fields[3],
			Type:         "dynamic",
			CollectionTS: collectionTime.Format(time.RFC3339),
		}

		// Parse age
		if age, err := strconv.Atoi(fields[1]); err == nil {
			entry.AgingSeconds = age * 60
		}

		// VLAN (optional 5th field)
		if len(fields) >= 5 {
			if v, err := strconv.Atoi(fields[4]); err == nil {
				entry.VLAN = v
			}
		}

		// Check if static
		for _, f := range fields {
			if strings.EqualFold(f, "static") || strings.EqualFold(f, "permanent") {
				entry.Type = "static"
				break
			}
		}

		entries = append(entries, entry)
	}
	return entries
}

// ===========================
// Routing Table Parsing
// ===========================

// ParseEOSRouteTable parses "show ip route" output
// EOS Format:
//   C        10.0.0.0/24 is directly connected, Vlan10
//   O        10.0.1.0/24 [110/20] via 10.0.0.1, Ethernet1/1
//   B        10.0.2.0/24 [200/0] via 10.0.0.2, Ethernet1/2
//   S*       0.0.0.0/0 [1/0] via 10.0.0.254, Management1
func ParseEOSRouteTable(output string, collectionTime time.Time) []models.EOSRouteEntry {
	var entries []models.EOSRouteEntry
	lines := strings.Split(output, "\n")

	routeCode := map[string]string{
		"C": "connected",
		"S": "static",
		"O": "ospf",
		"B": "bgp",
		"R": "rip",
		"I": "isis",
		"K": "kernel",
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Codes:") ||
			strings.HasPrefix(line, "Gateway") || strings.HasPrefix(line, "---") ||
			strings.HasPrefix(line, "Vrf") {
			continue
		}

		// Parse route code prefix (e.g., "C ", "O ", "B*", "S*")
		re := regexp.MustCompile(`^([A-Z])\*?\s+(.+)$`)
		matches := re.FindStringSubmatch(line)
		if matches == nil {
			continue
		}

		code := matches[1]
		rest := matches[2]

		entry := models.EOSRouteEntry{
			Protocol:    routeCode[code],
			CollectionTS: collectionTime.Format(time.RFC3339),
			IsActive:    true,
		}
		if strings.Contains(line, "*") {
			entry.IsBest = true
		}

		// Extract prefix: e.g. "10.0.0.0/24" or "0.0.0.0/0"
		parts := strings.SplitN(rest, " ", 2)
		if len(parts) >= 1 {
			prefixParts := strings.Split(parts[0], "/")
			if len(prefixParts) == 2 {
				entry.Prefix = prefixParts[0]
				if v, err := strconv.Atoi(prefixParts[1]); err == nil {
					entry.PrefixLen = v
				}
			}
		}

		if len(parts) < 2 {
			entries = append(entries, entry)
			continue
		}

		suffix := parts[1]

		// "is directly connected, <interface>"
		if strings.Contains(suffix, "directly connected") {
			if idx := strings.LastIndex(suffix, ","); idx >= 0 {
				entry.Interface = strings.TrimSpace(suffix[idx+1:])
			} else {
				entry.Interface = strings.TrimSpace(strings.ReplaceAll(suffix, "is directly connected", ""))
			}
			entry.Metric = 0
		} else if strings.Contains(suffix, "via ") {
			// e.g. "[110/20] via 10.0.0.1, Ethernet1/1"
			// Extract [distance/metric]
			if idx := strings.Index(suffix, "["); idx >= 0 {
				if endIdx := strings.Index(suffix[idx:], "]"); endIdx >= 0 {
					dm := strings.TrimPrefix(suffix[idx:idx+endIdx], "[")
					dmParts := strings.Split(dm, "/")
					if len(dmParts) >= 1 {
						if v, err := strconv.Atoi(dmParts[0]); err == nil {
							entry.Distance = v
						}
					}
					if len(dmParts) >= 2 {
						if v, err := strconv.Atoi(dmParts[1]); err == nil {
							entry.Metric = v
						}
					}
				}
			}

			// Extract next-hop
			if idx := strings.Index(suffix, "via "); idx >= 0 {
				afterVia := suffix[idx+4:]
				viaParts := strings.SplitN(afterVia, ",", 2)
				entry.NextHop = strings.TrimSpace(viaParts[0])
				if len(viaParts) >= 2 {
					entry.Interface = strings.TrimSpace(viaParts[1])
				}
			}
		}

		entries = append(entries, entry)
	}
	return entries
}

// ===========================
// Interface Status Parsing
// ===========================

// ParseEOSInterfaceStatus parses "show interfaces status" output
// EOS Format:
//   Port      Name     Status       Vlan    Duplex   Speed  Type
//   Et1       PoC-to-1 connected    1       a-full   10G   SFP+
//   Et2       PoC-to-2 notconnect   1       a-full   10G   SFP+
func ParseEOSInterfaceStatus(output string) []models.EOSInterfaceStatus {
	var entries []models.EOSInterfaceStatus
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Port") ||
			strings.HasPrefix(line, "---") || strings.HasPrefix(line, "Device") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}

		entry := models.EOSInterfaceStatus{
			Name:   fields[0],
			Status: fields[2],
			VLAN:   fields[3],
			Duplex: fields[4],
		}

		if len(fields) > 5 {
			entry.Speed = fields[5]
		}
		if len(fields) > 6 {
			entry.Type = strings.Join(fields[6:], " ")
		}

		// Name is usually in the 2nd position on multi-word tokens
		// "Port" header: Port    Name  Status ... -> Name is field[1]
		if len(fields) >= 2 {
			entry.Name = fields[0]
			if !strings.HasPrefix(entry.Name, "Et") &&
				!strings.HasPrefix(entry.Name, "Ma") &&
				!strings.HasPrefix(entry.Name, "Vl") &&
				!strings.HasPrefix(entry.Name, "Po") {
				// If first field doesn't look like a port name, skip this line
				continue
			}
		}

		// Try to extract the Name column (between Port and Status)
		// EOS sometimes has:  Et1   desc-here   connected  trunk  a-full  10G
		if len(fields) >= 5 {
			// field[1] might be the description/name
			possibleName := fields[1]
			if possibleName != "connected" && possibleName != "notconnect" &&
				possibleName != "disabled" && possibleName != "errdisabled" {
				entry.Description = possibleName
			}
		}

		// Detect error state
		if entry.Status == "errdisabled" {
			entry.ErrorState = fields[2]
		}

		entries = append(entries, entry)
	}
	return entries
}

// ===========================
// MLAG Operational Parsing
// ===========================

// ParseEOSMLAGStatus parses "show mlag" output
func ParseEOSMLAGStatus(output string) models.EOSMLAGOperational {
	state := models.EOSMLAGOperational{}
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "---") {
			continue
		}

		switch {
		case strings.HasPrefix(line, "domain-id:"):
			state.DomainID = extractValue(line, ":")
		case strings.HasPrefix(line, "state:"):
			s := strings.Fields(line)
			if len(s) >= 2 {
				state.State = s[1] // "Active" or "Inactive"
			}
		case strings.HasPrefix(line, "peer status:"):
			state.PeerStatus = extractValue(line, ":")
		case strings.HasPrefix(line, "peer-link status:"):
			state.PeerLinkStatus = extractValue(line, ":")
		case strings.HasPrefix(line, "negotiation done:"):
			// just note it's active
		case strings.Contains(line, "mlag") && strings.Contains(line, "ports:"):
			fields := strings.Fields(line)
			if len(fields) >= 1 {
				if v, err := strconv.Atoi(fields[0]); err == nil {
					state.MLAGPortCount = v
				}
			}
		case strings.Contains(line, "inactive"):
			fields := strings.Fields(line)
			for _, f := range fields {
				if v, err := strconv.Atoi(f); err == nil {
					state.InactivePorts = v
					break
				}
			}
		case strings.Contains(line, "last configuration change"):
			state.LastConfigChange = extractValue(line, ":")
		}
	}
	return state
}

// ===========================
// OSPF Neighbor Parsing
// ===========================

// OSPFNeighbor represents an OSPF neighbor
type OSPFNeighbor struct {
	NeighborID   string
	Priority     int
	State        string
	DeadTime     string
	Interface    string
	Area         string
}

// ParseEOSOSPFNeighbors parses "show ip ospf neighbor" output
func ParseEOSOSPFNeighbors(output string) []OSPFNeighbor {
	var neighbors []OSPFNeighbor
	lines := strings.Split(output, "\n")

	// Skip first 2 lines (header)
	headerSkipped := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "---") {
			continue
		}
		if !headerSkipped && strings.Contains(line, "Neighbor") {
			headerSkipped = true
			continue
		}
		if !headerSkipped {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}

		n := OSPFNeighbor{
			NeighborID: fields[0],
			State:      fields[2],
			DeadTime:   fields[3],
			Interface:  fields[4],
		}
		if v, err := strconv.Atoi(fields[1]); err == nil {
			n.Priority = v
		}
		if len(fields) > 5 {
			n.Area = fields[5]
		}
		neighbors = append(neighbors, n)
	}
	return neighbors
}

// ===========================
// BGP Summary Parsing
// ===========================

// BGPNeighbor represents a BGP peer
type BGPNeighbor struct {
	PeerIP         string
	ASN            int
	State          string
	Prefixes       int
	UpDown         string
	Description    string
}

// ParseEOSBGPNeighbors parses "show ip bgp summary" output
func ParseEOSBGPNeighbors(output string) []BGPNeighbor {
	var neighbors []BGPNeighbor
	lines := strings.Split(output, "\n")

	inTable := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "---") {
			continue
		}
		if strings.Contains(line, "Neighbor") && strings.Contains(line, "AS") {
			inTable = true
			continue
		}
		if strings.HasPrefix(line, "Total") {
			continue
		}
		if !inTable {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}

		n := BGPNeighbor{
			PeerIP: fields[0],
		}
		if v, err := strconv.Atoi(fields[1]); err == nil {
			n.ASN = v
		}
		n.UpDown = fields[3]
		if v, err := strconv.Atoi(fields[4]); err == nil {
			n.Prefixes = v
		}
		if len(fields) > 5 {
			n.State = fields[5]
		}
		if len(fields) > 6 {
			n.Description = strings.Join(fields[6:], " ")
		}
		neighbors = append(neighbors, n)
	}
	return neighbors
}

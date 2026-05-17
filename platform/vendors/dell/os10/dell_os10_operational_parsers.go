package os10

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mattpainter701/graphlayer/platform/models"
)

// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |O|p|e|r|a|t|i|o|n|a|l| |P|a|r|s|e|r|s|
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
/*
 * The following functions parse operational command outputs from Dell OS10 devices.
 * These supplement config-based parsing with runtime state for L2/L3 topology visibility.
 */

// ===========================
// LLDP Neighbor Parsing
// ===========================

// ParseOS10LLDPNeighbors parses "show lldp neighbors detail" output
// OS10 Format:
//   -------------------------------------------------------------------------------
//   Local Interface ethernet1/1/1 has 1 neighbor
//     Chassis ID:    00:11:22:33:44:55
//     Port ID:       ethernet1/1/1
//     System Name:   dist01.example.com
//     System Desc:   Dell EMC Networking OS10
//     Capabilities:  Bridge Router
//     Mgmt Address:  10.0.0.1
//     TTL:           120
func ParseOS10LLDPNeighbors(output string) []models.OS10LLDPNeighbor {
	var neighbors []models.OS10LLDPNeighbor
	lines := strings.Split(output, "\n")

	var current *models.OS10LLDPNeighbor
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "---") {
			continue
		}

		if strings.HasPrefix(line, "Local Interface") && strings.Contains(line, "has") {
			if current != nil {
				neighbors = append(neighbors, *current)
			}
			current = &models.OS10LLDPNeighbor{}
			// "Local Interface ethernet1/1/1 has 1 neighbor"
			parts := strings.SplitN(line, " ", 3)
			if len(parts) >= 3 {
				current.LocalInterface = parts[2]
				if idx := strings.Index(current.LocalInterface, " has"); idx >= 0 {
					current.LocalInterface = strings.TrimSpace(current.LocalInterface[:idx])
				}
			}
		} else if current != nil {
			switch {
			case strings.HasPrefix(line, "Chassis ID:"):
				current.ChassisID = extractValueOS10(line, ":")
			case strings.HasPrefix(line, "Port ID:"):
				current.PortID = extractValueOS10(line, ":")
			case strings.HasPrefix(line, "System Name:"):
				current.SystemName = extractValueOS10(line, ":")
			case strings.HasPrefix(line, "System Desc:"):
				current.SystemDesc = extractValueOS10(line, ":")
			case strings.HasPrefix(line, "Capabilities:"):
				caps := extractValueOS10(line, ":")
				current.Capabilities = strings.Fields(caps)
			case strings.HasPrefix(line, "Mgmt Address:") || strings.HasPrefix(line, "Management Address:"):
				current.ManagementAddr = extractValueOS10(line, ":")
			case strings.HasPrefix(line, "TTL:"):
				t := extractValueOS10(line, ":")
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

// extractValueOS10 returns the value after a colon separator
func extractValueOS10(line, sep string) string {
	parts := strings.SplitN(line, sep, 2)
	if len(parts) >= 2 {
		return strings.TrimSpace(parts[1])
	}
	return ""
}

// ===========================
// MAC Address Table Parsing
// ===========================

// ParseOS10MACAddressTable parses "show mac address-table" output
// OS10 Format:
//   VlanId   Mac Address          Type        Interface
//   10       00:11:22:33:44:55    dynamic     ethernet1/1/1
//   10       00:11:22:33:44:aa    static      port-channel10
func ParseOS10MACAddressTable(output string, collectionTime time.Time) []models.OS10MACEntry {
	var entries []models.OS10MACEntry
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Mac Address Table") ||
			strings.HasPrefix(line, "---") ||
			(strings.Contains(line, "VlanId") && strings.Contains(line, "Mac Address")) ||
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

		normalizedMAC := normalizeOS10MAC(macAddr)

		entry := models.OS10MACEntry{
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

// normalizeOS10MAC normalizes various MAC formats to standard colon-separated
func normalizeOS10MAC(mac string) string {
	clean := strings.ReplaceAll(mac, ".", "")
	clean = strings.ReplaceAll(clean, "-", "")
	if strings.Contains(clean, ":") {
		return strings.ToUpper(clean)
	}
	if len(clean) == 12 {
		return strings.ToUpper(
			clean[0:2] + ":" + clean[2:4] + ":" + clean[4:6] + ":" +
				clean[6:8] + ":" + clean[8:10] + ":" + clean[10:12],
		)
	}
	return strings.ToUpper(mac)
}

// ===========================
// ARP Table Parsing
// ===========================

// ParseOS10ARPTable parses "show ip arp" output
// OS10 Format:
//   Address         Hardware Address    Interface         Vlan
//   10.0.0.1        00:11:22:33:44:55   ethernet1/1/1     10
//   10.0.0.2        00:11:22:33:44:aa   vlan10            10
func ParseOS10ARPTable(output string, collectionTime time.Time) []models.OS10ARPEntry {
	var entries []models.OS10ARPEntry
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Address") || strings.HasPrefix(line, "---") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		entry := models.OS10ARPEntry{
			IP:           fields[0],
			MAC:          normalizeOS10MAC(fields[1]),
			Interface:    fields[2],
			Type:         "dynamic",
			CollectionTS: collectionTime.Format(time.RFC3339),
		}

		// VLAN (optional 4th or 5th field)
		if len(fields) >= 4 {
			if v, err := strconv.Atoi(fields[3]); err == nil {
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

// ParseOS10RouteTable parses "show ip route" output
// OS10 Format (similar to Cisco):
//   Codes: C - connected, S - static, O - OSPF, B - BGP
//
//   C    10.0.10.0/24 is directly connected, vlan10
//   O    10.0.1.0/24 [110/20] via 10.0.0.1, ethernet1/1/1
//   B    10.0.2.0/24 [200/0] via 10.0.0.2, ethernet1/1/2
//   S*   0.0.0.0/0 [1/0] via 192.168.1.1, mgmt1/1/1
func ParseOS10RouteTable(output string, collectionTime time.Time) []models.OS10RouteEntry {
	var entries []models.OS10RouteEntry
	lines := strings.Split(output, "\n")

	routeCode := map[string]string{
		"C": "connected",
		"S": "static",
		"O": "ospf",
		"B": "bgp",
		"R": "rip",
		"I": "isis",
		"K": "kernel",
		"D": "eigrp",
		"L": "local",
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

		entry := models.OS10RouteEntry{
			Protocol:     routeCode[code],
			CollectionTS: collectionTime.Format(time.RFC3339),
			IsActive:     true,
		}
		if strings.Contains(line, "*") {
			entry.IsBest = true
		}

		// Extract prefix
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

// ParseOS10InterfaceStatus parses "show interfaces status" output
// OS10 Format:
//   Port           Description    Status   Speed    Duplex   Vlan  Type
//   ethernet1/1/1  PoC-to-dist01  up       10G      full     trunk  SFP+
//   ethernet1/1/2  PoC-to-dist02  up       10G      full     trunk  SFP+
//   ethernet1/1/3                 down     auto     auto     1      BASE-T
func ParseOS10InterfaceStatus(output string) []models.OS10InterfaceStatus {
	var entries []models.OS10InterfaceStatus
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

		entry := models.OS10InterfaceStatus{
			Name:   fields[0],
			Status: fields[2],
			Speed:  fields[3],
			Duplex: fields[4],
		}

		if !strings.HasPrefix(entry.Name, "ethernet") &&
			!strings.HasPrefix(strings.ToLower(entry.Name), "mgmt") &&
			!strings.HasPrefix(strings.ToLower(entry.Name), "port-channel") &&
			!strings.HasPrefix(strings.ToLower(entry.Name), "vlan") {
			continue
		}

		// Description in field[1] (may be empty)
		if len(fields) >= 2 && fields[1] != "up" && fields[1] != "down" && fields[1] != "admin-down" {
			entry.Description = fields[1]
		}

		// VLAN
		if len(fields) >= 6 {
			entry.VLAN = fields[5]
		}

		// Type
		if len(fields) >= 7 {
			entry.Type = strings.Join(fields[6:], " ")
		}

		entries = append(entries, entry)
	}
	return entries
}

// ===========================
// VLT Operational Parsing
// ===========================

// ParseOS10VLTStatus parses "show vlt <domain-id>" output
// OS10 Format:
//   Domain ID           : 1
//   Unit ID             : 1
//   Role                : primary
//   Version             : 2.1
//   Local System MAC    : 00:11:22:33:44:55
//   VLT MAC address     : 00:00:00:00:00:01
//   IP address          : fda5:74c8:b79e:1::1
//   Delay-Restore       : 90
//   Peer-Routing        : Enabled
//   Peer-Routing-Timeout: 0
//   VLTi Link Status    : up
//   Peer Status         : up
//   Peer-link port-channel 10 status: up
//   VLT Peer Unit ID    : 2
//   System MAC          : 00:11:22:33:44:aa
func ParseOS10VLTStatus(output string) models.OS10VLTOperational {
	state := models.OS10VLTOperational{}
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "---") {
			continue
		}

		lower := strings.ToLower(line)

		switch {
		case strings.HasPrefix(lower, "domain id"):
			state.DomainID = extractValueOS10(line, ":")
		case strings.HasPrefix(lower, "unit id"):
			if v, err := strconv.Atoi(extractValueOS10(line, ":")); err == nil {
				state.VLTUnitID = v
			}
		case strings.HasPrefix(lower, "role"):
			state.Role = extractValueOS10(line, ":")
		case strings.HasPrefix(lower, "version"):
			state.Version = extractValueOS10(line, ":")
		case strings.HasPrefix(lower, "local system mac"):
			state.LocalSystemMAC = extractValueOS10(line, ":")
		case strings.HasPrefix(lower, "vlt mac"):
			state.VLTMAC = extractValueOS10(line, ":")
		case strings.HasPrefix(lower, "system mac") && !strings.HasPrefix(lower, "local"):
			state.SystemMAC = extractValueOS10(line, ":")
		case strings.HasPrefix(lower, "delay-restore"):
			if v, err := strconv.Atoi(extractValueOS10(line, ":")); err == nil {
				state.DelayRestore = v
			}
		case strings.HasPrefix(lower, "peer-routing"):
			state.PeerRouting = strings.Contains(lower, "enabled")
		case strings.HasPrefix(lower, "vlti link status"):
			state.VLTiLinkStatus = extractValueOS10(line, ":")
		case strings.HasPrefix(lower, "peer status"):
			state.PeerStatus = extractValueOS10(line, ":")
		case strings.Contains(lower, "peer-link") && strings.Contains(lower, "status"):
			state.PeerLinkStatus = extractValueOS10(line, ":")
		case strings.Contains(lower, "last configuration change"):
			state.LastConfigChange = extractValueOS10(line, ":")
		}
	}

	// Determine overall state
	if state.PeerLinkStatus == "up" && state.PeerStatus == "up" {
		state.State = "active"
	} else {
		state.State = "inactive"
	}

	return state
}

// ===========================
// OSPF Neighbor Parsing
// ===========================

// OS10OSPFNeighbor represents an OSPF neighbor
type OS10OSPFNeighbor struct {
	NeighborID string
	Priority   int
	State      string
	DeadTime   string
	Interface  string
	Area       string
}

// ParseOS10OSPFNeighbors parses "show ip ospf neighbor" output
func ParseOS10OSPFNeighbors(output string) []OS10OSPFNeighbor {
	var neighbors []OS10OSPFNeighbor
	lines := strings.Split(output, "\n")

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

		n := OS10OSPFNeighbor{
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

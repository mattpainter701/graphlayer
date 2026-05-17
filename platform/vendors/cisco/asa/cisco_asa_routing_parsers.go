package cisco

import (
	"fmt"
	"github.com/mattpainter701/graphlayer/platform/models"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
)

// Cisco ASA Version Support for L3 Routing:
// - ASA 9.8.x (minimum recommended for production)
// - ASA 9.12.x (LTS - Long Term Support)
// - ASA 9.14.x
// - ASA 9.16.x (LTS)
// - ASA 9.18.x
// - ASA 9.20.x (latest as of 2025)
//
// Commands Used (via SSH CLI):
// - show route [vrf <name>] - Display routing table
// - show running-config router ospf - OSPF configuration
// - show ospf neighbor - OSPF neighbor table
// - show running-config router bgp - BGP configuration
// - show bgp summary - BGP peer summary
// - show bgp neighbors - BGP neighbor details
//
// Security Context Support: Yes (multi-context mode)
// VRF Support: Limited (ASA has VRF-lite support from 9.0+)

// ParserContext provides logging and metadata for ASA routing parsers
type ASARoutingParserContext struct {
	Vendor        string       // "cisco"
	Platform      string       // "asa"
	OSVersion     string       // e.g., "9.16(4)"
	Command       string       // CLI command executed
	CommandOutput string       // Raw command output
	Logger        *slog.Logger // Structured logger
}

// ParseASARoutingTable parses the output of "show route" command.
// This includes routes from all protocols: static, connected, OSPF, BGP, EIGRP.
//
// Example output:
// Codes: L - local, C - connected, S - static, R - RIP, M - mobile, B - BGP
//        D - EIGRP, EX - EIGRP external, O - OSPF, IA - OSPF inter area
//        N1 - OSPF NSSA external type 1, N2 - OSPF NSSA external type 2
//        E1 - OSPF external type 1, E2 - OSPF external type 2
//        i - IS-IS, su - IS-IS summary, L1 - IS-IS level-1, L2 - IS-IS level-2
//
// Gateway of last resort is 10.0.0.1 to network 0.0.0.0
//
// S*   0.0.0.0 0.0.0.0 [1/0] via 10.0.0.1, outside
// C    10.1.1.0 255.255.255.0 is directly connected, inside
// L    10.1.1.1 255.255.255.255 is directly connected, inside
// O    192.168.1.0 255.255.255.0 [110/20] via 10.1.1.2, 00:15:23, inside
// O IA 192.168.100.0 255.255.255.0 [110/30] via 10.1.1.2, 00:10:45, inside
// B    172.16.0.0 255.255.0.0 [200/0] via 10.1.1.3, 01:23:45
func ParseASARoutingTable(output string) ([]models.L3Route, error) {
	var routes []models.L3Route
	lines := strings.Split(output, "\n")

	// Regex patterns for different route types
	// ASA route format: <code> <prefix> <mask> [AD/metric] via <nexthop>, <time>, <interface>
	routePattern := regexp.MustCompile(`^([A-Z*]+)\s+(\d+\.\d+\.\d+\.\d+)\s+(\d+\.\d+\.\d+\.\d+)\s+(?:\[(\d+)/(\d+)\]\s+)?via\s+(\d+\.\d+\.\d+\.\d+)(?:,\s+(.+?))(?:,\s+(.+))?$`)
	connectedPattern := regexp.MustCompile(`^([CL])\s+(\d+\.\d+\.\d+\.\d+)\s+(\d+\.\d+\.\d+\.\d+)\s+is\s+directly\s+connected,\s+(.+)$`)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Codes:") || strings.HasPrefix(line, "Gateway of last resort") {
			continue
		}

		// Try connected/local route pattern first
		if matches := connectedPattern.FindStringSubmatch(line); matches != nil {
			route := models.L3Route{
				Prefix:    convertASANetmaskToCIDR(matches[2], matches[3]),
				NextHop:   "0.0.0.0", // Directly connected
				Interface: matches[4],
				Metric:    0,
				Metadata:  make(map[string]interface{}),
			}

			// Protocol determination
			if matches[1] == "C" {
				route.Protocol = "connected"
				route.AdminDistance = 0
			} else if matches[1] == "L" {
				route.Protocol = "local"
				route.AdminDistance = 0
				route.Metadata["route_type"] = "local_interface_address"
			}

			routes = append(routes, route)
			continue
		}

		// Try dynamic/static route pattern
		if matches := routePattern.FindStringSubmatch(line); matches != nil {
			route := models.L3Route{
				Prefix:   convertASANetmaskToCIDR(matches[2], matches[3]),
				NextHop:  matches[6],
				Metadata: make(map[string]interface{}),
			}

			// Parse admin distance and metric
			if matches[4] != "" {
				if ad, err := strconv.ParseUint(matches[4], 10, 32); err == nil {
					route.AdminDistance = uint32(ad)
				}
			}
			if matches[5] != "" {
				if metric, err := strconv.ParseUint(matches[5], 10, 32); err == nil {
					route.Metric = uint32(metric)
				}
			}

			// Parse interface (may be in matches[7] or matches[8])
			if matches[8] != "" {
				route.Interface = strings.TrimSpace(matches[8])
			} else if matches[7] != "" && !strings.Contains(matches[7], ":") {
				// If it doesn't look like a timestamp, it's the interface
				route.Interface = strings.TrimSpace(matches[7])
			}

			// Parse route age if present
			if matches[7] != "" && strings.Contains(matches[7], ":") {
				route.Metadata["age"] = matches[7]
			}

			// Protocol determination from route code
			routeCode := matches[1]
			route.Protocol = parseASARouteCode(routeCode)
			route.Metadata["route_code"] = routeCode

			routes = append(routes, route)
		}
	}

	return routes, nil
}

// ParseASAOSPFConfig parses OSPF configuration from "show running-config router ospf"
//
// Example output:
// router ospf 1
//  router-id 10.0.0.1
//  network 10.1.1.0 255.255.255.0 area 0
//  network 192.168.1.0 255.255.255.0 area 10
//  area 10 stub
//  log-adj-changes
//  redistribute static subnets route-map STATIC-TO-OSPF
//  redistribute connected subnets
//  default-information originate
func ParseASAOSPFConfig(output string) (*models.OSPFProcess, error) {
	lines := strings.Split(output, "\n")

	var ospfProcess *models.OSPFProcess
	areaMap := make(map[string]*models.OSPFArea)

	// Pattern to match process ID
	processPattern := regexp.MustCompile(`^router ospf (\d+)`)
	routerIDPattern := regexp.MustCompile(`^\s+router-id\s+(\d+\.\d+\.\d+\.\d+)`)
	networkPattern := regexp.MustCompile(`^\s+network\s+(\d+\.\d+\.\d+\.\d+)\s+(\d+\.\d+\.\d+\.\d+)\s+area\s+(\d+)`)
	areaStubPattern := regexp.MustCompile(`^\s+area\s+(\d+)\s+stub`)
	areaDefaultCostPattern := regexp.MustCompile(`^\s+area\s+(\d+)\s+default-cost\s+(\d+)`)
	redistributePattern := regexp.MustCompile(`^\s+redistribute\s+(\w+)`)

	for _, line := range lines {
		line = strings.TrimRight(line, "\r")

		// Match process ID
		if matches := processPattern.FindStringSubmatch(line); matches != nil {
			ospfProcess = &models.OSPFProcess{
				ProcessID: matches[1],
				Areas:     []models.OSPFArea{},
				Neighbors: []models.OSPFNeighbor{},
				Metadata:  make(map[string]interface{}),
			}
			continue
		}

		if ospfProcess == nil {
			continue
		}

		// Match router ID
		if matches := routerIDPattern.FindStringSubmatch(line); matches != nil {
			ospfProcess.RouterID = matches[1]
			continue
		}

		// Match network statements
		if matches := networkPattern.FindStringSubmatch(line); matches != nil {
			network := matches[1]
			netmask := matches[2]
			areaID := matches[3]

			// Create area if it doesn't exist
			if _, exists := areaMap[areaID]; !exists {
				areaMap[areaID] = &models.OSPFArea{
					AreaID:     areaID,
					AreaType:   "normal",
					Networks:   []string{},
					Interfaces: []string{},
				}
			}

			// Add network to area
			cidr := convertASANetmaskToCIDR(network, netmask)
			areaMap[areaID].Networks = append(areaMap[areaID].Networks, cidr)
			continue
		}

		// Match area stub configuration
		if matches := areaStubPattern.FindStringSubmatch(line); matches != nil {
			areaID := matches[1]
			if area, exists := areaMap[areaID]; exists {
				area.AreaType = "stub"
			} else {
				areaMap[areaID] = &models.OSPFArea{
					AreaID:   areaID,
					AreaType: "stub",
				}
			}
			continue
		}

		// Match area default-cost
		if matches := areaDefaultCostPattern.FindStringSubmatch(line); matches != nil {
			areaID := matches[1]
			cost := matches[2]
			if area, exists := areaMap[areaID]; exists {
				if area.Metadata == nil {
					area.Metadata = make(map[string]interface{})
				}
				area.Metadata["default_cost"] = cost
			}
			continue
		}

		// Match redistribution
		if matches := redistributePattern.FindStringSubmatch(line); matches != nil {
			protocol := strings.ToLower(matches[1])
			switch protocol {
			case "static":
				ospfProcess.RedistributeStatic = true
			case "connected":
				ospfProcess.RedistributeConnected = true
			case "bgp":
				ospfProcess.RedistributeBGP = true
			}
			continue
		}
	}

	if ospfProcess == nil {
		return nil, fmt.Errorf("no OSPF configuration found")
	}

	// Convert area map to slice
	for _, area := range areaMap {
		ospfProcess.Areas = append(ospfProcess.Areas, *area)
	}

	return ospfProcess, nil
}

// ParseASAOSPFNeighbors parses OSPF neighbor output from "show ospf neighbor"
//
// Example output:
// Neighbor ID     Pri   State           Dead Time   Address         Interface
// 10.0.0.2          1   FULL/BDR        00:00:35    10.1.1.2        inside
// 10.0.0.3        100   FULL/DR         00:00:38    10.2.2.3        dmz
// 10.0.0.4          0   FULL/  -        00:00:32    10.3.3.4        outside
func ParseASAOSPFNeighbors(output string) ([]models.OSPFNeighbor, error) {
	var neighbors []models.OSPFNeighbor
	lines := strings.Split(output, "\n")

	// Skip header lines
	dataStarted := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Skip header
		if strings.HasPrefix(line, "Neighbor ID") {
			dataStarted = true
			continue
		}

		if !dataStarted {
			continue
		}

		// Parse neighbor line
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}

		neighbor := models.OSPFNeighbor{
			NeighborID: fields[0],
			IPAddress:  fields[4],
			Interface:  fields[5],
			Metadata:   make(map[string]interface{}),
		}

		// Parse priority
		if priority, err := strconv.ParseUint(fields[1], 10, 32); err == nil {
			neighbor.Priority = uint32(priority)
		}

		// Parse state (format: "FULL/BDR" or "FULL/  -")
		stateParts := strings.Split(fields[2], "/")
		if len(stateParts) > 0 {
			neighbor.State = strings.TrimSpace(stateParts[0])
			if len(stateParts) > 1 {
				role := strings.TrimSpace(stateParts[1])
				if role != "-" {
					neighbor.Metadata["role"] = role
				}
			}
		}

		// Dead time
		neighbor.Metadata["dead_time"] = fields[3]

		neighbors = append(neighbors, neighbor)
	}

	return neighbors, nil
}

// ParseASABGPConfig parses BGP configuration from "show running-config router bgp"
//
// Example output:
// router bgp 65001
//  bgp router-id 10.0.0.1
//  bgp log-neighbor-changes
//  address-family ipv4 unicast
//   neighbor 203.0.113.1 remote-as 65100
//   neighbor 203.0.113.1 description ISP1
//   neighbor 198.51.100.1 remote-as 65200
//   neighbor 10.1.1.10 remote-as 65001
//   neighbor 10.1.1.10 route-reflector-client
//   network 10.0.0.0 mask 255.0.0.0
//   redistribute static
//   redistribute connected
//  exit-address-family
func ParseASABGPConfig(output string) (*models.BGPProcess, error) {
	lines := strings.Split(output, "\n")

	var bgpProcess *models.BGPProcess
	neighborMap := make(map[string]*models.BGPNeighbor)

	// Patterns
	processPattern := regexp.MustCompile(`^router bgp (\d+)`)
	routerIDPattern := regexp.MustCompile(`^\s+bgp router-id\s+(\d+\.\d+\.\d+\.\d+)`)
	neighborASPattern := regexp.MustCompile(`^\s+neighbor\s+(\d+\.\d+\.\d+\.\d+)\s+remote-as\s+(\d+)`)
	neighborDescPattern := regexp.MustCompile(`^\s+neighbor\s+(\d+\.\d+\.\d+\.\d+)\s+description\s+(.+)`)
	neighborRRPattern := regexp.MustCompile(`^\s+neighbor\s+(\d+\.\d+\.\d+\.\d+)\s+route-reflector-client`)
	redistributePattern := regexp.MustCompile(`^\s+redistribute\s+(\w+)`)

	for _, line := range lines {
		line = strings.TrimRight(line, "\r")

		// Match BGP process
		if matches := processPattern.FindStringSubmatch(line); matches != nil {
			asNum, _ := strconv.ParseUint(matches[1], 10, 32)
			bgpProcess = &models.BGPProcess{
				LocalAS:   uint32(asNum),
				Neighbors: []models.BGPNeighbor{},
				Metadata:  make(map[string]interface{}),
			}
			continue
		}

		if bgpProcess == nil {
			continue
		}

		// Match router ID
		if matches := routerIDPattern.FindStringSubmatch(line); matches != nil {
			bgpProcess.RouterID = matches[1]
			continue
		}

		// Match neighbor remote-as
		if matches := neighborASPattern.FindStringSubmatch(line); matches != nil {
			neighborIP := matches[1]
			remoteAS, _ := strconv.ParseUint(matches[2], 10, 32)

			if _, exists := neighborMap[neighborIP]; !exists {
				neighborMap[neighborIP] = &models.BGPNeighbor{
					NeighborID: neighborIP, // Will be updated if we get real neighbor ID
					IPAddress:  neighborIP,
					RemoteAS:   uint32(remoteAS),
					Metadata:   make(map[string]interface{}),
				}
			} else {
				neighborMap[neighborIP].RemoteAS = uint32(remoteAS)
			}
			continue
		}

		// Match neighbor description
		if matches := neighborDescPattern.FindStringSubmatch(line); matches != nil {
			neighborIP := matches[1]
			description := matches[2]

			if neighbor, exists := neighborMap[neighborIP]; exists {
				neighbor.Metadata["description"] = description
			}
			continue
		}

		// Match route-reflector-client
		if matches := neighborRRPattern.FindStringSubmatch(line); matches != nil {
			neighborIP := matches[1]
			if neighbor, exists := neighborMap[neighborIP]; exists {
				neighbor.Metadata["is_route_reflector_client"] = true
			}
			continue
		}

		// Match redistribution
		if matches := redistributePattern.FindStringSubmatch(line); matches != nil {
			protocol := strings.ToLower(matches[1])
			switch protocol {
			case "static":
				bgpProcess.RedistributeStatic = true
			case "connected":
				bgpProcess.RedistributeConnected = true
			case "ospf":
				bgpProcess.RedistributeOSPF = true
			}
			continue
		}
	}

	if bgpProcess == nil {
		return nil, fmt.Errorf("no BGP configuration found")
	}

	// Convert neighbor map to slice
	for _, neighbor := range neighborMap {
		bgpProcess.Neighbors = append(bgpProcess.Neighbors, *neighbor)
	}

	// Set route reflector ID if any neighbors are RR clients
	for _, neighbor := range bgpProcess.Neighbors {
		if isRRClient, ok := neighbor.Metadata["is_route_reflector_client"].(bool); ok && isRRClient {
			bgpProcess.RouteReflectorID = bgpProcess.RouterID
			break
		}
	}

	return bgpProcess, nil
}

// ParseASABGPSummary parses BGP peer summary from "show bgp summary"
//
// Example output:
// BGP router identifier 10.0.0.1, local AS number 65001
// BGP table version is 145, main routing table version 145
//
// Neighbor        V           AS MsgRcvd MsgSent   TblVer  InQ OutQ Up/Down  State/PfxRcd
// 203.0.113.1     4        65100    1234    1230      145    0    0 01:23:45        50
// 198.51.100.1    4        65200     567     563      145    0    0 00:45:12        25
// 10.1.1.10       4        65001     890     885      145    0    0 02:15:30        100
func ParseASABGPSummary(output string) (map[string]models.BGPNeighbor, error) {
	neighbors := make(map[string]models.BGPNeighbor)
	lines := strings.Split(output, "\n")

	dataStarted := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Skip until we find the header
		if strings.HasPrefix(line, "Neighbor") {
			dataStarted = true
			continue
		}

		if !dataStarted {
			continue
		}

		// Parse neighbor line
		fields := strings.Fields(line)
		if len(fields) < 10 {
			continue
		}

		neighborIP := fields[0]
		remoteAS, _ := strconv.ParseUint(fields[2], 10, 32)
		uptime := fields[8]
		state := fields[9]

		neighbor := models.BGPNeighbor{
			NeighborID: neighborIP,
			IPAddress:  neighborIP,
			RemoteAS:   uint32(remoteAS),
			Metadata:   make(map[string]interface{}),
		}

		// Determine state
		if _, err := strconv.Atoi(state); err == nil {
			// It's a number, meaning Established with prefix count
			neighbor.State = "Established"
			neighbor.Metadata["prefix_count"] = state
		} else {
			// It's a state string (Idle, Active, Connect, etc.)
			neighbor.State = state
		}

		neighbor.Metadata["uptime"] = uptime
		neighbor.Metadata["messages_received"] = fields[3]
		neighbor.Metadata["messages_sent"] = fields[4]

		neighbors[neighborIP] = neighbor
	}

	return neighbors, nil
}

// Helper function: Convert ASA netmask format to CIDR
// Example: "10.1.1.0 255.255.255.0" -> "10.1.1.0/24"
func convertASANetmaskToCIDR(network, netmask string) string {
	// Count bits in netmask
	parts := strings.Split(netmask, ".")
	if len(parts) != 4 {
		return network // Return network as-is if invalid netmask
	}

	var bits int
	for _, part := range parts {
		octet, _ := strconv.Atoi(part)
		for i := 7; i >= 0; i-- {
			if (octet & (1 << i)) != 0 {
				bits++
			} else {
				break
			}
		}
	}

	return fmt.Sprintf("%s/%d", network, bits)
}

// Helper function: Parse ASA route code to protocol name
func parseASARouteCode(code string) string {
	code = strings.TrimSpace(code)

	// Handle special cases with asterisk (default route)
	code = strings.TrimSuffix(code, "*")

	switch code {
	case "C":
		return "connected"
	case "L":
		return "local"
	case "S":
		return "static"
	case "R":
		return "rip"
	case "M":
		return "mobile"
	case "B":
		return "bgp"
	case "D":
		return "eigrp"
	case "EX":
		return "eigrp_external"
	case "O":
		return "ospf"
	case "IA":
		return "ospf_inter_area"
	case "N1":
		return "ospf_nssa_type1"
	case "N2":
		return "ospf_nssa_type2"
	case "E1":
		return "ospf_external_type1"
	case "E2":
		return "ospf_external_type2"
	case "i":
		return "isis"
	case "su":
		return "isis_summary"
	case "L1":
		return "isis_level1"
	case "L2":
		return "isis_level2"
	default:
		return strings.ToLower(code)
	}
}

package ios

import (
	"bufio"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mattpainter701/graphlayer/platform/models"
)

// ========================================================================
// Phase 2: Cisco IOS L3 Routing Parsers
// ========================================================================
//
// This file contains parsers for Cisco IOS routing protocols:
// - Static routes (configuration + show ip route)
// - OSPF (configuration + show ip ospf + show ip ospf neighbor)
// - BGP (configuration + show ip bgp + show ip bgp summary)
//
// ========================================================================

// ========================================================================
// Static Route Parsing
// ========================================================================

// ParseStaticRoutes extracts static routes from configuration
func (p *CiscoIOSParser) ParseStaticRoutes(config string) []models.L3Route {
	routes := []models.L3Route{}

	// Regex for: ip route <prefix> <mask> <next-hop|interface> [distance] [name <name>]
	staticRouteRegex := regexp.MustCompile(`^\s*ip\s+route\s+(\S+)\s+(\S+)\s+(\S+)(?:\s+(\d+))?(?:\s+name\s+(.+))?`)

	scanner := bufio.NewScanner(strings.NewReader(config))
	for scanner.Scan() {
		line := scanner.Text()

		if matches := staticRouteRegex.FindStringSubmatch(line); matches != nil {
			prefix := matches[1]
			mask := matches[2]
			nextHopOrInterface := matches[3]
			adminDist := matches[4]
			routeName := matches[5]

			// Convert mask to CIDR
			cidr := convertMaskToCIDR(prefix, mask)
			if cidr == "" {
				continue
			}

			// Determine if next hop is IP or interface
			nextHop := ""
			iface := ""
			if net.ParseIP(nextHopOrInterface) != nil {
				nextHop = nextHopOrInterface
			} else {
				iface = nextHopOrInterface
			}

			// Parse admin distance (default: 1)
			ad := 1
			if adminDist != "" {
				if parsed, err := strconv.Atoi(adminDist); err == nil {
					ad = parsed
				}
			}

			route := models.L3Route{
				Prefix:            cidr,
				PrefixLength:      getPrefixLength(cidr),
				NextHop:           nextHop,
				Interface:         iface,
				Protocol:          "static",
				Metric:            0,
				AdminDistance:     ad,
				IsActive:          true, // Assume active until verified
				IsBest:            true, // Static routes are typically best
				RouteName:         strings.TrimSpace(routeName),
				SourceBlock:       line,
				SourceSnippetHash: computeSnippetHash([]string{line}),
				ConfidenceScore:   85, // High confidence for static routes
				EvidenceSources:   []string{"config"},
				LastSeen:          time.Now(),
			}

			routes = append(routes, route)
		}
	}

	return routes
}

// ParseShowIPRoute parses 'show ip route' output
func (p *CiscoIOSParser) ParseShowIPRoute(output string) []models.L3Route {
	routes := []models.L3Route{}

	// Regex patterns for route entries
	// Example: S    192.168.1.0/24 [1/0] via 10.0.0.1
	// Example: O    10.1.0.0/16 [110/20] via 10.0.0.2, 00:05:23, GigabitEthernet0/0
	// Example: C    10.0.0.0/24 is directly connected, GigabitEthernet0/1
	routeRegex := regexp.MustCompile(`^([SCDORBEIM\*]\*?)\s+(\S+)\s+(?:\[(\d+)/(\d+)\]\s+)?(?:via\s+(\S+))?(?:,\s+(\S+))?(?:,\s+(.+))?`)

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()

		if matches := routeRegex.FindStringSubmatch(line); matches != nil {
			protocolCode := strings.TrimSpace(matches[1])
			prefix := matches[2]
			adStr := matches[3]
			metricStr := matches[4]
			nextHop := matches[5]
			timeOrInterface := matches[6]
			iface := matches[7]

			// Parse protocol
			protocol := parseProtocolCode(protocolCode)

			// Parse admin distance and metric
			ad := 0
			metric := 0
			if adStr != "" {
				ad, _ = strconv.Atoi(adStr)
			}
			if metricStr != "" {
				metric, _ = strconv.Atoi(metricStr)
			}

			// Determine interface
			if iface == "" && timeOrInterface != "" {
				// Check if timeOrInterface is actually an interface
				if !strings.Contains(timeOrInterface, ":") {
					iface = timeOrInterface
				}
			}

			route := models.L3Route{
				Prefix:          prefix,
				PrefixLength:    getPrefixLength(prefix),
				NextHop:         nextHop,
				Interface:       iface,
				Protocol:        protocol,
				Metric:          metric,
				AdminDistance:   ad,
				IsActive:        true,
				IsBest:          strings.Contains(protocolCode, "*"),
				SourceBlock:     line,
				ConfidenceScore: calculateRouteConfidenceFromProtocol(protocol),
				EvidenceSources: []string{"show_ip_route"},
				LastSeen:        time.Now(),
			}

			routes = append(routes, route)
		}
	}

	return routes
}

// parseProtocolCode converts Cisco route code to protocol name
func parseProtocolCode(code string) string {
	code = strings.TrimSpace(code)
	code = strings.TrimSuffix(code, "*")

	protocolMap := map[string]string{
		"C": "connected",
		"S": "static",
		"D": "eigrp",
		"O": "ospf",
		"R": "rip",
		"B": "bgp",
		"E": "egp",
		"I": "isis",
		"M": "mobile",
	}

	if protocol, found := protocolMap[code]; found {
		return protocol
	}
	return "unknown"
}

// ========================================================================
// OSPF Parsing
// ========================================================================

// ParseOSPFConfig extracts OSPF configuration
func (p *CiscoIOSParser) ParseOSPFConfig(config string) *models.OSPFProcess {
	process := &models.OSPFProcess{
		Areas:       []models.OSPFArea{},
		Neighbors:   []models.OSPFNeighbor{},
		Networks:    []models.OSPFNetwork{},
		Interfaces:  []models.OSPFInterface{},
		ConfigHash:  computeSnippetHash([]string{config}),
		InsertedTime: time.Now(),
		LastModified: time.Now(),
		Active:      true,
	}

	// Extract OSPF process block
	ospfBlockRegex := regexp.MustCompile(`(?s)router\s+ospf\s+(\d+)(.*?)(?:^!|\z)`)
	matches := ospfBlockRegex.FindStringSubmatch(config)
	if matches == nil {
		return nil
	}

	processIDStr := matches[1]
	ospfBlock := matches[2]

	if pid, err := strconv.Atoi(processIDStr); err == nil {
		process.ProcessID = pid
	}

	// Extract router ID
	routerIDRegex := regexp.MustCompile(`router-id\s+(\S+)`)
	if rid := routerIDRegex.FindStringSubmatch(ospfBlock); rid != nil {
		process.RouterID = rid[1]
	}

	// Extract networks
	networkRegex := regexp.MustCompile(`network\s+(\S+)\s+(\S+)\s+area\s+(\S+)`)
	networkMatches := networkRegex.FindAllStringSubmatch(ospfBlock, -1)
	for _, nm := range networkMatches {
		network := models.OSPFNetwork{
			Network:  nm[1],
			Wildcard: nm[2],
			AreaID:   nm[3],
		}
		process.Networks = append(process.Networks, network)
	}

	// Extract passive interfaces
	passiveRegex := regexp.MustCompile(`passive-interface\s+(\S+)`)
	passiveMatches := passiveRegex.FindAllStringSubmatch(ospfBlock, -1)
	for _, pm := range passiveMatches {
		process.PassiveInterfaces = append(process.PassiveInterfaces, pm[1])
	}

	// Extract default-information originate
	if strings.Contains(ospfBlock, "default-information originate") {
		process.DefaultOriginate = true
	}

	// Extract redistribute statements
	redistRegex := regexp.MustCompile(`redistribute\s+(\S+)`)
	redistMatches := redistRegex.FindAllStringSubmatch(ospfBlock, -1)
	for _, rm := range redistMatches {
		process.RedistributedProtocols = append(process.RedistributedProtocols, rm[1])
	}

	return process
}

// ParseShowIPOSPFNeighbor parses 'show ip ospf neighbor' output
func (p *CiscoIOSParser) ParseShowIPOSPFNeighbor(output string) []models.OSPFNeighbor {
	neighbors := []models.OSPFNeighbor{}

	// Example: 10.0.0.2        1   FULL/DR      00:00:38    10.0.0.2    GigabitEthernet0/0
	neighborRegex := regexp.MustCompile(`(\S+)\s+(\d+)\s+(\S+)\s+(\S+)\s+(\S+)\s+(\S+)`)

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()

		if matches := neighborRegex.FindStringSubmatch(line); matches != nil {
			neighborID := matches[1]
			priorityStr := matches[2]
			stateRole := matches[3]
			deadTimeStr := matches[4]
			ipAddress := matches[5]
			iface := matches[6]

			// Parse priority
			priority, _ := strconv.Atoi(priorityStr)

			// Parse state and role
			parts := strings.Split(stateRole, "/")
			state := parts[0]
			role := ""
			if len(parts) > 1 {
				role = parts[1]
			}

			// Parse dead time (format: HH:MM:SS)
			deadTime := parseDeadTime(deadTimeStr)

			neighbor := models.OSPFNeighbor{
				NeighborID:   neighborID,
				IPAddress:    ipAddress,
				Interface:    iface,
				State:        state,
				Priority:     priority,
				DeadTime:     deadTime,
				Role:         role,
				LastChange:   time.Now(),
			}

			neighbors = append(neighbors, neighbor)
		}
	}

	return neighbors
}

// ParseShowIPOSPF parses 'show ip ospf' output
func (p *CiscoIOSParser) ParseShowIPOSPF(output string) map[string]models.OSPFArea {
	areas := make(map[string]models.OSPFArea)

	// Parse area information
	areaRegex := regexp.MustCompile(`Area\s+(\S+)`)
	areaMatches := areaRegex.FindAllStringSubmatch(output, -1)

	for _, am := range areaMatches {
		areaID := am[1]
		area := models.OSPFArea{
			AreaID:   areaID,
			AreaType: "normal",
		}

		// Check for stub/nssa
		if strings.Contains(output, fmt.Sprintf("Area %s is a stub", areaID)) {
			area.AreaType = "stub"
		} else if strings.Contains(output, fmt.Sprintf("Area %s is a NSSA", areaID)) {
			area.AreaType = "nssa"
		}

		areas[areaID] = area
	}

	return areas
}

// ========================================================================
// BGP Parsing
// ========================================================================

// ParseBGPConfig extracts BGP configuration
func (p *CiscoIOSParser) ParseBGPConfig(config string) *models.BGPProcess {
	process := &models.BGPProcess{
		Neighbors:      []models.BGPNeighbor{},
		Networks:       []models.BGPNetwork{},
		AggregateAddrs: []models.BGPAggregate{},
		ConfigHash:     computeSnippetHash([]string{config}),
		InsertedTime:   time.Now(),
		LastModified:   time.Now(),
		Active:         true,
	}

	// Extract BGP process block
	bgpBlockRegex := regexp.MustCompile(`(?s)router\s+bgp\s+(\d+)(.*?)(?:^!|\z)`)
	matches := bgpBlockRegex.FindStringSubmatch(config)
	if matches == nil {
		return nil
	}

	asNumStr := matches[1]
	bgpBlock := matches[2]

	if asNum, err := strconv.Atoi(asNumStr); err == nil {
		process.LocalAS = asNum
	}

	// Extract router ID (BGP router-id)
	routerIDRegex := regexp.MustCompile(`bgp\s+router-id\s+(\S+)`)
	if rid := routerIDRegex.FindStringSubmatch(bgpBlock); rid != nil {
		process.RouterID = rid[1]
	}

	// Extract neighbors
	neighborRegex := regexp.MustCompile(`neighbor\s+(\S+)\s+remote-as\s+(\d+)`)
	neighborMatches := neighborRegex.FindAllStringSubmatch(bgpBlock, -1)
	for _, nm := range neighborMatches {
		remoteAS, _ := strconv.Atoi(nm[2])
		neighbor := models.BGPNeighbor{
			IPAddress: nm[1],
			RemoteAS:  remoteAS,
			EBGP:      remoteAS != process.LocalAS,
		}

		// Extract neighbor description
		descRegex := regexp.MustCompile(fmt.Sprintf(`neighbor\s+%s\s+description\s+(.+)`, nm[1]))
		if desc := descRegex.FindStringSubmatch(bgpBlock); desc != nil {
			neighbor.Description = strings.TrimSpace(desc[1])
		}

		// Extract update-source
		updateSrcRegex := regexp.MustCompile(fmt.Sprintf(`neighbor\s+%s\s+update-source\s+(\S+)`, nm[1]))
		if us := updateSrcRegex.FindStringSubmatch(bgpBlock); us != nil {
			neighbor.UpdateSource = us[1]
		}

		// Extract next-hop-self
		if strings.Contains(bgpBlock, fmt.Sprintf("neighbor %s next-hop-self", nm[1])) {
			neighbor.NextHopSelf = true
		}

		// Extract route-reflector-client
		if strings.Contains(bgpBlock, fmt.Sprintf("neighbor %s route-reflector-client", nm[1])) {
			neighbor.RouteReflectorClient = true
		}

		process.Neighbors = append(process.Neighbors, neighbor)
	}

	// Extract networks
	networkRegex := regexp.MustCompile(`network\s+(\S+)(?:\s+mask\s+(\S+))?`)
	networkMatches := networkRegex.FindAllStringSubmatch(bgpBlock, -1)
	for _, nm := range networkMatches {
		network := models.BGPNetwork{
			Network: nm[1],
			Mask:    nm[2],
		}
		process.Networks = append(process.Networks, network)
	}

	// Extract aggregate-address
	aggRegex := regexp.MustCompile(`aggregate-address\s+(\S+)\s+(\S+)(?:\s+(summary-only))?(?:\s+(as-set))?`)
	aggMatches := aggRegex.FindAllStringSubmatch(bgpBlock, -1)
	for _, am := range aggMatches {
		agg := models.BGPAggregate{
			Network:     am[1],
			Mask:        am[2],
			SummaryOnly: am[3] == "summary-only",
			AsSet:       am[4] == "as-set",
		}
		process.AggregateAddrs = append(process.AggregateAddrs, agg)
	}

	// Extract default-information originate
	if strings.Contains(bgpBlock, "default-information originate") {
		process.DefaultOriginate = true
	}

	// Extract redistribute
	redistRegex := regexp.MustCompile(`redistribute\s+(\S+)`)
	redistMatches := redistRegex.FindAllStringSubmatch(bgpBlock, -1)
	for _, rm := range redistMatches {
		process.RedistributedProtocols = append(process.RedistributedProtocols, rm[1])
	}

	return process
}

// ParseShowIPBGPSummary parses 'show ip bgp summary' output
func (p *CiscoIOSParser) ParseShowIPBGPSummary(output string) []models.BGPNeighbor {
	neighbors := []models.BGPNeighbor{}

	// Example: 10.0.0.2         4 65002   12345   12345   12345    0    0 00:05:23        5
	neighborRegex := regexp.MustCompile(`(\S+)\s+\d+\s+(\d+)\s+\d+\s+\d+\s+\d+\s+\d+\s+\d+\s+(\S+)\s+(\d+)`)

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()

		if matches := neighborRegex.FindStringSubmatch(line); matches != nil {
			ipAddress := matches[1]
			remoteASStr := matches[2]
			uptime := matches[3]
			prefixesStr := matches[4]

			remoteAS, _ := strconv.Atoi(remoteASStr)
			prefixes, _ := strconv.Atoi(prefixesStr)
			uptimeSeconds := parseUptimeToSeconds(uptime)

			// Determine state from uptime
			state := "Established"
			if strings.Contains(uptime, "Active") || strings.Contains(uptime, "Idle") {
				state = uptime
			}

			neighbor := models.BGPNeighbor{
				IPAddress:        ipAddress,
				RemoteAS:         remoteAS,
				State:            state,
				UptimeSeconds:    uptimeSeconds,
				PrefixesReceived: prefixes,
			}

			neighbors = append(neighbors, neighbor)
		}
	}

	return neighbors
}

// ParseShowIPBGP parses 'show ip bgp' output
func (p *CiscoIOSParser) ParseShowIPBGP(output string) []models.BGPRIBEntry {
	entries := []models.BGPRIBEntry{}

	// Example: *> 192.168.1.0/24   10.0.0.2                 0    100      0 65002 i
	bgpRouteRegex := regexp.MustCompile(`^([*>sd\s]+)\s+(\S+)\s+(\S+)\s+(\d+)\s+(\d+)\s+(\d+)\s+([\d\s]+)\s+([ie\?])`)

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()

		if matches := bgpRouteRegex.FindStringSubmatch(line); matches != nil {
			status := strings.TrimSpace(matches[1])
			prefix := matches[2]
			nextHop := matches[3]
			metricStr := matches[4]
			localPrefStr := matches[5]
			weightStr := matches[6]
			asPathStr := strings.TrimSpace(matches[7])
			origin := matches[8]

			metric, _ := strconv.Atoi(metricStr)
			localPref, _ := strconv.Atoi(localPrefStr)
			weight, _ := strconv.Atoi(weightStr)

			// Parse AS path
			asPath := []int{}
			for _, asStr := range strings.Fields(asPathStr) {
				if as, err := strconv.Atoi(asStr); err == nil {
					asPath = append(asPath, as)
				}
			}

			// Parse origin code
			originMap := map[string]string{
				"i": "IGP",
				"e": "EGP",
				"?": "incomplete",
			}
			originStr := originMap[origin]

			entry := models.BGPRIBEntry{
				Prefix:       prefix,
				PrefixLength: getPrefixLength(prefix),
				NextHop:      nextHop,
				Path:         asPath,
				Origin:       originStr,
				LocalPref:    localPref,
				MED:          metric,
				Weight:       weight,
				IsBest:       strings.Contains(status, ">"),
				IsValid:      strings.Contains(status, "*"),
				Status:       "valid",
				LastModified: time.Now(),
			}

			if entry.IsBest {
				entry.Status = "best"
			}

			entries = append(entries, entry)
		}
	}

	return entries
}

// ========================================================================
// Helper Functions
// ========================================================================

// convertMaskToCIDR converts IP and subnet mask to CIDR notation
func convertMaskToCIDR(ip, mask string) string {
	// Parse mask to get prefix length
	maskIP := net.ParseIP(mask)
	if maskIP == nil {
		return ""
	}

	maskIPv4 := maskIP.To4()
	if maskIPv4 == nil {
		return ""
	}

	prefixLen, _ := net.IPMask(maskIPv4).Size()
	return fmt.Sprintf("%s/%d", ip, prefixLen)
}

// getPrefixLength extracts prefix length from CIDR
func getPrefixLength(cidr string) int {
	parts := strings.Split(cidr, "/")
	if len(parts) != 2 {
		return 0
	}
	prefixLen, _ := strconv.Atoi(parts[1])
	return prefixLen
}

// calculateRouteConfidenceFromProtocol returns confidence score for protocol
func calculateRouteConfidenceFromProtocol(protocol string) int {
	scores := map[string]int{
		"connected": 95,
		"static":    85,
		"ospf":      75,
		"eigrp":     75,
		"bgp":       65,
		"rip":       50,
	}

	if score, found := scores[protocol]; found {
		return score
	}
	return 40
}

// parseDeadTime converts HH:MM:SS to seconds
func parseDeadTime(timeStr string) int {
	parts := strings.Split(timeStr, ":")
	if len(parts) != 3 {
		return 0
	}

	hours, _ := strconv.Atoi(parts[0])
	minutes, _ := strconv.Atoi(parts[1])
	seconds, _ := strconv.Atoi(parts[2])

	return hours*3600 + minutes*60 + seconds
}

// parseUptimeToSeconds converts uptime string to seconds
func parseUptimeToSeconds(uptime string) int {
	// Format: HH:MM:SS or <time_value>
	if strings.Contains(uptime, ":") {
		return parseDeadTime(uptime)
	}

	// Handle other formats (e.g., "5d12h", "3w4d")
	// Simplified - just return 0 for now
	return 0
}

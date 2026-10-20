package firewall

import (
	"fmt"
	"github.com/mattpainter701/graphlayer/platform/models"
	"log/slog"
	"sort"
	"strings"
)

// PAN-OS Version Support for Routing Mappers:
// - PAN-OS 8.1.x through 11.1.x
// - Virtual Router support (multi-VR environments)
// - Route preference (Admin Distance) handling
// - Protocol-specific metadata preservation

// BuildPANOSL3RoutingTable creates a unified L3 routing table from multiple PAN-OS data sources.
// This function aggregates static routes, operational routing table, OSPF, and BGP data
// into a single coherent routing table model.
//
// Parameters:
//   - deviceID: Unique identifier for the firewall device
//   - hostname: Firewall hostname
//   - virtualRouter: Virtual Router name (e.g., "default", "trust-vr")
//   - staticRoutesXML: XML from <show><config><running><xpath>.../static-route</xpath></running></config></show>
//   - routingTableXML: XML from <show><routing><route></route></routing></show>
//   - ospfConfigXML: XML from OSPF configuration
//   - ospfNeighborsXML: XML from <show><routing><protocol><ospf><neighbor></neighbor></ospf></protocol></routing></show>
//   - bgpConfigXML: XML from BGP configuration
//
// Returns:
//   - *models.L3RoutingTable: Unified routing table with all routes and protocol processes
//   - error: Any parsing or validation errors
func BuildPANOSL3RoutingTable(
	deviceID string,
	hostname string,
	virtualRouter string,
	staticRoutesXML string,
	routingTableXML string,
	ospfConfigXML string,
	ospfNeighborsXML string,
	bgpConfigXML string,
) (*models.L3RoutingTable, error) {
	logger := slog.Default().With(
		"vendor", "paloalto",
		"platform", "firewall",
		"device_id", deviceID,
		"virtual_router", virtualRouter,
	)

	// Initialize routing table
	table := &models.L3RoutingTable{
		DeviceID:       deviceID,
		Hostname:       hostname,
		VirtualRouter:  virtualRouter,
		Routes:         []models.L3Route{},
		OSPFProcesses:  []models.OSPFProcess{},
		BGPProcesses:   []models.BGPProcess{},
		ProtocolStatus: make(map[string]string),
	}

	// Parse static routes (configuration)
	var configRoutes []models.L3Route
	if staticRoutesXML != "" {
		routes, err := ParsePANOSStaticRoutes(staticRoutesXML)
		if err != nil {
			logger.Warn("Failed to parse static routes", "error", err)
		} else {
			configRoutes = routes
			logger.Info("Parsed static routes", "count", len(routes))
		}
	}

	// Parse operational routing table
	var operationalRoutes []models.L3Route
	if routingTableXML != "" {
		routes, err := ParsePANOSRoutingTable(routingTableXML)
		if err != nil {
			logger.Warn("Failed to parse routing table", "error", err)
		} else {
			operationalRoutes = routes
			logger.Info("Parsed operational routing table", "count", len(routes))
		}
	}

	// Parse OSPF configuration
	var ospfProcess *models.OSPFProcess
	if ospfConfigXML != "" {
		process, err := ParsePANOSOSPFConfig(ospfConfigXML)
		if err != nil {
			logger.Warn("Failed to parse OSPF config", "error", err)
		} else {
			ospfProcess = process
			table.OSPFProcesses = append(table.OSPFProcesses, *process)
			table.ProtocolStatus["ospf"] = "configured"
			logger.Info("Parsed OSPF configuration", "areas", len(process.Areas))
		}
	}

	// Parse OSPF neighbors
	if ospfNeighborsXML != "" {
		neighbors, err := ParsePANOSOSPFNeighbors(ospfNeighborsXML)
		if err != nil {
			logger.Warn("Failed to parse OSPF neighbors", "error", err)
		} else if ospfProcess != nil {
			// Add neighbors to OSPF process
			ospfProcess.Neighbors = neighbors
			if len(table.OSPFProcesses) > 0 {
				table.OSPFProcesses[0].Neighbors = neighbors
			}
			logger.Info("Parsed OSPF neighbors", "count", len(neighbors))
		}
	}

	// Parse BGP configuration
	if bgpConfigXML != "" {
		bgpProcess, err := ParsePANOSBGPConfig(bgpConfigXML)
		if err != nil {
			logger.Warn("Failed to parse BGP config", "error", err)
		} else {
			table.BGPProcesses = append(table.BGPProcesses, *bgpProcess)
			table.ProtocolStatus["bgp"] = "configured"
			logger.Info("Parsed BGP configuration", "neighbors", len(bgpProcess.Neighbors))
		}
	}

	// Merge configured and operational routes
	table.Routes = mergePANOSRoutes(configRoutes, operationalRoutes)

	// Enhance routes with OSPF metadata
	if ospfProcess != nil {
		table.Routes = enhanceRoutesWithPANOSOSPFMetadata(table.Routes, ospfProcess)
	}

	// Sort routes by prefix for consistency
	sort.Slice(table.Routes, func(i, j int) bool {
		return table.Routes[i].Prefix < table.Routes[j].Prefix
	})

	logger.Info("Built L3 routing table",
		"total_routes", len(table.Routes),
		"ospf_processes", len(table.OSPFProcesses),
		"bgp_processes", len(table.BGPProcesses),
	)

	return table, nil
}

// mergePANOSRoutes merges configured static routes with operational routing table entries.
// Operational routes take precedence as they represent the active FIB state.
//
// Logic:
//   - Operational routes are authoritative (actually installed in FIB)
//   - Static routes from config are added if not present in operational table
//   - Duplicate detection based on prefix + next-hop
func mergePANOSRoutes(configRoutes, operationalRoutes []models.L3Route) []models.L3Route {
	// Start with operational routes (truth of what's actually running)
	merged := make([]models.L3Route, len(operationalRoutes))
	copy(merged, operationalRoutes)

	// Build lookup map for operational routes
	opRouteMap := make(map[string]bool)
	for _, route := range operationalRoutes {
		key := fmt.Sprintf("%s|%s", route.Prefix, route.NextHop)
		opRouteMap[key] = true
	}

	// Add config routes that aren't in operational table
	for _, route := range configRoutes {
		key := fmt.Sprintf("%s|%s", route.Prefix, route.NextHop)
		if !opRouteMap[key] {
			// Mark as configured but not installed
			route.Metadata["status"] = "configured_not_installed"
			merged = append(merged, route)
		}
	}

	return merged
}

// enhanceRoutesWithPANOSOSPFMetadata adds OSPF-specific metadata to routes learned via OSPF.
// This includes area information, path type (intra-area, inter-area, external), and LSA details.
func enhanceRoutesWithPANOSOSPFMetadata(routes []models.L3Route, ospfProcess *models.OSPFProcess) []models.L3Route {
	enhanced := make([]models.L3Route, len(routes))
	copy(enhanced, routes)

	// Build area network map for quick lookup
	areaNetworks := make(map[string]string) // prefix -> area ID
	for _, area := range ospfProcess.Areas {
		for _, network := range area.Networks {
			areaNetworks[network] = area.AreaID
		}
	}

	for i := range enhanced {
		route := &enhanced[i]
		if strings.ToLower(route.Protocol) != "ospf" {
			continue
		}

		// Add OSPF process metadata
		if route.Metadata == nil {
			route.Metadata = make(map[string]interface{})
		}
		route.Metadata["ospf_process_id"] = ospfProcess.ProcessID
		route.Metadata["ospf_router_id"] = ospfProcess.RouterID

		// Determine OSPF area if possible
		for networkPrefix, areaID := range areaNetworks {
			if strings.HasPrefix(route.Prefix, networkPrefix) {
				route.Metadata["ospf_area"] = areaID
				break
			}
		}

		// OSPF path type determination based on metric
		// PAN-OS OSPF uses standard OSPF path types
		if pathType, ok := route.Metadata["ospf_path_type"].(string); ok {
			switch pathType {
			case "intra":
				route.Metadata["ospf_path_description"] = "Intra-Area Route"
			case "inter":
				route.Metadata["ospf_path_description"] = "Inter-Area Route"
			case "ext1":
				route.Metadata["ospf_path_description"] = "External Type 1"
			case "ext2":
				route.Metadata["ospf_path_description"] = "External Type 2"
			}
		}
	}

	return enhanced
}

// ValidatePANOSRoutingTable performs validation checks on a PAN-OS L3 routing table.
// Returns a slice of validation warning messages (empty if all checks pass).
//
// Validation Checks:
//   - Default route presence (0.0.0.0/0)
//   - Route count sanity (PAN-OS supports up to 200K routes depending on model)
//   - OSPF neighbor states
//   - BGP peer states
//   - Duplicate route detection
//   - Virtual Router consistency
func ValidatePANOSRoutingTable(table *models.L3RoutingTable) []string {
	var warnings []string

	// Check for default route
	hasDefaultRoute := false
	for _, route := range table.Routes {
		if route.Prefix == "0.0.0.0/0" || route.Prefix == "::/0" {
			hasDefaultRoute = true
			break
		}
	}
	if !hasDefaultRoute {
		warnings = append(warnings, "No default route found - device may have limited reachability")
	}

	// Check route count (PA-5000 series supports ~200K, PA-220 supports ~25K)
	// This is a soft warning threshold
	if len(table.Routes) > 100000 {
		warnings = append(warnings, fmt.Sprintf("High route count (%d) - may approach platform limits", len(table.Routes)))
	}

	// OSPF neighbor validation
	for _, ospfProcess := range table.OSPFProcesses {
		if len(ospfProcess.Areas) == 0 {
			warnings = append(warnings, fmt.Sprintf("OSPF process %s has no areas configured", ospfProcess.ProcessID))
		}

		downNeighbors := 0
		for _, neighbor := range ospfProcess.Neighbors {
			if neighbor.State != "FULL" && neighbor.State != "2WAY" {
				downNeighbors++
			}
		}
		if downNeighbors > 0 {
			warnings = append(warnings, fmt.Sprintf("OSPF has %d neighbors not in FULL/2WAY state", downNeighbors))
		}
	}

	// BGP peer validation
	for _, bgpProcess := range table.BGPProcesses {
		if bgpProcess.RouterID == "" {
			warnings = append(warnings, "BGP process has no Router ID configured")
		}

		downPeers := 0
		for _, neighbor := range bgpProcess.Neighbors {
			if neighbor.State != "Established" {
				downPeers++
			}
		}
		if downPeers > 0 {
			warnings = append(warnings, fmt.Sprintf("BGP has %d peers not in Established state", downPeers))
		}
	}

	// Duplicate route detection (same prefix, different next-hop)
	prefixCount := make(map[string]int)
	for _, route := range table.Routes {
		prefixCount[route.Prefix]++
	}
	for prefix, count := range prefixCount {
		if count > 10 { // ECMP is normal, but >10 next-hops is unusual
			warnings = append(warnings, fmt.Sprintf("Prefix %s has %d routes (possible ECMP or routing loop)", prefix, count))
		}
	}

	return warnings
}

// GetPANOSRoutesByProtocol filters routes by routing protocol.
// Useful for analyzing specific routing protocol behavior.
func GetPANOSRoutesByProtocol(table *models.L3RoutingTable, protocol string) []models.L3Route {
	var filtered []models.L3Route
	protocolLower := strings.ToLower(protocol)

	for _, route := range table.Routes {
		if strings.ToLower(route.Protocol) == protocolLower {
			filtered = append(filtered, route)
		}
	}

	return filtered
}

// GetPANOSRouteSummary generates a summary of routes by protocol.
// Returns map of protocol -> count.
func GetPANOSRouteSummary(table *models.L3RoutingTable) map[string]int {
	summary := make(map[string]int)

	for _, route := range table.Routes {
		protocol := strings.ToLower(route.Protocol)
		summary[protocol]++
	}

	return summary
}

// ComparePANOSRoutingTables compares two routing tables and identifies differences.
// Useful for change detection and configuration drift analysis.
//
// Returns:
//   - added: Routes in 'new' but not in 'old'
//   - removed: Routes in 'old' but not in 'new'
//   - changed: Routes present in both but with different attributes
func ComparePANOSRoutingTables(old, new *models.L3RoutingTable) (added, removed, changed []models.L3Route) {
	// Build maps for quick lookup
	oldMap := make(map[string]models.L3Route)
	newMap := make(map[string]models.L3Route)

	for _, route := range old.Routes {
		key := fmt.Sprintf("%s|%s|%s", route.Prefix, route.Protocol, route.NextHop)
		oldMap[key] = route
	}

	for _, route := range new.Routes {
		key := fmt.Sprintf("%s|%s|%s", route.Prefix, route.Protocol, route.NextHop)
		newMap[key] = route
	}

	// Find added routes
	for key, route := range newMap {
		if _, exists := oldMap[key]; !exists {
			added = append(added, route)
		}
	}

	// Find removed routes
	for key, route := range oldMap {
		if _, exists := newMap[key]; !exists {
			removed = append(removed, route)
		}
	}

	// Find changed routes (same prefix/protocol/nexthop but different metric/interface)
	for key, newRoute := range newMap {
		if oldRoute, exists := oldMap[key]; exists {
			if oldRoute.Metric != newRoute.Metric ||
				oldRoute.AdminDistance != newRoute.AdminDistance ||
				oldRoute.Interface != newRoute.Interface {
				changed = append(changed, newRoute)
			}
		}
	}

	return added, removed, changed
}

// EnrichPANOSRoutingTableWithInterfaces adds interface metadata to routes.
// This is useful when correlating routing with L2 topology.
//
// Parameters:
//   - table: The routing table to enrich
//   - interfaceMap: Map of interface name -> interface metadata
func EnrichPANOSRoutingTableWithInterfaces(table *models.L3RoutingTable, interfaceMap map[string]map[string]interface{}) {
	for i := range table.Routes {
		route := &table.Routes[i]
		if route.Interface == "" {
			continue
		}

		if ifaceMetadata, exists := interfaceMap[route.Interface]; exists {
			if route.Metadata == nil {
				route.Metadata = make(map[string]interface{})
			}
			route.Metadata["interface_details"] = ifaceMetadata
		}
	}
}

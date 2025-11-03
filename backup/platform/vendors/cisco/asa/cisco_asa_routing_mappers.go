package asa

import (
	"fmt"
	"github.com/mattpainter701/graphlayer/platform/platform/models"
	"log/slog"
	"sort"
	"strings"
)

// Cisco ASA Version Support for Routing Mappers:
// - ASA 9.8.x through 9.20.x
// - Multi-context mode support
// - VRF-lite support (ASA 9.0+)

// BuildASAL3RoutingTable creates a unified L3 routing table from multiple ASA CLI outputs.
// This function aggregates routing table, OSPF, and BGP data into a single coherent model.
//
// Parameters:
//   - deviceID: Unique identifier for the ASA device
//   - hostname: ASA hostname
//   - context: Security context name (empty string for single-context mode)
//   - routingTableOutput: Output from "show route"
//   - ospfConfigOutput: Output from "show running-config router ospf"
//   - ospfNeighborsOutput: Output from "show ospf neighbor"
//   - bgpConfigOutput: Output from "show running-config router bgp"
//   - bgpSummaryOutput: Output from "show bgp summary"
//
// Returns:
//   - *models.L3RoutingTable: Unified routing table with all routes and protocol processes
//   - error: Any parsing or validation errors
func BuildASAL3RoutingTable(
	deviceID string,
	hostname string,
	context string,
	routingTableOutput string,
	ospfConfigOutput string,
	ospfNeighborsOutput string,
	bgpConfigOutput string,
	bgpSummaryOutput string,
) (*models.L3RoutingTable, error) {
	logger := slog.Default().With(
		"vendor", "cisco",
		"platform", "asa",
		"device_id", deviceID,
		"context", context,
	)

	// Initialize routing table
	table := &models.L3RoutingTable{
		DeviceID:       deviceID,
		Hostname:       hostname,
		VirtualRouter:  context, // ASA uses context instead of VRF
		Routes:         []models.L3Route{},
		OSPFProcesses:  []models.OSPFProcess{},
		BGPProcesses:   []models.BGPProcess{},
		ProtocolStatus: make(map[string]string),
	}

	// Parse routing table
	if routingTableOutput != "" {
		routes, err := ParseASARoutingTable(routingTableOutput)
		if err != nil {
			logger.Warn("Failed to parse routing table", "error", err)
		} else {
			table.Routes = routes
			logger.Info("Parsed routing table", "count", len(routes))
		}
	}

	// Parse OSPF configuration
	var ospfProcess *models.OSPFProcess
	if ospfConfigOutput != "" {
		process, err := ParseASAOSPFConfig(ospfConfigOutput)
		if err != nil {
			logger.Warn("Failed to parse OSPF config", "error", err)
		} else {
			ospfProcess = process
			table.OSPFProcesses = append(table.OSPFProcesses, *process)
			table.ProtocolStatus["ospf"] = "configured"
			logger.Info("Parsed OSPF configuration", "process_id", process.ProcessID)
		}
	}

	// Parse OSPF neighbors
	if ospfNeighborsOutput != "" {
		neighbors, err := ParseASAOSPFNeighbors(ospfNeighborsOutput)
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
	var bgpProcess *models.BGPProcess
	if bgpConfigOutput != "" {
		process, err := ParseASABGPConfig(bgpConfigOutput)
		if err != nil {
			logger.Warn("Failed to parse BGP config", "error", err)
		} else {
			bgpProcess = process
			table.BGPProcesses = append(table.BGPProcesses, *process)
			table.ProtocolStatus["bgp"] = "configured"
			logger.Info("Parsed BGP configuration", "local_as", process.LocalAS)
		}
	}

	// Parse BGP summary for operational state
	if bgpSummaryOutput != "" && bgpProcess != nil {
		summaryNeighbors, err := ParseASABGPSummary(bgpSummaryOutput)
		if err != nil {
			logger.Warn("Failed to parse BGP summary", "error", err)
		} else {
			// Merge BGP summary data with config data
			mergeASABGPNeighbors(bgpProcess, summaryNeighbors)
			if len(table.BGPProcesses) > 0 {
				table.BGPProcesses[0] = *bgpProcess
			}
			logger.Info("Merged BGP operational state", "neighbors", len(summaryNeighbors))
		}
	}

	// Enhance routes with protocol metadata
	if ospfProcess != nil {
		table.Routes = enhanceRoutesWithASAOSPFMetadata(table.Routes, ospfProcess)
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

// mergeASABGPNeighbors merges BGP operational state from "show bgp summary"
// into the BGP process configuration data
func mergeASABGPNeighbors(bgpProcess *models.BGPProcess, summaryNeighbors map[string]models.BGPNeighbor) {
	for i := range bgpProcess.Neighbors {
		neighbor := &bgpProcess.Neighbors[i]
		if summaryData, exists := summaryNeighbors[neighbor.IPAddress]; exists {
			// Update state from operational data
			neighbor.State = summaryData.State

			// Merge metadata
			for key, value := range summaryData.Metadata {
				neighbor.Metadata[key] = value
			}
		}
	}
}

// enhanceRoutesWithASAOSPFMetadata adds OSPF-specific metadata to routes learned via OSPF
func enhanceRoutesWithASAOSPFMetadata(routes []models.L3Route, ospfProcess *models.OSPFProcess) []models.L3Route {
	enhanced := make([]models.L3Route, len(routes))
	copy(enhanced, routes)

	// Build area network map
	areaNetworks := make(map[string]string) // prefix -> area ID
	for _, area := range ospfProcess.Areas {
		for _, network := range area.Networks {
			areaNetworks[network] = area.AreaID
		}
	}

	for i := range enhanced {
		route := &enhanced[i]
		protocol := strings.ToLower(route.Protocol)

		if !strings.HasPrefix(protocol, "ospf") {
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

		// OSPF path type based on protocol
		switch protocol {
		case "ospf":
			route.Metadata["ospf_path_type"] = "intra-area"
		case "ospf_inter_area":
			route.Metadata["ospf_path_type"] = "inter-area"
		case "ospf_external_type1":
			route.Metadata["ospf_path_type"] = "external-type1"
		case "ospf_external_type2":
			route.Metadata["ospf_path_type"] = "external-type2"
		case "ospf_nssa_type1":
			route.Metadata["ospf_path_type"] = "nssa-type1"
		case "ospf_nssa_type2":
			route.Metadata["ospf_path_type"] = "nssa-type2"
		}
	}

	return enhanced
}

// ValidateASARoutingTable performs validation checks on an ASA L3 routing table
func ValidateASARoutingTable(table *models.L3RoutingTable) []string {
	var warnings []string

	// Check for default route
	hasDefaultRoute := false
	for _, route := range table.Routes {
		if route.Prefix == "0.0.0.0/0" {
			hasDefaultRoute = true
			break
		}
	}
	if !hasDefaultRoute {
		warnings = append(warnings, "No default route found - ASA may have limited reachability")
	}

	// Check route count (ASA 5585-X supports up to 250K routes, smaller models much less)
	if len(table.Routes) > 50000 {
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

		// Check for backbone area
		hasBackbone := false
		for _, area := range ospfProcess.Areas {
			if area.AreaID == "0" || area.AreaID == "0.0.0.0" {
				hasBackbone = true
				break
			}
		}
		if !hasBackbone && len(ospfProcess.Areas) > 1 {
			warnings = append(warnings, "OSPF configured without backbone area 0 - multi-area OSPF requires area 0")
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

	// Duplicate route detection
	prefixCount := make(map[string]int)
	for _, route := range table.Routes {
		prefixCount[route.Prefix]++
	}
	for prefix, count := range prefixCount {
		if count > 8 { // ECMP on ASA supports up to 8 paths
			warnings = append(warnings, fmt.Sprintf("Prefix %s has %d routes (ASA ECMP limit is 8)", prefix, count))
		}
	}

	return warnings
}

// GetASARoutesByProtocol filters routes by routing protocol
func GetASARoutesByProtocol(table *models.L3RoutingTable, protocol string) []models.L3Route {
	var filtered []models.L3Route
	protocolLower := strings.ToLower(protocol)

	for _, route := range table.Routes {
		if strings.ToLower(route.Protocol) == protocolLower {
			filtered = append(filtered, route)
		}
	}

	return filtered
}

// GetASARouteSummary generates a summary of routes by protocol
func GetASARouteSummary(table *models.L3RoutingTable) map[string]int {
	summary := make(map[string]int)

	for _, route := range table.Routes {
		protocol := strings.ToLower(route.Protocol)
		summary[protocol]++
	}

	return summary
}

// CompareASARoutingTables compares two routing tables and identifies differences
func CompareASARoutingTables(old, new *models.L3RoutingTable) (added, removed, changed []models.L3Route) {
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

	// Find changed routes
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

// EnrichASARoutingTableWithInterfaces adds interface metadata to routes
func EnrichASARoutingTableWithInterfaces(table *models.L3RoutingTable, interfaceMap map[string]map[string]interface{}) {
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

// GetASADefaultRoute returns the default route if one exists
func GetASADefaultRoute(table *models.L3RoutingTable) *models.L3Route {
	for _, route := range table.Routes {
		if route.Prefix == "0.0.0.0/0" {
			return &route
		}
	}
	return nil
}

// GetASAOSPFRouteSummary returns OSPF-specific route statistics
func GetASAOSPFRouteSummary(table *models.L3RoutingTable) map[string]int {
	summary := map[string]int{
		"intra_area":     0,
		"inter_area":     0,
		"external_type1": 0,
		"external_type2": 0,
		"nssa_type1":     0,
		"nssa_type2":     0,
	}

	for _, route := range table.Routes {
		protocol := strings.ToLower(route.Protocol)
		switch protocol {
		case "ospf":
			summary["intra_area"]++
		case "ospf_inter_area":
			summary["inter_area"]++
		case "ospf_external_type1":
			summary["external_type1"]++
		case "ospf_external_type2":
			summary["external_type2"]++
		case "ospf_nssa_type1":
			summary["nssa_type1"]++
		case "ospf_nssa_type2":
			summary["nssa_type2"]++
		}
	}

	return summary
}

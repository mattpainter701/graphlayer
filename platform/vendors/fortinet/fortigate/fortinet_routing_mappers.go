package fortinet

import (
	"fmt"
	"time"

	"github.com/mattpainter701/graphlayer/platform/platform/models"
)

// ========================================================================
// Phase 3: FortiGate L3 Routing Mappers
// ========================================================================
//
// This file contains mapper functions that transform parsed FortiGate
// routing data into the unified L3RoutingTable model.
//
// Mappers:
// - BuildFortiGateL3RoutingTable: Combines config + operational data
// - EnhanceRoutesWithOSPFMetadata: Adds OSPF-specific attributes
// - EnhanceRoutesWithBGPMetadata: Adds BGP-specific attributes
// - MergeFortiGateRoutes: Merges config routes with operational routes
//
// ========================================================================

// BuildFortiGateL3RoutingTable builds a complete L3RoutingTable from FortiGate API data
// Parameters:
//   - deviceID: Unique device identifier
//   - hostname: Device hostname
//   - vdom: Virtual Domain name (or "root" for single VDOM)
//   - staticRoutesJSON: JSON from GET /api/v2/cmdb/router/static
//   - routingTableJSON: JSON from GET /api/v2/monitor/router/ipv4
//   - ospfConfigJSON: JSON from GET /api/v2/cmdb/router/ospf (optional)
//   - ospfNeighborsJSON: JSON from GET /api/v2/monitor/router/ospf/neighbors (optional)
//   - bgpConfigJSON: JSON from GET /api/v2/cmdb/router/bgp (optional)
func BuildFortiGateL3RoutingTable(
	deviceID string,
	hostname string,
	vdom string,
	staticRoutesJSON string,
	routingTableJSON string,
	ospfConfigJSON string,
	ospfNeighborsJSON string,
	bgpConfigJSON string,
) (*models.L3RoutingTable, error) {

	table := &models.L3RoutingTable{
		DeviceID:       deviceID,
		Hostname:       hostname,
		VirtualContext: vdom,
		VirtualRouter:  "default", // FortiGate uses single virtual router per VDOM
		Routes:         []models.L3Route{},
		ConfigHash:     computeSnippetHash([]string{staticRoutesJSON, routingTableJSON}),
		InsertedTime:   time.Now(),
		LastModified:   time.Now(),
		Active:         true,
	}

	// Step 1: Parse static routes from config
	staticRoutes, err := ParseFortiGateStaticRoutes(staticRoutesJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to parse static routes: %w", err)
	}

	// Step 2: Parse operational routing table
	operationalRoutes, err := ParseFortiGateRoutingTable(routingTableJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to parse routing table: %w", err)
	}

	// Step 3: Merge static and operational routes
	mergedRoutes := mergeFortiGateRoutes(staticRoutes, operationalRoutes)

	// Step 4: Parse OSPF data (if provided)
	var ospfProcess *models.OSPFProcess
	if ospfConfigJSON != "" {
		ospfProcess, err = ParseFortiGateOSPFConfig(ospfConfigJSON)
		if err != nil {
			// Log error but continue - OSPF may not be configured
			// TODO: Add logger
		}

		// Parse OSPF neighbors if available
		if ospfNeighborsJSON != "" && ospfProcess != nil {
			neighbors, err := ParseFortiGateOSPFNeighbors(ospfNeighborsJSON)
			if err == nil {
				ospfProcess.Neighbors = neighbors
			}
		}
	}

	// Step 5: Parse BGP data (if provided)
	var bgpProcess *models.BGPProcess
	if bgpConfigJSON != "" {
		bgpProcess, err = ParseFortiGateBGPConfig(bgpConfigJSON)
		if err != nil {
			// Log error but continue - BGP may not be configured
			// TODO: Add logger
		}
	}

	// Step 6: Enhance routes with protocol-specific metadata
	if ospfProcess != nil {
		mergedRoutes = enhanceRoutesWithFortiGateOSPFMetadata(mergedRoutes, ospfProcess)
	}

	if bgpProcess != nil {
		mergedRoutes = enhanceRoutesWithFortiGateBGPMetadata(mergedRoutes, bgpProcess)
	}

	// Step 7: Identify default route
	for i := range mergedRoutes {
		if mergedRoutes[i].Prefix == "0.0.0.0/0" {
			table.DefaultRoute = &mergedRoutes[i]
			break
		}
	}

	table.Routes = mergedRoutes

	return table, nil
}

// mergeFortiGateRoutes merges config-based and operational routes
func mergeFortiGateRoutes(configRoutes, operationalRoutes []models.L3Route) []models.L3Route {
	// Use map to deduplicate by prefix
	routeMap := make(map[string]models.L3Route)

	// Add operational routes first (they are authoritative for active state)
	for _, route := range operationalRoutes {
		routeMap[route.Prefix] = route
	}

	// Merge config routes (for additional metadata like descriptions)
	for _, configRoute := range configRoutes {
		if existing, found := routeMap[configRoute.Prefix]; found {
			// Merge: keep operational data but add config metadata
			existing.Description = configRoute.Description
			existing.EvidenceSources = append(existing.EvidenceSources, "config")
			routeMap[configRoute.Prefix] = existing
		} else {
			// Config-only route (not in operational table - may be inactive)
			configRoute.IsActive = false
			routeMap[configRoute.Prefix] = configRoute
		}
	}

	// Convert map back to slice
	merged := []models.L3Route{}
	for _, route := range routeMap {
		merged = append(merged, route)
	}

	return merged
}

// enhanceRoutesWithFortiGateOSPFMetadata adds OSPF-specific metadata to routes
func enhanceRoutesWithFortiGateOSPFMetadata(
	routes []models.L3Route,
	ospfProcess *models.OSPFProcess,
) []models.L3Route {
	enhanced := make([]models.L3Route, len(routes))
	copy(enhanced, routes)

	for i := range enhanced {
		if enhanced[i].Protocol == "ospf" {
			// Determine OSPF area from networks
			areaID := findFortiGateOSPFArea(enhanced[i].Prefix, ospfProcess.Networks)

			// Determine route type (simplified - would parse from LSDB in production)
			routeType := "intra-area" // Default
			if areaID == "0" || areaID == "0.0.0.0" {
				routeType = "intra-area"
			}

			enhanced[i].OSPFMetadata = &models.OSPFRouteMetadata{
				RouteType: routeType,
				AreaID:    areaID,
				Cost:      enhanced[i].Metric,
			}

			// Update evidence sources
			enhanced[i].EvidenceSources = append(enhanced[i].EvidenceSources, "fortigate_ospf_process")
		}
	}

	return enhanced
}

// enhanceRoutesWithFortiGateBGPMetadata adds BGP-specific metadata to routes
func enhanceRoutesWithFortiGateBGPMetadata(
	routes []models.L3Route,
	bgpProcess *models.BGPProcess,
) []models.L3Route {
	enhanced := make([]models.L3Route, len(routes))
	copy(enhanced, routes)

	for i := range enhanced {
		if enhanced[i].Protocol == "bgp" {
			// Default BGP metadata (would be enriched from BGP RIB in production)
			enhanced[i].BGPMetadata = &models.BGPRouteMetadata{
				ASPath:     []int{bgpProcess.LocalAS},
				LocalPref:  100, // Default
				MED:        enhanced[i].Metric,
				Origin:     "IGP",
				PathLength: 1,
			}

			// Update evidence sources
			enhanced[i].EvidenceSources = append(enhanced[i].EvidenceSources, "fortigate_bgp_process")
		}
	}

	return enhanced
}

// findFortiGateOSPFArea finds the OSPF area for a given prefix
func findFortiGateOSPFArea(prefix string, networks []models.OSPFNetwork) string {
	// Simplified: would perform actual subnet matching in production
	for _, network := range networks {
		// For now, just check if network prefix matches
		if network.Network == prefix {
			return network.AreaID
		}
	}
	return "0.0.0.0" // Default to backbone area
}

// CalculateFortiGateRoutingTableMetrics calculates aggregate metrics for routing table
func CalculateFortiGateRoutingTableMetrics(table *models.L3RoutingTable) map[string]int {
	metrics := map[string]int{
		"total_routes":     len(table.Routes),
		"active_routes":    0,
		"connected_routes": 0,
		"static_routes":    0,
		"ospf_routes":      0,
		"bgp_routes":       0,
		"default_route":    0,
	}

	for _, route := range table.Routes {
		if route.IsActive {
			metrics["active_routes"]++
		}

		switch route.Protocol {
		case "connected":
			metrics["connected_routes"]++
		case "static":
			metrics["static_routes"]++
		case "ospf":
			metrics["ospf_routes"]++
		case "bgp":
			metrics["bgp_routes"]++
		}
	}

	if table.DefaultRoute != nil {
		metrics["default_route"] = 1
	}

	return metrics
}

// ValidateFortiGateRoutingTable performs validation checks on routing table
func ValidateFortiGateRoutingTable(table *models.L3RoutingTable) []string {
	issues := []string{}

	// Check for default route
	if table.DefaultRoute == nil {
		issues = append(issues, "No default route configured (may be intentional for isolated VDOMs)")
	}

	// Check for duplicate prefixes
	prefixCount := make(map[string]int)
	for _, route := range table.Routes {
		prefixCount[route.Prefix]++
	}

	for prefix, count := range prefixCount {
		if count > 1 {
			issues = append(issues, fmt.Sprintf("Duplicate prefix: %s (%d routes - ECMP or routing loop?)", prefix, count))
		}
	}

	// Check for inactive routes
	inactiveCount := 0
	for _, route := range table.Routes {
		if !route.IsActive {
			inactiveCount++
		}
	}

	if inactiveCount > 0 {
		issues = append(issues, fmt.Sprintf("%d inactive routes in table (may indicate unreachable next-hops)", inactiveCount))
	}

	// FortiGate-specific: Check for black hole routes
	blackholeCount := 0
	for _, route := range table.Routes {
		if route.Interface == "blackhole" {
			blackholeCount++
		}
	}

	if blackholeCount > 0 {
		issues = append(issues, fmt.Sprintf("%d black hole routes configured (traffic will be dropped)", blackholeCount))
	}

	return issues
}

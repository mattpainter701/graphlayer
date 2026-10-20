package ios

import (
	"fmt"
	"time"

	"github.com/mattpainter701/graphlayer/platform/models"
)

// ========================================================================
// Phase 2: Cisco IOS L3 Routing Mappers
// ========================================================================
//
// This file contains mapper functions that transform parsed routing data
// into the unified L3RoutingTable model.
//
// Mappers:
// - BuildL3RoutingTable: Combines config + operational data
// - EnhanceRoutesWithOSPFMetadata: Adds OSPF-specific attributes
// - EnhanceRoutesWithBGPMetadata: Adds BGP-specific attributes
// - MergeRouteTables: Merges config routes with operational routes
//
// ========================================================================

// BuildL3RoutingTable builds a complete L3RoutingTable from config + show commands
func (p *CiscoIOSParser) BuildL3RoutingTable(
	deviceID string,
	hostname string,
	virtualContext string,
	virtualRouter string,
	config string,
	showIPRouteOutput string,
	ospfProcess *models.OSPFProcess,
	bgpProcess *models.BGPProcess,
) *models.L3RoutingTable {

	table := &models.L3RoutingTable{
		DeviceID:       deviceID,
		Hostname:       hostname,
		VirtualContext: virtualContext,
		VirtualRouter:  virtualRouter,
		Routes:         []models.L3Route{},
		ConfigHash:     computeSnippetHash([]string{config}),
		InsertedTime:   time.Now(),
		LastModified:   time.Now(),
		Active:         true,
	}

	// Step 1: Parse static routes from config
	staticRoutes := p.ParseStaticRoutes(config)

	// Step 2: Parse operational routing table
	operationalRoutes := p.ParseShowIPRoute(showIPRouteOutput)

	// Step 3: Merge static and operational routes
	mergedRoutes := p.mergeRoutes(staticRoutes, operationalRoutes)

	// Step 4: Enhance routes with protocol-specific metadata
	if ospfProcess != nil {
		mergedRoutes = p.enhanceRoutesWithOSPFMetadata(mergedRoutes, ospfProcess)
	}

	if bgpProcess != nil {
		mergedRoutes = p.enhanceRoutesWithBGPMetadata(mergedRoutes, bgpProcess)
	}

	// Step 5: Identify default route
	for i := range mergedRoutes {
		if mergedRoutes[i].Prefix == "0.0.0.0/0" || mergedRoutes[i].Prefix == "::/0" {
			table.DefaultRoute = &mergedRoutes[i]
			break
		}
	}

	table.Routes = mergedRoutes

	return table
}

// mergeRoutes merges config-based and operational routes
func (p *CiscoIOSParser) mergeRoutes(configRoutes, operationalRoutes []models.L3Route) []models.L3Route {
	// Use map to deduplicate by prefix
	routeMap := make(map[string]models.L3Route)

	// Add operational routes first (they are authoritative for active state)
	for _, route := range operationalRoutes {
		routeMap[route.Prefix] = route
	}

	// Merge config routes (for additional metadata like route names)
	for _, configRoute := range configRoutes {
		if existing, found := routeMap[configRoute.Prefix]; found {
			// Merge: keep operational data but add config metadata
			existing.RouteName = configRoute.RouteName
			existing.SourceBlock = existing.SourceBlock + "\n" + configRoute.SourceBlock
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

// enhanceRoutesWithOSPFMetadata adds OSPF-specific metadata to routes
func (p *CiscoIOSParser) enhanceRoutesWithOSPFMetadata(
	routes []models.L3Route,
	ospfProcess *models.OSPFProcess,
) []models.L3Route {
	enhanced := make([]models.L3Route, len(routes))
	copy(enhanced, routes)

	for i := range enhanced {
		if enhanced[i].Protocol == "ospf" {
			// Determine OSPF area from networks
			areaID := p.findOSPFArea(enhanced[i].Prefix, ospfProcess.Networks)

			// Determine route type (simplified - would parse from show ip ospf database)
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
			enhanced[i].EvidenceSources = append(enhanced[i].EvidenceSources, "ospf_process")
		}
	}

	return enhanced
}

// enhanceRoutesWithBGPMetadata adds BGP-specific metadata to routes
func (p *CiscoIOSParser) enhanceRoutesWithBGPMetadata(
	routes []models.L3Route,
	bgpProcess *models.BGPProcess,
) []models.L3Route {
	enhanced := make([]models.L3Route, len(routes))
	copy(enhanced, routes)

	for i := range enhanced {
		if enhanced[i].Protocol == "bgp" {
			// Default BGP metadata (would be enriched from show ip bgp output)
			enhanced[i].BGPMetadata = &models.BGPRouteMetadata{
				ASPath:     []int{bgpProcess.LocalAS},
				LocalPref:  100, // Default
				MED:        enhanced[i].Metric,
				Origin:     "IGP",
				PathLength: 1,
			}

			// Update evidence sources
			enhanced[i].EvidenceSources = append(enhanced[i].EvidenceSources, "bgp_process")
		}
	}

	return enhanced
}

// findOSPFArea finds the OSPF area for a given prefix
func (p *CiscoIOSParser) findOSPFArea(prefix string, networks []models.OSPFNetwork) string {
	// Simplified: would perform actual subnet matching in production
	for _, network := range networks {
		// For now, just check if network prefix matches
		if network.Network == prefix {
			return network.AreaID
		}
	}
	return "0" // Default to backbone area
}

// BuildEnhancedOSPFProcess enriches OSPF process with operational data
func (p *CiscoIOSParser) BuildEnhancedOSPFProcess(
	configProcess *models.OSPFProcess,
	neighbors []models.OSPFNeighbor,
	areas map[string]models.OSPFArea,
) *models.OSPFProcess {
	if configProcess == nil {
		return nil
	}

	enhanced := *configProcess
	enhanced.Neighbors = neighbors

	// Merge area information
	for areaID, operationalArea := range areas {
		found := false
		for i := range enhanced.Areas {
			if enhanced.Areas[i].AreaID == areaID {
				// Merge operational data
				enhanced.Areas[i].LSACount = operationalArea.LSACount
				enhanced.Areas[i].RouterCount = operationalArea.RouterCount
				enhanced.Areas[i].NetworkCount = operationalArea.NetworkCount
				found = true
				break
			}
		}

		if !found {
			enhanced.Areas = append(enhanced.Areas, operationalArea)
		}
	}

	return &enhanced
}

// BuildEnhancedBGPProcess enriches BGP process with operational data
func (p *CiscoIOSParser) BuildEnhancedBGPProcess(
	configProcess *models.BGPProcess,
	summaryNeighbors []models.BGPNeighbor,
	ribEntries []models.BGPRIBEntry,
) *models.BGPProcess {
	if configProcess == nil {
		return nil
	}

	enhanced := *configProcess

	// Merge neighbor state from summary
	for i := range enhanced.Neighbors {
		for _, summaryNeighbor := range summaryNeighbors {
			if enhanced.Neighbors[i].IPAddress == summaryNeighbor.IPAddress {
				enhanced.Neighbors[i].State = summaryNeighbor.State
				enhanced.Neighbors[i].UptimeSeconds = summaryNeighbor.UptimeSeconds
				enhanced.Neighbors[i].PrefixesReceived = summaryNeighbor.PrefixesReceived
				break
			}
		}
	}

	return &enhanced
}

// ConvertBGPRIBToRoutes converts BGP RIB entries to L3Route format
func (p *CiscoIOSParser) ConvertBGPRIBToRoutes(ribEntries []models.BGPRIBEntry, deviceID string) []models.L3Route {
	routes := []models.L3Route{}

	for _, entry := range ribEntries {
		route := models.L3Route{
			Prefix:          entry.Prefix,
			PrefixLength:    entry.PrefixLength,
			NextHop:         entry.NextHop,
			Protocol:        "bgp",
			Metric:          entry.MED,
			AdminDistance:   200, // BGP default AD
			IsActive:        entry.IsValid,
			IsBest:          entry.IsBest,
			ConfidenceScore: 65,
			EvidenceSources: []string{"show_ip_bgp"},
			LastSeen:        time.Now(),
			BGPMetadata: &models.BGPRouteMetadata{
				ASPath:       entry.Path,
				LocalPref:    entry.LocalPref,
				MED:          entry.MED,
				Origin:       entry.Origin,
				Weight:       entry.Weight,
				PathLength:   len(entry.Path),
				ReceivedFrom: entry.ReceivedFrom,
			},
		}

		routes = append(routes, route)
	}

	return routes
}

// CalculateRoutingTableMetrics calculates aggregate metrics for routing table
func (p *CiscoIOSParser) CalculateRoutingTableMetrics(table *models.L3RoutingTable) map[string]int {
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

// ValidateRoutingTable performs validation checks on routing table
func (p *CiscoIOSParser) ValidateRoutingTable(table *models.L3RoutingTable) []string {
	issues := []string{}

	// Check for default route
	if table.DefaultRoute == nil {
		issues = append(issues, "No default route configured")
	}

	// Check for duplicate prefixes
	prefixCount := make(map[string]int)
	for _, route := range table.Routes {
		prefixCount[route.Prefix]++
	}

	for prefix, count := range prefixCount {
		if count > 1 {
			issues = append(issues, fmt.Sprintf("Duplicate prefix: %s (%d routes)", prefix, count))
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
		issues = append(issues, fmt.Sprintf("%d inactive routes in table", inactiveCount))
	}

	return issues
}

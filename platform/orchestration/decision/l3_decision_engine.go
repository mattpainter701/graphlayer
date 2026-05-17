package decision

import (
	"crypto/sha256"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/mattpainter701/graphlayer/platform/models"
)

// ========================================================================
// Phase 2: L3 Decision Engine
// ========================================================================
//
// The L3 Decision Engine determines whether network traffic can reach a
// destination IP address based on routing tables (static, OSPF, BGP).
//
// Key Features:
// - Route lookup with longest prefix match
// - Multi-protocol route selection (admin distance + metric)
// - Routing loop detection
// - Black hole / null route detection
// - Confidence scoring based on route quality
// - L3-to-Firewall handoff detection
// - Decision caching for performance
//
// Decision Outcomes:
// - Allowed: Route exists and traffic can be forwarded
// - Blocked: No route, null route, or routing loop detected
//
// Confidence Scoring (0-100):
// - Route Protocol: Connected (+40), Static (+35), OSPF (+30), BGP (+25)
// - Route Age: Fresh <1hr (+20), Recent <24hr (+10), Stale >24hr (+0)
// - Admin Distance: Low AD (+15), Medium AD (+10), High AD (+5)
// - Routing Loop: Detected (-100, auto-block)
// - Black Hole: Detected (-100, auto-block)
//
// ========================================================================

// L3DecisionEngine makes L3 reachability decisions
type L3DecisionEngine struct {
	routingTables       map[string]*models.L3RoutingTable // deviceID+VRF -> routing table
	decisionCache       map[string]*models.L3DecisionCache
	cacheTTL            time.Duration
	allowedThreshold    int
	requireActiveRoute  bool
	adminDistanceMap    map[string]int
	maxHops             int // Max hops for routing path (loop detection)
}

// NewL3DecisionEngine creates a new L3 Decision Engine
func NewL3DecisionEngine() *L3DecisionEngine {
	return &L3DecisionEngine{
		routingTables:      make(map[string]*models.L3RoutingTable),
		decisionCache:      make(map[string]*models.L3DecisionCache),
		cacheTTL:           5 * time.Minute,
		allowedThreshold:   50, // Minimum confidence for Allowed
		requireActiveRoute: true,
		adminDistanceMap:   getDefaultAdminDistanceMap(),
		maxHops:            20, // Prevent infinite loops
	}
}

// getDefaultAdminDistanceMap returns vendor-agnostic admin distances
func getDefaultAdminDistanceMap() map[string]int {
	return map[string]int{
		"connected": 0,
		"static":    1,
		"eigrp":     90,
		"ospf":      110,
		"rip":       120,
		"bgp":       200,
		"unknown":   255,
	}
}

// ========================================================================
// Configuration Methods
// ========================================================================

// SetCacheTTL sets the decision cache TTL
func (e *L3DecisionEngine) SetCacheTTL(ttl time.Duration) {
	e.cacheTTL = ttl
}

// SetAllowedThreshold sets the minimum confidence for Allowed outcome
func (e *L3DecisionEngine) SetAllowedThreshold(threshold int) {
	e.allowedThreshold = threshold
}

// SetRequireActiveRoute sets whether only active routes are considered
func (e *L3DecisionEngine) SetRequireActiveRoute(require bool) {
	e.requireActiveRoute = require
}

// SetAdminDistanceMap sets custom admin distance map
func (e *L3DecisionEngine) SetAdminDistanceMap(adMap map[string]int) {
	e.adminDistanceMap = adMap
}

// SetMaxHops sets maximum hops for routing path
func (e *L3DecisionEngine) SetMaxHops(maxHops int) {
	e.maxHops = maxHops
}

// ========================================================================
// Evidence Loading Methods
// ========================================================================

// LoadRoutingTable loads a routing table for a device/VRF
func (e *L3DecisionEngine) LoadRoutingTable(table *models.L3RoutingTable) {
	key := fmt.Sprintf("%s:%s", table.DeviceID, table.VirtualContext)
	e.routingTables[key] = table
}

// LoadRoutingTables loads multiple routing tables
func (e *L3DecisionEngine) LoadRoutingTables(tables []*models.L3RoutingTable) {
	for _, table := range tables {
		e.LoadRoutingTable(table)
	}
}

// ClearRoutingTables clears all loaded routing tables
func (e *L3DecisionEngine) ClearRoutingTables() {
	e.routingTables = make(map[string]*models.L3RoutingTable)
}

// ClearCache clears the decision cache
func (e *L3DecisionEngine) ClearCache() {
	e.decisionCache = make(map[string]*models.L3DecisionCache)
}

// ========================================================================
// Core Decision Logic
// ========================================================================

// DetermineL3Reachability determines if dest IP is reachable from source
func (e *L3DecisionEngine) DetermineL3Reachability(query models.L3ReachabilityQuery) models.L3ReachabilityResult {
	startTime := time.Now()

	// Generate query hash for caching
	queryHash := e.computeQueryHash(query)

	// Check cache
	if cached, found := e.getCachedDecision(queryHash); found {
		cached.Result.DecisionTime = time.Since(startTime)
		return cached.Result
	}

	// Initialize result
	result := models.L3ReachabilityResult{
		QueryID:   queryHash[:16], // Short ID
		Query:     query,
		Timestamp: time.Now(),
		Evidence: models.L3ReachabilityEvidence{
			EvidenceSources:   []string{},
			ConfidenceFactors: make(map[string]int),
		},
	}

	// Step 1: Find routing table
	tableKey := fmt.Sprintf("%s:%s", query.SourceDevice, query.VirtualContext)
	routingTable, found := e.routingTables[tableKey]
	if !found {
		result.Outcome = "Blocked"
		result.ReasonCode = "L3_BLOCKED_NO_ROUTING_TABLE"
		result.ReasonDetail = fmt.Sprintf("No routing table found for device %s, VRF %s",
			query.SourceDevice, query.VirtualContext)
		result.ConfidenceScore = 0
		result.DecisionTime = time.Since(startTime)
		return result
	}

	// Step 2: Lookup route (longest prefix match)
	route := e.longestPrefixMatch(query.DestIP, routingTable.Routes)
	if route == nil {
		// Check for default route
		if routingTable.DefaultRoute != nil {
			route = routingTable.DefaultRoute
			result.Evidence.EvidenceSources = append(result.Evidence.EvidenceSources, "default_route")
		} else {
			result.Outcome = "Blocked"
			result.ReasonCode = "L3_BLOCKED_NO_ROUTE"
			result.ReasonDetail = fmt.Sprintf("No route found for destination %s", query.DestIP)
			result.ConfidenceScore = 0
			result.DecisionTime = time.Since(startTime)
			return result
		}
	}

	result.Evidence.RouteFound = true
	result.Evidence.RouteProtocol = route.Protocol
	result.Evidence.RouteMetric = route.Metric
	result.Evidence.RouteAdminDistance = route.AdminDistance
	result.Evidence.NextHop = route.NextHop
	result.Evidence.EgressInterface = route.Interface
	result.Evidence.EvidenceSources = append(result.Evidence.EvidenceSources, "routing_table")

	// Step 3: Check for null route / black hole
	if e.isNullRoute(route) {
		result.Outcome = "Blocked"
		result.ReasonCode = "L3_BLOCKED_NULL_ROUTE"
		result.ReasonDetail = fmt.Sprintf("Destination %s routes to null interface", query.DestIP)
		result.ConfidenceScore = 0
		result.Evidence.NullRoute = true
		result.DecisionTime = time.Since(startTime)
		return result
	}

	if e.isBlackHole(route) {
		result.Outcome = "Blocked"
		result.ReasonCode = "L3_BLOCKED_BLACK_HOLE"
		result.ReasonDetail = fmt.Sprintf("Destination %s routes to black hole", query.DestIP)
		result.ConfidenceScore = 0
		result.Evidence.BlackHole = true
		result.DecisionTime = time.Since(startTime)
		return result
	}

	// Step 4: Check if route is active
	if e.requireActiveRoute && !route.IsActive {
		result.Outcome = "Blocked"
		result.ReasonCode = "L3_BLOCKED_INACTIVE_ROUTE"
		result.ReasonDetail = fmt.Sprintf("Route exists but is not active in FIB")
		result.ConfidenceScore = 20 // Low confidence
		result.DecisionTime = time.Since(startTime)
		return result
	}

	// Step 5: Calculate confidence score
	confidenceScore := e.calculateRouteConfidence(route)
	result.ConfidenceScore = confidenceScore

	// Step 6: Determine outcome
	if confidenceScore >= e.allowedThreshold {
		result.Outcome = "Allowed"
		result.ReasonCode = "L3_ALLOWED_ROUTE_FOUND"
		result.ReasonDetail = fmt.Sprintf("Route found via %s (metric: %d, AD: %d)",
			route.Protocol, route.Metric, route.AdminDistance)
	} else {
		result.Outcome = "Blocked"
		result.ReasonCode = "L3_BLOCKED_LOW_CONFIDENCE"
		result.ReasonDetail = fmt.Sprintf("Route exists but confidence score (%d) below threshold (%d)",
			confidenceScore, e.allowedThreshold)
	}

	// Step 7: Build routing path (if Allowed)
	if result.Outcome == "Allowed" {
		routingPath := e.buildRoutingPath(query, routingTable, route)
		result.RoutingPath = routingPath

		// Check for routing loop
		if e.detectRoutingLoop(routingPath) {
			result.Outcome = "Blocked"
			result.ReasonCode = "L3_BLOCKED_ROUTING_LOOP"
			result.ReasonDetail = "Routing loop detected in path"
			result.ConfidenceScore = 0
			result.Evidence.RoutingLoopDetected = true
		}
	}

	// Step 8: Determine if firewall policy check required
	result.RequiresFirewall = e.requiresFirewallCheck(route, query)

	// Step 9: Cache decision
	result.TTLExpires = time.Now().Add(e.cacheTTL)
	result.DecisionTime = time.Since(startTime)
	e.cacheDecision(queryHash, result)

	return result
}

// ========================================================================
// Route Lookup Logic
// ========================================================================

// longestPrefixMatch finds the best matching route for an IP
func (e *L3DecisionEngine) longestPrefixMatch(destIP string, routes []models.L3Route) *models.L3Route {
	ip := net.ParseIP(destIP)
	if ip == nil {
		return nil
	}

	var bestRoute *models.L3Route
	longestPrefixLen := -1

	for i := range routes {
		route := &routes[i]

		// Parse route prefix
		_, routeNet, err := net.ParseCIDR(route.Prefix)
		if err != nil {
			continue
		}

		// Check if IP is in this subnet
		if routeNet.Contains(ip) {
			prefixLen, _ := routeNet.Mask.Size()
			if prefixLen > longestPrefixLen {
				longestPrefixLen = prefixLen
				bestRoute = route
			} else if prefixLen == longestPrefixLen {
				// Same prefix length - use admin distance + metric
				if bestRoute == nil {
					bestRoute = route
				} else if route.AdminDistance < bestRoute.AdminDistance {
					bestRoute = route
				} else if route.AdminDistance == bestRoute.AdminDistance && route.Metric < bestRoute.Metric {
					bestRoute = route
				}
			}
		}
	}

	return bestRoute
}

// isNullRoute checks if route is a null route
func (e *L3DecisionEngine) isNullRoute(route *models.L3Route) bool {
	if route == nil {
		return false
	}
	nullInterfaces := []string{"Null0", "null0", "Null", "null", "discard", "reject"}
	for _, nullIface := range nullInterfaces {
		if strings.Contains(strings.ToLower(route.Interface), strings.ToLower(nullIface)) {
			return true
		}
	}
	return route.NextHop == "0.0.0.0" && route.Interface == ""
}

// isBlackHole checks if route is a black hole (no next hop or interface)
func (e *L3DecisionEngine) isBlackHole(route *models.L3Route) bool {
	if route == nil {
		return false
	}
	// Black hole if no next hop and no interface (except for connected routes)
	if route.Protocol != "connected" && route.NextHop == "" && route.Interface == "" {
		return true
	}
	// Check for explicit drop/deny
	if strings.Contains(strings.ToLower(route.Interface), "drop") ||
		strings.Contains(strings.ToLower(route.Interface), "deny") {
		return true
	}
	return false
}

// ========================================================================
// Confidence Scoring
// ========================================================================

// calculateRouteConfidence calculates confidence score for a route
func (e *L3DecisionEngine) calculateRouteConfidence(route *models.L3Route) int {
	if route == nil {
		return 0
	}

	confidence := 0
	factors := make(map[string]int)

	// Factor 1: Route Protocol (+25 to +40)
	protocolScore := e.getProtocolScore(route.Protocol)
	confidence += protocolScore
	factors["protocol"] = protocolScore

	// Factor 2: Route Age (+0 to +20)
	ageScore := e.getAgeScore(route.Age)
	confidence += ageScore
	factors["age"] = ageScore

	// Factor 3: Administrative Distance (+5 to +15)
	adScore := e.getAdminDistanceScore(route.AdminDistance)
	confidence += adScore
	factors["admin_distance"] = adScore

	// Factor 4: Route is active (+10)
	if route.IsActive {
		confidence += 10
		factors["active"] = 10
	}

	// Factor 5: Route is best (+15)
	if route.IsBest {
		confidence += 15
		factors["best"] = 15
	}

	// Factor 6: Low metric (relative) (+0 to +10)
	metricScore := e.getMetricScore(route.Metric, route.Protocol)
	confidence += metricScore
	factors["metric"] = metricScore

	// Cap at 100
	if confidence > 100 {
		confidence = 100
	}

	return confidence
}

// getProtocolScore returns confidence score based on route protocol
func (e *L3DecisionEngine) getProtocolScore(protocol string) int {
	scores := map[string]int{
		"connected": 40, // Directly connected - highest confidence
		"static":    35, // Static route - high confidence
		"ospf":      30, // OSPF - medium-high confidence
		"eigrp":     30, // EIGRP - medium-high confidence
		"bgp":       25, // BGP - medium confidence
		"rip":       20, // RIP - lower confidence
		"isis":      30, // IS-IS - medium-high confidence
	}

	if score, found := scores[strings.ToLower(protocol)]; found {
		return score
	}
	return 15 // Unknown protocol
}

// getAgeScore returns confidence score based on route age
func (e *L3DecisionEngine) getAgeScore(ageSeconds int) int {
	if ageSeconds < 0 {
		return 0
	}

	ageHours := ageSeconds / 3600

	if ageHours < 1 {
		return 20 // Fresh - last hour
	} else if ageHours < 24 {
		return 10 // Recent - last day
	}
	return 0 // Stale - older than 24 hours
}

// getAdminDistanceScore returns confidence score based on admin distance
func (e *L3DecisionEngine) getAdminDistanceScore(ad int) int {
	if ad <= 1 {
		return 15 // Connected/Static - highest trust
	} else if ad <= 100 {
		return 10 // IGP - medium trust
	} else if ad <= 200 {
		return 5 // EGP/BGP - lower trust
	}
	return 0 // Very high AD - no bonus
}

// getMetricScore returns confidence score based on route metric
func (e *L3DecisionEngine) getMetricScore(metric int, protocol string) int {
	// Protocol-specific metric interpretation
	switch strings.ToLower(protocol) {
	case "ospf":
		if metric <= 10 {
			return 10
		} else if metric <= 100 {
			return 5
		}
		return 0
	case "bgp":
		// Lower MED is better
		if metric == 0 {
			return 10
		} else if metric <= 50 {
			return 5
		}
		return 0
	case "rip":
		// RIP hop count
		if metric <= 2 {
			return 10
		} else if metric <= 5 {
			return 5
		}
		return 0
	default:
		// Generic: lower is better
		if metric <= 10 {
			return 10
		} else if metric <= 100 {
			return 5
		}
		return 0
	}
}

// ========================================================================
// Routing Path Construction
// ========================================================================

// buildRoutingPath constructs the routing path from source to destination
func (e *L3DecisionEngine) buildRoutingPath(
	query models.L3ReachabilityQuery,
	startTable *models.L3RoutingTable,
	firstRoute *models.L3Route,
) []models.L3Hop {
	path := []models.L3Hop{}

	// First hop
	firstHop := models.L3Hop{
		DeviceID:        startTable.DeviceID,
		Hostname:        startTable.Hostname,
		EgressInterface: firstRoute.Interface,
		NextHop:         firstRoute.NextHop,
		Protocol:        firstRoute.Protocol,
		Metric:          firstRoute.Metric,
		AdminDistance:   firstRoute.AdminDistance,
	}
	path = append(path, firstHop)

	// For now, return single hop
	// TODO: Multi-hop path tracing in future iteration
	return path
}

// detectRoutingLoop checks if path contains a routing loop
func (e *L3DecisionEngine) detectRoutingLoop(path []models.L3Hop) bool {
	if len(path) > e.maxHops {
		return true // Exceeded max hops - likely a loop
	}

	// Check for duplicate devices in path
	seen := make(map[string]bool)
	for _, hop := range path {
		if seen[hop.DeviceID] {
			return true // Same device appears twice - loop detected
		}
		seen[hop.DeviceID] = true
	}

	return false
}

// ========================================================================
// Firewall Handoff Detection
// ========================================================================

// requiresFirewallCheck determines if firewall policy check is needed
func (e *L3DecisionEngine) requiresFirewallCheck(route *models.L3Route, query models.L3ReachabilityQuery) bool {
	// If route crosses zones, firewall check required
	// If protocol/port specified, firewall check required
	if query.Protocol != "" || query.Port > 0 {
		return true
	}

	// If next hop is a firewall device, firewall check required
	// TODO: Implement device type detection in future iteration

	return false
}

// ========================================================================
// Caching Logic
// ========================================================================

// computeQueryHash generates a hash of the query for caching
func (e *L3DecisionEngine) computeQueryHash(query models.L3ReachabilityQuery) string {
	data := fmt.Sprintf("%s|%s|%s|%d|%s|%s|%s",
		query.SourceIP,
		query.DestIP,
		query.Protocol,
		query.Port,
		query.SourceDevice,
		query.VirtualContext,
		query.VirtualRouter,
	)
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", hash)
}

// getCachedDecision retrieves a cached decision if valid
func (e *L3DecisionEngine) getCachedDecision(queryHash string) (*models.L3DecisionCache, bool) {
	cached, found := e.decisionCache[queryHash]
	if !found {
		return nil, false
	}

	// Check if expired
	if time.Now().After(cached.ExpiresAt) {
		delete(e.decisionCache, queryHash)
		return nil, false
	}

	return cached, true
}

// cacheDecision caches a decision result
func (e *L3DecisionEngine) cacheDecision(queryHash string, result models.L3ReachabilityResult) {
	cached := &models.L3DecisionCache{
		QueryHash: queryHash,
		Result:    result,
		CreatedAt: time.Now(),
		ExpiresAt: result.TTLExpires,
	}
	e.decisionCache[queryHash] = cached
}

// ========================================================================
// Utility Methods
// ========================================================================

// GetRoutingTable returns the routing table for a device/VRF
func (e *L3DecisionEngine) GetRoutingTable(deviceID, virtualContext string) *models.L3RoutingTable {
	key := fmt.Sprintf("%s:%s", deviceID, virtualContext)
	return e.routingTables[key]
}

// GetCacheStats returns cache statistics
func (e *L3DecisionEngine) GetCacheStats() map[string]int {
	return map[string]int{
		"total_entries": len(e.decisionCache),
		"routing_tables": len(e.routingTables),
	}
}

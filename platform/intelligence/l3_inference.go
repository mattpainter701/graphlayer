package intelligence

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/mattpainter701/graphlayer/platform/models"
)

// ========================================================================
// L3InferenceEngine — infers L3 topology from routing tables
// ========================================================================

// L3InferenceEngine infers Layer-3 topology from routing table data:
//   - Next-hop relationships → L3 links
//   - Path computation between endpoints
type L3InferenceEngine struct {
	ctx *InferenceContext

	// Cached lookups
	routesByDevice     map[string][]models.L3Route
	routesByNextHop    map[string][]models.L3Route
	devicesByIP        map[string]string // IP → device hostname
	interfacesByDevice map[string]map[string]*models.Interface // device → ifname → Interface
}

// NewL3InferenceEngine creates a new L3 inference engine.
func NewL3InferenceEngine(ctx *InferenceContext) *L3InferenceEngine {
	eng := &L3InferenceEngine{ctx: ctx}
	eng.buildIndexes()
	return eng
}

// buildIndexes constructs lookup maps from the inference context.
func (eng *L3InferenceEngine) buildIndexes() {
	eng.routesByDevice = make(map[string][]models.L3Route)
	eng.routesByNextHop = make(map[string][]models.L3Route)
	eng.devicesByIP = make(map[string]string)
	eng.interfacesByDevice = make(map[string]map[string]*models.Interface)

	// Index routes by device
	for _, rt := range eng.ctx.RoutingTables {
		for _, route := range rt.Routes {
			eng.routesByDevice[rt.Hostname] = append(eng.routesByDevice[rt.Hostname], route)
			if route.NextHop != "" {
				eng.routesByNextHop[route.NextHop] = append(eng.routesByNextHop[route.NextHop], route)
			}
		}
	}

	// Also index RoutingEntry objects
	for _, re := range eng.ctx.RoutingEntries {
		route := models.L3Route{
			Prefix:        re.Destination,
			NextHop:       re.NextHop,
			Interface:     re.Interface,
			Protocol:      re.Protocol,
			Metric:        re.Metric,
			AdminDistance: re.AdminDistance,
		}
		eng.routesByNextHop[re.NextHop] = append(eng.routesByNextHop[re.NextHop], route)
		if re.NextHop != "" {
			eng.routesByNextHop[re.NextHop] = append(eng.routesByNextHop[re.NextHop], route)
		}
	}

	// Index devices by IP
	for hostname, dev := range eng.ctx.Devices {
		if dev.ManagementIP != "" {
			eng.devicesByIP[dev.ManagementIP] = hostname
		}
	}

	// Index interfaces
	for key, iface := range eng.ctx.Interfaces {
		// Key format: "device:ifname"
		parts := strings.SplitN(key, ":", 2)
		if len(parts) != 2 {
			continue
		}
		device, ifname := parts[0], parts[1]
		if eng.interfacesByDevice[device] == nil {
			eng.interfacesByDevice[device] = make(map[string]*models.Interface)
		}
		ifaceCopy := *iface
		eng.interfacesByDevice[device][ifname] = &ifaceCopy
	}
}

// ========================================================================
// Routing Table → L3 Link Inference
// ========================================================================

// InferLinksFromRouting creates L3 links from routing table next-hop relationships.
// If device A has a route via next-hop IP X, and device B owns IP X, then A→B is a link.
func (eng *L3InferenceEngine) InferLinksFromRouting() []InferredLink {
	var links []InferredLink
	seen := make(map[string]bool)

	// Process routes from routing tables
	for _, rt := range eng.ctx.RoutingTables {
		for _, route := range rt.Routes {
			if !route.IsActive || route.NextHop == "" {
				continue
			}

			// Skip default routes for link inference (0.0.0.0/0)
			if route.Prefix == "0.0.0.0" && route.PrefixLength == 0 {
				continue
			}

			// Find the device that owns this next-hop IP
			targetDevice := eng.findDeviceByIP(route.NextHop)
			if targetDevice == "" || targetDevice == rt.Hostname {
				continue // Self or unknown
			}

			key := rt.Hostname + "|" + targetDevice
			if seen[key] {
				continue
			}
			seen[key] = true

			link := InferredLink{
				ID:              fmt.Sprintf("l3-route-%s-%s-%s", rt.Hostname, targetDevice, route.Protocol),
				FromDevice:      rt.Hostname,
				FromPort:        route.Interface,
				ToDevice:        targetDevice,
				ToPort:          "", // Unknown from routing alone
				LinkType:        "inferred-l3",
				Protocol:        route.Protocol,
				InferenceMethod: "routing-next-hop",
				Confidence:      0.70, // Good confidence from routing tables
				EvidenceSource:  models.ConfidenceSourceInferred,
				EvidenceDetail: fmt.Sprintf("Route via %s: %s → %s (%s, metric=%d, AD=%d)",
					route.NextHop, rt.Hostname, targetDevice, route.Protocol, route.Metric, route.AdminDistance),
				SupportingData: map[string]interface{}{
					"next_hop":       route.NextHop,
					"prefix":         route.Prefix,
					"prefix_length":  route.PrefixLength,
					"metric":         route.Metric,
					"admin_distance": route.AdminDistance,
				},
				InferredAt: time.Now(),
			}
			links = append(links, link)
		}
	}

	// Also process RoutingEntry objects for additional routes
	for _, re := range eng.ctx.RoutingEntries {
		if re.NextHop == "" {
			continue
		}

		targetDevice := eng.findDeviceByIP(re.NextHop)
		if targetDevice == "" {
			continue
		}

		key := "entry:" + re.NextHop + "|" + targetDevice
		if seen[key] {
			continue
		}
		seen[key] = true

		link := InferredLink{
			ID:              fmt.Sprintf("l3-entry-%s-%s-%s", re.NextHop, targetDevice, re.Protocol),
			FromDevice:      re.NextHop, // Source from the route
			ToDevice:        targetDevice,
			FromPort:        re.Interface,
			LinkType:        "inferred-l3",
			Protocol:        re.Protocol,
			InferenceMethod: "routing-next-hop",
			Confidence:      0.65,
			EvidenceSource:  models.ConfidenceSourceInferred,
			EvidenceDetail: fmt.Sprintf("Route entry via %s → %s (%s)",
				re.NextHop, targetDevice, re.Protocol),
			SupportingData: map[string]interface{}{
				"next_hop":       re.NextHop,
				"destination":    re.Destination,
				"metric":         re.Metric,
				"admin_distance": re.AdminDistance,
			},
			InferredAt: time.Now(),
		}
		links = append(links, link)
	}

	return links
}

// findDeviceByIP looks up a device by any of its IP addresses (management or interface).
func (eng *L3InferenceEngine) findDeviceByIP(ip string) string {
	// Check management IPs
	if hostname, ok := eng.devicesByIP[ip]; ok {
		return hostname
	}

	// Check interface IPs
	for key, iface := range eng.ctx.Interfaces {
		for _, addr := range iface.Addresses {
			if addr == ip || strings.Split(addr, "/")[0] == ip {
				// Extract device from key "device:ifname"
				parts := strings.SplitN(key, ":", 2)
				if len(parts) == 2 {
					return parts[0]
				}
			}
		}
	}

	return ""
}

// ========================================================================
// Path Inference
// ========================================================================

// ComputePath infers the routing path from sourceDevice to destIP.
// Uses Dijkstra-like algorithm through routing table next-hops.
func (eng *L3InferenceEngine) ComputePath(sourceDevice, destIP string) PathResult {
	result := PathResult{
		Source:      sourceDevice,
		Destination: destIP,
		ComputedAt:  time.Now(),
	}

	// Get routes for source device
	routes := eng.routesByDevice[sourceDevice]
	if len(routes) == 0 {
		result.PathType = "unknown"
		result.Confidence = 0.0
		return result
	}

	// Find the best route to destIP
	bestRoute := eng.findBestRoute(routes, destIP)
	if bestRoute == nil {
		result.PathType = "no-route"
		result.Confidence = 0.0
		return result
	}

	// Build path
	visited := make(map[string]bool)
	visited[sourceDevice] = true

	currentDevice := sourceDevice
	currentRoute := bestRoute
	maxHops := 16 // Prevent infinite loops

	hop := L3Hop{
		Device:         sourceDevice,
		NextHop:        currentRoute.NextHop,
		EgressInterface: currentRoute.Interface,
		Protocol:       currentRoute.Protocol,
		Metric:         currentRoute.Metric,
		AdminDistance:  currentRoute.AdminDistance,
	}
	result.Hops = append(result.Hops, hop)

	for i := 0; i < maxHops; i++ {
		nextHop := currentRoute.NextHop
		if nextHop == "" {
			result.PartialPath = true
			break
		}

		// Find device that owns this IP
		nextDevice := eng.findDeviceByIP(nextHop)
		if nextDevice == "" || nextDevice == currentDevice {
			// Directly connected or final hop
			result.Complete = true
			break
		}

		if visited[nextDevice] {
			result.PathType = "loop"
			result.Confidence = 0.3
			break
		}
		visited[nextDevice] = true

		// Check if this device can reach destIP directly
		nextRoutes := eng.routesByDevice[nextDevice]
		nextBest := eng.findBestRoute(nextRoutes, destIP)
		if nextBest == nil {
			result.PartialPath = true
			break
		}

		hop := L3Hop{
			Device:         nextDevice,
			NextHop:        nextBest.NextHop,
			EgressInterface: nextBest.Interface,
			Protocol:       nextBest.Protocol,
			Metric:         nextBest.Metric,
			AdminDistance:  nextBest.AdminDistance,
		}
		result.Hops = append(result.Hops, hop)

		// Check if we've reached the destination
		if eng.isDestinationReached(nextBest, destIP) {
			result.Complete = true
			break
		}

		currentDevice = nextDevice
		currentRoute = nextBest
	}

	if result.Complete {
		result.PathType = "routed"
		result.Confidence = 0.75
	} else {
		result.PathType = "partial"
		result.Confidence = 0.5
	}

	return result
}

// findBestRoute finds the most specific route to destIP (longest prefix match).
func (eng *L3InferenceEngine) findBestRoute(routes []models.L3Route, destIP string) *models.L3Route {
	dest := net.ParseIP(destIP)
	if dest == nil {
		return nil
	}

	var best *models.L3Route
	bestPrefixLen := -1

	for i, route := range routes {
		if !route.IsActive {
			continue
		}

		_, network, err := net.ParseCIDR(route.Prefix)
		if err != nil {
			continue
		}

		if network.Contains(dest) {
			prefixLen, _ := network.Mask.Size()
			if prefixLen > bestPrefixLen {
				best = &routes[i]
				bestPrefixLen = prefixLen
			}
		}
	}

	return best
}

// isDestinationReached checks if a route delivers packets to the destination.
func (eng *L3InferenceEngine) isDestinationReached(route *models.L3Route, destIP string) bool {
	if route.Protocol == "connected" || route.Protocol == "local" {
		dest := net.ParseIP(destIP)
		if dest == nil {
			return false
		}
		_, network, err := net.ParseCIDR(route.Prefix)
		if err != nil {
			return false
		}
		return network.Contains(dest)
	}
	return false
}

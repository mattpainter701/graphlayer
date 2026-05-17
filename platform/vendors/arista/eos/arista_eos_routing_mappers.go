package eos

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/mattpainter701/graphlayer/platform/models"
)

// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |R|o|u|t|i|n|g| |M|a|p|p|e|r|s|
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
/*
 * The following functions map Arista EOS routing configuration to
 * the standard L3 routing models used by the topology platform.
 */

// BuildEOSRoutingTable constructs a complete L3 routing table for the device
func (p *AristaEOSProcessor) BuildEOSRoutingTable() *models.L3RoutingTable {
	table := &models.L3RoutingTable{
		Hostname:  p.eosConfig.Hostname,
		Routes:    []models.L3Route{},
		Active:    true,
	}

	// Map static routes
	for _, route := range p.eosConfig.StaticRoutes {
		prefix := route.Prefix
		if route.PrefixLen > 0 {
			prefix = fmt.Sprintf("%s/%d", route.Prefix, route.PrefixLen)
		}
		l3Route := models.L3Route{
			Prefix:        prefix,
			PrefixLength:  route.PrefixLen,
			NextHop:       route.NextHop,
			Interface:     route.Interface,
			Protocol:      "static",
			AdminDistance: route.Distance,
			Metric:        0,
			IsActive:      true,
			IsBest:        true,
			Tag:           route.Tag,
			RouteName:     route.Name,
			ConfidenceScore: 100,
			EvidenceSources: []string{"config"},
			LastSeen:      time.Now(),
		}
		table.Routes = append(table.Routes, l3Route)
	}

	// Map OSPF routing
	if p.eosConfig.OSPFRouter != nil {
		for _, network := range p.eosConfig.OSPFRouter.Networks {
			cidr := network.Network
			if network.Wildcard != "" {
				// Convert wildcard to CIDR for OSPF network statements
				prefixLen := wildcardToPrefixLen(network.Wildcard)
				cidr = fmt.Sprintf("%s/%d", network.Network, prefixLen)
			}

			l3Route := models.L3Route{
				Prefix:        cidr,
				PrefixLength:  extractPrefixLen(cidr),
				Protocol:      "ospf",
				AdminDistance: 110,
				Metric:        0,
				IsActive:      true,
				IsBest:        true,
				ConfidenceScore: 90,
				EvidenceSources: []string{"config"},
				LastSeen:      time.Now(),
				OSPFMetadata: &models.OSPFRouteMetadata{
					AreaID: network.Area,
				},
			}
			table.Routes = append(table.Routes, l3Route)
		}
	}

	// Map BGP routing
	if p.eosConfig.BGPConfig != nil {
		for _, network := range p.eosConfig.BGPConfig.Networks {
			cidr := network
			if !strings.Contains(cidr, "/") {
				cidr = cidr + "/32"
			}
			l3Route := models.L3Route{
				Prefix:        cidr,
				PrefixLength:  extractPrefixLen(cidr),
				Protocol:      "bgp",
				AdminDistance: 200,
				Metric:        0,
				IsActive:      true,
				IsBest:        true,
				ConfidenceScore: 80,
				EvidenceSources: []string{"config"},
				LastSeen:      time.Now(),
				BGPMetadata: &models.BGPRouteMetadata{
					ASPath:    []int{p.eosConfig.BGPConfig.ASN},
					Origin:    "igp",
					LocalPref: 100,
				},
			}
			table.Routes = append(table.Routes, l3Route)
		}
	}

	// Set default route if present
	for _, route := range table.Routes {
		if route.Prefix == "0.0.0.0/0" || route.Prefix == "0.0.0.0" {
			table.DefaultRoute = &route
			break
		}
	}

	return table
}

// wildcardToPrefixLen converts a wildcard mask to prefix length
func wildcardToPrefixLen(wildcard string) int {
	ip := net.ParseIP(wildcard)
	if ip == nil {
		return 0
	}
	ipv4 := ip.To4()
	if ipv4 == nil {
		return 0
	}
	// Wildcard mask: 0.0.0.255 -> /24 (invert bits)
	mask := net.IPv4Mask(
		^ipv4[0] & 0xff,
		^ipv4[1] & 0xff,
		^ipv4[2] & 0xff,
		^ipv4[3] & 0xff,
	)
	ones, _ := mask.Size()
	return ones
}

// extractPrefixLen extracts the prefix length from a CIDR string
func extractPrefixLen(cidr string) int {
	parts := strings.Split(cidr, "/")
	if len(parts) == 2 {
		if v, err := strconv.Atoi(parts[1]); err == nil {
			return v
		}
	}
	return 32 // Default host route
}

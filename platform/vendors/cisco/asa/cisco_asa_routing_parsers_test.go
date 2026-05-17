package cisco

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestParseASARoutingTable tests parsing of ASA routing table output
func TestParseASARoutingTable(t *testing.T) {
	output := `Codes: L - local, C - connected, S - static, R - RIP, M - mobile, B - BGP
       D - EIGRP, EX - EIGRP external, O - OSPF, IA - OSPF inter area
       N1 - OSPF NSSA external type 1, N2 - OSPF NSSA external type 2
       E1 - OSPF external type 1, E2 - OSPF external type 2
       i - IS-IS, su - IS-IS summary, L1 - IS-IS level-1, L2 - IS-IS level-2

Gateway of last resort is 10.0.0.1 to network 0.0.0.0

S*   0.0.0.0 0.0.0.0 [1/0] via 10.0.0.1, outside
C    10.1.1.0 255.255.255.0 is directly connected, inside
L    10.1.1.1 255.255.255.255 is directly connected, inside
O    192.168.1.0 255.255.255.0 [110/20] via 10.1.1.2, 00:15:23, inside
O IA 192.168.100.0 255.255.255.0 [110/30] via 10.1.1.2, 00:10:45, inside
B    172.16.0.0 255.255.0.0 [200/0] via 10.1.1.3, 01:23:45
O E2 10.99.0.0 255.255.0.0 [110/1] via 10.1.1.4, 00:05:10, dmz`

	routes, err := ParseASARoutingTable(output)

	assert.NoError(t, err, "Should parse routing table without error")
	assert.Len(t, routes, 7, "Should parse all 7 routes")

	// Test static default route
	defaultRoute := routes[0]
	assert.Equal(t, "0.0.0.0/0", defaultRoute.Prefix)
	assert.Equal(t, "10.0.0.1", defaultRoute.NextHop)
	assert.Equal(t, "static", defaultRoute.Protocol)
	assert.Equal(t, uint32(1), defaultRoute.AdminDistance)
	assert.Equal(t, uint32(0), defaultRoute.Metric)
	assert.Equal(t, "outside", defaultRoute.Interface)
	assert.Equal(t, "S*", defaultRoute.Metadata["route_code"])

	// Test connected route
	connectedRoute := routes[1]
	assert.Equal(t, "10.1.1.0/24", connectedRoute.Prefix)
	assert.Equal(t, "0.0.0.0", connectedRoute.NextHop)
	assert.Equal(t, "connected", connectedRoute.Protocol)
	assert.Equal(t, uint32(0), connectedRoute.AdminDistance)
	assert.Equal(t, "inside", connectedRoute.Interface)

	// Test local route
	localRoute := routes[2]
	assert.Equal(t, "10.1.1.1/32", localRoute.Prefix)
	assert.Equal(t, "0.0.0.0", localRoute.NextHop)
	assert.Equal(t, "local", localRoute.Protocol)
	assert.Equal(t, "local_interface_address", localRoute.Metadata["route_type"])

	// Test OSPF intra-area route
	ospfRoute := routes[3]
	assert.Equal(t, "192.168.1.0/24", ospfRoute.Prefix)
	assert.Equal(t, "10.1.1.2", ospfRoute.NextHop)
	assert.Equal(t, "ospf", ospfRoute.Protocol)
	assert.Equal(t, uint32(110), ospfRoute.AdminDistance)
	assert.Equal(t, uint32(20), ospfRoute.Metric)
	assert.Equal(t, "inside", ospfRoute.Interface)
	assert.Equal(t, "00:15:23", ospfRoute.Metadata["age"])

	// Test OSPF inter-area route
	ospfInterRoute := routes[4]
	assert.Equal(t, "192.168.100.0/24", ospfInterRoute.Prefix)
	assert.Equal(t, "ospf_inter_area", ospfInterRoute.Protocol)
	assert.Equal(t, uint32(30), ospfInterRoute.Metric)

	// Test BGP route
	bgpRoute := routes[5]
	assert.Equal(t, "172.16.0.0/16", bgpRoute.Prefix)
	assert.Equal(t, "10.1.1.3", bgpRoute.NextHop)
	assert.Equal(t, "bgp", bgpRoute.Protocol)
	assert.Equal(t, uint32(200), bgpRoute.AdminDistance)

	// Test OSPF E2 route
	ospfE2Route := routes[6]
	assert.Equal(t, "10.99.0.0/16", ospfE2Route.Prefix)
	assert.Equal(t, "ospf_external_type2", ospfE2Route.Protocol)
	assert.Equal(t, "dmz", ospfE2Route.Interface)
}

// TestParseASAOSPFConfig tests parsing of OSPF configuration
func TestParseASAOSPFConfig(t *testing.T) {
	output := `router ospf 1
 router-id 10.0.0.1
 network 10.1.1.0 255.255.255.0 area 0
 network 192.168.1.0 255.255.255.0 area 10
 network 10.2.2.0 255.255.255.0 area 0
 area 10 stub
 log-adj-changes
 redistribute static subnets route-map STATIC-TO-OSPF
 redistribute connected subnets
 default-information originate`

	ospfProcess, err := ParseASAOSPFConfig(output)

	assert.NoError(t, err, "Should parse OSPF config without error")
	assert.NotNil(t, ospfProcess, "OSPF process should not be nil")

	// Test OSPF process
	assert.Equal(t, "1", ospfProcess.ProcessID)
	assert.Equal(t, "10.0.0.1", ospfProcess.RouterID)
	assert.Len(t, ospfProcess.Areas, 2, "Should have 2 OSPF areas")

	// Test redistribution
	assert.True(t, ospfProcess.RedistributeStatic, "Should redistribute static")
	assert.True(t, ospfProcess.RedistributeConnected, "Should redistribute connected")

	// Find area 0
	var area0 *models.OSPFArea
	for i := range ospfProcess.Areas {
		if ospfProcess.Areas[i].AreaID == "0" {
			area0 = &ospfProcess.Areas[i]
			break
		}
	}
	assert.NotNil(t, area0, "Should have area 0")
	assert.Equal(t, "normal", area0.AreaType)
	assert.Contains(t, area0.Networks, "10.1.1.0/24")
	assert.Contains(t, area0.Networks, "10.2.2.0/24")

	// Find area 10
	var area10 *models.OSPFArea
	for i := range ospfProcess.Areas {
		if ospfProcess.Areas[i].AreaID == "10" {
			area10 = &ospfProcess.Areas[i]
			break
		}
	}
	assert.NotNil(t, area10, "Should have area 10")
	assert.Equal(t, "stub", area10.AreaType)
	assert.Contains(t, area10.Networks, "192.168.1.0/24")
}

// TestParseASAOSPFNeighbors tests parsing of OSPF neighbor table
func TestParseASAOSPFNeighbors(t *testing.T) {
	output := `
Neighbor ID     Pri   State           Dead Time   Address         Interface
10.0.0.2          1   FULL/BDR        00:00:35    10.1.1.2        inside
10.0.0.3        100   FULL/DR         00:00:38    10.2.2.3        dmz
10.0.0.4          0   FULL/  -        00:00:32    10.3.3.4        outside`

	neighbors, err := ParseASAOSPFNeighbors(output)

	assert.NoError(t, err, "Should parse OSPF neighbors without error")
	assert.Len(t, neighbors, 3, "Should parse all 3 OSPF neighbors")

	// Test first neighbor (BDR)
	neighbor1 := neighbors[0]
	assert.Equal(t, "10.0.0.2", neighbor1.NeighborID)
	assert.Equal(t, "10.1.1.2", neighbor1.IPAddress)
	assert.Equal(t, "FULL", neighbor1.State)
	assert.Equal(t, "inside", neighbor1.Interface)
	assert.Equal(t, uint32(1), neighbor1.Priority)
	assert.Equal(t, "BDR", neighbor1.Metadata["role"])
	assert.Equal(t, "00:00:35", neighbor1.Metadata["dead_time"])

	// Test second neighbor (DR)
	neighbor2 := neighbors[1]
	assert.Equal(t, "10.0.0.3", neighbor2.NeighborID)
	assert.Equal(t, "10.2.2.3", neighbor2.IPAddress)
	assert.Equal(t, "FULL", neighbor2.State)
	assert.Equal(t, uint32(100), neighbor2.Priority)
	assert.Equal(t, "DR", neighbor2.Metadata["role"])

	// Test third neighbor (point-to-point, no DR/BDR)
	neighbor3 := neighbors[2]
	assert.Equal(t, "10.0.0.4", neighbor3.NeighborID)
	assert.Equal(t, "FULL", neighbor3.State)
	assert.Equal(t, uint32(0), neighbor3.Priority)
	assert.NotContains(t, neighbor3.Metadata, "role") // "-" role is not stored
}

// TestParseASABGPConfig tests parsing of BGP configuration
func TestParseASABGPConfig(t *testing.T) {
	output := `router bgp 65001
 bgp router-id 10.0.0.1
 bgp log-neighbor-changes
 address-family ipv4 unicast
  neighbor 203.0.113.1 remote-as 65100
  neighbor 203.0.113.1 description ISP1
  neighbor 198.51.100.1 remote-as 65200
  neighbor 198.51.100.1 description ISP2
  neighbor 10.1.1.10 remote-as 65001
  neighbor 10.1.1.10 route-reflector-client
  neighbor 10.1.1.11 remote-as 65001
  neighbor 10.1.1.11 route-reflector-client
  network 10.0.0.0 mask 255.0.0.0
  redistribute static
  redistribute connected
 exit-address-family`

	bgpProcess, err := ParseASABGPConfig(output)

	assert.NoError(t, err, "Should parse BGP config without error")
	assert.NotNil(t, bgpProcess, "BGP process should not be nil")

	// Test BGP process
	assert.Equal(t, "10.0.0.1", bgpProcess.RouterID)
	assert.Equal(t, uint32(65001), bgpProcess.LocalAS)
	assert.Len(t, bgpProcess.Neighbors, 4, "Should have 4 BGP neighbors")

	// Test redistribution
	assert.True(t, bgpProcess.RedistributeStatic, "Should redistribute static")
	assert.True(t, bgpProcess.RedistributeConnected, "Should redistribute connected")

	// Test route reflector
	assert.Equal(t, "10.0.0.1", bgpProcess.RouteReflectorID)

	// Find EBGP neighbor ISP1
	var ebgpNeighbor *models.BGPNeighbor
	for i := range bgpProcess.Neighbors {
		if bgpProcess.Neighbors[i].IPAddress == "203.0.113.1" {
			ebgpNeighbor = &bgpProcess.Neighbors[i]
			break
		}
	}
	assert.NotNil(t, ebgpNeighbor, "Should find EBGP neighbor")
	assert.Equal(t, uint32(65100), ebgpNeighbor.RemoteAS)
	assert.Equal(t, "ISP1", ebgpNeighbor.Metadata["description"])

	// Find IBGP route reflector client
	var rrClient *models.BGPNeighbor
	for i := range bgpProcess.Neighbors {
		if bgpProcess.Neighbors[i].IPAddress == "10.1.1.10" {
			rrClient = &bgpProcess.Neighbors[i]
			break
		}
	}
	assert.NotNil(t, rrClient, "Should find RR client")
	assert.Equal(t, uint32(65001), rrClient.RemoteAS)
	assert.True(t, rrClient.Metadata["is_route_reflector_client"].(bool))
}

// TestParseASABGPSummary tests parsing of BGP summary output
func TestParseASABGPSummary(t *testing.T) {
	output := `BGP router identifier 10.0.0.1, local AS number 65001
BGP table version is 145, main routing table version 145

Neighbor        V           AS MsgRcvd MsgSent   TblVer  InQ OutQ Up/Down  State/PfxRcd
203.0.113.1     4        65100    1234    1230      145    0    0 01:23:45        50
198.51.100.1    4        65200     567     563      145    0    0 00:45:12        25
10.1.1.10       4        65001     890     885      145    0    0 02:15:30        100
10.1.1.11       4        65001     450     448      145    0    0 01:05:22 Idle`

	neighbors, err := ParseASABGPSummary(output)

	assert.NoError(t, err, "Should parse BGP summary without error")
	assert.Len(t, neighbors, 4, "Should parse all 4 neighbors")

	// Test EBGP neighbor in Established state
	neighbor1 := neighbors["203.0.113.1"]
	assert.Equal(t, "203.0.113.1", neighbor1.IPAddress)
	assert.Equal(t, uint32(65100), neighbor1.RemoteAS)
	assert.Equal(t, "Established", neighbor1.State)
	assert.Equal(t, "50", neighbor1.Metadata["prefix_count"])
	assert.Equal(t, "01:23:45", neighbor1.Metadata["uptime"])
	assert.Equal(t, "1234", neighbor1.Metadata["messages_received"])
	assert.Equal(t, "1230", neighbor1.Metadata["messages_sent"])

	// Test IBGP neighbor in Established state
	neighbor3 := neighbors["10.1.1.10"]
	assert.Equal(t, uint32(65001), neighbor3.RemoteAS)
	assert.Equal(t, "Established", neighbor3.State)
	assert.Equal(t, "100", neighbor3.Metadata["prefix_count"])

	// Test neighbor in Idle state
	neighbor4 := neighbors["10.1.1.11"]
	assert.Equal(t, "Idle", neighbor4.State)
	assert.NotContains(t, neighbor4.Metadata, "prefix_count") // No prefix count when not Established
}

// TestConvertASANetmaskToCIDR tests netmask to CIDR conversion
func TestConvertASANetmaskToCIDR(t *testing.T) {
	tests := []struct {
		network  string
		netmask  string
		expected string
	}{
		{"10.0.0.0", "255.0.0.0", "10.0.0.0/8"},
		{"192.168.1.0", "255.255.255.0", "192.168.1.0/24"},
		{"172.16.0.0", "255.255.0.0", "172.16.0.0/16"},
		{"10.1.1.1", "255.255.255.255", "10.1.1.1/32"},
		{"0.0.0.0", "0.0.0.0", "0.0.0.0/0"},
		{"192.168.100.128", "255.255.255.192", "192.168.100.128/26"},
	}

	for _, test := range tests {
		result := convertASANetmaskToCIDR(test.network, test.netmask)
		assert.Equal(t, test.expected, result, "Failed for %s %s", test.network, test.netmask)
	}
}

// TestParseASARouteCode tests route code parsing
func TestParseASARouteCode(t *testing.T) {
	tests := []struct {
		code     string
		expected string
	}{
		{"C", "connected"},
		{"L", "local"},
		{"S", "static"},
		{"S*", "static"},
		{"O", "ospf"},
		{"O IA", "ospf_inter_area"},
		{"O E1", "ospf_external_type1"},
		{"O E2", "ospf_external_type2"},
		{"O N1", "ospf_nssa_type1"},
		{"O N2", "ospf_nssa_type2"},
		{"B", "bgp"},
		{"D", "eigrp"},
		{"EX", "eigrp_external"},
	}

	for _, test := range tests {
		result := parseASARouteCode(test.code)
		assert.Equal(t, test.expected, result, "Failed for code %s", test.code)
	}
}

// TestParseASARoutingTableEmpty tests empty routing table
func TestParseASARoutingTableEmpty(t *testing.T) {
	output := `Codes: L - local, C - connected, S - static

Gateway of last resort is not set`

	routes, err := ParseASARoutingTable(output)

	assert.NoError(t, err, "Should handle empty routing table")
	assert.Len(t, routes, 0, "Should return empty route slice")
}

// TestParseASAOSPFConfigMinimal tests minimal OSPF configuration
func TestParseASAOSPFConfigMinimal(t *testing.T) {
	output := `router ospf 1
 router-id 10.0.0.1
 network 10.1.1.0 255.255.255.0 area 0`

	ospfProcess, err := ParseASAOSPFConfig(output)

	assert.NoError(t, err, "Should parse minimal OSPF config")
	assert.NotNil(t, ospfProcess)
	assert.Equal(t, "1", ospfProcess.ProcessID)
	assert.Equal(t, "10.0.0.1", ospfProcess.RouterID)
	assert.Len(t, ospfProcess.Areas, 1)
	assert.Equal(t, "0", ospfProcess.Areas[0].AreaID)
	assert.Contains(t, ospfProcess.Areas[0].Networks, "10.1.1.0/24")
}

// TestParseASABGPConfigMinimal tests minimal BGP configuration
func TestParseASABGPConfigMinimal(t *testing.T) {
	output := `router bgp 65001
 bgp router-id 10.0.0.1
 address-family ipv4 unicast
  neighbor 10.1.1.2 remote-as 65002
 exit-address-family`

	bgpProcess, err := ParseASABGPConfig(output)

	assert.NoError(t, err, "Should parse minimal BGP config")
	assert.NotNil(t, bgpProcess)
	assert.Equal(t, "10.0.0.1", bgpProcess.RouterID)
	assert.Equal(t, uint32(65001), bgpProcess.LocalAS)
	assert.Len(t, bgpProcess.Neighbors, 1)
	assert.Equal(t, "10.1.1.2", bgpProcess.Neighbors[0].IPAddress)
	assert.Equal(t, uint32(65002), bgpProcess.Neighbors[0].RemoteAS)
}

// TestParseASARoutingTableEIGRP tests EIGRP route parsing
func TestParseASARoutingTableEIGRP(t *testing.T) {
	output := `Codes: D - EIGRP, EX - EIGRP external

D    10.10.0.0 255.255.0.0 [90/2297856] via 10.1.1.5, 00:30:15, inside
EX   172.20.0.0 255.255.0.0 [170/2681856] via 10.1.1.6, 01:15:45, dmz`

	routes, err := ParseASARoutingTable(output)

	assert.NoError(t, err, "Should parse EIGRP routes")
	assert.Len(t, routes, 2, "Should parse both EIGRP routes")

	// Test EIGRP internal route
	eigrpRoute := routes[0]
	assert.Equal(t, "10.10.0.0/16", eigrpRoute.Prefix)
	assert.Equal(t, "eigrp", eigrpRoute.Protocol)
	assert.Equal(t, uint32(90), eigrpRoute.AdminDistance)
	assert.Equal(t, uint32(2297856), eigrpRoute.Metric)

	// Test EIGRP external route
	eigrpExtRoute := routes[1]
	assert.Equal(t, "172.20.0.0/16", eigrpExtRoute.Prefix)
	assert.Equal(t, "eigrp_external", eigrpExtRoute.Protocol)
	assert.Equal(t, uint32(170), eigrpExtRoute.AdminDistance)
}

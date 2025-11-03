package ios

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ========================================================================
// Phase 2: Cisco IOS Routing Parsers Unit Tests
// ========================================================================

func TestParseStaticRoutes(t *testing.T) {
	parser := NewCiscoIOSParser()

	config := `
ip route 0.0.0.0 0.0.0.0 10.0.0.1
ip route 192.168.1.0 255.255.255.0 10.0.0.2 name Corporate_Network
ip route 172.16.0.0 255.255.0.0 GigabitEthernet0/0
ip route 10.1.1.0 255.255.255.0 10.0.0.3 5 name Backup_Route
`

	routes := parser.ParseStaticRoutes(config)

	assert.Len(t, routes, 4, "Expected 4 static routes")

	// Test default route
	assert.Equal(t, "0.0.0.0/0", routes[0].Prefix)
	assert.Equal(t, "10.0.0.1", routes[0].NextHop)
	assert.Equal(t, "static", routes[0].Protocol)
	assert.Equal(t, 1, routes[0].AdminDistance)

	// Test named route
	assert.Equal(t, "192.168.1.0/24", routes[1].Prefix)
	assert.Equal(t, "Corporate_Network", routes[1].RouteName)

	// Test interface-based route
	assert.Equal(t, "172.16.0.0/16", routes[2].Prefix)
	assert.Equal(t, "GigabitEthernet0/0", routes[2].Interface)

	// Test route with custom AD
	assert.Equal(t, "10.1.1.0/24", routes[3].Prefix)
	assert.Equal(t, 5, routes[3].AdminDistance)
	assert.Equal(t, "Backup_Route", routes[3].RouteName)
}

func TestParseShowIPRoute(t *testing.T) {
	parser := NewCiscoIOSParser()

	output := `
Codes: C - connected, S - static, R - RIP, M - mobile, B - BGP
       D - EIGRP, EX - EIGRP external, O - OSPF, IA - OSPF inter area

Gateway of last resort is 10.0.0.1 to network 0.0.0.0

S*   0.0.0.0/0 [1/0] via 10.0.0.1
C    10.0.0.0/24 is directly connected, GigabitEthernet0/1
     10.0.0.0/8 is variably subnetted, 3 subnets, 2 masks
O    10.1.0.0/16 [110/20] via 10.0.0.2, 00:05:23, GigabitEthernet0/0
S    192.168.1.0/24 [1/0] via 10.0.0.2
B    203.0.113.0/24 [200/0] via 10.0.0.3, 00:15:42
`

	routes := parser.ParseShowIPRoute(output)

	assert.GreaterOrEqual(t, len(routes), 4, "Expected at least 4 routes")

	// Find and test default route
	var defaultRoute *models.L3Route
	for i := range routes {
		if routes[i].Prefix == "0.0.0.0/0" {
			defaultRoute = &routes[i]
			break
		}
	}
	assert.NotNil(t, defaultRoute, "Expected to find default route")
	assert.Equal(t, "static", defaultRoute.Protocol)
	assert.True(t, defaultRoute.IsBest, "Default route should be marked as best (*)")

	// Find and test connected route
	var connectedRoute *models.L3Route
	for i := range routes {
		if routes[i].Prefix == "10.0.0.0/24" {
			connectedRoute = &routes[i]
			break
		}
	}
	assert.NotNil(t, connectedRoute, "Expected to find connected route")
	assert.Equal(t, "connected", connectedRoute.Protocol)
	assert.Equal(t, "GigabitEthernet0/1", connectedRoute.Interface)

	// Find and test OSPF route
	var ospfRoute *models.L3Route
	for i := range routes {
		if routes[i].Prefix == "10.1.0.0/16" {
			ospfRoute = &routes[i]
			break
		}
	}
	assert.NotNil(t, ospfRoute, "Expected to find OSPF route")
	assert.Equal(t, "ospf", ospfRoute.Protocol)
	assert.Equal(t, 110, ospfRoute.AdminDistance)
	assert.Equal(t, 20, ospfRoute.Metric)
}

func TestParseOSPFConfig(t *testing.T) {
	parser := NewCiscoIOSParser()

	config := `
router ospf 1
 router-id 10.0.0.1
 network 10.0.0.0 0.0.0.255 area 0
 network 10.1.0.0 0.0.255.255 area 1
 passive-interface GigabitEthernet0/2
 passive-interface Loopback0
 default-information originate
 redistribute connected subnets
!
`

	process := parser.ParseOSPFConfig(config)

	assert.NotNil(t, process, "Expected OSPF process to be parsed")
	assert.Equal(t, 1, process.ProcessID)
	assert.Equal(t, "10.0.0.1", process.RouterID)
	assert.Len(t, process.Networks, 2, "Expected 2 networks")
	assert.Equal(t, "10.0.0.0", process.Networks[0].Network)
	assert.Equal(t, "0", process.Networks[0].AreaID)
	assert.Len(t, process.PassiveInterfaces, 2, "Expected 2 passive interfaces")
	assert.True(t, process.DefaultOriginate)
	assert.Contains(t, process.RedistributedProtocols, "connected")
}

func TestParseShowIPOSPFNeighbor(t *testing.T) {
	parser := NewCiscoIOSParser()

	output := `
Neighbor ID     Pri   State           Dead Time   Address         Interface
10.0.0.2          1   FULL/DR         00:00:38    10.0.0.2        GigabitEthernet0/0
10.0.0.3          1   FULL/BDR        00:00:35    10.0.0.3        GigabitEthernet0/0
192.168.1.1       0   FULL/  -        00:00:32    192.168.1.1     Serial0/0/0
`

	neighbors := parser.ParseShowIPOSPFNeighbor(output)

	assert.Len(t, neighbors, 3, "Expected 3 OSPF neighbors")

	// Test first neighbor (DR)
	assert.Equal(t, "10.0.0.2", neighbors[0].NeighborID)
	assert.Equal(t, "FULL", neighbors[0].State)
	assert.Equal(t, "DR", neighbors[0].Role)
	assert.Equal(t, 1, neighbors[0].Priority)
	assert.Greater(t, neighbors[0].DeadTime, 0)

	// Test second neighbor (BDR)
	assert.Equal(t, "10.0.0.3", neighbors[1].NeighborID)
	assert.Equal(t, "BDR", neighbors[1].Role)

	// Test P2P neighbor
	assert.Equal(t, "192.168.1.1", neighbors[2].NeighborID)
	assert.Equal(t, 0, neighbors[2].Priority)
}

func TestParseBGPConfig(t *testing.T) {
	parser := NewCiscoIOSParser()

	config := `
router bgp 65001
 bgp router-id 10.0.0.1
 bgp log-neighbor-changes
 neighbor 10.0.0.2 remote-as 65002
 neighbor 10.0.0.2 description EBGP_to_Partner
 neighbor 10.0.0.2 update-source GigabitEthernet0/0
 neighbor 10.0.0.3 remote-as 65001
 neighbor 10.0.0.3 next-hop-self
 neighbor 10.0.0.3 route-reflector-client
 network 192.168.1.0 mask 255.255.255.0
 network 172.16.0.0
 aggregate-address 10.0.0.0 255.0.0.0 summary-only
 redistribute connected
!
`

	process := parser.ParseBGPConfig(config)

	assert.NotNil(t, process, "Expected BGP process to be parsed")
	assert.Equal(t, 65001, process.LocalAS)
	assert.Equal(t, "10.0.0.1", process.RouterID)
	assert.Len(t, process.Neighbors, 2, "Expected 2 BGP neighbors")

	// Test EBGP neighbor
	assert.Equal(t, "10.0.0.2", process.Neighbors[0].IPAddress)
	assert.Equal(t, 65002, process.Neighbors[0].RemoteAS)
	assert.True(t, process.Neighbors[0].EBGP)
	assert.Equal(t, "EBGP_to_Partner", process.Neighbors[0].Description)
	assert.Equal(t, "GigabitEthernet0/0", process.Neighbors[0].UpdateSource)

	// Test IBGP neighbor with RR
	assert.Equal(t, "10.0.0.3", process.Neighbors[1].IPAddress)
	assert.Equal(t, 65001, process.Neighbors[1].RemoteAS)
	assert.False(t, process.Neighbors[1].EBGP)
	assert.True(t, process.Neighbors[1].NextHopSelf)
	assert.True(t, process.Neighbors[1].RouteReflectorClient)

	// Test networks
	assert.Len(t, process.Networks, 2, "Expected 2 networks")
	assert.Equal(t, "192.168.1.0", process.Networks[0].Network)
	assert.Equal(t, "255.255.255.0", process.Networks[0].Mask)

	// Test aggregates
	assert.Len(t, process.AggregateAddrs, 1, "Expected 1 aggregate")
	assert.Equal(t, "10.0.0.0", process.AggregateAddrs[0].Network)
	assert.True(t, process.AggregateAddrs[0].SummaryOnly)

	// Test redistribution
	assert.Contains(t, process.RedistributedProtocols, "connected")
}

func TestParseShowIPBGPSummary(t *testing.T) {
	parser := NewCiscoIOSParser()

	output := `
BGP router identifier 10.0.0.1, local AS number 65001
BGP table version is 15, main routing table version 15
14 network entries using 3304 bytes of memory

Neighbor        V           AS MsgRcvd MsgSent   TblVer  InQ OutQ Up/Down  State/PfxRcd
10.0.0.2        4        65002   12345   12345       15    0    0 00:05:23        5
10.0.0.3        4        65001    8765    8765       15    0    0 01:23:45       10
192.168.1.1     4        65003       0       0        0    0    0 00:00:00 Idle
`

	neighbors := parser.ParseShowIPBGPSummary(output)

	assert.Len(t, neighbors, 3, "Expected 3 BGP neighbors")

	// Test established neighbor
	assert.Equal(t, "10.0.0.2", neighbors[0].IPAddress)
	assert.Equal(t, 65002, neighbors[0].RemoteAS)
	assert.Equal(t, "Established", neighbors[0].State)
	assert.Equal(t, 5, neighbors[0].PrefixesReceived)

	// Test second established neighbor
	assert.Equal(t, "10.0.0.3", neighbors[1].IPAddress)
	assert.Equal(t, 10, neighbors[1].PrefixesReceived)

	// Test idle neighbor
	assert.Equal(t, "192.168.1.1", neighbors[2].IPAddress)
	assert.Equal(t, 0, neighbors[2].PrefixesReceived)
}

func TestParseShowIPBGP(t *testing.T) {
	parser := NewCiscoIOSParser()

	output := `
BGP table version is 15, local router ID is 10.0.0.1
Status codes: s suppressed, d damped, h history, * valid, > best, i - internal

   Network          Next Hop            Metric LocPrf Weight Path
*> 192.168.1.0/24   10.0.0.2                 0    100      0 65002 i
*  192.168.2.0/24   10.0.0.3                10    100      0 65003 65004 i
*> 203.0.113.0/24   0.0.0.0                  0         32768 i
`

	entries := parser.ParseShowIPBGP(output)

	assert.Len(t, entries, 3, "Expected 3 BGP RIB entries")

	// Test best route from EBGP
	assert.Equal(t, "192.168.1.0/24", entries[0].Prefix)
	assert.Equal(t, "10.0.0.2", entries[0].NextHop)
	assert.True(t, entries[0].IsBest)
	assert.True(t, entries[0].IsValid)
	assert.Equal(t, 100, entries[0].LocalPref)
	assert.Contains(t, entries[0].Path, 65002)
	assert.Equal(t, "IGP", entries[0].Origin)

	// Test non-best route
	assert.Equal(t, "192.168.2.0/24", entries[1].Prefix)
	assert.False(t, entries[1].IsBest)
	assert.True(t, entries[1].IsValid)
	assert.Len(t, entries[1].Path, 2, "Expected 2 AS in path")
	assert.Equal(t, 65003, entries[1].Path[0])
	assert.Equal(t, 65004, entries[1].Path[1])

	// Test locally originated route
	assert.Equal(t, "203.0.113.0/24", entries[2].Prefix)
	assert.Equal(t, "0.0.0.0", entries[2].NextHop)
	assert.Equal(t, 32768, entries[2].Weight)
}

func TestConvertMaskToCIDR(t *testing.T) {
	tests := []struct {
		name         string
		ip           string
		mask         string
		expectedCIDR string
	}{
		{
			name:         "Class C mask",
			ip:           "192.168.1.0",
			mask:         "255.255.255.0",
			expectedCIDR: "192.168.1.0/24",
		},
		{
			name:         "Class B mask",
			ip:           "172.16.0.0",
			mask:         "255.255.0.0",
			expectedCIDR: "172.16.0.0/16",
		},
		{
			name:         "Class A mask",
			ip:           "10.0.0.0",
			mask:         "255.0.0.0",
			expectedCIDR: "10.0.0.0/8",
		},
		{
			name:         "Default route",
			ip:           "0.0.0.0",
			mask:         "0.0.0.0",
			expectedCIDR: "0.0.0.0/0",
		},
		{
			name:         "/30 subnet",
			ip:           "10.1.1.0",
			mask:         "255.255.255.252",
			expectedCIDR: "10.1.1.0/30",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cidr := convertMaskToCIDR(tt.ip, tt.mask)
			assert.Equal(t, tt.expectedCIDR, cidr)
		})
	}
}

func TestParseProtocolCode(t *testing.T) {
	tests := []struct {
		code             string
		expectedProtocol string
	}{
		{"C", "connected"},
		{"S", "static"},
		{"S*", "static"},
		{"O", "ospf"},
		{"D", "eigrp"},
		{"B", "bgp"},
		{"R", "rip"},
		{"I", "isis"},
		{"E", "egp"},
		{"M", "mobile"},
		{"X", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			protocol := parseProtocolCode(tt.code)
			assert.Equal(t, tt.expectedProtocol, protocol)
		})
	}
}

func TestCalculateRouteConfidenceFromProtocol(t *testing.T) {
	tests := []struct {
		protocol           string
		expectedConfidence int
	}{
		{"connected", 95},
		{"static", 85},
		{"ospf", 75},
		{"eigrp", 75},
		{"bgp", 65},
		{"rip", 50},
		{"unknown", 40},
	}

	for _, tt := range tests {
		t.Run(tt.protocol, func(t *testing.T) {
			confidence := calculateRouteConfidenceFromProtocol(tt.protocol)
			assert.Equal(t, tt.expectedConfidence, confidence)
		})
	}
}

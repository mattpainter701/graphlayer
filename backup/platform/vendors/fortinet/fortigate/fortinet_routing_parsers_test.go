package fortinet

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ========================================================================
// Phase 3: FortiGate L3 Routing Parsers Unit Tests
// ========================================================================

func TestParseFortiGateStaticRoutes(t *testing.T) {
	jsonData := `{
		"http_method": "GET",
		"results": [
			{
				"seq-num": 1,
				"status": "enable",
				"dst": "0.0.0.0/0",
				"gateway": "10.0.0.1",
				"distance": 10,
				"weight": 0,
				"priority": 0,
				"device": "wan1",
				"comment": "Default route to ISP",
				"vrf": 0,
				"bfd": "disable",
				"blackhole": "disable"
			},
			{
				"seq-num": 2,
				"status": "enable",
				"dst": "192.168.1.0/24",
				"gateway": "10.0.0.2",
				"distance": 1,
				"weight": 0,
				"priority": 0,
				"device": "port1",
				"comment": "Internal network",
				"vrf": 0,
				"bfd": "disable",
				"blackhole": "disable"
			},
			{
				"seq-num": 3,
				"status": "enable",
				"dst": "192.168.99.0/24",
				"gateway": "",
				"distance": 1,
				"weight": 0,
				"priority": 0,
				"device": "",
				"comment": "Blackhole route",
				"vrf": 0,
				"bfd": "disable",
				"blackhole": "enable"
			},
			{
				"seq-num": 4,
				"status": "disable",
				"dst": "10.1.1.0/24",
				"gateway": "10.0.0.3",
				"distance": 1,
				"weight": 0,
				"priority": 0,
				"device": "port2",
				"comment": "Disabled route",
				"vrf": 0,
				"bfd": "disable",
				"blackhole": "disable"
			}
		],
		"vdom": "root",
		"path": "router",
		"name": "static",
		"status": "success",
		"build": 2463
	}`

	routes, err := ParseFortiGateStaticRoutes(jsonData)

	assert.NoError(t, err, "Should parse without error")
	assert.Len(t, routes, 3, "Should have 3 routes (disabled route excluded)")

	// Test default route
	assert.Equal(t, "0.0.0.0/0", routes[0].Prefix)
	assert.Equal(t, "10.0.0.1", routes[0].NextHop)
	assert.Equal(t, "wan1", routes[0].Interface)
	assert.Equal(t, "static", routes[0].Protocol)
	assert.Equal(t, 10, routes[0].AdminDistance)
	assert.Equal(t, "Default route to ISP", routes[0].Description)
	assert.True(t, routes[0].IsActive)

	// Test internal network route
	assert.Equal(t, "192.168.1.0/24", routes[1].Prefix)
	assert.Equal(t, 24, routes[1].PrefixLength)
	assert.Equal(t, "10.0.0.2", routes[1].NextHop)
	assert.Equal(t, 1, routes[1].AdminDistance)

	// Test black hole route
	assert.Equal(t, "192.168.99.0/24", routes[2].Prefix)
	assert.Equal(t, "blackhole", routes[2].Interface)
	assert.Equal(t, "", routes[2].NextHop)
}

func TestParseFortiGateRoutingTable(t *testing.T) {
	jsonData := `{
		"http_method": "GET",
		"results": [
			{
				"ip_version": "4",
				"ip_mask": "0.0.0.0/0",
				"type": "static",
				"protocol": "S",
				"distance": 10,
				"metric": 0,
				"gateway": "10.0.0.1",
				"interface": "wan1",
				"age": 3600,
				"best": "*"
			},
			{
				"ip_version": "4",
				"ip_mask": "10.0.0.0/24",
				"type": "connected",
				"protocol": "C",
				"distance": 0,
				"metric": 0,
				"gateway": "0.0.0.0",
				"interface": "wan1",
				"age": 0,
				"best": "*"
			},
			{
				"ip_version": "4",
				"ip_mask": "192.168.1.0/24",
				"type": "ospf",
				"protocol": "O",
				"distance": 110,
				"metric": 20,
				"gateway": "10.0.0.2",
				"interface": "port1",
				"age": 1800,
				"best": "*"
			},
			{
				"ip_version": "4",
				"ip_mask": "203.0.113.0/24",
				"type": "bgp",
				"protocol": "B",
				"distance": 200,
				"metric": 0,
				"gateway": "10.0.0.3",
				"interface": "port2",
				"age": 7200,
				"best": ""
			}
		],
		"vdom": "root",
		"path": "router",
		"name": "ipv4",
		"status": "success",
		"build": 2463
	}`

	routes, err := ParseFortiGateRoutingTable(jsonData)

	assert.NoError(t, err, "Should parse without error")
	assert.Len(t, routes, 4, "Should have 4 routes")

	// Test default route
	assert.Equal(t, "0.0.0.0/0", routes[0].Prefix)
	assert.Equal(t, "static", routes[0].Protocol)
	assert.True(t, routes[0].IsBest)
	assert.Equal(t, 3600, routes[0].Age)

	// Test connected route
	assert.Equal(t, "10.0.0.0/24", routes[1].Prefix)
	assert.Equal(t, "connected", routes[1].Protocol)
	assert.Equal(t, 0, routes[1].AdminDistance)
	assert.True(t, routes[1].IsActive)

	// Test OSPF route
	assert.Equal(t, "192.168.1.0/24", routes[2].Prefix)
	assert.Equal(t, "ospf", routes[2].Protocol)
	assert.Equal(t, 110, routes[2].AdminDistance)
	assert.Equal(t, 20, routes[2].Metric)

	// Test BGP route (not best)
	assert.Equal(t, "203.0.113.0/24", routes[3].Prefix)
	assert.Equal(t, "bgp", routes[3].Protocol)
	assert.False(t, routes[3].IsBest)
}

func TestParseFortiGateOSPFConfig(t *testing.T) {
	jsonData := `{
		"http_method": "GET",
		"results": {
			"router-id": "10.0.0.1",
			"default-information-metric": 10,
			"default-information-metric-type": "2",
			"default-information-originate": "enable",
			"area": [
				{
					"id": "0.0.0.0",
					"type": "regular",
					"nssa-no-summary": "disable",
					"nssa-default-information-originate": "disable",
					"authentication": "none"
				},
				{
					"id": "0.0.0.1",
					"type": "stub",
					"nssa-no-summary": "enable",
					"nssa-default-information-originate": "disable",
					"authentication": "md5"
				}
			],
			"ospf-interface": [
				{
					"name": "port1-ospf",
					"interface": "port1",
					"ip": "192.168.1.1",
					"cost": 10,
					"priority": 1,
					"hello-interval": 10,
					"dead-interval": 40,
					"retransmit-interval": 5,
					"network-type": "broadcast",
					"authentication": "none"
				}
			],
			"network": [
				{
					"id": 1,
					"prefix": "192.168.1.0/24",
					"area": "0.0.0.0"
				},
				{
					"id": 2,
					"prefix": "10.0.0.0/24",
					"area": "0.0.0.1"
				}
			],
			"redistribute": [
				{
					"name": "connected",
					"status": "enable",
					"metric": 20,
					"metric-type": 2,
					"routemap": ""
				},
				{
					"name": "static",
					"status": "disable",
					"metric": 0,
					"metric-type": 1,
					"routemap": ""
				}
			],
			"passive-interface": [
				{"name": "port2"},
				{"name": "port3"}
			]
		},
		"vdom": "root",
		"path": "router",
		"name": "ospf",
		"status": "success",
		"build": 2463
	}`

	process, err := ParseFortiGateOSPFConfig(jsonData)

	assert.NoError(t, err, "Should parse without error")
	assert.NotNil(t, process, "OSPF process should not be nil")
	assert.Equal(t, "10.0.0.1", process.RouterID)
	assert.True(t, process.DefaultOriginate)
	assert.Len(t, process.Areas, 2, "Should have 2 areas")
	assert.Len(t, process.Networks, 2, "Should have 2 networks")
	assert.Len(t, process.Interfaces, 1, "Should have 1 interface")
	assert.Len(t, process.PassiveInterfaces, 2, "Should have 2 passive interfaces")

	// Test backbone area
	assert.Equal(t, "0.0.0.0", process.Areas[0].AreaID)
	assert.Equal(t, "normal", process.Areas[0].AreaType)
	assert.Equal(t, "none", process.Areas[0].AuthType)

	// Test stub area
	assert.Equal(t, "0.0.0.1", process.Areas[1].AreaID)
	assert.Equal(t, "stub", process.Areas[1].AreaType)
	assert.True(t, process.Areas[1].StubNoSummary)

	// Test OSPF interface
	assert.Equal(t, "port1", process.Interfaces[0].InterfaceName)
	assert.Equal(t, 10, process.Interfaces[0].Cost)
	assert.Equal(t, "broadcast", process.Interfaces[0].NetworkType)

	// Test redistribution
	assert.Contains(t, process.RedistributedProtocols, "connected")
	assert.NotContains(t, process.RedistributedProtocols, "static", "Disabled redistribution should not be included")
}

func TestParseFortiGateOSPFNeighbors(t *testing.T) {
	jsonData := `{
		"http_method": "GET",
		"results": [
			{
				"nbrRtrId": "10.0.0.2",
				"nbrPrio": 1,
				"state": "Full",
				"deadTime": 38,
				"address": "192.168.1.2",
				"interface": "port1",
				"areaId": "0.0.0.0"
			},
			{
				"nbrRtrId": "10.0.0.3",
				"nbrPrio": 0,
				"state": "2-Way",
				"deadTime": 35,
				"address": "192.168.1.3",
				"interface": "port1",
				"areaId": "0.0.0.0"
			}
		],
		"vdom": "root",
		"path": "router",
		"name": "ospf",
		"status": "success",
		"build": 2463
	}`

	neighbors, err := ParseFortiGateOSPFNeighbors(jsonData)

	assert.NoError(t, err, "Should parse without error")
	assert.Len(t, neighbors, 2, "Should have 2 neighbors")

	// Test FULL neighbor
	assert.Equal(t, "10.0.0.2", neighbors[0].NeighborID)
	assert.Equal(t, "Full", neighbors[0].State)
	assert.Equal(t, 1, neighbors[0].Priority)
	assert.Equal(t, 38, neighbors[0].DeadTime)
	assert.Equal(t, "0.0.0.0", neighbors[0].AreaID)

	// Test 2-WAY neighbor
	assert.Equal(t, "10.0.0.3", neighbors[1].NeighborID)
	assert.Equal(t, "2-Way", neighbors[1].State)
	assert.Equal(t, 0, neighbors[1].Priority, "Priority 0 indicates DROTHER")
}

func TestParseFortiGateBGPConfig(t *testing.T) {
	jsonData := `{
		"http_method": "GET",
		"results": {
			"as": 65001,
			"router-id": "10.0.0.1",
			"neighbor": [
				{
					"ip": "10.0.0.2",
					"remote-as": 65002,
					"update-source": "wan1",
					"next-hop-self": "enable",
					"route-reflector-client": "disable",
					"description": "EBGP to Partner",
					"password": "secret123",
					"keep-alive-timer": 60,
					"holdtime-timer": 180,
					"ebgp-multihop-ttl": 2
				},
				{
					"ip": "10.0.0.3",
					"remote-as": 65001,
					"update-source": "loopback0",
					"next-hop-self": "disable",
					"route-reflector-client": "enable",
					"description": "IBGP Route Reflector Client",
					"password": "",
					"keep-alive-timer": 60,
					"holdtime-timer": 180,
					"ebgp-multihop-ttl": 0
				}
			],
			"network": [
				{
					"id": 1,
					"prefix": "192.168.1.0/24"
				},
				{
					"id": 2,
					"prefix": "10.0.0.0/8"
				}
			],
			"aggregate-address": [
				{
					"id": 1,
					"prefix": "192.0.0.0/8",
					"summary-only": "enable",
					"as-set": "disable"
				}
			],
			"redistribute": [
				{
					"name": "connected",
					"status": "enable",
					"route-map": ""
				},
				{
					"name": "static",
					"status": "disable",
					"route-map": ""
				}
			],
			"default-local-preference": 100,
			"always-compare-med": "enable",
			"deterministic-med": "enable",
			"cluster-id": "10.0.0.1"
		},
		"vdom": "root",
		"path": "router",
		"name": "bgp",
		"status": "success",
		"build": 2463
	}`

	process, err := ParseFortiGateBGPConfig(jsonData)

	assert.NoError(t, err, "Should parse without error")
	assert.NotNil(t, process, "BGP process should not be nil")
	assert.Equal(t, 65001, process.LocalAS)
	assert.Equal(t, "10.0.0.1", process.RouterID)
	assert.True(t, process.RouteReflector)
	assert.Equal(t, "10.0.0.1", process.ClusterID)
	assert.Len(t, process.Neighbors, 2, "Should have 2 neighbors")
	assert.Len(t, process.Networks, 2, "Should have 2 networks")
	assert.Len(t, process.AggregateAddrs, 1, "Should have 1 aggregate")

	// Test EBGP neighbor
	assert.Equal(t, "10.0.0.2", process.Neighbors[0].IPAddress)
	assert.Equal(t, 65002, process.Neighbors[0].RemoteAS)
	assert.True(t, process.Neighbors[0].EBGP)
	assert.True(t, process.Neighbors[0].NextHopSelf)
	assert.False(t, process.Neighbors[0].RouteReflectorClient)
	assert.Equal(t, 2, process.Neighbors[0].Multihop)
	assert.True(t, process.Neighbors[0].PasswordConfigured)

	// Test IBGP neighbor (RR client)
	assert.Equal(t, "10.0.0.3", process.Neighbors[1].IPAddress)
	assert.Equal(t, 65001, process.Neighbors[1].RemoteAS)
	assert.False(t, process.Neighbors[1].EBGP)
	assert.True(t, process.Neighbors[1].RouteReflectorClient)
	assert.False(t, process.Neighbors[1].PasswordConfigured)

	// Test networks
	assert.Equal(t, "192.168.1.0/24", process.Networks[0].Network)

	// Test aggregate
	assert.Equal(t, "192.0.0.0/8", process.AggregateAddrs[0].Network)
	assert.True(t, process.AggregateAddrs[0].SummaryOnly)
	assert.False(t, process.AggregateAddrs[0].AsSet)

	// Test redistribution
	assert.Contains(t, process.RedistributedProtocols, "connected")
	assert.NotContains(t, process.RedistributedProtocols, "static")
}

func TestGetPrefixLengthFromCIDR(t *testing.T) {
	tests := []struct {
		cidr           string
		expectedLength int
	}{
		{"192.168.1.0/24", 24},
		{"10.0.0.0/8", 8},
		{"172.16.0.0/16", 16},
		{"0.0.0.0/0", 0},
		{"192.168.1.1/32", 32},
		{"invalid", 0},
	}

	for _, tt := range tests {
		t.Run(tt.cidr, func(t *testing.T) {
			length := getPrefixLengthFromCIDR(tt.cidr)
			assert.Equal(t, tt.expectedLength, length)
		})
	}
}

func TestNormalizeProtocol(t *testing.T) {
	tests := []struct {
		fgProtocol       string
		expectedProtocol string
	}{
		{"static", "static"},
		{"STATIC", "static"},
		{"connected", "connected"},
		{"CONNECTED", "connected"},
		{"ospf", "ospf"},
		{"bgp", "bgp"},
		{"rip", "rip"},
		{"isis", "isis"},
		{"unknown_proto", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.fgProtocol, func(t *testing.T) {
			protocol := normalizeProtocol(tt.fgProtocol)
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
		{"bgp", 65},
		{"rip", 50},
		{"isis", 75},
		{"unknown", 40},
	}

	for _, tt := range tests {
		t.Run(tt.protocol, func(t *testing.T) {
			confidence := calculateRouteConfidenceFromProtocol(tt.protocol)
			assert.Equal(t, tt.expectedConfidence, confidence)
		})
	}
}

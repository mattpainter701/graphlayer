package firewall

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestParsePANOSStaticRoutes tests parsing of PAN-OS static route configuration
func TestParsePANOSStaticRoutes(t *testing.T) {
	xmlData := `<?xml version="1.0"?>
<response status="success">
	<result>
		<entry name="default-route">
			<destination>0.0.0.0/0</destination>
			<nexthop>
				<ip-address>10.0.0.1</ip-address>
			</nexthop>
			<interface>ethernet1/1</interface>
			<metric>10</metric>
			<admin-dist>10</admin-dist>
		</entry>
		<entry name="internal-network">
			<destination>192.168.100.0/24</destination>
			<nexthop>
				<ip-address>10.1.1.1</ip-address>
			</nexthop>
			<interface>ethernet1/2</interface>
			<metric>5</metric>
			<admin-dist>1</admin-dist>
		</entry>
		<entry name="null-route">
			<destination>10.99.99.0/24</destination>
			<nexthop>
				<discard/>
			</nexthop>
			<admin-dist>5</admin-dist>
		</entry>
		<entry name="next-vr-route">
			<destination>172.16.0.0/12</destination>
			<nexthop>
				<next-vr>trust-vr</next-vr>
			</nexthop>
			<admin-dist>10</admin-dist>
		</entry>
	</result>
</response>`

	routes, err := ParsePANOSStaticRoutes(xmlData)

	assert.NoError(t, err, "Should parse static routes without error")
	assert.Len(t, routes, 4, "Should parse all 4 static routes")

	// Test default route
	defaultRoute := routes[0]
	assert.Equal(t, "0.0.0.0/0", defaultRoute.Prefix)
	assert.Equal(t, "10.0.0.1", defaultRoute.NextHop)
	assert.Equal(t, "ethernet1/1", defaultRoute.Interface)
	assert.Equal(t, "static", defaultRoute.Protocol)
	assert.Equal(t, uint32(10), defaultRoute.Metric)
	assert.Equal(t, uint32(10), defaultRoute.AdminDistance)
	assert.Equal(t, "default-route", defaultRoute.Metadata["route_name"])

	// Test internal network route
	internalRoute := routes[1]
	assert.Equal(t, "192.168.100.0/24", internalRoute.Prefix)
	assert.Equal(t, "10.1.1.1", internalRoute.NextHop)
	assert.Equal(t, "ethernet1/2", internalRoute.Interface)
	assert.Equal(t, uint32(5), internalRoute.Metric)
	assert.Equal(t, uint32(1), internalRoute.AdminDistance)

	// Test null route (discard)
	nullRoute := routes[2]
	assert.Equal(t, "10.99.99.0/24", nullRoute.Prefix)
	assert.Equal(t, "discard", nullRoute.NextHop)
	assert.Equal(t, "", nullRoute.Interface)
	assert.Equal(t, "discard", nullRoute.Metadata["nexthop_type"])

	// Test next-vr route
	nextVrRoute := routes[3]
	assert.Equal(t, "172.16.0.0/12", nextVrRoute.Prefix)
	assert.Equal(t, "next-vr:trust-vr", nextVrRoute.NextHop)
	assert.Equal(t, "next-vr", nextVrRoute.Metadata["nexthop_type"])
}

// TestParsePANOSRoutingTable tests parsing of operational routing table
func TestParsePANOSRoutingTable(t *testing.T) {
	xmlData := `<?xml version="1.0"?>
<response status="success">
	<result>
		<entry>
			<destination>0.0.0.0/0</destination>
			<nexthop>10.0.0.1</nexthop>
			<metric>10</metric>
			<flags>A S</flags>
			<age>123456</age>
			<interface>ethernet1/1</interface>
			<route-table>unicast</route-table>
		</entry>
		<entry>
			<destination>10.1.1.0/24</destination>
			<nexthop>0.0.0.0</nexthop>
			<metric>0</metric>
			<flags>A C</flags>
			<age>0</age>
			<interface>ethernet1/2</interface>
			<route-table>unicast</route-table>
		</entry>
		<entry>
			<destination>192.168.1.0/24</destination>
			<nexthop>10.1.1.2</nexthop>
			<metric>110</metric>
			<flags>A O</flags>
			<age>3600</age>
			<interface>ethernet1/2</interface>
			<route-table>unicast</route-table>
		</entry>
		<entry>
			<destination>172.16.0.0/16</destination>
			<nexthop>10.1.1.3</nexthop>
			<metric>200</metric>
			<flags>A B</flags>
			<age>7200</age>
			<interface>ethernet1/2</interface>
			<route-table>unicast</route-table>
		</entry>
	</result>
</response>`

	routes, err := ParsePANOSRoutingTable(xmlData)

	assert.NoError(t, err, "Should parse routing table without error")
	assert.Len(t, routes, 4, "Should parse all 4 routes")

	// Test static route
	staticRoute := routes[0]
	assert.Equal(t, "0.0.0.0/0", staticRoute.Prefix)
	assert.Equal(t, "10.0.0.1", staticRoute.NextHop)
	assert.Equal(t, "static", staticRoute.Protocol)
	assert.Equal(t, uint32(10), staticRoute.Metric)
	assert.Equal(t, "ethernet1/1", staticRoute.Interface)
	assert.Equal(t, "A S", staticRoute.Metadata["flags"])
	assert.Equal(t, "123456", staticRoute.Metadata["age"])

	// Test connected route
	connectedRoute := routes[1]
	assert.Equal(t, "10.1.1.0/24", connectedRoute.Prefix)
	assert.Equal(t, "0.0.0.0", connectedRoute.NextHop)
	assert.Equal(t, "connected", connectedRoute.Protocol)
	assert.Equal(t, uint32(0), connectedRoute.Metric)
	assert.Equal(t, "ethernet1/2", connectedRoute.Interface)

	// Test OSPF route
	ospfRoute := routes[2]
	assert.Equal(t, "192.168.1.0/24", ospfRoute.Prefix)
	assert.Equal(t, "10.1.1.2", ospfRoute.NextHop)
	assert.Equal(t, "ospf", ospfRoute.Protocol)
	assert.Equal(t, uint32(110), ospfRoute.Metric)

	// Test BGP route
	bgpRoute := routes[3]
	assert.Equal(t, "172.16.0.0/16", bgpRoute.Prefix)
	assert.Equal(t, "10.1.1.3", bgpRoute.NextHop)
	assert.Equal(t, "bgp", bgpRoute.Protocol)
	assert.Equal(t, uint32(200), bgpRoute.Metric)
}

// TestParsePANOSOSPFConfig tests parsing of OSPF configuration
func TestParsePANOSOSPFConfig(t *testing.T) {
	xmlData := `<?xml version="1.0"?>
<response status="success">
	<result>
		<router-id>10.0.0.1</router-id>
		<enable>yes</enable>
		<area>
			<entry name="0.0.0.0">
				<type>
					<normal/>
				</type>
				<interface>
					<entry name="ethernet1/2">
						<enable>yes</enable>
						<passive>no</passive>
						<link-type>
							<broadcast/>
						</link-type>
						<metric>10</metric>
					</entry>
					<entry name="ethernet1/3">
						<enable>yes</enable>
						<passive>yes</passive>
						<link-type>
							<p2p/>
						</link-type>
						<metric>20</metric>
					</entry>
				</interface>
				<range>
					<entry name="10.1.0.0/16">
						<action>
							<advertise/>
						</action>
					</entry>
				</range>
			</entry>
			<entry name="0.0.0.10">
				<type>
					<stub>
						<default-route>
							<disable/>
						</default-route>
					</stub>
				</type>
				<interface>
					<entry name="ethernet1/4">
						<enable>yes</enable>
						<passive>no</passive>
					</entry>
				</interface>
			</entry>
		</area>
		<redistribute>
			<connected>
				<enable>yes</enable>
				<metric>20</metric>
			</connected>
			<static>
				<enable>yes</enable>
				<metric>50</metric>
			</static>
		</redistribute>
	</result>
</response>`

	ospfProcess, err := ParsePANOSOSPFConfig(xmlData)

	assert.NoError(t, err, "Should parse OSPF config without error")
	assert.NotNil(t, ospfProcess, "OSPF process should not be nil")

	// Test OSPF process
	assert.Equal(t, "default", ospfProcess.ProcessID)
	assert.Equal(t, "10.0.0.1", ospfProcess.RouterID)
	assert.Len(t, ospfProcess.Areas, 2, "Should have 2 OSPF areas")

	// Test Area 0 (backbone)
	area0 := ospfProcess.Areas[0]
	assert.Equal(t, "0.0.0.0", area0.AreaID)
	assert.Equal(t, "normal", area0.AreaType)
	assert.Len(t, area0.Interfaces, 2, "Area 0 should have 2 interfaces")
	assert.Len(t, area0.Networks, 1, "Area 0 should have 1 summary network")

	// Test Area 0 interfaces
	iface1 := area0.Interfaces[0]
	assert.Equal(t, "ethernet1/2", iface1)

	iface2 := area0.Interfaces[1]
	assert.Equal(t, "ethernet1/3", iface2)

	// Test Area 10 (stub)
	area10 := ospfProcess.Areas[1]
	assert.Equal(t, "0.0.0.10", area10.AreaID)
	assert.Equal(t, "stub", area10.AreaType)
	assert.Len(t, area10.Interfaces, 1, "Area 10 should have 1 interface")

	// Test redistribution
	assert.True(t, ospfProcess.RedistributeConnected, "Should redistribute connected")
	assert.True(t, ospfProcess.RedistributeStatic, "Should redistribute static")
}

// TestParsePANOSOSPFNeighbors tests parsing of OSPF neighbor table
func TestParsePANOSOSPFNeighbors(t *testing.T) {
	xmlData := `<?xml version="1.0"?>
<response status="success">
	<result>
		<entry>
			<neighbor-id>10.0.0.2</neighbor-id>
			<priority>1</priority>
			<state>FULL</state>
			<dead-time>35</dead-time>
			<address>10.1.1.2</address>
			<interface>ethernet1/2</interface>
			<dr>10.1.1.1</dr>
			<bdr>10.1.1.2</bdr>
		</entry>
		<entry>
			<neighbor-id>10.0.0.3</neighbor-id>
			<priority>100</priority>
			<state>FULL</state>
			<dead-time>38</dead-time>
			<address>10.2.2.3</address>
			<interface>ethernet1/3</interface>
			<dr>10.2.2.3</dr>
			<bdr>10.2.2.1</bdr>
		</entry>
		<entry>
			<neighbor-id>10.0.0.4</neighbor-id>
			<priority>0</priority>
			<state>2WAY</state>
			<dead-time>32</dead-time>
			<address>10.1.1.4</address>
			<interface>ethernet1/2</interface>
			<dr>10.1.1.1</dr>
			<bdr>10.1.1.2</bdr>
		</entry>
	</result>
</response>`

	neighbors, err := ParsePANOSOSPFNeighbors(xmlData)

	assert.NoError(t, err, "Should parse OSPF neighbors without error")
	assert.Len(t, neighbors, 3, "Should parse all 3 OSPF neighbors")

	// Test first neighbor (FULL state)
	neighbor1 := neighbors[0]
	assert.Equal(t, "10.0.0.2", neighbor1.NeighborID)
	assert.Equal(t, "10.1.1.2", neighbor1.IPAddress)
	assert.Equal(t, "FULL", neighbor1.State)
	assert.Equal(t, "ethernet1/2", neighbor1.Interface)
	assert.Equal(t, uint32(1), neighbor1.Priority)
	assert.Equal(t, "10.1.1.1", neighbor1.Metadata["dr"])
	assert.Equal(t, "10.1.1.2", neighbor1.Metadata["bdr"])
	assert.Equal(t, "35", neighbor1.Metadata["dead_time"])

	// Test second neighbor (FULL state, high priority)
	neighbor2 := neighbors[1]
	assert.Equal(t, "10.0.0.3", neighbor2.NeighborID)
	assert.Equal(t, "10.2.2.3", neighbor2.IPAddress)
	assert.Equal(t, "FULL", neighbor2.State)
	assert.Equal(t, uint32(100), neighbor2.Priority)
	assert.Equal(t, "10.2.2.3", neighbor2.Metadata["dr"])

	// Test third neighbor (2WAY state, DROTHER)
	neighbor3 := neighbors[2]
	assert.Equal(t, "10.0.0.4", neighbor3.NeighborID)
	assert.Equal(t, "2WAY", neighbor3.State)
	assert.Equal(t, uint32(0), neighbor3.Priority)
}

// TestParsePANOSBGPConfig tests parsing of BGP configuration
func TestParsePANOSBGPConfig(t *testing.T) {
	xmlData := `<?xml version="1.0"?>
<response status="success">
	<result>
		<router-id>10.0.0.1</router-id>
		<local-as>65001</local-as>
		<enable>yes</enable>
		<peer-group>
			<entry name="EBGP-PEERS">
				<type>
					<ebgp>
						<remove-private-as>no</remove-private-as>
					</ebgp>
				</type>
				<peer>
					<entry name="ISP1">
						<peer-address>
							<ip>203.0.113.1</ip>
						</peer-address>
						<peer-as>65100</peer-as>
						<enable>yes</enable>
						<connection-options>
							<incoming-bgp-connection>
								<remote-port>0</remote-port>
								<allow>yes</allow>
							</incoming-bgp-connection>
							<outgoing-bgp-connection>
								<local-port>0</local-port>
								<allow>yes</allow>
							</outgoing-bgp-connection>
						</connection-options>
					</entry>
					<entry name="ISP2">
						<peer-address>
							<ip>198.51.100.1</ip>
						</peer-address>
						<peer-as>65200</peer-as>
						<enable>yes</enable>
					</entry>
				</peer>
			</entry>
			<entry name="IBGP-PEERS">
				<type>
					<ibgp>
						<cluster-id>10.0.0.1</cluster-id>
					</ibgp>
				</type>
				<peer>
					<entry name="RR-CLIENT-1">
						<peer-address>
							<ip>10.1.1.10</ip>
						</peer-address>
						<peer-as>65001</peer-as>
						<enable>yes</enable>
						<reflector-client>yes</reflector-client>
					</entry>
					<entry name="RR-CLIENT-2">
						<peer-address>
							<ip>10.1.1.11</ip>
						</peer-address>
						<peer-as>65001</peer-as>
						<enable>yes</enable>
						<reflector-client>yes</reflector-client>
					</entry>
				</peer>
			</entry>
		</peer-group>
		<redistribute>
			<connected>
				<enable>yes</enable>
			</connected>
			<static>
				<enable>yes</enable>
			</static>
		</redistribute>
	</result>
</response>`

	bgpProcess, err := ParsePANOSBGPConfig(xmlData)

	assert.NoError(t, err, "Should parse BGP config without error")
	assert.NotNil(t, bgpProcess, "BGP process should not be nil")

	// Test BGP process
	assert.Equal(t, "10.0.0.1", bgpProcess.RouterID)
	assert.Equal(t, uint32(65001), bgpProcess.LocalAS)
	assert.Len(t, bgpProcess.Neighbors, 4, "Should have 4 BGP neighbors total")

	// Test EBGP neighbor 1
	var ebgpNeighbor1 *struct {
		NeighborID string
		IPAddress  string
		RemoteAS   uint32
		PeerGroup  string
	}
	for _, n := range bgpProcess.Neighbors {
		if n.IPAddress == "203.0.113.1" {
			ebgpNeighbor1 = &struct {
				NeighborID string
				IPAddress  string
				RemoteAS   uint32
				PeerGroup  string
			}{
				NeighborID: n.NeighborID,
				IPAddress:  n.IPAddress,
				RemoteAS:   n.RemoteAS,
				PeerGroup:  n.Metadata["peer_group"].(string),
			}
			break
		}
	}
	assert.NotNil(t, ebgpNeighbor1, "Should find EBGP neighbor ISP1")
	assert.Equal(t, "203.0.113.1", ebgpNeighbor1.IPAddress)
	assert.Equal(t, uint32(65100), ebgpNeighbor1.RemoteAS)
	assert.Equal(t, "EBGP-PEERS", ebgpNeighbor1.PeerGroup)

	// Test IBGP route reflector client
	var rrClient1 *struct {
		IPAddress         string
		RemoteAS          uint32
		IsRouteReflector  bool
		RouteReflectorID  string
	}
	for _, n := range bgpProcess.Neighbors {
		if n.IPAddress == "10.1.1.10" {
			rrClient1 = &struct {
				IPAddress         string
				RemoteAS          uint32
				IsRouteReflector  bool
				RouteReflectorID  string
			}{
				IPAddress:        n.IPAddress,
				RemoteAS:         n.RemoteAS,
				IsRouteReflector: n.Metadata["is_route_reflector_client"].(bool),
				RouteReflectorID: bgpProcess.RouteReflectorID,
			}
			break
		}
	}
	assert.NotNil(t, rrClient1, "Should find IBGP RR client")
	assert.Equal(t, "10.1.1.10", rrClient1.IPAddress)
	assert.Equal(t, uint32(65001), rrClient1.RemoteAS)
	assert.True(t, rrClient1.IsRouteReflector, "Should be route reflector client")
	assert.Equal(t, "10.0.0.1", rrClient1.RouteReflectorID)

	// Test redistribution
	assert.True(t, bgpProcess.RedistributeConnected, "Should redistribute connected")
	assert.True(t, bgpProcess.RedistributeStatic, "Should redistribute static")
}

// TestParsePANOSStaticRoutesEmpty tests empty static route configuration
func TestParsePANOSStaticRoutesEmpty(t *testing.T) {
	xmlData := `<?xml version="1.0"?>
<response status="success">
	<result/>
</response>`

	routes, err := ParsePANOSStaticRoutes(xmlData)

	assert.NoError(t, err, "Should handle empty response without error")
	assert.Len(t, routes, 0, "Should return empty route slice")
}

// TestParsePANOSRoutingTableMalformed tests error handling for malformed XML
func TestParsePANOSRoutingTableMalformed(t *testing.T) {
	xmlData := `<?xml version="1.0"?>
<response status="success">
	<result>
		<entry>
			<destination>INVALID_PREFIX</destination>
			<nexthop>10.0.0.1</nexthop>
		</entry>
	</result>
</response>`

	routes, err := ParsePANOSRoutingTable(xmlData)

	// Should still parse but may have warnings or skip invalid entries
	assert.NoError(t, err, "Should handle malformed data gracefully")
	// Implementation detail: may skip invalid entries
	_ = routes
}

// TestParsePANOSOSPFConfigMinimal tests minimal OSPF configuration
func TestParsePANOSOSPFConfigMinimal(t *testing.T) {
	xmlData := `<?xml version="1.0"?>
<response status="success">
	<result>
		<router-id>10.0.0.1</router-id>
		<enable>yes</enable>
		<area>
			<entry name="0.0.0.0">
				<type>
					<normal/>
				</type>
			</entry>
		</area>
	</result>
</response>`

	ospfProcess, err := ParsePANOSOSPFConfig(xmlData)

	assert.NoError(t, err, "Should parse minimal OSPF config")
	assert.NotNil(t, ospfProcess)
	assert.Equal(t, "10.0.0.1", ospfProcess.RouterID)
	assert.Len(t, ospfProcess.Areas, 1)
	assert.Equal(t, "0.0.0.0", ospfProcess.Areas[0].AreaID)
}

// TestParsePANOSBGPConfigMinimal tests minimal BGP configuration
func TestParsePANOSBGPConfigMinimal(t *testing.T) {
	xmlData := `<?xml version="1.0"?>
<response status="success">
	<result>
		<router-id>10.0.0.1</router-id>
		<local-as>65001</local-as>
		<enable>yes</enable>
	</result>
</response>`

	bgpProcess, err := ParsePANOSBGPConfig(xmlData)

	assert.NoError(t, err, "Should parse minimal BGP config")
	assert.NotNil(t, bgpProcess)
	assert.Equal(t, "10.0.0.1", bgpProcess.RouterID)
	assert.Equal(t, uint32(65001), bgpProcess.LocalAS)
	assert.Len(t, bgpProcess.Neighbors, 0, "Should have no neighbors")
}

// TestParsePANOSRoutingTableIPv6 tests IPv6 route parsing
func TestParsePANOSRoutingTableIPv6(t *testing.T) {
	xmlData := `<?xml version="1.0"?>
<response status="success">
	<result>
		<entry>
			<destination>::/0</destination>
			<nexthop>2001:db8::1</nexthop>
			<metric>10</metric>
			<flags>A S</flags>
			<age>123456</age>
			<interface>ethernet1/1</interface>
			<route-table>unicast</route-table>
		</entry>
		<entry>
			<destination>2001:db8:1::/64</destination>
			<nexthop>::</nexthop>
			<metric>0</metric>
			<flags>A C</flags>
			<age>0</age>
			<interface>ethernet1/2</interface>
			<route-table>unicast</route-table>
		</entry>
	</result>
</response>`

	routes, err := ParsePANOSRoutingTable(xmlData)

	assert.NoError(t, err, "Should parse IPv6 routes")
	assert.Len(t, routes, 2, "Should parse both IPv6 routes")

	// Test IPv6 default route
	assert.Equal(t, "::/0", routes[0].Prefix)
	assert.Equal(t, "2001:db8::1", routes[0].NextHop)
	assert.Equal(t, "static", routes[0].Protocol)

	// Test IPv6 connected route
	assert.Equal(t, "2001:db8:1::/64", routes[1].Prefix)
	assert.Equal(t, "::", routes[1].NextHop)
	assert.Equal(t, "connected", routes[1].Protocol)
}

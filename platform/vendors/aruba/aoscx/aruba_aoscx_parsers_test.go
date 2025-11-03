package aoscx

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestParseAOSCXVLANs tests VLAN configuration parsing
func TestParseAOSCXVLANs(t *testing.T) {
	config := `
vlan 1
    name DEFAULT_VLAN

vlan 10
    name Engineering
    description Engineering Department VLAN

vlan 20
    name IT
    description IT Department

vlan 30
    name Guest
    shutdown

vlan 40
    name Voice
    description Voice VLAN for IP Phones
`

	vlans, err := ParseAOSCXVLANs(config)

	assert.NoError(t, err)
	assert.Len(t, vlans, 5, "Should parse all 5 VLANs")

	// Test VLAN 1
	vlan1 := vlans[0]
	assert.Equal(t, 1, vlan1.VLANID)
	assert.Equal(t, "DEFAULT_VLAN", vlan1.Name)
	assert.Equal(t, "active", vlan1.State)

	// Test VLAN 10
	vlan10 := vlans[1]
	assert.Equal(t, 10, vlan10.VLANID)
	assert.Equal(t, "Engineering", vlan10.Name)
	assert.Equal(t, "Engineering Department VLAN", vlan10.Description)
	assert.Equal(t, "active", vlan10.State)

	// Test VLAN 30 (suspended)
	vlan30 := vlans[3]
	assert.Equal(t, 30, vlan30.VLANID)
	assert.Equal(t, "Guest", vlan30.Name)
	assert.Equal(t, "suspended", vlan30.State)

	// Check snippet hashes are generated
	for _, vlan := range vlans {
		assert.NotEmpty(t, vlan.SourceSnippetHash, "VLAN %d should have snippet hash", vlan.VLANID)
	}
}

// TestParseAOSCXInterfaces tests interface configuration parsing
func TestParseAOSCXInterfaces(t *testing.T) {
	config := `
interface 1/1/1
    no shutdown
    description Access Port to Engineering
    vlan access 10
    ip dhcp-snooping trust

interface 1/1/2
    no shutdown
    description Access Port with Security
    vlan access 20
    port-access role employee
    mac-authentication

interface 1/1/24
    no shutdown
    description Trunk to Core Switch
    vlan trunk native 1
    vlan trunk allowed 10,20,30-40

interface 1/1/25
    no shutdown
    description LAG Member 1
    lag 1

interface 1/1/26
    no shutdown
    description LAG Member 2
    lag 1
`

	interfaces, err := ParseAOSCXInterfaces(config)

	assert.NoError(t, err)
	assert.Len(t, interfaces, 5, "Should parse all 5 interfaces")

	// Test access interface
	iface1 := interfaces[0]
	assert.Equal(t, "1/1/1", iface1.InterfaceName)
	assert.Equal(t, "up", iface1.AdminState)
	assert.Equal(t, "access", iface1.Mode)
	assert.Equal(t, 10, iface1.AccessVLAN)
	assert.True(t, iface1.DHCPSnoopingTrust)

	// Test interface with security
	iface2 := interfaces[1]
	assert.Equal(t, "1/1/2", iface2.InterfaceName)
	assert.Equal(t, 20, iface2.AccessVLAN)
	assert.True(t, iface2.Dot1XRequired, "Should detect port-access")
	assert.True(t, iface2.MABEnabled, "Should detect mac-authentication")

	// Test trunk interface
	iface24 := interfaces[2]
	assert.Equal(t, "1/1/24", iface24.InterfaceName)
	assert.Equal(t, "trunk", iface24.Mode)
	assert.Equal(t, 1, iface24.TrunkNativeVLAN)
	assert.Contains(t, iface24.TrunkAllowedVLANs, 10)
	assert.Contains(t, iface24.TrunkAllowedVLANs, 20)
	assert.Contains(t, iface24.TrunkAllowedVLANs, 30)
	assert.Contains(t, iface24.TrunkAllowedVLANs, 35)
	assert.Contains(t, iface24.TrunkAllowedVLANs, 40)

	// Test LAG member interfaces
	iface25 := interfaces[3]
	assert.Equal(t, 1, iface25.PortChannelID)
	assert.Equal(t, "lag 1", iface25.PortChannelName)

	iface26 := interfaces[4]
	assert.Equal(t, 1, iface26.PortChannelID)
}

// TestParseAOSCXPortChannels tests LAG configuration parsing
func TestParseAOSCXPortChannels(t *testing.T) {
	config := `
interface lag 1
    no shutdown
    description Uplink to Core
    vlan trunk native 1
    vlan trunk allowed all
    lacp mode active

interface lag 2
    no shutdown
    description Access LAG
    vlan access 10
    lacp mode passive

interface 1/1/25
    lag 1

interface 1/1/26
    lag 1

interface 1/1/27
    lag 2

interface 1/1/28
    lag 2
`

	portChannels, err := ParseAOSCXPortChannels(config)

	assert.NoError(t, err)
	assert.Len(t, portChannels, 2, "Should parse both LAGs")

	// Test LAG 1 (trunk)
	lag1 := portChannels[0]
	assert.Equal(t, 1, lag1.PortChannelID)
	assert.Equal(t, "lag 1", lag1.PortChannelName)
	assert.Equal(t, "trunk", lag1.Mode)
	assert.Equal(t, 1, lag1.TrunkNativeVLAN)
	assert.Equal(t, "active", lag1.LACPMode)
	assert.Len(t, lag1.MemberInterfaces, 2, "LAG 1 should have 2 members")
	assert.Contains(t, lag1.MemberInterfaces, "1/1/25")
	assert.Contains(t, lag1.MemberInterfaces, "1/1/26")

	// Test LAG 2 (access)
	lag2 := portChannels[1]
	assert.Equal(t, 2, lag2.PortChannelID)
	assert.Equal(t, "access", lag2.Mode)
	assert.Equal(t, 10, lag2.AccessVLAN)
	assert.Equal(t, "passive", lag2.LACPMode)
	assert.Len(t, lag2.MemberInterfaces, 2, "LAG 2 should have 2 members")
}

// TestParseAOSCXSVIs tests SVI configuration parsing
func TestParseAOSCXSVIs(t *testing.T) {
	config := `
interface vlan 10
    ip address 192.168.10.1/24
    description Engineering Gateway
    no shutdown

interface vlan 20
    ip address 192.168.20.1 255.255.255.0
    description IT Gateway
    no shutdown

interface vlan 30
    description Guest Gateway (disabled)
    shutdown
`

	svis, err := ParseAOSCXSVIs(config)

	assert.NoError(t, err)
	assert.Len(t, svis, 3, "Should parse all 3 SVIs")

	// Test SVI 10 (CIDR notation)
	svi10 := svis[0]
	assert.Equal(t, 10, svi10.VLANID)
	assert.Equal(t, "192.168.10.1", svi10.IPAddress)
	assert.Equal(t, "Engineering Gateway", svi10.Description)
	assert.Equal(t, "up", svi10.AdminState)

	// Test SVI 20 (subnet mask notation)
	svi20 := svis[1]
	assert.Equal(t, 20, svi20.VLANID)
	assert.Equal(t, "192.168.20.1", svi20.IPAddress)
	assert.Equal(t, "255.255.255.0", svi20.SubnetMask)
	assert.Equal(t, "up", svi20.AdminState)

	// Test SVI 30 (down)
	svi30 := svis[2]
	assert.Equal(t, 30, svi30.VLANID)
	assert.Equal(t, "down", svi30.AdminState)
}

// TestParseAOSCXMACAddressTable tests MAC address table parsing
func TestParseAOSCXMACAddressTable(t *testing.T) {
	output := `MAC Address Table

VLAN    MAC Address       Port        Type
----    -----------       ----        ----
10      00:11:22:33:44:55 1/1/1       dynamic
10      00:11:22:33:44:66 1/1/2       dynamic
20      00:aa:bb:cc:dd:ee 1/1/24      dynamic
30      00:ff:ee:dd:cc:bb lag1        dynamic
`

	entries, err := ParseAOSCXMACAddressTable(output)

	assert.NoError(t, err)
	assert.Len(t, entries, 4, "Should parse all 4 MAC entries")

	// Test first entry
	entry1 := entries[0]
	assert.Equal(t, 10, entry1.VLANID)
	assert.Equal(t, "00:11:22:33:44:55", entry1.MACAddress)
	assert.Equal(t, "1/1/1", entry1.Port)
	assert.Equal(t, "dynamic", entry1.Type)
	assert.Equal(t, 1.0, entry1.Confidence)
	assert.Equal(t, "mac-address-table", entry1.EvidenceSource)

	// Test LAG entry
	entry4 := entries[3]
	assert.Equal(t, "lag1", entry4.Port)
}

// TestParseAOSCXLLDPNeighbors tests LLDP neighbor parsing
func TestParseAOSCXLLDPNeighbors(t *testing.T) {
	output := `LLDP Neighbor Information

Local Port: 1/1/24
Chassis ID: 00:11:22:33:44:55
Port ID: Ethernet1
System Name: core-switch-01
System Description: Aruba JL635A 8360-24XF2C Switch
Capabilities: Bridge, Router
Management Address: 192.168.1.1

Local Port: 1/1/25
Chassis ID: 00:aa:bb:cc:dd:ee
Port ID: Ethernet2
System Name: dist-switch-02
System Description: Aruba 6300M
Capabilities: Bridge
Management Address: 192.168.1.2
`

	neighbors, err := ParseAOSCXLLDPNeighbors(output)

	assert.NoError(t, err)
	assert.Len(t, neighbors, 2, "Should parse both LLDP neighbors")

	// Test first neighbor
	neighbor1 := neighbors[0]
	assert.Equal(t, "1/1/24", neighbor1.LocalInterface)
	assert.Equal(t, "00:11:22:33:44:55", neighbor1.ChassisID)
	assert.Equal(t, "Ethernet1", neighbor1.RemotePortID)
	assert.Equal(t, "core-switch-01", neighbor1.RemoteSystemName)
	assert.Contains(t, neighbor1.RemoteSystemDesc, "8360")
	assert.Len(t, neighbor1.Capabilities, 2)
	assert.Contains(t, neighbor1.Capabilities, "Bridge")
	assert.Contains(t, neighbor1.Capabilities, "Router")
	assert.Equal(t, "192.168.1.1", neighbor1.ManagementIP)

	// Test second neighbor
	neighbor2 := neighbors[1]
	assert.Equal(t, "1/1/25", neighbor2.LocalInterface)
	assert.Equal(t, "dist-switch-02", neighbor2.RemoteSystemName)
}

// TestParseAOSCXDHCPSnoopingBinding tests DHCP snooping binding parsing
func TestParseAOSCXDHCPSnoopingBinding(t *testing.T) {
	output := `DHCP Snooping Binding Table

MAC Address       IP Address      VLAN  Interface  Lease Time
---------------   -------------   ----  ---------  ----------
00:11:22:33:44:55 192.168.10.100  10    1/1/1      86400
00:aa:bb:cc:dd:ee 192.168.10.101  10    1/1/2      86400
00:ff:ee:dd:cc:bb 192.168.20.50   20    1/1/5      43200
`

	bindings, err := ParseAOSCXDHCPSnoopingBinding(output)

	assert.NoError(t, err)
	assert.Len(t, bindings, 3, "Should parse all 3 bindings")

	// Test first binding
	binding1 := bindings[0]
	assert.Equal(t, "00:11:22:33:44:55", binding1.MACAddress)
	assert.Equal(t, "192.168.10.100", binding1.IPAddress)
	assert.Equal(t, 10, binding1.VLANID)
	assert.Equal(t, "1/1/1", binding1.Interface)
	assert.Equal(t, 86400, binding1.LeaseTime)
	assert.Equal(t, "dynamic", binding1.BindingType)

	// Test third binding (different lease time)
	binding3 := bindings[2]
	assert.Equal(t, 43200, binding3.LeaseTime)
}

// TestParseAOSCXAuthSessions tests authentication session parsing
func TestParseAOSCXAuthSessions(t *testing.T) {
	output := `Port Access Client Status

Interface  Client MAC        Auth Method  Auth State        VLAN
---------  ----------------  -----------  ----------------  ----
1/1/5      00:11:22:33:44:55 dot1x        authenticated     10
1/1/6      00:aa:bb:cc:dd:ee mac-auth     authenticated     20
1/1/7      00:ff:ee:dd:cc:bb dot1x        unauthenticated   999
`

	sessions, err := ParseAOSCXAuthSessions(output)

	assert.NoError(t, err)
	assert.Len(t, sessions, 3, "Should parse all 3 sessions")

	// Test 802.1X session
	session1 := sessions[0]
	assert.Equal(t, "1/1/5", session1.Interface)
	assert.Equal(t, "00:11:22:33:44:55", session1.MACAddress)
	assert.Equal(t, "dot1x", session1.AuthMethod)
	assert.Equal(t, "authenticated", session1.AuthState)
	assert.Equal(t, 10, session1.VLANID)

	// Test MAB session
	session2 := sessions[1]
	assert.Equal(t, "mab", session2.AuthMethod, "Should map mac-auth to mab")
	assert.Equal(t, 20, session2.VLANID)

	// Test unauthenticated session
	session3 := sessions[2]
	assert.Equal(t, "unauthenticated", session3.AuthState)
	assert.Equal(t, 999, session3.VLANID, "Should be in guest/unauth VLAN")
}

// TestParseVLANList tests VLAN list parsing helper function
func TestParseVLANList(t *testing.T) {
	tests := []struct {
		input    string
		expected []int
	}{
		{"10", []int{10}},
		{"10,20,30", []int{10, 20, 30}},
		{"10-15", []int{10, 11, 12, 13, 14, 15}},
		{"10,20-22,30", []int{10, 20, 21, 22, 30}},
		{"all", []int{}}, // Empty slice means all VLANs
	}

	for _, test := range tests {
		result := parseVLANList(test.input)
		if test.input == "all" {
			assert.Empty(t, result, "all should return empty slice")
		} else {
			assert.Equal(t, test.expected, result, "Failed for input: %s", test.input)
		}
	}
}

// TestParseAOSCXConfig tests complete config parsing integration
func TestParseAOSCXConfig(t *testing.T) {
	config := `! ArubaOS-CX Configuration
! Version 10.12.0001
hostname test-switch-01

vlan 1
    name DEFAULT_VLAN

vlan 10
    name Engineering

interface 1/1/1
    no shutdown
    description Access Port
    vlan access 10

interface vlan 10
    ip address 192.168.10.1/24
    no shutdown
`

	l2Config, err := ParseAOSCXConfig(config, "test-switch-01")

	assert.NoError(t, err)
	assert.NotNil(t, l2Config)
	assert.Equal(t, "test-switch-01", l2Config.Device.DeviceID)
	assert.Equal(t, "test-switch-01", l2Config.Device.Hostname)
	assert.Equal(t, "aos-cx", l2Config.Device.Platform)
	assert.Len(t, l2Config.VLANs, 2, "Should have 2 VLANs")
	assert.Len(t, l2Config.Interfaces, 1, "Should have 1 interface")
	assert.Len(t, l2Config.SVIs, 1, "Should have 1 SVI")
	assert.NotEmpty(t, l2Config.Device.ConfigHash, "Should have config hash")
}

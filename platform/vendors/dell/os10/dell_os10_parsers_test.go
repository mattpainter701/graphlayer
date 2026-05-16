package os10

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mattpainter701/graphlayer/platform/platform/models"
)

// Test data directory (relative to this file)
const testDataDir = "testdata"

// readTestFile helper
func readTestFile(t *testing.T, filename string) string {
	t.Helper()
	path := filepath.Join(testDataDir, filename)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read test file %s: %v", path, err)
	}
	return string(data)
}

// ===========================
// Config Parser Tests
// ===========================

func TestParseOS10InterfaceBlock_Trunk(t *testing.T) {
	block := []string{
		"interface ethernet1/1/5",
		"   description PoC-to-spine01",
		"   switchport mode trunk",
		"   switchport trunk allowed vlan 10,20,30,100",
		"   switchport trunk native vlan 200",
		"   channel-group 10 mode active",
		"   no shutdown",
	}

	result := parseOS10InterfaceBlock(block)

	if result.Description != "PoC-to-spine01" {
		t.Errorf("Expected description 'PoC-to-spine01', got '%s'", result.Description)
	}
	if result.Mode != "trunk" {
		t.Errorf("Expected mode 'trunk', got '%s'", result.Mode)
	}
	if result.TrunkNative != 200 {
		t.Errorf("Expected trunk native VLAN 200, got %d", result.TrunkNative)
	}
	if len(result.TrunkAllowed) != 4 {
		t.Errorf("Expected 4 allowed VLANs, got %d", len(result.TrunkAllowed))
	}
	if result.ChannelGroup != 10 {
		t.Errorf("Expected channel-group 10, got %d", result.ChannelGroup)
	}
	if !result.LACPActive {
		t.Error("Expected LACP active")
	}
	if result.Shutdown {
		t.Error("Expected no shutdown (false)")
	}
}

func TestParseOS10InterfaceBlock_Access(t *testing.T) {
	block := []string{
		"interface ethernet1/1/3",
		"   description server-rack1-port01",
		"   switchport mode access",
		"   switchport access vlan 10",
		"   spanning-tree portfast",
		"   no shutdown",
	}

	result := parseOS10InterfaceBlock(block)

	if result.Mode != "access" {
		t.Errorf("Expected mode 'access', got '%s'", result.Mode)
	}
	if result.Access != 10 {
		t.Errorf("Expected access VLAN 10, got %d", result.Access)
	}
	if result.Shutdown {
		t.Error("Expected no shutdown")
	}
}

func TestParseOS10InterfaceBlock_Shutdown(t *testing.T) {
	block := []string{
		"interface ethernet1/1/7",
		"   description unused-port",
		"   shutdown",
	}

	result := parseOS10InterfaceBlock(block)

	if !result.Shutdown {
		t.Error("Expected shutdown")
	}
}

func TestParseOS10InterfaceBlock_Routed(t *testing.T) {
	block := []string{
		"interface vlan10",
		"   description Users gateway",
		"   ip address 10.0.10.1/24",
		"   no shutdown",
	}

	result := parseOS10InterfaceBlock(block)

	if result.IPAddress != "10.0.10.1" {
		t.Errorf("Expected IP 10.0.10.1, got '%s'", result.IPAddress)
	}
	if result.Netmask != "255.255.255.0" {
		t.Errorf("Expected netmask 255.255.255.0, got '%s'", result.Netmask)
	}
}

func TestExpandOS10VLANList(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []int
	}{
		{"Simple list", "10,20,30", []int{10, 20, 30}},
		{"With range", "10,20-22,30", []int{10, 20, 21, 22, 30}},
		{"Multiple ranges", "10-12,20-22", []int{10, 11, 12, 20, 21, 22}},
		{"Single VLAN", "100", []int{100}},
		{"Empty", "", nil},
		{"Spaces", "10, 20, 30", []int{10, 20, 30}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandOS10VLANList(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d VLANs, got %d: %v", len(tt.expected), len(result), result)
				return
			}
			for i, expected := range tt.expected {
				if result[i] != expected {
					t.Errorf("At index %d: expected %d, got %d", i, expected, result[i])
				}
			}
		})
	}
}

// ===========================
// MAC Address Table Tests
// ===========================

func TestParseOS10MACAddressTable(t *testing.T) {
	output := readTestFile(t, "sample_os10_mac_address_table.txt")
	entries := ParseOS10MACAddressTable(output, time.Now())

	if len(entries) == 0 {
		t.Fatal("Expected MAC entries, got none")
	}

	t.Logf("Parsed %d MAC address entries", len(entries))

	first := entries[0]
	if !strings.Contains(first.MAC, ":") {
		t.Errorf("MAC address not normalized: %s", first.MAC)
	}
	if first.VLANID == 0 {
		t.Error("VLAN ID should not be 0")
	}
	if first.Port == "" {
		t.Error("Port should not be empty")
	}
	if first.CollectionTS == "" {
		t.Error("CollectionTS should not be empty")
	}

	t.Logf("First entry: MAC=%s, VLAN=%d, Port=%s, Type=%s", first.MAC, first.VLANID, first.Port, first.Type)
}

// ===========================
// LLDP Neighbor Tests
// ===========================

func TestParseOS10LLDPNeighbors(t *testing.T) {
	output := readTestFile(t, "sample_os10_lldp_neighbors.txt")
	neighbors := ParseOS10LLDPNeighbors(output)

	if len(neighbors) == 0 {
		t.Fatal("Expected LLDP neighbors, got none")
	}

	t.Logf("Parsed %d LLDP neighbors", len(neighbors))

	// Find leaf02
	var foundLeaf02 bool
	for _, n := range neighbors {
		if n.SystemName == "leaf02.example.com" {
			foundLeaf02 = true
			if n.LocalInterface != "ethernet1/1/1" {
				t.Errorf("Expected local interface ethernet1/1/1, got '%s'", n.LocalInterface)
			}
			if n.ChassisID == "" {
				t.Error("ChassisID should not be empty")
			}
			if n.PortID == "" {
				t.Error("PortID should not be empty")
			}
			t.Logf("Found leaf02: local=%s, chassis=%s, port=%s", n.LocalInterface, n.ChassisID, n.PortID)
			break
		}
	}
	if !foundLeaf02 {
		t.Error("leaf02.example.com not found in LLDP neighbors")
	}

	// Verify spine01
	var foundSpine01 bool
	for _, n := range neighbors {
		if n.SystemName == "spine01.example.com" {
			foundSpine01 = true
			break
		}
	}
	if !foundSpine01 {
		t.Error("spine01.example.com not found in LLDP neighbors")
	}

	// Verify spine02
	var foundSpine02 bool
	for _, n := range neighbors {
		if n.SystemName == "spine02.example.com" {
			foundSpine02 = true
			break
		}
	}
	if !foundSpine02 {
		t.Error("spine02.example.com not found in LLDP neighbors")
	}
}

// ===========================
// ARP Table Tests
// ===========================

func TestParseOS10ARPTable(t *testing.T) {
	output := readTestFile(t, "sample_os10_ip_arp.txt")
	entries := ParseOS10ARPTable(output, time.Now())

	if len(entries) == 0 {
		t.Fatal("Expected ARP entries, got none")
	}

	t.Logf("Parsed %d ARP entries", len(entries))

	for _, entry := range entries {
		if entry.IP == "" {
			t.Error("ARP entry has no IP")
		}
		if entry.MAC == "" {
			t.Errorf("ARP entry %s has no MAC", entry.IP)
		}
		if !strings.Contains(entry.MAC, ":") {
			t.Errorf("MAC not normalized: %s", entry.MAC)
		}
		t.Logf("ARP: IP=%s, MAC=%s, Interface=%s", entry.IP, entry.MAC, entry.Interface)
	}
}

// ===========================
// Route Table Tests
// ===========================

func TestParseOS10RouteTable(t *testing.T) {
	output := readTestFile(t, "sample_os10_ip_route.txt")
	entries := ParseOS10RouteTable(output, time.Now())

	if len(entries) == 0 {
		t.Fatal("Expected route entries, got none")
	}

	t.Logf("Parsed %d route entries", len(entries))

	// Check default route
	var hasDefault bool
	for _, entry := range entries {
		if entry.Prefix == "0.0.0.0" && entry.PrefixLen == 0 {
			hasDefault = true
			if entry.Protocol != "static" {
				t.Errorf("Expected default route protocol 'static', got '%s'", entry.Protocol)
			}
			if entry.NextHop != "10.0.0.2" {
				t.Errorf("Expected next-hop 10.0.0.2, got '%s'", entry.NextHop)
			}
			if !entry.IsActive {
				t.Error("Default route should be active")
			}
			break
		}
	}
	if !hasDefault {
		t.Error("Default route not found")
	}

	// Check connected route
	var hasConnected bool
	for _, entry := range entries {
		if entry.Prefix == "10.0.10.0" && entry.PrefixLen == 24 {
			hasConnected = true
			if entry.Protocol != "connected" {
				t.Errorf("Expected 'connected', got '%s'", entry.Protocol)
			}
			t.Logf("Found connected route: %s/%d via %s", entry.Prefix, entry.PrefixLen, entry.Interface)
			break
		}
	}
	if !hasConnected {
		t.Error("Connected route 10.0.10.0/24 not found")
	}

	// Check OSPF route
	var hasOSPF bool
	for _, entry := range entries {
		if entry.Protocol == "ospf" {
			hasOSPF = true
			if entry.Distance != 110 {
				t.Errorf("Expected OSPF distance 110, got %d", entry.Distance)
			}
			t.Logf("Found OSPF route: %s/%d metric=%d distance=%d", entry.Prefix, entry.PrefixLen, entry.Metric, entry.Distance)
			break
		}
	}
	if !hasOSPF {
		t.Error("No OSPF routes found")
	}

	// Check BGP route
	var hasBGP bool
	for _, entry := range entries {
		if entry.Protocol == "bgp" {
			hasBGP = true
			t.Logf("Found BGP route: %s/%d distance=%d", entry.Prefix, entry.PrefixLen, entry.Distance)
			break
		}
	}
	if !hasBGP {
		t.Error("No BGP routes found")
	}
}

// ===========================
// Interface Status Tests
// ===========================

func TestParseOS10InterfaceStatus(t *testing.T) {
	output := readTestFile(t, "sample_os10_interfaces_status.txt")
	interfaces := ParseOS10InterfaceStatus(output)

	if len(interfaces) == 0 {
		t.Fatal("Expected interface entries, got none")
	}

	t.Logf("Parsed %d interfaces", len(interfaces))

	// Check for connected interfaces
	var foundConnected bool
	for _, iface := range interfaces {
		if iface.Status == "up" {
			foundConnected = true
			if iface.Name == "" {
				t.Error("Connected interface should have a name")
				continue
			}
			t.Logf("Connected: %s, VLAN=%s, Speed=%s, Duplex=%s", iface.Name, iface.VLAN, iface.Speed, iface.Duplex)
			break
		}
	}
	if !foundConnected {
		t.Error("No up interfaces found")
	}
}

// ===========================
// VLT Config Parsing Tests
// ===========================

func TestParseOS10VLTConfig(t *testing.T) {
	lines := []string{
		"vlt-domain 1",
		"   discovery-interface ethernet1/1/1",
		"   peer-link port-channel1",
		"   backup destination 192.168.1.102",
		"   vlt-mac 00:00:00:00:00:01",
		"   delay-restore 90",
		"   peer-routing",
	}

	vlt := parseOS10VLTConfig(lines)

	if vlt == nil {
		t.Fatal("Expected VLT config, got nil")
	}

	if vlt.DomainID != "1" {
		t.Errorf("Expected domain-id '1', got '%s'", vlt.DomainID)
	}
	if vlt.DiscoveryInterface != "ethernet1/1/1" {
		t.Errorf("Expected discovery-interface 'ethernet1/1/1', got '%s'", vlt.DiscoveryInterface)
	}
	if vlt.PeerLink != "port-channel1" {
		t.Errorf("Expected peer-link 'port-channel1', got '%s'", vlt.PeerLink)
	}
	if vlt.BackupDestination != "192.168.1.102" {
		t.Errorf("Expected backup destination '192.168.1.102', got '%s'", vlt.BackupDestination)
	}
	if vlt.VLTMACAddress != "00:00:00:00:00:01" {
		t.Errorf("Expected VLT MAC '00:00:00:00:00:01', got '%s'", vlt.VLTMACAddress)
	}
	if vlt.DelayRestore != 90 {
		t.Errorf("Expected delay-restore 90, got %d", vlt.DelayRestore)
	}
	if !vlt.PeerRouting {
		t.Error("VLT peer-routing should be enabled")
	}
	if !vlt.Enabled {
		t.Error("VLT should be enabled")
	}
}

// ===========================
// Static Route Parsing Tests
// ===========================

func TestParseOS10StaticRoute(t *testing.T) {
	tests := []struct {
		name               string
		input              string
		expectedPrefix     string
		expectedPrefixLen  int
		expectedNextHop    string
		expectedDistance   int
	}{
		{
			name:              "Default route",
			input:             "ip route 0.0.0.0/0 10.0.0.2",
			expectedPrefix:    "0.0.0.0",
			expectedPrefixLen: 0,
			expectedNextHop:   "10.0.0.2",
		},
		{
			name:              "With distance",
			input:             "ip route 10.0.0.0/8 192.168.1.254 1",
			expectedPrefix:    "10.0.0.0",
			expectedPrefixLen: 8,
			expectedNextHop:   "192.168.1.254",
			expectedDistance:  1,
		},
		{
			name:              "With name and tag",
			input:             "ip route 172.16.0.0/16 10.255.255.2 tag 100",
			expectedPrefix:    "172.16.0.0",
			expectedPrefixLen: 16,
			expectedNextHop:   "10.255.255.2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route := parseOS10StaticRoute(tt.input)

			if route.Prefix != tt.expectedPrefix {
				t.Errorf("Expected prefix '%s', got '%s'", tt.expectedPrefix, route.Prefix)
			}
			if route.PrefixLen != tt.expectedPrefixLen {
				t.Errorf("Expected prefixLen %d, got %d", tt.expectedPrefixLen, route.PrefixLen)
			}
			if route.NextHop != tt.expectedNextHop {
				t.Errorf("Expected nextHop '%s', got '%s'", tt.expectedNextHop, route.NextHop)
			}
			if route.Distance != tt.expectedDistance {
				t.Errorf("Expected distance %d, got %d", tt.expectedDistance, route.Distance)
			}
		})
	}
}

// ===========================
// FTOS Backward Compatibility Tests
// ===========================

func TestNormalizeFTOSInterface(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"TenGigabitEthernet", "TenGigabitEthernet 0/1", "ethernet0/1/1"},
		{"GigabitEthernet", "GigabitEthernet 0/5", "ethernet0/1/5"},
		{"fortyGigE", "fortyGigE 0/49", "ethernet0/1/49"},
		{"Port-channel (unchanged)", "Port-channel 1", "port-channel1"},
		{"VLAN (unchanged)", "Vlan 10", "vlan10"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeFTOSInterface(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeFTOSInterface(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestDetectOSVersion(t *testing.T) {
	tests := []struct {
		name     string
		config   string
		expected string
	}{
		{"OS10 with VLT", "vlt-domain 1\n  peer-link port-channel1", "OS10"},
		{"FTOS legacy", "interface TenGigabitEthernet 0/1", "FTOS"},
		{"OS10 ethernet", "interface ethernet1/1/1\n  switchport mode trunk", "OS10"},
		{"FTOS default", "interface gigabitethernet 0/1", "FTOS"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectOSVersion(tt.config)
			if result != tt.expected {
				t.Errorf("DetectOSVersion() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// ===========================
// Config Processor Integration Test
// ===========================

func TestDellOS10Processor_ParseOS10Config(t *testing.T) {
	// Create a temp config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "os10_config.txt")

	// Copy sample config content
	configContent := readTestFile(t, "sample_os10_switch_config.txt")
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write temp config: %v", err)
	}

	processor := NewDellOS10Processor(configPath)
	err = processor.ProcessOS10File()
	if err != nil {
		t.Fatalf("ProcessOS10File failed: %v", err)
	}

	// Verify basic structure
	if processor.os10Config.Hostname != "leaf01" {
		t.Errorf("Expected hostname 'leaf01', got '%s'", processor.os10Config.Hostname)
	}

	if len(processor.os10Config.Interfaces) == 0 {
		t.Error("Expected interfaces, got none")
	}

	t.Logf("Parsed config: hostname=%s, interfaces=%d, VLANs=%d, port-channels=%d, static-routes=%d",
		processor.os10Config.Hostname,
		len(processor.os10Config.Interfaces),
		len(processor.os10Config.VLANs),
		len(processor.os10Config.PortChannels),
		len(processor.os10Config.StaticRoutes))

	// Verify VLT config
	if processor.os10Config.VLTConfig == nil {
		t.Error("Expected VLT config, got nil")
	} else {
		t.Logf("VLT domain=%s, peer-link=%s, backup=%s",
			processor.os10Config.VLTConfig.DomainID,
			processor.os10Config.VLTConfig.PeerLink,
			processor.os10Config.VLTConfig.BackupDestination)
	}

	// Verify OSPF router
	if processor.os10Config.OSPFRouter == nil {
		t.Error("Expected OSPF router, got nil")
	} else {
		t.Logf("OSPF process=%d, router-id=%s, networks=%d",
			processor.os10Config.OSPFRouter.ProcessID,
			processor.os10Config.OSPFRouter.RouterID,
			len(processor.os10Config.OSPFRouter.Networks))
	}

	// Verify BGP
	if processor.os10Config.BGPConfig == nil {
		t.Error("Expected BGP config, got nil")
	} else {
		t.Logf("BGP ASN=%d, router-id=%s, peers=%d, networks=%d",
			processor.os10Config.BGPConfig.ASN,
			processor.os10Config.BGPConfig.RouterID,
			len(processor.os10Config.BGPConfig.Peers),
			len(processor.os10Config.BGPConfig.Networks))
	}

	// Verify ACL
	if len(processor.os10Config.ExtendedACLs) == 0 {
		t.Error("Expected extended ACLs, got none")
	} else {
		t.Logf("Extended ACLs: %d", len(processor.os10Config.ExtendedACLs))
		acl := processor.os10Config.ExtendedACLs[0]
		if acl.Name != "ACL-INBOUND" {
			t.Errorf("Expected ACL name 'ACL-INBOUND', got '%s'", acl.Name)
		}
	}

	// Verify static routes
	if len(processor.os10Config.StaticRoutes) != 3 {
		t.Errorf("Expected 3 static routes, got %d", len(processor.os10Config.StaticRoutes))
	}

	// Check interface properties
	var foundEthernet1_5 bool
	for _, iface := range processor.os10Config.Interfaces {
		if strings.ToLower(iface.Name) == "ethernet1/1/5" {
			foundEthernet1_5 = true
			if iface.LAGGroup != 10 {
				t.Errorf("Expected ethernet1/1/5 LAG group 10, got %d", iface.LAGGroup)
			}
			if iface.SwitchportMode != "trunk" {
				t.Errorf("Expected ethernet1/1/5 mode 'trunk', got '%s'", iface.SwitchportMode)
			}
			break
		}
	}
	if !foundEthernet1_5 {
		t.Error("ethernet1/1/5 not found in interfaces")
	}

	// Verify topology model build
	if processor.TopologyDevice == nil {
		t.Error("Expected topology device, got nil")
	} else {
		t.Logf("Topology device: hostname=%s, vendor=%s, interfaces=%d",
			processor.TopologyDevice.Hostname,
			processor.TopologyDevice.Vendor,
			len(processor.TopologyDevice.Interfaces))
	}
}

// ===========================
// VLT Topology Inference Test
// ===========================

func TestInferVLTLinksFromConfig(t *testing.T) {
	// Create processor with sample config
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "os10_config.txt")
	configContent := readTestFile(t, "sample_os10_switch_config.txt")
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write temp config: %v", err)
	}

	processor := NewDellOS10Processor(configPath)
	err = processor.ProcessOS10File()
	if err != nil {
		t.Fatalf("ProcessOS10File failed: %v", err)
	}

	vltConfig := processor.os10Config.VLTConfig
	if vltConfig == nil {
		t.Fatal("Expected VLT config, got nil")
	}

	links := InferVLTLinksFromConfig("leaf01", vltConfig, processor.os10Config.Interfaces)

	if len(links) == 0 {
		t.Fatal("Expected VLT topology links, got none")
	}

	t.Logf("Inferred %d VLT topology links", len(links))

	// Verify peer-link is present
	var hasPeerLink, hasBackupLink, hasMemberLink bool
	for _, link := range links {
		t.Logf("  Link: %s -> %s type=%s", link.FromInterface, link.ToInterface, link.LinkType)
		switch link.LinkType {
		case "peer-link":
			hasPeerLink = true
		case "backup-link":
			hasBackupLink = true
		case "vlt-member":
			hasMemberLink = true
		}
	}
	if !hasPeerLink {
		t.Error("Missing peer-link in VLT topology")
	}
	if !hasBackupLink {
		t.Error("Missing backup-link in VLT topology")
	}
	if !hasMemberLink {
		t.Error("Missing vlt-member links in VLT topology")
	}
}

// ===========================
// Mixed-Vendor Topology Test
// ===========================

func TestMixedVendorTopology(t *testing.T) {
	// Verify that OS10 topology model follows the same ParsedNetworkTopology
	// structure as other vendors (Arista, Cisco, etc.) for interoperability
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "os10_config.txt")
	configContent := readTestFile(t, "sample_os10_switch_config.txt")
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write temp config: %v", err)
	}

	processor := NewDellOS10Processor(configPath)
	err = processor.ProcessOS10File()
	if err != nil {
		t.Fatalf("ProcessOS10File failed: %v", err)
	}

	topology := OS10Mapper(processor)

	// Verify ParsedNetworkTopology is populated
	if topology.Device.Hostname != "leaf01" {
		t.Errorf("Expected device hostname 'leaf01', got '%s'", topology.Device.Hostname)
	}
	if topology.Device.Vendor != "dell" {
		t.Errorf("Expected vendor 'dell', got '%s'", topology.Device.Vendor)
	}
	if len(topology.Interfaces) == 0 {
		t.Error("Expected topology interfaces, got none")
	}
	if len(topology.RoutingEntries) == 0 {
		t.Error("Expected topology routing entries, got none")
	}
	if len(topology.Addresses) == 0 {
		t.Error("Expected topology addresses, got none")
	}
	if len(topology.Zones) == 0 {
		t.Error("Expected topology zones, got none")
	}

	t.Logf("Mixed-vendor topology: device=%s, interfaces=%d, addresses=%d, routes=%d, zones=%d",
		topology.Device.Hostname,
		len(topology.Interfaces),
		len(topology.Addresses),
		len(topology.RoutingEntries),
		len(topology.Zones))

	// Verify interoperability: topology should have same schema as Arista
	// (ParsedNetworkTopology is the standard container)
	if topology.Device.Vendor != "dell" {
		t.Error("Vendor should be dell for OS10 topology")
	}
}

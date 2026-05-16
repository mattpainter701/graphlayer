package eos

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

func TestParseEOSInterfaceBlock(t *testing.T) {
	block := []string{
		"interface Ethernet1",
		"   description PoC-to-dist01",
		"   switchport mode trunk",
		"   switchport trunk allowed vlan 10,20,30,100",
		"   switchport trunk native vlan 200",
		"   channel-group 1 mode active",
		"   no shutdown",
	}

	result := parseEOSInterfaceBlock(block)

	if result.Description != "PoC-to-dist01" {
		t.Errorf("Expected description 'PoC-to-dist01', got '%s'", result.Description)
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
	if result.ChannelGroup != 1 {
		t.Errorf("Expected channel-group 1, got %d", result.ChannelGroup)
	}
	if result.Shutdown {
		t.Error("Expected no shutdown (false)")
	}
}

func TestParseEOSInterfaceBlock_Access(t *testing.T) {
	block := []string{
		"interface Ethernet3",
		"   description server-rack1-port01",
		"   switchport mode access",
		"   switchport access vlan 10",
		"   spanning-tree portfast",
		"   no shutdown",
	}

	result := parseEOSInterfaceBlock(block)

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

func TestParseEOSInterfaceBlock_Shutdown(t *testing.T) {
	block := []string{
		"interface Ethernet7",
		"   description unused-port",
		"   shutdown",
	}

	result := parseEOSInterfaceBlock(block)

	if !result.Shutdown {
		t.Error("Expected shutdown")
	}
}

func TestParseEOSInterfaceBlock_Routed(t *testing.T) {
	block := []string{
		"interface Vlan10",
		"   description Users gateway",
		"   ip address 10.0.10.1/24",
		"   no shutdown",
	}

	result := parseEOSInterfaceBlock(block)

	if result.IPAddress != "10.0.10.1" {
		t.Errorf("Expected IP 10.0.10.1, got '%s'", result.IPAddress)
	}
}

func TestExpandEOSVLANList(t *testing.T) {
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
			result := expandEOSVLANList(tt.input)
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

func TestParseEOSMACAddressTable(t *testing.T) {
	output := readTestFile(t, "sample_mac_address_table.txt")
	entries := ParseEOSMACAddressTable(output, time.Now())

	if len(entries) == 0 {
		t.Fatal("Expected MAC entries, got none")
	}

	t.Logf("Parsed %d MAC address entries", len(entries))

	// Validate first entry
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

func TestParseEOSLLDPNeighbors(t *testing.T) {
	output := readTestFile(t, "sample_lldp_neighbors.txt")
	neighbors := ParseEOSLLDPNeighbors(output)

	if len(neighbors) == 0 {
		t.Fatal("Expected LLDP neighbors, got none")
	}

	t.Logf("Parsed %d LLDP neighbors", len(neighbors))

	// Find dist01
	var foundDist01 bool
	for _, n := range neighbors {
		if n.SystemName == "dist01.example.com" {
			foundDist01 = true
			if n.LocalInterface != "Ethernet1" {
				t.Errorf("Expected local interface Ethernet1, got '%s'", n.LocalInterface)
			}
			if n.ChassisID == "" {
				t.Error("ChassisID should not be empty")
			}
			if n.PortID == "" {
				t.Error("PortID should not be empty")
			}
			t.Logf("Found dist01: local=%s, chassis=%s, port=%s", n.LocalInterface, n.ChassisID, n.PortID)
			break
		}
	}
	if !foundDist01 {
		t.Error("dist01.example.com not found in LLDP neighbors")
	}

	// Check peer-link entry
	var foundLeaf02 bool
	for _, n := range neighbors {
		if n.SystemName == "leaf02.example.com" {
			foundLeaf02 = true
			if n.LocalInterface != "Ethernet48" && n.LocalInterface != "Ethernet49" {
				t.Errorf("Unexpected local interface for leaf02: %s", n.LocalInterface)
			}
			break
		}
	}
	if !foundLeaf02 {
		t.Error("leaf02.example.com not found in LLDP neighbors")
	}
}

// ===========================
// ARP Table Tests
// ===========================

func TestParseEOSARPTable(t *testing.T) {
	output := readTestFile(t, "sample_ip_arp.txt")
	entries := ParseEOSARPTable(output, time.Now())

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
		t.Logf("ARP: IP=%s, MAC=%s, Interface=%s, Age=%ds", entry.IP, entry.MAC, entry.Interface, entry.AgingSeconds)
	}
}

// ===========================
// Route Table Tests
// ===========================

func TestParseEOSRouteTable(t *testing.T) {
	output := readTestFile(t, "sample_ip_route.txt")
	entries := ParseEOSRouteTable(output, time.Now())

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
			if entry.NextHop != "192.168.1.1" {
				t.Errorf("Expected next-hop 192.168.1.1, got '%s'", entry.NextHop)
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

	// Check static route
	var hasStatic bool
	for _, entry := range entries {
		if entry.Prefix == "10.0.0.0" && entry.PrefixLen == 8 {
			hasStatic = true
			if entry.Distance != 1 {
				t.Errorf("Expected admin distance 1, got %d", entry.Distance)
			}
			t.Logf("Found static route: %s/%d via %s (distance=%d)", entry.Prefix, entry.PrefixLen, entry.NextHop, entry.Distance)
			break
		}
	}
	if !hasStatic {
		t.Error("Static route 10.0.0.0/8 not found")
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

func TestParseEOSInterfaceStatus(t *testing.T) {
	output := readTestFile(t, "sample_interfaces_status.txt")
	interfaces := ParseEOSInterfaceStatus(output)

	if len(interfaces) == 0 {
		t.Fatal("Expected interface entries, got none")
	}

	t.Logf("Parsed %d interfaces", len(interfaces))

	// Check for connected interfaces
	var foundConnected bool
	for _, iface := range interfaces {
		if iface.Status == "connected" {
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
		t.Error("No connected interfaces found")
	}
}

// ===========================
// MLAG Config Parsing Tests
// ===========================

func TestParseEOSMLAGConfig(t *testing.T) {
	lines := []string{
		"mlag configuration",
		"   domain-id MLAG-DOMAIN-01",
		"   local-interface Vlan100",
		"   peer-address 10.255.255.2",
		"   peer-link Port-Channel1000",
		"   heartbeat-interval 5000",
		"   reload-delay 300",
		"   no shutdown",
	}

	mlag := parseEOSMLAGConfig(lines)

	if mlag == nil {
		t.Fatal("Expected MLAG config, got nil")
	}

	if mlag.DomainID != "MLAG-DOMAIN-01" {
		t.Errorf("Expected domain-id 'MLAG-DOMAIN-01', got '%s'", mlag.DomainID)
	}
	if mlag.LocalInterface != "Vlan100" {
		t.Errorf("Expected local-interface 'Vlan100', got '%s'", mlag.LocalInterface)
	}
	if mlag.PeerAddress != "10.255.255.2" {
		t.Errorf("Expected peer-address '10.255.255.2', got '%s'", mlag.PeerAddress)
	}
	if mlag.PeerLink != "Port-Channel1000" {
		t.Errorf("Expected peer-link 'Port-Channel1000', got '%s'", mlag.PeerLink)
	}
	if mlag.ReloadDelay != 300 {
		t.Errorf("Expected reload-delay 300, got %d", mlag.ReloadDelay)
	}
	if mlag.HeartbeatTimeout != 5000 {
		t.Errorf("Expected heartbeat 5000, got %d", mlag.HeartbeatTimeout)
	}
	if !mlag.Enabled {
		t.Error("MLAG should be enabled (no shutdown)")
	}
}

// ===========================
// VXLAN Config Parsing Tests
// ===========================

func TestParseEOSVXLANConfig(t *testing.T) {
	lines := []string{
		"interface Vxlan1",
		"   vxlan source-interface Loopback0",
		"   vxlan vlan 10 vni 10010",
		"   vxlan vlan 20 vni 10020",
		"   vxlan vlan 30 vni 10030",
		"   vxlan flood vtep 10.0.0.1 10.0.0.2",
		"   vxlan learn from arp",
	}

	vxlan := parseEOSVXLANConfig(lines)

	if vxlan == nil {
		t.Fatal("Expected VXLAN config, got nil")
	}

	if vxlan.SourceInterface != "Loopback0" {
		t.Errorf("Expected source-interface 'Loopback0', got '%s'", vxlan.SourceInterface)
	}
	if len(vxlan.VNItoVLAN) != 3 {
		t.Errorf("Expected 3 VNI mappings, got %d", len(vxlan.VNItoVLAN))
	}
	if vxlan.VNItoVLAN[10010] != 10 {
		t.Errorf("Expected VNI 10010 -> VLAN 10, got VLAN %d", vxlan.VNItoVLAN[10010])
	}
	if !vxlan.Enabled {
		t.Error("VXLAN should be enabled")
	}
	if len(vxlan.FloodList) != 2 {
		t.Errorf("Expected 2 flood VTEPs, got %d", len(vxlan.FloodList))
	}
}

// ===========================
// Static Route Parsing Tests
// ===========================

func TestParseEOSStaticRoute(t *testing.T) {
	tests := []struct {
		name             string
		input            string
		expectedPrefix   string
		expectedPrefixLen int
		expectedNextHop  string
		expectedDistance int
	}{
		{
			name:             "Default route",
			input:            "ip route 0.0.0.0/0 192.168.1.1",
			expectedPrefix:   "0.0.0.0",
			expectedPrefixLen: 0,
			expectedNextHop:  "192.168.1.1",
		},
		{
			name:             "With distance",
			input:            "ip route 10.0.0.0/8 192.168.1.254 1",
			expectedPrefix:   "10.0.0.0",
			expectedPrefixLen: 8,
			expectedNextHop:  "192.168.1.254",
			expectedDistance: 1,
		},
		{
			name:             "With name and tag",
			input:            "ip route 172.16.0.0/16 10.255.255.2 tag 100",
			expectedPrefix:   "172.16.0.0",
			expectedPrefixLen: 16,
			expectedNextHop:  "10.255.255.2",
			expectedDistance: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route := parseEOSStaticRoute(tt.input)

			if route.Prefix != tt.expectedPrefix {
				t.Errorf("Expected prefix '%s', got '%s'", tt.expectedPrefix, route.Prefix)
			}
			if route.PrefixLen != tt.expectedPrefixLen {
				t.Errorf("Expected prefixLen %d, got %d", tt.expectedPrefixLen, route.PrefixLen)
			}
			if route.NextHop != tt.expectedNextHop {
				t.Errorf("Expected next-hop '%s', got '%s'", tt.expectedNextHop, route.NextHop)
			}
			if tt.expectedDistance > 0 && route.Distance != tt.expectedDistance {
				t.Errorf("Expected distance %d, got %d", tt.expectedDistance, route.Distance)
			}
		})
	}
}

// ===========================
// MAC Normalization Tests
// ===========================

func TestNormalizeEOSMAC(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "EOS/ Cisco format",
			input:    "0004.4d41.4e31",
			expected: "00:04:4D:41:4E:31",
		},
		{
			name:     "Already colon format",
			input:    "00:04:4d:41:4e:31",
			expected: "00:04:4D:41:4E:31",
		},
		{
			name:     "Short",
			input:    "aabb.ccdd.eeff",
			expected: "AA:BB:CC:DD:EE:FF",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeEOSMAC(tt.input)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

// ===========================
// Port Security Tests
// ===========================

func TestParseEOSPortSecurity(t *testing.T) {
	output := `Port         Status     Max  Current  Violations  Action  Last Source
Et3          secure-up  1    1        0          protect 00:50:56:ab:cd:01
Et4          secure-up  1    1        0          protect 00:50:56:ab:cd:02
Et5          secure-up  2    1        0          shutdown 00:50:56:ab:cd:03
Et6          secure-up  1    1        0          shutdown 00:50:56:ab:cd:04`

	entries := ParseEOSPortSecurity(output, time.Now())

	if len(entries) == 0 {
		t.Fatal("Expected port-security entries, got none")
	}

	t.Logf("Parsed %d port-security entries", len(entries))
	if len(entries) != 4 {
		t.Errorf("Expected 4 entries, got %d", len(entries))
	}

	// Check Et3
	found := false
	for _, e := range entries {
		if e.Interface == "Et3" {
			found = true
			if e.Status != "secure-up" {
				t.Errorf("Expected status 'secure-up', got '%s'", e.Status)
			}
			if e.MaxMACCount != 1 {
				t.Errorf("Expected max 1, got %d", e.MaxMACCount)
			}
			break
		}
	}
	if !found {
		t.Error("Et3 not found in port-security entries")
	}
}

// ===========================
// OSPF Config Parsing Tests
// ===========================

func TestParseEOSOSPFRouter(t *testing.T) {
	header := "router ospf 100"
	lines := []string{
		"router ospf 100",
		"   router-id 10.0.0.101",
		"   network 10.0.0.0 0.0.0.255 area 0.0.0.0",
		"   network 10.255.255.0 0.0.0.3 area 0.0.0.0",
		"   network 192.168.1.0 0.0.0.255 area 0.0.0.10",
		"   passive-interface default",
		"   no passive-interface Vlan100",
	}

	ospf := parseEOSOSPFRouter(header, lines)

	if ospf == nil {
		t.Fatal("Expected OSPF config, got nil")
	}

	if ospf.ProcessID != 100 {
		t.Errorf("Expected process-id 100, got %d", ospf.ProcessID)
	}
	if ospf.RouterID != "10.0.0.101" {
		t.Errorf("Expected router-id '10.0.0.101', got '%s'", ospf.RouterID)
	}
	if len(ospf.Networks) != 3 {
		t.Errorf("Expected 3 networks, got %d", len(ospf.Networks))
	}
	if len(ospf.Areas) != 2 {
		t.Errorf("Expected 2 areas, got %d", len(ospf.Areas))
	}
}

// ===========================
// BGP Config Parsing Tests
// ===========================

func TestParseEOSBGPConfig(t *testing.T) {
	header := "router bgp 65001"
	lines := []string{
		"router bgp 65001",
		"   router-id 10.0.0.101",
		"   maximum-paths 4",
		"   neighbor 10.0.0.1 remote-as 65000",
		"   neighbor 10.0.0.1 description spine-01",
		"   neighbor 10.0.0.2 remote-as 65000",
		"   neighbor 10.0.0.2 description spine-02",
		"   network 10.0.10.0/24",
		"   network 10.0.20.0/24",
		"   vrf TENANT-A",
		"      rd 65001:100",
		"      neighbor 10.0.10.1 remote-as 65001",
		"      neighbor 10.0.10.1 update-source Vlan10",
	}

	bgp := parseEOSBGPConfig(header, lines)

	if bgp == nil {
		t.Fatal("Expected BGP config, got nil")
	}

	if bgp.ASN != 65001 {
		t.Errorf("Expected ASN 65001, got %d", bgp.ASN)
	}
	if bgp.RouterID != "10.0.0.101" {
		t.Errorf("Expected router-id '10.0.0.101', got '%s'", bgp.RouterID)
	}
	if len(bgp.Peers) != 2 {
		t.Errorf("Expected 2 peers, got %d", len(bgp.Peers))
	}
	if len(bgp.Networks) != 2 {
		t.Errorf("Expected 2 networks, got %d", len(bgp.Networks))
	}
	if bgp.MaximumPaths != 4 {
		t.Errorf("Expected maximum-paths 4, got %d", bgp.MaximumPaths)
	}

	// Check VRF
	if len(bgp.VRFs) != 1 {
		t.Errorf("Expected 1 VRF, got %d", len(bgp.VRFs))
	} else {
		vrf := bgp.VRFs[0]
		if vrf.Name != "TENANT-A" {
			t.Errorf("Expected VRF 'TENANT-A', got '%s'", vrf.Name)
		}
		if vrf.RD != "65001:100" {
			t.Errorf("Expected RD '65001:100', got '%s'", vrf.RD)
		}
		if len(vrf.Peers) != 1 {
			t.Errorf("Expected 1 VRF peer, got %d", len(vrf.Peers))
		}
	}
}

// ===========================
// Processor Integration Tests
// ===========================

func TestParseEOSConfigSample(t *testing.T) {
	cfg, err := readAndParseEOSConfig("sample_eos_switch_config.txt")
	if err != nil {
		t.Fatalf("Failed to parse EOS config: %v", err)
	}

	if cfg.Hostname != "leaf01" {
		t.Errorf("Expected hostname 'leaf01', got '%s'", cfg.Hostname)
	}
	if cfg.Version == "" {
		t.Error("Version should not be empty")
	}

	t.Logf("Parsed config: hostname=%s, version=%s", cfg.Hostname, cfg.Version)
	t.Logf("  Interfaces: %d", len(cfg.Interfaces))
	t.Logf("  VLANs: %d", len(cfg.VLANs))
	t.Logf("  PortChannels: %d", len(cfg.PortChannels))
	t.Logf("  StaticRoutes: %d", len(cfg.StaticRoutes))
	t.Logf("  MLAG: %v", cfg.MLAGConfig != nil)
	t.Logf("  VXLAN: %v", cfg.VXLANConfig != nil)
	t.Logf("  OSPF: %v", cfg.OSPFRouter != nil)
	t.Logf("  BGP: %v", cfg.BGPConfig != nil)
	t.Logf("  ACLs: %d", len(cfg.ExtendedACLs))
	t.Logf("  VRFs: %d", len(cfg.VRFConfigs))

	// Validate interfaces
	if len(cfg.Interfaces) == 0 {
		t.Error("Expected at least one interface")
	}

	// Check for MLAG interface
	foundMLAGPeerLink := false
	for _, po := range cfg.PortChannels {
		if po.ID == 1000 {
			foundMLAGPeerLink = true
			break
		}
	}
	if !foundMLAGPeerLink {
		t.Error("Port-Channel1000 not found (MLAG peer-link)")
	}

	// Check for LAG
	foundLAG := false
	for _, iface := range cfg.Interfaces {
		if iface.LAGGroup == 1 {
			foundLAG = true
			break
		}
	}
	if !foundLAG {
		t.Error("No interface with channel-group 1 (LAG)")
	}

	// Check MLAG config
	if cfg.MLAGConfig == nil {
		t.Error("Expected MLAG configuration")
	} else {
		if cfg.MLAGConfig.DomainID != "MLAG-DOMAIN-01" {
			t.Errorf("Expected MLAG domain 'MLAG-DOMAIN-01', got '%s'", cfg.MLAGConfig.DomainID)
		}
	}

	// Check VXLAN config
	if cfg.VXLANConfig == nil {
		t.Error("Expected VXLAN configuration")
	} else {
		if cfg.VXLANConfig.SourceInterface != "Loopback0" {
			t.Errorf("Expected source-interface 'Loopback0', got '%s'", cfg.VXLANConfig.SourceInterface)
		}
	}
}

func TestParseEOSConfigRoutedInterfaceCount(t *testing.T) {
	cfg, err := readAndParseEOSConfig("sample_eos_switch_config.txt")
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	// Count interface types
	phyCount := 0
	sviCount := 0
	loopCount := 0

	for _, iface := range cfg.Interfaces {
		lower := strings.ToLower(iface.Name)
		if strings.HasPrefix(lower, "ethernet") || strings.HasPrefix(lower, "e") {
			phyCount++
		} else if strings.HasPrefix(lower, "vlan") {
			sviCount++
		} else if strings.HasPrefix(lower, "loopback") {
			loopCount++
		}
	}

	t.Logf("Interface counts: physical=%d, SVI=%d, loopback=%d", phyCount, sviCount, loopCount)

	if phyCount == 0 {
		t.Error("Expected physical interfaces")
	}
	// loopback may not be present if not in config — that's fine
	// SVI should be present
	if sviCount == 0 {
		t.Error("Expected SVI interfaces (VlanX)")
	}
}

func TestBuildEOSL2Config(t *testing.T) {
	processor, err := newProcessorForTest("sample_eos_switch_config.txt")
	if err != nil {
		t.Fatalf("Failed to create processor: %v", err)
	}

	l2Config := processor.BuildEOSL2Config()

	if l2Config.Hostname != "leaf01" {
		t.Errorf("Expected hostname 'leaf01', got '%s'", l2Config.Hostname)
	}

	t.Logf("L2 config: %d VLANs, %d interfaces, %d port-channels, %d SVIs",
		len(l2Config.VLANs), len(l2Config.Interfaces), len(l2Config.PortChannels), len(l2Config.SVIs))

	if len(l2Config.VLANs) == 0 {
		t.Error("Expected VLANs in L2 config")
	}

	if l2Config.MLAGConfig == nil {
		t.Error("Expected MLAG config in L2 config")
	}

	// Check VXLAN
	if processor.eosConfig.VXLANConfig != nil {
		vniCount := len(processor.eosConfig.VXLANConfig.VNItoVLAN)
		t.Logf("VXLAN: %d VNI mappings", vniCount)
		if vniCount < 1 {
			t.Error("Expected VNI-to-VLAN mappings")
		}
	}
}

func TestInferMLAGTopology(t *testing.T) {
	processor, err := newProcessorForTest("sample_eos_switch_config.txt")
	if err != nil {
		t.Fatalf("Failed to create processor: %v", err)
	}

	if processor.eosConfig.MLAGConfig == nil {
		t.Skip("No MLAG config to test")
	}

	links := InferMLAGLinksFromConfig(processor.eosConfig.Hostname, processor.eosConfig.MLAGConfig, processor.eosConfig.Interfaces)

	if len(links) == 0 {
		t.Error("Expected MLAG inference to produce links")
	}

	t.Logf("Inferred %d MLAG topology links", len(links))
	for _, link := range links {
		t.Logf("  Link: %s -- %s (type=%s)", link.FromInterface, link.ToInterface, link.LinkType)
	}

	// Should find peer-link
	foundPeerLink := false
	for _, link := range links {
		if link.LinkType == "peer-link" {
			foundPeerLink = true
			if link.FromInterface == "" {
				t.Error("Peer-link from-interface should not be empty")
			}
			break
		}
	}
	if !foundPeerLink {
		t.Error("Expected MLAG peer-link")
	}
}

// ===========================
// Test Helpers
// ===========================

func readAndParseEOSConfig(filename string) (*models.EOSConfig, error) {
	path := filepath.Join(testDataDir, filename)
	processor := NewAristaEOSProcessor(path)
	return processor.ParseEOSConfig()
}

func newProcessorForTest(filename string) (*AristaEOSProcessor, error) {
	path := filepath.Join(testDataDir, filename)
	processor := NewAristaEOSProcessor(path)
	err := processor.ProcessEOSFile()
	if err != nil {
		return nil, err
	}
	return processor, nil
}

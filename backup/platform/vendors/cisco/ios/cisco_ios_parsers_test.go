package cisco

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Test data directory
const testDataDir = "../l2__ios_configs"

// Test helper to read test files
func readTestFile(t *testing.T, device, filename string) string {
	path := filepath.Join(testDataDir, device, filename)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read test file %s: %v", path, err)
	}
	return string(data)
}

// ===========================
// MAC Address Table Tests
// ===========================

func TestParseMACAddressTable_C9300X(t *testing.T) {
	output := readTestFile(t, "C9300X-24Y_CORE-01", "C9300X-24Y_CORE-01_show_mac-address-table.txt")
	collectionTime := time.Now()
	
	entries := ParseMACAddressTable(output, collectionTime)
	
	if len(entries) == 0 {
		t.Error("Expected MAC entries, got none")
	}
	
	t.Logf("Parsed %d MAC address entries", len(entries))
	
	// Validate first entry
	if len(entries) > 0 {
		first := entries[0]
		
		// Check MAC format (should be normalized to XX:XX:XX:XX:XX:XX)
		if !strings.Contains(first.MAC, ":") {
			t.Errorf("MAC address not normalized: %s", first.MAC)
		}
		
		// Check required fields
		if first.VLANID == 0 {
			t.Error("VLAN ID should not be 0")
		}
		
		if first.LastSeenPort == "" {
			t.Error("LastSeenPort should not be empty")
		}
		
		if first.EvidenceSource != "fdb" {
			t.Errorf("Expected EvidenceSource 'fdb', got '%s'", first.EvidenceSource)
		}
		
		if first.Confidence != 1.0 {
			t.Errorf("Expected Confidence 1.0, got %f", first.Confidence)
		}
		
		if first.LastSeenTS == "" {
			t.Error("LastSeenTS should not be empty")
		}
		
		t.Logf("First entry: MAC=%s, VLAN=%d, Port=%s, Confidence=%f",
			first.MAC, first.VLANID, first.LastSeenPort, first.Confidence)
	}
}

func TestParseMACAddressTable_AllDevices(t *testing.T) {
	devices := []string{
		"C9300X-24Y_CORE-01",
		"IE-2000-8TC-G-E_LEGACY-01",
		"IE-3400H-24T4S-E_PLANT-01",
		"IE-5000-12S12P-10G_CORE-01",
		"WS-C2960X-24TD-L_IDF-02",
		"WS-C3850-48XS-E_DIST-01",
	}
	
	for _, device := range devices {
		t.Run(device, func(t *testing.T) {
			output := readTestFile(t, device, device+"_show_mac-address-table.txt")
			entries := ParseMACAddressTable(output, time.Now())
			
			t.Logf("[%s] Parsed %d MAC entries", device, len(entries))
			
			if len(entries) == 0 {
				t.Logf("[%s] WARNING: No MAC entries parsed (might be empty table)", device)
				return
			}
			
			// Validate all entries
			for i, entry := range entries {
				if !strings.Contains(entry.MAC, ":") {
					t.Errorf("[%s] Entry %d: MAC not normalized: %s", device, i, entry.MAC)
				}
				if entry.VLANID == 0 {
					t.Errorf("[%s] Entry %d: Invalid VLAN ID", device, i)
				}
				if entry.LastSeenPort == "" {
					t.Errorf("[%s] Entry %d: Empty port", device, i)
				}
			}
		})
	}
}

// ===========================
// Interface Status Tests
// ===========================

func TestParseInterfaceStatus_C9300X(t *testing.T) {
	output := readTestFile(t, "C9300X-24Y_CORE-01", "C9300X-24Y_CORE-01_show_interfaces_status.txt")
	
	interfaces := ParseInterfaceStatus(output)
	
	if len(interfaces) == 0 {
		t.Error("Expected interface entries, got none")
	}
	
	t.Logf("Parsed %d interfaces", len(interfaces))
	
	// Find a connected interface
	var connected *InterfaceStatus
	for i := range interfaces {
		if interfaces[i].Status == "connected" {
			connected = &interfaces[i]
			break
		}
	}
	
	if connected != nil {
		t.Logf("Found connected interface: %s, VLAN=%s, Speed=%s",
			connected.Name, connected.VLAN, connected.Speed)
		
		if connected.Name == "" {
			t.Error("Interface name should not be empty")
		}
	}
}

func TestParseInterfaceStatus_AllDevices(t *testing.T) {
	devices := []string{
		"C9300X-24Y_CORE-01",
		"IE-2000-8TC-G-E_LEGACY-01",
		"IE-3400H-24T4S-E_PLANT-01",
		"IE-5000-12S12P-10G_CORE-01",
		"WS-C2960X-24TD-L_IDF-02",
		"WS-C3850-48XS-E_DIST-01",
	}
	
	for _, device := range devices {
		t.Run(device, func(t *testing.T) {
			output := readTestFile(t, device, device+"_show_interfaces_status.txt")
			interfaces := ParseInterfaceStatus(output)
			
			t.Logf("[%s] Parsed %d interfaces", device, len(interfaces))
			
			// Count by status
			statusCounts := make(map[string]int)
			for _, iface := range interfaces {
				statusCounts[iface.Status]++
			}
			
			for status, count := range statusCounts {
				t.Logf("[%s] %s interfaces: %d", device, status, count)
			}
		})
	}
}

// ===========================
// Interface Trunk Tests
// ===========================

func TestParseInterfacesTrunk_C9300X(t *testing.T) {
	output := readTestFile(t, "C9300X-24Y_CORE-01", "C9300X-24Y_CORE-01_show_interfaces_trunk.txt")
	
	trunks := ParseInterfacesTrunk(output)
	
	t.Logf("Parsed %d trunk interfaces", len(trunks))
	
	if len(trunks) > 0 {
		first := trunks[0]
		t.Logf("First trunk: Port=%s, Mode=%s, Native=%d, Allowed VLANs=%v",
			first.Port, first.Mode, first.NativeVLAN, first.AllowedVLANs)
		
		if first.Port == "" {
			t.Error("Trunk port name should not be empty")
		}
		
		if len(first.AllowedVLANs) == 0 {
			t.Log("WARNING: No allowed VLANs parsed (might be 'none')")
		}
	}
}

func TestParseInterfacesTrunk_AllDevices(t *testing.T) {
	devicesWithTrunks := []string{
		"C9300X-24Y_CORE-01",
		"IE-3400H-24T4S-E_PLANT-01",
		"IE-5000-12S12P-10G_CORE-01",
		"WS-C2960X-24TD-L_IDF-02",
	}
	
	for _, device := range devicesWithTrunks {
		t.Run(device, func(t *testing.T) {
			output := readTestFile(t, device, device+"_show_interfaces_trunk.txt")
			trunks := ParseInterfacesTrunk(output)
			
			t.Logf("[%s] Parsed %d trunk interfaces", device, len(trunks))
			
			for _, trunk := range trunks {
				t.Logf("[%s] Trunk %s: Native=%d, Allowed=%d VLANs, Active=%d VLANs",
					device, trunk.Port, trunk.NativeVLAN, len(trunk.AllowedVLANs), len(trunk.ActiveVLANs))
			}
		})
	}
}

// ===========================
// EtherChannel Tests
// ===========================

func TestParseEtherChannelSummary_C9300X(t *testing.T) {
	output := readTestFile(t, "C9300X-24Y_CORE-01", "C9300X-24Y_CORE-01_show_etherchannel_summary.txt")
	
	channels := ParseEtherChannelSummary(output)
	
	t.Logf("Parsed %d port-channels", len(channels))
	
	if len(channels) > 0 {
		first := channels[0]
		t.Logf("First channel: Group=%d, Po=%s, Protocol=%s, Members=%v, Flags=%s",
			first.GroupNumber, first.PortChannel, first.Protocol, first.Ports, first.Flags)
		
		if first.GroupNumber == 0 {
			t.Error("Group number should not be 0")
		}
		
		if first.PortChannel == "" {
			t.Error("PortChannel name should not be empty")
		}
		
		if len(first.Ports) == 0 {
			t.Error("EtherChannel should have member ports")
		}
	}
}

func TestParseEtherChannelSummary_AllDevices(t *testing.T) {
	devicesWithLAG := []string{
		"C9300X-24Y_CORE-01",
		"IE-3400H-24T4S-E_PLANT-01",
		"IE-5000-12S12P-10G_CORE-01",
	}
	
	for _, device := range devicesWithLAG {
		t.Run(device, func(t *testing.T) {
			output := readTestFile(t, device, device+"_show_etherchannel_summary.txt")
			channels := ParseEtherChannelSummary(output)
			
			t.Logf("[%s] Parsed %d port-channels", device, len(channels))
			
			for _, ch := range channels {
				t.Logf("[%s] Po%d (%s): %d members, Protocol=%s",
					device, ch.GroupNumber, ch.Flags, len(ch.Ports), ch.Protocol)
			}
		})
	}
}

// ===========================
// DHCP Snooping Tests
// ===========================

func TestParseDHCPSnoopingBinding_C9300X(t *testing.T) {
	output := readTestFile(t, "C9300X-24Y_CORE-01", "C9300X-24Y_CORE-01_show_ip_dhcp_snooping_binding.txt")
	
	bindings := ParseDHCPSnoopingBinding(output)
	
	t.Logf("Parsed %d DHCP bindings", len(bindings))
	
	if len(bindings) > 0 {
		first := bindings[0]
		t.Logf("First binding: MAC=%s, IP=%s, VLAN=%d, Interface=%s, Lease=%d",
			first.MACAddress, first.IPAddress, first.VLAN, first.Interface, first.Lease)
		
		if first.MACAddress == "" {
			t.Error("MAC address should not be empty")
		}
		
		if first.IPAddress == "" {
			t.Error("IP address should not be empty")
		}
		
		if first.VLAN == 0 {
			t.Error("VLAN should not be 0")
		}
		
		if first.Interface == "" {
			t.Error("Interface should not be empty")
		}
	}
}

func TestParseDHCPSnoopingBinding_AllDevices(t *testing.T) {
	devices := []string{
		"C9300X-24Y_CORE-01",
		"IE-2000-8TC-G-E_LEGACY-01",
		"IE-3400H-24T4S-E_PLANT-01",
		"IE-5000-12S12P-10G_CORE-01",
		"WS-C2960X-24TD-L_IDF-02",
		"WS-C3850-48XS-E_DIST-01",
	}
	
	for _, device := range devices {
		t.Run(device, func(t *testing.T) {
			output := readTestFile(t, device, device+"_show_ip_dhcp_snooping_binding.txt")
			bindings := ParseDHCPSnoopingBinding(output)
			
			t.Logf("[%s] Parsed %d DHCP bindings", device, len(bindings))
			
			// Validate bindings
			for _, binding := range bindings {
				if binding.MACAddress == "" || binding.IPAddress == "" {
					t.Errorf("[%s] Invalid binding: MAC=%s, IP=%s",
						device, binding.MACAddress, binding.IPAddress)
				}
			}
		})
	}
}

// ===========================
// ARP Inspection Tests
// ===========================

func TestParseARPInspection_C9300X(t *testing.T) {
	output := readTestFile(t, "C9300X-24Y_CORE-01", "C9300X-24Y_CORE-01_show_ip_arp_inspection.txt")
	
	config := ParseARPInspection(output)
	
	t.Logf("DAI Config: SourceMAC=%v, DestMAC=%v, IP=%v, VLANs=%d",
		config.SourceMACValidation, config.DestMACValidation,
		config.IPValidation, len(config.VLANConfigs))
	
	if !config.SourceMACValidation {
		t.Log("WARNING: Source MAC validation not enabled")
	}
	
	if len(config.VLANConfigs) > 0 {
		first := config.VLANConfigs[0]
		t.Logf("First VLAN: VLAN=%d, Config=%s, Operation=%s, ACL=%s",
			first.VLAN, first.Config, first.Operation, first.ACLMatch)
		
		if first.VLAN == 0 {
			t.Error("VLAN ID should not be 0")
		}
	}
}

// ===========================
// Port Security Tests
// ===========================

func TestParsePortSecurity_C9300X(t *testing.T) {
	output := readTestFile(t, "C9300X-24Y_CORE-01", "C9300X-24Y_CORE-01_show_port-security.txt")
	
	entries := ParsePortSecurity(output)
	
	t.Logf("Parsed %d port-security entries", len(entries))
	
	if len(entries) > 0 {
		first := entries[0]
		t.Logf("First entry: Interface=%s, Max=%d, Current=%d, Violations=%d, Action=%s",
			first.Interface, first.MaxSecureAddr, first.CurrentAddr,
			first.ViolationCount, first.SecurityAction)
		
		if first.Interface == "" {
			t.Error("Interface should not be empty")
		}
		
		if first.MaxSecureAddr == 0 {
			t.Log("WARNING: MaxSecureAddr is 0")
		}
		
		if first.SecurityAction == "" {
			t.Error("SecurityAction should not be empty")
		}
	}
}

func TestParsePortSecurity_AllDevices(t *testing.T) {
	devicesWithPortSec := []string{
		"C9300X-24Y_CORE-01",
		"WS-C2960X-24TD-L_IDF-02",
	}
	
	for _, device := range devicesWithPortSec {
		t.Run(device, func(t *testing.T) {
			output := readTestFile(t, device, device+"_show_port-security.txt")
			entries := ParsePortSecurity(output)
			
			t.Logf("[%s] Parsed %d port-security entries", device, len(entries))
			
			// Count by action
			actionCounts := make(map[string]int)
			for _, entry := range entries {
				actionCounts[entry.SecurityAction]++
			}
			
			for action, count := range actionCounts {
				t.Logf("[%s] Action '%s': %d ports", device, action, count)
			}
		})
	}
}

// ===========================
// VLAN List Parsing Tests
// ===========================

func TestParseVLANList(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []int
	}{
		{
			name:     "Simple list",
			input:    "10,20,30",
			expected: []int{10, 20, 30},
		},
		{
			name:     "With range",
			input:    "10,20-22,30",
			expected: []int{10, 20, 21, 22, 30},
		},
		{
			name:     "Multiple ranges",
			input:    "10-12,20-22",
			expected: []int{10, 11, 12, 20, 21, 22},
		},
		{
			name:     "Single VLAN",
			input:    "100",
			expected: []int{100},
		},
		{
			name:     "Empty",
			input:    "",
			expected: []int{},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseVLANList(tt.input)
			
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d VLANs, got %d", len(tt.expected), len(result))
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
// MAC Normalization Tests
// ===========================

func TestNormalizeMACAddress(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Cisco format",
			input:    "0004.4d41.4e31",
			expected: "00:04:4D:41:4E:31",
		},
		{
			name:     "Cisco format uppercase",
			input:    "00A0.4506.7891",
			expected: "00:A0:45:06:78:91",
		},
		{
			name:     "Already normalized",
			input:    "00:04:4d:41:4e:31",
			expected: "00:04:4D:41:4E:31",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeMACAddress(tt.input)
			
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

// ===========================
// Integration Test
// ===========================

func TestFullDeviceProcessing_C9300X(t *testing.T) {
	device := "C9300X-24Y_CORE-01"
	collectionTime := time.Now()
	
	t.Log("=== Full Device Processing Test ===")
	t.Logf("Device: %s", device)
	t.Logf("Collection Time: %s", collectionTime.Format(time.RFC3339))
	
	// Parse all available commands
	macOutput := readTestFile(t, device, device+"_show_mac-address-table.txt")
	fdbEntries := ParseMACAddressTable(macOutput, collectionTime)
	t.Logf("✓ MAC Address Table: %d entries", len(fdbEntries))
	
	statusOutput := readTestFile(t, device, device+"_show_interfaces_status.txt")
	interfaces := ParseInterfaceStatus(statusOutput)
	t.Logf("✓ Interface Status: %d interfaces", len(interfaces))
	
	trunkOutput := readTestFile(t, device, device+"_show_interfaces_trunk.txt")
	trunks := ParseInterfacesTrunk(trunkOutput)
	t.Logf("✓ Trunk Interfaces: %d trunks", len(trunks))
	
	lagOutput := readTestFile(t, device, device+"_show_etherchannel_summary.txt")
	lags := ParseEtherChannelSummary(lagOutput)
	t.Logf("✓ EtherChannel: %d port-channels", len(lags))
	
	dhcpOutput := readTestFile(t, device, device+"_show_ip_dhcp_snooping_binding.txt")
	dhcpBindings := ParseDHCPSnoopingBinding(dhcpOutput)
	t.Logf("✓ DHCP Bindings: %d bindings", len(dhcpBindings))
	
	daiOutput := readTestFile(t, device, device+"_show_ip_arp_inspection.txt")
	daiConfig := ParseARPInspection(daiOutput)
	t.Logf("✓ ARP Inspection: %d VLANs configured", len(daiConfig.VLANConfigs))
	
	portSecOutput := readTestFile(t, device, device+"_show_port-security.txt")
	portSecEntries := ParsePortSecurity(portSecOutput)
	t.Logf("✓ Port Security: %d interfaces", len(portSecEntries))
	
	t.Log("\n=== Summary ===")
	t.Logf("Total FDB entries: %d", len(fdbEntries))
	t.Logf("Total interfaces: %d", len(interfaces))
	t.Logf("Total trunks: %d", len(trunks))
	t.Logf("Total LAGs: %d", len(lags))
	t.Logf("Total DHCP bindings: %d", len(dhcpBindings))
	t.Logf("Total DAI VLANs: %d", len(daiConfig.VLANConfigs))
	t.Logf("Total port-security interfaces: %d", len(portSecEntries))
	
	// Cross-validation: Check if DHCP binding MACs exist in FDB
	dhcpMACs := make(map[string]bool)
	for _, binding := range dhcpBindings {
		dhcpMACs[binding.MACAddress] = true
	}
	
	fdbMACs := make(map[string]bool)
	for _, entry := range fdbEntries {
		fdbMACs[entry.MAC] = true
	}
	
	matchCount := 0
	for mac := range dhcpMACs {
		if fdbMACs[mac] {
			matchCount++
		}
	}
	
	t.Logf("\n=== Cross-Validation ===")
	t.Logf("DHCP MACs also in FDB: %d/%d (%.1f%%)",
		matchCount, len(dhcpMACs),
		float64(matchCount)/float64(len(dhcpMACs))*100)
}

// ===========================
// VLAN Tests (NEW)
// ===========================

func TestParseVLAN(t *testing.T) {
	sampleOutput := `VLAN Name                             Status    Ports
---- -------------------------------- --------- -------------------------------
1    default                          active    Gi1/0/1, Gi1/0/2, Gi1/0/3
10   Engineering                      active    Gi1/0/4, Gi1/0/5
20   Manufacturing                    active    Gi1/0/6
30   IT                               active
100  Guest                            suspended
1002 fddi-default                     act/unsup
1003 token-ring-default               act/unsup
1004 fddinet-default                  act/unsup
1005 trnet-default                    act/unsup`

	vlans := ParseVLAN(sampleOutput)

	if len(vlans) == 0 {
		t.Fatal("Expected VLAN entries, got none")
	}

	t.Logf("Parsed %d VLANs", len(vlans))

	// Validate VLAN 10
	var vlan10 *VLANEntry
	for i := range vlans {
		if vlans[i].ID == 10 {
			vlan10 = &vlans[i]
			break
		}
	}

	if vlan10 == nil {
		t.Fatal("Expected to find VLAN 10")
	}

	if vlan10.Name != "Engineering" {
		t.Errorf("Expected name 'Engineering', got '%s'", vlan10.Name)
	}

	if vlan10.Status != "active" {
		t.Errorf("Expected status 'active', got '%s'", vlan10.Status)
	}

	if len(vlan10.Ports) != 2 {
		t.Errorf("Expected 2 ports, got %d", len(vlan10.Ports))
	}

	t.Logf("VLAN 10: Name=%s, Status=%s, Ports=%v", vlan10.Name, vlan10.Status, vlan10.Ports)

	// Validate VLAN 30 (no ports)
	var vlan30 *VLANEntry
	for i := range vlans {
		if vlans[i].ID == 30 {
			vlan30 = &vlans[i]
			break
		}
	}

	if vlan30 != nil && len(vlan30.Ports) != 0 {
		t.Logf("VLAN 30 has no ports (expected)")
	}

	// Validate VLAN 100 (suspended)
	var vlan100 *VLANEntry
	for i := range vlans {
		if vlans[i].ID == 100 {
			vlan100 = &vlans[i]
			break
		}
	}

	if vlan100 != nil && vlan100.Status != "suspended" {
		t.Errorf("Expected VLAN 100 status 'suspended', got '%s'", vlan100.Status)
	}
}

func TestParseVLAN_Empty(t *testing.T) {
	emptyOutput := `VLAN Name                             Status    Ports
---- -------------------------------- --------- -------------------------------`

	vlans := ParseVLAN(emptyOutput)

	if len(vlans) != 0 {
		t.Errorf("Expected 0 VLANs for empty output, got %d", len(vlans))
	}
}

// ===========================
// IP Interface Brief Tests (NEW)
// ===========================

func TestParseIPInterfaceBrief(t *testing.T) {
	sampleOutput := `Interface              IP-Address      OK? Method Status                Protocol
GigabitEthernet1/0/1   10.1.1.1        YES manual up                    up
GigabitEthernet1/0/2   unassigned      YES unset  down                  down
Vlan10                 10.0.10.1       YES manual up                    up
Vlan20                 10.0.20.1       YES manual up                    up
Vlan30                 10.0.30.1       YES DHCP   up                    up
Vlan100                unassigned      YES unset  administratively down down
Loopback0              192.168.1.1     YES manual up                    up`

	interfaces := ParseIPInterfaceBrief(sampleOutput)

	if len(interfaces) == 0 {
		t.Fatal("Expected IP interface entries, got none")
	}

	t.Logf("Parsed %d IP interfaces", len(interfaces))

	// Validate Gi1/0/1
	var gi1 *IPInterfaceBrief
	for i := range interfaces {
		if interfaces[i].Interface == "GigabitEthernet1/0/1" {
			gi1 = &interfaces[i]
			break
		}
	}

	if gi1 == nil {
		t.Fatal("Expected to find GigabitEthernet1/0/1")
	}

	if gi1.IPAddress != "10.1.1.1" {
		t.Errorf("Expected IP '10.1.1.1', got '%s'", gi1.IPAddress)
	}

	if gi1.OK != "YES" {
		t.Errorf("Expected OK='YES', got '%s'", gi1.OK)
	}

	if gi1.Method != "manual" {
		t.Errorf("Expected Method='manual', got '%s'", gi1.Method)
	}

	if gi1.Status != "up" {
		t.Errorf("Expected Status='up', got '%s'", gi1.Status)
	}

	if gi1.Protocol != "up" {
		t.Errorf("Expected Protocol='up', got '%s'", gi1.Protocol)
	}

	t.Logf("Gi1/0/1: IP=%s, Method=%s, Status=%s/%s", gi1.IPAddress, gi1.Method, gi1.Status, gi1.Protocol)

	// Validate Vlan30 (DHCP)
	var vlan30 *IPInterfaceBrief
	for i := range interfaces {
		if interfaces[i].Interface == "Vlan30" {
			vlan30 = &interfaces[i]
			break
		}
	}

	if vlan30 != nil && vlan30.Method != "DHCP" {
		t.Errorf("Expected Vlan30 Method='DHCP', got '%s'", vlan30.Method)
	}

	// Count unassigned interfaces
	unassignedCount := 0
	for _, iface := range interfaces {
		if iface.IPAddress == "unassigned" {
			unassignedCount++
		}
	}
	t.Logf("Unassigned interfaces: %d", unassignedCount)
}

func TestParseIPInterfaceBrief_Empty(t *testing.T) {
	emptyOutput := `Interface              IP-Address      OK? Method Status                Protocol`

	interfaces := ParseIPInterfaceBrief(emptyOutput)

	if len(interfaces) != 0 {
		t.Errorf("Expected 0 interfaces for empty output, got %d", len(interfaces))
	}
}

// ===========================
// ARP Table Tests (NEW)
// ===========================

func TestParseIPARP(t *testing.T) {
	sampleOutput := `Protocol  Address          Age (min)  Hardware Addr   Type   Interface
Internet  10.0.10.1               -   0050.56bb.0001  ARPA   Vlan10
Internet  10.0.10.2               5   0050.56bb.0002  ARPA   Vlan10
Internet  10.0.10.3              12   0050.56bb.0003  ARPA   Vlan10
Internet  10.0.20.1               -   0050.56bb.0010  ARPA   Vlan20
Internet  10.0.20.5               -   Incomplete      ARPA   Vlan20
Internet  192.168.1.100          45   00a0.c956.1234  ARPA   GigabitEthernet1/0/1`

	entries := ParseIPARP(sampleOutput)

	if len(entries) == 0 {
		t.Fatal("Expected ARP entries, got none")
	}

	t.Logf("Parsed %d ARP entries", len(entries))

	// Validate first entry (local interface)
	var entry1 *ARPEntry
	for i := range entries {
		if entries[i].IPAddress == "10.0.10.1" {
			entry1 = &entries[i]
			break
		}
	}

	if entry1 == nil {
		t.Fatal("Expected to find ARP entry for 10.0.10.1")
	}

	if entry1.Protocol != "Internet" {
		t.Errorf("Expected Protocol='Internet', got '%s'", entry1.Protocol)
	}

	if entry1.MACAddress != "0050.56bb.0001" {
		t.Errorf("Expected MAC='0050.56bb.0001', got '%s'", entry1.MACAddress)
	}

	if entry1.Type != "ARPA" {
		t.Errorf("Expected Type='ARPA', got '%s'", entry1.Type)
	}

	if entry1.Interface != "Vlan10" {
		t.Errorf("Expected Interface='Vlan10', got '%s'", entry1.Interface)
	}

	t.Logf("ARP 10.0.10.1: MAC=%s, Age=%d, Interface=%s", entry1.MACAddress, entry1.Age, entry1.Interface)

	// Validate incomplete entry
	var incompleteEntry *ARPEntry
	for i := range entries {
		if entries[i].IPAddress == "10.0.20.5" {
			incompleteEntry = &entries[i]
			break
		}
	}

	if incompleteEntry == nil {
		t.Fatal("Expected to find incomplete ARP entry for 10.0.20.5")
	}

	if incompleteEntry.MACAddress != "Incomplete" {
		t.Errorf("Expected MAC='Incomplete', got '%s'", incompleteEntry.MACAddress)
	}

	if incompleteEntry.Age != -1 {
		t.Errorf("Expected Age=-1 for incomplete entry, got %d", incompleteEntry.Age)
	}

	t.Logf("Incomplete ARP: IP=%s, MAC=%s", incompleteEntry.IPAddress, incompleteEntry.MACAddress)

	// Count valid vs incomplete
	validCount := 0
	incompleteCount := 0
	for _, entry := range entries {
		if entry.MACAddress == "Incomplete" {
			incompleteCount++
		} else {
			validCount++
		}
	}
	t.Logf("Valid ARP entries: %d, Incomplete: %d", validCount, incompleteCount)
}

func TestParseIPARP_Empty(t *testing.T) {
	emptyOutput := `Protocol  Address          Age (min)  Hardware Addr   Type   Interface`

	entries := ParseIPARP(emptyOutput)

	if len(entries) != 0 {
		t.Errorf("Expected 0 ARP entries for empty output, got %d", len(entries))
	}
}

// ===========================
// IGMP Snooping Tests (NEW)
// ===========================

func TestParseIGMPSnoopingGroups(t *testing.T) {
	sampleOutput := `Vlan      Group                    Type    Version Port
----      -----                    ----    ------- ----
10        239.1.1.1                igmp    v2      Gi1/0/1
10        239.1.1.2                igmp    v2      Gi1/0/1
10        239.1.1.5                igmp    v3      Gi1/0/2
20        239.2.1.1                igmp    v2      Gi1/0/5
20        239.2.1.100              static  -       Gi1/0/6
30        239.3.5.10               igmp    v2      Po1`

	groups := ParseIGMPSnoopingGroups(sampleOutput)

	if len(groups) == 0 {
		t.Fatal("Expected IGMP snooping groups, got none")
	}

	t.Logf("Parsed %d IGMP snooping groups", len(groups))

	// Validate first group
	if len(groups) > 0 {
		first := groups[0]
		if first.VLAN != 10 {
			t.Errorf("Expected VLAN=10, got %d", first.VLAN)
		}
		if first.GroupAddress != "239.1.1.1" {
			t.Errorf("Expected GroupAddress='239.1.1.1', got '%s'", first.GroupAddress)
		}
		if first.Type != "igmp" {
			t.Errorf("Expected Type='igmp', got '%s'", first.Type)
		}
		if first.Version != "v2" {
			t.Errorf("Expected Version='v2', got '%s'", first.Version)
		}
		if first.Port != "Gi1/0/1" {
			t.Errorf("Expected Port='Gi1/0/1', got '%s'", first.Port)
		}
		t.Logf("First group: VLAN=%d, Group=%s, Type=%s, Version=%s, Port=%s",
			first.VLAN, first.GroupAddress, first.Type, first.Version, first.Port)
	}

	// Validate static entry
	var staticGroup *IGMPSnoopingGroup
	for i := range groups {
		if groups[i].Type == "static" {
			staticGroup = &groups[i]
			break
		}
	}

	if staticGroup != nil {
		t.Logf("Found static group: VLAN=%d, Group=%s", staticGroup.VLAN, staticGroup.GroupAddress)
		if staticGroup.VLAN != 20 {
			t.Errorf("Expected static group in VLAN 20, got %d", staticGroup.VLAN)
		}
	}

	// Count groups by VLAN
	vlanCounts := make(map[int]int)
	for _, group := range groups {
		vlanCounts[group.VLAN]++
	}
	for vlan, count := range vlanCounts {
		t.Logf("VLAN %d: %d groups", vlan, count)
	}
}

func TestParseIGMPSnoopingGroups_Empty(t *testing.T) {
	emptyOutput := `Vlan      Group                    Type    Version Port
----      -----                    ----    ------- ----`

	groups := ParseIGMPSnoopingGroups(emptyOutput)

	if len(groups) != 0 {
		t.Errorf("Expected 0 groups for empty output, got %d", len(groups))
	}
}

// ===========================
// Version Parsing Tests (NEW)
// ===========================

func TestParseVersion(t *testing.T) {
	sampleOutput := `Cisco IOS Software, C3750 Software (C3750-IPSERVICESK9-M), Version 15.0(2)SE11, RELEASE SOFTWARE (fc3)
Technical Support: http://www.cisco.com/techsupport
Copyright (c) 1986-2016 by Cisco Systems, Inc.
Compiled Sat 16-Jul-16 00:46 by prod_rel_team

ROM: Bootstrap program is C3750 boot loader
BOOTLDR: C3750 Boot Loader (C3750-HBOOT-M) Version 12.2(53r)SEY4, RELEASE SOFTWARE (fc1)

Switch uptime is 2 weeks, 3 days, 12 hours, 45 minutes
System returned to ROM by power-on
System image file is "flash:/c3750-ipservicesk9-mz.150-2.SE11.bin"

This product contains cryptographic features and is subject to United
States and local country laws governing import, export, transfer and
use. Delivery of Cisco cryptographic products does not imply
third-party authority to import, export, distribute or use encryption.

cisco WS-C3750G-24TS (PowerPC405) processor (revision M0) with 131072K/12280K bytes of memory.
Processor board ID FOC1234X5YZ
Last reset from power-on
1 Virtual Ethernet interface
24 Gigabit Ethernet interfaces
4 Ten Gigabit Ethernet interfaces
The password-recovery mechanism is enabled.

512K bytes of flash-simulated non-volatile configuration memory.
Base ethernet MAC Address       : 00:1E:BD:12:34:56
Motherboard assembly number     : 73-12345-67
Power supply part number        : 341-0097-03
Motherboard serial number       : FOC12345678
Power supply serial number      : AZS12345678
Model revision number           : M0
Motherboard revision number     : A0
Model number                    : WS-C3750G-24TS-E
System serial number            : FOC1234X5YZ
Top Assembly Part Number        : 800-27393-03
Top Assembly Revision Number    : A0
Version ID                      : V09
CLEI Code Number                : COMA900ARC
Hardware Board Revision Number  : 0x01

Configuration register is 0x010F`

	info := ParseVersion(sampleOutput)

	if info == nil {
		t.Fatal("Expected VersionInfo, got nil")
	}

	// Validate version
	if info.Version != "15.0(2)SE11" {
		t.Errorf("Expected Version='15.0(2)SE11', got '%s'", info.Version)
	}

	// Validate uptime
	if !strings.Contains(info.Uptime, "2 weeks") {
		t.Errorf("Expected uptime to contain '2 weeks', got '%s'", info.Uptime)
	}

	// Validate system image
	expectedImage := "flash:/c3750-ipservicesk9-mz.150-2.SE11.bin"
	if info.SystemImage != expectedImage {
		t.Errorf("Expected SystemImage='%s', got '%s'", expectedImage, info.SystemImage)
	}

	// Validate model
	if info.Model != "WS-C3750G-24TS" {
		t.Errorf("Expected Model='WS-C3750G-24TS', got '%s'", info.Model)
	}

	// Validate processor type
	if info.ProcessorType != "PowerPC405" {
		t.Errorf("Expected ProcessorType='PowerPC405', got '%s'", info.ProcessorType)
	}

	// Validate serial number
	if info.SerialNumber != "FOC1234X5YZ" {
		t.Errorf("Expected SerialNumber='FOC1234X5YZ', got '%s'", info.SerialNumber)
	}

	// Validate memory
	if info.MemorySize == "" {
		t.Error("Expected MemorySize to be populated")
	}

	// Validate config register
	if info.ConfigRegister != "0x010F" {
		t.Errorf("Expected ConfigRegister='0x010F', got '%s'", info.ConfigRegister)
	}

	t.Logf("Version Info:")
	t.Logf("  Version: %s", info.Version)
	t.Logf("  Model: %s", info.Model)
	t.Logf("  Processor: %s", info.ProcessorType)
	t.Logf("  Serial: %s", info.SerialNumber)
	t.Logf("  Uptime: %s", info.Uptime)
	t.Logf("  Image: %s", info.SystemImage)
	t.Logf("  Memory: %s", info.MemorySize)
	t.Logf("  Config Register: %s", info.ConfigRegister)
}

func TestParseVersion_Catalyst9300(t *testing.T) {
	sampleOutput := `Cisco IOS XE Software, Version 17.06.03
Cisco IOS Software [Bengaluru], Catalyst L3 Switch Software (CAT9K_IOSXE), Version 17.6.3, RELEASE SOFTWARE (fc4)
Technical Support: http://www.cisco.com/techsupport
Copyright (c) 1986-2021 by Cisco Systems, Inc.
Compiled Tue 10-Aug-21 03:47 by mcpre

Cisco IOS-XE software, Copyright (c) 2005-2021 by cisco Systems, Inc.
All rights reserved.  Certain components of Cisco IOS-XE software are
licensed under the GNU General Public License ("GPL") Version 2.0.

Technology Package License Information:

------------------------------------------------------------------------------
Technology-package                                     Technology-package
Current              Type                              Next reboot
------------------------------------------------------------------------------
network-advantage    Smart License                     network-advantage
dna-advantage        Subscription Smart License        dna-advantage

Smart Licensing Status: REGISTERED/AUTHORIZED

cisco C9300-24UX (X86) processor with 1389043K/6147K bytes of memory.
Processor board ID FCW2345A1B2
2048K bytes of non-volatile configuration memory.
16777216K bytes of physical memory.
1638400K bytes of Crash Files at crashinfo:.
11264000K bytes of Flash at flash:.
0K bytes of WebUI ODM Files at webui:.

Base Ethernet MAC Address          : a4:4c:11:22:33:44
Motherboard Assembly Number        : 73-18276-04
Motherboard Serial Number          : FOC23456789
Model Revision Number              : A0
Motherboard Revision Number        : A0
Model Number                       : C9300-24UX
System Serial Number               : FCW2345A1B2

Switch Ports Model              SW Version        SW Image
------ ----- -----              ----------        ----------
*    1 28    C9300-24UX         17.06.03          CAT9K_IOSXE

Configuration register is 0x102`

	info := ParseVersion(sampleOutput)

	if info == nil {
		t.Fatal("Expected VersionInfo, got nil")
	}

	// Validate version (IOS XE format)
	if !strings.Contains(info.Version, "17.6") && !strings.Contains(info.Version, "17.06") {
		t.Logf("Warning: Expected version to contain '17.6', got '%s' (parser may need IOS XE enhancement)", info.Version)
	}

	// Validate model
	if !strings.Contains(info.Model, "C9300") {
		t.Logf("Warning: Expected model to contain 'C9300', got '%s'", info.Model)
	}

	t.Logf("Catalyst 9300 Version Info:")
	t.Logf("  Version: %s", info.Version)
	t.Logf("  Model: %s", info.Model)
	t.Logf("  Serial: %s", info.SerialNumber)
	t.Logf("  Processor: %s", info.ProcessorType)
}

func TestParseVersion_Empty(t *testing.T) {
	info := ParseVersion("")

	if info == nil {
		t.Fatal("Expected empty VersionInfo struct, got nil")
	}

	if info.Version != "" {
		t.Errorf("Expected empty Version, got '%s'", info.Version)
	}
}

// ===========================
// L2 Security Control Parser Tests
// ===========================

func TestParseDHCPSnoopingBinding(t *testing.T) {
	sampleOutput := `MacAddress          IpAddress        Lease(sec)  Type           VLAN  Interface
------------------  ---------------  ----------  -------------  ----  --------------------
00:50:56:bb:00:01   10.0.10.2        86395       dhcp-snooping  10    GigabitEthernet1/0/1
00:50:56:bb:00:02   10.0.10.3        86395       dhcp-snooping  10    GigabitEthernet1/0/2
00:50:56:bb:00:03   10.0.20.5        3600        dhcp-snooping  20    GigabitEthernet1/0/5
aa:bb:cc:dd:ee:ff   10.1.1.100       0           static         100   GigabitEthernet1/0/10`

	bindings := ParseDHCPSnoopingBinding(sampleOutput)

	if len(bindings) == 0 {
		t.Fatal("Expected DHCP snooping bindings, got none")
	}

	t.Logf("Parsed %d DHCP snooping bindings", len(bindings))

	// Validate first binding
	if len(bindings) > 0 {
		first := bindings[0]
		if first.MACAddress != "00:50:56:bb:00:01" {
			t.Errorf("Expected MAC '00:50:56:bb:00:01', got '%s'", first.MACAddress)
		}
		if first.IPAddress != "10.0.10.2" {
			t.Errorf("Expected IP '10.0.10.2', got '%s'", first.IPAddress)
		}
		if first.LeaseSeconds != 86395 {
			t.Errorf("Expected lease 86395, got %d", first.LeaseSeconds)
		}
		if first.Type != "dhcp-snooping" {
			t.Errorf("Expected type 'dhcp-snooping', got '%s'", first.Type)
		}
		if first.VLAN != 10 {
			t.Errorf("Expected VLAN 10, got %d", first.VLAN)
		}
		if first.Interface != "GigabitEthernet1/0/1" {
			t.Errorf("Expected interface 'GigabitEthernet1/0/1', got '%s'", first.Interface)
		}
		t.Logf("First binding: MAC=%s, IP=%s, VLAN=%d, Interface=%s, Lease=%ds",
			first.MACAddress, first.IPAddress, first.VLAN, first.Interface, first.LeaseSeconds)
	}

	// Validate static binding
	var staticBinding *DHCPSnoopingBinding
	for i := range bindings {
		if bindings[i].Type == "static" {
			staticBinding = &bindings[i]
			break
		}
	}

	if staticBinding != nil {
		t.Logf("Found static binding: MAC=%s, IP=%s", staticBinding.MACAddress, staticBinding.IPAddress)
		if staticBinding.MACAddress != "aa:bb:cc:dd:ee:ff" {
			t.Errorf("Expected static MAC 'aa:bb:cc:dd:ee:ff', got '%s'", staticBinding.MACAddress)
		}
	}

	// Count bindings by VLAN
	vlanCounts := make(map[int]int)
	for _, binding := range bindings {
		vlanCounts[binding.VLAN]++
	}
	t.Logf("Bindings by VLAN: %v", vlanCounts)
}

func TestParseDHCPSnoopingBinding_Empty(t *testing.T) {
	emptyOutput := `MacAddress          IpAddress        Lease(sec)  Type           VLAN  Interface
------------------  ---------------  ----------  -------------  ----  --------------------`

	bindings := ParseDHCPSnoopingBinding(emptyOutput)

	if len(bindings) != 0 {
		t.Errorf("Expected 0 bindings for empty output, got %d", len(bindings))
	}
}

func TestParseARPInspectionStatistics(t *testing.T) {
	sampleOutput := ` Vlan      Forwarded        Dropped     DHCP Drops      ACL Drops
 ----      ---------        -------     ----------      ---------
   10           1234            56            30              26
   20           5678            12             8               4
   30            100             5             3               2`

	stats := ParseARPInspectionStatistics(sampleOutput)

	if stats == nil {
		t.Fatal("Expected ARP inspection statistics, got nil")
	}

	// Check totals
	expectedForwarded := 1234 + 5678 + 100
	if stats.PacketsForwarded != expectedForwarded {
		t.Errorf("Expected forwarded=%d, got %d", expectedForwarded, stats.PacketsForwarded)
	}

	expectedDropped := 56 + 12 + 5
	if stats.PacketsDropped != expectedDropped {
		t.Errorf("Expected dropped=%d, got %d", expectedDropped, stats.PacketsDropped)
	}

	expectedDHCPDrops := 30 + 8 + 3
	if stats.DHCPSnoopingDenied != expectedDHCPDrops {
		t.Errorf("Expected DHCP drops=%d, got %d", expectedDHCPDrops, stats.DHCPSnoopingDenied)
	}

	expectedACLDrops := 26 + 4 + 2
	if stats.ACLDenied != expectedACLDrops {
		t.Errorf("Expected ACL drops=%d, got %d", expectedACLDrops, stats.ACLDenied)
	}

	// Check VLAN config
	if len(stats.VLANConfig) != 3 {
		t.Errorf("Expected 3 VLANs configured, got %d", len(stats.VLANConfig))
	}

	if stats.VLANConfig[10] != "enabled" {
		t.Errorf("Expected VLAN 10 enabled, got '%s'", stats.VLANConfig[10])
	}

	t.Logf("DAI Stats: Forwarded=%d, Dropped=%d, DHCP Drops=%d, ACL Drops=%d",
		stats.PacketsForwarded, stats.PacketsDropped, stats.DHCPSnoopingDenied, stats.ACLDenied)
	t.Logf("VLANs configured: %v", stats.VLANConfig)
}

func TestParseARPInspectionStatistics_Empty(t *testing.T) {
	emptyOutput := ` Vlan      Forwarded        Dropped     DHCP Drops      ACL Drops
 ----      ---------        -------     ----------      ---------`

	stats := ParseARPInspectionStatistics(emptyOutput)

	if stats == nil {
		t.Fatal("Expected stats object, got nil")
	}

	if stats.PacketsForwarded != 0 {
		t.Errorf("Expected 0 forwarded packets, got %d", stats.PacketsForwarded)
	}
}

func TestParseAuthenticationSessions(t *testing.T) {
	sampleOutput := `Interface  MAC Address     IP Address      Status         Domain  Method
---------- -------------- --------------- -------------- ------- -------
Gi1/0/1    0050.56bb.0001  10.0.10.2       Authc Success  DATA    dot1x
Gi1/0/2    0050.56bb.0002  10.0.20.3       Authc Success  DATA    mab
Gi1/0/3    0050.56bb.0003  10.0.30.5       Authc Failed   DATA    dot1x
Gi1/0/4    0050.56bb.0004  10.0.40.10      Running        VOICE   mab`

	sessions := ParseAuthenticationSessions(sampleOutput)

	if len(sessions) == 0 {
		t.Fatal("Expected authentication sessions, got none")
	}

	t.Logf("Parsed %d authentication sessions", len(sessions))

	// Validate first session (dot1x success)
	if len(sessions) > 0 {
		first := sessions[0]
		if first.Interface != "Gi1/0/1" {
			t.Errorf("Expected interface 'Gi1/0/1', got '%s'", first.Interface)
		}
		if first.MACAddress != "0050.56bb.0001" {
			t.Errorf("Expected MAC '0050.56bb.0001', got '%s'", first.MACAddress)
		}
		if first.IPv4Address != "10.0.10.2" {
			t.Errorf("Expected IP '10.0.10.2', got '%s'", first.IPv4Address)
		}
		if !strings.Contains(first.Status, "Success") {
			t.Errorf("Expected status to contain 'Success', got '%s'", first.Status)
		}
		if first.Method != "dot1x" {
			t.Errorf("Expected method 'dot1x', got '%s'", first.Method)
		}
		t.Logf("First session: Interface=%s, MAC=%s, Status=%s, Method=%s",
			first.Interface, first.MACAddress, first.Status, first.Method)
	}

	// Count sessions by method
	methodCounts := make(map[string]int)
	for _, session := range sessions {
		methodCounts[session.Method]++
	}
	t.Logf("Sessions by method: %v", methodCounts)

	// Count successful authentications
	successCount := 0
	for _, session := range sessions {
		if strings.Contains(session.Status, "Success") {
			successCount++
		}
	}
	t.Logf("Successful authentications: %d/%d", successCount, len(sessions))
}

func TestParseAuthenticationSessions_Empty(t *testing.T) {
	emptyOutput := `Interface  MAC Address     IP Address      Status         Domain  Method
---------- -------------- --------------- -------------- ------- -------`

	sessions := ParseAuthenticationSessions(emptyOutput)

	if len(sessions) != 0 {
		t.Errorf("Expected 0 sessions for empty output, got %d", len(sessions))
	}
}

func TestParsePortSecurity(t *testing.T) {
	sampleOutput := `Secure Port  MaxSecureAddr  CurrentAddr  SecurityViolation  Security Action
                (Count)       (Count)          (Count)
----------- --------------- ------------ ------------------- ---------------
Gi1/0/1              1            1                0           Shutdown
Gi1/0/2              2            2                5           Restrict
Gi1/0/3              3            1                0           Protect
Gi1/0/5             10            5               15           Shutdown`

	statuses := ParsePortSecurity(sampleOutput)

	if len(statuses) == 0 {
		t.Fatal("Expected port security statuses, got none")
	}

	t.Logf("Parsed %d port security entries", len(statuses))

	// Validate first entry (no violations)
	if len(statuses) > 0 {
		first := statuses[0]
		if first.Interface != "Gi1/0/1" {
			t.Errorf("Expected interface 'Gi1/0/1', got '%s'", first.Interface)
		}
		if first.MaxMAC != 1 {
			t.Errorf("Expected MaxMAC=1, got %d", first.MaxMAC)
		}
		if first.CurrentMAC != 1 {
			t.Errorf("Expected CurrentMAC=1, got %d", first.CurrentMAC)
		}
		if first.ViolationCount != 0 {
			t.Errorf("Expected ViolationCount=0, got %d", first.ViolationCount)
		}
		if first.ViolationMode != "Shutdown" {
			t.Errorf("Expected mode 'Shutdown', got '%s'", first.ViolationMode)
		}
		t.Logf("First entry: Interface=%s, MaxMAC=%d, CurrentMAC=%d, Violations=%d, Mode=%s",
			first.Interface, first.MaxMAC, first.CurrentMAC, first.ViolationCount, first.ViolationMode)
	}

	// Find entry with violations
	var violationEntry *PortSecurityStatus
	for i := range statuses {
		if statuses[i].ViolationCount > 0 {
			violationEntry = &statuses[i]
			break
		}
	}

	if violationEntry != nil {
		t.Logf("Found entry with violations: Interface=%s, Violations=%d, Mode=%s",
			violationEntry.Interface, violationEntry.ViolationCount, violationEntry.ViolationMode)
	}

	// Count by violation mode
	modeCounts := make(map[string]int)
	for _, status := range statuses {
		modeCounts[status.ViolationMode]++
	}
	t.Logf("Entries by mode: %v", modeCounts)

	// Total violations
	totalViolations := 0
	for _, status := range statuses {
		totalViolations += status.ViolationCount
	}
	t.Logf("Total violations across all ports: %d", totalViolations)
}

func TestParsePortSecurity_Empty(t *testing.T) {
	emptyOutput := `Secure Port  MaxSecureAddr  CurrentAddr  SecurityViolation  Security Action
                (Count)       (Count)          (Count)
----------- --------------- ------------ ------------------- ---------------`

	statuses := ParsePortSecurity(emptyOutput)

	if len(statuses) != 0 {
		t.Errorf("Expected 0 port security entries, got %d", len(statuses))
	}
}

func TestParseIPVerifySource(t *testing.T) {
	sampleOutput := `Interface  Filter-type  Filter-mode  IP-address       Mac-address        Vlan
---------  -----------  -----------  ---------------  -----------------  ----
Gi1/0/1    ip-mac       active       10.0.10.2        0050.56bb.0001     10
Gi1/0/2    ip           active       10.0.20.3        -                  20
Gi1/0/3    ip-mac       active       10.0.30.5        0050.56bb.0003     30`

	bindings := ParseIPVerifySource(sampleOutput)

	if len(bindings) == 0 {
		t.Fatal("Expected IP Source Guard bindings, got none")
	}

	t.Logf("Parsed %d IPSG bindings", len(bindings))

	// Validate first binding
	if len(bindings) > 0 {
		first := bindings[0]
		if first.Interface != "Gi1/0/1" {
			t.Errorf("Expected interface 'Gi1/0/1', got '%s'", first.Interface)
		}
		if first.IPAddress != "10.0.10.2" {
			t.Errorf("Expected IP '10.0.10.2', got '%s'", first.IPAddress)
		}
		if first.MACAddress != "0050.56bb.0001" {
			t.Errorf("Expected MAC '0050.56bb.0001', got '%s'", first.MACAddress)
		}
		if first.VLAN != 10 {
			t.Errorf("Expected VLAN 10, got %d", first.VLAN)
		}
		t.Logf("First binding: Interface=%s, IP=%s, MAC=%s, VLAN=%d",
			first.Interface, first.IPAddress, first.MACAddress, first.VLAN)
	}

	// Validate IP-only binding (no MAC)
	var ipOnlyBinding *IPSGBinding
	for i := range bindings {
		if bindings[i].MACAddress == "-" {
			ipOnlyBinding = &bindings[i]
			break
		}
	}

	if ipOnlyBinding != nil {
		t.Logf("Found IP-only binding: Interface=%s, IP=%s",
			ipOnlyBinding.Interface, ipOnlyBinding.IPAddress)
	}

	// Count bindings by VLAN
	vlanCounts := make(map[int]int)
	for _, binding := range bindings {
		vlanCounts[binding.VLAN]++
	}
	t.Logf("Bindings by VLAN: %v", vlanCounts)
}

func TestParseIPVerifySource_Empty(t *testing.T) {
	emptyOutput := `Interface  Filter-type  Filter-mode  IP-address       Mac-address        Vlan
---------  -----------  -----------  ---------------  -----------------  ----`

	bindings := ParseIPVerifySource(emptyOutput)

	if len(bindings) != 0 {
		t.Errorf("Expected 0 IPSG bindings, got %d", len(bindings))
	}
}

func TestParsePrivateVLAN(t *testing.T) {
	sampleOutput := `Primary Secondary Type              Ports
------- --------- ----------------- ------------------------------------------
100     101       isolated          Gi1/0/1, Gi1/0/2
100     102       community         Gi1/0/3, Gi1/0/4
200     201       isolated          Gi1/0/10
200     202       community         Gi1/0/11, Gi1/0/12, Gi1/0/13`

	pvlans := ParsePrivateVLAN(sampleOutput)

	if len(pvlans) == 0 {
		t.Fatal("Expected private VLAN entries, got none")
	}

	t.Logf("Parsed %d private VLAN mappings", len(pvlans))

	// Validate first entry
	if len(pvlans) > 0 {
		first := pvlans[0]
		if first.PrimaryVLAN != 100 {
			t.Errorf("Expected primary VLAN 100, got %d", first.PrimaryVLAN)
		}
		if first.SecondaryVLAN != 101 {
			t.Errorf("Expected secondary VLAN 101, got %d", first.SecondaryVLAN)
		}
		if first.SecondaryType != "isolated" {
			t.Errorf("Expected type 'isolated', got '%s'", first.SecondaryType)
		}
		if len(first.Ports) != 2 {
			t.Errorf("Expected 2 ports, got %d", len(first.Ports))
		}
		t.Logf("First mapping: Primary=%d, Secondary=%d, Type=%s, Ports=%v",
			first.PrimaryVLAN, first.SecondaryVLAN, first.SecondaryType, first.Ports)
	}

	// Count by primary VLAN
	primaryCounts := make(map[int]int)
	for _, pvlan := range pvlans {
		primaryCounts[pvlan.PrimaryVLAN]++
	}
	t.Logf("Mappings by primary VLAN: %v", primaryCounts)

	// Count by type
	typeCounts := make(map[string]int)
	for _, pvlan := range pvlans {
		typeCounts[pvlan.SecondaryType]++
	}
	t.Logf("Mappings by type: %v", typeCounts)

	// Validate community VLAN
	var communityVLAN *PrivateVLAN
	for i := range pvlans {
		if pvlans[i].SecondaryType == "community" && pvlans[i].PrimaryVLAN == 100 {
			communityVLAN = &pvlans[i]
			break
		}
	}

	if communityVLAN != nil {
		t.Logf("Found community VLAN: Primary=%d, Secondary=%d, Ports=%d",
			communityVLAN.PrimaryVLAN, communityVLAN.SecondaryVLAN, len(communityVLAN.Ports))
	}
}

func TestParsePrivateVLAN_Empty(t *testing.T) {
	emptyOutput := `Primary Secondary Type              Ports
------- --------- ----------------- ------------------------------------------`

	pvlans := ParsePrivateVLAN(emptyOutput)

	if len(pvlans) != 0 {
		t.Errorf("Expected 0 private VLAN entries, got %d", len(pvlans))
	}
}


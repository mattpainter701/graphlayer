package cisco

import (
	"testing"
)

// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |N|X|O|S| |L|2| |M|a|p|p|e|r| |U|n|i|t| |T|e|s|t|s|
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+

// TestBuildNXOSL2SecurityControls tests NXOS L2 security controls mapper
func TestBuildNXOSL2SecurityControls(t *testing.T) {
	// Create test processor
	p := &CiscoNXOSProcessor{
		nxosConfig: &NXOSConfig{Hostname: "test-nexus-01"},
	}

	// Test data (same structure as IOS but for NXOS)
	dhcpBindings := []DHCPSnoopingBinding{
		{
			MACAddress:   "00:50:56:bb:00:01",
			IPAddress:    "10.0.10.2",
			LeaseSeconds: 86395,
			Type:         "dhcp-snooping",
			VLAN:         10,
			Interface:    "Ethernet1/1",
		},
	}

	daiStats := ARPInspectionStatistics{
		Forwarded: 2500,
		Dropped:   25,
	}

	authSessions := []AuthenticationSession{
		{
			Interface:   "Ethernet1/1",
			MACAddress:  "00:50:56:bb:00:01",
			IPv4Address: "10.0.10.2",
			Status:      "Authc Success",
			Domain:      "DATA",
			Method:      "dot1x",
		},
	}

	portSecEntries := []PortSecurityEntry{
		{
			Interface:               "Ethernet1/2",
			MaximumMAC:              10,
			TotalMAC:                5,
			SecurityViolation:       "Restrict",
			SecurityViolationCount:  0,
		},
	}

	ipsgBindings := []IPVerifySourceBinding{
		{
			Interface:  "Ethernet1/1",
			Filter:     "ip-mac",
			IPAddress:  "10.0.10.2",
			MACAddress: "00:50:56:bb:00:01",
		},
	}

	pvlanInfo := []PrivateVLANInfo{}

	// Execute mapper
	globalControls, interfaceSecurity := p.buildNXOSL2SecurityControls(
		dhcpBindings,
		daiStats,
		authSessions,
		portSecEntries,
		ipsgBindings,
		pvlanInfo,
	)

	// Validate global controls
	if !globalControls.DHCPSnoopingEnabled {
		t.Error("Expected DHCP snooping to be enabled")
	}
	if !globalControls.DAIEnabled {
		t.Error("Expected DAI to be enabled")
	}
	if !globalControls.IPSGEnabled {
		t.Error("Expected IPSG to be enabled")
	}
	if !globalControls.Dot1XEnabled {
		t.Error("Expected 802.1X to be enabled")
	}

	t.Logf("NXOS Global Controls: DHCP=%v, DAI=%v, IPSG=%v, Dot1X=%v",
		globalControls.DHCPSnoopingEnabled,
		globalControls.DAIEnabled,
		globalControls.IPSGEnabled,
		globalControls.Dot1XEnabled)

	// Validate interface security for Eth1/1
	if sec, exists := interfaceSecurity["Ethernet1/1"]; exists {
		if sec.DHCPSnoopingBindingCount != 1 {
			t.Errorf("Expected 1 DHCP binding on Eth1/1, got %d", sec.DHCPSnoopingBindingCount)
		}
		if !sec.Dot1XEnabled {
			t.Error("Expected 802.1X to be enabled on Eth1/1")
		}
		if sec.AuthenticationStatus != "Authc Success" {
			t.Errorf("Expected auth status 'Authc Success', got '%s'", sec.AuthenticationStatus)
		}
		if !sec.IPSGEnabled {
			t.Error("Expected IPSG to be enabled on Eth1/1")
		}

		// Security score should be high
		if sec.SecurityScore < 60 {
			t.Errorf("Expected security score >= 60 for Eth1/1, got %d", sec.SecurityScore)
		}

		t.Logf("Eth1/1 Security: Score=%d, Level=%s, DHCP=%d, Auth=%s, IPSG=%v",
			sec.SecurityScore, sec.SecurityLevel, sec.DHCPSnoopingBindingCount,
			sec.AuthenticationStatus, sec.IPSGEnabled)
	} else {
		t.Error("Expected security state for Ethernet1/1")
	}
}

// TestCalculateNXOSInterfaceSecurityScore tests NXOS security scoring
func TestCalculateNXOSInterfaceSecurityScore(t *testing.T) {
	// Same scoring algorithm as IOS - test consistency
	testCases := []struct {
		name          string
		security      CiscoL2InterfaceSecurity
		expectedScore int
	}{
		{
			name: "Full NXOS security stack",
			security: CiscoL2InterfaceSecurity{
				DHCPSnoopingBindingCount: 1,
				AuthenticationStatus:     "Authc Success",
				ARPInspectionTrusted:     true,
				IPSGEnabled:              true,
				IPSGBindingCount:         1,
				PortSecurityEnabled:      true,
				PortSecurityViolationCount: 0,
				PrivateVLANType:          "isolated",
			},
			expectedScore: 100,
		},
		{
			name: "NXOS DHCP + Auth only",
			security: CiscoL2InterfaceSecurity{
				DHCPSnoopingBindingCount: 1,
				AuthenticationStatus:     "Authc Success",
			},
			expectedScore: 50,
		},
		{
			name:          "No security controls",
			security:      CiscoL2InterfaceSecurity{},
			expectedScore: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			score := calculateNXOSInterfaceSecurityScore(tc.security)
			if score != tc.expectedScore {
				t.Errorf("Expected score %d, got %d", tc.expectedScore, score)
			}
			t.Logf("NXOS Test '%s': Score=%d", tc.name, score)
		})
	}
}

// TestBuildNXOSEnhancedFDBEntriesVPC tests VPC-aware confidence scoring
func TestBuildNXOSEnhancedFDBEntriesVPC(t *testing.T) {
	// Create test processor
	p := &CiscoNXOSProcessor{
		nxosConfig: &NXOSConfig{Hostname: "test-nexus-01"},
	}

	// Test data: NXOS MAC table with VPC markers
	macTable := []NXOSMACEntry{
		{
			MACAddress:    "00:50:56:bb:00:01",
			VLAN:          10,
			Port:          "Ethernet1/1",
			Type:          "dynamic",
			Age:           300,
			IsVPCPeerLink: true,  // VPC peer-link marker
			IsPrimary:     true,  // Primary entry marker
		},
		{
			MACAddress:    "00:50:56:bb:00:02",
			VLAN:          10,
			Port:          "Ethernet1/2",
			Type:          "dynamic",
			Age:           400,
			IsVPCPeerLink: false,
			IsPrimary:     false,
		},
	}

	// DHCP bindings
	dhcpBindings := []DHCPSnoopingBinding{
		{
			MACAddress:   "00:50:56:bb:00:01",
			IPAddress:    "10.0.10.2",
			LeaseSeconds: 86395,
			Interface:    "Ethernet1/1",
		},
		{
			MACAddress:   "00:50:56:bb:00:02",
			IPAddress:    "10.0.10.3",
			LeaseSeconds: 86395,
			Interface:    "Ethernet1/2",
		},
	}

	// Auth sessions
	authSessions := []AuthenticationSession{
		{
			MACAddress:  "00:50:56:bb:00:01",
			IPv4Address: "10.0.10.2",
			Status:      "Authc Success",
			Method:      "dot1x",
		},
	}

	ipsgBindings := []IPVerifySourceBinding{}
	arpEntries := []NXOSARPEntry{}

	// Execute mapper
	fdbEntries := p.buildNXOSEnhancedFDBEntries(
		macTable,
		dhcpBindings,
		authSessions,
		ipsgBindings,
		arpEntries,
	)

	if len(fdbEntries) != 2 {
		t.Fatalf("Expected 2 FDB entries, got %d", len(fdbEntries))
	}

	// Test VPC Entry 1: VPC peer-link marker should add metadata and confidence
	entry1 := fdbEntries[0]
	if entry1.MAC != "00:50:56:bb:00:01" {
		t.Errorf("Expected MAC 00:50:56:bb:00:01, got %s", entry1.MAC)
	}

	// Check VPC metadata
	if vpcMarker, exists := entry1.Metadata["vpc_peer_link"]; !exists || vpcMarker != "true" {
		t.Error("Expected vpc_peer_link=true in metadata for entry 1")
	}
	if primaryMarker, exists := entry1.Metadata["primary_entry"]; !exists || primaryMarker != "true" {
		t.Error("Expected primary_entry=true in metadata for entry 1")
	}

	// Check evidence sources include VPC
	hasVPCEvidence := false
	for _, source := range entry1.EvidenceSources {
		if source == "vpc-peer-link" {
			hasVPCEvidence = true
			break
		}
	}
	if !hasVPCEvidence {
		t.Error("Expected 'vpc-peer-link' in evidence sources for entry 1")
	}

	// Confidence should include VPC bonus (+5)
	// Expected: 5 (dynamic) + 40 (DHCP) + 35 (auth) + 5 (VPC) = 85
	if entry1.ConfidenceScore < 80 {
		t.Errorf("Expected confidence >= 80 for VPC entry, got %d", entry1.ConfidenceScore)
	}

	t.Logf("VPC Entry 1: MAC=%s, Confidence=%d, Metadata=%v, Sources=%v",
		entry1.MAC, entry1.ConfidenceScore, entry1.Metadata, entry1.EvidenceSources)

	// Test Non-VPC Entry 2: Should not have VPC markers
	entry2 := fdbEntries[1]
	if entry2.MAC != "00:50:56:bb:00:02" {
		t.Errorf("Expected MAC 00:50:56:bb:00:02, got %s", entry2.MAC)
	}

	// Should not have VPC metadata
	if _, exists := entry2.Metadata["vpc_peer_link"]; exists {
		t.Error("Did not expect vpc_peer_link in metadata for entry 2")
	}
	if _, exists := entry2.Metadata["primary_entry"]; exists {
		t.Error("Did not expect primary_entry in metadata for entry 2")
	}

	// Should not have VPC evidence source
	for _, source := range entry2.EvidenceSources {
		if source == "vpc-peer-link" {
			t.Error("Did not expect 'vpc-peer-link' in evidence sources for entry 2")
		}
	}

	t.Logf("Non-VPC Entry 2: MAC=%s, Confidence=%d, Metadata=%v, Sources=%v",
		entry2.MAC, entry2.ConfidenceScore, entry2.Metadata, entry2.EvidenceSources)
}

// TestBuildNXOSL2DecisionCacheVPC tests VPC metadata preservation in decision cache
func TestBuildNXOSL2DecisionCacheVPC(t *testing.T) {
	// Create test processor
	p := &CiscoNXOSProcessor{
		nxosConfig: &NXOSConfig{Hostname: "test-nexus-01"},
	}

	// Test FDB entries with VPC metadata
	fdbEntries := []CiscoFDBEntry{
		{
			DeviceID:        "test-nexus-01",
			MAC:             "00:50:56:bb:00:01",
			IPAddress:       "10.0.10.2",
			VLAN:            10,
			Interface:       "Ethernet1/1",
			Authenticated:   true,
			DHCPSnoopingBound: true,
			ConfidenceScore: 85,
			Confidence:      "high",
			Metadata: map[string]string{
				"vpc_peer_link": "true",
				"primary_entry": "true",
			},
			EvidenceSources: []string{"mac-table", "vpc-peer-link", "dhcp-snooping", "dot1x"},
		},
		{
			DeviceID:        "test-nexus-01",
			MAC:             "00:50:56:bb:00:02",
			IPAddress:       "10.0.10.3",
			VLAN:            10,
			Interface:       "Ethernet1/2",
			Authenticated:   false,
			DHCPSnoopingBound: true,
			ConfidenceScore: 50,
			Confidence:      "medium",
			Metadata:        make(map[string]string), // No VPC metadata
			EvidenceSources: []string{"mac-table", "dhcp-snooping"},
		},
	}

	interfaceSecurity := map[string]CiscoL2InterfaceSecurity{}

	// Execute mapper
	ttlSeconds := 3600
	decisions := p.buildNXOSL2DecisionCache(fdbEntries, interfaceSecurity, ttlSeconds)

	if len(decisions) != 2 {
		t.Fatalf("Expected 2 decision entries, got %d", len(decisions))
	}

	// Test VPC Decision 1: VPC metadata should be preserved
	decision1 := decisions[0]
	if decision1.Decision != "Allowed" {
		t.Errorf("Expected decision 'Allowed' for VPC entry 1, got '%s'", decision1.Decision)
	}

	// Check VPC metadata preservation
	if vpcMarker, exists := decision1.Metadata["vpc_peer_link"]; !exists || vpcMarker != "true" {
		t.Error("Expected vpc_peer_link=true in decision metadata for entry 1")
	}
	if primaryMarker, exists := decision1.Metadata["primary_entry"]; !exists || primaryMarker != "true" {
		t.Error("Expected primary_entry=true in decision metadata for entry 1")
	}

	t.Logf("VPC Decision 1: MAC=%s, Decision=%s, Metadata=%v, Confidence=%d",
		decision1.MAC, decision1.Decision, decision1.Metadata, decision1.DecisionConfidence)

	// Test Non-VPC Decision 2: Should not have VPC metadata
	decision2 := decisions[1]
	if decision2.Decision != "Conditional" {
		t.Errorf("Expected decision 'Conditional' for entry 2, got '%s'", decision2.Decision)
	}

	// Should not have VPC metadata
	if _, exists := decision2.Metadata["vpc_peer_link"]; exists {
		t.Error("Did not expect vpc_peer_link in decision metadata for entry 2")
	}

	t.Logf("Non-VPC Decision 2: MAC=%s, Decision=%s, Metadata=%v, Confidence=%d",
		decision2.MAC, decision2.Decision, decision2.Metadata, decision2.DecisionConfidence)
}

// TestClassifyNXOSFreshnessFromAge tests NXOS freshness classification
func TestClassifyNXOSFreshnessFromAge(t *testing.T) {
	// Same thresholds as IOS - test consistency
	testCases := []struct {
		age               int
		expectedFreshness string
	}{
		{age: 300, expectedFreshness: "fresh"},    // 5 min
		{age: 600, expectedFreshness: "fresh"},    // 10 min
		{age: 1800, expectedFreshness: "stale"},   // 30 min
		{age: 3600, expectedFreshness: "stale"},   // 1 hour
		{age: 7200, expectedFreshness: "old"},     // 2 hours
	}

	for _, tc := range testCases {
		t.Run(string(rune(tc.age)), func(t *testing.T) {
			freshness := classifyNXOSFreshnessFromAge(tc.age)
			if freshness != tc.expectedFreshness {
				t.Errorf("Age %ds: expected freshness '%s', got '%s'",
					tc.age, tc.expectedFreshness, freshness)
			}
		})
	}
}

// TestNXOSIOSConsistency tests that NXOS and IOS use consistent scoring
func TestNXOSIOSConsistency(t *testing.T) {
	// Same security config for IOS and NXOS
	security := CiscoL2InterfaceSecurity{
		DHCPSnoopingBindingCount: 1,
		AuthenticationStatus:     "Authc Success",
		ARPInspectionTrusted:     true,
		IPSGEnabled:              true,
		IPSGBindingCount:         1,
		PortSecurityEnabled:      true,
		PortSecurityViolationCount: 0,
	}

	iosScore := calculateInterfaceSecurityScore(security)
	nxosScore := calculateNXOSInterfaceSecurityScore(security)

	if iosScore != nxosScore {
		t.Errorf("IOS and NXOS scoring inconsistent: IOS=%d, NXOS=%d", iosScore, nxosScore)
	}

	iosLevel := classifySecurityLevel(iosScore)
	nxosLevel := classifyNXOSSecurityLevel(nxosScore)

	if iosLevel != nxosLevel {
		t.Errorf("IOS and NXOS level classification inconsistent: IOS=%s, NXOS=%s", iosLevel, nxosLevel)
	}

	t.Logf("IOS/NXOS Consistency: Score=%d, Level=%s (consistent)", iosScore, iosLevel)
}

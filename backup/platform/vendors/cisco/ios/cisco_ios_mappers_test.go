package cisco

import (
	"testing"
	"time"
)

// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |I|O|S| |L|2| |M|a|p|p|e|r| |U|n|i|t| |T|e|s|t|s|
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+

// TestBuildL2SecurityControls tests the L2 security controls mapper
func TestBuildL2SecurityControls(t *testing.T) {
	// Create test processor
	p := &CiscoIOSProcessor{
		iosConfig: &IOSConfig{Hostname: "test-switch-01"},
	}

	// Test data: DHCP snooping bindings
	dhcpBindings := []DHCPSnoopingBinding{
		{
			MACAddress:   "00:50:56:bb:00:01",
			IPAddress:    "10.0.10.2",
			LeaseSeconds: 86395,
			Type:         "dhcp-snooping",
			VLAN:         10,
			Interface:    "GigabitEthernet1/0/1",
		},
		{
			MACAddress:   "00:50:56:bb:00:02",
			IPAddress:    "10.0.10.3",
			LeaseSeconds: 86395,
			Type:         "dhcp-snooping",
			VLAN:         10,
			Interface:    "GigabitEthernet1/0/2",
		},
	}

	// Test data: DAI statistics
	daiStats := ARPInspectionStatistics{
		Forwarded: 1250,
		Dropped:   15,
	}

	// Test data: Authentication sessions
	authSessions := []AuthenticationSession{
		{
			Interface:   "GigabitEthernet1/0/1",
			MACAddress:  "00:50:56:bb:00:01",
			IPv4Address: "10.0.10.2",
			Status:      "Authc Success",
			Domain:      "DATA",
			Method:      "dot1x",
		},
	}

	// Test data: Port security
	portSecEntries := []PortSecurityEntry{
		{
			Interface:               "GigabitEthernet1/0/3",
			MaximumMAC:              5,
			TotalMAC:                2,
			SecurityViolation:       "Shutdown",
			SecurityViolationCount:  0,
		},
	}

	// Test data: IPSG bindings
	ipsgBindings := []IPVerifySourceBinding{
		{
			Interface:  "GigabitEthernet1/0/1",
			Filter:     "ip-mac",
			IPAddress:  "10.0.10.2",
			MACAddress: "00:50:56:bb:00:01",
		},
	}

	// Test data: PVLAN info
	pvlanInfo := []PrivateVLANInfo{}

	// Execute mapper
	globalControls, interfaceSecurity := p.buildL2SecurityControls(
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

	t.Logf("Global Controls: DHCP=%v, DAI=%v, IPSG=%v, Dot1X=%v",
		globalControls.DHCPSnoopingEnabled,
		globalControls.DAIEnabled,
		globalControls.IPSGEnabled,
		globalControls.Dot1XEnabled)

	// Validate interface security for Gi1/0/1 (has DHCP + auth + IPSG)
	if sec, exists := interfaceSecurity["GigabitEthernet1/0/1"]; exists {
		if sec.DHCPSnoopingBindingCount != 1 {
			t.Errorf("Expected 1 DHCP binding on Gi1/0/1, got %d", sec.DHCPSnoopingBindingCount)
		}
		if !sec.Dot1XEnabled {
			t.Error("Expected 802.1X to be enabled on Gi1/0/1")
		}
		if sec.AuthenticationStatus != "Authc Success" {
			t.Errorf("Expected auth status 'Authc Success', got '%s'", sec.AuthenticationStatus)
		}
		if sec.AuthenticationMethod != "dot1x" {
			t.Errorf("Expected auth method 'dot1x', got '%s'", sec.AuthenticationMethod)
		}
		if !sec.IPSGEnabled {
			t.Error("Expected IPSG to be enabled on Gi1/0/1")
		}
		if sec.IPSGFilterType != "ip-mac" {
			t.Errorf("Expected IPSG filter type 'ip-mac', got '%s'", sec.IPSGFilterType)
		}

		// Security score should be high (DHCP=20 + auth success=30 + IPSG=15 = 65+)
		if sec.SecurityScore < 60 {
			t.Errorf("Expected security score >= 60 for Gi1/0/1, got %d", sec.SecurityScore)
		}
		if sec.SecurityLevel != "medium" && sec.SecurityLevel != "high" {
			t.Errorf("Expected security level medium/high for Gi1/0/1, got '%s'", sec.SecurityLevel)
		}

		t.Logf("Gi1/0/1 Security: Score=%d, Level=%s, DHCP=%d bindings, Auth=%s, IPSG=%v",
			sec.SecurityScore, sec.SecurityLevel, sec.DHCPSnoopingBindingCount,
			sec.AuthenticationStatus, sec.IPSGEnabled)
	} else {
		t.Error("Expected security state for GigabitEthernet1/0/1")
	}

	// Validate interface security for Gi1/0/3 (has port security)
	if sec, exists := interfaceSecurity["GigabitEthernet1/0/3"]; exists {
		if !sec.PortSecurityEnabled {
			t.Error("Expected port security to be enabled on Gi1/0/3")
		}
		if sec.PortSecurityMaxMAC != 5 {
			t.Errorf("Expected max MAC 5, got %d", sec.PortSecurityMaxMAC)
		}
		if sec.PortSecurityCurrentMAC != 2 {
			t.Errorf("Expected current MAC 2, got %d", sec.PortSecurityCurrentMAC)
		}

		t.Logf("Gi1/0/3 Security: Score=%d, Level=%s, PortSec=%v (max=%d, current=%d)",
			sec.SecurityScore, sec.SecurityLevel, sec.PortSecurityEnabled,
			sec.PortSecurityMaxMAC, sec.PortSecurityCurrentMAC)
	} else {
		t.Error("Expected security state for GigabitEthernet1/0/3")
	}

	t.Logf("Total interfaces with security state: %d", len(interfaceSecurity))
}

// TestCalculateInterfaceSecurityScore tests the security scoring algorithm
func TestCalculateInterfaceSecurityScore(t *testing.T) {
	testCases := []struct {
		name          string
		security      CiscoL2InterfaceSecurity
		expectedScore int
		expectedLevel string
	}{
		{
			name: "Full security stack (max score)",
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
			expectedScore: 100, // 20+30+15+15+10+10 = 100
			expectedLevel: "high",
		},
		{
			name: "DHCP + Auth only",
			security: CiscoL2InterfaceSecurity{
				DHCPSnoopingBindingCount: 1,
				AuthenticationStatus:     "Authc Success",
			},
			expectedScore: 50, // 20+30 = 50
			expectedLevel: "medium",
		},
		{
			name: "DHCP + Auth enabled but not successful",
			security: CiscoL2InterfaceSecurity{
				DHCPSnoopingBindingCount: 1,
				Dot1XEnabled:             true,
				AuthenticationStatus:     "Running",
			},
			expectedScore: 30, // 20+10 = 30
			expectedLevel: "low",
		},
		{
			name: "Port security only",
			security: CiscoL2InterfaceSecurity{
				PortSecurityEnabled:        true,
				PortSecurityViolationCount: 0,
			},
			expectedScore: 10, // 10
			expectedLevel: "none",
		},
		{
			name: "Port security with violations",
			security: CiscoL2InterfaceSecurity{
				PortSecurityEnabled:        true,
				PortSecurityViolationCount: 5,
			},
			expectedScore: 0, // 0 (violations = no credit)
			expectedLevel: "none",
		},
		{
			name:          "No security controls",
			security:      CiscoL2InterfaceSecurity{},
			expectedScore: 0,
			expectedLevel: "none",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			score := calculateInterfaceSecurityScore(tc.security)
			level := classifySecurityLevel(score)

			if score != tc.expectedScore {
				t.Errorf("Expected score %d, got %d", tc.expectedScore, score)
			}
			if level != tc.expectedLevel {
				t.Errorf("Expected level '%s', got '%s'", tc.expectedLevel, level)
			}

			t.Logf("Test '%s': Score=%d, Level=%s", tc.name, score, level)
		})
	}
}

// TestBuildEnhancedFDBEntries tests FDB confidence scoring
func TestBuildEnhancedFDBEntries(t *testing.T) {
	// Create test processor
	p := &CiscoIOSProcessor{
		iosConfig: &IOSConfig{Hostname: "test-switch-01"},
	}

	// Test data: MAC table
	macTable := []models.MACTableEntry{
		{
			MACAddress: "00:50:56:bb:00:01",
			VLAN:       10,
			Port:       "GigabitEthernet1/0/1",
			Type:       "dynamic",
			Age:        300, // 5 minutes = fresh
		},
		{
			MACAddress: "00:50:56:bb:00:02",
			VLAN:       10,
			Port:       "GigabitEthernet1/0/2",
			Type:       "dynamic",
			Age:        3600, // 1 hour = stale
		},
		{
			MACAddress: "aa:bb:cc:dd:ee:ff",
			VLAN:       20,
			Port:       "GigabitEthernet1/0/3",
			Type:       "dynamic",
			Age:        7200, // 2 hours = old
		},
	}

	// Test data: DHCP snooping bindings
	dhcpBindings := []DHCPSnoopingBinding{
		{
			MACAddress:   "00:50:56:bb:00:01",
			IPAddress:    "10.0.10.2",
			LeaseSeconds: 86395,
			Type:         "dhcp-snooping",
			VLAN:         10,
			Interface:    "GigabitEthernet1/0/1",
		},
		{
			MACAddress:   "00:50:56:bb:00:02",
			IPAddress:    "10.0.10.3",
			LeaseSeconds: 86395,
			Type:         "dhcp-snooping",
			VLAN:         10,
			Interface:    "GigabitEthernet1/0/2",
		},
	}

	// Test data: Authentication sessions
	authSessions := []AuthenticationSession{
		{
			Interface:   "GigabitEthernet1/0/1",
			MACAddress:  "00:50:56:bb:00:01",
			IPv4Address: "10.0.10.2",
			Status:      "Authc Success",
			Method:      "dot1x",
		},
	}

	// Test data: IPSG bindings
	ipsgBindings := []IPVerifySourceBinding{
		{
			Interface:  "GigabitEthernet1/0/1",
			Filter:     "ip-mac",
			IPAddress:  "10.0.10.2",
			MACAddress: "00:50:56:bb:00:01",
		},
	}

	// Test data: ARP entries
	arpEntries := []models.ARPEntry{
		{
			IPAddress:  "10.0.10.2",
			MACAddress: "00:50:56:bb:00:01",
			Interface:  "GigabitEthernet1/0/1",
		},
		{
			IPAddress:  "10.0.10.3",
			MACAddress: "00:50:56:bb:00:02",
			Interface:  "GigabitEthernet1/0/2",
		},
	}

	// Execute mapper
	fdbEntries := p.buildEnhancedFDBEntries(
		macTable,
		dhcpBindings,
		authSessions,
		ipsgBindings,
		arpEntries,
	)

	if len(fdbEntries) != 3 {
		t.Fatalf("Expected 3 FDB entries, got %d", len(fdbEntries))
	}

	// Test Entry 1: Full security stack (MAC table + DHCP + Auth + IPSG + ARP)
	// Expected: 5 (dynamic) + 40 (DHCP) + 35 (auth success) + 15 (IPSG) + 10 (ARP) = 105 → 100 (capped)
	entry1 := fdbEntries[0]
	if entry1.MAC != "00:50:56:bb:00:01" {
		t.Errorf("Expected MAC 00:50:56:bb:00:01, got %s", entry1.MAC)
	}
	if !entry1.DHCPSnoopingBound {
		t.Error("Expected DHCP snooping bound for entry 1")
	}
	if !entry1.Authenticated {
		t.Error("Expected authenticated for entry 1")
	}
	if !entry1.IPSGBound {
		t.Error("Expected IPSG bound for entry 1")
	}
	if entry1.IPAddress != "10.0.10.2" {
		t.Errorf("Expected IP 10.0.10.2, got %s", entry1.IPAddress)
	}

	// Should have high confidence (>= 70)
	if entry1.ConfidenceScore < 70 {
		t.Errorf("Expected confidence >= 70 for entry 1 (full security), got %d", entry1.ConfidenceScore)
	}
	if entry1.Confidence != "high" {
		t.Errorf("Expected high confidence for entry 1, got '%s'", entry1.Confidence)
	}

	// Should be fresh
	if entry1.Freshness != "fresh" {
		t.Errorf("Expected fresh freshness, got '%s'", entry1.Freshness)
	}

	// Should have 5 evidence sources
	expectedSources := 5 // mac-table, dhcp-snooping, dot1x, ipsg, arp
	if len(entry1.EvidenceSources) != expectedSources {
		t.Errorf("Expected %d evidence sources for entry 1, got %d: %v",
			expectedSources, len(entry1.EvidenceSources), entry1.EvidenceSources)
	}

	t.Logf("Entry 1: MAC=%s, Confidence=%s (%d), Freshness=%s, Sources=%v, IP=%s",
		entry1.MAC, entry1.Confidence, entry1.ConfidenceScore,
		entry1.Freshness, entry1.EvidenceSources, entry1.IPAddress)

	// Test Entry 2: DHCP + ARP only (no auth, no IPSG)
	// Expected: 5 (dynamic) + 40 (DHCP) + 10 (ARP) = 55
	entry2 := fdbEntries[1]
	if entry2.MAC != "00:50:56:bb:00:02" {
		t.Errorf("Expected MAC 00:50:56:bb:00:02, got %s", entry2.MAC)
	}
	if !entry2.DHCPSnoopingBound {
		t.Error("Expected DHCP snooping bound for entry 2")
	}
	if entry2.Authenticated {
		t.Error("Expected not authenticated for entry 2")
	}

	// Should have medium confidence (40-69)
	if entry2.ConfidenceScore < 40 || entry2.ConfidenceScore >= 70 {
		t.Errorf("Expected confidence 40-69 for entry 2, got %d", entry2.ConfidenceScore)
	}
	if entry2.Confidence != "medium" {
		t.Errorf("Expected medium confidence for entry 2, got '%s'", entry2.Confidence)
	}

	// Should be stale (3600s = 1 hour)
	if entry2.Freshness != "stale" {
		t.Errorf("Expected stale freshness, got '%s'", entry2.Freshness)
	}

	t.Logf("Entry 2: MAC=%s, Confidence=%s (%d), Freshness=%s, Sources=%v",
		entry2.MAC, entry2.Confidence, entry2.ConfidenceScore,
		entry2.Freshness, entry2.EvidenceSources)

	// Test Entry 3: MAC table only (no DHCP, no auth, no IPSG, no ARP)
	// Expected: 5 (dynamic) = 5
	entry3 := fdbEntries[2]
	if entry3.MAC != "aa:bb:cc:dd:ee:ff" {
		t.Errorf("Expected MAC aa:bb:cc:dd:ee:ff, got %s", entry3.MAC)
	}
	if entry3.DHCPSnoopingBound {
		t.Error("Expected DHCP snooping not bound for entry 3")
	}
	if entry3.Authenticated {
		t.Error("Expected not authenticated for entry 3")
	}

	// Should have low confidence (< 20)
	if entry3.ConfidenceScore >= 20 {
		t.Errorf("Expected confidence < 20 for entry 3, got %d", entry3.ConfidenceScore)
	}
	if entry3.Confidence != "low" {
		t.Errorf("Expected low confidence for entry 3, got '%s'", entry3.Confidence)
	}

	// Should be old (7200s = 2 hours)
	if entry3.Freshness != "old" {
		t.Errorf("Expected old freshness, got '%s'", entry3.Freshness)
	}

	// Should have 1 evidence source (mac-table only)
	if len(entry3.EvidenceSources) != 1 {
		t.Errorf("Expected 1 evidence source for entry 3, got %d: %v",
			len(entry3.EvidenceSources), entry3.EvidenceSources)
	}

	t.Logf("Entry 3: MAC=%s, Confidence=%s (%d), Freshness=%s, Sources=%v",
		entry3.MAC, entry3.Confidence, entry3.ConfidenceScore,
		entry3.Freshness, entry3.EvidenceSources)
}

// TestBuildL2DecisionCache tests L2 decision logic
func TestBuildL2DecisionCache(t *testing.T) {
	// Create test processor
	p := &CiscoIOSProcessor{
		iosConfig: &IOSConfig{Hostname: "test-switch-01"},
	}

	// Test FDB entries with different confidence levels
	fdbEntries := []CiscoFDBEntry{
		{
			DeviceID:        "test-switch-01",
			MAC:             "00:50:56:bb:00:01",
			IPAddress:       "10.0.10.2",
			VLAN:            10,
			Interface:       "GigabitEthernet1/0/1",
			Authenticated:   true,
			DHCPSnoopingBound: true,
			ConfidenceScore: 75, // High confidence
			Confidence:      "high",
			EvidenceSources: []string{"mac-table", "dhcp-snooping", "dot1x", "ipsg"},
		},
		{
			DeviceID:        "test-switch-01",
			MAC:             "00:50:56:bb:00:02",
			IPAddress:       "10.0.10.3",
			VLAN:            10,
			Interface:       "GigabitEthernet1/0/2",
			Authenticated:   true,
			DHCPSnoopingBound: false, // Auth but no DHCP
			ConfidenceScore: 60,
			Confidence:      "medium",
			EvidenceSources: []string{"mac-table", "dot1x"},
		},
		{
			DeviceID:        "test-switch-01",
			MAC:             "00:50:56:bb:00:03",
			IPAddress:       "10.0.10.4",
			VLAN:            10,
			Interface:       "GigabitEthernet1/0/3",
			Authenticated:   false,
			DHCPSnoopingBound: true, // DHCP but no auth
			ConfidenceScore: 50,
			Confidence:      "medium",
			EvidenceSources: []string{"mac-table", "dhcp-snooping"},
		},
		{
			DeviceID:        "test-switch-01",
			MAC:             "aa:bb:cc:dd:ee:ff",
			IPAddress:       "",
			VLAN:            20,
			Interface:       "GigabitEthernet1/0/4",
			Authenticated:   false,
			DHCPSnoopingBound: false,
			ConfidenceScore: 5, // Low confidence
			Confidence:      "low",
			EvidenceSources: []string{"mac-table"},
		},
	}

	// Interface security state
	interfaceSecurity := map[string]CiscoL2InterfaceSecurity{
		"GigabitEthernet1/0/1": {
			SecurityScore: 90,
			SecurityLevel: "high",
		},
		"GigabitEthernet1/0/4": {
			SecurityScore:              10,
			SecurityLevel:              "none",
			PortSecurityViolationCount: 5, // Has violations
		},
	}

	// Execute mapper with 1 hour TTL
	ttlSeconds := 3600
	decisions := p.buildL2DecisionCache(fdbEntries, interfaceSecurity, ttlSeconds)

	if len(decisions) != 4 {
		t.Fatalf("Expected 4 decision entries, got %d", len(decisions))
	}

	// Test Decision 1: ALLOWED (auth + DHCP + high confidence)
	decision1 := decisions[0]
	if decision1.Decision != "Allowed" {
		t.Errorf("Expected decision 'Allowed' for entry 1, got '%s'", decision1.Decision)
	}
	if !decision1.Authenticated {
		t.Error("Expected authenticated for decision 1")
	}
	if !decision1.DHCPSnoopingBound {
		t.Error("Expected DHCP bound for decision 1")
	}
	if decision1.DecisionConfidence != 75 {
		t.Errorf("Expected confidence 75, got %d", decision1.DecisionConfidence)
	}

	// Verify TTL
	cachedTime, err := time.Parse(time.RFC3339, decision1.CachedAt)
	if err != nil {
		t.Errorf("Failed to parse cached time: %v", err)
	}
	expiresTime, err := time.Parse(time.RFC3339, decision1.ExpiresAt)
	if err != nil {
		t.Errorf("Failed to parse expires time: %v", err)
	}
	expectedExpiry := cachedTime.Add(time.Duration(ttlSeconds) * time.Second)
	if expiresTime.Sub(expectedExpiry).Abs() > time.Second {
		t.Errorf("Expected expiry time ~%v, got %v", expectedExpiry, expiresTime)
	}

	t.Logf("Decision 1: MAC=%s, Decision=%s, Reason='%s', Confidence=%d, TTL=%ds",
		decision1.MAC, decision1.Decision, decision1.DecisionReason,
		decision1.DecisionConfidence, decision1.TTLSeconds)

	// Test Decision 2: CONDITIONAL (auth but no DHCP)
	decision2 := decisions[1]
	if decision2.Decision != "Conditional" {
		t.Errorf("Expected decision 'Conditional' for entry 2, got '%s'", decision2.Decision)
	}
	if !decision2.Authenticated {
		t.Error("Expected authenticated for decision 2")
	}
	if decision2.DHCPSnoopingBound {
		t.Error("Expected DHCP not bound for decision 2")
	}

	t.Logf("Decision 2: MAC=%s, Decision=%s, Reason='%s', Confidence=%d",
		decision2.MAC, decision2.Decision, decision2.DecisionReason,
		decision2.DecisionConfidence)

	// Test Decision 3: CONDITIONAL (DHCP but no auth)
	decision3 := decisions[2]
	if decision3.Decision != "Conditional" {
		t.Errorf("Expected decision 'Conditional' for entry 3, got '%s'", decision3.Decision)
	}
	if decision3.Authenticated {
		t.Error("Expected not authenticated for decision 3")
	}
	if !decision3.DHCPSnoopingBound {
		t.Error("Expected DHCP bound for decision 3")
	}

	t.Logf("Decision 3: MAC=%s, Decision=%s, Reason='%s', Confidence=%d",
		decision3.MAC, decision3.Decision, decision3.DecisionReason,
		decision3.DecisionConfidence)

	// Test Decision 4: BLOCKED (low confidence)
	decision4 := decisions[3]
	if decision4.Decision != "Blocked" {
		t.Errorf("Expected decision 'Blocked' for entry 4, got '%s'", decision4.Decision)
	}
	if decision4.Authenticated {
		t.Error("Expected not authenticated for decision 4")
	}
	if decision4.DHCPSnoopingBound {
		t.Error("Expected DHCP not bound for decision 4")
	}

	// Decision reason should mention violations (from interface security)
	if decision4.DecisionReason == "" {
		t.Error("Expected decision reason for blocked entry")
	}

	t.Logf("Decision 4: MAC=%s, Decision=%s, Reason='%s', Confidence=%d",
		decision4.MAC, decision4.Decision, decision4.DecisionReason,
		decision4.DecisionConfidence)
}

// TestClassifyFreshnessFromAge tests freshness classification
func TestClassifyFreshnessFromAge(t *testing.T) {
	testCases := []struct {
		age               int
		expectedFreshness string
	}{
		{age: 0, expectedFreshness: "fresh"},
		{age: 300, expectedFreshness: "fresh"},    // 5 min
		{age: 600, expectedFreshness: "fresh"},    // 10 min
		{age: 601, expectedFreshness: "stale"},    // 10 min 1s
		{age: 1800, expectedFreshness: "stale"},   // 30 min
		{age: 3600, expectedFreshness: "stale"},   // 1 hour
		{age: 3601, expectedFreshness: "old"},     // 1 hour 1s
		{age: 7200, expectedFreshness: "old"},     // 2 hours
		{age: 86400, expectedFreshness: "old"},    // 1 day
	}

	for _, tc := range testCases {
		t.Run(string(rune(tc.age)), func(t *testing.T) {
			freshness := classifyFreshnessFromAge(tc.age)
			if freshness != tc.expectedFreshness {
				t.Errorf("Age %ds: expected freshness '%s', got '%s'",
					tc.age, tc.expectedFreshness, freshness)
			}
		})
	}
}

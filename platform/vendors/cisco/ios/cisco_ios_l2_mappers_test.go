package cisco

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattpainter701/graphlayer/platform/models"
)

// TestBuildL2SecurityControls tests the L2 security controls mapping function.
func TestBuildL2SecurityControls(t *testing.T) {
	tests := []struct {
		name                  string
		dhcpBindings          []DHCPSnoopingBinding
		daiStats              ARPInspectionStatistics
		authSessions          []AuthenticationSession
		portSecEntries        []PortSecurityEntry
		ipsgBindings          []IPVerifySourceBinding
		pvlanInfo             []PrivateVLANInfo
		expectedGlobalEnabled map[string]bool
		expectedIfaceCount    int
		expectedIfaceSecurity map[string]int // interface → expected security score
	}{
		{
			name: "Full security stack - high confidence",
			dhcpBindings: []DHCPSnoopingBinding{
				{
					MACAddress:   "00:11:22:33:44:55",
					IPAddress:    "192.168.1.10",
					VLANId:       100,
					Interface:    "GigabitEthernet1/0/1",
					LeaseSeconds: 86400,
					Type:         "dhcp-snooping",
					CollectedAt:  time.Now(),
				},
			},
			daiStats: ARPInspectionStatistics{
				VLANId:      100,
				Forwarded:   1000,
				Dropped:     5,
				DHCPDrops:   3,
				ACLDrops:    2,
				CollectedAt: time.Now(),
			},
			authSessions: []AuthenticationSession{
				{
					MACAddress:  "00:11:22:33:44:55",
					Interface:   "GigabitEthernet1/0/1",
					Status:      "Authc Success",
					Method:      "dot1x",
					IPv4Address: "192.168.1.10",
					VLAN:        100,
					CollectedAt: time.Now(),
				},
			},
			portSecEntries: []PortSecurityEntry{
				{
					Interface:              "GigabitEthernet1/0/1",
					MaximumMAC:             2,
					TotalMAC:               1,
					SecurityViolation:      "Shutdown",
					SecurityViolationCount: 0,
					CollectedAt:            time.Now(),
				},
			},
			ipsgBindings: []IPVerifySourceBinding{
				{
					Interface: "GigabitEthernet1/0/1",
					Filter:    "ip-mac",
					IPAddress: "192.168.1.10",
					MACAddress: "00:11:22:33:44:55",
					VLAN:      100,
				},
			},
			pvlanInfo: []PrivateVLANInfo{},
			expectedGlobalEnabled: map[string]bool{
				"dhcp": true,
				"dai":  true,
				"ipsg": true,
				"dot1x": true,
			},
			expectedIfaceCount: 1,
			expectedIfaceSecurity: map[string]int{
				"GigabitEthernet1/0/1": 70, // High security score
			},
		},
		{
			name: "DHCP snooping only - medium confidence",
			dhcpBindings: []DHCPSnoopingBinding{
				{
					MACAddress:   "aa:bb:cc:dd:ee:ff",
					IPAddress:    "10.0.0.100",
					VLANId:       200,
					Interface:    "GigabitEthernet1/0/2",
					LeaseSeconds: 3600,
					Type:         "dhcp-snooping",
					CollectedAt:  time.Now(),
				},
			},
			daiStats:       ARPInspectionStatistics{},
			authSessions:   []AuthenticationSession{},
			portSecEntries: []PortSecurityEntry{},
			ipsgBindings:   []IPVerifySourceBinding{},
			pvlanInfo:      []PrivateVLANInfo{},
			expectedGlobalEnabled: map[string]bool{
				"dhcp": true,
				"dai":  false,
				"ipsg": false,
				"dot1x": false,
			},
			expectedIfaceCount: 1,
			expectedIfaceSecurity: map[string]int{
				"GigabitEthernet1/0/2": 20, // Only DHCP snooping
			},
		},
		{
			name:           "No security controls - low confidence",
			dhcpBindings:   []DHCPSnoopingBinding{},
			daiStats:       ARPInspectionStatistics{},
			authSessions:   []AuthenticationSession{},
			portSecEntries: []PortSecurityEntry{},
			ipsgBindings:   []IPVerifySourceBinding{},
			pvlanInfo:      []PrivateVLANInfo{},
			expectedGlobalEnabled: map[string]bool{
				"dhcp": false,
				"dai":  false,
				"ipsg": false,
				"dot1x": false,
			},
			expectedIfaceCount:    0,
			expectedIfaceSecurity: map[string]int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock processor
			p := &CiscoIOSProcessor{}

			// Call the function under test
			globalControls, ifaceSecurity := p.buildL2SecurityControls(
				tt.dhcpBindings,
				tt.daiStats,
				tt.authSessions,
				tt.portSecEntries,
				tt.ipsgBindings,
				tt.pvlanInfo,
			)

			// Verify global controls
			assert.Equal(t, tt.expectedGlobalEnabled["dhcp"], globalControls.DHCPSnoopingEnabled,
				"DHCP snooping enabled mismatch")
			assert.Equal(t, tt.expectedGlobalEnabled["dai"], globalControls.DAIEnabled,
				"DAI enabled mismatch")
			assert.Equal(t, tt.expectedGlobalEnabled["ipsg"], globalControls.IPSGEnabled,
				"IPSG enabled mismatch")
			assert.Equal(t, tt.expectedGlobalEnabled["dot1x"], globalControls.Dot1XEnabled,
				"802.1X enabled mismatch")

			// Verify interface count
			assert.Equal(t, tt.expectedIfaceCount, len(ifaceSecurity),
				"Interface count mismatch")

			// Verify interface security scores
			for iface, expectedScore := range tt.expectedIfaceSecurity {
				sec, exists := ifaceSecurity[iface]
				require.True(t, exists, "Interface %s not found in security map", iface)
				assert.GreaterOrEqual(t, sec.SecurityScore, expectedScore-10,
					"Security score for %s too low (expected ~%d, got %d)", iface, expectedScore, sec.SecurityScore)
				assert.LessOrEqual(t, sec.SecurityScore, expectedScore+10,
					"Security score for %s too high (expected ~%d, got %d)", iface, expectedScore, sec.SecurityScore)
			}
		})
	}
}

// TestCalculateInterfaceSecurityScore tests the security score calculation algorithm.
func TestCalculateInterfaceSecurityScore(t *testing.T) {
	tests := []struct {
		name          string
		security      CiscoL2InterfaceSecurity
		expectedScore int
		expectedLevel string
	}{
		{
			name: "Maximum security - all controls enabled and successful",
			security: CiscoL2InterfaceSecurity{
				DHCPSnoopingBindingCount:   2,
				Dot1XEnabled:               true,
				AuthenticationStatus:       "Authc Success",
				ARPInspectionTrusted:       true,
				IPSGEnabled:                true,
				IPSGBindingCount:           2,
				PortSecurityEnabled:        true,
				PortSecurityViolationCount: 0,
				PrivateVLANType:            "isolated",
			},
			expectedScore: 100,
			expectedLevel: "high",
		},
		{
			name: "High security - auth + DHCP + DAI",
			security: CiscoL2InterfaceSecurity{
				DHCPSnoopingBindingCount: 1,
				Dot1XEnabled:             true,
				AuthenticationStatus:     "Authorized",
				ARPInspectionTrusted:     true,
			},
			expectedScore: 65,
			expectedLevel: "medium",
		},
		{
			name: "Medium security - DHCP + port security",
			security: CiscoL2InterfaceSecurity{
				DHCPSnoopingBindingCount:   1,
				PortSecurityEnabled:        true,
				PortSecurityViolationCount: 0,
			},
			expectedScore: 30,
			expectedLevel: "low",
		},
		{
			name: "Low security - only auth enabled but not successful",
			security: CiscoL2InterfaceSecurity{
				Dot1XEnabled:         true,
				AuthenticationStatus: "Authc Failed",
			},
			expectedScore: 10,
			expectedLevel: "none",
		},
		{
			name:          "No security - no controls enabled",
			security:      CiscoL2InterfaceSecurity{},
			expectedScore: 0,
			expectedLevel: "none",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := calculateInterfaceSecurityScore(tt.security)
			level := classifySecurityLevel(score)

			assert.Equal(t, tt.expectedScore, score,
				"Security score mismatch")
			assert.Equal(t, tt.expectedLevel, level,
				"Security level mismatch")
		})
	}
}

// TestBuildEnhancedFDBEntries tests the FDB confidence scoring algorithm.
func TestBuildEnhancedFDBEntries(t *testing.T) {
	tests := []struct {
		name                string
		macTable            []models.MACTableEntry
		dhcpBindings        []DHCPSnoopingBinding
		authSessions        []AuthenticationSession
		ipsgBindings        []IPVerifySourceBinding
		arpEntries          []models.ARPEntry
		expectedFDBCount    int
		expectedConfidence  map[string]string // MAC → confidence level
		expectedEvidence    map[string]int    // MAC → evidence source count
	}{
		{
			name: "High confidence - DHCP + Auth + IPSG + ARP",
			macTable: []models.MACTableEntry{
				{
					MACAddress: "00:11:22:33:44:55",
					VLAN:       100,
					Port:       "Gi1/0/1",
					Type:       "dynamic",
					Age:        300,
				},
			},
			dhcpBindings: []DHCPSnoopingBinding{
				{
					MACAddress:  "00:11:22:33:44:55",
					IPAddress:   "192.168.1.10",
					VLANId:      100,
					Interface:   "Gi1/0/1",
					CollectedAt: time.Now(),
				},
			},
			authSessions: []AuthenticationSession{
				{
					MACAddress:  "00:11:22:33:44:55",
					Interface:   "Gi1/0/1",
					Status:      "Authc Success",
					Method:      "dot1x",
					IPv4Address: "192.168.1.10",
				},
			},
			ipsgBindings: []IPVerifySourceBinding{
				{
					Interface:  "Gi1/0/1",
					MACAddress: "00:11:22:33:44:55",
					IPAddress:  "192.168.1.10",
				},
			},
			arpEntries: []models.ARPEntry{
				{
					MACAddress: "00:11:22:33:44:55",
					IPAddress:  "192.168.1.10",
					Interface:  "Gi1/0/1",
				},
			},
			expectedFDBCount: 1,
			expectedConfidence: map[string]string{
				"00:11:22:33:44:55": "high", // 40 (DHCP) + 35 (auth) + 15 (IPSG) + 10 (ARP) + 5 (fresh) = 105 → capped at 100
			},
			expectedEvidence: map[string]int{
				"00:11:22:33:44:55": 5, // mac-table, dhcp-snooping, dot1x, ipsg, arp
			},
		},
		{
			name: "Medium confidence - DHCP only",
			macTable: []models.MACTableEntry{
				{
					MACAddress: "aa:bb:cc:dd:ee:ff",
					VLAN:       200,
					Port:       "Gi1/0/2",
					Type:       "dynamic",
					Age:        600,
				},
			},
			dhcpBindings: []DHCPSnoopingBinding{
				{
					MACAddress:  "aa:bb:cc:dd:ee:ff",
					IPAddress:   "10.0.0.100",
					VLANId:      200,
					Interface:   "Gi1/0/2",
					CollectedAt: time.Now(),
				},
			},
			authSessions:     []AuthenticationSession{},
			ipsgBindings:     []IPVerifySourceBinding{},
			arpEntries:       []models.ARPEntry{},
			expectedFDBCount: 1,
			expectedConfidence: map[string]string{
				"aa:bb:cc:dd:ee:ff": "medium", // 40 (DHCP) + 5 (fresh) = 45
			},
			expectedEvidence: map[string]int{
				"aa:bb:cc:dd:ee:ff": 2, // mac-table, dhcp-snooping
			},
		},
		{
			name: "Low confidence - MAC table only",
			macTable: []models.MACTableEntry{
				{
					MACAddress: "11:22:33:44:55:66",
					VLAN:       300,
					Port:       "Gi1/0/3",
					Type:       "dynamic",
					Age:        3600,
				},
			},
			dhcpBindings:     []DHCPSnoopingBinding{},
			authSessions:     []AuthenticationSession{},
			ipsgBindings:     []IPVerifySourceBinding{},
			arpEntries:       []models.ARPEntry{},
			expectedFDBCount: 1,
			expectedConfidence: map[string]string{
				"11:22:33:44:55:66": "low", // 0 (no fresh MAC) = 0
			},
			expectedEvidence: map[string]int{
				"11:22:33:44:55:66": 1, // mac-table only
			},
		},
		{
			name: "Secure MAC - port security learned",
			macTable: []models.MACTableEntry{
				{
					MACAddress: "77:88:99:aa:bb:cc",
					VLAN:       400,
					Port:       "Gi1/0/4",
					Type:       "secure",
					Age:        0,
				},
			},
			dhcpBindings:     []DHCPSnoopingBinding{},
			authSessions:     []AuthenticationSession{},
			ipsgBindings:     []IPVerifySourceBinding{},
			arpEntries:       []models.ARPEntry{},
			expectedFDBCount: 1,
			expectedConfidence: map[string]string{
				"77:88:99:aa:bb:cc": "low", // 20 (secure MAC) = 20
			},
			expectedEvidence: map[string]int{
				"77:88:99:aa:bb:cc": 2, // mac-table, port-security
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock processor
			p := &CiscoIOSProcessor{
				iosConfig: &CiscoIOSConfig{
					Hostname: "test-switch",
				},
			}

			// Call the function under test
			fdbEntries := p.buildEnhancedFDBEntries(
				tt.macTable,
				tt.dhcpBindings,
				tt.authSessions,
				tt.ipsgBindings,
				tt.arpEntries,
			)

			// Verify FDB entry count
			assert.Equal(t, tt.expectedFDBCount, len(fdbEntries),
				"FDB entry count mismatch")

			// Verify confidence levels and evidence sources
			for _, fdb := range fdbEntries {
				expectedConf, exists := tt.expectedConfidence[fdb.MAC]
				require.True(t, exists, "Unexpected MAC in FDB: %s", fdb.MAC)

				assert.Equal(t, expectedConf, fdb.Confidence,
					"Confidence level mismatch for MAC %s", fdb.MAC)

				expectedEvidenceCount := tt.expectedEvidence[fdb.MAC]
				assert.Equal(t, expectedEvidenceCount, len(fdb.EvidenceSources),
					"Evidence source count mismatch for MAC %s (expected %d, got %d: %v)",
					fdb.MAC, expectedEvidenceCount, len(fdb.EvidenceSources), fdb.EvidenceSources)
			}
		})
	}
}

// TestClassifyFreshnessFromAge tests the freshness classification algorithm.
func TestClassifyFreshnessFromAge(t *testing.T) {
	tests := []struct {
		name               string
		ageSeconds         int
		expectedFreshness  string
	}{
		{"Fresh - 5 minutes", 300, "fresh"},
		{"Fresh - 10 minutes (boundary)", 600, "fresh"},
		{"Stale - 15 minutes", 900, "stale"},
		{"Stale - 30 minutes", 1800, "stale"},
		{"Stale - 1 hour (boundary)", 3600, "stale"},
		{"Old - 2 hours", 7200, "old"},
		{"Old - 1 day", 86400, "old"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			freshness := classifyFreshnessFromAge(tt.ageSeconds)
			assert.Equal(t, tt.expectedFreshness, freshness,
				"Freshness classification mismatch for age %d seconds", tt.ageSeconds)
		})
	}
}

// TestClassifyFreshness tests the timestamp-based freshness classification.
func TestClassifyFreshness(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name              string
		timestamp         string
		expectedFreshness string
	}{
		{
			name:              "Fresh - 5 minutes ago",
			timestamp:         now.Add(-5 * time.Minute).Format(time.RFC3339),
			expectedFreshness: "fresh",
		},
		{
			name:              "Fresh - 10 minutes ago (boundary)",
			timestamp:         now.Add(-10 * time.Minute).Format(time.RFC3339),
			expectedFreshness: "fresh",
		},
		{
			name:              "Stale - 30 minutes ago",
			timestamp:         now.Add(-30 * time.Minute).Format(time.RFC3339),
			expectedFreshness: "stale",
		},
		{
			name:              "Stale - 1 hour ago (boundary)",
			timestamp:         now.Add(-1 * time.Hour).Format(time.RFC3339),
			expectedFreshness: "stale",
		},
		{
			name:              "Old - 2 hours ago",
			timestamp:         now.Add(-2 * time.Hour).Format(time.RFC3339),
			expectedFreshness: "old",
		},
		{
			name:              "Old - 1 day ago",
			timestamp:         now.Add(-24 * time.Hour).Format(time.RFC3339),
			expectedFreshness: "old",
		},
		{
			name:              "Unknown - empty timestamp",
			timestamp:         "",
			expectedFreshness: "unknown",
		},
		{
			name:              "Unknown - invalid timestamp",
			timestamp:         "invalid-timestamp",
			expectedFreshness: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			freshness := classifyFreshness(tt.timestamp)
			assert.Equal(t, tt.expectedFreshness, freshness,
				"Freshness classification mismatch")
		})
	}
}

// TestBuildL2DecisionCache tests the L2 decision cache building.
func TestBuildL2DecisionCache(t *testing.T) {
	tests := []struct {
		name                  string
		fdbEntries            []CiscoFDBEntry
		interfaceSecurity     map[string]CiscoL2InterfaceSecurity
		ttlSeconds            int
		expectedDecisionCount int
		expectedOutcomes      map[string]string // MAC → expected outcome
		expectedReasons       map[string]string // MAC → expected reason substring
	}{
		{
			name: "Allowed - authenticated + DHCP bound + high confidence",
			fdbEntries: []CiscoFDBEntry{
				{
					DeviceID:         "switch1",
					MAC:              "00:11:22:33:44:55",
					VLAN:             100,
					Interface:        "Gi1/0/1",
					Authenticated:    true,
					DHCPSnoopingBound: true,
					ConfidenceScore:  85,
					Confidence:       "high",
				},
			},
			interfaceSecurity: map[string]CiscoL2InterfaceSecurity{
				"Gi1/0/1": {
					SecurityScore: 80,
					SecurityLevel: "high",
				},
			},
			ttlSeconds:            300,
			expectedDecisionCount: 1,
			expectedOutcomes: map[string]string{
				"00:11:22:33:44:55": "Allowed",
			},
			expectedReasons: map[string]string{
				"00:11:22:33:44:55": "Authenticated",
			},
		},
		{
			name: "Conditional - authenticated but no DHCP",
			fdbEntries: []CiscoFDBEntry{
				{
					DeviceID:         "switch1",
					MAC:              "aa:bb:cc:dd:ee:ff",
					VLAN:             200,
					Interface:        "Gi1/0/2",
					Authenticated:    true,
					DHCPSnoopingBound: false,
					ConfidenceScore:  60,
					Confidence:       "medium",
				},
			},
			interfaceSecurity: map[string]CiscoL2InterfaceSecurity{
				"Gi1/0/2": {
					SecurityScore: 50,
					SecurityLevel: "medium",
				},
			},
			ttlSeconds:            300,
			expectedDecisionCount: 1,
			expectedOutcomes: map[string]string{
				"aa:bb:cc:dd:ee:ff": "Conditional",
			},
			expectedReasons: map[string]string{
				"aa:bb:cc:dd:ee:ff": "missing DHCP",
			},
		},
		{
			name: "Blocked - low confidence",
			fdbEntries: []CiscoFDBEntry{
				{
					DeviceID:         "switch1",
					MAC:              "11:22:33:44:55:66",
					VLAN:             300,
					Interface:        "Gi1/0/3",
					Authenticated:    false,
					DHCPSnoopingBound: false,
					ConfidenceScore:  15,
					Confidence:       "low",
				},
			},
			interfaceSecurity: map[string]CiscoL2InterfaceSecurity{
				"Gi1/0/3": {
					SecurityScore: 10,
					SecurityLevel: "low",
				},
			},
			ttlSeconds:            300,
			expectedDecisionCount: 1,
			expectedOutcomes: map[string]string{
				"11:22:33:44:55:66": "Blocked",
			},
			expectedReasons: map[string]string{
				"11:22:33:44:55:66": "Low confidence",
			},
		},
		{
			name: "Blocked - port security violations",
			fdbEntries: []CiscoFDBEntry{
				{
					DeviceID:         "switch1",
					MAC:              "77:88:99:aa:bb:cc",
					VLAN:             400,
					Interface:        "Gi1/0/4",
					Authenticated:    false,
					DHCPSnoopingBound: false,
					ConfidenceScore:  50,
					Confidence:       "medium",
				},
			},
			interfaceSecurity: map[string]CiscoL2InterfaceSecurity{
				"Gi1/0/4": {
					PortSecurityEnabled:        true,
					PortSecurityViolationCount: 5,
					SecurityScore:              30,
					SecurityLevel:              "low",
				},
			},
			ttlSeconds:            300,
			expectedDecisionCount: 1,
			expectedOutcomes: map[string]string{
				"77:88:99:aa:bb:cc": "Blocked",
			},
			expectedReasons: map[string]string{
				"77:88:99:aa:bb:cc": "violations",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock processor
			p := &CiscoIOSProcessor{
				iosConfig: &CiscoIOSConfig{
					Hostname: "test-switch",
				},
			}

			// Call the function under test
			decisions := p.buildL2DecisionCache(
				tt.fdbEntries,
				tt.interfaceSecurity,
				tt.ttlSeconds,
			)

			// Verify decision count
			assert.Equal(t, tt.expectedDecisionCount, len(decisions),
				"Decision count mismatch")

			// Verify outcomes and reasons
			for _, decision := range decisions {
				expectedOutcome, exists := tt.expectedOutcomes[decision.MAC]
				require.True(t, exists, "Unexpected MAC in decisions: %s", decision.MAC)

				assert.Equal(t, expectedOutcome, decision.Decision,
					"Decision outcome mismatch for MAC %s", decision.MAC)

				expectedReason := tt.expectedReasons[decision.MAC]
				assert.Contains(t, decision.DecisionReason, expectedReason,
					"Decision reason mismatch for MAC %s (expected substring: %s, got: %s)",
					decision.MAC, expectedReason, decision.DecisionReason)

				// Verify TTL is set correctly
				expectedExpiry := time.Now().Add(time.Duration(tt.ttlSeconds) * time.Second)
				parsedExpiry, err := time.Parse(time.RFC3339, decision.ExpiresAt)
				require.NoError(t, err, "Failed to parse ExpiresAt timestamp")

				assert.WithinDuration(t, expectedExpiry, parsedExpiry, 5*time.Second,
					"ExpiresAt timestamp outside expected range")
			}
		})
	}
}

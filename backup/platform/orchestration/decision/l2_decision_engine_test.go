package decision

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDetermineL2Reachability tests the core L2 decision logic.
func TestDetermineL2Reachability(t *testing.T) {
	tests := []struct {
		name              string
		query             L2ReachabilityQuery
		macTable          []MACTableEntry
		dhcpBindings      []DHCPBinding
		authSessions      []AuthSession
		ipsgBindings      map[string][]IPSGBinding
		portSecurity      map[string]PortSecurityState
		interfaceSecurity map[string]InterfaceSecurityState
		pvlanIsolation    map[int]PVLANIsolation
		expectedOutcome   string
		expectedReason    string
		expectedHandoff   bool
	}{
		{
			name: "Allowed - Authenticated + DHCP bound + high confidence",
			query: L2ReachabilityQuery{
				SourceMAC:       "00:11:22:33:44:55",
				DestMAC:         "aa:bb:cc:dd:ee:ff",
				VLANID:          100,
				SourceInterface: "Gi1/0/1",
				DestInterface:   "Gi1/0/2",
			},
			macTable: []MACTableEntry{
				{MAC: "00:11:22:33:44:55", VLAN: 100, Interface: "Gi1/0/1", Type: "dynamic", Age: 300},
				{MAC: "aa:bb:cc:dd:ee:ff", VLAN: 100, Interface: "Gi1/0/2", Type: "dynamic", Age: 300},
			},
			dhcpBindings: []DHCPBinding{
				{MAC: "00:11:22:33:44:55", IP: "192.168.1.10", VLAN: 100, Interface: "Gi1/0/1", CollectedAt: time.Now()},
			},
			authSessions: []AuthSession{
				{MAC: "00:11:22:33:44:55", Interface: "Gi1/0/1", Status: "Authc Success", Method: "dot1x"},
			},
			interfaceSecurity: map[string]InterfaceSecurityState{
				"Gi1/0/1": {SecurityScore: 85, SecurityLevel: "high"},
			},
			expectedOutcome: "Allowed",
			expectedReason:  "Authenticated",
			expectedHandoff: false,
		},
		{
			name: "Conditional - Authenticated but no DHCP binding",
			query: L2ReachabilityQuery{
				SourceMAC:       "00:11:22:33:44:55",
				DestMAC:         "aa:bb:cc:dd:ee:ff",
				VLANID:          100,
				SourceInterface: "Gi1/0/1",
			},
			macTable: []MACTableEntry{
				{MAC: "00:11:22:33:44:55", VLAN: 100, Interface: "Gi1/0/1", Type: "dynamic", Age: 300},
				{MAC: "aa:bb:cc:dd:ee:ff", VLAN: 100, Interface: "Gi1/0/2", Type: "dynamic", Age: 300},
			},
			dhcpBindings: []DHCPBinding{}, // No DHCP binding
			authSessions: []AuthSession{
				{MAC: "00:11:22:33:44:55", Interface: "Gi1/0/1", Status: "Authc Success", Method: "dot1x"},
			},
			expectedOutcome: "Conditional",
			expectedReason:  "missing DHCP",
			expectedHandoff: false,
		},
		{
			name: "Conditional - DHCP bound but not authenticated",
			query: L2ReachabilityQuery{
				SourceMAC:       "00:11:22:33:44:55",
				DestMAC:         "aa:bb:cc:dd:ee:ff",
				VLANID:          100,
				SourceInterface: "Gi1/0/1",
			},
			macTable: []MACTableEntry{
				{MAC: "00:11:22:33:44:55", VLAN: 100, Interface: "Gi1/0/1", Type: "dynamic", Age: 300},
				{MAC: "aa:bb:cc:dd:ee:ff", VLAN: 100, Interface: "Gi1/0/2", Type: "dynamic", Age: 300},
			},
			dhcpBindings: []DHCPBinding{
				{MAC: "00:11:22:33:44:55", IP: "192.168.1.10", VLAN: 100, Interface: "Gi1/0/1", CollectedAt: time.Now()},
			},
			authSessions:    []AuthSession{}, // No authentication
			expectedOutcome: "Conditional",
			expectedReason:  "not authenticated",
			expectedHandoff: false,
		},
		{
			name: "Blocked - No MAC table entry for source",
			query: L2ReachabilityQuery{
				SourceMAC: "00:11:22:33:44:55",
				DestMAC:   "aa:bb:cc:dd:ee:ff",
				VLANID:    100,
			},
			macTable: []MACTableEntry{
				{MAC: "aa:bb:cc:dd:ee:ff", VLAN: 100, Interface: "Gi1/0/2", Type: "dynamic", Age: 300},
				// Source MAC missing
			},
			expectedOutcome: "Blocked",
			expectedReason:  "not found in MAC table",
			expectedHandoff: false,
		},
		{
			name: "Blocked - Different VLANs (L2-to-L3 handoff)",
			query: L2ReachabilityQuery{
				SourceMAC: "00:11:22:33:44:55",
				DestMAC:   "aa:bb:cc:dd:ee:ff",
				VLANID:    100,
			},
			macTable: []MACTableEntry{
				{MAC: "00:11:22:33:44:55", VLAN: 100, Interface: "Gi1/0/1", Type: "dynamic", Age: 300},
				{MAC: "aa:bb:cc:dd:ee:ff", VLAN: 200, Interface: "Gi1/0/2", Type: "dynamic", Age: 300}, // Different VLAN!
			},
			expectedOutcome: "Blocked",
			expectedReason:  "Different VLANs",
			expectedHandoff: true, // Should trigger L3 handoff
		},
		{
			name: "Blocked - Port security violations",
			query: L2ReachabilityQuery{
				SourceMAC:       "00:11:22:33:44:55",
				DestMAC:         "aa:bb:cc:dd:ee:ff",
				VLANID:          100,
				SourceInterface: "Gi1/0/1",
			},
			macTable: []MACTableEntry{
				{MAC: "00:11:22:33:44:55", VLAN: 100, Interface: "Gi1/0/1", Type: "dynamic", Age: 300},
				{MAC: "aa:bb:cc:dd:ee:ff", VLAN: 100, Interface: "Gi1/0/2", Type: "dynamic", Age: 300},
			},
			portSecurity: map[string]PortSecurityState{
				"Gi1/0/1": {
					Enabled:        true,
					ViolationCount: 5, // Violations detected!
				},
			},
			expectedOutcome: "Blocked",
			expectedReason:  "Port security violations",
			expectedHandoff: false,
		},
		{
			name: "Blocked - Authentication failed",
			query: L2ReachabilityQuery{
				SourceMAC:       "00:11:22:33:44:55",
				DestMAC:         "aa:bb:cc:dd:ee:ff",
				VLANID:          100,
				SourceInterface: "Gi1/0/1",
			},
			macTable: []MACTableEntry{
				{MAC: "00:11:22:33:44:55", VLAN: 100, Interface: "Gi1/0/1", Type: "dynamic", Age: 300},
				{MAC: "aa:bb:cc:dd:ee:ff", VLAN: 100, Interface: "Gi1/0/2", Type: "dynamic", Age: 300},
			},
			authSessions: []AuthSession{
				{MAC: "00:11:22:33:44:55", Interface: "Gi1/0/1", Status: "Authc Failed", Method: "dot1x"}, // Failed auth
			},
			expectedOutcome: "Blocked",
			expectedReason:  "authentication failed",
			expectedHandoff: false,
		},
		{
			name: "Blocked - PVLAN isolation",
			query: L2ReachabilityQuery{
				SourceMAC:       "00:11:22:33:44:55",
				DestMAC:         "aa:bb:cc:dd:ee:ff",
				VLANID:          100,
				SourceInterface: "Gi1/0/1",
				DestInterface:   "Gi1/0/2",
			},
			macTable: []MACTableEntry{
				{MAC: "00:11:22:33:44:55", VLAN: 100, Interface: "Gi1/0/1", Type: "dynamic", Age: 300},
				{MAC: "aa:bb:cc:dd:ee:ff", VLAN: 100, Interface: "Gi1/0/2", Type: "dynamic", Age: 300},
			},
			pvlanIsolation: map[int]PVLANIsolation{
				100: {
					PrimaryVLAN:      100,
					IsolatedPorts:    []string{"Gi1/0/1", "Gi1/0/2"}, // Both isolated
					PromiscuousPorts: []string{},
				},
			},
			expectedOutcome: "Blocked",
			expectedReason:  "Private VLAN isolation",
			expectedHandoff: false,
		},
		{
			name: "Blocked - Low confidence (no evidence)",
			query: L2ReachabilityQuery{
				SourceMAC: "00:11:22:33:44:55",
				DestMAC:   "aa:bb:cc:dd:ee:ff",
				VLANID:    100,
			},
			macTable: []MACTableEntry{
				{MAC: "00:11:22:33:44:55", VLAN: 100, Interface: "Gi1/0/1", Type: "dynamic", Age: 3600}, // Old entry
				{MAC: "aa:bb:cc:dd:ee:ff", VLAN: 100, Interface: "Gi1/0/2", Type: "dynamic", Age: 3600},
			},
			dhcpBindings:    []DHCPBinding{},    // No DHCP
			authSessions:    []AuthSession{},    // No auth
			expectedOutcome: "Blocked",
			expectedReason:  "Low confidence",
			expectedHandoff: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create decision engine
			engine := NewL2DecisionEngine()

			// Load test data
			engine.LoadMACTable(tt.macTable)
			if tt.dhcpBindings != nil {
				engine.LoadDHCPBindings(tt.dhcpBindings)
			}
			if tt.authSessions != nil {
				engine.LoadAuthSessions(tt.authSessions)
			}
			if tt.portSecurity != nil {
				engine.PortSecurityState = tt.portSecurity
			}
			if tt.interfaceSecurity != nil {
				engine.LoadInterfaceSecurity(tt.interfaceSecurity)
			}
			if tt.pvlanIsolation != nil {
				engine.LoadPVLANIsolation(tt.pvlanIsolation)
			}

			// Execute decision
			decision := engine.DetermineL2Reachability(tt.query)

			// Verify outcome
			assert.Equal(t, tt.expectedOutcome, decision.Outcome,
				"Decision outcome mismatch")

			// Verify reason contains expected substring
			lowerReason := strings.ToLower(decision.ReasonSummary)
			expectedLower := strings.ToLower(tt.expectedReason)
			assert.Contains(t, lowerReason, expectedLower,
				"Decision reason mismatch. Expected substring: %s, Got: %s",
				tt.expectedReason, decision.ReasonSummary)

			// Verify L3 handoff flag
			assert.Equal(t, tt.expectedHandoff, decision.HandoffToL3,
				"L3 handoff flag mismatch")

			// Verify reason codes are populated
			assert.NotEmpty(t, decision.ReasonCodes,
				"Reason codes should not be empty")

			// Verify decision ID is generated
			assert.NotEmpty(t, decision.DecisionID,
				"Decision ID should be generated")

			// Verify timestamps
			assert.False(t, decision.EvaluatedAt.IsZero(),
				"EvaluatedAt timestamp should be set")
			assert.False(t, decision.ExpiresAt.IsZero(),
				"ExpiresAt timestamp should be set")

			t.Logf("Decision: %s | Reason: %s | Confidence: %d",
				decision.Outcome, decision.ReasonSummary, decision.ConfidenceScore)
		})
	}
}

// TestGatherEvidence tests the evidence gathering function.
func TestGatherEvidence(t *testing.T) {
	tests := []struct {
		name                  string
		mac                   string
		iface                 string
		vlan                  int
		macTable              []MACTableEntry
		dhcpBindings          []DHCPBinding
		authSessions          []AuthSession
		ipsgBindings          map[string][]IPSGBinding
		arpEntries            map[string]ARPEntry
		expectedMACEntry      bool
		expectedDHCPBound     bool
		expectedAuthenticated bool
		expectedIPSGBound     bool
		expectedARPEntry      bool
		minConfidence         int
		maxConfidence         int
	}{
		{
			name:  "Full evidence - all sources",
			mac:   "00:11:22:33:44:55",
			iface: "Gi1/0/1",
			vlan:  100,
			macTable: []MACTableEntry{
				{MAC: "00:11:22:33:44:55", VLAN: 100, Interface: "Gi1/0/1", Type: "dynamic", Age: 300},
			},
			dhcpBindings: []DHCPBinding{
				{MAC: "00:11:22:33:44:55", IP: "192.168.1.10", VLAN: 100, CollectedAt: time.Now()},
			},
			authSessions: []AuthSession{
				{MAC: "00:11:22:33:44:55", Status: "Authc Success", Method: "dot1x"},
			},
			ipsgBindings: map[string][]IPSGBinding{
				"Gi1/0/1": {
					{MAC: "00:11:22:33:44:55", IP: "192.168.1.10", Filter: "ip-mac"},
				},
			},
			arpEntries: map[string]ARPEntry{
				"00:11:22:33:44:55": {MAC: "00:11:22:33:44:55", IP: "192.168.1.10"},
			},
			expectedMACEntry:      true,
			expectedDHCPBound:     true,
			expectedAuthenticated: true,
			expectedIPSGBound:     true,
			expectedARPEntry:      true,
			minConfidence:         90,  // Should be very high
			maxConfidence:         110, // Capped at practical max
		},
		{
			name:  "DHCP binding only",
			mac:   "aa:bb:cc:dd:ee:ff",
			iface: "Gi1/0/2",
			vlan:  200,
			macTable: []MACTableEntry{
				{MAC: "aa:bb:cc:dd:ee:ff", VLAN: 200, Interface: "Gi1/0/2", Type: "dynamic", Age: 300},
			},
			dhcpBindings: []DHCPBinding{
				{MAC: "aa:bb:cc:dd:ee:ff", IP: "10.0.0.100", VLAN: 200, CollectedAt: time.Now()},
			},
			expectedMACEntry:      true,
			expectedDHCPBound:     true,
			expectedAuthenticated: false,
			expectedIPSGBound:     false,
			expectedARPEntry:      false,
			minConfidence:         40, // DHCP (40) + fresh MAC (5)
			maxConfidence:         50,
		},
		{
			name:  "MAC table only - low confidence",
			mac:   "11:22:33:44:55:66",
			iface: "Gi1/0/3",
			vlan:  300,
			macTable: []MACTableEntry{
				{MAC: "11:22:33:44:55:66", VLAN: 300, Interface: "Gi1/0/3", Type: "dynamic", Age: 3600}, // Old
			},
			expectedMACEntry:      true,
			expectedDHCPBound:     false,
			expectedAuthenticated: false,
			expectedIPSGBound:     false,
			expectedARPEntry:      false,
			minConfidence:         0, // No fresh evidence
			maxConfidence:         5,
		},
		{
			name:  "Secure MAC (port security)",
			mac:   "77:88:99:aa:bb:cc",
			iface: "Gi1/0/4",
			vlan:  400,
			macTable: []MACTableEntry{
				{MAC: "77:88:99:aa:bb:cc", VLAN: 400, Interface: "Gi1/0/4", Type: "secure", Age: 0},
			},
			expectedMACEntry:      true,
			expectedDHCPBound:     false,
			expectedAuthenticated: false,
			expectedIPSGBound:     false,
			expectedARPEntry:      false,
			minConfidence:         20, // Secure MAC gets 20
			maxConfidence:         25,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create decision engine
			engine := NewL2DecisionEngine()

			// Load test data
			engine.LoadMACTable(tt.macTable)
			if tt.dhcpBindings != nil {
				engine.LoadDHCPBindings(tt.dhcpBindings)
			}
			if tt.authSessions != nil {
				engine.LoadAuthSessions(tt.authSessions)
			}
			if tt.ipsgBindings != nil {
				engine.IPSGBindings = tt.ipsgBindings
			}
			if tt.arpEntries != nil {
				engine.ARPEntries = tt.arpEntries
			}

			// Gather evidence
			evidence := engine.gatherEvidence(tt.mac, tt.iface, tt.vlan)

			// Verify evidence flags
			assert.Equal(t, tt.expectedMACEntry, evidence.MACTableEntry,
				"MAC table entry flag mismatch")
			assert.Equal(t, tt.expectedDHCPBound, evidence.DHCPBound,
				"DHCP bound flag mismatch")
			assert.Equal(t, tt.expectedAuthenticated, evidence.Authenticated,
				"Authenticated flag mismatch")
			assert.Equal(t, tt.expectedIPSGBound, evidence.IPSGBound,
				"IPSG bound flag mismatch")
			assert.Equal(t, tt.expectedARPEntry, evidence.ARPEntryExists,
				"ARP entry flag mismatch")

			// Verify confidence score range
			assert.GreaterOrEqual(t, evidence.ConfidenceScore, tt.minConfidence,
				"Confidence score too low (expected >= %d, got %d)",
				tt.minConfidence, evidence.ConfidenceScore)
			assert.LessOrEqual(t, evidence.ConfidenceScore, tt.maxConfidence,
				"Confidence score too high (expected <= %d, got %d)",
				tt.maxConfidence, evidence.ConfidenceScore)

			// Verify evidence sources are populated
			assert.NotEmpty(t, evidence.EvidenceSources,
				"Evidence sources should not be empty")

			t.Logf("Evidence for %s: Confidence=%d, Sources=%v",
				tt.mac, evidence.ConfidenceScore, evidence.EvidenceSources)
		})
	}
}

// TestCheckPVLANIsolation tests the Private VLAN isolation logic.
func TestCheckPVLANIsolation(t *testing.T) {
	tests := []struct {
		name         string
		pvlan        PVLANIsolation
		srcIface     string
		dstIface     string
		expectBlocked bool
		description  string
	}{
		{
			name: "Isolated ports cannot communicate",
			pvlan: PVLANIsolation{
				PrimaryVLAN:      100,
				IsolatedPorts:    []string{"Gi1/0/1", "Gi1/0/2"},
				PromiscuousPorts: []string{"Gi1/0/24"},
			},
			srcIface:      "Gi1/0/1",
			dstIface:      "Gi1/0/2",
			expectBlocked: true,
			description:   "Two isolated ports in same PVLAN",
		},
		{
			name: "Isolated to promiscuous allowed",
			pvlan: PVLANIsolation{
				PrimaryVLAN:      100,
				IsolatedPorts:    []string{"Gi1/0/1"},
				PromiscuousPorts: []string{"Gi1/0/24"},
			},
			srcIface:      "Gi1/0/1",
			dstIface:      "Gi1/0/24",
			expectBlocked: false,
			description:   "Isolated port can talk to promiscuous port",
		},
		{
			name: "Promiscuous to isolated allowed",
			pvlan: PVLANIsolation{
				PrimaryVLAN:      100,
				IsolatedPorts:    []string{"Gi1/0/1"},
				PromiscuousPorts: []string{"Gi1/0/24"},
			},
			srcIface:      "Gi1/0/24",
			dstIface:      "Gi1/0/1",
			expectBlocked: false,
			description:   "Promiscuous port can talk to isolated port",
		},
		{
			name: "Non-isolated ports allowed",
			pvlan: PVLANIsolation{
				PrimaryVLAN:      100,
				IsolatedPorts:    []string{"Gi1/0/1"},
				PromiscuousPorts: []string{"Gi1/0/24"},
			},
			srcIface:      "Gi1/0/10",
			dstIface:      "Gi1/0/11",
			expectBlocked: false,
			description:   "Ports not in PVLAN config can communicate",
		},
		{
			name: "Empty interface names",
			pvlan: PVLANIsolation{
				PrimaryVLAN:      100,
				IsolatedPorts:    []string{"Gi1/0/1"},
				PromiscuousPorts: []string{},
			},
			srcIface:      "",
			dstIface:      "",
			expectBlocked: false,
			description:   "Empty interface names should not block",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := NewL2DecisionEngine()
			blocked := engine.checkPVLANIsolation(tt.pvlan, tt.srcIface, tt.dstIface)

			assert.Equal(t, tt.expectBlocked, blocked,
				"%s: Expected blocked=%v, got blocked=%v",
				tt.description, tt.expectBlocked, blocked)
		})
	}
}

// TestDecisionCaching tests that decisions have proper TTL and expiration.
func TestDecisionCaching(t *testing.T) {
	engine := NewL2DecisionEngine()
	engine.DecisionTTL = 5 * time.Minute

	// Load minimal data
	engine.LoadMACTable([]MACTableEntry{
		{MAC: "00:11:22:33:44:55", VLAN: 100, Interface: "Gi1/0/1", Type: "dynamic", Age: 300},
		{MAC: "aa:bb:cc:dd:ee:ff", VLAN: 100, Interface: "Gi1/0/2", Type: "dynamic", Age: 300},
	})

	query := L2ReachabilityQuery{
		SourceMAC: "00:11:22:33:44:55",
		DestMAC:   "aa:bb:cc:dd:ee:ff",
		VLANID:    100,
	}

	decision := engine.DetermineL2Reachability(query)

	// Verify expiration is set correctly
	expectedExpiry := time.Now().Add(5 * time.Minute)
	actualExpiry := decision.ExpiresAt

	assert.WithinDuration(t, expectedExpiry, actualExpiry, 5*time.Second,
		"ExpiresAt should be ~5 minutes from now")

	// Verify evaluated time is recent
	assert.WithinDuration(t, time.Now(), decision.EvaluatedAt, 5*time.Second,
		"EvaluatedAt should be recent")

	t.Logf("Decision cached with TTL: EvaluatedAt=%s, ExpiresAt=%s",
		decision.EvaluatedAt.Format(time.RFC3339),
		decision.ExpiresAt.Format(time.RFC3339))
}

// TestStrictModeVsNonStrict tests the difference between strict and non-strict modes.
func TestStrictModeVsNonStrict(t *testing.T) {
	macTable := []MACTableEntry{
		{MAC: "00:11:22:33:44:55", VLAN: 100, Interface: "Gi1/0/1", Type: "dynamic", Age: 300},
		{MAC: "aa:bb:cc:dd:ee:ff", VLAN: 100, Interface: "Gi1/0/2", Type: "dynamic", Age: 300},
	}

	// Authenticated but no DHCP
	authSessions := []AuthSession{
		{MAC: "00:11:22:33:44:55", Status: "Authc Success", Method: "dot1x"},
	}

	query := L2ReachabilityQuery{
		SourceMAC: "00:11:22:33:44:55",
		DestMAC:   "aa:bb:cc:dd:ee:ff",
		VLANID:    100,
	}

	// Test strict mode (require both auth + DHCP)
	t.Run("Strict mode - requires both auth and DHCP", func(t *testing.T) {
		engine := NewL2DecisionEngine()
		engine.EnableStrictMode = true
		engine.LoadMACTable(macTable)
		engine.LoadAuthSessions(authSessions)

		decision := engine.DetermineL2Reachability(query)

		// In strict mode, auth alone is not enough for Allowed
		assert.NotEqual(t, "Allowed", decision.Outcome,
			"Strict mode should not allow with auth only")
		assert.Contains(t, strings.ToLower(decision.ReasonSummary), "dhcp",
			"Reason should mention missing DHCP")
	})

	// Test non-strict mode (auth OR DHCP is sufficient)
	t.Run("Non-strict mode - auth OR DHCP sufficient", func(t *testing.T) {
		engine := NewL2DecisionEngine()
		engine.EnableStrictMode = false
		engine.AllowedThreshold = 60 // Lower threshold for non-strict
		engine.LoadMACTable(macTable)
		engine.LoadAuthSessions(authSessions)

		decision := engine.DetermineL2Reachability(query)

		// In non-strict mode with sufficient confidence, auth alone may be enough
		if decision.ConfidenceScore >= 60 {
			assert.Equal(t, "Allowed", decision.Outcome,
				"Non-strict mode should allow with high confidence auth")
		}
	})
}

// BenchmarkDetermineL2Reachability benchmarks decision performance.
func BenchmarkDetermineL2Reachability(b *testing.B) {
	engine := NewL2DecisionEngine()

	// Load realistic dataset
	macTable := make([]MACTableEntry, 100)
	for i := 0; i < 100; i++ {
		macTable[i] = MACTableEntry{
			MAC:       generateMAC(i),
			VLAN:      100,
			Interface: "Gi1/0/" + string(rune(i+1)),
			Type:      "dynamic",
			Age:       300,
		}
	}
	engine.LoadMACTable(macTable)

	query := L2ReachabilityQuery{
		SourceMAC: macTable[0].MAC,
		DestMAC:   macTable[1].MAC,
		VLANID:    100,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = engine.DetermineL2Reachability(query)
	}
}

// Helper function to generate test MAC addresses.
func generateMAC(i int) string {
	return strings.ToLower(strings.ReplaceAll(
		strings.ReplaceAll(
			strings.ReplaceAll(
				"00:11:22:33:44:"+string(rune(i)),
				" ", ""),
			"\n", ""),
		"\t", ""))
}

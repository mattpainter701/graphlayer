package decision

import (
	"testing"

	"github.com/mattpainter701/graphlayer/platform/models"
	asaparsers "github.com/mattpainter701/graphlayer/platform/vendors/cisco/asa"
	fortigateparsers "github.com/mattpainter701/graphlayer/platform/vendors/fortinet/fortigate"
	paloaltoparsers "github.com/mattpainter701/graphlayer/platform/vendors/paloalto/firewall"
	"github.com/stretchr/testify/assert"
)

// TestCiscoASAFirewallDecision tests Cisco ASA firewall policy evaluation
func TestCiscoASAFirewallDecision(t *testing.T) {
	engine := NewL4FirewallDecisionEngine(true, true, false)

	// Load sample Cisco ASA ACLs
	aclEntries := []asaparsers.ASAACLEntry{
		{
			ACLName:        "outside_in",
			LineNumber:     10,
			Action:         "permit",
			Protocol:       "tcp",
			SourceAddress:  "any",
			SourceMask:     "",
			SourcePort:     "",
			DestAddress:    "10.1.1.100",
			DestMask:       "255.255.255.255",
			DestPort:       "443",
			DestPortOperator: "eq",
			LogEnabled:     true,
		},
		{
			ACLName:        "outside_in",
			LineNumber:     20,
			Action:         "deny",
			Protocol:       "tcp",
			SourceAddress:  "any",
			SourceMask:     "",
			SourcePort:     "",
			DestAddress:    "10.1.1.0",
			DestMask:       "255.255.255.0",
			DestPort:       "23",
			DestPortOperator: "eq",
			LogEnabled:     true,
		},
	}

	engine.LoadCiscoASAPolicies(aclEntries)

	// Test Case 1: Permitted HTTPS traffic
	flow1 := TrafficFlow{
		SourceIP:   "203.0.113.50",
		SourcePort: 54321,
		DestIP:     "10.1.1.100",
		DestPort:   443,
		Protocol:   "tcp",
		DeviceID:   "asa-01",
	}

	decision1, err := engine.EvaluateFlow(flow1)
	assert.NoError(t, err)
	assert.Equal(t, "permit", decision1.Action)
	assert.Equal(t, "cisco_asa", decision1.Vendor)
	assert.Equal(t, "outside_in", decision1.MatchedRule)
	assert.True(t, decision1.LogEnabled)
	assert.NotEmpty(t, decision1.Explanation)

	// Test Case 2: Denied Telnet traffic
	flow2 := TrafficFlow{
		SourceIP:   "203.0.113.75",
		SourcePort: 55555,
		DestIP:     "10.1.1.50",
		DestPort:   23,
		Protocol:   "tcp",
		DeviceID:   "asa-01",
	}

	decision2, err := engine.EvaluateFlow(flow2)
	assert.NoError(t, err)
	assert.Equal(t, "deny", decision2.Action)
	assert.Equal(t, 1, decision2.RuleIndex) // Second ACE (index 1)

	// Test Case 3: Implicit deny (no matching ACE)
	flow3 := TrafficFlow{
		SourceIP:   "203.0.113.100",
		SourcePort: 12345,
		DestIP:     "10.2.2.50",
		DestPort:   80,
		Protocol:   "tcp",
		DeviceID:   "asa-01",
	}

	decision3, err := engine.EvaluateFlow(flow3)
	assert.NoError(t, err)
	assert.Equal(t, "deny", decision3.Action)
	assert.Contains(t, decision3.Reason, "Implicit deny")
}

// TestFortiGateFirewallDecision tests FortiGate firewall policy evaluation
func TestFortiGateFirewallDecision(t *testing.T) {
	engine := NewL4FirewallDecisionEngine(true, true, false)

	// Load sample FortiGate policies
	policies := []fortigateparsers.FortiGateFirewallPolicy{
		{
			PolicyID:         1,
			Name:             "Allow-Internet",
			Status:           "enable",
			Action:           "accept",
			SourceInterfaces: []string{"internal"},
			DestInterfaces:   []string{"wan1"},
			SourceAddresses:  []string{"Internal-Subnet"},
			DestAddresses:    []string{"all"},
			Services:         []string{"HTTP", "HTTPS"},
			NAT:              true,
			LogTraffic:       "all",
			IPSProfile:       "default",
			AVProfile:        "default",
		},
		{
			PolicyID:         2,
			Name:             "Block-P2P",
			Status:           "enable",
			Action:           "deny",
			SourceInterfaces: []string{"internal"},
			DestInterfaces:   []string{"wan1"},
			SourceAddresses:  []string{"all"},
			DestAddresses:    []string{"all"},
			Services:         []string{"BitTorrent"},
			LogTraffic:       "all",
		},
	}

	engine.LoadFortiGatePolicies(policies)

	// Test Case 1: Permitted HTTP traffic with NAT
	flow1 := TrafficFlow{
		SourceIP:        "10.1.1.50",
		SourcePort:      54321,
		SourceInterface: "internal",
		DestIP:          "8.8.8.8",
		DestPort:        80,
		DestInterface:   "wan1",
		Protocol:        "tcp",
		DeviceID:        "fg-01",
	}

	decision1, err := engine.EvaluateFlow(flow1)
	assert.NoError(t, err)
	assert.Equal(t, "accept", decision1.Action)
	assert.Equal(t, "fortinet", decision1.Vendor)
	assert.Equal(t, "Allow-Internet", decision1.MatchedRule)
	assert.True(t, decision1.NATApplied)
	assert.Contains(t, decision1.ThreatProfile, "IPS")
	assert.Contains(t, decision1.ThreatProfile, "AV")

	// Test Case 2: Implicit deny (no matching policy)
	flow2 := TrafficFlow{
		SourceIP:        "10.1.1.50",
		SourcePort:      12345,
		SourceInterface: "dmz",
		DestIP:          "192.168.1.1",
		DestPort:        22,
		DestInterface:   "internal",
		Protocol:        "tcp",
		DeviceID:        "fg-01",
	}

	decision2, err := engine.EvaluateFlow(flow2)
	assert.NoError(t, err)
	assert.Equal(t, "deny", decision2.Action)
	assert.Contains(t, decision2.Reason, "Implicit deny")
}

// TestPaloAltoFirewallDecision tests Palo Alto firewall policy evaluation
func TestPaloAltoFirewallDecision(t *testing.T) {
	engine := NewL4FirewallDecisionEngine(true, true, false)

	// Load sample Palo Alto policies
	policies := []paloaltoparsers.PANOSSecurityPolicy{
		{
			Name:             "Allow-Web-Browsing",
			RuleType:         "universal",
			SourceZones:      []string{"trust"},
			DestZones:        []string{"untrust"},
			SourceAddresses:  []string{"any"},
			DestAddresses:    []string{"any"},
			Applications:     []string{"web-browsing", "ssl"},
			Services:         []string{"application-default"},
			Action:           "allow",
			ProfileType:      "group",
			GroupProfile:     "strict",
			LogEnd:           true,
			Disabled:         false,
		},
		{
			Name:             "Deny-Torrent",
			RuleType:         "universal",
			SourceZones:      []string{"trust"},
			DestZones:        []string{"any"},
			SourceAddresses:  []string{"any"},
			DestAddresses:    []string{"any"},
			Applications:     []string{"bittorrent"},
			Services:         []string{"any"},
			Action:           "deny",
			LogEnd:           true,
			Disabled:         false,
		},
	}

	engine.LoadPaloAltoPolicies(policies)

	// Test Case 1: Permitted web browsing
	flow1 := TrafficFlow{
		SourceIP:    "10.1.1.50",
		SourcePort:  54321,
		SourceZone:  "trust",
		DestIP:      "8.8.8.8",
		DestPort:    443,
		DestZone:    "untrust",
		Protocol:    "tcp",
		Application: "ssl",
		DeviceID:    "pa-01",
	}

	decision1, err := engine.EvaluateFlow(flow1)
	assert.NoError(t, err)
	assert.Equal(t, "allow", decision1.Action)
	assert.Equal(t, "paloalto", decision1.Vendor)
	assert.Equal(t, "Allow-Web-Browsing", decision1.MatchedRule)
	assert.Contains(t, decision1.ThreatProfile, "strict")
	assert.True(t, decision1.LogEnabled)

	// Test Case 2: Denied BitTorrent
	flow2 := TrafficFlow{
		SourceIP:    "10.1.1.75",
		SourcePort:  55555,
		SourceZone:  "trust",
		DestIP:      "203.0.113.100",
		DestPort:    6881,
		DestZone:    "untrust",
		Protocol:    "tcp",
		Application: "bittorrent",
		DeviceID:    "pa-01",
	}

	decision2, err := engine.EvaluateFlow(flow2)
	assert.NoError(t, err)
	assert.Equal(t, "deny", decision2.Action)
	assert.Equal(t, "Deny-Torrent", decision2.MatchedRule)
}

// TestL4WithL3Integration tests L4 firewall decisions with L3 routing integration
func TestL4WithL3Integration(t *testing.T) {
	engine := NewL4FirewallDecisionEngine(true, true, false)

	// Load Cisco ASA ACL
	aclEntries := []asaparsers.ASAACLEntry{
		{
			ACLName:          "outside_in",
			LineNumber:       10,
			Action:           "permit",
			Protocol:         "tcp",
			SourceAddress:    "any",
			DestAddress:      "10.1.1.100",
			DestMask:         "255.255.255.255",
			DestPort:         "80",
			DestPortOperator: "eq",
		},
	}
	engine.LoadCiscoASAPolicies(aclEntries)

	// Load L3 routing table
	routingTable := &models.L3RoutingTable{
		DeviceID: "asa-01",
		Routes: []models.L3Route{
			{
				Prefix:    "0.0.0.0/0",
				NextHop:   "10.0.0.1",
				Interface: "outside",
				Protocol:  "static",
			},
			{
				Prefix:    "10.1.1.0/24",
				NextHop:   "0.0.0.0",
				Interface: "inside",
				Protocol:  "connected",
			},
		},
	}
	engine.LoadRoutingTable("asa-01", routingTable)

	// Evaluate flow
	flow := TrafficFlow{
		SourceIP:   "203.0.113.50",
		SourcePort: 54321,
		DestIP:     "10.1.1.100",
		DestPort:   80,
		Protocol:   "tcp",
		DeviceID:   "asa-01",
	}

	decision, err := engine.EvaluateFlow(flow)
	assert.NoError(t, err)
	assert.Equal(t, "permit", decision.Action)
	assert.Equal(t, "0.0.0.0", decision.NextHop)        // Connected network
	assert.Equal(t, "inside", decision.EgressIface)     // Destination is on inside interface
}

// TestMatchesIPAddress tests IP address matching logic
func TestMatchesIPAddress(t *testing.T) {
	tests := []struct {
		flowIP   string
		ruleIP   string
		ruleMask string
		expected bool
	}{
		{"10.1.1.50", "any", "", true},
		{"10.1.1.50", "10.1.1.50", "255.255.255.255", true},
		{"10.1.1.50", "10.1.1.0", "255.255.255.0", true},
		{"10.1.1.50", "10.1.2.0", "255.255.255.0", false},
		{"192.168.1.100", "192.168.0.0", "255.255.0.0", true},
		{"172.16.5.10", "172.16.0.0", "255.255.0.0", true},
		{"172.17.1.1", "172.16.0.0", "255.255.0.0", false},
	}

	for _, test := range tests {
		result := matchesIPAddress(test.flowIP, test.ruleIP, test.ruleMask)
		assert.Equal(t, test.expected, result, "Failed for %s in %s/%s", test.flowIP, test.ruleIP, test.ruleMask)
	}
}

// TestMatchesPort tests port matching logic
func TestMatchesPort(t *testing.T) {
	tests := []struct {
		flowPort uint16
		rulePort string
		operator string
		expected bool
	}{
		{80, "80", "eq", true},
		{80, "443", "eq", false},
		{1024, "1023", "gt", true},
		{1022, "1023", "gt", false},
		{80, "443", "lt", true},
		{443, "80", "lt", false},
		{80, "443", "neq", true},
		{443, "443", "neq", false},
		{12345, "any", "", true},
	}

	for _, test := range tests {
		result := matchesPort(test.flowPort, test.rulePort, test.operator)
		assert.Equal(t, test.expected, result, "Failed for port %d %s %s", test.flowPort, test.operator, test.rulePort)
	}
}

// TestMatchesProtocol tests protocol matching logic
func TestMatchesProtocol(t *testing.T) {
	tests := []struct {
		flowProto string
		ruleProto string
		expected  bool
	}{
		{"tcp", "tcp", true},
		{"tcp", "TCP", true},
		{"tcp", "udp", false},
		{"tcp", "ip", true},
		{"udp", "any", true},
		{"icmp", "icmp", true},
		{"icmp", "tcp", false},
	}

	for _, test := range tests {
		result := matchesProtocol(test.flowProto, test.ruleProto)
		assert.Equal(t, test.expected, result, "Failed for %s vs %s", test.flowProto, test.ruleProto)
	}
}

// TestResolveNextHop tests next-hop resolution
func TestResolveNextHop(t *testing.T) {
	engine := NewL4FirewallDecisionEngine(false, false, false)

	routingTable := &models.L3RoutingTable{
		DeviceID: "test-device",
		Routes: []models.L3Route{
			{
				Prefix:    "0.0.0.0/0",
				NextHop:   "10.0.0.1",
				Interface: "outside",
			},
			{
				Prefix:    "192.168.1.0/24",
				NextHop:   "10.1.1.1",
				Interface: "inside",
			},
			{
				Prefix:    "192.168.1.128/25",
				NextHop:   "10.1.1.2",
				Interface: "dmz",
			},
		},
	}

	tests := []struct {
		destIP        string
		expectedNH    string
		expectedIface string
	}{
		{"8.8.8.8", "10.0.0.1", "outside"},           // Default route
		{"192.168.1.50", "10.1.1.1", "inside"},       // /24 match
		{"192.168.1.200", "10.1.1.2", "dmz"},         // /25 match (more specific)
	}

	for _, test := range tests {
		nextHop, iface := engine.resolveNextHop(test.destIP, routingTable)
		assert.Equal(t, test.expectedNH, nextHop, "Next hop mismatch for %s", test.destIP)
		assert.Equal(t, test.expectedIface, iface, "Interface mismatch for %s", test.destIP)
	}
}

// TestMultiVendorScenario tests a realistic multi-vendor scenario
func TestMultiVendorScenario(t *testing.T) {
	// This test simulates traffic flowing through multiple firewalls:
	// Internet -> Palo Alto (edge) -> FortiGate (datacenter) -> Cisco ASA (server farm)

	// Edge firewall (Palo Alto)
	paEngine := NewL4FirewallDecisionEngine(true, true, false)
	paEngine.LoadPaloAltoPolicies([]paloaltoparsers.PANOSSecurityPolicy{
		{
			Name:             "Internet-Inbound",
			SourceZones:      []string{"untrust"},
			DestZones:        []string{"dmz"},
			SourceAddresses:  []string{"any"},
			DestAddresses:    []string{"DMZ-Servers"},
			Applications:     []string{"web-browsing"},
			Action:           "allow",
			LogEnd:           true,
		},
	})

	// Datacenter firewall (FortiGate)
	fgEngine := NewL4FirewallDecisionEngine(true, true, false)
	fgEngine.LoadFortiGatePolicies([]fortigateparsers.FortiGateFirewallPolicy{
		{
			PolicyID:         1,
			Name:             "DMZ-to-ServerFarm",
			Status:           "enable",
			Action:           "accept",
			SourceInterfaces: []string{"dmz"},
			DestInterfaces:   []string{"server-farm"},
			SourceAddresses:  []string{"all"},
			DestAddresses:    []string{"AppServers"},
			Services:         []string{"HTTP"},
		},
	})

	// Server farm firewall (Cisco ASA)
	asaEngine := NewL4FirewallDecisionEngine(true, true, false)
	asaEngine.LoadCiscoASAPolicies([]asaparsers.ASAACLEntry{
		{
			ACLName:          "server_farm_in",
			Action:           "permit",
			Protocol:         "tcp",
			SourceAddress:    "any",
			DestAddress:      "10.100.1.0",
			DestMask:         "255.255.255.0",
			DestPort:         "80",
			DestPortOperator: "eq",
		},
	})

	// Test traffic flow through all three firewalls
	// Flow: Internet client -> Web server

	// Step 1: Edge firewall (Palo Alto)
	flow1 := TrafficFlow{
		SourceIP:    "203.0.113.50",
		SourceZone:  "untrust",
		DestIP:      "198.51.100.10",
		DestZone:    "dmz",
		Application: "web-browsing",
		Protocol:    "tcp",
		DestPort:    80,
		DeviceID:    "pa-edge",
	}

	decision1, err := paEngine.EvaluateFlow(flow1)
	assert.NoError(t, err)
	assert.Equal(t, "allow", decision1.Action, "Edge firewall should allow")

	// Step 2: Datacenter firewall (FortiGate)
	flow2 := TrafficFlow{
		SourceIP:        "198.51.100.10",
		SourceInterface: "dmz",
		DestIP:          "10.100.1.50",
		DestInterface:   "server-farm",
		Protocol:        "tcp",
		DestPort:        80,
		DeviceID:        "fg-dc",
	}

	decision2, err := fgEngine.EvaluateFlow(flow2)
	assert.NoError(t, err)
	assert.Equal(t, "accept", decision2.Action, "Datacenter firewall should accept")

	// Step 3: Server farm firewall (Cisco ASA)
	flow3 := TrafficFlow{
		SourceIP:   "198.51.100.10",
		DestIP:     "10.100.1.50",
		Protocol:   "tcp",
		DestPort:   80,
		DeviceID:   "asa-sf",
	}

	decision3, err := asaEngine.EvaluateFlow(flow3)
	assert.NoError(t, err)
	assert.Equal(t, "permit", decision3.Action, "Server farm firewall should permit")

	// All three firewalls permitted the traffic
	t.Log("Traffic successfully traversed all three firewalls:")
	t.Logf("  1. Palo Alto (edge): %s", decision1.Action)
	t.Logf("  2. FortiGate (DC): %s", decision2.Action)
	t.Logf("  3. Cisco ASA (SF): %s", decision3.Action)
}

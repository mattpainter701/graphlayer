package intelligence

import (
	"strings"
	"testing"
	"time"

	"github.com/mattpainter701/graphlayer/platform/models"
)

// ========================================================================
// Test Helpers
// ========================================================================

func makeTestContext() *InferenceContext {
	return &InferenceContext{
		Devices:       make(map[string]*models.Device),
		Interfaces:    make(map[string]*models.Interface),
		LLDPNeighbors: make(map[string][]LLDPNeighborData),
		FDBEntries:    make([]models.FDBEntry, 0),
		RoutingTables: make([]models.L3RoutingTable, 0),
		RoutingEntries: make([]models.RoutingEntry, 0),
		NATRules:      make([]models.NAT, 0),
		VIPs:          make([]models.VIP, 0),
		L2Switches:    make([]models.L2Switch, 0),
		L2Links:       make([]models.L2Link, 0),
		NetworkNodes:  make([]models.NetworkNode, 0),
		Tunnels:       make([]models.Tunnel, 0),
		CollectionTime: time.Now(),
	}
}

func makeDevice(hostname, mgmtIP, vendor string) *models.Device {
	return &models.Device{
		Hostname:     hostname,
		ManagementIP: mgmtIP,
		Vendor:       vendor,
	}
}

func makeInterface(device, name, ip string) *models.Interface {
	return &models.Interface{
		Name:      name,
		Addresses: []string{ip},
		Status:    "up",
	}
}

func makeL3Route(prefix string, prefixLen int, nextHop, iface, protocol string, metric, ad int) models.L3Route {
	// Format as proper CIDR for ParseCIDR compatibility
	cidr := prefix
	if prefixLen > 0 && !strings.Contains(prefix, "/") {
		cidr = prefix + "/" + itoa(prefixLen)
	}
	return models.L3Route{
		Prefix:        cidr,
		PrefixLength:  prefixLen,
		NextHop:       nextHop,
		Interface:     iface,
		Protocol:      protocol,
		Metric:        metric,
		AdminDistance: ad,
		IsActive:      true,
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	s := ""
	for i > 0 {
		s = string(rune('0'+i%10)) + s
		i /= 10
	}
	return s
}

// ========================================================================
// L2 Inference Tests
// ========================================================================

func TestL2InferFromLLDP(t *testing.T) {
	ctx := makeTestContext()
	ctx.LLDPNeighbors["switch-1"] = []LLDPNeighborData{
		{
			LocalInterface:  "Gi1/0/1",
			RemoteSystemName: "switch-2",
			RemoteInterface: "Gi1/0/1",
			RemoteChassisID: "aa:bb:cc:dd:ee:ff",
			Protocol:        "lldp",
		},
	}

	eng := NewL2InferenceEngine(ctx)
	links := eng.InferFromLLDP()

	if len(links) != 1 {
		t.Fatalf("expected 1 LLDP-inferred link, got %d", len(links))
	}

	link := links[0]
	if link.FromDevice != "switch-1" || link.ToDevice != "switch-2" {
		t.Errorf("unexpected link endpoints: %s → %s", link.FromDevice, link.ToDevice)
	}
	if link.InferenceMethod != "lldp-match" {
		t.Errorf("unexpected inference method: %s", link.InferenceMethod)
	}
	if link.Confidence < 0.90 {
		t.Errorf("LLDP link confidence too low: %.2f", link.Confidence)
	}
}

func TestL2InferFromMACTable(t *testing.T) {
	ctx := makeTestContext()
	ctx.FDBEntries = []models.FDBEntry{
		{DeviceID: "switch-a", MAC: "AA:BB:CC:DD:EE:01", LastSeenPort: "Gi1/0/1", VLANID: 10, Confidence: 1.0},
		{DeviceID: "switch-a", MAC: "AA:BB:CC:DD:EE:02", LastSeenPort: "Gi1/0/2", VLANID: 10, Confidence: 1.0},
		{DeviceID: "switch-a", MAC: "AA:BB:CC:DD:EE:03", LastSeenPort: "Gi1/0/3", VLANID: 10, Confidence: 1.0},
		{DeviceID: "switch-b", MAC: "AA:BB:CC:DD:EE:01", LastSeenPort: "Gi1/0/1", VLANID: 10, Confidence: 1.0},
		{DeviceID: "switch-b", MAC: "AA:BB:CC:DD:EE:02", LastSeenPort: "Gi1/0/2", VLANID: 10, Confidence: 1.0},
		{DeviceID: "switch-b", MAC: "AA:BB:CC:DD:EE:03", LastSeenPort: "Gi1/0/3", VLANID: 10, Confidence: 1.0},
	}

	eng := NewL2InferenceEngine(ctx)
	links := eng.InferFromMACTable()

	if len(links) != 1 {
		t.Fatalf("expected 1 MAC-inferred link (3 overlapping), got %d", len(links))
	}

	link := links[0]
	if link.InferenceMethod != "mac-correlation" {
		t.Errorf("unexpected inference method: %s", link.InferenceMethod)
	}
}

func TestL2InferFromMACTableInsufficientOverlap(t *testing.T) {
	ctx := makeTestContext()
	ctx.FDBEntries = []models.FDBEntry{
		{DeviceID: "switch-a", MAC: "AA:BB:CC:DD:EE:01", LastSeenPort: "Gi1/0/1", VLANID: 10},
		{DeviceID: "switch-b", MAC: "AA:BB:CC:DD:EE:01", LastSeenPort: "Gi1/0/1", VLANID: 10},
	}

	eng := NewL2InferenceEngine(ctx)
	links := eng.InferFromMACTable()

	if len(links) != 0 {
		t.Errorf("expected 0 links (<3 overlapping MACs), got %d", len(links))
	}
}

// ========================================================================
// L3 Inference Tests
// ========================================================================

func TestL3InferLinksFromRouting(t *testing.T) {
	ctx := makeTestContext()
	ctx.Devices["router-a"] = makeDevice("router-a", "10.1.1.1", "cisco")
	ctx.Devices["router-b"] = makeDevice("router-b", "10.1.1.2", "cisco")
	ctx.Interfaces["router-a:Gi0/0"] = makeInterface("router-a", "Gi0/0", "10.1.1.1/30")
	ctx.Interfaces["router-b:Gi0/0"] = makeInterface("router-b", "Gi0/0", "10.1.1.2/30")

	ctx.RoutingTables = []models.L3RoutingTable{
		{
			Hostname: "router-a",
			Routes: []models.L3Route{
				makeL3Route("192.168.1.0", 24, "10.1.1.2", "Gi0/0", "ospf", 10, 110),
			},
		},
	}

	eng := NewL3InferenceEngine(ctx)
	links := eng.InferLinksFromRouting()

	if len(links) != 1 {
		t.Fatalf("expected 1 L3-inferred link, got %d", len(links))
	}

	link := links[0]
	if link.FromDevice != "router-a" || link.ToDevice != "router-b" {
		t.Errorf("unexpected link: %s → %s", link.FromDevice, link.ToDevice)
	}
	if link.InferenceMethod != "routing-next-hop" {
		t.Errorf("unexpected method: %s", link.InferenceMethod)
	}
}

func TestL3PathComputation(t *testing.T) {
	ctx := makeTestContext()
	ctx.Devices["router-a"] = makeDevice("router-a", "10.1.1.1", "cisco")
	ctx.Devices["router-b"] = makeDevice("router-b", "10.1.1.2", "cisco")
	ctx.Interfaces["router-a:Gi0/0"] = makeInterface("router-a", "Gi0/0", "10.1.1.1/30")
	ctx.Interfaces["router-b:Gi0/0"] = makeInterface("router-b", "Gi0/0", "10.1.1.2/30")

	ctx.RoutingTables = []models.L3RoutingTable{
		{
			Hostname: "router-a",
			Routes: []models.L3Route{
				makeL3Route("192.168.1.0", 24, "10.1.1.2", "Gi0/0", "ospf", 10, 110),
			},
		},
		{
			Hostname: "router-b",
			Routes: []models.L3Route{
				makeL3Route("192.168.1.0", 24, "0.0.0.0", "Gi0/1", "connected", 0, 0),
			},
		},
	}

	eng := NewL3InferenceEngine(ctx)
	result := eng.ComputePath("router-a", "192.168.1.10")

	if len(result.Hops) == 0 {
		t.Fatal("expected at least 1 hop in path")
	}
	if result.Hops[0].Device != "router-a" {
		t.Errorf("first hop should be router-a, got %s", result.Hops[0].Device)
	}
}

// ========================================================================
// Subnet Discovery Tests
// ========================================================================

func TestSubnetDiscovery(t *testing.T) {
	ctx := makeTestContext()
	ctx.Interfaces["dev-a:eth0"] = makeInterface("dev-a", "eth0", "10.1.1.10/24")
	ctx.Interfaces["dev-b:eth0"] = makeInterface("dev-b", "eth0", "10.1.1.20/24")
	ctx.Interfaces["dev-c:eth0"] = makeInterface("dev-c", "eth0", "192.168.1.10/24")

	sd := NewSubnetDiscovery(ctx)
	groups := sd.DiscoverSubnets()

	if len(groups) < 2 {
		t.Fatalf("expected at least 2 subnet groups, got %d", len(groups))
	}

	// Find the 10.1.1.0/24 group
	var found bool
	for _, g := range groups {
		if g.Subnet == "10.1.1.0/24" {
			found = true
			if len(g.Devices) < 2 {
				t.Error("10.1.1.0/24 group should have at least 2 devices")
			}
		}
	}
	if !found {
		t.Error("missing 10.1.1.0/24 subnet group")
	}
}

func TestGatewayInference(t *testing.T) {
	ctx := makeTestContext()
	ctx.Interfaces["router:eth0"] = makeInterface("router", "eth0", "10.1.1.1/24")
	ctx.Interfaces["host:eth0"] = makeInterface("host", "eth0", "10.1.1.100/24")

	ctx.RoutingTables = []models.L3RoutingTable{
		{
			Hostname: "router",
			Routes: []models.L3Route{
				makeL3Route("10.1.1.0", 24, "0.0.0.0", "eth0", "connected", 0, 0),
			},
		},
	}

	sd := NewSubnetDiscovery(ctx)
	group := SubnetGroup{
		Subnet: "10.1.1.0/24",
		Interfaces: []SubnetInterface{
			{Device: "router", IP: "10.1.1.1"},
			{Device: "host", IP: "10.1.1.100"},
		},
	}

	gwDevice, gwIP := sd.InferGateway(group)

	if gwDevice != "router" {
		t.Errorf("expected gateway router, got %s (IP: %s)", gwDevice, gwIP)
	}
	if gwIP != "10.1.1.1" {
		t.Errorf("expected gateway IP 10.1.1.1, got %s", gwIP)
	}
}

// ========================================================================
// Implicit Link Tests
// ========================================================================

func TestImplicitLinkDetection(t *testing.T) {
	ctx := makeTestContext()
	ctx.Interfaces["dev-a:eth1"] = makeInterface("dev-a", "eth1", "10.255.0.1/30")
	ctx.Interfaces["dev-b:eth1"] = makeInterface("dev-b", "eth1", "10.255.0.2/30")

	detector := NewImplicitLinkDetector(ctx)
	links := detector.DetectLinks()

	if len(links) != 1 {
		t.Fatalf("expected 1 implicit link, got %d", len(links))
	}

	link := links[0]
	if link.InferenceMethod != "subnet-match" {
		t.Errorf("unexpected method: %s", link.InferenceMethod)
	}
	if link.LinkType != "implicit" {
		t.Errorf("unexpected link type: %s", link.LinkType)
	}
	// /30 subnets should have confidence >= 0.65
	if link.Confidence < 0.60 {
		t.Errorf("/30 implicit link confidence too low: %.2f", link.Confidence)
	}
}

func TestImplicitLinkSkippedWhenExplicitExists(t *testing.T) {
	ctx := makeTestContext()
	ctx.Interfaces["dev-a:eth1"] = makeInterface("dev-a", "eth1", "10.255.0.1/30")
	ctx.Interfaces["dev-b:eth1"] = makeInterface("dev-b", "eth1", "10.255.0.2/30")
	ctx.L2Links = []models.L2Link{
		{FromDevice: "dev-a", ToDevice: "dev-b", LinkType: "lldp"},
	}

	detector := NewImplicitLinkDetector(ctx)
	links := detector.DetectLinks()

	if len(links) != 0 {
		t.Errorf("expected 0 implicit links (explicit exists), got %d", len(links))
	}
}

// ========================================================================
// Boundary Detection Tests
// ========================================================================

func TestNetworkBoundaryDetection(t *testing.T) {
	ctx := makeTestContext()
	ctx.Interfaces["fw:outside"] = makeInterface("fw", "outside", "203.0.113.1/24")
	ctx.Interfaces["fw:inside"] = makeInterface("fw", "inside", "10.1.1.1/24")

	detector := NewBoundaryDetector(ctx)
	boundaries := detector.DetectNetworkBoundaries()

	if len(boundaries) == 0 {
		t.Fatal("expected at least 1 network boundary (fw has private+public)")
	}

	found := false
	for _, b := range boundaries {
		if b.Device == "fw" {
			found = true
			if b.BoundaryType != "internet-edge" {
				t.Errorf("expected internet-edge, got %s", b.BoundaryType)
			}
		}
	}
	if !found {
		t.Error("missing boundary for firewall device")
	}
}

func TestNATBoundaryInference(t *testing.T) {
	ctx := makeTestContext()
	ctx.NATRules = []models.NAT{
		{
			Type:             "source",
			OriginalSource:   "10.1.1.0/24",
			TranslatedSource: "203.0.113.10",
			OutboundInterface: "outside",
		},
	}

	detector := NewBoundaryDetector(ctx)
	boundaries := detector.InferNATBoundaries()

	if len(boundaries) == 0 {
		t.Fatal("expected at least 1 NAT boundary from rule")
	}

	b := boundaries[0]
	if b.PrivateIP != "10.1.1.0/24" {
		t.Errorf("unexpected private IP: %s", b.PrivateIP)
	}
	if b.Confidence < 0.90 {
		t.Errorf("NAT rule confidence too low: %.2f", b.Confidence)
	}
}

// ========================================================================
// Rule Engine Tests
// ========================================================================

func TestRuleEngineDefaults(t *testing.T) {
	re := NewRuleEngine()
	rules := re.GetEnabledRules()

	if len(rules) == 0 {
		t.Fatal("expected default rules")
	}
	if len(rules) != 7 {
		t.Errorf("expected 7 default rules, got %d", len(rules))
	}

	// Verify sorted by priority
	for i := 1; i < len(rules); i++ {
		if rules[i].Priority > rules[i-1].Priority {
			t.Errorf("rules not sorted by priority: %s(%d) > %s(%d)",
				rules[i].Name, rules[i].Priority,
				rules[i-1].Name, rules[i-1].Priority)
		}
	}
}

func TestRuleEngineEnableDisable(t *testing.T) {
	re := NewRuleEngine()
	ruleName := DefaultInferenceRules()[0].Name

	if !re.DisableRule(ruleName) {
		t.Error("expected to disable existing rule")
	}

	enabled := re.GetEnabledRules()
	if len(enabled) != 6 {
		t.Errorf("expected 6 rules after disable, got %d", len(enabled))
	}

	if !re.EnableRule(ruleName) {
		t.Error("expected to re-enable rule")
	}

	enabled = re.GetEnabledRules()
	if len(enabled) != 7 {
		t.Errorf("expected 7 rules after re-enable, got %d", len(enabled))
	}
}

func TestRuleEngineConflictResolution(t *testing.T) {
	re := NewRuleEngine()
	rule := InferenceRule{
		Name:            "test-rule",
		Priority:        PriorityHigh,
		ResolveConflict: "prefer",
	}

	conflict := re.ResolveConflict(rule, InferredLink{LinkType: "l2"}, InferredLink{LinkType: "l3"})

	if conflict.Resolution != "prefer-a" {
		t.Errorf("expected prefer-a, got %s", conflict.Resolution)
	}
	if conflict.ConflictType != "link" {
		t.Errorf("expected link conflict type, got %s", conflict.ConflictType)
	}
}

func TestRuleEnginePriorityManagement(t *testing.T) {
	re := NewRuleEngine()
	ruleName := DefaultInferenceRules()[0].Name

	if !re.SetPriority(ruleName, PriorityLowest) {
		t.Error("expected to set priority")
	}

	rules := re.GetRules()
	for _, r := range rules {
		if r.Name == ruleName && r.Priority != PriorityLowest {
			t.Errorf("expected priority %d, got %d", PriorityLowest, r.Priority)
		}
	}
}

func TestValidateRule(t *testing.T) {
	validRule := InferenceRule{
		Name:            "test",
		Priority:        50,
		RuleType:        "l3-link",
		ResolveConflict: "prefer",
		Enabled:         true,
	}
	if err := ValidateRule(validRule); err != nil {
		t.Errorf("valid rule should not error: %v", err)
	}

	noName := validRule
	noName.Name = ""
	if err := ValidateRule(noName); err == nil {
		t.Error("rule with no name should error")
	}

	badPriority := validRule
	badPriority.Priority = 150
	if err := ValidateRule(badPriority); err == nil {
		t.Error("rule with priority > 100 should error")
	}

	badType := validRule
	badType.RuleType = "invalid-type"
	if err := ValidateRule(badType); err == nil {
		t.Error("rule with invalid type should error")
	}
}

// ========================================================================
// Full Inference Engine Pipeline Tests
// ========================================================================

func TestInferenceEngineFullPipeline(t *testing.T) {
	ctx := makeTestContext()

	// Devices
	ctx.Devices["router-1"] = makeDevice("router-1", "10.0.0.1", "cisco")
	ctx.Devices["router-2"] = makeDevice("router-2", "10.0.0.2", "cisco")
	ctx.Devices["switch-1"] = makeDevice("switch-1", "10.0.0.10", "cisco")
	ctx.Devices["switch-2"] = makeDevice("switch-2", "10.0.0.11", "cisco")

	// Interfaces
	ctx.Interfaces["router-1:Gi0/0"] = makeInterface("router-1", "Gi0/0", "10.0.0.1/30")
	ctx.Interfaces["router-2:Gi0/0"] = makeInterface("router-2", "Gi0/0", "10.0.0.2/30")
	ctx.Interfaces["switch-1:Gi1/0/1"] = makeInterface("switch-1", "Gi1/0/1", "10.0.0.10/24")
	ctx.Interfaces["switch-2:Gi1/0/1"] = makeInterface("switch-2", "Gi1/0/1", "10.0.0.11/24")

	// LLDP neighbors
	ctx.LLDPNeighbors["switch-1"] = []LLDPNeighborData{
		{
			LocalInterface:  "Gi1/0/24",
			RemoteSystemName: "switch-2",
			RemoteInterface: "Gi1/0/24",
			RemoteChassisID: "00:11:22:33:44:55",
			Protocol:        "lldp",
		},
	}

	// Routing tables
	ctx.RoutingTables = []models.L3RoutingTable{
		{
			Hostname: "router-1",
			Routes: []models.L3Route{
				makeL3Route("192.168.0.0", 16, "10.0.0.2", "Gi0/0", "ospf", 10, 110),
			},
		},
	}

	// NAT rules
	ctx.NATRules = []models.NAT{
		{
			Type:              "source",
			OriginalSource:    "10.0.0.0/24",
			TranslatedSource:  "203.0.113.1",
			OutboundInterface: "Gi0/1",
		},
	}

	// Run full inference pipeline
	eng := NewInferenceEngine(ctx)
	result := eng.Run()

	// Verify results
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// Should have at least 1 L2 link from LLDP
	if result.Stats.L2LinksInferred == 0 {
		t.Error("expected at least 1 L2 link from LLDP")
	}

	// Should have at least 1 L3 link from routing
	if result.Stats.L3LinksInferred == 0 {
		t.Error("expected at least 1 L3 link from routing")
	}

	// Should have subnet groups
	if result.Stats.SubnetGroupsFound == 0 {
		t.Error("expected subnet groups")
	}

	// Should have NAT boundaries
	if result.Stats.NATBoundariesInferred == 0 {
		t.Error("expected NAT boundaries")
	}

	// All rules should execute
	if result.Stats.RulesExecuted != 7 {
		t.Errorf("expected 7 rules executed, got %d", result.Stats.RulesExecuted)
	}

	// Verify engine version
	if result.EngineVersion != "inference-v1.0.0" {
		t.Errorf("unexpected version: %s", result.EngineVersion)
	}
}

func TestInferenceEngineCustomRules(t *testing.T) {
	ctx := makeTestContext()

	customRules := []InferenceRule{
		{
			Name:            "custom-l2",
			Description:     "Custom L2 only",
			Priority:        PriorityHighest,
			Enabled:         true,
			RuleType:        "l2-link",
			ResolveConflict: "prefer",
		},
	}

	eng := NewInferenceEngineWithRules(ctx, customRules)
	result := eng.Run()

	if result.Stats.RulesExecuted != 1 {
		t.Errorf("expected 1 rule executed, got %d", result.Stats.RulesExecuted)
	}
}

// ========================================================================
// Deduplication Tests
// ========================================================================

func TestDeduplicateLinks(t *testing.T) {
	links := []InferredLink{
		{ID: "a", FromDevice: "dev1", ToDevice: "dev2"},
		{ID: "b", FromDevice: "dev2", ToDevice: "dev1"}, // Duplicate (reversed)
		{ID: "c", FromDevice: "dev1", ToDevice: "dev3"}, // Unique
	}

	result := deduplicateLinks(links)
	if len(result) != 2 {
		t.Errorf("expected 2 links after dedup, got %d", len(result))
	}
}

func TestDeduplicateSubnetGroups(t *testing.T) {
	groups := []SubnetGroup{
		{Subnet: "10.1.1.0/24", Devices: []string{"dev-a"}},
		{Subnet: "10.1.1.0/24", Devices: []string{"dev-b"}}, // Duplicate
		{Subnet: "192.168.1.0/24", Devices: []string{"dev-c"}},
	}

	result := deduplicateSubnetGroups(groups)
	if len(result) != 2 {
		t.Errorf("expected 2 groups after dedup, got %d", len(result))
	}

	// Verify devices merged
	for _, g := range result {
		if g.Subnet == "10.1.1.0/24" && len(g.Devices) != 2 {
			t.Errorf("expected 2 devices in merged group, got %d", len(g.Devices))
		}
	}
}

// ========================================================================
// Edge Case Tests
// ========================================================================

func TestEmptyContext(t *testing.T) {
	ctx := makeTestContext()
	eng := NewInferenceEngine(ctx)
	result := eng.Run()

	if result == nil {
		t.Fatal("expected non-nil result even for empty context")
	}
	if result.Stats.TotalLinksInferred != 0 {
		t.Errorf("expected 0 links for empty context, got %d", result.Stats.TotalLinksInferred)
	}
}

func TestNullRouteSkipped(t *testing.T) {
	ctx := makeTestContext()
	ctx.Devices["router"] = makeDevice("router", "10.0.0.1", "cisco")

	ctx.RoutingTables = []models.L3RoutingTable{
		{
			Hostname: "router",
			Routes: []models.L3Route{
				makeL3Route("0.0.0.0", 0, "203.0.113.1", "Gi0/1", "static", 1, 1),
			},
		},
	}

	eng := NewL3InferenceEngine(ctx)
	links := eng.InferLinksFromRouting()

	// Default routes should be skipped for link inference
	if len(links) != 0 {
		t.Errorf("expected 0 links from default route (skipped), got %d", len(links))
	}
}

func TestPrivateIPClassification(t *testing.T) {
	tests := []struct {
		ip      string
		private bool
	}{
		{"10.0.0.1", true},
		{"172.16.0.1", true},
		{"192.168.1.1", true},
		{"100.64.0.1", true},  // CGNAT
		{"169.254.1.1", true}, // Link-local
		{"8.8.8.8", false},
		{"203.0.113.1", false},
		{"127.0.0.1", false}, // Loopback (not private, but not public either)
	}

	for _, tc := range tests {
		result := isPrivateIP(tc.ip)
		if result != tc.private {
			t.Errorf("isPrivateIP(%s) = %v, want %v", tc.ip, result, tc.private)
		}
	}
}

func TestUplinkInterfaceDetection(t *testing.T) {
	detector := &ImplicitLinkDetector{}

	tests := []struct {
		name   string
		uplink bool
	}{
		{"Uplink-to-Core", true},
		{"Transit-Network", true},
		{"GigabitEthernet0/1", false},
		{"peer-link", true},
		{"Backbone-port", true},
	}

	for _, tc := range tests {
		if detector.isUplinkInterface(tc.name) != tc.uplink {
			t.Errorf("isUplinkInterface(%s) = %v, want %v", tc.name, !tc.uplink, tc.uplink)
		}
	}
}

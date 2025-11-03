package decision

import (
	"testing"
	"time"

	"github.com/mattpainter701/graphlayer/platform/platform/models"
	"github.com/stretchr/testify/assert"
)

// ========================================================================
// Phase 2: L3 Decision Engine Unit Tests
// ========================================================================

func TestDetermineL3Reachability(t *testing.T) {
	tests := []struct {
		name                string
		query               models.L3ReachabilityQuery
		routingTable        *models.L3RoutingTable
		expectedOutcome     string
		expectedReasonCode  string
		expectedConfidence  int
		expectedRequiresFirewall bool
	}{
		{
			name: "Allowed - Connected route with high confidence",
			query: models.L3ReachabilityQuery{
				SourceIP:       "10.1.1.1",
				DestIP:         "10.1.1.100",
				SourceDevice:   "router1",
				VirtualContext: "default",
			},
			routingTable: &models.L3RoutingTable{
				DeviceID:       "router1",
				Hostname:       "R1",
				VirtualContext: "default",
				Routes: []models.L3Route{
					{
						Prefix:          "10.1.1.0/24",
						PrefixLength:    24,
						NextHop:         "",
						Interface:       "GigabitEthernet0/0",
						Protocol:        "connected",
						Metric:          0,
						AdminDistance:   0,
						IsActive:        true,
						IsBest:          true,
						ConfidenceScore: 95,
						Age:             100,
					},
				},
			},
			expectedOutcome:    "Allowed",
			expectedReasonCode: "L3_ALLOWED_ROUTE_FOUND",
			expectedConfidence: 85, // 40(protocol) + 20(age) + 15(AD) + 10(active)
		},
		{
			name: "Allowed - Static route",
			query: models.L3ReachabilityQuery{
				SourceIP:       "10.1.1.1",
				DestIP:         "192.168.1.100",
				SourceDevice:   "router1",
				VirtualContext: "default",
			},
			routingTable: &models.L3RoutingTable{
				DeviceID:       "router1",
				Hostname:       "R1",
				VirtualContext: "default",
				Routes: []models.L3Route{
					{
						Prefix:          "192.168.1.0/24",
						PrefixLength:    24,
						NextHop:         "10.0.0.1",
						Interface:       "GigabitEthernet0/1",
						Protocol:        "static",
						Metric:          0,
						AdminDistance:   1,
						IsActive:        true,
						IsBest:          true,
						ConfidenceScore: 85,
						Age:             500,
					},
				},
			},
			expectedOutcome:    "Allowed",
			expectedReasonCode: "L3_ALLOWED_ROUTE_FOUND",
			expectedConfidence: 75, // 35(static) + 20(age<1hr) + 15(AD) + 10(active) - 5(metric)
		},
		{
			name: "Allowed - OSPF route",
			query: models.L3ReachabilityQuery{
				SourceIP:       "10.1.1.1",
				DestIP:         "172.16.0.100",
				SourceDevice:   "router1",
				VirtualContext: "default",
			},
			routingTable: &models.L3RoutingTable{
				DeviceID:       "router1",
				Hostname:       "R1",
				VirtualContext: "default",
				Routes: []models.L3Route{
					{
						Prefix:          "172.16.0.0/16",
						PrefixLength:    16,
						NextHop:         "10.0.0.2",
						Interface:       "GigabitEthernet0/1",
						Protocol:        "ospf",
						Metric:          20,
						AdminDistance:   110,
						IsActive:        true,
						IsBest:          true,
						ConfidenceScore: 75,
						Age:             2000,
					},
				},
			},
			expectedOutcome:    "Allowed",
			expectedReasonCode: "L3_ALLOWED_ROUTE_FOUND",
			expectedConfidence: 70, // 30(ospf) + 20(age) + 10(AD<100) + 10(active)
		},
		{
			name: "Blocked - No route to destination",
			query: models.L3ReachabilityQuery{
				SourceIP:       "10.1.1.1",
				DestIP:         "203.0.113.100",
				SourceDevice:   "router1",
				VirtualContext: "default",
			},
			routingTable: &models.L3RoutingTable{
				DeviceID:       "router1",
				Hostname:       "R1",
				VirtualContext: "default",
				Routes: []models.L3Route{
					{
						Prefix:          "10.1.1.0/24",
						PrefixLength:    24,
						NextHop:         "",
						Interface:       "GigabitEthernet0/0",
						Protocol:        "connected",
						Metric:          0,
						AdminDistance:   0,
						IsActive:        true,
						IsBest:          true,
					},
				},
				DefaultRoute: nil, // No default route
			},
			expectedOutcome:    "Blocked",
			expectedReasonCode: "L3_BLOCKED_NO_ROUTE",
			expectedConfidence: 0,
		},
		{
			name: "Blocked - Null route",
			query: models.L3ReachabilityQuery{
				SourceIP:       "10.1.1.1",
				DestIP:         "192.168.99.100",
				SourceDevice:   "router1",
				VirtualContext: "default",
			},
			routingTable: &models.L3RoutingTable{
				DeviceID:       "router1",
				Hostname:       "R1",
				VirtualContext: "default",
				Routes: []models.L3Route{
					{
						Prefix:          "192.168.99.0/24",
						PrefixLength:    24,
						NextHop:         "",
						Interface:       "Null0",
						Protocol:        "static",
						Metric:          0,
						AdminDistance:   1,
						IsActive:        true,
						IsBest:          true,
					},
				},
			},
			expectedOutcome:    "Blocked",
			expectedReasonCode: "L3_BLOCKED_NULL_ROUTE",
			expectedConfidence: 0,
		},
		{
			name: "Blocked - Black hole route",
			query: models.L3ReachabilityQuery{
				SourceIP:       "10.1.1.1",
				DestIP:         "192.168.88.100",
				SourceDevice:   "router1",
				VirtualContext: "default",
			},
			routingTable: &models.L3RoutingTable{
				DeviceID:       "router1",
				Hostname:       "R1",
				VirtualContext: "default",
				Routes: []models.L3Route{
					{
						Prefix:          "192.168.88.0/24",
						PrefixLength:    24,
						NextHop:         "",
						Interface:       "",
						Protocol:        "static",
						Metric:          0,
						AdminDistance:   1,
						IsActive:        true,
						IsBest:          true,
					},
				},
			},
			expectedOutcome:    "Blocked",
			expectedReasonCode: "L3_BLOCKED_BLACK_HOLE",
			expectedConfidence: 0,
		},
		{
			name: "Allowed - Default route fallback",
			query: models.L3ReachabilityQuery{
				SourceIP:       "10.1.1.1",
				DestIP:         "8.8.8.8",
				SourceDevice:   "router1",
				VirtualContext: "default",
			},
			routingTable: &models.L3RoutingTable{
				DeviceID:       "router1",
				Hostname:       "R1",
				VirtualContext: "default",
				Routes: []models.L3Route{
					{
						Prefix:          "10.1.1.0/24",
						PrefixLength:    24,
						NextHop:         "",
						Interface:       "GigabitEthernet0/0",
						Protocol:        "connected",
						Metric:          0,
						AdminDistance:   0,
						IsActive:        true,
						IsBest:          true,
					},
				},
				DefaultRoute: &models.L3Route{
					Prefix:          "0.0.0.0/0",
					PrefixLength:    0,
					NextHop:         "10.0.0.1",
					Interface:       "GigabitEthernet0/1",
					Protocol:        "static",
					Metric:          0,
					AdminDistance:   1,
					IsActive:        true,
					IsBest:          true,
					Age:             100,
				},
			},
			expectedOutcome:    "Allowed",
			expectedReasonCode: "L3_ALLOWED_ROUTE_FOUND",
			expectedConfidence: 80, // Using default route
		},
		{
			name: "Blocked - Inactive route",
			query: models.L3ReachabilityQuery{
				SourceIP:       "10.1.1.1",
				DestIP:         "192.168.50.100",
				SourceDevice:   "router1",
				VirtualContext: "default",
			},
			routingTable: &models.L3RoutingTable{
				DeviceID:       "router1",
				Hostname:       "R1",
				VirtualContext: "default",
				Routes: []models.L3Route{
					{
						Prefix:          "192.168.50.0/24",
						PrefixLength:    24,
						NextHop:         "10.0.0.1",
						Interface:       "GigabitEthernet0/1",
						Protocol:        "static",
						Metric:          0,
						AdminDistance:   1,
						IsActive:        false, // Not in FIB
						IsBest:          false,
					},
				},
			},
			expectedOutcome:    "Blocked",
			expectedReasonCode: "L3_BLOCKED_INACTIVE_ROUTE",
			expectedConfidence: 20,
		},
		{
			name: "Allowed - BGP route",
			query: models.L3ReachabilityQuery{
				SourceIP:       "10.1.1.1",
				DestIP:         "203.0.113.100",
				SourceDevice:   "router1",
				VirtualContext: "default",
			},
			routingTable: &models.L3RoutingTable{
				DeviceID:       "router1",
				Hostname:       "R1",
				VirtualContext: "default",
				Routes: []models.L3Route{
					{
						Prefix:          "203.0.113.0/24",
						PrefixLength:    24,
						NextHop:         "10.0.0.3",
						Interface:       "GigabitEthernet0/2",
						Protocol:        "bgp",
						Metric:          0,
						AdminDistance:   200,
						IsActive:        true,
						IsBest:          true,
						ConfidenceScore: 65,
						Age:             5000,
					},
				},
			},
			expectedOutcome:    "Allowed",
			expectedReasonCode: "L3_ALLOWED_ROUTE_FOUND",
			expectedConfidence: 55, // 25(bgp) + 10(age<24hr) + 5(AD) + 10(active) + 15(best) - 10(metric)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := NewL3DecisionEngine()
			engine.LoadRoutingTable(tt.routingTable)

			result := engine.DetermineL3Reachability(tt.query)

			assert.Equal(t, tt.expectedOutcome, result.Outcome,
				"Expected outcome %s, got %s", tt.expectedOutcome, result.Outcome)
			assert.Equal(t, tt.expectedReasonCode, result.ReasonCode,
				"Expected reason code %s, got %s", tt.expectedReasonCode, result.ReasonCode)
			assert.GreaterOrEqual(t, result.ConfidenceScore, tt.expectedConfidence-10,
				"Confidence score too low")
			assert.LessOrEqual(t, result.ConfidenceScore, tt.expectedConfidence+10,
				"Confidence score too high")
		})
	}
}

func TestLongestPrefixMatch(t *testing.T) {
	engine := NewL3DecisionEngine()

	routes := []models.L3Route{
		{
			Prefix:       "0.0.0.0/0",
			PrefixLength: 0,
			NextHop:      "10.0.0.1",
			Protocol:     "static",
		},
		{
			Prefix:       "10.0.0.0/8",
			PrefixLength: 8,
			NextHop:      "10.0.0.2",
			Protocol:     "ospf",
		},
		{
			Prefix:       "10.1.0.0/16",
			PrefixLength: 16,
			NextHop:      "10.0.0.3",
			Protocol:     "ospf",
		},
		{
			Prefix:       "10.1.1.0/24",
			PrefixLength: 24,
			NextHop:      "10.0.0.4",
			Protocol:     "connected",
		},
	}

	tests := []struct {
		name           string
		destIP         string
		expectedPrefix string
		expectedNextHop string
	}{
		{
			name:           "Most specific - /24",
			destIP:         "10.1.1.100",
			expectedPrefix: "10.1.1.0/24",
			expectedNextHop: "10.0.0.4",
		},
		{
			name:           "Medium specific - /16",
			destIP:         "10.1.2.100",
			expectedPrefix: "10.1.0.0/16",
			expectedNextHop: "10.0.0.3",
		},
		{
			name:           "Broad - /8",
			destIP:         "10.5.5.100",
			expectedPrefix: "10.0.0.0/8",
			expectedNextHop: "10.0.0.2",
		},
		{
			name:           "Default route - /0",
			destIP:         "8.8.8.8",
			expectedPrefix: "0.0.0.0/0",
			expectedNextHop: "10.0.0.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route := engine.longestPrefixMatch(tt.destIP, routes)

			assert.NotNil(t, route, "Expected to find a route")
			assert.Equal(t, tt.expectedPrefix, route.Prefix,
				"Expected prefix %s, got %s", tt.expectedPrefix, route.Prefix)
			assert.Equal(t, tt.expectedNextHop, route.NextHop,
				"Expected next hop %s, got %s", tt.expectedNextHop, route.NextHop)
		})
	}
}

func TestCalculateRouteConfidence(t *testing.T) {
	engine := NewL3DecisionEngine()

	tests := []struct {
		name               string
		route              *models.L3Route
		expectedConfidence int
	}{
		{
			name: "High confidence - Connected, fresh, active, best",
			route: &models.L3Route{
				Protocol:      "connected",
				AdminDistance: 0,
				Age:           100,
				Metric:        0,
				IsActive:      true,
				IsBest:        true,
			},
			expectedConfidence: 100, // 40+20+15+10+15 = 100
		},
		{
			name: "High confidence - Static route",
			route: &models.L3Route{
				Protocol:      "static",
				AdminDistance: 1,
				Age:           500,
				Metric:        0,
				IsActive:      true,
				IsBest:        true,
			},
			expectedConfidence: 95, // 35+20+15+10+15 = 95
		},
		{
			name: "Medium confidence - OSPF route",
			route: &models.L3Route{
				Protocol:      "ospf",
				AdminDistance: 110,
				Age:           1000,
				Metric:        10,
				IsActive:      true,
				IsBest:        false,
			},
			expectedConfidence: 70, // 30+20+10+10+10 = 80 (with metric adjustment)
		},
		{
			name: "Lower confidence - BGP route",
			route: &models.L3Route{
				Protocol:      "bgp",
				AdminDistance: 200,
				Age:           10000,
				Metric:        50,
				IsActive:      true,
				IsBest:        true,
			},
			expectedConfidence: 60, // 25+10+5+10+15 = 65 (with metric adjustment)
		},
		{
			name: "Low confidence - Stale route",
			route: &models.L3Route{
				Protocol:      "ospf",
				AdminDistance: 110,
				Age:           100000, // >24 hours
				Metric:        100,
				IsActive:      false,
				IsBest:        false,
			},
			expectedConfidence: 40, // 30+0+10+0+0 = 40
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			confidence := engine.calculateRouteConfidence(tt.route)

			assert.GreaterOrEqual(t, confidence, tt.expectedConfidence-10,
				"Confidence score %d too low (expected ~%d)", confidence, tt.expectedConfidence)
			assert.LessOrEqual(t, confidence, tt.expectedConfidence+10,
				"Confidence score %d too high (expected ~%d)", confidence, tt.expectedConfidence)
		})
	}
}

func TestNullRouteDetection(t *testing.T) {
	engine := NewL3DecisionEngine()

	tests := []struct {
		name       string
		route      *models.L3Route
		expectNull bool
	}{
		{
			name: "Null route - Null0 interface",
			route: &models.L3Route{
				Interface: "Null0",
				NextHop:   "",
			},
			expectNull: true,
		},
		{
			name: "Null route - null interface (lowercase)",
			route: &models.L3Route{
				Interface: "null0",
				NextHop:   "",
			},
			expectNull: true,
		},
		{
			name: "Null route - 0.0.0.0 next hop",
			route: &models.L3Route{
				Interface: "",
				NextHop:   "0.0.0.0",
			},
			expectNull: true,
		},
		{
			name: "Normal route - valid next hop",
			route: &models.L3Route{
				Interface: "GigabitEthernet0/0",
				NextHop:   "10.0.0.1",
			},
			expectNull: false,
		},
		{
			name: "Connected route - interface only",
			route: &models.L3Route{
				Interface: "GigabitEthernet0/0",
				NextHop:   "",
				Protocol:  "connected",
			},
			expectNull: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isNull := engine.isNullRoute(tt.route)
			assert.Equal(t, tt.expectNull, isNull,
				"Expected isNullRoute=%v, got %v", tt.expectNull, isNull)
		})
	}
}

func TestBlackHoleDetection(t *testing.T) {
	engine := NewL3DecisionEngine()

	tests := []struct {
		name            string
		route           *models.L3Route
		expectBlackHole bool
	}{
		{
			name: "Black hole - No next hop or interface",
			route: &models.L3Route{
				Interface: "",
				NextHop:   "",
				Protocol:  "static",
			},
			expectBlackHole: true,
		},
		{
			name: "Black hole - Drop interface",
			route: &models.L3Route{
				Interface: "drop",
				NextHop:   "",
			},
			expectBlackHole: true,
		},
		{
			name: "Not black hole - Connected route",
			route: &models.L3Route{
				Interface: "GigabitEthernet0/0",
				NextHop:   "",
				Protocol:  "connected",
			},
			expectBlackHole: false,
		},
		{
			name: "Not black hole - Valid next hop",
			route: &models.L3Route{
				Interface: "GigabitEthernet0/0",
				NextHop:   "10.0.0.1",
			},
			expectBlackHole: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isBlackHole := engine.isBlackHole(tt.route)
			assert.Equal(t, tt.expectBlackHole, isBlackHole,
				"Expected isBlackHole=%v, got %v", tt.expectBlackHole, isBlackHole)
		})
	}
}

func TestCaching(t *testing.T) {
	engine := NewL3DecisionEngine()
	engine.SetCacheTTL(1 * time.Second)

	routingTable := &models.L3RoutingTable{
		DeviceID:       "router1",
		Hostname:       "R1",
		VirtualContext: "default",
		Routes: []models.L3Route{
			{
				Prefix:          "10.1.1.0/24",
				PrefixLength:    24,
				NextHop:         "",
				Interface:       "GigabitEthernet0/0",
				Protocol:        "connected",
				Metric:          0,
				AdminDistance:   0,
				IsActive:        true,
				IsBest:          true,
			},
		},
	}

	engine.LoadRoutingTable(routingTable)

	query := models.L3ReachabilityQuery{
		SourceIP:       "10.1.1.1",
		DestIP:         "10.1.1.100",
		SourceDevice:   "router1",
		VirtualContext: "default",
	}

	// First call - should compute
	result1 := engine.DetermineL3Reachability(query)
	assert.Equal(t, "Allowed", result1.Outcome)

	// Second call - should use cache
	result2 := engine.DetermineL3Reachability(query)
	assert.Equal(t, "Allowed", result2.Outcome)
	assert.Less(t, result2.DecisionTime, result1.DecisionTime,
		"Cached decision should be faster")

	// Wait for cache to expire
	time.Sleep(1500 * time.Millisecond)

	// Third call - cache expired, should recompute
	result3 := engine.DetermineL3Reachability(query)
	assert.Equal(t, "Allowed", result3.Outcome)
}

func TestConfigurationOptions(t *testing.T) {
	engine := NewL3DecisionEngine()

	// Test SetAllowedThreshold
	engine.SetAllowedThreshold(80)
	assert.Equal(t, 80, engine.allowedThreshold)

	// Test SetCacheTTL
	engine.SetCacheTTL(10 * time.Minute)
	assert.Equal(t, 10*time.Minute, engine.cacheTTL)

	// Test SetRequireActiveRoute
	engine.SetRequireActiveRoute(false)
	assert.False(t, engine.requireActiveRoute)

	// Test SetMaxHops
	engine.SetMaxHops(30)
	assert.Equal(t, 30, engine.maxHops)
}

// Benchmark tests
func BenchmarkDetermineL3Reachability(b *testing.B) {
	engine := NewL3DecisionEngine()

	routingTable := &models.L3RoutingTable{
		DeviceID:       "router1",
		Hostname:       "R1",
		VirtualContext: "default",
		Routes: []models.L3Route{
			{
				Prefix:          "10.1.1.0/24",
				PrefixLength:    24,
				NextHop:         "",
				Interface:       "GigabitEthernet0/0",
				Protocol:        "connected",
				AdminDistance:   0,
				IsActive:        true,
				IsBest:          true,
			},
		},
	}

	engine.LoadRoutingTable(routingTable)

	query := models.L3ReachabilityQuery{
		SourceIP:       "10.1.1.1",
		DestIP:         "10.1.1.100",
		SourceDevice:   "router1",
		VirtualContext: "default",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.DetermineL3Reachability(query)
	}
}

func BenchmarkLongestPrefixMatch(b *testing.B) {
	engine := NewL3DecisionEngine()

	routes := []models.L3Route{}
	for i := 0; i < 100; i++ {
		routes = append(routes, models.L3Route{
			Prefix:       "10.0.0.0/8",
			PrefixLength: 8,
			NextHop:      "10.0.0.1",
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.longestPrefixMatch("10.1.1.100", routes)
	}
}

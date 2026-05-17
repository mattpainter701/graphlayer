package fortinet

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/mattpainter701/graphlayer/platform/models"
)

// ========================================================================
// Phase 3: FortiGate L3 Routing Parsers
// ========================================================================
//
// FortiOS Version Support:
// - FortiOS 6.0.x (REST API v2 introduced)
// - FortiOS 6.2.x
// - FortiOS 6.4.x (LTS - Long Term Support)
// - FortiOS 7.0.x
// - FortiOS 7.2.x (LTS)
// - FortiOS 7.4.x
// - FortiOS 7.6.x (latest as of 2025)
//
// API Endpoints:
// - Static Routes: GET /api/v2/cmdb/router/static
// - OSPF Config: GET /api/v2/cmdb/router/ospf
// - OSPF Monitor: GET /api/v2/monitor/router/ospf/neighbors
// - BGP Config: GET /api/v2/cmdb/router/bgp
// - BGP Monitor: GET /api/v2/monitor/router/bgp/neighbors
// - Routing Table: GET /api/v2/monitor/router/ipv4
//
// Required Permissions: Super Admin or read-only admin with routing scope
// VDOM Support: Yes (multi-VDOM environments supported)
//
// ========================================================================

// ParserContext provides context for parsing operations
type ParserContext struct {
	Vendor        string       // "fortinet"
	Platform      string       // "fortigate"
	OSVersion     string       // e.g., "7.2.5"
	Command       string       // API endpoint
	CommandOutput string       // Raw JSON response
	Logger        *slog.Logger // Structured logger
}

// ========================================================================
// Static Route Parsing
// ========================================================================

// FortiGateStaticRoute represents a FortiGate static route
// API: GET /api/v2/cmdb/router/static
type FortiGateStaticRoute struct {
	SeqNum      int      `json:"seq-num"`
	Status      string   `json:"status"` // "enable" or "disable"
	Destination string   `json:"dst"`    // CIDR format
	Gateway     string   `json:"gateway"`
	Distance    int      `json:"distance"`
	Weight      int      `json:"weight"`
	Priority    int      `json:"priority"`
	Device      string   `json:"device"`    // Egress interface
	Comment     string   `json:"comment"`
	VRF         int      `json:"vrf"`       // VRF ID
	BFD         string   `json:"bfd"`       // "enable" or "disable"
	Blackhole   string   `json:"blackhole"` // "enable" or "disable"
}

// ParseFortiGateStaticRoutes parses static routes from FortiGate REST API
// Command: GET /api/v2/cmdb/router/static?vdom={vdom_name}
// FortiOS 6.0+
func ParseFortiGateStaticRoutes(jsonData string) ([]models.L3Route, error) {
	var response struct {
		HTTPMethod string `json:"http_method"`
		Results    []FortiGateStaticRoute `json:"results"`
		VDOM       string `json:"vdom"`
		Path       string `json:"path"`
		Name       string `json:"name"`
		Status     string `json:"status"`
		Build      int    `json:"build"`
	}

	if err := json.Unmarshal([]byte(jsonData), &response); err != nil {
		return nil, fmt.Errorf("failed to parse FortiGate static routes JSON: %w", err)
	}

	if response.Status != "success" {
		return nil, fmt.Errorf("FortiGate API returned non-success status: %s", response.Status)
	}

	routes := make([]models.L3Route, 0, len(response.Results))

	for _, fgRoute := range response.Results {
		// Skip disabled routes
		if fgRoute.Status != "enable" {
			continue
		}

		// Parse prefix and prefix length
		prefix := fgRoute.Destination
		prefixLen := getPrefixLengthFromCIDR(prefix)

		// Determine if black hole
		isBlackhole := fgRoute.Blackhole == "enable"

		route := models.L3Route{
			Prefix:            prefix,
			PrefixLength:      prefixLen,
			NextHop:           fgRoute.Gateway,
			Interface:         fgRoute.Device,
			Protocol:          "static",
			Metric:            fgRoute.Weight,
			AdminDistance:     fgRoute.Distance,
			IsActive:          fgRoute.Status == "enable",
			IsBest:            true, // Static routes are typically best
			Description:       fgRoute.Comment,
			ConfidenceScore:   85, // High confidence for static routes
			EvidenceSources:   []string{"fortigate_rest_api"},
			LastSeen:          time.Now(),
		}

		// Handle black hole routes
		if isBlackhole {
			route.Interface = "blackhole"
			route.NextHop = ""
		}

		routes = append(routes, route)
	}

	return routes, nil
}

// ParseFortiGateRoutingTable parses the active routing table
// Command: GET /api/v2/monitor/router/ipv4
// FortiOS 6.0+
func ParseFortiGateRoutingTable(jsonData string) ([]models.L3Route, error) {
	var response struct {
		HTTPMethod string `json:"http_method"`
		Results    []struct {
			IPVersion   string `json:"ip_version"`
			IPMask      string `json:"ip_mask"` // "192.168.1.0/24"
			Type        string `json:"type"`     // "static", "connected", "ospf", "bgp"
			Protocol    string `json:"protocol"`
			Distance    int    `json:"distance"`
			Metric      int    `json:"metric"`
			Gateway     string `json:"gateway"`
			Interface   string `json:"interface"`
			Age         int    `json:"age"` // Seconds
			BestRoute   string `json:"best"` // "*" if best route
		} `json:"results"`
		VDOM   string `json:"vdom"`
		Path   string `json:"path"`
		Name   string `json:"name"`
		Status string `json:"status"`
		Build  int    `json:"build"`
	}

	if err := json.Unmarshal([]byte(jsonData), &response); err != nil {
		return nil, fmt.Errorf("failed to parse FortiGate routing table JSON: %w", err)
	}

	if response.Status != "success" {
		return nil, fmt.Errorf("FortiGate API returned non-success status: %s", response.Status)
	}

	routes := make([]models.L3Route, 0, len(response.Results))

	for _, fgRoute := range response.Results {
		prefix := fgRoute.IPMask
		prefixLen := getPrefixLengthFromCIDR(prefix)

		route := models.L3Route{
			Prefix:          prefix,
			PrefixLength:    prefixLen,
			NextHop:         fgRoute.Gateway,
			Interface:       fgRoute.Interface,
			Protocol:        normalizeProtocol(fgRoute.Type),
			Metric:          fgRoute.Metric,
			AdminDistance:   fgRoute.Distance,
			Age:             fgRoute.Age,
			IsActive:        true, // Routes in FIB are active
			IsBest:          fgRoute.BestRoute == "*",
			ConfidenceScore: calculateRouteConfidenceFromProtocol(normalizeProtocol(fgRoute.Type)),
			EvidenceSources: []string{"fortigate_routing_table"},
			LastSeen:        time.Now(),
		}

		routes = append(routes, route)
	}

	return routes, nil
}

// ========================================================================
// OSPF Parsing
// ========================================================================

// FortiGateOSPFConfig represents OSPF configuration
// API: GET /api/v2/cmdb/router/ospf
type FortiGateOSPFConfig struct {
	RouterID          string `json:"router-id"`
	DefaultInfoMetric int    `json:"default-information-metric"`
	DefaultInfoMetricType string `json:"default-information-metric-type"`
	DefaultInfoOriginate  string `json:"default-information-originate"` // "enable" or "disable"
	Areas             []FortiGateOSPFArea `json:"area"`
	OSPFInterface     []FortiGateOSPFInterface `json:"ospf-interface"`
	Network           []FortiGateOSPFNetwork `json:"network"`
	Redistribute      []FortiGateOSPFRedistribute `json:"redistribute"`
	PassiveInterface  []struct {
		Name string `json:"name"`
	} `json:"passive-interface"`
}

// FortiGateOSPFArea represents an OSPF area
type FortiGateOSPFArea struct {
	ID                string `json:"id"` // Area ID (e.g., "0.0.0.0")
	Type              string `json:"type"` // "regular", "stub", "nssa"
	StubNoSummary     string `json:"nssa-no-summary"` // "enable" or "disable"
	NSSADefaultInfoOriginate string `json:"nssa-default-information-originate"`
	Authentication    string `json:"authentication"` // "none", "text", "md5"
}

// FortiGateOSPFInterface represents an OSPF-enabled interface
type FortiGateOSPFInterface struct {
	Name           string `json:"name"`
	Interface      string `json:"interface"`
	IP             string `json:"ip"`
	Cost           int    `json:"cost"`
	Priority       int    `json:"priority"`
	HelloInterval  int    `json:"hello-interval"`
	DeadInterval   int    `json:"dead-interval"`
	RetransmitInterval int `json:"retransmit-interval"`
	NetworkType    string `json:"network-type"` // "broadcast", "point-to-point"
	Authentication string `json:"authentication"`
}

// FortiGateOSPFNetwork represents a network advertised into OSPF
type FortiGateOSPFNetwork struct {
	ID     int    `json:"id"`
	Prefix string `json:"prefix"` // CIDR format
	Area   string `json:"area"`   // Area ID
}

// FortiGateOSPFRedistribute represents redistribution config
type FortiGateOSPFRedistribute struct {
	Name       string `json:"name"` // "connected", "static", "bgp"
	Status     string `json:"status"` // "enable" or "disable"
	Metric     int    `json:"metric"`
	MetricType int    `json:"metric-type"` // 1 or 2
	RouteMap   string `json:"routemap"`
}

// ParseFortiGateOSPFConfig parses OSPF configuration
// Command: GET /api/v2/cmdb/router/ospf?vdom={vdom_name}
// FortiOS 6.0+
func ParseFortiGateOSPFConfig(jsonData string) (*models.OSPFProcess, error) {
	var response struct {
		HTTPMethod string `json:"http_method"`
		Results    FortiGateOSPFConfig `json:"results"`
		VDOM       string `json:"vdom"`
		Path       string `json:"path"`
		Name       string `json:"name"`
		Status     string `json:"status"`
		Build      int    `json:"build"`
	}

	if err := json.Unmarshal([]byte(jsonData), &response); err != nil {
		return nil, fmt.Errorf("failed to parse FortiGate OSPF config JSON: %w", err)
	}

	if response.Status != "success" {
		return nil, fmt.Errorf("FortiGate API returned non-success status: %s", response.Status)
	}

	ospf := response.Results

	process := &models.OSPFProcess{
		ProcessID:       1, // FortiGate uses single OSPF process
		RouterID:        ospf.RouterID,
		Areas:           make([]models.OSPFArea, 0, len(ospf.Areas)),
		Networks:        make([]models.OSPFNetwork, 0, len(ospf.Network)),
		Interfaces:      make([]models.OSPFInterface, 0, len(ospf.OSPFInterface)),
		PassiveInterfaces: make([]string, 0, len(ospf.PassiveInterface)),
		RedistributedProtocols: make([]string, 0),
		DefaultOriginate: ospf.DefaultInfoOriginate == "enable",
		ConfigHash:      computeSnippetHash([]string{jsonData}),
		InsertedTime:    time.Now(),
		LastModified:    time.Now(),
		Active:          true,
	}

	// Map areas
	for _, area := range ospf.Areas {
		areaType := "normal"
		switch area.Type {
		case "stub":
			areaType = "stub"
		case "nssa":
			areaType = "nssa"
		}

		processArea := models.OSPFArea{
			AreaID:        area.ID,
			AreaType:      areaType,
			AuthType:      area.Authentication,
			StubNoSummary: area.StubNoSummary == "enable",
			NSSANoRedist:  area.NSSADefaultInfoOriginate == "enable",
		}
		process.Areas = append(process.Areas, processArea)
	}

	// Map networks
	for _, network := range ospf.Network {
		processNetwork := models.OSPFNetwork{
			Network: network.Prefix,
			AreaID:  network.Area,
		}
		process.Networks = append(process.Networks, processNetwork)
	}

	// Map interfaces
	for _, iface := range ospf.OSPFInterface {
		processInterface := models.OSPFInterface{
			InterfaceName:  iface.Interface,
			IPAddress:      iface.IP,
			Cost:           iface.Cost,
			Priority:       iface.Priority,
			HelloInterval:  iface.HelloInterval,
			DeadInterval:   iface.DeadInterval,
			RetransmitTime: iface.RetransmitInterval,
			NetworkType:    iface.NetworkType,
			AuthType:       iface.Authentication,
		}
		process.Interfaces = append(process.Interfaces, processInterface)
	}

	// Map passive interfaces
	for _, passive := range ospf.PassiveInterface {
		process.PassiveInterfaces = append(process.PassiveInterfaces, passive.Name)
	}

	// Map redistribution
	for _, redist := range ospf.Redistribute {
		if redist.Status == "enable" {
			process.RedistributedProtocols = append(process.RedistributedProtocols, redist.Name)
		}
	}

	return process, nil
}

// FortiGateOSPFNeighbor represents an OSPF neighbor
// API: GET /api/v2/monitor/router/ospf/neighbors
type FortiGateOSPFNeighbor struct {
	NeighborID string `json:"nbrRtrId"`
	Priority   int    `json:"nbrPrio"`
	State      string `json:"state"` // "Full", "2-Way", "Exstart"
	DeadTime   int    `json:"deadTime"` // Seconds
	Address    string `json:"address"`
	Interface  string `json:"interface"`
	Area       string `json:"areaId"`
}

// ParseFortiGateOSPFNeighbors parses OSPF neighbor table
// Command: GET /api/v2/monitor/router/ospf/neighbors?vdom={vdom_name}
// FortiOS 6.0+
func ParseFortiGateOSPFNeighbors(jsonData string) ([]models.OSPFNeighbor, error) {
	var response struct {
		HTTPMethod string `json:"http_method"`
		Results    []FortiGateOSPFNeighbor `json:"results"`
		VDOM       string `json:"vdom"`
		Path       string `json:"path"`
		Name       string `json:"name"`
		Status     string `json:"status"`
		Build      int    `json:"build"`
	}

	if err := json.Unmarshal([]byte(jsonData), &response); err != nil {
		return nil, fmt.Errorf("failed to parse FortiGate OSPF neighbors JSON: %w", err)
	}

	if response.Status != "success" {
		return nil, fmt.Errorf("FortiGate API returned non-success status: %s", response.Status)
	}

	neighbors := make([]models.OSPFNeighbor, 0, len(response.Results))

	for _, fgNeighbor := range response.Results {
		neighbor := models.OSPFNeighbor{
			NeighborID: fgNeighbor.NeighborID,
			IPAddress:  fgNeighbor.Address,
			Interface:  fgNeighbor.Interface,
			State:      fgNeighbor.State,
			Priority:   fgNeighbor.Priority,
			DeadTime:   fgNeighbor.DeadTime,
			AreaID:     fgNeighbor.Area,
			LastChange: time.Now(),
		}
		neighbors = append(neighbors, neighbor)
	}

	return neighbors, nil
}

// ========================================================================
// BGP Parsing
// ========================================================================

// FortiGateBGPConfig represents BGP configuration
// API: GET /api/v2/cmdb/router/bgp
type FortiGateBGPConfig struct {
	AS                int    `json:"as"`
	RouterID          string `json:"router-id"`
	Neighbor          []FortiGateBGPNeighborConfig `json:"neighbor"`
	Network           []FortiGateBGPNetwork `json:"network"`
	AggregateAddress  []FortiGateBGPAggregate `json:"aggregate-address"`
	Redistribute      []FortiGateBGPRedistribute `json:"redistribute"`
	DefaultLocalPref  int    `json:"default-local-preference"`
	AlwaysCompareMED  string `json:"always-compare-med"` // "enable" or "disable"
	DeterministicMED  string `json:"deterministic-med"`
	RouteReflectorID  string `json:"cluster-id"`
}

// FortiGateBGPNeighborConfig represents BGP neighbor configuration
type FortiGateBGPNeighborConfig struct {
	IP                  string `json:"ip"`
	RemoteAS            int    `json:"remote-as"`
	UpdateSource        string `json:"update-source"`
	NextHopSelf         string `json:"next-hop-self"` // "enable" or "disable"
	RouteReflectorClient string `json:"route-reflector-client"`
	Description         string `json:"description"`
	Password            string `json:"password"`
	KeepAliveTimer      int    `json:"keep-alive-timer"`
	HoldtimeTimer       int    `json:"holdtime-timer"`
	EBGPMultihopTTL     int    `json:"ebgp-multihop-ttl"`
}

// FortiGateBGPNetwork represents a BGP network advertisement
type FortiGateBGPNetwork struct {
	ID     int    `json:"id"`
	Prefix string `json:"prefix"` // CIDR format
}

// FortiGateBGPAggregate represents a BGP aggregate address
type FortiGateBGPAggregate struct {
	ID          int    `json:"id"`
	Prefix      string `json:"prefix"`
	SummaryOnly string `json:"summary-only"` // "enable" or "disable"
	AsSet       string `json:"as-set"` // "enable" or "disable"
}

// FortiGateBGPRedistribute represents BGP redistribution config
type FortiGateBGPRedistribute struct {
	Name     string `json:"name"` // "connected", "static", "ospf"
	Status   string `json:"status"` // "enable" or "disable"
	RouteMap string `json:"route-map"`
}

// ParseFortiGateBGPConfig parses BGP configuration
// Command: GET /api/v2/cmdb/router/bgp?vdom={vdom_name}
// FortiOS 6.0+
func ParseFortiGateBGPConfig(jsonData string) (*models.BGPProcess, error) {
	var response struct {
		HTTPMethod string `json:"http_method"`
		Results    FortiGateBGPConfig `json:"results"`
		VDOM       string `json:"vdom"`
		Path       string `json:"path"`
		Name       string `json:"name"`
		Status     string `json:"status"`
		Build      int    `json:"build"`
	}

	if err := json.Unmarshal([]byte(jsonData), &response); err != nil {
		return nil, fmt.Errorf("failed to parse FortiGate BGP config JSON: %w", err)
	}

	if response.Status != "success" {
		return nil, fmt.Errorf("FortiGate API returned non-success status: %s", response.Status)
	}

	bgp := response.Results

	process := &models.BGPProcess{
		LocalAS:          bgp.AS,
		RouterID:         bgp.RouterID,
		Neighbors:        make([]models.BGPNeighbor, 0, len(bgp.Neighbor)),
		Networks:         make([]models.BGPNetwork, 0, len(bgp.Network)),
		AggregateAddrs:   make([]models.BGPAggregate, 0, len(bgp.AggregateAddress)),
		RedistributedProtocols: make([]string, 0),
		RouteReflector:   bgp.RouteReflectorID != "",
		ClusterID:        bgp.RouteReflectorID,
		ConfigHash:       computeSnippetHash([]string{jsonData}),
		InsertedTime:     time.Now(),
		LastModified:     time.Now(),
		Active:           true,
	}

	// Map neighbors
	for _, neighbor := range bgp.Neighbor {
		processNeighbor := models.BGPNeighbor{
			IPAddress:            neighbor.IP,
			RemoteAS:             neighbor.RemoteAS,
			Description:          neighbor.Description,
			UpdateSource:         neighbor.UpdateSource,
			NextHopSelf:          neighbor.NextHopSelf == "enable",
			RouteReflectorClient: neighbor.RouteReflectorClient == "enable",
			EBGP:                 neighbor.RemoteAS != bgp.AS,
			Multihop:             neighbor.EBGPMultihopTTL,
			PasswordConfigured:   neighbor.Password != "",
			HoldTime:             neighbor.HoldtimeTimer,
			KeepAlive:            neighbor.KeepAliveTimer,
		}
		process.Neighbors = append(process.Neighbors, processNeighbor)
	}

	// Map networks
	for _, network := range bgp.Network {
		processNetwork := models.BGPNetwork{
			Network: network.Prefix,
		}
		process.Networks = append(process.Networks, processNetwork)
	}

	// Map aggregates
	for _, agg := range bgp.AggregateAddress {
		processAggregate := models.BGPAggregate{
			Network:     agg.Prefix,
			SummaryOnly: agg.SummaryOnly == "enable",
			AsSet:       agg.AsSet == "enable",
		}
		process.AggregateAddrs = append(process.AggregateAddrs, processAggregate)
	}

	// Map redistribution
	for _, redist := range bgp.Redistribute {
		if redist.Status == "enable" {
			process.RedistributedProtocols = append(process.RedistributedProtocols, redist.Name)
		}
	}

	return process, nil
}

// ========================================================================
// Helper Functions
// ========================================================================

// getPrefixLengthFromCIDR extracts prefix length from CIDR notation
func getPrefixLengthFromCIDR(cidr string) int {
	parts := strings.Split(cidr, "/")
	if len(parts) != 2 {
		return 0
	}
	prefixLen, _ := strconv.Atoi(parts[1])
	return prefixLen
}

// normalizeProtocol converts FortiGate protocol names to standard names
func normalizeProtocol(fgProtocol string) string {
	protocolMap := map[string]string{
		"static":    "static",
		"connected": "connected",
		"ospf":      "ospf",
		"bgp":       "bgp",
		"rip":       "rip",
		"isis":      "isis",
	}

	normalized := strings.ToLower(fgProtocol)
	if protocol, found := protocolMap[normalized]; found {
		return protocol
	}
	return "unknown"
}

// calculateRouteConfidenceFromProtocol returns confidence score for protocol
func calculateRouteConfidenceFromProtocol(protocol string) int {
	scores := map[string]int{
		"connected": 95,
		"static":    85,
		"ospf":      75,
		"bgp":       65,
		"rip":       50,
		"isis":      75,
	}

	if score, found := scores[protocol]; found {
		return score
	}
	return 40
}

// computeSnippetHash computes SHA-256 hash of config snippet
// Imported from shared evidence hashing utilities
func computeSnippetHash(lines []string) string {
	// TODO: Import from platform/vendors/shared/evidence_hashing.go
	// For now, return placeholder
	return "sha256:placeholder"
}

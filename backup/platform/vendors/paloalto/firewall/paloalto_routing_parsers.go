package paloalto

import (
	"encoding/xml"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/mattpainter701/graphlayer/platform/platform/models"
)

// ========================================================================
// Phase 3: Palo Alto PAN-OS L3 Routing Parsers
// ========================================================================
//
// PAN-OS Version Support:
// - PAN-OS 8.1.x (minimum for modern API)
// - PAN-OS 9.0.x
// - PAN-OS 9.1.x
// - PAN-OS 10.0.x
// - PAN-OS 10.1.x (LTS - Long Term Support)
// - PAN-OS 10.2.x
// - PAN-OS 11.0.x (latest as of 2025)
// - PAN-OS 11.1.x (latest)
//
// API Type: XML API (not REST - REST API is for Panorama only)
// Required: API key with "show" permissions
// Virtual Router Support: Yes (multiple VRs per device)
//
// XML API Commands:
// - Static Routes: <show><config><running><xpath>virtual-router/static-route</xpath></running></config></show>
// - Routing Table: <show><routing><route></route></routing></show>
// - OSPF Config: <show><routing><protocol><ospf></ospf></protocol></routing></show>
// - BGP Config: <show><routing><protocol><bgp></bgp></protocol></routing></show>
//
// ========================================================================

// ParserContext provides context for parsing operations
type PANOSParserContext struct {
	Vendor        string       // "paloalto"
	Platform      string       // "panos"
	OSVersion     string       // e.g., "10.1.3"
	Command       string       // XML API command
	CommandOutput string       // Raw XML response
	Logger        *slog.Logger // Structured logger
}

// ========================================================================
// Static Route Parsing
// ========================================================================

// PANOSStaticRoute represents a PAN-OS static route
// XML Path: config/devices/entry/network/virtual-router/entry/routing-table/static-route/entry
type PANOSStaticRoute struct {
	XMLName     xml.Name `xml:"entry"`
	Name        string   `xml:"name,attr"`
	Destination string   `xml:"destination"`
	Nexthop     struct {
		IPAddress    string `xml:"ip-address"`
		NextVR       string `xml:"next-vr"`
		Discard      string `xml:"discard"` // Empty or present
	} `xml:"nexthop"`
	Interface   string `xml:"interface"`
	Metric      int    `xml:"metric"`
	AdminDist   int    `xml:"admin-dist"`
	RouteTable  string `xml:"route-table"` // "unicast", "multicast", "both"
	Description string `xml:"description"`
	BFDProfile  string `xml:"bfd>profile"`
}

// PANOSStaticRouteConfig wraps static route configuration
type PANOSStaticRouteConfig struct {
	XMLName xml.Name `xml:"response"`
	Status  string   `xml:"status,attr"`
	Result  struct {
		Entry []PANOSStaticRoute `xml:"entry"`
	} `xml:"result"`
}

// ParsePANOSStaticRoutes parses static routes from PAN-OS XML API
// Command: <show><config><running><xpath>virtual-router/entry[@name='default']/routing-table/static-route</xpath></running></config></show>
// PAN-OS 8.1+
func ParsePANOSStaticRoutes(xmlData string) ([]models.L3Route, error) {
	var config PANOSStaticRouteConfig
	if err := xml.Unmarshal([]byte(xmlData), &config); err != nil {
		return nil, fmt.Errorf("failed to parse PAN-OS static routes XML: %w", err)
	}

	if config.Status != "success" {
		return nil, fmt.Errorf("PAN-OS API returned non-success status: %s", config.Status)
	}

	routes := make([]models.L3Route, 0, len(config.Result.Entry))

	for _, panosRoute := range config.Result.Entry {
		// Determine next hop
		nextHop := panosRoute.Nexthop.IPAddress
		iface := panosRoute.Interface
		isDiscard := panosRoute.Nexthop.Discard != ""

		// Determine admin distance (default: 10 for static routes)
		adminDist := panosRoute.AdminDist
		if adminDist == 0 {
			adminDist = 10 // PAN-OS default
		}

		route := models.L3Route{
			Prefix:          panosRoute.Destination,
			PrefixLength:    getPrefixLengthFromCIDR(panosRoute.Destination),
			NextHop:         nextHop,
			Interface:       iface,
			Protocol:        "static",
			Metric:          panosRoute.Metric,
			AdminDistance:   adminDist,
			IsActive:        true, // Assume active until verified
			IsBest:          true, // Static routes are typically best
			RouteName:       panosRoute.Name,
			Description:     panosRoute.Description,
			ConfidenceScore: 85, // High confidence for static routes
			EvidenceSources: []string{"panos_xml_api"},
			LastSeen:        time.Now(),
		}

		// Handle discard/null routes
		if isDiscard {
			route.Interface = "discard"
			route.NextHop = ""
		}

		routes = append(routes, route)
	}

	return routes, nil
}

// PANOSRIBEntry represents a routing table entry
// XML Path: show/routing/route/entry
type PANOSRIBEntry struct {
	XMLName       xml.Name `xml:"entry"`
	Destination   string   `xml:"destination"`
	Nexthop       string   `xml:"nexthop"`
	Metric        int      `xml:"metric"`
	Flags         string   `xml:"flags"` // "A" = active, "C" = connect, "S" = static, "O" = OSPF, "B" = BGP
	Age           int      `xml:"age"` // Seconds
	Interface     string   `xml:"interface"`
	RouteTable    string   `xml:"route-table"` // "unicast"
	VirtualRouter string   `xml:"virtual-router"`
}

// PANOSRoutingTableResponse wraps routing table response
type PANOSRoutingTableResponse struct {
	XMLName xml.Name `xml:"response"`
	Status  string   `xml:"status,attr"`
	Result  struct {
		Entry []PANOSRIBEntry `xml:"entry"`
	} `xml:"result"`
}

// ParsePANOSRoutingTable parses the active routing table
// Command: <show><routing><route></route></routing></show>
// PAN-OS 8.1+
func ParsePANOSRoutingTable(xmlData string) ([]models.L3Route, error) {
	var response PANOSRoutingTableResponse
	if err := xml.Unmarshal([]byte(xmlData), &response); err != nil {
		return nil, fmt.Errorf("failed to parse PAN-OS routing table XML: %w", err)
	}

	if response.Status != "success" {
		return nil, fmt.Errorf("PAN-OS API returned non-success status: %s", response.Status)
	}

	routes := make([]models.L3Route, 0, len(response.Result.Entry))

	for _, ribEntry := range response.Result.Entry {
		// Parse protocol from flags
		protocol := parseProtocolFromFlags(ribEntry.Flags)

		// Parse admin distance from protocol
		adminDist := getDefaultAdminDistance(protocol)

		route := models.L3Route{
			Prefix:          ribEntry.Destination,
			PrefixLength:    getPrefixLengthFromCIDR(ribEntry.Destination),
			NextHop:         ribEntry.Nexthop,
			Interface:       ribEntry.Interface,
			Protocol:        protocol,
			Metric:          ribEntry.Metric,
			AdminDistance:   adminDist,
			Age:             ribEntry.Age,
			IsActive:        strings.Contains(ribEntry.Flags, "A"),
			IsBest:          strings.Contains(ribEntry.Flags, "A"), // Active routes are best
			ConfidenceScore: calculateRouteConfidenceFromProtocol(protocol),
			EvidenceSources: []string{"panos_routing_table"},
			LastSeen:        time.Now(),
		}

		routes = append(routes, route)
	}

	return routes, nil
}

// ========================================================================
// OSPF Parsing
// ========================================================================

// PANOSOSPFConfig represents OSPF configuration
// XML Path: config/devices/entry/network/virtual-router/entry/protocol/ospf
type PANOSOSPFConfig struct {
	XMLName  xml.Name `xml:"ospf"`
	Enable   string   `xml:"enable"` // "yes" or "no"
	RouterID string   `xml:"router-id"`
	Area     []PANOSOSPFArea `xml:"area>entry"`
	ExportRules []PANOSOSPFExportRule `xml:"export-rules>entry"`
}

// PANOSOSPFArea represents an OSPF area
type PANOSOSPFArea struct {
	XMLName xml.Name `xml:"entry"`
	Name    string   `xml:"name,attr"` // Area ID
	Type    struct {
		Normal string `xml:"normal"` // Empty or present
		Stub   struct {
			DefaultRoute string `xml:"default-route"` // "disable", "advertise"
			AcceptSummary string `xml:"accept-summary"` // "yes" or "no"
		} `xml:"stub"`
		NSSA struct {
			DefaultRoute string `xml:"default-route"`
			AcceptSummary string `xml:"accept-summary"`
		} `xml:"nssa"`
	} `xml:"type"`
	Interface []PANOSOSPFInterface `xml:"interface>entry"`
	Range     []PANOSOSPFRange `xml:"range>entry"`
}

// PANOSOSPFInterface represents an OSPF-enabled interface
type PANOSOSPFInterface struct {
	XMLName          xml.Name `xml:"entry"`
	Name             string   `xml:"name,attr"` // Interface name
	Enable           string   `xml:"enable"` // "yes" or "no"
	Passive          string   `xml:"passive"` // "yes" or "no"
	LinkType         string   `xml:"link-type"` // "broadcast", "p2p"
	Metric           int      `xml:"metric"`
	Priority         int      `xml:"priority"`
	HelloInterval    int      `xml:"hello-interval"`
	DeadCounts       int      `xml:"dead-counts"`
	RetransmitInterval int    `xml:"retransmit-interval"`
	TransitDelay     int      `xml:"transit-delay"`
	GrDelay          int      `xml:"gr-delay"`
	Authentication   struct {
		None     string `xml:"none"` // Empty or present
		Password string `xml:"password"`
		MD5      []struct {
			KeyID int    `xml:"keyid,attr"`
			Key   string `xml:"key"`
		} `xml:"md5>entry"`
	} `xml:"authentication"`
}

// PANOSOSPFRange represents an OSPF area range (summary)
type PANOSOSPFRange struct {
	XMLName xml.Name `xml:"entry"`
	Name    string   `xml:"name,attr"` // CIDR
	Action  string   `xml:"action"` // "advertise" or "suppress"
}

// PANOSOSPFExportRule represents OSPF redistribution
type PANOSOSPFExportRule struct {
	XMLName xml.Name `xml:"entry"`
	Name    string   `xml:"name,attr"`
	NewPathType string `xml:"new-path-type"` // "ext-1" or "ext-2"
	NewTag      int    `xml:"new-tag"`
	Metric      int    `xml:"metric"`
}

// PANOSOSPFConfigResponse wraps OSPF config response
type PANOSOSPFConfigResponse struct {
	XMLName xml.Name `xml:"response"`
	Status  string   `xml:"status,attr"`
	Result  struct {
		OSPF PANOSOSPFConfig `xml:"ospf"`
	} `xml:"result"`
}

// ParsePANOSOSPFConfig parses OSPF configuration
// Command: <show><config><running><xpath>virtual-router/entry[@name='default']/protocol/ospf</xpath></running></config></show>
// PAN-OS 8.1+
func ParsePANOSOSPFConfig(xmlData string) (*models.OSPFProcess, error) {
	var response PANOSOSPFConfigResponse
	if err := xml.Unmarshal([]byte(xmlData), &response); err != nil {
		return nil, fmt.Errorf("failed to parse PAN-OS OSPF config XML: %w", err)
	}

	if response.Status != "success" {
		return nil, fmt.Errorf("PAN-OS API returned non-success status: %s", response.Status)
	}

	ospf := response.Result.OSPF

	// Check if OSPF is enabled
	if ospf.Enable != "yes" {
		return nil, fmt.Errorf("OSPF is disabled")
	}

	process := &models.OSPFProcess{
		ProcessID:       1, // PAN-OS uses single OSPF process per VR
		RouterID:        ospf.RouterID,
		Areas:           make([]models.OSPFArea, 0, len(ospf.Area)),
		Networks:        make([]models.OSPFNetwork, 0),
		Interfaces:      make([]models.OSPFInterface, 0),
		PassiveInterfaces: make([]string, 0),
		RedistributedProtocols: make([]string, 0),
		ConfigHash:      computeSnippetHash([]string{xmlData}),
		InsertedTime:    time.Now(),
		LastModified:    time.Now(),
		Active:          true,
	}

	// Map areas
	for _, area := range ospf.Area {
		areaType := "normal"
		if area.Type.Stub.DefaultRoute != "" {
			areaType = "stub"
		} else if area.Type.NSSA.DefaultRoute != "" {
			areaType = "nssa"
		}

		processArea := models.OSPFArea{
			AreaID:        area.Name,
			AreaType:      areaType,
			StubNoSummary: area.Type.Stub.AcceptSummary == "no",
			NSSANoRedist:  area.Type.NSSA.AcceptSummary == "no",
		}

		// Map interfaces in this area
		for _, iface := range area.Interface {
			if iface.Enable != "yes" {
				continue
			}

			processInterface := models.OSPFInterface{
				InterfaceName:  iface.Name,
				AreaID:         area.Name,
				NetworkType:    iface.LinkType,
				Cost:           iface.Metric,
				Priority:       iface.Priority,
				HelloInterval:  iface.HelloInterval,
				RetransmitTime: iface.RetransmitInterval,
				Passive:        iface.Passive == "yes",
			}

			// Determine authentication type
			if iface.Authentication.None != "" {
				processInterface.AuthType = "none"
			} else if iface.Authentication.Password != "" {
				processInterface.AuthType = "simple"
			} else if len(iface.Authentication.MD5) > 0 {
				processInterface.AuthType = "md5"
			}

			process.Interfaces = append(process.Interfaces, processInterface)

			// Add to passive list if passive
			if iface.Passive == "yes" {
				process.PassiveInterfaces = append(process.PassiveInterfaces, iface.Name)
			}
		}

		process.Areas = append(process.Areas, processArea)
	}

	// Map redistribution
	for _, rule := range ospf.ExportRules {
		// Extract protocol from rule name (convention: "redist-connected", "redist-static")
		if strings.Contains(rule.Name, "connected") {
			process.RedistributedProtocols = append(process.RedistributedProtocols, "connected")
		} else if strings.Contains(rule.Name, "static") {
			process.RedistributedProtocols = append(process.RedistributedProtocols, "static")
		}
	}

	return process, nil
}

// PANOSOSPFNeighbor represents an OSPF neighbor
// XML Path: show/routing/protocol/ospf/neighbor/entry
type PANOSOSPFNeighbor struct {
	XMLName    xml.Name `xml:"entry"`
	NeighborID string   `xml:"neighbor-id"`
	Priority   int      `xml:"priority"`
	State      string   `xml:"state"` // "Full", "2-Way", etc.
	DeadTime   int      `xml:"dead-time"` // Seconds
	Address    string   `xml:"address"`
	Interface  string   `xml:"interface"`
	AreaID     string   `xml:"area-id"`
}

// PANOSOSPFNeighborResponse wraps OSPF neighbor response
type PANOSOSPFNeighborResponse struct {
	XMLName xml.Name `xml:"response"`
	Status  string   `xml:"status,attr"`
	Result  struct {
		Entry []PANOSOSPFNeighbor `xml:"entry"`
	} `xml:"result"`
}

// ParsePANOSOSPFNeighbors parses OSPF neighbor table
// Command: <show><routing><protocol><ospf><neighbor></neighbor></ospf></protocol></routing></show>
// PAN-OS 8.1+
func ParsePANOSOSPFNeighbors(xmlData string) ([]models.OSPFNeighbor, error) {
	var response PANOSOSPFNeighborResponse
	if err := xml.Unmarshal([]byte(xmlData), &response); err != nil {
		return nil, fmt.Errorf("failed to parse PAN-OS OSPF neighbors XML: %w", err)
	}

	if response.Status != "success" {
		return nil, fmt.Errorf("PAN-OS API returned non-success status: %s", response.Status)
	}

	neighbors := make([]models.OSPFNeighbor, 0, len(response.Result.Entry))

	for _, panosNeighbor := range response.Result.Entry {
		neighbor := models.OSPFNeighbor{
			NeighborID: panosNeighbor.NeighborID,
			IPAddress:  panosNeighbor.Address,
			Interface:  panosNeighbor.Interface,
			State:      panosNeighbor.State,
			Priority:   panosNeighbor.Priority,
			DeadTime:   panosNeighbor.DeadTime,
			AreaID:     panosNeighbor.AreaID,
			LastChange: time.Now(),
		}
		neighbors = append(neighbors, neighbor)
	}

	return neighbors, nil
}

// ========================================================================
// BGP Parsing
// ========================================================================

// PANOSBGPConfig represents BGP configuration
// XML Path: config/devices/entry/network/virtual-router/entry/protocol/bgp
type PANOSBGPConfig struct {
	XMLName  xml.Name `xml:"bgp"`
	Enable   string   `xml:"enable"` // "yes" or "no"
	RouterID string   `xml:"router-id"`
	LocalAS  int      `xml:"local-as"`
	PeerGroup []PANOSBGPPeerGroup `xml:"peer-group>entry"`
	Redist    []PANOSBGPRedist `xml:"redist-rules>entry"`
	Install   struct {
		Route string `xml:"route"` // "yes" or "no"
	} `xml:"install-route"`
}

// PANOSBGPPeerGroup represents a BGP peer group
type PANOSBGPPeerGroup struct {
	XMLName xml.Name `xml:"entry"`
	Name    string   `xml:"name,attr"`
	Type    struct {
		EBGP struct {
			RemovePrivateAS string `xml:"remove-private-as"` // "yes" or "no"
		} `xml:"ebgp"`
		IBGP struct {
			ClusterID string `xml:"cluster-id"`
		} `xml:"ibgp"`
	} `xml:"type"`
	Peer []PANOSBGPPeer `xml:"peer>entry"`
}

// PANOSBGPPeer represents a BGP peer
type PANOSBGPPeer struct {
	XMLName              xml.Name `xml:"entry"`
	Name                 string   `xml:"name,attr"` // Usually neighbor IP
	Enable               string   `xml:"enable"` // "yes" or "no"
	PeerAS               int      `xml:"peer-as"`
	LocalAddress         struct {
		IP        string `xml:"ip"`
		Interface string `xml:"interface"`
	} `xml:"local-address"`
	PeerAddress struct {
		IP string `xml:"ip"`
	} `xml:"peer-address"`
	Reflector struct {
		Client string `xml:"client"` // "yes" or "no"
	} `xml:"reflector-client"`
	Connection struct {
		MultiHop       int `xml:"multi-hop"`
		KeepAlive      int `xml:"keep-alive-interval"`
		OpenDelay      int `xml:"open-delay-time"`
		HoldTime       int `xml:"hold-time"`
		IdleHold       int `xml:"idle-hold-time"`
	} `xml:"connection-options"`
}

// PANOSBGPRedist represents BGP redistribution
type PANOSBGPRedist struct {
	XMLName xml.Name `xml:"entry"`
	Name    string   `xml:"name,attr"`
	Enable  string   `xml:"enable"` // "yes" or "no"
	SetMED  int      `xml:"set-med"`
	SetASPath []int  `xml:"set-as-path-limit>entry"`
}

// PANOSBGPConfigResponse wraps BGP config response
type PANOSBGPConfigResponse struct {
	XMLName xml.Name `xml:"response"`
	Status  string   `xml:"status,attr"`
	Result  struct {
		BGP PANOSBGPConfig `xml:"bgp"`
	} `xml:"result"`
}

// ParsePANOSBGPConfig parses BGP configuration
// Command: <show><config><running><xpath>virtual-router/entry[@name='default']/protocol/bgp</xpath></running></config></show>
// PAN-OS 8.1+
func ParsePANOSBGPConfig(xmlData string) (*models.BGPProcess, error) {
	var response PANOSBGPConfigResponse
	if err := xml.Unmarshal([]byte(xmlData), &response); err != nil {
		return nil, fmt.Errorf("failed to parse PAN-OS BGP config XML: %w", err)
	}

	if response.Status != "success" {
		return nil, fmt.Errorf("PAN-OS API returned non-success status: %s", response.Status)
	}

	bgp := response.Result.BGP

	// Check if BGP is enabled
	if bgp.Enable != "yes" {
		return nil, fmt.Errorf("BGP is disabled")
	}

	process := &models.BGPProcess{
		LocalAS:          bgp.LocalAS,
		RouterID:         bgp.RouterID,
		Neighbors:        make([]models.BGPNeighbor, 0),
		Networks:         make([]models.BGPNetwork, 0),
		AggregateAddrs:   make([]models.BGPAggregate, 0),
		RedistributedProtocols: make([]string, 0),
		RouteReflector:   false,
		ConfigHash:       computeSnippetHash([]string{xmlData}),
		InsertedTime:     time.Now(),
		LastModified:     time.Now(),
		Active:           true,
	}

	// Map peer groups and peers
	for _, peerGroup := range bgp.PeerGroup {
		// Check if route reflector
		if peerGroup.Type.IBGP.ClusterID != "" {
			process.RouteReflector = true
			process.ClusterID = peerGroup.Type.IBGP.ClusterID
		}

		for _, peer := range peerGroup.Peer {
			if peer.Enable != "yes" {
				continue
			}

			neighbor := models.BGPNeighbor{
				IPAddress:            peer.PeerAddress.IP,
				RemoteAS:             peer.PeerAS,
				UpdateSource:         peer.LocalAddress.Interface,
				LocalIPAddress:       peer.LocalAddress.IP,
				RouteReflectorClient: peer.Reflector.Client == "yes",
				EBGP:                 peer.PeerAS != bgp.LocalAS,
				Multihop:             peer.Connection.MultiHop,
				HoldTime:             peer.Connection.HoldTime,
				KeepAlive:            peer.Connection.KeepAlive,
			}
			process.Neighbors = append(process.Neighbors, neighbor)
		}
	}

	// Map redistribution
	for _, redist := range bgp.Redist {
		if redist.Enable == "yes" {
			// Extract protocol from rule name
			if strings.Contains(redist.Name, "connected") {
				process.RedistributedProtocols = append(process.RedistributedProtocols, "connected")
			} else if strings.Contains(redist.Name, "static") {
				process.RedistributedProtocols = append(process.RedistributedProtocols, "static")
			} else if strings.Contains(redist.Name, "ospf") {
				process.RedistributedProtocols = append(process.RedistributedProtocols, "ospf")
			}
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

// parseProtocolFromFlags converts PAN-OS route flags to protocol name
func parseProtocolFromFlags(flags string) string {
	flags = strings.ToUpper(flags)
	if strings.Contains(flags, "C") {
		return "connected"
	} else if strings.Contains(flags, "S") {
		return "static"
	} else if strings.Contains(flags, "O") {
		return "ospf"
	} else if strings.Contains(flags, "B") {
		return "bgp"
	} else if strings.Contains(flags, "R") {
		return "rip"
	}
	return "unknown"
}

// getDefaultAdminDistance returns default AD for a protocol
func getDefaultAdminDistance(protocol string) int {
	adMap := map[string]int{
		"connected": 0,
		"static":    10,  // PAN-OS default
		"ospf":      110,
		"bgp":       200,
		"rip":       120,
	}

	if ad, found := adMap[protocol]; found {
		return ad
	}
	return 255
}

// calculateRouteConfidenceFromProtocol returns confidence score for protocol
func calculateRouteConfidenceFromProtocol(protocol string) int {
	scores := map[string]int{
		"connected": 95,
		"static":    85,
		"ospf":      75,
		"bgp":       65,
		"rip":       50,
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

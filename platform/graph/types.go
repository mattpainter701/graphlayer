package graph

import "github.com/mattpainter701/graphlayer/platform/models"

// ExportFormat enumerates supported graph serialization formats.
type ExportFormat string

const (
	FormatGraphML   ExportFormat = "graphml"
	FormatGEXF      ExportFormat = "gexf"
	FormatCytoscape ExportFormat = "cytoscape"
	FormatD3        ExportFormat = "d3"
	FormatDOT        ExportFormat = "dot"
)

// ViewFilter controls which device types appear in the output.
type ViewFilter string

const (
	ViewAll              ViewFilter = "all"
	ViewRoutingOnly      ViewFilter = "routing"
	ViewAccessOnly       ViewFilter = "access"
	ViewSecurityOnly     ViewFilter = "security"
	ViewCoreOnly         ViewFilter = "core"
	ViewFirewallsOnly    ViewFilter = "firewalls"
	ViewSwitchesOnly     ViewFilter = "switches"
)

// GraphOverlay controls optional visual annotations.
type GraphOverlay struct {
	SecurityPolicy bool `json:"security_policy,omitempty"` // firewall rules as edge annotations
	VLANGrouping   bool `json:"vlan_grouping,omitempty"`   // collapse/expand by VLAN segment
	TrafficFlow    bool `json:"traffic_flow,omitempty"`    // show flow volume on links
	ConfidenceEdge bool `json:"confidence_edge,omitempty"` // edge thickness by confidence
}

// ExportOptions bundles all configuration for a graph export.
type ExportOptions struct {
	Format      ExportFormat `json:"format"`
	ViewFilter  ViewFilter   `json:"view_filter,omitempty"`
	Overlays    GraphOverlay `json:"overlays,omitempty"`
	Title       string       `json:"title,omitempty"`
	Description string       `json:"description,omitempty"`
}

// VendorColors maps vendor names to hex colors.
var VendorColors = map[string]string{
	"cisco":           "#049FD9",
	"paloalto":        "#FA582D",
	"palo alto":       "#FA582D",
	"fortinet":        "#EE3124",
	"juniper":         "#84B135",
	"arista":          "#0072CE",
	"aruba":           "#FF8300",
	"dell":            "#007DB8",
	"checkpoint":      "#F58220",
	"check point":     "#F58220",
	"f5":              "#EB2127",
	"huawei":          "#CF0A2C",
	"vmware":          "#607078",
	"citrix":          "#652379",
	"brocade":         "#0082C3",
	"extreme":         "#74247A",
	"hp":              "#0096D6",
	"hpe":             "#01A982",
	"unknown":         "#9E9E9E",
	"":                "#9E9E9E",
}

// VendorColor returns the hex color for a vendor, lowercased and fuzzy-matched.
func VendorColor(vendor string) string {
	if c, ok := VendorColors[vendor]; ok {
		return c
	}
	// try lowercase match
	for k, v := range VendorColors {
		if k != "" && containsIgnoreCase(k, vendor) {
			return v
		}
	}
	return VendorColors["unknown"]
}

func containsIgnoreCase(container, s string) bool {
	return len(container) > 0 && len(s) > 0 &&
		(container == s || len(container) <= len(s) && s[:len(container)] == container)
}

// NodeWithMeta wraps a topology node with graph metadata.
type NodeWithMeta struct {
	Label      string  `json:"label"`
	NodeType   string  `json:"node_type"`   // device, network, asset
	Vendor     string  `json:"vendor"`
	Role       string  `json:"role"`
	Subnet     string  `json:"subnet,omitempty"`
	VLANID     int     `json:"vlan_id,omitempty"`
	VLANName   string  `json:"vlan_name,omitempty"`
	Segment    string  `json:"segment,omitempty"`
	Confidence float64 `json:"confidence"`
	Color      string  `json:"color"`
}

// EdgeWithMeta wraps a topology link with graph metadata.
type EdgeWithMeta struct {
	Label        string  `json:"label"`
	EdgeType     string  `json:"edge_type"` // l2, l3, policy, flow
	Protocol     string  `json:"protocol,omitempty"`
	Confidence   float64 `json:"confidence"`
	Thickness    float64 `json:"thickness"` // 1.0-5.0 based on confidence
	FlowVolume   float64 `json:"flow_volume,omitempty"`
	FlowUnit     string  `json:"flow_unit,omitempty"`
	Action       string  `json:"action,omitempty"`       // allow, deny for policy edges
	RuleName     string  `json:"rule_name,omitempty"`    // firewall rule name
	SourcePorts  []string `json:"source_ports,omitempty"`
	DestPorts    []string `json:"dest_ports,omitempty"`
	VLANIDs      []int    `json:"vlan_ids,omitempty"`
	SegmentID    string   `json:"segment_id,omitempty"`
}

// TopologyGraph is the unified graph model consumed by all exporters.
type TopologyGraph struct {
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Nodes       []GraphNode    `json:"nodes"`
	Edges       []GraphEdge    `json:"edges"`
	Options     ExportOptions  `json:"options"`
}

// GraphNode is a node ready for serialization.
type GraphNode struct {
	ID         string     `json:"id"`
	Label      string     `json:"label"`
	Meta       NodeWithMeta `json:"meta"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

// GraphEdge is an edge ready for serialization.
type GraphEdge struct {
	ID         string     `json:"id"`
	Source     string     `json:"source"`
	Target     string     `json:"target"`
	Label      string     `json:"label,omitempty"`
	Meta       EdgeWithMeta `json:"meta"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

// GraphBuilder constructs a TopologyGraph from raw model objects.
type GraphBuilder struct {
	nodes       map[string]GraphNode
	edges       []GraphEdge
	options     ExportOptions
	networkDevs map[string]models.Device
	assets      map[string]models.AssetNode
	networks    map[string]models.NetworkNode
	links       []models.Link
	l2Links     []models.L2Link
	policies    []models.FirewallPolicy
}

// NewGraphBuilder creates a GraphBuilder with default options.
func NewGraphBuilder(opts ExportOptions) *GraphBuilder {
	return &GraphBuilder{
		nodes:       make(map[string]GraphNode),
		options:     opts,
		networkDevs: make(map[string]models.Device),
		assets:      make(map[string]models.AssetNode),
		networks:    make(map[string]models.NetworkNode),
	}
}

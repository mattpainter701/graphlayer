package graph

import (
	"encoding/json"
)

// D3Graph is the D3.js force-directed graph JSON structure.
type D3Graph struct {
	Nodes []D3Node `json:"nodes"`
	Links []D3Link `json:"links"`
	Meta  *D3Meta  `json:"meta,omitempty"`
}

// D3Node represents a node in the D3 force layout.
type D3Node struct {
	ID     string `json:"id"`
	Label  string `json:"label,omitempty"`
	Group  string `json:"group,omitempty"`  // Used for coloring by group
	Radius float64 `json:"radius,omitempty"` // Node size
	Vendor string `json:"vendor,omitempty"`
	Role   string `json:"role,omitempty"`
	Type   string `json:"type,omitempty"`
	Color  string `json:"color,omitempty"`
	VLANID int    `json:"vlan_id,omitempty"`
	VLAN   string `json:"vlan,omitempty"`
	Segment string `json:"segment,omitempty"`
	Subnet string `json:"subnet,omitempty"`
	Confidence float64 `json:"confidence,omitempty"`
	// Fixed position for pre-laid-out nodes
	Fx *float64 `json:"fx,omitempty"`
	Fy *float64 `json:"fy,omitempty"`
}

// D3Link represents an edge in the D3 force layout.
type D3Link struct {
	Source     interface{} `json:"source"`  // string ID or index
	Target     interface{} `json:"target"`  // string ID or index
	Label      string      `json:"label,omitempty"`
	Value      float64     `json:"value,omitempty"`      // Link strength/thickness
	Type       string      `json:"type,omitempty"`       // l2, l3, policy, flow
	Protocol   string      `json:"protocol,omitempty"`
	Confidence float64     `json:"confidence,omitempty"`
	Action     string      `json:"action,omitempty"`      // allow, deny
	RuleName   string      `json:"rule_name,omitempty"`
	FlowVolume float64     `json:"flow_volume,omitempty"`
	FlowUnit   string      `json:"flow_unit,omitempty"`
}

// D3Meta holds graph-level metadata for D3 consumers.
type D3Meta struct {
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	VendorColored bool `json:"vendor_colored,omitempty"`
}

// ExportD3 serializes the graph to D3.js force-directed JSON format.
func ExportD3(g TopologyGraph) ([]byte, error) {
	d3 := D3Graph{
		Nodes: make([]D3Node, 0, len(g.Nodes)),
		Links: make([]D3Link, 0, len(g.Edges)),
		Meta: &D3Meta{
			Title:         g.Title,
			Description:   g.Description,
			VendorColored: g.Options.Overlays.ConfidenceEdge || true,
		},
	}

	// Build node index for link resolution
	nodeIndex := make(map[string]int)

	for i, n := range g.Nodes {
		nodeIndex[n.ID] = i
		group := n.Meta.Vendor
		if group == "" {
			group = n.Meta.NodeType
		}
		if group == "" {
			group = "unknown"
		}

		radius := 8.0
		switch n.Meta.NodeType {
		case "network":
			radius = 12.0
		case "firewall", "router", "routing":
			radius = 10.0
		case "zone":
			radius = 6.0
		case "segment_group":
			radius = 14.0
		}

		dn := D3Node{
			ID:         n.ID,
			Label:      n.Label,
			Group:      group,
			Radius:     radius,
			Vendor:     n.Meta.Vendor,
			Role:       n.Meta.Role,
			Type:       n.Meta.NodeType,
			Color:      n.Meta.Color,
			VLANID:     n.Meta.VLANID,
			VLAN:       n.Meta.VLANName,
			Segment:    n.Meta.Segment,
			Subnet:     n.Meta.Subnet,
			Confidence: n.Meta.Confidence,
		}
		d3.Nodes = append(d3.Nodes, dn)
	}

	for _, e := range g.Edges {
		// Use string IDs for D3 v4+ (supports both string and index)
		link := D3Link{
			Source:     e.Source,
			Target:     e.Target,
			Label:      e.Label,
			Value:      e.Meta.Thickness,
			Type:       e.Meta.EdgeType,
			Protocol:   e.Meta.Protocol,
			Confidence: e.Meta.Confidence,
			Action:     e.Meta.Action,
			RuleName:   e.Meta.RuleName,
			FlowVolume: e.Meta.FlowVolume,
			FlowUnit:   e.Meta.FlowUnit,
		}

		// If flow overlay is enabled, set link value to flow volume
		if g.Options.Overlays.TrafficFlow && e.Meta.FlowVolume > 0 {
			link.Value = e.Meta.FlowVolume
		}

		d3.Links = append(d3.Links, link)
	}

	return json.MarshalIndent(d3, "", "  ")
}

// D3HTMLTemplate generates a complete HTML page with embedded D3 visualization.
func D3HTMLTemplate(jsonData []byte) string {
	return `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>GraphLayer — Network Topology</title>
<style>
  body { margin: 0; background: #1a1a2e; font-family: -apple-system, sans-serif; }
  #graph { width: 100vw; height: 100vh; }
  .node { cursor: pointer; }
  .node text { font-size: 10px; fill: #e0e0e0; pointer-events: none; }
  .link { stroke: #555; stroke-opacity: 0.6; }
  .link-policy { stroke: #E91E63; stroke-dasharray: 5,5; }
  .tooltip {
    position: absolute;
    padding: 8px 12px;
    background: rgba(0,0,0,0.85);
    color: #e0e0e0;
    border-radius: 4px;
    font-size: 12px;
    pointer-events: none;
    max-width: 280px;
    z-index: 1000;
  }
</style>
</head>
<body>
<div id="graph"></div>
<div class="tooltip" style="display:none"></div>
<script src="https://d3js.org/d3.v7.min.js"></script>
<script>
const data = ` + jsonString(string(jsonData)) + `;

const width = window.innerWidth;
const height = window.innerHeight;

const color = d3.scaleOrdinal(d3.schemeCategory10);

const svg = d3.select("#graph")
  .append("svg")
  .attr("width", width)
  .attr("height", height);

const g = svg.append("g");

const zoom = d3.zoom()
  .scaleExtent([0.1, 4])
  .on("zoom", (event) => g.attr("transform", event.transform));

svg.call(zoom);

const tooltip = d3.select(".tooltip");

const simulation = d3.forceSimulation(data.nodes)
  .force("link", d3.forceLink(data.links).id(d => d.id).distance(100))
  .force("charge", d3.forceManyBody().strength(-300))
  .force("center", d3.forceCenter(width / 2, height / 2))
  .force("collision", d3.forceCollide().radius(d => d.radius + 2));

const link = g.append("g")
  .selectAll("line")
  .data(data.links)
  .join("line")
  .attr("class", d => "link" + (d.type === "policy" ? " link-policy" : ""))
  .attr("stroke-width", d => Math.max(1, d.value || 1));

const node = g.append("g")
  .selectAll("g")
  .data(data.nodes)
  .join("g")
  .attr("class", "node")
  .call(d3.drag()
    .on("start", (event, d) => { if (!event.active) simulation.alphaTarget(0.3).restart(); d.fx = d.x; d.fy = d.y; })
    .on("drag", (event, d) => { d.fx = event.x; d.fy = event.y; })
    .on("end", (event, d) => { if (!event.active) simulation.alphaTarget(0); d.fx = null; d.fy = null; }));

node.append("circle")
  .attr("r", d => d.radius || 6)
  .attr("fill", d => d.color || color(d.group));

node.append("text")
  .attr("dy", d => -d.radius - 2)
  .text(d => d.label);

node.on("mouseover", (event, d) => {
  const lines = [];
  if (d.label) lines.push("<b>" + d.label + "</b>");
  if (d.vendor) lines.push("Vendor: " + d.vendor);
  if (d.role) lines.push("Role: " + d.role);
  if (d.type) lines.push("Type: " + d.type);
  if (d.subnet) lines.push("Subnet: " + d.subnet);
  if (d.vlan) lines.push("VLAN: " + d.vlan);
  if (d.confidence) lines.push("Confidence: " + d.confidence.toFixed(2));
  tooltip.style("display", "block").html(lines.join("<br>"));
}).on("mousemove", (event) => {
  tooltip.style("left", (event.pageX + 12) + "px")
    .style("top", (event.pageY - 12) + "px");
}).on("mouseout", () => tooltip.style("display", "none"));

simulation.on("tick", () => {
  link
    .attr("x1", d => d.source.x)
    .attr("y1", d => d.source.y)
    .attr("x2", d => d.target.x)
    .attr("y2", d => d.target.y);
  node.attr("transform", d => "translate(" + d.x + "," + d.y + ")");
});
</script>
</body>
</html>`
}

func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

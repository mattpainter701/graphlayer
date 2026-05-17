package graph

import (
	"encoding/xml"
	"fmt"
)

// GEXF root element.
type gexfDoc struct {
	XMLName xml.Name  `xml:"gexf"`
	Xmlns   string    `xml:"xmlns,attr"`
	Viz     string    `xml:"xmlns:viz,attr,omitempty"`
	Version string    `xml:"version,attr"`
	Meta    gexfMeta  `xml:"meta"`
	Graph   gexfGraph `xml:"graph"`
}

type gexfMeta struct {
	LastModified string `xml:"lastmodifieddate,attr"`
	Creator      string `xml:"creator"`
	Description  string `xml:"description"`
}

type gexfGraph struct {
	Mode      string        `xml:"mode,attr"`
	DefaultEdgeType string  `xml:"defaultedgetype,attr"`
	Nodes     []gexfNode    `xml:"nodes>node"`
	Edges     []gexfEdge    `xml:"edges>edge"`
	Attributes gexfAttributes `xml:"attributes"`
}

type gexfAttributes struct {
	Class      string         `xml:"class,attr"`
	NodeAttrs  []gexfAttribute `xml:"attribute"`
	EdgeAttrs  []gexfAttribute `xml:"-"`
}

type gexfAttribute struct {
	ID    string `xml:"id,attr"`
	Title string `xml:"title,attr"`
	Type  string `xml:"type,attr"`
}

type gexfNode struct {
	ID      string         `xml:"id,attr"`
	Label   string         `xml:"label,attr"`
	AttVals []gexfAttValue `xml:"attvalues>attvalue"`
	Color   *gexfColor     `xml:"viz:color,omitempty"`
	Size    *gexfSize      `xml:"viz:size,omitempty"`
}

type gexfEdge struct {
	ID      string         `xml:"id,attr,omitempty"`
	Source  string         `xml:"source,attr"`
	Target  string         `xml:"target,attr"`
	Label   string         `xml:"label,attr,omitempty"`
	AttVals []gexfAttValue `xml:"attvalues>attvalue"`
	Weight  float64        `xml:"weight,attr,omitempty"`
}

type gexfAttValue struct {
	For   string `xml:"for,attr"`
	Value string `xml:"value,attr"`
}

type gexfColor struct {
	R int `xml:"r,attr"`
	G int `xml:"g,attr"`
	B int `xml:"b,attr"`
}

type gexfSize struct {
	Value float64 `xml:"value,attr"`
}

// ExportGEXF serializes the graph to GEXF format.
func ExportGEXF(g TopologyGraph) ([]byte, error) {
	doc := gexfDoc{
		Xmlns:   "http://www.gexf.net/1.3",
		Viz:     "http://www.gexf.net/1.3/viz",
		Version: "1.3",
		Meta: gexfMeta{
			LastModified: "2026-05-17",
			Creator:      "GraphLayer",
			Description:  g.Description,
		},
		Graph: gexfGraph{
			Mode:            "static",
			DefaultEdgeType: "directed",
		},
	}

	// Node attribute declarations
	nodeAttrs := []gexfAttribute{
		{ID: "vendor", Title: "Vendor", Type: "string"},
		{ID: "role", Title: "Role", Type: "string"},
		{ID: "node_type", Title: "Node Type", Type: "string"},
		{ID: "subnet", Title: "Subnet", Type: "string"},
		{ID: "vlan_id", Title: "VLAN ID", Type: "integer"},
		{ID: "vlan_name", Title: "VLAN Name", Type: "string"},
		{ID: "segment", Title: "Segment", Type: "string"},
	}

	edgeAttrs := []gexfAttribute{
		{ID: "edge_type", Title: "Edge Type", Type: "string"},
		{ID: "protocol", Title: "Protocol", Type: "string"},
		{ID: "confidence", Title: "Confidence", Type: "double"},
		{ID: "action", Title: "Action", Type: "string"},
		{ID: "rule_name", Title: "Rule Name", Type: "string"},
	}

	// GEXF structure: attributes outside graph, then nodes/edges inside
	// We need to build node and edge attribute sections
	_ = nodeAttrs
	_ = edgeAttrs

	for _, n := range g.Nodes {
		gn := gexfNode{
			ID:    sanitizeID(n.ID),
			Label: n.Label,
		}
		gn.AttVals = append(gn.AttVals, gexfAttValue{For: "vendor", Value: n.Meta.Vendor})
		gn.AttVals = append(gn.AttVals, gexfAttValue{For: "role", Value: n.Meta.Role})
		gn.AttVals = append(gn.AttVals, gexfAttValue{For: "node_type", Value: n.Meta.NodeType})
		gn.AttVals = append(gn.AttVals, gexfAttValue{For: "subnet", Value: n.Meta.Subnet})
		if n.Meta.VLANID > 0 {
			gn.AttVals = append(gn.AttVals, gexfAttValue{For: "vlan_id", Value: fmt.Sprintf("%d", n.Meta.VLANID)})
		}
		if n.Meta.VLANName != "" {
			gn.AttVals = append(gn.AttVals, gexfAttValue{For: "vlan_name", Value: n.Meta.VLANName})
		}
		if n.Meta.Segment != "" {
			gn.AttVals = append(gn.AttVals, gexfAttValue{For: "segment", Value: n.Meta.Segment})
		}
		if n.Meta.Color != "" {
			gn.Color = hexToGexfColor(n.Meta.Color)
		}
		doc.Graph.Nodes = append(doc.Graph.Nodes, gn)
	}

	for _, e := range g.Edges {
		ge := gexfEdge{
			ID:     sanitizeID(e.ID),
			Source: sanitizeID(e.Source),
			Target: sanitizeID(e.Target),
			Label:  e.Label,
			Weight: e.Meta.Thickness,
		}
		ge.AttVals = append(ge.AttVals, gexfAttValue{For: "edge_type", Value: e.Meta.EdgeType})
		ge.AttVals = append(ge.AttVals, gexfAttValue{For: "protocol", Value: e.Meta.Protocol})
		ge.AttVals = append(ge.AttVals, gexfAttValue{For: "confidence", Value: fmt.Sprintf("%.2f", e.Meta.Confidence)})
		if e.Meta.Action != "" {
			ge.AttVals = append(ge.AttVals, gexfAttValue{For: "action", Value: e.Meta.Action})
		}
		if e.Meta.RuleName != "" {
			ge.AttVals = append(ge.AttVals, gexfAttValue{For: "rule_name", Value: e.Meta.RuleName})
		}
		doc.Graph.Edges = append(doc.Graph.Edges, ge)
	}

	// Build the attributes section
	doc.Graph.Attributes = gexfAttributes{
		Class:     "node",
		NodeAttrs: nodeAttrs,
	}

	out, err := xml.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("gexf marshal: %w", err)
	}
	return append([]byte(xml.Header), out...), nil
}

func sanitizeID(id string) string {
	if id == "" {
		return "n0"
	}
	result := make([]byte, 0, len(id))
	for _, c := range []byte(id) {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-' {
			result = append(result, c)
		} else {
			result = append(result, '_')
		}
	}
	if len(result) == 0 {
		return "n0"
	}
	return string(result)
}

// hexToGexfColor converts a hex color string to GEXF RGB components.
func hexToGexfColor(hex string) *gexfColor {
	if len(hex) != 7 || hex[0] != '#' {
		return nil
	}
	var r, g, b int
	fmt.Sscanf(hex, "#%02x%02x%02x", &r, &g, &b)
	return &gexfColor{R: r, G: g, B: b}
}

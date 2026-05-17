package graph

import (
	"encoding/json"
	"fmt"
)

// CytoscapeGraph is the root Cytoscape.js JSON structure.
type CytoscapeGraph struct {
	Elements CytoscapeElements  `json:"elements"`
	Style    []CytoscapeStyle   `json:"style,omitempty"`
	Layout   *CytoscapeLayout   `json:"layout,omitempty"`
	Data     *CytoscapeMetadata `json:"data,omitempty"`
}

// CytoscapeElements holds the nodes and edges arrays.
type CytoscapeElements struct {
	Nodes []CytoscapeElement `json:"nodes"`
	Edges []CytoscapeElement `json:"edges"`
}

// CytoscapeElement is a single node or edge in Cytoscape format.
type CytoscapeElement struct {
	Data       map[string]interface{} `json:"data"`
	Position   *CytoscapePosition     `json:"position,omitempty"`
	Selected   bool                   `json:"selected,omitempty"`
	Selectable bool                   `json:"selectable,omitempty"`
	Locked     bool                   `json:"locked,omitempty"`
	Grabbable  bool                   `json:"grabbable,omitempty"`
	Classes    string                 `json:"classes,omitempty"`
}

// CytoscapePosition holds x,y coordinates.
type CytoscapePosition struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// CytoscapeStyle defines visual styling using Cytoscape.js style syntax.
type CytoscapeStyle struct {
	Selector    string                 `json:"selector"`
	Style       map[string]interface{} `json:"style"`
}

// CytoscapeLayout configures the initial layout algorithm.
type CytoscapeLayout struct {
	Name string                 `json:"name"`
	Opts map[string]interface{} `json:"options,omitempty"`
}

// CytoscapeMetadata holds graph-level data.
type CytoscapeMetadata struct {
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
}

// ExportCytoscape serializes the graph to Cytoscape.js JSON format.
func ExportCytoscape(g TopologyGraph, vendorColored bool) ([]byte, error) {
	cy := CytoscapeGraph{
		Elements: CytoscapeElements{
			Nodes: make([]CytoscapeElement, 0, len(g.Nodes)),
			Edges: make([]CytoscapeElement, 0, len(g.Edges)),
		},
		Layout: &CytoscapeLayout{
			Name: "cose", // Compound Spring Embedder — good for network topologies
			Opts: map[string]interface{}{
				"nodeRepulsion": 8000,
				"idealEdgeLength": 100,
				"gravity": 0.25,
			},
		},
		Data: &CytoscapeMetadata{
			Title:       g.Title,
			Description: g.Description,
		},
	}

	// Generate vendor-based styles
	if vendorColored {
		cy.Style = generateVendorStyles(g)
	} else {
		cy.Style = generateDefaultStyles()
	}

	// Position nodes in a grid layout for initial placement
	gridSize := intSqrt(len(g.Nodes)) + 1
	for i, n := range g.Nodes {
		data := map[string]interface{}{
			"id":         n.ID,
			"label":      n.Label,
			"vendor":     n.Meta.Vendor,
			"role":       n.Meta.Role,
			"node_type":  n.Meta.NodeType,
			"subnet":     n.Meta.Subnet,
			"color":      n.Meta.Color,
			"confidence": n.Meta.Confidence,
		}
		if n.Meta.VLANID > 0 {
			data["vlan_id"] = n.Meta.VLANID
		}
		if n.Meta.VLANName != "" {
			data["vlan_name"] = n.Meta.VLANName
		}
		if n.Meta.Segment != "" {
			data["segment"] = n.Meta.Segment
		}

		// Add extra attributes
		for k, v := range n.Attributes {
			if _, exists := data[k]; !exists {
				data[k] = v
			}
		}

		el := CytoscapeElement{
			Data: data,
			Position: &CytoscapePosition{
				X: float64(i%gridSize) * 200,
				Y: float64(i/gridSize) * 150,
			},
			Selectable: true,
			Grabbable:  true,
			Classes:    n.Meta.NodeType,
		}
		cy.Elements.Nodes = append(cy.Elements.Nodes, el)
	}

	for _, e := range g.Edges {
		data := map[string]interface{}{
			"id":         e.ID,
			"source":     e.Source,
			"target":     e.Target,
			"label":      e.Label,
			"edge_type":  e.Meta.EdgeType,
			"protocol":   e.Meta.Protocol,
			"confidence": e.Meta.Confidence,
		}
		if e.Meta.Action != "" {
			data["action"] = e.Meta.Action
		}
		if e.Meta.RuleName != "" {
			data["rule_name"] = e.Meta.RuleName
		}
		if e.Meta.FlowVolume > 0 {
			data["flow_volume"] = e.Meta.FlowVolume
			data["flow_unit"] = e.Meta.FlowUnit
		}

		el := CytoscapeElement{
			Data:    data,
			Classes: fmt.Sprintf("edge-%s", e.Meta.EdgeType),
		}
		cy.Elements.Edges = append(cy.Elements.Edges, el)
	}

	return json.MarshalIndent(cy, "", "  ")
}

// generateVendorStyles creates Cytoscape style rules colored by vendor.
func generateVendorStyles(g TopologyGraph) []CytoscapeStyle {
	vendorSet := make(map[string]bool)
	for _, n := range g.Nodes {
		if n.Meta.Vendor != "" {
			vendorSet[n.Meta.Vendor] = true
		}
	}

	styles := []CytoscapeStyle{
		// Base node style
		{
			Selector: "node",
			Style: map[string]interface{}{
				"label":            "data(label)",
				"text-valign":      "center",
				"text-halign":      "center",
				"font-size":        "11px",
				"background-color": "#9E9E9E",
				"border-width":     2,
				"border-color":     "#555",
				"width":            40,
				"height":           40,
				"shape":            "ellipse",
			},
		},
		// Edge styles
		{
			Selector: "edge",
			Style: map[string]interface{}{
				"width":          "mapData(confidence, 0, 1, 1, 5)",
				"line-color":     "#999",
				"target-arrow-color": "#999",
				"target-arrow-shape": "triangle",
				"curve-style":    "bezier",
				"label":          "data(label)",
				"font-size":      "9px",
				"text-rotation":  "autorotate",
			},
		},
		// Policy edges
		{
			Selector: ".edge-policy",
			Style: map[string]interface{}{
				"line-color":   "#E91E63",
				"line-style":   "dashed",
				"target-arrow-color": "#E91E63",
			},
		},
		// Network nodes
		{
			Selector: ".network",
			Style: map[string]interface{}{
				"shape":  "round-rectangle",
				"width":  60,
				"height": 30,
			},
		},
		// Zone nodes
		{
			Selector: ".zone",
			Style: map[string]interface{}{
				"shape":  "diamond",
				"width":  35,
				"height": 35,
			},
		},
		// Asset nodes
		{
			Selector: ".asset",
			Style: map[string]interface{}{
				"shape":  "rectangle",
				"width":  35,
				"height": 25,
			},
		},
		// Segment group nodes
		{
			Selector: ".segment_group",
			Style: map[string]interface{}{
				"shape":            "round-rectangle",
				"width":            80,
				"height":           50,
				"background-color": "#FFC107",
				"border-style":     "double",
			},
		},
	}

	// Per-vendor node colors
	for vendor := range vendorSet {
		color := VendorColor(vendor)
		if color == "" {
			continue
		}
		cls := "vendor-" + sanitizeID(vendor)
		styles = append(styles, CytoscapeStyle{
			Selector: "." + cls,
			Style: map[string]interface{}{
				"background-color": color,
			},
		})
	}

	return styles
}

// generateDefaultStyles creates basic Cytoscape styles without vendor coloring.
func generateDefaultStyles() []CytoscapeStyle {
	return []CytoscapeStyle{
		{
			Selector: "node",
			Style: map[string]interface{}{
				"label":            "data(label)",
				"text-valign":      "center",
				"text-halign":      "center",
				"font-size":        "11px",
				"background-color": "#3498DB",
				"border-width":     2,
				"border-color":     "#2980B9",
				"width":            40,
				"height":           40,
				"shape":            "ellipse",
			},
		},
		{
			Selector: "edge",
			Style: map[string]interface{}{
				"width":              2,
				"line-color":         "#999",
				"target-arrow-color": "#999",
				"target-arrow-shape": "triangle",
				"curve-style":        "bezier",
				"label":              "data(label)",
				"font-size":          "9px",
			},
		},
	}
}

// intSqrt returns the integer square root of n.
func intSqrt(n int) int {
	if n <= 0 {
		return 0
	}
	x := n
	y := (x + 1) / 2
	for y < x {
		x = y
		y = (x + n/x) / 2
	}
	return x
}

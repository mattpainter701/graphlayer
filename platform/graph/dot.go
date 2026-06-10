package graph

import (
	"fmt"
	"strings"
)

// ExportDOT serializes the graph to Graphviz DOT format with default styling.
func ExportDOT(g TopologyGraph) ([]byte, error) {
	return ExportToDOTCustom(&g, nil, nil)
}

// ExportToDOTCustom serializes the graph to Graphviz DOT format with optional
// per-node and per-edge attribute overrides.
//
// nodeAttrs maps node IDs to attribute key/value pairs (e.g. {"router1": {"color": "red"}}).
// edgeAttrs maps "source->target" strings to attribute key/value pairs.
func ExportToDOTCustom(g *TopologyGraph, nodeAttrs, edgeAttrs map[string]map[string]string) ([]byte, error) {
	if g == nil {
		return nil, fmt.Errorf("dot: nil graph")
	}

	var b strings.Builder
	b.WriteString("digraph G {\n")

	// Graph-level attributes
	b.WriteString("  graph [rankdir=LR, fontname=\"Helvetica\"];\n")
	b.WriteString("  node [fontname=\"Helvetica\", shape=box, style=filled];\n")
	b.WriteString("  edge [fontname=\"Helvetica\"];\n\n")

	// Title comment
	if g.Title != "" {
		b.WriteString(fmt.Sprintf("  // %s\n", g.Title))
	}

	// Nodes
	for _, node := range g.Nodes {
		attrs := nodeDOTAttrs(node)
		// Apply custom overrides
		if nodeAttrs != nil {
			if overrides, ok := nodeAttrs[node.ID]; ok {
				for k, v := range overrides {
					attrs[k] = v
				}
			}
		}
		b.WriteString(fmt.Sprintf("  %s [%s];\n", dotID(node.ID), dotAttrs(attrs)))
	}

	if len(g.Nodes) > 0 && len(g.Edges) > 0 {
		b.WriteString("\n")
	}

	// Edges
	for _, edge := range g.Edges {
		attrs := edgeDOTAttrs(edge)
		// Apply custom overrides
		if edgeAttrs != nil {
			key := fmt.Sprintf("%s->%s", edge.Source, edge.Target)
			if overrides, ok := edgeAttrs[key]; ok {
				for k, v := range overrides {
					attrs[k] = v
				}
			}
		}
		b.WriteString(fmt.Sprintf("  %s -> %s [%s];\n",
			dotID(edge.Source), dotID(edge.Target), dotAttrs(attrs)))
	}

	b.WriteString("}\n")
	return []byte(b.String()), nil
}

// dotID sanitizes an identifier for use in DOT format.
// If the ID contains special characters, it is quoted.
func dotID(id string) string {
	if id == "" {
		return `""`
	}
	special := false
	for _, c := range id {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
			special = true
			break
		}
	}
	if special || isDOTKeyword(id) {
		return fmt.Sprintf("%q", id)
	}
	return id
}

// isDOTKeyword checks if a string is a DOT language keyword.
func isDOTKeyword(s string) bool {
	keywords := []string{"digraph", "graph", "node", "edge", "subgraph", "strict"}
	for _, kw := range keywords {
		if s == kw {
			return true
		}
	}
	return false
}

// dotAttrs formats a map of attributes as DOT "key=value" pairs.
func dotAttrs(attrs map[string]string) string {
	parts := make([]string, 0, len(attrs))
	for k, v := range attrs {
		parts = append(parts, fmt.Sprintf("%s=%q", k, v))
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, ", ")
}

// nodeDOTAttrs generates default DOT attributes for a node based on its metadata.
func nodeDOTAttrs(node GraphNode) map[string]string {
	attrs := map[string]string{
		"label": dotLabel(node),
	}

	// Color from vendor
	if node.Meta.Color != "" {
		attrs["fillcolor"] = node.Meta.Color
		attrs["fontcolor"] = "white"
	}

	// Shape based on node type
	switch node.Meta.NodeType {
	case "device", "firewall", "router", "core", "edge", "access", "distribution", "switch":
		attrs["shape"] = deviceShape(node.Meta.Role)
	case "network", "segment_group":
		attrs["shape"] = "ellipse"
		attrs["style"] = "filled,rounded"
	case "asset":
		attrs["shape"] = "note"
	case "zone":
		attrs["shape"] = "doubleoctagon"
		attrs["style"] = "filled,rounded"
	default:
		attrs["shape"] = "box"
	}

	return attrs
}

// edgeDOTAttrs generates default DOT attributes for an edge based on its metadata.
func edgeDOTAttrs(edge GraphEdge) map[string]string {
	attrs := map[string]string{}

	if edge.Label != "" {
		attrs["label"] = edge.Label
	} else if edge.Meta.EdgeType != "" {
		attrs["label"] = edge.Meta.EdgeType
	}

	// Edge style based on type
	switch edge.Meta.EdgeType {
	case "policy":
		attrs["style"] = "dashed"
		attrs["color"] = "red"
		if edge.Meta.Action == "deny" {
			attrs["color"] = "darkred"
			attrs["style"] = "bold"
		}
	case "l2":
		attrs["style"] = "bold"
		attrs["color"] = "#666666"
	default:
		attrs["color"] = "#333333"
	}

	// Pen width based on confidence/thickness
	if edge.Meta.Thickness > 0 {
		attrs["penwidth"] = fmt.Sprintf("%.1f", edge.Meta.Thickness)
	}

	return attrs
}

// deviceShape returns a DOT shape appropriate for the device role.
func deviceShape(role string) string {
	switch strings.ToLower(role) {
	case "firewall":
		return "doubleoctagon"
	case "router", "core":
		return "diamond"
	case "switch", "access":
		return "box"
	default:
		return "box"
	}
}

// dotLabel builds a display label for a node.
func dotLabel(node GraphNode) string {
	label := node.Meta.Label
	if label == "" {
		label = node.ID
	}
	if node.Meta.NodeType == "device" && node.Meta.Vendor != "" {
		return fmt.Sprintf("%s\\n[%s]", label, node.Meta.Vendor)
	}
	if node.Meta.NodeType == "network" && node.Meta.Subnet != "" {
		return fmt.Sprintf("%s\\n%s", label, node.Meta.Subnet)
	}
	return label
}

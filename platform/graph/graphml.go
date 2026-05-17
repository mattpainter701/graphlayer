package graph

import (
	"encoding/xml"
	"fmt"
	"strings"
)

// GraphML represents the root GraphML XML structure.
type graphmlDoc struct {
	XMLName xml.Name     `xml:"graphml"`
	Xmlns   string       `xml:"xmlns,attr"`
	Xsi     string       `xml:"xmlns:xsi,attr,omitempty"`
	Schema  string       `xml:"xsi:schemaLocation,attr,omitempty"`
	Keys    []graphmlKey `xml:"key"`
	Graph   graphmlGraph `xml:"graph"`
}

type graphmlKey struct {
	ID        string `xml:"id,attr"`
	For       string `xml:"for,attr"`
	AttrName  string `xml:"attr.name,attr"`
	AttrType  string `xml:"attr.type,attr"`
}

type graphmlGraph struct {
	ID        string          `xml:"id,attr"`
	EdgeDefault string        `xml:"edgedefault,attr"`
	Nodes     []graphmlNode   `xml:"node"`
	Edges     []graphmlEdge   `xml:"edge"`
}

type graphmlNode struct {
	ID   string        `xml:"id,attr"`
	Data []graphmlData `xml:"data"`
}

type graphmlEdge struct {
	ID     string        `xml:"id,attr"`
	Source string        `xml:"source,attr"`
	Target string        `xml:"target,attr"`
	Data   []graphmlData `xml:"data"`
}

type graphmlData struct {
	Key   string `xml:"key,attr"`
	Value string `xml:",chardata"`
}

// ExportGraphML serializes the graph to GraphML format.
func ExportGraphML(g TopologyGraph) ([]byte, error) {
	doc := graphmlDoc{
		Xmlns: "http://graphml.graphdrawing.org/xmlns",
		Xsi:   "http://www.w3.org/2001/XMLSchema-instance",
		Schema: "http://graphml.graphdrawing.org/xmlns http://graphml.graphdrawing.org/xmlns/1.0/graphml.xsd",
	}

	// Define keys for node and edge attributes
	doc.Keys = []graphmlKey{
		{ID: "d0", For: "node", AttrName: "label", AttrType: "string"},
		{ID: "d1", For: "node", AttrName: "vendor", AttrType: "string"},
		{ID: "d2", For: "node", AttrName: "role", AttrType: "string"},
		{ID: "d3", For: "node", AttrName: "node_type", AttrType: "string"},
		{ID: "d4", For: "node", AttrName: "subnet", AttrType: "string"},
		{ID: "d5", For: "node", AttrName: "color", AttrType: "string"},
		{ID: "d6", For: "node", AttrName: "vlan_id", AttrType: "int"},
		{ID: "d7", For: "node", AttrName: "vlan_name", AttrType: "string"},
		{ID: "d8", For: "node", AttrName: "segment", AttrType: "string"},
		{ID: "d9", For: "node", AttrName: "confidence", AttrType: "double"},
		{ID: "e0", For: "edge", AttrName: "label", AttrType: "string"},
		{ID: "e1", For: "edge", AttrName: "edge_type", AttrType: "string"},
		{ID: "e2", For: "edge", AttrName: "protocol", AttrType: "string"},
		{ID: "e3", For: "edge", AttrName: "confidence", AttrType: "double"},
		{ID: "e4", For: "edge", AttrName: "thickness", AttrType: "double"},
		{ID: "e5", For: "edge", AttrName: "action", AttrType: "string"},
		{ID: "e6", For: "edge", AttrName: "rule_name", AttrType: "string"},
		{ID: "e7", For: "edge", AttrName: "flow_volume", AttrType: "double"},
	}

	doc.Graph = graphmlGraph{
		ID:          "G",
		EdgeDefault: "directed",
	}

	for _, n := range g.Nodes {
		gn := graphmlNode{ID: sanitizeGraphMLID(n.ID)}
		gn.Data = append(gn.Data, graphmlData{Key: "d0", Value: n.Label})
		gn.Data = append(gn.Data, graphmlData{Key: "d1", Value: n.Meta.Vendor})
		gn.Data = append(gn.Data, graphmlData{Key: "d2", Value: n.Meta.Role})
		gn.Data = append(gn.Data, graphmlData{Key: "d3", Value: n.Meta.NodeType})
		gn.Data = append(gn.Data, graphmlData{Key: "d4", Value: n.Meta.Subnet})
		gn.Data = append(gn.Data, graphmlData{Key: "d5", Value: n.Meta.Color})
		if n.Meta.VLANID > 0 {
			gn.Data = append(gn.Data, graphmlData{Key: "d6", Value: fmt.Sprintf("%d", n.Meta.VLANID)})
		}
		if n.Meta.VLANName != "" {
			gn.Data = append(gn.Data, graphmlData{Key: "d7", Value: n.Meta.VLANName})
		}
		if n.Meta.Segment != "" {
			gn.Data = append(gn.Data, graphmlData{Key: "d8", Value: n.Meta.Segment})
		}
		gn.Data = append(gn.Data, graphmlData{Key: "d9", Value: fmt.Sprintf("%.2f", n.Meta.Confidence)})
		doc.Graph.Nodes = append(doc.Graph.Nodes, gn)
	}

	for _, e := range g.Edges {
		ge := graphmlEdge{
			ID:     sanitizeGraphMLID(e.ID),
			Source: sanitizeGraphMLID(e.Source),
			Target: sanitizeGraphMLID(e.Target),
		}
		ge.Data = append(ge.Data, graphmlData{Key: "e0", Value: e.Label})
		ge.Data = append(ge.Data, graphmlData{Key: "e1", Value: e.Meta.EdgeType})
		ge.Data = append(ge.Data, graphmlData{Key: "e2", Value: e.Meta.Protocol})
		ge.Data = append(ge.Data, graphmlData{Key: "e3", Value: fmt.Sprintf("%.2f", e.Meta.Confidence)})
		ge.Data = append(ge.Data, graphmlData{Key: "e4", Value: fmt.Sprintf("%.2f", e.Meta.Thickness)})
		if e.Meta.Action != "" {
			ge.Data = append(ge.Data, graphmlData{Key: "e5", Value: e.Meta.Action})
		}
		if e.Meta.RuleName != "" {
			ge.Data = append(ge.Data, graphmlData{Key: "e6", Value: e.Meta.RuleName})
		}
		if e.Meta.FlowVolume > 0 {
			ge.Data = append(ge.Data, graphmlData{Key: "e7", Value: fmt.Sprintf("%.2f", e.Meta.FlowVolume)})
		}
		doc.Graph.Edges = append(doc.Graph.Edges, ge)
	}

	out, err := xml.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("graphml marshal: %w", err)
	}
	return append([]byte(xml.Header), out...), nil
}

// sanitizeGraphMLID replaces characters that are problematic in XML IDs.
func sanitizeGraphMLID(id string) string {
	if id == "" {
		return "n0"
	}
	id = strings.ReplaceAll(id, ":", "_")
	id = strings.ReplaceAll(id, " ", "_")
	id = strings.ReplaceAll(id, "/", "_")
	id = strings.ReplaceAll(id, ".", "_")
	id = strings.ReplaceAll(id, "#", "_")
	return id
}

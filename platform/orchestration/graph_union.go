package orchestration

import "fmt"

// Union merges two topology graphs into a new graph.
//
// Nodes are merged by ID and edges are merged by source/target pair. When a
// node or edge exists in both graphs, attributes from g2 override attributes
// from g1 on conflict. For overlapping edges, the weight from g2 wins.
func Union(g1, g2 *TopologyGraph) (*TopologyGraph, error) {
	if g1 == nil && g2 == nil {
		return &TopologyGraph{}, nil
	}
	if g1 == nil {
		return cloneTopologyGraph(g2), nil
	}
	if g2 == nil {
		return cloneTopologyGraph(g1), nil
	}

	merged := &TopologyGraph{
		Nodes: make([]GraphNode, 0, len(g1.Nodes)+len(g2.Nodes)),
		Edges: make([]GraphEdge, 0, len(g1.Edges)+len(g2.Edges)),
	}

	nodeIndex := make(map[string]int, len(g1.Nodes)+len(g2.Nodes))
	edgeIndex := make(map[string]int, len(g1.Edges)+len(g2.Edges))

	for _, node := range g1.Nodes {
		if node.ID == "" {
			return nil, fmt.Errorf("graph node ID cannot be empty")
		}
		merged.Nodes = append(merged.Nodes, cloneGraphNode(node))
		nodeIndex[node.ID] = len(merged.Nodes) - 1
	}

	for _, node := range g2.Nodes {
		if node.ID == "" {
			return nil, fmt.Errorf("graph node ID cannot be empty")
		}
		if idx, ok := nodeIndex[node.ID]; ok {
			merged.Nodes[idx].Attributes = mergeStringMaps(merged.Nodes[idx].Attributes, node.Attributes)
			continue
		}
		merged.Nodes = append(merged.Nodes, cloneGraphNode(node))
		nodeIndex[node.ID] = len(merged.Nodes) - 1
	}

	for _, edge := range g1.Edges {
		if edge.Source == "" || edge.Target == "" {
			return nil, fmt.Errorf("graph edge source and target cannot be empty")
		}
		merged.Edges = append(merged.Edges, cloneGraphEdge(edge))
		edgeIndex[edgeKey(edge.Source, edge.Target)] = len(merged.Edges) - 1
	}

	for _, edge := range g2.Edges {
		if edge.Source == "" || edge.Target == "" {
			return nil, fmt.Errorf("graph edge source and target cannot be empty")
		}
		key := edgeKey(edge.Source, edge.Target)
		if idx, ok := edgeIndex[key]; ok {
			merged.Edges[idx].Weight = edge.Weight
			merged.Edges[idx].Attributes = mergeStringMaps(merged.Edges[idx].Attributes, edge.Attributes)
			continue
		}
		merged.Edges = append(merged.Edges, cloneGraphEdge(edge))
		edgeIndex[key] = len(merged.Edges) - 1
	}

	return merged, nil
}

func edgeKey(source, target string) string {
	return source + "\x00" + target
}

func cloneTopologyGraph(g *TopologyGraph) *TopologyGraph {
	if g == nil {
		return nil
	}

	clone := &TopologyGraph{
		Nodes: make([]GraphNode, 0, len(g.Nodes)),
		Edges: make([]GraphEdge, 0, len(g.Edges)),
	}
	for _, node := range g.Nodes {
		clone.Nodes = append(clone.Nodes, cloneGraphNode(node))
	}
	for _, edge := range g.Edges {
		clone.Edges = append(clone.Edges, cloneGraphEdge(edge))
	}
	return clone
}

func cloneGraphNode(node GraphNode) GraphNode {
	clone := GraphNode{ID: node.ID}
	if len(node.Attributes) > 0 {
		clone.Attributes = mergeStringMaps(nil, node.Attributes)
	}
	return clone
}

func cloneGraphEdge(edge GraphEdge) GraphEdge {
	clone := GraphEdge{
		Source: edge.Source,
		Target: edge.Target,
		Weight: edge.Weight,
	}
	if len(edge.Attributes) > 0 {
		clone.Attributes = mergeStringMaps(nil, edge.Attributes)
	}
	return clone
}

func mergeStringMaps(base, override map[string]string) map[string]string {
	if len(base) == 0 && len(override) == 0 {
		return nil
	}

	merged := make(map[string]string, len(base)+len(override))
	for k, v := range base {
		merged[k] = v
	}
	for k, v := range override {
		merged[k] = v
	}
	return merged
}

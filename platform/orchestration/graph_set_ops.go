package orchestration

import "fmt"

// GraphUnion returns a new TopologyGraph containing all nodes and edges from
// both input graphs. When a node or edge exists in both graphs, attributes
// from g2 override attributes from g1 on conflict. For overlapping edges,
// the weight from g2 wins. Returns an error if any node ID or edge
// source/target is empty.
func GraphUnion(g1, g2 *TopologyGraph) (*TopologyGraph, error) {
	return Union(g1, g2)
}

// GraphIntersection returns a new TopologyGraph containing only nodes and
// edges that are present in both input graphs. A node is considered present
// in both if its ID exists in both graphs. An edge is considered present in
// both if its source/target pair exists in both graphs.
//
// On conflict, attributes from g2 override attributes from g1, and the
// weight from g2 wins. Returns an error if any node ID or edge source/target
// is empty.
func GraphIntersection(g1, g2 *TopologyGraph) (*TopologyGraph, error) {
	if g1 == nil || g2 == nil {
		return &TopologyGraph{}, nil
	}

	// Build node ID set from g1.
	g1NodeIDs := make(map[string]GraphNode, len(g1.Nodes))
	for _, node := range g1.Nodes {
		if node.ID == "" {
			return nil, fmt.Errorf("graph node ID cannot be empty")
		}
		g1NodeIDs[node.ID] = node
	}

	// Build edge key set from g1.
	g1EdgeKeys := make(map[string]GraphEdge, len(g1.Edges))
	for _, edge := range g1.Edges {
		if edge.Source == "" || edge.Target == "" {
			return nil, fmt.Errorf("graph edge source and target cannot be empty")
		}
		g1EdgeKeys[edgeKey(edge.Source, edge.Target)] = edge
	}

	result := &TopologyGraph{
		Nodes: make([]GraphNode, 0),
		Edges: make([]GraphEdge, 0),
	}

	// Step 2: validate g2 nodes/edges, then intersect.
	for _, node := range g2.Nodes {
		if node.ID == "" {
			return nil, fmt.Errorf("graph node ID cannot be empty")
		}
		if _, ok := g1NodeIDs[node.ID]; ok {
			merged := cloneGraphNode(g1NodeIDs[node.ID])
			merged.Attributes = mergeStringMaps(merged.Attributes, node.Attributes)
			result.Nodes = append(result.Nodes, merged)
		}
	}

	for _, edge := range g2.Edges {
		if edge.Source == "" || edge.Target == "" {
			return nil, fmt.Errorf("graph edge source and target cannot be empty")
		}
		key := edgeKey(edge.Source, edge.Target)
		if g1Edge, ok := g1EdgeKeys[key]; ok {
			merged := cloneGraphEdge(g1Edge)
			merged.Weight = edge.Weight
			merged.Attributes = mergeStringMaps(merged.Attributes, edge.Attributes)
			result.Edges = append(result.Edges, merged)
		}
	}

	return result, nil
}

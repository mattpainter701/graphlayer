package orchestration

// Edge is kept as a compatibility alias for GraphEdge.
type Edge = GraphEdge

// GraphDiff computes the symmetric difference between two topology graphs.
//
// Nodes are identified by ID and edges are identified by their source/target
// pair. The returned added and removed edge slices contain the full edge values
// from the respective input graph.
func GraphDiff(g1, g2 *TopologyGraph) (addedNodes, removedNodes []string, addedEdges, removedEdges []Edge, err error) {
	if g1 == nil {
		g1 = &TopologyGraph{}
	}
	if g2 == nil {
		g2 = &TopologyGraph{}
	}

	oldNodes := make(map[string]struct{}, len(g1.Nodes))
	for _, node := range g1.Nodes {
		if node.ID == "" {
			return nil, nil, nil, nil, ErrInvalidGraphNodeID
		}
		oldNodes[node.ID] = struct{}{}
	}

	newNodes := make(map[string]struct{}, len(g2.Nodes))
	for _, node := range g2.Nodes {
		if node.ID == "" {
			return nil, nil, nil, nil, ErrInvalidGraphNodeID
		}
		newNodes[node.ID] = struct{}{}
	}

	for _, node := range g2.Nodes {
		if _, exists := oldNodes[node.ID]; !exists {
			addedNodes = append(addedNodes, node.ID)
		}
	}
	for _, node := range g1.Nodes {
		if _, exists := newNodes[node.ID]; !exists {
			removedNodes = append(removedNodes, node.ID)
		}
	}

	oldEdges := make(map[string]Edge, len(g1.Edges))
	for _, edge := range g1.Edges {
		if edge.Source == "" || edge.Target == "" {
			return nil, nil, nil, nil, ErrInvalidGraphEdgeEndpoints
		}
		oldEdges[edgeKey(edge.Source, edge.Target)] = edge
	}

	newEdges := make(map[string]Edge, len(g2.Edges))
	for _, edge := range g2.Edges {
		if edge.Source == "" || edge.Target == "" {
			return nil, nil, nil, nil, ErrInvalidGraphEdgeEndpoints
		}
		newEdges[edgeKey(edge.Source, edge.Target)] = edge
	}

	for _, edge := range g2.Edges {
		key := edgeKey(edge.Source, edge.Target)
		if _, exists := oldEdges[key]; !exists {
			addedEdges = append(addedEdges, edge)
		}
	}
	for _, edge := range g1.Edges {
		key := edgeKey(edge.Source, edge.Target)
		if _, exists := newEdges[key]; !exists {
			removedEdges = append(removedEdges, edge)
		}
	}

	return addedNodes, removedNodes, addedEdges, removedEdges, nil
}

// ErrInvalidGraphNodeID is returned when a graph contains an empty node ID.
var ErrInvalidGraphNodeID = graphDiffError("graph node ID cannot be empty")

// ErrInvalidGraphEdgeEndpoints is returned when a graph contains an edge with
// an empty source or target.
var ErrInvalidGraphEdgeEndpoints = graphDiffError("graph edge source and target cannot be empty")

type graphDiffError string

func (e graphDiffError) Error() string { return string(e) }

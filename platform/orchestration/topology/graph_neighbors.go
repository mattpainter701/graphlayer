package topology

import "fmt"

// NeighborCounts returns a map of node ID to the number of neighbors (degree)
// for each node in the graph. Isolated nodes appear with count 0.
func NeighborCounts(g *Graph) map[string]int {
	counts := make(map[string]int)
	if g == nil {
		return counts
	}

	// Initialize all explicitly listed nodes to 0 so isolated nodes appear.
	for _, node := range g.Nodes {
		counts[node.ID] = 0
	}

	// Count each edge endpoint.
	for _, edge := range g.Edges {
		if edge.Source != "" {
			counts[edge.Source]++
		}
		if edge.Target != "" {
			counts[edge.Target]++
		}
	}

	return counts
}

// NeighborCount returns the number of neighbors (degree) for a single node.
// Returns an error if the node ID does not exist in the graph.
func NeighborCount(g *Graph, nodeID string) (int, error) {
	if g == nil {
		return 0, fmt.Errorf("graph is nil")
	}

	// Verify the node exists in the graph.
	found := false
	for _, node := range g.Nodes {
		if node.ID == nodeID {
			found = true
			break
		}
	}
	if !found {
		return 0, fmt.Errorf("node %q not found in graph", nodeID)
	}

	count := 0
	for _, edge := range g.Edges {
		if edge.Source == nodeID {
			count++
		}
		if edge.Target == nodeID {
			count++
		}
	}

	return count, nil
}

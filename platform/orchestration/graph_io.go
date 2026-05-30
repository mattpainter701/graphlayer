package orchestration

import "fmt"

// ExportAdjacencyList exports the topology graph as an adjacency list
// mapping each node ID to a slice of its direct neighbor node IDs.
// The adjacency list follows edge direction (Source -> Target).
func ExportAdjacencyList(g *TopologyGraph) (map[string][]string, error) {
	if g == nil {
		return nil, fmt.Errorf("graph is nil")
	}

	adj := make(map[string][]string, len(g.Nodes))

	// Initialize all nodes with empty slices so isolated nodes appear.
	for _, node := range g.Nodes {
		if _, ok := adj[node.ID]; !ok {
			adj[node.ID] = []string{}
		}
	}

	// Walk edges and add directed adjacency entries (Source -> Target).
	for _, edge := range g.Edges {
		if _, ok := adj[edge.Source]; !ok {
			continue // edge references unknown node; skip
		}
		adj[edge.Source] = append(adj[edge.Source], edge.Target)
	}

	return adj, nil
}

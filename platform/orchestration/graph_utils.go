package orchestration

import "fmt"

// TopologicalSort performs a topological sort on a directed topology graph
// using Kahn's algorithm.
//
// Edges in the graph are treated as directed from Source (dependency) to
// Target (dependent): if there is an edge A→B, then A must appear before B
// in the result.
//
// Returns a slice of node IDs in topological order if the graph is a DAG.
// Returns an error if the graph is nil or contains a cycle.
func TopologicalSort(g *TopologyGraph) ([]string, error) {
	if g == nil {
		return nil, fmt.Errorf("graph is nil")
	}

	// Collect valid node IDs, skipping empty IDs.
	nodeIDs := make(map[string]struct{}, len(g.Nodes))
	for _, node := range g.Nodes {
		if node.ID == "" {
			continue
		}
		nodeIDs[node.ID] = struct{}{}
	}

	if len(nodeIDs) == 0 {
		return []string{}, nil
	}

	// Initialize in-degree map for all valid nodes.
	inDegree := make(map[string]int, len(nodeIDs))
	// Build directed adjacency list (Source -> Target).
	adjacency := make(map[string][]string, len(nodeIDs))
	for nodeID := range nodeIDs {
		inDegree[nodeID] = 0
	}

	for _, edge := range g.Edges {
		// Skip malformed edges.
		if edge.Source == "" || edge.Target == "" || edge.Source == edge.Target {
			continue
		}
		// Skip edges referencing nodes not in the graph.
		if _, ok := nodeIDs[edge.Source]; !ok {
			continue
		}
		if _, ok := nodeIDs[edge.Target]; !ok {
			continue
		}
		adjacency[edge.Source] = append(adjacency[edge.Source], edge.Target)
		inDegree[edge.Target]++
	}

	// Seed the queue with nodes that have no incoming edges.
	queue := make([]string, 0, len(nodeIDs))
	for nodeID := range nodeIDs {
		if inDegree[nodeID] == 0 {
			queue = append(queue, nodeID)
		}
	}

	result := make([]string, 0, len(nodeIDs))
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		result = append(result, node)

		for _, neighbor := range adjacency[node] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
			}
		}
	}

	// If not all nodes were processed, a cycle exists.
	if len(result) != len(nodeIDs) {
		return nil, fmt.Errorf(
			"graph contains a cycle: topological sort cannot proceed (processed %d of %d nodes)",
			len(result), len(nodeIDs),
		)
	}

	return result, nil
}

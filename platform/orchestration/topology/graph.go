package topology

import "fmt"

// NodeLevels performs a BFS from rootID and returns a map of node ID to its
// depth level (root = 0). Returns an error if rootID does not exist in the graph.
func NodeLevels(graph *Graph, rootID string) (map[string]int, error) {
	if graph == nil {
		return nil, fmt.Errorf("graph is nil")
	}

	adjacency := buildAdjacency(graph)
	if _, ok := adjacency[rootID]; !ok {
		return nil, fmt.Errorf("node %q not found in graph", rootID)
	}

	levels := map[string]int{rootID: 0}
	queue := []string{rootID}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		for neighbor := range adjacency[current] {
			if _, visited := levels[neighbor]; !visited {
				levels[neighbor] = levels[current] + 1
				queue = append(queue, neighbor)
			}
		}
	}

	return levels, nil
}

// ClusteringCoefficient returns the local clustering coefficient for nodeID.
//
// The value is the ratio of actual edges among the node's neighbors to the
// number of possible edges between those neighbors. The result is always in
// the range [0, 1]. Nodes with fewer than two neighbors have coefficient 0.
func ClusteringCoefficient(graph *Graph, nodeID string) float64 {
	adjacency := buildAdjacency(graph)
	return clusteringCoefficientFromAdjacency(adjacency, nodeID)
}

// GlobalClusteringCoefficient returns the average local clustering coefficient
// across all nodes in the graph.
func GlobalClusteringCoefficient(graph *Graph) float64 {
	adjacency := buildAdjacency(graph)
	if len(adjacency) == 0 {
		return 0
	}

	total := 0.0
	for nodeID := range adjacency {
		total += clusteringCoefficientFromAdjacency(adjacency, nodeID)
	}

	return total / float64(len(adjacency))
}

func clusteringCoefficientFromAdjacency(adjacency map[string]map[string]struct{}, nodeID string) float64 {
	neighbors, ok := adjacency[nodeID]
	if !ok {
		return 0
	}

	neighborIDs := make([]string, 0, len(neighbors))
	for neighborID := range neighbors {
		if neighborID == nodeID {
			continue
		}
		neighborIDs = append(neighborIDs, neighborID)
	}

	degree := len(neighborIDs)
	if degree < 2 {
		return 0
	}

	possibleEdges := degree * (degree - 1) / 2
	actualEdges := 0
	for i := 0; i < degree; i++ {
		for j := i + 1; j < degree; j++ {
			if _, ok := adjacency[neighborIDs[i]][neighborIDs[j]]; ok {
				actualEdges++
			}
		}
	}

	return float64(actualEdges) / float64(possibleEdges)
}

func buildAdjacency(graph *Graph) map[string]map[string]struct{} {
	adjacency := make(map[string]map[string]struct{})
	if graph == nil {
		return adjacency
	}

	ensureNode := func(nodeID string) {
		if nodeID == "" {
			return
		}
		if _, ok := adjacency[nodeID]; !ok {
			adjacency[nodeID] = make(map[string]struct{})
		}
	}

	for _, node := range graph.Nodes {
		ensureNode(node.ID)
	}

	for _, edge := range graph.Edges {
		if edge.Source == "" || edge.Target == "" {
			continue
		}

		ensureNode(edge.Source)
		ensureNode(edge.Target)
		if edge.Source == edge.Target {
			continue
		}

		adjacency[edge.Source][edge.Target] = struct{}{}
		adjacency[edge.Target][edge.Source] = struct{}{}
	}

	return adjacency
}

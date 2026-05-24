package graph

// BetweennessCentrality computes betweenness centrality for every node in the
// topology graph using Brandes' algorithm.
//
// The graph is treated as undirected for topology analysis so that a simple
// chain or star topology produces the expected centrality values regardless of
// edge direction in the source data.
func (graph *TopologyGraph) BetweennessCentrality() map[string]float64 {
	return BetweennessCentrality(graph)
}

// BetweennessCentrality computes betweenness centrality for every node in the
// topology graph using Brandes' algorithm.
func BetweennessCentrality(graph *TopologyGraph) map[string]float64 {
	centrality := make(map[string]float64)
	if graph == nil {
		return centrality
	}

	nodeSet := make(map[string]struct{}, len(graph.Nodes))
	for _, node := range graph.Nodes {
		if node.ID == "" {
			continue
		}
		nodeSet[node.ID] = struct{}{}
	}
	for _, edge := range graph.Edges {
		if edge.Source != "" {
			nodeSet[edge.Source] = struct{}{}
		}
		if edge.Target != "" {
			nodeSet[edge.Target] = struct{}{}
		}
	}

	nodeIDs := make([]string, 0, len(nodeSet))
	adjacency := make(map[string]map[string]struct{}, len(nodeSet))
	for nodeID := range nodeSet {
		nodeIDs = append(nodeIDs, nodeID)
		centrality[nodeID] = 0
		adjacency[nodeID] = make(map[string]struct{})
	}

	for _, edge := range graph.Edges {
		if edge.Source == "" || edge.Target == "" || edge.Source == edge.Target {
			continue
		}
		adjacency[edge.Source][edge.Target] = struct{}{}
		adjacency[edge.Target][edge.Source] = struct{}{}
	}

	for _, source := range nodeIDs {
		stack := make([]string, 0, len(nodeIDs))
		predecessors := make(map[string][]string, len(nodeIDs))
		sigma := make(map[string]float64, len(nodeIDs))
		distance := make(map[string]int, len(nodeIDs))
		for _, nodeID := range nodeIDs {
			distance[nodeID] = -1
		}
		distance[source] = 0
		sigma[source] = 1

		queue := make([]string, 0, len(nodeIDs))
		queue = append(queue, source)
		for head := 0; head < len(queue); head++ {
			v := queue[head]
			stack = append(stack, v)
			for w := range adjacency[v] {
				if distance[w] < 0 {
					distance[w] = distance[v] + 1
					queue = append(queue, w)
				}
				if distance[w] == distance[v]+1 {
					sigma[w] += sigma[v]
					predecessors[w] = append(predecessors[w], v)
				}
			}
		}

		delta := make(map[string]float64, len(nodeIDs))
		for i := len(stack) - 1; i >= 0; i-- {
			w := stack[i]
			for _, v := range predecessors[w] {
				if sigma[w] == 0 {
					continue
				}
				delta[v] += (sigma[v] / sigma[w]) * (1 + delta[w])
			}
			if w != source {
				centrality[w] += delta[w]
			}
		}
	}

	for nodeID := range centrality {
		centrality[nodeID] /= 2
	}

	return centrality
}

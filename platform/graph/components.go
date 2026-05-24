package graph

import "sort"

// ConnectedComponents returns the connected components of the topology graph.
//
// The graph is treated as undirected for topology analysis so that connectivity
// reflects physical or logical reachability regardless of edge direction in the
// source data.
func (graph *TopologyGraph) ConnectedComponents() [][]string {
	return ConnectedComponents(graph)
}

// ConnectedComponents returns the connected components of the topology graph.
func ConnectedComponents(graph *TopologyGraph) [][]string {
	if graph == nil {
		return [][]string{}
	}

	nodeSet := make(map[string]struct{}, len(graph.Nodes))
	order := make([]string, 0, len(graph.Nodes)+len(graph.Edges)*2)
	addNode := func(id string) {
		if id == "" {
			return
		}
		if _, exists := nodeSet[id]; exists {
			return
		}
		nodeSet[id] = struct{}{}
		order = append(order, id)
	}

	for _, node := range graph.Nodes {
		addNode(node.ID)
	}
	for _, edge := range graph.Edges {
		addNode(edge.Source)
		addNode(edge.Target)
	}

	if len(order) == 0 {
		return [][]string{}
	}

	adjacency := make(map[string][]string, len(nodeSet))
	neighborSets := make(map[string]map[string]struct{}, len(nodeSet))
	for nodeID := range nodeSet {
		neighborSets[nodeID] = make(map[string]struct{})
	}

	for _, edge := range graph.Edges {
		if edge.Source == "" || edge.Target == "" || edge.Source == edge.Target {
			continue
		}
		neighborSets[edge.Source][edge.Target] = struct{}{}
		neighborSets[edge.Target][edge.Source] = struct{}{}
	}

	for nodeID, neighbors := range neighborSets {
		adjacency[nodeID] = make([]string, 0, len(neighbors))
		for neighbor := range neighbors {
			adjacency[nodeID] = append(adjacency[nodeID], neighbor)
		}
		sort.Strings(adjacency[nodeID])
	}

	visited := make(map[string]struct{}, len(nodeSet))
	components := make([][]string, 0)

	for _, start := range order {
		if _, seen := visited[start]; seen {
			continue
		}

		queue := []string{start}
		visited[start] = struct{}{}
		component := make([]string, 0, len(nodeSet))

		for head := 0; head < len(queue); head++ {
			current := queue[head]
			component = append(component, current)
			for _, neighbor := range adjacency[current] {
				if _, seen := visited[neighbor]; seen {
					continue
				}
				visited[neighbor] = struct{}{}
				queue = append(queue, neighbor)
			}
		}

		sort.Strings(component)
		components = append(components, component)
	}

	return components
}

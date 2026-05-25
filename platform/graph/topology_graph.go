package graph

import (
	"fmt"
	"sort"
)

// NodeDegreeStats returns degree distribution statistics for a directed graph.
//
// The degree of each node is computed as in-degree + out-degree, counting each
// edge once toward its source and once toward its target. The returned histogram
// maps a degree value to the number of nodes with that degree.
func NodeDegreeStats(graph *TopologyGraph) (min, max, avg float64, histogram map[int]int) {
	histogram = make(map[int]int)
	if graph == nil || len(graph.Nodes) == 0 {
		return 0, 0, 0, histogram
	}

	inDegree := make(map[string]int, len(graph.Nodes))
	outDegree := make(map[string]int, len(graph.Nodes))
	for _, node := range graph.Nodes {
		inDegree[node.ID] = 0
		outDegree[node.ID] = 0
	}

	for _, edge := range graph.Edges {
		if _, ok := outDegree[edge.Source]; ok {
			outDegree[edge.Source]++
		}
		if _, ok := inDegree[edge.Target]; ok {
			inDegree[edge.Target]++
		}
	}

	first := true
	total := 0
	for _, node := range graph.Nodes {
		degree := inDegree[node.ID] + outDegree[node.ID]
		histogram[degree]++
		total += degree

		if first {
			min = float64(degree)
			max = float64(degree)
			first = false
			continue
		}

		if float64(degree) < min {
			min = float64(degree)
		}
		if float64(degree) > max {
			max = float64(degree)
		}
	}

	avg = float64(total) / float64(len(graph.Nodes))
	return min, max, avg, histogram
}

// ShortestPath returns the shortest path between two node IDs using BFS.
//
// The graph is treated as undirected for topology analysis so that reachability
// reflects the physical or logical connectivity represented by the topology.
func ShortestPath(graph *TopologyGraph, fromID, toID string) ([]string, error) {
	if graph == nil {
		return nil, fmt.Errorf("graph is nil")
	}
	if fromID == "" || toID == "" {
		return nil, fmt.Errorf("fromID and toID must be non-empty")
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

	if _, ok := nodeSet[fromID]; !ok {
		return nil, fmt.Errorf("node not found: %s", fromID)
	}
	if _, ok := nodeSet[toID]; !ok {
		return nil, fmt.Errorf("node not found: %s", toID)
	}
	if fromID == toID {
		return []string{fromID}, nil
	}

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

	adjacency := make(map[string][]string, len(nodeSet))
	for nodeID, neighbors := range neighborSets {
		adjacency[nodeID] = make([]string, 0, len(neighbors))
		for neighbor := range neighbors {
			adjacency[nodeID] = append(adjacency[nodeID], neighbor)
		}
		sort.Strings(adjacency[nodeID])
	}

	type pathState struct {
		node string
		path []string
	}

	visited := make(map[string]struct{}, len(nodeSet))
	queue := []pathState{{node: fromID, path: []string{fromID}}}
	visited[fromID] = struct{}{}

	for head := 0; head < len(queue); head++ {
		current := queue[head]
		for _, neighbor := range adjacency[current.node] {
			if _, seen := visited[neighbor]; seen {
				continue
			}
			nextPath := append(append([]string{}, current.path...), neighbor)
			if neighbor == toID {
				return nextPath, nil
			}
			visited[neighbor] = struct{}{}
			queue = append(queue, pathState{node: neighbor, path: nextPath})
		}
	}

	return nil, fmt.Errorf("no path found from %s to %s", fromID, toID)
}

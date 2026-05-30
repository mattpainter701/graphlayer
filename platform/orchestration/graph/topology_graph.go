package graph

// NodeDegreeStats computes the degree distribution for all nodes in the graph.
//
// For each node, the total degree is computed as out-degree + in-degree.
// out-degree: number of edges originating from the node.
// in-degree:  number of edges terminating at the node.
//
// Returns the minimum, maximum, and average total degree across all nodes,
// and a histogram mapping each degree value to the number of nodes with
// that degree. Isolated nodes contribute degree 0 to the histogram.
//
// If the graph is nil or has no nodes, min, max, avg are 0 and the
// histogram is empty.
func NodeDegreeStats(graph *TopologyGraph) (min, max, avg float64, histogram map[int]int) {
	deg := computeDegrees(graph)
	if len(deg) == 0 {
		return 0, 0, 0, map[int]int{}
	}

	histogram = make(map[int]int)
	min = float64(deg[0])
	max = float64(deg[0])
	sum := 0

	for _, d := range deg {
		if float64(d) < min {
			min = float64(d)
		}
		if float64(d) > max {
			max = float64(d)
		}
		sum += d
		histogram[d]++
	}

	avg = float64(sum) / float64(len(deg))
	return min, max, avg, histogram
}

// computeDegrees returns the total degree for each node in the graph.
// The result is a slice of total degrees (out-degree + in-degree) per node.
func computeDegrees(graph *TopologyGraph) []int {
	if graph == nil {
		return nil
	}

	// out-degree: number of outgoing edges
	// in-degree:  number of incoming edges
	outDeg := make(map[string]int, len(graph.Nodes))
	inDeg := make(map[string]int, len(graph.Nodes))

	// Initialize all nodes to 0 so isolated nodes appear.
	for _, node := range graph.Nodes {
		outDeg[node.ID] = 0
		inDeg[node.ID] = 0
	}

	// Count outgoing edges.
	for _, srcEdges := range graph.Edges {
		for _, edge := range srcEdges {
			outDeg[edge.Source]++
			inDeg[edge.Target]++
		}
	}

	// Build total degree slice.
	degrees := make([]int, 0, len(graph.Nodes))
	for _, node := range graph.Nodes {
		degrees = append(degrees, outDeg[node.ID]+inDeg[node.ID])
	}

	return degrees
}

package graph

// EdgeTypeDistribution counts edges by their Type field and returns a map
// of edge type to count. Handles nil graphs, empty type strings, and graphs
// with no edges gracefully.
func EdgeTypeDistribution(g *TopologyGraph) map[string]int {
	if g == nil {
		return map[string]int{}
	}

	dist := make(map[string]int)
	for _, edgeMap := range g.Edges {
		for _, edge := range edgeMap {
			dist[edge.Type]++
		}
	}
	return dist
}

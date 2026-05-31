package graph

// ArticulationPoints returns all articulation points (cut vertices) in the
// graph using Tarjan's DFS-based algorithm. An articulation point is a node
// whose removal increases the number of connected components in the graph.
//
// The graph is treated as undirected. Self-loops and duplicate edges are
// ignored. Returns nil for a nil or empty graph.
func ArticulationPoints(g *TopologyGraph) []string {
	if g == nil || len(g.Nodes) == 0 {
		return nil
	}

	nodeIDs, adjacency := buildUndirectedAdjacency(g)
	if len(nodeIDs) == 0 {
		return nil
	}

	// Discard nodes that are absent from the adjacency map (not reachable
	// from any edge). Single isolated nodes have zero neighbors and are not
	// articulation points.
	hasEdges := false
	for _, neighbors := range adjacency {
		if len(neighbors) > 0 {
			hasEdges = true
			break
		}
	}
	if !hasEdges {
		return nil
	}

	disc := make(map[string]int, len(nodeIDs))
	low := make(map[string]int, len(nodeIDs))
	parent := make(map[string]string, len(nodeIDs))
	visited := make(map[string]bool, len(nodeIDs))
	artPoints := make(map[string]bool)
	// Use a pointer to a time counter so the inner closure can mutate it.
	time := 0

	var dfs func(u string)
	dfs = func(u string) {
		visited[u] = true
		time++
		disc[u] = time
		low[u] = time
		childCount := 0

		for v := range adjacency[u] {
			if !visited[v] {
				childCount++
				parent[v] = u
				dfs(v)

				if low[v] < low[u] {
					low[u] = low[v]
				}

				// u is an articulation point in the DFS tree (non-root):
				// if no back edge from subtree of v reaches an ancestor of u.
				if parent[u] != "" && low[v] >= disc[u] {
					artPoints[u] = true
				}
			} else if v != parent[u] {
				// Back edge: v is already visited and not the parent.
				if disc[v] < low[u] {
					low[u] = disc[v]
				}
			}
		}

		// Root of DFS tree: articulation point if it has 2+ children.
		if parent[u] == "" && childCount > 1 {
			artPoints[u] = true
		}
	}

	for _, nodeID := range nodeIDs {
		if !visited[nodeID] {
			dfs(nodeID)
		}
	}

	result := make([]string, 0, len(artPoints))
	for nodeID := range artPoints {
		result = append(result, nodeID)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

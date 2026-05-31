package graph

import "sort"

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

	// Build a list of all node IDs.
	nodeIDs := make([]string, 0, len(g.Nodes))
	for id := range g.Nodes {
		nodeIDs = append(nodeIDs, id)
	}
	sort.Strings(nodeIDs)

	// Build undirected adjacency (deduplicated).
	adj := make(map[string]map[string]struct{}, len(g.Nodes))
	for _, id := range nodeIDs {
		adj[id] = make(map[string]struct{})
	}
	for src, targets := range g.Edges {
		for tgt := range targets {
			if src == tgt {
				continue // self-loops are irrelevant
			}
			if _, ok := adj[src]; ok {
				adj[src][tgt] = struct{}{}
			}
			if _, ok := adj[tgt]; ok {
				adj[tgt][src] = struct{}{}
			}
		}
	}

	// Check if any node has at least one neighbor.
	hasEdges := false
	for _, neighbors := range adj {
		if len(neighbors) > 0 {
			hasEdges = true
			break
		}
	}
	if !hasEdges {
		return nil
	}

	// --- Tarjan's DFS ------------------------------------------------------
	disc := make(map[string]int, len(nodeIDs))
	low := make(map[string]int, len(nodeIDs))
	parent := make(map[string]string, len(nodeIDs))
	visited := make(map[string]bool, len(nodeIDs))
	artPoints := make(map[string]bool)
	time := 0

	var dfs func(u string)
	dfs = func(u string) {
		visited[u] = true
		time++
		disc[u] = time
		low[u] = time
		childCount := 0

		for v := range adj[u] {
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

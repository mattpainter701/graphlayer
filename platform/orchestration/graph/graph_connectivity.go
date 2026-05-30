package graph

import "sort"

// FindBridges returns all bridges (cut edges) in the undirected representation
// of the topology graph using Tarjan's DFS-based algorithm.
//
// The graph stores directed edges; FindBridges treats each unordered pair of
// nodes with at least one directed edge as an undirected connection.  A bridge
// is reported only when exactly one directed edge connects the pair — multiple
// directed edges between the same nodes (e.g., both A→B and B→A) make the
// connection redundant and it is not a bridge.
//
// Returns a sorted, deduplicated slice of [2]string pairs where each pair
// represents a bridge (cut edge).  The pairs are sorted with the smaller node
// ID first.
func FindBridges(g *TopologyGraph) [][2]string {
	if g == nil {
		return [][2]string{}
	}

	// edgeCount tracks how many directed edges exist for each unordered pair.
	type pair struct{ a, b string }
	edgeCount := make(map[pair]int)

	// Build undirected adjacency (deduplicated) plus edge counts.
	adj := make(map[string][]string)
	for src, targets := range g.Edges {
		for tgt := range targets {
			if src == tgt {
				continue // self-loops are irrelevant
			}
			p := pair{src, tgt}
			if src > tgt {
				p = pair{tgt, src}
			}
			edgeCount[p]++

			if !contains(adj[src], tgt) {
				adj[src] = append(adj[src], tgt)
			}
			if !contains(adj[tgt], src) {
				adj[tgt] = append(adj[tgt], src)
			}
		}
	}
	// Ensure isolated nodes appear in the adjacency map.
	for id := range g.Nodes {
		if _, ok := adj[id]; !ok {
			adj[id] = nil
		}
	}

	// --- Tarjan's DFS ------------------------------------------------------
	disc := make(map[string]int) // discovery time
	low := make(map[string]int)  // low-link value
	visited := make(map[string]bool)
	time := 0

	resultSet := make(map[pair]bool)

	var dfs func(u, parent string)
	dfs = func(u, parent string) {
		visited[u] = true
		disc[u] = time
		low[u] = time
		time++

		for _, v := range adj[u] {
			if v == parent {
				continue
			}
			if !visited[v] {
				dfs(v, u)
				if low[v] < low[u] {
					low[u] = low[v]
				}
				if low[v] > disc[u] {
					// undirected edge (u,v) is a bridge iff exactly one directed
					// edge exists between this pair.
					p := pair{u, v}
					if u > v {
						p = pair{v, u}
					}
					if edgeCount[p] == 1 {
						resultSet[p] = true
					}
				}
			} else {
				if disc[v] < low[u] {
					low[u] = disc[v]
				}
			}
		}
	}

	// Stable DFS order.
	nodeIDs := make([]string, 0, len(g.Nodes))
	for id := range g.Nodes {
		nodeIDs = append(nodeIDs, id)
	}
	sort.Strings(nodeIDs)

	for _, id := range nodeIDs {
		if !visited[id] {
			dfs(id, "")
		}
	}

	// Build sorted result from set.
	bridges := make([][2]string, 0, len(resultSet))
	for p := range resultSet {
		a, b := p.a, p.b
		if a > b {
			a, b = b, a
		}
		bridges = append(bridges, [2]string{a, b})
	}
	sort.Slice(bridges, func(i, j int) bool {
		if bridges[i][0] != bridges[j][0] {
			return bridges[i][0] < bridges[j][0]
		}
		return bridges[i][1] < bridges[j][1]
	})

	return bridges
}

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

package commands

import (
	"fmt"
	"strings"

	"github.com/mattpainter701/graphlayer/platform/orchestration/topology"
)

// FindOrphanNodes returns a slice of node IDs that have zero incident edges
// (degree 0 / isolated nodes) in the given graph. Returns nil for a nil graph.
func FindOrphanNodes(g *topology.Graph) []string {
	if g == nil {
		return nil
	}

	// Build a set of node IDs that appear in at least one edge endpoint.
	inEdge := make(map[string]bool)
	for _, edge := range g.Edges {
		if edge.Source != "" {
			inEdge[edge.Source] = true
		}
		if edge.Target != "" {
			inEdge[edge.Target] = true
		}
	}

	var orphans []string
	for _, node := range g.Nodes {
		if !inEdge[node.ID] {
			orphans = append(orphans, node.ID)
		}
	}
	return orphans
}

// CountDisconnectedSubgraphs returns the number of connected components in the
// graph using undirected BFS. Every isolated node counts as its own component.
// Returns 0 for a nil or empty graph.
func CountDisconnectedSubgraphs(g *topology.Graph) int {
	if g == nil || len(g.Nodes) == 0 {
		return 0
	}

	// Build undirected adjacency map.
	adj := make(map[string]map[string]struct{})
	for _, node := range g.Nodes {
		if _, ok := adj[node.ID]; !ok {
			adj[node.ID] = make(map[string]struct{})
		}
	}
	for _, edge := range g.Edges {
		if edge.Source == "" || edge.Target == "" {
			continue
		}
		if _, ok := adj[edge.Source]; !ok {
			adj[edge.Source] = make(map[string]struct{})
		}
		if _, ok := adj[edge.Target]; !ok {
			adj[edge.Target] = make(map[string]struct{})
		}
		adj[edge.Source][edge.Target] = struct{}{}
		adj[edge.Target][edge.Source] = struct{}{}
	}

	visited := make(map[string]bool)
	components := 0

	for _, node := range g.Nodes {
		if visited[node.ID] {
			continue
		}
		components++

		// BFS from this unvisited node.
		queue := []string{node.ID}
		visited[node.ID] = true
		for len(queue) > 0 {
			cur := queue[0]
			queue = queue[1:]
			for neighbor := range adj[cur] {
				if !visited[neighbor] {
					visited[neighbor] = true
					queue = append(queue, neighbor)
				}
			}
		}
	}

	return components
}

// OrphanResult holds the structured output of an orphan analysis.
type OrphanResult struct {
	OrphanNodeIDs         []string `json:"orphan_node_ids"`
	DisconnectedSubgraphs int      `json:"disconnected_subgraphs"`
	TotalNodes            int      `json:"total_nodes"`
	TotalEdges            int      `json:"total_edges"`
}

// AnalyzeOrphans runs the full orphan analysis on a graph and returns a
// structured result. The graph is treated as undirected.
func AnalyzeOrphans(g *topology.Graph) OrphanResult {
	result := OrphanResult{}
	if g == nil {
		return result
	}
	result.TotalNodes = len(g.Nodes)
	result.TotalEdges = len(g.Edges)
	result.OrphanNodeIDs = FindOrphanNodes(g)
	result.DisconnectedSubgraphs = CountDisconnectedSubgraphs(g)
	return result
}

// FormatOrphansOutput renders an OrphanResult as a human-readable string.
// When showSubgraphs is false the disconnected-subgraph line is omitted.
func FormatOrphansOutput(result OrphanResult, showSubgraphs bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Topology has %d nodes and %d edges.\n", result.TotalNodes, result.TotalEdges)

	if len(result.OrphanNodeIDs) > 0 {
		fmt.Fprintf(&b, "Orphan nodes (zero edges): %d\n", len(result.OrphanNodeIDs))
		fmt.Fprintf(&b, "  %s\n", strings.Join(result.OrphanNodeIDs, ", "))
	} else {
		fmt.Fprintf(&b, "No orphan nodes found.\n")
	}

	if showSubgraphs {
		fmt.Fprintf(&b, "Disconnected subgraphs: %d\n", result.DisconnectedSubgraphs)
	}
	return b.String()
}

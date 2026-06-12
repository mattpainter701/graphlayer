package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mattpainter701/graphlayer/platform/platform/orchestration/graph"
	"github.com/spf13/cobra"
)

var (
	shortestPathInput  string
	shortestPathSource string
	shortestPathTarget string
)

// graphShortestPathCmd finds the shortest path between two nodes.
var graphShortestPathCmd = &cobra.Command{
	Use:   "shortest-path",
	Short: "Find shortest path between two nodes in a topology graph",
	Long: `Compute the shortest (unweighted) path between a source and target node
using BFS. The topology graph is read from a JSON file.

Examples:
  graphlayer graph shortest-path --input graph.json --source R1 --target R5
  graphlayer graph shortest-path --input net.json --source FW1 --target SW3`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runShortestPath(cmd)
	},
}

func init() {
	graphCmd.AddCommand(graphShortestPathCmd)
	graphShortestPathCmd.Flags().StringVar(&shortestPathInput, "input", "", "Path to graph JSON file")
	graphShortestPathCmd.Flags().StringVar(&shortestPathSource, "source", "", "Source node ID")
	graphShortestPathCmd.Flags().StringVar(&shortestPathTarget, "target", "", "Target node ID")
	_ = graphShortestPathCmd.MarkFlagRequired("input")
	_ = graphShortestPathCmd.MarkFlagRequired("source")
	_ = graphShortestPathCmd.MarkFlagRequired("target")
}

func runShortestPath(cmd *cobra.Command) error {
	data, err := os.ReadFile(shortestPathInput)
	if err != nil {
		return fmt.Errorf("failed to read input file: %w", err)
	}

	var g graph.TopologyGraph
	if err := json.Unmarshal(data, &g); err != nil {
		return fmt.Errorf("failed to parse graph JSON: %w", err)
	}

	src := shortestPathSource
	tgt := shortestPathTarget

	if _, ok := g.Nodes[src]; !ok {
		return fmt.Errorf("source node %q not found in graph", src)
	}
	if _, ok := g.Nodes[tgt]; !ok {
		return fmt.Errorf("target node %q not found in graph", tgt)
	}

	path, found := bfsShortestPath(&g, src, tgt)
	if !found {
		fmt.Fprintf(cmd.OutOrStdout(), "No path found from %s to %s\n", src, tgt)
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Shortest path (%d hops): %s\n", len(path)-1, strings.Join(path, " -> "))
	return nil
}

// bfsShortestPath returns the shortest path from src to tgt using BFS.
// The graph is treated as undirected for pathfinding purposes.
// Returns the path as a slice of node IDs and whether a path was found.
func bfsShortestPath(g *graph.TopologyGraph, src, tgt string) ([]string, bool) {
	if src == tgt {
		return []string{src}, true
	}

	// Build undirected adjacency list
	adj := make(map[string][]string, len(g.Nodes))
	for id := range g.Nodes {
		adj[id] = nil
	}
	for srcID, targets := range g.Edges {
		for tgtID := range targets {
			if srcID == tgtID {
				continue
			}
			adj[srcID] = append(adj[srcID], tgtID)
			adj[tgtID] = append(adj[tgtID], srcID)
		}
	}

	// BFS
	visited := map[string]bool{src: true}
	parent := map[string]string{}
	queue := []string{src}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		for _, neighbor := range adj[current] {
			if visited[neighbor] {
				continue
			}
			visited[neighbor] = true
			parent[neighbor] = current

			if neighbor == tgt {
				// Reconstruct path
				path := []string{tgt}
				for p := parent[tgt]; p != src; p = parent[p] {
					path = append([]string{p}, path...)
				}
				path = append([]string{src}, path...)
				return path, true
			}

			queue = append(queue, neighbor)
		}
	}

	return nil, false
}

package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/mattpainter701/graphlayer/platform/platform/orchestration/graph"
	"github.com/spf13/cobra"
)

var (
	graphStatsInput string
)

var graphStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Compute graph statistics from a topology graph JSON file",
	Long: `Read a topology graph from a JSON file and print summary statistics:
node count, edge count, average degree, number of connected components,
and graph density.

The input JSON must match the TopologyGraph format:
{
  "nodes": [{"id": "..."}, ...],
  "edges": [{"source": "...", "target": "..."}, ...]
}`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runGraphStats(cmd)
	},
}

func init() {
	graphCmd.AddCommand(graphStatsCmd)
	graphStatsCmd.Flags().StringVar(&graphStatsInput, "input", "", "Path to graph JSON file")
	_ = graphStatsCmd.MarkFlagRequired("input")
}

func runGraphStats(cmd *cobra.Command) error {
	// Read the input JSON file
	data, err := os.ReadFile(graphStatsInput)
	if err != nil {
		return fmt.Errorf("failed to read input file: %w", err)
	}

	// Parse into TopologyGraph
	var g graph.TopologyGraph
	if err := json.Unmarshal(data, &g); err != nil {
		return fmt.Errorf("failed to parse graph JSON: %w", err)
	}

	nodeCount := len(g.Nodes)
	edgeCount := len(g.Edges)

	// Average degree = 2*E / N
	var avgDegree float64
	if nodeCount > 0 {
		avgDegree = 2.0 * float64(edgeCount) / float64(nodeCount)
	}

	// Graph density = 2*E / (N * (N-1))
	var density float64
	if nodeCount > 1 {
		density = 2.0 * float64(edgeCount) / float64(nodeCount*(nodeCount-1))
	}

	// Number of connected components (BFS traversal)
	components := countConnectedComponents(&g)

	fmt.Fprintf(cmd.OutOrStdout(), "Node count: %d\n", nodeCount)
	fmt.Fprintf(cmd.OutOrStdout(), "Edge count: %d\n", edgeCount)
	fmt.Fprintf(cmd.OutOrStdout(), "Average degree: %.4f\n", avgDegree)
	fmt.Fprintf(cmd.OutOrStdout(), "Connected components: %d\n", components)
	fmt.Fprintf(cmd.OutOrStdout(), "Graph density: %.6f\n", density)

	return nil
}

// countConnectedComponents returns the number of connected components in an
// undirected topology graph, using BFS.
func countConnectedComponents(g *graph.TopologyGraph) int {
	if g == nil || len(g.Nodes) == 0 {
		return 0
	}

	// Build adjacency set
	nodeIDs := make([]string, 0, len(g.Nodes))
	adj := make(map[string]map[string]struct{}, len(g.Nodes))
	for _, node := range g.Nodes {
		nodeIDs = append(nodeIDs, node.ID)
		adj[node.ID] = make(map[string]struct{})
	}

	for _, edge := range g.Edges {
		if edge.Source == "" || edge.Target == "" || edge.Source == edge.Target {
			continue
		}
		if _, ok := adj[edge.Source]; ok {
			adj[edge.Source][edge.Target] = struct{}{}
		}
		if _, ok := adj[edge.Target]; ok {
			adj[edge.Target][edge.Source] = struct{}{}
		}
	}

	visited := make(map[string]bool, len(nodeIDs))
	components := 0

	for _, nodeID := range nodeIDs {
		if visited[nodeID] {
			continue
		}
		components++
		// BFS
		queue := []string{nodeID}
		visited[nodeID] = true
		for len(queue) > 0 {
			current := queue[0]
			queue = queue[1:]
			for neighbor := range adj[current] {
				if !visited[neighbor] {
					visited[neighbor] = true
					queue = append(queue, neighbor)
				}
			}
		}
	}

	return components
}

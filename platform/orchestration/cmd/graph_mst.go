package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mattpainter701/graphlayer/platform/orchestration/graph"
	"github.com/spf13/cobra"
)

var mstInput string

// graphMSTCmd computes the minimum spanning tree of a topology graph.
var graphMSTCmd = &cobra.Command{
	Use:   "mst",
	Short: "Compute minimum spanning tree of a topology graph",
	Long: `Compute the minimum spanning tree using Kruskal's algorithm with Union-Find.
The topology graph is read from a JSON file. The graph is treated as undirected.

Examples:
  graphlayer graph mst --input graph.json
  graphlayer graph mst --input network.json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runMST(cmd)
	},
}

func init() {
	graphCmd.AddCommand(graphMSTCmd)
	graphMSTCmd.Flags().StringVar(&mstInput, "input", "", "Path to graph JSON file")
	_ = graphMSTCmd.MarkFlagRequired("input")
}

func runMST(cmd *cobra.Command) error {
	data, err := os.ReadFile(mstInput)
	if err != nil {
		return fmt.Errorf("failed to read input file: %w", err)
	}

	var g graph.TopologyGraph
	if err := json.Unmarshal(data, &g); err != nil {
		return fmt.Errorf("failed to parse graph JSON: %w", err)
	}

	edges, total, err := graph.MinimumSpanningTree(&g)
	if err != nil {
		return fmt.Errorf("MST computation failed: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Minimum Spanning Tree (%d edges, total weight: %.1f):\n", len(edges), total)
	for _, e := range edges {
		fmt.Fprintf(cmd.OutOrStdout(), "  %s --[%.1f]--> %s\n", e.Source, e.Weight, e.Target)
	}

	nodes := make([]string, 0, len(g.Nodes))
	for id := range g.Nodes {
		nodes = append(nodes, id)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "\nNodes: %s\n", strings.Join(nodes, ", "))

	return nil
}

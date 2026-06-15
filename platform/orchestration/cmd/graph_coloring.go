package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/mattpainter701/graphlayer/platform/platform/orchestration/graph"
	"github.com/spf13/cobra"
)

var coloringInput string

var graphColoringCmd = &cobra.Command{
	Use:   "coloring",
	Short: "Compute graph coloring (Welsh-Powell algorithm)",
	Long:  "Compute a greedy graph coloring using the Welsh-Powell algorithm.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runColoring(cmd)
	},
}

func init() {
	graphCmd.AddCommand(graphColoringCmd)
	graphColoringCmd.Flags().StringVar(&coloringInput, "input", "", "Path to graph JSON file")
	_ = graphColoringCmd.MarkFlagRequired("input")
}

func runColoring(cmd *cobra.Command) error {
	data, err := os.ReadFile(coloringInput)
	if err != nil {
		return fmt.Errorf("failed to read input file: %w", err)
	}
	var g graph.TopologyGraph
	if err := json.Unmarshal(data, &g); err != nil {
		return fmt.Errorf("failed to parse graph JSON: %w", err)
	}
	result, err := graph.GraphColoring(&g)
	if err != nil {
		return fmt.Errorf("coloring computation failed: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Graph Coloring (%d colors used):\n", result.NumColors)
	type nodeColor struct {
		id    string
		color int
	}
	var sorted []nodeColor
	for id, c := range result.Colors {
		sorted = append(sorted, nodeColor{id: id, color: c})
	}
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].color == sorted[j].color {
			return sorted[i].id < sorted[j].id
		}
		return sorted[i].color < sorted[j].color
	})
	for _, nc := range sorted {
		fmt.Fprintf(cmd.OutOrStdout(), "  %s -> color %d\n", nc.id, nc.color)
	}
	return nil
}

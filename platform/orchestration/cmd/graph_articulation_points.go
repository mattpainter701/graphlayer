package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/mattpainter701/graphlayer/platform/platform/orchestration/graph"
	"github.com/spf13/cobra"
)

var (
	graphArticulationPointsInput string
	graphArticulationPointsJSON  bool
)

var graphArticulationPointsCmd = &cobra.Command{
	Use:   "articulation-points",
	Short: "Find articulation points (cut vertices) in a topology graph",
	Long: `Read a topology graph from a JSON file and find all articulation points
(cut vertices) using Tarjan's DFS-based algorithm.

An articulation point is a node whose removal increases the number of
connected components in the graph.

The input JSON must match the TopologyGraph format:
{
  "nodes": [{"id": "..."}, ...],
  "edges": [{"source": "...", "target": "..."}, ...]
}

Examples:
  graphlayer graph articulation-points --input graph.json
  graphlayer graph articulation-points --json --input graph.json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runGraphArticulationPoints()
	},
}

func init() {
	graphCmd.AddCommand(graphArticulationPointsCmd)
	graphArticulationPointsCmd.Flags().StringVar(&graphArticulationPointsInput, "input", "", "Path to graph JSON file")
	graphArticulationPointsCmd.Flags().BoolVarP(&graphArticulationPointsJSON, "json", "j", false, "Output as JSON array")
	_ = graphArticulationPointsCmd.MarkFlagRequired("input")
}

func runGraphArticulationPoints() error {
	data, err := os.ReadFile(graphArticulationPointsInput)
	if err != nil {
		return fmt.Errorf("failed to read input file: %w", err)
	}

	var g graph.TopologyGraph
	if err := json.Unmarshal(data, &g); err != nil {
		return fmt.Errorf("failed to parse graph JSON: %w", err)
	}

	points := graph.ArticulationPoints(&g)

	if graphArticulationPointsJSON {
		if points == nil {
			fmt.Fprint(os.Stdout, "[]\n")
			return nil
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(points); err != nil {
			return fmt.Errorf("failed to encode JSON output: %w", err)
		}
		return nil
	}

	if len(points) == 0 {
		fmt.Fprintln(os.Stdout, "No articulation points found.")
		return nil
	}

	sorted := make([]string, len(points))
	copy(sorted, points)
	sort.Strings(sorted)

	fmt.Fprintf(os.Stdout, "Articulation points (%d):\n", len(sorted))
	for _, id := range sorted {
		fmt.Fprintf(os.Stdout, "  %s\n", id)
	}

	return nil
}

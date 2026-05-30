package cmd

import (
	"fmt"
	"strings"

	"github.com/mattpainter701/graphlayer/platform/platform/orchestration/orchestration"
	"github.com/spf13/cobra"
)

var bridgesGraphID string

var graphCmd = &cobra.Command{
	Use:   "graph",
	Short: "Graph analysis utilities",
}

var bridgesCmd = &cobra.Command{
	Use:   "bridges",
	Short: "Find all bridges (cut edges) in a topology graph",
	Long: `Find all bridges (cut edges) in the undirected representation of a topology graph.

A bridge is an edge whose removal disconnects the graph, increasing the number
of connected components.  The underlying algorithm is Tarjan's DFS-based
bridge-finding algorithm.

Examples:
  graphlayer graph bridges --graph tree
  graphlayer graph bridges --graph line
  graphlayer graph bridges --graph triangle`,

	RunE: func(cmd *cobra.Command, args []string) error {
		bridges, err := orchestration.Bridges(bridgesGraphID)
		if err != nil {
			return fmt.Errorf("failed to find bridges: %w", err)
		}

		if len(bridges) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No bridges found.")
			return nil
		}

		pairs := make([]string, 0, len(bridges))
		for _, b := range bridges {
			pairs = append(pairs, fmt.Sprintf("  %s -- %s", b[0], b[1]))
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Bridges (%d):\n", len(bridges))
		fmt.Fprintln(cmd.OutOrStdout(), strings.Join(pairs, "\n"))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(graphCmd)
	graphCmd.AddCommand(bridgesCmd)

	bridgesCmd.Flags().StringVar(&bridgesGraphID, "graph", "", "Graph ID to analyze for bridges")
	_ = bridgesCmd.MarkFlagRequired("graph")
}

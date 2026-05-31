package cmd

import (
	"github.com/spf13/cobra"
)

// graphCmd represents the graph analysis subcommand
var graphCmd = &cobra.Command{
	Use:   "graph",
	Short: "Graph analysis utilities",
	Long: `Analyze topology graphs with subcommands for statistics, centrality, and more.

Examples:
  graphlayer graph stats --input graph.json`,
}

func init() {
	rootCmd.AddCommand(graphCmd)
}

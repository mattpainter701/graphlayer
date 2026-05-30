package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/mattpainter701/graphlayer/platform/platform/orchestration/commands"
	"github.com/mattpainter701/graphlayer/platform/platform/orchestration/topology"
	"github.com/spf13/cobra"
)

var (
	orphansShowSubgraphs bool
	orphansOutputJSON    bool
)

// topologyCmd represents the "topology" command group.
var topologyCmd = &cobra.Command{
	Use:   "topology",
	Short: "Topology analysis commands",
}

// graphCmd represents the "topology graph" subcommand group.
var graphCmd = &cobra.Command{
	Use:   "graph",
	Short: "Graph topology analysis commands",
}

// orphansCmd represents the "topology graph orphans" subcommand.
var orphansCmd = &cobra.Command{
	Use:   "orphans",
	Short: "Find orphan nodes and disconnected subgraphs",
	Long: `Find all nodes with zero edges (isolated/orphan nodes) and optionally count
the number of disconnected subgraphs in the topology graph.

An orphan node is a node with degree 0 — it has no incident edges.
A disconnected subgraph is a connected component that is isolated
from the rest of the graph.

Examples:
  graphlayer topology graph orphans graph.json
  graphlayer topology graph orphans --subgraphs graph.json
  graphlayer topology graph orphans --json graph.json
  cat graph.json | graphlayer topology graph orphans`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runOrphans()
	},
}

func init() {
	topologyCmd.AddCommand(graphCmd)
	graphCmd.AddCommand(orphansCmd)
	rootCmd.AddCommand(topologyCmd)

	orphansCmd.Flags().BoolVarP(&orphansShowSubgraphs, "subgraphs", "s", false, "Show number of disconnected subgraphs")
	orphansCmd.Flags().BoolVarP(&orphansOutputJSON, "json", "j", false, "Output as JSON")
}

func runOrphans() error {
	graphPath := ""
	args := orphansCmd.Flags().Args()
	if len(args) > 0 {
		graphPath = args[0]
	}

	var graph *topology.Graph
	var err error

	if graphPath != "" {
		graph, err = topology.ImportGraphJSON(graphPath)
		if err != nil {
			return fmt.Errorf("error loading graph from %q: %w", graphPath, err)
		}
	} else {
		// Try reading from stdin pipe.
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			data, err := os.ReadFile("/dev/stdin")
			if err != nil {
				return fmt.Errorf("error reading graph from stdin: %w", err)
			}
			// Manually unmarshal so we can work with a temp file or stdin
			graph, err = parseGraphJSON(data)
			if err != nil {
				return fmt.Errorf("error parsing graph from stdin: %w", err)
			}
		} else {
			return fmt.Errorf("usage: graphlayer topology graph orphans [<graph.json>]\n" +
				"  Provide a graph JSON file path or pipe one via stdin")
		}
	}

	result := commands.AnalyzeOrphans(graph)

	if orphansOutputJSON {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(result); err != nil {
			return fmt.Errorf("error encoding JSON output: %w", err)
		}
	} else {
		text := commands.FormatOrphansOutput(result, orphansShowSubgraphs)
		fmt.Print(text)
	}

	return nil
}

// parseGraphJSON unmarshals a topology.Graph from raw JSON bytes.
func parseGraphJSON(data []byte) (*topology.Graph, error) {
	var graph topology.Graph
	if err := json.Unmarshal(data, &graph); err != nil {
		return nil, fmt.Errorf("invalid graph JSON: %w", err)
	}
	return &graph, nil
}

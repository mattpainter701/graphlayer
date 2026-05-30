package cmd

import (
	"fmt"
	"strings"

	"github.com/mattpainter701/graphlayer/platform/platform/orchestration/orchestration"
	"github.com/spf13/cobra"
)

var topologyCmd = &cobra.Command{
	Use:   "topology",
	Short: "Topology utilities",
}

var (
	detectCycleGraphID  string
	randomWalkGraphID   string
	randomWalkStartNode string
	randomWalkSteps     int
	orphansGraphID      string
	orphansDisconnected bool
)

var detectCycleCmd = &cobra.Command{
	Use:   "detect-cycle",
	Short: "Detect whether a topology graph contains a cycle",
	RunE: func(cmd *cobra.Command, args []string) error {
		hasCycle, cycle, err := orchestration.HasCycle(detectCycleGraphID)
		if err != nil {
			return fmt.Errorf("failed to detect cycle: %w", err)
		}

		if hasCycle {
			fmt.Fprintln(cmd.OutOrStdout(), "Cycle detected: true")
			fmt.Fprintf(cmd.OutOrStdout(), "Cycle nodes: %s\n", strings.Join(cycle, " -> "))
			return nil
		}

		fmt.Fprintln(cmd.OutOrStdout(), "Cycle detected: false")
		return nil
	},
}

var randomWalkCmd = &cobra.Command{
	Use:   "random-walk",
	Short: "Perform a random walk across a topology graph",
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := orchestration.RandomWalk(randomWalkGraphID, randomWalkStartNode, randomWalkSteps)
		if err != nil {
			return fmt.Errorf("failed to perform random walk: %w", err)
		}

		fmt.Fprintln(cmd.OutOrStdout(), strings.Join(path, " -> "))
		return nil
	},
}

var orphansCmd = &cobra.Command{
	Use:   "orphans",
	Short: "Find orphan nodes and disconnected subgraphs",
	Long: `Find all nodes with zero edges (isolated nodes) and count disconnected
subgraphs in a topology graph.

Orphans are nodes that belong to a connected component of size 1 — they have
no edges to any other node.

Example:
  graphlayer topology orphans --graph tree
  graphlayer topology orphans --graph single --disconnected`,
	RunE: func(cmd *cobra.Command, args []string) error {
		result, err := orchestration.Orphans(orphansGraphID)
		if err != nil {
			return fmt.Errorf("failed to find orphans: %w", err)
		}

		if len(result.OrphanNodeIDs) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No orphan nodes found.")
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "Orphan nodes: %s\n", strings.Join(result.OrphanNodeIDs, ", "))
		}

		if orphansDisconnected {
			fmt.Fprintf(cmd.OutOrStdout(), "Disconnected subgraphs: %d\n", result.DisconnectedSubgraphs)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(topologyCmd)
	topologyCmd.AddCommand(detectCycleCmd)
	topologyCmd.AddCommand(randomWalkCmd)
	topologyCmd.AddCommand(orphansCmd)

	detectCycleCmd.Flags().StringVar(&detectCycleGraphID, "graph", "", "Graph ID to analyze")
	_ = detectCycleCmd.MarkFlagRequired("graph")

	randomWalkCmd.Flags().StringVar(&randomWalkGraphID, "graph", "", "Graph ID to walk")
	randomWalkCmd.Flags().StringVar(&randomWalkStartNode, "start", "", "Starting node ID")
	randomWalkCmd.Flags().IntVar(&randomWalkSteps, "steps", 0, "Number of steps to walk")
	_ = randomWalkCmd.MarkFlagRequired("graph")
	_ = randomWalkCmd.MarkFlagRequired("start")
	_ = randomWalkCmd.MarkFlagRequired("steps")

	orphansCmd.Flags().StringVar(&orphansGraphID, "graph", "", "Graph ID to analyze for orphans")
	orphansCmd.Flags().BoolVar(&orphansDisconnected, "disconnected", false, "Show disconnected subgraph count")
	_ = orphansCmd.MarkFlagRequired("graph")
}

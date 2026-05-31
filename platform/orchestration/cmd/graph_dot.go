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
	graphDotInput    string
	graphDotNodeAttr []string
	graphDotEdgeAttr []string
)

var graphDotCmd = &cobra.Command{
	Use:   "dot",
	Short: "Export a topology graph to Graphviz DOT format",
	Long: `Read a topology graph from a JSON file and export it to Graphviz DOT format.

The input JSON must match the TopologyGraph format:
{
  "nodes": [{"id": "...", "attributes": {...}}, ...],
  "edges": [{"source": "...", "target": "...", "attributes": {...}}, ...]
}

Optional attribute flags let you customize node and edge visual properties:
  --node-attr nodeId:key=value    e.g. --node-attr router1:color=red
  --edge-attr src->tgt:key=value  e.g. --edge-attr router1->switch1:style=bold

Multiple --node-attr and --edge-attr flags can be specified.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runGraphDot()
	},
}

func init() {
	graphCmd.AddCommand(graphDotCmd)
	graphDotCmd.Flags().StringVar(&graphDotInput, "input", "", "Path to graph JSON file")
	graphDotCmd.Flags().StringArrayVar(&graphDotNodeAttr, "node-attr", nil, "Node attribute (nodeId:key=value, repeatable)")
	graphDotCmd.Flags().StringArrayVar(&graphDotEdgeAttr, "edge-attr", nil, "Edge attribute (src->tgt:key=value, repeatable)")
	_ = graphDotCmd.MarkFlagRequired("input")
}

func runGraphDot() error {
	data, err := os.ReadFile(graphDotInput)
	if err != nil {
		return fmt.Errorf("failed to read input file: %w", err)
	}

	var g graph.TopologyGraph
	if err := json.Unmarshal(data, &g); err != nil {
		return fmt.Errorf("failed to parse graph JSON: %w", err)
	}

	nodeAttrs := parseAttrFlags(graphDotNodeAttr, func(raw string) (string, string, string) {
		// Format: nodeId:key=value
		colonIdx := strings.Index(raw, ":")
		if colonIdx < 1 {
			return "", "", fmt.Sprintf("invalid node-attr format %q: expected nodeId:key=value", raw)
		}
		nodeID := raw[:colonIdx]
		kv := raw[colonIdx+1:]
		eqIdx := strings.Index(kv, "=")
		if eqIdx < 1 {
			return "", "", fmt.Sprintf("invalid node-attr format %q: expected nodeId:key=value", raw)
		}
		return nodeID, kv[:eqIdx], kv[eqIdx+1:]
	})

	edgeAttrs := parseAttrFlags(graphDotEdgeAttr, func(raw string) (string, string, string) {
		// Format: src->tgt:key=value
		colonIdx := strings.Index(raw, ":")
		if colonIdx < 1 {
			return "", "", fmt.Sprintf("invalid edge-attr format %q: expected src->tgt:key=value", raw)
		}
		edgeKey := raw[:colonIdx]
		kv := raw[colonIdx+1:]
		eqIdx := strings.Index(kv, "=")
		if eqIdx < 1 {
			return "", "", fmt.Sprintf("invalid edge-attr format %q: expected src->tgt:key=value", raw)
		}
		return edgeKey, kv[:eqIdx], kv[eqIdx+1:]
	})

	dot, err := graph.ExportToDOTCustom(&g, nodeAttrs, edgeAttrs)
	if err != nil {
		return fmt.Errorf("dot export failed: %w", err)
	}

	fmt.Fprint(os.Stdout, dot)
	return nil
}

// parseAttrFlags parses repeated --node-attr / --edge-attr flag values into
// the nested map structure expected by ExportToDOTCustom. The extractor
// function should return (key, attrName, attrValue) from a raw flag value.
func parseAttrFlags(flags []string, extractor func(raw string) (string, string, string)) map[string]map[string]string {
	if len(flags) == 0 {
		return nil
	}
	result := make(map[string]map[string]string)
	for _, flag := range flags {
		key, attrName, attrValue := extractor(flag)
		if result[key] == nil {
			result[key] = make(map[string]string)
		}
		result[key][attrName] = attrValue
	}
	return result
}

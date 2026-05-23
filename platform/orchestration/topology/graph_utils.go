package topology

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Graph represents a topology graph with nodes, edges, and export metadata.
type Graph struct {
	Nodes    []Node        `json:"nodes"`
	Edges    []Edge        `json:"edges"`
	Metadata GraphMetadata `json:"metadata"`
}

// Node represents a graph node.
type Node struct {
	ID         string            `json:"id"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

// Edge represents a graph edge.
type Edge struct {
	Source     string            `json:"source"`
	Target     string            `json:"target"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

// GraphMetadata captures export information.
type GraphMetadata struct {
	NodeCount       int    `json:"node_count"`
	EdgeCount       int    `json:"edge_count"`
	ExportTimestamp string `json:"export_timestamp"`
}

// ExportGraphJSON writes a graph to a JSON file with metadata.
func ExportGraphJSON(graph *Graph, outputPath string) error {
	if graph == nil {
		return fmt.Errorf("graph cannot be nil")
	}

	exportGraph := *graph

	if exportGraph.Nodes == nil {
		exportGraph.Nodes = []Node{}
		graph.Nodes = []Node{}
	}
	if exportGraph.Edges == nil {
		exportGraph.Edges = []Edge{}
		graph.Edges = []Edge{}
	}

	exportGraph.Metadata = GraphMetadata{
		NodeCount:       len(exportGraph.Nodes),
		EdgeCount:       len(exportGraph.Edges),
		ExportTimestamp: time.Now().UTC().Format(time.RFC3339Nano),
	}
	graph.Metadata = exportGraph.Metadata

	payload, err := json.MarshalIndent(exportGraph, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal graph json: %w", err)
	}

	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	if err := os.WriteFile(outputPath, payload, 0o644); err != nil {
		return fmt.Errorf("write graph json: %w", err)
	}

	return nil
}

// ImportGraphJSON reads a graph from a JSON file.
func ImportGraphJSON(outputPath string) (*Graph, error) {
	payload, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, fmt.Errorf("read graph json: %w", err)
	}

	var graph Graph
	if err := json.Unmarshal(payload, &graph); err != nil {
		return nil, fmt.Errorf("unmarshal graph json: %w", err)
	}

	return &graph, nil
}

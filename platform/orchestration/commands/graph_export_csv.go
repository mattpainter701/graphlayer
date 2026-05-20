package commands

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// TopologyGraph represents a network topology graph.
type TopologyGraph struct {
	Nodes []*Node
	Edges []*Edge
}

// Node represents a node in the topology graph.
type Node struct {
	ID         string
	Label      string
	Type       string
	Confidence float64
	Tags       []string
}

// Edge represents an edge in the topology graph.
type Edge struct {
	SourceID string
	TargetID string
	Label    string
	Weight   float64
}

// ExportNodesCSV writes the nodes of the topology graph as CSV to w.
// Columns: id, label, type, confidence, tags
func ExportNodesCSV(graph *TopologyGraph, w io.Writer) error {
	if graph == nil {
		return fmt.Errorf("graph is nil")
	}
	writer := csv.NewWriter(w)
	defer writer.Flush()

	// Write header
	if err := writer.Write([]string{"id", "label", "type", "confidence", "tags"}); err != nil {
		return fmt.Errorf("writing header: %w", err)
	}

	for _, node := range graph.Nodes {
		record := []string{
			node.ID,
			node.Label,
			node.Type,
			strconv.FormatFloat(node.Confidence, 'f', 2, 64),
			strings.Join(node.Tags, ","),
		}
		if err := writer.Write(record); err != nil {
			return fmt.Errorf("writing node record: %w", err)
		}
	}
	return nil
}

// ExportEdgesCSV writes the edges of the topology graph as CSV to w.
// Columns: source_id, target_id, label, weight
func ExportEdgesCSV(graph *TopologyGraph, w io.Writer) error {
	if graph == nil {
		return fmt.Errorf("graph is nil")
	}
	writer := csv.NewWriter(w)
	defer writer.Flush()

	// Write header
	if err := writer.Write([]string{"source_id", "target_id", "label", "weight"}); err != nil {
		return fmt.Errorf("writing header: %w", err)
	}

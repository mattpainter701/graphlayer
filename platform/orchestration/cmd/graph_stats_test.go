package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mattpainter701/graphlayer/platform/orchestration/graph"
)

func executeGraphStats(args []string) (*bytes.Buffer, error) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs(args)
	err := rootCmd.Execute()
	return buf, err
}

func TestGraphStatsCommand_SimpleGraph(t *testing.T) {
	// Build a small topology graph: 3 nodes in a line A-B-C
	tg := graph.TopologyGraph{
		Nodes: []graph.GraphNode{
			{ID: "A"},
			{ID: "B"},
			{ID: "C"},
		},
		Edges: []graph.GraphEdge{
			{Source: "A", Target: "B"},
			{Source: "B", Target: "C"},
		},
	}

	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "graph.json")
	data, _ := json.Marshal(tg)
	if err := os.WriteFile(inputPath, data, 0644); err != nil {
		t.Fatalf("failed to write test graph: %v", err)
	}

	output, err := executeGraphStats([]string{"graph", "stats", "--input", inputPath})
	if err != nil {
		t.Fatalf("graph stats command failed: %v", err)
	}

	got := output.String()

	// 3 nodes, 2 edges, avg degree = 2*2/3 = 1.3333, 1 component, density = 2*2/(3*2) = 0.666667
	if !strings.Contains(got, "Node count: 3") {
		t.Errorf("expected 'Node count: 3', got:\n%s", got)
	}
	if !strings.Contains(got, "Edge count: 2") {
		t.Errorf("expected 'Edge count: 2', got:\n%s", got)
	}
	if !strings.Contains(got, "Average degree: 1.3333") {
		t.Errorf("expected 'Average degree: 1.3333', got:\n%s", got)
	}
	if !strings.Contains(got, "Connected components: 1") {
		t.Errorf("expected 'Connected components: 1', got:\n%s", got)
	}
	if !strings.Contains(got, "Graph density: 0.666667") {
		t.Errorf("expected 'Graph density: 0.666667', got:\n%s", got)
	}
}

func TestGraphStatsCommand_TwoComponents(t *testing.T) {
	// Two disconnected components: A-B and C-D
	tg := graph.TopologyGraph{
		Nodes: []graph.GraphNode{
			{ID: "A"},
			{ID: "B"},
			{ID: "C"},
			{ID: "D"},
		},
		Edges: []graph.GraphEdge{
			{Source: "A", Target: "B"},
			{Source: "C", Target: "D"},
		},
	}

	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "graph.json")
	data, _ := json.Marshal(tg)
	if err := os.WriteFile(inputPath, data, 0644); err != nil {
		t.Fatalf("failed to write test graph: %v", err)
	}

	output, err := executeGraphStats([]string{"graph", "stats", "--input", inputPath})
	if err != nil {
		t.Fatalf("graph stats command failed: %v", err)
	}

	got := output.String()

	if !strings.Contains(got, "Node count: 4") {
		t.Errorf("expected 'Node count: 4', got:\n%s", got)
	}
	if !strings.Contains(got, "Edge count: 2") {
		t.Errorf("expected 'Edge count: 2', got:\n%s", got)
	}
	if !strings.Contains(got, "Connected components: 2") {
		t.Errorf("expected 'Connected components: 2', got:\n%s", got)
	}
}

func TestGraphStatsCommand_EmptyGraph(t *testing.T) {
	tg := graph.TopologyGraph{
		Nodes: []graph.GraphNode{},
		Edges: []graph.GraphEdge{},
	}

	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "graph.json")
	data, _ := json.Marshal(tg)
	if err := os.WriteFile(inputPath, data, 0644); err != nil {
		t.Fatalf("failed to write test graph: %v", err)
	}

	output, err := executeGraphStats([]string{"graph", "stats", "--input", inputPath})
	if err != nil {
		t.Fatalf("graph stats command failed: %v", err)
	}

	got := output.String()

	if !strings.Contains(got, "Node count: 0") {
		t.Errorf("expected 'Node count: 0', got:\n%s", got)
	}
	if !strings.Contains(got, "Edge count: 0") {
		t.Errorf("expected 'Edge count: 0', got:\n%s", got)
	}
}

func TestGraphStatsCommand_MissingFlag(t *testing.T) {
	output, err := executeGraphStats([]string{"graph", "stats"})
	if err == nil {
		got := output.String()
		t.Fatalf("expected error for missing --input flag, got nil, output:\n%s", got)
	}
}

func TestGraphStatsCommand_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "bad.json")
	if err := os.WriteFile(inputPath, []byte("not valid json"), 0644); err != nil {
		t.Fatalf("failed to write invalid file: %v", err)
	}

	output, err := executeGraphStats([]string{"graph", "stats", "--input", inputPath})
	if err == nil {
		got := output.String()
		t.Fatalf("expected error for invalid JSON, got nil, output:\n%s", got)
	}
}
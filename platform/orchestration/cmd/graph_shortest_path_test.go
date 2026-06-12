package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/mattpainter701/graphlayer/platform/platform/orchestration/graph"
)

func TestBfsShortestPath_Simple(t *testing.T) {
	g := &graph.TopologyGraph{
		Nodes: map[string]*graph.Node{
			"A": {ID: "A"},
			"B": {ID: "B"},
			"C": {ID: "C"},
			"D": {ID: "D"},
		},
		Edges: map[string]map[string]*graph.Edge{
			"A": {"B": {Source: "A", Target: "B"}},
			"B": {"C": {Source: "B", Target: "C"}},
			"C": {"D": {Source: "C", Target: "D"}},
		},
	}

	path, found := bfsShortestPath(g, "A", "D")
	if !found {
		t.Fatal("expected path to be found")
	}
	if len(path) != 4 {
		t.Fatalf("expected 4 nodes in path, got %d: %v", len(path), path)
	}
	expected := []string{"A", "B", "C", "D"}
	for i, node := range expected {
		if path[i] != node {
			t.Errorf("path[%d] = %q, want %q", i, path[i], node)
		}
	}
}

func TestBfsShortestPath_SameNode(t *testing.T) {
	g := &graph.TopologyGraph{
		Nodes: map[string]*graph.Node{"A": {ID: "A"}},
		Edges: map[string]map[string]*graph.Edge{},
	}

	path, found := bfsShortestPath(g, "A", "A")
	if !found {
		t.Fatal("expected path to be found")
	}
	if len(path) != 1 || path[0] != "A" {
		t.Fatalf("expected [A], got %v", path)
	}
}

func TestBfsShortestPath_NoPath(t *testing.T) {
	g := &graph.TopologyGraph{
		Nodes: map[string]*graph.Node{
			"A": {ID: "A"},
			"B": {ID: "B"},
			"C": {ID: "C"},
		},
		Edges: map[string]map[string]*graph.Edge{
			"A": {"B": {Source: "A", Target: "B"}},
			// C is isolated
		},
	}

	_, found := bfsShortestPath(g, "A", "C")
	if found {
		t.Fatal("expected no path")
	}
}

func TestBfsShortestPath_MultiplePaths(t *testing.T) {
	// Diamond: A->B, A->C, B->D, C->D
	g := &graph.TopologyGraph{
		Nodes: map[string]*graph.Node{
			"A": {ID: "A"},
			"B": {ID: "B"},
			"C": {ID: "C"},
			"D": {ID: "D"},
		},
		Edges: map[string]map[string]*graph.Edge{
			"A": {"B": {Source: "A", Target: "B"}, "C": {Source: "A", Target: "C"}},
			"B": {"D": {Source: "B", Target: "D"}},
			"C": {"D": {Source: "C", Target: "D"}},
		},
	}

	path, found := bfsShortestPath(g, "A", "D")
	if !found {
		t.Fatal("expected path to be found")
	}
	if len(path) != 3 {
		t.Fatalf("expected 3 nodes in shortest path, got %d: %v", len(path), path)
	}
	// First hop must be A
	if path[0] != "A" {
		t.Errorf("path[0] = %q, want A", path[0])
	}
	// Last node must be D
	if path[len(path)-1] != "D" {
		t.Errorf("last = %q, want D", path[len(path)-1])
	}
}

func TestRunShortestPath(t *testing.T) {
	g := graph.TopologyGraph{
		Nodes: map[string]*graph.Node{
			"R1": {ID: "R1"},
			"SW1": {ID: "SW1"},
			"R2": {ID: "R2"},
		},
		Edges: map[string]map[string]*graph.Edge{
			"R1":  {"SW1": {Source: "R1", Target: "SW1"}},
			"SW1": {"R2": {Source: "SW1", Target: "R2"}},
		},
	}

	data, err := json.Marshal(g)
	if err != nil {
		t.Fatal(err)
	}

	tmpFile, err := os.CreateTemp("", "graph-test-*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(data); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	buf := new(bytes.Buffer)
	cmd := graphShortestPathCmd
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--input", tmpFile.Name(), "--source", "R1", "--target", "R2"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("command failed: %v", err)
	}

	output := buf.String()
	if len(output) == 0 {
		t.Fatal("expected output, got none")
	}
	t.Logf("Output: %s", output)
}

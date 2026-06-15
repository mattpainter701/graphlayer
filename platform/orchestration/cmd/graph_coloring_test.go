package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/mattpainter701/graphlayer/platform/platform/orchestration/graph"
)

func TestRunColoring(t *testing.T) {
	g := graph.TopologyGraph{
		Nodes: map[string]*graph.Node{
			"A": {ID: "A"}, "B": {ID: "B"}, "C": {ID: "C"},
		},
		Edges: map[string]map[string]*graph.Edge{
			"A": {"B": {Source: "A", Target: "B"}, "C": {Source: "A", Target: "C"}},
			"B": {"C": {Source: "B", Target: "C"}},
		},
	}
	data, err := json.Marshal(g)
	if err != nil {
		t.Fatal(err)
	}
	tmpFile, err := os.CreateTemp("", "graph-coloring-test-*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	if _, err := tmpFile.Write(data); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()
	buf := new(bytes.Buffer)
	cmd := graphColoringCmd
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--input", tmpFile.Name()})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("command failed: %v", err)
	}
	output := buf.String()
	if len(output) == 0 {
		t.Fatal("expected output, got none")
	}
	t.Logf("Output: %s", output)
}

package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/mattpainter701/graphlayer/platform/orchestration/graph"
)

func TestRunMST(t *testing.T) {
	g := graph.TopologyGraph{
		Nodes: map[string]*graph.Node{
			"A": {ID: "A"}, "B": {ID: "B"}, "C": {ID: "C"},
		},
		Edges: map[string]map[string]*graph.Edge{
			"A": {"B": {Source: "A", Target: "B", Weight: 1}, "C": {Source: "A", Target: "C", Weight: 3}},
			"B": {"C": {Source: "B", Target: "C", Weight: 2}},
		},
	}
	data, err := json.Marshal(g)
	if err != nil {
		t.Fatal(err)
	}
	tmpFile, err := os.CreateTemp("", "graph-mst-test-*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	if _, err := tmpFile.Write(data); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()
	buf := new(bytes.Buffer)
	cmd := graphMSTCmd
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

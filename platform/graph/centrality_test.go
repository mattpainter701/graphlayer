package graph

import (
	"math"
	"testing"
)

func assertCentrality(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("expected centrality %v, got %v", want, got)
	}
}

func TestBetweennessCentrality_LineGraph(t *testing.T) {
	g := &TopologyGraph{
		Nodes: []GraphNode{
			{ID: "n1"},
			{ID: "n2"},
			{ID: "n3"},
		},
		Edges: []GraphEdge{
			{Source: "n1", Target: "n2"},
			{Source: "n2", Target: "n3"},
		},
	}

	got := g.BetweennessCentrality()
	assertCentrality(t, got["n1"], 0)
	assertCentrality(t, got["n2"], 1)
	assertCentrality(t, got["n3"], 0)
}

func TestBetweennessCentrality_StarGraph(t *testing.T) {
	g := &TopologyGraph{
		Nodes: []GraphNode{
			{ID: "center"},
			{ID: "leaf-1"},
			{ID: "leaf-2"},
			{ID: "leaf-3"},
			{ID: "leaf-4"},
		},
		Edges: []GraphEdge{
			{Source: "center", Target: "leaf-1"},
			{Source: "center", Target: "leaf-2"},
			{Source: "center", Target: "leaf-3"},
			{Source: "center", Target: "leaf-4"},
		},
	}

	got := g.BetweennessCentrality()
	assertCentrality(t, got["center"], 6)
	assertCentrality(t, got["leaf-1"], 0)
	assertCentrality(t, got["leaf-2"], 0)
	assertCentrality(t, got["leaf-3"], 0)
	assertCentrality(t, got["leaf-4"], 0)
}

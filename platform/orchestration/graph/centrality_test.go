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

func makeLineGraph(n int) *TopologyGraph {
	nodes := make([]GraphNode, 0, n)
	edges := make([]GraphEdge, 0, n-1)
	for i := 1; i <= n; i++ {
		nodes = append(nodes, GraphNode{ID: "n" + string(rune('0'+i))})
		if i > 1 {
			edges = append(edges, GraphEdge{Source: "n" + string(rune('0'+i-1)), Target: "n" + string(rune('0'+i))})
		}
	}
	return &TopologyGraph{Nodes: nodes, Edges: edges}
}

func TestBetweennessCentrality_LineGraph(t *testing.T) {
	g := &TopologyGraph{
		Nodes: []GraphNode{
			{ID: "n1"},
			{ID: "n2"},
			{ID: "n3"},
			{ID: "n4"},
			{ID: "n5"},
		},
		Edges: []GraphEdge{
			{Source: "n1", Target: "n2"},
			{Source: "n2", Target: "n3"},
			{Source: "n3", Target: "n4"},
			{Source: "n4", Target: "n5"},
		},
	}

	got := g.BetweennessCentrality()
	assertCentrality(t, got["n1"], 0)
	assertCentrality(t, got["n2"], 3)
	assertCentrality(t, got["n3"], 4)
	assertCentrality(t, got["n4"], 3)
	assertCentrality(t, got["n5"], 0)
}

func TestApproximateBetweennessCentrality_MatchesExactWithFullSample(t *testing.T) {
	g := &TopologyGraph{
		Nodes: []GraphNode{
			{ID: "n1"},
			{ID: "n2"},
			{ID: "n3"},
			{ID: "n4"},
			{ID: "n5"},
		},
		Edges: []GraphEdge{
			{Source: "n1", Target: "n2"},
			{Source: "n2", Target: "n3"},
			{Source: "n3", Target: "n4"},
			{Source: "n4", Target: "n5"},
		},
	}

	exact := g.BetweennessCentrality()
	approx, err := ApproximateBetweennessCentrality(g, len(g.Nodes))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, nodeID := range []string{"n1", "n2", "n3", "n4", "n5"} {
		assertCentrality(t, approx[nodeID], exact[nodeID])
	}
}

func TestApproximateBetweennessCentrality_SampledLineGraph(t *testing.T) {
	g := &TopologyGraph{
		Nodes: []GraphNode{
			{ID: "n1"},
			{ID: "n2"},
			{ID: "n3"},
			{ID: "n4"},
			{ID: "n5"},
		},
		Edges: []GraphEdge{
			{Source: "n1", Target: "n2"},
			{Source: "n2", Target: "n3"},
			{Source: "n3", Target: "n4"},
			{Source: "n4", Target: "n5"},
		},
	}

	exact := g.BetweennessCentrality()
	approx, err := ApproximateBetweennessCentrality(g, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, nodeID := range []string{"n1", "n2", "n3", "n4", "n5"} {
		if math.Abs(approx[nodeID]-exact[nodeID]) > 2.5 {
			t.Fatalf("expected approximate centrality for %s to be close to exact: exact=%v approx=%v", nodeID, exact[nodeID], approx[nodeID])
		}
	}
	if approx["n3"] < approx["n2"] || approx["n2"] < approx["n1"] {
		t.Fatalf("expected centrality to increase toward the middle: %#v", approx)
	}
}

func TestApproximateBetweennessCentrality_InvalidSampleSize(t *testing.T) {
	if _, err := ApproximateBetweennessCentrality(&TopologyGraph{}, 0); err == nil {
		t.Fatal("expected error for non-positive sample size")
	}
}

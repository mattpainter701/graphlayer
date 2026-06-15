package graph

import (
	"testing"
)

func TestMinimumSpanningTree_LinearGraph(t *testing.T) {
	g := &TopologyGraph{
		Nodes: map[string]*Node{
			"A": {ID: "A"}, "B": {ID: "B"}, "C": {ID: "C"}, "D": {ID: "D"},
		},
		Edges: map[string]map[string]*Edge{
			"A": {"B": {Source: "A", Target: "B", Weight: 1}},
			"B": {"C": {Source: "B", Target: "C", Weight: 2}},
			"C": {"D": {Source: "C", Target: "D", Weight: 3}},
		},
	}
	edges, total, err := MinimumSpanningTree(g)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(edges) != 3 {
		t.Fatalf("expected 3 MST edges, got %d", len(edges))
	}
	if total != 6.0 {
		t.Errorf("expected total weight 6.0, got %f", total)
	}
}

func TestMinimumSpanningTree_ChoosesMinimum(t *testing.T) {
	g := &TopologyGraph{
		Nodes: map[string]*Node{
			"A": {ID: "A"}, "B": {ID: "B"}, "C": {ID: "C"},
		},
		Edges: map[string]map[string]*Edge{
			"A": {"B": {Source: "A", Target: "B", Weight: 1}, "C": {Source: "A", Target: "C", Weight: 2}},
			"B": {"C": {Source: "B", Target: "C", Weight: 3}},
		},
	}
	edges, total, err := MinimumSpanningTree(g)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(edges) != 2 {
		t.Fatalf("expected 2 MST edges, got %d", len(edges))
	}
	if total != 3.0 {
		t.Errorf("expected total weight 3.0, got %f", total)
	}
}

func TestMinimumSpanningTree_Disconnected(t *testing.T) {
	g := &TopologyGraph{
		Nodes: map[string]*Node{
			"A": {ID: "A"}, "B": {ID: "B"}, "C": {ID: "C"},
		},
		Edges: map[string]map[string]*Edge{
			"A": {"B": {Source: "A", Target: "B", Weight: 1}},
		},
	}
	_, _, err := MinimumSpanningTree(g)
	if err == nil {
		t.Fatal("expected error for disconnected graph")
	}
}

func TestMinimumSpanningTree_Empty(t *testing.T) {
	_, _, err := MinimumSpanningTree(nil)
	if err == nil {
		t.Fatal("expected error for nil graph")
	}
}

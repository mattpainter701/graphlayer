package graph

import (
	"fmt"
	"math"
	"testing"
)

// makeLineGraph creates a simple line of n nodes: n0 - n1 - n2 - ... - n{n-1}
func makeLineGraph(n int) *TopologyGraph {
	nodes := make(map[string]*Node, n)
	edges := make(map[string]map[string]*Edge, n)
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("n%d", i)
		nodes[id] = &Node{ID: id}
	}
	for i := 0; i < n-1; i++ {
		src := fmt.Sprintf("n%d", i)
		tgt := fmt.Sprintf("n%d", i+1)
		if edges[src] == nil {
			edges[src] = make(map[string]*Edge)
		}
		if edges[tgt] == nil {
			edges[tgt] = make(map[string]*Edge)
		}
		edges[src][tgt] = &Edge{Source: src, Target: tgt, Weight: 1}
		edges[tgt][src] = &Edge{Source: tgt, Target: src, Weight: 1}
	}
	return &TopologyGraph{Nodes: nodes, Edges: edges}
}

func TestBetweennessCentrality_Line(t *testing.T) {
	g := makeLineGraph(4)
	// n0 — n1 — n2 — n3
	// Expected betweenness: n1 > n2, n0 = n3 = 0
	cb := BetweennessCentrality(g)
	if cb["n0"] != 0 || cb["n3"] != 0 {
		t.Errorf("expected endpoints to have 0 betweenness, got n0=%f n3=%f", cb["n0"], cb["n3"])
	}
	if cb["n1"] <= 0 || cb["n2"] <= 0 {
		t.Errorf("expected inner nodes to have positive betweenness, got n1=%f n2=%f", cb["n1"], cb["n2"])
	}
	// In a 4-node line, n1 should have exactly 2.0 betweenness (undirected).
	if math.Abs(cb["n1"]-2.0) > 0.001 {
		t.Errorf("expected n1 betweenness ≈ 3.0, got %f", cb["n1"])
	}
}

func TestBetweennessCentrality_Nil(t *testing.T) {
	cb := BetweennessCentrality(nil)
	if cb == nil {
		t.Error("expected empty map, got nil")
	}
}

func TestBetweennessCentrality_Empty(t *testing.T) {
	cb := BetweennessCentrality(&TopologyGraph{})
	if cb == nil {
		t.Error("expected empty map, got nil")
	}
}

func TestApproximateBetweennessCentrality_FullSample(t *testing.T) {
	g := makeLineGraph(4)
	// sampleSize >= len(nodes) should produce exact (full) betweenness.
	cb, err := ApproximateBetweennessCentrality(g, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	exact := BetweennessCentrality(g)
	for _, id := range []string{"n0", "n1", "n2", "n3"} {
		if math.Abs(cb[id]-exact[id]) > 0.001 {
			t.Errorf("node %s: approximate %f != exact %f", id, cb[id], exact[id])
		}
	}
}

func TestApproximateBetweennessCentrality_InvalidSampleSize(t *testing.T) {
	g := makeLineGraph(4)
	if _, err := ApproximateBetweennessCentrality(g, 0); err == nil {
		t.Error("expected error for sampleSize=0")
	}
	if _, err := ApproximateBetweennessCentrality(g, -1); err == nil {
		t.Error("expected error for sampleSize=-1")
	}
}

func TestBetweennessCentrality_Star(t *testing.T) {
	// Center C connected to L1, L2, L3, L4.
	g := &TopologyGraph{
		Nodes: map[string]*Node{
			"C": {ID: "C"}, "L1": {ID: "L1"}, "L2": {ID: "L2"}, "L3": {ID: "L3"}, "L4": {ID: "L4"},
		},
		Edges: map[string]map[string]*Edge{
			"C":  {"L1": {Source: "C", Target: "L1", Weight: 1}, "L2": {Source: "C", Target: "L2", Weight: 1}, "L3": {Source: "C", Target: "L3", Weight: 1}, "L4": {Source: "C", Target: "L4", Weight: 1}},
			"L1": {"C": {Source: "L1", Target: "C", Weight: 1}},
			"L2": {"C": {Source: "L2", Target: "C", Weight: 1}},
			"L3": {"C": {Source: "L3", Target: "C", Weight: 1}},
			"L4": {"C": {Source: "L4", Target: "C", Weight: 1}},
		},
	}
	cb := BetweennessCentrality(g)
	if cb["C"] <= 0 {
		t.Errorf("expected center to have positive betweenness, got %f", cb["C"])
	}
	for _, leaf := range []string{"L1", "L2", "L3", "L4"} {
		if cb[leaf] != 0 {
			t.Errorf("expected leaf %s to have 0 betweenness, got %f", leaf, cb[leaf])
		}
	}
}

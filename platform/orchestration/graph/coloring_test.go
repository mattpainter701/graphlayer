package graph

import (
	"testing"
)

func TestGraphColoring_SimpleTriangle(t *testing.T) {
	g := &TopologyGraph{
		Nodes: map[string]*Node{
			"A": {ID: "A"}, "B": {ID: "B"}, "C": {ID: "C"},
		},
		Edges: map[string]map[string]*Edge{
			"A": {"B": {Source: "A", Target: "B"}, "C": {Source: "A", Target: "C"}},
			"B": {"C": {Source: "B", Target: "C"}},
		},
	}
	result, err := GraphColoring(g)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.NumColors < 3 {
		t.Errorf("triangle needs at least 3 colors, got %d", result.NumColors)
	}
	for src, targets := range g.Edges {
		for tgt := range targets {
			if result.Colors[src] == result.Colors[tgt] {
				t.Errorf("nodes %s and %s share color %d", src, tgt, result.Colors[src])
			}
		}
	}
}

func TestGraphColoring_LinearChain(t *testing.T) {
	g := &TopologyGraph{
		Nodes: map[string]*Node{
			"A": {ID: "A"}, "B": {ID: "B"}, "C": {ID: "C"}, "D": {ID: "D"},
		},
		Edges: map[string]map[string]*Edge{
			"A": {"B": {Source: "A", Target: "B"}},
			"B": {"C": {Source: "B", Target: "C"}},
			"C": {"D": {Source: "C", Target: "D"}},
		},
	}
	result, err := GraphColoring(g)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.NumColors > 2 {
		t.Errorf("chain needs at most 2 colors, got %d", result.NumColors)
	}
}

func TestGraphColoring_Star(t *testing.T) {
	g := &TopologyGraph{
		Nodes: map[string]*Node{
			"A": {ID: "A"}, "B": {ID: "B"}, "C": {ID: "C"}, "D": {ID: "D"},
		},
		Edges: map[string]map[string]*Edge{
			"A": {"B": {Source: "A", Target: "B"}, "C": {Source: "A", Target: "C"}, "D": {Source: "A", Target: "D"}},
		},
	}
	result, err := GraphColoring(g)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.NumColors > 2 {
		t.Errorf("star needs at most 2 colors, got %d", result.NumColors)
	}
}

func TestGraphColoring_Empty(t *testing.T) {
	_, err := GraphColoring(nil)
	if err == nil {
		t.Fatal("expected error for nil graph")
	}
}

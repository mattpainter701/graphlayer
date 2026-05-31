package graph

import (
	"reflect"
	"testing"
)

// buildMixedTypeGraph creates a graph with edges of various types.
func buildMixedTypeGraph() *TopologyGraph {
	g := &TopologyGraph{
		Nodes: make(map[string]*Node),
		Edges: make(map[string]map[string]*Edge),
	}
	g.Nodes["A"] = &Node{ID: "A", Name: "A"}
	g.Nodes["B"] = &Node{ID: "B", Name: "B"}
	g.Nodes["C"] = &Node{ID: "C", Name: "C"}
	g.Nodes["D"] = &Node{ID: "D", Name: "D"}
	g.Nodes["E"] = &Node{ID: "E", Name: "E"}

	addTypedEdge(g, "A", "B", "physical")
	addTypedEdge(g, "B", "C", "physical")
	addTypedEdge(g, "C", "D", "logical")
	addTypedEdge(g, "D", "E", "virtual")
	addTypedEdge(g, "E", "A", "logical")

	return g
}

// buildSingleTypeGraph creates a graph where all edges have the same type.
func buildSingleTypeGraph() *TopologyGraph {
	g := &TopologyGraph{
		Nodes: make(map[string]*Node),
		Edges: make(map[string]map[string]*Edge),
	}
	g.Nodes["X"] = &Node{ID: "X", Name: "X"}
	g.Nodes["Y"] = &Node{ID: "Y", Name: "Y"}
	g.Nodes["Z"] = &Node{ID: "Z", Name: "Z"}

	addTypedEdge(g, "X", "Y", "physical")
	addTypedEdge(g, "Y", "Z", "physical")

	return g
}

// buildNoEdgesGraph creates a graph with nodes but no edges.
func buildNoEdgesGraph() *TopologyGraph {
	g := &TopologyGraph{
		Nodes: make(map[string]*Node),
		Edges: make(map[string]map[string]*Edge),
	}
	g.Nodes["A"] = &Node{ID: "A", Name: "A"}
	g.Nodes["B"] = &Node{ID: "B", Name: "B"}
	return g
}

// addTypedEdge adds a directed edge with the given type.
func addTypedEdge(g *TopologyGraph, src, tgt, edgeType string) {
	if g.Edges[src] == nil {
		g.Edges[src] = make(map[string]*Edge)
	}
	g.Edges[src][tgt] = &Edge{Source: src, Target: tgt, Type: edgeType, Weight: 1.0}
}

// --- Mixed type tests ---

func TestEdgeTypeDistribution_MixedTypes(t *testing.T) {
	g := buildMixedTypeGraph()
	dist := EdgeTypeDistribution(g)

	want := map[string]int{
		"physical": 2,
		"logical":  2,
		"virtual":  1,
	}
	if !reflect.DeepEqual(dist, want) {
		t.Errorf("mixed type graph distribution = %v, want %v", dist, want)
	}
}

// --- Single type tests ---

func TestEdgeTypeDistribution_SingleType(t *testing.T) {
	g := buildSingleTypeGraph()
	dist := EdgeTypeDistribution(g)

	want := map[string]int{
		"physical": 2,
	}
	if !reflect.DeepEqual(dist, want) {
		t.Errorf("single type graph distribution = %v, want %v", dist, want)
	}
}

// --- No edges tests ---

func TestEdgeTypeDistribution_NoEdges(t *testing.T) {
	g := buildNoEdgesGraph()
	dist := EdgeTypeDistribution(g)

	if len(dist) != 0 {
		t.Errorf("no-edges graph: expected empty map, got %v", dist)
	}
}

func TestEdgeTypeDistribution_EmptyGraph(t *testing.T) {
	g := &TopologyGraph{
		Nodes: make(map[string]*Node),
		Edges: make(map[string]map[string]*Edge),
	}
	dist := EdgeTypeDistribution(g)

	if len(dist) != 0 {
		t.Errorf("empty graph: expected empty map, got %v", dist)
	}
}

// --- Nil graph ---

func TestEdgeTypeDistribution_NilGraph(t *testing.T) {
	dist := EdgeTypeDistribution(nil)

	if len(dist) != 0 {
		t.Errorf("nil graph: expected empty map, got %v", dist)
	}
}

// --- Empty / nil type strings ---

func TestEdgeTypeDistribution_EmptyTypeString(t *testing.T) {
	g := &TopologyGraph{
		Nodes: make(map[string]*Node),
		Edges: make(map[string]map[string]*Edge),
	}
	g.Nodes["A"] = &Node{ID: "A", Name: "A"}
	g.Nodes["B"] = &Node{ID: "B", Name: "B"}

	addTypedEdge(g, "A", "B", "") // empty type

	dist := EdgeTypeDistribution(g)
	want := map[string]int{"": 1}
	if !reflect.DeepEqual(dist, want) {
		t.Errorf("empty type string: expected %v, got %v", want, dist)
	}
}

func TestEdgeTypeDistribution_MixedEmptyAndNonEmpty(t *testing.T) {
	g := &TopologyGraph{
		Nodes: make(map[string]*Node),
		Edges: make(map[string]map[string]*Edge),
	}
	g.Nodes["A"] = &Node{ID: "A", Name: "A"}
	g.Nodes["B"] = &Node{ID: "B", Name: "B"}
	g.Nodes["C"] = &Node{ID: "C", Name: "C"}

	addTypedEdge(g, "A", "B", "physical")
	addTypedEdge(g, "B", "C", "") // empty type — simulate missing type info

	dist := EdgeTypeDistribution(g)
	want := map[string]int{
		"physical": 1,
		"":         1,
	}
	if !reflect.DeepEqual(dist, want) {
		t.Errorf("mixed empty and non-empty: expected %v, got %v", want, dist)
	}
}

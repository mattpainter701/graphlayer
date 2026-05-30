package graph

import (
	"testing"
)

// --- Test helpers ---

// buildPath3 creates a directed 3-node path: A -> B -> C
func buildPath3() *TopologyGraph {
	g := &TopologyGraph{
		Nodes: make(map[string]*Node),
		Edges: make(map[string]map[string]*Edge),
	}
	g.Nodes["A"] = &Node{ID: "A", Name: "A"}
	g.Nodes["B"] = &Node{ID: "B", Name: "B"}
	g.Nodes["C"] = &Node{ID: "C", Name: "C"}
	g.Edges["A"] = map[string]*Edge{"B": {Source: "A", Target: "B", Weight: 1.0}}
	g.Edges["B"] = map[string]*Edge{"C": {Source: "B", Target: "C", Weight: 1.0}}
	return g
}

// buildPath3Renamed creates a structurally identical 3-node path with
// different node IDs: X -> Y -> Z
func buildPath3Renamed() *TopologyGraph {
	g := &TopologyGraph{
		Nodes: make(map[string]*Node),
		Edges: make(map[string]map[string]*Edge),
	}
	g.Nodes["X"] = &Node{ID: "X", Name: "X"}
	g.Nodes["Y"] = &Node{ID: "Y", Name: "Y"}
	g.Nodes["Z"] = &Node{ID: "Z", Name: "Z"}
	g.Edges["X"] = map[string]*Edge{"Y": {Source: "X", Target: "Y", Weight: 1.0}}
	g.Edges["Y"] = map[string]*Edge{"Z": {Source: "Y", Target: "Z", Weight: 1.0}}
	return g
}

// buildStar3 creates a directed 3-node star: C -> L1, C -> L2
func buildStar3() *TopologyGraph {
	g := &TopologyGraph{
		Nodes: make(map[string]*Node),
		Edges: make(map[string]map[string]*Edge),
	}
	g.Nodes["C"] = &Node{ID: "C", Name: "C"}
	g.Nodes["L1"] = &Node{ID: "L1", Name: "L1"}
	g.Nodes["L2"] = &Node{ID: "L2", Name: "L2"}
	g.Edges["C"] = map[string]*Edge{
		"L1": {Source: "C", Target: "L1", Weight: 1.0},
		"L2": {Source: "C", Target: "L2", Weight: 1.0},
	}
	return g
}

// --- Edge cases ---

func TestAreIsomorphic_BothNil(t *testing.T) {
	ok, err := AreIsomorphic(nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("two nil graphs should be isomorphic")
	}
}

func TestAreIsomorphic_OneNil(t *testing.T) {
	g := &TopologyGraph{
		Nodes: make(map[string]*Node),
		Edges: make(map[string]map[string]*Edge),
	}
	g.Nodes["A"] = &Node{ID: "A", Name: "A"}

	ok, err := AreIsomorphic(nil, g)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("nil and non-nil should not be isomorphic")
	}

	ok, err = AreIsomorphic(g, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("non-nil and nil should not be isomorphic")
	}
}

func TestAreIsomorphic_TwoEmptyGraphs(t *testing.T) {
	a := &TopologyGraph{
		Nodes: make(map[string]*Node),
		Edges: make(map[string]map[string]*Edge),
	}
	b := &TopologyGraph{
		Nodes: make(map[string]*Node),
		Edges: make(map[string]map[string]*Edge),
	}
	ok, err := AreIsomorphic(a, b)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("two empty graphs should be isomorphic")
	}
}

// --- Node count mismatch ---

func TestAreIsomorphic_DifferentNodeCounts(t *testing.T) {
	a := buildPath3() // 3 nodes
	b := buildStar3()
	delete(b.Nodes, "L2")
	delete(b.Edges, "C") // 2 nodes now

	ok, err := AreIsomorphic(a, b)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("graphs with different node counts should not be isomorphic")
	}
}

// --- Same graph (identity) ---

func TestAreIsomorphic_SameGraph(t *testing.T) {
	g := buildPath3()
	ok, err := AreIsomorphic(g, g)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("a graph should be isomorphic to itself")
	}
}

// --- Isomorphic pairs ---

func TestAreIsomorphic_Path3IsomorphicPath3(t *testing.T) {
	a := buildPath3()
	b := buildPath3Renamed()
	ok, err := AreIsomorphic(a, b)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("two 3-node paths should be isomorphic")
	}
}

func TestAreIsomorphic_SingleNodeGraphs(t *testing.T) {
	a := &TopologyGraph{
		Nodes: map[string]*Node{"A": {ID: "A", Name: "A"}},
		Edges: make(map[string]map[string]*Edge),
	}
	b := &TopologyGraph{
		Nodes: map[string]*Node{"X": {ID: "X", Name: "X"}},
		Edges: make(map[string]map[string]*Edge),
	}
	ok, err := AreIsomorphic(a, b)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("two single-node graphs should be isomorphic")
	}
}

func TestAreIsomorphic_TwoNodeGraphs(t *testing.T) {
	// Both single directed edge: A->B and X->Y
	a := &TopologyGraph{
		Nodes: map[string]*Node{"A": {ID: "A"}, "B": {ID: "B"}},
		Edges: map[string]map[string]*Edge{
			"A": {"B": {Source: "A", Target: "B", Weight: 1.0}},
		},
	}
	b := &TopologyGraph{
		Nodes: map[string]*Node{"X": {ID: "X"}, "Y": {ID: "Y"}},
		Edges: map[string]map[string]*Edge{
			"X": {"Y": {Source: "X", Target: "Y", Weight: 1.0}},
		},
	}
	ok, err := AreIsomorphic(a, b)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("two single-edge graphs should be isomorphic")
	}
}

func TestAreIsomorphic_BidirectionalPair(t *testing.T) {
	// A<->B and X<->Y (each has two directed edges)
	a := &TopologyGraph{
		Nodes: map[string]*Node{"A": {ID: "A"}, "B": {ID: "B"}},
		Edges: map[string]map[string]*Edge{
			"A": {"B": {Source: "A", Target: "B", Weight: 1.0}},
			"B": {"A": {Source: "B", Target: "A", Weight: 1.0}},
		},
	}
	b := &TopologyGraph{
		Nodes: map[string]*Node{"X": {ID: "X"}, "Y": {ID: "Y"}},
		Edges: map[string]map[string]*Edge{
			"X": {"Y": {Source: "X", Target: "Y", Weight: 1.0}},
			"Y": {"X": {Source: "Y", Target: "X", Weight: 1.0}},
		},
	}
	ok, err := AreIsomorphic(a, b)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("two bidirectional pairs should be isomorphic")
	}
}

// --- Non-isomorphic pairs ---

func TestAreIsomorphic_Path3NotIsomorphicStar3(t *testing.T) {
	// Both have degree sequence [1,1,2] but different structure.
	// Path:  A -> B -> C  (linear chain)
	// Star:  C -> L1, C -> L2 (center with two leaves)
	a := buildPath3()
	b := buildStar3()
	ok, err := AreIsomorphic(a, b)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("3-node path should not be isomorphic to 3-node star")
	}
}

func TestAreIsomorphic_DifferentDegreeSequence(t *testing.T) {
	// A<->B (bidirectional): degrees [2,2]
	// C->D (one directed edge): degrees [1,1]
	a := &TopologyGraph{
		Nodes: map[string]*Node{"A": {ID: "A"}, "B": {ID: "B"}},
		Edges: map[string]map[string]*Edge{
			"A": {"B": {Source: "A", Target: "B"}},
			"B": {"A": {Source: "B", Target: "A"}},
		},
	}
	b := &TopologyGraph{
		Nodes: map[string]*Node{"C": {ID: "C"}, "D": {ID: "D"}},
		Edges: map[string]map[string]*Edge{
			"C": {"D": {Source: "C", Target: "D"}},
		},
	}
	ok, err := AreIsomorphic(a, b)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("graphs with different degree sequences should not be isomorphic")
	}
}

// --- Large graph error ---

func TestAreIsomorphic_GraphTooLarge(t *testing.T) {
	a := &TopologyGraph{
		Nodes: make(map[string]*Node),
		Edges: make(map[string]map[string]*Edge),
	}
	b := &TopologyGraph{
		Nodes: make(map[string]*Node),
		Edges: make(map[string]map[string]*Edge),
	}
	for i := 0; i < 9; i++ {
		id := string(rune('A' + i))
		a.Nodes[id] = &Node{ID: id, Name: id}
		b.Nodes[id] = &Node{ID: id, Name: id}
	}

	_, err := AreIsomorphic(a, b)
	if err != ErrGraphTooLarge {
		t.Errorf("expected ErrGraphTooLarge for 9-node graph, got: %v", err)
	}
}

// --- 5-node isomorphic permutation ---

func TestAreIsomorphic_FiveNodeIsomorphicChains(t *testing.T) {
	// Five nodes in two different-but-isomorphic chains
	buildChain := func(prefix string) *TopologyGraph {
		ids := []string{prefix + "1", prefix + "2", prefix + "3", prefix + "4", prefix + "5"}
		g := &TopologyGraph{
			Nodes: make(map[string]*Node),
			Edges: make(map[string]map[string]*Edge),
		}
		for _, id := range ids {
			g.Nodes[id] = &Node{ID: id, Name: id}
		}
		for i := 0; i < len(ids)-1; i++ {
			src := ids[i]
			tgt := ids[i+1]
			if g.Edges[src] == nil {
				g.Edges[src] = make(map[string]*Edge)
			}
			g.Edges[src][tgt] = &Edge{Source: src, Target: tgt, Weight: 1.0}
		}
		return g
	}
	a := buildChain("A")
	b := buildChain("B")

	ok, err := AreIsomorphic(a, b)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("two 4-edge chains (5 nodes) should be isomorphic")
	}
}

// --- 4-node cycle vs path ---

func TestAreIsomorphic_Cycle4NotIsomorphicStar4(t *testing.T) {
	// Cycle: A->B->C->D->A (each node degree 2)
	cycle := &TopologyGraph{
		Nodes: make(map[string]*Node),
		Edges: make(map[string]map[string]*Edge),
	}
	for _, id := range []string{"A", "B", "C", "D"} {
		cycle.Nodes[id] = &Node{ID: id, Name: id}
	}
	cycle.Edges["A"] = map[string]*Edge{"B": {Source: "A", Target: "B"}}
	cycle.Edges["B"] = map[string]*Edge{"C": {Source: "B", Target: "C"}}
	cycle.Edges["C"] = map[string]*Edge{"D": {Source: "C", Target: "D"}}
	cycle.Edges["D"] = map[string]*Edge{"A": {Source: "D", Target: "A"}}

	// Star: center->all three leaves
	star := &TopologyGraph{
		Nodes: make(map[string]*Node),
		Edges: make(map[string]map[string]*Edge),
	}
	star.Nodes["C"] = &Node{ID: "C", Name: "C"}
	star.Nodes["L1"] = &Node{ID: "L1", Name: "L1"}
	star.Nodes["L2"] = &Node{ID: "L2", Name: "L2"}
	star.Nodes["L3"] = &Node{ID: "L3", Name: "L3"}
	star.Edges["C"] = map[string]*Edge{
		"L1": {Source: "C", Target: "L1"},
		"L2": {Source: "C", Target: "L2"},
		"L3": {Source: "C", Target: "L3"},
	}

	ok, err := AreIsomorphic(cycle, star)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("4-cycle should not be isomorphic to 4-node star")
	}
}

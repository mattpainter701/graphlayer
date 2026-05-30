package graph

import (
	"reflect"
	"testing"
)

// --- Test helpers -----------------------------------------------------------

// buildK3 returns a 3-node complete graph (triangle) with bidirectional edges.
func buildK3(prefix string) *TopologyGraph {
	nodes := []string{prefix + "1", prefix + "2", prefix + "3"}
	g := &TopologyGraph{
		Nodes: make(map[string]*Node, 3),
		Edges: make(map[string]map[string]*Edge),
	}
	for _, id := range nodes {
		g.Nodes[id] = &Node{ID: id, Name: id}
	}
	// All pairs, both directions -> each pair has 2 directed edges.
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			if i == j {
				continue
			}
			src, tgt := nodes[i], nodes[j]
			if g.Edges[src] == nil {
				g.Edges[src] = make(map[string]*Edge)
			}
			g.Edges[src][tgt] = &Edge{Source: src, Target: tgt, Weight: 1.0}
		}
	}
	return g
}

// buildSingleEdge returns two nodes connected by a single directed edge.
func buildSingleEdge() *TopologyGraph {
	g := &TopologyGraph{
		Nodes: map[string]*Node{
			"A": {ID: "A", Name: "A"},
			"B": {ID: "B", Name: "B"},
		},
		Edges: map[string]map[string]*Edge{
			"A": {"B": {Source: "A", Target: "B", Weight: 1.0}},
		},
	}
	return g
}

// buildBidirectionalEdge returns two nodes connected by edges in both directions.
func buildBidirectionalEdge() *TopologyGraph {
	g := &TopologyGraph{
		Nodes: map[string]*Node{
			"A": {ID: "A", Name: "A"},
			"B": {ID: "B", Name: "B"},
		},
		Edges: map[string]map[string]*Edge{
			"A": {"B": {Source: "A", Target: "B", Weight: 1.0}},
			"B": {"A": {Source: "B", Target: "A", Weight: 1.0}},
		},
	}
	return g
}

// buildTriangle creates a 3-cycle with single directed edges: A→B, B→C, C→A.
func buildTriangle() *TopologyGraph {
	g := &TopologyGraph{
		Nodes: map[string]*Node{
			"A": {ID: "A"},
			"B": {ID: "B"},
			"C": {ID: "C"},
		},
		Edges: map[string]map[string]*Edge{
			"A": {"B": {Source: "A", Target: "B", Weight: 1.0}},
			"B": {"C": {Source: "B", Target: "C", Weight: 1.0}},
			"C": {"A": {Source: "C", Target: "A", Weight: 1.0}},
		},
	}
	return g
}

// --- Edge cases -------------------------------------------------------------

func TestFindBridges_NilGraph(t *testing.T) {
	bridges := FindBridges(nil)
	if len(bridges) != 0 {
		t.Errorf("expected empty result for nil graph, got %v", bridges)
	}
}

func TestFindBridges_EmptyGraph(t *testing.T) {
	g := &TopologyGraph{
		Nodes: make(map[string]*Node),
		Edges: make(map[string]map[string]*Edge),
	}
	bridges := FindBridges(g)
	if len(bridges) != 0 {
		t.Errorf("expected empty result for empty graph, got %v", bridges)
	}
}

func TestFindBridges_SingleNode(t *testing.T) {
	g := &TopologyGraph{
		Nodes: map[string]*Node{"A": {ID: "A"}},
		Edges: make(map[string]map[string]*Edge),
	}
	bridges := FindBridges(g)
	if len(bridges) != 0 {
		t.Errorf("expected empty result for single node, got %v", bridges)
	}
}

func TestFindBridges_TwoIsolatedNodes(t *testing.T) {
	g := &TopologyGraph{
		Nodes: map[string]*Node{
			"A": {ID: "A"},
			"B": {ID: "B"},
		},
		Edges: make(map[string]map[string]*Edge),
	}
	bridges := FindBridges(g)
	if len(bridges) != 0 {
		t.Errorf("expected empty result for isolated nodes, got %v", bridges)
	}
}

// --- Single-edge graphs -----------------------------------------------------

func TestFindBridges_SingleDirectedEdge(t *testing.T) {
	g := buildSingleEdge() // A--B with one directed edge
	bridges := FindBridges(g)
	if len(bridges) != 1 {
		t.Fatalf("expected 1 bridge, got %d: %v", len(bridges), bridges)
	}
	if bridges[0] != [2]string{"A", "B"} && bridges[0] != [2]string{"B", "A"} {
		t.Errorf("expected bridge between A and B, got %v", bridges[0])
	}
}

func TestFindBridges_BidirectionalEdge_NoBridge(t *testing.T) {
	g := buildBidirectionalEdge() // A↔B, two directed edges
	bridges := FindBridges(g)
	if len(bridges) != 0 {
		t.Errorf("expected no bridges for bidirectional pair, got %v", bridges)
	}
}

// --- Cycle (no bridges) -----------------------------------------------------

func TestFindBridges_Triangle_NoBridges(t *testing.T) {
	g := buildTriangle() // A→B→C→A
	bridges := FindBridges(g)
	if len(bridges) != 0 {
		t.Errorf("expected no bridges in a triangle cycle, got %v", bridges)
	}
}

func TestFindBridges_FullBidirectionalTriangle_NoBridges(t *testing.T) {
	g := buildK3("") // all 6 directed edges → each pair has 2
	bridges := FindBridges(g)
	if len(bridges) != 0 {
		t.Errorf("expected no bridges in a fully-connected K3, got %v", bridges)
	}
}

// --- Graph with a known bridge ----------------------------------------------
//
// Two K3s (complete graphs) connected by a single directed edge C1→D1.
//
//   K3a: A1↔A2↔A3↔A1 (fully connected, no bridges)
//   K3b: B1↔B2↔B3↔B1 (fully connected, no bridges)
//   Bridge: C1→D1 (single directed edge connecting the two components)
//
// Verbatim from the task spec: "two complete graphs connected by a single edge".

func TestFindBridges_TwoK3sConnectedBySingleEdge(t *testing.T) {
	a := buildK3("A") // A1, A2, A3
	b := buildK3("B") // B1, B2, B3

	// Merge nodes.
	for id, node := range b.Nodes {
		a.Nodes[id] = node
	}
	// Merge edges.
	for src, targets := range b.Edges {
		if a.Edges[src] == nil {
			a.Edges[src] = make(map[string]*Edge)
		}
		for tgt, edge := range targets {
			a.Edges[src][tgt] = edge
		}
	}
	// Add a single directed edge connecting the two K3s.
	a.Edges["A1"] = a.Edges["A1"]  // ensure exists
	if a.Edges["A1"] == nil {
		a.Edges["A1"] = make(map[string]*Edge)
	}
	a.Edges["A1"]["B1"] = &Edge{Source: "A1", Target: "B1", Weight: 1.0}

	bridges := FindBridges(a)

	if len(bridges) != 1 {
		t.Fatalf("expected exactly 1 bridge (the connection between the two K3s), got %d: %v", len(bridges), bridges)
	}
	if bridges[0] != [2]string{"A1", "B1"} {
		t.Errorf("expected bridge between A1 and B1, got %v", bridges[0])
	}
}

// --- Line graph -------------------------------------------------------------
//
// A--B--C--D  (all single directed edges)
// Bridges: A-B, B-C, C-D (every edge is a bridge in a line)

func TestFindBridges_LineGraph(t *testing.T) {
	g := &TopologyGraph{
		Nodes: map[string]*Node{
			"A": {ID: "A"}, "B": {ID: "B"},
			"C": {ID: "C"}, "D": {ID: "D"},
		},
		Edges: map[string]map[string]*Edge{
			"A": {"B": {Source: "A", Target: "B", Weight: 1.0}},
			"B": {"C": {Source: "B", Target: "C", Weight: 1.0}},
			"C": {"D": {Source: "C", Target: "D", Weight: 1.0}},
		},
	}

	bridges := FindBridges(g)
	if len(bridges) != 3 {
		t.Fatalf("expected 3 bridges in a 4-node line, got %d: %v", len(bridges), bridges)
	}

	expected := [][2]string{{"A", "B"}, {"B", "C"}, {"C", "D"}}
	if !reflect.DeepEqual(bridges, expected) {
		t.Errorf("expected %v, got %v", expected, bridges)
	}
}

// --- Mixed: a diamond-shaped graph with one bridge --------------------------
//
//   A
//  / \
// B---C
// |
// D
//
// Edges: A→B, A→C, B→C, C→B, B→D
// The diamond A-B-C is 2-edge-connected (A-B, A-C, B-C form a cycle).
// The edge B-D is a bridge.

func TestFindBridges_DiamondWithBridge(t *testing.T) {
	g := &TopologyGraph{
		Nodes: map[string]*Node{
			"A": {ID: "A"}, "B": {ID: "B"},
			"C": {ID: "C"}, "D": {ID: "D"},
		},
		Edges: map[string]map[string]*Edge{
			"A": {"B": {Source: "A", Target: "B", Weight: 1.0},
				"C": {Source: "A", Target: "C", Weight: 1.0}},
			"B": {"C": {Source: "B", Target: "C", Weight: 1.0},
				"D": {Source: "B", Target: "D", Weight: 1.0}},
			"C": {"B": {Source: "C", Target: "B", Weight: 1.0}},
		},
	}

	bridges := FindBridges(g)
	if len(bridges) != 1 {
		t.Fatalf("expected 1 bridge (B-D), got %d: %v", len(bridges), bridges)
	}
	if bridges[0] != [2]string{"B", "D"} {
		t.Errorf("expected bridge between B and D, got %v", bridges[0])
	}
}

// --- Disconnected components ------------------------------------------------
//
// Two separate lines:
//   A--B   C--D
//
// All four edges are bridges (each component is a line).

func TestFindBridges_TwoDisconnectedLines(t *testing.T) {
	g := &TopologyGraph{
		Nodes: map[string]*Node{
			"A": {ID: "A"}, "B": {ID: "B"},
			"C": {ID: "C"}, "D": {ID: "D"},
		},
		Edges: map[string]map[string]*Edge{
			"A": {"B": {Source: "A", Target: "B", Weight: 1.0}},
			"C": {"D": {Source: "C", Target: "D", Weight: 1.0}},
		},
	}

	bridges := FindBridges(g)
	if len(bridges) != 2 {
		t.Fatalf("expected 2 bridges (one per line), got %d: %v", len(bridges), bridges)
	}
}

// --- Self-loop should be ignored --------------------------------------------

func TestFindBridges_SelfLoopOnly(t *testing.T) {
	g := &TopologyGraph{
		Nodes: map[string]*Node{"A": {ID: "A"}},
		Edges: map[string]map[string]*Edge{
			"A": {"A": {Source: "A", Target: "A", Weight: 1.0}},
		},
	}
	bridges := FindBridges(g)
	if len(bridges) != 0 {
		t.Errorf("expected no bridges for self-loop only, got %v", bridges)
	}
}

func TestFindBridges_SelfLoopWithBridge(t *testing.T) {
	g := &TopologyGraph{
		Nodes: map[string]*Node{"A": {ID: "A"}, "B": {ID: "B"}},
		Edges: map[string]map[string]*Edge{
			"A": {"A": {Source: "A", Target: "A", Weight: 1.0},
				"B": {Source: "A", Target: "B", Weight: 1.0}},
		},
	}
	bridges := FindBridges(g)
	if len(bridges) != 1 {
		t.Fatalf("expected 1 bridge (A-B), got %d: %v", len(bridges), bridges)
	}
	if bridges[0] != [2]string{"A", "B"} {
		t.Errorf("expected bridge between A and B, got %v", bridges[0])
	}
}

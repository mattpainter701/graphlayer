package graph

import (
	"testing"
)

// buildBowTieGraph creates a bow-tie shape:
//
//   A — B — C
//       |
//       D — E
//
// B is the central articulation point connecting the two triangles
// (A-B-C-A and B-D-E-B). Removing B splits the graph into two components.
func buildBowTieGraph() *TopologyGraph {
	return &TopologyGraph{
		Nodes: map[string]*Node{
			"A": {ID: "A"},
			"B": {ID: "B"},
			"C": {ID: "C"},
			"D": {ID: "D"},
			"E": {ID: "E"},
		},
		Edges: map[string]map[string]*Edge{
			"A": {"B": {Source: "A", Target: "B", Weight: 1}, "C": {Source: "A", Target: "C", Weight: 1}},
			"B": {"A": {Source: "B", Target: "A", Weight: 1}, "C": {Source: "B", Target: "C", Weight: 1}, "D": {Source: "B", Target: "D", Weight: 1}, "E": {Source: "B", Target: "E", Weight: 1}},
			"C": {"A": {Source: "C", Target: "A", Weight: 1}, "B": {Source: "C", Target: "B", Weight: 1}},
			"D": {"B": {Source: "D", Target: "B", Weight: 1}, "E": {Source: "D", Target: "E", Weight: 1}},
			"E": {"B": {Source: "E", Target: "B", Weight: 1}, "D": {Source: "E", Target: "D", Weight: 1}},
		},
	}
}

// buildCycleGraph creates a simple cycle: A - B - C - A
func buildCycleGraph() *TopologyGraph {
	return &TopologyGraph{
		Nodes: map[string]*Node{
			"A": {ID: "A"},
			"B": {ID: "B"},
			"C": {ID: "C"},
		},
		Edges: map[string]map[string]*Edge{
			"A": {"B": {Source: "A", Target: "B", Weight: 1}, "C": {Source: "A", Target: "C", Weight: 1}},
			"B": {"A": {Source: "B", Target: "A", Weight: 1}, "C": {Source: "B", Target: "C", Weight: 1}},
			"C": {"A": {Source: "C", Target: "A", Weight: 1}, "B": {Source: "C", Target: "B", Weight: 1}},
		},
	}
}

func TestArticulationPoints_BowTie(t *testing.T) {
	g := buildBowTieGraph()
	points := ArticulationPoints(g)
	if len(points) != 1 {
		t.Fatalf("expected 1 articulation point (B), got %d: %v", len(points), points)
	}
	// B should be the articulation point.
	found := false
	for _, id := range points {
		if id == "B" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected articulation point B, got %v", points)
	}
}

func TestArticulationPoints_SimpleCycle(t *testing.T) {
	g := buildCycleGraph()
	points := ArticulationPoints(g)
	if points != nil {
		// A simple cycle has no articulation points — any single node
		// removal leaves the rest connected (the cycle breaks to a path).
		t.Errorf("expected no articulation points for a cycle, got %v", points)
	}
}

func TestArticulationPoints_NilGraph(t *testing.T) {
	points := ArticulationPoints(nil)
	if points != nil {
		t.Errorf("expected nil for nil graph, got %v", points)
	}
}

func TestArticulationPoints_EmptyGraph(t *testing.T) {
	g := &TopologyGraph{}
	points := ArticulationPoints(g)
	if points != nil {
		t.Errorf("expected nil for empty graph, got %v", points)
	}
}

func TestArticulationPoints_SingleNode(t *testing.T) {
	g := &TopologyGraph{
		Nodes: map[string]*Node{"A": {ID: "A"}},
	}
	points := ArticulationPoints(g)
	if points != nil {
		t.Errorf("expected nil for single node (no edges), got %v", points)
	}
}

func TestArticulationPoints_TwoConnectedNodes(t *testing.T) {
	g := &TopologyGraph{
		Nodes: map[string]*Node{"A": {ID: "A"}, "B": {ID: "B"}},
		Edges: map[string]map[string]*Edge{
			"A": {"B": {Source: "A", Target: "B", Weight: 1}},
			"B": {"A": {Source: "B", Target: "A", Weight: 1}},
		},
	}
	points := ArticulationPoints(g)
	if points != nil {
		t.Errorf("expected no articulation points for 2-node edge, got %v", points)
	}
}

func TestArticulationPoints_LineGraph(t *testing.T) {
	// A — B — C — D
	// B and C are articulation points.
	g := &TopologyGraph{
		Nodes: map[string]*Node{
			"A": {ID: "A"}, "B": {ID: "B"}, "C": {ID: "C"}, "D": {ID: "D"},
		},
		Edges: map[string]map[string]*Edge{
			"A": {"B": {Source: "A", Target: "B", Weight: 1}},
			"B": {"A": {Source: "B", Target: "A", Weight: 1}, "C": {Source: "B", Target: "C", Weight: 1}},
			"C": {"B": {Source: "C", Target: "B", Weight: 1}, "D": {Source: "C", Target: "D", Weight: 1}},
			"D": {"C": {Source: "D", Target: "C", Weight: 1}},
		},
	}
	points := ArticulationPoints(g)
	if len(points) != 2 {
		t.Fatalf("expected 2 articulation points (B and C), got %d: %v", len(points), points)
	}
	bFound, cFound := false, false
	for _, id := range points {
		if id == "B" {
			bFound = true
		}
		if id == "C" {
			cFound = true
		}
	}
	if !bFound {
		t.Errorf("expected B as articulation point, got %v", points)
	}
	if !cFound {
		t.Errorf("expected C as articulation point, got %v", points)
	}
}

func TestArticulationPoints_StarGraph(t *testing.T) {
	// Center "C" connected to leaves L1, L2, L3, L4.
	// Center is an articulation point; leaves are not.
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
	points := ArticulationPoints(g)
	if len(points) != 1 || points[0] != "C" {
		t.Errorf("expected articulation point [C], got %v", points)
	}
}

func TestArticulationPoints_DisconnectedWithArticulation(t *testing.T) {
	// Component 1: A — B — C — D  (B and C are articulation points)
	// Component 2: E — F  (neither is articulation point)
	g := &TopologyGraph{
		Nodes: map[string]*Node{
			"A": {ID: "A"}, "B": {ID: "B"}, "C": {ID: "C"}, "D": {ID: "D"},
			"E": {ID: "E"}, "F": {ID: "F"},
		},
		Edges: map[string]map[string]*Edge{
			"A": {"B": {Source: "A", Target: "B", Weight: 1}},
			"B": {"A": {Source: "B", Target: "A", Weight: 1}, "C": {Source: "B", Target: "C", Weight: 1}},
			"C": {"B": {Source: "C", Target: "B", Weight: 1}, "D": {Source: "C", Target: "D", Weight: 1}},
			"D": {"C": {Source: "D", Target: "C", Weight: 1}},
			"E": {"F": {Source: "E", Target: "F", Weight: 1}},
			"F": {"E": {Source: "F", Target: "E", Weight: 1}},
		},
	}
	points := ArticulationPoints(g)
	if len(points) != 2 {
		t.Fatalf("expected 2 articulation points (B and C), got %d: %v", len(points), points)
	}
	bFound, cFound := false, false
	for _, id := range points {
		if id == "B" {
			bFound = true
		}
		if id == "C" {
			cFound = true
		}
	}
	if !bFound {
		t.Errorf("expected B as articulation point, got %v", points)
	}
	if !cFound {
		t.Errorf("expected C as articulation point, got %v", points)
	}
}

func TestArticulationPoints_IsolatedNodesOnly(t *testing.T) {
	g := &TopologyGraph{
		Nodes: map[string]*Node{
			"X": {ID: "X"}, "Y": {ID: "Y"}, "Z": {ID: "Z"},
		},
	}
	points := ArticulationPoints(g)
	if points != nil {
		t.Errorf("expected nil for isolated nodes (no edges), got %v", points)
	}
}

func TestArticulationPoints_SelfLoopOnly(t *testing.T) {
	g := &TopologyGraph{
		Nodes: map[string]*Node{"A": {ID: "A"}},
		Edges: map[string]map[string]*Edge{
			"A": {"A": {Source: "A", Target: "A", Weight: 1}},
		},
	}
	points := ArticulationPoints(g)
	if points != nil {
		t.Errorf("expected nil for self-loop-only graph, got %v", points)
	}
}

func TestArticulationPoints_BidirectionalEdge(t *testing.T) {
	g := &TopologyGraph{
		Nodes: map[string]*Node{"A": {ID: "A"}, "B": {ID: "B"}},
		Edges: map[string]map[string]*Edge{
			"A": {"B": {Source: "A", Target: "B", Weight: 1}},
			"B": {"A": {Source: "B", Target: "A", Weight: 1}},
		},
	}
	points := ArticulationPoints(g)
	if points != nil {
		t.Errorf("expected no articulation points for bidirectional edge, got %v", points)
	}
}

func TestArticulationPoints_TwoTrianglesConnectedByEdge(t *testing.T) {
	// Triangle 1: A-B-C with all connections
	// Triangle 2: D-E-F with all connections
	// Connected by: C — D
	// C and D are articulation points.
	g := &TopologyGraph{
		Nodes: map[string]*Node{
			"A": {ID: "A"}, "B": {ID: "B"}, "C": {ID: "C"},
			"D": {ID: "D"}, "E": {ID: "E"}, "F": {ID: "F"},
		},
		Edges: map[string]map[string]*Edge{
			"A": {"B": {Source: "A", Target: "B", Weight: 1}, "C": {Source: "A", Target: "C", Weight: 1}},
			"B": {"A": {Source: "B", Target: "A", Weight: 1}, "C": {Source: "B", Target: "C", Weight: 1}},
			"C": {"A": {Source: "C", Target: "A", Weight: 1}, "B": {Source: "C", Target: "B", Weight: 1}, "D": {Source: "C", Target: "D", Weight: 1}},
			"D": {"C": {Source: "D", Target: "C", Weight: 1}, "E": {Source: "D", Target: "E", Weight: 1}, "F": {Source: "D", Target: "F", Weight: 1}},
			"E": {"D": {Source: "E", Target: "D", Weight: 1}, "F": {Source: "E", Target: "F", Weight: 1}},
			"F": {"D": {Source: "F", Target: "D", Weight: 1}, "E": {Source: "F", Target: "E", Weight: 1}},
		},
	}
	points := ArticulationPoints(g)
	if len(points) != 2 {
		t.Fatalf("expected 2 articulation points (C and D), got %d: %v", len(points), points)
	}
	cFound, dFound := false, false
	for _, id := range points {
		if id == "C" {
			cFound = true
		}
		if id == "D" {
			dFound = true
		}
	}
	if !cFound {
		t.Errorf("expected C as articulation point, got %v", points)
	}
	if !dFound {
		t.Errorf("expected D as articulation point, got %v", points)
	}
}

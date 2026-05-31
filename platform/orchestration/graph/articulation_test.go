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
		Nodes: []GraphNode{
			{ID: "A"}, {ID: "B"}, {ID: "C"}, {ID: "D"}, {ID: "E"},
		},
		Edges: []GraphEdge{
			{Source: "A", Target: "B"},
			{Source: "B", Target: "C"},
			{Source: "C", Target: "A"},
			{Source: "B", Target: "D"},
			{Source: "D", Target: "E"},
			{Source: "E", Target: "B"},
		},
	}
}

// buildCycleGraph creates a simple cycle: A - B - C - A
func buildCycleGraph() *TopologyGraph {
	return &TopologyGraph{
		Nodes: []GraphNode{
			{ID: "A"}, {ID: "B"}, {ID: "C"},
		},
		Edges: []GraphEdge{
			{Source: "A", Target: "B"},
			{Source: "B", Target: "C"},
			{Source: "C", Target: "A"},
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
		Nodes: []GraphNode{{ID: "A"}},
	}
	points := ArticulationPoints(g)
	if points != nil {
		t.Errorf("expected nil for single node (no edges), got %v", points)
	}
}

func TestArticulationPoints_TwoConnectedNodes(t *testing.T) {
	g := &TopologyGraph{
		Nodes: []GraphNode{{ID: "A"}, {ID: "B"}},
		Edges: []GraphEdge{{Source: "A", Target: "B"}},
	}
	points := ArticulationPoints(g)
	// A single edge: removing either node leaves a single isolated node,
	// BUT an articulation point definition says that a node whose removal
	// increases the number of connected components. Removing A from A-B
	// leaves just B (1 component), same as before when the graph had
	// 2 nodes and 1 edge = 1 component. So neither is an articulation point.
	// Actually: The classic Tarjan algorithm does NOT report nodes in a
	// 2-node graph as articulation points because the root (first visited)
	// has only 1 child (so childCount <= 1), and the other node has a parent
	// but no grandchildren, so low[v] >= disc[u] won't trigger.
	if points != nil {
		t.Errorf("expected no articulation points for 2-node edge, got %v", points)
	}
}

func TestArticulationPoints_LineGraph(t *testing.T) {
	// A — B — C — D
	// B and C are articulation points.
	g := &TopologyGraph{
		Nodes: []GraphNode{
			{ID: "A"}, {ID: "B"}, {ID: "C"}, {ID: "D"},
		},
		Edges: []GraphEdge{
			{Source: "A", Target: "B"},
			{Source: "B", Target: "C"},
			{Source: "C", Target: "D"},
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
		Nodes: []GraphNode{
			{ID: "C"}, {ID: "L1"}, {ID: "L2"}, {ID: "L3"}, {ID: "L4"},
		},
		Edges: []GraphEdge{
			{Source: "C", Target: "L1"},
			{Source: "C", Target: "L2"},
			{Source: "C", Target: "L3"},
			{Source: "C", Target: "L4"},
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
		Nodes: []GraphNode{
			{ID: "A"}, {ID: "B"}, {ID: "C"}, {ID: "D"},
			{ID: "E"}, {ID: "F"},
		},
		Edges: []GraphEdge{
			{Source: "A", Target: "B"},
			{Source: "B", Target: "C"},
			{Source: "C", Target: "D"},
			{Source: "E", Target: "F"},
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
		Nodes: []GraphNode{
			{ID: "X"}, {ID: "Y"}, {ID: "Z"},
		},
	}
	points := ArticulationPoints(g)
	if points != nil {
		t.Errorf("expected nil for isolated nodes (no edges), got %v", points)
	}
}

func TestArticulationPoints_SelfLoopOnly(t *testing.T) {
	g := &TopologyGraph{
		Nodes: []GraphNode{{ID: "A"}},
		Edges: []GraphEdge{{Source: "A", Target: "A"}},
	}
	points := ArticulationPoints(g)
	// A self-loop does not connect A to any other node, so A is isolated from
	// the perspective of articulation point analysis.
	if points != nil {
		t.Errorf("expected nil for self-loop-only graph, got %v", points)
	}
}

func TestArticulationPoints_BidirectionalEdge(t *testing.T) {
	// A ↔ B — the bidirectional edges create a 2-node graph that is
	// effectively equivalent to a single undirected edge.
	// Neither node is an articulation point (same reasoning as
	// TestArticulationPoints_TwoConnectedNodes).
	g := &TopologyGraph{
		Nodes: []GraphNode{{ID: "A"}, {ID: "B"}},
		Edges: []GraphEdge{
			{Source: "A", Target: "B"},
			{Source: "B", Target: "A"},
		},
	}
	points := ArticulationPoints(g)
	if points != nil {
		t.Errorf("expected no articulation points for bidirectional edge, got %v", points)
	}
}

func TestArticulationPoints_TwoTrianglesConnectedByEdge(t *testing.T) {
	// A — B — C     D — E — F
	// |         |   |         |
	// +----B----+   +----E----+
	//
	// Triangle 1: A-B-C (with cross edge C-A making it a triangle)
	// Triangle 2: D-E-F (with cross edge F-D making it a triangle)
	// Connected by: C — D
	// C and D are articulation points.
	g := &TopologyGraph{
		Nodes: []GraphNode{
			{ID: "A"}, {ID: "B"}, {ID: "C"},
			{ID: "D"}, {ID: "E"}, {ID: "F"},
		},
		Edges: []GraphEdge{
			{Source: "A", Target: "B"},
			{Source: "B", Target: "C"},
			{Source: "C", Target: "A"},
			{Source: "D", Target: "E"},
			{Source: "E", Target: "F"},
			{Source: "F", Target: "D"},
			{Source: "C", Target: "D"},
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

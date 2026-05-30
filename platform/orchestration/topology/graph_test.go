package topology

import (
	"math"
	"testing"
)

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

func TestClusteringCoefficientTriangle(t *testing.T) {
	graph := &Graph{
		Nodes: []Node{
			{ID: "A"},
			{ID: "B"},
			{ID: "C"},
		},
		Edges: []Edge{
			{Source: "A", Target: "B"},
			{Source: "B", Target: "C"},
			{Source: "C", Target: "A"},
		},
	}

	for _, nodeID := range []string{"A", "B", "C"} {
		if got := ClusteringCoefficient(graph, nodeID); !almostEqual(got, 1.0) {
			t.Fatalf("ClusteringCoefficient(%q) = %v, want 1.0", nodeID, got)
		}
	}

	if got := GlobalClusteringCoefficient(graph); !almostEqual(got, 1.0) {
		t.Fatalf("GlobalClusteringCoefficient(triangle) = %v, want 1.0", got)
	}
}

func TestClusteringCoefficientStarGraph(t *testing.T) {
	graph := &Graph{
		Nodes: []Node{
			{ID: "center"},
			{ID: "leaf-1"},
			{ID: "leaf-2"},
			{ID: "leaf-3"},
			{ID: "leaf-4"},
		},
		Edges: []Edge{
			{Source: "center", Target: "leaf-1"},
			{Source: "center", Target: "leaf-2"},
			{Source: "center", Target: "leaf-3"},
			{Source: "center", Target: "leaf-4"},
		},
	}

	if got := ClusteringCoefficient(graph, "center"); !almostEqual(got, 0.0) {
		t.Fatalf("ClusteringCoefficient(center) = %v, want 0.0", got)
	}

	for _, leafID := range []string{"leaf-1", "leaf-2", "leaf-3", "leaf-4"} {
		if got := ClusteringCoefficient(graph, leafID); !almostEqual(got, 0.0) {
			t.Fatalf("ClusteringCoefficient(%q) = %v, want 0.0", leafID, got)
		}
	}

	if got := GlobalClusteringCoefficient(graph); !almostEqual(got, 0.0) {
		t.Fatalf("GlobalClusteringCoefficient(star) = %v, want 0.0", got)
	}
}

func TestClusteringCoefficientMixedGraph(t *testing.T) {
	graph := &Graph{
		Nodes: []Node{
			{ID: "A"},
			{ID: "B"},
			{ID: "C"},
			{ID: "D"},
		},
		Edges: []Edge{
			{Source: "A", Target: "B"},
			{Source: "B", Target: "C"},
			{Source: "C", Target: "A"},
			{Source: "C", Target: "D"},
		},
	}

	cases := map[string]float64{
		"A": 1.0,
		"B": 1.0,
		"C": 1.0 / 3.0,
		"D": 0.0,
	}

	for nodeID, want := range cases {
		if got := ClusteringCoefficient(graph, nodeID); !almostEqual(got, want) {
			t.Fatalf("ClusteringCoefficient(%q) = %v, want %v", nodeID, got, want)
		}
	}

	if got := GlobalClusteringCoefficient(graph); !almostEqual(got, 7.0/12.0) {
		t.Fatalf("GlobalClusteringCoefficient(mixed) = %v, want %v", got, 7.0/12.0)
	}
}

// --- NodeLevels tests ---

func TestNodeLevelsSimpleTree(t *testing.T) {
	graph := &Graph{
		Nodes: []Node{
			{ID: "root"},
			{ID: "A"},
			{ID: "B"},
			{ID: "C"},
			{ID: "D"},
			{ID: "E"},
		},
		Edges: []Edge{
			{Source: "root", Target: "A"},
			{Source: "root", Target: "B"},
			{Source: "A", Target: "C"},
			{Source: "A", Target: "D"},
			{Source: "B", Target: "E"},
		},
	}

	levels, err := NodeLevels(graph, "root")
	if err != nil {
		t.Fatalf("NodeLevels(root) returned error: %v", err)
	}

	want := map[string]int{
		"root": 0,
		"A":    1,
		"B":    1,
		"C":    2,
		"D":    2,
		"E":    2,
	}

	for nodeID, expectedLevel := range want {
		got, ok := levels[nodeID]
		if !ok {
			t.Fatalf("NodeLevels: missing node %q in result", nodeID)
		}
		if got != expectedLevel {
			t.Fatalf("NodeLevels[%q] = %d, want %d", nodeID, got, expectedLevel)
		}
	}

	if got, want := len(levels), len(want); got != want {
		t.Fatalf("NodeLevels returned %d entries, want %d", got, want)
	}
}

func TestNodeLevelsDisconnectedNodes(t *testing.T) {
	graph := &Graph{
		Nodes: []Node{
			{ID: "X"},
			{ID: "Y"},
			{ID: "Z"},
			{ID: "isolated"},
		},
		Edges: []Edge{
			{Source: "X", Target: "Y"},
			{Source: "Y", Target: "Z"},
		},
	}

	levels, err := NodeLevels(graph, "X")
	if err != nil {
		t.Fatalf("NodeLevels(X) returned error: %v", err)
	}

	if _, ok := levels["isolated"]; ok {
		t.Fatal("NodeLevels: isolated node should not appear in result")
	}

	if levels["X"] != 0 {
		t.Fatalf("NodeLevels[X] = %d, want 0", levels["X"])
	}
	if levels["Y"] != 1 {
		t.Fatalf("NodeLevels[Y] = %d, want 1", levels["Y"])
	}
	if levels["Z"] != 2 {
		t.Fatalf("NodeLevels[Z] = %d, want 2", levels["Z"])
	}

	if got, want := len(levels), 3; got != want {
		t.Fatalf("NodeLevels returned %d entries, want %d (isolated omitted)", got, want)
	}
}

func TestNodeLevelsSingleNode(t *testing.T) {
	graph := &Graph{
		Nodes: []Node{{ID: "solo"}},
		Edges: []Edge{},
	}

	levels, err := NodeLevels(graph, "solo")
	if err != nil {
		t.Fatalf("NodeLevels(solo) returned error: %v", err)
	}

	if got, want := levels["solo"], 0; got != want {
		t.Fatalf("NodeLevels[solo] = %d, want %d", got, want)
	}

	if got, want := len(levels), 1; got != want {
		t.Fatalf("NodeLevels returned %d entries, want %d", got, want)
	}
}

func TestNodeLevelsNonexistentRoot(t *testing.T) {
	graph := &Graph{
		Nodes: []Node{{ID: "A"}, {ID: "B"}},
		Edges: []Edge{{Source: "A", Target: "B"}},
	}

	_, err := NodeLevels(graph, "NONEXISTENT")
	if err == nil {
		t.Fatal("NodeLevels(NONEXISTENT) expected error, got nil")
	}
}

func TestNodeLevelsNilGraph(t *testing.T) {
	_, err := NodeLevels(nil, "root")
	if err == nil {
		t.Fatal("NodeLevels(nil, root) expected error, got nil")
	}
}

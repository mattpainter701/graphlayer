package topology

import "testing"

func TestNeighborCountsSimpleGraph(t *testing.T) {
	graph := &Graph{
		Nodes: []Node{
			{ID: "A"},
			{ID: "B"},
			{ID: "C"},
		},
		Edges: []Edge{
			{Source: "A", Target: "B"},
			{Source: "B", Target: "C"},
		},
	}

	counts := NeighborCounts(graph)

	cases := map[string]int{
		"A": 1,
		"B": 2,
		"C": 1,
	}
	for nodeID, want := range cases {
		if got := counts[nodeID]; got != want {
			t.Errorf("NeighborCounts[%q] = %d, want %d", nodeID, got, want)
		}
	}
}

func TestNeighborCountsIsolatedNodes(t *testing.T) {
	graph := &Graph{
		Nodes: []Node{
			{ID: "isolated"},
			{ID: "lonely"},
		},
		Edges: nil,
	}

	counts := NeighborCounts(graph)

	if counts["isolated"] != 0 {
		t.Errorf("isolated node should have 0 neighbors, got %d", counts["isolated"])
	}
	if counts["lonely"] != 0 {
		t.Errorf("lonely node should have 0 neighbors, got %d", counts["lonely"])
	}
}

func TestNeighborCountsNilGraph(t *testing.T) {
	counts := NeighborCounts(nil)
	if len(counts) != 0 {
		t.Errorf("nil graph should return empty map, got %v", counts)
	}
}

func TestNeighborCountNonExistentNode(t *testing.T) {
	graph := &Graph{
		Nodes: []Node{
			{ID: "A"},
		},
		Edges: nil,
	}

	_, err := NeighborCount(graph, "Z")
	if err == nil {
		t.Error("expected error for non-existent node, got nil")
	}
}

func TestNeighborCountExistingNode(t *testing.T) {
	graph := &Graph{
		Nodes: []Node{
			{ID: "center"},
			{ID: "leaf-1"},
			{ID: "leaf-2"},
			{ID: "leaf-3"},
		},
		Edges: []Edge{
			{Source: "center", Target: "leaf-1"},
			{Source: "center", Target: "leaf-2"},
			{Source: "center", Target: "leaf-3"},
		},
	}

	count, err := NeighborCount(graph, "center")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 3 {
		t.Errorf("NeighborCount(center) = %d, want 3", count)
	}

	// A leaf should have 1 neighbor (the center).
	count, err = NeighborCount(graph, "leaf-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 1 {
		t.Errorf("NeighborCount(leaf-2) = %d, want 1", count)
	}
}

func TestNeighborCountNilGraph(t *testing.T) {
	_, err := NeighborCount(nil, "A")
	if err == nil {
		t.Error("expected error for nil graph, got nil")
	}
}

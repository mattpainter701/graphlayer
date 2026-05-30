package orchestration

import (
	"sort"
	"testing"
)

func TestExportAdjacencyList_SimpleGraph(t *testing.T) {
	g := &TopologyGraph{
		Nodes: []GraphNode{
			{ID: "a"},
			{ID: "b"},
			{ID: "c"},
		},
		Edges: []GraphEdge{
			{Source: "a", Target: "b"},
			{Source: "a", Target: "c"},
			{Source: "b", Target: "c"},
		},
	}

	adj, err := ExportAdjacencyList(g)
	if err != nil {
		t.Fatalf("ExportAdjacencyList returned error: %v", err)
	}

	if len(adj) != 3 {
		t.Fatalf("expected 3 nodes in adjacency list, got %d", len(adj))
	}

	expectNeighbors(t, adj, "a", []string{"b", "c"})
	expectNeighbors(t, adj, "b", []string{"c"})
	expectNeighbors(t, adj, "c", []string{})
}

func TestExportAdjacencyList_IsolatedNode(t *testing.T) {
	g := &TopologyGraph{
		Nodes: []GraphNode{
			{ID: "isolated"},
			{ID: "connected"},
		},
		Edges: []GraphEdge{
			{Source: "connected", Target: "somewhere"},
		},
	}

	adj, err := ExportAdjacencyList(g)
	if err != nil {
		t.Fatalf("ExportAdjacencyList returned error: %v", err)
	}

	// Isolated node should have an empty neighbor slice.
	if len(adj["isolated"]) != 0 {
		t.Fatalf("expected isolated node to have 0 neighbors, got %d: %v", len(adj["isolated"]), adj["isolated"])
	}
}

func TestExportAdjacencyList_NilGraph(t *testing.T) {
	_, err := ExportAdjacencyList(nil)
	if err == nil {
		t.Fatal("expected error for nil graph, got nil")
	}
}

func TestExportAdjacencyList_EdgeToUnknownNode(t *testing.T) {
	g := &TopologyGraph{
		Nodes: []GraphNode{
			{ID: "a"},
		},
		Edges: []GraphEdge{
			{Source: "a", Target: "missing"},
		},
	}

	adj, err := ExportAdjacencyList(g)
	if err != nil {
		t.Fatalf("ExportAdjacencyList returned error: %v", err)
	}

	// Edge to unknown node should still appear from "a".
	if len(adj["a"]) != 1 || adj["a"][0] != "missing" {
		t.Fatalf("expected a -> [missing], got %v", adj["a"])
	}
}

func TestExportAdjacencyList_DisconnectedGraph(t *testing.T) {
	g := &TopologyGraph{
		Nodes: []GraphNode{
			{ID: "x"},
			{ID: "y"},
		},
	}

	adj, err := ExportAdjacencyList(g)
	if err != nil {
		t.Fatalf("ExportAdjacencyList returned error: %v", err)
	}

	if len(adj) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(adj))
	}
	if len(adj["x"]) != 0 {
		t.Fatalf("expected x to have 0 neighbors, got %d", len(adj["x"]))
	}
	if len(adj["y"]) != 0 {
		t.Fatalf("expected y to have 0 neighbors, got %d", len(adj["y"]))
	}
}

func TestExportAdjacencyList_MultiEdgeBetweenSameNodes(t *testing.T) {
	g := &TopologyGraph{
		Nodes: []GraphNode{
			{ID: "a"},
			{ID: "b"},
		},
		Edges: []GraphEdge{
			{Source: "a", Target: "b"},
			{Source: "a", Target: "b"},
		},
	}

	adj, err := ExportAdjacencyList(g)
	if err != nil {
		t.Fatalf("ExportAdjacencyList returned error: %v", err)
	}

	expectNeighbors(t, adj, "a", []string{"b", "b"})
	expectNeighbors(t, adj, "b", []string{})
}

// expectNeighbors checks that the adjacency list for nodeID contains
// exactly the expected neighbor IDs (order matters — we match as-is).
func expectNeighbors(t *testing.T, adj map[string][]string, nodeID string, expected []string) {
	t.Helper()

	got, ok := adj[nodeID]
	if !ok {
		t.Fatalf("node %q not found in adjacency list", nodeID)
	}

	if len(got) != len(expected) {
		t.Fatalf("node %q: expected %d neighbors, got %d: %v", nodeID, len(expected), len(got), got)
	}

	// Sort both for comparison when order doesn't matter.
	sortedGot := make([]string, len(got))
	copy(sortedGot, got)
	sort.Strings(sortedGot)

	sortedExpected := make([]string, len(expected))
	copy(sortedExpected, expected)
	sort.Strings(sortedExpected)

	for i := range sortedGot {
		if sortedGot[i] != sortedExpected[i] {
			t.Fatalf("node %q: neighbor mismatch at index %d: got %q, expected %q (got=%v, expected=%v)",
				nodeID, i, sortedGot[i], sortedExpected[i], got, expected)
		}
	}
}

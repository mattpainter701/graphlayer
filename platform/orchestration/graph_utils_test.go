package orchestration

import (
	"testing"
)

func TestTopologicalSortValidDAG(t *testing.T) {
	g := &TopologyGraph{
		Nodes: []GraphNode{
			{ID: "a"},
			{ID: "b"},
			{ID: "c"},
			{ID: "d"},
			{ID: "e"},
		},
		Edges: []GraphEdge{
			{Source: "a", Target: "b"},
			{Source: "a", Target: "c"},
			{Source: "b", Target: "d"},
			{Source: "c", Target: "d"},
			{Source: "d", Target: "e"},
		},
	}

	got, err := TopologicalSort(g)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 5 {
		t.Fatalf("expected 5 nodes, got %d: %v", len(got), got)
	}

	// Verify topological property: for every edge Source→Target,
	// Source must appear before Target in the result.
	pos := make(map[string]int, len(got))
	for i, id := range got {
		pos[id] = i
	}
	for _, edge := range g.Edges {
		if pos[edge.Source] > pos[edge.Target] {
			t.Fatalf("topological order violated: edge %s->%s but %s appears after %s",
				edge.Source, edge.Target, edge.Source, edge.Target)
		}
	}
}

func TestTopologicalSortDisconnectedDAG(t *testing.T) {
	// Graph with isolated nodes alongside a DAG chain.
	g := &TopologyGraph{
		Nodes: []GraphNode{
			{ID: "a"},
			{ID: "b"},
			{ID: "c"},
		},
		Edges: []GraphEdge{
			{Source: "a", Target: "b"},
		},
	}

	got, err := TopologicalSort(g)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 3 {
		t.Fatalf("expected 3 nodes, got %d: %v", len(got), got)
	}

	// 'a' must come before 'b'.
	pos := make(map[string]int, len(got))
	for i, id := range got {
		pos[id] = i
	}
	if pos["a"] > pos["b"] {
		t.Fatalf("topological order violated: a->b but a appears after b")
	}
}

func TestTopologicalSortWithCycle(t *testing.T) {
	g := &TopologyGraph{
		Nodes: []GraphNode{
			{ID: "a"},
			{ID: "b"},
			{ID: "c"},
		},
		Edges: []GraphEdge{
			{Source: "a", Target: "b"},
			{Source: "b", Target: "c"},
			{Source: "c", Target: "a"},
		},
	}

	_, err := TopologicalSort(g)
	if err == nil {
		t.Fatal("expected error for cyclic graph, got nil")
	}
}

func TestTopologicalSortSelfLoopsIgnored(t *testing.T) {
	g := &TopologyGraph{
		Nodes: []GraphNode{
			{ID: "a"},
			{ID: "b"},
		},
		Edges: []GraphEdge{
			{Source: "a", Target: "a"}, // self-loop, should be ignored
			{Source: "a", Target: "b"},
		},
	}

	got, err := TopologicalSort(g)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 nodes, got %d: %v", len(got), got)
	}
}

func TestTopologicalSortEmptyGraph(t *testing.T) {
	g := &TopologyGraph{}
	got, err := TopologicalSort(g)
	if err != nil {
		t.Fatalf("unexpected error for empty graph: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty result, got %v", got)
	}
}

func TestTopologicalSortNilGraph(t *testing.T) {
	_, err := TopologicalSort(nil)
	if err == nil {
		t.Fatal("expected error for nil graph, got nil")
	}
}

func TestTopologicalSortSingleNode(t *testing.T) {
	g := &TopologyGraph{
		Nodes: []GraphNode{{ID: "solo"}},
	}
	got, err := TopologicalSort(g)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0] != "solo" {
		t.Fatalf("expected [solo], got %v", got)
	}
}

func TestTopologicalSortEmptyNodeID(t *testing.T) {
	g := &TopologyGraph{
		Nodes: []GraphNode{
			{ID: ""},
			{ID: "a"},
			{ID: "b"},
		},
		Edges: []GraphEdge{
			{Source: "a", Target: "b"},
		},
	}

	got, err := TopologicalSort(g)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 nodes (empty ID skipped), got %d: %v", len(got), got)
	}
}

func TestTopologicalSortMultipleRoots(t *testing.T) {
	g := &TopologyGraph{
		Nodes: []GraphNode{
			{ID: "a"},
			{ID: "b"},
			{ID: "c"},
			{ID: "d"},
		},
		Edges: []GraphEdge{
			{Source: "a", Target: "c"},
			{Source: "b", Target: "d"},
		},
	}

	got, err := TopologicalSort(g)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 4 {
		t.Fatalf("expected 4 nodes, got %d: %v", len(got), got)
	}

	// a must come before c, b must come before d.
	pos := make(map[string]int, len(got))
	for i, id := range got {
		pos[id] = i
	}
	if pos["a"] > pos["c"] {
		t.Fatal("a must appear before c")
	}
	if pos["b"] > pos["d"] {
		t.Fatal("b must appear before d")
	}
}

func TestTopologicalSortDiamondDAG(t *testing.T) {
	// Diamond: a -> b, a -> c, b -> d, c -> d
	g := &TopologyGraph{
		Nodes: []GraphNode{
			{ID: "a"},
			{ID: "b"},
			{ID: "c"},
			{ID: "d"},
		},
		Edges: []GraphEdge{
			{Source: "a", Target: "b"},
			{Source: "a", Target: "c"},
			{Source: "b", Target: "d"},
			{Source: "c", Target: "d"},
		},
	}

	got, err := TopologicalSort(g)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 4 {
		t.Fatalf("expected 4 nodes, got %d: %v", len(got), got)
	}

	pos := make(map[string]int, len(got))
	for i, id := range got {
		pos[id] = i
	}
	if pos["a"] > pos["b"] || pos["a"] > pos["c"] {
		t.Fatal("a must appear before its dependents")
	}
	if pos["b"] > pos["d"] || pos["c"] > pos["d"] {
		t.Fatal("dependents must appear before d")
	}
}

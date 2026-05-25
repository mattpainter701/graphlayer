package graph

import (
	"reflect"
	"testing"
)

func TestNodeDegreeStats_StarGraph(t *testing.T) {
	g := &TopologyGraph{
		Nodes: []GraphNode{
			{ID: "center"},
			{ID: "leaf-1"},
			{ID: "leaf-2"},
			{ID: "leaf-3"},
			{ID: "leaf-4"},
		},
		Edges: []GraphEdge{
			{Source: "center", Target: "leaf-1"},
			{Source: "center", Target: "leaf-2"},
			{Source: "center", Target: "leaf-3"},
			{Source: "center", Target: "leaf-4"},
		},
	}

	min, max, avg, histogram := NodeDegreeStats(g)

	if min != 1 {
		t.Fatalf("expected min degree 1, got %v", min)
	}
	if max != 4 {
		t.Fatalf("expected max degree 4, got %v", max)
	}
	if avg != 1.6 {
		t.Fatalf("expected avg degree 1.6, got %v", avg)
	}

	expectedHistogram := map[int]int{1: 4, 4: 1}
	if !reflect.DeepEqual(histogram, expectedHistogram) {
		t.Fatalf("expected histogram %v, got %v", expectedHistogram, histogram)
	}
}

func TestNodeDegreeStats_LineGraph(t *testing.T) {
	g := &TopologyGraph{
		Nodes: []GraphNode{
			{ID: "n1"},
			{ID: "n2"},
			{ID: "n3"},
			{ID: "n4"},
		},
		Edges: []GraphEdge{
			{Source: "n1", Target: "n2"},
			{Source: "n2", Target: "n3"},
			{Source: "n3", Target: "n4"},
		},
	}

	min, max, avg, histogram := NodeDegreeStats(g)

	if min != 1 {
		t.Fatalf("expected min degree 1, got %v", min)
	}
	if max != 2 {
		t.Fatalf("expected max degree 2, got %v", max)
	}
	if avg != 1.5 {
		t.Fatalf("expected avg degree 1.5, got %v", avg)
	}

	expectedHistogram := map[int]int{1: 2, 2: 2}
	if !reflect.DeepEqual(histogram, expectedHistogram) {
		t.Fatalf("expected histogram %v, got %v", expectedHistogram, histogram)
	}
}

func TestShortestPath_DirectNeighbor(t *testing.T) {
	g := &TopologyGraph{
		Nodes: []GraphNode{{ID: "a"}, {ID: "b"}},
		Edges: []GraphEdge{{Source: "a", Target: "b"}},
	}

	path, err := ShortestPath(g, "a", "b")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expected := []string{"a", "b"}
	if !reflect.DeepEqual(path, expected) {
		t.Fatalf("expected path %v, got %v", expected, path)
	}
}

func TestShortestPath_MultiHop(t *testing.T) {
	g := &TopologyGraph{
		Nodes: []GraphNode{{ID: "a"}, {ID: "b"}, {ID: "c"}, {ID: "d"}},
		Edges: []GraphEdge{
			{Source: "a", Target: "b"},
			{Source: "b", Target: "c"},
			{Source: "c", Target: "d"},
		},
	}

	path, err := ShortestPath(g, "a", "d")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expected := []string{"a", "b", "c", "d"}
	if !reflect.DeepEqual(path, expected) {
		t.Fatalf("expected path %v, got %v", expected, path)
	}
}

func TestShortestPath_Unreachable(t *testing.T) {
	g := &TopologyGraph{
		Nodes: []GraphNode{{ID: "a"}, {ID: "b"}, {ID: "c"}},
		Edges: []GraphEdge{{Source: "a", Target: "b"}},
	}

	path, err := ShortestPath(g, "a", "c")
	if err == nil {
		t.Fatalf("expected error, got path %v", path)
	}
	if path != nil {
		t.Fatalf("expected nil path, got %v", path)
	}
}

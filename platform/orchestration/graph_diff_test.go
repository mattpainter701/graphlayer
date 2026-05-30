package orchestration

import (
	"reflect"
	"testing"
)

func TestGraphDiff_AddingNode(t *testing.T) {
	g1 := &TopologyGraph{
		Nodes: []GraphNode{{ID: "a"}},
	}
	g2 := &TopologyGraph{
		Nodes: []GraphNode{{ID: "a"}, {ID: "b"}},
	}

	addedNodes, removedNodes, addedEdges, removedEdges, err := GraphDiff(g1, g2)
	if err != nil {
		t.Fatalf("GraphDiff returned error: %v", err)
	}

	if !reflect.DeepEqual(addedNodes, []string{"b"}) {
		t.Fatalf("expected added nodes [b], got %#v", addedNodes)
	}
	if len(removedNodes) != 0 {
		t.Fatalf("expected no removed nodes, got %#v", removedNodes)
	}
	if len(addedEdges) != 0 {
		t.Fatalf("expected no added edges, got %#v", addedEdges)
	}
	if len(removedEdges) != 0 {
		t.Fatalf("expected no removed edges, got %#v", removedEdges)
	}
}

func TestGraphDiff_RemovingNode(t *testing.T) {
	g1 := &TopologyGraph{
		Nodes: []GraphNode{{ID: "a"}, {ID: "b"}},
	}
	g2 := &TopologyGraph{
		Nodes: []GraphNode{{ID: "a"}},
	}

	addedNodes, removedNodes, addedEdges, removedEdges, err := GraphDiff(g1, g2)
	if err != nil {
		t.Fatalf("GraphDiff returned error: %v", err)
	}

	if len(addedNodes) != 0 {
		t.Fatalf("expected no added nodes, got %#v", addedNodes)
	}
	if !reflect.DeepEqual(removedNodes, []string{"b"}) {
		t.Fatalf("expected removed nodes [b], got %#v", removedNodes)
	}
	if len(addedEdges) != 0 {
		t.Fatalf("expected no added edges, got %#v", addedEdges)
	}
	if len(removedEdges) != 0 {
		t.Fatalf("expected no removed edges, got %#v", removedEdges)
	}
}

func TestGraphDiff_AddingEdge(t *testing.T) {
	g1 := &TopologyGraph{
		Nodes: []GraphNode{{ID: "a"}, {ID: "b"}},
	}
	g2 := &TopologyGraph{
		Nodes: []GraphNode{{ID: "a"}, {ID: "b"}},
		Edges: []GraphEdge{{Source: "a", Target: "b", Weight: 1.5}},
	}

	addedNodes, removedNodes, addedEdges, removedEdges, err := GraphDiff(g1, g2)
	if err != nil {
		t.Fatalf("GraphDiff returned error: %v", err)
	}

	if len(addedNodes) != 0 || len(removedNodes) != 0 {
		t.Fatalf("expected no node changes, got added=%#v removed=%#v", addedNodes, removedNodes)
	}
	if !reflect.DeepEqual(addedEdges, []Edge{{Source: "a", Target: "b", Weight: 1.5}}) {
		t.Fatalf("expected added edge a->b, got %#v", addedEdges)
	}
	if len(removedEdges) != 0 {
		t.Fatalf("expected no removed edges, got %#v", removedEdges)
	}
}

func TestGraphDiff_RemovingEdge(t *testing.T) {
	g1 := &TopologyGraph{
		Nodes: []GraphNode{{ID: "a"}, {ID: "b"}},
		Edges: []GraphEdge{{Source: "a", Target: "b", Weight: 1.5}},
	}
	g2 := &TopologyGraph{
		Nodes: []GraphNode{{ID: "a"}, {ID: "b"}},
	}

	addedNodes, removedNodes, addedEdges, removedEdges, err := GraphDiff(g1, g2)
	if err != nil {
		t.Fatalf("GraphDiff returned error: %v", err)
	}

	if len(addedNodes) != 0 || len(removedNodes) != 0 {
		t.Fatalf("expected no node changes, got added=%#v removed=%#v", addedNodes, removedNodes)
	}
	if len(addedEdges) != 0 {
		t.Fatalf("expected no added edges, got %#v", addedEdges)
	}
	if !reflect.DeepEqual(removedEdges, []Edge{{Source: "a", Target: "b", Weight: 1.5}}) {
		t.Fatalf("expected removed edge a->b, got %#v", removedEdges)
	}
}

func TestGraphDiff_NoChanges(t *testing.T) {
	g := &TopologyGraph{
		Nodes: []GraphNode{{ID: "a"}, {ID: "b"}},
		Edges: []GraphEdge{{Source: "a", Target: "b", Weight: 1.5}},
	}

	addedNodes, removedNodes, addedEdges, removedEdges, err := GraphDiff(g, g)
	if err != nil {
		t.Fatalf("GraphDiff returned error: %v", err)
	}

	if len(addedNodes) != 0 || len(removedNodes) != 0 || len(addedEdges) != 0 || len(removedEdges) != 0 {
		t.Fatalf("expected no changes, got addedNodes=%#v removedNodes=%#v addedEdges=%#v removedEdges=%#v", addedNodes, removedNodes, addedEdges, removedEdges)
	}
}

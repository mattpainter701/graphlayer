package orchestration

import (
	"testing"
)

func TestGraphIntersection_DisjointGraphs(t *testing.T) {
	g1 := &TopologyGraph{
		Nodes: []GraphNode{
			{ID: "a", Attributes: map[string]string{"env": "prod"}},
			{ID: "b", Attributes: map[string]string{"role": "switch"}},
		},
		Edges: []GraphEdge{
			{Source: "a", Target: "b", Weight: 1.0},
		},
	}
	g2 := &TopologyGraph{
		Nodes: []GraphNode{
			{ID: "c", Attributes: map[string]string{"role": "router"}},
			{ID: "d"},
		},
		Edges: []GraphEdge{
			{Source: "c", Target: "d", Weight: 2.0},
		},
	}

	got, err := GraphIntersection(g1, g2)
	if err != nil {
		t.Fatalf("GraphIntersection returned error: %v", err)
	}
	if len(got.Nodes) != 0 {
		t.Fatalf("expected 0 nodes for disjoint graphs, got %d", len(got.Nodes))
	}
	if len(got.Edges) != 0 {
		t.Fatalf("expected 0 edges for disjoint graphs, got %d", len(got.Edges))
	}
}

func TestGraphIntersection_OverlappingNodes(t *testing.T) {
	g1 := &TopologyGraph{
		Nodes: []GraphNode{
			{ID: "a", Attributes: map[string]string{"env": "prod", "owner": "team-a"}},
			{ID: "b", Attributes: map[string]string{"role": "switch"}},
			{ID: "c", Attributes: map[string]string{"role": "firewall"}},
		},
		Edges: []GraphEdge{
			{Source: "a", Target: "b", Weight: 1.0, Attributes: map[string]string{"kind": "l2"}},
			{Source: "a", Target: "c", Weight: 2.0, Attributes: map[string]string{"kind": "l3"}},
		},
	}
	g2 := &TopologyGraph{
		Nodes: []GraphNode{
			{ID: "a", Attributes: map[string]string{"env": "stage", "region": "us-east"}},
			{ID: "b"},
			{ID: "d", Attributes: map[string]string{"role": "load-balancer"}},
		},
		Edges: []GraphEdge{
			{Source: "a", Target: "b", Weight: 3.0, Attributes: map[string]string{"metric": "42"}},
			{Source: "b", Target: "d", Weight: 4.0},
		},
	}

	got, err := GraphIntersection(g1, g2)
	if err != nil {
		t.Fatalf("GraphIntersection returned error: %v", err)
	}

	if len(got.Nodes) != 2 {
		t.Fatalf("expected 2 nodes (a, b), got %d", len(got.Nodes))
	}
	if len(got.Edges) != 1 {
		t.Fatalf("expected 1 edge (a->b), got %d", len(got.Edges))
	}

	// Node "a" should have merged attributes (g1 base, g2 override).
	var nodeA *GraphNode
	for i, n := range got.Nodes {
		if n.ID == "a" {
			nodeA = &got.Nodes[i]
			break
		}
	}
	if nodeA == nil {
		t.Fatal("expected node 'a' to be in intersection")
	}
	if nodeA.Attributes["env"] != "stage" {
		t.Fatalf("expected env=stage (g2 override), got %q", nodeA.Attributes["env"])
	}
	if nodeA.Attributes["owner"] != "team-a" {
		t.Fatalf("expected owner=team-a from g1, got %q", nodeA.Attributes["owner"])
	}
	if nodeA.Attributes["region"] != "us-east" {
		t.Fatalf("expected region=us-east from g2, got %q", nodeA.Attributes["region"])
	}

	// Edge a->b should exist with g2 weight and merged attributes.
	if got.Edges[0].Source != "a" || got.Edges[0].Target != "b" {
		t.Fatalf("expected edge a->b, got %s->%s", got.Edges[0].Source, got.Edges[0].Target)
	}
	if got.Edges[0].Weight != 3.0 {
		t.Fatalf("expected weight 3.0 from g2, got %v", got.Edges[0].Weight)
	}
	if got.Edges[0].Attributes["kind"] != "l2" {
		t.Fatalf("expected kind=l2 from g1, got %q", got.Edges[0].Attributes["kind"])
	}
	if got.Edges[0].Attributes["metric"] != "42" {
		t.Fatalf("expected metric=42 from g2, got %q", got.Edges[0].Attributes["metric"])
	}
}

func TestGraphIntersection_IdenticalGraphs(t *testing.T) {
	g1 := &TopologyGraph{
		Nodes: []GraphNode{
			{ID: "a", Attributes: map[string]string{"env": "prod"}},
			{ID: "b", Attributes: map[string]string{"role": "switch"}},
		},
		Edges: []GraphEdge{
			{Source: "a", Target: "b", Weight: 1.0},
		},
	}

	got, err := GraphIntersection(g1, g1)
	if err != nil {
		t.Fatalf("GraphIntersection returned error: %v", err)
	}

	if len(got.Nodes) != 2 {
		t.Fatalf("expected 2 nodes for identical graphs, got %d", len(got.Nodes))
	}
	if len(got.Edges) != 1 {
		t.Fatalf("expected 1 edge for identical graphs, got %d", len(got.Edges))
	}
}

func TestGraphIntersection_EmptyGraphs(t *testing.T) {
	// Both nil.
	got, err := GraphIntersection(nil, nil)
	if err != nil {
		t.Fatalf("GraphIntersection(nil, nil) returned error: %v", err)
	}
	if len(got.Nodes) != 0 || len(got.Edges) != 0 {
		t.Fatal("expected empty result for (nil, nil)")
	}

	// First nil.
	g2 := &TopologyGraph{
		Nodes: []GraphNode{{ID: "a"}},
	}
	got, err = GraphIntersection(nil, g2)
	if err != nil {
		t.Fatalf("GraphIntersection(nil, g2) returned error: %v", err)
	}
	if len(got.Nodes) != 0 || len(got.Edges) != 0 {
		t.Fatal("expected empty result for (nil, g2)")
	}

	// Second nil.
	g1 := &TopologyGraph{
		Nodes: []GraphNode{{ID: "a"}},
	}
	got, err = GraphIntersection(g1, nil)
	if err != nil {
		t.Fatalf("GraphIntersection(g1, nil) returned error: %v", err)
	}
	if len(got.Nodes) != 0 || len(got.Edges) != 0 {
		t.Fatal("expected empty result for (g1, nil)")
	}

	// Both empty.
	empty1 := &TopologyGraph{}
	empty2 := &TopologyGraph{}
	got, err = GraphIntersection(empty1, empty2)
	if err != nil {
		t.Fatalf("GraphIntersection(empty, empty) returned error: %v", err)
	}
	if len(got.Nodes) != 0 || len(got.Edges) != 0 {
		t.Fatal("expected empty result for (empty, empty)")
	}
}

func TestGraphIntersection_EmptyNodeID(t *testing.T) {
	g1 := &TopologyGraph{
		Nodes: []GraphNode{{ID: ""}},
	}
	g2 := &TopologyGraph{
		Nodes: []GraphNode{{ID: "a"}},
	}

	_, err := GraphIntersection(g1, g2)
	if err == nil {
		t.Fatal("expected error for empty node ID in g1")
	}

	// Empty node ID in g2.
	g1 = &TopologyGraph{Nodes: []GraphNode{{ID: "a"}}}
	g2 = &TopologyGraph{Nodes: []GraphNode{{ID: ""}}}
	_, err = GraphIntersection(g1, g2)
	if err == nil {
		t.Fatal("expected error for empty node ID in g2")
	}
}

func TestGraphIntersection_EmptyEdgeSourceTarget(t *testing.T) {
	g1 := &TopologyGraph{
		Edges: []GraphEdge{{Source: "", Target: "b"}},
	}
	g2 := &TopologyGraph{
		Edges: []GraphEdge{{Source: "a", Target: "b"}},
	}

	_, err := GraphIntersection(g1, g2)
	if err == nil {
		t.Fatal("expected error for empty edge source in g1")
	}

	// Empty target in g2.
	g1 = &TopologyGraph{Edges: []GraphEdge{{Source: "a", Target: "b"}}}
	g2 = &TopologyGraph{Edges: []GraphEdge{{Source: "a", Target: ""}}}
	_, err = GraphIntersection(g1, g2)
	if err == nil {
		t.Fatal("expected error for empty edge target in g2")
	}
}

func TestGraphUnion_DelegatesToUnion(t *testing.T) {
	g1 := &TopologyGraph{
		Nodes: []GraphNode{{ID: "a", Attributes: map[string]string{"env": "prod"}}},
		Edges: []GraphEdge{{Source: "a", Target: "b", Weight: 1.5}},
	}
	g2 := &TopologyGraph{
		Nodes: []GraphNode{{ID: "c", Attributes: map[string]string{"role": "router"}}},
		Edges: []GraphEdge{{Source: "c", Target: "d", Weight: 2.5}},
	}

	got, err := GraphUnion(g1, g2)
	if err != nil {
		t.Fatalf("GraphUnion returned error: %v", err)
	}

	if len(got.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(got.Nodes))
	}
	if len(got.Edges) != 2 {
		t.Fatalf("expected 2 edges, got %d", len(got.Edges))
	}
}

func TestGraphUnion_NilInputs(t *testing.T) {
	// Both nil.
	got, err := GraphUnion(nil, nil)
	if err != nil {
		t.Fatalf("GraphUnion(nil, nil) returned error: %v", err)
	}
	if len(got.Nodes) != 0 || len(got.Edges) != 0 {
		t.Fatal("expected empty result for (nil, nil)")
	}

	// g1 nil, g2 non-nil.
	g2 := &TopologyGraph{
		Nodes: []GraphNode{{ID: "a"}},
	}
	got, err = GraphUnion(nil, g2)
	if err != nil {
		t.Fatalf("GraphUnion(nil, g2) returned error: %v", err)
	}
	if len(got.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(got.Nodes))
	}

	// g1 non-nil, g2 nil.
	g1 := &TopologyGraph{
		Nodes: []GraphNode{{ID: "b"}},
	}
	got, err = GraphUnion(g1, nil)
	if err != nil {
		t.Fatalf("GraphUnion(g1, nil) returned error: %v", err)
	}
	if len(got.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(got.Nodes))
	}
}

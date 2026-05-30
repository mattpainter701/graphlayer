package orchestration

import "testing"

func TestUnion_DisjointGraphs(t *testing.T) {
	g1 := &TopologyGraph{
		Nodes: []GraphNode{{ID: "a", Attributes: map[string]string{"env": "prod"}}},
		Edges: []GraphEdge{{Source: "a", Target: "b", Weight: 1.5, Attributes: map[string]string{"kind": "l2"}}},
	}
	g2 := &TopologyGraph{
		Nodes: []GraphNode{{ID: "c", Attributes: map[string]string{"role": "router"}}},
		Edges: []GraphEdge{{Source: "c", Target: "d", Weight: 2.5, Attributes: map[string]string{"kind": "l3"}}},
	}

	got, err := Union(g1, g2)
	if err != nil {
		t.Fatalf("Union returned error: %v", err)
	}

	if len(got.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(got.Nodes))
	}
	if len(got.Edges) != 2 {
		t.Fatalf("expected 2 edges, got %d", len(got.Edges))
	}
}

func TestUnion_OverlappingNodes(t *testing.T) {
	g1 := &TopologyGraph{
		Nodes: []GraphNode{
			{ID: "a", Attributes: map[string]string{"env": "prod", "owner": "team-a"}},
			{ID: "b", Attributes: map[string]string{"role": "switch"}},
		},
	}
	g2 := &TopologyGraph{
		Nodes: []GraphNode{
			{ID: "a", Attributes: map[string]string{"env": "stage", "region": "us-east"}},
			{ID: "c", Attributes: map[string]string{"role": "firewall"}},
		},
	}

	got, err := Union(g1, g2)
	if err != nil {
		t.Fatalf("Union returned error: %v", err)
	}

	if len(got.Nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(got.Nodes))
	}

	var merged GraphNode
	for _, node := range got.Nodes {
		if node.ID == "a" {
			merged = node
			break
		}
	}

	if merged.ID != "a" {
		t.Fatal("expected merged node a to exist")
	}
	if merged.Attributes["env"] != "stage" {
		t.Fatalf("expected env from g2, got %q", merged.Attributes["env"])
	}
	if merged.Attributes["owner"] != "team-a" {
		t.Fatalf("expected owner from g1, got %q", merged.Attributes["owner"])
	}
	if merged.Attributes["region"] != "us-east" {
		t.Fatalf("expected region from g2, got %q", merged.Attributes["region"])
	}
}

func TestUnion_OverlappingEdges(t *testing.T) {
	g1 := &TopologyGraph{
		Edges: []GraphEdge{
			{Source: "a", Target: "b", Weight: 1.25, Attributes: map[string]string{"protocol": "lldp", "state": "up"}},
		},
	}
	g2 := &TopologyGraph{
		Edges: []GraphEdge{
			{Source: "a", Target: "b", Weight: 3.75, Attributes: map[string]string{"protocol": "cdp", "metric": "42"}},
		},
	}

	got, err := Union(g1, g2)
	if err != nil {
		t.Fatalf("Union returned error: %v", err)
	}

	if len(got.Edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(got.Edges))
	}

	edge := got.Edges[0]
	if edge.Weight != 3.75 {
		t.Fatalf("expected weight from g2, got %v", edge.Weight)
	}
	if edge.Attributes["protocol"] != "cdp" {
		t.Fatalf("expected protocol from g2, got %q", edge.Attributes["protocol"])
	}
	if edge.Attributes["state"] != "up" {
		t.Fatalf("expected state from g1, got %q", edge.Attributes["state"])
	}
	if edge.Attributes["metric"] != "42" {
		t.Fatalf("expected metric from g2, got %q", edge.Attributes["metric"])
	}
}

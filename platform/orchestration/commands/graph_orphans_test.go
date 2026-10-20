package commands

import (
	"strings"
	"testing"

	"github.com/mattpainter701/graphlayer/platform/orchestration/topology"
)

func TestFindOrphanNodes_NoOrphans(t *testing.T) {
	g := &topology.Graph{
		Nodes: []topology.Node{
			{ID: "A"}, {ID: "B"}, {ID: "C"},
		},
		Edges: []topology.Edge{
			{Source: "A", Target: "B"},
			{Source: "B", Target: "C"},
		},
	}

	orphans := FindOrphanNodes(g)
	if len(orphans) != 0 {
		t.Fatalf("expected 0 orphans, got %d: %v", len(orphans), orphans)
	}
}

func TestFindOrphanNodes_SomeOrphans(t *testing.T) {
	g := &topology.Graph{
		Nodes: []topology.Node{
			{ID: "A"}, {ID: "B"}, {ID: "C"}, {ID: "D"}, {ID: "E"},
		},
		Edges: []topology.Edge{
			{Source: "A", Target: "B"},
			{Source: "B", Target: "C"},
		},
	}

	orphans := FindOrphanNodes(g)
	if len(orphans) != 2 {
		t.Fatalf("expected 2 orphans, got %d: %v", len(orphans), orphans)
	}
	// D and E are orphans (no edges)
	for _, id := range []string{"D", "E"} {
		found := false
		for _, o := range orphans {
			if o == id {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected orphan %q in result", id)
		}
	}
}

func TestFindOrphanNodes_AllOrphans(t *testing.T) {
	g := &topology.Graph{
		Nodes: []topology.Node{
			{ID: "X"}, {ID: "Y"}, {ID: "Z"},
		},
		Edges: []topology.Edge{},
	}

	orphans := FindOrphanNodes(g)
	if len(orphans) != 3 {
		t.Fatalf("expected 3 orphans, got %d: %v", len(orphans), orphans)
	}
}

func TestFindOrphanNodes_NilGraph(t *testing.T) {
	orphans := FindOrphanNodes(nil)
	if orphans != nil {
		t.Fatalf("expected nil for nil graph, got %v", orphans)
	}
}

func TestFindOrphanNodes_EmptyGraph(t *testing.T) {
	g := &topology.Graph{
		Nodes: []topology.Node{},
		Edges: []topology.Edge{},
	}
	orphans := FindOrphanNodes(g)
	if len(orphans) != 0 {
		t.Fatalf("expected 0 orphans for empty graph, got %d", len(orphans))
	}
}

func TestCountDisconnectedSubgraphs_ConnectedGraph(t *testing.T) {
	g := &topology.Graph{
		Nodes: []topology.Node{
			{ID: "A"}, {ID: "B"}, {ID: "C"},
		},
		Edges: []topology.Edge{
			{Source: "A", Target: "B"},
			{Source: "B", Target: "C"},
		},
	}

	count := CountDisconnectedSubgraphs(g)
	if count != 1 {
		t.Fatalf("expected 1 connected component, got %d", count)
	}
}

func TestCountDisconnectedSubgraphs_TwoComponents(t *testing.T) {
	g := &topology.Graph{
		Nodes: []topology.Node{
			{ID: "A"}, {ID: "B"}, {ID: "C"}, {ID: "D"},
		},
		Edges: []topology.Edge{
			{Source: "A", Target: "B"},
			{Source: "C", Target: "D"},
		},
	}

	count := CountDisconnectedSubgraphs(g)
	if count != 2 {
		t.Fatalf("expected 2 connected components, got %d", count)
	}
}

func TestCountDisconnectedSubgraphs_IsolatedNodes(t *testing.T) {
	g := &topology.Graph{
		Nodes: []topology.Node{
			{ID: "X"}, {ID: "Y"}, {ID: "Z"},
		},
		Edges: []topology.Edge{},
	}

	count := CountDisconnectedSubgraphs(g)
	if count != 3 {
		t.Fatalf("expected 3 components (each node isolated), got %d", count)
	}
}

func TestCountDisconnectedSubgraphs_Mixed(t *testing.T) {
	g := &topology.Graph{
		Nodes: []topology.Node{
			{ID: "A"}, {ID: "B"}, {ID: "C"},
			{ID: "D"}, {ID: "E"},
			{ID: "X"}, {ID: "Y"},
		},
		Edges: []topology.Edge{
			{Source: "A", Target: "B"},
			{Source: "B", Target: "C"},
			{Source: "D", Target: "E"},
		},
	}

	count := CountDisconnectedSubgraphs(g)
	if count != 3 {
		// Component 1: A-B-C, Component 2: D-E, Component 3: X-Y (both isolated)
		t.Fatalf("expected 3 components, got %d", count)
	}
}

func TestCountDisconnectedSubgraphs_NilGraph(t *testing.T) {
	count := CountDisconnectedSubgraphs(nil)
	if count != 0 {
		t.Fatalf("expected 0 for nil graph, got %d", count)
	}
}

func TestCountDisconnectedSubgraphs_EmptyGraph(t *testing.T) {
	g := &topology.Graph{
		Nodes: []topology.Node{},
		Edges: []topology.Edge{},
	}
	count := CountDisconnectedSubgraphs(g)
	if count != 0 {
		t.Fatalf("expected 0 for empty graph, got %d", count)
	}
}

func TestCountDisconnectedSubgraphs_SelfLoopNode(t *testing.T) {
	g := &topology.Graph{
		Nodes: []topology.Node{
			{ID: "A"}, {ID: "B"},
		},
		Edges: []topology.Edge{
			{Source: "A", Target: "A"}, // self-loop
		},
	}

	count := CountDisconnectedSubgraphs(g)
	if count != 2 {
		// A has a self-loop but no connection to B, so 2 components
		t.Fatalf("expected 2 components (self-loop doesn't connect), got %d", count)
	}
}

func TestAnalyzeOrphans_Simple(t *testing.T) {
	g := &topology.Graph{
		Nodes: []topology.Node{
			{ID: "A"}, {ID: "B"}, {ID: "C"},
		},
		Edges: []topology.Edge{
			{Source: "A", Target: "B"},
		},
	}

	result := AnalyzeOrphans(g)
	if result.TotalNodes != 3 {
		t.Errorf("expected 3 nodes, got %d", result.TotalNodes)
	}
	if result.TotalEdges != 1 {
		t.Errorf("expected 1 edge, got %d", result.TotalEdges)
	}
	if len(result.OrphanNodeIDs) != 1 || result.OrphanNodeIDs[0] != "C" {
		t.Errorf("expected orphan [C], got %v", result.OrphanNodeIDs)
	}
	if result.DisconnectedSubgraphs != 2 {
		t.Errorf("expected 2 components, got %d", result.DisconnectedSubgraphs)
	}
}

func TestAnalyzeOrphans_NilGraph(t *testing.T) {
	result := AnalyzeOrphans(nil)
	if result.TotalNodes != 0 {
		t.Errorf("expected 0 nodes for nil, got %d", result.TotalNodes)
	}
	if result.OrphanNodeIDs != nil {
		t.Errorf("expected nil orphans for nil graph, got %v", result.OrphanNodeIDs)
	}
}

func TestFormatOrphansOutput(t *testing.T) {
	result := OrphanResult{
		OrphanNodeIDs:         []string{"C", "D"},
		DisconnectedSubgraphs: 3,
		TotalNodes:            5,
		TotalEdges:            2,
	}

	// With showSubgraphs=true
	output := FormatOrphansOutput(result, true)
	if !strings.Contains(output, "5 nodes") {
		t.Errorf("output should mention 5 nodes")
	}
	if !strings.Contains(output, "2 edges") {
		t.Errorf("output should mention 2 edges")
	}
	if !strings.Contains(output, "Orphan nodes") {
		t.Errorf("output should mention orphan nodes")
	}
	if !strings.Contains(output, "C") || !strings.Contains(output, "D") {
		t.Errorf("output should list orphan IDs")
	}
	if !strings.Contains(output, "3") {
		t.Errorf("output should mention disconnected subgraphs count")
	}

	// With showSubgraphs=false
	output2 := FormatOrphansOutput(result, false)
	if strings.Contains(output2, "Disconnected subgraphs") {
		t.Errorf("output should NOT mention disconnected subgraphs when showSubgraphs=false, got: %s", output2)
	}
}

func TestFormatOrphansOutput_NoOrphans(t *testing.T) {
	result := OrphanResult{
		OrphanNodeIDs:         []string{},
		DisconnectedSubgraphs: 1,
		TotalNodes:            3,
		TotalEdges:            2,
	}

	output := FormatOrphansOutput(result, false)
	if !strings.Contains(output, "No orphan nodes") {
		t.Errorf("output should state no orphan nodes, got: %s", output)
	}
}

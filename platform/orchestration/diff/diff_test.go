package diff

import (
	"sort"
	"testing"
)

func TestDiffTopologies(t *testing.T) {
	old := Topology{
		Nodes: []Node{
			{ID: "node1", Confidence: 0.9},
			{ID: "node2", Confidence: 0.5},
			{ID: "node3", Confidence: 0.7},
		},
		Edges: []Edge{
			{ID: "edge1", Source: "node1", Target: "node2", Confidence: 0.8},
			{ID: "edge2", Source: "node2", Target: "node3", Confidence: 0.6},
		},
	}
	new := Topology{
		Nodes: []Node{
			{ID: "node1", Confidence: 0.95}, // changed
			{ID: "node3", Confidence: 0.7},  // unchanged confidence
			{ID: "node4", Confidence: 0.3},  // added
		},
		Edges: []Edge{
			{ID: "edge1", Source: "node1", Target: "node2", Confidence: 0.9}, // changed
			{ID: "edge3", Source: "node1", Target: "node4", Confidence: 0.4}, // added
		},
	}

	result := DiffTopologies(old, new)

	// --- Node assertions ---
	expectedNodes := []NodeDiff{
		{ID: "node1", ChangeType: Changed, OldConfidence: 0.9, NewConfidence: 0.95},
		{ID: "node2", ChangeType: Removed, OldConfidence: 0.5},
		{ID: "node4", ChangeType: Added, NewConfidence: 0.3},
	}

	if len(result.NodeDiffs) != len(expectedNodes) {
		t.Errorf("expected %d node diffs, got %d", len(expectedNodes), len(result.NodeDiffs))
	}

	// Sort to make comparison order-independent
	sort.Slice(result.NodeDiffs, func(i, j int) bool {
		return result.NodeDiffs[i].ID < result.NodeDiffs[j].ID
	})
	sort.Slice(expectedNodes, func(i, j int) bool {
		return expectedNodes[i].ID < expectedNodes[j].ID
	})

	for i := range expectedNodes {
		if i >= len(result.NodeDiffs) {
			t.Errorf("missing node diff: %+v", expectedNodes[i])
			continue
		}
		a := result.NodeDiffs[i]
		e := expectedNodes[i]
		if a.ID != e.ID || a.ChangeType != e.ChangeType ||
			a.OldConfidence != e.OldConfidence || a.NewConfidence != e.NewConfidence {
			t.Errorf("node diff mismatch at %d: got %+v, want %+v", i, a, e)
		}
	}

	// --- Edge assertions ---
	expectedEdges := []EdgeDiff{
		{ID: "edge1", ChangeType: Changed, OldConfidence: 0.8, NewConfidence: 0.9},
		{ID: "edge2", ChangeType: Removed, OldConfidence: 0.6},
		{ID: "edge3", ChangeType: Added, NewConfidence: 0.4},
	}

	if len(result.EdgeDiffs) != len(expectedEdges) {
		t.Errorf("expected %d edge diffs, got %d", len(expectedEdges), len(result.EdgeDiffs))
	}

	sort.Slice(result.EdgeDiffs, func(i, j int) bool {
		return result.EdgeDiffs[i].ID < result.EdgeDiffs[j].ID
	})
	sort.Slice(expectedEdges, func(i, j int) bool {
		return expectedEdges[i].ID < expectedEdges[j].ID
	})

	for i := range expectedEdges {
		if i >= len(result.EdgeDiffs) {
			t.Errorf("missing edge diff: %+v", expectedEdges[i])
			continue
		}
		a := result.EdgeDiffs[i]
		e := expectedEdges[i]
		if a.ID != e.ID || a.ChangeType != e.ChangeType ||
			a.OldConfidence != e.OldConfidence || a.NewConfidence != e.NewConfidence {
			t.Errorf("edge diff mismatch at %d: got %+v, want %+v", i, a, e)
		}
	}

	// --- Edge case: empty topologies ---
	emptyResult := DiffTopologies(Topology{}, Topology{})
	if len(emptyResult.NodeDiffs) != 0 || len(emptyResult.EdgeDiffs) != 0 {
		t.Error("expected empty diffs for empty topologies")
	}
}

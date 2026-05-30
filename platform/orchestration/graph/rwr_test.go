package graph

import (
	"math"
	"testing"
)

func assertRWRProb(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-6 {
		t.Fatalf("expected probability %v, got %v", want, got)
	}
}

// totalProb returns the sum of probabilities in the result map.
func totalProb(m map[string]float64) float64 {
	var sum float64
	for _, v := range m {
		sum += v
	}
	return sum
}

func TestRandomWalkWithRestart_Line3_SeedAtEnd(t *testing.T) {
	// 3-node line: n1 -- n2 -- n3, seed at n1 (one end).
	g := &TopologyGraph{
		Nodes: []GraphNode{
			{ID: "n1"},
			{ID: "n2"},
			{ID: "n3"},
		},
		Edges: []GraphEdge{
			{Source: "n1", Target: "n2", Weight: 1},
			{Source: "n2", Target: "n3", Weight: 1},
		},
	}

	result, err := RandomWalkWithRestart(g, "n1", 0.15, 100, 1e-8)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// In a 3-node line with seed at a leaf (n1), the middle node (n2) acts as a
	// hub and accumulates the highest steady-state probability. The leaf (n3)
	// farthest from the seed gets the least.
	if result["n2"] <= result["n1"] || result["n1"] <= result["n3"] {
		t.Fatalf("expected n2 > n1 > n3 (hub > seed > far leaf), got n1=%v n2=%v n3=%v",
			result["n1"], result["n2"], result["n3"])
	}

	// Total probability should be approximately 1.
	if sum := totalProb(result); math.Abs(sum-1.0) > 0.01 {
		t.Fatalf("expected total probability ~1.0, got %v", sum)
	}

	// All three nodes should have non-zero probability.
	for _, id := range []string{"n1", "n2", "n3"} {
		if _, ok := result[id]; !ok {
			t.Fatalf("expected node %s to be in result", id)
		}
	}
}

func TestRandomWalkWithRestart_Cycle4(t *testing.T) {
	// 4-node cycle: n1 -- n2 -- n3 -- n4 -- n1.
	g := &TopologyGraph{
		Nodes: []GraphNode{
			{ID: "n1"},
			{ID: "n2"},
			{ID: "n3"},
			{ID: "n4"},
		},
		Edges: []GraphEdge{
			{Source: "n1", Target: "n2", Weight: 1},
			{Source: "n2", Target: "n3", Weight: 1},
			{Source: "n3", Target: "n4", Weight: 1},
			{Source: "n4", Target: "n1", Weight: 1},
		},
	}

	result, err := RandomWalkWithRestart(g, "n1", 0.15, 100, 1e-8)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// In a symmetric 4-cycle with seed at n1, symmetry dictates:
	// n2 and n4 should have equal probability; n3 should be lowest.
	if math.Abs(result["n2"]-result["n4"]) > 1e-6 {
		t.Fatalf("expected n2 == n4 in symmetric cycle, got n2=%v n4=%v",
			result["n2"], result["n4"])
	}

	if result["n1"] <= result["n2"] {
		t.Fatalf("expected seed n1 > neighbours, got n1=%v n2=%v n3=%v n4=%v",
			result["n1"], result["n2"], result["n3"], result["n4"])
	}

	if result["n2"] <= result["n3"] {
		t.Fatalf("expected neighbours closer to seed than opposite, got n1=%v n2=%v n3=%v n4=%v",
			result["n1"], result["n2"], result["n3"], result["n4"])
	}

	// Total probability ~1.
	if sum := totalProb(result); math.Abs(sum-1.0) > 0.01 {
		t.Fatalf("expected total probability ~1.0, got %v", sum)
	}
}

func TestRandomWalkWithRestart_NilGraph(t *testing.T) {
	_, err := RandomWalkWithRestart(nil, "n1", 0.15, 100, 1e-8)
	if err == nil {
		t.Fatal("expected error for nil graph")
	}
}

func TestRandomWalkWithRestart_SeedNotFound(t *testing.T) {
	g := &TopologyGraph{
		Nodes: []GraphNode{{ID: "n1"}, {ID: "n2"}},
		Edges: []GraphEdge{{Source: "n1", Target: "n2"}},
	}
	_, err := RandomWalkWithRestart(g, "nobody", 0.15, 100, 1e-8)
	if err == nil {
		t.Fatal("expected error for missing seed node")
	}
}

func TestRandomWalkWithRestart_InvalidParams(t *testing.T) {
	g := &TopologyGraph{
		Nodes: []GraphNode{{ID: "n1"}},
	}

	// Negative restart probability.
	_, err := RandomWalkWithRestart(g, "n1", -0.1, 100, 1e-8)
	if err == nil {
		t.Fatal("expected error for negative restart probability")
	}

	// Restart probability > 1.
	_, err = RandomWalkWithRestart(g, "n1", 1.5, 100, 1e-8)
	if err == nil {
		t.Fatal("expected error for restart probability > 1")
	}

	// Non-positive maxIterations.
	_, err = RandomWalkWithRestart(g, "n1", 0.15, 0, 1e-8)
	if err == nil {
		t.Fatal("expected error for zero maxIterations")
	}

	// Negative tolerance.
	_, err = RandomWalkWithRestart(g, "n1", 0.15, 100, -1)
	if err == nil {
		t.Fatal("expected error for negative tolerance")
	}
}

func TestRandomWalkWithRestart_ConvergesToSteadyState(t *testing.T) {
	// Single node with self-loop — trivial steady state.
	g := &TopologyGraph{
		Nodes: []GraphNode{{ID: "n1"}},
		Edges: []GraphEdge{{Source: "n1", Target: "n1", Weight: 1}},
	}

	result, err := RandomWalkWithRestart(g, "n1", 0.5, 100, 1e-8)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertRWRProb(t, result["n1"], 1.0)
}

func TestRandomWalkWithRestart_IsolatedNode(t *testing.T) {
	// Graph with isolated node — seed is isolated.
	g := &TopologyGraph{
		Nodes: []GraphNode{{ID: "n1"}, {ID: "n2"}},
		Edges: []GraphEdge{},
	}

	result, err := RandomWalkWithRestart(g, "n1", 0.15, 100, 1e-8)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Seed should have p=1, other nodes 0 due to restart bringing it all back.
	assertRWRProb(t, result["n1"], 1.0)

	// n2 should not be in the result (zero probability).
	if _, ok := result["n2"]; ok {
		t.Fatal("expected n2 to have zero probability (isolated from seed)")
	}
}

func TestRandomWalkWithRestart_EdgeWeightInfluence(t *testing.T) {
	// Weighted edges bias the walk: n1 --(w=10)-- n2 --(w=1)-- n3.
	// The heavy n1-n2 edge creates strong bidirectional flow, and with restart
	// pulling back to seed n1, the seed should dominate.
	g := &TopologyGraph{
		Nodes: []GraphNode{
			{ID: "n1"},
			{ID: "n2"},
			{ID: "n3"},
		},
		Edges: []GraphEdge{
			{Source: "n1", Target: "n2", Weight: 10},
			{Source: "n2", Target: "n3", Weight: 1},
		},
	}

	// Compare with the unweighted version.
	gUnweighted := &TopologyGraph{
		Nodes: []GraphNode{
			{ID: "n1"},
			{ID: "n2"},
			{ID: "n3"},
		},
		Edges: []GraphEdge{
			{Source: "n1", Target: "n2", Weight: 1},
			{Source: "n2", Target: "n3", Weight: 1},
		},
	}

	w, _ := RandomWalkWithRestart(g, "n1", 0.15, 100, 1e-8)
	u, _ := RandomWalkWithRestart(gUnweighted, "n1", 0.15, 100, 1e-8)

	// Weighted: seed n1 dominates due to strong n1-n2 flow + restart.
	// Unweighted: middle node n2 is the hub and dominates.
	if w["n1"] <= w["n2"] {
		t.Fatalf("weighted: expected n1 > n2 (strong edge + restart), got n1=%v n2=%v n3=%v",
			w["n1"], w["n2"], w["n3"])
	}
	if w["n2"] <= w["n3"] {
		t.Fatalf("weighted: expected n2 > n3, got n1=%v n2=%v n3=%v",
			w["n1"], w["n2"], w["n3"])
	}

	// Unweighted: hub n2 dominates.
	if u["n2"] <= u["n1"] || u["n1"] <= u["n3"] {
		t.Fatalf("unweighted: expected n2 > n1 > n3, got n1=%v n2=%v n3=%v",
			u["n1"], u["n2"], u["n3"])
	}

	// The heavy edge should make n1's probability higher than in the unweighted case.
	if w["n1"] <= u["n1"] {
		t.Fatalf("weighted seed probability should exceed unweighted: w=%v u=%v", w["n1"], u["n1"])
	}
}

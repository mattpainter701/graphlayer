package graph

import (
	"math"
	"testing"
)

func assertRankClose(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-4 {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func totalRank(m map[string]float64) float64 {
	var sum float64
	for _, v := range m {
		sum += v
	}
	return sum
}

func TestPageRank_SimpleLine(t *testing.T) {
	// 3-node directed line: n1 -> n2 -> n3.
	// With damping=0.85, PageRank decreases along the chain.
	g := &TopologyGraph{
		Nodes: map[string]*Node{
			"n1": {ID: "n1"},
			"n2": {ID: "n2"},
			"n3": {ID: "n3"},
		},
		Edges: map[string]map[string]*Edge{
			"n1": {"n2": {Source: "n1", Target: "n2", Weight: 1}},
			"n2": {"n3": {Source: "n2", Target: "n3", Weight: 1}},
		},
	}

	pr, err := PageRank(g, 0.85, 100, 1e-8)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// n1 (no incoming) should have lower rank than n2, n3 due to the dangling
	// node treatment. n3 has the highest rank because it accumulates from n2.
	if pr["n1"] >= pr["n2"] {
		t.Fatalf("expected n2 > n1 (accumulates rank), got n1=%v n2=%v n3=%v", pr["n1"], pr["n2"], pr["n3"])
	}

	// Sum should be ~1.
	if sum := totalRank(pr); math.Abs(sum-1.0) > 0.01 {
		t.Fatalf("expected total rank ~1.0, got %v", sum)
	}
}

func TestPageRank_Cycle(t *testing.T) {
	// 3-node directed cycle: n1 -> n2 -> n3 -> n1.
	// In a symmetric cycle, all nodes should have equal rank.
	g := &TopologyGraph{
		Nodes: map[string]*Node{
			"n1": {ID: "n1"},
			"n2": {ID: "n2"},
			"n3": {ID: "n3"},
		},
		Edges: map[string]map[string]*Edge{
			"n1": {"n2": {Source: "n1", Target: "n2", Weight: 1}},
			"n2": {"n3": {Source: "n2", Target: "n3", Weight: 1}},
			"n3": {"n1": {Source: "n3", Target: "n1", Weight: 1}},
		},
	}

	pr, err := PageRank(g, 0.85, 100, 1e-8)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// All nodes should have approximately equal rank in a symmetric cycle.
	expected := 1.0 / 3.0
	for _, id := range []string{"n1", "n2", "n3"} {
		if math.Abs(pr[id]-expected) > 0.01 {
			t.Fatalf("expected %s rank ~%.4f, got %v", id, expected, pr[id])
		}
	}
}

func TestPageRank_DampingFactorZero(t *testing.T) {
	g := &TopologyGraph{
		Nodes: map[string]*Node{
			"n1": {ID: "n1"},
			"n2": {ID: "n2"},
		},
		Edges: map[string]map[string]*Edge{
			"n1": {"n2": {Source: "n1", Target: "n2", Weight: 1}},
		},
	}

	// With damping=0, all nodes get uniform rank from teleportation.
	pr, err := PageRank(g, 0, 100, 1e-8)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// With damping=0, every node should get rank = 1/n from teleportation.
	expected := 0.5
	for _, id := range []string{"n1", "n2"} {
		assertRankClose(t, pr[id], expected)
	}
}

func TestPageRank_NilGraph(t *testing.T) {
	_, err := PageRank(nil, 0.85, 100, 1e-8)
	if err == nil {
		t.Fatal("expected error for nil graph")
	}
}

func TestPageRank_InvalidParams(t *testing.T) {
	g := &TopologyGraph{
		Nodes: map[string]*Node{"n1": {ID: "n1"}},
	}

	// Damping >= 1.
	_, err := PageRank(g, 1.0, 100, 1e-8)
	if err == nil {
		t.Fatal("expected error for dampingFactor >= 1")
	}

	// Negative damping.
	_, err = PageRank(g, -0.1, 100, 1e-8)
	if err == nil {
		t.Fatal("expected error for negative dampingFactor")
	}

	// Non-positive maxIterations.
	_, err = PageRank(g, 0.85, 0, 1e-8)
	if err == nil {
		t.Fatal("expected error for zero maxIterations")
	}

	// Negative tolerance.
	_, err = PageRank(g, 0.85, 100, -1)
	if err == nil {
		t.Fatal("expected error for negative tolerance")
	}
}

func TestPageRank_DanglingNodes(t *testing.T) {
	// Dangling node (no outgoing edges) should distribute its rank uniformly.
	g := &TopologyGraph{
		Nodes: map[string]*Node{
			"n1": {ID: "n1"},
			"n2": {ID: "n2"},
		},
		Edges: map[string]map[string]*Edge{
			"n1": {"n2": {Source: "n1", Target: "n2", Weight: 1}},
		},
	}

	pr, err := PageRank(g, 0.85, 100, 1e-8)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// n2 is dangling (no outgoing), so its rank gets redistributed.
	// Both nodes should have non-zero rank and sum to ~1.
	if pr["n1"] <= 0 || pr["n2"] <= 0 {
		t.Fatalf("expected both nodes to have positive rank, got n1=%v n2=%v", pr["n1"], pr["n2"])
	}
	if sum := totalRank(pr); math.Abs(sum-1.0) > 0.01 {
		t.Fatalf("expected total rank ~1.0, got %v", sum)
	}
}

func TestPageRank_EmptyGraph(t *testing.T) {
	_, err := PageRank(&TopologyGraph{}, 0.85, 100, 1e-8)
	if err == nil {
		t.Fatal("expected error for empty graph")
	}
}

func TestEigenvectorCentrality_StarGraph(t *testing.T) {
	// Star graph: center connected to 4 leaves.
	// Center should have the highest eigenvector centrality, leaves equal.
	g := &TopologyGraph{
		Nodes: map[string]*Node{
			"center": {ID: "center"},
			"L1":     {ID: "L1"},
			"L2":     {ID: "L2"},
			"L3":     {ID: "L3"},
			"L4":     {ID: "L4"},
		},
		Edges: map[string]map[string]*Edge{
			"center": {
				"L1": {Source: "center", Target: "L1", Weight: 1},
				"L2": {Source: "center", Target: "L2", Weight: 1},
				"L3": {Source: "center", Target: "L3", Weight: 1},
				"L4": {Source: "center", Target: "L4", Weight: 1},
			},
			"L1": {"center": {Source: "L1", Target: "center", Weight: 1}},
			"L2": {"center": {Source: "L2", Target: "center", Weight: 1}},
			"L3": {"center": {Source: "L3", Target: "center", Weight: 1}},
			"L4": {"center": {Source: "L4", Target: "center", Weight: 1}},
		},
	}

	ec, err := EigenvectorCentrality(g, 100, 1e-8)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Center should have highest centrality.
	if ec["center"] <= ec["L1"] {
		t.Fatalf("expected center > leaves, got center=%v L1=%v L2=%v L3=%v L4=%v",
			ec["center"], ec["L1"], ec["L2"], ec["L3"], ec["L4"])
	}

	// All leaves should have equal centrality.
	if math.Abs(ec["L1"]-ec["L2"]) > 1e-6 || math.Abs(ec["L1"]-ec["L3"]) > 1e-6 || math.Abs(ec["L1"]-ec["L4"]) > 1e-6 {
		t.Fatalf("expected all leaves equal, got L1=%v L2=%v L3=%v L4=%v",
			ec["L1"], ec["L2"], ec["L3"], ec["L4"])
	}
}

func TestEigenvectorCentrality_LineGraph(t *testing.T) {
	// Undirected line: n1 -- n2 -- n3.
	// Middle node (n2) should have highest eigenvector centrality.
	g := &TopologyGraph{
		Nodes: map[string]*Node{
			"n1": {ID: "n1"},
			"n2": {ID: "n2"},
			"n3": {ID: "n3"},
		},
		Edges: map[string]map[string]*Edge{
			"n1": {"n2": {Source: "n1", Target: "n2", Weight: 1}},
			"n2": {"n1": {Source: "n2", Target: "n1", Weight: 1}, "n3": {Source: "n2", Target: "n3", Weight: 1}},
			"n3": {"n2": {Source: "n3", Target: "n2", Weight: 1}},
		},
	}

	ec, err := EigenvectorCentrality(g, 100, 1e-8)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Middle node should have highest centrality.
	if ec["n2"] <= ec["n1"] || ec["n2"] <= ec["n3"] {
		t.Fatalf("expected middle node n2 to have highest centrality, got n1=%v n2=%v n3=%v",
			ec["n1"], ec["n2"], ec["n3"])
	}

	// End nodes should have equal centrality.
	if math.Abs(ec["n1"]-ec["n3"]) > 1e-6 {
		t.Fatalf("expected n1 == n3 (symmetric ends), got n1=%v n3=%v", ec["n1"], ec["n3"])
	}
}

func TestEigenvectorCentrality_IsolatedNodes(t *testing.T) {
	// Two isolated nodes — no edges.
	g := &TopologyGraph{
		Nodes: map[string]*Node{
			"n1": {ID: "n1"},
			"n2": {ID: "n2"},
		},
	}

	ec, err := EigenvectorCentrality(g, 100, 1e-8)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Isolated nodes should both get zero eigenvector centrality.
	if ec["n1"] != 0 || ec["n2"] != 0 {
		t.Fatalf("expected isolated nodes to have 0 centrality, got n1=%v n2=%v", ec["n1"], ec["n2"])
	}
}

func TestEigenvectorCentrality_NilGraph(t *testing.T) {
	_, err := EigenvectorCentrality(nil, 100, 1e-8)
	if err == nil {
		t.Fatal("expected error for nil graph")
	}
}

func TestEigenvectorCentrality_InvalidParams(t *testing.T) {
	g := &TopologyGraph{
		Nodes: map[string]*Node{"n1": {ID: "n1"}},
	}

	// Non-positive maxIterations.
	_, err := EigenvectorCentrality(g, 0, 1e-8)
	if err == nil {
		t.Fatal("expected error for zero maxIterations")
	}

	// Negative tolerance.
	_, err = EigenvectorCentrality(g, 100, -1)
	if err == nil {
		t.Fatal("expected error for negative tolerance")
	}
}

func TestEigenvectorCentrality_EmptyGraph(t *testing.T) {
	_, err := EigenvectorCentrality(&TopologyGraph{}, 100, 1e-8)
	if err == nil {
		t.Fatal("expected error for empty graph")
	}
}

func TestEigenvectorCentrality_Weighted(t *testing.T) {
	// Weighted graph: n1 --(w=10)-- n2 --(w=1)-- n3.
	// The heavy edge makes n1 and n2 more central than n3.
	g := &TopologyGraph{
		Nodes: map[string]*Node{
			"n1": {ID: "n1"},
			"n2": {ID: "n2"},
			"n3": {ID: "n3"},
		},
		Edges: map[string]map[string]*Edge{
			"n1": {"n2": {Source: "n1", Target: "n2", Weight: 10}},
			"n2": {"n1": {Source: "n2", Target: "n1", Weight: 10}, "n3": {Source: "n2", Target: "n3", Weight: 1}},
			"n3": {"n2": {Source: "n3", Target: "n2", Weight: 1}},
		},
	}

	ec, err := EigenvectorCentrality(g, 100, 1e-8)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// n1 and n2 connected via heavy weight should both have higher centrality than n3.
	if ec["n3"] >= ec["n1"] || ec["n3"] >= ec["n2"] {
		t.Fatalf("expected n3 to have lowest centrality (weak edge), got n1=%v n2=%v n3=%v",
			ec["n1"], ec["n2"], ec["n3"])
	}
}

func TestPageRank_Method(t *testing.T) {
	g := &TopologyGraph{
		Nodes: map[string]*Node{"n1": {ID: "n1"}},
	}
	pr, err := g.PageRank(0.85, 100, 1e-8)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertRankClose(t, pr["n1"], 1.0)
}

func TestEigenvectorCentrality_Method(t *testing.T) {
	g := &TopologyGraph{
		Nodes: map[string]*Node{"n1": {ID: "n1"}, "n2": {ID: "n2"}},
		Edges: map[string]map[string]*Edge{
			"n1": {"n2": {Source: "n1", Target: "n2", Weight: 1}},
			"n2": {"n1": {Source: "n2", Target: "n1", Weight: 1}},
		},
	}
	ec, err := g.EigenvectorCentrality(100, 1e-8)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Two-node symmetric graph: both nodes should have equal centrality.
	if math.Abs(ec["n1"]-ec["n2"]) > 1e-6 {
		t.Fatalf("expected equal centrality in symmetric 2-node graph, got n1=%v n2=%v", ec["n1"], ec["n2"])
	}
}

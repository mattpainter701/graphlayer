package graph

import (
	"reflect"
	"testing"
)

// newNode creates a Node with the given ID.
func newNode(id string) *Node {
	return &Node{ID: id, Name: id}
}

// addEdge is a test helper that adds a directed edge to the graph.
func addEdge(g *TopologyGraph, src, tgt string) {
	if g.Edges[src] == nil {
		g.Edges[src] = make(map[string]*Edge)
	}
	g.Edges[src][tgt] = &Edge{Source: src, Target: tgt, Weight: 1.0}
}

// buildStarGraph creates a directed star graph:
//   center "C" has outgoing edges to leaves "L1", "L2", "L3"
//   (star with center out-degree = 3, each leaf in-degree = 1)
func buildStarGraph() *TopologyGraph {
	g := &TopologyGraph{
		Nodes: make(map[string]*Node),
		Edges: make(map[string]map[string]*Edge),
	}
	g.Nodes["C"] = newNode("C")
	g.Nodes["L1"] = newNode("L1")
	g.Nodes["L2"] = newNode("L2")
	g.Nodes["L3"] = newNode("L3")
	addEdge(g, "C", "L1")
	addEdge(g, "C", "L2")
	addEdge(g, "C", "L3")
	return g
}

// buildLineGraph creates a directed line graph:
//   A -> B -> C -> D
//   (each node except ends has one incoming and one outgoing edge)
func buildLineGraph() *TopologyGraph {
	g := &TopologyGraph{
		Nodes: make(map[string]*Node),
		Edges: make(map[string]map[string]*Edge),
	}
	g.Nodes["A"] = newNode("A")
	g.Nodes["B"] = newNode("B")
	g.Nodes["C"] = newNode("C")
	g.Nodes["D"] = newNode("D")
	addEdge(g, "A", "B")
	addEdge(g, "B", "C")
	addEdge(g, "C", "D")
	return g
}

// buildDisconnectedGraph creates a graph with isolated nodes.
func buildDisconnectedGraph() *TopologyGraph {
	g := &TopologyGraph{
		Nodes: make(map[string]*Node),
		Edges: make(map[string]map[string]*Edge),
	}
	g.Nodes["X"] = newNode("X")
	g.Nodes["Y"] = newNode("Y")
	// No edges — both nodes isolated.
	return g
}

// --- Star graph tests ---

func TestNodeDegreeStats_StarGraph_Min(t *testing.T) {
	g := buildStarGraph()
	min, _, _, _ := NodeDegreeStats(g)
	if min != 1.0 {
		t.Errorf("star graph min degree = %v, want 1.0 (leaves have degree 1)", min)
	}
}

func TestNodeDegreeStats_StarGraph_Max(t *testing.T) {
	g := buildStarGraph()
	_, max, _, _ := NodeDegreeStats(g)
	if max != 3.0 {
		t.Errorf("star graph max degree = %v, want 3.0 (center has degree 3)", max)
	}
}

func TestNodeDegreeStats_StarGraph_Avg(t *testing.T) {
	g := buildStarGraph()
	_, _, avg, _ := NodeDegreeStats(g)
	// Center=3, three leaves=1 each => (3+1+1+1)/4 = 1.5
	want := 1.5
	if avg != want {
		t.Errorf("star graph avg degree = %v, want %v", avg, want)
	}
}

func TestNodeDegreeStats_StarGraph_Histogram(t *testing.T) {
	g := buildStarGraph()
	_, _, _, hist := NodeDegreeStats(g)
	want := map[int]int{1: 3, 3: 1}
	if !reflect.DeepEqual(hist, want) {
		t.Errorf("star graph histogram = %v, want %v", hist, want)
	}
}

// --- Line graph tests ---

func TestNodeDegreeStats_LineGraph_Min(t *testing.T) {
	g := buildLineGraph()
	min, _, _, _ := NodeDegreeStats(g)
	if min != 1.0 {
		t.Errorf("line graph min degree = %v, want 1.0 (ends have degree 1)", min)
	}
}

func TestNodeDegreeStats_LineGraph_Max(t *testing.T) {
	g := buildLineGraph()
	_, max, _, _ := NodeDegreeStats(g)
	if max != 2.0 {
		t.Errorf("line graph max degree = %v, want 2.0 (middle nodes have degree 2)", max)
	}
}

func TestNodeDegreeStats_LineGraph_Avg(t *testing.T) {
	g := buildLineGraph()
	_, _, avg, _ := NodeDegreeStats(g)
	// A=1, B=2, C=2, D=1 => (1+2+2+1)/4 = 1.5
	want := 1.5
	if avg != want {
		t.Errorf("line graph avg degree = %v, want %v", avg, want)
	}
}

func TestNodeDegreeStats_LineGraph_Histogram(t *testing.T) {
	g := buildLineGraph()
	_, _, _, hist := NodeDegreeStats(g)
	want := map[int]int{1: 2, 2: 2}
	if !reflect.DeepEqual(hist, want) {
		t.Errorf("line graph histogram = %v, want %v", hist, want)
	}
}

// --- Edge cases ---

func TestNodeDegreeStats_NilGraph(t *testing.T) {
	min, max, avg, hist := NodeDegreeStats(nil)
	if min != 0 || max != 0 || avg != 0 {
		t.Errorf("nil graph: expected 0,0,0 got %v,%v,%v", min, max, avg)
	}
	if len(hist) != 0 {
		t.Errorf("nil graph: expected empty histogram, got %v", hist)
	}
}

func TestNodeDegreeStats_EmptyGraph(t *testing.T) {
	g := &TopologyGraph{
		Nodes: make(map[string]*Node),
		Edges: make(map[string]map[string]*Edge),
	}
	min, max, avg, hist := NodeDegreeStats(g)
	if min != 0 || max != 0 || avg != 0 {
		t.Errorf("empty graph: expected 0,0,0 got %v,%v,%v", min, max, avg)
	}
	if len(hist) != 0 {
		t.Errorf("empty graph: expected empty histogram, got %v", hist)
	}
}

func TestNodeDegreeStats_DisconnectedGraph(t *testing.T) {
	g := buildDisconnectedGraph()
	min, max, avg, hist := NodeDegreeStats(g)
	if min != 0 || max != 0 || avg != 0 {
		t.Errorf("disconnected graph: expected 0,0,0 got %v,%v,%v", min, max, avg)
	}
	want := map[int]int{0: 2}
	if !reflect.DeepEqual(hist, want) {
		t.Errorf("disconnected graph histogram = %v, want %v", hist, want)
	}
}

func TestNodeDegreeStats_BidirectionalEdge(t *testing.T) {
	// A <-> B (two directed edges, one each way)
	g := &TopologyGraph{
		Nodes: make(map[string]*Node),
		Edges: make(map[string]map[string]*Edge),
	}
	g.Nodes["A"] = newNode("A")
	g.Nodes["B"] = newNode("B")
	addEdge(g, "A", "B")
	addEdge(g, "B", "A")

	min, max, avg, hist := NodeDegreeStats(g)
	if min != 2.0 || max != 2.0 || avg != 2.0 {
		t.Errorf("bidirectional: expected min=2 max=2 avg=2 got %v,%v,%v", min, max, avg)
	}
	want := map[int]int{2: 2}
	if !reflect.DeepEqual(hist, want) {
		t.Errorf("bidirectional histogram = %v, want %v", hist, want)
	}
}

func TestNodeDegreeStats_SelfLoopIsSkipped(t *testing.T) {
	// A -> A (self-loop) — the graph adjacency allows self-loops but computeDegrees
	// still counts them. We verify the behavior.
	g := &TopologyGraph{
		Nodes: make(map[string]*Node),
		Edges: make(map[string]map[string]*Edge),
	}
	g.Nodes["A"] = newNode("A")
	addEdge(g, "A", "A")

	min, max, avg, hist := NodeDegreeStats(g)
	// Self-loop: out=1 (A->A), in=1 (A<-A), total=2
	if min != 2.0 || max != 2.0 || avg != 2.0 {
		t.Errorf("self-loop: expected 2,2,2 got %v,%v,%v", min, max, avg)
	}
	if hist[2] != 1 {
		t.Errorf("self-loop histogram: expected {2:1}, got %v", hist)
	}
}

func TestNodeDegreeStats_SingleNodeNoEdges(t *testing.T) {
	g := &TopologyGraph{
		Nodes: make(map[string]*Node),
		Edges: make(map[string]map[string]*Edge),
	}
	g.Nodes["A"] = newNode("A")

	min, max, avg, hist := NodeDegreeStats(g)
	if min != 0 || max != 0 || avg != 0 {
		t.Errorf("single isolated node: expected 0,0,0 got %v,%v,%v", min, max, avg)
	}
	want := map[int]int{0: 1}
	if !reflect.DeepEqual(hist, want) {
		t.Errorf("single isolated node histogram = %v, want %v", hist, want)
	}
}

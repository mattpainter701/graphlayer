package graph

import (
	"testing"
)

func createTestGraph() *TopologyGraph {
	graph := &TopologyGraph{
		Nodes: make(map[string]*Node),
		Edges: make(map[string]map[string]*Edge),
	}

	// Add some test nodes
	graph.Nodes["A"] = &Node{ID: "A", Name: "Node A"}
	graph.Nodes["B"] = &Node{ID: "B", Name: "Node B"}
	graph.Nodes["C"] = &Node{ID: "C", Name: "Node C"}

	// Add some test edges
	if graph.Edges["A"] == nil {
		graph.Edges["A"] = make(map[string]*Edge)
	}
	graph.Edges["A"]["B"] = &Edge{Source: "A", Target: "B", Weight: 1.0}

	if graph.Edges["B"] == nil {
		graph.Edges["B"] = make(map[string]*Edge)
	}
	graph.Edges["B"]["C"] = &Edge{Source: "B", Target: "C", Weight: 2.0}

	return graph
}

func TestSetEdgeWeight_ValidUpdate(t *testing.T) {
	graph := createTestGraph()

	err := SetEdgeWeight(graph, "A", "B", 5.0)
	if err != nil {
		t.Fatalf("Expected no error when updating existing edge, got: %v", err)
	}

	weight, err := GetEdgeWeight(graph, "A", "B")
	if err != nil {
		t.Fatalf("Expected no error when getting updated edge weight, got: %v", err)
	}

	if weight != 5.0 {
		t.Errorf("Expected weight to be 5.0 after update, got: %f", weight)
	}
}

func TestSetEdgeWeight_NonExistentEdge(t *testing.T) {
	graph := createTestGraph()

	err := SetEdgeWeight(graph, "A", "C", 3.0)
	if err == nil {
		t.Fatal("Expected error when updating non-existent edge, got none")
	}

	expectedErrMsg := "edge does not exist: A -> C"
	if err.Error() != expectedErrMsg {
		t.Errorf("Expected error message '%s', got: %v", expectedErrMsg, err)
	}
}

func TestSetEdgeWeight_NegativeWeight(t *testing.T) {
	graph := createTestGraph()

	err := SetEdgeWeight(graph, "A", "B", -1.0)
	if err == nil {
		t.Fatal("Expected error when setting negative weight, got none")
	}

	expectedErrMsg := "edge weight cannot be negative"
	if err.Error() != expectedErrMsg {
		t.Errorf("Expected error message '%s', got: %v", expectedErrMsg, err)
	}
}

func TestGetEdgeWeight_ValidEdge(t *testing.T) {
	graph := createTestGraph()

	weight, err := GetEdgeWeight(graph, "A", "B")
	if err != nil {
		t.Fatalf("Expected no error when getting existing edge weight, got: %v", err)
	}

	if weight != 1.0 {
		t.Errorf("Expected weight to be 1.0, got: %f", weight)
	}
}

func TestGetEdgeWeight_NonExistentEdge(t *testing.T) {
	graph := createTestGraph()

	_, err := GetEdgeWeight(graph, "A", "C")
	if err == nil {
		t.Fatal("Expected error when getting non-existent edge weight, got none")
	}

	expectedErrMsg := "edge does not exist: A -> C"
	if err.Error() != expectedErrMsg {
		t.Errorf("Expected error message '%s', got: %v", expectedErrMsg, err)
	}
}

func TestGetEdgeWeight_NonExistentSourceNode(t *testing.T) {
	graph := createTestGraph()

	_, err := GetEdgeWeight(graph, "X", "A")
	if err == nil {
		t.Fatal("Expected error when getting edge from non-existent source node, got none")
	}

	expectedErrMsg := "source node does not exist: X"
	if err.Error() != expectedErrMsg {
		t.Errorf("Expected error message '%s', got: %v", expectedErrMsg, err)
	}
}

func TestGetEdgeWeight_NonExistentTargetNode(t *testing.T) {
	graph := createTestGraph()

	_, err := GetEdgeWeight(graph, "A", "X")
	if err == nil {
		t.Fatal("Expected error when getting edge to non-existent target node, got none")
	}

	expectedErrMsg := "target node does not exist: X"
	if err.Error() != expectedErrMsg {
		t.Errorf("Expected error message '%s', got: %v", expectedErrMsg, err)
	}
}

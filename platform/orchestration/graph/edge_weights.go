package graph

import (
	"errors"
	"fmt"
)

// TopologyGraph represents a network topology with weighted edges
type TopologyGraph struct {
	Nodes map[string]*Node
	Edges map[string]map[string]*Edge
}

// Node represents a network node/device
type Node struct {
	ID       string
	Name     string
	Type     string
	IP       string
	Metadata map[string]interface{}
}

// Edge represents a connection between two nodes
type Edge struct {
	Source   string
	Target   string
	Weight   float64
	Type     string
	Metadata map[string]interface{}
}

// SetEdgeWeight updates the weight of an existing directed edge
func SetEdgeWeight(graph *TopologyGraph, source, target string, weight float64) error {
	if weight < 0 {
		return errors.New("edge weight cannot be negative")
	}

	// Check if source node exists
	if _, exists := graph.Nodes[source]; !exists {
		return fmt.Errorf("source node does not exist: %s", source)
	}

	// Check if target node exists
	if _, exists := graph.Nodes[target]; !exists {
		return fmt.Errorf("target node does not exist: %s", target)
	}

	// Check if edge exists
	sourceEdges, exists := graph.Edges[source]
	if !exists {
		return fmt.Errorf("edge does not exist: %s -> %s", source, target)
	}

	edge, exists := sourceEdges[target]
	if !exists {
		return fmt.Errorf("edge does not exist: %s -> %s", source, target)
	}

	// Update the weight
	edge.Weight = weight
	return nil
}

// GetEdgeWeight returns the weight of an existing directed edge
func GetEdgeWeight(graph *TopologyGraph, source, target string) (float64, error) {
	// Check if source node exists
	if _, exists := graph.Nodes[source]; !exists {
		return 0, fmt.Errorf("source node does not exist: %s", source)
	}

	// Check if target node exists
	if _, exists := graph.Nodes[target]; !exists {
		return 0, fmt.Errorf("target node does not exist: %s", target)
	}

	// Check if edge exists
	sourceEdges, exists := graph.Edges[source]
	if !exists {
		return 0, fmt.Errorf("edge does not exist: %s -> %s", source, target)
	}

	edge, exists := sourceEdges[target]
	if !exists {
		return 0, fmt.Errorf("edge does not exist: %s -> %s", source, target)
	}

	return edge.Weight, nil
}

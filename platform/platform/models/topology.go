// Package models defines vendor-agnostic topology data structures.
package models

// Topology represents a complete network topology graph.
type Topology struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

// Node represents a device or endpoint in the topology.
type Node struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Type       string  `json:"type"`
	Confidence float64 `json:"confidence,omitempty"`
	// Additional metadata
	Labels map[string]string `json:"labels,omitempty"`
}

// Edge represents a link or relationship between two nodes.
type Edge struct {
	ID         string  `json:"id"`
	Source     string  `json:"source"`
	Target     string  `json:"target"`
	Name       string  `json:"name,omitempty"`
	Type       string  `json:"type,omitempty"`
	Confidence float64 `json:"confidence,omitempty"`
	// Additional metadata
	Labels map[string]string `json:"labels,omitempty"`
}

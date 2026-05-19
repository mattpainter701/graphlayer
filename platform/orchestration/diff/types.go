package diff

// ChangeType represents the type of change detected.
type ChangeType string

const (
	Added   ChangeType = "added"
	Removed ChangeType = "removed"
	Changed ChangeType = "changed"
)

// Node represents a node in the topology.
type Node struct {
	ID         string  `json:"id"`
	Confidence float64 `json:"confidence"`
}

// Edge represents an edge in the topology.
type Edge struct {
	ID         string  `json:"id"`
	Source     string  `json:"source"`
	Target     string  `json:"target"`
	Confidence float64 `json:"confidence"`
}

// Topology holds the graph structure.
type Topology struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

// NodeDiff captures a change to a node.
type NodeDiff struct {
	ID           string     `json:"id"`
	ChangeType   ChangeType `json:"change_type"`
	OldConfidence float64   `json:"old_confidence,omitempty"`
	NewConfidence float64   `json:"new_confidence,omitempty"`
}

// EdgeDiff captures a change to an edge.
type EdgeDiff struct {
	ID           string     `json:"id"`
	ChangeType   ChangeType `json:"change_type"`
	OldConfidence float64   `json:"old_confidence,omitempty"`
	NewConfidence float64   `json:"new_confidence,omitempty"`
}

// DiffResult contains the aggregated differences.
type DiffResult struct {
	NodeDiffs []NodeDiff `json:"node_diffs"`
	EdgeDiffs []EdgeDiff `json:"edge_diffs"`
}

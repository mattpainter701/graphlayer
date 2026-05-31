package graph

// GraphNode represents a node in a topology graph.
type GraphNode struct {
	ID         string            `json:"id" yaml:"id"`
	Attributes map[string]string `json:"attributes,omitempty" yaml:"attributes,omitempty"`
}

// GraphEdge represents a weighted edge in a topology graph.
type GraphEdge struct {
	Source     string            `json:"source" yaml:"source"`
	Target     string            `json:"target" yaml:"target"`
	Weight     float64           `json:"weight" yaml:"weight"`
	Attributes map[string]string `json:"attributes,omitempty" yaml:"attributes,omitempty"`
}

// TopologyGraph is a lightweight node/edge graph used by topology utilities.
type TopologyGraph struct {
	Nodes []GraphNode `json:"nodes" yaml:"nodes"`
	Edges []GraphEdge `json:"edges" yaml:"edges"`
}

// NodeCount returns the number of nodes in the graph.
func (g *TopologyGraph) NodeCount() int {
	if g == nil {
		return 0
	}
	return len(g.Nodes)
}

// EdgeCount returns the number of edges in the graph.
func (g *TopologyGraph) EdgeCount() int {
	if g == nil {
		return 0
	}
	return len(g.Edges)
}

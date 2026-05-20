package topology

// Node represents a topology node with attributes.
type Node struct {
    ID         string
    Type       string
    Attributes map[string]string
}

// TopologyGraph holds the topology graph structure.
type TopologyGraph struct {
    Nodes map[string]*Node
}

package orchestration

import (
	"fmt"
	"math/rand"
	"sort"

	"github.com/mattpainter701/graphlayer/platform/platform/orchestration/graph"
)

// TopologyFilter represents filters for topology queries
type TopologyFilter struct {
	Source     string  `json:"source,omitempty" yaml:"source,omitempty"`
	DeviceName string  `json:"device_name,omitempty" yaml:"device_name,omitempty"`
	Confidence float64 `json:"confidence,omitempty" yaml:"confidence,omitempty"`
}

// TopologyNode represents a node in the network topology
type TopologyNode struct {
	ID         string  `json:"id" yaml:"id"`
	Name       string  `json:"name" yaml:"name"`
	Source     string  `json:"source" yaml:"source"`
	Confidence float64 `json:"confidence" yaml:"confidence"`
	Type       string  `json:"type" yaml:"type"`
	IPAddress  string  `json:"ip_address,omitempty" yaml:"ip_address,omitempty"`
	Vendor     string  `json:"vendor,omitempty" yaml:"vendor,omitempty"`
}

// TopologyEdge represents a connection between two nodes
type TopologyEdge struct {
	SourceID   string  `json:"source_id" yaml:"source_id"`
	TargetID   string  `json:"target_id" yaml:"target_id"`
	SourceName string  `json:"source_name" yaml:"source_name"`
	TargetName string  `json:"target_name" yaml:"target_name"`
	Protocol   string  `json:"protocol" yaml:"protocol"`
	Confidence float64 `json:"confidence" yaml:"confidence"`
}

// TopologyResult represents the complete topology query result
type TopologyResult struct {
	Nodes []TopologyNode `json:"nodes" yaml:"nodes"`
	Edges []TopologyEdge `json:"edges" yaml:"edges"`
}

// OrchestrationEngine represents the main orchestration engine
type OrchestrationEngine struct{}

// NewOrchestrationEngine creates a new orchestration engine
func NewOrchestrationEngine() *OrchestrationEngine {
	return &OrchestrationEngine{}
}

// topologyGraphDefinition is a small in-memory graph registry used by the CLI and tests.
type topologyGraphDefinition struct {
	nodes []string
	edges [][2]string
}

var topologyGraphs = map[string]topologyGraphDefinition{
	"tree": {
		nodes: []string{"root", "left", "right", "left-left"},
		edges: [][2]string{
			{"root", "left"},
			{"root", "right"},
			{"left", "left-left"},
		},
	},
	"triangle": {
		nodes: []string{"A", "B", "C"},
		edges: [][2]string{
			{"A", "B"},
			{"B", "C"},
			{"C", "A"},
		},
	},
	"single": {
		nodes: []string{"solo"},
		edges: [][2]string{},
	},
	"demo": {
		nodes: []string{"node-1", "node-2", "node-3", "node-4"},
		edges: [][2]string{
			{"node-1", "node-2"},
			{"node-2", "node-3"},
			{"node-3", "node-4"},
		},
	},
	"star": {
		nodes: []string{"center", "leaf-1", "leaf-2", "leaf-3", "leaf-4"},
		edges: [][2]string{
			{"center", "leaf-1"},
			{"center", "leaf-2"},
			{"center", "leaf-3"},
			{"center", "leaf-4"},
		},
	},
	"line": {
		nodes: []string{"n1", "n2", "n3", "n4"},
		edges: [][2]string{
			{"n1", "n2"},
			{"n2", "n3"},
			{"n3", "n4"},
		},
	},
	"forest": {
		nodes: []string{"A", "B", "C", "D", "E", "F"},
		edges: [][2]string{
			{"A", "B"},
			{"B", "C"},
			{"D", "E"},
		},
	},
}

func getTopologyGraphDefinition(graphID string) (topologyGraphDefinition, error) {
	graph, ok := topologyGraphs[graphID]
	if !ok {
		return topologyGraphDefinition{}, fmt.Errorf("unknown graph %q", graphID)
	}
	return graph, nil
}

func buildTopologyAdjacency(definition topologyGraphDefinition) map[string][]string {
	adjacency := make(map[string][]string, len(definition.nodes))
	for _, nodeID := range definition.nodes {
		if _, ok := adjacency[nodeID]; !ok {
			adjacency[nodeID] = nil
		}
	}

	for _, edge := range definition.edges {
		source, target := edge[0], edge[1]
		if _, ok := adjacency[source]; !ok {
			adjacency[source] = nil
		}
		if _, ok := adjacency[target]; !ok {
			adjacency[target] = nil
		}
		adjacency[source] = append(adjacency[source], target)
		adjacency[target] = append(adjacency[target], source)
	}

	for nodeID := range adjacency {
		sort.Strings(adjacency[nodeID])
	}

	return adjacency
}

// HasCycle reports whether the graph contains a cycle and returns one cycle path if found.
func HasCycle(graphID string) (bool, []string, error) {
	definition, err := getTopologyGraphDefinition(graphID)
	if err != nil {
		return false, nil, err
	}

	adjacency := buildTopologyAdjacency(definition)
	state := make(map[string]int, len(adjacency))
	stack := make([]string, 0, len(adjacency))
	stackIndex := make(map[string]int, len(adjacency))
	var cycle []string

	var dfs func(nodeID, parentID string) bool
	dfs = func(nodeID, parentID string) bool {
		state[nodeID] = 1
		stackIndex[nodeID] = len(stack)
		stack = append(stack, nodeID)

		for _, neighborID := range adjacency[nodeID] {
			if neighborID == parentID {
				continue
			}

			switch state[neighborID] {
			case 0:
				if dfs(neighborID, nodeID) {
					return true
				}
			case 1:
				start := stackIndex[neighborID]
				cycle = append([]string{}, stack[start:]...)
				cycle = append(cycle, neighborID)
				return true
			}
		}

		stack = stack[:len(stack)-1]
		delete(stackIndex, nodeID)
		state[nodeID] = 2
		return false
	}

	nodes := make([]string, 0, len(adjacency))
	for nodeID := range adjacency {
		nodes = append(nodes, nodeID)
	}
	sort.Strings(nodes)

	for _, nodeID := range nodes {
		if state[nodeID] == 0 && dfs(nodeID, "") {
			return true, cycle, nil
		}
	}

	return false, nil, nil
}

// BetweennessCentrality computes normalized betweenness centrality for each node
// in an unweighted, undirected graph using Brandes' algorithm.
func BetweennessCentrality(graphID string) (map[string]float64, error) {
	definition, err := getTopologyGraphDefinition(graphID)
	if err != nil {
		return nil, err
	}

	adjacency := make(map[string][]string, len(definition.nodes))
	centrality := make(map[string]float64, len(definition.nodes))
	for _, nodeID := range definition.nodes {
		adjacency[nodeID] = nil
		centrality[nodeID] = 0
	}

	for _, edge := range definition.edges {
		source, target := edge[0], edge[1]
		if _, ok := adjacency[source]; !ok {
			adjacency[source] = nil
			centrality[source] = 0
		}
		if _, ok := adjacency[target]; !ok {
			adjacency[target] = nil
			centrality[target] = 0
		}
		adjacency[source] = append(adjacency[source], target)
		adjacency[target] = append(adjacency[target], source)
	}

	nodes := make([]string, 0, len(adjacency))
	for nodeID := range adjacency {
		nodes = append(nodes, nodeID)
	}
	sort.Strings(nodes)
	for _, nodeID := range nodes {
		sort.Strings(adjacency[nodeID])
	}

	if len(nodes) <= 2 {
		return centrality, nil
	}

	for _, source := range nodes {
		stack := make([]string, 0, len(nodes))
		predecessors := make(map[string][]string, len(nodes))
		sigma := make(map[string]float64, len(nodes))
		distance := make(map[string]int, len(nodes))
		for _, nodeID := range nodes {
			distance[nodeID] = -1
			sigma[nodeID] = 0
		}
		sigma[source] = 1
		distance[source] = 0

		queue := []string{source}
		for len(queue) > 0 {
			v := queue[0]
			queue = queue[1:]
			stack = append(stack, v)

			for _, neighbor := range adjacency[v] {
				if distance[neighbor] < 0 {
					distance[neighbor] = distance[v] + 1
					queue = append(queue, neighbor)
				}
				if distance[neighbor] == distance[v]+1 {
					sigma[neighbor] += sigma[v]
					predecessors[neighbor] = append(predecessors[neighbor], v)
				}
			}
		}

		delta := make(map[string]float64, len(nodes))
		for i := len(stack) - 1; i >= 0; i-- {
			w := stack[i]
			for _, v := range predecessors[w] {
				if sigma[w] == 0 {
					continue
				}
				delta[v] += (sigma[v] / sigma[w]) * (1.0 + delta[w])
			}
			if w != source {
				centrality[w] += delta[w]
			}
		}
	}

	for nodeID := range centrality {
		centrality[nodeID] /= 2.0
	}

	scale := 2.0 / float64((len(nodes)-1)*(len(nodes)-2))
	for nodeID := range centrality {
		centrality[nodeID] *= scale
		if centrality[nodeID] < 0 {
			centrality[nodeID] = 0
		}
		if centrality[nodeID] > 1 {
			centrality[nodeID] = 1
		}
	}

	return centrality, nil
}

// OrphansResult holds the result of an orphan / disconnected-component analysis.
type OrphansResult struct {
	OrphanNodeIDs         []string `json:"orphan_node_ids"`
	DisconnectedSubgraphs int      `json:"disconnected_subgraphs"`
}

// Orphans finds all nodes with zero edges (isolated nodes) and counts the
// number of disconnected subgraphs (connected components beyond the main one).
// A node is an orphan if it belongs to a connected component of size 1 — i.e.
// it has no edges to any other node.
func Orphans(graphID string) (*OrphansResult, error) {
	definition, err := getTopologyGraphDefinition(graphID)
	if err != nil {
		return nil, err
	}

	adjacency := buildTopologyAdjacency(definition)

	// Find all connected components via BFS.
	visited := make(map[string]bool, len(adjacency))
	var components [][]string

	for _, nodeID := range definition.nodes {
		if visited[nodeID] {
			continue
		}

		// BFS from nodeID
		component := []string{}
		queue := []string{nodeID}
		visited[nodeID] = true

		for len(queue) > 0 {
			cur := queue[0]
			queue = queue[1:]
			component = append(component, cur)

			for _, neighbor := range adjacency[cur] {
				if !visited[neighbor] {
					visited[neighbor] = true
					queue = append(queue, neighbor)
				}
			}
		}

		components = append(components, component)
	}

	// Classify: size-1 components = orphans.
	// "Disconnected subgraphs" = every component with size > 1, unless the
	// whole graph is a single connected component (meaning the graph has no
	// truly disconnected parts).
	var orphanIDs []string
	disconnectedCount := 0
	for _, comp := range components {
		if len(comp) == 1 {
			orphanIDs = append(orphanIDs, comp[0])
		} else if len(components) > 1 {
			disconnectedCount++
		}
	}

	if orphanIDs == nil {
		orphanIDs = []string{}
	}
	sort.Strings(orphanIDs)

	return &OrphansResult{
		OrphanNodeIDs:         orphanIDs,
		DisconnectedSubgraphs: disconnectedCount,
	}, nil
}

// RandomWalk performs a random walk across the in-memory topology graph.
func RandomWalk(graphID, startNodeID string, steps int) ([]string, error) {
	if steps < 0 {
		return nil, fmt.Errorf("steps must be non-negative")
	}

	definition, err := getTopologyGraphDefinition(graphID)
	if err != nil {
		return nil, err
	}

	adjacency := buildTopologyAdjacency(definition)
	if _, ok := adjacency[startNodeID]; !ok {
		return nil, fmt.Errorf("unknown start node %q in graph %q", startNodeID, graphID)
	}

	path := []string{startNodeID}
	current := startNodeID
	for i := 0; i < steps; i++ {
		neighbors := adjacency[current]
		if len(neighbors) == 0 {
			break
		}

		current = neighbors[rand.Intn(len(neighbors))]
		path = append(path, current)
	}

	return path, nil
}

// QueryTopology queries the network topology with the given filter
func (e *OrchestrationEngine) QueryTopology(filter TopologyFilter) (*TopologyResult, error) {
	// For now, return mock data that respects the filter
	// In production, this would query the actual topology store
	result := &TopologyResult{
		Nodes: []TopologyNode{
			{
				ID:         "node-1",
				Name:       "router-01",
				Source:     "cisco",
				Confidence: 0.95,
				Type:       "router",
				IPAddress:  "10.0.0.1",
				Vendor:     "Cisco",
			},
			{
				ID:         "node-2",
				Name:       "switch-01",
				Source:     "cisco",
				Confidence: 0.90,
				Type:       "switch",
				IPAddress:  "10.0.0.2",
				Vendor:     "Cisco",
			},
			{
				ID:         "node-3",
				Name:       "firewall-01",
				Source:     "paloalto",
				Confidence: 0.85,
				Type:       "firewall",
				IPAddress:  "10.0.0.3",
				Vendor:     "Palo Alto",
			},
			{
				ID:         "node-4",
				Name:       "fortigate-01",
				Source:     "fortinet",
				Confidence: 0.75,
				Type:       "firewall",
				IPAddress:  "10.0.0.4",
				Vendor:     "Fortinet",
			},
		},
		Edges: []TopologyEdge{
			{
				SourceID:   "node-1",
				TargetID:   "node-2",
				SourceName: "router-01",
				TargetName: "switch-01",
				Protocol:   "Ethernet",
				Confidence: 0.95,
			},
			{
				SourceID:   "node-2",
				TargetID:   "node-3",
				SourceName: "switch-01",
				TargetName: "firewall-01",
				Protocol:   "Ethernet",
				Confidence: 0.90,
			},
			{
				SourceID:   "node-3",
				TargetID:   "node-4",
				SourceName: "firewall-01",
				TargetName: "fortigate-01",
				Protocol:   "Ethernet",
				Confidence: 0.80,
			},
		},
	}

	// Apply filters
	var filteredNodes []TopologyNode
	for _, node := range result.Nodes {
		if filter.Source != "" && node.Source != filter.Source {
			continue
		}
		if filter.DeviceName != "" && node.Name != filter.DeviceName {
			continue
		}
		if filter.Confidence > 0 && node.Confidence < filter.Confidence {
			continue
		}
		filteredNodes = append(filteredNodes, node)
	}

	// Filter edges based on filtered nodes
	nodeIDs := make(map[string]bool)
	for _, node := range filteredNodes {
		nodeIDs[node.ID] = true
	}

	var filteredEdges []TopologyEdge
	for _, edge := range result.Edges {
		if !nodeIDs[edge.SourceID] || !nodeIDs[edge.TargetID] {
			continue
		}
		if filter.Confidence > 0 && edge.Confidence < filter.Confidence {
			continue
		}
		filteredEdges = append(filteredEdges, edge)
	}

	result.Nodes = filteredNodes
	result.Edges = filteredEdges

	return result, nil
}

// Bridges finds all cut edges (bridges) in the topology graph identified by
// graphID, using Tarjan's DFS-based algorithm.
func Bridges(graphID string) ([][2]string, error) {
	definition, err := getTopologyGraphDefinition(graphID)
	if err != nil {
		return nil, err
	}

	// Convert the flat definition to a graph.TopologyGraph.
	g := &graph.TopologyGraph{
		Nodes: make(map[string]*graph.Node, len(definition.nodes)),
		Edges: make(map[string]map[string]*graph.Edge),
	}
	for _, id := range definition.nodes {
		g.Nodes[id] = &graph.Node{ID: id, Name: id}
	}
	for _, edge := range definition.edges {
		src, tgt := edge[0], edge[1]
		if g.Edges[src] == nil {
			g.Edges[src] = make(map[string]*graph.Edge)
		}
		g.Edges[src][tgt] = &graph.Edge{Source: src, Target: tgt, Weight: 1.0}
	}

	return graph.FindBridges(g), nil
}

// Helper function to generate random ID (not used in mock, but available for future use)
func generateID() string {
	return fmt.Sprintf("node-%d", rand.Intn(10000))
}

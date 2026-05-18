package orchestration

import (
	"fmt"
	"time"
)

// TopologyNode represents a network device in the topology
type TopologyNode struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	DeviceType string            `json:"device_type"`
	Source     string            `json:"source"`
	Confidence float64           `json:"confidence"`
	Properties map[string]string `json:"properties,omitempty"`
}

// TopologyLink represents a connection between two devices
type TopologyLink struct {
	SourceID      string  `json:"source_id"`
	TargetID      string  `json:"target_id"`
	LinkType      string  `json:"link_type"`
	Confidence    float64 `json:"confidence"`
	SourcePort    string  `json:"source_port,omitempty"`
	TargetPort    string  `json:"target_port,omitempty"`
}

// TopologyQuery represents the query parameters
type TopologyQuery struct {
	Source            string  `json:"source,omitempty"`
	DeviceName        string  `json:"device_name,omitempty"`
	MinConfidence     float64 `json:"min_confidence,omitempty"`
}

// TopologyResult represents the query result
type TopologyResult struct {
	Nodes      []TopologyNode `json:"nodes"`
	Links      []TopologyLink `json:"links"`
	TotalNodes int            `json:"total_nodes"`
	TotalLinks int            `json:"total_links"`
	QueryTime  string         `json:"query_time"`
}

// Engine is the orchestration engine
type Engine struct {
	nodes []TopologyNode
	links []TopologyLink
}

// NewEngine creates a new orchestration engine
func NewEngine() *Engine {
	return &Engine{
		nodes: make([]TopologyNode, 0),
		links: make([]TopologyLink, 0),
	}
}

// LoadTestTopology loads a known test topology for testing
func (e *Engine) LoadTestTopology() {
	e.nodes = []TopologyNode{
		{ID: "1", Name: "router-01", DeviceType: "router", Source: "cdp", Confidence: 0.95, Properties: map[string]string{"vendor": "cisco", "model": "ISR4451"}},
		{ID: "2", Name: "switch-01", DeviceType: "switch", Source: "lldp", Confidence: 0.90, Properties: map[string]string{"vendor": "cisco", "model": "C9300"}},
		{ID: "3", Name: "firewall-01", DeviceType: "firewall", Source: "arp", Confidence: 0.85, Properties: map[string]string{"vendor": "paloalto", "model": "PA-5250"}},
		{ID: "4", Name: "router-02", DeviceType: "router", Source: "cdp", Confidence: 0.70, Properties: map[string]string{"vendor": "cisco", "model": "ASR1001"}},
		{ID: "5", Name: "switch-02", DeviceType: "switch", Source: "lldp", Confidence: 0.60, Properties: map[string]string{"vendor": "cisco", "model": "C2960"}},
	}

	e.links = []TopologyLink{
		{SourceID: "1", TargetID: "2", LinkType: "ethernet", Confidence: 0.95, SourcePort: "Gig0/0/1", TargetPort: "Gig1/0/1"},
		{SourceID: "2", TargetID: "3", LinkType: "ethernet", Confidence: 0.90, SourcePort: "Gig1/0/2", TargetPort: "eth1/1"},
		{SourceID: "1", TargetID: "4", LinkType: "ethernet", Confidence: 0.75, SourcePort: "Gig0/0/2", TargetPort: "Gig0/0/0"},
		{SourceID: "4", TargetID: "5", LinkType: "ethernet", Confidence: 0.65, SourcePort: "Gig0/0/1", TargetPort: "Gig0/1"},
	}
}

// QueryTopology queries the topology with the given parameters
func (e *Engine) QueryTopology(query TopologyQuery) (*TopologyResult, error) {
	startTime := time.Now()

	var filteredNodes []TopologyNode
	var filteredLinks []TopologyLink

	// Filter nodes
	for _, node := range e.nodes {
		if query.Source != "" && node.Source != query.Source {
			continue
		}
		if query.DeviceName != "" && node.Name != query.DeviceName {
			continue
		}
		if query.MinConfidence > 0 && node.Confidence < query.MinConfidence {
			continue
		}
		filteredNodes = append(filteredNodes, node)
	}

	// Build a set of filtered node IDs for link filtering
	nodeIDs := make(map[string]bool)
	for _, node := range filteredNodes {
		nodeIDs[node.ID] = true
	}

	// Filter links based on filtered nodes
	for _, link := range e.links {
		if nodeIDs[link.SourceID] && nodeIDs[link.TargetID] {
			filteredLinks = append(filteredLinks, link)
		}
	}

	result := &TopologyResult{
		Nodes:      filteredNodes,
		Links:      filteredLinks,
		TotalNodes: len(filteredNodes),
		TotalLinks: len(filteredLinks),
		QueryTime:  time.Since(startTime).String(),
	}

	return result, nil
}

// GetTopologySummary returns a summary of the topology
func (e *Engine) GetTopologySummary() string {
	return fmt.Sprintf("Topology: %d nodes, %d links", len(e.nodes), len(e.links))
}

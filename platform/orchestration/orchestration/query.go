package orchestration

import (
	"fmt"
	"math"
	"strings"
)

// TopologyFilter represents filtering criteria for topology queries
type TopologyFilter struct {
	Source     string  `json:"source,omitempty" yaml:"source,omitempty"`
	DeviceName string  `json:"device_name,omitempty" yaml:"device_name,omitempty"`
	DeviceType string  `json:"device_type,omitempty" yaml:"device_type,omitempty"`
	Confidence float64 `json:"confidence,omitempty" yaml:"confidence,omitempty"`
}

// TopologyResult represents a single topology entry
type TopologyResult struct {
	ID         string  `json:"id" yaml:"id"`
	Source     string  `json:"source" yaml:"source"`
	DeviceName string  `json:"device_name" yaml:"device_name"`
	DeviceType string  `json:"device_type" yaml:"device_type"`
	IPAddress  string  `json:"ip_address" yaml:"ip_address"`
	Neighbors  []Neighbor `json:"neighbors" yaml:"neighbors"`
	Confidence float64 `json:"confidence" yaml:"confidence"`
	Metadata   map[string]interface{} `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// Neighbor represents a network neighbor
type Neighbor struct {
	DeviceName string `json:"device_name" yaml:"device_name"`
	Interface  string `json:"interface" yaml:"interface"`
	Protocol   string `json:"protocol" yaml:"protocol"`
}

// OrchestrationEngine handles topology queries
type OrchestrationEngine struct {
	// In a real implementation, this would have a database or cache connection
	topologyData []TopologyResult
}

// NewOrchestrationEngine creates a new orchestration engine
func NewOrchestrationEngine() *OrchestrationEngine {
	return &OrchestrationEngine{
		topologyData: getDefaultTopologyData(),
	}
}

// QueryTopology queries the topology with the given filter
func (e *OrchestrationEngine) QueryTopology(filter TopologyFilter) ([]TopologyResult, error) {
	var results []TopologyResult

	for _, entry := range e.topologyData {
		// Apply source filter
		if filter.Source != "" && !strings.EqualFold(entry.Source, filter.Source) {
			continue
		}

		// Apply device name filter (substring match)
		if filter.DeviceName != "" && !strings.Contains(strings.ToLower(entry.DeviceName), strings.ToLower(filter.DeviceName)) {
			continue
		}

		// Apply device type filter (case-insensitive)
		if filter.DeviceType != "" && !strings.EqualFold(entry.DeviceType, filter.DeviceType) {
			continue
		}

		// Apply confidence threshold
		if filter.Confidence > 0 && entry.Confidence < filter.Confidence {
			continue
		}

		results = append(results, entry)
	}

	return results, nil
}

// getDefaultTopologyData returns sample topology data for testing
func getDefaultTopologyData() []TopologyResult {
	return []TopologyResult{
		{
			ID:         "1",
			Source:     "cisco",
			DeviceName: "core-router-01",
			DeviceType: "router",
			IPAddress:  "10.0.0.1",
			Confidence: 0.95,
			Neighbors: []Neighbor{
				{DeviceName: "core-switch-01", Interface: "GigabitEthernet0/0/1", Protocol: "CDP"},
				{DeviceName: "edge-router-01", Interface: "GigabitEthernet0/0/2", Protocol: "OSPF"},
			},
			Metadata: map[string]interface{}{
				"vendor":  "Cisco",
				"model":   "ISR 4451",
				"version": "16.12.3",
			},
		},
		{
			ID:         "2",
			Source:     "cisco",
			DeviceName: "core-switch-01",
			DeviceType: "switch",
			IPAddress:  "10.0.0.2",
			Confidence: 0.92,
			Neighbors: []Neighbor{
				{DeviceName: "core-router-01", Interface: "GigabitEthernet1/0/1", Protocol: "CDP"},
				{DeviceName: "access-switch-01", Interface: "GigabitEthernet1/0/2", Protocol: "LLDP"},
			},
			Metadata: map[string]interface{}{
				"vendor":  "Cisco",
				"model":   "Catalyst 9300",
				"version": "16.12.3",
			},
		},
		{
			ID:         "3",
			Source:     "paloalto",
			DeviceName: "firewall-01",
			DeviceType: "firewall",
			IPAddress:  "10.0.1.1",
			Confidence: 0.88,
			Neighbors: []Neighbor{
				{DeviceName: "core-router-01", Interface: "ethernet1/1", Protocol: "ARP"},
			},
			Metadata: map[string]interface{}{
				"vendor":  "Palo Alto",
				"model":   "PA-5250",
				"version": "10.1.0",
			},
		},
		{
			ID:         "4",
			Source:     "fortinet",
			DeviceName: "fortigate-01",
			DeviceType: "firewall",
			IPAddress:  "10.0.2.1",
			Confidence: 0.75,
			Neighbors: []Neighbor{
				{DeviceName: "core-switch-01", Interface: "port1", Protocol: "LLDP"},
			},
			Metadata: map[string]interface{}{
				"vendor":  "Fortinet",
				"model":   "FortiGate 600E",
				"version": "7.0.1",
			},
		},
		{
			ID:         "5",
			Source:     "cisco",
			DeviceName: "access-switch-01",
			DeviceType: "switch",
			IPAddress:  "10.0.0.3",
			Confidence: 0.85,
			Neighbors: []Neighbor{
				{DeviceName: "core-switch-01", Interface: "GigabitEthernet1/0/1", Protocol: "LLDP"},
			},
			Metadata: map[string]interface{}{
				"vendor":  "Cisco",
				"model":   "Catalyst 2960",
				"version": "15.2.7",
			},
		},
		{
			ID:         "6",
			Source:     "cisco",
			DeviceName: "edge-router-01",
			DeviceType: "router",
			IPAddress:  "10.0.0.4",
			Confidence: 0.90,
			Neighbors: []Neighbor{
				{DeviceName: "core-router-01", Interface: "GigabitEthernet0/0/0", Protocol: "OSPF"},
			},
			Metadata: map[string]interface{}{
				"vendor":  "Cisco",
				"model":   "ISR 4331",
				"version": "16.12.3",
			},
		},
	}
}

// ValidateConfidence checks if the confidence value is valid
func ValidateConfidence(confidence float64) error {
	if confidence < 0 || confidence > 1 {
		return fmt.Errorf("confidence must be between 0.0 and 1.0, got %f", confidence)
	}
	return nil
}

// RoundConfidence rounds confidence to 2 decimal places
func RoundConfidence(confidence float64) float64 {
	return math.Round(confidence*100) / 100
}

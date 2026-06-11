package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/mattpainter701/graphlayer/platform/platform/orchestration/orchestration"
)

func TestQueryCommandJSONOutput(t *testing.T) {
	// Create a temporary file to capture output
	tmpFile, err := os.CreateTemp("", "query-test-*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// Save original stdout and restore after test
	oldStdout := os.Stdout
	os.Stdout = tmpFile
	defer func() { os.Stdout = oldStdout }()

	// Run the query logic directly
	engine := orchestration.NewOrchestrationEngine()
	filter := orchestration.TopologyFilter{
		Source: "cisco",
	}
	topology, err := engine.QueryTopology(filter)
	if err != nil {
		t.Fatalf("QueryTopology failed: %v", err)
	}

	// Encode to JSON
	encoder := json.NewEncoder(tmpFile)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(topology); err != nil {
		t.Fatalf("Failed to encode JSON: %v", err)
	}
	tmpFile.Close()

	// Read and verify output
	data, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to read output: %v", err)
	}

	var result orchestration.TopologyResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Output is not valid JSON: %v", err)
	}

	// Verify structure
	if len(result.Nodes) == 0 {
		t.Error("Expected at least one node in result")
	}
	if len(result.Edges) == 0 {
		t.Error("Expected at least one edge in result")
	}

	// Verify filtering worked (only Cisco nodes)
	for _, node := range result.Nodes {
		if node.Source != "cisco" {
			t.Errorf("Expected all nodes to have source 'cisco', got '%s'", node.Source)
		}
	}
}

func TestQueryCommandYAMLOutput(t *testing.T) {
	// Create a temporary file to capture output
	tmpFile, err := os.CreateTemp("", "query-test-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// Run the query logic directly
	engine := orchestration.NewOrchestrationEngine()
	filter := orchestration.TopologyFilter{
		DeviceName: "router-01",
	}
	topology, err := engine.QueryTopology(filter)
	if err != nil {
		t.Fatalf("QueryTopology failed: %v", err)
	}

	// Encode to YAML
	yamlData, err := yaml.Marshal(topology)
	if err != nil {
		t.Fatalf("Failed to encode YAML: %v", err)
	}

	if _, err := tmpFile.Write(yamlData); err != nil {
		t.Fatalf("Failed to write YAML: %v", err)
	}
	tmpFile.Close()

	// Read and verify output
	data, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to read output: %v", err)
	}

	// Verify YAML structure
	content := string(data)
	if !strings.Contains(content, "nodes:") {
		t.Error("YAML output should contain 'nodes:' key")
	}
	if !strings.Contains(content, "edges:") {
		t.Error("YAML output should contain 'edges:' key")
	}
	if !strings.Contains(content, "router-01") {
		t.Error("YAML output should contain the device name 'router-01'")
	}
}

func TestQueryCommandConfidenceFilter(t *testing.T) {
	engine := orchestration.NewOrchestrationEngine()
	filter := orchestration.TopologyFilter{
		Confidence: 0.9,
	}
	topology, err := engine.QueryTopology(filter)
	if err != nil {
		t.Fatalf("QueryTopology failed: %v", err)
	}

	// Verify all nodes meet confidence threshold
	for _, node := range topology.Nodes {
		if node.Confidence < 0.9 {
			t.Errorf("Node %s has confidence %f, expected >= 0.9", node.Name, node.Confidence)
		}
	}

	// Verify all edges meet confidence threshold
	for _, edge := range topology.Edges {
		if edge.Confidence < 0.9 {
			t.Errorf("Edge %s->%s has confidence %f, expected >= 0.9", edge.SourceName, edge.TargetName, edge.Confidence)
		}
	}
}

func TestQueryCommandNoFilter(t *testing.T) {
	engine := orchestration.NewOrchestrationEngine()
	filter := orchestration.TopologyFilter{}
	topology, err := engine.QueryTopology(filter)
	if err != nil {
		t.Fatalf("QueryTopology failed: %v", err)
	}

	// Verify all nodes are returned
	if len(topology.Nodes) != 4 {
		t.Errorf("Expected 4 nodes with no filter, got %d", len(topology.Nodes))
	}
	if len(topology.Edges) != 3 {
		t.Errorf("Expected 3 edges with no filter, got %d", len(topology.Edges))
	}
}

func TestQueryCommandEmptyFilter(t *testing.T) {
	engine := orchestration.NewOrchestrationEngine()
	filter := orchestration.TopologyFilter{
		Source:     "nonexistent",
		DeviceName: "nonexistent",
		Confidence: 0.99,
	}
	topology, err := engine.QueryTopology(filter)
	if err != nil {
		t.Fatalf("QueryTopology failed: %v", err)
	}

	// Verify empty result
	if len(topology.Nodes) != 0 {
		t.Errorf("Expected 0 nodes with nonexistent filter, got %d", len(topology.Nodes))
	}
	if len(topology.Edges) != 0 {
		t.Errorf("Expected 0 edges with nonexistent filter, got %d", len(topology.Edges))
	}
}

func TestQueryCommandDeviceTypeFilter(t *testing.T) {
	engine := orchestration.NewOrchestrationEngine()
	filter := orchestration.TopologyFilter{
		DeviceType: "router",
	}
	results, err := engine.QueryTopology(filter)
	if err != nil {
		t.Fatalf("QueryTopology failed: %v", err)
	}

	// Should return only router devices
	if len(results) == 0 {
		t.Fatal("Expected at least one router device")
	}
	for _, result := range results {
		if result.DeviceType != "router" {
			t.Errorf("Expected device type router, got %s for device %s", result.DeviceType, result.DeviceName)
		}
	}
}

func TestQueryCommandDeviceTypeFilterFirewall(t *testing.T) {
	engine := orchestration.NewOrchestrationEngine()
	filter := orchestration.TopologyFilter{
		DeviceType: "firewall",
	}
	results, err := engine.QueryTopology(filter)
	if err != nil {
		t.Fatalf("QueryTopology failed: %v", err)
	}

	// Should return only firewall devices
	if len(results) == 0 {
		t.Fatal("Expected at least one firewall device")
	}
	for _, result := range results {
		if result.DeviceType != "firewall" {
			t.Errorf("Expected device type firewall, got %s for device %s", result.DeviceType, result.DeviceName)
		}
	}
}

func TestQueryCommandDeviceTypeCaseInsensitive(t *testing.T) {
	engine := orchestration.NewOrchestrationEngine()
	filter := orchestration.TopologyFilter{
		DeviceType: "Router",
	}
	results, err := engine.QueryTopology(filter)
	if err != nil {
		t.Fatalf("QueryTopology failed: %v", err)
	}

	// Should match "router" devices (case-insensitive)
	if len(results) == 0 {
		t.Fatal("Expected at least one router device with case-insensitive match")
	}
	for _, result := range results {
		if result.DeviceType != "router" {
			t.Errorf("Expected device type router, got %s", result.DeviceType)
		}
	}
}

func TestQueryCommandDeviceTypeCombined(t *testing.T) {
	engine := orchestration.NewOrchestrationEngine()
	filter := orchestration.TopologyFilter{
		Source:     "cisco",
		DeviceType: "switch",
	}
	results, err := engine.QueryTopology(filter)
	if err != nil {
		t.Fatalf("QueryTopology failed: %v", err)
	}

	// Should return only Cisco switches
	for _, result := range results {
		if result.Source != "cisco" {
			t.Errorf("Expected source cisco, got %s", result.Source)
		}
		if result.DeviceType != "switch" {
			t.Errorf("Expected device type switch, got %s", result.DeviceType)
		}
	}
}

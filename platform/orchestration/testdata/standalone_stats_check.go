// Standalone test for graph stats logic.
// Tests the core computation functions (node count, edge count, avg degree,
// connected components, density) without depending on cobra command execution.
//
//go:build standalone
// +build standalone

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// GraphNode represents a node in a topology graph.
type GraphNode struct {
	ID string `json:"id"`
}

// GraphEdge represents an edge in a topology graph.
type GraphEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

// TopologyGraph is a lightweight node/edge graph.
type TopologyGraph struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
}

// countConnectedComponents returns the number of connected components in an
// undirected topology graph, using BFS.
func countConnectedComponents(g *TopologyGraph) int {
	if g == nil || len(g.Nodes) == 0 {
		return 0
	}

	nodeIDs := make([]string, 0, len(g.Nodes))
	adj := make(map[string]map[string]struct{}, len(g.Nodes))
	for _, node := range g.Nodes {
		nodeIDs = append(nodeIDs, node.ID)
		adj[node.ID] = make(map[string]struct{})
	}

	for _, edge := range g.Edges {
		if edge.Source == "" || edge.Target == "" || edge.Source == edge.Target {
			continue
		}
		if _, ok := adj[edge.Source]; ok {
			adj[edge.Source][edge.Target] = struct{}{}
		}
		if _, ok := adj[edge.Target]; ok {
			adj[edge.Target][edge.Source] = struct{}{}
		}
	}

	visited := make(map[string]bool, len(nodeIDs))
	components := 0

	for _, nodeID := range nodeIDs {
		if visited[nodeID] {
			continue
		}
		components++
		queue := []string{nodeID}
		visited[nodeID] = true
		for len(queue) > 0 {
			current := queue[0]
			queue = queue[1:]
			for neighbor := range adj[current] {
				if !visited[neighbor] {
					visited[neighbor] = true
					queue = append(queue, neighbor)
				}
			}
		}
	}
	return components
}

// computeStats returns nodeCount, edgeCount, avgDegree, components, density.
func computeStats(g *TopologyGraph) (int, int, float64, int, float64) {
	if g == nil {
		return 0, 0, 0.0, 0, 0.0
	}
	nodeCount := len(g.Nodes)
	edgeCount := len(g.Edges)

	var avgDegree float64
	if nodeCount > 0 {
		avgDegree = 2.0 * float64(edgeCount) / float64(nodeCount)
	}

	var density float64
	if nodeCount > 1 {
		density = 2.0 * float64(edgeCount) / float64(nodeCount*(nodeCount-1))
	}

	components := countConnectedComponents(g)
	return nodeCount, edgeCount, avgDegree, components, density
}

func main() {
	pass := 0
	fail := 0

	// Test 1: Simple line A-B-C
	{
		g := &TopologyGraph{
			Nodes: []GraphNode{{ID: "A"}, {ID: "B"}, {ID: "C"}},
			Edges: []GraphEdge{{Source: "A", Target: "B"}, {Source: "B", Target: "C"}},
		}
		n, e, deg, comp, dens := computeStats(g)
		if n == 3 && e == 2 && comp == 1 && fmt.Sprintf("%.4f", deg) == "1.3333" && fmt.Sprintf("%.6f", dens) == "0.666667" {
			fmt.Printf("PASS: Test 1 (simple line)\n")
			pass++
		} else {
			fmt.Printf("FAIL: Test 1 (simple line): got n=%d e=%d deg=%.4f comp=%d dens=%.6f\n", n, e, deg, comp, dens)
			fail++
		}
	}

	// Test 2: Two disconnected components A-B and C-D
	{
		g := &TopologyGraph{
			Nodes: []GraphNode{{ID: "A"}, {ID: "B"}, {ID: "C"}, {ID: "D"}},
			Edges: []GraphEdge{{Source: "A", Target: "B"}, {Source: "C", Target: "D"}},
		}
		n, e, deg, comp, dens := computeStats(g)
		if n == 4 && e == 2 && comp == 2 && fmt.Sprintf("%.4f", deg) == "1.0000" && fmt.Sprintf("%.6f", dens) == "0.333333" {
			fmt.Printf("PASS: Test 2 (two components)\n")
			pass++
		} else {
			fmt.Printf("FAIL: Test 2 (two components): got n=%d e=%d deg=%.4f comp=%d dens=%.6f\n", n, e, deg, comp, dens)
			fail++
		}
	}

	// Test 3: Empty graph
	{
		g := &TopologyGraph{
			Nodes: []GraphNode{},
			Edges: []GraphEdge{},
		}
		n, e, deg, comp, dens := computeStats(g)
		if n == 0 && e == 0 && comp == 0 && deg == 0.0 && dens == 0.0 {
			fmt.Printf("PASS: Test 3 (empty graph)\n")
			pass++
		} else {
			fmt.Printf("FAIL: Test 3 (empty graph): got n=%d e=%d deg=%.4f comp=%d dens=%.6f\n", n, e, deg, comp, dens)
			fail++
		}
	}

	// Test 4: Fully connected triangle A-B-C
	{
		g := &TopologyGraph{
			Nodes: []GraphNode{{ID: "A"}, {ID: "B"}, {ID: "C"}},
			Edges: []GraphEdge{
				{Source: "A", Target: "B"},
				{Source: "B", Target: "C"},
				{Source: "A", Target: "C"},
			},
		}
		n, e, deg, comp, dens := computeStats(g)
		// avg degree = 2*3/3 = 2.0
		// density = 2*3/(3*2) = 1.0
		if n == 3 && e == 3 && comp == 1 && fmt.Sprintf("%.4f", deg) == "2.0000" && fmt.Sprintf("%.6f", dens) == "1.000000" {
			fmt.Printf("PASS: Test 4 (triangle)\n")
			pass++
		} else {
			fmt.Printf("FAIL: Test 4 (triangle): got n=%d e=%d deg=%.4f comp=%d dens=%.6f\n", n, e, deg, comp, dens)
			fail++
		}
	}

	// Test 5: Single node, no edges
	{
		g := &TopologyGraph{
			Nodes: []GraphNode{{ID: "A"}},
			Edges: []GraphEdge{},
		}
		n, e, deg, comp, dens := computeStats(g)
		if n == 1 && e == 0 && comp == 1 && deg == 0.0 && dens == 0.0 {
			fmt.Printf("PASS: Test 5 (single node)\n")
			pass++
		} else {
			fmt.Printf("FAIL: Test 5 (single node): got n=%d e=%d deg=%.4f comp=%d dens=%.6f\n", n, e, deg, comp, dens)
			fail++
		}
	}

	// Test 6: nil graph
	{
		n, e, deg, comp, dens := computeStats(nil)
		if n == 0 && e == 0 && comp == 0 && deg == 0.0 && dens == 0.0 {
			fmt.Printf("PASS: Test 6 (nil graph)\n")
			pass++
		} else {
			fmt.Printf("FAIL: Test 6 (nil graph): got n=%d e=%d deg=%.4f comp=%d dens=%.6f\n", n, e, deg, comp, dens)
			fail++
		}
	}

	// Test 7: JSON file test - read simple_line.json
	{
		data, err := os.ReadFile("testdata/simple_line.json")
		if err != nil {
			fmt.Printf("FAIL: Test 7 (JSON read): %v\n", err)
			fail++
		} else {
			var g TopologyGraph
			if err := json.Unmarshal(data, &g); err != nil {
				fmt.Printf("FAIL: Test 7 (JSON parse): %v\n", err)
				fail++
			} else {
				n, e, deg, comp, dens := computeStats(&g)
				if n == 3 && e == 2 && comp == 1 && strings.Contains(fmt.Sprintf("%.4f", deg), "1.3333") && strings.Contains(fmt.Sprintf("%.6f", dens), "0.666667") {
					fmt.Printf("PASS: Test 7 (JSON file simple_line)\n")
					pass++
				} else {
					fmt.Printf("FAIL: Test 7 (JSON file simple_line): got n=%d e=%d deg=%.4f comp=%d dens=%.6f\n", n, e, deg, comp, dens)
					fail++
				}
			}
		}
	}

	// Test 8: JSON file test - read two_components.json
	{
		data, err := os.ReadFile("testdata/two_components.json")
		if err != nil {
			fmt.Printf("FAIL: Test 8 (JSON read): %v\n", err)
			fail++
		} else {
			var g TopologyGraph
			if err := json.Unmarshal(data, &g); err != nil {
				fmt.Printf("FAIL: Test 8 (JSON parse): %v\n", err)
				fail++
			} else {
				n, e, deg, comp, dens := computeStats(&g)
				if n == 4 && e == 2 && comp == 2 && strings.Contains(fmt.Sprintf("%.4f", deg), "1.0000") && strings.Contains(fmt.Sprintf("%.6f", dens), "0.333333") {
					fmt.Printf("PASS: Test 8 (JSON file two_components)\n")
					pass++
				} else {
					fmt.Printf("FAIL: Test 8 (JSON file two_components): got n=%d e=%d deg=%.4f comp=%d dens=%.6f\n", n, e, deg, comp, dens)
					fail++
				}
			}
		}
	}

	fmt.Printf("\n%d passed, %d failed out of %d tests\n", pass, fail, pass+fail)
	if fail > 0 {
		os.Exit(1)
	}
}

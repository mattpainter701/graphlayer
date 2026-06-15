package graph

import (
	"errors"
	"sort"
)

// ColoringResult holds the result of a graph coloring algorithm.
type ColoringResult struct {
	Colors    map[string]int
	NumColors int
}

// GraphColoring computes a greedy graph coloring using the Welsh-Powell algorithm.
func GraphColoring(g *TopologyGraph) (*ColoringResult, error) {
	if g == nil || len(g.Nodes) == 0 {
		return nil, errors.New("graph is nil or empty")
	}

	adj := make(map[string]map[string]bool, len(g.Nodes))
	for id := range g.Nodes {
		adj[id] = make(map[string]bool)
	}
	for srcID, targets := range g.Edges {
		for tgtID := range targets {
			if srcID == tgtID {
				continue
			}
			adj[srcID][tgtID] = true
			adj[tgtID][srcID] = true
		}
	}

	type nodeDegree struct {
		id     string
		degree int
	}
	nodes := make([]nodeDegree, 0, len(g.Nodes))
	for id, neighbors := range adj {
		nodes = append(nodes, nodeDegree{id: id, degree: len(neighbors)})
	}
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].degree == nodes[j].degree {
			return nodes[i].id < nodes[j].id
		}
		return nodes[i].degree > nodes[j].degree
	})

	colors := make(map[string]int, len(g.Nodes))
	for _, n := range nodes {
		usedColors := make(map[int]bool)
		for neighbor := range adj[n.id] {
			if c, ok := colors[neighbor]; ok {
				usedColors[c] = true
			}
		}
		color := 1
		for usedColors[color] {
			color++
		}
		colors[n.id] = color
	}

	maxColor := 0
	for _, c := range colors {
		if c > maxColor {
			maxColor = c
		}
	}

	return &ColoringResult{
		Colors:    colors,
		NumColors: maxColor,
	}, nil
}

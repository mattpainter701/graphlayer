package graph

import (
	"errors"
	"fmt"
	"sort"
)

// UnionFind implements a union-find (disjoint set) data structure.
type UnionFind struct {
	parent map[string]string
	rank   map[string]int
}

// NewUnionFind creates a new UnionFind for the given nodes.
func NewUnionFind(nodes []string) *UnionFind {
	parent := make(map[string]string, len(nodes))
	rank := make(map[string]int, len(nodes))
	for _, n := range nodes {
		parent[n] = n
		rank[n] = 0
	}
	return &UnionFind{parent: parent, rank: rank}
}

// Find returns the root representative for x.
func (uf *UnionFind) Find(x string) string {
	if uf.parent[x] != x {
		uf.parent[x] = uf.Find(uf.parent[x])
	}
	return uf.parent[x]
}

// Union merges the sets containing x and y. Returns true if they were merged.
func (uf *UnionFind) Union(x, y string) bool {
	rx, ry := uf.Find(x), uf.Find(y)
	if rx == ry {
		return false
	}
	if uf.rank[rx] < uf.rank[ry] {
		uf.parent[rx] = ry
	} else if uf.rank[rx] > uf.rank[ry] {
		uf.parent[ry] = rx
	} else {
		uf.parent[ry] = rx
		uf.rank[rx]++
	}
	return true
}

// MSTEdge represents an edge in the minimum spanning tree.
type MSTEdge struct {
	Source string
	Target string
	Weight float64
}

// MinimumSpanningTree computes the MST of an undirected weighted graph
// using Kruskal's algorithm with Union-Find.
func MinimumSpanningTree(g *TopologyGraph) ([]MSTEdge, float64, error) {
	if g == nil || len(g.Nodes) == 0 {
		return nil, 0, errors.New("graph is nil or empty")
	}

	type rawEdge struct {
		source string
		target string
		weight float64
	}
	var edges []rawEdge
	seen := make(map[[2]string]bool)

	for srcID, targets := range g.Edges {
		for tgtID, edge := range targets {
			if srcID == tgtID {
				continue
			}
			key := [2]string{srcID, tgtID}
			if srcID > tgtID {
				key = [2]string{tgtID, srcID}
			}
			if seen[key] {
				continue
			}
			seen[key] = true
			w := 1.0
			if edge != nil && edge.Weight > 0 {
				w = edge.Weight
			}
			edges = append(edges, rawEdge{source: srcID, target: tgtID, weight: w})
		}
	}

	if len(edges) == 0 {
		return nil, 0, errors.New("graph has no edges")
	}

	sort.Slice(edges, func(i, j int) bool {
		if edges[i].weight == edges[j].weight {
			return edges[i].source < edges[j].source
		}
		return edges[i].weight < edges[j].weight
	})

	nodeIDs := make([]string, 0, len(g.Nodes))
	for id := range g.Nodes {
		nodeIDs = append(nodeIDs, id)
	}
	uf := NewUnionFind(nodeIDs)

	var mstEdges []MSTEdge
	var totalWeight float64

	for _, e := range edges {
		if uf.Union(e.source, e.target) {
			mstEdges = append(mstEdges, MSTEdge{
				Source: e.source,
				Target: e.target,
				Weight: e.weight,
			})
			totalWeight += e.weight
			if len(mstEdges) == len(g.Nodes)-1 {
				break
			}
		}
	}

	if len(mstEdges) != len(g.Nodes)-1 {
		return nil, 0, fmt.Errorf("graph is not connected: MST has %d edges, expected %d", len(mstEdges), len(g.Nodes)-1)
	}

	return mstEdges, totalWeight, nil
}

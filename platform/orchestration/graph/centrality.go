package graph

import (
	"fmt"
	"math/rand"
	"sort"
)

// BetweennessCentrality computes betweenness centrality for every node in the
// topology graph using Brandes' algorithm.
//
// The graph is treated as undirected for topology analysis so that a simple
// chain or star topology produces the expected centrality values regardless of
// edge direction in the source data.
func (g *TopologyGraph) BetweennessCentrality() map[string]float64 {
	return BetweennessCentrality(g)
}

// ApproximateBetweennessCentrality computes betweenness centrality by sampling
// a subset of source nodes and scaling the accumulated result.
func (g *TopologyGraph) ApproximateBetweennessCentrality(sampleSize int) (map[string]float64, error) {
	return ApproximateBetweennessCentrality(g, sampleSize)
}

// BetweennessCentrality computes betweenness centrality for every node in the
// topology graph using Brandes' algorithm.
func BetweennessCentrality(g *TopologyGraph) map[string]float64 {
	nodeIDs, adjacency := buildUndirectedAdjacency(g)
	centrality := make(map[string]float64, len(nodeIDs))
	for _, nodeID := range nodeIDs {
		centrality[nodeID] = 0
	}
	if len(nodeIDs) == 0 {
		return centrality
	}

	return accumulateBetweenness(nodeIDs, adjacency, nodeIDs)
}

// ApproximateBetweennessCentrality computes betweenness centrality by sampling
// source nodes and scaling the accumulated score by N/sampleSize.
func ApproximateBetweennessCentrality(g *TopologyGraph, sampleSize int) (map[string]float64, error) {
	if sampleSize <= 0 {
		return nil, fmt.Errorf("sampleSize must be positive")
	}

	nodeIDs, adjacency := buildUndirectedAdjacency(g)
	centrality := make(map[string]float64, len(nodeIDs))
	for _, nodeID := range nodeIDs {
		centrality[nodeID] = 0
	}
	if len(nodeIDs) == 0 {
		return centrality, nil
	}

	if sampleSize >= len(nodeIDs) {
		return accumulateBetweenness(nodeIDs, adjacency, nodeIDs), nil
	}

	rng := rand.New(rand.NewSource(42))
	perm := rng.Perm(len(nodeIDs))
	sources := make([]string, sampleSize)
	for i := 0; i < sampleSize; i++ {
		sources[i] = nodeIDs[perm[i]]
	}

	centrality = accumulateBetweenness(nodeIDs, adjacency, sources)
	scale := float64(len(nodeIDs)) / float64(sampleSize)
	for nodeID := range centrality {
		centrality[nodeID] *= scale
	}

	return centrality, nil
}

// buildUndirectedAdjacency builds a sorted list of node IDs and a deduplicated
// undirected adjacency map from the topology graph.
func buildUndirectedAdjacency(g *TopologyGraph) ([]string, map[string]map[string]struct{}) {
	if g == nil {
		return nil, map[string]map[string]struct{}{}
	}

	// Collect all node IDs from the map-based Nodes field.
	nodeIDs := make([]string, 0, len(g.Nodes))
	for id := range g.Nodes {
		nodeIDs = append(nodeIDs, id)
	}
	sort.Strings(nodeIDs)

	// Build adjacency sets.
	adjacency := make(map[string]map[string]struct{}, len(nodeIDs))
	for _, id := range nodeIDs {
		adjacency[id] = make(map[string]struct{})
	}

	// Also collect any node IDs referenced in edges but not in Nodes.
	for src, targets := range g.Edges {
		for tgt := range targets {
			if src == tgt {
				continue
			}
			if _, ok := adjacency[src]; !ok {
				adjacency[src] = make(map[string]struct{})
				nodeIDs = append(nodeIDs, src)
			}
			if _, ok := adjacency[tgt]; !ok {
				adjacency[tgt] = make(map[string]struct{})
				nodeIDs = append(nodeIDs, tgt)
			}
			adjacency[src][tgt] = struct{}{}
			adjacency[tgt][src] = struct{}{}
		}
	}

	sort.Strings(nodeIDs)
	return nodeIDs, adjacency
}

func accumulateBetweenness(nodeIDs []string, adjacency map[string]map[string]struct{}, sources []string) map[string]float64 {
	centrality := make(map[string]float64, len(nodeIDs))
	for _, nodeID := range nodeIDs {
		centrality[nodeID] = 0
	}

	for _, source := range sources {
		stack := make([]string, 0, len(nodeIDs))
		predecessors := make(map[string][]string, len(nodeIDs))
		sigma := make(map[string]float64, len(nodeIDs))
		distance := make(map[string]int, len(nodeIDs))
		for _, nodeID := range nodeIDs {
			distance[nodeID] = -1
		}
		distance[source] = 0
		sigma[source] = 1

		queue := make([]string, 0, len(nodeIDs))
		queue = append(queue, source)
		for head := 0; head < len(queue); head++ {
			v := queue[head]
			stack = append(stack, v)
			for w := range adjacency[v] {
				if distance[w] < 0 {
					distance[w] = distance[v] + 1
					queue = append(queue, w)
				}
				if distance[w] == distance[v]+1 {
					sigma[w] += sigma[v]
					predecessors[w] = append(predecessors[w], v)
				}
			}
		}

		delta := make(map[string]float64, len(nodeIDs))
		for i := len(stack) - 1; i >= 0; i-- {
			w := stack[i]
			for _, v := range predecessors[w] {
				if sigma[w] == 0 {
					continue
				}
				delta[v] += (sigma[v] / sigma[w]) * (1 + delta[w])
			}
			if w != source {
				centrality[w] += delta[w]
			}
		}
	}

	for nodeID := range centrality {
		centrality[nodeID] /= 2
	}

	return centrality
}

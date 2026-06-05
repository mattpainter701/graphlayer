package graph

import (
	"fmt"
	"math"
)

// PageRank computes the PageRank scores for all nodes in the graph using the
// standard iterative algorithm with a damping factor.
//
// The graph is treated as directed: edges go from Source to Target. If directed
// analysis is not needed, ensure edges are added in both directions in the
// TopologyGraph.
//
// Parameters:
//   - dampingFactor: probability of following an edge (typically 0.85). Must be
//     in [0, 1). A value of 0 treats every node as equally important.
//   - maxIterations: maximum number of iterations before convergence.
//   - tolerance: convergence threshold (L1 norm of change between iterations).
//
// Returns a map from node ID to PageRank score. Scores sum to 1 (or the number
// of nodes if you prefer — the standard normalisation is sum-to-1).
func PageRank(g *TopologyGraph, dampingFactor float64, maxIterations int, tolerance float64) (map[string]float64, error) {
	if g == nil {
		return nil, fmt.Errorf("graph is nil")
	}
	if dampingFactor < 0 || dampingFactor >= 1 {
		return nil, fmt.Errorf("dampingFactor must be in [0, 1), got %f", dampingFactor)
	}
	if maxIterations <= 0 {
		return nil, fmt.Errorf("maxIterations must be positive, got %d", maxIterations)
	}
	if tolerance < 0 {
		return nil, fmt.Errorf("tolerance must be non-negative, got %f", tolerance)
	}

	// Collect node IDs and build index.
	nodeIDs := make([]string, 0, len(g.Nodes))
	nodeIndex := make(map[string]int, len(g.Nodes))
	for id := range g.Nodes {
		if id == "" {
			continue
		}
		if _, exists := nodeIndex[id]; exists {
			continue
		}
		nodeIndex[id] = len(nodeIDs)
		nodeIDs = append(nodeIDs, id)
	}

	n := len(nodeIDs)
	if n == 0 {
		return nil, fmt.Errorf("graph has no nodes")
	}

	// Build directed adjacency: outEdges[u] = list of v where u -> v exists.
	outEdges := make([][]int, n)
	outDegree := make([]int, n)

	for src, targets := range g.Edges {
		srcIdx, srcOK := nodeIndex[src]
		if !srcOK {
			continue
		}
		for tgt := range targets {
			if src == "" || tgt == "" || src == tgt {
				continue
			}
			tgtIdx, tgtOK := nodeIndex[tgt]
			if !tgtOK {
				continue
			}
			outEdges[srcIdx] = append(outEdges[srcIdx], tgtIdx)
		}
	}

	for i := 0; i < n; i++ {
		outDegree[i] = len(outEdges[i])
	}

	// Initialise rank vector: uniform probability.
	rank := make([]float64, n)
	initialRank := 1.0 / float64(n)
	for i := 0; i < n; i++ {
		rank[i] = initialRank
	}

	teleportWeight := (1 - dampingFactor) / float64(n)

	for iter := 0; iter < maxIterations; iter++ {
		newRank := make([]float64, n)

		// Teleportation contribution (same for all nodes).
		for i := 0; i < n; i++ {
			newRank[i] = teleportWeight
		}

		// Distribute rank along edges.
		var danglingSum float64
		for i := 0; i < n; i++ {
			if outDegree[i] == 0 {
				danglingSum += rank[i]
			} else {
				contribution := dampingFactor * rank[i] / float64(outDegree[i])
				for _, j := range outEdges[i] {
					newRank[j] += contribution
				}
			}
		}

		// Distribute dangling node rank uniformly.
		if danglingSum > 0 {
			danglingContribution := dampingFactor * danglingSum / float64(n)
			for i := 0; i < n; i++ {
				newRank[i] += danglingContribution
			}
		}

		// Check convergence.
		var diff float64
		for i := 0; i < n; i++ {
			diff += math.Abs(newRank[i] - rank[i])
		}
		rank = newRank

		if diff < tolerance {
			break
		}
	}

	// Build result map.
	result := make(map[string]float64, n)
	for i, id := range nodeIDs {
		result[id] = rank[i]
	}

	return result, nil
}

// EigenvectorCentrality computes the eigenvector centrality for every node
// using power iteration on the adjacency matrix.
//
// The graph is treated as undirected — edges contribute symmetrically to the
// adjacency matrix. Edge weights are used when present; otherwise a default
// weight of 1.0 is assumed.
//
// For bipartite graphs, power iteration can oscillate between two subspaces.
// This implementation adds a tiny self-loop (epsilon) to each node to break
// exact eigenvalue symmetry and ensure convergence. Isolated nodes (with no
// edges) are assigned zero centrality.
//
// Parameters:
//   - maxIterations: maximum number of iterations before convergence.
//   - tolerance: convergence threshold (L2 norm of change between iterations).
//
// Returns a map from node ID to centrality score. Scores are L2-normalised
// (unit vector) so that the magnitude of the vector is 1. Higher scores
// indicate greater importance.
func EigenvectorCentrality(g *TopologyGraph, maxIterations int, tolerance float64) (map[string]float64, error) {
	if g == nil {
		return nil, fmt.Errorf("graph is nil")
	}
	if maxIterations <= 0 {
		return nil, fmt.Errorf("maxIterations must be positive, got %d", maxIterations)
	}
	if tolerance < 0 {
		return nil, fmt.Errorf("tolerance must be non-negative, got %f", tolerance)
	}

	// Collect node IDs and build index.
	nodeIDs := make([]string, 0, len(g.Nodes))
	nodeIndex := make(map[string]int, len(g.Nodes))
	for id := range g.Nodes {
		if id == "" {
			continue
		}
		if _, exists := nodeIndex[id]; exists {
			continue
		}
		nodeIndex[id] = len(nodeIDs)
		nodeIDs = append(nodeIDs, id)
	}

	n := len(nodeIDs)
	if n == 0 {
		return nil, fmt.Errorf("graph has no nodes")
	}

	// Build undirected weighted adjacency as a sparse matrix.
	adj := make([][]struct {
		idx    int
		weight float64
	}, n)

	// Track seen edges to avoid double-counting undirected edges.
	seen := make(map[[2]int]struct{})

	for src, targets := range g.Edges {
		srcIdx, srcOK := nodeIndex[src]
		if !srcOK {
			continue
		}
		for tgt, edge := range targets {
			if src == "" || tgt == "" || src == tgt {
				continue
			}
			tgtIdx, tgtOK := nodeIndex[tgt]
			if !tgtOK {
				continue
			}

			a, b := srcIdx, tgtIdx
			if a > b {
				a, b = b, a
			}
			key := [2]int{a, b}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}

			w := edge.Weight
			if w <= 0 {
				w = 1.0
			}

			adj[srcIdx] = append(adj[srcIdx], struct {
				idx    int
				weight float64
			}{tgtIdx, w})
			adj[tgtIdx] = append(adj[tgtIdx], struct {
				idx    int
				weight float64
			}{srcIdx, w})
		}
	}

	// Add a tiny self-loop (epsilon) to every node to handle bipartite graphs
	// where power iteration would otherwise oscillate. This shifts the
	// eigenvalues and guarantees convergence.
	epsilon := 1e-6

	// Initialise centrality vector: uniform.
	centrality := make([]float64, n)
	initial := 1.0 / math.Sqrt(float64(n))
	for i := 0; i < n; i++ {
		centrality[i] = initial
	}

	for iter := 0; iter < maxIterations; iter++ {
		newCentrality := make([]float64, n)
		for i := 0; i < n; i++ {
			var sum float64
			for _, nb := range adj[i] {
				sum += nb.weight * centrality[nb.idx]
			}
			sum += epsilon * centrality[i]
			newCentrality[i] = sum
		}

		var norm float64
		for _, v := range newCentrality {
			norm += v * v
		}
		norm = math.Sqrt(norm)

		if norm == 0 {
			for i := range newCentrality {
				newCentrality[i] = 0
			}
		} else {
			for i := range newCentrality {
				newCentrality[i] /= norm
			}
		}

		var diff float64
		for i := 0; i < n; i++ {
			d := newCentrality[i] - centrality[i]
			diff += d * d
		}
		diff = math.Sqrt(diff)

		copy(centrality, newCentrality)

		if diff < tolerance {
			break
		}
	}

	// Build result map.
	result := make(map[string]float64, n)
	for i, id := range nodeIDs {
		result[id] = centrality[i]
	}

	// Zero out isolated nodes (no edges) — they should have zero centrality.
	for i, id := range nodeIDs {
		if len(adj[i]) == 0 {
			result[id] = 0
		}
	}

	return result, nil
}

// PageRankOnGraph is a convenience method on TopologyGraph.
func (g *TopologyGraph) PageRank(dampingFactor float64, maxIterations int, tolerance float64) (map[string]float64, error) {
	return PageRank(g, dampingFactor, maxIterations, tolerance)
}

// EigenvectorCentralityOnGraph is a convenience method on TopologyGraph.
func (g *TopologyGraph) EigenvectorCentrality(maxIterations int, tolerance float64) (map[string]float64, error) {
	return EigenvectorCentrality(g, maxIterations, tolerance)
}

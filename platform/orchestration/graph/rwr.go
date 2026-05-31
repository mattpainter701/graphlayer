package graph

import (
	"fmt"
	"math"
)

// RandomWalkWithRestart computes the steady-state probability of each node
// reachable from seedNodeID using the random walk with restart (RWR) algorithm.
//
// The graph is treated as undirected for random-walk topology analysis. Edge
// weights are used when present (via the Weight field); otherwise a default
// weight of 1.0 is assumed. The adjacency (transition) matrix is normalised so
// that each column sums to 1. The iteration follows:
//
//	p_{new} = (1 - restartProbability) * adjacency_normalised * p
//	        + restartProbability * seed_vector
//
// Convergence is declared when the L1 norm of the change between successive
// iterations drops below tolerance. A maximum of maxIterations is enforced to
// protect against non-convergent cases.
func RandomWalkWithRestart(g *TopologyGraph, seedNodeID string, restartProbability float64, maxIterations int, tolerance float64) (map[string]float64, error) {
	if g == nil {
		return nil, fmt.Errorf("graph is nil")
	}
	if restartProbability < 0 || restartProbability > 1 {
		return nil, fmt.Errorf("restartProbability must be in [0, 1], got %f", restartProbability)
	}
	if maxIterations <= 0 {
		return nil, fmt.Errorf("maxIterations must be positive, got %d", maxIterations)
	}
	if tolerance < 0 {
		return nil, fmt.Errorf("tolerance must be non-negative, got %f", tolerance)
	}

	// Build node index.
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

	seedIdx, ok := nodeIndex[seedNodeID]
	if !ok {
		return nil, fmt.Errorf("seed node %q not found in graph", seedNodeID)
	}

	n := len(nodeIDs)
	if n == 0 {
		return nil, fmt.Errorf("graph has no nodes")
	}

	// Build undirected adjacency with weights.
	// adj[j] holds pairs (neighbourIdx, weight) for all neighbours of node j.
	adj := make([][]struct {
		idx    int
		weight float64
	}, n)

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

			w := edge.Weight
			if w <= 0 {
				w = 1.0
			}

			// Undirected: add edge in both directions.
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

	// Precompute column sums for normalisation and handle dangling nodes.
	colSums := make([]float64, n)
	for j := 0; j < n; j++ {
		var sum float64
		for _, nb := range adj[j] {
			sum += nb.weight
		}
		colSums[j] = sum
	}

	// Initialise probability vector: 1.0 at seed, 0 elsewhere.
	p := make([]float64, n)
	p[seedIdx] = 1.0

	// Seed vector is constant throughout.
	seedVec := make([]float64, n)
	seedVec[seedIdx] = 1.0

	alpha := 1.0 - restartProbability

	for iter := 0; iter < maxIterations; iter++ {
		pNew := make([]float64, n)

		for j := 0; j < n; j++ {
			if p[j] == 0 {
				continue
			}
			colSum := colSums[j]

			if colSum == 0 {
				// Dangling node: redistribute probability to the seed node.
				pNew[seedIdx] += alpha * p[j]
			} else {
				colFactor := alpha * p[j] / colSum
				for _, nb := range adj[j] {
					pNew[nb.idx] += nb.weight * colFactor
				}
			}
		}

		// Add restart term.
		for i := 0; i < n; i++ {
			pNew[i] += restartProbability * seedVec[i]
		}

		// L1 norm change for convergence check.
		var change float64
		for i := 0; i < n; i++ {
			change += math.Abs(pNew[i] - p[i])
		}

		p = pNew

		if change < tolerance {
			break
		}
	}

	// Build result map.
	result := make(map[string]float64, n)
	for i, nodeID := range nodeIDs {
		if p[i] > 0 {
			result[nodeID] = p[i]
		}
	}

	return result, nil
}

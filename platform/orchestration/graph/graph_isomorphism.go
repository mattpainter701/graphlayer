package graph

import (
	"errors"
	"sort"
)

// ErrGraphTooLarge is returned when a graph has more than 8 nodes and the naive
// brute-force permutation check cannot be performed.
var ErrGraphTooLarge = errors.New("graph too large for naive isomorphism check (max 8 nodes)")

// AreIsomorphic checks whether two topology graphs are isomorphic using a naive
// brute-force permutation algorithm.
//
// It performs a degree-sequence pre-check first; if degree sequences differ the
// graphs are not isomorphic. For graphs with ≤8 nodes it exhaustively checks all
// node-label permutations (8! = 40320 max). For larger graphs it returns
// ErrGraphTooLarge.
func AreIsomorphic(a, b *TopologyGraph) (bool, error) {
	// Both nil — trivially isomorphic
	if a == nil && b == nil {
		return true, nil
	}
	// One nil — not isomorphic
	if a == nil || b == nil {
		return false, nil
	}

	// Different number of nodes — cannot be isomorphic
	if len(a.Nodes) != len(b.Nodes) {
		return false, nil
	}

	// Both have zero nodes — trivially isomorphic
	if len(a.Nodes) == 0 {
		return true, nil
	}

	// Enforce practical limit for brute-force permutation
	if len(a.Nodes) > 8 {
		return false, ErrGraphTooLarge
	}

	// Degree-sequence pre-check (necessary condition)
	if !degreeSequencesMatch(a, b) {
		return false, nil
	}

	// Brute-force permutation check
	aIDs := sortedNodeIDs(a)
	bIDs := sortedNodeIDs(b)
	aAdj := buildAdjacencyMatrix(a, aIDs)

	n := len(aIDs)
	perm := make([]int, n)
	for i := 0; i < n; i++ {
		perm[i] = i
	}

	match, _ := tryPermutations(aAdj, b, bIDs, perm, 0)
	return match, nil
}

// sortedNodeIDs returns the sorted list of node IDs from the graph.
func sortedNodeIDs(g *TopologyGraph) []string {
	ids := make([]string, 0, len(g.Nodes))
	for id := range g.Nodes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// degreeSequencesMatch checks whether the sorted degree sequences of two graphs
// are equal. This is a necessary but not sufficient condition for isomorphism.
func degreeSequencesMatch(a, b *TopologyGraph) bool {
	da := computeDegreeSequence(a)
	db := computeDegreeSequence(b)

	if len(da) != len(db) {
		return false
	}
	for i := range da {
		if da[i] != db[i] {
			return false
		}
	}
	return true
}

// computeDegreeSequence returns the sorted total degree (out-degree + in-degree)
// for each node in the graph.
func computeDegreeSequence(g *TopologyGraph) []int {
	outDeg := make(map[string]int, len(g.Nodes))
	inDeg := make(map[string]int, len(g.Nodes))

	// Initialise every node to zero so isolated nodes appear with degree 0.
	for id := range g.Nodes {
		outDeg[id] = 0
		inDeg[id] = 0
	}

	for src, targets := range g.Edges {
		for tgt := range targets {
			outDeg[src]++
			inDeg[tgt]++
		}
	}

	seq := make([]int, 0, len(g.Nodes))
	for id := range g.Nodes {
		seq = append(seq, outDeg[id]+inDeg[id])
	}
	sort.Ints(seq)
	return seq
}

// buildAdjacencyMatrix builds an N×N boolean adjacency matrix where entry [i][j]
// is true iff there is a directed edge from ids[i] to ids[j].
func buildAdjacencyMatrix(g *TopologyGraph, ids []string) [][]bool {
	n := len(ids)
	idIndex := make(map[string]int, n)
	for i, id := range ids {
		idIndex[id] = i
	}

	mat := make([][]bool, n)
	for i := range mat {
		mat[i] = make([]bool, n)
	}

	for src, targets := range g.Edges {
		si, ok := idIndex[src]
		if !ok {
			continue
		}
		for tgt := range targets {
			ti, ok := idIndex[tgt]
			if !ok {
				continue
			}
			mat[si][ti] = true
		}
	}

	return mat
}

// adjacencyEqual reports whether two N×N boolean matrices are identical.
func adjacencyEqual(a, b [][]bool) bool {
	n := len(a)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			if a[i][j] != b[i][j] {
				return false
			}
		}
	}
	return true
}

// tryPermutations recursively generates permutations of B's node indices
// (Heap's algorithm). For each full permutation it builds a reordered adjacency
// matrix for B and compares it with A's reference matrix. Returns true at the
// first match.
func tryPermutations(aAdj [][]bool, b *TopologyGraph, bIDs []string, perm []int, pos int) (bool, bool) {
	n := len(bIDs)

	if pos == n-1 {
		// Build B's adjacency matrix with the current node ordering
		reordered := make([]string, n)
		for i, p := range perm {
			reordered[i] = bIDs[p]
		}
		bAdj := buildAdjacencyMatrix(b, reordered)
		if adjacencyEqual(aAdj, bAdj) {
			return true, true
		}
		return false, false
	}

	for i := pos; i < n; i++ {
		perm[pos], perm[i] = perm[i], perm[pos]
		if ok, _ := tryPermutations(aAdj, b, bIDs, perm, pos+1); ok {
			return true, true
		}
		perm[pos], perm[i] = perm[i], perm[pos]
	}

	return false, false
}

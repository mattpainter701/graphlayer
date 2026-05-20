package topology

import (
    "errors"
    "strings"
)

// NodeMatch represents a single matching node from a search.
type NodeMatch struct {
    NodeID       string
    MatchedField string
    Score        float64
}

// SearchNodes performs fuzzy search on topology graph nodes.
// When threshold is 0 it uses simple case‑insensitive substring match.
// Otherwise it uses Levenshtein distance scaled to a score between 0 and 1,
// returning matches with score >= threshold.
func SearchNodes(graph *TopologyGraph, query string, threshold float64) ([]NodeMatch, error) {
    if graph == nil {
        return nil, errors.New("graph is nil")
    }
    if query == "" {
        return nil, errors.New("query is empty")
    }

    query = strings.ToLower(query)
    var results []NodeMatch

    for _, node := range graph.Nodes {
        if node == nil {
            continue
        }

        // Check node ID
        score := matchScore(node.ID, query, threshold)
        if score >= threshold {
            results = append(results, NodeMatch{
                NodeID:       node.ID,
                MatchedField: "ID",
                Score:        score,
            })
        }

        // Check every string attribute value
        for key, val := range node.Attributes {
            score := matchScore(val, query, threshold)
            if score >= threshold {
                results = append(results, NodeMatch{
                    NodeID:       node.ID,
                    MatchedField: key,
                    Score:        score,
                })
            }
        }
    }

    return results, nil
}

// matchScore computes similarity between a single value and the query.
func matchScore(value, query string, threshold float64) float64 {
    lower := strings.ToLower(value)
    if threshold == 0 {
        if strings.Contains(lower, query) {
            return 1.0
        }
        return 0.0
    }

    dist := levenshtein(lower, query)
    maxLen := len(lower)
    if len(query) > maxLen {
        maxLen = len(query)
    }
    if maxLen == 0 {
        return 1.0
    }
    return 1.0 - float64(dist)/float64(maxLen)
}

// levenshtein computes the edit distance between two strings.
func levenshtein(a, b string) int {
    alen, blen := len(a), len(b)
    if alen == 0 {
        return blen
    }
    if blen == 0 {
        return alen
    }

    d := make([][]int, alen+1)
    for i := range d {
        d[i] = make([]int, blen+1)
    }

    for i := 0; i <= alen; i++ {
        d[i][0] = i
    }
    for j := 0; j <= blen; j++ {
        d[0][j] = j
    }

    for i := 1; i <= alen; i++ {
        for j := 1; j <= blen; j++ {
            cost := 0
            if a[i-1] != b[j-1] {
                cost = 1
            }
            d[i][j] = min3(
                d[i-1][j]+1,
                d[i][j-1]+1,
                d[i-1][j-1]+cost,
            )
        }
    }
    return d[alen][blen]
}

func min3(a, b, c int) int {
    if a < b {
        if a < c {
            return a
        }
        return c
    }
    if b < c {
        return b
    }
    return c
}

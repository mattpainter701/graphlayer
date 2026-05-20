package topology

import (
    "testing"
)

func buildTestGraph() *TopologyGraph {
    return &TopologyGraph{
        Nodes: map[string]*Node{
            "node1": {
                ID:   "Router1",
                Type: "router",
                Attributes: map[string]string{
                    "hostname": "Router1-Core",
                    "os":       "IOS",
                    "location": "DataCenter1",
                },
            },
            "node2": {
                ID:   "Switch2",
                Type: "switch",
                Attributes: map[string]string{
                    "hostname": "Switch2-Dist",
                    "model":    "Catalyst",
                },
            },
            "node3": {
                ID:   "Firewall3",
                Type: "firewall",
                Attributes: map[string]string{
                    "vendor": "PaloAlto",
                    "serial": "PA-5220-001",
                },
            },
        },
    }
}

func TestSearchNodes_ExactMatch(t *testing.T) {
    graph := buildTestGraph()
    results, err := SearchNodes(graph, "Router1", 0.0)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if len(results) == 0 {
        t.Fatal("expected at least one result for exact ID")
    }
    found := false
    for _, r := range results {
        if r.NodeID == "Router1" && r.MatchedField == "ID" {
            found = true
            if r.Score != 1.0 {
                t.Errorf("expected score 1.0 for exact match, got %f", r.Score)
            }
        }
    }
    if !found {
        t.Errorf("did not find exact ID match")
    }
}

func TestSearchNodes_FuzzyMatch(t *testing.T) {
    graph := buildTestGraph()
    // "Rooter1" is close to "Router1" (Levenshtein distance 1)
    results, err := SearchNodes(graph, "Rooter1", 0.8)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if len(results) == 0 {
        t.Fatal("expected fuzzy match for 'Rooter1'")
    }
    for _, r := range results {
        if r.NodeID == "Router1" {
            if r.Score < 0.8 {
                t.Errorf("expected score >= 0.8, got %f", r.Score)
            }
            return // success
        }
    }
    t.Errorf("did not find fuzzy match for Router1")
}

func TestSearchNodes_NoMatch(t *testing.T) {
    graph := buildTestGraph()
    results, err := SearchNodes(graph, "NonExistent", 0.9)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if len(results) != 0 {
        t.Errorf("expected no results, got %d", len(results))
        for _, r := range results {
            t.Logf("unexpected match: %+v", r)
        }
    }
}

func TestSearchNodes_EmptyQuery(t *

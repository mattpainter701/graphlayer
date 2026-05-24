package graph

import (
	"reflect"
	"sort"
	"strings"
	"testing"
)

func normalizeComponents(components [][]string) [][]string {
	normalized := make([][]string, len(components))
	for i, component := range components {
		copyComponent := append([]string(nil), component...)
		sort.Strings(copyComponent)
		normalized[i] = copyComponent
	}

	sort.Slice(normalized, func(i, j int) bool {
		left := strings.Join(normalized[i], ",")
		right := strings.Join(normalized[j], ",")
		if left == right {
			return len(normalized[i]) < len(normalized[j])
		}
		return left < right
	})

	return normalized
}

func TestConnectedComponents_SingleComponent(t *testing.T) {
	g := &TopologyGraph{
		Nodes: []GraphNode{{ID: "A"}, {ID: "B"}, {ID: "C"}},
		Edges: []GraphEdge{{Source: "A", Target: "B"}, {Source: "B", Target: "C"}},
	}

	got := normalizeComponents(g.ConnectedComponents())
	want := [][]string{{"A", "B", "C"}}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestConnectedComponents_MultipleComponents(t *testing.T) {
	g := &TopologyGraph{
		Nodes: []GraphNode{{ID: "A"}, {ID: "B"}, {ID: "C"}, {ID: "D"}},
		Edges: []GraphEdge{{Source: "A", Target: "B"}, {Source: "C", Target: "D"}},
	}

	got := normalizeComponents(g.ConnectedComponents())
	want := [][]string{{"A", "B"}, {"C", "D"}}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestConnectedComponents_IsolatedNode(t *testing.T) {
	g := &TopologyGraph{
		Nodes: []GraphNode{{ID: "isolated"}},
	}

	got := normalizeComponents(g.ConnectedComponents())
	want := [][]string{{"isolated"}}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestConnectedComponents_EmptyGraph(t *testing.T) {
	g := &TopologyGraph{}

	got := g.ConnectedComponents()
	if len(got) != 0 {
		t.Fatalf("expected empty result, got %v", got)
	}
}

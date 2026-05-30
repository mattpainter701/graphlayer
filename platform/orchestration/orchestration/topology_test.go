package orchestration

import (
	"reflect"
	"testing"
)

func TestHasCycle(t *testing.T) {
	tests := []struct {
		name      string
		graphID   string
		wantCycle bool
		wantPath  []string
	}{
		{
			name:      "acyclic tree",
			graphID:   "tree",
			wantCycle: false,
			wantPath:  nil,
		},
		{
			name:      "cyclic triangle",
			graphID:   "triangle",
			wantCycle: true,
			wantPath:  []string{"A", "B", "C", "A"},
		},
		{
			name:      "single node",
			graphID:   "single",
			wantCycle: false,
			wantPath:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCycle, gotPath, err := HasCycle(tt.graphID)
			if err != nil {
				t.Fatalf("HasCycle(%q) returned error: %v", tt.graphID, err)
			}

			if gotCycle != tt.wantCycle {
				t.Fatalf("HasCycle(%q) = %v, want %v", tt.graphID, gotCycle, tt.wantCycle)
			}

			if tt.wantCycle {
				if !reflect.DeepEqual(gotPath, tt.wantPath) {
					t.Fatalf("HasCycle(%q) cycle = %#v, want %#v", tt.graphID, gotPath, tt.wantPath)
				}
				return
			}

			if len(gotPath) != 0 {
				t.Fatalf("HasCycle(%q) cycle path = %#v, want empty", tt.graphID, gotPath)
			}
		})
	}
}

func TestRandomWalk(t *testing.T) {
	path, err := RandomWalk("line", "n1", 3)
	if err != nil {
		t.Fatalf("RandomWalk returned error: %v", err)
	}

	if got, want := len(path), 4; got != want {
		t.Fatalf("RandomWalk path length = %d, want %d", got, want)
	}

	if path[0] != "n1" {
		t.Fatalf("RandomWalk start node = %q, want %q", path[0], "n1")
	}

	definition, err := getTopologyGraphDefinition("line")
	if err != nil {
		t.Fatalf("getTopologyGraphDefinition returned error: %v", err)
	}

	existing := make(map[string]struct{}, len(definition.nodes))
	for _, nodeID := range definition.nodes {
		existing[nodeID] = struct{}{}
	}

	for i, nodeID := range path {
		if _, ok := existing[nodeID]; !ok {
			t.Fatalf("RandomWalk path[%d] = %q, which is not in the graph", i, nodeID)
		}
	}
}

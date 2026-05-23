package topology

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestExportImportGraphJSONRoundTrip(t *testing.T) {
	original := &Graph{
		Nodes: []Node{
			{
				ID: "node-1",
				Attributes: map[string]string{
					"name":   "router-01",
					"vendor": "cisco",
				},
			},
			{
				ID: "node-2",
				Attributes: map[string]string{
					"name":   "switch-01",
					"vendor": "aruba",
				},
			},
		},
		Edges: []Edge{
			{
				Source: "node-1",
				Target: "node-2",
				Attributes: map[string]string{
					"link_type": "ethernet",
					"speed":     "10g",
				},
			},
		},
	}

	outputPath := filepath.Join(t.TempDir(), "graph.json")

	if err := ExportGraphJSON(original, outputPath); err != nil {
		t.Fatalf("ExportGraphJSON() error = %v", err)
	}

	imported, err := ImportGraphJSON(outputPath)
	if err != nil {
		t.Fatalf("ImportGraphJSON() error = %v", err)
	}

	if !reflect.DeepEqual(original, imported) {
		t.Fatalf("round-trip graph mismatch:\noriginal: %#v\nimported: %#v", original, imported)
	}

	if got, want := imported.Metadata.NodeCount, len(original.Nodes); got != want {
		t.Fatalf("metadata node count = %d, want %d", got, want)
	}
	if got, want := imported.Metadata.EdgeCount, len(original.Edges); got != want {
		t.Fatalf("metadata edge count = %d, want %d", got, want)
	}
	if _, err := time.Parse(time.RFC3339Nano, imported.Metadata.ExportTimestamp); err != nil {
		t.Fatalf("export timestamp is not RFC3339Nano: %v", err)
	}

	if _, err := os.Stat(outputPath); err != nil {
		t.Fatalf("exported file missing: %v", err)
	}
}

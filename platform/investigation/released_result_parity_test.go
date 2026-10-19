package investigation

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/mattpainter701/graphlayer/platform/networkpath"
	"github.com/mattpainter701/graphlayer/platform/pathanalysis"
)

// Compare the persisted contracts directly as well as opening the workspace.
// This keeps failures actionable without relaxing identity or replay validation.
func TestReleasedInvestigationResultParity(t *testing.T) {
	for version := 1; version <= 7; version++ {
		files, err := filepath.Glob(filepath.Join("testdata", fmt.Sprintf("workspace-v%d", version), "*.json"))
		if err != nil {
			t.Fatal(err)
		}
		store := &Store{snapshots: map[string][]byte{}}
		type record struct {
			Kind string          `json:"kind"`
			Data json.RawMessage `json:"data"`
		}
		records := map[string]record{}
		for _, file := range files {
			raw, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			var row record
			if err := json.Unmarshal(raw, &row); err != nil {
				t.Fatal(err)
			}
			records[file] = row
			if row.Kind == "snapshot" {
				var snap Snapshot
				if err := json.Unmarshal(row.Data, &snap); err != nil {
					t.Fatal(err)
				}
				store.snapshots[snap.ID] = row.Data
			}
		}
		for _, file := range files {
			row := records[file]
			if row.Kind != "path-investigation" && row.Kind != "network-investigation" && row.Kind != "network-exploration" {
				continue
			}
			t.Run(fmt.Sprintf("v%d/%s", version, filepath.Base(file)), func(t *testing.T) {
				var expected, actual any
				var err error
				switch row.Kind {
				case "path-investigation":
					var inv PathInvestigation
					if err := json.Unmarshal(row.Data, &inv); err != nil {
						t.Fatal(err)
					}
					var snap Snapshot
					if err := json.Unmarshal(store.snapshots[inv.SnapshotID], &snap); err != nil {
						t.Fatal(err)
					}
					expected = inv.Result
					actual, err = pathanalysis.Evaluate(pathInput(&snap, inv.Result.IngressInterface, inv.Result.OriginalFlow))
				case "network-investigation":
					var inv NetworkInvestigation
					if err := json.Unmarshal(row.Data, &inv); err != nil {
						t.Fatal(err)
					}
					in, inputErr := store.networkInput(inv.Request)
					if inputErr != nil {
						t.Fatal(inputErr)
					}
					expected = inv.Result
					actual, err = networkpath.Evaluate(in)
				case "network-exploration":
					var inv NetworkExploration
					if err := json.Unmarshal(row.Data, &inv); err != nil {
						t.Fatal(err)
					}
					in, inputErr := store.networkInput(inv.Request)
					if inputErr != nil {
						t.Fatal(inputErr)
					}
					expected = inv.Result
					actual, err = networkpath.Explore(in)
				}
				if err != nil {
					t.Fatal(err)
				}
				decode := func(v any) any {
					raw, err := json.Marshal(v)
					if err != nil {
						t.Fatal(err)
					}
					var value any
					if err := json.Unmarshal(raw, &value); err != nil {
						t.Fatal(err)
					}
					return value
				}
				if difference := releasedJSONDifference(decode(expected), decode(actual), ""); difference != "" {
					t.Fatal(difference)
				}
			})
		}
	}
}

func releasedJSONDifference(expected, actual any, path string) string {
	if reflect.DeepEqual(expected, actual) {
		return ""
	}
	switch want := expected.(type) {
	case map[string]any:
		if got, ok := actual.(map[string]any); ok {
			keys := make([]string, 0, len(want)+len(got))
			for key := range want {
				keys = append(keys, key)
			}
			for key := range got {
				if _, exists := want[key]; !exists {
					keys = append(keys, key)
				}
			}
			sort.Strings(keys)
			for _, key := range keys {
				x, xok := want[key]
				y, yok := got[key]
				if xok != yok {
					return path + "/" + key + ": field presence differs"
				}
				if difference := releasedJSONDifference(x, y, path+"/"+key); difference != "" {
					return difference
				}
			}
		}
	case []any:
		if got, ok := actual.([]any); ok {
			if len(want) != len(got) {
				return fmt.Sprintf("%s: array length %d != %d", path, len(want), len(got))
			}
			for i := range want {
				if difference := releasedJSONDifference(want[i], got[i], fmt.Sprintf("%s/%d", path, i)); difference != "" {
					return difference
				}
			}
		}
	}
	return fmt.Sprintf("%s: expected %v, actual %v", path, expected, actual)
}

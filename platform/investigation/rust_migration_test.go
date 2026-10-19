package investigation

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/mattpainter701/graphlayer/platform/firewall"
)

func TestRustMigrationReplaysEveryReleasedWorkspaceWithoutRewritingEvidence(t *testing.T) {
	for version := 1; version <= 6; version++ {
		t.Run(fmt.Sprintf("v%d", version), func(t *testing.T) {
			directory, original := copyActualWorkspace(t, fmt.Sprintf("workspace-v%d", version))
			worker := os.Getenv("GRAPHLAYER_ENGINE")
			store, err := NewStore(directory)
			if err != nil {
				t.Fatal(err)
			}
			if len(store.Summaries()) == 0 {
				t.Fatal("released snapshots were not recovered")
			}
			if err = store.Close(); err != nil {
				t.Fatal(err)
			}
			assertWorkspaceBytes(t, directory, original)
			// An installation failure must not be mistaken for corrupt saved history.
			t.Setenv("GRAPHLAYER_ENGINE", filepath.Join(t.TempDir(), "missing-worker"))
			if failed, err := NewStore(directory); !errors.Is(err, firewall.ErrAnalysisEngine) {
				if failed != nil {
					_ = failed.Close()
				}
				t.Fatalf("missing worker was not identifiable: %v", err)
			}
			assertWorkspaceBytes(t, directory, original)
			t.Setenv("GRAPHLAYER_ENGINE", worker)
			reopened, err := NewStore(directory)
			if err != nil {
				t.Fatal(err)
			}
			if err = reopened.Close(); err != nil {
				t.Fatal(err)
			}
			assertWorkspaceBytes(t, directory, original)
		})
	}
}

package investigation

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestReleasedV6BytesSurviveWorkspaceV7Transactions(t *testing.T) {
	dir, fixture := copyActualWorkspace(t, "workspace-v6")
	s, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Summaries()) != 4 || len(s.ExplorationSummaries()) != 1 {
		t.Fatal("released v6 history is incomplete")
	}
	old, err := s.NetworkExploration(s.ExplorationSummaries()[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	oldBytes, _ := json.Marshal(old)
	w, err := s.RenameWorkspace("Recovered operations", 0)
	if err != nil {
		t.Fatal(err)
	}
	site, err := s.CreateSite("Chicago", "Released evidence", w.Revision)
	if err != nil {
		t.Fatal(err)
	}
	snaps := make([]Snapshot, 0, len(old.Request.Nodes))
	for _, node := range old.Request.Nodes {
		snap, err := s.Snapshot(node.SnapshotID)
		if err != nil {
			t.Fatal(err)
		}
		snaps = append(snaps, *snap)
	}
	set, created, err := s.PublishSnapshotSet("Released baseline", site.ID, snaps, s.WorkspaceInfo().Revision)
	if err != nil || !created {
		t.Fatalf("publish legacy set: %v created=%v", err, created)
	}
	identity := s.WorkspaceInfo().ID
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	assertWorkspaceBytes(t, dir, fixture)
	reopened, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if got := reopened.WorkspaceInfo(); got.ID != identity || got.ActiveSetID != set.ID || got.Name != "Recovered operations" || len(got.Sites) != 1 {
		t.Fatalf("workspace state lost: %#v", got)
	}
	if len(reopened.Summaries()) != 4 {
		t.Fatal("set publication duplicated released snapshots")
	}
	replayed, err := reopened.NetworkExploration(old.ID)
	if err != nil {
		t.Fatal(err)
	}
	newBytes, _ := json.Marshal(replayed)
	if !bytes.Equal(oldBytes, newBytes) {
		t.Fatal("workspace metadata changed the released exploration")
	}
	assertWorkspaceBytes(t, dir, fixture)
}

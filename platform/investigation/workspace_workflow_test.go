package investigation

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
)

func TestBatchPublicationSurvivesRestartAndInvestigatesPinnedEvidence(t *testing.T) {
	b, store := newTestBrowser(t)
	sample := explorationNetworkSample()
	items := make([]map[string]string, 0, len(sample.Configs))
	for _, config := range sample.Configs {
		items = append(items, map[string]string{"name": config.Name, "config": config.Config, "vendor": config.Vendor})
	}
	resp, err := b.request(http.MethodPost, "/api/import-previews", mustJSON(map[string]any{"items": items}), true)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("preview: %d %s", resp.StatusCode, readBody(t, resp))
	}
	var preview struct {
		Preview importPreviewResponse `json:"preview"`
	}
	if err := json.Unmarshal([]byte(readBody(t, resp)), &preview); err != nil {
		t.Fatal(err)
	}
	if !preview.Preview.Valid || len(store.Summaries()) != 0 {
		t.Fatal("preview should validate all four files without publishing")
	}
	resp, err = b.request(http.MethodPost, "/api/import-previews/"+preview.Preview.ID+"/publish", mustJSON(map[string]any{"set_name": "Mixed vendor baseline", "site_id": "", "expected_revision": 0}), true)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("publish: %d %s", resp.StatusCode, readBody(t, resp))
	}
	var published struct {
		SnapshotSet SnapshotSet `json:"snapshot_set"`
	}
	if err := json.Unmarshal([]byte(readBody(t, resp)), &published); err != nil {
		t.Fatal(err)
	}
	if len(published.SnapshotSet.Members) != 4 {
		t.Fatal("publication lost members")
	}
	dir := store.dir
	b.ts.Close()
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	if len(reopened.Summaries()) != 4 || reopened.WorkspaceInfo().ActiveSetID != published.SnapshotSet.ID {
		t.Fatal("journal-only snapshots or active set were lost on restart")
	}
	req := NetworkRequest{Source: sample.Source, Destination: sample.Destination, Links: sample.Links, Flow: sample.Flow}
	for i, member := range published.SnapshotSet.Members {
		snapshot, err := reopened.Snapshot(member.SnapshotID)
		if err != nil || snapshot.Name != sample.Configs[i].Name || member.Name != sample.Configs[i].Name {
			t.Fatalf("pinned member %d did not survive: %v", i, err)
		}
		req.Nodes = append(req.Nodes, NetworkNodeRef{NodeID: sample.Configs[i].NodeID, SnapshotID: member.SnapshotID})
	}
	result, err := reopened.ExploreNetwork(req)
	if err != nil {
		t.Fatal(err)
	}
	if result.Result.Disposition != "indeterminate" || len(result.Result.Branches) != 2 {
		t.Fatalf("unexpected pinned workflow result: %#v", result.Result)
	}
	original, _ := json.Marshal(result)
	if err := reopened.Close(); err != nil {
		t.Fatal(err)
	}
	replayedStore, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer replayedStore.Close()
	replayed, err := replayedStore.NetworkExploration(result.ID)
	if err != nil {
		t.Fatal(err)
	}
	actual, _ := json.Marshal(replayed)
	if !bytes.Equal(original, actual) || replayedStore.WorkspaceInfo().ActiveSetID != published.SnapshotSet.ID {
		t.Fatal("saved investigation or active set changed after restart")
	}
}

package investigation

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mattpainter701/graphlayer/platform/networkpath"
)

func TestReleasedV5WorkspaceBytesSurviveExplorationV6Reopen(t *testing.T) {
	dir, originalBytes := copyActualWorkspace(t, "workspace-v5")
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("open released workspace-v5 fixture: %v", err)
	}
	if got := len(store.Summaries()); got != 3 {
		t.Fatalf("released workspace snapshot count = %d, want 3", got)
	}
	networks := store.NetworkSummaries()
	if len(networks) != 1 {
		t.Fatalf("released workspace network count = %d, want 1", len(networks))
	}
	oldNetwork, err := store.NetworkInvestigation(networks[0].ID)
	if err != nil {
		t.Fatalf("read released vendor network result: %v", err)
	}
	if oldNetwork.Result.Disposition != "modeled_round_trip" {
		t.Fatalf("released vendor network disposition = %q, want modeled_round_trip", oldNetwork.Result.Disposition)
	}
	oldNetworkResult, err := json.Marshal(oldNetwork.Result)
	if err != nil {
		t.Fatal(err)
	}

	exploration, err := store.ExploreNetwork(importExplorationSample(t, store))
	if err != nil {
		t.Fatalf("add exploration-v6 result to released workspace: %v", err)
	}
	if len(exploration.Result.Branches) != 2 {
		t.Fatalf("exploration branch count = %d, want 2", len(exploration.Result.Branches))
	}
	explorationJSON, err := json.Marshal(exploration)
	if err != nil {
		t.Fatal(err)
	}
	explorationPath := filepath.Join(dir, "network-exploration-"+exploration.ID+".json")
	persistedExploration, err := os.ReadFile(explorationPath)
	if err != nil {
		t.Fatalf("read persisted exploration JSON: %v", err)
	}
	var envelope diskRecord
	if err := json.Unmarshal(persistedExploration, &envelope); err != nil || envelope.Version != workspaceVersion || !bytes.Equal(envelope.Data, explorationJSON) {
		t.Fatal("persisted exploration envelope differs from its exact JSON representation")
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	assertWorkspaceBytes(t, dir, originalBytes)

	reopened, err := NewStore(dir)
	if err != nil {
		t.Fatalf("reopen mixed released-v5/exploration-v6 workspace: %v", err)
	}
	defer reopened.Close()
	reopenedOldNetwork, err := reopened.NetworkInvestigation(oldNetwork.ID)
	if err != nil {
		t.Fatalf("read released vendor network after reopen: %v", err)
	}
	reopenedOldResult, err := json.Marshal(reopenedOldNetwork.Result)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(oldNetworkResult, reopenedOldResult) {
		t.Fatal("released vendor network result changed after exploration-v6 reopen")
	}
	reopenedExploration, err := reopened.NetworkExploration(exploration.ID)
	if err != nil {
		t.Fatalf("read exploration-v6 record after reopen: %v", err)
	}
	reopenedExplorationJSON, err := json.Marshal(reopenedExploration)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(explorationJSON, reopenedExplorationJSON) {
		t.Fatal("exploration-v6 request/result JSON changed after reopen")
	}
	reopenedPersisted, err := os.ReadFile(explorationPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(persistedExploration, reopenedPersisted) {
		t.Fatal("persisted exploration bytes changed during reopen")
	}
	assertWorkspaceBytes(t, dir, originalBytes)
	if exploration.Result.EngineContractVersion != networkpath.ExplorationContractVersion {
		t.Fatalf("exploration contract version = %q", exploration.Result.EngineContractVersion)
	}
}

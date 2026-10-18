package investigation

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/mattpainter701/graphlayer/platform/firewall"
)

func TestWorkspaceTransactionsAndRevisionConflicts(t *testing.T) {
	dir := t.TempDir()
	s, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	initial := s.WorkspaceInfo()
	if initial.Revision != 0 || initial.Name != "Local workspace" {
		t.Fatalf("unexpected default workspace: %#v", initial)
	}
	if _, err = s.RenameWorkspace("Operations", 1); !errors.Is(err, ErrWorkspaceConflict) {
		t.Fatalf("stale rename = %v", err)
	}
	w, err := s.RenameWorkspace("Operations", 0)
	if err != nil {
		t.Fatal(err)
	}
	site, err := s.CreateSite("Branch A", "", w.Revision)
	if err != nil {
		t.Fatal(err)
	}
	snap, created, err := s.Import("router", firewall.SampleASA())
	if err != nil || !created {
		t.Fatalf("import: created=%v err=%v", created, err)
	}
	set, created, err := s.PublishSnapshotSet("Baseline", site.ID, []Snapshot{*snap}, s.WorkspaceInfo().Revision)
	if err != nil || !created {
		t.Fatalf("publish: created=%v err=%v", created, err)
	}
	if s.WorkspaceInfo().ActiveSetID != set.ID {
		t.Fatal("published set was not activated atomically")
	}
	if _, created, err = s.PublishSnapshotSet("Baseline", site.ID, []Snapshot{*snap}, s.WorkspaceInfo().Revision); err != nil || created {
		t.Fatalf("idempotent publish: created=%v err=%v", created, err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if got := s.WorkspaceInfo(); got.Revision != 3 || got.ActiveSetID != set.ID || len(got.Sites) != 1 {
		t.Fatalf("reopened workspace = %#v", got)
	}
	if got := s.SnapshotSets(); len(got) != 1 || got[0].ID != set.ID {
		t.Fatalf("reopened sets = %#v", got)
	}
}

func TestWorkspaceInputAndOutputAreDetached(t *testing.T) {
	s, _ := openTestStore(t)
	site, err := s.CreateSite("Branch A", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	site.Name = "changed"
	w := s.WorkspaceInfo()
	w.Sites[0].Name = "changed"
	if s.WorkspaceInfo().Sites[0].Name != "Branch A" {
		t.Fatal("workspace output aliases internal state")
	}
}

func TestRepublishingExistingSetActivatesItWithoutDuplicatingSnapshots(t *testing.T) {
	s, _ := openTestStore(t)
	snap, _, err := s.Import("edge.cfg", firewall.SampleASA())
	if err != nil {
		t.Fatal(err)
	}
	a, _, err := s.PublishSnapshotSet("A", "", []Snapshot{*snap}, 0)
	if err != nil {
		t.Fatal(err)
	}
	b, _, err := s.PublishSnapshotSet("B", "", []Snapshot{*snap}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if s.WorkspaceInfo().ActiveSetID != b.ID {
		t.Fatal("B must start active")
	}
	again, created, err := s.PublishSnapshotSet("A", "", []Snapshot{*snap}, 2)
	if err != nil || created || again.ID != a.ID {
		t.Fatalf("reuse A = %#v, %v, %v", again, created, err)
	}
	if got := s.WorkspaceInfo(); got.ActiveSetID != a.ID || got.Revision != 3 {
		t.Fatalf("reused set was not activated: %#v", got)
	}
	if len(s.Summaries()) != 1 || len(s.SnapshotSets()) != 2 {
		t.Fatal("reactivation duplicated immutable records")
	}
}

func TestWorkspacePublishFailuresLeaveStateAndDiskUnchanged(t *testing.T) {
	dir := t.TempDir()
	s, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	snap, _, err := s.Import("canonical", firewall.SampleASA())
	if err != nil {
		t.Fatal(err)
	}
	before := s.WorkspaceInfo()
	entriesBefore, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.PublishSnapshotSet("bad", "missing-site", []Snapshot{*snap}, before.Revision); !errors.Is(err, ErrWorkspaceInvalid) {
		t.Fatalf("unknown site error = %v", err)
	}
	badName := *snap
	badName.Name = string(make([]byte, 129))
	if _, _, err := s.PublishSnapshotSet("bad-name", "", []Snapshot{badName}, before.Revision); !errors.Is(err, ErrWorkspaceInvalid) {
		t.Fatalf("invalid member name error = %v", err)
	}
	if got := s.WorkspaceInfo(); got.Revision != before.Revision || len(s.SnapshotSets()) != 0 {
		t.Fatal("invalid publish mutated in-memory state")
	}
	entriesAfter, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entriesAfter) != len(entriesBefore) {
		t.Fatal("invalid publish wrote a journal record")
	}
	// Force CreateTemp to fail after validation and verify the prepared state is discarded.
	s.dir = filepath.Join(dir, "missing-directory")
	if _, _, err := s.PublishSnapshotSet("write-failure", "", []Snapshot{*snap}, before.Revision); err == nil {
		t.Fatal("expected journal write failure")
	}
	if got := s.WorkspaceInfo(); got.Revision != before.Revision || len(s.SnapshotSets()) != 0 {
		t.Fatal("failed write mutated in-memory state")
	}
	s.dir = dir
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if got := s.WorkspaceInfo(); got.Revision != before.Revision || len(s.SnapshotSets()) != 0 {
		t.Fatal("failed publish survived reopen")
	}
}

func TestWorkspaceUnassignedSetsAndQuota(t *testing.T) {
	s, _ := openTestStore(t)
	snap, _, err := s.Import("canonical", firewall.SampleASA())
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < maxSnapshotSets; i++ {
		name := "set-" + string(rune('A'+i%26)) + string(rune('A'+i/26))
		if _, created, err := s.PublishSnapshotSet(name, "", []Snapshot{*snap}, s.WorkspaceInfo().Revision); err != nil || !created {
			t.Fatalf("publish set %d: created=%v err=%v", i, created, err)
		}
	}
	rev := s.WorkspaceInfo().Revision
	if _, _, err := s.PublishSnapshotSet("over-limit", "", []Snapshot{*snap}, rev); !errors.Is(err, ErrQuota) {
		t.Fatalf("65th set error = %v", err)
	}
	if s.WorkspaceInfo().Revision != rev || len(s.SnapshotSets()) != maxSnapshotSets {
		t.Fatal("over-quota publish changed state")
	}
}

func TestWorkspaceIdentitySurvivesCopyAndJournalTamperingIsRejected(t *testing.T) {
	dir := t.TempDir()
	s, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	w, err := s.RenameWorkspace("Copied", 0)
	if err != nil {
		t.Fatal(err)
	}
	identity := w.ID
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	copyDir := t.TempDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() == ".workspace.lock" {
			continue
		}
		data, e := os.ReadFile(filepath.Join(dir, entry.Name()))
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(filepath.Join(copyDir, entry.Name()), data, 0600); e != nil {
			t.Fatal(e)
		}
	}
	copied, err := NewStore(copyDir)
	if err != nil {
		t.Fatalf("reopen copied workspace: %v", err)
	}
	if copied.WorkspaceInfo().ID != identity {
		t.Fatal("workspace identity changed after copying directory")
	}
	if err := copied.Close(); err != nil {
		t.Fatal(err)
	}
	mutateRecord(t, copyDir, "workspace-transaction", func(record *diskRecord) {
		var tx workspaceTransaction
		if json.Unmarshal(record.Data, &tx) != nil {
			t.Fatal("decode transaction")
		}
		tx.Workspace.Name = "forged"
		record.Data, _ = json.Marshal(tx)
		record.SHA256 = digest(record.Data)
	})
	if bad, err := NewStore(copyDir); err == nil {
		_ = bad.Close()
		t.Fatal("accepted transaction tampered with a recomputed envelope checksum")
	}
}

func TestWorkspaceTransactionGapIsRejected(t *testing.T) {
	dir := t.TempDir()
	s, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.RenameWorkspace("Journal", 0); err != nil {
		t.Fatal(err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, entry := range entries {
		if entry.Name() == ".workspace.lock" || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var record diskRecord
		if err = json.Unmarshal(raw, &record); err != nil {
			t.Fatal(err)
		}
		if record.Kind != "workspace-transaction" {
			continue
		}
		var tx workspaceTransaction
		if err = json.Unmarshal(record.Data, &tx); err != nil {
			t.Fatal(err)
		}
		tx.PrevID = digest([]byte("missing predecessor"))
		payload := transactionPayload{tx.PrevID, tx.Workspace, tx.Set, tx.NewSnapshots, tx.CreatedAt}
		payloadBytes, _ := json.Marshal(payload)
		tx.ID = digest(payloadBytes)
		record.Data, _ = json.Marshal(tx)
		record.SHA256 = digest(record.Data)
		encoded, _ := json.Marshal(record)
		newPath := filepath.Join(dir, "workspace-transaction-"+tx.ID+".json")
		if err = os.WriteFile(newPath, encoded, 0600); err != nil {
			t.Fatal(err)
		}
		if err = os.Remove(path); err != nil {
			t.Fatal(err)
		}
		found = true
		break
	}
	if !found {
		t.Fatal("workspace transaction record was not created")
	}
	if bad, err := NewStore(dir); err == nil {
		_ = bad.Close()
		t.Fatal("accepted a validly rehashed journal with a missing predecessor")
	}
}

func TestWorkspaceSetReuseKeepsCanonicalSnapshotBytes(t *testing.T) {
	s, _ := openTestStore(t)
	snap, _, err := s.Import("canonical", firewall.SampleASA())
	if err != nil {
		t.Fatal(err)
	}
	copy := *snap
	copy.Name = "source-file-name"
	if _, created, err := s.PublishSnapshotSet("baseline", "", []Snapshot{copy}, 0); err != nil || !created {
		t.Fatalf("publish: created=%v err=%v", created, err)
	}
	got, err := s.Snapshot(snap.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "canonical" {
		t.Fatalf("canonical snapshot name changed to %q", got.Name)
	}
}

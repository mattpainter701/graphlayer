package investigation

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/mattpainter701/graphlayer/platform/firewall"
	"github.com/mattpainter701/graphlayer/platform/policy"
)

func openTestStore(t *testing.T) (*Store, string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "workspace")
	s, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s, dir
}

func sampleFlow(port int) policy.Flow {
	return policy.Flow{Source: "192.0.2.19", Destination: "10.20.4.15", Protocol: "tcp", SourcePort: 51000, DestinationPort: port}
}

func TestStoreImportEvaluateReplayAndImmutableEvidence(t *testing.T) {
	s, dir := openTestStore(t)
	raw := firewall.SampleASA()
	snap, created, err := s.Import("edge-fw", raw)
	if err != nil || !created {
		t.Fatalf("Import() = (%v, %v, %v)", snap, created, err)
	}
	if snap.ArtifactSHA256 != digest(raw) {
		t.Fatalf("artifact hash = %q", snap.ArtifactSHA256)
	}
	if len(snap.Chains) != 1 {
		t.Fatalf("chain count = %d", len(snap.Chains))
	}
	chain := snap.Chains[0]
	evidenceByID := make(map[string]firewall.Evidence, len(snap.Evidence))
	for _, evidence := range snap.Evidence {
		if evidence.ID == "" || evidence.StartLine < 1 || evidence.EndLine < evidence.StartLine || evidence.EndLine > len(snap.SourceLines) {
			t.Fatalf("invalid persisted source evidence: %#v", evidence)
		}
		if _, duplicate := evidenceByID[evidence.ID]; duplicate {
			t.Fatalf("duplicate evidence identity %q", evidence.ID)
		}
		evidenceByID[evidence.ID] = evidence
	}
	for _, rule := range chain.Rules {
		if rule.ID == "" {
			t.Fatal("persisted rule lacks a stable identity")
		}
		for _, ref := range rule.EvidenceRefs {
			if _, ok := evidenceByID[ref]; !ok {
				t.Fatalf("rule %s references missing evidence %q", rule.ID, ref)
			}
		}
	}
	for _, tc := range []struct {
		port int
		want string
	}{{443, "permit"}, {22, "deny"}, {8443, "indeterminate"}} {
		inv, err := s.Evaluate(snap.ID, chain.ID, sampleFlow(tc.port))
		if err != nil {
			t.Fatalf("Evaluate port %d: %v", tc.port, err)
		}
		if inv.Result.Disposition != tc.want {
			t.Errorf("port %d disposition = %q, want %q", tc.port, inv.Result.Disposition, tc.want)
		}
		if inv.SnapshotID != snap.ID || inv.ChainID != chain.ID || inv.Result.Scope.SnapshotID != snap.ID {
			t.Errorf("investigation scope drift: %#v", inv)
		}
		if tc.port == 443 || tc.port == 22 {
			if inv.Result.DecisiveRule == "" || len(inv.Result.EvidenceRefs) == 0 {
				t.Errorf("port %d lacks decisive rule/evidence: %#v", tc.port, inv.Result)
			}
		} else if len(inv.Result.EvidenceRefs) == 0 {
			t.Errorf("unresolved candidate lacks evidence: %#v", inv.Result)
		}
	}
	if len(s.Investigations()) != 3 {
		t.Fatalf("history count = %d", len(s.Investigations()))
	}
	before, err := json.Marshal(snap)
	if err != nil {
		t.Fatal(err)
	}
	// Returned records are detached copies; callers cannot rewrite the stored manifest.
	snap.SourceLines[0].Text = "caller mutation"
	snap.Chains[0].Rules[0].Action = "deny"
	after, err := s.Snapshot(snap.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.SourceLines[0].Text == "caller mutation" || after.Chains[0].Rules[0].Action == "deny" {
		t.Fatal("mutating returned snapshot changed stored state")
	}
	_ = before

	// Duplicate imports resolve to the original immutable snapshot, even if the display name differs.
	dup, newRecord, err := s.Import("renamed", raw)
	if err != nil || newRecord || dup.ID != after.ID || dup.Name != "edge-fw" {
		t.Fatalf("duplicate import = (%#v, %v, %v)", dup, newRecord, err)
	}
	old, err := s.Evaluate(after.ID, after.Chains[0].ID, sampleFlow(443))
	if err != nil {
		t.Fatal(err)
	}
	oldJSON, _ := json.Marshal(old)

	// Changing only source bytes changes artifact identity and adds a new manifest.
	changed := append([]byte(nil), raw...)
	changed = append(changed, []byte("! harmless source revision\n")...)
	newSnap, newRecord, err := s.Import("edge-fw revision", changed)
	if err != nil || !newRecord || newSnap.ID == after.ID {
		t.Fatalf("changed import = (%#v, %v, %v)", newSnap, newRecord, err)
	}
	oldAgain, err := s.Investigation(old.ID)
	if err != nil {
		t.Fatal(err)
	}
	oldAgainJSON, _ := json.Marshal(oldAgain)
	if string(oldJSON) != string(oldAgainJSON) {
		t.Fatal("new source revision mutated an earlier saved result")
	}

	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := NewStore(dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer reopened.Close()
	gotSnap, err := reopened.Snapshot(after.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotSnap, after) {
		t.Fatal("snapshot differs after restart")
	}
	gotInv, err := reopened.Investigation(old.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotInv, oldAgain) {
		t.Fatal("investigation differs after restart")
	}
	if len(reopened.Summaries()) != 2 || len(reopened.Investigations()) != 3 {
		t.Fatal("restart lost manifests or history")
	}
	if runtime.GOOS != "windows" {
		for _, path := range []string{dir, filepath.Join(dir, "snapshot-"+after.ID+".json"), filepath.Join(dir, "investigation-"+old.ID+".json")} {
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode().Perm()&0077 != 0 {
				t.Errorf("permissions on %s are %o", path, info.Mode().Perm())
			}
		}
	}
}

func TestStoreFailedImportsAndWritesPreservePriorState(t *testing.T) {
	s, dir := openTestStore(t)
	prior, _, err := s.Import("known", firewall.SampleASA())
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = s.Import("bad", []byte("hostname broken\n"+strings.Repeat("x", 8200))); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("parse error = %v", err)
	}
	if len(s.Summaries()) != 1 || s.Summaries()[0].ID != prior.ID {
		t.Fatal("failed parse changed existing snapshots")
	}
	// Force the atomic publisher to fail at the target path; no in-memory entry may appear.
	raw := append(append([]byte(nil), firewall.SampleASA()...), []byte("! distinct artifact\n")...)
	parsed, err := firewall.ParseASAConfig(raw)
	if err != nil {
		t.Fatal(err)
	}
	id := snapshotID(*parsed)
	if err := os.Mkdir(filepath.Join(dir, "snapshot-"+id+".json"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Import("write-failure", raw); err == nil {
		t.Fatal("expected publish failure")
	}
	if len(s.Summaries()) != 1 || s.Summaries()[0].ID != prior.ID {
		t.Fatal("failed write changed in-memory snapshot map")
	}
}

func TestStoreExclusiveWorkspaceLockAndRefusedDirectoryPermissions(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "shared")
	if err := os.Mkdir(dir, 0751); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "keep.txt"), []byte("user file"), 0600); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(dir, 0751); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := NewStore(dir); err == nil {
		t.Fatal("opened a directory containing unrelated files")
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(dir)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0751 {
			t.Fatalf("refused directory mode changed to %o", info.Mode().Perm())
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "keep.txt")); err != nil {
		t.Fatal("refused workspace changed existing contents:", err)
	}

	lockedDir := filepath.Join(t.TempDir(), "workspace")
	first, err := NewStore(lockedDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = NewStore(lockedDir); err == nil {
		t.Fatal("second opener acquired an active workspace")
	}
	if err = first.Close(); err != nil {
		t.Fatal(err)
	}
	second, err := NewStore(lockedDir)
	if err != nil {
		t.Fatalf("workspace did not reopen after close: %v", err)
	}
	if err = second.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestStoreEnforcesSnapshotAndInvestigationQuotas(t *testing.T) {
	t.Run("snapshots", func(t *testing.T) {
		s, _ := openTestStore(t)
		for i := 0; i < maxSnapshots; i++ {
			raw := append(append([]byte(nil), firewall.SampleASA()...), []byte(fmt.Sprintf("! revision %d\n", i))...)
			if _, _, err := s.Import("revision", raw); err != nil {
				t.Fatalf("snapshot %d: %v", i, err)
			}
		}
		raw := append(append([]byte(nil), firewall.SampleASA()...), []byte("! quota overflow\n")...)
		if _, _, err := s.Import("overflow", raw); !errors.Is(err, ErrQuota) {
			t.Fatalf("snapshot overflow = %v", err)
		}
		if len(s.Summaries()) != maxSnapshots {
			t.Fatalf("snapshot count = %d", len(s.Summaries()))
		}
	})
	t.Run("investigations", func(t *testing.T) {
		s, _ := openTestStore(t)
		snap, _, err := s.Import("edge", firewall.SampleASA())
		if err != nil {
			t.Fatal(err)
		}
		for i := 1; i <= maxInvestigations; i++ {
			flow := sampleFlow(443)
			flow.SourcePort = i
			if _, err := s.Evaluate(snap.ID, snap.Chains[0].ID, flow); err != nil {
				t.Fatalf("investigation %d: %v", i, err)
			}
		}
		flow := sampleFlow(443)
		flow.SourcePort = maxInvestigations + 1
		if _, err := s.Evaluate(snap.ID, snap.Chains[0].ID, flow); !errors.Is(err, ErrQuota) {
			t.Fatalf("investigation overflow = %v", err)
		}
		if len(s.Investigations()) != maxInvestigations {
			t.Fatalf("investigation count = %d", len(s.Investigations()))
		}
	})
}

func TestStoreRejectsCorruptRecordsAndReferenceScopeTampering(t *testing.T) {
	t.Run("checksum", func(t *testing.T) {
		s, dir := openTestStore(t)
		snap, _, err := s.Import("edge", firewall.SampleASA())
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Close(); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, "snapshot-"+snap.ID+".json")
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		b = []byte(strings.Replace(string(b), "edge-fw", "edXe-fw", 1))
		if err = os.WriteFile(path, b, 0600); err != nil {
			t.Fatal(err)
		}
		if reopened, openErr := NewStore(dir); openErr == nil {
			_ = reopened.Close()
			t.Fatal("opened workspace with corrupt record checksum")
		}
	})
	t.Run("cross-snapshot-reference", func(t *testing.T) {
		s, dir := openTestStore(t)
		snap, _, err := s.Import("edge", firewall.SampleASA())
		if err != nil {
			t.Fatal(err)
		}
		inv, err := s.Evaluate(snap.ID, snap.Chains[0].ID, sampleFlow(443))
		if err != nil {
			t.Fatal(err)
		}
		if err = s.Close(); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, "investigation-"+inv.ID+".json")
		outer, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var rec diskRecord
		if err = json.Unmarshal(outer, &rec); err != nil {
			t.Fatal(err)
		}
		var stored Investigation
		if err = json.Unmarshal(rec.Data, &stored); err != nil {
			t.Fatal(err)
		}
		stored.Result.Scope.SnapshotID = strings.Repeat("0", 64)
		rec.Data, _ = json.Marshal(stored)
		rec.SHA256 = digest(rec.Data)
		outer, _ = json.Marshal(rec)
		if err = os.WriteFile(path, outer, 0600); err != nil {
			t.Fatal(err)
		}
		if _, err = NewStore(dir); err == nil {
			t.Fatal("opened workspace with cross-snapshot result scope")
		}
	})
}

package investigation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mattpainter701/graphlayer/platform/firewall"
	"github.com/mattpainter701/graphlayer/platform/policy"
)

// The records in testdata were written by the released v1 implementation. Copy
// them byte-for-byte before opening: NewStore is allowed to create a lock file,
// but compatibility checks must never mutate the committed evidence.
func copyWorkspaceV1(t *testing.T) (string, map[string][]byte) {
	t.Helper()
	source := filepath.Join("testdata", "workspace-v1")
	dest := t.TempDir()
	entries, err := os.ReadDir(source)
	if err != nil {
		t.Fatal(err)
	}
	want := make(map[string][]byte, len(entries))
	for _, entry := range entries {
		b, err := os.ReadFile(filepath.Join(source, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		want[entry.Name()] = append([]byte(nil), b...)
		if err := os.WriteFile(filepath.Join(dest, entry.Name()), b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	return dest, want
}

func assertWorkspaceBytes(t *testing.T, dir string, want map[string][]byte) {
	t.Helper()
	for name, expected := range want {
		got, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(expected) {
			t.Errorf("legacy record %s was rewritten", name)
		}
	}
}

func TestWorkspaceV1CompatibilityAndMixedReopen(t *testing.T) {
	dir, fixture := copyWorkspaceV1(t)
	s, err := NewStore(dir)
	if err != nil {
		t.Fatalf("open released workspace-v1 fixture: %v", err)
	}
	if got := len(s.Investigations()); got != 4 {
		t.Fatalf("legacy history count = %d, want 4", got)
	}
	for _, tc := range []struct {
		port int
		want string
	}{{443, "permit"}, {22, "deny"}, {8443, "indeterminate"}, {80, "deny"}} {
		var found bool
		for _, old := range s.Investigations() {
			inv, err := s.Investigation(old.ID)
			if err != nil {
				t.Fatal(err)
			}
			if inv.Result.Flow.DestinationPort != tc.port {
				continue
			}
			found = true
			if inv.Result.Disposition != tc.want {
				t.Errorf("legacy port %d = %s, want %s", tc.port, inv.Result.Disposition, tc.want)
			}
			if tc.port == 80 && !inv.Result.UsedDefault {
				t.Errorf("port 80 legacy result was not marked as the implicit default: %#v", inv.Result)
			}
			if inv.Result.EngineContractVersion != "graphlayer-policy-v2" {
				t.Errorf("legacy engine = %q", inv.Result.EngineContractVersion)
			}
			if len(inv.Result.RuleTraces) != 0 {
				t.Errorf("legacy result fabricated traces: %#v", inv.Result.RuleTraces)
			}
		}
		if !found {
			t.Errorf("no saved v1 result for port %d", tc.port)
		}
	}
	// A new evaluation in the current engine can coexist with immutable legacy rows.
	snapshots := s.Summaries()
	if len(snapshots) != 1 {
		t.Fatalf("legacy snapshot count = %d", len(snapshots))
	}
	snap, err := s.Snapshot(snapshots[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	flow := policy.Flow{Source: "192.0.2.42", Destination: "10.20.4.15", Protocol: "tcp", SourcePort: 52000, DestinationPort: 443}
	current, err := s.Evaluate(snap.ID, snap.Chains[0].ID, flow)
	if err != nil {
		t.Fatalf("evaluate current engine against v1 snapshot: %v", err)
	}
	if len(current.Result.RuleTraces) == 0 {
		t.Fatal("current evaluation has no predicate traces")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	assertWorkspaceBytes(t, dir, fixture)
	reopened, err := NewStore(dir)
	if err != nil {
		t.Fatalf("reopen mixed v1/v2 workspace: %v", err)
	}
	defer reopened.Close()
	if len(reopened.Investigations()) != 5 {
		t.Fatalf("mixed history count = %d, want 5", len(reopened.Investigations()))
	}
	assertWorkspaceBytes(t, dir, fixture)
}

func TestWorkspaceV2RejectsCorruptOrDowngradedRecords(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(t *testing.T, dir string)
	}{
		{"unknown workspace version", func(t *testing.T, dir string) {
			mutateRecord(t, dir, "snapshot", func(r *diskRecord) { r.Version = "future-workspace" })
		}},
		{"tampered trace with recomputed envelope", func(t *testing.T, dir string) {
			mutateRecord(t, dir, "investigation", func(r *diskRecord) {
				var v map[string]any
				_ = json.Unmarshal(r.Data, &v)
				result := v["result"].(map[string]any)
				traces := result["rule_traces"].([]any)
				traces[0].(map[string]any)["outcome"] = "forged"
				r.Data, _ = json.Marshal(v)
				h := sha256.Sum256(r.Data)
				r.SHA256 = hex.EncodeToString(h[:])
			})
		}},
		{"downgraded current result", func(t *testing.T, dir string) {
			mutateRecord(t, dir, "investigation", func(r *diskRecord) { r.Version = legacyWorkspaceVersion })
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			s, err := NewStore(dir)
			if err != nil {
				t.Fatal(err)
			}
			snap, _, err := s.Import("sample", []byte("hostname edge\naccess-list IN extended permit tcp any any eq 443\naccess-group IN in interface outside\n"))
			if err != nil {
				t.Fatal(err)
			}
			inv, err := s.Evaluate(snap.ID, snap.Chains[0].ID, policy.Flow{Source: "192.0.2.1", Destination: "192.0.2.2", Protocol: "tcp", SourcePort: 50000, DestinationPort: 443})
			if err != nil {
				t.Fatal(err)
			}
			_ = inv
			_ = s.Close()
			tc.mutate(t, dir)
			bad, err := NewStore(dir)
			if err == nil {
				_ = bad.Close()
				t.Fatal("accepted invalid workspace record")
			}
		})
	}
}

func TestWorkspaceV2RejectsInvalidInventorySemantics(t *testing.T) {
	mutations := []struct {
		name   string
		mutate func(t *testing.T, snapshot map[string]any)
	}{
		{"origin object reference crosses namespaces", func(t *testing.T, snapshot map[string]any) {
			origins := snapshot["rule_origins"].([]any)
			origins[0].(map[string]any)["source_object_ids"] = []any{"service_group:WEB-APPS"}
		}},
		{"incomplete rule origin coverage", func(t *testing.T, snapshot map[string]any) {
			origins := snapshot["rule_origins"].([]any)
			snapshot["rule_origins"] = origins[:len(origins)-1]
		}},
		{"resolved dependency points to missing object", func(t *testing.T, snapshot map[string]any) {
			for _, raw := range snapshot["objects"].([]any) {
				obj := raw.(map[string]any)
				if obj["kind"] != "service_group" {
					continue
				}
				for _, memberRaw := range obj["members"].([]any) {
					member := memberRaw.(map[string]any)
					if member["kind"] == "reference" {
						member["object_id"] = "service_group:DOES-NOT-EXIST"
						return
					}
				}
			}
			t.Fatal("fixture has no service-group dependency edge")
		}},
		{"malformed address member literal", func(t *testing.T, snapshot map[string]any) {
			for _, raw := range snapshot["objects"].([]any) {
				obj := raw.(map[string]any)
				if obj["kind"] != "network_group" {
					continue
				}
				for _, memberRaw := range obj["members"].([]any) {
					member := memberRaw.(map[string]any)
					if member["kind"] == "address" {
						member["value"] = "192.0.2.999/24"
						return
					}
				}
			}
			t.Fatal("fixture has no network address member")
		}},
		{"noncanonical masked prefix", func(t *testing.T, snapshot map[string]any) {
			for _, raw := range snapshot["objects"].([]any) {
				obj := raw.(map[string]any)
				if obj["kind"] != "network_group" {
					continue
				}
				for _, memberRaw := range obj["members"].([]any) {
					member := memberRaw.(map[string]any)
					if member["kind"] == "address" {
						member["value"] = "192.0.2.15/24"
						return
					}
				}
			}
			t.Fatal("fixture has no network address member")
		}},
		{"resolved dependency chain exceeds depth limit", func(t *testing.T, snapshot map[string]any) {
			objects := snapshot["objects"].([]any)
			evidence := snapshot["evidence"].([]any)[0].(map[string]any)["id"]
			child := "service_object:WEB-HTTPS"
			for index := 17; index >= 0; index-- {
				name := "deep-" + strconv.Itoa(index)
				id := "service_group:" + name
				objects = append(objects, map[string]any{
					"id": id, "name": name, "kind": "service_group", "resolved": true,
					"evidence_refs": []any{evidence},
					"members":       []any{map[string]any{"kind": "reference", "object_id": child, "evidence_refs": []any{evidence}}},
				})
				child = id
			}
			snapshot["objects"] = objects
		}},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			s, err := NewStore(dir)
			if err != nil {
				t.Fatal(err)
			}
			if _, _, err := s.Import("service fixture", firewall.SampleServicesASA()); err != nil {
				t.Fatalf("import validator fixture: %v", err)
			}
			_ = s.Close()
			mutateRecord(t, dir, "snapshot", func(record *diskRecord) {
				var snapshot map[string]any
				if err := json.Unmarshal(record.Data, &snapshot); err != nil {
					t.Fatal(err)
				}
				tc.mutate(t, snapshot)
				record.Data, err = json.Marshal(snapshot)
				if err != nil {
					t.Fatal(err)
				}
				h := sha256.Sum256(record.Data)
				record.SHA256 = hex.EncodeToString(h[:])
			})
			bad, err := NewStore(dir)
			if err == nil {
				_ = bad.Close()
				t.Fatal("accepted semantically invalid inventory record with valid envelope checksum")
			}
		})
	}
}

func TestStoreOversizedRecordWritePublishesNothing(t *testing.T) {
	s, dir := openTestStore(t)
	data, err := json.Marshal(strings.Repeat("x", maxRecordBytes))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.write("snapshot", strings.Repeat("a", 64), data); !errors.Is(err, ErrRecordTooLarge) {
		t.Fatalf("oversized record write error = %v, want ErrRecordTooLarge", err)
	}
	if len(s.Summaries()) != 0 || len(s.Investigations()) != 0 {
		t.Fatal("oversized snapshot partially changed workspace state")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != ".workspace.lock" {
		t.Fatalf("oversized import left a partial workspace artifact: %#v", entries)
	}
}

func mutateRecord(t *testing.T, dir string, kind string, mutate func(*diskRecord)) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var r diskRecord
		if err = json.Unmarshal(b, &r); err != nil {
			t.Fatal(err)
		}
		if r.Kind != kind {
			continue
		}
		mutate(&r)
		out, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(path, out, 0600); err != nil {
			t.Fatal(err)
		}
		return
	}
	t.Fatalf("no %s workspace record", kind)
}

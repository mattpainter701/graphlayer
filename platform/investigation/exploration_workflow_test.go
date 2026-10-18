package investigation

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func importExplorationSample(t *testing.T, s *Store) NetworkRequest {
	t.Helper()
	sample := explorationNetworkSample()
	req := NetworkRequest{Links: sample.Links, Source: sample.Source, Destination: sample.Destination, Flow: sample.Flow}
	for _, item := range sample.Configs {
		snap, _, err := s.ImportVendor(item.Name, []byte(item.Config), item.Vendor)
		if err != nil {
			t.Fatalf("import %s: %v", item.NodeID, err)
		}
		req.Nodes = append(req.Nodes, NetworkNodeRef{NodeID: item.NodeID, SnapshotID: snap.ID})
	}
	return req
}

func TestExplorationNativeDiamondKeepsSymmetricAndAsymmetricEvidence(t *testing.T) {
	s, dir := openTestStore(t)
	req := importExplorationSample(t, s)
	inv, err := s.ExploreNetwork(req)
	if err != nil {
		t.Fatal(err)
	}
	if !inv.Result.Complete || inv.Result.Truncated || len(inv.Result.Branches) != 2 || inv.Result.Disposition != "indeterminate" {
		t.Fatalf("expected two complete alternatives with mixed state certainty: %#v", inv.Result)
	}
	var symmetric, asymmetric bool
	for _, branch := range inv.Result.Branches {
		if len(branch.Forward.Hops) != 3 || len(branch.Returns) != 1 {
			t.Fatalf("missing per-branch traces: %#v", branch)
		}
		ret := branch.Returns[0]
		if branch.Forward.Hops[1].NodeID == "north" {
			asymmetric = true
			if !ret.Asymmetric || ret.Disposition != "indeterminate" || len(ret.Direction.Hops) != 3 || ret.Direction.Hops[1].NodeID != "south" {
				t.Fatalf("north branch concealed asymmetric route/state: %#v", ret)
			}
		} else if branch.Forward.Hops[1].NodeID == "south" {
			symmetric = true
			if ret.Asymmetric || branch.Disposition != "modeled_round_trip" {
				t.Fatalf("south branch did not retain supported symmetric result: %#v", branch)
			}
		}
		for _, hop := range append(branch.Forward.Hops, ret.Direction.Hops...) {
			if len(hop.Evidence) == 0 {
				t.Fatal("hop lacks snapshot-qualified source evidence")
			}
			for _, e := range hop.Evidence {
				owner, err := s.Snapshot(e.SnapshotID)
				if err != nil {
					t.Fatal(err)
				}
				found := false
				for _, ref := range owner.Evidence {
					if ref.ID == e.Ref && ref.StartLine > 0 && ref.EndLine <= len(owner.SourceLines) {
						found = true
					}
				}
				if !found {
					t.Fatalf("hop references missing source evidence: %#v", e)
				}
			}
		}
	}
	if !symmetric || !asymmetric {
		t.Fatal("missing one configured route alternative")
	}
	before, _ := json.Marshal(inv)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	after, err := reopened.NetworkExploration(inv.ID)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(after)
	if string(before) != string(encoded) {
		t.Fatal("exploration changed after immutable replay")
	}
}

func TestExplorationSharedQuotaIdempotencyAndDetachedRecords(t *testing.T) {
	s, _ := openTestStore(t)
	req := importExplorationSample(t, s)
	first, err := s.ExploreNetwork(req)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := json.Marshal(first)
	for i := 0; i < maxInvestigations-1; i++ {
		id := digest([]byte{byte(i), byte(i >> 8)})
		s.networks[id] = []byte(`{}`)
	}
	duplicate, err := s.ExploreNetwork(req)
	if err != nil || duplicate.ID != first.ID {
		t.Fatalf("exact duplicate at shared quota = (%v, %v)", duplicate, err)
	}
	req.Flow.SourcePort++
	if _, err := s.ExploreNetwork(req); !errors.Is(err, ErrQuota) {
		t.Fatalf("new exploration at shared quota: %v", err)
	}
	req.Nodes[0].NodeID = "caller-mutated"
	first.Request.Nodes[0].NodeID = "returned-mutated"
	first.Result.Branches[0].Forward.Hops[0].Path.Reasons = []string{"returned-mutated"}
	got, err := s.NetworkExploration(first.ID)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(got)
	if string(want) != string(encoded) || len(s.explorations) != 1 {
		t.Fatal("canonical exploration changed through a caller alias or quota rejection")
	}
}

func TestHTTPExplorationStrictRequestsAndExactResultExport(t *testing.T) {
	b, s := newTestBrowser(t)
	req := importExplorationSample(t, s)
	payload, _ := json.Marshal(req)
	for _, verified := range []bool{false, true} {
		resp, err := b.request(http.MethodPost, "/api/network-explorations", payload, verified)
		if err != nil {
			t.Fatal(err)
		}
		want := http.StatusOK
		if !verified {
			want = http.StatusForbidden
		}
		body := readBody(t, resp)
		if resp.StatusCode != want {
			t.Fatalf("exploration status=%d want=%d body=%s", resp.StatusCode, want, body)
		}
		if verified {
			var result struct {
				Exploration NetworkExploration `json:"exploration"`
			}
			if err := json.Unmarshal([]byte(body), &result); err != nil {
				t.Fatal(err)
			}
			resp, err := b.request(http.MethodGet, "/api/network-explorations/"+result.Exploration.ID, nil, false)
			if err != nil {
				t.Fatal(err)
			}
			if fetched := readBody(t, resp); fetched != body {
				t.Fatal("retrieved exploration differs from exact saved request/result")
			}
		}
	}
	for _, invalid := range [][]byte{
		append([]byte(`{"unknown":true,`), payload[1:]...),
		append([]byte(`{"nodes":[],`), payload[1:]...),
	} {
		resp, err := b.request(http.MethodPost, "/api/network-explorations", invalid, true)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("non-strict request status=%d body=%s", resp.StatusCode, readBody(t, resp))
		}
		resp.Body.Close()
	}
}

func TestExplorationReopenRejectsForgedResultWithRecomputedChecksum(t *testing.T) {
	s, dir := openTestStore(t)
	inv, err := s.ExploreNetwork(importExplorationSample(t, s))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "network-exploration-"+inv.ID+".json")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var record diskRecord
	if err := json.Unmarshal(b, &record); err != nil {
		t.Fatal(err)
	}
	inv.Result.Branches[0].Returns[0].Asymmetric = !inv.Result.Branches[0].Returns[0].Asymmetric
	record.Data, _ = json.Marshal(inv)
	record.SHA256 = digest(record.Data)
	b, _ = json.Marshal(record)
	if err := os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
	if corrupt, err := NewStore(dir); err == nil {
		_ = corrupt.Close()
		t.Fatal("forged source-pinned exploration accepted despite failed replay")
	}
}

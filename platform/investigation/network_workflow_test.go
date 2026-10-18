package investigation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mattpainter701/graphlayer/platform/networkpath"
)

func importNetworkSample(t *testing.T, s *Store) NetworkRequest {
	t.Helper()
	sample := networkSample()
	req := NetworkRequest{Links: sample.Links, Source: sample.Source, Destination: sample.Destination, Flow: sample.Flow}
	for _, item := range sample.Configs {
		snap, _, err := s.Import(item.Name, []byte(item.Config))
		if err != nil {
			t.Fatalf("import %s: %v", item.NodeID, err)
		}
		req.Nodes = append(req.Nodes, NetworkNodeRef{NodeID: item.NodeID, SnapshotID: snap.ID})
	}
	return req
}

func TestNetworkWorkflowModelsStaticNATRoundTripAndNamespacedEvidence(t *testing.T) {
	s, dir := openTestStore(t)
	req := importNetworkSample(t, s)
	inv, err := s.EvaluateNetwork(req)
	if err != nil {
		t.Fatalf("EvaluateNetwork(): %v", err)
	}
	r := inv.Result
	if r.EngineContractVersion != networkpath.ContractVersion || r.Disposition != "modeled_round_trip" {
		t.Fatalf("sample disposition/version = %q/%q, want modeled_round_trip/%q; reasons=%v", r.Disposition, r.EngineContractVersion, networkpath.ContractVersion, r.Reasons)
	}
	if len(r.Forward.Hops) != 2 || r.Forward.Disposition != "modeled_forward" {
		t.Fatalf("forward path = %q across %d hops, want modeled_forward across 2: %#v", r.Forward.Disposition, len(r.Forward.Hops), r.Forward)
	}
	if r.Forward.Hops[0].Path.EgressFlow.Source != "198.51.100.42" || r.Forward.Hops[1].Path.PolicyFlow.Destination != "10.2.0.15" {
		t.Fatalf("forward SNAT/DNAT tuples were not carried between nodes: %#v", r.Forward.Hops)
	}
	if len(r.Return.Hops) != 2 || r.Return.Disposition != "modeled_forward" {
		t.Fatalf("return path = %q across %d hops, want modeled_forward across 2", r.Return.Disposition, len(r.Return.Hops))
	}
	if r.Return.Hops[0].Path.PolicyFlow.Source != "10.2.0.15" || r.Return.Hops[0].Path.PolicyFlow.Destination != "198.51.100.42" || r.Return.Hops[1].Path.EgressFlow.Destination != "10.1.0.42" {
		t.Fatalf("return tuple did not restore the original client/server addresses: %#v", r.Return.Hops)
	}
	if !containsString(r.Assumptions, "established") {
		t.Fatalf("round-trip result does not disclose established-state assumption: %v", r.Assumptions)
	}

	// Evidence references are local to a snapshot. The fixtures intentionally
	// reuse evidence IDs, so verify every returned reference retains its owner.
	snapshotIDs := map[string]bool{}
	for _, n := range req.Nodes {
		snapshotIDs[n.SnapshotID] = true
	}
	seenEvidence := map[string]bool{}
	for _, direction := range []networkpath.Direction{r.Forward, r.Return} {
		for _, hop := range direction.Hops {
			for _, ref := range hop.Evidence {
				if !snapshotIDs[ref.SnapshotID] || ref.Ref == "" {
					t.Errorf("unscoped network evidence: %#v", ref)
				}
				snap, err := s.Snapshot(ref.SnapshotID)
				if err != nil {
					t.Fatalf("load evidence owner %s: %v", ref.SnapshotID, err)
				}
				found := false
				for _, evidence := range snap.Evidence {
					if evidence.ID == ref.Ref {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("evidence ref %q does not resolve in its owning snapshot %s", ref.Ref, ref.SnapshotID)
				}
				seenEvidence[ref.SnapshotID+"\x00"+ref.Ref] = true
			}
		}
	}
	if len(seenEvidence) == 0 {
		t.Fatal("network path contains no snapshot-scoped evidence")
	}

	before, _ := json.Marshal(inv)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := NewStore(dir)
	if err != nil {
		t.Fatalf("reopen network workspace: %v", err)
	}
	defer reopened.Close()
	after, err := reopened.NetworkInvestigation(inv.ID)
	if err != nil {
		t.Fatal(err)
	}
	afterJSON, _ := json.Marshal(after)
	if string(before) != string(afterJSON) {
		t.Fatal("stored network investigation changed after reopening")
	}
	if got := len(reopened.NetworkSummaries()); got != 1 {
		t.Fatalf("network history count = %d, want 1", got)
	}
}

func TestNetworkWorkflowUncertainAndBlockedTopologyOutcomes(t *testing.T) {
	t.Run("terminal interface mismatch", func(t *testing.T) {
		s, _ := openTestStore(t)
		req := importNetworkSample(t, s)
		req.Destination.Interface = "transit"
		inv, err := s.EvaluateNetwork(req)
		if err != nil || inv.Result.Disposition != "indeterminate" {
			t.Fatalf("terminal mismatch disposition = (%v, %v), want indeterminate", resultDisposition(inv), err)
		}
	})
	t.Run("missing explicit link", func(t *testing.T) {
		s, _ := openTestStore(t)
		req := importNetworkSample(t, s)
		req.Links = nil
		inv, err := s.EvaluateNetwork(req)
		if err != nil || inv.Result.Disposition != "indeterminate" {
			t.Fatalf("missing link disposition = (%v, %v), want indeterminate", resultDisposition(inv), err)
		}
	})
	t.Run("wrong next-hop peer", func(t *testing.T) {
		s, _ := openTestStore(t)
		req := importNetworkSample(t, s)
		// Keep the configured endpoint relationship but point the edge route at
		// an address that is not the core's linked transit interface.
		snap, _, err := s.Import("wrong next hop", []byte(strings.ReplaceAll(networkSample().Configs[0].Config, "172.16.0.2", "172.16.0.3")))
		if err != nil {
			t.Fatal(err)
		}
		req.Nodes[0].SnapshotID = snap.ID
		inv, err := s.EvaluateNetwork(req)
		if err != nil || inv.Result.Disposition != "indeterminate" {
			t.Fatalf("wrong next hop disposition = (%v, %v), want indeterminate", resultDisposition(inv), err)
		}
	})
	t.Run("second hop deny", func(t *testing.T) {
		s, _ := openTestStore(t)
		req := importNetworkSample(t, s)
		core := networkSample().Configs[1].Config
		core = strings.Replace(core, "access-list TRANSIT-IN extended permit tcp host 198.51.100.42 host 10.2.0.15 eq 443", "access-list TRANSIT-IN extended deny tcp host 198.51.100.42 host 10.2.0.15 eq 443", 1)
		snap, _, err := s.Import("denying core", []byte(core))
		if err != nil {
			t.Fatal(err)
		}
		req.Nodes[1].SnapshotID = snap.ID
		inv, err := s.EvaluateNetwork(req)
		if err != nil || inv.Result.Disposition != "blocked" || len(inv.Result.Forward.Hops) != 2 || inv.Result.Forward.Hops[1].Path.Disposition != "blocked" {
			t.Fatalf("second-hop deny disposition/hops = (%v, %d, %v), want blocked at hop 2", resultDisposition(inv), hopCount(inv), err)
		}
	})
	t.Run("missing return route", func(t *testing.T) {
		s, _ := openTestStore(t)
		req := importNetworkSample(t, s)
		core := networkSample().Configs[1].Config
		core = strings.Replace(core, "route transit 198.51.100.42 255.255.255.255 172.16.0.1\n", "", 1)
		snap, _, err := s.Import("no return route", []byte(core))
		if err != nil {
			t.Fatal(err)
		}
		req.Nodes[1].SnapshotID = snap.ID
		inv, err := s.EvaluateNetwork(req)
		if err != nil || inv.Result.Disposition != "blocked" || inv.Result.Return.Disposition != "blocked" {
			t.Fatalf("complete topology with no return route = (%v, %v), want blocked", resultDisposition(inv), err)
		}
	})
	t.Run("asymmetric return next hop", func(t *testing.T) {
		s, _ := openTestStore(t)
		req := importNetworkSample(t, s)
		core := networkSample().Configs[1].Config
		core = strings.Replace(core, "route transit 198.51.100.42 255.255.255.255 172.16.0.1", "route transit 198.51.100.42 255.255.255.255 172.16.0.3", 1)
		snap, _, err := s.Import("asymmetric return route", []byte(core))
		if err != nil {
			t.Fatal(err)
		}
		req.Nodes[1].SnapshotID = snap.ID
		inv, err := s.EvaluateNetwork(req)
		if err != nil || inv.Result.Forward.Disposition != "modeled_forward" || inv.Result.Return.Disposition != "indeterminate" || inv.Result.Disposition != "indeterminate" {
			t.Fatalf("asymmetric return route result=(%v, %v), want indeterminate return after forward success", resultDisposition(inv), err)
		}
	})
	t.Run("unsupported NAT", func(t *testing.T) {
		s, _ := openTestStore(t)
		sample := networkSample()
		edge := sample.Configs[0].Config + "nat (inside,transit) source static CLIENT CLIENT destination static WEB WEB\n"
		req := importNetworkSample(t, s)
		snap, _, err := s.Import("unsupported twice NAT", []byte(edge))
		if err != nil {
			t.Fatal(err)
		}
		req.Nodes[0].SnapshotID = snap.ID
		inv, err := s.EvaluateNetwork(req)
		if err != nil || inv.Result.Disposition != "indeterminate" {
			t.Fatalf("unsupported NAT result=(%v, %v), want indeterminate", resultDisposition(inv), err)
		}
	})
}

func TestNetworkWorkflowHTTPBoundsVerificationAndReplay(t *testing.T) {
	b, store := newTestBrowser(t)
	req := importNetworkSample(t, store)
	payload, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := b.request(http.MethodPost, "/api/network-investigations", payload, true)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("valid network POST status = %d: %s", resp.StatusCode, responseBody(t, resp))
	}
	var response struct {
		Investigation NetworkInvestigation `json:"investigation"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if response.Investigation.ID == "" {
		t.Fatal("network POST returned no saved investigation")
	}

	for _, tc := range []struct {
		name   string
		mutate func(*NetworkRequest)
		want   int
	}{
		{name: "too many nodes", want: http.StatusBadRequest, mutate: func(r *NetworkRequest) {
			for len(r.Nodes) < 17 {
				r.Nodes = append(r.Nodes, NetworkNodeRef{NodeID: "extra-" + string(rune('a'+len(r.Nodes))), SnapshotID: r.Nodes[0].SnapshotID})
			}
		}},
		{name: "too many links", want: http.StatusBadRequest, mutate: func(r *NetworkRequest) { r.Links = make([]networkpath.Link, 33) }},
		{name: "duplicate node id", want: http.StatusBadRequest, mutate: func(r *NetworkRequest) { r.Nodes[1].NodeID = r.Nodes[0].NodeID }},
		{name: "duplicate snapshot", want: http.StatusBadRequest, mutate: func(r *NetworkRequest) { r.Nodes[1].SnapshotID = r.Nodes[0].SnapshotID }},
		{name: "unknown snapshot", want: http.StatusNotFound, mutate: func(r *NetworkRequest) { r.Nodes[0].SnapshotID = strings.Repeat("0", 64) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bad := req
			bad.Nodes = append([]NetworkNodeRef(nil), req.Nodes...)
			bad.Links = append([]networkpath.Link(nil), req.Links...)
			tc.mutate(&bad)
			body, _ := json.Marshal(bad)
			got, err := b.request(http.MethodPost, "/api/network-investigations", body, true)
			if err != nil {
				t.Fatal(err)
			}
			got.Body.Close()
			if got.StatusCode != tc.want {
				t.Fatalf("status = %d, want %d", got.StatusCode, tc.want)
			}
		})
	}

	for _, verified := range []bool{false, true} {
		got, err := b.request(http.MethodPost, "/api/network-investigations", payload, verified)
		if err != nil {
			t.Fatal(err)
		}
		want := http.StatusForbidden
		if verified {
			want = http.StatusOK
		}
		got.Body.Close()
		if got.StatusCode != want {
			t.Fatalf("verified=%v POST status=%d, want %d", verified, got.StatusCode, want)
		}
	}
	got, err := b.request(http.MethodGet, "/api/network-investigations/"+response.Investigation.ID, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if got.StatusCode != http.StatusOK {
		t.Fatalf("reopen detail status=%d", got.StatusCode)
	}
	got.Body.Close()
}

func TestNetworkWorkflowRejectsRechecksummedForgedRecordAndPreservesV3(t *testing.T) {
	s, dir := openTestStore(t)
	req := importNetworkSample(t, s)
	inv, err := s.EvaluateNetwork(req)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	mutateRecord(t, dir, "network-investigation", func(record *diskRecord) {
		var payload map[string]any
		if err := json.Unmarshal(record.Data, &payload); err != nil {
			t.Fatal(err)
		}
		payload["result"].(map[string]any)["disposition"] = "blocked"
		record.Data, _ = json.Marshal(payload)
		hash := sha256.Sum256(record.Data)
		record.SHA256 = hex.EncodeToString(hash[:])
	})
	if forged, err := NewStore(dir); err == nil {
		_ = forged.Close()
		t.Fatal("reopened a forged network trace with a recomputed envelope checksum")
	}
	if _, err := os.Stat(filepath.Join(dir, "network-investigation-"+inv.ID+".json")); err != nil {
		t.Fatalf("forged record unexpectedly disappeared: %v", err)
	}

	legacyDir, legacyBytes := copyActualWorkspace(t, "workspace-v3")
	legacyStore, err := NewStore(legacyDir)
	if err != nil {
		t.Fatalf("open released v3 workspace: %v", err)
	}
	oldPaths := legacyStore.PathSummaries()
	if len(oldPaths) != 2 {
		t.Fatalf("released v3 path count = %d, want 2", len(oldPaths))
	}
	oldResults := make(map[string][]byte, len(oldPaths))
	for _, summary := range oldPaths {
		old, err := legacyStore.PathInvestigation(summary.ID)
		if err != nil {
			t.Fatal(err)
		}
		encoded, _ := json.Marshal(old.Result)
		oldResults[summary.ID] = encoded
		replayed, err := legacyStore.EvaluatePath(old.SnapshotID, old.Result.IngressInterface, old.Result.OriginalFlow)
		if err != nil {
			t.Fatalf("replay released v3 path %s: %v", summary.ID, err)
		}
		encodedReplay, _ := json.Marshal(replayed.Result)
		if string(encodedReplay) != string(encoded) {
			t.Fatalf("released v3 path %s changed on replay", summary.ID)
		}
	}
	req = importNetworkSample(t, legacyStore)
	inv, err = legacyStore.EvaluateNetwork(req)
	if err != nil {
		t.Fatal(err)
	}
	if err := legacyStore.Close(); err != nil {
		t.Fatal(err)
	}
	assertWorkspaceBytes(t, legacyDir, legacyBytes)
	reopenedV4, err := NewStore(legacyDir)
	if err != nil {
		t.Fatalf("reopen mixed released-v3/current-v4 workspace: %v", err)
	}
	defer reopenedV4.Close()
	if got := len(reopenedV4.PathSummaries()); got != 2 {
		t.Fatalf("mixed workspace path history count = %d, want 2", got)
	}
	if got := len(reopenedV4.NetworkSummaries()); got != 1 {
		t.Fatalf("mixed workspace network history count = %d, want 1", got)
	}
	for id, want := range oldResults {
		old, err := reopenedV4.PathInvestigation(id)
		if err != nil {
			t.Fatal(err)
		}
		encoded, _ := json.Marshal(old.Result)
		if string(encoded) != string(want) {
			t.Fatalf("released v3 path %s changed after v4 reopen", id)
		}
	}
	if reopened, err := reopenedV4.NetworkInvestigation(inv.ID); err != nil || reopened.Result.Disposition != inv.Result.Disposition {
		t.Fatalf("reopened v4 network investigation = (%#v, %v)", reopened, err)
	}
}

func copyActualWorkspace(t *testing.T, name string) (string, map[string][]byte) {
	t.Helper()
	source := filepath.Join("testdata", name)
	destination := t.TempDir()
	entries, err := os.ReadDir(source)
	if err != nil {
		t.Fatal(err)
	}
	want := make(map[string][]byte, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(source, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		want[entry.Name()] = append([]byte(nil), data...)
		if err := os.WriteFile(filepath.Join(destination, entry.Name()), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	return destination, want
}

func containsString(values []string, part string) bool {
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), strings.ToLower(part)) {
			return true
		}
	}
	return false
}

func resultDisposition(inv *NetworkInvestigation) string {
	if inv == nil {
		return "<nil>"
	}
	return inv.Result.Disposition
}

func hopCount(inv *NetworkInvestigation) int {
	if inv == nil {
		return 0
	}
	return len(inv.Result.Forward.Hops)
}

func responseBody(t *testing.T, response *http.Response) string {
	t.Helper()
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

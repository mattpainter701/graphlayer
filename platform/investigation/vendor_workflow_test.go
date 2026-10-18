package investigation

import (
	"encoding/json"
	"testing"

	"github.com/mattpainter701/graphlayer/platform/firewall"
)

func importVendorNetworkSample(t *testing.T, s *Store) NetworkRequest {
	t.Helper()
	sample := vendorNetworkSample()
	req := NetworkRequest{Source: sample.Source, Destination: sample.Destination, Links: sample.Links, Flow: sample.Flow}
	for _, item := range sample.Configs {
		snap, _, err := s.ImportVendor(item.Name, []byte(item.Config), item.Vendor)
		if err != nil {
			t.Fatalf("import %s (%s): %v", item.NodeID, item.Vendor, err)
		}
		req.Nodes = append(req.Nodes, NetworkNodeRef{NodeID: item.NodeID, SnapshotID: snap.ID})
	}
	return req
}

func TestVendorSnapshotRejectsIncompatibleTranslationReferences(t *testing.T) {
	s, _ := openTestStore(t)
	snap, _, err := s.ImportVendor("FortiGate", firewall.SampleFortiGate(), "fortinet-fortigate")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		mutate func(*Snapshot)
	}{
		{"VIP points to SNAT", func(s *Snapshot) { s.Path.Translations[0].Kind = "snat" }},
		{"source NAT points to VIP", func(s *Snapshot) { s.Path.Policies[0].SourceNAT = s.Path.Translations[0].ID }},
		{"source NAT belongs to another policy", func(s *Snapshot) {
			s.Path.Policies[0].VIP = ""
			s.Path.Policies[0].SourceNAT = s.Path.Translations[0].ID
			s.Path.Translations[0].Kind = "snat"
			s.Path.Translations[0].PolicyID = s.Path.Policies[1].ID
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			encoded, _ := json.Marshal(snap)
			var corrupt Snapshot
			if err := json.Unmarshal(encoded, &corrupt); err != nil {
				t.Fatal(err)
			}
			tc.mutate(&corrupt)
			if err := validateSnapshot(&corrupt); err == nil {
				t.Fatal("invalid translation dependency accepted for persistence/replay")
			}
		})
	}
}

func TestMultiVendorWorkflowThreeVendorsNATReturnEvidenceReplay(t *testing.T) {
	s, dir := openTestStore(t)
	req := importVendorNetworkSample(t, s)
	inv, err := s.EvaluateNetwork(req)
	if err != nil {
		t.Fatal(err)
	}
	if inv.Result.Disposition != "modeled_round_trip" {
		t.Fatalf("expected mixed-vendor round trip: %#v", inv.Result)
	}
	if len(inv.Result.Forward.Hops) != 3 || len(inv.Result.Return.Hops) != 3 {
		t.Fatalf("expected three hops both directions: %#v", inv.Result)
	}
	fwd, rev := inv.Result.Forward.Hops, inv.Result.Return.Hops
	if fwd[0].Path.PolicyFlow.Source != "10.1.0.42" || fwd[0].Path.EgressFlow.Source != "198.51.100.42" || fwd[2].Path.PolicyFlow.Destination != "10.2.0.15" || rev[2].Path.EgressFlow.Destination != "10.1.0.42" {
		t.Fatalf("translated tuple continuity failed: %#v", inv.Result)
	}
	for _, node := range req.Nodes {
		snap, _ := s.Snapshot(node.SnapshotID)
		if len(snap.SourceLines) == 0 || len(snap.Evidence) == 0 {
			t.Fatal("missing source evidence")
		}
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
	after, err := reopened.NetworkInvestigation(inv.ID)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(after)
	if string(before) != string(encoded) {
		t.Fatal("mixed-vendor investigation changed after reopen")
	}
}

package investigation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mattpainter701/graphlayer/platform/firewall"
	"github.com/mattpainter701/graphlayer/platform/policy"
)

func TestPathWorkflowImportEvaluateEvidenceAndReplay(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "workspace")
	s, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	snap, created, err := s.Import("routing-nat sample", firewall.SamplePathASA())
	if err != nil || !created {
		t.Fatalf("Import path sample = (%v, %v, %v)", snap, created, err)
	}
	if snap.ParserVersion != "asa-config-v3" || snap.Path == nil {
		t.Fatalf("new sample lacks path metadata: parser=%s path=%#v", snap.ParserVersion, snap.Path)
	}
	evidence := make(map[string]firewall.Evidence, len(snap.Evidence))
	for _, item := range snap.Evidence {
		evidence[item.ID] = item
	}
	for _, iface := range snap.Path.Interfaces {
		if len(iface.EvidenceRefs) == 0 {
			t.Errorf("interface %s has no physical source evidence", iface.Name)
		}
		for _, ref := range iface.EvidenceRefs {
			item, ok := evidence[ref]
			if !ok || item.StartLine < 1 || item.EndLine > len(snap.SourceLines) {
				t.Errorf("interface %s has invalid evidence %q: %#v", iface.Name, ref, item)
			}
		}
	}

	flow := func(source, destination, protocol string, sourcePort, destinationPort int) policy.Flow {
		return policy.Flow{Source: source, Destination: destination, Protocol: protocol, SourcePort: sourcePort, DestinationPort: destinationPort}
	}
	dnat, err := s.EvaluatePath(snap.ID, "outside", flow("198.51.100.42", "198.51.100.15", "tcp", 52000, 443))
	if err != nil {
		t.Fatal(err)
	}
	if dnat.Result.Disposition != "modeled_forward" || dnat.Result.PolicyFlow.Destination != "10.20.4.15" || dnat.Result.EgressInterface != "dmz" {
		t.Fatalf("static DNAT path did not use real tuple and direct route: %#v", dnat.Result)
	}
	if dnat.Result.Policy == nil || dnat.Result.Policy.Disposition != "permit" || len(dnat.Result.NATRuleIDs) == 0 {
		t.Fatalf("DNAT/ACL path lacks NAT or policy evidence: %#v", dnat.Result)
	}
	if len(dnat.Result.EvidenceRefs) == 0 || dnat.Result.SelectedRoute == nil {
		t.Fatalf("DNAT/route path lacks physical evidence: %#v", dnat.Result)
	}

	denied, err := s.EvaluatePath(snap.ID, "outside", flow("198.51.100.42", "198.51.100.15", "tcp", 52000, 22))
	if err != nil || denied.Result.Disposition != "blocked" || denied.Result.Stages[2].Status != "blocked" {
		t.Fatalf("static DNAT ACL deny = (%#v, %v)", denied, err)
	}

	translatedConfig := strings.Replace(string(firewall.SamplePathASA()), "nat (inside,outside) static 10.0.0.20", "nat (inside,outside) static 198.51.100.20", 1)
	snatSnap, _, err := s.Import("translated source NAT sample", []byte(translatedConfig))
	if err != nil {
		t.Fatal(err)
	}
	snat, err := s.EvaluatePath(snatSnap.ID, "inside", flow("10.0.0.20", "203.0.113.10", "tcp", 52000, 443))
	if err != nil || snat.Result.Disposition != "modeled_forward" || snat.Result.Stages[4].Status != "matched" {
		t.Fatalf("outbound static source NAT = (%#v, %v)", snat, err)
	}
	if snat.Result.EgressFlow.Source != "198.51.100.20" || snat.Result.SelectedRoute == nil || snat.Result.SelectedRoute.Prefix != "203.0.113.0/24" {
		t.Fatalf("source NAT or longest-prefix route was not retained: %#v", snat.Result)
	}
	if snat.Result.SelectedRoute.NextHop != "192.0.2.253" || snat.Result.EgressInterface != "outside" {
		t.Fatalf("specific route next hop was lost: %#v", snat.Result.SelectedRoute)
	}

	before, _ := json.Marshal(dnat)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := NewStore(dir)
	if err != nil {
		t.Fatalf("reopen path workspace: %v", err)
	}
	defer reopened.Close()
	after, err := reopened.PathInvestigation(dnat.ID)
	if err != nil {
		t.Fatal(err)
	}
	afterJSON, _ := json.Marshal(after)
	if string(before) != string(afterJSON) {
		t.Fatal("saved path trace changed after reopen")
	}
	if len(reopened.PathSummaries()) != 3 {
		t.Fatalf("path history count = %d, want 3", len(reopened.PathSummaries()))
	}
}

func TestPathWorkflowUnsupportedRoutingAndMissingRouteStayIndeterminate(t *testing.T) {
	withoutStaticRoutes := func(raw []byte) []byte {
		text := strings.ReplaceAll(string(raw), "\r\n", "\n")
		lines := strings.Split(text, "\n")
		kept := lines[:0]
		for _, line := range lines {
			fields := strings.Fields(line)
			if len(fields) > 0 && strings.EqualFold(fields[0], "route") {
				continue
			}
			kept = append(kept, line)
		}
		return []byte(strings.Join(kept, "\n"))
	}
	noRouteCheck := func(t *testing.T, inv *PathInvestigation) {
		if inv.Result.Disposition != "blocked" {
			t.Fatalf("missing route produced %s", inv.Result.Disposition)
		}
		if inv.Result.Stages[3].Status != "blocked" || inv.Result.Stages[5].Status == "matched" {
			t.Fatalf("missing route fabricated forwarding: %#v", inv.Result.Stages)
		}
	}
	tests := []struct {
		name           string
		raw            []byte
		ingress        string
		source         string
		destination    string
		noStaticRoutes bool
		check          func(*testing.T, *PathInvestigation)
	}{
		{name: "dynamic routing", raw: append(firewall.SamplePathASA(), []byte("router ospf 1\n")...), ingress: "outside", source: "198.51.100.42", destination: "198.51.100.15", check: func(t *testing.T, inv *PathInvestigation) {
			if inv.Result.Disposition != "indeterminate" {
				t.Fatalf("dynamic routing produced %s", inv.Result.Disposition)
			}
		}},
		{name: "no configured route", raw: withoutStaticRoutes(firewall.SamplePathASA()), ingress: "inside", source: "10.0.0.20", destination: "100.64.0.1", noStaticRoutes: true, check: noRouteCheck},
		{name: "no configured route with CRLF", raw: withoutStaticRoutes([]byte(strings.ReplaceAll(strings.ReplaceAll(string(firewall.SamplePathASA()), "\r\n", "\n"), "\n", "\r\n"))), ingress: "inside", source: "10.0.0.20", destination: "100.64.0.1", noStaticRoutes: true, check: noRouteCheck},
		{name: "manual twice NAT", raw: append(firewall.SamplePathASA(), []byte("nat (inside,outside) source static IDENTITY-CLIENT IDENTITY-CLIENT destination static WEB WEB\n")...), ingress: "outside", source: "198.51.100.42", destination: "198.51.100.15", check: func(t *testing.T, inv *PathInvestigation) {
			if inv.Result.Disposition != "indeterminate" {
				t.Fatalf("unsupported manual NAT produced %s", inv.Result.Disposition)
			}
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, _ := openTestStore(t)
			snap, _, err := s.Import("unsupported path", tc.raw)
			if err != nil {
				t.Fatalf("Import(): %v", err)
			}
			if tc.noStaticRoutes {
				for _, line := range strings.Split(string(tc.raw), "\n") {
					fields := strings.Fields(line)
					if len(fields) > 0 && strings.EqualFold(fields[0], "route") {
						t.Fatalf("no-route fixture still contains a static route directive: %q", line)
					}
				}
			}
			inv, err := s.EvaluatePath(snap.ID, tc.ingress, policy.Flow{Source: tc.source, Destination: tc.destination, Protocol: "tcp", SourcePort: 52000, DestinationPort: 443})
			if err != nil {
				t.Fatalf("EvaluatePath(): %v", err)
			}
			tc.check(t, inv)
		})
	}
}

func TestPathWorkflowRejectsRechecksummedForgedTraceAndBadRequests(t *testing.T) {
	s, dir := openTestStore(t)
	snap, _, err := s.Import("path sample", firewall.SamplePathASA())
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.EvaluatePath(snap.ID, "outside", policy.Flow{Source: "198.51.100.42", Destination: "198.51.100.15", Protocol: "tcp", SourcePort: 52000, DestinationPort: 443})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	mutateRecord(t, dir, "path-investigation", func(record *diskRecord) {
		var payload map[string]any
		if err := json.Unmarshal(record.Data, &payload); err != nil {
			t.Fatal(err)
		}
		result := payload["result"].(map[string]any)
		result["disposition"] = "blocked"
		record.Data, _ = json.Marshal(payload)
		hash := sha256.Sum256(record.Data)
		record.SHA256 = hex.EncodeToString(hash[:])
	})
	if forged, err := NewStore(dir); err == nil {
		_ = forged.Close()
		t.Fatal("reopened a forged path trace with a recomputed envelope checksum")
	}

	b, store := newTestBrowser(t)
	valid, _, err := store.Import("path request sample", firewall.SamplePathASA())
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, ingress, source, destination, protocol string
		sourcePort, destinationPort, wantStatus      int
	}{
		{name: "missing ingress", ingress: "", source: "198.51.100.42", destination: "198.51.100.15", protocol: "tcp", sourcePort: 52000, destinationPort: 443, wantStatus: http.StatusBadRequest},
		{name: "unknown ingress", ingress: "outside-typo", source: "198.51.100.42", destination: "198.51.100.15", protocol: "tcp", sourcePort: 52000, destinationPort: 443, wantStatus: http.StatusBadRequest},
		{name: "malformed IPv4 tuple", ingress: "outside", source: "999.1.1.1", destination: "198.51.100.15", protocol: "tcp", sourcePort: 52000, destinationPort: 443, wantStatus: http.StatusBadRequest},
		{name: "IPv6 tuple", ingress: "outside", source: "2001:db8::1", destination: "198.51.100.15", protocol: "tcp", sourcePort: 52000, destinationPort: 443, wantStatus: http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload, _ := json.Marshal(map[string]any{"ingress_interface": tc.ingress, "flow": map[string]any{"source": tc.source, "destination": tc.destination, "protocol": tc.protocol, "source_port": tc.sourcePort, "destination_port": tc.destinationPort}})
			resp, err := b.request(http.MethodPost, "/api/snapshots/"+valid.ID+"/path-evaluate", payload, true)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != tc.wantStatus {
				t.Fatalf("status = %d, want %d", resp.StatusCode, tc.wantStatus)
			}
		})
	}
	legacyDir, _ := copyWorkspaceV1(t)
	legacyStore, err := NewStore(legacyDir)
	if err != nil {
		t.Fatal(err)
	}
	defer legacyStore.Close()
	legacyID := legacyStore.Summaries()[0].ID
	if _, err := legacyStore.EvaluatePath(legacyID, "outside", policy.Flow{Source: "192.0.2.4", Destination: "10.20.4.15", Protocol: "tcp", SourcePort: 52000, DestinationPort: 443}); !errors.Is(err, ErrPathUnavailable) {
		t.Fatalf("released parser-v1 snapshot path error = %v, want ErrPathUnavailable", err)
	}
}

func copyActualWorkspaceV2(t *testing.T) (string, map[string][]byte) {
	t.Helper()
	source := filepath.Join("testdata", "workspace-v2")
	destination := t.TempDir()
	entries, err := os.ReadDir(source)
	if err != nil {
		t.Fatal(err)
	}
	want := make(map[string][]byte, len(entries))
	for _, entry := range entries {
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

func TestActualWorkspaceV2PathUnavailableAndMixedWorkspacePreservesLegacyBytes(t *testing.T) {
	dir, legacyBytes := copyActualWorkspaceV2(t)
	s, err := NewStore(dir)
	if err != nil {
		t.Fatalf("open actual released workspace-v2 fixture: %v", err)
	}
	if len(s.Summaries()) != 1 {
		t.Fatalf("workspace-v2 snapshot count = %d, want 1", len(s.Summaries()))
	}
	legacy, err := s.Snapshot(s.Summaries()[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if legacy.ParserVersion != "asa-config-v2" || legacy.Path != nil {
		t.Fatalf("released v2 snapshot gained a path model: parser=%q path=%#v", legacy.ParserVersion, legacy.Path)
	}
	if _, err := s.EvaluatePath(legacy.ID, "outside", policy.Flow{Source: "192.0.2.42", Destination: "10.20.4.15", Protocol: "tcp", SourcePort: 52000, DestinationPort: 443}); !errors.Is(err, ErrPathUnavailable) {
		t.Fatalf("released v2 path error = %v, want ErrPathUnavailable", err)
	}
	newSnap, _, err := s.Import("current path snapshot", firewall.SamplePathASA())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.EvaluatePath(newSnap.ID, "outside", policy.Flow{Source: "198.51.100.42", Destination: "198.51.100.15", Protocol: "tcp", SourcePort: 52000, DestinationPort: 443}); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	assertWorkspaceBytes(t, dir, legacyBytes)
	reopened, err := NewStore(dir)
	if err != nil {
		t.Fatalf("reopen mixed v2/v3 workspace: %v", err)
	}
	defer reopened.Close()
	if len(reopened.Summaries()) != 2 || len(reopened.PathSummaries()) != 1 {
		t.Fatalf("mixed workspace reopened with %d snapshots and %d paths", len(reopened.Summaries()), len(reopened.PathSummaries()))
	}
	assertWorkspaceBytes(t, dir, legacyBytes)
}

func TestPathSnapshotRejectsRechecksummedConnectedPrefixTampering(t *testing.T) {
	s, dir := openTestStore(t)
	if _, _, err := s.Import("path snapshot", firewall.SamplePathASA()); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	mutateRecord(t, dir, "snapshot", func(record *diskRecord) {
		var payload map[string]any
		if err := json.Unmarshal(record.Data, &payload); err != nil {
			t.Fatal(err)
		}
		path := payload["path"].(map[string]any)
		routes := path["routes"].([]any)
		for _, item := range routes {
			route := item.(map[string]any)
			if route["kind"] == "connected" {
				route["prefix"] = "172.16.1.0/24"
				break
			}
		}
		record.Data, _ = json.Marshal(payload)
		hash := sha256.Sum256(record.Data)
		record.SHA256 = hex.EncodeToString(hash[:])
	})
	if corrupt, err := NewStore(dir); err == nil {
		_ = corrupt.Close()
		t.Fatal("reopened a snapshot with a forged connected route prefix and recomputed checksum")
	}
}

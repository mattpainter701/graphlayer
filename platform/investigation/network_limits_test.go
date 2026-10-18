package investigation

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestNetworkLimitsValidationDoesNotPersistPartialRecord(t *testing.T) {
	s, _ := openTestStore(t)
	req := importNetworkSample(t, s)

	cases := map[string]func(*NetworkRequest){
		"unknown node reference": func(r *NetworkRequest) { r.Nodes[0].NodeID = "missing" },
		"duplicate node ID": func(r *NetworkRequest) { r.Nodes[1].NodeID = r.Nodes[0].NodeID },
		"IPv6 flow": func(r *NetworkRequest) { r.Flow.Source = "2001:db8::1" },
		"oversized interface": func(r *NetworkRequest) { r.Source.Interface = string(make([]byte, 49)) },
	}
	for name, alter := range cases {
		t.Run(name, func(t *testing.T) {
			bad := req
			bad.Nodes = append([]NetworkNodeRef(nil), req.Nodes...)
			bad.Links = append(bad.Links[:0:0], req.Links...)
			alter(&bad)
			if _, err := s.EvaluateNetwork(bad); !errors.Is(err, ErrInvalidFlow) {
				t.Fatalf("EvaluateNetwork() error = %v, want ErrInvalidFlow", err)
			}
			if got := len(s.NetworkSummaries()); got != 0 {
				t.Fatalf("invalid request persisted %d network records", got)
			}
		})
	}
}

func TestNetworkLimitsRejectScopeInjectionJSON(t *testing.T) {
	b, store := newTestBrowser(t)
	req := importNetworkSample(t, store)
	valid, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		"unknown scope field": strings.TrimSuffix(string(valid), "}") + `,"snapshot_id":"attacker"}`,
		"case alias duplicate": strings.TrimSuffix(string(valid), "}") + `,"Nodes":[]}`,
	}
	for name, payload := range cases {
		t.Run(name, func(t *testing.T) {
			resp, err := b.request(http.MethodPost, "/api/network-investigations", []byte(payload), true)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			_, _ = io.Copy(io.Discard, resp.Body)
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("POST status = %d, want 400", resp.StatusCode)
			}
			if got := len(store.NetworkSummaries()); got != 0 {
				t.Fatalf("rejected scope injection persisted %d records", got)
			}
		})
	}
}

func TestNetworkLimitsDuplicateAtSharedQuotaIsIdempotent(t *testing.T) {
	s, _ := openTestStore(t)
	req := importNetworkSample(t, s)
	first, err := s.EvaluateNetwork(req)
	if err != nil {
		t.Fatal(err)
	}

	// Occupy the remaining shared slots with existing ACL and path artifacts.
	for i := 0; i < maxInvestigations-1; i++ {
		id := digest([]byte{byte(i), byte(i >> 8), byte(i >> 16)})
		if i%2 == 0 {
			s.investigations[id] = []byte(`{}`)
		} else {
			s.paths[id] = []byte(`{}`)
		}
	}
	if got := s.investigationCount(); got != maxInvestigations {
		t.Fatalf("shared count = %d, want %d", got, maxInvestigations)
	}

	duplicate, err := s.EvaluateNetwork(req)
	if err != nil || duplicate.ID != first.ID {
		t.Fatalf("exact duplicate at quota = (%v, %v), want original record", duplicate, err)
	}
	if got := len(s.networks); got != 1 {
		t.Fatalf("network record count after duplicate = %d, want 1", got)
	}
}

func TestNetworkLimitsReturnedRecordsAreDetached(t *testing.T) {
	s, _ := openTestStore(t)
	req := importNetworkSample(t, s)
	wantSource := req.Source
	inv, err := s.EvaluateNetwork(req)
	if err != nil {
		t.Fatal(err)
	}

	// Mutate both the original caller input and the returned object. Neither
	// should be able to alter the canonical bytes held by the store.
	req.Nodes[0].NodeID = "caller-mutated"
	req.Links[0].A.Interface = "caller-mutated"
	inv.Request.Nodes[0].NodeID = "returned-mutated"
	inv.Request.Links[0].A.Interface = "returned-mutated"
	inv.Result.Disposition = "returned-mutated"

	stored, err := s.NetworkInvestigation(inv.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Request.Source != wantSource || stored.Request.Nodes[0].NodeID == "caller-mutated" || stored.Request.Nodes[0].NodeID == "returned-mutated" || stored.Request.Links[0].A.Interface == "returned-mutated" || stored.Result.Disposition == "returned-mutated" {
		t.Fatalf("stored record was changed through an alias: %#v", stored)
	}
}

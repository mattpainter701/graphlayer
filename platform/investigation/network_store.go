package investigation

import (
	"encoding/json"
	"errors"
	"net/netip"
	"sort"
	"time"

	"github.com/mattpainter701/graphlayer/platform/networkpath"
	"github.com/mattpainter701/graphlayer/platform/policy"
)

// NetworkNodeRef pins each topology node to an immutable imported snapshot.
type NetworkNodeRef struct {
	NodeID     string `json:"node_id"`
	SnapshotID string `json:"snapshot_id"`
}

// NetworkFlow deliberately excludes scope: each hop receives its snapshot scope.
type NetworkFlow struct {
	Source          string `json:"source"`
	Destination     string `json:"destination"`
	Protocol        string `json:"protocol"`
	SourcePort      int    `json:"source_port,omitempty"`
	DestinationPort int    `json:"destination_port,omitempty"`
}

type NetworkRequest struct {
	Nodes       []NetworkNodeRef     `json:"nodes"`
	Links       []networkpath.Link   `json:"links"`
	Source      networkpath.Endpoint `json:"source"`
	Destination networkpath.Endpoint `json:"destination"`
	Flow        NetworkFlow          `json:"flow"`
}

type NetworkInvestigation struct {
	ID        string             `json:"id"`
	CreatedAt time.Time          `json:"created_at"`
	Request   NetworkRequest     `json:"request"`
	Result    networkpath.Result `json:"result"`
}

type NetworkSummary struct {
	ID                 string               `json:"id"`
	CreatedAt          time.Time            `json:"created_at"`
	Disposition        string               `json:"disposition"`
	Source             networkpath.Endpoint `json:"source"`
	Destination        networkpath.Endpoint `json:"destination"`
	NodeCount          int                  `json:"node_count"`
	BranchCount        int                  `json:"branch_count,omitempty"`
	SourceAddress      string               `json:"source_address"`
	DestinationAddress string               `json:"destination_address"`
	Protocol           string               `json:"protocol"`
}

// Caller holds at least a read lock. Snapshots are immutable byte records.
func (s *Store) networkInput(req NetworkRequest) (networkpath.Input, error) {
	if len(req.Nodes) < 1 || len(req.Nodes) > 16 || len(req.Links) > 32 {
		return networkpath.Input{}, ErrInvalidFlow
	}
	in := networkpath.Input{Links: req.Links, Source: req.Source, Destination: req.Destination,
		Flow: policy.Flow{Source: req.Flow.Source, Destination: req.Flow.Destination, Protocol: req.Flow.Protocol, SourcePort: req.Flow.SourcePort, DestinationPort: req.Flow.DestinationPort}}
	seen := map[string]bool{}
	seenSnapshots := map[string]bool{}
	for _, ref := range req.Nodes {
		if !validNetworkNodeID(ref.NodeID) || !validID(ref.SnapshotID) || seen[ref.NodeID] || seenSnapshots[ref.SnapshotID] {
			return in, ErrInvalidFlow
		}
		seen[ref.NodeID], seenSnapshots[ref.SnapshotID] = true, true
		data, ok := s.snapshots[ref.SnapshotID]
		if !ok {
			return in, ErrNotFound
		}
		var snap Snapshot
		if json.Unmarshal(data, &snap) != nil {
			return in, errors.New("invalid network snapshot")
		}
		if snap.Path == nil {
			return in, ErrPathUnavailable
		}
		scope := pathInput(&snap, "", policy.Flow{}).Scope
		in.Nodes = append(in.Nodes, networkpath.Node{ID: ref.NodeID, Scope: scope, Config: snap.Config})
		if req.Source.NodeID == ref.NodeID {
			in.Flow.Scope = scope
		}
	}
	for _, endpoint := range append([]networkpath.Endpoint{req.Source, req.Destination}, networkLinkEndpoints(req.Links)...) {
		if !seen[endpoint.NodeID] || len(endpoint.Interface) == 0 || len(endpoint.Interface) > 48 {
			return in, ErrInvalidFlow
		}
	}
	if err := policy.Validate(policy.Bundle{Scope: in.Flow.Scope, Complete: true, DefaultAction: "deny", Rules: []policy.Rule{}}, in.Flow); err != nil {
		return in, ErrInvalidFlow
	}
	source, sourceErr := netip.ParseAddr(in.Flow.Source)
	destination, destinationErr := netip.ParseAddr(in.Flow.Destination)
	if sourceErr != nil || destinationErr != nil || !source.Is4() || !destination.Is4() {
		return in, ErrInvalidFlow
	}
	return in, nil
}

func validNetworkNodeID(id string) bool {
	if len(id) == 0 || len(id) > 64 {
		return false
	}
	for _, c := range id {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.') {
			return false
		}
	}
	return true
}

func networkLinkEndpoints(links []networkpath.Link) []networkpath.Endpoint {
	var endpoints []networkpath.Endpoint
	for _, l := range links {
		endpoints = append(endpoints, l.A, l.B)
	}
	return endpoints
}

func networkIdentity(req NetworkRequest, result networkpath.Result) string {
	b, _ := json.Marshal(struct {
		Request NetworkRequest     `json:"request"`
		Result  networkpath.Result `json:"result"`
	}{req, result})
	return digest(append([]byte("network\x00"), b...))
}

func (s *Store) EvaluateNetwork(req NetworkRequest) (*NetworkInvestigation, error) {
	// Detach all request slices from the caller before evaluation and persistence.
	raw, err := json.Marshal(req)
	if err != nil {
		return nil, ErrInvalidFlow
	}
	if err = json.Unmarshal(raw, &req); err != nil {
		return nil, ErrInvalidFlow
	}
	s.mu.RLock()
	in, err := s.networkInput(req)
	s.mu.RUnlock()
	if err != nil {
		return nil, err
	}
	result, err := networkpath.Evaluate(in)
	if err != nil {
		return nil, ErrInvalidFlow
	}
	inv := &NetworkInvestigation{ID: networkIdentity(req, result), CreatedAt: time.Now().UTC(), Request: req, Result: result}
	data, err := json.Marshal(inv)
	if err != nil {
		return nil, errors.New("cannot encode network investigation")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, errors.New("workspace is closed")
	}
	if old, ok := s.networks[inv.ID]; ok {
		var existing NetworkInvestigation
		if json.Unmarshal(old, &existing) != nil {
			return nil, errors.New("invalid network investigation")
		}
		return &existing, nil
	}
	if s.investigationCount() >= maxInvestigations {
		return nil, ErrQuota
	}
	if err = s.write("network-investigation", inv.ID, data); err != nil {
		return nil, err
	}
	s.networks[inv.ID] = data
	return inv, nil
}

func (s *Store) NetworkInvestigation(id string) (*NetworkInvestigation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, ok := s.networks[id]
	if !ok {
		return nil, ErrNotFound
	}
	var inv NetworkInvestigation
	if json.Unmarshal(data, &inv) != nil {
		return nil, errors.New("invalid network investigation")
	}
	return &inv, nil
}

func (s *Store) NetworkSummaries() []NetworkSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows := make([]NetworkSummary, 0, len(s.networks))
	for _, data := range s.networks {
		var inv NetworkInvestigation
		if json.Unmarshal(data, &inv) != nil {
			continue
		}
		rows = append(rows, NetworkSummary{ID: inv.ID, CreatedAt: inv.CreatedAt, Disposition: inv.Result.Disposition, Source: inv.Request.Source, Destination: inv.Request.Destination, NodeCount: len(inv.Request.Nodes), SourceAddress: inv.Request.Flow.Source, DestinationAddress: inv.Request.Flow.Destination, Protocol: inv.Request.Flow.Protocol})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].CreatedAt.Equal(rows[j].CreatedAt) {
			return rows[i].ID < rows[j].ID
		}
		return rows[i].CreatedAt.After(rows[j].CreatedAt)
	})
	return rows
}

func (s *Store) validateNetworkInvestigation(inv *NetworkInvestigation) error {
	if !validID(inv.ID) || inv.CreatedAt.IsZero() || (inv.Result.EngineContractVersion != networkpath.ContractVersion && inv.Result.EngineContractVersion != networkpath.VendorContractVersion) {
		return errors.New("invalid network investigation identity or version")
	}
	in, err := s.networkInput(inv.Request)
	if err != nil {
		return err
	}
	result, err := networkpath.Evaluate(in)
	if err != nil {
		return err
	}
	want, _ := json.Marshal(result)
	got, _ := json.Marshal(inv.Result)
	if string(want) != string(got) || inv.ID != networkIdentity(inv.Request, result) {
		return errors.New("stored network investigation does not replay")
	}
	return nil
}

// Caller holds the store lock, or is initializing an unpublished store.
func (s *Store) investigationCount() int {
	return len(s.investigations) + len(s.paths) + len(s.networks) + len(s.explorations)
}

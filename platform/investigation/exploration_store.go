package investigation

import (
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/mattpainter701/graphlayer/platform/networkpath"
)

// NetworkExploration pins the complete topology request and every bounded path
// alternative to the immutable source snapshots used by the engine.
type NetworkExploration struct {
	ID        string                        `json:"id"`
	CreatedAt time.Time                     `json:"created_at"`
	Request   NetworkRequest                `json:"request"`
	Result    networkpath.ExplorationResult `json:"result"`
}

func explorationIdentity(req NetworkRequest, result networkpath.ExplorationResult) string {
	b, _ := json.Marshal(struct {
		Request NetworkRequest                `json:"request"`
		Result  networkpath.ExplorationResult `json:"result"`
	}{req, result})
	return digest(append([]byte("network-exploration\x00"), b...))
}

func (s *Store) ExploreNetwork(req NetworkRequest) (*NetworkExploration, error) {
	// Detach caller-owned slices before evaluating, hashing or persisting them.
	raw, err := json.Marshal(req)
	if err != nil {
		return nil, ErrInvalidFlow
	}
	if json.Unmarshal(raw, &req) != nil {
		return nil, ErrInvalidFlow
	}
	s.mu.RLock()
	in, err := s.networkInput(req)
	s.mu.RUnlock()
	if err != nil {
		return nil, err
	}
	result, err := networkpath.Explore(in)
	if err != nil {
		return nil, ErrInvalidFlow
	}
	inv := &NetworkExploration{ID: explorationIdentity(req, result), CreatedAt: time.Now().UTC(), Request: req, Result: result}
	data, err := json.Marshal(inv)
	if err != nil {
		return nil, errors.New("cannot encode network exploration")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, errors.New("workspace is closed")
	}
	if old, ok := s.explorations[inv.ID]; ok {
		var existing NetworkExploration
		if json.Unmarshal(old, &existing) != nil {
			return nil, errors.New("invalid network exploration")
		}
		return &existing, nil
	}
	if s.investigationCount() >= maxInvestigations {
		return nil, ErrQuota
	}
	if err := s.write("network-exploration", inv.ID, data); err != nil {
		return nil, err
	}
	s.explorations[inv.ID] = data
	return inv, nil
}

func (s *Store) NetworkExploration(id string) (*NetworkExploration, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, errors.New("workspace is closed")
	}
	data, ok := s.explorations[id]
	if !ok {
		return nil, ErrNotFound
	}
	var inv NetworkExploration
	if json.Unmarshal(data, &inv) != nil {
		return nil, errors.New("invalid network exploration")
	}
	return &inv, nil
}

func (s *Store) ExplorationSummaries() []NetworkSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows := make([]NetworkSummary, 0, len(s.explorations))
	for _, data := range s.explorations {
		var inv NetworkExploration
		if json.Unmarshal(data, &inv) != nil {
			continue
		}
		rows = append(rows, NetworkSummary{ID: inv.ID, CreatedAt: inv.CreatedAt, Disposition: inv.Result.Disposition, Source: inv.Request.Source, Destination: inv.Request.Destination, NodeCount: len(inv.Request.Nodes), BranchCount: len(inv.Result.Branches), SourceAddress: inv.Request.Flow.Source, DestinationAddress: inv.Request.Flow.Destination, Protocol: inv.Request.Flow.Protocol})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].CreatedAt.Equal(rows[j].CreatedAt) {
			return rows[i].ID < rows[j].ID
		}
		return rows[i].CreatedAt.After(rows[j].CreatedAt)
	})
	return rows
}

func (s *Store) validateNetworkExploration(inv *NetworkExploration) error {
	if !validID(inv.ID) || inv.CreatedAt.IsZero() || inv.Result.EngineContractVersion != networkpath.ExplorationContractVersion {
		return errors.New("invalid network exploration identity or version")
	}
	in, err := s.networkInput(inv.Request)
	if err != nil {
		return err
	}
	want, err := networkpath.Explore(in)
	if err != nil {
		return err
	}
	wantBytes, _ := json.Marshal(want)
	gotBytes, _ := json.Marshal(inv.Result)
	if string(wantBytes) != string(gotBytes) || inv.ID != explorationIdentity(inv.Request, want) {
		return errors.New("stored network exploration does not replay")
	}
	return nil
}

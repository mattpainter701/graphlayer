package investigation

import (
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/mattpainter701/graphlayer/platform/pathanalysis"
	"github.com/mattpainter701/graphlayer/platform/policy"
)

type PathInvestigation struct {
	ID         string              `json:"id"`
	CreatedAt  time.Time           `json:"created_at"`
	SnapshotID string              `json:"snapshot_id"`
	Result     pathanalysis.Result `json:"result"`
}

type PathSummary struct {
	ID               string    `json:"id"`
	CreatedAt        time.Time `json:"created_at"`
	SnapshotID       string    `json:"snapshot_id"`
	IngressInterface string    `json:"ingress_interface"`
	Source           string    `json:"source"`
	Destination      string    `json:"destination"`
	Protocol         string    `json:"protocol"`
	Disposition      string    `json:"disposition"`
	EgressInterface  string    `json:"egress_interface,omitempty"`
}

func pathInput(s *Snapshot, ingress string, flow policy.Flow) pathanalysis.Input {
	scope := policy.Scope{SnapshotID: s.ID, DeviceID: snapshotDeviceID(s), Context: s.Context}
	flow.Scope = scope
	return pathanalysis.Input{Scope: scope, Config: s.Config, IngressInterface: ingress, Flow: flow}
}

func pathIdentity(snapshotID string, result pathanalysis.Result) string {
	b, _ := json.Marshal(result)
	return digest([]byte(snapshotID + "\x00path\x00" + string(b)))
}

func (s *Store) EvaluatePath(snapshotID, ingress string, flow policy.Flow) (*PathInvestigation, error) {
	snap, err := s.Snapshot(snapshotID)
	if err != nil {
		return nil, err
	}
	if snap.Path == nil {
		return nil, ErrPathUnavailable
	}
	result, err := pathanalysis.Evaluate(pathInput(snap, ingress, flow))
	if err != nil {
		return nil, flowEvaluationError(err)
	}
	inv := &PathInvestigation{ID: pathIdentity(snap.ID, result), CreatedAt: time.Now().UTC(), SnapshotID: snap.ID, Result: result}
	data, err := json.Marshal(inv)
	if err != nil {
		return nil, errors.New("cannot encode path investigation")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, errors.New("workspace is closed")
	}
	if existing, ok := s.paths[inv.ID]; ok {
		var old PathInvestigation
		if json.Unmarshal(existing, &old) != nil {
			return nil, errors.New("invalid path investigation")
		}
		return &old, nil
	}
	if s.investigationCount() >= maxInvestigations {
		return nil, ErrQuota
	}
	if err = s.write("path-investigation", inv.ID, data); err != nil {
		return nil, err
	}
	s.paths[inv.ID] = data
	return inv, nil
}

func (s *Store) PathInvestigation(id string) (*PathInvestigation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, ok := s.paths[id]
	if !ok {
		return nil, ErrNotFound
	}
	var inv PathInvestigation
	if json.Unmarshal(data, &inv) != nil {
		return nil, errors.New("invalid path investigation")
	}
	return &inv, nil
}

func (s *Store) PathSummaries() []PathSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows := make([]PathSummary, 0, len(s.paths))
	for _, data := range s.paths {
		var inv PathInvestigation
		if json.Unmarshal(data, &inv) != nil {
			continue
		}
		r := inv.Result
		rows = append(rows, PathSummary{ID: inv.ID, CreatedAt: inv.CreatedAt, SnapshotID: inv.SnapshotID, IngressInterface: r.IngressInterface, Source: r.OriginalFlow.Source, Destination: r.OriginalFlow.Destination, Protocol: r.OriginalFlow.Protocol, Disposition: r.Disposition, EgressInterface: r.EgressInterface})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].CreatedAt.Equal(rows[j].CreatedAt) {
			return rows[i].ID < rows[j].ID
		}
		return rows[i].CreatedAt.After(rows[j].CreatedAt)
	})
	return rows
}

func (s *Store) validatePathInvestigation(inv *PathInvestigation) error {
	if !validID(inv.ID) || inv.CreatedAt.IsZero() || (inv.Result.EngineContractVersion != pathanalysis.ContractVersion && inv.Result.EngineContractVersion != pathanalysis.VendorContractVersion) {
		return errors.New("invalid path investigation identity or version")
	}
	data, exists := s.snapshots[inv.SnapshotID]
	if !exists {
		return ErrNotFound
	}
	var snap Snapshot
	if json.Unmarshal(data, &snap) != nil || snap.Path == nil {
		return errors.New("path snapshot metadata is unavailable")
	}
	result, err := pathanalysis.Evaluate(pathInput(&snap, inv.Result.IngressInterface, inv.Result.OriginalFlow))
	if err != nil {
		return err
	}
	want, _ := json.Marshal(result)
	got, _ := json.Marshal(inv.Result)
	if string(want) != string(got) || inv.ID != pathIdentity(snap.ID, result) {
		return errors.New("stored path does not replay")
	}
	return nil
}

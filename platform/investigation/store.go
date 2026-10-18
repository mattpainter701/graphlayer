// Package investigation implements the local, single-user firewall workspace.
// Its persisted artifacts contain sanitized evidence, never original uploads.
package investigation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mattpainter701/graphlayer/platform/firewall"
	"github.com/mattpainter701/graphlayer/platform/policy"
)

const maxSnapshots = 32
const maxInvestigations = 256
const maxRecordBytes = 8 << 20

const workspaceVersion = "graphlayer-workspace-v2"
const legacyWorkspaceVersion = "graphlayer-workspace-v1"

var ErrNotFound = errors.New("resource not found")
var ErrQuota = errors.New("workspace quota reached")
var ErrInvalidConfig = errors.New("invalid or unsupported ASA configuration")
var ErrInvalidFlow = errors.New("invalid policy flow")
var ErrRecordTooLarge = errors.New("expanded artifact exceeds the storage budget")

type Snapshot struct {
	ID              string    `json:"id"`
	CreatedAt       time.Time `json:"created_at"`
	Name            string    `json:"name"`
	SanitizedSHA256 string    `json:"sanitized_sha256"`
	firewall.Config
}

type SnapshotSummary struct {
	ID              string    `json:"id"`
	CreatedAt       time.Time `json:"created_at"`
	Name            string    `json:"name"`
	Vendor          string    `json:"vendor"`
	ParserVersion   string    `json:"parser_version"`
	Hostname        string    `json:"hostname"`
	Context         string    `json:"context"`
	ArtifactSHA256  string    `json:"artifact_sha256"`
	SanitizedSHA256 string    `json:"sanitized_sha256"`
	ChainCount      int       `json:"chain_count"`
	RuleCount       int       `json:"rule_count"`
	WarningCount    int       `json:"warning_count"`
}

type Investigation struct {
	ID         string        `json:"id"`
	CreatedAt  time.Time     `json:"created_at"`
	SnapshotID string        `json:"snapshot_id"`
	ChainID    string        `json:"chain_id"`
	Result     policy.Result `json:"result"`
}

type Store struct {
	mu             sync.RWMutex
	dir            string
	lock           *os.File
	closed         bool
	snapshots      map[string][]byte
	investigations map[string][]byte
}

type diskRecord struct {
	Version string          `json:"version"`
	Kind    string          `json:"kind"`
	SHA256  string          `json:"sha256"`
	Data    json.RawMessage `json:"data"`
}

func digest(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func validID(id string) bool {
	if len(id) != 64 {
		return false
	}
	_, err := hex.DecodeString(id)
	return err == nil && strings.ToLower(id) == id
}

func snapshotID(c firewall.Config) string {
	return digest([]byte(c.ArtifactSHA256 + "\x00" + c.ParserVersion + "\x00" + c.Hostname + "\x00" + c.Context))
}

// NewStore verifies versioned records before making any stored snapshot ready.
// Checksums detect corruption; they are not signatures or proof of device origin.
func NewStore(dir string) (*Store, error) {
	if dir == "" {
		return nil, errors.New("workspace directory is required")
	}
	absolute, err := filepath.Abs(dir)
	if err != nil {
		return nil, errors.New("invalid workspace directory")
	}
	if err = os.MkdirAll(absolute, 0700); err != nil {
		return nil, errors.New("cannot create workspace")
	}
	info, err := os.Lstat(absolute)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("workspace must be a real directory")
	}
	s := &Store{dir: absolute, snapshots: map[string][]byte{}, investigations: map[string][]byte{}}
	entries, err := os.ReadDir(absolute)
	if err != nil {
		return nil, errors.New("cannot read workspace")
	}
	// Refuse unrelated directories before changing any existing permissions.
	for _, entry := range entries {
		name := entry.Name()
		known := name == ".workspace.lock" || strings.HasPrefix(name, ".pending-")
		for _, kind := range []string{"snapshot-", "investigation-"} {
			if strings.HasPrefix(name, kind) && strings.HasSuffix(name, ".json") && validID(strings.TrimSuffix(strings.TrimPrefix(name, kind), ".json")) {
				known = true
			}
		}
		if !known || entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			return nil, errors.New("select a dedicated empty or existing GraphLayer workspace directory")
		}
	}
	if err = protectPath(absolute, true); err != nil {
		return nil, errors.New("cannot protect workspace directory")
	}
	lock, err := os.OpenFile(filepath.Join(absolute, ".workspace.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, errors.New("cannot open workspace lock")
	}
	keepLock := false
	defer func() {
		if !keepLock {
			_ = lock.Close()
		}
	}()
	if err = protectPath(lock.Name(), false); err != nil {
		return nil, errors.New("cannot protect workspace lock")
	}
	if err = lockWorkspace(lock); err != nil {
		return nil, errors.New("this workspace is already open in another process")
	}
	s.lock = lock
	legacyInvestigations := map[string]bool{}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		if entry.Type()&os.ModeSymlink != 0 || entry.IsDir() {
			return nil, errors.New("invalid workspace record")
		}
		if err = protectPath(filepath.Join(absolute, entry.Name()), false); err != nil {
			return nil, errors.New("cannot protect workspace record")
		}
		f, err := os.Open(filepath.Join(absolute, entry.Name()))
		if err != nil {
			return nil, errors.New("cannot read workspace record")
		}
		data, readErr := io.ReadAll(io.LimitReader(f, maxRecordBytes+1))
		closeErr := f.Close()
		if readErr != nil || closeErr != nil || len(data) > maxRecordBytes {
			return nil, errors.New("invalid workspace record size")
		}
		var record diskRecord
		if json.Unmarshal(data, &record) != nil || (record.Version != workspaceVersion && record.Version != legacyWorkspaceVersion) || record.SHA256 != digest(record.Data) {
			return nil, errors.New("workspace record is corrupt or incompatible")
		}
		switch record.Kind {
		case "snapshot":
			var snap Snapshot
			if json.Unmarshal(record.Data, &snap) != nil || validateSnapshot(&snap) != nil || entry.Name() != "snapshot-"+snap.ID+".json" {
				return nil, errors.New("invalid stored snapshot")
			}
			if record.Version == legacyWorkspaceVersion && snap.ParserVersion != "asa-config-v1" || record.Version == workspaceVersion && snap.ParserVersion != firewall.ParserVersion {
				return nil, errors.New("snapshot format and parser version are incompatible")
			}
			s.snapshots[snap.ID] = append([]byte(nil), record.Data...)
		case "investigation":
			var inv Investigation
			if json.Unmarshal(record.Data, &inv) != nil || !validID(inv.ID) || entry.Name() != "investigation-"+inv.ID+".json" {
				return nil, errors.New("invalid stored investigation")
			}
			legacyInvestigations[inv.ID] = record.Version == legacyWorkspaceVersion
			s.investigations[inv.ID] = append([]byte(nil), record.Data...)
		default:
			return nil, errors.New("unknown workspace record type")
		}
	}
	if len(s.snapshots) > maxSnapshots || len(s.investigations) > maxInvestigations {
		return nil, ErrQuota
	}
	for _, data := range s.investigations {
		var inv Investigation
		if json.Unmarshal(data, &inv) != nil {
			return nil, errors.New("invalid stored investigation")
		}
		if err := s.validateInvestigation(&inv, legacyInvestigations[inv.ID]); err != nil {
			return nil, errors.New("stored investigation does not match its snapshot")
		}
	}
	keepLock = true
	return s, nil
}

func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	return s.lock.Close()
}

func validateSnapshot(s *Snapshot) error {
	if !validID(s.ID) || !validID(s.ArtifactSHA256) || s.ParserVersion == "" || s.Hostname == "" || s.Context == "" || s.ID != snapshotID(s.Config) || s.CreatedAt.IsZero() || len(s.Chains) == 0 || len(s.SourceLines) == 0 {
		return errors.New("invalid snapshot identity")
	}
	lines, err := json.Marshal(s.SourceLines)
	if err != nil || digest(lines) != s.SanitizedSHA256 {
		return errors.New("invalid sanitized evidence checksum")
	}
	for i, line := range s.SourceLines {
		if line.Number != i+1 {
			return errors.New("invalid source line sequence")
		}
	}
	refs := map[string]bool{}
	for _, e := range s.Evidence {
		if e.ID == "" || refs[e.ID] || e.StartLine < 1 || e.EndLine < e.StartLine || e.EndLine > len(s.SourceLines) {
			return errors.New("invalid evidence locator")
		}
		refs[e.ID] = true
	}
	chains := map[string]bool{}
	for _, chain := range s.Chains {
		if chain.ID == "" || chains[chain.ID] {
			return errors.New("invalid chain identity")
		}
		chains[chain.ID] = true
		for _, ref := range chain.EvidenceRefs {
			if !refs[ref] {
				return errors.New("missing chain evidence")
			}
		}
		for _, rule := range chain.Rules {
			for _, ref := range rule.EvidenceRefs {
				if !refs[ref] {
					return errors.New("missing rule evidence")
				}
			}
		}
		b := bundle(s, chain)
		probe := policy.Flow{Scope: b.Scope, Source: "192.0.2.1", Destination: "198.51.100.1", Protocol: "tcp", SourcePort: 49152, DestinationPort: 443}
		if policy.Validate(b, probe) != nil {
			return errors.New("invalid normalized policy chain")
		}
	}
	return validateInventory(s)
}

func bundle(s *Snapshot, chain firewall.Chain) policy.Bundle {
	return policy.Bundle{Scope: policy.Scope{SnapshotID: s.ID, DeviceID: "asa-" + digest([]byte(s.Hostname))[:16], Context: s.Context + "/" + chain.ID}, Complete: chain.Complete, DefaultAction: "deny", Rules: chain.Rules}
}

func (s *Store) write(kind, id string, data []byte) error {
	record := diskRecord{Version: workspaceVersion, Kind: kind, SHA256: digest(data), Data: data}
	encoded, err := json.Marshal(record)
	if err != nil {
		return errors.New("cannot encode workspace record")
	}
	if len(encoded) > maxRecordBytes {
		return ErrRecordTooLarge
	}
	tmp, err := os.CreateTemp(s.dir, ".pending-")
	if err != nil {
		return errors.New("cannot prepare workspace record")
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if err = protectPath(tmpName, false); err != nil {
		_ = tmp.Close()
		return errors.New("cannot protect workspace record")
	}
	if _, err = tmp.Write(encoded); err == nil {
		err = tmp.Sync()
	}
	closeErr := tmp.Close()
	if err != nil || closeErr != nil {
		return errors.New("cannot write workspace record")
	}
	target := filepath.Join(s.dir, kind+"-"+id+".json")
	if _, err = os.Lstat(target); err == nil {
		return errors.New("workspace record already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return errors.New("cannot inspect workspace record")
	}
	if os.Rename(tmpName, target) != nil {
		return errors.New("cannot publish workspace record")
	}
	return nil
}

func (s *Store) Import(name string, raw []byte) (*Snapshot, bool, error) {
	parsed, err := firewall.ParseASAConfig(raw)
	if err != nil {
		return nil, false, ErrInvalidConfig
	}
	if len(parsed.Chains) == 0 || len(parsed.SourceLines) == 0 {
		return nil, false, ErrInvalidConfig
	}
	if parsed.ArtifactSHA256 != digest(raw) {
		return nil, false, errors.New("parser artifact identity mismatch")
	}
	lines, err := json.Marshal(parsed.SourceLines)
	if err != nil {
		return nil, false, errors.New("cannot encode evidence")
	}
	snap := &Snapshot{ID: snapshotID(*parsed), CreatedAt: time.Now().UTC(), Name: name, SanitizedSHA256: digest(lines), Config: *parsed}
	if err = validateSnapshot(snap); err != nil {
		return nil, false, err
	}
	data, err := json.Marshal(snap)
	if err != nil {
		return nil, false, errors.New("cannot encode snapshot")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, false, errors.New("workspace is closed")
	}
	if existing, ok := s.snapshots[snap.ID]; ok {
		var old Snapshot
		if json.Unmarshal(existing, &old) != nil {
			return nil, false, errors.New("invalid snapshot")
		}
		return &old, false, nil
	}
	if len(s.snapshots) >= maxSnapshots {
		return nil, false, ErrQuota
	}
	if err = s.write("snapshot", snap.ID, data); err != nil {
		return nil, false, err
	}
	s.snapshots[snap.ID] = data
	return snap, true, nil
}

func (s *Store) Snapshot(id string) (*Snapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, errors.New("workspace is closed")
	}
	data, ok := s.snapshots[id]
	if !ok {
		return nil, ErrNotFound
	}
	var out Snapshot
	if json.Unmarshal(data, &out) != nil {
		return nil, errors.New("invalid snapshot")
	}
	return &out, nil
}

func (s *Store) Summaries() []SnapshotSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]SnapshotSummary, 0, len(s.snapshots))
	for _, data := range s.snapshots {
		var snap Snapshot
		if json.Unmarshal(data, &snap) != nil {
			continue
		}
		rules, warnings := 0, len(snap.Warnings)
		for _, c := range snap.Chains {
			rules += len(c.Rules)
			warnings += len(c.Warnings)
		}
		out = append(out, SnapshotSummary{ID: snap.ID, CreatedAt: snap.CreatedAt, Name: snap.Name, Vendor: snap.Vendor, ParserVersion: snap.ParserVersion, Hostname: snap.Hostname, Context: snap.Context, ArtifactSHA256: snap.ArtifactSHA256, SanitizedSHA256: snap.SanitizedSHA256, ChainCount: len(snap.Chains), RuleCount: rules, WarningCount: warnings})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID < out[j].ID
		}
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out
}

func (s *Store) validateInvestigation(inv *Investigation, legacy bool) error {
	if legacy {
		if inv.Result.EngineContractVersion != "graphlayer-policy-v2" || len(inv.Result.RuleTraces) != 0 {
			return errors.New("legacy record has an incompatible result contract")
		}
	} else if inv.Result.EngineContractVersion != policy.ContractVersion {
		return errors.New("result contract does not match workspace format")
	}
	data, ok := s.snapshots[inv.SnapshotID]
	if !ok {
		return ErrNotFound
	}
	var snap Snapshot
	if json.Unmarshal(data, &snap) != nil {
		return errors.New("invalid snapshot")
	}
	for _, chain := range snap.Chains {
		if chain.ID != inv.ChainID {
			continue
		}
		if inv.Result.Scope != bundle(&snap, chain).Scope || inv.CreatedAt.IsZero() {
			return errors.New("investigation scope mismatch")
		}
		result, err := policy.Evaluate(bundle(&snap, chain), inv.Result.Flow)
		if err != nil {
			return err
		}
		// Default deny is inferred from the selected attached ASA chain; its
		// evidence anchor is the binding, not a fabricated explicit deny line.
		if result.UsedDefault {
			result.EvidenceRefs = append([]string(nil), chain.EvidenceRefs...)
		}
		// The v3 engine preserves v2 verdict semantics. Compare an explicit
		// legacy projection without rewriting old bytes or inventing traces.
		if legacy {
			result.EngineContractVersion = "graphlayer-policy-v2"
			result.RuleTraces = nil
		}
		want, _ := json.Marshal(result)
		got, _ := json.Marshal(inv.Result)
		if string(want) != string(got) {
			return errors.New("stored result does not replay")
		}
		identity := digest([]byte(inv.SnapshotID + "\x00" + inv.ChainID + "\x00" + string(want)))
		if inv.ID != identity {
			return errors.New("invalid investigation identity")
		}
		return nil
	}
	return ErrNotFound
}

func (s *Store) Evaluate(snapshotID, chainID string, flow policy.Flow) (*Investigation, error) {
	snap, err := s.Snapshot(snapshotID)
	if err != nil {
		return nil, err
	}
	var selected *firewall.Chain
	for i := range snap.Chains {
		if snap.Chains[i].ID == chainID {
			selected = &snap.Chains[i]
			break
		}
	}
	if selected == nil {
		return nil, ErrNotFound
	}
	b := bundle(snap, *selected)
	flow.Scope = b.Scope
	result, err := policy.Evaluate(b, flow)
	if err != nil {
		return nil, ErrInvalidFlow
	}
	if result.UsedDefault {
		result.EvidenceRefs = append([]string(nil), selected.EvidenceRefs...)
	}
	resultData, err := json.Marshal(result)
	if err != nil {
		return nil, errors.New("cannot encode investigation")
	}
	inv := &Investigation{ID: digest([]byte(snap.ID + "\x00" + chainID + "\x00" + string(resultData))), CreatedAt: time.Now().UTC(), SnapshotID: snap.ID, ChainID: chainID, Result: result}
	data, err := json.Marshal(inv)
	if err != nil {
		return nil, errors.New("cannot encode investigation")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, errors.New("workspace is closed")
	}
	if existing, ok := s.investigations[inv.ID]; ok {
		var old Investigation
		if json.Unmarshal(existing, &old) != nil {
			return nil, errors.New("invalid investigation")
		}
		return &old, nil
	}
	if len(s.investigations) >= maxInvestigations {
		return nil, ErrQuota
	}
	if err = s.write("investigation", inv.ID, data); err != nil {
		return nil, err
	}
	s.investigations[inv.ID] = data
	return inv, nil
}

func (s *Store) Investigations() []Investigation {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Investigation, 0, len(s.investigations))
	for _, data := range s.investigations {
		var inv Investigation
		if json.Unmarshal(data, &inv) == nil {
			out = append(out, inv)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID < out[j].ID
		}
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out
}

func (s *Store) Investigation(id string) (*Investigation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, ok := s.investigations[id]
	if !ok {
		return nil, ErrNotFound
	}
	var inv Investigation
	if json.Unmarshal(data, &inv) != nil {
		return nil, errors.New("invalid investigation")
	}
	return &inv, nil
}

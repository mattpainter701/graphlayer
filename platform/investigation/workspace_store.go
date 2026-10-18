package investigation

import (
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const maxWorkspaceSites = 32
const maxSnapshotSets = 64
const maxWorkspaceTransactions = 256
const maxSetMembers = 8

type workspaceTransaction struct {
	ID           string       `json:"id"`
	PrevID       string       `json:"prev_id"`
	Workspace    Workspace    `json:"workspace"`
	Set          *SnapshotSet `json:"set,omitempty"`
	NewSnapshots []Snapshot   `json:"new_snapshots"`
	CreatedAt    time.Time    `json:"created_at"`
}

type transactionPayload struct {
	PrevID       string       `json:"prev_id"`
	Workspace    Workspace    `json:"workspace"`
	Set          *SnapshotSet `json:"set,omitempty"`
	NewSnapshots []Snapshot   `json:"new_snapshots"`
	CreatedAt    time.Time    `json:"created_at"`
}

func stableWorkspaceID(dir string) string {
	return digest([]byte("graphlayer-local-workspace\x00" + dir))
}

func (s *Store) ensureWorkspace() {
	if s.sets == nil {
		s.sets = map[string]SnapshotSet{}
	}
	if s.workspace.ID == "" {
		s.workspace = Workspace{ID: stableWorkspaceID(s.dir), Name: "Local workspace", Sites: []Site{}}
	}
}

func validWorkspaceName(v string) bool { return validLabel(v, 1, 80) }
func validDescription(v string) bool {
	if !utf8.ValidString(v) || len(v) > 512 {
		return false
	}
	for _, r := range v {
		if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
			return false
		}
	}
	return true
}
func validLabel(v string, min, max int) bool {
	if !utf8.ValidString(v) || len(v) < min || len(v) > max || strings.TrimSpace(v) != v {
		return false
	}
	n := 0
	for _, r := range v {
		n++
		if unicode.IsControl(r) {
			return false
		}
	}
	return n >= min && n <= max
}

func cloneWorkspace(w Workspace) Workspace {
	b, _ := json.Marshal(w)
	var v Workspace
	_ = json.Unmarshal(b, &v)
	return v
}
func cloneSet(v SnapshotSet) SnapshotSet {
	b, _ := json.Marshal(v)
	var out SnapshotSet
	_ = json.Unmarshal(b, &out)
	return out
}

func (s *Store) loadWorkspaceTransactions(records map[string][]byte) error {
	s.ensureWorkspace()
	if len(records) > maxWorkspaceTransactions {
		return ErrQuota
	}
	byRev := map[int]workspaceTransaction{}
	for fileID, data := range records {
		var tx workspaceTransaction
		if json.Unmarshal(data, &tx) != nil || tx.ID != fileID || !validID(tx.ID) {
			return ErrWorkspaceInvalid
		}
		if _, ok := byRev[tx.Workspace.Revision]; ok {
			return ErrWorkspaceInvalid
		}
		byRev[tx.Workspace.Revision] = tx
	}
	prev := ""
	for rev := 1; rev <= len(byRev); rev++ {
		tx, ok := byRev[rev]
		if !ok || tx.PrevID != prev {
			return ErrWorkspaceInvalid
		}
		payload := transactionPayload{tx.PrevID, tx.Workspace, tx.Set, tx.NewSnapshots, tx.CreatedAt}
		b, err := json.Marshal(payload)
		if err != nil || digest(b) != tx.ID {
			return ErrWorkspaceInvalid
		}
		if err = s.applyWorkspaceTransaction(tx); err != nil {
			return err
		}
		prev = tx.ID
		s.workspaceHead = tx.ID
	}
	if len(byRev) > 0 && byRev[len(byRev)].ID != prev {
		return ErrWorkspaceInvalid
	}
	s.transactions = len(byRev)
	return nil
}

func (s *Store) applyWorkspaceTransaction(tx workspaceTransaction) error {
	w := tx.Workspace
	if !validID(w.ID) || (s.workspace.Revision > 0 && w.ID != s.workspace.ID) || w.Revision != s.workspace.Revision+1 || !validWorkspaceName(w.Name) || w.UpdatedAt.IsZero() || tx.CreatedAt.IsZero() || len(w.Sites) > maxWorkspaceSites {
		return ErrWorkspaceInvalid
	}
	if len(w.Sites) < len(s.workspace.Sites) || len(w.Sites) > len(s.workspace.Sites)+1 {
		return ErrWorkspaceInvalid
	}
	seen := map[string]bool{}
	for i, site := range w.Sites {
		if !validID(site.ID) || site.ID != digest([]byte(w.ID+"\x00site\x00"+strings.ToLower(site.Name))) || !validLabel(site.Name, 1, 80) || !validDescription(site.Description) || site.CreatedAt.IsZero() || seen[site.ID] {
			return ErrWorkspaceInvalid
		}
		if i < len(s.workspace.Sites) && site != s.workspace.Sites[i] {
			return ErrWorkspaceInvalid
		}
		seen[site.ID] = true
	}
	if len(s.sets) > maxSnapshotSets || (tx.Set == nil && len(tx.NewSnapshots) != 0) {
		return ErrWorkspaceInvalid
	}
	newIDs := map[string]bool{}
	memberIDs := map[string]bool{}
	if tx.Set != nil {
		for _, m := range tx.Set.Members {
			if memberIDs[m.SnapshotID] {
				return ErrWorkspaceInvalid
			}
			memberIDs[m.SnapshotID] = true
		}
	}
	for _, snap := range tx.NewSnapshots {
		if validateSnapshot(&snap) != nil {
			return ErrWorkspaceInvalid
		}
		if _, ok := s.snapshots[snap.ID]; ok || newIDs[snap.ID] || !memberIDs[snap.ID] {
			return ErrWorkspaceInvalid
		}
		newIDs[snap.ID] = true
	}
	if len(s.snapshots)+len(tx.NewSnapshots) > maxSnapshots {
		return ErrQuota
	}
	if tx.Set != nil {
		set := *tx.Set
		if len(s.sets) >= maxSnapshotSets {
			return ErrQuota
		}
		if !validID(set.ID) || !validLabel(set.Name, 1, 80) || (set.SiteID != "" && !seen[set.SiteID]) || !siteExists(set.SiteID, w.Sites) || set.CreatedAt.IsZero() || len(set.Members) < 1 || len(set.Members) > maxSetMembers {
			return ErrWorkspaceInvalid
		}
		if _, exists := s.sets[set.ID]; exists {
			return ErrWorkspaceInvalid
		}
		ids := map[string]bool{}
		for _, m := range set.Members {
			if !validID(m.SnapshotID) || !validLabel(m.Name, 1, 128) || ids[m.SnapshotID] {
				return ErrWorkspaceInvalid
			}
			ids[m.SnapshotID] = true
			if _, ok := s.snapshots[m.SnapshotID]; !ok && !newIDs[m.SnapshotID] {
				return ErrWorkspaceInvalid
			}
		}
		for _, prior := range s.sets {
			if prior.SiteID == set.SiteID && strings.EqualFold(prior.Name, set.Name) {
				return ErrWorkspaceInvalid
			}
		}
		idData, _ := json.Marshal(struct {
			Site    string              `json:"site"`
			Name    string              `json:"name"`
			Members []SnapshotSetMember `json:"members"`
		}{set.SiteID, set.Name, set.Members})
		if digest(idData) != set.ID {
			return ErrWorkspaceInvalid
		}
		s.sets[set.ID] = cloneSet(set)
	}
	for _, set := range s.sets {
		if set.SiteID != "" && !seen[set.SiteID] {
			return ErrWorkspaceInvalid
		}
	}
	for sid := range memberIDs {
		if !newIDs[sid] {
			if _, ok := s.snapshots[sid]; !ok {
				return ErrWorkspaceInvalid
			}
		}
	}
	if w.ActiveSetID != "" {
		if _, ok := s.sets[w.ActiveSetID]; !ok {
			return ErrWorkspaceInvalid
		}
	}
	for _, snap := range tx.NewSnapshots {
		b, err := json.Marshal(snap)
		if err != nil {
			return ErrWorkspaceInvalid
		}
		s.snapshots[snap.ID] = b
	}
	s.workspace = cloneWorkspace(w)
	return nil
}

func siteExists(id string, sites []Site) bool {
	if id == "" {
		return true
	}
	for _, site := range sites {
		if site.ID == id {
			return true
		}
	}
	return false
}

func (s *Store) commitWorkspace(w Workspace, set *SnapshotSet, snaps []Snapshot) error {
	if s.closed {
		return errors.New("workspace is closed")
	}
	if s.transactions >= maxWorkspaceTransactions {
		return ErrQuota
	}
	w.UpdatedAt = time.Now().UTC()
	var prev string
	// The current head is the transaction with the current revision.
	if s.workspace.Revision > 0 { // IDs are deterministic from their full payload, so reconstruct the prior record by scanning is unnecessary: cached below.
		prev = s.workspaceHead
	}
	// Clone all mutable state and validate the complete transaction before publishing it.
	prepared := &Store{dir: s.dir, workspace: cloneWorkspace(s.workspace), snapshots: cloneBytesMap(s.snapshots), sets: cloneSetsMap(s.sets), transactions: s.transactions, workspaceHead: s.workspaceHead}
	setCopy := (*SnapshotSet)(nil)
	if set != nil {
		c := cloneSet(*set)
		setCopy = &c
	}
	snapshotCopies := make([]Snapshot, len(snaps))
	for i := range snaps {
		b, err := json.Marshal(snaps[i])
		if err != nil || json.Unmarshal(b, &snapshotCopies[i]) != nil {
			return ErrWorkspaceInvalid
		}
	}
	tx := workspaceTransaction{PrevID: prev, Workspace: cloneWorkspace(w), Set: setCopy, NewSnapshots: snapshotCopies, CreatedAt: time.Now().UTC()}
	p := transactionPayload{tx.PrevID, tx.Workspace, tx.Set, tx.NewSnapshots, tx.CreatedAt}
	b, err := json.Marshal(p)
	if err != nil {
		return ErrWorkspaceInvalid
	}
	tx.ID = digest(b)
	b, err = json.Marshal(tx)
	if err != nil {
		return ErrWorkspaceInvalid
	}
	if err = prepared.applyWorkspaceTransaction(tx); err != nil {
		return err
	}
	if err = s.write("workspace-transaction", tx.ID, b); err != nil {
		return err
	}
	s.workspace = prepared.workspace
	s.snapshots = prepared.snapshots
	s.sets = prepared.sets
	s.workspaceHead = tx.ID
	s.transactions++
	return nil
}

func cloneBytesMap(in map[string][]byte) map[string][]byte {
	out := make(map[string][]byte, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
func cloneSetsMap(in map[string]SnapshotSet) map[string]SnapshotSet {
	out := make(map[string]SnapshotSet, len(in))
	for k, v := range in {
		out[k] = cloneSet(v)
	}
	return out
}

func (s *Store) WorkspaceInfo() Workspace {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.workspace.ID == "" {
		return Workspace{ID: stableWorkspaceID(s.dir), Name: "Local workspace", Sites: []Site{}}
	}
	return cloneWorkspace(s.workspace)
}

func (s *Store) WorkspaceOverview() (Workspace, []SnapshotSet) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	w := cloneWorkspace(s.workspace)
	if w.ID == "" {
		w = Workspace{ID: stableWorkspaceID(s.dir), Name: "Local workspace", Sites: []Site{}}
	}
	sets := make([]SnapshotSet, 0, len(s.sets))
	for _, v := range s.sets {
		sets = append(sets, cloneSet(v))
	}
	sort.Slice(sets, func(i, j int) bool {
		if sets[i].CreatedAt.Equal(sets[j].CreatedAt) {
			return sets[i].ID < sets[j].ID
		}
		return sets[i].CreatedAt.Before(sets[j].CreatedAt)
	})
	return w, sets
}
func (s *Store) SnapshotSets() []SnapshotSet {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]SnapshotSet, 0, len(s.sets))
	for _, v := range s.sets {
		out = append(out, cloneSet(v))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID < out[j].ID
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out
}
func (s *Store) SnapshotSet(id string) (*SnapshotSet, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, errors.New("workspace is closed")
	}
	v, ok := s.sets[id]
	if !ok {
		return nil, ErrNotFound
	}
	out := cloneSet(v)
	return &out, nil
}

func (s *Store) RenameWorkspace(name string, expectedRevision int) (Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureWorkspace()
	if s.closed {
		return Workspace{}, errors.New("workspace is closed")
	}
	if expectedRevision != s.workspace.Revision {
		return Workspace{}, ErrWorkspaceConflict
	}
	if !validWorkspaceName(name) {
		return Workspace{}, ErrWorkspaceInvalid
	}
	w := cloneWorkspace(s.workspace)
	w.Name = name
	w.Revision++
	if err := s.commitWorkspace(w, nil, nil); err != nil {
		return Workspace{}, err
	}
	return cloneWorkspace(s.workspace), nil
}

func (s *Store) CreateSite(name, description string, expectedRevision int) (*Site, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureWorkspace()
	if s.closed {
		return nil, errors.New("workspace is closed")
	}
	if expectedRevision != s.workspace.Revision {
		return nil, ErrWorkspaceConflict
	}
	if !validLabel(name, 1, 80) || !validDescription(description) {
		return nil, ErrWorkspaceInvalid
	}
	for _, site := range s.workspace.Sites {
		if strings.EqualFold(site.Name, name) {
			return nil, ErrWorkspaceConflict
		}
	}
	if len(s.workspace.Sites) >= maxWorkspaceSites {
		return nil, ErrQuota
	}
	site := Site{ID: digest([]byte(s.workspace.ID + "\x00site\x00" + strings.ToLower(name))), Name: name, Description: description, CreatedAt: time.Now().UTC()}
	w := cloneWorkspace(s.workspace)
	w.Sites = append(w.Sites, site)
	w.Revision++
	if err := s.commitWorkspace(w, nil, nil); err != nil {
		return nil, err
	}
	copy := site
	return &copy, nil
}

func (s *Store) ActivateSnapshotSet(id string, expectedRevision int) (Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureWorkspace()
	if s.closed {
		return Workspace{}, errors.New("workspace is closed")
	}
	if expectedRevision != s.workspace.Revision {
		return Workspace{}, ErrWorkspaceConflict
	}
	if _, ok := s.sets[id]; !ok {
		return Workspace{}, ErrNotFound
	}
	w := cloneWorkspace(s.workspace)
	w.ActiveSetID = id
	w.Revision++
	if err := s.commitWorkspace(w, nil, nil); err != nil {
		return Workspace{}, err
	}
	return cloneWorkspace(s.workspace), nil
}

func (s *Store) PublishSnapshotSet(name, siteID string, snapshots []Snapshot, expectedRevision int) (*SnapshotSet, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureWorkspace()
	if s.closed {
		return nil, false, errors.New("workspace is closed")
	}
	if expectedRevision != s.workspace.Revision {
		return nil, false, ErrWorkspaceConflict
	}
	if !validLabel(name, 1, 80) || len(snapshots) < 1 || len(snapshots) > maxSetMembers {
		return nil, false, ErrWorkspaceInvalid
	}
	validSite := siteID == ""
	for _, site := range s.workspace.Sites {
		if site.ID == siteID {
			validSite = true
		}
	}
	if !validSite {
		return nil, false, ErrWorkspaceInvalid
	}
	for _, old := range s.sets {
		if old.SiteID == siteID && strings.EqualFold(old.Name, name) {
			same := len(old.Members) == len(snapshots)
			for i, snap := range snapshots {
				if !same || old.Members[i].SnapshotID != snap.ID || old.Members[i].Name != snap.Name {
					same = false
					break
				}
			}
			if same {
				if s.workspace.ActiveSetID != old.ID {
					w := cloneWorkspace(s.workspace)
					w.ActiveSetID = old.ID
					w.Revision++
					if err := s.commitWorkspace(w, nil, nil); err != nil {
						return nil, false, err
					}
				}
				v := cloneSet(old)
				return &v, false, nil
			}
			return nil, false, ErrWorkspaceConflict
		}
	}
	seen := map[string]bool{}
	members := make([]SnapshotSetMember, 0, len(snapshots))
	news := make([]Snapshot, 0, len(snapshots))
	for _, input := range snapshots {
		snap := input
		if validateSnapshot(&snap) != nil || seen[snap.ID] {
			return nil, false, ErrWorkspaceInvalid
		}
		seen[snap.ID] = true
		member := SnapshotSetMember{SnapshotID: snap.ID, Name: snap.Name}
		members = append(members, member)
		if data, ok := s.snapshots[snap.ID]; ok {
			var old Snapshot
			if json.Unmarshal(data, &old) != nil {
				return nil, false, ErrWorkspaceInvalid
			}
			continue
		}
		news = append(news, snap)
	}
	if len(s.snapshots)+len(news) > maxSnapshots {
		return nil, false, ErrQuota
	}
	setIDPayload, _ := json.Marshal(struct {
		Site    string              `json:"site"`
		Name    string              `json:"name"`
		Members []SnapshotSetMember `json:"members"`
	}{siteID, name, members})
	set := SnapshotSet{ID: digest(setIDPayload), Name: name, SiteID: siteID, CreatedAt: time.Now().UTC(), Members: members}
	w := cloneWorkspace(s.workspace)
	w.ActiveSetID = set.ID
	w.Revision++
	if _, err := json.Marshal(workspaceTransaction{Workspace: w, Set: &set, NewSnapshots: news}); err != nil {
		return nil, false, ErrWorkspaceInvalid
	}
	if err := s.commitWorkspace(w, &set, news); err != nil {
		return nil, false, err
	}
	out := cloneSet(set)
	return &out, true, nil
}

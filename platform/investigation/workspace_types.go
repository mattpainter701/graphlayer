package investigation

import "time"

// Workspace is the local store's persisted operator context. Sites group set
// membership; they do not alter immutable parser/snapshot identities.
type Workspace struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Revision    int       `json:"revision"`
	Sites       []Site    `json:"sites"`
	ActiveSetID string    `json:"active_set_id"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Site struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

type SnapshotSetMember struct {
	SnapshotID string `json:"snapshot_id"`
	Name       string `json:"name"`
}

type SnapshotSet struct {
	ID        string              `json:"id"`
	Name      string              `json:"name"`
	SiteID    string              `json:"site_id"`
	CreatedAt time.Time           `json:"created_at"`
	Members   []SnapshotSetMember `json:"members"`
}

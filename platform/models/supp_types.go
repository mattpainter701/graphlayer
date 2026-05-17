package models

// ========================================================================
// Missing type stubs — added during Sprint 11 to resolve pre-existing
// compilation issues in the models package. These types are referenced
// by graph_models.go but were never defined.
// ========================================================================

// RecordLifecycle tracks the lifecycle state of a topology record.
type RecordLifecycle struct {
	CreatedAt string `json:"created_at,omitempty" bson:"created_at,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty" bson:"updated_at,omitempty"`
	DeletedAt string `json:"deleted_at,omitempty" bson:"deleted_at,omitempty"`
	IsActive  bool   `json:"is_active" bson:"is_active"`
	Version   int    `json:"version" bson:"version"`
}

// EndpointData holds endpoint-specific integration data for an asset.
type EndpointData struct {
	Source      string            `json:"source,omitempty" bson:"source,omitempty"`
	LastSeen    string            `json:"last_seen,omitempty" bson:"last_seen,omitempty"`
	FirstSeen   string            `json:"first_seen,omitempty" bson:"first_seen,omitempty"`
	Agent       string            `json:"agent,omitempty" bson:"agent,omitempty"`
	Extra       map[string]string `json:"extra,omitempty" bson:"extra,omitempty"`
}

// EPSSDataItem holds EPSS (Exploit Prediction Scoring System) data.
type EPSSDataItem struct {
	Epss       string `json:"epss,omitempty" bson:"epss,omitempty"`
	Percentile string `json:"percentile,omitempty" bson:"percentile,omitempty"`
	Date       string `json:"date,omitempty" bson:"date,omitempty"`
}

// VulnGraphLayerDetailedCatalog provides enriched vulnerability data.
type VulnGraphLayerDetailedCatalog struct {
	ExploitAvailable   bool     `json:"exploit_available" bson:"exploit_available"`
	ExploitLinks       []string `json:"exploit_links,omitempty" bson:"exploit_links,omitempty"`
	RansomwareAssociated bool  `json:"ransomware_associated" bson:"ransomware_associated"`
	ThreatActorCount   int      `json:"threat_actor_count" bson:"threat_actor_count"`
	RemediationLevel   string   `json:"remediation_level,omitempty" bson:"remediation_level,omitempty"`
	ReportConfidence   string   `json:"report_confidence,omitempty" bson:"report_confidence,omitempty"`
}

// PaginationParams defines pagination options for API requests.
type PaginationParams struct {
	Page     int  `json:"page" bson:"page"`
	PageSize int  `json:"page_size" bson:"page_size"`
	Offset   int  `json:"offset,omitempty" bson:"offset,omitempty"`
	Limit    int  `json:"limit" bson:"limit"`
	Total    bool `json:"total,omitempty" bson:"total,omitempty"`
}

// SortDirection defines sort ordering.
type SortDirection int

const (
	SortAsc  SortDirection = 1
	SortDesc SortDirection = -1
)

// SortParams defines sorting options for API requests.
type SortParams struct {
	Field     string        `json:"field" bson:"field"`
	Direction SortDirection `json:"direction" bson:"direction"`
}

// ConvertSortDirectionFromString converts a string to SortDirection.
func ConvertSortDirectionFromString(s string) (SortDirection, bool) {
	switch s {
	case "asc", "ASC", "ascending":
		return SortAsc, true
	case "desc", "DESC", "descending":
		return SortDesc, true
	default:
		return SortAsc, false
	}
}

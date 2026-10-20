// Package models
// Supplemental type definitions needed for graph_models.go.
// These types are referenced but were not included in the repo.
// Sprint 10: Added to unblock comparison harness compilation.

package models

// ====================================================================
// Record Lifecycle
// ====================================================================

// RecordLifecycle tracks the lifecycle of a topology record.
type RecordLifecycle struct {
	Created  string `json:"created" bson:"created"`
	Updated  string `json:"updated" bson:"updated"`
	CreatedBy string `json:"created_by" bson:"created_by"`
	UpdatedBy string `json:"updated_by" bson:"updated_by"`
	Source   string `json:"source" bson:"source"`
}

// ====================================================================
// Endpoint Data
// ====================================================================

// EndpointData carries endpoint integration specific fields.
type EndpointData struct {
	AgentVersion      string `json:"agent_version,omitempty" bson:"agent_version,omitempty"`
	LastCheckin       string `json:"last_checkin,omitempty" bson:"last_checkin,omitempty"`
	EndpointType      string `json:"endpoint_type,omitempty" bson:"endpoint_type,omitempty"`
	ComplianceStatus  string `json:"compliance_status,omitempty" bson:"compliance_status,omitempty"`
}

// ====================================================================
// EPSS Data
// ====================================================================

// EPSSDataItem carries EPSS (Exploit Prediction Scoring System) information.
type EPSSDataItem struct {
	Epss       string `json:"epss" bson:"epss"`
	Percentile string `json:"percentile" bson:"percentile"`
	Date       string `json:"date" bson:"date"`
}

// ====================================================================
// Vulnerability Catalog
// ====================================================================

// VulnerabilityCatalogEntry carries enriched vulnerability data.
type VulnerabilityCatalogEntry struct {
	CVEName         string   `json:"cve_name" bson:"cve_name"`
	VulnerabilityScore     float64  `json:"vulnerability_score" bson:"vulnerability_score"`
	Tactics         []string `json:"tactics,omitempty" bson:"tactics,omitempty"`
	Techniques      []string `json:"techniques,omitempty" bson:"techniques,omitempty"`
	Remediation     string   `json:"remediation,omitempty" bson:"remediation,omitempty"`
	ExploitVerified bool     `json:"exploit_verified" bson:"exploit_verified"`
}

// ====================================================================
// Pagination & Sorting
// ====================================================================

// PaginationParams defines pagination options for list queries.
type PaginationParams struct {
	Page  int32 `json:"page" bson:"page"`
	Limit int32 `json:"limit" bson:"limit"`
	Skip  int32 `json:"skip,omitempty" bson:"skip,omitempty"`
}

// SortDirection defines sort order.
type SortDirection int

const (
	SortAscending  SortDirection = 1
	SortDescending SortDirection = -1
)

// SortParams defines sorting options.
type SortParams struct {
	Field     string        `json:"field" bson:"field"`
	Direction SortDirection `json:"direction" bson:"direction"`
}

// ConvertSortDirectionFromString converts a string to SortDirection.
func ConvertSortDirectionFromString(s string) (SortDirection, bool) {
	switch s {
	case "asc", "ascending", "ASC":
		return SortAscending, true
	case "desc", "descending", "DESC":
		return SortDescending, true
	default:
		return 0, false
	}
}

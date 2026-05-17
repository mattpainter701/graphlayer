// Package correlation provides the cross-source deduplication and correlation
// engine for GraphLayer. Sprint 8: When two sources describe the same device
// or link, merge intelligently with confidence-weighted resolution.
package correlation

import (
	"time"

	"github.com/mattpainter701/graphlayer/platform/models"
)

// ========================================================================
// Core Correlation Types
// ========================================================================

// MatchKey identifies the attribute used for matching records across sources.
type MatchKey string

const (
	MatchKeyManagementIP MatchKey = "management_ip"
	MatchKeyHostname     MatchKey = "hostname"
	MatchKeySerial       MatchKey = "serial"
	MatchKeyMAC          MatchKey = "mac"
	MatchKeyInterfaceMAC MatchKey = "interface_mac"
	MatchKeyNeighbor     MatchKey = "neighbor"
	MatchKeyCustom       MatchKey = "custom"
)

// MatchPriority assigns an ordinal priority to match key types.
// Lower number = higher priority (matched first, trusted more).
var MatchPriority = map[MatchKey]int{
	MatchKeySerial:       1,
	MatchKeyManagementIP: 2,
	MatchKeyMAC:          3,
	MatchKeyHostname:     4,
	MatchKeyNeighbor:     5,
	MatchKeyInterfaceMAC: 6,
	MatchKeyCustom:       7,
}

// ========================================================================
// Source Record
// ========================================================================

// SourceRecord wraps a topology record with its origin metadata.
// This is the input unit for the correlation engine.
type SourceRecord struct {
	// SourceName identifies the connector/parser that produced this record.
	// e.g., "netbox", "ssh-cisco-ios", "servicenow-cmdb"
	SourceName string `json:"source_name"`

	// SourceType classifies the evidence type.
	SourceType models.ConfidenceSource `json:"source_type"`

	// Device is the actual device record (non-nil for device records).
	Device *models.Device `json:"device,omitempty"`

	// Asset is the asset node record (non-nil for asset records).
	Asset *models.AssetNode `json:"asset,omitempty"`

	// Links are topology links associated with this record.
	Links []models.Link `json:"links,omitempty"`

	// L2Links are L2 adjacency links.
	L2Links []models.L2Link `json:"l2_links,omitempty"`

	// Interfaces are interface records from this source.
	Interfaces []models.Interface `json:"interfaces,omitempty"`

	// CollectedAt is when this data was collected.
	CollectedAt time.Time `json:"collected_at"`

	// Confidence is optional pre-computed confidence for this source.
	Confidence *models.ConfidenceProfile `json:"confidence,omitempty"`

	// Metadata carries arbitrary source-specific key-value pairs.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// PrimaryIdentifier returns the best stable identifier for this source record.
func (sr *SourceRecord) PrimaryIdentifier() string {
	if sr.Device != nil {
		if sr.Device.ManagementIP != "" {
			return sr.Device.ManagementIP
		}
		if sr.Device.Hostname != "" {
			return sr.Device.Hostname
		}
		return sr.Device.ID
	}
	if sr.Asset != nil {
		if sr.Asset.IP != "" {
			return sr.Asset.IP
		}
		if sr.Asset.Hostname != "" {
			return sr.Asset.Hostname
		}
		if sr.Asset.Mac != "" {
			return sr.Asset.Mac
		}
		if sr.Asset.Serial != "" {
			return sr.Asset.Serial
		}
		return sr.Asset.Id
	}
	return sr.SourceName
}

// ========================================================================
// Match Result
// ========================================================================

// MatchResult records a single match between two records.
type MatchResult struct {
	// RecordA and RecordB are the matched records.
	RecordA *SourceRecord `json:"record_a"`
	RecordB *SourceRecord `json:"record_b"`

	// MatchKey is the attribute that produced the match.
	MatchKey MatchKey `json:"match_key"`

	// MatchValue is the value that matched.
	MatchValue string `json:"match_value"`

	// Confidence is how confident we are in this match (0.0–1.0).
	Confidence float64 `json:"confidence"`

	// MatchTime is when the match was computed.
	MatchTime time.Time `json:"match_time"`
}

// ========================================================================
// DedupResult
// ========================================================================

// DedupResult is the output of device deduplication.
type DedupResult struct {
	// UniqueDevices contains the deduplicated device set.
	// Each entry represents a merged device from one or more sources.
	UniqueDevices []*SourceRecord `json:"unique_devices"`

	// Duplicates maps each merged-away record to its surviving canonical record.
	Duplicates map[string]string `json:"duplicates"`

	// Matches records all pairwise matches found during deduplication.
	Matches []MatchResult `json:"matches"`

	// Stats provides summary statistics.
	Stats DedupStats `json:"stats"`
}

// DedupStats summarizes deduplication results.
type DedupStats struct {
	// TotalInput is the number of records fed to the deduplicator.
	TotalInput int `json:"total_input"`

	// TotalOutput is the number of unique records after deduplication.
	TotalOutput int `json:"total_output"`

	// MatchesFound is the number of pairwise matches identified.
	MatchesFound int `json:"matches_found"`

	// MergedCount is the number of records merged into other records.
	MergedCount int `json:"merged_count"`

	// MatchByKey breaks down matches by the attribute that produced them.
	MatchByKey map[MatchKey]int `json:"match_by_key"`
}

// ========================================================================
// Merge Result
// ========================================================================

// MergeResult represents the result of merging two records.
type MergeResult struct {
	// Canonical is the merged record that should be used.
	Canonical *SourceRecord `json:"canonical"`

	// Sources lists all original sources merged into the canonical.
	Sources []string `json:"sources"`

	// Conflicts lists any conflicting field values found during merge.
	Conflicts []ConflictEntry `json:"conflicts,omitempty"`

	// MergedAt is when the merge was performed.
	MergedAt time.Time `json:"merged_at"`
}

// ========================================================================
// Conflict Detection
// ========================================================================

// ConflictEntry records a single field-level conflict between sources.
type ConflictEntry struct {
	// Field is the attribute that conflicts (e.g., "hostname", "vendor").
	Field string `json:"field"`

	// ValueA and ValueB are the conflicting values from each source.
	ValueA string `json:"value_a"`
	ValueB string `json:"value_b"`

	// SourceA and SourceB identify which sources produced each value.
	SourceA string `json:"source_a"`
	SourceB string `json:"source_b"`

	// ConfidenceA and ConfidenceB are the confidence scores for each value.
	ConfidenceA float64 `json:"confidence_a"`
	ConfidenceB float64 `json:"confidence_b"`

	// Resolution describes how the conflict was resolved.
	// "higher-confidence", "manual-override", "unresolved", "rule-based"
	Resolution string `json:"resolution"`

	// WinningValue is the value selected after resolution.
	WinningValue string `json:"winning_value"`

	// Severity is the conflict severity: "critical", "warning", "info".
	Severity string `json:"severity"`
}

// ConflictSeverity maps field names to their conflict severity.
// Device identity fields have higher severity — disagreement there is critical.
var ConflictSeverity = map[string]string{
	"hostname":      "critical",
	"vendor":        "critical",
	"model":         "critical",
	"management_ip": "critical",
	"serial":        "critical",
	"mac":           "critical",
	"os_name":       "warning",
	"os_version":    "warning",
	"site":          "warning",
	"role":          "info",
	"version":       "info",
}

// SeverityForField returns the conflict severity for a given field name.
func SeverityForField(field string) string {
	if s, ok := ConflictSeverity[field]; ok {
		return s
	}
	return "info"
}

// ========================================================================
// Link Correlation
// ========================================================================

// LinkCorrelationResult bundles the results of link correlation.
type LinkCorrelationResult struct {
	// CorrelatedLinks contains the matched links across sources.
	CorrelatedLinks []CorrelatedLink `json:"correlated_links"`

	// UnmatchedLinks are links that could not be correlated with any other source.
	UnmatchedLinks []models.Link `json:"unmatched_links"`

	// Stats provides link correlation statistics.
	Stats LinkCorrelationStats `json:"stats"`
}

// CorrelatedLink groups links from different sources that describe the same
// physical/logical connection.
type CorrelatedLink struct {
	// Canonical is the merged/best link representation.
	Canonical models.Link `json:"canonical"`

	// Sources lists the individual link records from each source.
	Sources []models.Link `json:"sources"`

	// SourceNames identifies which connectors produced each source link.
	SourceNames []string `json:"source_names"`

	// MatchConfidence is how confident we are in this correlation.
	MatchConfidence float64 `json:"match_confidence"`
}

// LinkCorrelationStats summarizes link correlation results.
type LinkCorrelationStats struct {
	TotalInput     int `json:"total_input"`
	Correlated     int `json:"correlated"`
	Unmatched      int `json:"unmatched"`
	CorrelatedSets int `json:"correlated_sets"`
}

// ========================================================================
// Correlation Report
// ========================================================================

// CorrelationReport is the final output of a correlation run.
type CorrelationReport struct {
	// GeneratedAt is when the report was produced.
	GeneratedAt time.Time `json:"generated_at"`

	// EngineVersion is the version of the correlation engine.
	EngineVersion string `json:"engine_version"`

	// InputSources lists all data sources that were correlated.
	InputSources []string `json:"input_sources"`

	// DeviceDedup contains the device deduplication results.
	DeviceDedup DedupResult `json:"device_dedup"`

	// LinkCorrelation contains link correlation results.
	LinkCorrelation LinkCorrelationResult `json:"link_correlation"`

	// Conflicts lists all cross-source conflicts found.
	Conflicts []ConflictEntry `json:"conflicts"`

	// Overrides lists manual overrides applied.
	Overrides []OverrideRecord `json:"overrides,omitempty"`

	// Summary is a human-readable summary.
	Summary string `json:"summary"`
}

// ========================================================================
// Manual Override
// ========================================================================

// OverrideRecord records a user-applied manual override for conflict resolution.
type OverrideRecord struct {
	// ID uniquely identifies this override.
	ID string `json:"id"`

	// Field is the attribute being overridden.
	Field string `json:"field"`

	// DeviceIdentifier identifies which device the override applies to.
	// This is the deduplicated device ID or a primary key.
	DeviceIdentifier string `json:"device_identifier"`

	// Source specifies which source's value to prefer, or "manual".
	Source string `json:"source"`

	// ManualValue is the explicit value when Source == "manual".
	ManualValue string `json:"manual_value,omitempty"`

	// Reason is why the override was applied.
	Reason string `json:"reason"`

	// AppliedBy identifies who applied the override.
	AppliedBy string `json:"applied_by"`

	// AppliedAt is when the override was created.
	AppliedAt time.Time `json:"applied_at"`

	// ExpiresAt is an optional expiration for temporary overrides.
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// Active returns whether the override is currently active.
func (o *OverrideRecord) Active() bool {
	if o.ExpiresAt == nil {
		return true
	}
	return time.Now().Before(*o.ExpiresAt)
}

// ========================================================================
// Correlation Rules Engine
// ========================================================================

// CorrelationRule defines a user-definable matching rule.
type CorrelationRule struct {
	// ID uniquely identifies this rule.
	ID string `json:"id"`

	// Name is a human-readable label.
	Name string `json:"name"`

	// Priority determines evaluation order (lower = evaluated first).
	Priority int `json:"priority"`

	// Enabled controls whether this rule is active.
	Enabled bool `json:"enabled"`

	// MatchKeys specifies which attributes to match on.
	MatchKeys []MatchKey `json:"match_keys"`

	// MatchMode determines how multiple keys are combined.
	// "any" = match on any key (OR), "all" = match on all keys (AND).
	MatchMode string `json:"match_mode"`

	// SourceFilters restricts which sources this rule applies to.
	// Empty means all sources.
	SourceFilters []string `json:"source_filters,omitempty"`

	// DeviceTypeFilter restricts which device types this rule applies to.
	DeviceTypeFilter []string `json:"device_type_filter,omitempty"`

	// MinConfidence is the minimum confidence threshold for matches from this rule.
	MinConfidence float64 `json:"min_confidence"`

	// Description explains the rule's purpose.
	Description string `json:"description"`

	// CreatedBy identifies who created the rule.
	CreatedBy string `json:"created_by"`

	// CreatedAt is when the rule was created.
	CreatedAt time.Time `json:"created_at"`
}

// RuleEngine manages a collection of correlation rules.
type RuleEngine struct {
	Rules []CorrelationRule `json:"rules"`
}

// NewRuleEngine creates a new rule engine with default rules.
func NewRuleEngine() *RuleEngine {
	return &RuleEngine{
		Rules: []CorrelationRule{
			{
				ID:          "default-serial",
				Name:        "Serial Number Match",
				Priority:    1,
				Enabled:     true,
				MatchKeys:   []MatchKey{MatchKeySerial},
				MatchMode:   "any",
				MinConfidence: 0.9,
				Description: "Match devices by serial number — highest confidence match",
				CreatedAt:   time.Now(),
			},
			{
				ID:          "default-ip",
				Name:        "Management IP Match",
				Priority:    2,
				Enabled:     true,
				MatchKeys:   []MatchKey{MatchKeyManagementIP},
				MatchMode:   "any",
				MinConfidence: 0.7,
				Description: "Match devices by management IP address",
				CreatedAt:   time.Now(),
			},
			{
				ID:          "default-mac",
				Name:        "MAC Address Match",
				Priority:    3,
				Enabled:     true,
				MatchKeys:   []MatchKey{MatchKeyMAC},
				MatchMode:   "any",
				MinConfidence: 0.6,
				Description: "Match devices by MAC address",
				CreatedAt:   time.Now(),
			},
			{
				ID:          "default-hostname",
				Name:        "Hostname Match",
				Priority:    4,
				Enabled:     true,
				MatchKeys:   []MatchKey{MatchKeyHostname},
				MatchMode:   "any",
				MinConfidence: 0.5,
				Description: "Match devices by hostname — lower confidence due to naming collisions",
				CreatedAt:   time.Now(),
			},
		},
	}
}

// ActiveRules returns enabled rules sorted by priority.
func (re *RuleEngine) ActiveRules() []CorrelationRule {
	active := make([]CorrelationRule, 0)
	for _, r := range re.Rules {
		if r.Enabled {
			active = append(active, r)
		}
	}
	// Sort by priority (already ordered in default set, but ensure stability).
	for i := 0; i < len(active); i++ {
		for j := i + 1; j < len(active); j++ {
			if active[j].Priority < active[i].Priority {
				active[i], active[j] = active[j], active[i]
			}
		}
	}
	return active
}

// AddRule appends a new rule to the engine.
func (re *RuleEngine) AddRule(rule CorrelationRule) {
	re.Rules = append(re.Rules, rule)
}

// RemoveRule removes a rule by ID. Returns true if found and removed.
func (re *RuleEngine) RemoveRule(id string) bool {
	for i, r := range re.Rules {
		if r.ID == id {
			re.Rules = append(re.Rules[:i], re.Rules[i+1:]...)
			return true
		}
	}
	return false
}

// GetRule retrieves a rule by ID. Returns nil if not found.
func (re *RuleEngine) GetRule(id string) *CorrelationRule {
	for i := range re.Rules {
		if re.Rules[i].ID == id {
			return &re.Rules[i]
		}
	}
	return nil
}

// ========================================================================
// Correlation Engine Config
// ========================================================================

// CorrelationConfig provides configuration for the correlation engine.
type CorrelationConfig struct {
	// DedupMinConfidence is the minimum confidence for a device match.
	DedupMinConfidence float64 `json:"dedup_min_confidence"`

	// LinkMatchMinConfidence is the minimum confidence for a link match.
	LinkMatchMinConfidence float64 `json:"link_match_min_confidence"`

	// MaxMergeDepth limits how many sources can merge into one device.
	MaxMergeDepth int `json:"max_merge_depth"`

	// StrictSerialMatch requires exact serial match (no fuzzy).
	StrictSerialMatch bool `json:"strict_serial_match"`

	// NormalizeMAC when true normalizes MAC addresses before comparison.
	NormalizeMAC bool `json:"normalize_mac"`

	// NormalizeHostname when true lowercases and strips domains before comparison.
	NormalizeHostname bool `json:"normalize_hostname"`

	// Rules is the correlation rules engine.
	Rules *RuleEngine `json:"-"` // Handled separately
}

// DefaultCorrelationConfig returns sensible defaults.
func DefaultCorrelationConfig() CorrelationConfig {
	return CorrelationConfig{
		DedupMinConfidence:     0.5,
		LinkMatchMinConfidence: 0.5,
		MaxMergeDepth:          10,
		StrictSerialMatch:      true,
		NormalizeMAC:           true,
		NormalizeHostname:      true,
	}
}

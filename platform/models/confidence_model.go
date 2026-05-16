package models

import "time"

// ========================================================================
// Confidence Scoring v2 — Per-Attribute, Propagation, Corroboration, Staleness
// ========================================================================
// Sprint 9: Every device, interface, and link has a quantified confidence
// score that analysts can trust. This file defines the confidence model
// types used throughout the platform.

// ConfidenceSource identifies where confidence evidence originates.
type ConfidenceSource string

const (
	ConfidenceSourceConfig      ConfidenceSource = "config"      // From device configuration parsing
	ConfidenceSourceOperational ConfidenceSource = "operational" // From show commands / operational state
	ConfidenceSourceInferred    ConfidenceSource = "inferred"    // Derived from topology inference
	ConfidenceSourceConnector   ConfidenceSource = "connector"   // From external connectors (NetBox, ServiceNow, etc.)
	ConfidenceSourceManual      ConfidenceSource = "manual"      // User-provided override
)

// ConfidenceTier maps score ranges to human-readable tiers.
type ConfidenceTier string

const (
	ConfidenceTierHigh   ConfidenceTier = "high"   // >= 0.80
	ConfidenceTierMedium ConfidenceTier = "medium" // >= 0.50, < 0.80
	ConfidenceTierLow    ConfidenceTier = "low"    // >= 0.20, < 0.50
	ConfidenceTierNone   ConfidenceTier = "none"   // < 0.20
)

// SourceWeights define default confidence weights for each evidence source type.
// Config sources are most trusted, inferred least.
var SourceWeights = map[ConfidenceSource]float64{
	ConfidenceSourceConfig:      0.90,
	ConfidenceSourceOperational: 0.80,
	ConfidenceSourceConnector:   0.70,
	ConfidenceSourceInferred:    0.40,
	ConfidenceSourceManual:      1.00,
}

// ========================================================================
// Per-Attribute Confidence
// ========================================================================

// AttributeConfidence tracks confidence for a single field on a record.
type AttributeConfidence struct {
	Field     string           `json:"field"`     // e.g., "hostname", "vendor", "ip_address"
	Score     float64          `json:"score"`     // 0.0 – 1.0
	Source    ConfidenceSource `json:"source"`    // Where the evidence came from
	Rationale string           `json:"rationale"` // Human-readable explanation
}

// PerAttributeConfidence is a map of field name to its confidence score.
// Attached to any topology record to provide per-field trust quantification.
type PerAttributeConfidence map[string]float64

// ConfidenceProfile attaches full confidence metadata to a topology record.
// This is the primary structure attached to Device, Interface, Link, etc.
type ConfidenceProfile struct {
	// Overall confidence score (0.0–1.0) — derived from per-attribute scores.
	Overall float64 `json:"overall"`

	// Per-attribute confidence scores keyed by field name.
	PerAttribute PerAttributeConfidence `json:"per_attribute"`

	// Attributes contains detailed per-field confidence breakdown.
	Attributes []AttributeConfidence `json:"attributes,omitempty"`

	// Tier is the human-readable confidence tier derived from Overall.
	Tier ConfidenceTier `json:"tier"`

	// EvidenceSources lists all sources contributing to this profile.
	EvidenceSources []ConfidenceSource `json:"evidence_sources"`

	// EvidenceCount is the number of independent evidence items.
	EvidenceCount int `json:"evidence_count"`

	// CorroborationBoost is the bonus applied from multi-source agreement.
	CorroborationBoost float64 `json:"corroboration_boost"`

	// StalenessDecay is the penalty applied from data aging.
	StalenessDecay float64 `json:"staleness_decay"`

	// LastUpdated is when the confidence was last computed.
	LastUpdated time.Time `json:"last_updated"`

	// ComputedBy identifies the engine version that produced this profile.
	ComputedBy string `json:"computed_by"`
}

// NewConfidenceProfile creates a new profile with sensible defaults.
func NewConfidenceProfile() *ConfidenceProfile {
	return &ConfidenceProfile{
		Overall:            0.0,
		PerAttribute:       make(PerAttributeConfidence),
		Attributes:         make([]AttributeConfidence, 0),
		Tier:               ConfidenceTierNone,
		EvidenceSources:    make([]ConfidenceSource, 0),
		EvidenceCount:      0,
		CorroborationBoost: 0.0,
		StalenessDecay:     0.0,
		LastUpdated:        time.Now(),
		ComputedBy:         "confidence-v2",
	}
}

// ComputeTier determines the confidence tier from the overall score.
func (cp *ConfidenceProfile) ComputeTier() ConfidenceTier {
	switch {
	case cp.Overall >= 0.80:
		return ConfidenceTierHigh
	case cp.Overall >= 0.50:
		return ConfidenceTierMedium
	case cp.Overall >= 0.20:
		return ConfidenceTierLow
	default:
		return ConfidenceTierNone
	}
}

// SetOverall computes the overall score from per-attribute scores.
// Uses weighted average with equal field weights by default.
func (cp *ConfidenceProfile) SetOverall() {
	if len(cp.PerAttribute) == 0 {
		cp.Overall = 0.0
		cp.Tier = ConfidenceTierNone
		return
	}
	var sum float64
	for _, score := range cp.PerAttribute {
		sum += score
	}
	cp.Overall = sum / float64(len(cp.PerAttribute))

	// Apply corroboration boost (capped at 1.0)
	if cp.CorroborationBoost > 0 {
		cp.Overall = clampScore(cp.Overall + cp.CorroborationBoost)
	}

	// Apply staleness decay
	if cp.StalenessDecay > 0 {
		cp.Overall = clampScore(cp.Overall - cp.StalenessDecay)
	}

	cp.Tier = cp.ComputeTier()
}

// AddAttribute adds a per-attribute confidence entry.
func (cp *ConfidenceProfile) AddAttribute(field string, score float64, source ConfidenceSource, rationale string) {
	cp.PerAttribute[field] = score
	cp.Attributes = append(cp.Attributes, AttributeConfidence{
		Field:     field,
		Score:     score,
		Source:    source,
		Rationale: rationale,
	})
	// Track unique sources
	if !containsSource(cp.EvidenceSources, source) {
		cp.EvidenceSources = append(cp.EvidenceSources, source)
	}
	cp.EvidenceCount++
}

// ========================================================================
// Confidence Propagation
// ========================================================================

// PropagationConfig controls how confidence flows through the topology.
type PropagationConfig struct {
	// DecayFactor is the multiplier applied at each propagation hop (0.0–1.0).
	// Default: 0.95 — each hop loses 5% confidence.
	DecayFactor float64

	// MaxHops limits how far confidence propagates. 0 = unlimited.
	MaxHops int

	// PropagateFrom defines which entity types serve as propagation sources.
	PropagateFrom []string // e.g., ["Device", "Interface"]
}

// DefaultPropagationConfig returns sensible propagation defaults.
func DefaultPropagationConfig() PropagationConfig {
	return PropagationConfig{
		DecayFactor:   0.95,
		MaxHops:       5,
		PropagateFrom: []string{"Device", "Interface"},
	}
}

// ========================================================================
// Corroboration Multiplier
// ========================================================================

// CorroborationConfig controls the multi-source agreement boost.
type CorroborationConfig struct {
	// BoostPerSource is the confidence boost per corroborating source (0.0–1.0).
	// Default: 0.05 — each agreeing source adds 5% confidence.
	BoostPerSource float64

	// MaxBoost caps the total corroboration boost (0.0–1.0).
	// Default: 0.20 — maximum 20% boost from corroboration.
	MaxBoost float64

	// RequireIndependentSources if true, only counts distinct source types.
	RequireIndependentSources bool
}

// DefaultCorroborationConfig returns sensible corroboration defaults.
func DefaultCorroborationConfig() CorroborationConfig {
	return CorroborationConfig{
		BoostPerSource:            0.05,
		MaxBoost:                  0.20,
		RequireIndependentSources: true,
	}
}

// CorroborationEvidence records one piece of corroborating evidence.
type CorroborationEvidence struct {
	SourceType ConfidenceSource `json:"source_type"`
	SourceName string           `json:"source_name"` // e.g., "netbox", "ssh-cisco-ios"
	Field      string           `json:"field"`       // Field being corroborated
	Value      string           `json:"value"`       // The agreed-upon value
	Timestamp  time.Time        `json:"timestamp"`   // When this evidence was collected
}

// ========================================================================
// Staleness Decay
// ========================================================================

// StalenessConfig controls how confidence decays over time.
type StalenessConfig struct {
	// HalfLifeDays is the number of days after which confidence halves.
	// Default: 30 — after 30 days without refresh, confidence = 0.5.
	HalfLifeDays float64

	// MaxAgeDays is the age after which confidence reaches zero.
	// Default: 365 — records older than a year have zero confidence.
	MaxAgeDays float64

	// DecayModel selects the decay curve.
	// "exponential" (default), "linear"
	DecayModel string
}

// DefaultStalenessConfig returns sensible staleness defaults.
func DefaultStalenessConfig() StalenessConfig {
	return StalenessConfig{
		HalfLifeDays: 30,
		MaxAgeDays:   365,
		DecayModel:   "exponential",
	}
}

// ========================================================================
// Confidence Visualization Hints
// ========================================================================

// VisualizationHint provides rendering guidance for graph consumers.
type VisualizationHint struct {
	// Color is a hex color string (e.g., "#00ff00" for high confidence).
	Color string `json:"color"`

	// Opacity is the alpha value (0.0–1.0) derived from confidence.
	Opacity float64 `json:"opacity"`

	// Thickness is the edge/line thickness multiplier (applies to links).
	Thickness float64 `json:"thickness,omitempty"`

	// Size is the node size multiplier (applies to devices).
	Size float64 `json:"size,omitempty"`

	// Label is an optional text annotation for the confidence level.
	Label string `json:"label,omitempty"`

	// DashPattern is an SVG-like dash array for uncertain links.
	DashPattern string `json:"dash_pattern,omitempty"`
}

// ConfidenceColorMap maps confidence tiers to visualization colors.
var ConfidenceColorMap = map[ConfidenceTier]string{
	ConfidenceTierHigh:   "#22c55e", // Green
	ConfidenceTierMedium: "#eab308", // Yellow/amber
	ConfidenceTierLow:    "#f97316", // Orange
	ConfidenceTierNone:   "#ef4444", // Red
}

// ComputeVisualizationHint derives visualization properties from a confidence score.
func ComputeVisualizationHint(score float64, isLink bool) VisualizationHint {
	hint := VisualizationHint{}

	switch {
	case score >= 0.80:
		hint.Color = ConfidenceColorMap[ConfidenceTierHigh]
		hint.Opacity = 0.95
		hint.Label = "High"
	case score >= 0.50:
		hint.Color = ConfidenceColorMap[ConfidenceTierMedium]
		hint.Opacity = 0.75
		hint.Label = "Medium"
	case score >= 0.20:
		hint.Color = ConfidenceColorMap[ConfidenceTierLow]
		hint.Opacity = 0.55
		hint.Label = "Low"
		hint.DashPattern = "5,5"
	default:
		hint.Color = ConfidenceColorMap[ConfidenceTierNone]
		hint.Opacity = 0.35
		hint.Label = "None"
		hint.DashPattern = "2,4"
	}

	if isLink {
		// Links use thickness to convey confidence
		hint.Thickness = 0.5 + (score * 2.5) // range: 0.5 to 3.0
	} else {
		// Nodes use size
		hint.Size = 0.8 + (score * 0.4) // range: 0.8 to 1.2
	}

	return hint
}

// ========================================================================
// Summary Statistics
// ========================================================================

// ConfidenceSummaryStats provides aggregate confidence metrics for a topology export.
type ConfidenceSummaryStats struct {
	// TotalRecords is the total number of scored records.
	TotalRecords int `json:"total_records"`

	// RecordsByTier breaks down records by confidence tier.
	RecordsByTier map[ConfidenceTier]int `json:"records_by_tier"`

	// AverageConfidence is the mean overall score across all records.
	AverageConfidence float64 `json:"average_confidence"`

	// MedianConfidence is the median overall score.
	MedianConfidence float64 `json:"median_confidence"`

	// MinConfidence is the lowest overall score.
	MinConfidence float64 `json:"min_confidence"`

	// MaxConfidence is the highest overall score.
	MaxConfidence float64 `json:"max_confidence"`

	// AverageCorroboration is the mean corroboration boost across records.
	AverageCorroboration float64 `json:"average_corroboration"`

	// AverageStalenessDecay is the mean staleness penalty across records.
	AverageStalenessDecay float64 `json:"average_staleness_decay"`

	// RecordsWithCorroboration counts records that benefited from multi-source agreement.
	RecordsWithCorroboration int `json:"records_with_corroboration"`

	// RecordsStale counts records that have staleness decay applied.
	RecordsStale int `json:"records_stale"`

	// AverageEvidenceCount is the mean number of evidence sources per record.
	AverageEvidenceCount float64 `json:"average_evidence_count"`

	// GeneratedAt is when these stats were computed.
	GeneratedAt time.Time `json:"generated_at"`
}

// NewConfidenceSummaryStats initializes an empty stats structure.
func NewConfidenceSummaryStats() *ConfidenceSummaryStats {
	return &ConfidenceSummaryStats{
		RecordsByTier: make(map[ConfidenceTier]int),
		GeneratedAt:   time.Now(),
	}
}

// ========================================================================
// Min-Confidence Filter
// ========================================================================

// ConfidenceFilter provides filtering options for exports.
type ConfidenceFilter struct {
	// MinConfidence is the minimum overall confidence threshold (0.0–1.0).
	MinConfidence float64 `json:"min_confidence"`

	// MinTier is an alternative tier-based filter.
	MinTier ConfidenceTier `json:"min_tier,omitempty"`

	// ExcludeSources filters out records from specific evidence sources.
	ExcludeSources []ConfidenceSource `json:"exclude_sources,omitempty"`

	// RequireCorroboration only includes records with at least N sources.
	RequireCorroboration int `json:"require_corroboration,omitempty"`

	// MaxStaleness excludes records with staleness decay above this threshold.
	MaxStaleness float64 `json:"max_staleness,omitempty"`
}

// DefaultConfidenceFilter returns a filter that passes everything.
func DefaultConfidenceFilter() ConfidenceFilter {
	return ConfidenceFilter{
		MinConfidence: 0.0,
	}
}

// ========================================================================
// Helpers
// ========================================================================

func clampScore(s float64) float64 {
	if s < 0.0 {
		return 0.0
	}
	if s > 1.0 {
		return 1.0
	}
	return s
}

func containsSource(sources []ConfidenceSource, target ConfidenceSource) bool {
	for _, s := range sources {
		if s == target {
			return true
		}
	}
	return false
}

// TierFromScore returns the confidence tier for a given score.
func TierFromScore(score float64) ConfidenceTier {
	switch {
	case score >= 0.80:
		return ConfidenceTierHigh
	case score >= 0.50:
		return ConfidenceTierMedium
	case score >= 0.20:
		return ConfidenceTierLow
	default:
		return ConfidenceTierNone
	}
}

// MinTierScore returns the minimum score for a given tier.
func MinTierScore(tier ConfidenceTier) float64 {
	switch tier {
	case ConfidenceTierHigh:
		return 0.80
	case ConfidenceTierMedium:
		return 0.50
	case ConfidenceTierLow:
		return 0.20
	default:
		return 0.0
	}
}

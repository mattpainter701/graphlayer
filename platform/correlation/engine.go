// Package correlation
// Sprint 8: Cross-Source Deduplication and Correlation Engine
//
// The CorrelationEngine is the central hub that orchestrates device dedup,
// link correlation, conflict detection, and report generation across
// multiple data sources.
package correlation

import (
	"fmt"
	"time"

	"github.com/mattpainter701/graphlayer/platform/models"
)

// ========================================================================
// Correlation Engine — main orchestrator
// ========================================================================

// CorrelationEngine ties together dedup, link correlation, conflict
// detection, and reporting into a unified pipeline.
type CorrelationEngine struct {
	Config           CorrelationConfig
	Deduplicator     *DeviceDeduplicator
	LinkCorrelator   *LinkCorrelator
	ConflictDetector *ConflictDetector
	ReportGenerator  *ReportGenerator
}

// NewCorrelationEngine creates a new engine with default configuration.
func NewCorrelationEngine() *CorrelationEngine {
	cfg := DefaultCorrelationConfig()
	return &CorrelationEngine{
		Config:           cfg,
		Deduplicator:     NewDeviceDeduplicatorWithConfig(cfg),
		LinkCorrelator:   NewLinkCorrelatorWithConfig(cfg),
		ConflictDetector: NewConflictDetector(),
		ReportGenerator:  NewReportGenerator(),
	}
}

// NewCorrelationEngineWithConfig creates an engine with custom configuration.
func NewCorrelationEngineWithConfig(cfg CorrelationConfig) *CorrelationEngine {
	return &CorrelationEngine{
		Config:           cfg,
		Deduplicator:     NewDeviceDeduplicatorWithConfig(cfg),
		LinkCorrelator:   NewLinkCorrelatorWithConfig(cfg),
		ConflictDetector: NewConflictDetector(),
		ReportGenerator:  NewReportGenerator(),
	}
}

// ========================================================================
// Main Pipeline
// ========================================================================

// CorrelateInput is the main entry point: takes source records from multiple
// connectors and produces a deduplicated, correlated topology with conflict
// reporting.
func (ce *CorrelationEngine) CorrelateInput(records []*SourceRecord) *CorrelationReport {
	inputSources := ce.extractSourceNames(records)

	// Step 1: Device deduplication
	dedupResult := ce.Deduplicator.Deduplicate(records)

	// Step 2: Link correlation
	sourceLinks := ce.extractLinks(records)
	linkResult := ce.LinkCorrelator.CorrelateLinks(sourceLinks)

	// Step 3: Cross-source conflict detection
	conflicts := ce.ConflictDetector.DetectAllConflicts(dedupResult, records)

	// Step 4: Generate report
	report := ce.ReportGenerator.GenerateReport(
		inputSources,
		dedupResult,
		linkResult,
		conflicts,
		ce.ConflictDetector.GetActiveOverrides(),
	)

	return report
}

// CorrelateWithRules runs correlation using a specific rules engine.
func (ce *CorrelationEngine) CorrelateWithRules(records []*SourceRecord, rules *RuleEngine) *CorrelationReport {
	// Apply rules to filter/prioritize record matching
	filteredRecords := ce.applyRules(records, rules)
	return ce.CorrelateInput(filteredRecords)
}

// ========================================================================
// Convenience Methods
// ========================================================================

// DeduplicateDevices is a shortcut for device-only dedup.
func (ce *CorrelationEngine) DeduplicateDevices(records []*SourceRecord) *DedupResult {
	return ce.Deduplicator.Deduplicate(records)
}

// CorrelateLinksOnly is a shortcut for link-only correlation.
func (ce *CorrelationEngine) CorrelateLinksOnly(sourceLinks []SourceLink) *LinkCorrelationResult {
	return ce.LinkCorrelator.CorrelateLinks(sourceLinks)
}

// DetectConflictsOnly is a shortcut for conflict detection.
func (ce *CorrelationEngine) DetectConflictsOnly(
	canonical *SourceRecord,
	sources []*SourceRecord,
) []ConflictEntry {
	return ce.ConflictDetector.DetectConflicts(canonical, sources)
}

// AddOverride registers a manual override.
func (ce *CorrelationEngine) AddOverride(override OverrideRecord) {
	ce.ConflictDetector.AddOverride(override)
}

// RemoveOverride removes a manual override by ID.
func (ce *CorrelationEngine) RemoveOverride(id string) bool {
	return ce.ConflictDetector.RemoveOverride(id)
}

// GetRules returns the active rules engine.
func (ce *CorrelationEngine) GetRules() *RuleEngine {
	if ce.Config.Rules == nil {
		ce.Config.Rules = NewRuleEngine()
	}
	return ce.Config.Rules
}

// ========================================================================
// Internal Helpers
// ========================================================================

func (ce *CorrelationEngine) extractSourceNames(records []*SourceRecord) []string {
	seen := make(map[string]bool)
	names := make([]string, 0)
	for _, r := range records {
		if !seen[r.SourceName] {
			seen[r.SourceName] = true
			names = append(names, r.SourceName)
		}
	}
	return names
}

func (ce *CorrelationEngine) extractLinks(records []*SourceRecord) []SourceLink {
	// Group links by source name
	linkMap := make(map[string][]models.Link)
	for _, r := range records {
		linkMap[r.SourceName] = append(linkMap[r.SourceName], r.Links...)
	}

	result := make([]SourceLink, 0)
	for src, links := range linkMap {
		result = append(result, SourceLink{
			SourceName: src,
			Links:      links,
		})
	}
	return result
}

func (ce *CorrelationEngine) applyRules(records []*SourceRecord, rules *RuleEngine) []*SourceRecord {
	if rules == nil {
		return records
	}

	activeRules := rules.ActiveRules()
	if len(activeRules) == 0 {
		return records
	}

	// Apply source filters from rules
	filtered := make([]*SourceRecord, 0)
	for _, r := range records {
		include := true
		for _, rule := range activeRules {
			// If rule has source filters and this record's source isn't in them, skip
			if len(rule.SourceFilters) > 0 {
				found := false
				for _, sf := range rule.SourceFilters {
					if sf == r.SourceName {
						found = true
						break
					}
				}
				if !found {
					include = false
					break
				}
			}

			// Apply device type filter
			if len(rule.DeviceTypeFilter) > 0 && r.Device != nil {
				found := false
				for _, dt := range rule.DeviceTypeFilter {
					if dt == r.Device.Role || dt == r.Device.Vendor {
						found = true
						break
					}
				}
				if !found {
					include = false
					break
				}
			}
		}
		if include {
			filtered = append(filtered, r)
		}
	}

	return filtered
}

// ========================================================================
// Static Convenience Functions
// ========================================================================

// QuickCorrelate is a one-shot convenience function for simple use cases.
// It takes a list of source records and returns a correlation report.
func QuickCorrelate(records []*SourceRecord) *CorrelationReport {
	engine := NewCorrelationEngine()
	return engine.CorrelateInput(records)
}

// ========================================================================
// Utility: Build SourceRecord from common types
// ========================================================================

// NewSourceRecordFromDevice creates a SourceRecord from a Device model.
func NewSourceRecordFromDevice(sourceName string, sourceType models.ConfidenceSource, device *models.Device) *SourceRecord {
	return &SourceRecord{
		SourceName:  sourceName,
		SourceType:  sourceType,
		Device:      device,
		CollectedAt: time.Now(),
		Metadata:    make(map[string]string),
	}
}

// NewSourceRecordFromAsset creates a SourceRecord from an AssetNode model.
func NewSourceRecordFromAsset(sourceName string, sourceType models.ConfidenceSource, asset *models.AssetNode) *SourceRecord {
	return &SourceRecord{
		SourceName:  sourceName,
		SourceType:  sourceType,
		Asset:       asset,
		CollectedAt: time.Now(),
		Metadata:    make(map[string]string),
	}
}

// ========================================================================
// Engine Info
// ========================================================================

// Version returns the engine version string.
func (ce *CorrelationEngine) Version() string {
	return "correlation-v1"
}

// EngineStats returns runtime statistics about the engine configuration.
func (ce *CorrelationEngine) EngineStats() map[string]interface{} {
	return map[string]interface{}{
		"version":                ce.Version(),
		"dedup_min_confidence":   ce.Config.DedupMinConfidence,
		"link_min_confidence":    ce.Config.LinkMatchMinConfidence,
		"max_merge_depth":        ce.Config.MaxMergeDepth,
		"strict_serial_match":    ce.Config.StrictSerialMatch,
		"normalize_mac":          ce.Config.NormalizeMAC,
		"normalize_hostname":     ce.Config.NormalizeHostname,
		"active_overrides":       len(ce.ConflictDetector.GetActiveOverrides()),
		"active_rules":           len(ce.GetRules().ActiveRules()),
	}
}

// Ensure fmt import used
var _ = fmt.Sprintf

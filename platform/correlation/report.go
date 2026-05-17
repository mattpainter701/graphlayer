package correlation

import (
	"fmt"
	"strings"
	"time"
)

// ========================================================================
// Correlation Report Generator
// ========================================================================

// ReportGenerator produces the final correlation report from engine results.
type ReportGenerator struct {
	EngineVersion string
}

// NewReportGenerator creates a new report generator.
func NewReportGenerator() *ReportGenerator {
	return &ReportGenerator{
		EngineVersion: "correlation-v1",
	}
}

// GenerateReport produces a comprehensive correlation report.
func (rg *ReportGenerator) GenerateReport(
	inputSources []string,
	dedup *DedupResult,
	linkCorr *LinkCorrelationResult,
	conflicts []ConflictEntry,
	overrides []OverrideRecord,
) *CorrelationReport {

	report := &CorrelationReport{
		GeneratedAt:    time.Now(),
		EngineVersion:  rg.EngineVersion,
		InputSources:   inputSources,
		DeviceDedup:    *dedup,
		LinkCorrelation: *linkCorr,
		Conflicts:      conflicts,
		Overrides:      overrides,
		Summary:        rg.buildSummary(dedup, linkCorr, conflicts),
	}

	return report
}

func (rg *ReportGenerator) buildSummary(
	dedup *DedupResult,
	linkCorr *LinkCorrelationResult,
	conflicts []ConflictEntry,
) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Correlation Report — %s\n", time.Now().Format(time.RFC3339)))
	sb.WriteString(strings.Repeat("=", 60) + "\n\n")

	// Device dedup summary
	sb.WriteString("Device Deduplication:\n")
	sb.WriteString(fmt.Sprintf("  Input records:    %d\n", dedup.Stats.TotalInput))
	sb.WriteString(fmt.Sprintf("  Unique devices:   %d\n", dedup.Stats.TotalOutput))
	sb.WriteString(fmt.Sprintf("  Merged away:      %d\n", dedup.Stats.MergedCount))
	sb.WriteString(fmt.Sprintf("  Matches found:    %d\n", dedup.Stats.MatchesFound))

	if len(dedup.Stats.MatchByKey) > 0 {
		sb.WriteString("  Match breakdown:\n")
		for key, count := range dedup.Stats.MatchByKey {
			sb.WriteString(fmt.Sprintf("    %-18s %d\n", string(key)+":", count))
		}
	}
	sb.WriteString("\n")

	// Link correlation summary
	sb.WriteString("Link Correlation:\n")
	sb.WriteString(fmt.Sprintf("  Total links:      %d\n", linkCorr.Stats.TotalInput))
	sb.WriteString(fmt.Sprintf("  Correlated:       %d\n", linkCorr.Stats.Correlated))
	sb.WriteString(fmt.Sprintf("  Unmatched:        %d\n", linkCorr.Stats.Unmatched))
	sb.WriteString(fmt.Sprintf("  Correlated sets:  %d\n", linkCorr.Stats.CorrelatedSets))
	sb.WriteString("\n")

	// Conflict summary
	conflictSummary := SummarizeConflicts(conflicts)
	sb.WriteString("Cross-Source Conflicts:\n")
	sb.WriteString(fmt.Sprintf("  %s\n", conflictSummary.FormatSummary()))

	if conflictSummary.TotalConflicts > 0 {
		sb.WriteString("  By field:\n")
		for field, count := range conflictSummary.ByField {
			sb.WriteString(fmt.Sprintf("    %-18s %d\n", field+":", count))
		}
	}

	return sb.String()
}

// ========================================================================
// Report Formatting Helpers
// ========================================================================

// FormatMatchResult returns a human-readable string for a match result.
func FormatMatchResult(mr MatchResult) string {
	return fmt.Sprintf(
		"MATCH: %-15s %s  <=>  %s  (via %s = %s, confidence=%.2f)",
		mr.MatchKey,
		mr.RecordA.PrimaryIdentifier(),
		mr.RecordB.PrimaryIdentifier(),
		mr.MatchKey,
		mr.MatchValue,
		mr.Confidence,
	)
}

// FormatConflict returns a human-readable string for a conflict entry.
func FormatConflict(ce ConflictEntry) string {
	return fmt.Sprintf(
		"[%s] %s: %s (%s, %.2f) vs %s (%s, %.2f) -> %s = %s",
		strings.ToUpper(ce.Severity),
		ce.Field,
		ce.ValueA, ce.SourceA, ce.ConfidenceA,
		ce.ValueB, ce.SourceB, ce.ConfidenceB,
		ce.Resolution,
		ce.WinningValue,
	)
}

// ========================================================================
// Device-Level Reports
// ========================================================================

// DeviceDedupReport provides detailed dedup info for a single device.
type DeviceDedupReport struct {
	CanonicalID     string        `json:"canonical_id"`
	CanonicalName   string        `json:"canonical_name"`
	NumSources      int           `json:"num_sources"`
	SourceNames     []string      `json:"source_names"`
	Matches         []MatchResult `json:"matches"`
	Conflicts       []ConflictEntry `json:"conflicts"`
	OverrideCount   int           `json:"override_count"`
}

// BuildDeviceDedupReport builds a per-device dedup report.
func BuildDeviceDedupReport(
	canonical *SourceRecord,
	matches []MatchResult,
	conflicts []ConflictEntry,
	overrides []OverrideRecord,
) DeviceDedupReport {

	report := DeviceDedupReport{
		CanonicalID:   canonical.PrimaryIdentifier(),
		CanonicalName: canonical.SourceName,
		SourceNames:   make([]string, 0),
		Matches:       make([]MatchResult, 0),
		Conflicts:     make([]ConflictEntry, 0),
	}

	// Extract source names from metadata
	if mergedFrom, ok := canonical.Metadata["merged_from"]; ok {
		report.SourceNames = strings.Split(mergedFrom, ",")
	}
	if mc, ok := canonical.Metadata["merge_count"]; ok {
		fmt.Sscanf(mc, "%d", &report.NumSources)
	}
	if report.NumSources == 0 {
		report.NumSources = 1
		report.SourceNames = []string{canonical.SourceName}
	}

	// Filter matches for this device
	canonicalID := canonical.PrimaryIdentifier()
	for _, m := range matches {
		if m.RecordA.PrimaryIdentifier() == canonicalID || m.RecordB.PrimaryIdentifier() == canonicalID {
			report.Matches = append(report.Matches, m)
		}
	}

	// Filter conflicts for this device
	report.Conflicts = conflicts

	// Count active overrides for this device
	for _, o := range overrides {
		if o.Active() && o.DeviceIdentifier == canonicalID {
			report.OverrideCount++
		}
	}

	return report
}

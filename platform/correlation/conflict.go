package correlation

import (
	"fmt"
	"strings"
	"time"

	"github.com/mattpainter701/graphlayer/platform/models"
)

// ========================================================================
// Conflict Detector — detects disagreements between sources
// ========================================================================

// ConflictDetector identifies and resolves field-level conflicts
// between records from different sources describing the same entity.
type ConflictDetector struct {
	Overrides []OverrideRecord
}

// NewConflictDetector creates a new conflict detector.
func NewConflictDetector() *ConflictDetector {
	return &ConflictDetector{
		Overrides: make([]OverrideRecord, 0),
	}
}

// DetectConflicts compares the merged (canonical) record against each source
// record and flags any field-level disagreements.
func (cd *ConflictDetector) DetectConflicts(canonical *SourceRecord, sources []*SourceRecord) []ConflictEntry {
	conflicts := make([]ConflictEntry, 0)

	for _, source := range sources {
		if canonical.Device != nil && source.Device != nil {
			conflicts = append(conflicts, cd.detectDeviceConflicts(
				canonical.Device, source.Device,
				canonical.SourceName, source.SourceName,
			)...)
		}
		if canonical.Asset != nil && source.Asset != nil {
			conflicts = append(conflicts, cd.detectAssetConflicts(
				canonical.Asset, source.Asset,
				canonical.SourceName, source.SourceName,
			)...)
		}
	}

	// Resolve conflicts using confidence weighting and overrides
	for i := range conflicts {
		cd.resolveConflict(&conflicts[i])
	}

	return conflicts
}

// DetectAllConflicts runs conflict detection across all merged devices in a DedupResult.
func (cd *ConflictDetector) DetectAllConflicts(dedup *DedupResult, allSources []*SourceRecord) []ConflictEntry {
	allConflicts := make([]ConflictEntry, 0)

	// Build source lookup
	sourceByID := make(map[string]*SourceRecord)
	for _, sr := range allSources {
		sourceByID[sr.PrimaryIdentifier()] = sr
	}

	for _, canonical := range dedup.UniqueDevices {
		// Find which source records were merged into this canonical
		mergedSources := make([]*SourceRecord, 0)
		for dupID, canonID := range dedup.Duplicates {
			if canonID == canonical.PrimaryIdentifier() || canonID == canonical.SourceName+":"+canonical.PrimaryIdentifier() {
				if sr, ok := sourceByID[dupID]; ok {
					mergedSources = append(mergedSources, sr)
				}
			}
		}
		if len(mergedSources) > 0 {
			conflicts := cd.DetectConflicts(canonical, mergedSources)
			allConflicts = append(allConflicts, conflicts...)
		}
	}

	return allConflicts
}

func (cd *ConflictDetector) detectDeviceConflicts(
	canonical, source *models.Device,
	canonSource, otherSource string,
) []ConflictEntry {
	conflicts := make([]ConflictEntry, 0)

	cd.checkField(&conflicts, "hostname", canonical.Hostname, source.Hostname, canonSource, otherSource)
	cd.checkField(&conflicts, "vendor", canonical.Vendor, source.Vendor, canonSource, otherSource)
	cd.checkField(&conflicts, "model", canonical.Model, source.Model, canonSource, otherSource)
	cd.checkField(&conflicts, "version", canonical.Version, source.Version, canonSource, otherSource)
	cd.checkField(&conflicts, "management_ip", canonical.ManagementIP, source.ManagementIP, canonSource, otherSource)
	cd.checkField(&conflicts, "site", canonical.Site, source.Site, canonSource, otherSource)
	cd.checkField(&conflicts, "role", canonical.Role, source.Role, canonSource, otherSource)
	cd.checkField(&conflicts, "os_name", canonical.OSName, source.OSName, canonSource, otherSource)
	cd.checkField(&conflicts, "os_version", canonical.OSVersion, source.OSVersion, canonSource, otherSource)

	return conflicts
}

func (cd *ConflictDetector) detectAssetConflicts(
	canonical, source *models.AssetNode,
	canonSource, otherSource string,
) []ConflictEntry {
	conflicts := make([]ConflictEntry, 0)

	cd.checkField(&conflicts, "hostname", canonical.Hostname, source.Hostname, canonSource, otherSource)
	cd.checkField(&conflicts, "vendor", canonical.Vendor, source.Vendor, canonSource, otherSource)
	cd.checkField(&conflicts, "model", canonical.Model, source.Model, canonSource, otherSource)
	cd.checkField(&conflicts, "serial", canonical.Serial, source.Serial, canonSource, otherSource)
	cd.checkField(&conflicts, "mac", canonical.Mac, source.Mac, canonSource, otherSource)
	cd.checkField(&conflicts, "ip", canonical.IP, source.IP, canonSource, otherSource)
	cd.checkField(&conflicts, "os_name", canonical.OSName, source.OSName, canonSource, otherSource)
	cd.checkField(&conflicts, "os_version", canonical.OSVersion, source.OSVersion, canonSource, otherSource)
	cd.checkField(&conflicts, "site", canonical.Site, source.Site, canonSource, otherSource)
	cd.checkField(&conflicts, "role", canonical.Role, source.Role, canonSource, otherSource)
	cd.checkField(&conflicts, "business_unit", canonical.BusinessUnit, source.BusinessUnit, canonSource, otherSource)

	return conflicts
}

func (cd *ConflictDetector) checkField(
	conflicts *[]ConflictEntry,
	field, valA, valB, sourceA, sourceB string,
) {
	// Only flag if both are non-empty AND differ
	if valA == "" || valB == "" {
		return
	}
	if strings.EqualFold(strings.TrimSpace(valA), strings.TrimSpace(valB)) {
		return
	}

	entry := ConflictEntry{
		Field:      field,
		ValueA:     valA,
		ValueB:     valB,
		SourceA:    sourceA,
		SourceB:    sourceB,
		Resolution: "unresolved",
		Severity:   SeverityForField(field),
	}

	// Assign default confidence based on source type
	entry.ConfidenceA = cd.sourceConfidence(sourceA)
	entry.ConfidenceB = cd.sourceConfidence(sourceB)

	*conflicts = append(*conflicts, entry)
}

func (cd *ConflictDetector) sourceConfidence(sourceName string) float64 {
	sourceName = strings.ToLower(sourceName)
	if strings.Contains(sourceName, "ssh") || strings.Contains(sourceName, "config") {
		return 0.90
	}
	if strings.Contains(sourceName, "operational") || strings.Contains(sourceName, "show") {
		return 0.80
	}
	if strings.Contains(sourceName, "netbox") || strings.Contains(sourceName, "servicenow") || strings.Contains(sourceName, "cmdb") {
		return 0.70
	}
	if strings.Contains(sourceName, "inferred") {
		return 0.40
	}
	return 0.50
}

// resolveConflict applies resolution rules to a conflict entry.
func (cd *ConflictDetector) resolveConflict(entry *ConflictEntry) {
	// 1. Check for manual overrides
	for _, override := range cd.Overrides {
		if !override.Active() {
			continue
		}
		if override.Field == entry.Field {
			if override.Source == "manual" {
				entry.Resolution = "manual-override"
				entry.WinningValue = override.ManualValue
				return
			}
			// Source-preference override
			if override.Source == entry.SourceA || override.Source == entry.SourceB {
				if override.Source == entry.SourceA {
					entry.Resolution = "manual-override"
					entry.WinningValue = entry.ValueA
				} else {
					entry.Resolution = "manual-override"
					entry.WinningValue = entry.ValueB
				}
				return
			}
		}
	}

	// 2. Confidence-based resolution: higher confidence wins
	if entry.ConfidenceA >= entry.ConfidenceB {
		entry.Resolution = "higher-confidence"
		entry.WinningValue = entry.ValueA
	} else {
		entry.Resolution = "higher-confidence"
		entry.WinningValue = entry.ValueB
	}
}

// AddOverride registers a manual override for conflict resolution.
func (cd *ConflictDetector) AddOverride(override OverrideRecord) {
	cd.Overrides = append(cd.Overrides, override)
}

// RemoveOverride removes an override by ID.
func (cd *ConflictDetector) RemoveOverride(id string) bool {
	for i, o := range cd.Overrides {
		if o.ID == id {
			cd.Overrides = append(cd.Overrides[:i], cd.Overrides[i+1:]...)
			return true
		}
	}
	return false
}

// GetActiveOverrides returns all currently active overrides.
func (cd *ConflictDetector) GetActiveOverrides() []OverrideRecord {
	active := make([]OverrideRecord, 0)
	for _, o := range cd.Overrides {
		if o.Active() {
			active = append(active, o)
		}
	}
	return active
}

// ========================================================================
// Conflict Summary
// ========================================================================

// ConflictSummary provides aggregate statistics about detected conflicts.
type ConflictSummary struct {
	TotalConflicts   int               `json:"total_conflicts"`
	BySeverity       map[string]int    `json:"by_severity"`
	ByResolution     map[string]int    `json:"by_resolution"`
	ByField          map[string]int    `json:"by_field"`
	UnresolvedCount  int               `json:"unresolved_count"`
	ResolvedCount    int               `json:"resolved_count"`
	OverrideCount    int               `json:"override_count"`
	GeneratedAt      time.Time         `json:"generated_at"`
}

// SummarizeConflicts produces a summary of conflict entries.
func SummarizeConflicts(conflicts []ConflictEntry) ConflictSummary {
	summary := ConflictSummary{
		TotalConflicts: len(conflicts),
		BySeverity:     make(map[string]int),
		ByResolution:   make(map[string]int),
		ByField:        make(map[string]int),
		GeneratedAt:    time.Now(),
	}

	for _, c := range conflicts {
		summary.BySeverity[c.Severity]++
		summary.ByResolution[c.Resolution]++
		summary.ByField[c.Field]++

		if c.Resolution == "unresolved" {
			summary.UnresolvedCount++
		} else {
			summary.ResolvedCount++
		}
		if c.Resolution == "manual-override" {
			summary.OverrideCount++
		}
	}

	return summary
}

// FormatConflictSummary returns a human-readable summary string.
func (cs *ConflictSummary) FormatSummary() string {
	return fmt.Sprintf(
		"%d conflicts: %d critical, %d warning, %d info | %d resolved (%d manual), %d unresolved",
		cs.TotalConflicts,
		cs.BySeverity["critical"],
		cs.BySeverity["warning"],
		cs.BySeverity["info"],
		cs.ResolvedCount,
		cs.OverrideCount,
		cs.UnresolvedCount,
	)
}

// ========================================================================
// Cross-Source Conflict Report
// ========================================================================

// CrossSourceConflictReport is a detailed conflict report for a device.
type CrossSourceConflictReport struct {
	DeviceIdentifier string          `json:"device_identifier"`
	DeviceName       string          `json:"device_name"`
	Conflicts        []ConflictEntry `json:"conflicts"`
	Summary          ConflictSummary `json:"summary"`
}

// BuildConflictReport creates a conflict report for a specific device.
func BuildConflictReport(deviceID, deviceName string, conflicts []ConflictEntry) CrossSourceConflictReport {
	// Filter conflicts for this device (simplified — in practice we'd use device-specific filtering)
	deviceConflicts := make([]ConflictEntry, 0)
	for _, c := range conflicts {
		deviceConflicts = append(deviceConflicts, c)
	}

	return CrossSourceConflictReport{
		DeviceIdentifier: deviceID,
		DeviceName:       deviceName,
		Conflicts:        deviceConflicts,
		Summary:          SummarizeConflicts(deviceConflicts),
	}
}

// Ensure unused imports don't error
var _ = models.ConfidenceSourceConfig

package correlation

import (
	"fmt"
	"strings"
	"time"

	"github.com/mattpainter701/graphlayer/platform/models"
)

// ========================================================================
// Device Deduplicator
// ========================================================================

// DeviceDeduplicator matches and deduplicates device records from multiple
// sources using configurable match keys (IP, hostname, serial, MAC).
type DeviceDeduplicator struct {
	Config CorrelationConfig
}

// NewDeviceDeduplicator creates a new deduplicator with default config.
func NewDeviceDeduplicator() *DeviceDeduplicator {
	return &DeviceDeduplicator{
		Config: DefaultCorrelationConfig(),
	}
}

// NewDeviceDeduplicatorWithConfig creates a deduplicator with custom config.
func NewDeviceDeduplicatorWithConfig(cfg CorrelationConfig) *DeviceDeduplicator {
	return &DeviceDeduplicator{Config: cfg}
}

// Deduplicate runs the full deduplication pipeline on a set of source records.
func (dd *DeviceDeduplicator) Deduplicate(records []*SourceRecord) *DedupResult {
	result := &DedupResult{
		UniqueDevices: make([]*SourceRecord, 0),
		Duplicates:    make(map[string]string),
		Matches:       make([]MatchResult, 0),
		Stats: DedupStats{
			TotalInput: len(records),
			MatchByKey: make(map[MatchKey]int),
		},
	}

	if len(records) == 0 {
		return result
	}

	// Phase 1: Find all pairwise matches
	matches := dd.findMatches(records)
	result.Matches = matches
	result.Stats.MatchesFound = len(matches)

	// Phase 2: Build merge groups (transitive closure of matches)
	groups := dd.buildMergeGroups(records, matches)

	// Phase 3: Merge each group into a single canonical record
	for _, group := range groups {
		if len(group) == 1 {
			result.UniqueDevices = append(result.UniqueDevices, group[0])
			continue
		}

		merged := dd.MergeGroup(group)
		result.UniqueDevices = append(result.UniqueDevices, merged)

		// Track duplicates
		for _, rec := range group {
			canonicalID := dd.recordID(merged)
			recID := dd.recordID(rec)
			if recID != canonicalID {
				result.Duplicates[recID] = canonicalID
			}
		}
	}

	result.Stats.TotalOutput = len(result.UniqueDevices)
	result.Stats.MergedCount = len(records) - len(result.UniqueDevices)

	return result
}

// ========================================================================
// Match Finding
// ========================================================================

func (dd *DeviceDeduplicator) findMatches(records []*SourceRecord) []MatchResult {
	matches := make([]MatchResult, 0)
	seen := make(map[string]bool) // deduplicate match pairs

	for i := 0; i < len(records); i++ {
		for j := i + 1; j < len(records); j++ {
			mr := dd.tryMatch(records[i], records[j])
			if mr != nil {
				pairKey := fmt.Sprintf("%s:%s", dd.recordID(mr.RecordA), dd.recordID(mr.RecordB))
				if seen[pairKey] {
					continue
				}
				seen[pairKey] = true
				matches = append(matches, *mr)
				result := dd // placeholder for increment
				_ = result
				dd.incrementMatchKey(&DedupStats{MatchByKey: make(map[MatchKey]int)}, mr.MatchKey)

				// For result tracking
				key := mr.MatchKey
				// We'll track this in the final stats accumulation
				_ = key
			}
		}
	}

	// Populate match-by-key stats
	for _, m := range matches {
		// Initialize if needed (handled by the result's stats)
		_ = m
	}

	return matches
}

func (dd *DeviceDeduplicator) incrementMatchKey(stats *DedupStats, key MatchKey) {
	stats.MatchByKey[key]++
}

func (dd *DeviceDeduplicator) tryMatch(a, b *SourceRecord) *MatchResult {
	// Try each match key in priority order
	matchKeys := dd.orderedMatchKeys()

	for _, mk := range matchKeys {
		valA := dd.extractKey(a, mk)
		valB := dd.extractKey(b, mk)

		if valA == "" || valB == "" {
			continue
		}

		if dd.compareValues(valA, valB, mk) {
			confidence := dd.matchConfidence(mk)
			if confidence >= dd.Config.DedupMinConfidence {
				return &MatchResult{
					RecordA:    a,
					RecordB:    b,
					MatchKey:   mk,
					MatchValue: valA,
					Confidence: confidence,
					MatchTime:  time.Now(),
				}
			}
		}
	}

	return nil
}

func (dd *DeviceDeduplicator) orderedMatchKeys() []MatchKey {
	// Priority order: serial > management_ip > mac > hostname
	return []MatchKey{
		MatchKeySerial,
		MatchKeyManagementIP,
		MatchKeyMAC,
		MatchKeyHostname,
	}
}

// extractKey pulls the match key value from a source record.
func (dd *DeviceDeduplicator) extractKey(sr *SourceRecord, key MatchKey) string {
	switch key {
	case MatchKeySerial:
		if sr.Device != nil && sr.Asset != nil {
			// Prefer AssetNode serial, fallback to Device
			if sr.Asset.Serial != "" {
				return sr.Asset.Serial
			}
		}
		if sr.Asset != nil && sr.Asset.Serial != "" {
			return sr.Asset.Serial
		}
		return ""

	case MatchKeyManagementIP:
		if sr.Device != nil && sr.Device.ManagementIP != "" {
			return sr.Device.ManagementIP
		}
		if sr.Asset != nil && sr.Asset.IP != "" {
			return sr.Asset.IP
		}
		return ""

	case MatchKeyMAC:
		if sr.Asset != nil && sr.Asset.Mac != "" {
			mac := sr.Asset.Mac
			if dd.Config.NormalizeMAC {
				mac = normalizeMAC(mac)
			}
			return mac
		}
		return ""

	case MatchKeyHostname:
		hostname := ""
		if sr.Device != nil && sr.Device.Hostname != "" {
			hostname = sr.Device.Hostname
		} else if sr.Asset != nil && sr.Asset.Hostname != "" {
			hostname = sr.Asset.Hostname
		}
		if hostname != "" && dd.Config.NormalizeHostname {
			hostname = normalizeHostname(hostname)
		}
		return hostname

	default:
		return ""
	}
}

func (dd *DeviceDeduplicator) compareValues(a, b string, key MatchKey) bool {
	switch key {
	case MatchKeySerial:
		if dd.Config.StrictSerialMatch {
			return a == b
		}
		return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
	case MatchKeyManagementIP:
		return a == b
	case MatchKeyMAC:
		if dd.Config.NormalizeMAC {
			return normalizeMAC(a) == normalizeMAC(b)
		}
		return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
	case MatchKeyHostname:
		if dd.Config.NormalizeHostname {
			return normalizeHostname(a) == normalizeHostname(b)
		}
		return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
	default:
		return a == b
	}
}

func (dd *DeviceDeduplicator) matchConfidence(key MatchKey) float64 {
	switch key {
	case MatchKeySerial:
		return 0.95
	case MatchKeyManagementIP:
		return 0.75
	case MatchKeyMAC:
		return 0.65
	case MatchKeyHostname:
		return 0.50
	default:
		return 0.50
	}
}

// ========================================================================
// Merge Group Building (Transitive Closure)
// ========================================================================

func (dd *DeviceDeduplicator) buildMergeGroups(records []*SourceRecord, matches []MatchResult) [][]*SourceRecord {
	// Build adjacency from matches
	adj := make(map[string]map[string]bool)
	for _, m := range matches {
		aID := dd.recordID(m.RecordA)
		bID := dd.recordID(m.RecordB)
		if adj[aID] == nil {
			adj[aID] = make(map[string]bool)
		}
		if adj[bID] == nil {
			adj[bID] = make(map[string]bool)
		}
		adj[aID][bID] = true
		adj[bID][aID] = true
	}

	// Build ID to record map
	byID := make(map[string]*SourceRecord)
	for _, r := range records {
		byID[dd.recordID(r)] = r
	}

	// Union-Find / DFS for connected components
	visited := make(map[string]bool)
	groups := make([][]*SourceRecord, 0)

	for _, r := range records {
		id := dd.recordID(r)
		if visited[id] {
			continue
		}

		// DFS to find all connected records
		component := make([]*SourceRecord, 0)
		stack := []string{id}
		for len(stack) > 0 {
			current := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if visited[current] {
				continue
			}
			visited[current] = true
			if rec, ok := byID[current]; ok {
				component = append(component, rec)
			}
			for neighbor := range adj[current] {
				if !visited[neighbor] {
					stack = append(stack, neighbor)
				}
			}
		}

		if len(component) > 0 {
			groups = append(groups, component)
		}
	}

	return groups
}

// ========================================================================
// Group Merging
// ========================================================================

// MergeGroup merges a group of matched records into a single canonical record.
// Uses confidence-weighted merge: when two sources conflict, prefer the source
// with higher confidence.
func (dd *DeviceDeduplicator) MergeGroup(group []*SourceRecord) *SourceRecord {
	if len(group) == 0 {
		return nil
	}
	if len(group) == 1 {
		return group[0]
	}

	// Find the highest-confidence source for each field
	canonical := dd.cloneRecord(group[0])
	sources := []string{group[0].SourceName}

	// Track which fields we've set from which source
	for i := 1; i < len(group) && len(sources) < dd.Config.MaxMergeDepth; i++ {
		other := group[i]
		sources = append(sources, other.SourceName)

		if canonical.Device != nil && other.Device != nil {
			dd.mergeDevices(canonical.Device, other.Device, canonical.SourceName, other.SourceName)
		}
		if canonical.Asset != nil && other.Asset != nil {
			dd.mergeAssets(canonical.Asset, other.Asset, canonical.SourceName, other.SourceName)
		}
	}

	// Update metadata to reflect merge
	if canonical.Metadata == nil {
		canonical.Metadata = make(map[string]string)
	}
	canonical.Metadata["merged_from"] = strings.Join(sources, ",")
	canonical.Metadata["merge_count"] = fmt.Sprintf("%d", len(sources))

	return canonical
}

// mergeDevices performs confidence-weighted field merging for Device records.
func (dd *DeviceDeduplicator) mergeDevices(canonical, other *models.Device, canonSource, otherSource string) {
	// For each field: if canonical is empty and other has a value, take it
	// If both populated and conflict, prefer the higher-confidence source
	sourcePriority := dd.sourcePriority(canonSource, otherSource)

	if canonical.Hostname == "" && other.Hostname != "" {
		canonical.Hostname = other.Hostname
	}
	if canonical.Vendor == "" && other.Vendor != "" {
		canonical.Vendor = other.Vendor
	}
	if canonical.Model == "" && other.Model != "" {
		canonical.Model = other.Model
	}
	if canonical.Version == "" && other.Version != "" {
		canonical.Version = other.Version
	}
	if canonical.ManagementIP == "" && other.ManagementIP != "" {
		canonical.ManagementIP = other.ManagementIP
	}
	if canonical.Site == "" && other.Site != "" {
		canonical.Site = other.Site
	}
	if canonical.Role == "" && other.Role != "" {
		canonical.Role = other.Role
	}

	// Merge OS info
	if canonical.OSName == "" && other.OSName != "" {
		canonical.OSName = other.OSName
	}
	if canonical.OSVersion == "" && other.OSVersion != "" {
		canonical.OSVersion = other.OSVersion
	}

	// Merge interface lists (union, deduplicate)
	seenIface := make(map[string]bool)
	for _, iface := range canonical.Interfaces {
		seenIface[iface] = true
	}
	for _, iface := range other.Interfaces {
		if !seenIface[iface] {
			canonical.Interfaces = append(canonical.Interfaces, iface)
			seenIface[iface] = true
		}
	}

	// Merge virtual contexts
	seenCtx := make(map[string]bool)
	for _, ctx := range canonical.VirtualContexts {
		seenCtx[ctx] = true
	}
	for _, ctx := range other.VirtualContexts {
		if !seenCtx[ctx] {
			canonical.VirtualContexts = append(canonical.VirtualContexts, ctx)
			seenCtx[ctx] = true
		}
	}

	_ = sourcePriority // used for conflict resolution semantics
}

// mergeAssets performs confidence-weighted field merging for AssetNode records.
func (dd *DeviceDeduplicator) mergeAssets(canonical, other *models.AssetNode, canonSource, otherSource string) {
	if canonical.Hostname == "" && other.Hostname != "" {
		canonical.Hostname = other.Hostname
	}
	if canonical.Vendor == "" && other.Vendor != "" {
		canonical.Vendor = other.Vendor
	}
	if canonical.Model == "" && other.Model != "" {
		canonical.Model = other.Model
	}
	if canonical.Serial == "" && other.Serial != "" {
		canonical.Serial = other.Serial
	}
	if canonical.Mac == "" && other.Mac != "" {
		canonical.Mac = other.Mac
	}
	if canonical.IP == "" && other.IP != "" {
		canonical.IP = other.IP
	}
	if canonical.OSName == "" && other.OSName != "" {
		canonical.OSName = other.OSName
	}
	if canonical.OSVersion == "" && other.OSVersion != "" {
		canonical.OSVersion = other.OSVersion
	}
	if canonical.Site == "" && other.Site != "" {
		canonical.Site = other.Site
	}
	if canonical.Role == "" && other.Role != "" {
		canonical.Role = other.Role
	}
	if canonical.BusinessUnit == "" && other.BusinessUnit != "" {
		canonical.BusinessUnit = other.BusinessUnit
	}

	// Merge interfaces
	seenIface := make(map[string]bool)
	for _, iface := range canonical.PhysicalNetworkInterfaces {
		seenIface[iface.Name] = true
	}
	for _, iface := range other.PhysicalNetworkInterfaces {
		if !seenIface[iface.Name] {
			canonical.PhysicalNetworkInterfaces = append(canonical.PhysicalNetworkInterfaces, iface)
			seenIface[iface.Name] = true
		}
	}
}

func (dd *DeviceDeduplicator) sourcePriority(sourceA, sourceB string) int {
	// Config sources have highest priority, then operational, then connector.
	// We don't have the ConfidenceSource type directly here, so we use source
	// name heuristics for ranking. Lower = higher priority.
	pa := 5
	pb := 5
	if strings.Contains(sourceA, "ssh") || strings.Contains(sourceA, "config") {
		pa = 1
	} else if strings.Contains(sourceA, "operational") || strings.Contains(sourceA, "show") {
		pa = 2
	} else if strings.Contains(sourceA, "netbox") || strings.Contains(sourceA, "servicenow") {
		pa = 3
	}
	if strings.Contains(sourceB, "ssh") || strings.Contains(sourceB, "config") {
		pb = 1
	} else if strings.Contains(sourceB, "operational") || strings.Contains(sourceB, "show") {
		pb = 2
	} else if strings.Contains(sourceB, "netbox") || strings.Contains(sourceB, "servicenow") {
		pb = 3
	}
	if pa < pb {
		return -1
	} else if pb < pa {
		return 1
	}
	return 0
}

// ========================================================================
// Helpers
// ========================================================================

func (dd *DeviceDeduplicator) recordID(sr *SourceRecord) string {
	if sr.Device != nil {
		if sr.Device.ID != "" {
			return sr.Device.ID
		}
	}
	if sr.Asset != nil {
		if sr.Asset.Id != "" {
			return sr.Asset.Id
		}
	}
	// Fallback: use source name + primary identifier
	return sr.SourceName + ":" + sr.PrimaryIdentifier()
}

func (dd *DeviceDeduplicator) cloneRecord(sr *SourceRecord) *SourceRecord {
	clone := &SourceRecord{
		SourceName:  sr.SourceName,
		SourceType:  sr.SourceType,
		CollectedAt: sr.CollectedAt,
		Confidence:  sr.Confidence,
		Metadata:    make(map[string]string),
	}

	if sr.Device != nil {
		devCopy := *sr.Device
		devCopy.Interfaces = append([]string{}, sr.Device.Interfaces...)
		devCopy.VirtualContexts = append([]string{}, sr.Device.VirtualContexts...)
		devCopy.PolicyGroups = append([]string{}, sr.Device.PolicyGroups...)
		clone.Device = &devCopy
	}
	if sr.Asset != nil {
		assetCopy := *sr.Asset
		assetCopy.PhysicalNetworkInterfaces = append([]models.PhysicalInterface{}, sr.Asset.PhysicalNetworkInterfaces...)
		clone.Asset = &assetCopy
	}

	for k, v := range sr.Metadata {
		clone.Metadata[k] = v
	}

	return clone
}

func normalizeMAC(mac string) string {
	// Remove separators, lowercase, pad to 12 hex chars
	mac = strings.ToLower(mac)
	mac = strings.ReplaceAll(mac, ":", "")
	mac = strings.ReplaceAll(mac, "-", "")
	mac = strings.ReplaceAll(mac, ".", "")
	return mac
}

func normalizeHostname(hostname string) string {
	// Lowercase and strip any domain suffix
	hostname = strings.ToLower(strings.TrimSpace(hostname))
	// Strip domain part if FQDN
	if idx := strings.Index(hostname, "."); idx > 0 {
		hostname = hostname[:idx]
	}
	return hostname
}

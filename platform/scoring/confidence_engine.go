// Package scoring provides the confidence scoring v2 engine for GraphLayer.
// Sprint 9: Per-attribute confidence, propagation, corroboration, staleness decay,
// visualization hints, summary statistics, and min-confidence filtering.
package scoring

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/mattpainter701/graphlayer/platform/models"
)

// ========================================================================
// ConfidenceEngine — the core scoring engine
// ========================================================================

// ConfidenceEngine computes confidence scores for topology records.
// It handles per-attribute scoring, propagation, corroboration, and staleness.
type ConfidenceEngine struct {
	PropagationConfig   models.PropagationConfig
	CorroborationConfig models.CorroborationConfig
	StalenessConfig     models.StalenessConfig
}

// NewConfidenceEngine creates a new engine with default configs.
func NewConfidenceEngine() *ConfidenceEngine {
	return &ConfidenceEngine{
		PropagationConfig:   models.DefaultPropagationConfig(),
		CorroborationConfig: models.DefaultCorroborationConfig(),
		StalenessConfig:     models.DefaultStalenessConfig(),
	}
}

// NewConfidenceEngineWithConfig creates an engine with custom configs.
func NewConfidenceEngineWithConfig(
	prop models.PropagationConfig,
	corr models.CorroborationConfig,
	stale models.StalenessConfig,
) *ConfidenceEngine {
	return &ConfidenceEngine{
		PropagationConfig:   prop,
		CorroborationConfig: corr,
		StalenessConfig:     stale,
	}
}

// ========================================================================
// Per-Attribute Confidence Scoring
// ========================================================================

// ScoreAttribute computes a confidence score for a single attribute.
// source: where the evidence came from (config, operational, inferred, connector)
// evidenceQuality: 0.0–1.0 factor for the quality of this specific evidence
// Returns a score clamped to [0.0, 1.0].
func (ce *ConfidenceEngine) ScoreAttribute(source models.ConfidenceSource, evidenceQuality float64) float64 {
	baseWeight := models.SourceWeights[source]
	score := baseWeight * evidenceQuality
	return clamp(score)
}

// ScoreDeviceAttributes computes per-attribute confidence for a device record.
// It scores standard device fields based on evidence source and quality.
func (ce *ConfidenceEngine) ScoreDeviceAttributes(
	hostnameSource models.ConfidenceSource,
	vendorSource models.ConfidenceSource,
	modelSource models.ConfidenceSource,
	versionSource models.ConfidenceSource,
	mgmtIPSource models.ConfidenceSource,
	siteSource models.ConfidenceSource,
	roleSource models.ConfidenceSource,
) *models.ConfidenceProfile {
	profile := models.NewConfidenceProfile()

	// Hostname: config parsing provides high-quality hostname extraction
	profile.AddAttribute("hostname",
		ce.ScoreAttribute(hostnameSource, 1.0),
		hostnameSource,
		fmt.Sprintf("hostname extracted from %s", hostnameSource))

	// Vendor: config parsing is definitive, connectors/MAC OUI are medium-quality
	vendorQuality := 1.0
	if vendorSource == models.ConfidenceSourceConnector || vendorSource == models.ConfidenceSourceInferred {
		vendorQuality = 0.6 // MAC OUI lookup or connector inference is less definitive
	}
	profile.AddAttribute("vendor",
		ce.ScoreAttribute(vendorSource, vendorQuality),
		vendorSource,
		fmt.Sprintf("vendor identified from %s (quality=%.1f)", vendorSource, vendorQuality))

	// Model: config and connectors are good, inferred is weak
	modelQuality := 1.0
	if modelSource == models.ConfidenceSourceInferred {
		modelQuality = 0.3
	}
	profile.AddAttribute("model",
		ce.ScoreAttribute(modelSource, modelQuality),
		modelSource,
		fmt.Sprintf("model from %s", modelSource))

	// Version: config is best, operational is good
	profile.AddAttribute("version",
		ce.ScoreAttribute(versionSource, 0.9),
		versionSource,
		fmt.Sprintf("version from %s", versionSource))

	// Management IP: config and connectors are solid, inferred less so
	mgmtQuality := 1.0
	if mgmtIPSource == models.ConfidenceSourceInferred {
		mgmtQuality = 0.4
	}
	profile.AddAttribute("management_ip",
		ce.ScoreAttribute(mgmtIPSource, mgmtQuality),
		mgmtIPSource,
		fmt.Sprintf("management IP from %s", mgmtIPSource))

	// Site: connectors (NetBox) are best, config may have hints
	siteQuality := 1.0
	if siteSource == models.ConfidenceSourceConfig {
		siteQuality = 0.5 // SNMP location hints in config are unreliable
	}
	profile.AddAttribute("site",
		ce.ScoreAttribute(siteSource, siteQuality),
		siteSource,
		fmt.Sprintf("site from %s", siteSource))

	// Role: connectors are good, config-based role detection is medium
	roleQuality := 1.0
	if roleSource == models.ConfidenceSourceConfig || roleSource == models.ConfidenceSourceInferred {
		roleQuality = 0.5
	}
	profile.AddAttribute("role",
		ce.ScoreAttribute(roleSource, roleQuality),
		roleSource,
		fmt.Sprintf("role from %s", roleSource))

	profile.SetOverall()
	return profile
}

// ScoreInterfaceAttributes computes per-attribute confidence for an interface record.
func (ce *ConfidenceEngine) ScoreInterfaceAttributes(
	nameSource models.ConfidenceSource,
	ipSource models.ConfidenceSource,
	vlanSource models.ConfidenceSource,
	statusSource models.ConfidenceSource,
) *models.ConfidenceProfile {
	profile := models.NewConfidenceProfile()

	profile.AddAttribute("name",
		ce.ScoreAttribute(nameSource, 1.0),
		nameSource,
		fmt.Sprintf("interface name from %s", nameSource))

	ipQuality := 1.0
	if ipSource == models.ConfidenceSourceInferred {
		ipQuality = 0.3
	}
	profile.AddAttribute("ip_address",
		ce.ScoreAttribute(ipSource, ipQuality),
		ipSource,
		fmt.Sprintf("IP address from %s", ipSource))

	vlanQuality := 1.0
	if vlanSource == models.ConfidenceSourceInferred {
		vlanQuality = 0.5
	}
	profile.AddAttribute("vlan_id",
		ce.ScoreAttribute(vlanSource, vlanQuality),
		vlanSource,
		fmt.Sprintf("VLAN from %s", vlanSource))

	profile.AddAttribute("status",
		ce.ScoreAttribute(statusSource, 0.9),
		statusSource,
		fmt.Sprintf("status from %s", statusSource))

	profile.SetOverall()
	return profile
}

// ScoreLinkAttributes computes per-attribute confidence for a link record.
func (ce *ConfidenceEngine) ScoreLinkAttributes(
	fromSource models.ConfidenceSource,
	toSource models.ConfidenceSource,
	linkTypeSource models.ConfidenceSource,
) *models.ConfidenceProfile {
	profile := models.NewConfidenceProfile()

	profile.AddAttribute("from_device",
		ce.ScoreAttribute(fromSource, 1.0),
		fromSource,
		fmt.Sprintf("from endpoint from %s", fromSource))

	profile.AddAttribute("to_device",
		ce.ScoreAttribute(toSource, 1.0),
		toSource,
		fmt.Sprintf("to endpoint from %s", toSource))

	linkQuality := 1.0
	if linkTypeSource == models.ConfidenceSourceInferred {
		linkQuality = 0.4
	}
	profile.AddAttribute("link_type",
		ce.ScoreAttribute(linkTypeSource, linkQuality),
		linkTypeSource,
		fmt.Sprintf("link type from %s", linkTypeSource))

	profile.SetOverall()
	return profile
}

// ========================================================================
// Confidence Propagation
// ========================================================================

// PropagateConfidence propagates confidence from a source profile to a derived profile.
// Used when a device's interfaces inherit device confidence with decay.
//
// sourceProfile: the parent entity's confidence (e.g., a Device)
// hopCount: number of propagation hops (0 = direct, 1 = one hop away, etc.)
// derivedFields: the fields on the derived entity to score
// derivedSource: how the derived entity's evidence was obtained
//
// Returns a new profile for the derived entity.
func (ce *ConfidenceEngine) PropagateConfidence(
	sourceProfile *models.ConfidenceProfile,
	hopCount int,
	derivedFields []string,
	derivedSource models.ConfidenceSource,
) *models.ConfidenceProfile {
	// Check hop limit
	if ce.PropagationConfig.MaxHops > 0 && hopCount > ce.PropagationConfig.MaxHops {
		profile := models.NewConfidenceProfile()
		profile.Overall = 0.0
		profile.Tier = models.ConfidenceTierNone
		return profile
	}

	// Compute decayed overall score
	decayFactor := math.Pow(ce.PropagationConfig.DecayFactor, float64(hopCount))
	propagatedOverall := sourceProfile.Overall * decayFactor

	profile := models.NewConfidenceProfile()

	// Score each derived field using the propagated confidence as a base,
	// weighted against the derived source's own weight
	for _, field := range derivedFields {
		derivedBaseWeight := models.SourceWeights[derivedSource]
		// Blend: 60% propagated, 40% derived source weight
		fieldScore := math.Min(propagatedOverall, (propagatedOverall*0.6)+(derivedBaseWeight*0.4))
		fieldScore = clamp(fieldScore)

		profile.AddAttribute(field,
			fieldScore,
			derivedSource,
			fmt.Sprintf("propagated from parent (hop=%d, decay=%.3f) + %s evidence",
				hopCount, decayFactor, derivedSource))
	}

	profile.SetOverall()
	return profile
}

// PropagateDeviceToInterfaces propagates device confidence to its interfaces.
func (ce *ConfidenceEngine) PropagateDeviceToInterfaces(
	deviceProfile *models.ConfidenceProfile,
	interfaceFields []string,
) *models.ConfidenceProfile {
	// Interfaces are hop=1 from device (direct relationship)
	return ce.PropagateConfidence(deviceProfile, 1, interfaceFields, models.ConfidenceSourceConfig)
}

// PropagateInterfaceToLink propagates interface confidence to a link.
func (ce *ConfidenceEngine) PropagateInterfaceToLink(
	ifaceProfile *models.ConfidenceProfile,
	linkFields []string,
) *models.ConfidenceProfile {
	// Links are hop=2 from device (device -> interface -> link)
	return ce.PropagateConfidence(ifaceProfile, 1, linkFields, models.ConfidenceSourceConfig)
}

// ========================================================================
// Corroboration Multiplier
// ========================================================================

// ApplyCorroboration boosts confidence when multiple independent sources agree.
//
// evidence: list of corroborating evidence items for a single record
// baseProfile: the profile before corroboration
//
// Returns the boosted profile.
func (ce *ConfidenceEngine) ApplyCorroboration(
	evidence []models.CorroborationEvidence,
	baseProfile *models.ConfidenceProfile,
) *models.ConfidenceProfile {
	// Only sources that agree on the same field and value corroborate one
	// another. Counting unrelated facts from different sources would inflate
	// confidence without supporting the same claim.
	agreement := make(map[string]map[string]map[string]bool)
	for _, ev := range evidence {
		if ev.Field == "" || ev.Value == "" || ev.SourceName == "" {
			continue
		}
		key := ev.Field + "\x00" + ev.Value
		if agreement[key] == nil {
			agreement[key] = make(map[string]map[string]bool)
		}
		identity := ev.SourceName
		if ce.CorroborationConfig.RequireIndependentSources {
			identity = string(ev.SourceType)
		}
		if agreement[key][identity] == nil {
			agreement[key][identity] = make(map[string]bool)
		}
		agreement[key][identity][ev.SourceName] = true
	}
	corroboratingSources := 0
	for _, sourceGroups := range agreement {
		count := len(sourceGroups)
		if count > corroboratingSources {
			corroboratingSources = count
		}
	}

	boost := 0.0
	if corroboratingSources >= 2 {
		// The evidence supplied here defines the current contribution. Replacing
		// the old contribution makes repeated calls idempotent and lets removed
		// or conflicting evidence clear a previously earned boost.
		boost = ce.CorroborationConfig.BoostPerSource * float64(corroboratingSources-1)
		boost = math.Min(boost, ce.CorroborationConfig.MaxBoost)
		// A profile already aged by staleness has its corroboration contribution
		// reduced by the same remaining-confidence factor as its attributes.
		boost *= clamp(1 - baseProfile.StalenessDecay)
	}

	// Create a new profile with the boost applied
	boosted := models.NewConfidenceProfile()
	boosted.PerAttribute = make(models.PerAttributeConfidence)

	// Copy per-attribute scores
	for k, v := range baseProfile.PerAttribute {
		boosted.PerAttribute[k] = v
	}
	boosted.Attributes = append(boosted.Attributes, baseProfile.Attributes...)
	boosted.EvidenceSources = append(boosted.EvidenceSources, baseProfile.EvidenceSources...)
	boosted.EvidenceCount = baseProfile.EvidenceCount
	if corroboratingSources < 2 {
		corroboratingSources = 0
	}
	boosted.CorroboratingSourceCount = corroboratingSources
	boosted.CorroborationBoost = boost
	boosted.StalenessDecay = baseProfile.StalenessDecay

	boosted.SetOverall()
	return boosted
}

// CorroborateAcrossSources checks agreement across multiple connectors/parsers
// and applies corroboration boosts where they agree.
//
// records is a map of source_name -> confidence_profile
// Returns a map of source_name -> boosted profile.
func (ce *ConfidenceEngine) CorroborateAcrossSources(
	records map[string]*models.ConfidenceProfile,
) map[string]*models.ConfidenceProfile {
	result := make(map[string]*models.ConfidenceProfile, len(records))

	for name, profile := range records {
		// Profiles contain confidence scores but no observed values. Shared field
		// names therefore cannot establish agreement; callers with values should
		// use ApplyCorroboration with explicit value-bearing evidence.
		result[name] = profile
	}

	return result
}

// ========================================================================
// Staleness Decay
// ========================================================================

// ApplyStalenessDecay reduces confidence for records that haven't been refreshed.
//
// profile: the confidence profile to decay
// lastRefreshed: when the record's data was last updated
// now: the current time (usually time.Now())
//
// Returns a new profile with staleness decay applied.
func (ce *ConfidenceEngine) ApplyStalenessDecay(
	profile *models.ConfidenceProfile,
	lastRefreshed time.Time,
	now time.Time,
) *models.ConfidenceProfile {
	ageHours := now.Sub(lastRefreshed).Hours()
	ageDays := ageHours / 24.0

	var decayFactor float64
	if ageDays <= 0 {
		decayFactor = 1
	} else {
		switch ce.StalenessConfig.DecayModel {
		case "linear":
			decayFactor = ce.computeLinearDecay(ageDays)
		default: // exponential
			decayFactor = ce.computeExponentialDecay(ageDays)
		}
	}

	// Clamp decay to max age
	if ageDays >= ce.StalenessConfig.MaxAgeDays {
		decayFactor = 0.0
	}

	previousFactor := clamp(1 - profile.StalenessDecay)
	adjustmentFactor := 0.0
	if previousFactor > 0 {
		adjustmentFactor = decayFactor / previousFactor
	} else if decayFactor == 0 {
		adjustmentFactor = 1
	}
	decayed := models.NewConfidenceProfile()
	decayed.PerAttribute = make(models.PerAttributeConfidence)
	for k, v := range profile.PerAttribute {
		decayed.PerAttribute[k] = clamp(v * adjustmentFactor)
	}
	decayed.Attributes = append(decayed.Attributes, profile.Attributes...)
	for i := range decayed.Attributes {
		if score, ok := decayed.PerAttribute[decayed.Attributes[i].Field]; ok {
			decayed.Attributes[i].Score = score
		}
	}
	decayed.EvidenceSources = append(decayed.EvidenceSources, profile.EvidenceSources...)
	decayed.EvidenceCount = profile.EvidenceCount
	decayed.CorroborationBoost = profile.CorroborationBoost * adjustmentFactor
	decayed.CorroboratingSourceCount = profile.CorroboratingSourceCount
	decayed.StalenessDecay = clamp(1.0 - decayFactor)
	decayed.LastUpdated = now

	decayed.SetOverall()
	return decayed
}

// computeExponentialDecay uses half-life decay: score * 0.5^(age/halfLife)
func (ce *ConfidenceEngine) computeExponentialDecay(ageDays float64) float64 {
	if ce.StalenessConfig.HalfLifeDays <= 0 {
		return 1.0
	}
	return math.Pow(0.5, ageDays/ce.StalenessConfig.HalfLifeDays)
}

// computeLinearDecay uses linear interpolation from 1.0 at age=0 to 0.0 at maxAge.
func (ce *ConfidenceEngine) computeLinearDecay(ageDays float64) float64 {
	if ce.StalenessConfig.MaxAgeDays <= 0 {
		return 1.0
	}
	ratio := ageDays / ce.StalenessConfig.MaxAgeDays
	return clamp(1.0 - ratio)
}

// ========================================================================
// Summary Statistics
// ========================================================================

// ComputeSummaryStats computes aggregate confidence statistics for a set of profiles.
func (ce *ConfidenceEngine) ComputeSummaryStats(profiles []*models.ConfidenceProfile) *models.ConfidenceSummaryStats {
	stats := models.NewConfidenceSummaryStats()
	stats.TotalRecords = len(profiles)

	if len(profiles) == 0 {
		return stats
	}

	// Collect scores for statistical computation
	scores := make([]float64, len(profiles))
	var corroborationSum, stalenessSum float64
	var evidenceCountSum float64
	corroborationCount := 0
	staleCount := 0

	for i, profile := range profiles {
		scores[i] = profile.Overall
		stats.RecordsByTier[profile.Tier]++

		if profile.CorroborationBoost > 0 {
			corroborationSum += profile.CorroborationBoost
			corroborationCount++
		}
		if profile.StalenessDecay > 0 {
			staleCount++
		}
		stalenessSum += profile.StalenessDecay
		evidenceCountSum += float64(profile.EvidenceCount)
	}

	// Sort for median
	sort.Float64s(scores)

	stats.AverageConfidence = sum(scores) / float64(len(scores))
	stats.MedianConfidence = scores[len(scores)/2]
	stats.MinConfidence = scores[0]
	stats.MaxConfidence = scores[len(scores)-1]

	if corroborationCount > 0 {
		stats.AverageCorroboration = corroborationSum / float64(corroborationCount)
	}
	stats.RecordsWithCorroboration = corroborationCount

	if len(profiles) > 0 {
		stats.AverageStalenessDecay = stalenessSum / float64(len(profiles))
	}
	stats.RecordsStale = staleCount
	stats.AverageEvidenceCount = evidenceCountSum / float64(len(profiles))

	return stats
}

// ========================================================================
// Min-Confidence Filter
// ========================================================================

// FilterByConfidence filters a slice of profiles by the given filter criteria.
// Returns only profiles that pass all filter conditions.
func (ce *ConfidenceEngine) FilterByConfidence(
	profiles []*models.ConfidenceProfile,
	filter models.ConfidenceFilter,
) []*models.ConfidenceProfile {
	var result []*models.ConfidenceProfile

	for _, profile := range profiles {
		if !ce.passesFilter(profile, filter) {
			continue
		}
		result = append(result, profile)
	}

	return result
}

// passesFilter checks if a single profile meets all filter criteria.
func (ce *ConfidenceEngine) passesFilter(profile *models.ConfidenceProfile, filter models.ConfidenceFilter) bool {
	// MinConfidence threshold
	if profile.Overall < filter.MinConfidence {
		return false
	}

	// MinTier threshold (if set)
	if filter.MinTier != "" {
		minScore := models.MinTierScore(filter.MinTier)
		if profile.Overall < minScore {
			return false
		}
	}

	// Exclude specific sources
	if len(filter.ExcludeSources) > 0 {
		for _, excluded := range filter.ExcludeSources {
			for _, source := range profile.EvidenceSources {
				if source == excluded {
					return false
				}
			}
		}
	}

	// Require minimum corroboration
	if filter.RequireCorroboration > 0 {
		if profile.CorroboratingSourceCount < filter.RequireCorroboration {
			return false
		}
	}

	// Max staleness
	if filter.MaxStaleness > 0 {
		if profile.StalenessDecay > filter.MaxStaleness {
			return false
		}
	}

	return true
}

// ========================================================================
// Visualization
// ========================================================================

// GetVisualizationHint computes a visualization hint for a confidence profile.
func (ce *ConfidenceEngine) GetVisualizationHint(profile *models.ConfidenceProfile, isLink bool) models.VisualizationHint {
	return models.ComputeVisualizationHint(profile.Overall, isLink)
}

// ========================================================================
// Bulk Operations
// ========================================================================

// ScoreDeviceProfile is a convenience method that scores a device and returns
// a profile suitable for attaching to a models.Device or similar record.
func (ce *ConfidenceEngine) ScoreDeviceProfile(
	hostname, vendor, model, version, mgmtIP, site, role string,
	source models.ConfidenceSource,
) *models.ConfidenceProfile {
	return ce.ScoreDeviceAttributes(source, source, source, source, source, source, source)
}

// ========================================================================
// Helpers
// ========================================================================

func clamp(v float64) float64 {
	if v < 0.0 {
		return 0.0
	}
	if v > 1.0 {
		return 1.0
	}
	return v
}

func sum(vals []float64) float64 {
	var s float64
	for _, v := range vals {
		s += v
	}
	return s
}

// PerAttributeConfidence is a type alias for convenience.
type PerAttributeConfidence = models.PerAttributeConfidence

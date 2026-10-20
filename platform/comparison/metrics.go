// Package comparison
// Sprint 10: Comparison metrics — device count/accuracy, link accuracy,
// vendor coverage breadth, evidence quality, false positive rate.
package comparison

import (
	"fmt"
	"strings"
)

// ====================================================================
// Comparison Metrics
// ====================================================================

// ComparisonMetrics holds all quantified comparison results between
// GraphLayer and reference topologies.
type ComparisonMetrics struct {
	// Device metrics
	DeviceMetrics DeviceMetrics `json:"device_metrics"`

	// Link metrics
	LinkMetrics LinkMetrics `json:"link_metrics"`

	// Vendor coverage
	VendorCoverage VendorCoverage `json:"vendor_coverage"`

	// Evidence quality
	EvidenceQuality EvidenceQuality `json:"evidence_quality"`

	// False positive analysis
	FalsePositives FalsePositiveAnalysis `json:"false_positives"`

	// Overall scores
	OverallScore float64 `json:"overall_score"`

	// Per-vendor breakdown
	PerVendor map[string]VendorBreakdown `json:"per_vendor"`
}

// DeviceMetrics tracks device discovery accuracy.
type DeviceMetrics struct {
	// GraphLayer device count
	GraphLayerCount int `json:"graphlayer_count"`

	// reference device count
	ReferenceCount int `json:"reference_count"`

	// MatchedDevices is the number of devices found by both tools.
	MatchedDevices int `json:"matched_devices"`

	// ExtraDevices is devices found by GraphLayer but NOT by reference.
	ExtraDevices int `json:"extra_devices"`

	// MissingDevices is devices found by reference but NOT by GraphLayer.
	MissingDevices int `json:"missing_devices"`

	// DeviceAccuracy is matched / total_unique_devices.
	// total_unique = matched + extra + missing.
	DeviceAccuracy float64 `json:"device_accuracy"`

	// DeviceRecall is matched / reference_total.
	DeviceRecall float64 `json:"device_recall"`

	// DevicePrecision is matched / graphlayer_total.
	DevicePrecision float64 `json:"device_precision"`

	// F1Score is the harmonic mean of precision and recall.
	F1Score float64 `json:"f1_score"`

	// MatchedDeviceIDs lists the IDs of matched devices.
	MatchedDeviceIDs []string `json:"matched_device_ids,omitempty"`

	// ExtraDeviceIDs lists devices only in GraphLayer.
	ExtraDeviceIDs []string `json:"extra_device_ids,omitempty"`

	// MissingDeviceIDs lists devices only in reference.
	MissingDeviceIDs []string `json:"missing_device_ids,omitempty"`
}

// LinkMetrics tracks link discovery accuracy.
type LinkMetrics struct {
	// GraphLayer link count
	GraphLayerCount int `json:"graphlayer_count"`

	// reference link count
	ReferenceCount int `json:"reference_count"`

	// MatchedLinks is links found by both tools.
	MatchedLinks int `json:"matched_links"`

	// ExtraLinks is links found by GraphLayer but NOT by reference.
	ExtraLinks int `json:"extra_links"`

	// MissingLinks is links found by reference but NOT by GraphLayer.
	MissingLinks int `json:"missing_links"`

	// LinkAccuracy is matched / total_unique_links.
	LinkAccuracy float64 `json:"link_accuracy"`

	// LinkRecall is matched / reference_total.
	LinkRecall float64 `json:"link_recall"`

	// LinkPrecision is matched / graphlayer_total.
	LinkPrecision float64 `json:"link_precision"`

	// F1Score is the harmonic mean of precision and recall.
	F1Score float64 `json:"f1_score"`

	// L2LinksMatched counts L2-specific links that matched.
	L2LinksMatched int `json:"l2_links_matched"`

	// L3LinksMatched counts L3-specific links that matched.
	L3LinksMatched int `json:"l3_links_matched"`
}

// VendorCoverage tracks how many vendor families each tool covers.
type VendorCoverage struct {
	// GraphLayerVendors lists unique vendor names in GraphLayer output.
	GraphLayerVendors []string `json:"graphlayer_vendors"`

	// ReferenceVendors lists unique vendor names in reference output.
	ReferenceVendors []string `json:"reference_vendors"`

	// GraphLayerVendorCount is the number of unique vendors in GraphLayer.
	GraphLayerVendorCount int `json:"graphlayer_vendor_count"`

	// ReferenceVendorCount is the number of unique vendors in reference.
	ReferenceVendorCount int `json:"reference_vendor_count"`

	// ExtraVendors is vendors covered by GraphLayer but NOT reference.
	ExtraVendors []string `json:"extra_vendors"`

	// MissingVendors is vendors covered by reference but NOT GraphLayer.
	MissingVendors []string `json:"missing_vendors"`

	// VendorBreadthScore is graphlayer_vendor_count / max_vendor_count.
	VendorBreadthScore float64 `json:"vendor_breadth_score"`
}

// EvidenceQuality measures provenance and data richness.
type EvidenceQuality struct {
	// GraphLayerAvgEvidence is the mean evidence count per device.
	GraphLayerAvgEvidence float64 `json:"graphlayer_avg_evidence"`

	// ReferenceAvgEvidence is the mean evidence count per device.
	ReferenceAvgEvidence float64 `json:"reference_avg_evidence"`

	// GraphLayerRecordsWithEvidence is count of devices with at least one source.
	GraphLayerRecordsWithEvidence int `json:"graphlayer_records_with_evidence"`

	// ReferenceRecordsWithEvidence is count of devices with at least one source.
	ReferenceRecordsWithEvidence int `json:"reference_records_with_evidence"`

	// GraphLayerEvidenceRatio is records_with_evidence / total_devices.
	GraphLayerEvidenceRatio float64 `json:"graphlayer_evidence_ratio"`

	// ReferenceEvidenceRatio is records_with_evidence / total_devices.
	ReferenceEvidenceRatio float64 `json:"reference_evidence_ratio"`

	// GraphLayerAvgConfidence is the mean confidence across all devices.
	GraphLayerAvgConfidence float64 `json:"graphlayer_avg_confidence"`

	// ReferenceAvgConfidence is the mean confidence across all devices.
	ReferenceAvgConfidence float64 `json:"reference_avg_confidence"`

	// EvidenceQualityScore is graphlayer_ratio / max(1, reference_ratio).
	EvidenceQualityScore float64 `json:"evidence_quality_score"`
}

// FalsePositiveAnalysis tracks fabricated or incorrect devices/links.
type FalsePositiveAnalysis struct {
	// GraphLayerFalsePositiveDevices is devices in GraphLayer that are
	// NOT validated by reference and have low evidence.
	GraphLayerFalsePositiveDevices int `json:"graphlayer_false_positive_devices"`

	// ReferenceFalsePositiveDevices is devices in reference NOT validated by GraphLayer.
	ReferenceFalsePositiveDevices int `json:"reference_false_positive_devices"`

	// GraphLayerFalsePositiveLinks is links in GraphLayer NOT validated by reference.
	GraphLayerFalsePositiveLinks int `json:"graphlayer_false_positive_links"`

	// ReferenceFalsePositiveLinks is links in reference NOT validated by GraphLayer.
	ReferenceFalsePositiveLinks int `json:"reference_false_positive_links"`

	// GraphLayerFPRate is false_positive_devices / total_graphlayer_devices.
	GraphLayerFPRate float64 `json:"graphlayer_fp_rate"`

	// ReferenceFPRate is false_positive_devices / total_reference_devices.
	ReferenceFPRate float64 `json:"reference_fp_rate"`

	// GraphLayerLowConfidenceUnmatched is unmatched GraphLayer devices with < 0.3 confidence.
	GraphLayerLowConfidenceUnmatched int `json:"graphlayer_low_confidence_unmatched"`

	// ReferenceLowConfidenceUnmatched is unmatched reference devices with < 0.3 confidence.
	ReferenceLowConfidenceUnmatched int `json:"reference_low_confidence_unmatched"`
}

// VendorBreakdown provides per-vendor comparison details.
type VendorBreakdown struct {
	Vendor string `json:"vendor"`

	// Device counts
	GraphLayerDevices int `json:"graphlayer_devices"`
	ReferenceDevices     int `json:"reference_devices"`
	MatchedDevices    int `json:"matched_devices"`

	// Link counts
	GraphLayerLinks int `json:"graphlayer_links"`
	ReferenceLinks     int `json:"reference_links"`
	MatchedLinks    int `json:"matched_links"`

	// Accuracy scores
	DeviceAccuracy float64 `json:"device_accuracy"`
	LinkAccuracy   float64 `json:"link_accuracy"`

	// Evidence
	AvgEvidence float64 `json:"avg_evidence"`
	AvgConf     float64 `json:"avg_confidence"`
}

// ====================================================================
// Comparison Engine
// ====================================================================

// CompareTopologies runs the full comparison between GraphLayer and reference exports.
func CompareTopologies(graphLayer, reference *ReferenceExportTopology) *ComparisonMetrics {
	m := &ComparisonMetrics{
		PerVendor: make(map[string]VendorBreakdown),
	}

	// Device comparison
	m.DeviceMetrics = compareDevices(graphLayer.Devices, reference.Devices)

	// Link comparison
	m.LinkMetrics = compareLinks(graphLayer.Links, reference.Links)

	// Vendor coverage
	m.VendorCoverage = compareVendors(graphLayer.Devices, reference.Devices)

	// Evidence quality
	m.EvidenceQuality = compareEvidence(graphLayer.Devices, reference.Devices)

	// False positives
	m.FalsePositives = analyzeFalsePositives(
		graphLayer.Devices, reference.Devices,
		graphLayer.Links, reference.Links,
		m.DeviceMetrics, m.LinkMetrics,
	)

	// Per-vendor breakdown
	m.PerVendor = computeVendorBreakdowns(graphLayer.Devices, reference.Devices,
		graphLayer.Links, reference.Links)

	// Overall score (weighted average of all metrics)
	m.OverallScore = computeOverallScore(m)

	return m
}

// ====================================================================
// Device Comparison
// ====================================================================

func compareDevices(gl, reference []ReferenceDevice) DeviceMetrics {
	dm := DeviceMetrics{
		GraphLayerCount: len(gl),
		ReferenceCount:     len(reference),
	}

	// Build index sets
	glIndex := deviceIndex(gl)
	referenceIndex := deviceIndex(reference)

	// Match devices: try exact ID match first, then fuzzy
	matched := make([]string, 0)
	extra := make([]string, 0)
	missing := make([]string, 0)

	for _, d := range gl {
		if _, ok := referenceIndex[d.ID]; ok {
			matched = append(matched, d.ID)
		} else if fuzzyMatch(d, reference) != "" {
			matched = append(matched, d.ID)
		} else {
			extra = append(extra, d.ID)
		}
	}

	for _, d := range reference {
		if _, ok := glIndex[d.ID]; !ok && fuzzyMatch(d, gl) == "" {
			missing = append(missing, d.ID)
		}
	}

	dm.MatchedDevices = len(matched)
	dm.ExtraDevices = len(extra)
	dm.MissingDevices = len(missing)
	dm.MatchedDeviceIDs = matched
	dm.ExtraDeviceIDs = extra
	dm.MissingDeviceIDs = missing

	// Calculate accuracy scores
	totalUnique := dm.MatchedDevices + dm.ExtraDevices + dm.MissingDevices
	if totalUnique > 0 {
		dm.DeviceAccuracy = float64(dm.MatchedDevices) / float64(totalUnique)
	}
	if dm.ReferenceCount > 0 {
		dm.DeviceRecall = float64(dm.MatchedDevices) / float64(dm.ReferenceCount)
	}
	if dm.GraphLayerCount > 0 {
		dm.DevicePrecision = float64(dm.MatchedDevices) / float64(dm.GraphLayerCount)
	}
	if dm.DevicePrecision+dm.DeviceRecall > 0 {
		dm.F1Score = 2 * dm.DevicePrecision * dm.DeviceRecall / (dm.DevicePrecision + dm.DeviceRecall)
	}

	return dm
}

// fuzzyMatch tries to match a device when exact ID fails.
// Falls back to ManagementIP then Hostname.
func fuzzyMatch(d ReferenceDevice, candidates []ReferenceDevice) string {
	if d.ManagementIP != "" {
		for _, c := range candidates {
			if c.ManagementIP == d.ManagementIP && c.ID != d.ID {
				return c.ID
			}
		}
	}
	if d.Hostname != "" {
		for _, c := range candidates {
			if strings.EqualFold(c.Hostname, d.Hostname) && c.ID != d.ID {
				return c.ID
			}
		}
	}
	return ""
}

// ====================================================================
// Link Comparison
// ====================================================================

func compareLinks(gl, reference []ReferenceLink) LinkMetrics {
	lm := LinkMetrics{
		GraphLayerCount: len(gl),
		ReferenceCount:     len(reference),
	}

	referenceIndex := linkIndex(reference)
	glIDs := linkIDSet(gl)

	matched := make(map[string]bool)
	for _, l := range gl {
		if _, ok := referenceIndex[l.ID]; ok {
			matched[l.ID] = true
		} else if linkFuzzyMatch(l, reference) {
			matched[l.ID] = true
		}
	}

	// Count matched in reference too
	for _, l := range reference {
		if _, ok := linkFuzzyMatchSet(l, gl); ok {
			// already counted
		}
	}

	lm.MatchedLinks = len(matched)
	lm.ExtraLinks = len(glIDs) - len(matched)

	referenceMatched := 0
	for _, l := range reference {
		if _, ok := matched[l.ID]; ok || linkFuzzyMatch(l, gl) {
			referenceMatched++
		}
	}
	lm.MissingLinks = len(reference) - referenceMatched

	// Count L2/L3 breakdown
	for _, l := range gl {
		if l.LinkType == "l2" {
			if _, ok := matched[l.ID]; ok {
				lm.L2LinksMatched++
			}
		} else {
			if _, ok := matched[l.ID]; ok {
				lm.L3LinksMatched++
			}
		}
	}

	totalUnique := lm.MatchedLinks + lm.ExtraLinks + lm.MissingLinks
	if totalUnique > 0 {
		lm.LinkAccuracy = float64(lm.MatchedLinks) / float64(totalUnique)
	}
	if lm.ReferenceCount > 0 {
		lm.LinkRecall = float64(lm.MatchedLinks) / float64(lm.ReferenceCount)
	}
	if lm.GraphLayerCount > 0 {
		lm.LinkPrecision = float64(lm.MatchedLinks) / float64(lm.GraphLayerCount)
	}
	if lm.LinkPrecision+lm.LinkRecall > 0 {
		lm.F1Score = 2 * lm.LinkPrecision * lm.LinkRecall / (lm.LinkPrecision + lm.LinkRecall)
	}

	return lm
}

func linkFuzzyMatch(l ReferenceLink, candidates []ReferenceLink) bool {
	for _, c := range candidates {
		if l.FromDevice == c.FromDevice && l.ToDevice == c.ToDevice {
			if portsMatch(l.FromPort, c.FromPort) || portsMatch(l.ToPort, c.ToPort) {
				return true
			}
		}
		// Also try swapped direction
		if l.FromDevice == c.ToDevice && l.ToDevice == c.FromDevice {
			if portsMatch(l.FromPort, c.ToPort) || portsMatch(l.ToPort, c.FromPort) {
				return true
			}
		}
	}
	return false
}

func linkFuzzyMatchSet(l ReferenceLink, candidates []ReferenceLink) (string, bool) {
	for _, c := range candidates {
		if l.FromDevice == c.FromDevice && l.ToDevice == c.ToDevice {
			if portsMatch(l.FromPort, c.FromPort) || portsMatch(l.ToPort, c.ToPort) {
				return c.ID, true
			}
		}
		if l.FromDevice == c.ToDevice && l.ToDevice == c.FromDevice {
			if portsMatch(l.FromPort, c.ToPort) || portsMatch(l.ToPort, c.FromPort) {
				return c.ID, true
			}
		}
	}
	return "", false
}

// portsMatch checks whether two port/interface names refer to the same port,
// handling common naming conventions across vendors (Cisco, Arista, Juniper, etc.).
// Examples:
//   "Gi1/0/1" matches "GigabitEthernet1/0/1"
//   "Et1" matches "Ethernet1"
//   "Te1/1" matches "TenGigabitEthernet1/1"
//   "ge-0/0/0" matches "ge-0/0/0" (Juniper-style)
func portsMatch(a, b string) bool {
	if a == b {
		return true
	}
	na := normalizePort(a)
	nb := normalizePort(b)
	return na == nb
}

// normalizePort strips vendor prefix variations to a canonical form.
func normalizePort(port string) string {
	// Common Cisco-style abbreviations
	replacements := map[string]string{
		"gigabitethernet": "gi",
		"tengigabitethernet": "te",
		"fastethernet":       "fa",
		"ethernet":           "et",
		"fortygigabitethernet": "fo",
		"hundredgigabitethernet": "hu",
		"port-channel":       "po",
		"loopback":           "lo",
		"vlan":               "vl",
		"tunnel":             "tu",
	}
	lower := strings.ToLower(port)
	for long, short := range replacements {
		if strings.HasPrefix(lower, long) {
			return short + lower[len(long):]
		}
	}
	return lower
}

// ====================================================================
// Vendor Coverage
// ====================================================================

func compareVendors(glDevices, referenceDevices []ReferenceDevice) VendorCoverage {
	vc := VendorCoverage{}
	glVendors := make(map[string]bool)
	referenceVendors := make(map[string]bool)

	for _, d := range glDevices {
		if d.Vendor != "" {
			glVendors[d.Vendor] = true
		}
	}
	for _, d := range referenceDevices {
		if d.Vendor != "" {
			referenceVendors[d.Vendor] = true
		}
	}

	vc.GraphLayerVendors = sortedKeys(glVendors)
	vc.GraphLayerVendorCount = len(glVendors)
	vc.ReferenceVendors = sortedKeys(referenceVendors)
	vc.ReferenceVendorCount = len(referenceVendors)

	// Extra vendors
	for v := range glVendors {
		if !referenceVendors[v] {
			vc.ExtraVendors = append(vc.ExtraVendors, v)
		}
	}

	// Missing vendors
	for v := range referenceVendors {
		if !glVendors[v] {
			vc.MissingVendors = append(vc.MissingVendors, v)
		}
	}

	maxVendors := vc.GraphLayerVendorCount
	if vc.ReferenceVendorCount > maxVendors {
		maxVendors = vc.ReferenceVendorCount
	}
	if maxVendors > 0 {
		vc.VendorBreadthScore = float64(vc.GraphLayerVendorCount) / float64(maxVendors)
	}

	return vc
}

// ====================================================================
// Evidence Quality
// ====================================================================

func compareEvidence(glDevices, referenceDevices []ReferenceDevice) EvidenceQuality {
	eq := EvidenceQuality{}

	if len(glDevices) > 0 {
		var sumEvidence, sumConf float64
		withEvidence := 0
		for _, d := range glDevices {
			sumEvidence += float64(d.EvidenceCount)
			sumConf += d.Confidence
			if d.EvidenceCount > 0 {
				withEvidence++
			}
		}
		eq.GraphLayerAvgEvidence = sumEvidence / float64(len(glDevices))
		eq.GraphLayerRecordsWithEvidence = withEvidence
		eq.GraphLayerEvidenceRatio = float64(withEvidence) / float64(len(glDevices))
		eq.GraphLayerAvgConfidence = sumConf / float64(len(glDevices))
	}

	if len(referenceDevices) > 0 {
		var sumEvidence, sumConf float64
		withEvidence := 0
		for _, d := range referenceDevices {
			sumEvidence += float64(d.EvidenceCount)
			sumConf += d.Confidence
			if d.EvidenceCount > 0 {
				withEvidence++
			}
		}
		eq.ReferenceAvgEvidence = sumEvidence / float64(len(referenceDevices))
		eq.ReferenceRecordsWithEvidence = withEvidence
		eq.ReferenceEvidenceRatio = float64(withEvidence) / float64(len(referenceDevices))
		eq.ReferenceAvgConfidence = sumConf / float64(len(referenceDevices))
	}

	denom := eq.ReferenceEvidenceRatio
	if denom == 0 {
		denom = 1.0
	}
	eq.EvidenceQualityScore = eq.GraphLayerEvidenceRatio / denom

	return eq
}

// ====================================================================
// False Positive Analysis
// ====================================================================

func analyzeFalsePositives(
	glDevices, referenceDevices []ReferenceDevice,
	glLinks, referenceLinks []ReferenceLink,
	dm DeviceMetrics, lm LinkMetrics,
) FalsePositiveAnalysis {
	fp := FalsePositiveAnalysis{}

	// Devices: unmatched GraphLayer devices = potential false positives
	fp.GraphLayerFalsePositiveDevices = dm.ExtraDevices
	fp.ReferenceFalsePositiveDevices = dm.MissingDevices

	// Links: unmatched links = potential false positives
	fp.GraphLayerFalsePositiveLinks = lm.ExtraLinks
	fp.ReferenceFalsePositiveLinks = lm.MissingLinks

	// FP rates
	if len(glDevices) > 0 {
		fp.GraphLayerFPRate = float64(fp.GraphLayerFalsePositiveDevices) / float64(len(glDevices))
	}
	if len(referenceDevices) > 0 {
		fp.ReferenceFPRate = float64(fp.ReferenceFalsePositiveDevices) / float64(len(referenceDevices))
	}

	// Low-confidence unmatched
	for _, d := range glDevices {
		if d.Confidence < 0.3 {
			found := false
			for _, id := range dm.MatchedDeviceIDs {
				if d.ID == id {
					found = true
					break
				}
			}
			if !found {
				fp.GraphLayerLowConfidenceUnmatched++
			}
		}
	}
	for _, d := range referenceDevices {
		if d.Confidence < 0.3 {
			found := false
			for _, id := range dm.MatchedDeviceIDs {
				if d.ID == id {
					found = true
					break
				}
			}
			if !found {
				fp.ReferenceLowConfidenceUnmatched++
			}
		}
	}

	return fp
}

// ====================================================================
// Per-Vendor Breakdown
// ====================================================================

func computeVendorBreakdowns(
	glDevices, referenceDevices []ReferenceDevice,
	glLinks, referenceLinks []ReferenceLink,
) map[string]VendorBreakdown {
	breakdowns := make(map[string]VendorBreakdown)
	allVendors := make(map[string]bool)

	// Index devices by vendor
	glByVendor := make(map[string][]ReferenceDevice)
	referenceByVendor := make(map[string][]ReferenceDevice)

	for _, d := range glDevices {
		v := d.Vendor
		if v == "" {
			v = "unknown"
		}
		allVendors[v] = true
		glByVendor[v] = append(glByVendor[v], d)
	}
	for _, d := range referenceDevices {
		v := d.Vendor
		if v == "" {
			v = "unknown"
		}
		allVendors[v] = true
		referenceByVendor[v] = append(referenceByVendor[v], d)
	}

	for vendor := range allVendors {
		vb := VendorBreakdown{Vendor: vendor}
		glDevs := glByVendor[vendor]
		referenceDevs := referenceByVendor[vendor]

		vb.GraphLayerDevices = len(glDevs)
		vb.ReferenceDevices = len(referenceDevs)

		// Match devices for this vendor
		referenceIndex := deviceIndex(referenceDevs)
		matched := 0
		for _, d := range glDevs {
			if _, ok := referenceIndex[d.ID]; ok || fuzzyMatch(d, referenceDevs) != "" {
				matched++
			}
		}
		vb.MatchedDevices = matched

		// Accuracy
		total := vb.GraphLayerDevices + vb.ReferenceDevices - matched
		if total > 0 {
			vb.DeviceAccuracy = float64(matched) / float64(total)
		}

		// Evidence averages
		if len(glDevs) > 0 {
			var sumEv, sumConf float64
			for _, d := range glDevs {
				sumEv += float64(d.EvidenceCount)
				sumConf += d.Confidence
			}
			vb.AvgEvidence = sumEv / float64(len(glDevs))
			vb.AvgConf = sumConf / float64(len(glDevs))
		}

		// Links for this vendor (approximate)
		glDevSet := make(map[string]bool)
		for _, d := range glDevs {
			glDevSet[d.ID] = true
		}
		for _, l := range glLinks {
			if glDevSet[l.FromDevice] || glDevSet[l.ToDevice] {
				vb.GraphLayerLinks++
			}
		}
		referenceDevSet := make(map[string]bool)
		for _, d := range referenceDevs {
			referenceDevSet[d.ID] = true
		}
		for _, l := range referenceLinks {
			if referenceDevSet[l.FromDevice] || referenceDevSet[l.ToDevice] {
				vb.ReferenceLinks++
			}
		}

		breakdowns[vendor] = vb
	}

	return breakdowns
}

// ====================================================================
// Overall Score
// ====================================================================

// computeOverallScore calculates a weighted composite score (0.0–1.0).
// Weights: devices 30%, links 25%, vendors 15%, evidence 20%, FP-rate 10%.
func computeOverallScore(m *ComparisonMetrics) float64 {
	weights := map[string]float64{
		"devices":  0.30,
		"links":    0.25,
		"vendors":  0.15,
		"evidence": 0.20,
		"fp":       0.10,
	}

	score := 0.0
	score += m.DeviceMetrics.F1Score * weights["devices"]
	score += m.LinkMetrics.F1Score * weights["links"]
	score += m.VendorCoverage.VendorBreadthScore * weights["vendors"]
	score += m.EvidenceQuality.EvidenceQualityScore * weights["evidence"]

	// FP score: 1.0 = no false positives, 0.0 = all false positives
	fpScore := 1.0 - m.FalsePositives.GraphLayerFPRate
	if fpScore < 0 {
		fpScore = 0
	}
	score += fpScore * weights["fp"]

	return score
}

// ====================================================================
// Index helpers
// ====================================================================

func deviceIndex(devices []ReferenceDevice) map[string]ReferenceDevice {
	m := make(map[string]ReferenceDevice, len(devices))
	for _, d := range devices {
		m[d.ID] = d
	}
	return m
}

func deviceIDSet(devices []ReferenceDevice) map[string]bool {
	m := make(map[string]bool, len(devices))
	for _, d := range devices {
		m[d.ID] = true
	}
	return m
}

func linkIndex(links []ReferenceLink) map[string]ReferenceLink {
	m := make(map[string]ReferenceLink, len(links))
	for _, l := range links {
		m[l.ID] = l
	}
	return m
}

func linkIDSet(links []ReferenceLink) map[string]bool {
	m := make(map[string]bool, len(links))
	for _, l := range links {
		m[l.ID] = true
	}
	return m
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// Simple insertion sort for small sets
	for i := 1; i < len(keys); i++ {
		j := i
		for j > 0 && keys[j] < keys[j-1] {
			keys[j], keys[j-1] = keys[j-1], keys[j]
			j--
		}
	}
	return keys
}

// FormatPct formats a float as a percentage string.
func FormatPct(v float64) string {
	return fmt.Sprintf("%.1f%%", v*100)
}

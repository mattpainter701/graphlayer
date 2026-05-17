package intelligence

import (
	"fmt"
	"strings"
	"time"

	"github.com/mattpainter701/graphlayer/platform/models"
)

// ========================================================================
// L2InferenceEngine — infers L2 topology from MAC + LLDP/CDP data
// ========================================================================

// L2InferenceEngine infers Layer-2 links from operational data:
//   - LLDP/CDP neighbor tables → direct L2 links (highest confidence)
//   - MAC address table correlation → inferred L2 links (medium confidence)
type L2InferenceEngine struct {
	ctx *InferenceContext
}

// NewL2InferenceEngine creates a new L2 inference engine.
func NewL2InferenceEngine(ctx *InferenceContext) *L2InferenceEngine {
	return &L2InferenceEngine{ctx: ctx}
}

// ========================================================================
// LLDP/CDP → L2 Link Inference
// ========================================================================

// InferFromLLDP creates L2 links from LLDP/CDP neighbor data.
// This is the highest-confidence L2 inference method.
func (eng *L2InferenceEngine) InferFromLLDP() []InferredLink {
	var links []InferredLink

	for device, neighbors := range eng.ctx.LLDPNeighbors {
		for _, n := range neighbors {
			// Build link key for deduplication
			link := InferredLink{
				ID:              fmt.Sprintf("l2-lldp-%s-%s-%s", device, n.LocalInterface, n.RemoteSystemName),
				FromDevice:      device,
				FromPort:        n.LocalInterface,
				ToDevice:        n.RemoteSystemName,
				ToPort:          n.RemoteInterface,
				LinkType:        "l2",
				Protocol:        n.Protocol,
				InferenceMethod: "lldp-match",
				Confidence:      0.95, // Very high confidence from LLDP
				EvidenceSource:  models.ConfidenceSourceInferred,
				EvidenceDetail: fmt.Sprintf("LLDP adjacency: %s:%s ↔ %s:%s (chassis %s)",
					device, n.LocalInterface, n.RemoteSystemName, n.RemoteInterface, n.RemoteChassisID),
				SupportingData: map[string]interface{}{
					"remote_chassis_id": n.RemoteChassisID,
					"remote_mgmt_ip":    n.RemoteMgmtIP,
					"capabilities":      n.Capabilities,
				},
				InferredAt: time.Now(),
			}

			links = append(links, link)
		}
	}

	return links
}

// ========================================================================
// MAC Table → L2 Link Inference
// ========================================================================

// InferFromMACTable creates inferred L2 links from MAC address table correlation.
// If two switches see the same MAC on ports facing each other, they're likely linked.
func (eng *L2InferenceEngine) InferFromMACTable() []InferredLink {
	var links []InferredLink

	if len(eng.ctx.FDBEntries) == 0 {
		return links
	}

	// Group FDB entries by device
	deviceFDBs := make(map[string][]models.FDBEntry)
	for _, fdb := range eng.ctx.FDBEntries {
		deviceFDBs[fdb.DeviceID] = append(deviceFDBs[fdb.DeviceID], fdb)
	}

	// Cross-correlate: for each pair of devices, check if they have MAC overlap
	// suggesting a possible link
	devices := make([]string, 0, len(deviceFDBs))
	for d := range deviceFDBs {
		devices = append(devices, d)
	}

	for i := 0; i < len(devices); i++ {
		for j := i + 1; j < len(devices); j++ {
			devA := devices[i]
			devB := devices[j]

			overlapCount, evidence := eng.computeMACOverlap(deviceFDBs[devA], deviceFDBs[devB])
			if overlapCount >= 3 { // At least 3 overlapping MACs to suggest a link
				link := InferredLink{
					ID:              fmt.Sprintf("l2-mac-%s-%s", devA, devB),
					FromDevice:      devA,
					ToDevice:        devB,
					LinkType:        "inferred-l2",
					InferenceMethod: "mac-correlation",
					Confidence:      0.55, // Medium confidence from MAC correlation
					EvidenceSource:  models.ConfidenceSourceInferred,
					EvidenceDetail: fmt.Sprintf("MAC table correlation: %d overlapping MAC entries between %s and %s",
						overlapCount, devA, devB),
					SupportingData: map[string]interface{}{
						"overlapping_macs": overlapCount,
						"sample_macs":      evidence,
					},
					InferredAt: time.Now(),
				}
				links = append(links, link)
			}
		}
	}

	return links
}

// computeMACOverlap counts MAC addresses shared between two devices' FDB tables.
// Returns the count and up to 5 sample MACs as evidence.
func (eng *L2InferenceEngine) computeMACOverlap(fdbA, fdbB []models.FDBEntry) (int, []string) {
	macsA := make(map[string]bool)
	for _, fdb := range fdbA {
		macsA[strings.ToUpper(fdb.MAC)] = true
	}

	var samples []string
	count := 0
	for _, fdb := range fdbB {
		mac := strings.ToUpper(fdb.MAC)
		if macsA[mac] {
			count++
			if len(samples) < 5 {
				samples = append(samples, mac)
			}
		}
	}

	return count, samples
}

// ========================================================================
// Combined L2 Inference
// ========================================================================

// InferL2Links runs all L2 inference methods and returns deduplicated results.
func (eng *L2InferenceEngine) InferL2Links() []InferredLink {
	var allLinks []InferredLink

	allLinks = append(allLinks, eng.InferFromLLDP()...)
	allLinks = append(allLinks, eng.InferFromMACTable()...)

	return deduplicateLinks(allLinks)
}

// ========================================================================
// Helper: Deduplicate links
// ========================================================================

func deduplicateLinks(links []InferredLink) []InferredLink {
	seen := make(map[string]bool)
	var result []InferredLink

	for _, link := range links {
		// Normalize key: sort from/to
		key := link.FromDevice + "|" + link.ToDevice
		altKey := link.ToDevice + "|" + link.FromDevice
		if seen[key] || seen[altKey] {
			continue
		}
		seen[key] = true
		seen[altKey] = true
		result = append(result, link)
	}

	return result
}

func deduplicateSubnetGroups(groups []SubnetGroup) []SubnetGroup {
	seen := make(map[string]bool)
	var result []SubnetGroup

	for _, g := range groups {
		if seen[g.Subnet] {
			// Merge devices into existing group
			for i := range result {
				if result[i].Subnet == g.Subnet {
					result[i].Devices = appendUnique(result[i].Devices, g.Devices...)
					result[i].Interfaces = append(result[i].Interfaces, g.Interfaces...)
					break
				}
			}
			continue
		}
		seen[g.Subnet] = true
		result = append(result, g)
	}

	return result
}

func appendUnique(slice []string, items ...string) []string {
	seen := make(map[string]bool)
	for _, s := range slice {
		seen[s] = true
	}
	for _, item := range items {
		if !seen[item] {
			slice = append(slice, item)
			seen[item] = true
		}
	}
	return slice
}

// ========================================================================
// Conflict detection for links
// ========================================================================

func detectLinkConflicts(links []InferredLink) []Conflict {
	var conflicts []Conflict

	// Group by device pair
	pairs := make(map[string][]InferredLink)
	for _, l := range links {
		key := l.FromDevice + "|" + l.ToDevice
		altKey := l.ToDevice + "|" + l.FromDevice
		if existing, ok := pairs[key]; ok {
			pairs[key] = append(existing, l)
		} else if existing, ok := pairs[altKey]; ok {
			pairs[altKey] = append(existing, l)
		} else {
			pairs[key] = []InferredLink{l}
		}
	}

	// Detect conflicts: multiple links for same pair with different link types
	for _, pairLinks := range pairs {
		if len(pairLinks) < 2 {
			continue
		}

		// Check if link types differ
		types := make(map[string]bool)
		for _, l := range pairLinks {
			types[l.LinkType] = true
		}

		if len(types) > 1 {
			// Conflict: same device pair with different link types
			for i := 0; i < len(pairLinks); i++ {
				for j := i + 1; j < len(pairLinks); j++ {
					conflict := Conflict{
						ConflictType: "link",
						ClaimA: ConflictClaim{
							RuleName: "l2-inference",
							Priority: PriorityHigh,
							Value:    pairLinks[i],
						},
						ClaimB: ConflictClaim{
							RuleName: "l3-inference",
							Priority: PriorityNormal,
							Value:    pairLinks[j],
						},
						Resolution:       "prefer-a",
						ResolutionReason: "LLDP/CDP-derived L2 links take priority over routing-inferred L3 links",
					}
					conflicts = append(conflicts, conflict)
				}
			}
		}
	}

	return conflicts
}

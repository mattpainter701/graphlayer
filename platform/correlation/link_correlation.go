package correlation

import (
	"strings"
	"time"

	"github.com/mattpainter701/graphlayer/platform/models"
)

// ========================================================================
// Link Correlator — matches links across different sources by neighbor info
// ========================================================================

// LinkCorrelator correlates topology links from multiple sources.
type LinkCorrelator struct {
	Config CorrelationConfig
}

// NewLinkCorrelator creates a new link correlator with default config.
func NewLinkCorrelator() *LinkCorrelator {
	return &LinkCorrelator{
		Config: DefaultCorrelationConfig(),
	}
}

// NewLinkCorrelatorWithConfig creates a link correlator with custom config.
func NewLinkCorrelatorWithConfig(cfg CorrelationConfig) *LinkCorrelator {
	return &LinkCorrelator{Config: cfg}
}

// CorrelateLinks processes links from multiple sources and groups related links.
func (lc *LinkCorrelator) CorrelateLinks(sourceLinks []SourceLink) *LinkCorrelationResult {
	result := &LinkCorrelationResult{
		CorrelatedLinks: make([]CorrelatedLink, 0),
		UnmatchedLinks:  make([]models.Link, 0),
		Stats: LinkCorrelationStats{
			TotalInput: len(sourceLinks),
		},
	}

	if len(sourceLinks) == 0 {
		return result
	}

	// Phase 1: Extract all links into flat list
	allLinks := make([]models.Link, 0)
	linkSources := make(map[int]string) // index -> source name
	idx := 0
	for _, sl := range sourceLinks {
		for _, link := range sl.Links {
			allLinks = append(allLinks, link)
			linkSources[idx] = sl.SourceName
			idx++
		}
	}

	if len(allLinks) == 0 {
		return result
	}

	// Phase 2: Group links by endpoint pairs
	groups := lc.groupByEndpoints(allLinks, linkSources)

	// Phase 3: Build correlated and unmatched results
	for _, group := range groups {
		if len(group.Links) == 1 {
			result.UnmatchedLinks = append(result.UnmatchedLinks, group.Links[0])
		} else {
			canonical := lc.buildCanonicalLink(group.Links)
			result.CorrelatedLinks = append(result.CorrelatedLinks, CorrelatedLink{
				Canonical:       canonical,
				Sources:         group.Links,
				SourceNames:     group.SourceNames,
				MatchConfidence: lc.computeLinkMatchConfidence(group.Links),
			})
		}
	}

	result.Stats.Correlated = len(allLinks) - len(result.UnmatchedLinks)
	result.Stats.Unmatched = len(result.UnmatchedLinks)
	result.Stats.CorrelatedSets = len(result.CorrelatedLinks)

	return result
}

// SourceLink bundles links from a single source with its name.
type SourceLink struct {
	SourceName string
	Links      []models.Link
}

// linkGroup holds links that match on endpoint pairs.
type linkGroup struct {
	Links       []models.Link
	SourceNames []string
}

// groupByEndpoints groups links by (from_device, to_device) pairs.
func (lc *LinkCorrelator) groupByEndpoints(links []models.Link, sources map[int]string) []linkGroup {
	// Build endpoint key -> indices map
	groups := make(map[string][]int)

	for i, link := range links {
		// Normalize endpoint pair
		key := lc.endpointKey(link)
		if key == "" {
			continue
		}
		groups[key] = append(groups[key], i)
	}

	result := make([]linkGroup, 0)
	for _, indices := range groups {
		lg := linkGroup{
			Links:       make([]models.Link, 0),
			SourceNames: make([]string, 0),
		}
		seenSources := make(map[string]bool)
		for _, idx := range indices {
			lg.Links = append(lg.Links, links[idx])
			src := sources[idx]
			if !seenSources[src] {
				lg.SourceNames = append(lg.SourceNames, src)
				seenSources[src] = true
			}
		}
		result = append(result, lg)
	}

	return result
}

// endpointKey produces a normalized, ordered key for a link's endpoints.
func (lc *LinkCorrelator) endpointKey(link models.Link) string {
	// Try to identify endpoints by device IDs first, then names/IPs
	from := link.FromID
	to := link.ToID

	if from == "" {
		from = link.Source
	}
	if to == "" {
		to = link.Destination
	}

	// If still empty, try interface neighbor info
	if from == "" && link.Interface.Name != "" {
		from = link.Interface.Name
	}
	if to == "" && link.DestInterface.Name != "" {
		to = link.DestInterface.Name
	}

	if from == "" || to == "" {
		return ""
	}

	// Order lexicographically so A->B and B->A produce the same key
	if from > to {
		from, to = to, from
	}

	// Normalize
	from = strings.ToLower(strings.TrimSpace(from))
	to = strings.ToLower(strings.TrimSpace(to))

	return from + "|||" + to
}

// buildCanonicalLink selects or merges links into a canonical representation.
func (lc *LinkCorrelator) buildCanonicalLink(links []models.Link) models.Link {
	if len(links) == 0 {
		return models.Link{}
	}
	if len(links) == 1 {
		return links[0]
	}

	// Use the link with the most complete information as canonical
	canonical := links[0]
	bestScore := linkCompletenessScore(canonical)

	for i := 1; i < len(links); i++ {
		score := linkCompletenessScore(links[i])
		if score > bestScore {
			canonical = links[i]
			bestScore = score
		}
	}

	return canonical
}

// linkCompletenessScore rates a link by how many fields are populated.
func linkCompletenessScore(link models.Link) int {
	score := 0
	if link.FromID != "" {
		score++
	}
	if link.ToID != "" {
		score++
	}
	if link.Source != "" {
		score++
	}
	if link.Destination != "" {
		score++
	}
	if link.Interface.Name != "" {
		score++
	}
	if link.DestInterface.Name != "" {
		score++
	}
	if link.Protocol != "" {
		score++
	}
	if link.Port != "" {
		score++
	}
	if len(link.SourcePorts) > 0 {
		score++
	}
	if len(link.DestinationPorts) > 0 {
		score++
	}
	return score
}

// computeLinkMatchConfidence calculates confidence for a link match set.
func (lc *LinkCorrelator) computeLinkMatchConfidence(links []models.Link) float64 {
	if len(links) <= 1 {
		return 0.0
	}

	// Base confidence increases with corroborating sources
	baseConfidence := 0.5 + float64(len(links)-1)*0.15
	if baseConfidence > 1.0 {
		baseConfidence = 1.0
	}

	// Penalize if interfaces don't match across sources
	interfaceMatch := lc.checkInterfaceConsistency(links)
	if !interfaceMatch {
		baseConfidence -= 0.2
	}

	if baseConfidence < 0.0 {
		baseConfidence = 0.0
	}

	return baseConfidence
}

// checkInterfaceConsistency verifies that all links in a set reference
// consistent interfaces.
func (lc *LinkCorrelator) checkInterfaceConsistency(links []models.Link) bool {
	if len(links) < 2 {
		return true
	}

	// Check source interface consistency
	srcInterfaces := make(map[string]bool)
	for _, link := range links {
		if link.Interface.Name != "" {
			srcInterfaces[strings.ToLower(strings.TrimSpace(link.Interface.Name))] = true
		}
	}
	// If we have multiple distinct interface names, inconsistent
	if len(srcInterfaces) > 1 {
		return false
	}

	// Check dest interface consistency
	dstInterfaces := make(map[string]bool)
	for _, link := range links {
		if link.DestInterface.Name != "" {
			dstInterfaces[strings.ToLower(strings.TrimSpace(link.DestInterface.Name))] = true
		}
	}
	if len(dstInterfaces) > 1 {
		return false
	}

	return true
}

// ========================================================================
// L2 Link Correlation
// ========================================================================

// CorrelateL2Links correlates L2 adjacency links from multiple sources.
func (lc *LinkCorrelator) CorrelateL2Links(sourceLinks []SourceL2Link) *LinkCorrelationResult {
	result := &LinkCorrelationResult{
		CorrelatedLinks: make([]CorrelatedLink, 0),
		UnmatchedLinks:  make([]models.Link, 0),
		Stats: LinkCorrelationStats{
			TotalInput: 0,
		},
	}

	// Convert L2Links to generic Links for grouping
	allLinks := make([]models.Link, 0)
	linkSources := make(map[int]string)
	idx := 0
	for _, sl := range sourceLinks {
		for _, l2 := range sl.L2Links {
			link := models.Link{
				Id:     l2.ID,
				FromID: l2.FromDevice,
				ToID:   l2.ToDevice,
				Source: l2.FromDevice,
				Destination: l2.ToDevice,
				Interface: models.PhysicalInterface{
					Name: l2.FromPort,
				},
				DestInterface: models.PhysicalInterface{
					Name: l2.ToPort,
				},
				EdgeName: l2.LinkType,
			}
			allLinks = append(allLinks, link)
			linkSources[idx] = sl.SourceName
			idx++
		}
		result.Stats.TotalInput++
	}

	if len(allLinks) == 0 {
		return result
	}

	groups := lc.groupByEndpoints(allLinks, linkSources)

	for _, group := range groups {
		if len(group.Links) == 1 {
			result.UnmatchedLinks = append(result.UnmatchedLinks, group.Links[0])
		} else {
			canonical := lc.buildCanonicalLink(group.Links)
			result.CorrelatedLinks = append(result.CorrelatedLinks, CorrelatedLink{
				Canonical:       canonical,
				Sources:         group.Links,
				SourceNames:     group.SourceNames,
				MatchConfidence: lc.computeLinkMatchConfidence(group.Links),
			})
		}
	}

	result.Stats.Correlated = len(allLinks) - len(result.UnmatchedLinks)
	result.Stats.Unmatched = len(result.UnmatchedLinks)
	result.Stats.CorrelatedSets = len(result.CorrelatedLinks)

	return result
}

// SourceL2Link bundles L2 links from a single source.
type SourceL2Link struct {
	SourceName string
	L2Links    []models.L2Link
}

// ========================================================================
// Interface Correlation
// ========================================================================

// CorrelateInterfaces matches interface records across different sources.
func (lc *LinkCorrelator) CorrelateInterfaces(sourceInterfaces []SourceInterface) []CorrelatedInterface {
	correlated := make([]CorrelatedInterface, 0)

	// Group by device + interface name
	groups := make(map[string][]sourceIface)
	for _, si := range sourceInterfaces {
		for _, iface := range si.Interfaces {
			key := strings.ToLower(si.DeviceID + ":" + iface.Name)
			groups[key] = append(groups[key], sourceIface{
				sourceName: si.SourceName,
				iface:      iface,
			})
		}
	}

	for _, items := range groups {
		if len(items) < 2 {
			continue
		}
		ci := CorrelatedInterface{
			Interface:  items[0].iface,
			SourceNames: make([]string, 0),
		}
		for _, item := range items {
			ci.SourceNames = append(ci.SourceNames, item.sourceName)
		}
		correlated = append(correlated, ci)
	}

	return correlated
}

// SourceInterface bundles interfaces from a single source.
type SourceInterface struct {
	SourceName string
	DeviceID   string
	Interfaces []models.Interface
}

// CorrelatedInterface represents an interface matched across sources.
type CorrelatedInterface struct {
	Interface   models.Interface `json:"interface"`
	SourceNames []string         `json:"source_names"`
}

type sourceIface struct {
	sourceName string
	iface      models.Interface
}

// Ensure time import used
var _ = time.Now

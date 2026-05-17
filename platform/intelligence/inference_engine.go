package intelligence

import (
	"sort"
	"sync"
	"time"

	"github.com/mattpainter701/graphlayer/platform/models"
)

const engineVersion = "inference-v1.0.0"

// ========================================================================
// InferenceEngine — the main graph intelligence engine
// ========================================================================

// InferenceEngine orchestrates all topology inference operations.
// It runs inference rules in priority order, resolves conflicts,
// and produces a complete InferenceResult.
type InferenceEngine struct {
	// Configurable rules (defaults from DefaultInferenceRules)
	Rules []InferenceRule

	// Input data context
	Context *InferenceContext

	// Inference components
	l2Infer     *L2InferenceEngine
	l3Infer     *L3InferenceEngine
	subnetDisc  *SubnetDiscovery
	implicitLink *ImplicitLinkDetector
	boundaryDet *BoundaryDetector
	ruleEngine  *RuleEngineView // reference to rule engine for conflict resolution
}

// NewInferenceEngine creates a new inference engine with default rules.
func NewInferenceEngine(ctx *InferenceContext) *InferenceEngine {
	return &InferenceEngine{
		Rules:   DefaultInferenceRules(),
		Context: ctx,
	}
}

// NewInferenceEngineWithRules creates an engine with custom rules.
func NewInferenceEngineWithRules(ctx *InferenceContext, rules []InferenceRule) *InferenceEngine {
	return &InferenceEngine{
		Rules:   rules,
		Context: ctx,
	}
}

// ========================================================================
// Main Inference Pipeline
// ========================================================================

// Run executes all enabled inference rules in priority order and returns results.
func (eng *InferenceEngine) Run() *InferenceResult {
	startTime := time.Now()
	result := &InferenceResult{
		GeneratedAt:   startTime,
		EngineVersion: engineVersion,
	}

	// Sort rules by priority (highest first)
	sorted := make([]InferenceRule, len(eng.Rules))
	copy(sorted, eng.Rules)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Priority > sorted[j].Priority
	})

	// Initialize sub-engines
	eng.l2Infer = NewL2InferenceEngine(eng.Context)
	eng.l3Infer = NewL3InferenceEngine(eng.Context)
	eng.subnetDisc = NewSubnetDiscovery(eng.Context)
	eng.implicitLink = NewImplicitLinkDetector(eng.Context)
	eng.boundaryDet = NewBoundaryDetector(eng.Context)

	// Execute rules in priority order
	for _, rule := range sorted {
		if !rule.Enabled {
			continue
		}

		ruleStart := time.Now()
		ruleResult := eng.executeRule(rule)
		ruleResult.DurationMs = time.Since(ruleStart).Milliseconds()
		result.RuleResults = append(result.RuleResults, ruleResult)

		// Collect outputs
		eng.collectRuleOutputs(result, ruleResult)
	}

	// Run conflict resolution
	eng.resolveConflicts(result)

	// Compute statistics
	eng.computeStats(result)
	result.Stats.TotalDurationMs = time.Since(startTime).Milliseconds()

	return result
}

// executeRule runs a single inference rule and returns its result.
func (eng *InferenceEngine) executeRule(rule InferenceRule) RuleResult {
	result := RuleResult{
		RuleName:   rule.Name,
		RuleType:   rule.RuleType,
		Priority:   rule.Priority,
		Success:    false,
		ExecutedAt: time.Now(),
		Metadata:   make(map[string]interface{}),
	}

	switch rule.RuleType {
	case "l2-link":
		return eng.executeL2Rule(rule)
	case "l3-link":
		return eng.executeL3Rule(rule)
	case "subnet-group":
		return eng.executeSubnetRule(rule)
	case "implicit-link":
		return eng.executeImplicitLinkRule(rule)
	case "network-boundary":
		return eng.executeNetworkBoundaryRule(rule)
	case "nat-boundary":
		return eng.executeNATBoundaryRule(rule)
	default:
		result.Warnings = append(result.Warnings, "unknown rule type: "+rule.RuleType)
	}

	return result
}

// ========================================================================
// Rule Executors
// ========================================================================

func (eng *InferenceEngine) executeL2Rule(rule InferenceRule) RuleResult {
	result := RuleResult{
		RuleName:   rule.Name,
		RuleType:   rule.RuleType,
		Priority:   rule.Priority,
		ExecutedAt: time.Now(),
		Metadata:   make(map[string]interface{}),
	}

	// Run LLDP/CDP inference if LLDP data available
	if len(eng.Context.LLDPNeighbors) > 0 {
		lldpLinks := eng.l2Infer.InferFromLLDP()
		for i := range lldpLinks {
			result.Outputs = append(result.Outputs, lldpLinks[i])
		}
		result.Metadata["lldp_links_found"] = len(lldpLinks)
	}

	// Run MAC table inference if FDB data available
	if rule.Name == "l2-mac-table-links" && len(eng.Context.FDBEntries) > 0 {
		macLinks := eng.l2Infer.InferFromMACTable()
		for i := range macLinks {
			result.Outputs = append(result.Outputs, macLinks[i])
		}
		result.Metadata["mac_links_found"] = len(macLinks)
	}

	result.Success = true
	return result
}

func (eng *InferenceEngine) executeL3Rule(rule InferenceRule) RuleResult {
	result := RuleResult{
		RuleName:   rule.Name,
		RuleType:   rule.RuleType,
		Priority:   rule.Priority,
		ExecutedAt: time.Now(),
		Metadata:   make(map[string]interface{}),
	}

	// Infer L3 links from routing tables
	l3Links := eng.l3Infer.InferLinksFromRouting()
	for i := range l3Links {
		result.Outputs = append(result.Outputs, l3Links[i])
	}
	result.Metadata["routing_links_found"] = len(l3Links)

	result.Success = true
	return result
}

func (eng *InferenceEngine) executeSubnetRule(rule InferenceRule) RuleResult {
	result := RuleResult{
		RuleName:   rule.Name,
		RuleType:   rule.RuleType,
		Priority:   rule.Priority,
		ExecutedAt: time.Now(),
		Metadata:   make(map[string]interface{}),
	}

	groups := eng.subnetDisc.DiscoverSubnets()
	for i := range groups {
		result.Outputs = append(result.Outputs, groups[i])
	}
	result.Metadata["subnet_groups_found"] = len(groups)

	// Infer gateways for each subnet
	gatewaysFound := 0
	for _, group := range groups {
		gwDevice, gwIP := eng.subnetDisc.InferGateway(group)
		if gwDevice != "" {
			group.GatewayDevice = gwDevice
			group.GatewayIP = gwIP
			gatewaysFound++
		}
	}
	result.Metadata["gateways_inferred"] = gatewaysFound

	result.Success = true
	return result
}

func (eng *InferenceEngine) executeImplicitLinkRule(rule InferenceRule) RuleResult {
	result := RuleResult{
		RuleName:   rule.Name,
		RuleType:   rule.RuleType,
		Priority:   rule.Priority,
		ExecutedAt: time.Now(),
		Metadata:   make(map[string]interface{}),
	}

	links := eng.implicitLink.DetectLinks()
	for i := range links {
		result.Outputs = append(result.Outputs, links[i])
	}
	result.Metadata["implicit_links_found"] = len(links)

	result.Success = true
	return result
}

func (eng *InferenceEngine) executeNetworkBoundaryRule(rule InferenceRule) RuleResult {
	result := RuleResult{
		RuleName:   rule.Name,
		RuleType:   rule.RuleType,
		Priority:   rule.Priority,
		ExecutedAt: time.Now(),
		Metadata:   make(map[string]interface{}),
	}

	boundaries := eng.boundaryDet.DetectNetworkBoundaries()
	for i := range boundaries {
		result.Outputs = append(result.Outputs, boundaries[i])
	}
	result.Metadata["network_boundaries_found"] = len(boundaries)

	result.Success = true
	return result
}

func (eng *InferenceEngine) executeNATBoundaryRule(rule InferenceRule) RuleResult {
	result := RuleResult{
		RuleName:   rule.Name,
		RuleType:   rule.RuleType,
		Priority:   rule.Priority,
		ExecutedAt: time.Now(),
		Metadata:   make(map[string]interface{}),
	}

	boundaries := eng.boundaryDet.InferNATBoundaries()
	for i := range boundaries {
		result.Outputs = append(result.Outputs, boundaries[i])
	}
	result.Metadata["nat_boundaries_found"] = len(boundaries)

	result.Success = true
	return result
}

// ========================================================================
// Output Collection
// ========================================================================

func (eng *InferenceEngine) collectRuleOutputs(result *InferenceResult, ruleResult RuleResult) {
	for _, output := range ruleResult.Outputs {
		switch v := output.(type) {
		case InferredLink:
			switch v.LinkType {
			case "l2", "inferred-l2":
				result.L2Links = append(result.L2Links, v)
			case "l3", "inferred-l3":
				result.L3Links = append(result.L3Links, v)
			case "implicit":
				result.ImplicitLinks = append(result.ImplicitLinks, v)
			default:
				result.L3Links = append(result.L3Links, v)
			}
		case SubnetGroup:
			result.SubnetGroups = append(result.SubnetGroups, v)
		case NATBoundary:
			result.NATBoundaries = append(result.NATBoundaries, v)
		case NetworkBoundary:
			result.NetworkBoundaries = append(result.NetworkBoundaries, v)
		}
	}
}

// ========================================================================
// Conflict Resolution
// ========================================================================

func (eng *InferenceEngine) resolveConflicts(result *InferenceResult) {
	// Deduplicate links by (from, to) pair
	result.L2Links = deduplicateLinks(result.L2Links)
	result.L3Links = deduplicateLinks(result.L3Links)
	result.ImplicitLinks = deduplicateLinks(result.ImplicitLinks)

	// Deduplicate subnet groups
	result.SubnetGroups = deduplicateSubnetGroups(result.SubnetGroups)

	// Detect cross-type conflicts (e.g., L2 link conflicts with L3 link for same pair)
	allLinks := append(append([]InferredLink{}, result.L2Links...), result.L3Links...)
	allLinks = append(allLinks, result.ImplicitLinks...)
	result.Conflicts = detectLinkConflicts(allLinks)

	result.Stats.ConflictsDetected = len(result.Conflicts)
	result.Stats.ConflictsResolved = len(result.Conflicts) // All resolved by priority
}

// ========================================================================
// Path Inference
// ========================================================================

// ComputePath infers the routing path between two endpoints.
func (eng *InferenceEngine) ComputePath(sourceDevice, destIP string) PathResult {
	if eng.l3Infer == nil {
		eng.l3Infer = NewL3InferenceEngine(eng.Context)
	}
	return eng.l3Infer.ComputePath(sourceDevice, destIP)
}

// ComputePaths computes paths for multiple source-destination pairs concurrently.
func (eng *InferenceEngine) ComputePaths(queries []models.L3ReachabilityQuery) []PathResult {
	if eng.l3Infer == nil {
		eng.l3Infer = NewL3InferenceEngine(eng.Context)
	}

	results := make([]PathResult, len(queries))
	var wg sync.WaitGroup

	for i, q := range queries {
		wg.Add(1)
		go func(idx int, query models.L3ReachabilityQuery) {
			defer wg.Done()
			results[idx] = eng.l3Infer.ComputePath(query.SourceDevice, query.DestIP)
		}(i, q)
	}

	wg.Wait()
	return results
}

// ========================================================================
// Statistics
// ========================================================================

func (eng *InferenceEngine) computeStats(result *InferenceResult) {
	stats := &result.Stats

	stats.L2LinksInferred = len(result.L2Links)
	stats.L3LinksInferred = len(result.L3Links)
	stats.ImplicitLinksInferred = len(result.ImplicitLinks)
	stats.TotalLinksInferred = stats.L2LinksInferred + stats.L3LinksInferred + stats.ImplicitLinksInferred
	stats.SubnetGroupsFound = len(result.SubnetGroups)
	stats.NATBoundariesInferred = len(result.NATBoundaries)
	stats.NetworkBoundariesFound = len(result.NetworkBoundaries)
	stats.PathsComputed = len(result.Paths)
	stats.RulesExecuted = len(result.RuleResults)

	for _, rr := range result.RuleResults {
		if !rr.Success {
			stats.RulesFailed++
		}
	}

	// Average confidence across all links
	var confSum float64
	confCount := 0
	for _, l := range result.L2Links {
		confSum += l.Confidence
		confCount++
	}
	for _, l := range result.L3Links {
		confSum += l.Confidence
		confCount++
	}
	for _, l := range result.ImplicitLinks {
		confSum += l.Confidence
		confCount++
	}
	if confCount > 0 {
		stats.AverageConfidence = confSum / float64(confCount)
	}
}

package intelligence

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// ========================================================================
// RuleEngine — priority-based inference rule execution & conflict resolution
// ========================================================================

// RuleEngine manages inference rule lifecycle: registration, prioritization,
// execution ordering, and conflict resolution.
type RuleEngine struct {
	mu    sync.RWMutex
	rules []InferenceRule

	// Runtime state
	results     []RuleResult
	conflicts   []Conflict
	executedAt  time.Time
	totalDuration time.Duration
}

// RuleEngineView is a read-only interface to the rule engine (used by InferenceEngine).
type RuleEngineView struct {
	engine *RuleEngine
}

// NewRuleEngine creates a new rule engine with default rules.
func NewRuleEngine() *RuleEngine {
	re := &RuleEngine{}
	re.rules = DefaultInferenceRules()
	return re
}

// NewRuleEngineWithRules creates a rule engine with custom rules.
func NewRuleEngineWithRules(rules []InferenceRule) *RuleEngine {
	re := &RuleEngine{}
	re.rules = make([]InferenceRule, len(rules))
	copy(re.rules, rules)
	return re
}

// ========================================================================
// Rule Management
// ========================================================================

// AddRule adds a new inference rule to the engine.
func (re *RuleEngine) AddRule(rule InferenceRule) {
	re.mu.Lock()
	defer re.mu.Unlock()
	re.rules = append(re.rules, rule)
}

// RemoveRule removes a rule by name.
func (re *RuleEngine) RemoveRule(name string) bool {
	re.mu.Lock()
	defer re.mu.Unlock()

	for i, rule := range re.rules {
		if rule.Name == name {
			re.rules = append(re.rules[:i], re.rules[i+1:]...)
			return true
		}
	}
	return false
}

// EnableRule enables a rule by name.
func (re *RuleEngine) EnableRule(name string) bool {
	re.mu.Lock()
	defer re.mu.Unlock()

	for i := range re.rules {
		if re.rules[i].Name == name {
			re.rules[i].Enabled = true
			return true
		}
	}
	return false
}

// DisableRule disables a rule by name.
func (re *RuleEngine) DisableRule(name string) bool {
	re.mu.Lock()
	defer re.mu.Unlock()

	for i := range re.rules {
		if re.rules[i].Name == name {
			re.rules[i].Enabled = false
			return true
		}
	}
	return false
}

// GetRules returns all rules sorted by priority.
func (re *RuleEngine) GetRules() []InferenceRule {
	re.mu.RLock()
	defer re.mu.RUnlock()

	sorted := make([]InferenceRule, len(re.rules))
	copy(sorted, re.rules)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Priority > sorted[j].Priority
	})
	return sorted
}

// GetEnabledRules returns only enabled rules sorted by priority.
func (re *RuleEngine) GetEnabledRules() []InferenceRule {
	re.mu.RLock()
	defer re.mu.RUnlock()

	var enabled []InferenceRule
	for _, rule := range re.rules {
		if rule.Enabled {
			enabled = append(enabled, rule)
		}
	}
	sort.Slice(enabled, func(i, j int) bool {
		return enabled[i].Priority > enabled[j].Priority
	})
	return enabled
}

// ========================================================================
// Conflict Resolution
// ========================================================================

// ResolveConflict resolves a conflict between two claims using the rule's strategy.
func (re *RuleEngine) ResolveConflict(rule InferenceRule, claimA, claimB interface{}) Conflict {
	conflict := Conflict{
		ClaimA: ConflictClaim{
			RuleName: rule.Name,
			Priority: rule.Priority,
			Value:    claimA,
		},
		ClaimB: ConflictClaim{
			RuleName: "competing-rule",
			Priority: PriorityNormal,
			Value:    claimB,
		},
	}

	switch rule.ResolveConflict {
	case "prefer":
		// Prefer the higher-priority rule's claim
		conflict.Resolution = "prefer-a"
		conflict.ResolutionReason = fmt.Sprintf("Rule '%s' (priority=%d) takes precedence", rule.Name, rule.Priority)
	case "discard":
		// Discard both conflicting results
		conflict.Resolution = "discard-both"
		conflict.ResolutionReason = fmt.Sprintf("Rule '%s' configured to discard conflicting results", rule.Name)
	case "merge":
		// Attempt to merge both claims
		conflict.Resolution = "merge"
		conflict.ResolutionReason = fmt.Sprintf("Rule '%s' configured to merge — both claims retained", rule.Name)
	case "warn":
		// Keep both but flag the conflict
		conflict.Resolution = "unresolved"
		conflict.ResolutionReason = fmt.Sprintf("Rule '%s' configured to warn — manual review recommended", rule.Name)
	default:
		conflict.Resolution = "prefer-a"
		conflict.ResolutionReason = "Default resolution: prefer higher priority"
	}

	// Determine conflict type based on claim values
	conflict.ConflictType = re.detectConflictType(claimA, claimB)

	return conflict
}

// ResolveConflicts resolves all conflicts in a batch of results.
// Uses priority-based resolution: higher priority rules win.
func (re *RuleEngine) ResolveConflicts(results []RuleResult) []Conflict {
	var conflicts []Conflict

	// Group results by rule type for conflict detection
	byType := make(map[string][]RuleResult)
	for _, r := range results {
		byType[r.RuleType] = append(byType[r.RuleType], r)
	}

	// Detect conflicts within each type
	for _, typeResults := range byType {
		if len(typeResults) < 2 {
			continue
		}

		// Sort by priority (highest first)
		sort.Slice(typeResults, func(i, j int) bool {
			return typeResults[i].Priority > typeResults[j].Priority
		})

		// Higher priority results preempt lower ones
		highest := typeResults[0]
		for i := 1; i < len(typeResults); i++ {
			conflict := Conflict{
				ConflictType: highest.RuleType,
				ClaimA: ConflictClaim{
					RuleName: highest.RuleName,
					Priority: highest.Priority,
				},
				ClaimB: ConflictClaim{
					RuleName: typeResults[i].RuleName,
					Priority: typeResults[i].Priority,
				},
				Resolution:       "prefer-a",
				ResolutionReason: fmt.Sprintf("Priority %d > %d — higher priority rule '%s' takes precedence",
					highest.Priority, typeResults[i].Priority, highest.RuleName),
			}
			conflicts = append(conflicts, conflict)
		}
	}

	re.mu.Lock()
	re.conflicts = conflicts
	re.mu.Unlock()

	return conflicts
}

// detectConflictType determines the type of conflict from claim values.
func (re *RuleEngine) detectConflictType(a, b interface{}) string {
	switch a.(type) {
	case InferredLink:
		return "link"
	case SubnetGroup:
		return "subnet"
	case NATBoundary:
		return "nat-boundary"
	case NetworkBoundary:
		return "network-boundary"
	case PathResult:
		return "path"
	default:
		return "unknown"
	}
}

// ========================================================================
// Priority Management
// ========================================================================

// SetPriority updates the priority of a rule.
func (re *RuleEngine) SetPriority(name string, priority RulePriority) bool {
	re.mu.Lock()
	defer re.mu.Unlock()

	for i := range re.rules {
		if re.rules[i].Name == name {
			re.rules[i].Priority = priority
			re.rules[i].UpdatedAt = time.Now()
			return true
		}
	}
	return false
}

// ReprioritizeRules reorders rules by a custom priority function.
func (re *RuleEngine) ReprioritizeRules(priorities map[string]RulePriority) {
	re.mu.Lock()
	defer re.mu.Unlock()

	for i := range re.rules {
		if p, ok := priorities[re.rules[i].Name]; ok {
			re.rules[i].Priority = p
			re.rules[i].UpdatedAt = time.Now()
		}
	}
}

// ========================================================================
// Rule Validation
// ========================================================================

// ValidateRule checks if a rule definition is valid.
func ValidateRule(rule InferenceRule) error {
	if rule.Name == "" {
		return fmt.Errorf("rule name is required")
	}
	if rule.Priority < 0 || rule.Priority > 100 {
		return fmt.Errorf("priority must be between 0 and 100")
	}
	validTypes := map[string]bool{
		"l3-link": true, "l2-link": true, "subnet-group": true,
		"nat-boundary": true, "network-boundary": true, "implicit-link": true,
	}
	if !validTypes[rule.RuleType] {
		return fmt.Errorf("invalid rule type: %s", rule.RuleType)
	}
	validResolutions := map[string]bool{
		"prefer": true, "discard": true, "merge": true, "warn": true,
	}
	if !validResolutions[rule.ResolveConflict] {
		return fmt.Errorf("invalid conflict resolution: %s", rule.ResolveConflict)
	}
	return nil
}

// ValidateAllRules validates all rules in the engine.
func (re *RuleEngine) ValidateAllRules() []error {
	re.mu.RLock()
	defer re.mu.RUnlock()

	var errs []error
	for _, rule := range re.rules {
		if err := ValidateRule(rule); err != nil {
			errs = append(errs, fmt.Errorf("rule %s: %w", rule.Name, err))
		}
	}
	return errs
}

package policy

import "github.com/mattpainter701/graphlayer/platform/rustengine"

const ContractVersion = "graphlayer-policy-v3"

// ValidateScope delegates scope validation to the authoritative Rust engine.
func ValidateScope(scope Scope) error {
	var result struct {
		Valid bool `json:"valid"`
	}
	return rustengine.Evaluate("validate_scope", map[string]any{"scope": scope}, &result)
}

func Validate(bundle Bundle, flow Flow) error {
	var result struct {
		Valid bool `json:"valid"`
	}
	return rustengine.Evaluate("validate_policy", map[string]any{"bundle": bundle, "flow": flow}, &result)
}

func Evaluate(bundle Bundle, flow Flow) (Result, error) {
	var result Result
	err := rustengine.Evaluate("evaluate_policy", map[string]any{"bundle": bundle, "flow": flow}, &result)
	return result, err
}

// EvaluateRecord asks Rust to preserve the evidence anchor and exact historical
// contract used by a persisted workspace record. Storage keeps the original bytes.
func EvaluateRecord(bundle Bundle, flow Flow, defaultEvidence []string, legacy bool) (Result, error) {
	var result Result
	err := rustengine.Evaluate("evaluate_policy", map[string]any{"bundle": bundle, "flow": flow, "default_evidence": defaultEvidence, "legacy": legacy}, &result)
	return result, err
}

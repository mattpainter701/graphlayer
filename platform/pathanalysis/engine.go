package pathanalysis

import "github.com/mattpainter701/graphlayer/platform/rustengine"

const VendorContractVersion = "graphlayer-vendor-path-v1"
const maxRouteCandidates = 8

type Candidate struct {
	ID            string `json:"id"`
	Result        Result `json:"result"`
	Resolved      bool   `json:"resolved"`
	SessionStatus string `json:"session_status"`
}

func request(in Input) map[string]any {
	return map[string]any{"scope": in.Scope, "config": in.Config, "ingress_interface": in.IngressInterface, "flow": in.Flow}
}
func Evaluate(in Input) (Result, error) {
	var result Result
	err := rustengine.Evaluate("evaluate_path", request(in), &result)
	return result, err
}
func EvaluateReturn(in Input, forward Result) (Result, error) {
	input := request(in)
	input["forward"] = forward
	var result Result
	err := rustengine.Evaluate("evaluate_return", input, &result)
	return result, err
}
func Explore(in Input) ([]Candidate, bool, error) { return explore("explore_path", request(in)) }
func ExploreReturn(in Input, forward *Result) ([]Candidate, bool, error) {
	input := request(in)
	input["forward"] = forward
	return explore("explore_return", input)
}
func explore(operation string, input any) ([]Candidate, bool, error) {
	var result struct {
		Candidates []Candidate `json:"candidates"`
		Truncated  bool        `json:"truncated"`
	}
	err := rustengine.Evaluate(operation, input, &result)
	return result.Candidates, result.Truncated, err
}

func IsASAVendor(vendor string) bool {
	var result bool
	return rustengine.Evaluate("path.is_asa_vendor", map[string]any{"vendor": vendor}, &result) == nil && result
}

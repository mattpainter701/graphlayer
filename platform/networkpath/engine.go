package networkpath

import "github.com/mattpainter701/graphlayer/platform/rustengine"

const maxExplorationBranches = 32

func Evaluate(input Input) (Result, error) {
	var result Result
	err := rustengine.Evaluate("evaluate_network", input, &result)
	return result, err
}
func Explore(input Input) (ExplorationResult, error) {
	var result ExplorationResult
	err := rustengine.Evaluate("explore_network", input, &result)
	return result, err
}

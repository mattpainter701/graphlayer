package pathanalysis

import "github.com/mattpainter701/graphlayer/platform/policy"

// Helpers used only to construct reverse-flow fixtures and inspect assertions.
func reverseFlow(flow policy.Flow, scope policy.Scope) policy.Flow {
	flow.Scope = scope
	flow.Source, flow.Destination = flow.Destination, flow.Source
	flow.SourcePort, flow.DestinationPort = flow.DestinationPort, flow.SourcePort
	return flow
}
func stageByName(stages []Stage, name string) *Stage {
	for i := range stages {
		if stages[i].Name == name {
			return &stages[i]
		}
	}
	return nil
}

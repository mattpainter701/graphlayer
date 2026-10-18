// Package pathanalysis evaluates a bounded single-device forwarding model.
package pathanalysis

import (
	"github.com/mattpainter701/graphlayer/platform/firewall"
	"github.com/mattpainter701/graphlayer/platform/policy"
)

const ContractVersion = "graphlayer-path-v1"

type Input struct {
	Scope            policy.Scope
	Config           firewall.Config
	IngressInterface string
	Flow             policy.Flow
}

type Stage struct {
	Name         string       `json:"name"`
	Status       string       `json:"status"`
	Detail       string       `json:"detail"`
	Before       *policy.Flow `json:"before,omitempty"`
	After        *policy.Flow `json:"after,omitempty"`
	EvidenceRefs []string     `json:"evidence_refs,omitempty"`
	CandidateIDs []string     `json:"candidate_ids,omitempty"`
}

type Result struct {
	EngineContractVersion string              `json:"engine_contract_version"`
	Scope                 policy.Scope        `json:"scope"`
	IngressInterface      string              `json:"ingress_interface"`
	OriginalFlow          policy.Flow         `json:"original_flow"`
	PolicyFlow            policy.Flow         `json:"policy_flow"`
	EgressFlow            policy.Flow         `json:"egress_flow"`
	Disposition           string              `json:"disposition"`
	EgressInterface       string              `json:"egress_interface,omitempty"`
	Stages                []Stage             `json:"stages"`
	SelectedRoute         *firewall.PathRoute `json:"selected_route,omitempty"`
	NATRuleIDs            []string            `json:"nat_rule_ids,omitempty"`
	ChainID               string              `json:"chain_id,omitempty"`
	Policy                *policy.Result      `json:"policy,omitempty"`
	Reasons               []string            `json:"reasons"`
	Assumptions           []string            `json:"assumptions"`
	EvidenceRefs          []string            `json:"evidence_refs,omitempty"`
}

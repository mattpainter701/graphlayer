// Package networkpath composes bounded, evidence-backed ASA path analyses.
package networkpath

import (
	"github.com/mattpainter701/graphlayer/platform/firewall"
	"github.com/mattpainter701/graphlayer/platform/pathanalysis"
	"github.com/mattpainter701/graphlayer/platform/policy"
)

const ContractVersion = "graphlayer-network-path-v1"
const VendorContractVersion = "graphlayer-network-path-v2"

type Endpoint struct {
	NodeID    string `json:"node_id"`
	Interface string `json:"interface"`
}
type Node struct {
	ID     string          `json:"id"`
	Scope  policy.Scope    `json:"scope"`
	Config firewall.Config `json:"config"`
}
type Link struct {
	A Endpoint `json:"a"`
	B Endpoint `json:"b"`
}
type Input struct {
	Nodes       []Node      `json:"nodes"`
	Links       []Link      `json:"links"`
	Source      Endpoint    `json:"source"`
	Destination Endpoint    `json:"destination"`
	Flow        policy.Flow `json:"flow"`
}
type EvidenceRef struct {
	SnapshotID string `json:"snapshot_id"`
	Ref        string `json:"ref"`
}
type Hop struct {
	NodeID     string              `json:"node_id"`
	SnapshotID string              `json:"snapshot_id"`
	Path       pathanalysis.Result `json:"path"`
	Link       *Link               `json:"link,omitempty"`
	Evidence   []EvidenceRef       `json:"evidence,omitempty"`
}
type Direction struct {
	Disposition string      `json:"disposition"`
	Hops        []Hop       `json:"hops"`
	FinalFlow   policy.Flow `json:"final_flow"`
	Reasons     []string    `json:"reasons"`
}
type Result struct {
	EngineContractVersion string      `json:"engine_contract_version"`
	OriginalFlow          policy.Flow `json:"original_flow"`
	Forward               Direction   `json:"forward"`
	Return                Direction   `json:"return"`
	Disposition           string      `json:"disposition"`
	Reasons               []string    `json:"reasons"`
	Assumptions           []string    `json:"assumptions"`
}

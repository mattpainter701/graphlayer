package networkpath

import "github.com/mattpainter701/graphlayer/platform/policy"

const ExplorationContractVersion = "graphlayer-network-exploration-v1"

type ExplorationResult struct {
	EngineContractVersion string              `json:"engine_contract_version"`
	OriginalFlow          policy.Flow         `json:"original_flow"`
	Branches              []ExplorationBranch `json:"branches"`
	Disposition           string              `json:"disposition"`
	Complete              bool                `json:"complete"`
	Truncated             bool                `json:"truncated"`
	Reasons               []string            `json:"reasons"`
	Assumptions           []string            `json:"assumptions"`
}

type ExplorationBranch struct {
	ID          string         `json:"id"`
	Forward     Direction      `json:"forward"`
	Returns     []ReturnBranch `json:"returns"`
	Disposition string         `json:"disposition"`
	Reasons     []string       `json:"reasons"`
}

type ReturnBranch struct {
	ID            string    `json:"id"`
	Direction     Direction `json:"direction"`
	Asymmetric    bool      `json:"asymmetric"`
	SessionStatus string    `json:"session_status"`
	Disposition   string    `json:"disposition"`
	Reasons       []string  `json:"reasons"`
}

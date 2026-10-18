// Package policy provides a conservative, policy-only evaluator for explicit
// single-device snapshots. It does not model routing, NAT, identity, time, or
// application semantics.
package policy

type Scope struct {
	SnapshotID string `json:"snapshot_id"`
	DeviceID   string `json:"device_id"`
	Context    string `json:"context"`
}

type Bundle struct {
	Scope         Scope  `json:"scope"`
	Complete      bool   `json:"complete"`
	DefaultAction string `json:"default_action"`
	Rules         []Rule `json:"rules"`
}

// Rule predicates support literal IPs/CIDRs/"any", numeric protocol numbers
// (or tcp, udp, icmp, icmpv6), and numeric ports/ranges. Any other non-empty
// predicate is retained as unsupported and can make evaluation indeterminate.
type Rule struct {
	ID              string   `json:"id"`
	Position        int      `json:"position"`
	Enabled         bool     `json:"enabled"`
	Action          string   `json:"action"`
	Source          string   `json:"source"`
	Destination     string   `json:"destination"`
	Protocol        string   `json:"protocol"`
	SourcePort      string   `json:"source_port"`
	DestinationPort string   `json:"destination_port"`
	EvidenceRefs    []string `json:"evidence_refs,omitempty"`
	Unsupported     []string `json:"unsupported,omitempty"`
}

type Flow struct {
	Scope           Scope  `json:"scope"`
	Source          string `json:"source"`
	Destination     string `json:"destination"`
	Protocol        string `json:"protocol"`
	SourcePort      int    `json:"source_port,omitempty"`
	DestinationPort int    `json:"destination_port,omitempty"`
}

type Result struct {
	EngineContractVersion string   `json:"engine_contract_version"`
	Scope                 Scope    `json:"scope"`
	Flow                  Flow     `json:"flow"`
	Disposition           string   `json:"disposition"`
	DecisiveRule          string   `json:"decisive_rule,omitempty"`
	CandidateRule         string   `json:"candidate_rule,omitempty"`
	UsedDefault           bool     `json:"used_default"`
	Considered            []string `json:"considered_rules"`
	EvidenceRefs          []string `json:"evidence_refs,omitempty"`
	Reasons               []string `json:"reasons"`
	Supported             []string `json:"supported_semantics"`
	UnsupportedDimensions []string `json:"unsupported_dimensions"`
	Assumptions           []string `json:"assumptions"`
}

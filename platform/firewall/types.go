// Package firewall adapts vendor configuration exports into conservative,
// evidence-linked policy chains. It never claims end-to-end reachability.
package firewall

import "github.com/mattpainter701/graphlayer/platform/policy"

type SourceLine struct {
	Number int    `json:"number"`
	Text   string `json:"text"`
}

type Evidence struct {
	ID        string `json:"id"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	Kind      string `json:"kind"`
	Summary   string `json:"summary"`
}

type Warning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Line    int    `json:"line,omitempty"`
}

type Chain struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Interface    string        `json:"interface"`
	Direction    string        `json:"direction"`
	Complete     bool          `json:"complete"`
	Rules        []policy.Rule `json:"rules"`
	EvidenceRefs []string      `json:"evidence_refs"`
	Warnings     []Warning     `json:"warnings"`
}

// Object describes one named policy object within this exact snapshot. Its
// members retain dependency links even when expansion cannot safely resolve.
type Object struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	Kind         string         `json:"kind"`
	Protocol     string         `json:"protocol,omitempty"`
	Resolved     bool           `json:"resolved"`
	Members      []ObjectMember `json:"members"`
	EvidenceRefs []string       `json:"evidence_refs"`
	Warnings     []Warning      `json:"warnings,omitempty"`
}

type ObjectMember struct {
	Kind            string   `json:"kind"`
	Value           string   `json:"value,omitempty"`
	ObjectID        string   `json:"object_id,omitempty"`
	Protocol        string   `json:"protocol,omitempty"`
	SourcePort      string   `json:"source_port,omitempty"`
	DestinationPort string   `json:"destination_port,omitempty"`
	EvidenceRefs    []string `json:"evidence_refs,omitempty"`
}

// RuleOrigin links an expanded normalized rule back to its original ACE and
// named dependencies. RuleID is unique across all chains in one snapshot.
type RuleOrigin struct {
	RuleID               string   `json:"rule_id"`
	ACLName              string   `json:"acl_name"`
	Line                 int      `json:"line"`
	SourceObjectIDs      []string `json:"source_object_ids,omitempty"`
	DestinationObjectIDs []string `json:"destination_object_ids,omitempty"`
	ServiceObjectIDs     []string `json:"service_object_ids,omitempty"`
}

type Config struct {
	Vendor         string       `json:"vendor"`
	ParserVersion  string       `json:"parser_version"`
	Hostname       string       `json:"hostname"`
	Context        string       `json:"context"`
	ArtifactSHA256 string       `json:"artifact_sha256"`
	SourceLines    []SourceLine `json:"source_lines"`
	Evidence       []Evidence   `json:"evidence"`
	Chains         []Chain      `json:"chains"`
	Warnings       []Warning    `json:"warnings"`
	Capabilities   []string     `json:"capabilities"`
	Objects        []Object     `json:"objects,omitempty"`
	RuleOrigins    []RuleOrigin `json:"rule_origins,omitempty"`
	Path           *PathConfig  `json:"path,omitempty"`
}

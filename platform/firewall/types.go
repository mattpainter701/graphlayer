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
}

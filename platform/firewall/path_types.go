package firewall

import "github.com/mattpainter701/graphlayer/platform/policy"

// PathConfig is a bounded, configuration-derived vendor forwarding model. It is
// separate from live route, link, connection, and translation state.
type PathConfig struct {
	Version         string            `json:"version"`
	Interfaces      []PathInterface   `json:"interfaces"`
	Routes          []PathRoute       `json:"routes"`
	NATRules        []PathNATRule     `json:"nat_rules"`
	RoutingComplete bool              `json:"routing_complete"`
	NATComplete     bool              `json:"nat_complete"`
	InterInterface  bool              `json:"inter_interface"`
	Warnings        []Warning         `json:"warnings"`
	EvidenceRefs    []string          `json:"evidence_refs"`
	Policies        []PathPolicyRule  `json:"policies,omitempty"`
	Translations    []PathTranslation `json:"translations,omitempty"`
	PolicyComplete  bool              `json:"policy_complete,omitempty"`
}

type PathInterface struct {
	Name          string   `json:"name"`
	Physical      string   `json:"physical"`
	Address       string   `json:"address,omitempty"`
	SecurityLevel *int     `json:"security_level,omitempty"`
	Shutdown      bool     `json:"shutdown"`
	Resolved      bool     `json:"resolved"`
	EvidenceRefs  []string `json:"evidence_refs"`
	Zone          string   `json:"zone,omitempty"`
}

// PathPolicyRule retains ordered vendor zone/interface policy semantics.
type PathPolicyRule struct {
	ID           string      `json:"id"`
	From         string      `json:"from"`
	To           string      `json:"to"`
	Match        policy.Rule `json:"match"`
	VIP          string      `json:"vip,omitempty"`
	SourceNAT    string      `json:"source_nat,omitempty"`
	EvidenceRefs []string    `json:"evidence_refs"`
}

// PathTranslation is a bounded static translation, separate from ASA object NAT.
type PathTranslation struct {
	ID               string      `json:"id"`
	Kind             string      `json:"kind"`
	From             string      `json:"from"`
	To               string      `json:"to"`
	Match            policy.Rule `json:"match"`
	OriginalPrefix   string      `json:"original_prefix,omitempty"`
	TranslatedPrefix string      `json:"translated_prefix,omitempty"`
	PolicyID         string      `json:"policy_id,omitempty"`
	EvidenceRefs     []string    `json:"evidence_refs"`
}

type PathRoute struct {
	ID           string   `json:"id"`
	Kind         string   `json:"kind"`
	Prefix       string   `json:"prefix"`
	Interface    string   `json:"interface"`
	NextHop      string   `json:"next_hop,omitempty"`
	Distance     int      `json:"distance"`
	EvidenceRefs []string `json:"evidence_refs"`
}

type PathNATRule struct {
	ID              string   `json:"id"`
	ObjectName      string   `json:"object_name"`
	RealInterface   string   `json:"real_interface"`
	MappedInterface string   `json:"mapped_interface"`
	RealPrefix      string   `json:"real_prefix"`
	MappedPrefix    string   `json:"mapped_prefix"`
	Protocol        string   `json:"protocol,omitempty"`
	RealPort        int      `json:"real_port,omitempty"`
	MappedPort      int      `json:"mapped_port,omitempty"`
	EvidenceRefs    []string `json:"evidence_refs"`
}

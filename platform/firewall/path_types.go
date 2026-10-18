package firewall

// PathConfig is a bounded, configuration-derived ASA forwarding model. It is
// separate from live route, link, connection, and translation state.
type PathConfig struct {
	Version         string          `json:"version"`
	Interfaces      []PathInterface `json:"interfaces"`
	Routes          []PathRoute     `json:"routes"`
	NATRules        []PathNATRule   `json:"nat_rules"`
	RoutingComplete bool            `json:"routing_complete"`
	NATComplete     bool            `json:"nat_complete"`
	InterInterface  bool            `json:"inter_interface"`
	Warnings        []Warning       `json:"warnings"`
	EvidenceRefs    []string        `json:"evidence_refs"`
}

type PathInterface struct {
	Name          string   `json:"name"`
	Physical      string   `json:"physical"`
	Address       string   `json:"address,omitempty"`
	SecurityLevel *int     `json:"security_level,omitempty"`
	Shutdown      bool     `json:"shutdown"`
	Resolved      bool     `json:"resolved"`
	EvidenceRefs  []string `json:"evidence_refs"`
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

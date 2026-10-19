package cisco

import "github.com/mattpainter701/graphlayer/platform/rustengine"

// ServiceObject and ServiceGroup retain the common TCP/UDP service grammar.
// Unsupported members are represented only by a bounded static label.
type ServiceObject struct {
	Name                        string
	Protocol                    string
	SourcePort, DestinationPort string
	MemberLine                  int
	Lines                       []int
	UnsupportedLines            []int
}
type ServiceMember struct {
	Kind, Protocol, SourcePort, DestinationPort, Reference string
	ReferenceKind                                          string
	Line                                                   int
}
type ServiceGroup struct {
	Name             string
	Protocol         string
	Members          []ServiceMember
	Lines            []int
	UnsupportedLines []int
}

func servicePortFor(protocol, name string) (string, bool) {
	var result struct {
		Port string `json:"port"`
		OK   bool   `json:"ok"`
	}
	if rustengine.Evaluate("asa.service_port", map[string]any{"protocol": protocol, "name": name}, &result) != nil {
		return "", false
	}
	return result.Port, result.OK
}
func parsePortAlternativesFor(fields []string, protocol string) ([]string, int, bool) {
	return portAlternatives(fields, protocol, false)
}
func parsePortObjectAlternatives(fields []string, protocol string) ([]string, int, bool) {
	return portAlternatives(fields, protocol, true)
}
func portAlternatives(fields []string, protocol string, object bool) ([]string, int, bool) {
	var result struct {
		Ports    []string `json:"ports"`
		Consumed int      `json:"consumed"`
		OK       bool     `json:"ok"`
	}
	if rustengine.Evaluate("asa.port_alternatives", map[string]any{"fields": fields, "protocol": protocol, "object": object}, &result) != nil {
		return nil, 0, false
	}
	return result.Ports, result.Consumed, result.OK
}

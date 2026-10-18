package firewall

import (
	"fmt"
	"sort"
	"strings"

	asa "github.com/mattpainter701/graphlayer/platform/vendors/cisco/asa"
)

type serviceTerm struct{ protocol, source, destination string }

func expandServiceGroup(c *asa.ASAConfig, name string) ([]serviceTerm, []int, error) {
	terms := []serviceTerm{}
	lines := []int{}
	var visit func(string, map[string]bool, int, string) error
	visit = func(n string, stack map[string]bool, depth int, parentProtocol string) error {
		if depth > asa.MaxObjectDepth {
			return fmt.Errorf("service nesting limit")
		}
		if stack[n] {
			return fmt.Errorf("cyclic service group")
		}
		g, ok := c.ServiceGroups[n]
		if !ok {
			return fmt.Errorf("missing service group")
		}
		if parentProtocol != "" && g.Protocol != "" && g.Protocol != parentProtocol {
			return fmt.Errorf("incompatible nested service group protocol")
		}
		lines = append(lines, g.Lines...)
		if len(g.UnsupportedLines) > 0 {
			return fmt.Errorf("unsupported service member")
		}
		if len(g.Members) == 0 {
			return fmt.Errorf("empty service group")
		}
		stack[n] = true
		defer delete(stack, n)
		for _, m := range g.Members {
			lines = append(lines, m.Line)
			switch m.Kind {
			case "unsupported":
				return fmt.Errorf("unsupported service member")
			case "reference":
				if m.ReferenceKind == "service_object" {
					o := c.ServiceObjects[m.Reference]
					if o.Name == "" {
						return fmt.Errorf("missing service object")
					}
					lines = append(lines, o.Lines...)
					if len(o.UnsupportedLines) > 0 || o.Protocol == "" {
						return fmt.Errorf("unresolved service object")
					}
					appendTerm(&terms, o.Protocol, o.SourcePort, o.DestinationPort)
				} else if m.ReferenceKind == "service_group" {
					if _, ok := c.ServiceGroups[m.Reference]; !ok {
						return fmt.Errorf("missing service reference")
					}
					if err := visit(m.Reference, stack, depth+1, g.Protocol); err != nil {
						return err
					}
				} else {
					return fmt.Errorf("unknown service reference kind")
				}
			case "service":
				appendTerm(&terms, m.Protocol, m.SourcePort, m.DestinationPort)
			}
			if len(terms) > asa.MaxACEExpansion {
				return fmt.Errorf("service expansion limit")
			}
		}
		return nil
	}
	err := visit(name, map[string]bool{}, 0, "")
	if err != nil {
		return nil, unique(lines), err
	}
	return terms, unique(lines), nil
}
func appendTerm(out *[]serviceTerm, p, sp, dp string) {
	protocols := []string{p}
	if p == "tcp-udp" {
		protocols = []string{"tcp", "udp"}
	}
	for _, source := range servicePortValues(sp) {
		for _, destination := range servicePortValues(dp) {
			for _, protocol := range protocols {
				*out = append(*out, serviceTerm{protocol, source, destination})
			}
		}
	}
}

func servicePortValues(value string) []string {
	if value == "" {
		return []string{""}
	}
	return strings.Split(value, "|")
}

func addASAInventory(c *Config, parsed *asa.ASAConfig) {
	refs := map[int]string{}
	evidenceIDs := make(map[string]bool, len(c.Evidence))
	for _, evidence := range c.Evidence {
		evidenceIDs[evidence.ID] = true
	}
	addEvidence := func(line int, kind, summary string) string {
		if id, ok := refs[line]; ok {
			return id
		}
		markSourceLines(c, []int{line}, parsed.Lines)
		id := evidenceID(c.ArtifactSHA256, []int{line})
		refs[line] = id
		if evidenceIDs[id] {
			return id
		}
		c.Evidence = append(c.Evidence, Evidence{ID: id, StartLine: line, EndLine: line, Kind: kind, Summary: summary})
		evidenceIDs[id] = true
		return id
	}
	names := make([]string, 0, len(parsed.Objects))
	for n := range parsed.Objects {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		o := parsed.Objects[n]
		x := Object{ID: "network_object:" + n, Name: n, Kind: "network_object", Resolved: len(o.Members) > 0 && len(o.UnsupportedLines) == 0, Members: []ObjectMember{}, EvidenceRefs: []string{}}
		if o.Group {
			x.ID = "network_group:" + n
			x.Kind = "network_group"
		}
		for _, line := range unique(o.Lines) {
			x.EvidenceRefs = append(x.EvidenceRefs, addEvidence(line, "object_member", "Network object declaration or member"))
		}
		for i, m := range o.Members {
			member := ObjectMember{Kind: "address", Value: strings.TrimPrefix(m, "@")}
			if i < len(o.MemberLines) && o.MemberLines[i] > 0 {
				member.EvidenceRefs = []string{addEvidence(o.MemberLines[i], "object_member", "Network object member")}
			}
			if strings.HasPrefix(m, "@") {
				ref := strings.TrimPrefix(m, "@")
				wantKind := "network_group"
				typedRef := false
				if strings.HasPrefix(ref, "network_object:") {
					wantKind = "network_object"
					ref = strings.TrimPrefix(ref, "network_object:")
					typedRef = true
				} else if strings.HasPrefix(ref, "network_group:") {
					ref = strings.TrimPrefix(ref, "network_group:")
					typedRef = true
				}
				member.Kind = "reference"
				member.Value = ""
				member.ObjectID = wantKind + ":" + ref
				if !typedRef {
					if _, ok := parsed.Objects[ref]; ok && parsed.Objects[ref].Group {
						member.ObjectID = "network_group:" + ref
					} else if _, ok := parsed.Objects[ref]; ok {
						member.ObjectID = "network_object:" + ref
					}
				}
				if _, ok := parsed.Objects[ref]; !ok {
					x.Resolved = false
				}
			}
			x.Members = append(x.Members, member)
		}
		if _, _, err := parsed.ExpandAddress("@" + n); err != nil {
			x.Resolved = false
		}
		if len(o.UnsupportedLines) > 0 {
			x.Resolved = false
			x.Warnings = append(x.Warnings, Warning{Code: "unsupported_object_member", Message: "Network object has unsupported members", Line: o.UnsupportedLines[0]})
		}
		for _, line := range o.UnsupportedLines {
			x.Members = append(x.Members, ObjectMember{Kind: "unsupported", Value: "unsupported network member", EvidenceRefs: []string{addEvidence(line, "object_member", "Unsupported network object member")}})
		}
		c.Objects = append(c.Objects, x)
	}
	serviceNames := make([]string, 0, len(parsed.ServiceObjects))
	for n := range parsed.ServiceObjects {
		serviceNames = append(serviceNames, n)
	}
	sort.Strings(serviceNames)
	for _, n := range serviceNames {
		o := parsed.ServiceObjects[n]
		x := Object{ID: "service_object:" + n, Name: n, Kind: "service_object", Protocol: o.Protocol, Resolved: o.Protocol != "" && len(o.UnsupportedLines) == 0, Members: []ObjectMember{}, EvidenceRefs: []string{}}
		for _, line := range unique(o.Lines) {
			x.EvidenceRefs = append(x.EvidenceRefs, addEvidence(line, "service_object", "Service object declaration or member"))
		}
		if o.Protocol != "" {
			for _, sourcePort := range servicePortValues(o.SourcePort) {
				for _, destinationPort := range servicePortValues(o.DestinationPort) {
					member := ObjectMember{Kind: "service", Protocol: o.Protocol, SourcePort: sourcePort, DestinationPort: destinationPort}
					if o.MemberLine > 0 {
						member.EvidenceRefs = []string{addEvidence(o.MemberLine, "service_member", "Service object definition")}
					}
					x.Members = append(x.Members, member)
				}
			}
		}
		if !x.Resolved {
			x.Warnings = append(x.Warnings, Warning{Code: "unresolved_service_object", Message: "Service object has unsupported or missing service definition"})
		}
		for _, line := range o.UnsupportedLines {
			x.Members = append(x.Members, ObjectMember{Kind: "unsupported", Value: "unsupported service object member", EvidenceRefs: []string{addEvidence(line, "service_member", "Unsupported service object member")}})
		}
		c.Objects = append(c.Objects, x)
	}
	serviceGroupNames := make([]string, 0, len(parsed.ServiceGroups))
	for n := range parsed.ServiceGroups {
		serviceGroupNames = append(serviceGroupNames, n)
	}
	sort.Strings(serviceGroupNames)
	for _, n := range serviceGroupNames {
		g := parsed.ServiceGroups[n]
		x := Object{ID: "service_group:" + n, Name: n, Kind: "service_group", Protocol: g.Protocol, Resolved: len(g.Members) > 0 && len(g.UnsupportedLines) == 0, Members: []ObjectMember{}, EvidenceRefs: []string{}}
		for _, line := range unique(g.Lines) {
			x.EvidenceRefs = append(x.EvidenceRefs, addEvidence(line, "service_group", "Service group declaration or member"))
		}
		for _, m := range g.Members {
			om := ObjectMember{Kind: m.Kind, Protocol: m.Protocol, SourcePort: m.SourcePort, DestinationPort: m.DestinationPort, EvidenceRefs: []string{addEvidence(m.Line, "service_member", "Service group member")}}
			if m.Kind == "reference" {
				om.ObjectID = m.ReferenceKind + ":" + m.Reference
				if m.ReferenceKind == "service_object" {
					if _, ok := parsed.ServiceObjects[m.Reference]; !ok {
						x.Resolved = false
					}
				} else if m.ReferenceKind == "service_group" {
					if _, ok := parsed.ServiceGroups[m.Reference]; !ok {
						x.Resolved = false
					}
				} else {
					x.Resolved = false
				}
			}
			x.Members = append(x.Members, om)
		}
		if len(g.UnsupportedLines) > 0 {
			x.Resolved = false
			x.Warnings = append(x.Warnings, Warning{Code: "unsupported_service_member", Message: "Service group has unsupported members", Line: g.UnsupportedLines[0]})
		}
		if _, _, err := expandServiceGroup(parsed, n); err != nil {
			x.Resolved = false
		}
		c.Objects = append(c.Objects, x)
	}
}

func serviceRuleTerms(c *asa.ASAConfig, ace asa.ConfigACE) ([]serviceTerm, []int, error) {
	if ace.ServiceGroup != "" {
		if ace.SourcePort != "" || ace.DestinationPort != "" {
			return nil, nil, fmt.Errorf("service group ACE has additional port predicates")
		}
		return expandServiceGroup(c, ace.ServiceGroup)
	}
	if ace.ServiceObject != "" {
		if ace.SourcePort != "" || ace.DestinationPort != "" {
			return nil, nil, fmt.Errorf("service object ACE has additional port predicates")
		}
		o, ok := c.ServiceObjects[ace.ServiceObject]
		if !ok || o.Protocol == "" || len(o.UnsupportedLines) > 0 {
			return nil, append([]int(nil), o.Lines...), fmt.Errorf("unresolved service object")
		}
		terms := []serviceTerm{}
		appendTerm(&terms, o.Protocol, o.SourcePort, o.DestinationPort)
		return terms, unique(o.Lines), nil
	}
	sourcePorts, destinationPorts := servicePortValues(ace.SourcePort), servicePortValues(ace.DestinationPort)
	lines := []int{}
	for _, item := range []struct {
		value  string
		source bool
	}{{ace.SourcePort, true}, {ace.DestinationPort, false}} {
		if !strings.HasPrefix(item.value, "@") {
			continue
		}
		groupName := strings.TrimPrefix(item.value, "@")
		terms, groupLines, err := expandServiceGroup(c, groupName)
		lines = append(lines, groupLines...)
		if err != nil {
			return nil, unique(lines), err
		}
		if len(terms) == 0 {
			return nil, unique(lines), fmt.Errorf("empty port group")
		}
		group, exists := c.ServiceGroups[groupName]
		if !exists || (group.Protocol != ace.Protocol && !(group.Protocol == "tcp-udp" && (ace.Protocol == "tcp" || ace.Protocol == "udp"))) {
			return nil, unique(lines), fmt.Errorf("port group protocol mismatch")
		}
		ports := []string{}
		for _, term := range terms {
			if term.source != "" || term.destination == "" {
				return nil, unique(lines), fmt.Errorf("service group is not a port-only group")
			}
			if term.protocol != ace.Protocol {
				if group.Protocol == "tcp-udp" && (term.protocol == "tcp" || term.protocol == "udp") {
					continue
				}
				return nil, unique(lines), fmt.Errorf("port group protocol mismatch")
			}
			ports = append(ports, term.destination)
		}
		ports = uniqueStrings(ports)
		if len(ports) == 0 {
			return nil, unique(lines), fmt.Errorf("port group has no alternatives for ACE protocol")
		}
		if item.source {
			sourcePorts = ports
		} else {
			destinationPorts = ports
		}
		if len(sourcePorts)*len(destinationPorts) > asa.MaxACEExpansion {
			return nil, unique(lines), fmt.Errorf("port group cross-product limit")
		}
	}
	if len(sourcePorts)*len(destinationPorts) > asa.MaxACEExpansion {
		return nil, unique(lines), fmt.Errorf("port predicate cross-product limit")
	}
	terms := make([]serviceTerm, 0, len(sourcePorts)*len(destinationPorts))
	for _, source := range sourcePorts {
		for _, destination := range destinationPorts {
			terms = append(terms, serviceTerm{protocol: ace.Protocol, source: source, destination: destination})
		}
	}
	return terms, unique(lines), nil
}

func addressObjectID(c *asa.ASAConfig, expr string) string {
	ref := strings.TrimPrefix(expr, "@")
	kind, name, ok := strings.Cut(ref, ":")
	if ok && (kind == "network_object" || kind == "network_group") {
		return kind + ":" + name
	}
	if o, found := c.Objects[ref]; found && o.Group {
		return "network_group:" + ref
	}
	return "network_object:" + ref
}

func serviceObjectIDs(ace asa.ConfigACE) []string {
	out := []string{}
	if ace.ServiceGroup != "" {
		out = append(out, "service_group:"+ace.ServiceGroup)
	}
	if ace.ServiceObject != "" {
		out = append(out, "service_object:"+ace.ServiceObject)
	}
	for _, v := range []string{ace.SourcePort, ace.DestinationPort} {
		if strings.HasPrefix(v, "@") {
			out = append(out, "service_group:"+strings.TrimPrefix(v, "@"))
		}
	}
	sort.Strings(out)
	return uniqueStrings(out)
}

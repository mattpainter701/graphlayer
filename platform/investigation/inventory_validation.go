package investigation

import (
	"errors"
	"net/netip"
	"strconv"
	"strings"

	"github.com/mattpainter701/graphlayer/platform/firewall"
)

func objectFamily(id string) string {
	kind, name, ok := strings.Cut(id, ":")
	if !ok || strings.TrimSpace(name) == "" {
		return ""
	}
	switch kind {
	case "network_object", "network_group":
		return "network"
	case "service_object", "service_group":
		return "service"
	}
	return ""
}

// validateInventory checks snapshot-local references before an inventory can
// be inspected or replayed. Missing dependencies remain explicit on unresolved
// objects/rules; they cannot make a resolved object appear complete.
func validateInventory(s *Snapshot) error {
	if s.ParserVersion == "asa-config-v1" {
		if len(s.Objects) != 0 || len(s.RuleOrigins) != 0 {
			return errors.New("legacy snapshot cannot claim a new object inventory")
		}
		return nil
	}
	if s.ParserVersion != firewall.ParserVersion {
		return errors.New("unsupported snapshot parser version")
	}
	evidence := map[string]bool{}
	for _, e := range s.Evidence {
		evidence[e.ID] = true
	}
	checkRefs := func(refs []string) bool {
		for _, ref := range refs {
			if !evidence[ref] {
				return false
			}
		}
		return true
	}
	objects := map[string]firewall.Object{}
	for _, object := range s.Objects {
		family := objectFamily(object.ID)
		if family == "" || object.ID != object.Kind+":"+object.Name || objects[object.ID].ID != "" || len(object.EvidenceRefs) == 0 || !checkRefs(object.EvidenceRefs) {
			return errors.New("invalid object identity or evidence")
		}
		if object.Resolved && len(object.Members) == 0 {
			return errors.New("resolved object has no members")
		}
		for _, member := range object.Members {
			if !checkRefs(member.EvidenceRefs) {
				return errors.New("missing member evidence")
			}
			switch member.Kind {
			case "reference":
				if objectFamily(member.ObjectID) != family || member.Value != "" || member.Protocol != "" || member.SourcePort != "" || member.DestinationPort != "" {
					return errors.New("object dependency crosses namespaces")
				}
			case "address":
				if family != "network" || !nativeAddressMember(member.Value) {
					return errors.New("invalid address member")
				}
			case "service":
				if family != "service" || (member.Protocol != "tcp" && member.Protocol != "udp" && member.Protocol != "tcp-udp") || !normalizedPort(member.SourcePort) || !normalizedPort(member.DestinationPort) {
					return errors.New("invalid service member")
				}
			case "unsupported":
				staticLabel := member.Value == "" || member.Value == "unsupported network member" || member.Value == "unsupported service object member" || member.Value == "unsupported service member"
				if object.Resolved || !staticLabel || member.ObjectID != "" || member.Protocol != "" || member.SourcePort != "" || member.DestinationPort != "" {
					return errors.New("unsupported member cannot be resolved")
				}
			default:
				return errors.New("unknown object member kind")
			}
		}
		objects[object.ID] = object
	}
	states, depths := map[string]int{}, map[string]int{}
	var visit func(string, int) (int, error)
	visit = func(id string, level int) (int, error) {
		if level > 16 {
			return 0, errors.New("object dependency depth exceeds the supported budget")
		}
		object, exists := objects[id]
		if !exists || !object.Resolved || states[id] == 1 {
			return 0, errors.New("invalid resolved object dependency")
		}
		if states[id] == 2 {
			if level+depths[id] > 16 {
				return 0, errors.New("object dependency depth exceeds the supported budget")
			}
			return depths[id], nil
		}
		states[id] = 1
		depth := 0
		for _, member := range object.Members {
			if member.Kind == "reference" {
				childDepth, err := visit(member.ObjectID, level+1)
				if err != nil {
					return 0, err
				}
				if childDepth+1 > depth {
					depth = childDepth + 1
				}
			}
		}
		if depth > 16 {
			return 0, errors.New("object dependency depth exceeds the supported budget")
		}
		states[id], depths[id] = 2, depth
		return depth, nil
	}
	for id, object := range objects {
		if object.Resolved {
			if _, err := visit(id, 0); err != nil {
				return err
			}
		}
	}
	rules := map[string]string{}
	for _, chain := range s.Chains {
		for _, rule := range chain.Rules {
			if _, exists := rules[rule.ID]; exists {
				return errors.New("rule identity reused across chains")
			}
			rules[rule.ID] = chain.Name
		}
	}
	origins := map[string]bool{}
	for _, origin := range s.RuleOrigins {
		acl, exists := rules[origin.RuleID]
		if !exists || origins[origin.RuleID] || acl != origin.ACLName || origin.Line < 1 || origin.Line > len(s.SourceLines) {
			return errors.New("invalid rule origin")
		}
		for _, id := range append(append([]string(nil), origin.SourceObjectIDs...), origin.DestinationObjectIDs...) {
			if objectFamily(id) != "network" {
				return errors.New("invalid address origin namespace")
			}
		}
		for _, id := range origin.ServiceObjectIDs {
			if objectFamily(id) != "service" {
				return errors.New("invalid service origin namespace")
			}
		}
		origins[origin.RuleID] = true
	}
	if len(origins) != len(rules) {
		return errors.New("rule origins do not cover the supplied chains")
	}
	return nil
}

func nativeAddressMember(value string) bool {
	if p, err := netip.ParsePrefix(value); err == nil {
		return p == p.Masked() && p.Addr().Zone() == "" && !p.Addr().Is4In6()
	}
	a, err := netip.ParseAddr(value)
	return err == nil && a.Zone() == "" && !a.Is4In6()
}

func normalizedPort(value string) bool {
	if value == "" {
		return true
	}
	parts := strings.Split(value, "-")
	if len(parts) > 2 {
		return false
	}
	lo, err := strconv.Atoi(parts[0])
	if err != nil || lo < 1 || lo > 65535 {
		return false
	}
	if len(parts) == 1 {
		return true
	}
	hi, err := strconv.Atoi(parts[1])
	return err == nil && hi >= lo && hi <= 65535
}

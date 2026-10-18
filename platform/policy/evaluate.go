package policy

import (
	"fmt"
	"net/netip"
	"sort"
	"strconv"
	"strings"
)

const maxRules = 10000
const ContractVersion = "graphlayer-policy-v3"

var supported = []string{"IPv4/IPv6 literal, CIDR and any/any4/any6 address matching", "numeric protocol matching (including TCP, UDP, ICMP, ICMPv6 aliases) and rule-only IP protocol wildcard", "numeric source/destination ports and inclusive ranges", "ordered first-match rules", "enabled state", "explicit complete-policy default action", "snapshot/device/context scope"}
var unsupportedDimensionLabels = map[string]bool{"application": true, "user": true, "schedule": true, "nat": true, "route": true, "return_path": true, "stateful": true, "zone_inheritance": true, "negation": true, "object_expansion": true, "vendor_default": true, "device_state": true}

func ValidateScope(s Scope) error {
	if strings.TrimSpace(s.SnapshotID) == "" || strings.TrimSpace(s.DeviceID) == "" || strings.TrimSpace(s.Context) == "" {
		return fmt.Errorf("snapshot_id, device_id, and context are required")
	}
	return nil
}

func Validate(b Bundle, f Flow) error {
	if err := ValidateScope(b.Scope); err != nil {
		return fmt.Errorf("bundle scope: %w", err)
	}
	if err := ValidateScope(f.Scope); err != nil {
		return fmt.Errorf("flow scope: %w", err)
	}
	if b.Scope != f.Scope {
		return fmt.Errorf("flow scope does not match bundle scope")
	}
	if len(b.Rules) > maxRules {
		return fmt.Errorf("rule count exceeds limit %d", maxRules)
	}
	if b.DefaultAction != "permit" && b.DefaultAction != "deny" {
		return fmt.Errorf("default_action must be permit or deny")
	}
	ids := map[string]bool{}
	positions := map[int]bool{}
	for i, r := range b.Rules {
		if strings.TrimSpace(r.ID) == "" {
			return fmt.Errorf("rule %d has empty id", i)
		}
		if ids[r.ID] {
			return fmt.Errorf("duplicate rule id %q", r.ID)
		}
		ids[r.ID] = true
		if r.Position < 0 || positions[r.Position] {
			return fmt.Errorf("invalid or duplicate position %d", r.Position)
		}
		positions[r.Position] = true
		if r.Action != "permit" && r.Action != "deny" {
			return fmt.Errorf("rule %q action must be permit or deny", r.ID)
		}
		if strings.TrimSpace(r.Source) == "" || strings.TrimSpace(r.Destination) == "" || strings.TrimSpace(r.Protocol) == "" {
			return fmt.Errorf("rule %q must specify source, destination, and protocol", r.ID)
		}
		if anyProtocol(r.Protocol) && (r.SourcePort != "" || r.DestinationPort != "") {
			return fmt.Errorf("rule %q cannot combine an IP protocol wildcard with port predicates", r.ID)
		}
		if p, err := parseProtocol(r.Protocol); err == nil && p != 6 && p != 17 && (r.SourcePort != "" || r.DestinationPort != "") {
			return fmt.Errorf("rule %q has port predicates on a non-TCP/UDP protocol", r.ID)
		}
		if _, err := parseProtocol(r.Protocol); err != nil && numericLike(r.Protocol) {
			return fmt.Errorf("rule %q protocol: %w", r.ID, err)
		}
		for _, p := range []string{r.Source, r.Destination} {
			if _, err := parseAddressPredicate(p); err != nil && (!knownUnsupported(p) || rejectedAddressForm(p)) {
				return fmt.Errorf("rule %q address predicate: %w", r.ID, err)
			}
		}
		for _, p := range []string{r.SourcePort, r.DestinationPort} {
			if p != "" {
				if _, err := parsePortPredicate(p); err != nil && numericLike(p) {
					return fmt.Errorf("rule %q port predicate: %w", r.ID, err)
				}
			}
		}
		for _, x := range r.Unsupported {
			if strings.TrimSpace(x) == "" {
				return fmt.Errorf("rule %q has empty unsupported dimension", r.ID)
			}
			if !unsupportedDimensionLabels[x] {
				return fmt.Errorf("rule %q has an unknown unsupported dimension label", r.ID)
			}
		}
	}
	addr1, e := netip.ParseAddr(f.Source)
	if e != nil {
		return fmt.Errorf("invalid flow source address")
	}
	addr2, e := netip.ParseAddr(f.Destination)
	if e != nil {
		return fmt.Errorf("invalid flow destination address")
	}
	if addr1.Is4() != addr2.Is4() {
		return fmt.Errorf("flow source and destination address families differ")
	}
	if addr1.Zone() != "" || addr2.Zone() != "" || addr1.Is4In6() || addr2.Is4In6() {
		return fmt.Errorf("flow addresses must be unscoped native IPv4 or IPv6 addresses")
	}
	proto, e := parseProtocol(f.Protocol)
	if e != nil {
		return fmt.Errorf("invalid flow protocol: %w", e)
	}
	if f.SourcePort < 0 || f.SourcePort > 65535 || f.DestinationPort < 0 || f.DestinationPort > 65535 {
		return fmt.Errorf("flow ports must be between 0 and 65535")
	}
	if proto == 6 || proto == 17 {
		if f.SourcePort == 0 || f.DestinationPort == 0 {
			return fmt.Errorf("TCP/UDP flow requires concrete nonzero source_port and destination_port")
		}
	}
	if proto != 6 && proto != 17 && (f.SourcePort != 0 || f.DestinationPort != 0) {
		return fmt.Errorf("non-TCP/UDP flow cannot carry port values")
	}
	if proto == 1 && !addr1.Is4() {
		return fmt.Errorf("ICMP flow requires IPv4 addresses")
	}
	if proto == 58 && !addr1.Is6() {
		return fmt.Errorf("ICMPv6 flow requires IPv6 addresses")
	}
	return nil
}

func Evaluate(b Bundle, f Flow) (Result, error) {
	if err := Validate(b, f); err != nil {
		return Result{}, err
	}
	rules := append([]Rule(nil), b.Rules...)
	sort.Slice(rules, func(i, j int) bool { return rules[i].Position < rules[j].Position })
	res := Result{EngineContractVersion: ContractVersion, Scope: b.Scope, Flow: f, Disposition: "indeterminate", Reasons: []string{}, UnsupportedDimensions: []string{}, Supported: append([]string(nil), supported...), Assumptions: []string{"policy-only; no route, NAT, return-path, application, user, schedule, or device-state conclusion"}}
	unknownEarlier := false
	for _, r := range rules {
		if !r.Enabled {
			continue
		}
		res.Considered = append(res.Considered, r.ID)
		m, predicates := traceRule(r, f)
		outcome := "matched"
		if m == noMatch {
			outcome = "not_matched"
		} else if m == unknown {
			outcome = "unresolved"
		}
		res.RuleTraces = append(res.RuleTraces, RuleTrace{RuleID: r.ID, Position: r.Position, Outcome: outcome, Predicates: predicates, EvidenceRefs: append([]string(nil), r.EvidenceRefs...)})
		if m == noMatch {
			continue
		}
		if m == unknown {
			unknownEarlier = true
			res.Reasons = append(res.Reasons, "rule "+r.ID+" may match but contains unsupported or unresolved semantics")
			res.EvidenceRefs = appendUnique(res.EvidenceRefs, r.EvidenceRefs...)
			res.UnsupportedDimensions = appendUnique(res.UnsupportedDimensions, ruleUnsupportedDimensions(r)...)
			continue
		}
		if unknownEarlier {
			res.Reasons = append(res.Reasons, "an earlier candidate rule has unresolved semantics")
			return res, nil
		}
		res.EvidenceRefs = appendUnique(res.EvidenceRefs, r.EvidenceRefs...)
		if !b.Complete {
			res.CandidateRule = r.ID
			res.Reasons = append(res.Reasons, "policy bundle is incomplete; omitted earlier rules may change this candidate result")
			return res, nil
		}
		res.Disposition = r.Action
		res.DecisiveRule = r.ID
		res.Reasons = append(res.Reasons, "first matching enabled rule")
		return res, nil
	}
	if unknownEarlier {
		return res, nil
	}
	if !b.Complete {
		res.Reasons = append(res.Reasons, "policy bundle is incomplete; default action is not established")
		return res, nil
	}
	res.Disposition = b.DefaultAction
	res.UsedDefault = true
	res.Reasons = append(res.Reasons, "complete policy bundle explicit default action")
	return res, nil
}

type match int

const (
	noMatch match = iota
	yesMatch
	unknown
)

// traceRule applies the evaluator's conservative combination rules while
// retaining the status of each predicate. A known mismatch takes precedence
// over an unresolved predicate, just as it does in the verdict evaluator.
func traceRule(r Rule, f Flow) (match, []PredicateTrace) {
	traces := make([]PredicateTrace, 0, 5+len(r.Unsupported))
	add := func(dimension string, state match, reason string) {
		name := "match"
		if state == noMatch {
			name = "not_match"
		} else if state == unknown {
			name = "unresolved"
		}
		traces = append(traces, PredicateTrace{Dimension: dimension, State: name, Reason: reason})
	}

	for _, item := range []struct{ dimension, predicate, value string }{
		{"source", r.Source, f.Source},
		{"destination", r.Destination, f.Destination},
	} {
		state := matchAddress(item.predicate, item.value)
		reason := "address predicate matches flow address"
		if strings.TrimSpace(item.predicate) == "" {
			reason = "no address predicate; dimension is unconstrained"
		} else if state == noMatch {
			reason = "address predicate excludes flow address"
		} else if state == unknown {
			reason = "address predicate cannot be resolved by the supported matcher"
		}
		add(item.dimension, state, reason)
	}

	p1, e1 := parseProtocol(r.Protocol)
	p2, e2 := parseProtocol(f.Protocol)
	protocolState := yesMatch
	protocolReason := "protocol predicate matches flow protocol"
	if anyProtocol(r.Protocol) {
		protocolReason = "IP protocol wildcard matches flow protocol"
	} else if e1 != nil {
		protocolState = unknown
		protocolReason = "protocol predicate cannot be resolved by the supported matcher"
	} else if e2 != nil {
		protocolState = unknown
		protocolReason = "flow protocol cannot be resolved by the supported matcher"
	} else if p1 != p2 {
		protocolState = noMatch
		protocolReason = "protocol predicate excludes flow protocol"
	}
	add("protocol", protocolState, protocolReason)

	flowHasPorts := e2 == nil && (p2 == 6 || p2 == 17)
	for _, item := range []struct {
		dimension string
		predicate string
		value     int
	}{{"source_port", r.SourcePort, f.SourcePort}, {"destination_port", r.DestinationPort, f.DestinationPort}} {
		state := yesMatch
		reason := "no port predicate; dimension is unconstrained"
		if item.predicate != "" {
			if !flowHasPorts {
				state = unknown
				reason = "port predicate has no defined semantics for this flow protocol"
			} else if ports, err := parsePortPredicate(item.predicate); err != nil {
				state = unknown
				reason = "port predicate cannot be resolved by the supported matcher"
			} else if item.value < ports[0] || item.value > ports[1] {
				state = noMatch
				reason = "port predicate excludes flow port"
			} else {
				reason = "port predicate matches flow port"
			}
		}
		add(item.dimension, state, reason)
	}

	for _, dimension := range r.Unsupported {
		add(dimension, unknown, "rule uses an unsupported policy dimension")
	}

	result := yesMatch
	for _, trace := range traces {
		if trace.State == "not_match" {
			return noMatch, traces
		}
		if trace.State == "unresolved" {
			result = unknown
		}
	}
	return result, traces
}

func ruleUnsupportedDimensions(r Rule) []string {
	dims := append([]string(nil), r.Unsupported...)
	if _, err := parseAddressPredicate(r.Source); err != nil {
		dims = append(dims, "source address expression")
	}
	if _, err := parseAddressPredicate(r.Destination); err != nil {
		dims = append(dims, "destination address expression")
	}
	if _, err := parseProtocol(r.Protocol); err != nil && !anyProtocol(r.Protocol) {
		dims = append(dims, "protocol expression")
	}
	if r.SourcePort != "" {
		if _, err := parsePortPredicate(r.SourcePort); err != nil {
			dims = append(dims, "source port expression")
		}
	}
	if r.DestinationPort != "" {
		if _, err := parsePortPredicate(r.DestinationPort); err != nil {
			dims = append(dims, "destination port expression")
		}
	}
	return dims
}
func appendUnique(dst []string, values ...string) []string {
	seen := map[string]bool{}
	for _, v := range dst {
		seen[v] = true
	}
	for _, v := range values {
		if !seen[v] {
			dst = append(dst, v)
			seen[v] = true
		}
	}
	sort.Strings(dst)
	return dst
}
func matchAddress(pred, value string) match {
	ranges, e := parseAddressPredicate(pred)
	if e != nil {
		return unknown
	}
	if ranges == nil {
		return yesMatch
	}
	a, e := netip.ParseAddr(value)
	if e != nil {
		return noMatch
	}
	for _, p := range ranges {
		if p.Contains(a) {
			return yesMatch
		}
	}
	return noMatch
}
func parseAddressPredicate(s string) ([]netip.Prefix, error) {
	s = strings.TrimSpace(s)
	if strings.EqualFold(s, "any") {
		return nil, nil
	}
	if strings.EqualFold(s, "any4") {
		return []netip.Prefix{netip.MustParsePrefix("0.0.0.0/0")}, nil
	}
	if strings.EqualFold(s, "any6") {
		return []netip.Prefix{netip.MustParsePrefix("::/0")}, nil
	}
	if p, e := netip.ParsePrefix(s); e == nil {
		if p.Addr().Zone() != "" || p.Addr().Is4In6() {
			return nil, fmt.Errorf("scoped or IPv4-mapped addresses are unsupported")
		}
		return []netip.Prefix{p.Masked()}, nil
	}
	if a, e := netip.ParseAddr(s); e == nil {
		if a.Zone() != "" || a.Is4In6() {
			return nil, fmt.Errorf("scoped or IPv4-mapped addresses are unsupported")
		}
		bits := 128
		if a.Is4() {
			bits = 32
		}
		return []netip.Prefix{netip.PrefixFrom(a, bits)}, nil
	}
	return nil, fmt.Errorf("unsupported address expression")
}
func rejectedAddressForm(s string) bool {
	if p, e := netip.ParsePrefix(strings.TrimSpace(s)); e == nil {
		return p.Addr().Zone() != "" || p.Addr().Is4In6()
	}
	if a, e := netip.ParseAddr(strings.TrimSpace(s)); e == nil {
		return a.Zone() != "" || a.Is4In6()
	}
	return false
}
func anyProtocol(s string) bool {
	return strings.EqualFold(strings.TrimSpace(s), "any") || strings.EqualFold(strings.TrimSpace(s), "ip")
}

func parseProtocol(s string) (int, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "tcp":
		return 6, nil
	case "udp":
		return 17, nil
	case "icmp":
		return 1, nil
	case "icmpv6":
		return 58, nil
	}
	n, e := strconv.Atoi(s)
	if e != nil || n < 0 || n > 255 {
		return 0, fmt.Errorf("protocol must be numeric 0..255 or a supported alias")
	}
	return n, nil
}
func parsePortPredicate(s string) ([2]int, error) {
	var out [2]int
	parts := strings.Split(s, "-")
	if len(parts) > 2 {
		return out, fmt.Errorf("port must be a number or range")
	}
	lo, e := strconv.Atoi(parts[0])
	if e != nil || lo < 1 || lo > 65535 {
		return out, fmt.Errorf("port must be 1..65535")
	}
	hi := lo
	if len(parts) == 2 {
		hi, e = strconv.Atoi(parts[1])
		if e != nil || hi < lo || hi > 65535 {
			return out, fmt.Errorf("invalid port range")
		}
	}
	return [2]int{lo, hi}, nil
}
func knownUnsupported(s string) bool {
	return strings.TrimSpace(s) != "" && strings.ToLower(strings.TrimSpace(s)) != "any"
}
func numericLike(s string) bool {
	_, err := strconv.Atoi(strings.TrimSpace(s))
	return err == nil || strings.TrimSpace(s) == "" || strings.TrimSpace(s)[0] >= '0' && strings.TrimSpace(s)[0] <= '9'
}

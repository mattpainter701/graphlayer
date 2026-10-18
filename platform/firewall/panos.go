package firewall

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"net"
	"strconv"
	"strings"
	"unicode"

	"github.com/mattpainter701/graphlayer/platform/policy"
)

const PANOSParserVersion = "panos-config-v1"
const panosPathVersion = "panos-path-v1"

//go:embed testdata/panos_sample.txt
var panosSample embed.FS

func SamplePANOS() []byte {
	b, _ := panosSample.ReadFile("testdata/panos_sample.txt")
	return []byte(strings.ReplaceAll(string(b), "\r\n", "\n"))
}

type panLine struct {
	n int
	t []string
}
type panAddr struct {
	prefix string
	line   int
}
type panSvc struct {
	proto string
	port  string
	line  int
}
type panRule struct {
	name                               string
	line                               int
	from, to, src, dst, apps, services []string
	action                             string
	disabled                           bool
	srcX, dstX                         string
	translated                         string
	static                             bool
	hasSrc, hasDst, hasService         bool
	unsupported                        bool
	lines                              []int
}

func panTokenize(s string) ([]string, error) {
	var out []string
	var b strings.Builder
	quoted, esc, active := false, false, false
	for _, r := range s {
		if esc {
			b.WriteRune(r)
			esc = false
			active = true
			continue
		}
		if quoted && r == '\\' {
			esc = true
			continue
		}
		if r == '"' {
			quoted = !quoted
			active = true
			continue
		}
		if !quoted && unicode.IsSpace(r) {
			if active {
				out = append(out, b.String())
				b.Reset()
				active = false
			}
			continue
		}
		b.WriteRune(r)
		active = true
	}
	if quoted || esc {
		return nil, fmt.Errorf("malformed quoted SET command")
	}
	if active {
		out = append(out, b.String())
	}
	// Native SET exports use bracketed lists. Remove only balanced standalone
	// delimiters so every configured list member reaches bounded expansion.
	values := make([]string, 0, len(out))
	list := false
	for _, value := range out {
		switch value {
		case "[":
			if list {
				return nil, fmt.Errorf("nested SET list")
			}
			list = true
		case "]":
			if !list {
				return nil, fmt.Errorf("unbalanced SET list")
			}
			list = false
		default:
			values = append(values, value)
		}
	}
	if list {
		return nil, fmt.Errorf("unbalanced SET list")
	}
	return values, nil
}
func panRef(hash string, lines ...int) string {
	h := sha256.New()
	h.Write([]byte(hash))
	for _, n := range lines {
		fmt.Fprintf(h, ":%d", n)
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}
func panPrefix(s string) (string, error) {
	if ip, netw, e := net.ParseCIDR(s); e == nil {
		ones, _ := netw.Mask.Size()
		return fmt.Sprintf("%s/%d", ip.Mask(netw.Mask), ones), nil
	}
	if ip := net.ParseIP(s); ip != nil {
		if ip.To4() != nil {
			return ip.String() + "/32", nil
		}
		return ip.String() + "/128", nil
	}
	return "", fmt.Errorf("invalid IPv4/IPv6 prefix")
}
func panAddressMask(s string) (string, error) {
	if ip, n, e := net.ParseCIDR(s); e == nil {
		bits, _ := n.Mask.Size()
		return fmt.Sprintf("%s/%d", ip, bits), nil
	}
	if ip := net.ParseIP(s); ip != nil {
		if ip.To4() != nil {
			return ip.String() + "/32", nil
		}
		return ip.String() + "/128", nil
	}
	return "", fmt.Errorf("invalid IP address mask")
}
func panSafe(s string) bool {
	if len(s) == 0 || len(s) > 128 {
		return false
	}
	for _, r := range s {
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune("_.-:/", r)) {
			return false
		}
	}
	return true
}

// ParsePANOSConfig accepts a bounded PAN-OS native SET-format single-vsys export.
func ParsePANOSConfig(input []byte) (*Config, error) {
	if len(input) > 2<<20 {
		return nil, fmt.Errorf("PAN-OS config exceeds 2 MiB limit")
	}
	raw := strings.ReplaceAll(string(input), "\r\n", "\n")
	raw = strings.ReplaceAll(raw, "\r", "\n")
	lines := strings.Split(raw, "\n")
	if len(lines) > 50001 {
		return nil, fmt.Errorf("PAN-OS config exceeds 50,000 line limit")
	}
	parsed := make([]panLine, 0, len(lines))
	for i, s := range lines {
		t, e := panTokenize(strings.TrimSpace(s))
		if e != nil {
			return nil, fmt.Errorf("line %d: %w", i+1, e)
		}
		if len(t) == 0 {
			continue
		}
		if t[0] != "set" {
			if strings.HasPrefix(strings.TrimSpace(s), "#") {
				continue
			}
			return nil, fmt.Errorf("line %d: expected native SET command", i+1)
		}
		if len(t) < 2 {
			return nil, fmt.Errorf("line %d: incomplete SET command", i+1)
		}
		parsed = append(parsed, panLine{i + 1, t[1:]})
	}
	h := sha256.Sum256(input)
	hash := hex.EncodeToString(h[:])
	c := &Config{Vendor: "paloalto-panos", ParserVersion: PANOSParserVersion, Context: "vsys1", ArtifactSHA256: hash, SourceLines: make([]SourceLine, len(lines)), Evidence: []Evidence{}, Warnings: []Warning{}, Chains: []Chain{}, Capabilities: []string{"PAN-OS SET format", "IPv4 static and connected routes", "ordered security policy", "bounded static NAT"}}
	for i := range lines {
		c.SourceLines[i] = SourceLine{Number: i + 1, Text: "[OMITTED NON-POLICY CONFIGURATION]"}
	}
	p := &PathConfig{Version: panosPathVersion, Interfaces: []PathInterface{}, Routes: []PathRoute{}, NATRules: []PathNATRule{}, Policies: []PathPolicyRule{}, Translations: []PathTranslation{}, RoutingComplete: true, NATComplete: true, PolicyComplete: true, InterInterface: false, Warnings: []Warning{}}
	addresses := map[string]panAddr{}
	invalidAddresses := map[string]bool{}
	groups := map[string][]string{}
	groupLines := map[string][]int{}
	services := map[string]panSvc{}
	zones := map[string][]string{}
	zoneLines := map[string][]int{}
	interfaceLines := map[string][]int{}
	vrLines := map[string][]int{}
	interfaces := map[string]PathInterface{}
	ifIPs := map[string][]string{}
	vrInterfaces := map[string][]string{}
	vrNames := map[string]bool{}
	statics := []struct {
		name, dest, next, iface string
		line                    int
		lines                   []int
	}{}
	rules := []panRule{}
	nat := []panRule{}
	hostname := ""
	unsafeScope := false
	unsupported := func(code, msg string, n int) {
		p.Warnings = append(p.Warnings, Warning{code, msg, n})
		c.Warnings = append(c.Warnings, Warning{code, msg, n})
		p.PolicyComplete = false
	}
	for _, l := range parsed {
		t := l.t
		low := strings.ToLower(strings.Join(t, " "))
		if len(t) >= 4 && t[0] == "deviceconfig" && t[1] == "system" && t[2] == "hostname" {
			if panSafe(t[3]) {
				hostname = t[3]
			} else {
				hostname = "panos-device"
			}
			continue
		}
		if len(t) >= 4 && t[0] == "address" {
			name := t[1]
			if !panSafe(name) || len(name) > 128 {
				unsupported("unsupported_address", "Address object name is invalid", l.n)
			} else if len(t) == 4 && t[2] == "ip-netmask" {
				if x, e := panPrefix(t[3]); e == nil {
					if prior, ok := addresses[name]; ok && prior.prefix != x {
						delete(addresses, name)
						invalidAddresses[name] = true
						unsupported("conflicting_address_object", "Address object is redefined with a different value", l.n)
					} else if !invalidAddresses[name] {
						if _, ok := addresses[name]; !ok {
							addresses[name] = panAddr{x, l.n}
						}
					}
				} else {
					unsupported("unsupported_address", "Address object could not be normalized", l.n)
				}
			} else {
				unsupported("unsupported_address", "Unsupported address object syntax", l.n)
			}
			continue
		}
		if len(t) >= 4 && t[0] == "address-group" {
			name := t[1]
			members := panNames(t[3:])
			validMembers := len(members) > 0 && len(members) <= 256
			for _, member := range members {
				validMembers = validMembers && panSafe(member)
			}
			if !panSafe(name) || !validMembers {
				unsupported("unsupported_address_group", "Address group name or member is invalid", l.n)
			} else if t[2] == "static" {
				groups[name] = append(groups[name], members...)
				groupLines[name] = append(groupLines[name], l.n)
			} else {
				unsupported("unsupported_address_group", "Only static address groups are supported", l.n)
			}
			continue
		}
		if len(t) >= 5 && t[0] == "service" && t[2] == "protocol" && (t[3] == "tcp" || t[3] == "udp") && t[4] == "port" {
			if len(t) == 6 && panSafe(t[1]) && len(t[1]) <= 128 {
				services[t[1]] = panSvc{t[3], t[5], l.n}
			} else {
				unsupported("unsupported_service", "Unsupported custom service syntax", l.n)
			}
			continue
		}
		if t[0] == "service" {
			unsupported("unsupported_service", "Custom service has unmodeled matching controls", l.n)
			continue
		}
		if len(t) == 5 && t[0] == "zone" && t[2] == "network" && t[3] == "layer3" {
			if panSafe(t[1]) && len(t[1]) <= 64 && panSafe(t[4]) && len(t[4]) <= 48 {
				zones[t[1]] = append(zones[t[1]], t[4])
			} else {
				unsupported("unsupported_zone", "Zone or interface name is invalid", l.n)
			}
			zoneLines[t[4]] = append(zoneLines[t[4]], l.n)
			continue
		}
		if len(t) >= 6 && t[0] == "network" && t[1] == "interface" && t[2] == "ethernet" {
			name := t[3]
			if !panSafe(name) || len(name) > 48 {
				unsupported("unsupported_interface", "Interface name is invalid", l.n)
				continue
			}
			in := interfaces[name]
			in.Name = name
			in.Physical = name
			in.Resolved = true
			if t[4] == "layer3" && t[5] == "ip" && len(t) == 7 {
				if x, e := panAddressMask(t[6]); e == nil {
					ifIPs[name] = append(ifIPs[name], x)
				} else {
					unsupported("unsupported_interface_ip", "Invalid interface address", l.n)
				}
				interfaceLines[name] = append(interfaceLines[name], l.n)
			} else if t[4] == "link-state" && len(t) == 6 && (t[5] == "down" || t[5] == "up") {
				in.Shutdown = t[5] == "down"
				interfaceLines[name] = append(interfaceLines[name], l.n)
			} else {
				in.Resolved = false
				p.RoutingComplete = false
				unsupported("unsupported_interface_control", "Interface forwarding control is outside the supported subset", l.n)
			}
			interfaces[name] = in
			continue
		}
		if len(t) >= 5 && t[0] == "network" && t[1] == "virtual-router" {
			vr := t[2]
			if !panSafe(vr) || len(vr) > 128 {
				unsupported("unsupported_virtual_router", "Virtual router name is invalid", l.n)
				p.RoutingComplete = false
				continue
			}
			vrNames[vr] = true
			if t[3] == "interface" && len(t) == 5 {
				vrInterfaces[vr] = append(vrInterfaces[vr], t[4])
				vrLines[t[4]] = append(vrLines[t[4]], l.n)
				continue
			}
			if t[3] == "routing-table" && t[4] == "ip" && len(t) >= 8 && t[5] == "static-route" {
				name := t[6]
				if t[7] == "destination" && len(t) == 9 && panSafe(name) && len(name) <= 128 {
					statics = append(statics, struct {
						name, dest, next, iface string
						line                    int
						lines                   []int
					}{name, t[8], "", "", l.n, []int{l.n}})
				} else if t[7] == "interface" && len(t) == 9 && panSafe(t[8]) && len(t[8]) <= 128 && len(statics) > 0 && statics[len(statics)-1].name == name {
					statics[len(statics)-1].iface = t[8]
					statics[len(statics)-1].lines = append(statics[len(statics)-1].lines, l.n)
				} else if t[7] == "nexthop" && len(t) == 10 && t[8] == "ip-address" && net.ParseIP(t[9]) != nil && len(statics) > 0 && statics[len(statics)-1].name == name {
					statics[len(statics)-1].next = net.ParseIP(t[9]).String()
					statics[len(statics)-1].lines = append(statics[len(statics)-1].lines, l.n)
				} else {
					unsupported("unsupported_route", "Unsupported static route control", l.n)
				}
				continue
			}
			if strings.Contains(low, "protocol") || strings.Contains(low, "redistribution") || strings.Contains(low, "ecmp") {
				unsupported("unsupported_dynamic_route", "Dynamic or advanced routing control is not modeled", l.n)
			}
			if strings.Contains(low, "pbf") || strings.Contains(low, "path-monitor") {
				unsafeScope = true
				unsupported("unsupported_pbf", "Policy-based forwarding is outside the modeled routing subset", l.n)
			}
			continue
		}
		if len(t) >= 5 && t[0] == "rulebase" && t[1] == "security" && t[2] == "rules" {
			if !panSafe(t[3]) || len(t[3]) > 128 {
				unsupported("unsupported_policy_name", "Security rule name is invalid", l.n)
				continue
			}
			r := getPanRule(&rules, t[3], l.n)
			parsePanRule(r, t[4:], l.n, unsupported, false)
			continue
		}
		if len(t) >= 5 && t[0] == "rulebase" && t[1] == "nat" && t[2] == "rules" {
			if !panSafe(t[3]) || len(t[3]) > 128 {
				p.NATComplete = false
				unsupported("unsupported_nat_name", "NAT rule name is invalid", l.n)
				continue
			}
			r := getPanRule(&nat, t[3], l.n)
			parsePanRule(r, t[4:], l.n, func(code, msg string, n int) { p.NATComplete = false; unsupported(code, msg, n) }, true)
			continue
		}
		if t[0] == "shared" || strings.Contains(low, "vsys") || strings.Contains(low, "panorama") || strings.Contains(low, "global") || strings.Contains(low, "pbf") || strings.Contains(low, "pre-rulebase") || strings.Contains(low, "post-rulebase") || strings.Contains(low, "rulebase") || strings.Contains(low, "virtual-router") || strings.Contains(low, "translation") {
			if strings.Contains(low, "vsys") || strings.Contains(low, "panorama") || strings.Contains(low, "global") || strings.Contains(low, "pbf") || t[0] == "shared" || (t[0] == "rulebase" && (len(t) < 2 || t[1] != "security" && t[1] != "nat")) {
				unsafeScope = true
			}
			if t[0] == "rulebase" && (len(t) < 3 || (t[1] == "security" && t[2] != "rules") || (t[1] == "nat" && t[2] != "rules")) {
				unsafeScope = true
			}
			unsupported("unsupported_control", "Relevant PAN-OS control is outside the supported SET subset", l.n)
		}
	}
	for _, l := range parsed {
		if safe := safePANOSSourceCommand(l.t); safe != "" {
			c.SourceLines[l.n-1].Text = safe
		}
	}
	if len(rules) > 4096 || len(nat) > 4096 || len(addresses) > 4096 || len(services) > 4096 || len(groups) > 4096 {
		return nil, fmt.Errorf("PAN-OS object or policy count limit exceeded")
	}
	if len(vrNames) != 1 {
		p.RoutingComplete = false
		unsupported("ambiguous_virtual_router", "Exactly one virtual router is required", 0)
	}
	for name, in := range interfaces {
		if len(ifIPs[name]) > 1 {
			unsupported("multiple_interface_ips", "Multiple interface addresses are unsupported", 0)
		}
		if len(ifIPs[name]) == 1 {
			in.Address = ifIPs[name][0]
		}
		in.Zone = ""
		for z, mem := range zones {
			for _, m := range mem {
				if m == name {
					if in.Zone != "" {
						unsupported("multiple_interface_zones", "Interface is in multiple zones", 0)
					}
					in.Zone = z
				}
			}
		}
		interfaces[name] = in
		refs := []string{}
		for _, n := range append(append([]int{}, interfaceLines[name]...), zoneLines[name]...) {
			refs = append(refs, addPanEvidence(c, hash, n, "interface", "Interface address or zone membership"))
		}
		for _, n := range vrLines[name] {
			refs = append(refs, addPanEvidence(c, hash, n, "virtual_router_membership", "Virtual router interface membership"))
		}
		in.EvidenceRefs = refs
		interfaces[name] = in
	}
	if len(interfaces) > 1024 {
		return nil, fmt.Errorf("PAN-OS interface limit exceeded")
	}
	for _, in := range interfaces {
		p.Interfaces = append(p.Interfaces, in)
	}
	memberInterfaces := map[string]bool{}
	if len(vrNames) == 1 {
		vr := ""
		for n := range vrNames {
			vr = n
		}
		for _, ifn := range vrInterfaces[vr] {
			memberInterfaces[ifn] = true
			in, ok := interfaces[ifn]
			if !ok || in.Address == "" || !in.Resolved || in.Shutdown {
				p.RoutingComplete = false
				unsupported("unresolved_virtual_router_interface", "Virtual router interface lacks a modeled IPv4 address", 0)
			}
		}
	}
	for name, in := range interfaces {
		if in.Address != "" && memberInterfaces[name] && !in.Shutdown && in.Resolved {
			prefix, _ := netPrefix(in.Address)
			p.Routes = append(p.Routes, PathRoute{ID: "connected:" + name + ":" + prefix, Kind: "connected", Prefix: prefix, Interface: name, Distance: 0, EvidenceRefs: append([]string(nil), in.EvidenceRefs...)})
		}
	}
	if len(statics)+len(p.Routes) > 4096 {
		return nil, fmt.Errorf("PAN-OS route limit exceeded")
	}
	for _, r := range statics {
		dst, e := panPrefix(r.dest)
		if e != nil || r.next == "" || r.iface == "" {
			p.RoutingComplete = false
			unsupported("incomplete_static_route", "Static route is missing a valid destination, next hop, or interface", r.line)
			continue
		}
		refs := panRuleEvidence(c, hash, r.lines, "route", "Static route")
		p.Routes = append(p.Routes, PathRoute{ID: "static:" + r.name, Kind: "static", Prefix: dst, Interface: r.iface, NextHop: r.next, Distance: 10, EvidenceRefs: refs})
	}
	for _, r := range rules {
		if unsafeScope {
			continue
		}
		if len(p.Policies) >= 4096 {
			return nil, fmt.Errorf("PAN-OS security rule limit exceeded")
		}
		if !panZonesKnown(r.from, zones) || !panZonesKnown(r.to, zones) {
			unsupported("unknown_policy_zone", "Security rule references an unknown zone", r.line)
			continue
		}
		if e := expandPanSecurity(r, addresses, groups, groupLines, services, p, c, hash); e != nil {
			unsupported("unsupported_policy_expansion", e.Error(), r.line)
		}
	}
	for _, r := range nat {
		if unsafeScope {
			continue
		}
		if len(p.Translations) >= 4096 {
			return nil, fmt.Errorf("PAN-OS NAT rule limit exceeded")
		}
		if !panZonesKnown(r.from, zones) || !panZonesKnown(r.to, zones) {
			p.NATComplete = false
			unsupported("unknown_nat_zone", "NAT rule references an unknown zone", r.line)
			continue
		}
		if e := expandPanNAT(r, addresses, groups, groupLines, services, p, c, hash); e != nil {
			p.NATComplete = false
			unsupported("unsupported_nat", e.Error(), r.line)
		}
	}
	for i := range p.Policies {
		pp := p.Policies[i]
		rr := pp.Match
		ch := Chain{ID: "panos-policy:" + pp.ID, Name: pp.ID, Interface: "", Direction: "zone", Complete: false, Rules: []policy.Rule{rr}, EvidenceRefs: append([]string(nil), rr.EvidenceRefs...), Warnings: []Warning{{"zone_context_required", "Zone context is required to evaluate this rule", 0}}}
		c.Chains = append(c.Chains, ch)
		c.RuleOrigins = append(c.RuleOrigins, RuleOrigin{RuleID: rr.ID, ACLName: pp.ID, Line: sourceLineForRef(c, rr.EvidenceRefs)})
	}
	for _, w := range p.Warnings {
		if w.Line > 0 && w.Line <= len(c.SourceLines) {
			if strings.HasPrefix(c.SourceLines[w.Line-1].Text, "[OMITTED") {
				c.SourceLines[w.Line-1].Text = "[UNSUPPORTED PATH CONFIGURATION]"
			}
			p.EvidenceRefs = append(p.EvidenceRefs, addPanEvidence(c, hash, w.Line, "unsupported_path_input", w.Message))
		}
		if w.Code == "unsupported_control" || w.Code == "unsupported_dynamic_route" || w.Code == "unsupported_nat" || w.Code == "unsupported_address" || w.Code == "unsupported_service" {
			p.RoutingComplete = false
			p.NATComplete = false
		}
	}
	p.Warnings = dedupePanWarnings(p.Warnings)
	c.Warnings = dedupePanWarnings(c.Warnings)
	if len(c.Chains) == 0 && len(rules) > 0 {
		c.Chains = append(c.Chains, Chain{ID: "panos:policies", Name: "PAN-OS unsupported policy inventory", Direction: "zone", Complete: false, Rules: []policy.Rule{}, EvidenceRefs: append([]string(nil), p.EvidenceRefs...), Warnings: append([]Warning(nil), p.Warnings...)})
	}
	c.Path = p
	if hostname != "" {
		c.Hostname = hostname
	}
	return c, nil
}

func netPrefix(s string) (string, bool) {
	_, n, e := net.ParseCIDR(s)
	if e != nil {
		return "", false
	}
	return n.String(), true
}
func getPanRule(rs *[]panRule, name string, line int) *panRule {
	for i := range *rs {
		if (*rs)[i].name == name {
			return &(*rs)[i]
		}
	}
	*rs = append(*rs, panRule{name: name, line: line})
	return &(*rs)[len(*rs)-1]
}
func parsePanRule(r *panRule, t []string, line int, warn func(string, string, int), isNAT bool) {
	r.lines = append(r.lines, line)
	if line < r.line {
		r.line = line
	}
	if len(t) == 0 {
		return
	}
	k := t[0]
	vals := t[1:]
	switch k {
	case "from":
		r.from = append(r.from, vals...)
	case "to":
		r.to = append(r.to, vals...)
	case "source":
		r.src = append(r.src, vals...)
		r.hasSrc = true
	case "destination":
		r.dst = append(r.dst, vals...)
		r.hasDst = true
	case "service":
		r.services = append(r.services, vals...)
		r.hasService = true
	case "application":
		r.apps = append(r.apps, vals...)
		if !isNAT && strings.Join(vals, " ") != "any" {
			r.unsupported = true
			warn("unsupported_application", "Application-ID semantics are unsupported", line)
		}
	case "action":
		if len(vals) == 1 {
			r.action = vals[0]
		} else {
			r.unsupported = true
			warn("unsupported_rule_control", "Invalid policy action", line)
		}
	case "disabled":
		if len(vals) != 1 || (vals[0] != "yes" && vals[0] != "no") {
			r.unsupported = true
			warn("unsupported_rule_control", "Invalid rule enabled state", line)
		} else {
			r.disabled = vals[0] == "yes"
		}
	case "source-translation":
		if len(vals) == 3 && vals[0] == "static-ip" && vals[1] == "translated-address" {
			r.srcX = vals[2]
			r.static = true
		} else if strings.Join(vals, " ") != "none" {
			r.unsupported = true
			warn("unsupported_nat_mode", "Dynamic source NAT is unsupported", line)
		}
	case "destination-translation":
		if len(vals) == 2 && vals[0] == "translated-address" {
			r.dstX = vals[1]
		} else {
			r.unsupported = true
			warn("unsupported_nat_mode", "Unsupported destination NAT mode", line)
		}
	default:
		r.unsupported = true
		warn("unsupported_rule_control", "Unsupported policy rule control", line)
	}
}
func addPanEvidence(c *Config, h string, n int, kind, summary string) string {
	ref := panRef(h, n)
	for _, e := range c.Evidence {
		if e.ID == ref {
			return ref
		}
	}
	c.Evidence = append(c.Evidence, Evidence{ID: ref, StartLine: n, EndLine: n, Kind: kind, Summary: summary})
	return ref
}
func resolvePanName(n string, addr map[string]panAddr, groups map[string][]string, depth int) ([]panAddr, error) {
	if n == "any" {
		return []panAddr{{"0.0.0.0/0", 0}}, nil
	}
	if prefix, err := panPrefix(n); err == nil {
		return []panAddr{{prefix, 0}}, nil
	}
	if a, ok := addr[n]; ok {
		return []panAddr{a}, nil
	}
	if depth > 8 {
		return nil, fmt.Errorf("address group nesting limit exceeded")
	}
	m, ok := groups[n]
	if !ok {
		return nil, fmt.Errorf("unresolved address object")
	}
	out := []panAddr{}
	for _, x := range m {
		v, e := resolvePanName(x, addr, groups, depth+1)
		if e != nil {
			return nil, e
		}
		out = append(out, v...)
		if len(out) > 256 {
			return nil, fmt.Errorf("address group expansion exceeds 256")
		}
	}
	return out, nil
}
func panService(n string, sv map[string]panSvc) (string, string, error) {
	if n == "any" {
		return "ip", "", nil
	}
	s, ok := sv[n]
	if !ok {
		return "", "", fmt.Errorf("unresolved service")
	}
	if strings.Contains(s.port, "-") {
		p := strings.Split(s.port, "-")
		a, e := strconv.Atoi(p[0])
		if e != nil {
			return "", "", e
		}
		b, e := strconv.Atoi(p[1])
		if e != nil || b < a {
			return "", "", fmt.Errorf("invalid service port range")
		}
		return s.proto, fmt.Sprintf("%d-%d", a, b), nil
	}
	nport, e := strconv.Atoi(s.port)
	if e != nil || nport < 1 || nport > 65535 {
		return "", "", fmt.Errorf("invalid service port")
	}
	return s.proto, strconv.Itoa(nport), nil
}
func panNames(v []string) []string {
	if len(v) == 0 {
		return []string{"any"}
	}
	out := make([]string, 0, len(v))
	seen := map[string]bool{}
	for _, x := range v {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}
func expandPanSecurity(r panRule, addr map[string]panAddr, groups map[string][]string, groupLines map[string][]int, sv map[string]panSvc, p *PathConfig, c *Config, h string) error {
	if r.unsupported {
		return fmt.Errorf("rule contains unsupported policy semantics")
	}
	if !r.hasSrc || !r.hasDst || !r.hasService || len(r.src) == 0 || len(r.dst) == 0 || len(r.services) == 0 || len(r.apps) != 1 || r.apps[0] != "any" || len(r.from) == 0 || len(r.to) == 0 || r.action == "" {
		return fmt.Errorf("security rule is missing a required explicit match field")
	}
	if r.action == "" {
		return fmt.Errorf("security rule has no supported action")
	}
	action := strings.ToLower(r.action)
	if action == "allow" {
		action = "permit"
	}
	if action == "drop" {
		action = "deny"
	}
	if action != "permit" && action != "deny" {
		return fmt.Errorf("unsupported security action")
	}
	src, e := resolvePanList(panNames(r.src), addr, groups)
	if e != nil {
		return e
	}
	dst, e := resolvePanList(panNames(r.dst), addr, groups)
	if e != nil {
		return e
	}
	zonesF, zonesT := panNames(r.from), panNames(r.to)
	services := panNames(r.services)
	count, ok := boundedPANOSProduct(len(src), len(dst), len(zonesF), len(zonesT), len(services))
	if !ok || count > 4096-len(p.Policies) {
		return fmt.Errorf("security rule expansion exceeds 256")
	}
	lineRefs := panRuleEvidence(c, h, r.lines, "security_rule", "Security policy rule")
	for _, f := range zonesF {
		for _, t := range zonesT {
			for _, a := range src {
				for _, b := range dst {
					for _, sn := range services {
						proto, port, er := panService(sn, sv)
						if er != nil {
							return er
						}
						refs := append([]string(nil), lineRefs...)
						depLines := append([]int{a.line, b.line, sv[sn].line}, panGroupDependencyLines(append(append([]string{}, r.src...), r.dst...), groups, groupLines)...)
						for _, depLine := range depLines {
							if depLine > 0 {
								refs = append(refs, addPanEvidence(c, h, depLine, "policy_object", "Address or service dependency"))
							}
						}
						id := fmt.Sprintf("panos:%s:%s:%s:%s:%s:%s:%s", r.name, f, t, a.prefix, b.prefix, sn, proto+"-"+port)
						rule := policy.Rule{ID: id, Position: len(p.Policies) + 1, Enabled: !r.disabled, Action: action, Source: a.prefix, Destination: b.prefix, Protocol: proto, DestinationPort: port, EvidenceRefs: refs}
						p.Policies = append(p.Policies, PathPolicyRule{ID: id, From: f, To: t, Match: rule, EvidenceRefs: refs})
					}
				}
			}
		}
	}
	return nil
}
func resolvePanList(ns []string, a map[string]panAddr, g map[string][]string) ([]panAddr, error) {
	out := []panAddr{}
	for _, n := range ns {
		x, e := resolvePanName(n, a, g, 0)
		if e != nil {
			return nil, e
		}
		out = append(out, x...)
		if len(out) > 256 {
			return nil, fmt.Errorf("address list expansion exceeds 256")
		}
	}
	return out, nil
}
func boundedPANOSProduct(factors ...int) (int, bool) {
	product := 1
	for _, n := range factors {
		if n <= 0 || n > 256 || product > 256/n {
			return 0, false
		}
		product *= n
	}
	return product, true
}
func expandPanNAT(r panRule, addr map[string]panAddr, g map[string][]string, groupLines map[string][]int, sv map[string]panSvc, p *PathConfig, c *Config, h string) error {
	if r.unsupported || !r.hasSrc || !r.hasDst || !r.hasService || len(r.src) == 0 || len(r.dst) == 0 || len(r.services) == 0 || len(r.from) != 1 || len(r.to) != 1 {
		return fmt.Errorf("NAT requires one source and destination zone")
	}
	src, e := resolvePanList(panNames(r.src), addr, g)
	if e != nil {
		return e
	}
	dst, e := resolvePanList(panNames(r.dst), addr, g)
	if e != nil {
		return e
	}
	if len(src) != 1 || len(dst) != 1 {
		return fmt.Errorf("NAT must resolve to one original source and destination")
	}
	service := panNames(r.services)
	if len(service) != 1 {
		return fmt.Errorf("NAT must resolve to one service")
	}
	proto, port, e := panService(service[0], sv)
	if e != nil {
		return e
	}
	kind := "none"
	orig, translated := "", ""
	matchSrc, matchDst := src[0].prefix, dst[0].prefix
	if r.srcX != "" {
		kind = "snat"
		orig = src[0].prefix
		translated, e = panPrefix(r.srcX)
	}
	if r.dstX != "" {
		if kind != "none" {
			return fmt.Errorf("combined source and destination translation is unsupported")
		}
		kind = "dnat"
		orig = dst[0].prefix
		translated, e = panPrefix(r.dstX)
	}
	if e != nil {
		return fmt.Errorf("invalid translated address")
	}
	if kind != "none" {
		a, _, _ := net.ParseCIDR(orig)
		b, _, _ := net.ParseCIDR(translated)
		ao, _ := netMaskBits(orig)
		bb, _ := netMaskBits(translated)
		if ao != bb || a == nil || b == nil {
			return fmt.Errorf("static NAT requires equal prefix lengths")
		}
	}
	refs := panRuleEvidence(c, h, r.lines, "nat_rule", "NAT policy rule")
	depLines := append([]int{src[0].line, dst[0].line, sv[service[0]].line}, panGroupDependencyLines(append(append([]string{}, r.src...), r.dst...), g, groupLines)...)
	for _, dep := range depLines {
		if dep > 0 {
			refs = append(refs, addPanEvidence(c, h, dep, "policy_object", "Address or service dependency"))
		}
	}
	id := "panos-nat:" + r.name
	rule := policy.Rule{ID: id, Position: len(p.Translations) + 1, Enabled: !r.disabled, Action: "permit", Source: matchSrc, Destination: matchDst, Protocol: proto, DestinationPort: port, EvidenceRefs: refs}
	p.Translations = append(p.Translations, PathTranslation{ID: id, Kind: kind, From: r.from[0], To: r.to[0], Match: rule, OriginalPrefix: orig, TranslatedPrefix: translated, EvidenceRefs: refs})
	return nil
}
func netMaskBits(s string) (int, bool) {
	_, n, e := net.ParseCIDR(s)
	if e != nil {
		return 0, false
	}
	x, _ := n.Mask.Size()
	return x, true
}

func panZonesKnown(names []string, zones map[string][]string) bool {
	if len(names) == 0 {
		return false
	}
	for _, name := range names {
		if name != "any" {
			if _, ok := zones[name]; !ok {
				return false
			}
		}
	}
	return true
}

func panRuleEvidence(c *Config, hash string, lines []int, kind, summary string) []string {
	refs := []string{}
	seen := map[int]bool{}
	for _, n := range lines {
		if n > 0 && !seen[n] {
			seen[n] = true
			refs = append(refs, addPanEvidence(c, hash, n, kind, summary))
		}
	}
	return refs
}

func panGroupDependencyLines(names []string, groups map[string][]string, lines map[string][]int) []int {
	seenNames, seenLines := map[string]bool{}, map[int]bool{}
	out := []int{}
	var walk func(string, int)
	walk = func(name string, depth int) {
		if depth > 8 || seenNames[name] {
			return
		}
		seenNames[name] = true
		for _, n := range lines[name] {
			if n > 0 && !seenLines[n] {
				seenLines[n] = true
				out = append(out, n)
			}
		}
		for _, member := range groups[name] {
			walk(member, depth+1)
		}
	}
	for _, name := range names {
		walk(name, 0)
	}
	return out
}

// safePANOSSourceCommand reconstructs only recognized syntax from validated
// tokens. It never copies an original line or a description/comment value.
func safePANOSSourceCommand(t []string) string {
	if len(t) == 0 {
		return ""
	}
	parts := []string{"set"}
	add := func(xs ...string) string { return strings.Join(append(append([]string{}, parts...), xs...), " ") }
	validName := func(s string) bool { return panSafe(s) && len(s) <= 128 }
	validZone := func(s string) bool { return panSafe(s) && len(s) <= 64 }
	validInterface := func(s string) bool { return panSafe(s) && len(s) <= 48 }
	switch {
	case len(t) == 4 && t[0] == "address" && validName(t[1]) && t[2] == "ip-netmask":
		if p, e := panPrefix(t[3]); e == nil {
			return add("address", t[1], "ip-netmask", p)
		}
	case len(t) >= 4 && t[0] == "address-group" && validName(t[1]) && t[2] == "static":
		members := panNames(t[3:])
		if len(members) == 0 || len(members) > 256 {
			return ""
		}
		for _, member := range members {
			if !validName(member) {
				return ""
			}
		}
		return add("address-group", t[1], "static", "[", strings.Join(members, " "), "]")
	case len(t) == 6 && t[0] == "service" && validName(t[1]) && t[2] == "protocol" && (t[3] == "tcp" || t[3] == "udp") && t[4] == "port":
		if _, _, e := panService("_", map[string]panSvc{"_": {proto: t[3], port: t[5]}}); e == nil {
			return add("service", t[1], "protocol", t[3], "port", t[5])
		}
	case len(t) == 5 && t[0] == "zone" && validZone(t[1]) && t[2] == "network" && t[3] == "layer3" && validInterface(t[4]):
		return add(t...)
	case len(t) == 6 && t[0] == "network" && t[1] == "interface" && t[2] == "ethernet" && validInterface(t[3]) && t[4] == "link-state" && (t[5] == "up" || t[5] == "down"):
		return add(t...)
	case len(t) == 7 && t[0] == "network" && t[1] == "interface" && t[2] == "ethernet" && validInterface(t[3]) && t[4] == "layer3" && t[5] == "ip":
		if p, e := panAddressMask(t[6]); e == nil {
			return add(t[:6]...) + " " + p
		}
	case len(t) == 5 && t[0] == "network" && t[1] == "virtual-router" && validName(t[2]) && t[3] == "interface" && validInterface(t[4]):
		return add(t...)
	case len(t) == 9 && t[0] == "network" && t[1] == "virtual-router" && validName(t[2]) && t[3] == "routing-table" && t[4] == "ip" && t[5] == "static-route" && validName(t[6]) && t[7] == "destination":
		if p, e := panPrefix(t[8]); e == nil {
			return add(t[:8]...) + " " + p
		}
	case len(t) == 9 && t[0] == "network" && t[1] == "virtual-router" && validName(t[2]) && t[3] == "routing-table" && t[4] == "ip" && t[5] == "static-route" && validName(t[6]) && t[7] == "interface" && validInterface(t[8]):
		return add(t...)
	case len(t) == 10 && t[0] == "network" && t[1] == "virtual-router" && validName(t[2]) && t[3] == "routing-table" && t[4] == "ip" && t[5] == "static-route" && validName(t[6]) && t[7] == "nexthop" && t[8] == "ip-address":
		if ip := net.ParseIP(t[9]); ip != nil {
			return add(t[:9]...) + " " + ip.String()
		}
	case len(t) >= 5 && t[0] == "rulebase" && (t[1] == "security" || t[1] == "nat") && t[2] == "rules" && validName(t[3]):
		field, vals := t[4], t[5:]
		if len(vals) == 0 {
			return ""
		}
		validNames := func() bool {
			for _, x := range vals {
				if !validName(x) {
					return false
				}
			}
			return true
		}
		switch field {
		case "from", "to":
			valid := true
			for _, x := range vals {
				if !validZone(x) {
					valid = false
				}
			}
			if valid {
				return add(t[:5]...) + " " + strings.Join(vals, " ")
			}
		case "source", "destination":
			if validNames() {
				for _, x := range vals {
					if _, e := panPrefix(x); e != nil && x != "any" && !validName(x) {
						return ""
					}
				}
				return add(t[:5]...) + " " + strings.Join(vals, " ")
			}
		case "service":
			if validNames() {
				return add(t[:5]...) + " " + strings.Join(vals, " ")
			}
		case "application":
			if len(vals) == 1 && vals[0] == "any" {
				return add(t[:5]...) + " any"
			}
		case "action":
			if len(vals) == 1 && (vals[0] == "allow" || vals[0] == "deny" || vals[0] == "drop") {
				return add(t[:5]...) + " " + vals[0]
			}
		case "disabled":
			if len(vals) == 1 && (vals[0] == "yes" || vals[0] == "no") {
				return add(t[:5]...) + " " + vals[0]
			}
		case "source-translation":
			if len(vals) == 1 && vals[0] == "none" {
				return add(t[:5]...) + " none"
			}
			if len(vals) == 3 && vals[0] == "static-ip" && vals[1] == "translated-address" {
				if p, e := panPrefix(vals[2]); e == nil {
					return add(t[:5]...) + " static-ip translated-address " + p
				}
			}
		case "destination-translation":
			if len(vals) == 2 && vals[0] == "translated-address" {
				if p, e := panPrefix(vals[1]); e == nil {
					return add(t[:5]...) + " translated-address " + p
				}
			}
		}
	}
	return ""
}
func sourceLineForRef(c *Config, refs []string) int {
	for _, e := range c.Evidence {
		for _, r := range refs {
			if e.ID == r {
				return e.StartLine
			}
		}
	}
	return 0
}
func dedupePanWarnings(ws []Warning) []Warning {
	out := []Warning{}
	seen := map[string]bool{}
	for _, w := range ws {
		k := fmt.Sprintf("%s/%d", w.Code, w.Line)
		if !seen[k] {
			seen[k] = true
			out = append(out, w)
		}
	}
	return out
}

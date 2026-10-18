package firewall

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"net/netip"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/mattpainter701/graphlayer/platform/policy"
)

const FortiGateParserVersion = "fortios-config-v1"
const fortiPathVersion = "fortios-path-v1"

const (
	fortiMaxBytes      = 2 << 20
	fortiMaxLines      = 50000
	fortiMaxInterfaces = 1024
	fortiMaxRoutes     = 4096
	fortiMaxRules      = 4096
	fortiMaxExpansion  = 256
)

//go:embed testdata/fortigate_sample.txt
var fortiSample embed.FS

// SampleFortiGate returns the sanitized native CLI example.
func SampleFortiGate() []byte {
	b, _ := fortiSample.ReadFile("testdata/fortigate_sample.txt")
	return b
}

type fortiEntry struct {
	name   string
	sets   map[string][]string
	lines  []int
	parent string
}
type fortiBlock struct {
	name string
	line int
}

// ParseFortiGateConfig reads a bounded FortiOS native configuration export.
// Only a deliberately small IPv4 routing, policy, static VIP and static SNAT
// subset is normalized; unsupported inputs make the corresponding inventory
// incomplete and are never copied verbatim into the returned source lines.
func ParseFortiGateConfig(input []byte) (*Config, error) {
	if len(input) > fortiMaxBytes {
		return nil, fmt.Errorf("FortiOS configuration exceeds 2 MiB limit")
	}
	if strings.IndexByte(string(input), 0) >= 0 {
		return nil, fmt.Errorf("FortiOS configuration contains a NUL byte")
	}
	lines := strings.Split(strings.ReplaceAll(string(input), "\r\n", "\n"), "\n")
	if len(lines) > fortiMaxLines {
		return nil, fmt.Errorf("FortiOS configuration exceeds 50000 line limit")
	}
	h := sha256.Sum256(input)
	hash := hex.EncodeToString(h[:])
	c := &Config{Vendor: "fortinet-fortigate", ParserVersion: FortiGateParserVersion, Hostname: "fortigate-root", Context: "root", ArtifactSHA256: hash,
		SourceLines: make([]SourceLine, len(lines)), Evidence: []Evidence{}, Chains: []Chain{}, Warnings: []Warning{},
		Capabilities: []string{"FortiOS root VDOM IPv4 interfaces and connected routes", "static IPv4 routes", "ordered firewall policies", "literal subnet addresses", "numeric TCP/UDP services", "static one-to-one IPv4 VIP and IP pool translations"}}
	for i := range lines {
		c.SourceLines[i] = SourceLine{Number: i + 1, Text: "[OMITTED CONFIGURATION]"}
	}
	entries := map[string]map[string]*fortiEntry{}
	unsupportedSections := map[string]int{}
	unsupportedCommands := []int{}
	var stack []fortiBlock
	var current *fortiEntry
	lineRefs := map[int]string{}
	evSeen := map[string]bool{}
	addEv := func(n int, kind, summary string) string {
		id := fmt.Sprintf("forti:%s:%d", hash[:16], n)
		if n < 1 || n > len(lines) {
			return ""
		}
		if !evSeen[id] {
			evSeen[id] = true
			c.Evidence = append(c.Evidence, Evidence{ID: id, StartLine: n, EndLine: n, Kind: kind, Summary: summary})
		}
		lineRefs[n] = id
		return id
	}
	warn := func(code, msg string, n int) {
		c.Warnings = append(c.Warnings, Warning{Code: code, Message: msg, Line: n})
	}
	setLine := func(n int, text string) {
		if n > 0 && n <= len(c.SourceLines) {
			c.SourceLines[n-1].Text = text
		}
	}
	for i, raw := range lines {
		n := i + 1
		s := strings.TrimSpace(raw)
		if s == "" || strings.HasPrefix(s, "#") {
			continue
		}
		f, ok := fortiTokens(s)
		if !ok || len(f) == 0 {
			return nil, fmt.Errorf("malformed FortiOS syntax at line %d", n)
		}
		switch strings.ToLower(f[0]) {
		case "config":
			if len(f) < 2 || len(stack) > 16 {
				return nil, fmt.Errorf("malformed or over-nested config at line %d", n)
			}
			name := strings.Join(f[1:], " ")
			stack = append(stack, fortiBlock{name, n})
			current = nil
			if fortiKnownSection(name) {
				setLine(n, "config "+name)
			} else {
				setLine(n, "config [UNSUPPORTED SECTION]")
				unsupportedSections[strings.ToLower(name)] = n
			}
			addEv(n, "configuration_scope", "Configuration section")
		case "edit":
			if len(stack) == 0 || len(f) != 2 || current != nil {
				return nil, fmt.Errorf("malformed edit at line %d", n)
			}
			path := fortiPath(stack)
			key := strings.ToLower(path)
			name := f[1]
			if !fortiName(name) || len(name) > 79 {
				return nil, fmt.Errorf("invalid object name at line %d", n)
			}
			if entries[key] == nil {
				entries[key] = map[string]*fortiEntry{}
			}
			if entries[key][name] != nil {
				return nil, fmt.Errorf("duplicate %s entry at line %d", path, n)
			}
			current = &fortiEntry{name: name, sets: map[string][]string{}, lines: []int{n}, parent: path}
			entries[key][name] = current
			if fortiKnownSection(path) {
				setLine(n, "edit "+name)
			} else {
				setLine(n, "[OMITTED CONFIGURATION ENTRY]")
			}
			addEv(n, "configuration_entry", "Configuration object")
		case "set":
			directSection := strings.ToLower(fortiPath(stack))
			direct := directSection == "system settings" || directSection == "system global" || (len(stack) > 0 && !fortiKnownSection(directSection))
			directEntry := current == nil && direct
			if directEntry && len(f) >= 3 {
				if entries[directSection] == nil {
					entries[directSection] = map[string]*fortiEntry{}
				}
				current = entries[directSection]["root"]
				if current == nil {
					current = &fortiEntry{name: "root", sets: map[string][]string{}, lines: []int{n}, parent: directSection}
					entries[directSection]["root"] = current
				}
			}
			if current == nil || len(f) < 3 {
				return nil, fmt.Errorf("set outside edit or missing value at line %d", n)
			}
			k := strings.ToLower(f[1])
			if _, dup := current.sets[k]; dup {
				return nil, fmt.Errorf("duplicate set %s at line %d", k, n)
			}
			current.sets[k] = f[2:]
			if fortiKnownSection(fortiPath(stack)) && fortiKnownSetKey(fortiPath(stack), k) && safeFortiSourceValue(k, f[2:]) {
				current.lines = append(current.lines, n)
				setLine(n, "set "+k+" "+strings.Join(f[2:], " "))
			}
			if directEntry {
				current = nil
			}
		case "next":
			if current == nil || len(f) != 1 {
				return nil, fmt.Errorf("unbalanced next at line %d", n)
			}
			current = nil
			setLine(n, "next")
			addEv(n, "configuration_entry", "Configuration object boundary")
		case "end":
			if current != nil || len(stack) == 0 || len(f) != 1 {
				return nil, fmt.Errorf("unbalanced end at line %d", n)
			}
			stack = stack[:len(stack)-1]
			setLine(n, "end")
			addEv(n, "configuration_scope", "Configuration section boundary")
		default:
			unsupportedCommands = append(unsupportedCommands, n)
		}
	}
	if current != nil || len(stack) != 0 {
		return nil, fmt.Errorf("truncated FortiOS configuration block")
	}
	// Recognized set commands are added to evidence only after path-specific validation.
	for _, group := range entries {
		for _, e := range group {
			for _, ln := range e.lines[1:] {
				addEv(ln, "configuration_value", "Normalized configuration value")
			}
		}
	}
	get := func(path, name, key string) []string {
		if e := entries[strings.ToLower(path)][name]; e != nil {
			return e.sets[key]
		}
		return nil
	}
	entry := func(path string) map[string]*fortiEntry { return entries[strings.ToLower(path)] }
	if hostname := fortiScalar(entry("system global")["root"], "hostname"); fortiName(hostname) && len(hostname) <= 64 {
		c.Hostname = hostname
	}
	p := &PathConfig{Version: fortiPathVersion, Interfaces: []PathInterface{}, Routes: []PathRoute{}, NATRules: []PathNATRule{}, Policies: []PathPolicyRule{}, Translations: []PathTranslation{}, RoutingComplete: true, NATComplete: true, PolicyComplete: true, InterInterface: false, Warnings: []Warning{}, EvidenceRefs: []string{}}
	mark := func(e *fortiEntry, kind, summary string) []string {
		refs := []string{}
		if e != nil {
			for _, ln := range e.lines {
				refs = append(refs, addEv(ln, kind, summary))
			}
		}
		return fortiUnique(refs)
	}
	pwarn := func(code, msg string, e *fortiEntry, routing bool) {
		line := 0
		if e != nil && len(e.lines) > 0 {
			line = e.lines[0]
		}
		p.Warnings = append(p.Warnings, Warning{Code: code, Message: msg, Line: line})
		if routing {
			p.RoutingComplete = false
		} else {
			p.NATComplete = false
		}
		if line > 0 {
			p.EvidenceRefs = append(p.EvidenceRefs, addEv(line, "unsupported_path_input", msg))
		}
	}
	for _, line := range unsupportedCommands {
		setLine(line, "[UNSUPPORTED CONFIGURATION COMMAND]")
		pwarn("unsupported_config", "Configuration contains syntax outside the supported parser model", &fortiEntry{lines: []int{line}}, true)
		p.PolicyComplete = false
		p.NATComplete = false
	}
	for section, line := range unsupportedSections {
		if strings.HasPrefix(section, "router ") || strings.HasPrefix(section, "system sdwan") || strings.Contains(section, "policy-route") || strings.Contains(section, "pbr") {
			pwarn("unsupported_routing_section", "Dynamic, policy, or SD-WAN routing configuration is outside the supported model", &fortiEntry{lines: []int{line}}, true)
		}
		if strings.Contains(section, "vipgrp") || strings.Contains(section, "central-snat") || strings.Contains(section, "central-dnat") || strings.Contains(section, "ippool6") || strings.Contains(section, "vip6") {
			pwarn("unsupported_nat_section", "NAT configuration section is outside the supported model", &fortiEntry{lines: []int{line}}, false)
			p.PolicyComplete = false
		}
		if strings.HasPrefix(section, "firewall policy") || strings.HasPrefix(section, "firewall local-in") {
			p.PolicyComplete = false
			pwarn("unsupported_policy_section", "Firewall policy section is outside the supported model", &fortiEntry{lines: []int{line}}, false)
		}
	}
	// Only one explicit root VDOM is modeled.
	if mode := fortiScalar(entry("system global")["root"], "vdom-mode"); mode != "" && !strings.EqualFold(mode, "no-vdom") {
		p.PolicyComplete = false
		pwarn("unsupported_vdom", "Global VDOM mode is outside the supported single-root model", entry("system global")["root"], true)
	}
	if vdoms := entry("vdom"); len(vdoms) > 1 || (len(vdoms) == 1 && vdoms["root"] == nil) {
		p.PolicyComplete = false
		pwarn("unsupported_vdom", "Multiple or non-root VDOM configuration is outside the supported model", firstFortiEntry(vdoms), true)
	}
	if vals := get("system settings", "root", "central-nat"); len(vals) > 0 && strings.EqualFold(vals[0], "enable") {
		pwarn("central_nat_unsupported", "Central NAT changes translation semantics outside this model", entry("system settings")["root"], false)
		p.PolicyComplete = false
	}
	if len(entry("firewall central-snat-map")) > 0 {
		pwarn("central_snat_unsupported", "Central SNAT maps are outside the supported model", firstFortiEntry(entry("firewall central-snat-map")), false)
	}
	if len(entry("firewall central-dnat-map")) > 0 {
		pwarn("central_dnat_unsupported", "Central DNAT maps are outside the supported model", firstFortiEntry(entry("firewall central-dnat-map")), false)
	}
	intfs := entry("system interface")
	if len(intfs) > fortiMaxInterfaces {
		return nil, fmt.Errorf("FortiOS interface count exceeds limit")
	}
	iby := map[string]PathInterface{}
	for _, name := range fortiEntryNames(intfs) {
		e := intfs[name]
		if len(name) > 48 {
			pwarn("unsupported_interface", "Interface name exceeds the supported limit", e, true)
			continue
		}
		if !fortiOnlyKeys(e, "vdom", "ip", "status", "type", "alias", "role", "description", "allowaccess", "mtu", "speed") {
			pwarn("unsupported_interface", "Interface has unsupported forwarding properties", e, true)
			continue
		}
		zone := fortiScalar(e, "vdom")
		if !strings.EqualFold(zone, "root") {
			pwarn("non_root_vdom", "Interface is not explicitly assigned to the root VDOM", e, true)
			continue
		}
		ip := fortiPair(e, "ip")
		addr := ""
		resolved := ip != ""
		if ip != "" {
			a, err := netip.ParseAddr(ip)
			mask := fortiScalar(e, "ip")
			_ = mask
			if err == nil && a.Is4() { // FortiOS stores address and dotted mask as a pair.
				vals := e.sets["ip"]
				if len(vals) == 2 {
					bits, ok := fortiMaskBits(vals[1])
					if ok {
						addr = netip.PrefixFrom(a, bits).String()
					} else {
						resolved = false
					}
				} else {
					resolved = false
				}
			} else {
				resolved = false
			}
		}
		status := fortiScalar(e, "status")
		if status != "" && !strings.EqualFold(status, "up") && !strings.EqualFold(status, "down") {
			resolved = false
		}
		pi := PathInterface{Name: name, Physical: name, Address: addr, Shutdown: strings.EqualFold(status, "down"), Resolved: resolved, EvidenceRefs: mark(e, "path_interface", "Interface addressing and state")}
		iby[strings.ToLower(name)] = pi
		if !resolved {
			message := "Interface address or state is outside the supported model"
			if ip == "" {
				message = "Interface has no explicit IPv4 address"
			}
			pwarn("unsupported_interface", message, e, true)
		}
		p.Interfaces = append(p.Interfaces, pi)
		if resolved && addr != "" && !pi.Shutdown {
			pr, _ := netip.ParsePrefix(addr)
			p.Routes = append(p.Routes, PathRoute{ID: "connected:" + name, Kind: "connected", Prefix: pr.Masked().String(), Interface: name, EvidenceRefs: append([]string{}, pi.EvidenceRefs...)})
		}
	}
	for _, e := range entry("router static") {
		_ = e
	}
	// Router static entries are indexed by their edit identifiers.
	for _, id := range fortiEntryNames(entry("router static")) {
		e := entry("router static")[id]
		gw := fortiScalar(e, "gateway")
		dev := fortiScalar(e, "device")
		dist := fortiIntDefault(e, "distance", 10)
		priority := fortiIntDefault(e, "priority", 0)
		pre, pe := netip.ParsePrefix("0.0.0.0/0")
		if len(e.sets["dst"]) == 2 {
			a, ae := netip.ParseAddr(e.sets["dst"][0])
			bits, bok := fortiMaskBits(e.sets["dst"][1])
			if ae == nil && a.Is4() && bok {
				pre = netip.PrefixFrom(a, bits)
			} else {
				pe = fmt.Errorf("invalid route prefix")
			}
		} else {
			pe = fmt.Errorf("invalid route prefix")
		}
		hop, he := netip.ParseAddr(gw)
		if !fortiOnlyKeys(e, "dst", "gateway", "device", "distance", "priority") || pe != nil || !pre.Addr().Is4() || he != nil || !hop.Is4() || dev == "" || priority != 0 || dist < 1 || dist > 254 || !fortiName(dev) || len(dev) > 48 || len(p.Routes) >= fortiMaxRoutes {
			pwarn("unsupported_route", "Static route syntax, options, or limit is outside the supported model", e, true)
			continue
		}
		if _, ok := iby[strings.ToLower(dev)]; !ok {
			pwarn("unresolved_route_interface", "Static route device is not a configured interface", e, true)
			continue
		}
		p.Routes = append(p.Routes, PathRoute{ID: "static:" + id, Kind: "static", Prefix: pre.Masked().String(), Interface: dev, NextHop: hop.String(), Distance: dist, EvidenceRefs: mark(e, "static_route", "Static IPv4 route")})
	}
	addresses := map[string]string{}
	for name, e := range entry("firewall address") {
		v := e.sets["subnet"]
		if len(v) == 2 {
			a, err := netip.ParseAddr(v[0])
			bits, ok := fortiMaskBits(v[1])
			if err == nil && a.Is4() && ok {
				addresses[name] = netip.PrefixFrom(a, bits).Masked().String()
			}
		}
	}
	services := map[string]fortiService{}
	for name, e := range entry("firewall service custom") {
		if !fortiOnlyKeys(e, "protocol", "tcp-portrange", "udp-portrange", "comments", "uuid") {
			continue
		}
		protoSetting := strings.ToUpper(fortiScalar(e, "protocol"))
		if protoSetting != "" && protoSetting != "TCP" && protoSetting != "UDP" && protoSetting != "TCP/UDP/SCTP" {
			continue
		}
		proto := ""
		port := ""
		tcpPort := fortiScalar(e, "tcp-portrange")
		udpPort := fortiScalar(e, "udp-portrange")
		if tcpPort != "" && udpPort == "" {
			proto, port = "tcp", tcpPort
		} else if udpPort != "" && tcpPort == "" {
			proto, port = "udp", udpPort
		} else {
			continue
		}
		if protoSetting == "TCP" && proto != "tcp" || protoSetting == "UDP" && proto != "udp" {
			continue
		}
		if strings.Contains(port, "-") {
			parts := strings.Split(port, "-")
			if len(parts) != 2 || parts[0] != parts[1] {
				continue
			}
			port = parts[0]
		}
		n, err := strconv.Atoi(port)
		if err == nil && n > 0 && n <= 65535 {
			services[name] = fortiService{proto, strconv.Itoa(n)}
		}
	}
	// Static, single-address VIPs only. The original destination is preserved
	// in the translation, and policies selecting that VIP target the real host.
	vips := map[string]*fortiEntry{}
	vipAmbiguous := false
	for _, name := range fortiEntryNames(entry("firewall vip")) {
		e := entry("firewall vip")[name]
		vips[name] = e
		ext := fortiScalar(e, "extip")
		mapped := fortiScalar(e, "mappedip")
		iface := fortiScalar(e, "extintf")
		typ := fortiScalar(e, "type")
		start := fortiScalar(e, "portforward")
		a, ae := netip.ParseAddr(ext)
		b, be := netip.ParseAddr(mapped)
		if !fortiOnlyKeys(e, "extip", "mappedip", "extintf", "type", "portforward", "comments", "uuid", "arp-reply", "nat-source-vip") || strings.EqualFold(fortiScalar(e, "nat-source-vip"), "enable") {
			pwarn("unsupported_vip", "VIP has unsupported translation options", e, false)
			p.PolicyComplete = false
			continue
		}
		if len(e.sets["portforward"]) > 0 && !strings.EqualFold(start, "disable") {
			pwarn("vip_port_forward_unsupported", "Port-forward VIP is outside the supported model", e, false)
			p.PolicyComplete = false
			continue
		}
		if (typ != "" && !strings.EqualFold(typ, "static-nat")) || ae != nil || be != nil || !a.Is4() || !b.Is4() || iface == "" || !fortiName(iface) {
			pwarn("unsupported_vip", "VIP is not a static one-to-one IPv4 mapping", e, false)
			p.PolicyComplete = false
			continue
		}
		for _, other := range fortiEntryNames(entry("firewall vip")) {
			on := entry("firewall vip")[other]
			if other != name && fortiScalar(on, "extip") == ext && fortiScalar(on, "extintf") == iface {
				vipAmbiguous = true
			}
		}
		orig := netip.PrefixFrom(a, 32).String()
		translated := netip.PrefixFrom(b, 32).String()
		refs := mark(e, "static_vip", "Static one-to-one virtual IP")
		p.Translations = append(p.Translations, PathTranslation{ID: "vip:" + name, Kind: "dnat", From: iface, To: "any", Match: policy.Rule{ID: "vip:" + name, Enabled: true, Action: "permit", Source: "any", Destination: orig, Protocol: "ip", EvidenceRefs: append([]string(nil), refs...)}, OriginalPrefix: orig, TranslatedPrefix: translated, EvidenceRefs: refs})
	}
	// Configured IP pools can intercept packets addressed to their IPs, even
	// when no policy references them. FortiOS behavior depends on firmware; the
	// parser therefore models a pool only when ARP reply is explicitly disabled.
	type fortiPool struct {
		ip string
	}
	pools := map[string]fortiPool{}
	for name, e := range entry("firewall ippool") {
		typ := fortiScalar(e, "type")
		start := fortiScalar(e, "startip")
		end := fortiScalar(e, "endip")
		arpReply := strings.ToLower(fortiScalar(e, "arp-reply"))
		a, ae := netip.ParseAddr(start)
		b, be := netip.ParseAddr(end)
		if !fortiOnlyKeys(e, "type", "startip", "endip", "arp-reply", "comments", "uuid") {
			pwarn("unsupported_ippool", "IP pool has unsupported address or forwarding options", e, false)
			continue
		}
		if typ != "one-to-one" || ae != nil || be != nil || !a.Is4() || a != b {
			pwarn("unsupported_ippool", "IP pool is not a single-address one-to-one pool", e, false)
			continue
		}
		if arpReply != "disable" {
			pwarn("ippool_local_address_unsupported", "IP pool ARP reply is enabled or defaults to enabled; local-address behavior is outside the supported model", e, false)
			continue
		}
		pools[name] = fortiPool{ip: a.String()}
	}
	// An enabled central NAT setting means firewall-policy NAT flags are ignored.
	policyEntries := entry("firewall policy")
	if len(policyEntries) > fortiMaxRules {
		return nil, fmt.Errorf("FortiOS policy count exceeds limit")
	}
	policyComplete := len(unsupportedCommands) == 0
	poolSources := map[string]map[string]bool{}
	for _, id := range fortiPolicyEntryNames(policyEntries) {
		e := policyEntries[id]
		if strings.EqualFold(fortiScalar(e, "status"), "disable") {
			continue
		}
		from := fortiScalar(e, "srcintf")
		to := fortiScalar(e, "dstintf")
		srcNames := e.sets["srcaddr"]
		dstNames := e.sets["dstaddr"]
		svcNames := e.sets["service"]
		action := strings.ToLower(fortiScalar(e, "action"))
		schedule := fortiScalar(e, "schedule")
		if len(srcNames) != 1 || len(dstNames) != 1 || len(svcNames) < 1 || from == "" || to == "" || schedule != "always" || (action != "accept" && action != "deny") || fortiHasUnsupportedPolicyFields(e) {
			policyComplete = false
			pwarn("unsupported_policy", "Firewall policy has unsupported matching or action semantics", e, false)
			continue
		}
		if _, ok := iby[strings.ToLower(from)]; !ok {
			policyComplete = false
			pwarn("unresolved_policy_interface", "Policy interface is not configured", e, false)
			continue
		}
		if strings.EqualFold(fortiScalar(e, "nat"), "enable") && strings.EqualFold(fortiScalar(e, "ippool"), "enable") && len(e.sets["poolname"]) != 1 {
			policyComplete = false
			pwarn("unsupported_policy_snat", "Policy selects an unsupported dynamic or multiple IP pool", e, false)
			continue
		}
		src, ok := fortiAddress(srcNames[0], addresses)
		if !ok {
			policyComplete = false
			pwarn("unsupported_policy_address", "Policy source address is unresolved", e, false)
			continue
		}
		vipName := ""
		dst := ""
		if ve := vips[dstNames[0]]; ve != nil {
			vipName = dstNames[0]
			dst = fortiScalar(ve, "mappedip")
			da, er := netip.ParseAddr(dst)
			if er != nil || !da.Is4() {
				policyComplete = false
				pwarn("unsupported_policy_vip", "Policy-selected VIP cannot be normalized", e, false)
				continue
			}
			dst = netip.PrefixFrom(da, 32).String()
			if vipAmbiguous {
				policyComplete = false
				pwarn("ambiguous_vip", "Overlapping static VIPs make destination translation ambiguous", e, false)
				continue
			}
		} else {
			var found bool
			dst, found = fortiAddress(dstNames[0], addresses)
			if !found {
				policyComplete = false
				pwarn("unsupported_policy_address", "Policy destination address is unresolved", e, false)
				continue
			}
		}
		svcs := []fortiService{}
		bad := false
		for _, sn := range svcNames {
			if strings.EqualFold(sn, "ALL") {
				svcs = append(svcs, fortiService{"any", ""})
			} else if v, ok := services[sn]; ok {
				svcs = append(svcs, v)
			} else {
				bad = true
			}
		}
		if bad || len(svcs) == 0 || len(svcs) > fortiMaxExpansion {
			policyComplete = false
			pwarn("unsupported_policy_service", "Policy service is unresolved or exceeds expansion limit", e, false)
			continue
		}
		poolName := ""
		nat := strings.EqualFold(fortiScalar(e, "nat"), "enable")
		poolEnabled := strings.EqualFold(fortiScalar(e, "ippool"), "enable")
		if nat && poolEnabled {
			poolName = fortiScalar(e, "poolname")
			sourcePrefix, sourceErr := netip.ParsePrefix(src)
			if sourceErr != nil || !sourcePrefix.Addr().Is4() || sourcePrefix.Bits() != 32 {
				policyComplete = false
				pwarn("unsupported_policy_snat", "Static one-to-one SNAT requires one literal IPv4 source address", e, false)
				continue
			}
			if _, ok := pools[poolName]; !ok {
				policyComplete = false
				pwarn("unsupported_policy_snat", "Policy IP pool is unresolved or its local-address behavior is unsupported", e, false)
				continue
			}
		} else if nat {
			p.NATComplete = false
			pwarn("dynamic_snat_unsupported", "Default-interface or dynamic SNAT is outside the static model", e, false)
		}
		refs := mark(e, "firewall_policy", "Ordered firewall policy")
		for _, sn := range srcNames {
			refs = append(refs, mark(entry("firewall address")[sn], "policy_address", "Policy address object")...)
		}
		for _, sn := range svcNames {
			refs = append(refs, mark(entry("firewall service custom")[sn], "policy_service", "Policy service object")...)
		}
		if vipName != "" {
			refs = append(refs, mark(vips[vipName], "static_vip", "Static one-to-one virtual IP")...)
		}
		for _, svc := range svcs {
			pos := len(p.Policies)
			rid := "policy:" + id + ":" + strconv.Itoa(pos+1)
			policyAction := action
			if action == "accept" {
				policyAction = "permit"
			} else {
				policyAction = "deny"
			}
			match := policy.Rule{ID: rid, Position: pos, Enabled: true, Action: policyAction, Source: src, Destination: dst, Protocol: svc.proto, DestinationPort: svc.port, EvidenceRefs: fortiUnique(refs)}
			vipID := ""
			if vipName != "" {
				vipID = "vip:" + vipName
			}
			p.Policies = append(p.Policies, PathPolicyRule{ID: rid, From: from, To: to, Match: match, VIP: vipID, EvidenceRefs: match.EvidenceRefs})
			c.RuleOrigins = append(c.RuleOrigins, RuleOrigin{RuleID: rid, ACLName: "FortiGate firewall policy", Line: e.lines[0]})
			if poolName != "" {
				poolIP := pools[poolName].ip
				if poolSources[poolIP] == nil {
					poolSources[poolIP] = map[string]bool{}
				}
				if len(poolSources[poolIP]) > 0 && !poolSources[poolIP][src] {
					p.NATComplete = false
					pwarn("ippool_address_reused", "One-to-one IP pool address is reused for different source prefixes", entry("firewall ippool")[poolName], false)
				}
				poolSources[poolIP][src] = true
				snatID := "snat:" + id + ":" + strconv.Itoa(pos+1)
				srcPrefix := src
				if src == "any" {
					srcPrefix = "0.0.0.0/0"
				}
				tr := PathTranslation{ID: snatID, Kind: "snat", From: from, To: to, Match: policy.Rule{ID: snatID, Enabled: true, Action: "permit", Source: src, Destination: dst, Protocol: svc.proto, DestinationPort: svc.port}, OriginalPrefix: srcPrefix, TranslatedPrefix: netip.PrefixFrom(netip.MustParseAddr(poolIP), 32).String(), PolicyID: rid, EvidenceRefs: fortiUnique(append(append([]string{}, refs...), mark(entry("firewall ippool")[poolName], "static_ippool", "Static one-address IP pool")...))}
				tr.Match.EvidenceRefs = append([]string(nil), tr.EvidenceRefs...)
				p.Translations = append(p.Translations, tr)
				p.Policies[len(p.Policies)-1].SourceNAT = tr.ID
			}
		}
		if len(p.Policies) > fortiMaxRules {
			return nil, fmt.Errorf("FortiOS expanded policy count exceeds limit")
		}
	}
	if len(policyEntries) == 0 {
		policyComplete = false
		warn("no_policy_configuration", "No firewall policies were present in the supported configuration", 0)
	}
	p.PolicyComplete = policyComplete && len(policyEntries) > 0
	if vipAmbiguous {
		p.NATComplete = false
	}
	// Config-level references include only validated and unsupported relevant lines.
	for _, i := range p.Interfaces {
		p.EvidenceRefs = append(p.EvidenceRefs, i.EvidenceRefs...)
	}
	for _, r := range p.Routes {
		p.EvidenceRefs = append(p.EvidenceRefs, r.EvidenceRefs...)
	}
	for _, r := range p.Policies {
		p.EvidenceRefs = append(p.EvidenceRefs, r.EvidenceRefs...)
	}
	for _, r := range p.Translations {
		p.EvidenceRefs = append(p.EvidenceRefs, r.EvidenceRefs...)
	}
	p.EvidenceRefs = fortiUnique(p.EvidenceRefs)
	// Populate policy chains as ordered inventory; a selected chain alone does not establish all interface/VIP context.
	ch := Chain{ID: "fortigate:policies", Name: "FortiGate firewall policy", Direction: "forward", Complete: false, Rules: []policy.Rule{}, EvidenceRefs: []string{}, Warnings: []Warning{{Code: "incomplete_chain_context", Message: "Selected policy chain lacks full interface and VIP context"}}}
	for _, pol := range p.Policies {
		ch.Rules = append(ch.Rules, pol.Match)
		ch.EvidenceRefs = append(ch.EvidenceRefs, pol.EvidenceRefs...)
	}
	ch.EvidenceRefs = fortiUnique(ch.EvidenceRefs)
	if len(ch.Rules) > 0 {
		c.Chains = append(c.Chains, ch)
	}
	c.Path = p
	sort.Slice(c.Evidence, func(i, j int) bool {
		if c.Evidence[i].StartLine == c.Evidence[j].StartLine {
			return c.Evidence[i].ID < c.Evidence[j].ID
		}
		return c.Evidence[i].StartLine < c.Evidence[j].StartLine
	})
	sort.Slice(p.Translations, func(i, j int) bool { return p.Translations[i].ID < p.Translations[j].ID })
	for _, warnings := range [][]Warning{c.Warnings, p.Warnings} {
		sort.SliceStable(warnings, func(i, j int) bool {
			if warnings[i].Line == warnings[j].Line {
				return warnings[i].Code < warnings[j].Code
			}
			return warnings[i].Line < warnings[j].Line
		})
	}
	return c, nil
}

type fortiService struct{ proto, port string }

func fortiTokens(s string) ([]string, bool) {
	var out []string
	var b strings.Builder
	quoted := false
	escaped := false
	active := false
	for _, r := range s {
		if escaped {
			b.WriteRune(r)
			escaped = false
			active = true
			continue
		}
		if r == '\\' && quoted {
			escaped = true
			continue
		}
		if r == '"' {
			quoted = !quoted
			active = true
			continue
		}
		if unicode.IsSpace(r) && !quoted {
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
	if quoted || escaped {
		return nil, false
	}
	if active {
		out = append(out, b.String())
	}
	return out, true
}
func fortiPath(s []fortiBlock) string {
	a := make([]string, len(s))
	for i, b := range s {
		a[i] = b.name
	}
	return strings.Join(a, " ")
}
func fortiSafePath(s string) string {
	if len(s) > 120 {
		return "[SUPPORTED SECTION]"
	}
	for _, r := range s {
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == ' ') {
			return "[SUPPORTED SECTION]"
		}
	}
	return s
}

func fortiKnownSection(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "system interface", "router static", "firewall address", "firewall service custom", "firewall policy", "firewall vip", "firewall ippool", "system settings", "vdom", "firewall central-snat-map", "firewall central-dnat-map":
		return true
	default:
		return false
	}
}

func fortiKnownSetKey(section, key string) bool {
	switch strings.ToLower(strings.TrimSpace(section)) {
	case "system interface":
		return fortiOneOf(key, "vdom", "ip", "status", "type", "alias", "role", "description", "allowaccess", "mtu", "speed")
	case "router static":
		return fortiOneOf(key, "dst", "gateway", "device", "distance", "priority")
	case "firewall address":
		return fortiOneOf(key, "subnet", "type")
	case "firewall service custom":
		return fortiOneOf(key, "protocol", "tcp-portrange", "udp-portrange")
	case "firewall policy":
		return fortiOneOf(key, "srcintf", "dstintf", "srcaddr", "dstaddr", "service", "schedule", "action", "status", "nat", "ippool", "poolname", "name", "utm-status")
	case "firewall vip":
		return fortiOneOf(key, "extip", "mappedip", "extintf", "type", "portforward", "nat-source-vip")
	case "firewall ippool":
		return fortiOneOf(key, "type", "startip", "endip", "arp-reply")
	case "system settings":
		return fortiOneOf(key, "central-nat")
	default:
		return false
	}
}

func fortiOneOf(value string, choices ...string) bool {
	for _, choice := range choices {
		if strings.EqualFold(value, choice) {
			return true
		}
	}
	return false
}
func fortiName(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune("_.-", r)) {
			return false
		}
	}
	return true
}
func fortiScalar(e *fortiEntry, k string) string {
	if e == nil || len(e.sets[k]) != 1 {
		return ""
	}
	return e.sets[k][0]
}
func fortiPair(e *fortiEntry, k string) string {
	if e == nil || len(e.sets[k]) != 2 {
		return ""
	}
	return e.sets[k][0]
}
func fortiMaskBits(s string) (int, bool) {
	a, err := netip.ParseAddr(s)
	if err != nil || !a.Is4() {
		return 0, false
	}
	b := a.As4()
	bits := 0
	zero := false
	for _, x := range b {
		for i := 7; i >= 0; i-- {
			one := x&(1<<i) != 0
			if zero && one {
				return 0, false
			}
			if one {
				bits++
			} else {
				zero = true
			}
		}
	}
	return bits, true
}
func fortiIntDefault(e *fortiEntry, k string, d int) int {
	v := fortiScalar(e, k)
	if v == "" {
		return d
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return -1
	}
	return n
}
func fortiAddress(n string, addresses map[string]string) (string, bool) {
	if strings.EqualFold(n, "all") {
		return "any", true
	}
	v, ok := addresses[n]
	return v, ok
}
func fortiHasUnsupportedPolicyFields(e *fortiEntry) bool {
	allowed := map[string]bool{"srcintf": true, "dstintf": true, "srcaddr": true, "dstaddr": true, "service": true, "schedule": true, "action": true, "status": true, "nat": true, "ippool": true, "poolname": true, "logtraffic": true, "name": true, "comments": true, "uuid": true, "policyid": true, "utm-status": true}
	for k := range e.sets {
		if !allowed[k] {
			return true
		}
		if k == "utm-status" && len(e.sets[k]) == 1 && strings.EqualFold(e.sets[k][0], "enable") {
			return true
		}
	}
	return false
}

func fortiOnlyKeys(e *fortiEntry, keys ...string) bool {
	allowed := map[string]bool{}
	for _, k := range keys {
		allowed[k] = true
	}
	for k := range e.sets {
		if !allowed[k] {
			return false
		}
	}
	return true
}
func fortiUnique(a []string) []string {
	seen := map[string]bool{}
	o := []string{}
	for _, s := range a {
		if s != "" && !seen[s] {
			seen[s] = true
			o = append(o, s)
		}
	}
	sort.Strings(o)
	return o
}
func firstFortiEntry(m map[string]*fortiEntry) *fortiEntry {
	for _, name := range fortiEntryNames(m) {
		return m[name]
	}
	return nil
}

func fortiEntryNames(m map[string]*fortiEntry) []string {
	names := make([]string, 0, len(m))
	for name := range m {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		a, ae := strconv.Atoi(names[i])
		b, be := strconv.Atoi(names[j])
		if ae == nil && be == nil {
			return a < b
		}
		return strings.ToLower(names[i]) < strings.ToLower(names[j])
	})
	return names
}

func fortiPolicyEntryNames(m map[string]*fortiEntry) []string {
	names := fortiEntryNames(m)
	sort.SliceStable(names, func(i, j int) bool { return m[names[i]].lines[0] < m[names[j]].lines[0] })
	return names
}

func safeFortiSourceValue(key string, values []string) bool {
	if fortiOneOf(key, "alias", "role", "description", "allowaccess", "name", "comments", "uuid") {
		return false
	}
	for _, value := range values {
		if len(value) == 0 || len(value) > 128 {
			return false
		}
		for _, r := range value {
			if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("_.-/:", r)) {
				return false
			}
		}
	}
	return true
}

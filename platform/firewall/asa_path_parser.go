package firewall

import (
	_ "embed"
	"fmt"
	"net/netip"
	"sort"
	"strconv"
	"strings"

	asa "github.com/mattpainter701/graphlayer/platform/vendors/cisco/asa"
)

const asaPathVersion = "asa-path-v1"

const (
	maxPathInterfaces = 1024
	maxPathRoutes     = 4096
	maxPathNATRules   = 4096
)

//go:embed testdata/asa_path_sample.txt
var samplePathASA []byte

// SamplePathASA returns a copy of the sanitized path-investigation example.
func SamplePathASA() []byte { return append([]byte(nil), samplePathASA...) }

// addASAPathInventory adds the normalized, configuration-derived forwarding
// inputs used by path investigations. Unknown commands are represented only
// by fixed warning text and source references; raw command text is never copied
// into the path model.
func addASAPathInventory(c *Config, parsed *asa.ASAConfig) {
	p := &PathConfig{
		Version: asaPathVersion, Interfaces: []PathInterface{}, Routes: []PathRoute{},
		NATRules: []PathNATRule{}, RoutingComplete: true, NATComplete: true,
		Warnings: []Warning{}, EvidenceRefs: []string{},
	}
	if c == nil || parsed == nil {
		return
	}
	c.Path = p

	evidenceIDs := make(map[string]bool, len(c.Evidence))
	for _, e := range c.Evidence {
		evidenceIDs[e.ID] = true
	}
	addEvidence := func(line int, kind, summary string) string {
		if line < 1 || line > len(parsed.Lines) {
			return ""
		}
		id := evidenceID(c.ArtifactSHA256, []int{line})
		markSourceLines(c, []int{line}, parsed.Lines)
		if !evidenceIDs[id] {
			c.Evidence = append(c.Evidence, Evidence{ID: id, StartLine: line, EndLine: line, Kind: kind, Summary: summary})
			evidenceIDs[id] = true
		}
		return id
	}
	addRefs := func(lines []int, kind, summary string) []string {
		refs := []string{}
		for _, line := range unique(lines) {
			if ref := addEvidence(line, kind, summary); ref != "" {
				refs = append(refs, ref)
			}
		}
		p.EvidenceRefs = append(p.EvidenceRefs, refs...)
		return refs
	}
	warn := func(code, message string, line int, routing bool) {
		p.Warnings = append(p.Warnings, Warning{Code: code, Message: message, Line: line})
		if routing {
			p.RoutingComplete = false
		} else {
			p.NATComplete = false
		}
		if ref := addEvidence(line, "unsupported_path_input", message); ref != "" {
			p.EvidenceRefs = append(p.EvidenceRefs, ref)
			// A parser warning records only the location and reason. Keep the
			// unsupported command's source slot useful without retaining its raw
			// values in the returned evidence payload.
			if line > 0 && line <= len(c.SourceLines) {
				c.SourceLines[line-1].Text = "[OMITTED UNSUPPORTED PATH CONFIGURATION]"
			}
		}
	}

	interfaces, interfaceLines, unsupportedInterfaceLines := parseASAPathInterfaces(parsed.Lines)
	if len(interfaces) > maxPathInterfaces {
		warn("path_interface_limit", "Interface count exceeds the supported path input limit", 0, true)
		interfaces = interfaces[:maxPathInterfaces]
	}
	interfaceByName := map[string]*PathInterface{}
	duplicateInterface := make([]bool, len(interfaces))
	for i := range interfaces {
		it := &interfaces[i]
		if it.Name == "" || len(it.Name) > 48 {
			it.Resolved = false
		} else if _, exists := interfaceByName[strings.ToLower(it.Name)]; exists {
			it.Resolved = false
			interfaceByName[strings.ToLower(it.Name)].Resolved = false
			duplicateInterface[i] = true
		} else {
			interfaceByName[strings.ToLower(it.Name)] = it
		}
	}
	for i := range interfaces {
		interfaces[i].EvidenceRefs = addRefs(interfaceLines[i], "path_interface", "Interface forwarding properties")
		if interfaces[i].Name == "" || len(interfaces[i].Name) > 48 {
			line := 0
			if len(interfaceLines[i]) > 0 {
				line = interfaceLines[i][0]
			}
			warn("unresolved_interface_name", "Interface name is missing or exceeds the supported length", line, true)
			continue
		}
		if duplicateInterface[i] {
			// Keep one stable record for the ambiguous name and mark it unresolved.
			warn("ambiguous_interface_name", "Interface name is assigned to multiple interface stanzas", interfaceLines[i][0], true)
			continue
		}
		p.Interfaces = append(p.Interfaces, interfaces[i])
	}
	for _, line := range unsupportedInterfaceLines {
		warn("unsupported_interface", "Interface forwarding properties are incomplete", line, true)
	}
	// Connected routes follow only from an interface with a resolved name and
	// static IPv4 address. The host address is retained on the interface while
	// the route prefix is canonicalized to its network address.
	for _, intf := range p.Interfaces {
		if !intf.Resolved || intf.Address == "" {
			continue
		}
		prefix, err := netip.ParsePrefix(intf.Address)
		if err != nil || !prefix.Addr().Is4() {
			continue
		}
		ref := ""
		if len(intf.EvidenceRefs) > 0 {
			ref = intf.EvidenceRefs[0]
		}
		p.Routes = append(p.Routes, PathRoute{
			ID: "asa-connected:" + ref + ":" + strings.ToLower(intf.Name), Kind: "connected",
			Prefix: prefix.Masked().String(), Interface: intf.Name, Distance: 0,
			EvidenceRefs: append([]string(nil), intf.EvidenceRefs...),
		})
	}

	seenRoutes := map[string]bool{}
	routeIndexByKey := map[string]int{}
	for _, l := range parsed.Lines {
		f := strings.Fields(strings.TrimSpace(l.Text))
		if len(f) == 0 {
			continue
		}
		cmd := strings.ToLower(f[0])
		switch cmd {
		case "route":
			route, ok := parseASAStaticPathRoute(f, l.Number, c.ArtifactSHA256)
			if !ok {
				warn("unsupported_route", "Static route syntax or options are outside the supported path model", l.Number, true)
				continue
			}
			if strings.EqualFold(route.Interface, "null0") {
				warn("unsupported_route", "Null interface routes are outside the supported path model", l.Number, true)
				continue
			}
			if intf, ok := interfaceByName[strings.ToLower(route.Interface)]; !ok || !intf.Resolved {
				warn("unresolved_route_interface", "Static route interface cannot be resolved to one configured interface", l.Number, true)
				continue
			}
			route.Interface = interfaceByName[strings.ToLower(route.Interface)].Name
			route.EvidenceRefs = addRefs([]int{l.Number}, "static_route", "Static IPv4 route")
			key := route.Prefix + "/" + strings.ToLower(route.Interface) + "/" + route.NextHop + "/" + strconv.Itoa(route.Distance)
			if !seenRoutes[key] {
				seenRoutes[key] = true
				routeIndexByKey[key] = len(p.Routes)
				p.Routes = append(p.Routes, route)
			} else {
				index := routeIndexByKey[key]
				p.Routes[index].EvidenceRefs = uniqueStrings(append(p.Routes[index].EvidenceRefs, route.EvidenceRefs...))
			}
		case "ipv6":
			if len(f) > 1 && strings.EqualFold(f[1], "route") {
				warn("unsupported_route", "IPv6 route is outside the IPv4 path model", l.Number, true)
			}
		case "router":
			if len(f) > 1 && isASADynamicRoutingProtocol(f[1]) {
				warn("dynamic_routing", "Dynamic routing configuration is outside the static path model", l.Number, true)
			}
		case "policy-route", "pbr":
			warn("policy_routing", "Policy routing configuration is outside the static path model", l.Number, true)
		case "route-map":
			warn("policy_routing", "Route-map routing behavior is outside the static path model", l.Number, true)
		case "firewall":
			if len(f) > 1 && strings.EqualFold(f[1], "transparent") {
				warn("transparent_mode", "Transparent firewall mode is outside the routed path model", l.Number, true)
			}
		case "crypto":
			if len(f) > 1 && (strings.EqualFold(f[1], "map") || strings.EqualFold(f[1], "dynamic-map")) {
				warn("vpn_routing", "VPN crypto-map routing is outside the static path model", l.Number, true)
			}
		case "tunnel-group":
			warn("vpn_routing", "VPN tunnel configuration is outside the static path model", l.Number, true)
		case "nat-control":
			warn("unsupported_nat", "Global NAT control is outside the supported path model", l.Number, false)
		case "static", "global":
			warn("unsupported_nat", "Legacy NAT configuration is outside the supported object NAT model", l.Number, false)
		case "ip":
			if len(f) > 2 && strings.EqualFold(f[1], "verify") && strings.EqualFold(f[2], "reverse-path") {
				warn("reverse_path_verification", "Reverse-path verification is outside the static path model", l.Number, true)
			} else if len(f) > 1 && strings.EqualFold(f[1], "policy") {
				warn("policy_routing", "IP policy routing is outside the static path model", l.Number, true)
			}
		case "mode":
			if len(f) > 1 && strings.EqualFold(f[1], "multiple") {
				warn("ambiguous_context", "Multiple-context routing attribution is incomplete", l.Number, true)
			} else if len(f) > 1 && strings.EqualFold(f[1], "transparent") {
				warn("transparent_mode", "Transparent firewall mode is outside the routed path model", l.Number, true)
			}
		case "context":
			warn("ambiguous_context", "Multiple-context routing attribution is incomplete", l.Number, true)
		case "vrf":
			warn("unsupported_vrf", "VRF routing is outside the supported path model", l.Number, true)
		}
	}
	if len(p.Routes) > maxPathRoutes {
		p.Routes = p.Routes[:maxPathRoutes]
		warn("path_route_limit", "Route count exceeds the supported path input limit", 0, true)
	}

	for _, l := range parsed.Lines {
		f := strings.Fields(strings.TrimSpace(l.Text))
		if len(f) > 0 && !strings.HasPrefix(l.Text, " ") && !strings.HasPrefix(l.Text, "\t") && strings.EqualFold(f[0], "nat") {
			warn("unsupported_nat", "Manual or twice NAT is outside the supported object NAT path model", l.Number, false)
		}
	}
	nats, unsupportedNAT := parseASAObjectPathNAT(parsed, c.ArtifactSHA256)
	for _, nat := range nats {
		realIntf, realOK := interfaceByName[strings.ToLower(nat.RealInterface)]
		if !realOK || !realIntf.Resolved {
			warn("unresolved_nat_interface", "Static NAT real interface cannot be resolved", nat.natLine, false)
			continue
		}
		mappedIntf, mappedOK := interfaceByName[strings.ToLower(nat.MappedInterface)]
		if !mappedOK || !mappedIntf.Resolved {
			warn("unresolved_nat_interface", "Static NAT mapped interface cannot be resolved", nat.natLine, false)
			continue
		}
		nat.RealInterface = realIntf.Name
		nat.MappedInterface = mappedIntf.Name
		refs := []int{nat.declarationLine, nat.addressLine, nat.natLine}
		nat.PathNATRule.EvidenceRefs = addRefs(refs, "static_object_nat", "Static object NAT mapping")
		p.NATRules = append(p.NATRules, nat.PathNATRule)
	}
	for _, line := range unsupportedNAT {
		warn("unsupported_nat", "Object NAT syntax or address mapping is outside the supported path model", line, false)
	}
	if len(p.NATRules) > maxPathNATRules {
		p.NATRules = p.NATRules[:maxPathNATRules]
		warn("path_nat_limit", "NAT rule count exceeds the supported path input limit", 0, false)
	}

	for _, l := range parsed.Lines {
		f := strings.Fields(strings.TrimSpace(l.Text))
		if len(f) == 3 && strings.EqualFold(f[0], "same-security-traffic") && strings.EqualFold(f[1], "permit") && strings.EqualFold(f[2], "inter-interface") {
			p.InterInterface = true
			p.EvidenceRefs = append(p.EvidenceRefs, addEvidence(l.Number, "path_setting", "Inter-interface traffic setting"))
		} else if len(f) == 4 && strings.EqualFold(f[0], "no") && strings.EqualFold(f[1], "same-security-traffic") && strings.EqualFold(f[2], "permit") && strings.EqualFold(f[3], "inter-interface") {
			p.InterInterface = false
			p.EvidenceRefs = append(p.EvidenceRefs, addEvidence(l.Number, "path_setting", "Inter-interface traffic setting"))
		} else if (len(f) == 3 && strings.EqualFold(f[0], "same-security-traffic") && strings.EqualFold(f[1], "permit") && strings.EqualFold(f[2], "intra-interface")) || (len(f) == 4 && strings.EqualFold(f[0], "no") && strings.EqualFold(f[1], "same-security-traffic") && strings.EqualFold(f[2], "permit") && strings.EqualFold(f[3], "intra-interface")) {
			warn("intra_interface_traffic", "Intra-interface forwarding is outside the supported path model", l.Number, true)
		}
	}
	p.EvidenceRefs = uniqueStrings(p.EvidenceRefs)
	sort.Slice(p.Interfaces, func(i, j int) bool {
		if p.Interfaces[i].Name == p.Interfaces[j].Name {
			return p.Interfaces[i].Physical < p.Interfaces[j].Physical
		}
		return p.Interfaces[i].Name < p.Interfaces[j].Name
	})
	sort.Slice(p.Routes, func(i, j int) bool { return p.Routes[i].ID < p.Routes[j].ID })
	sort.Slice(p.NATRules, func(i, j int) bool { return p.NATRules[i].ID < p.NATRules[j].ID })
}

type parsedPathInterface struct {
	value PathInterface
	lines []int
}

func parseASAPathInterfaces(lines []asa.ConfigLine) ([]PathInterface, [][]int, []int) {
	items := []parsedPathInterface{}
	unsupported := []int{}
	current := -1
	seenName, seenAddress, seenLevel := false, false, false
	for _, l := range lines {
		trim := strings.TrimSpace(l.Text)
		f := strings.Fields(trim)
		if len(f) == 0 {
			continue
		}
		if !strings.HasPrefix(l.Text, " ") && !strings.HasPrefix(l.Text, "\t") {
			current = -1
			if strings.EqualFold(f[0], "interface") && len(f) >= 2 {
				items = append(items, parsedPathInterface{value: PathInterface{Physical: strings.Join(f[1:], " "), Resolved: true}, lines: []int{l.Number}})
				current = len(items) - 1
				seenName, seenAddress, seenLevel = false, false, false
			}
			continue
		}
		if current < 0 {
			continue
		}
		it := &items[current]
		switch strings.ToLower(f[0]) {
		case "nameif":
			it.lines = append(it.lines, l.Number)
			if len(f) != 2 || seenName {
				it.value.Resolved = false
				unsupported = append(unsupported, l.Number)
			} else {
				it.value.Name = f[1]
				seenName = true
			}
		case "ip":
			if len(f) > 1 && strings.EqualFold(f[1], "address") {
				it.lines = append(it.lines, l.Number)
				if seenAddress || len(f) < 3 || strings.EqualFold(f[2], "dhcp") || strings.EqualFold(f[2], "pppoe") {
					it.value.Resolved = false
					unsupported = append(unsupported, l.Number)
				} else {
					addr, err := netip.ParseAddr(f[2])
					if err != nil || !addr.Is4() || addr.Zone() != "" {
						it.value.Resolved = false
						unsupported = append(unsupported, l.Number)
					} else {
						mask := "255.255.255.255"
						if len(f) >= 4 && !strings.EqualFold(f[3], "standby") {
							mask = f[3]
						}
						prefix, e := ipv4Prefix(addr, mask)
						if e != nil {
							it.value.Resolved = false
							unsupported = append(unsupported, l.Number)
						} else {
							it.value.Address = addr.String() + "/" + strconv.Itoa(prefix.Bits())
							seenAddress = true
							if len(f) > 4 && strings.EqualFold(f[4], "standby") {
								// The standby address belongs to the failover peer. The
								// path model cannot distinguish active and standby state,
								// so retain neither as a resolved transit interface.
								it.value.Resolved = false
								unsupported = append(unsupported, l.Number)
							} else if len(f) > 4 {
								it.value.Resolved = false
								unsupported = append(unsupported, l.Number)
							}
						}
					}
				}
			} else {
				it.value.Resolved = false
				unsupported = append(unsupported, l.Number)
			}
		case "security-level":
			it.lines = append(it.lines, l.Number)
			if len(f) != 2 || seenLevel {
				it.value.Resolved = false
				unsupported = append(unsupported, l.Number)
			} else if n, err := strconv.Atoi(f[1]); err != nil || n < 0 || n > 100 {
				it.value.Resolved = false
				unsupported = append(unsupported, l.Number)
			} else {
				it.value.SecurityLevel = &n
				seenLevel = true
			}
		case "shutdown":
			it.lines = append(it.lines, l.Number)
			it.value.Shutdown = true
		case "no":
			if len(f) > 1 && strings.EqualFold(f[1], "shutdown") {
				it.lines = append(it.lines, l.Number)
				it.value.Shutdown = false
			} else if len(f) > 1 && (strings.EqualFold(f[1], "nameif") || strings.EqualFold(f[1], "ip")) {
				it.value.Resolved = false
				unsupported = append(unsupported, l.Number)
			} else if len(f) < 2 || !isASAPathInterfaceMetadata(strings.ToLower(f[1])) {
				it.value.Resolved = false
				unsupported = append(unsupported, l.Number)
			}
		case "ipv6", "bridge-group", "pppoe", "management-only", "standby":
			it.value.Resolved = false
			unsupported = append(unsupported, l.Number)
		default:
			if !isASAPathInterfaceMetadata(strings.ToLower(f[0])) {
				it.value.Resolved = false
				unsupported = append(unsupported, l.Number)
			}
		}
	}
	values, refs := make([]PathInterface, len(items)), make([][]int, len(items))
	for i, item := range items {
		if item.value.Name == "" || item.value.Address == "" {
			item.value.Resolved = false
		}
		values[i], refs[i] = item.value, unique(item.lines)
	}
	return values, refs, unique(unsupported)
}

func isASAPathInterfaceMetadata(command string) bool {
	switch command {
	case "description", "speed", "duplex", "mtu", "flowcontrol", "mac-address", "media-type", "bandwidth", "delay", "monitor-interface":
		return true
	default:
		return false
	}
}

func ipv4Prefix(addr netip.Addr, mask string) (netip.Prefix, error) {
	parsedMask, err := netip.ParseAddr(mask)
	if err != nil || !parsedMask.Is4() {
		return netip.Prefix{}, fmt.Errorf("invalid IPv4 mask")
	}
	bits := 0
	zeroSeen := false
	for _, b := range parsedMask.As4() {
		for bit := 7; bit >= 0; bit-- {
			one := (b & (1 << bit)) != 0
			if one && zeroSeen {
				return netip.Prefix{}, fmt.Errorf("non-contiguous IPv4 mask")
			}
			if one {
				bits++
			} else {
				zeroSeen = true
			}
		}
	}
	return netip.PrefixFrom(addr, bits), nil
}

func parseASAStaticPathRoute(f []string, line int, artifact string) (PathRoute, bool) {
	if len(f) < 5 || !strings.EqualFold(f[0], "route") {
		return PathRoute{}, false
	}
	iface := f[1]
	destToken := f[2]
	if destToken == "0" {
		destToken = "0.0.0.0"
	}
	dest, err := netip.ParseAddr(destToken)
	if err != nil || !dest.Is4() {
		return PathRoute{}, false
	}
	mask := f[3]
	if mask == "0" {
		mask = "0.0.0.0"
	}
	prefix, err := ipv4Prefix(dest, mask)
	if err != nil {
		return PathRoute{}, false
	}
	next, err := netip.ParseAddr(f[4])
	if err != nil || !next.Is4() || next.Zone() != "" {
		return PathRoute{}, false
	}
	distance := 1
	if len(f) > 5 {
		if n, convErr := strconv.Atoi(f[5]); convErr == nil && n >= 1 && n <= 254 {
			distance = n
		} else if strings.EqualFold(f[5], "tunneled") {
			return PathRoute{}, false
		} else {
			return PathRoute{}, false
		}
	}
	if len(f) > 6 {
		return PathRoute{}, false
	}
	ref := evidenceID(artifact, []int{line})
	return PathRoute{ID: "asa-route:" + ref, Kind: "static", Prefix: prefix.Masked().String(), Interface: iface, NextHop: next.String(), Distance: distance, EvidenceRefs: []string{}}, true
}

func isASADynamicRoutingProtocol(s string) bool {
	switch strings.ToLower(s) {
	case "bgp", "ospf", "eigrp", "rip", "isis":
		return true
	default:
		return false
	}
}

type parsedPathNAT struct {
	PathNATRule
	declarationLine int
	addressLine     int
	natLine         int
}

func parseASAObjectPathNAT(parsed *asa.ASAConfig, artifact string) ([]parsedPathNAT, []int) {
	lines := parsed.Lines
	type objectState struct {
		declarations []int
		address      string
		addressLine  int
		natFields    []string
		natLine      int
		badLines     []int
	}
	objects := map[string]*objectState{}
	unsupported := []int{}
	current := ""
	for _, l := range lines {
		trim := strings.TrimSpace(l.Text)
		f := strings.Fields(trim)
		if len(f) == 0 {
			continue
		}
		if !strings.HasPrefix(l.Text, " ") && !strings.HasPrefix(l.Text, "\t") {
			current = ""
			if len(f) >= 3 && strings.EqualFold(f[0], "object") && strings.EqualFold(f[1], "network") {
				current = f[2]
				o := objects[current]
				if o == nil {
					o = &objectState{}
					objects[current] = o
				}
				o.declarations = append(o.declarations, l.Number)
			} else if strings.EqualFold(f[0], "object-group") && len(f) > 1 && strings.EqualFold(f[1], "network") {
				current = ""
			}
			if strings.EqualFold(f[0], "nat") {
				unsupported = append(unsupported, l.Number)
			}
			continue
		}
		if current == "" {
			continue
		}
		o := objects[current]
		switch strings.ToLower(f[0]) {
		case "host":
			if len(f) != 2 {
				o.badLines = append(o.badLines, l.Number)
				continue
			}
			a, err := netip.ParseAddr(f[1])
			if err != nil || !a.Is4() || a.Zone() != "" {
				o.badLines = append(o.badLines, l.Number)
			} else {
				value := netip.PrefixFrom(a, 32).String()
				if o.address != "" && o.address != value {
					o.badLines = append(o.badLines, l.Number)
				} else if o.address == "" {
					o.address = value
					o.addressLine = l.Number
				}
			}
		case "subnet":
			if len(f) != 3 {
				o.badLines = append(o.badLines, l.Number)
				continue
			}
			a, err := netip.ParseAddr(f[1])
			if err != nil || !a.Is4() {
				o.badLines = append(o.badLines, l.Number)
				continue
			}
			prefix, err := ipv4Prefix(a, f[2])
			if err != nil {
				o.badLines = append(o.badLines, l.Number)
			} else {
				value := prefix.Masked().String()
				if o.address != "" && o.address != value {
					o.badLines = append(o.badLines, l.Number)
				} else if o.address == "" {
					o.address = value
					o.addressLine = l.Number
				}
			}
		case "nat":
			if o.natLine != 0 {
				o.badLines = append(o.badLines, l.Number)
			} else {
				o.natFields, o.natLine = f, l.Number
			}
		case "range", "object", "network-object", "network-object-group":
			o.badLines = append(o.badLines, l.Number)
		}
	}

	names := make([]string, 0, len(objects))
	for name := range objects {
		names = append(names, name)
	}
	sort.Strings(names)
	results := []parsedPathNAT{}
	for _, name := range names {
		o := objects[name]
		if o.natLine == 0 {
			continue
		}
		if sourceObject, ok := parsed.Objects[name]; ok {
			unsupported = append(unsupported, sourceObject.UnsupportedLines...)
		}
		if len(o.badLines) > 0 || o.address == "" || len(o.declarations) == 0 {
			unsupported = append(unsupported, o.natLine)
			unsupported = append(unsupported, o.badLines...)
			continue
		}
		f := o.natFields
		if len(f) != 4 || !strings.EqualFold(f[0], "nat") || !strings.HasPrefix(f[1], "(") || !strings.HasSuffix(f[1], ")") || !strings.EqualFold(f[2], "static") {
			unsupported = append(unsupported, o.natLine)
			continue
		}
		ifaces := strings.Split(strings.Trim(f[1], "()"), ",")
		if len(ifaces) != 2 || ifaces[0] == "" || ifaces[1] == "" || strings.EqualFold(ifaces[0], "any") || strings.EqualFold(ifaces[1], "any") {
			unsupported = append(unsupported, o.natLine)
			continue
		}
		mapped, err := netip.ParseAddr(f[3])
		if err != nil || !mapped.Is4() || mapped.Zone() != "" {
			unsupported = append(unsupported, o.natLine)
			continue
		}
		realPrefix, err := netip.ParsePrefix(o.address)
		if err != nil || !realPrefix.Addr().Is4() {
			unsupported = append(unsupported, o.natLine)
			continue
		}
		sourceObject, found := parsed.Objects[name]
		if !found || sourceObject.Group || len(sourceObject.UnsupportedLines) != 0 || len(sourceObject.Members) != 1 || !sameASAPathObjectAddress(sourceObject.Members[0], realPrefix) {
			unsupported = append(unsupported, o.natLine)
			continue
		}
		mappedPrefix := netip.PrefixFrom(mapped, realPrefix.Bits())
		if mappedPrefix.Masked().Addr() != mapped {
			unsupported = append(unsupported, o.natLine)
			continue
		}
		results = append(results, parsedPathNAT{PathNATRule: PathNATRule{
			ID: "asa-nat:" + evidenceID(artifact, []int{o.natLine}), ObjectName: name,
			RealInterface: ifaces[0], MappedInterface: ifaces[1], RealPrefix: realPrefix.Masked().String(),
			MappedPrefix: mappedPrefix.Masked().String(), EvidenceRefs: []string{},
		}, declarationLine: o.declarations[0], addressLine: o.addressLine, natLine: o.natLine})
	}
	return results, unique(unsupported)
}

func sameASAPathObjectAddress(value string, want netip.Prefix) bool {
	if prefix, err := netip.ParsePrefix(value); err == nil {
		return prefix.Addr().Is4() && prefix.Masked() == want.Masked()
	}
	addr, err := netip.ParseAddr(value)
	return err == nil && addr.Is4() && netip.PrefixFrom(addr, 32) == want.Masked()
}

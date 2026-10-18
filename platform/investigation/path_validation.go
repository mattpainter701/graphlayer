package investigation

import (
	"errors"
	"net/netip"
	"strings"
)

func validatePathConfig(s *Snapshot) error {
	if s.ParserVersion != "asa-config-v3" {
		if s.Path != nil {
			return errors.New("older parser cannot claim path metadata")
		}
		return nil
	}
	p := s.Path
	if p == nil || p.Version != "asa-path-v1" || len(p.Interfaces) > 1024 || len(p.Routes) > 10000 || len(p.NATRules) > 10000 || len(p.Warnings) > 50000 {
		return errors.New("invalid path model version or budget")
	}
	evidence := make(map[string]bool, len(s.Evidence))
	for _, e := range s.Evidence {
		evidence[e.ID] = true
	}
	refsOK := func(refs []string, required bool) bool {
		if required && len(refs) == 0 {
			return false
		}
		for _, ref := range refs {
			if !evidence[ref] {
				return false
			}
		}
		return true
	}
	if !refsOK(p.EvidenceRefs, false) {
		return errors.New("missing path coverage evidence")
	}
	interfaces := map[string]bool{}
	foldedInterfaces := map[string]bool{}
	interfacePrefixes := map[string]netip.Prefix{}
	for _, iface := range p.Interfaces {
		if iface.Name == "" || len(iface.Name) > 48 || strings.ContainsAny(iface.Name, "\r\n\x00") || foldedInterfaces[strings.ToLower(iface.Name)] || iface.Physical == "" || !refsOK(iface.EvidenceRefs, true) {
			return errors.New("invalid path interface identity or evidence")
		}
		interfaces[iface.Name] = true
		foldedInterfaces[strings.ToLower(iface.Name)] = true
		if iface.SecurityLevel != nil && (*iface.SecurityLevel < 0 || *iface.SecurityLevel > 100) {
			return errors.New("invalid interface security level")
		}
		if iface.Address != "" {
			address, err := netip.ParsePrefix(iface.Address)
			if err != nil || !address.Addr().Is4() {
				return errors.New("invalid interface IPv4 prefix")
			}
			interfacePrefixes[iface.Name] = address
		} else if iface.Resolved {
			return errors.New("resolved interface has no address")
		}
	}
	routes := map[string]bool{}
	for _, route := range p.Routes {
		prefix, err := netip.ParsePrefix(route.Prefix)
		if route.ID == "" || len(route.ID) > 256 || routes[route.ID] || err != nil || !prefix.Addr().Is4() || prefix != prefix.Masked() || !interfaces[route.Interface] || !refsOK(route.EvidenceRefs, true) {
			return errors.New("invalid path route or evidence")
		}
		routes[route.ID] = true
		switch route.Kind {
		case "connected":
			address, known := interfacePrefixes[route.Interface]
			if route.Distance != 0 || route.NextHop != "" || !known || address.Masked() != prefix {
				return errors.New("invalid connected route")
			}
		case "static":
			hop, hopErr := netip.ParseAddr(route.NextHop)
			if hopErr != nil || !hop.Is4() || route.Distance < 1 || route.Distance > 254 {
				return errors.New("invalid static route")
			}
		default:
			return errors.New("unknown route kind")
		}
	}
	natIDs := map[string]bool{}
	for _, rule := range p.NATRules {
		real, realErr := netip.ParsePrefix(rule.RealPrefix)
		mapped, mappedErr := netip.ParsePrefix(rule.MappedPrefix)
		if rule.ID == "" || len(rule.ID) > 256 || natIDs[rule.ID] || rule.ObjectName == "" || !interfaces[rule.RealInterface] || !interfaces[rule.MappedInterface] || !refsOK(rule.EvidenceRefs, true) || realErr != nil || mappedErr != nil || !real.Addr().Is4() || !mapped.Addr().Is4() || real != real.Masked() || mapped != mapped.Masked() || real.Bits() != mapped.Bits() {
			return errors.New("invalid static NAT mapping or evidence")
		}
		natIDs[rule.ID] = true
		if rule.Protocol == "" {
			if rule.RealPort != 0 || rule.MappedPort != 0 {
				return errors.New("NAT port mapping has no protocol")
			}
		} else if (rule.Protocol != "tcp" && rule.Protocol != "udp") || rule.RealPort < 1 || rule.RealPort > 65535 || rule.MappedPort < 1 || rule.MappedPort > 65535 {
			return errors.New("invalid NAT port mapping")
		}
	}
	for _, warning := range p.Warnings {
		if warning.Code == "" || warning.Line < 0 || warning.Line > len(s.SourceLines) || len(warning.Message) > 256 {
			return errors.New("invalid path warning")
		}
	}
	return nil
}

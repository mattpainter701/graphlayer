package investigation

import (
	"errors"
	"net/netip"
	"strings"

	"github.com/mattpainter701/graphlayer/platform/firewall"
	"github.com/mattpainter701/graphlayer/platform/policy"
)

func validatePathConfig(s *Snapshot) error {
	vendorPath := s.ParserVersion == "panos-config-v1" || s.ParserVersion == "fortios-config-v1"
	if s.ParserVersion != "asa-config-v3" && !vendorPath {
		if s.Path != nil {
			return errors.New("older parser cannot claim path metadata")
		}
		return nil
	}
	p := s.Path
	wantVersion := "asa-path-v1"
	if s.Vendor == "paloalto-panos" {
		wantVersion = "panos-path-v1"
	}
	if s.Vendor == "fortinet-fortigate" {
		wantVersion = "fortios-path-v1"
	}
	if p == nil || p.Version != wantVersion || len(p.Interfaces) > 1024 || len(p.Routes) > 10000 || len(p.NATRules) > 10000 || len(p.Warnings) > 50000 || len(p.Policies) > 4096 || len(p.Translations) > 4096 {
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
		if len(iface.Zone) > 64 || strings.ContainsAny(iface.Zone, "\r\n\x00") {
			return errors.New("invalid interface zone")
		}
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
	if !vendorPath {
		if len(p.Policies) > 0 || len(p.Translations) > 0 || p.PolicyComplete {
			return errors.New("ASA model cannot claim foreign policy semantics")
		}
		for _, iface := range p.Interfaces {
			if iface.Zone != "" {
				return errors.New("ASA model cannot claim zones")
			}
		}
		return nil
	}
	if len(p.NATRules) != 0 || p.InterInterface {
		return errors.New("vendor model cannot claim ASA NAT semantics")
	}
	policies := map[string]bool{}
	translations := map[string]firewall.PathTranslation{}
	for _, rule := range p.Policies {
		if rule.ID == "" || len(rule.ID) > 256 || policies[rule.ID] || rule.ID != rule.Match.ID || len(rule.From) == 0 || len(rule.To) == 0 || len(rule.From) > 64 || len(rule.To) > 64 || !refsOK(rule.EvidenceRefs, true) || !refsOK(rule.Match.EvidenceRefs, true) {
			return errors.New("invalid vendor policy identity or evidence")
		}
		policies[rule.ID] = true
		if err := validateVendorMatch(rule.Match); err != nil {
			return err
		}
	}
	for _, rule := range p.Translations {
		if rule.ID == "" || len(rule.ID) > 256 || translations[rule.ID].ID != "" || len(rule.From) == 0 || len(rule.To) == 0 || len(rule.From) > 64 || len(rule.To) > 64 || !refsOK(rule.EvidenceRefs, true) || !refsOK(rule.Match.EvidenceRefs, true) {
			return errors.New("invalid vendor translation identity or evidence")
		}
		translations[rule.ID] = rule
		if err := validateVendorMatch(rule.Match); err != nil {
			return err
		}
		if rule.Kind == "none" {
			if rule.OriginalPrefix != "" || rule.TranslatedPrefix != "" {
				return errors.New("invalid identity NAT")
			}
		} else {
			from, fe := netip.ParsePrefix(rule.OriginalPrefix)
			to, te := netip.ParsePrefix(rule.TranslatedPrefix)
			if (rule.Kind != "snat" && rule.Kind != "dnat") || fe != nil || te != nil || !from.Addr().Is4() || !to.Addr().Is4() || from != from.Masked() || to != to.Masked() || from.Bits() != to.Bits() {
				return errors.New("invalid vendor static translation")
			}
		}
		if rule.PolicyID != "" && !policies[rule.PolicyID] {
			return errors.New("translation references missing policy")
		}
	}
	for _, rule := range p.Policies {
		if rule.VIP != "" && translations[rule.VIP].Kind != "dnat" {
			return errors.New("policy references missing or incompatible VIP")
		}
		if rule.SourceNAT != "" {
			translation := translations[rule.SourceNAT]
			if translation.Kind != "snat" || translation.PolicyID != rule.ID || translation.From != rule.From || translation.To != rule.To {
				return errors.New("policy references missing or incompatible source NAT")
			}
		}
	}
	return nil
}

func validateVendorMatch(rule policy.Rule) error {
	scope := policy.Scope{SnapshotID: "validation", DeviceID: "validation", Context: "validation"}
	return policy.Validate(policy.Bundle{Scope: scope, Complete: false, DefaultAction: "deny", Rules: []policy.Rule{rule}}, policy.Flow{Scope: scope, Source: "192.0.2.1", Destination: "198.51.100.1", Protocol: "tcp", SourcePort: 52000, DestinationPort: 443})
}

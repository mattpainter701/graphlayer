package pathanalysis

import (
	"fmt"
	"net/netip"
	"reflect"
	"strings"

	"github.com/mattpainter701/graphlayer/platform/firewall"
	"github.com/mattpainter701/graphlayer/platform/policy"
)

const VendorContractVersion = "graphlayer-vendor-path-v1"

func isASAVendor(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "cisco-asa", "cisco_asa", "asa":
		return true
	default:
		return false
	}
}

// IsASAVendor reports whether the normalized device vendor uses ASA semantics.
func IsASAVendor(v string) bool { return isASAVendor(v) }

type vendorKind uint8

const (
	vendorUnknown vendorKind = iota
	vendorPAN
	vendorForti
)

func identifyVendor(v string) (vendorKind, string) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "palo-alto", "palo_alto", "paloalto", "paloalto-panos", "pan-os", "panos":
		return vendorPAN, "panos-path-v1"
	case "fortinet", "fortigate", "fortinet-fortigate", "fortios":
		return vendorForti, "fortios-path-v1"
	default:
		return vendorUnknown, ""
	}
}

func vendorNameMatch(kind vendorKind, a, b string) bool {
	a, b = strings.TrimSpace(a), strings.TrimSpace(b)
	if strings.EqualFold(a, "any") || a == "*" || strings.EqualFold(b, "any") || b == "*" {
		return true
	}
	if kind == vendorPAN {
		return a == b
	}
	return strings.EqualFold(a, b)
}

func findVendorInterface(kind vendorKind, all []firewall.PathInterface, name string) (firewall.PathInterface, int) {
	var value firewall.PathInterface
	count := 0
	for _, iface := range all {
		match := iface.Name == name
		if kind == vendorForti {
			match = strings.EqualFold(iface.Name, name)
		}
		if match {
			value, count = iface, count+1
		}
	}
	return value, count
}

// vendorEvaluate models the intentionally small common subset of PAN-OS and
// FortiGate forwarding. Device-specific policy and NAT never enter the ASA
// evaluator.
func vendorEvaluate(in Input, established *Result) (Result, error) {
	if err := policy.ValidateScope(in.Scope); err != nil {
		return Result{}, fmt.Errorf("scope: %w", err)
	}
	if in.Flow.Scope != in.Scope {
		return Result{}, fmt.Errorf("flow scope does not match path scope")
	}
	if in.IngressInterface == "" {
		return Result{}, fmt.Errorf("ingress_interface is required")
	}
	if err := policy.Validate(policy.Bundle{Scope: in.Scope, Complete: true, DefaultAction: "deny", Rules: []policy.Rule{}}, in.Flow); err != nil {
		return Result{}, fmt.Errorf("flow: %w", err)
	}
	if !netip.MustParseAddr(in.Flow.Source).Is4() || !netip.MustParseAddr(in.Flow.Destination).Is4() || !isTransitAddress(netip.MustParseAddr(in.Flow.Source)) || !isTransitAddress(netip.MustParseAddr(in.Flow.Destination)) {
		return Result{}, fmt.Errorf("vendor path requires native IPv4 unicast source and destination")
	}
	res := Result{EngineContractVersion: VendorContractVersion, Scope: in.Scope, IngressInterface: in.IngressInterface, OriginalFlow: in.Flow, PolicyFlow: in.Flow, Disposition: "indeterminate", Stages: []Stage{}, Reasons: []string{}, Assumptions: []string{"configuration-derived PAN-OS or FortiGate single-device path model", "configuration evidence completeness is assumed where explicitly asserted", "configured interface and route facts do not establish live link, neighbor, or session state", "return modeling uses the supplied forward permit as established TCP/UDP state"}}
	for _, n := range []string{"ingress", "destination_nat", "ingress_policy", "route", "source_nat", "egress"} {
		res.Stages = append(res.Stages, Stage{Name: n, Status: "not_evaluated"})
	}
	stage := func(name string) *Stage {
		for i := range res.Stages {
			if res.Stages[i].Name == name {
				return &res.Stages[i]
			}
		}
		panic("missing stage")
	}
	finish := func(d, why string) (Result, error) {
		res.Disposition = d
		if why != "" {
			res.Reasons = append(res.Reasons, why)
		}
		return res, nil
	}
	add := func(refs ...string) { res.EvidenceRefs = appendUnique(res.EvidenceRefs, refs...) }
	p := in.Config.Path
	if in.Config.Context != in.Scope.Context || p == nil {
		return finish("indeterminate", "configuration context or normalized path model is unavailable")
	}
	kind, version := identifyVendor(in.Config.Vendor)
	if kind == vendorUnknown {
		return finish("indeterminate", "vendor path semantics are unsupported")
	}
	if p.Version != version {
		return finish("indeterminate", "vendor path configuration version is unsupported")
	}
	add(p.EvidenceRefs...)
	ingress, count := findVendorInterface(kind, p.Interfaces, in.IngressInterface)
	if count != 1 {
		stage("ingress").Status = "unresolved"
		stage("ingress").Detail = "ingress interface is missing or ambiguous"
		return finish("indeterminate", "explicit ingress interface could not be resolved")
	}
	stage("ingress").EvidenceRefs = appendUnique(nil, ingress.EvidenceRefs...)
	add(ingress.EvidenceRefs...)
	if !ingress.Resolved || ingress.Shutdown {
		stage("ingress").Status = "unresolved"
		return finish("indeterminate", "ingress interface is not resolved and enabled")
	}
	stage("ingress").Status = "matched"
	stage("ingress").Detail = "explicit ingress interface is resolved and enabled"
	if !p.RoutingComplete || !p.PolicyComplete || !p.NATComplete {
		stage("route").Status = "unresolved"
		stage("ingress_policy").Status = "unresolved"
		stage("destination_nat").Status = "unresolved"
		return finish("indeterminate", "vendor routing, policy, or translation evidence is incomplete")
	}
	translationIDs := map[string]bool{}
	for _, t := range p.Translations {
		if t.ID == "" || translationIDs[t.ID] || strings.TrimSpace(t.From) == "" || strings.TrimSpace(t.To) == "" {
			stage("destination_nat").Status = "unresolved"
			return finish("indeterminate", "translation identifiers or zone/interface references are missing or duplicated")
		}
		translationIDs[t.ID] = true
		if t.Kind != "none" {
			orig, e1 := netip.ParsePrefix(t.OriginalPrefix)
			mapped, e2 := netip.ParsePrefix(t.TranslatedPrefix)
			if (t.Kind != "snat" && t.Kind != "dnat") || e1 != nil || e2 != nil || !orig.Addr().Is4() || !mapped.Addr().Is4() || orig != orig.Masked() || mapped != mapped.Masked() || orig.Bits() != mapped.Bits() {
				stage("destination_nat").Status = "unresolved"
				return finish("indeterminate", "translation uses unsupported or invalid IPv4 mapping semantics")
			}
		}
		if err := validateTranslationMatch(t, in.Scope); err != nil {
			stage("destination_nat").Status = "unresolved"
			return finish("indeterminate", err.Error())
		}
	}
	if why := validateRouteInputs(p.Routes, p.Interfaces); why != "" {
		stage("route").Status = "unresolved"
		stage("route").Detail = why
		return finish("indeterminate", why)
	}
	for _, route := range p.Routes {
		if _, n := findVendorInterface(kind, p.Interfaces, route.Interface); n != 1 {
			stage("route").Status = "unresolved"
			return finish("indeterminate", "route interface reference is missing or ambiguous")
		}
	}
	for _, i := range p.Interfaces {
		if i.Name == "" || !i.Resolved {
			stage("route").Status = "unresolved"
			return finish("indeterminate", "an interface fact is missing or unresolved")
		}
	}
	if refs := localAddressEvidence(p.Interfaces, in.Flow.Destination); len(refs) > 0 {
		stage("destination_nat").Status = "unresolved"
		stage("destination_nat").EvidenceRefs = appendUnique(nil, refs...)
		return finish("indeterminate", "flow destination addresses the firewall itself")
	}
	flow := in.Flow
	var dnat, snat *firewall.PathTranslation
	var selectedPolicy *firewall.PathPolicyRule
	var preRoute *firewall.PathRoute
	noNAT := false
	if established == nil && kind == vendorPAN {
		var why string
		preRoute, why = bestRoute(p.Routes, p.Interfaces, flow.Destination)
		if preRoute == nil {
			if strings.HasPrefix(why, "no supported connected or static route matches") {
				stage("route").Status = "blocked"
				return finish("blocked", why)
			}
			stage("route").Status = "unresolved"
			stage("route").Detail = why
			return finish("indeterminate", why)
		}
		preIface, _ := findVendorInterface(kind, p.Interfaces, preRoute.Interface)
		if ingress.Zone == "" || preIface.Zone == "" {
			stage("route").Status = "unresolved"
			return finish("indeterminate", "PAN-OS ingress or pre-NAT egress zone is unresolved")
		}
		stage("route").EvidenceRefs = appendUnique(nil, preRoute.EvidenceRefs...)
		add(preRoute.EvidenceRefs...)
		chosen, err := selectTranslation(p.Translations, ingress.Zone, preIface.Zone, flow, "")
		if err != nil {
			stage("destination_nat").Status = "unresolved"
			return finish("indeterminate", err.Error())
		}
		if chosen != nil {
			if chosen.Kind == "dnat" {
				dnat = chosen
			} else if chosen.Kind == "snat" {
				snat = chosen
			} else if chosen.Kind == "none" {
				noNAT = true
			}
		}
	} else if established == nil && kind == vendorForti {
		// A FortiGate VIP is usable only through a policy that names that VIP.
		for i := range p.Policies {
			r := &p.Policies[i]
			if !vendorNameMatch(kind, r.From, in.IngressInterface) || r.VIP == "" {
				continue
			}
			tr, n := findTranslation(p.Translations, r.VIP)
			if n != 1 {
				continue
			}
			if tr.Kind != "dnat" || !vendorNameMatch(kind, tr.From, in.IngressInterface) {
				continue
			}
			matched, err := translationMatches(*tr, flow, in.Scope)
			if err != nil {
				stage("destination_nat").Status = "unresolved"
				return finish("indeterminate", err.Error())
			}
			if matched {
				if dnat != nil && dnat.ID != tr.ID {
					stage("destination_nat").Status = "unresolved"
					return finish("indeterminate", "multiple active VIP translations may match")
				}
				dnat = tr
				selectedPolicy = r
			}
		}
		if dnat == nil {
			for i := range p.Translations {
				t := &p.Translations[i]
				if t.Kind != "dnat" {
					continue
				}
				matched, err := translationMatches(*t, in.Flow, in.Scope)
				if err != nil {
					stage("destination_nat").Status = "unresolved"
					return finish("indeterminate", err.Error())
				}
				if matched {
					stage("destination_nat").Status = "unresolved"
					stage("destination_nat").CandidateIDs = []string{t.ID}
					return finish("indeterminate", "a matching FortiGate VIP is not authorized by a uniquely selected VIP policy")
				}
			}
		}
	}
	if dnat != nil {
		translated, err := applyTranslation(flow, *dnat, true)
		if err != nil {
			stage("destination_nat").Status = "unresolved"
			return finish("indeterminate", err.Error())
		}
		before := flow
		flow = translated
		stage("destination_nat").Status = "matched"
		stage("destination_nat").Detail = "destination translated using the original packet tuple"
		stage("destination_nat").Before = copyFlow(before)
		stage("destination_nat").After = copyFlow(flow)
		stage("destination_nat").CandidateIDs = []string{dnat.ID}
		stage("destination_nat").EvidenceRefs = appendUnique(nil, dnat.EvidenceRefs...)
		res.NATRuleIDs = append(res.NATRuleIDs, dnat.ID)
		add(dnat.EvidenceRefs...)
	} else {
		stage("destination_nat").Status = "not_applied"
		stage("destination_nat").Detail = "no supported destination translation matched"
		if noNAT {
			stage("destination_nat").Detail = "explicit no-NAT rule matched and terminated ordered translation selection"
			for i := range p.Translations {
				if p.Translations[i].Kind == "none" {
					m, _ := translationMatches(p.Translations[i], in.Flow, in.Scope)
					if m {
						stage("destination_nat").CandidateIDs = []string{p.Translations[i].ID}
						stage("destination_nat").EvidenceRefs = appendUnique(nil, p.Translations[i].EvidenceRefs...)
						add(p.Translations[i].EvidenceRefs...)
						break
					}
				}
			}
		}
	}
	res.PolicyFlow = flow
	if localAddressEvidence(p.Interfaces, flow.Destination) != nil {
		if refs := localAddressEvidence(p.Interfaces, flow.Destination); len(refs) > 0 {
			stage("ingress_policy").Status = "unresolved"
			stage("ingress_policy").EvidenceRefs = appendUnique(nil, refs...)
			return finish("indeterminate", "translated destination addresses the firewall itself")
		}
	}
	var chosenRoute *firewall.PathRoute
	var routeWhy string
	chosenRoute, routeWhy = bestRoute(p.Routes, p.Interfaces, flow.Destination)
	if chosenRoute == nil {
		if strings.HasPrefix(routeWhy, "no supported connected or static route matches") {
			stage("route").Status = "blocked"
			return finish("blocked", routeWhy)
		}
		stage("route").Status = "unresolved"
		stage("route").Detail = routeWhy
		return finish("indeterminate", routeWhy)
	}
	egress, n := findVendorInterface(kind, p.Interfaces, chosenRoute.Interface)
	if n != 1 || !egress.Resolved || egress.Shutdown {
		stage("route").Status = "unresolved"
		return finish("indeterminate", "selected egress interface is unresolved or disabled")
	}
	stage("route").Status = "matched"
	stage("route").Detail = "selected longest-prefix route for the post-translation destination"
	stage("route").CandidateIDs = []string{chosenRoute.ID}
	if preRoute != nil {
		stage("route").CandidateIDs = appendUnique(stage("route").CandidateIDs, preRoute.ID)
	}
	routeRefs := append([]string{}, chosenRoute.EvidenceRefs...)
	routeRefs = append(routeRefs, egress.EvidenceRefs...)
	stage("route").EvidenceRefs = appendUnique(stage("route").EvidenceRefs, routeRefs...)
	res.SelectedRoute = copyRoute(chosenRoute)
	add(chosenRoute.EvidenceRefs...)
	add(egress.EvidenceRefs...)
	from, to := ingress.Zone, egress.Zone
	if kind == vendorForti {
		from, to = in.IngressInterface, egress.Name
	}
	if vendorNameMatch(kind, in.IngressInterface, egress.Name) {
		stage("egress").Status = "unresolved"
		return finish("indeterminate", "same-interface hairpin forwarding is outside the vendor path model")
	}
	if established != nil {
		stage("ingress_policy").Status = "matched"
		stage("ingress_policy").Detail = "reverse packet uses established state from the modeled forward permit"
		if established.Policy != nil {
			stage("ingress_policy").EvidenceRefs = appendUnique(nil, established.Policy.EvidenceRefs...)
			add(established.Policy.EvidenceRefs...)
		}
		res.Policy = established.Policy
	} else {
		policyFlow := in.Flow
		if kind == vendorForti {
			policyFlow = flow
		}
		pr, err := evaluateVendorPolicy(p.Policies, in.Scope, from, to, policyFlow, kind)
		if err != nil {
			stage("ingress_policy").Status = "unresolved"
			return finish("indeterminate", err.Error())
		}
		res.Policy = pr
		if pr != nil {
			stage("ingress_policy").EvidenceRefs = appendUnique(nil, pr.EvidenceRefs...)
			add(pr.EvidenceRefs...)
			for i := range p.Policies {
				if p.Policies[i].ID == pr.DecisiveRule {
					selectedPolicy = &p.Policies[i]
					break
				}
			}
		}
		if pr == nil || pr.Disposition != "permit" {
			if pr != nil && pr.Disposition == "deny" && !pr.UsedDefault {
				stage("ingress_policy").Status = "blocked"
				return finish("blocked", "first matching vendor policy denies the policy packet tuple")
			}
			if pr != nil && pr.Disposition == "deny" && pr.UsedDefault {
				stage("ingress_policy").Status = "blocked"
				stage("ingress_policy").Detail = "complete policy has no matching rule; default deny applies"
				stage("ingress_policy").EvidenceRefs = appendUnique(stage("ingress_policy").EvidenceRefs, p.EvidenceRefs...)
				add(p.EvidenceRefs...)
				return finish("blocked", "complete vendor policy default denies the flow")
			}
			if pr == nil {
				stage("ingress_policy").Status = "blocked"
				stage("ingress_policy").Detail = "complete policy has no matching rule; default deny applies"
				stage("ingress_policy").EvidenceRefs = appendUnique(nil, p.EvidenceRefs...)
				add(p.EvidenceRefs...)
				return finish("blocked", "complete vendor policy default denies the flow")
			}
			stage("ingress_policy").Status = "unresolved"
			return finish("indeterminate", "vendor policy does not establish a permit")
		}
		if kind == vendorForti && dnat != nil && (selectedPolicy == nil || selectedPolicy.VIP != dnat.ID) {
			stage("ingress_policy").Status = "unresolved"
			return finish("indeterminate", "a plain or mismatched FortiGate policy cannot authorize the selected VIP")
		}
		stage("ingress_policy").Status = "matched"
		stage("ingress_policy").Detail = "ordered zone or interface policy permits the original packet tuple"
		if kind == vendorForti {
			stage("ingress_policy").Detail = "ordered interface policy permits the post-DNAT packet tuple and selected VIP dependency"
		}
	}
	if dnat != nil && snat != nil {
		stage("source_nat").Status = "unresolved"
		return finish("indeterminate", "composed source and destination translation is unsupported")
	}
	if established == nil && !noNAT {
		for i := range p.Translations {
			t := &p.Translations[i]
			if kind == vendorPAN || t.Kind != "snat" {
				continue
			}
			if selectedPolicy == nil || selectedPolicy.SourceNAT != t.ID || (t.PolicyID != "" && t.PolicyID != selectedPolicy.ID) {
				continue
			}
			if !vendorNameMatch(kind, t.From, from) || !vendorNameMatch(kind, t.To, to) {
				continue
			}
			match, err := translationMatches(*t, in.Flow, in.Scope)
			if err != nil {
				stage("source_nat").Status = "unresolved"
				return finish("indeterminate", err.Error())
			}
			if match {
				if snat != nil {
					stage("source_nat").Status = "unresolved"
					return finish("indeterminate", "multiple source translations may match")
				}
				snat = t
			}
		}
		if kind == vendorForti && selectedPolicy != nil && selectedPolicy.SourceNAT != "" {
			bound, n := findTranslation(p.Translations, selectedPolicy.SourceNAT)
			if n != 1 || bound.Kind != "snat" || (bound.PolicyID != "" && bound.PolicyID != selectedPolicy.ID) || !vendorNameMatch(kind, bound.From, from) || !vendorNameMatch(kind, bound.To, to) {
				stage("source_nat").Status = "unresolved"
				return finish("indeterminate", "selected FortiGate policy source NAT reference is missing or inconsistent")
			}
			matched, err := translationMatches(*bound, in.Flow, in.Scope)
			if err != nil || !matched {
				stage("source_nat").Status = "unresolved"
				return finish("indeterminate", "selected FortiGate policy source NAT does not match the original tuple")
			}
			snat = bound
		}
	}
	if snat != nil {
		if dnat != nil {
			stage("source_nat").Status = "unresolved"
			return finish("indeterminate", "composed source and destination translation is unsupported")
		}
		after, err := applyTranslation(flow, *snat, false)
		if err != nil {
			stage("source_nat").Status = "unresolved"
			return finish("indeterminate", err.Error())
		}
		before := flow
		flow = after
		stage("source_nat").Status = "matched"
		stage("source_nat").Detail = "source translated after policy evaluation"
		stage("source_nat").Before = copyFlow(before)
		stage("source_nat").After = copyFlow(flow)
		stage("source_nat").CandidateIDs = []string{snat.ID}
		stage("source_nat").EvidenceRefs = appendUnique(nil, snat.EvidenceRefs...)
		res.NATRuleIDs = append(res.NATRuleIDs, snat.ID)
		add(snat.EvidenceRefs...)
	} else {
		stage("source_nat").Status = "not_applied"
		stage("source_nat").Detail = "no supported source translation matched"
	}
	if established != nil {
		res.NATRuleIDs = append([]string(nil), established.NATRuleIDs...)
	}
	stage("egress").Status = "matched"
	stage("egress").Detail = "packet exits through the selected enabled interface"
	stage("egress").Before = copyFlow(flow)
	stage("egress").After = copyFlow(flow)
	stage("egress").EvidenceRefs = appendUnique(nil, egress.EvidenceRefs...)
	res.EgressInterface = egress.Name
	res.PolicyFlow = in.Flow
	if established == nil && res.Policy != nil {
		res.PolicyFlow = res.Policy.Flow
	}
	res.EgressFlow = flow
	return finish("modeled_forward", "")
}

func vendorReturn(in Input, forward Result) (Result, error) {
	if forward.EngineContractVersion != VendorContractVersion || forward.Disposition != "modeled_forward" || forward.Policy == nil || forward.Policy.Disposition != "permit" {
		return Result{}, fmt.Errorf("forward result must establish a modeled permitted vendor path")
	}
	proto := strings.ToLower(strings.TrimSpace(in.Flow.Protocol))
	if proto != "tcp" && proto != "udp" && proto != "6" && proto != "17" {
		return Result{}, fmt.Errorf("established return modeling supports TCP and UDP only")
	}
	kind, _ := identifyVendor(in.Config.Vendor)
	if in.Scope != forward.Scope || !vendorNameMatch(kind, in.IngressInterface, forward.EgressInterface) || in.Flow != reverseFlow(forward.EgressFlow, in.Scope) {
		return Result{}, fmt.Errorf("return scope, ingress, and tuple must exactly reverse the forward path")
	}
	verified, err := vendorEvaluate(Input{Scope: forward.Scope, Config: in.Config, IngressInterface: forward.IngressInterface, Flow: forward.OriginalFlow}, nil)
	if err != nil || !reflect.DeepEqual(verified, forward) {
		return Result{}, fmt.Errorf("forward result does not match supplied configuration evidence")
	}
	p := in.Config.Path
	flow := in.Flow
	policyFlow := flow
	var reverseSNAT *firewall.PathTranslation
	var reverseDNAT *firewall.PathTranslation
	for _, id := range forward.NATRuleIDs {
		t, n := findTranslationID(p.Translations, id)
		if n != 1 {
			return Result{}, fmt.Errorf("forward translation evidence is missing or ambiguous")
		}
		switch t.Kind {
		case "snat":
			reverseSNAT = t
		case "dnat":
			reverseDNAT = t
		}
	}
	// Undo forward SNAT on the reply destination before selecting its route.
	if reverseSNAT != nil {
		var e error
		flow, e = applyTranslation(flow, *reverseSNAT, true)
		if e != nil {
			return Result{}, e
		}
	}
	policyFlow = flow
	res, err := vendorEvaluate(Input{Scope: in.Scope, Config: in.Config, IngressInterface: in.IngressInterface, Flow: flow}, &forward)
	if err != nil {
		return res, err
	}
	res.OriginalFlow = in.Flow
	res.PolicyFlow = policyFlow
	res.NATRuleIDs = append([]string(nil), forward.NATRuleIDs...)
	if reverseSNAT != nil {
		st := stageByName(res.Stages, "destination_nat")
		st.Status = "matched"
		st.Detail = "return destination was restored through the forward source NAT mapping"
		st.Before = copyFlow(in.Flow)
		st.After = copyFlow(policyFlow)
		st.CandidateIDs = []string{reverseSNAT.ID}
		st.EvidenceRefs = appendUnique(nil, reverseSNAT.EvidenceRefs...)
		res.EvidenceRefs = appendUnique(res.EvidenceRefs, reverseSNAT.EvidenceRefs...)
	}
	if reverseDNAT != nil {
		before := policyFlow
		if res.Disposition == "modeled_forward" {
			before = res.EgressFlow
		}
		after, e := applyTranslation(before, *reverseDNAT, false)
		if e != nil {
			return Result{}, e
		}
		st := stageByName(res.Stages, "source_nat")
		st.Status = "matched"
		st.Detail = "return source was restored through the forward destination NAT mapping"
		st.Before = copyFlow(before)
		st.After = copyFlow(after)
		st.CandidateIDs = []string{reverseDNAT.ID}
		st.EvidenceRefs = appendUnique(nil, reverseDNAT.EvidenceRefs...)
		res.EvidenceRefs = appendUnique(res.EvidenceRefs, reverseDNAT.EvidenceRefs...)
		if res.Disposition == "modeled_forward" {
			res.EgressFlow = after
		}
	}
	if res.Disposition != "modeled_forward" {
		return res, nil
	}
	if !vendorNameMatch(kind, res.EgressInterface, forward.IngressInterface) {
		res.Disposition = "indeterminate"
		res.EgressFlow = policy.Flow{}
		res.Reasons = append(res.Reasons, "return route does not use the forward ingress interface")
		st := stageByName(res.Stages, "egress")
		st.Status = "unresolved"
		st.Detail = "return route is asymmetric with the forward ingress interface"
		return res, nil
	}
	if res.EgressFlow != reverseFlow(forward.OriginalFlow, in.Scope) {
		res.Disposition = "indeterminate"
		res.EgressFlow = policy.Flow{}
		res.Reasons = append(res.Reasons, "return translation is not an exact inverse of the forward tuple")
		st := stageByName(res.Stages, "egress")
		st.Status = "unresolved"
		st.Detail = "return translation did not restore the exact forward tuple"
		return res, nil
	}
	for i := range res.Stages {
		if res.Stages[i].Name == "egress" {
			res.Stages[i].After = copyFlow(res.EgressFlow)
		}
	}
	res.Assumptions = append(res.Assumptions, "reverse policy handling relies on the supplied modeled forward TCP/UDP permit as established session state")
	return res, nil
}

func stageByName(stages []Stage, name string) *Stage {
	for i := range stages {
		if stages[i].Name == name {
			return &stages[i]
		}
	}
	return nil
}

func findTranslation(all []firewall.PathTranslation, id string) (*firewall.PathTranslation, int) {
	var out *firewall.PathTranslation
	n := 0
	for i := range all {
		if all[i].ID == id {
			out = &all[i]
			n++
		}
	}
	return out, n
}
func findTranslationID(all []firewall.PathTranslation, id string) (*firewall.PathTranslation, int) {
	return findTranslation(all, id)
}
func translationMatches(t firewall.PathTranslation, f policy.Flow, s policy.Scope) (bool, error) {
	if t.ID == "" || t.Kind != "snat" && t.Kind != "dnat" && t.Kind != "none" {
		return false, fmt.Errorf("translation has invalid identifier or kind")
	}
	r := t.Match
	if r.ID == "" {
		return false, fmt.Errorf("translation match is incomplete")
	}
	if !r.Enabled {
		return false, nil
	}
	r.Action = "permit"
	r.Position = 0
	r.Unsupported = append([]string(nil), r.Unsupported...)
	flow := f
	flow.Scope = s
	b := policy.Bundle{Scope: s, Complete: true, DefaultAction: "deny", Rules: []policy.Rule{r}}
	got, e := policy.Evaluate(b, flow)
	if e != nil {
		return false, e
	}
	if got.Disposition == "indeterminate" {
		return false, fmt.Errorf("translation match contains unsupported semantics")
	}
	return got.Disposition == "permit", nil
}

func validateTranslationMatch(t firewall.PathTranslation, s policy.Scope) error {
	r := t.Match
	if r.ID == "" || strings.TrimSpace(r.Source) == "" || strings.TrimSpace(r.Destination) == "" || strings.TrimSpace(r.Protocol) == "" {
		return fmt.Errorf("translation %s has an incomplete match predicate", t.ID)
	}
	r.Action = "permit"
	r.Position = 0
	f := policy.Flow{Scope: s, Source: "192.0.2.1", Destination: "192.0.2.2", Protocol: "tcp", SourcePort: 12345, DestinationPort: 443}
	if err := policy.Validate(policy.Bundle{Scope: s, Complete: true, DefaultAction: "deny", Rules: []policy.Rule{r}}, f); err != nil {
		return fmt.Errorf("translation %s match is invalid: %w", t.ID, err)
	}
	return nil
}
func selectTranslation(all []firewall.PathTranslation, from, to string, f policy.Flow, onlyID string) (*firewall.PathTranslation, error) {
	seen := map[string]bool{}
	for i := range all {
		t := &all[i]
		if t.ID == "" || seen[t.ID] {
			return nil, fmt.Errorf("translation identifiers are missing or duplicated")
		}
		seen[t.ID] = true
		if onlyID != "" && t.ID != onlyID {
			continue
		}
		if !vendorNameMatch(vendorPAN, t.From, from) || !vendorNameMatch(vendorPAN, t.To, to) {
			continue
		}
		m, e := translationMatches(*t, f, f.Scope)
		if e != nil {
			return nil, e
		}
		if m {
			return t, nil
		}
	}
	return nil, nil
}
func applyTranslation(f policy.Flow, t firewall.PathTranslation, destination bool) (policy.Flow, error) {
	fromS, toS := t.OriginalPrefix, t.TranslatedPrefix
	if destination && t.Kind == "dnat" || !destination && t.Kind == "snat" {
		fromS, toS = t.OriginalPrefix, t.TranslatedPrefix
	} else {
		fromS, toS = t.TranslatedPrefix, t.OriginalPrefix
	}
	from, e := netip.ParsePrefix(fromS)
	if e != nil || !from.Addr().Is4() || from != from.Masked() {
		return f, fmt.Errorf("translation has invalid original prefix")
	}
	to, e := netip.ParsePrefix(toS)
	if e != nil || !to.Addr().Is4() || to != to.Masked() || from.Bits() != to.Bits() {
		return f, fmt.Errorf("translation is not a supported equal-size IPv4 prefix mapping")
	}
	value := f.Source
	if destination {
		value = f.Destination
	}
	addr, e := netip.ParseAddr(value)
	if e != nil {
		return f, fmt.Errorf("translation address is invalid")
	}
	mapped, ok := translate(addr, from, to)
	if !ok {
		return f, fmt.Errorf("translation address is outside the configured prefix")
	}
	if destination {
		f.Destination = mapped.String()
	} else {
		f.Source = mapped.String()
	}
	return f, nil
}
func evaluateVendorPolicy(all []firewall.PathPolicyRule, s policy.Scope, from, to string, f policy.Flow, kind vendorKind) (*policy.Result, error) {
	rules := []policy.Rule{}
	refs := []string{}
	ids := map[string]bool{}
	for i := range all {
		x := all[i]
		if !vendorNameMatch(kind, x.From, from) || !vendorNameMatch(kind, x.To, to) {
			continue
		}
		r := x.Match
		if r.ID == "" || x.ID == "" {
			return nil, fmt.Errorf("vendor policy has a missing identifier")
		}
		r.ID = x.ID
		if ids[x.ID] {
			return nil, fmt.Errorf("vendor policy rule identifiers are duplicated")
		}
		ids[x.ID] = true
		if r.Position < 0 {
			r.Position = i
		}
		r.EvidenceRefs = appendUnique(r.EvidenceRefs, x.EvidenceRefs...)
		rules = append(rules, r)
		refs = appendUnique(refs, x.EvidenceRefs...)
	}
	if len(rules) == 0 {
		return nil, nil
	}
	ctx := s
	ctx.Context += "/vendor-policy/" + from + "/" + to
	flow := f
	flow.Scope = ctx
	result, e := policy.Evaluate(policy.Bundle{Scope: ctx, Complete: true, DefaultAction: "deny", Rules: rules}, flow)
	if e != nil {
		return nil, e
	}
	result.EvidenceRefs = appendUnique(result.EvidenceRefs, refs...)
	return &result, nil
}

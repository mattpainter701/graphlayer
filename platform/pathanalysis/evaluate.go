package pathanalysis

import (
	"fmt"
	"net/netip"
	"reflect"
	"sort"
	"strings"

	"github.com/mattpainter701/graphlayer/platform/firewall"
	"github.com/mattpainter701/graphlayer/platform/policy"
)

// Evaluate explains one configuration-derived, single-device forward path.
func Evaluate(in Input) (Result, error) { return evaluate(in, nil) }

// EvaluateReturn evaluates the reverse leg of a previously modeled path. It
// bypasses only the new inbound ACL decision, relying on the supplied forward
// permit as established TCP/UDP state, and requires the exact reverse tuple and
// exact inverse NAT result before returning modeled_forward.
func EvaluateReturn(in Input, forward Result) (Result, error) {
	if forward.Disposition != "modeled_forward" || forward.Policy == nil || forward.Policy.Disposition != "permit" {
		return Result{}, fmt.Errorf("forward result must establish a modeled permitted path")
	}
	if in.Scope != forward.Scope || !strings.EqualFold(in.IngressInterface, forward.EgressInterface) {
		return Result{}, fmt.Errorf("return scope and ingress must match the forward egress")
	}
	protocol := strings.ToLower(strings.TrimSpace(in.Flow.Protocol))
	if protocol != "tcp" && protocol != "udp" && protocol != "6" && protocol != "17" {
		return Result{}, fmt.Errorf("established return modeling supports TCP and UDP only")
	}
	want := reverseFlow(forward.EgressFlow, in.Scope)
	if in.Flow != want {
		return Result{}, fmt.Errorf("return tuple must exactly reverse the forward egress tuple")
	}
	verified, err := Evaluate(Input{Scope: forward.Scope, Config: in.Config, IngressInterface: forward.IngressInterface, Flow: forward.OriginalFlow})
	if err != nil || !reflect.DeepEqual(verified, forward) {
		return Result{}, fmt.Errorf("forward result does not match the supplied configuration evidence")
	}
	res, err := evaluate(in, &forward)
	if err != nil || res.Disposition != "modeled_forward" {
		return res, err
	}
	if !strings.EqualFold(res.EgressInterface, forward.IngressInterface) {
		res.Disposition = "indeterminate"
		res.EgressFlow = policy.Flow{}
		res.Reasons = append(res.Reasons, "return route does not use the forward ingress interface")
		for i := range res.Stages {
			if res.Stages[i].Name == "egress" {
				res.Stages[i].Status = "unresolved"
				res.Stages[i].Detail = "return route is asymmetric with the forward ingress interface"
			}
		}
		return res, nil
	}
	if res.EgressFlow != reverseFlow(forward.OriginalFlow, in.Scope) || !sameIDs(res.NATRuleIDs, forward.NATRuleIDs) {
		res.Disposition = "indeterminate"
		res.EgressFlow = policy.Flow{}
		res.Reasons = append(res.Reasons, "return translation does not exactly reverse the modeled forward tuple")
		for i := range res.Stages {
			if res.Stages[i].Name == "egress" {
				res.Stages[i].Status = "unresolved"
				res.Stages[i].Detail = "return translation is not an exact inverse"
			}
		}
		return res, nil
	}
	filtered := res.Assumptions[:0]
	for _, a := range res.Assumptions {
		if !strings.Contains(a, "does not establish live device state, return traffic") {
			filtered = append(filtered, a)
		}
	}
	res.Assumptions = append(filtered, "reverse ACL handling relies on the supplied modeled forward TCP/UDP permit as established session state")
	return res, nil
}

func reverseFlow(f policy.Flow, scope policy.Scope) policy.Flow {
	f.Scope = scope
	f.Source, f.Destination = f.Destination, f.Source
	f.SourcePort, f.DestinationPort = f.DestinationPort, f.SourcePort
	return f
}
func sameIDs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	counts := map[string]int{}
	for _, v := range a {
		counts[v]++
	}
	for _, v := range b {
		counts[v]--
		if counts[v] < 0 {
			return false
		}
	}
	for _, v := range counts {
		if v != 0 {
			return false
		}
	}
	return true
}

func evaluate(in Input, established *Result) (Result, error) {
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
	if !netip.MustParseAddr(in.Flow.Source).Is4() || !netip.MustParseAddr(in.Flow.Destination).Is4() {
		return Result{}, fmt.Errorf("path analysis currently requires IPv4 source and destination")
	}
	if !isTransitAddress(netip.MustParseAddr(in.Flow.Source)) || !isTransitAddress(netip.MustParseAddr(in.Flow.Destination)) {
		return Result{}, fmt.Errorf("flow source and destination must be unicast transit addresses")
	}
	res := Result{EngineContractVersion: ContractVersion, Scope: in.Scope, IngressInterface: in.IngressInterface, OriginalFlow: in.Flow, Disposition: "indeterminate", Stages: []Stage{}, Reasons: []string{}, Assumptions: []string{"single-device forward-path model derived from supplied configuration evidence", "supplied configuration is assumed complete for the modeled interfaces, ACLs, routes, and NAT; absent statements are not independently verified", "configured interface and next-hop facts do not establish operational link, ARP, or neighbor state", "does not establish live device state, return traffic, or multi-hop reachability"}}
	for _, name := range []string{"ingress", "destination_nat", "ingress_policy", "route", "source_nat", "egress"} {
		res.Stages = append(res.Stages, Stage{Name: name, Status: "not_evaluated"})
	}
	finish := func(disposition, reason string) (Result, error) {
		res.Disposition = disposition
		if reason != "" {
			res.Reasons = append(res.Reasons, reason)
		}
		return res, nil
	}
	stage := func(name string) *Stage {
		for i := range res.Stages {
			if res.Stages[i].Name == name {
				return &res.Stages[i]
			}
		}
		panic("missing fixed stage")
	}
	addEvidence := func(refs ...string) { res.EvidenceRefs = appendUnique(res.EvidenceRefs, refs...) }
	if in.Config.Context != in.Scope.Context {
		return finish("indeterminate", "configuration context does not match the requested scope")
	}
	if in.Config.Path == nil {
		return finish("indeterminate", "configuration has no normalized path model")
	}
	if !strings.EqualFold(in.Config.Vendor, "cisco-asa") {
		return finish("indeterminate", "path model is not identified as Cisco ASA configuration")
	}
	p := in.Config.Path
	addEvidence(p.EvidenceRefs...)
	stage("destination_nat").EvidenceRefs = appendUnique(p.EvidenceRefs)
	stage("route").EvidenceRefs = appendUnique(p.EvidenceRefs)
	stage("source_nat").EvidenceRefs = appendUnique(p.EvidenceRefs)
	if p.Version != "asa-path-v1" {
		return finish("indeterminate", "path configuration version is unsupported")
	}
	for _, warnings := range [][]firewall.Warning{in.Config.Warnings, p.Warnings} {
		for _, warning := range warnings {
			if warning.Code == "ambiguous_context" {
				stage("ingress").Status = "unresolved"
				stage("ingress").Detail = "configuration context is ambiguous"
				return finish("indeterminate", "configuration evidence has an ambiguous context")
			}
		}
	}
	ingress, n := findInterface(p.Interfaces, in.IngressInterface)
	if n == 0 {
		return Result{}, fmt.Errorf("ingress_interface is not present in the supplied path model")
	}
	if n != 1 {
		stage("ingress").Status = "unresolved"
		stage("ingress").Detail = "ingress interface is missing or ambiguous"
		return finish("indeterminate", "explicit ingress interface could not be resolved")
	}
	stage("ingress").EvidenceRefs = appendUnique(ingress.EvidenceRefs)
	addEvidence(ingress.EvidenceRefs...)
	if !ingress.Resolved || ingress.Shutdown {
		stage("ingress").Status = "unresolved"
		stage("ingress").Detail = "ingress interface resolution or enabled state is not established"
		return finish("indeterminate", "ingress interface cannot be established as usable")
	}
	stage("ingress").Status = "matched"
	stage("ingress").Detail = "explicit ingress interface is resolved and enabled"
	res.IngressInterface = ingress.Name
	if refs := localAddressEvidence(p.Interfaces, in.Flow.Destination); len(refs) > 0 {
		stage("destination_nat").Status = "unresolved"
		stage("destination_nat").Detail = "destination is an ASA interface address; control-plane traffic is outside the transit path model"
		stage("destination_nat").EvidenceRefs = appendUnique(stage("destination_nat").EvidenceRefs, refs...)
		return finish("indeterminate", "flow destination addresses the ASA itself")
	}
	if !p.NATComplete {
		stage("destination_nat").Status = "unresolved"
		stage("destination_nat").Detail = "NAT evidence is incomplete"
		return finish("indeterminate", "incomplete NAT evidence may change the translated tuple")
	}
	flow := in.Flow
	var dnat, snat *firewall.PathNATRule
	orderedNAT := append([]firewall.PathNATRule(nil), p.NATRules...)
	for i := range p.NATRules {
		if err := validateNAT(p.NATRules[i]); err != nil {
			stage("destination_nat").Status = "unresolved"
			stage("destination_nat").Detail = "NAT rule uses unsupported or invalid translation semantics"
			stage("destination_nat").CandidateIDs = []string{p.NATRules[i].ID}
			return finish("indeterminate", "NAT evidence cannot be interpreted safely")
		}
	}
	// ASA section-2 object NAT evaluates static before dynamic and orders
	// overlapping static objects by the size of their real address range, then
	// numeric address, then object name. Dynamic NAT is excluded by this model.
	sort.SliceStable(orderedNAT, func(i, j int) bool { return natLess(orderedNAT[i], orderedNAT[j]) })
	for i := range orderedNAT {
		r := &orderedNAT[i]
		if dnat == nil && matchesDNAT(*r, in.IngressInterface, flow.Destination) {
			dnat = r
		}
		if snat == nil && matchesSNAT(*r, in.IngressInterface, flow.Source) {
			snat = r
		}
	}
	if dnat != nil && snat != nil {
		stage("destination_nat").Status = "unresolved"
		stage("destination_nat").Detail = "simultaneous source and destination translation is unsupported"
		return finish("indeterminate", "composed source and destination NAT semantics are not modeled")
	}
	if dnat != nil {
		from, _ := netip.ParsePrefix(dnat.MappedPrefix)
		to, _ := netip.ParsePrefix(dnat.RealPrefix)
		translated, ok := translate(netip.MustParseAddr(flow.Destination), from, to)
		if !ok {
			return finish("indeterminate", "destination NAT prefix translation could not be proven")
		}
		before := flow
		flow.Destination = translated.String()
		stage("destination_nat").Status = "matched"
		stage("destination_nat").Detail = "static object NAT destination was untranslated to the real address; NAT selects the real interface"
		stage("destination_nat").Before = copyFlow(before)
		stage("destination_nat").After = copyFlow(flow)
		stage("destination_nat").CandidateIDs = []string{dnat.ID}
		stage("destination_nat").EvidenceRefs = appendUnique(dnat.EvidenceRefs)
		res.NATRuleIDs = append(res.NATRuleIDs, dnat.ID)
		addEvidence(dnat.EvidenceRefs...)
		res.PolicyFlow = flow
		if refs := localAddressEvidence(p.Interfaces, flow.Destination); len(refs) > 0 {
			stage("ingress_policy").Status = "unresolved"
			stage("ingress_policy").Detail = "translated destination is an ASA interface address; control-plane traffic is outside the transit path model"
			stage("ingress_policy").EvidenceRefs = appendUnique(stage("ingress_policy").EvidenceRefs, refs...)
			addEvidence(refs...)
			return finish("indeterminate", "translated destination addresses the ASA itself")
		}
	} else {
		stage("destination_nat").Status = "not_applied"
		stage("destination_nat").Detail = "no supported destination static object NAT rule matched"
		stage("destination_nat").EvidenceRefs = appendUnique(p.EvidenceRefs)
	}
	res.PolicyFlow = flow
	if established != nil {
		// A reverse packet in the same modeled TCP/UDP session is admitted from
		// connection state. Carry the forward ACL evidence without evaluating a
		// second inbound connection against that ACL.
		stage("ingress_policy").Status = "matched"
		stage("ingress_policy").Detail = "reverse packet uses established state from the modeled forward permit"
		stage("ingress_policy").EvidenceRefs = appendUnique(nil, established.Policy.EvidenceRefs...)
		addEvidence(established.Policy.EvidenceRefs...)
		stage("ingress_policy").Before = copyFlow(flow)
		stage("ingress_policy").After = copyFlow(flow)
	} else {
		for _, warning := range in.Config.Warnings {
			if warning.Code == "unsupported_binding" || warning.Code == "ambiguous_binding" {
				stage("ingress_policy").Status = "unresolved"
				stage("ingress_policy").Detail = "configuration contains an unsupported or ambiguous access-group binding"
				return finish("indeterminate", "unsupported global or ambiguous access-group attachment may affect policy")
			}
		}
		for _, c := range in.Config.Chains {
			dir := strings.ToLower(strings.TrimSpace(c.Direction))
			if c.Interface == "" || (dir != "in" && dir != "out") {
				stage("ingress_policy").Status = "unresolved"
				stage("ingress_policy").Detail = "a global or unsupported ACL attachment is present"
				stage("ingress_policy").EvidenceRefs = appendUnique(stage("ingress_policy").EvidenceRefs, c.EvidenceRefs...)
				addEvidence(c.EvidenceRefs...)
				return finish("indeterminate", "global or unsupported ACL attachment is outside this model")
			}
		}
		chain, n := findIngressChain(in.Config.Chains, in.IngressInterface)
		if n != 1 {
			stage("ingress_policy").Status = "unresolved"
			stage("ingress_policy").Detail = "an explicit unambiguous inbound ACL is required"
			return finish("indeterminate", "ingress policy could not be selected")
		}
		res.ChainID = chain.ID
		if !chain.Complete {
			stage("ingress_policy").Status = "unresolved"
			stage("ingress_policy").Detail = "ingress ACL evidence is incomplete"
			return finish("indeterminate", "incomplete ingress ACL may change the policy decision")
		}
		bundle := policy.Bundle{Scope: policy.Scope{SnapshotID: in.Scope.SnapshotID, DeviceID: in.Scope.DeviceID, Context: in.Scope.Context + "/" + chain.ID}, Complete: true, DefaultAction: "deny", Rules: chain.Rules}
		flow.Scope = bundle.Scope
		res.PolicyFlow = flow
		decision, err := policy.Evaluate(bundle, flow)
		if err != nil {
			return Result{}, fmt.Errorf("ingress policy: %w", err)
		}
		decision.EvidenceRefs = appendUnique(decision.EvidenceRefs, chain.EvidenceRefs...)
		res.Policy = &decision
		stage("ingress_policy").Before = copyFlow(flow)
		stage("ingress_policy").After = copyFlow(flow)
		stage("ingress_policy").EvidenceRefs = appendUnique([]string{}, decision.EvidenceRefs...)
		addEvidence(decision.EvidenceRefs...)
		if decision.Disposition == "deny" {
			stage("ingress_policy").Status = "blocked"
			stage("ingress_policy").Detail = "ingress ACL denies the real-address tuple"
			return finish("blocked", "ingress ACL denies the forward flow")
		}
		if decision.Disposition != "permit" {
			stage("ingress_policy").Status = "unresolved"
			stage("ingress_policy").Detail = "ingress ACL evaluation is indeterminate"
			return finish("indeterminate", "ingress ACL semantics do not establish a permit")
		}
		stage("ingress_policy").Status = "matched"
		stage("ingress_policy").Detail = "ingress ACL permits the real-address tuple"
	}
	if !p.RoutingComplete {
		stage("route").Status = "unresolved"
		stage("route").Detail = "routing evidence is incomplete"
		return finish("indeterminate", "incomplete routing evidence may change the egress interface")
	}
	if reason := validateRouteInputs(p.Routes, p.Interfaces); reason != "" {
		stage("route").Status = "unresolved"
		stage("route").Detail = reason
		return finish("indeterminate", reason)
	}
	var chosen *firewall.PathRoute
	egress := ""
	if dnat != nil {
		egress = dnat.RealInterface
		chosen, n = connectedRoute(p.Routes, p.Interfaces, egress, flow.Destination)
		if n != 1 {
			stage("route").Status = "unresolved"
			stage("route").Detail = "destination NAT divert requires a directly connected route on the NAT real interface"
			return finish("indeterminate", "translated destination is not proven directly connected on the NAT real interface")
		}
		stage("route").Status = "matched"
		stage("route").Detail = "destination NAT selects its real interface and destination is directly connected"
	} else {
		var why string
		chosen, why = bestRoute(p.Routes, p.Interfaces, flow.Destination)
		if chosen == nil {
			stage("route").Status = "unresolved"
			stage("route").Detail = why
			if why == "no supported connected or static route matches the destination" {
				stage("route").Status = "blocked"
				return finish("blocked", why)
			}
			return finish("indeterminate", why)
		}
		egress = chosen.Interface
		stage("route").Status = "matched"
		stage("route").Detail = "longest-prefix route selected; lower administrative distance breaks prefix ties"
	}
	res.SelectedRoute = copyRoute(chosen)
	res.EgressInterface = egress
	stage("route").CandidateIDs = []string{chosen.ID}
	stage("route").EvidenceRefs = appendUnique(p.EvidenceRefs, chosen.EvidenceRefs...)
	addEvidence(chosen.EvidenceRefs...)
	if snat != nil {
		if !strings.EqualFold(snat.MappedInterface, egress) {
			stage("source_nat").Status = "unresolved"
			stage("source_nat").Detail = "source NAT mapped interface disagrees with selected route egress"
			return finish("indeterminate", "source NAT egress interface and route selection disagree")
		}
		from, _ := netip.ParsePrefix(snat.RealPrefix)
		to, _ := netip.ParsePrefix(snat.MappedPrefix)
		translated, ok := translate(netip.MustParseAddr(flow.Source), from, to)
		if !ok {
			return finish("indeterminate", "source NAT prefix translation could not be proven")
		}
		before := flow
		egressFlow := flow
		egressFlow.Source = translated.String()
		stage("source_nat").Status = "matched"
		stage("source_nat").Detail = "static object NAT source was translated after route selection"
		stage("source_nat").Before = copyFlow(before)
		stage("source_nat").After = copyFlow(egressFlow)
		stage("source_nat").CandidateIDs = []string{snat.ID}
		stage("source_nat").EvidenceRefs = appendUnique(snat.EvidenceRefs)
		res.NATRuleIDs = append(res.NATRuleIDs, snat.ID)
		addEvidence(snat.EvidenceRefs...)
		flow = egressFlow
	} else {
		stage("source_nat").Status = "not_applied"
		stage("source_nat").Detail = "no supported source static object NAT rule matched"
		stage("source_nat").EvidenceRefs = appendUnique(p.EvidenceRefs)
	}
	if strings.EqualFold(egress, in.IngressInterface) {
		stage("egress").Status = "unresolved"
		stage("egress").Detail = "same-interface hairpin forwarding is unsupported"
		return finish("indeterminate", "same-interface hairpin forwarding is outside this model")
	}
	out, n := findInterface(p.Interfaces, egress)
	if n != 1 || !out.Resolved || out.Shutdown {
		stage("egress").Status = "unresolved"
		stage("egress").Detail = "egress interface is missing, ambiguous, unresolved, or shutdown"
		return finish("indeterminate", "egress interface state is not established")
	}
	for _, c := range in.Config.Chains {
		if strings.EqualFold(c.Interface, egress) && strings.EqualFold(c.Direction, "out") {
			stage("egress").Status = "unresolved"
			stage("egress").Detail = "an egress ACL is present but egress policy evaluation is unsupported"
			return finish("indeterminate", "an egress ACL is present but egress policy evaluation is unsupported")
		}
	}
	stage("egress").EvidenceRefs = appendUnique(p.EvidenceRefs, out.EvidenceRefs...)
	addEvidence(out.EvidenceRefs...)
	if ingress.SecurityLevel == nil || out.SecurityLevel == nil {
		stage("egress").Status = "unresolved"
		stage("egress").Detail = "interface security levels are incomplete"
		return finish("indeterminate", "interface security-level behavior is not established")
	}
	if *ingress.SecurityLevel == *out.SecurityLevel && !p.InterInterface {
		stage("egress").Status = "blocked"
		stage("egress").Detail = "same-security inter-interface forwarding is disabled"
		return finish("blocked", "same-security inter-interface forwarding is disabled")
	}
	stage("egress").Status = "matched"
	stage("egress").Detail = "selected egress interface is resolved and enabled"
	finalFlow := flow
	finalFlow.Scope = in.Scope
	res.EgressFlow = finalFlow
	stage("egress").Before = copyFlow(finalFlow)
	stage("egress").After = copyFlow(finalFlow)
	return finish("modeled_forward", "configuration-derived stages establish a modeled forward path on this device")
}

func validateNAT(r firewall.PathNATRule) error {
	if r.ID == "" || r.ObjectName == "" || r.RealInterface == "" || r.MappedInterface == "" {
		return fmt.Errorf("missing NAT identity or interface")
	}
	if r.Protocol != "" || r.RealPort != 0 || r.MappedPort != 0 {
		return fmt.Errorf("service NAT or PAT is unsupported")
	}
	real, err := netip.ParsePrefix(r.RealPrefix)
	if err != nil || !real.Addr().Is4() || real != real.Masked() {
		return fmt.Errorf("invalid real IPv4 prefix")
	}
	mapped, err := netip.ParsePrefix(r.MappedPrefix)
	if err != nil || !mapped.Addr().Is4() || mapped != mapped.Masked() || mapped.Bits() != real.Bits() {
		return fmt.Errorf("invalid mapped IPv4 prefix")
	}
	return nil
}

func natLess(a, b firewall.PathNATRule) bool {
	pa, ea := netip.ParsePrefix(a.RealPrefix)
	pb, eb := netip.ParsePrefix(b.RealPrefix)
	if ea == nil && eb == nil && pa.Bits() != pb.Bits() {
		return pa.Bits() > pb.Bits()
	}
	if ea == nil && eb == nil && pa.Addr() != pb.Addr() {
		return pa.Addr().Less(pb.Addr())
	}
	if a.ObjectName != b.ObjectName {
		return a.ObjectName < b.ObjectName
	}
	return a.ID < b.ID
}

func matchesDNAT(r firewall.PathNATRule, ingress, destination string) bool {
	a, err := netip.ParseAddr(destination)
	p, pErr := netip.ParsePrefix(r.MappedPrefix)
	return err == nil && pErr == nil && strings.EqualFold(r.MappedInterface, ingress) && p.Contains(a)
}
func matchesSNAT(r firewall.PathNATRule, ingress, source string) bool {
	a, err := netip.ParseAddr(source)
	p, pErr := netip.ParsePrefix(r.RealPrefix)
	return err == nil && pErr == nil && strings.EqualFold(r.RealInterface, ingress) && p.Contains(a)
}
func translate(addr netip.Addr, from, to netip.Prefix) (netip.Addr, bool) {
	if !addr.Is4() || !from.Contains(addr) || from.Bits() != to.Bits() {
		return netip.Addr{}, false
	}
	b, x := from.Masked().Addr().As4(), addr.As4()
	delta := uint32(x[0]^b[0])<<24 | uint32(x[1]^b[1])<<16 | uint32(x[2]^b[2])<<8 | uint32(x[3]^b[3])
	t := to.Masked().Addr().As4()
	return netip.AddrFrom4([4]byte{t[0] ^ byte(delta>>24), t[1] ^ byte(delta>>16), t[2] ^ byte(delta>>8), t[3] ^ byte(delta)}), true
}

func bestRoute(routes []firewall.PathRoute, interfaces []firewall.PathInterface, destination string) (*firewall.PathRoute, string) {
	a, err := netip.ParseAddr(destination)
	if err != nil || !a.Is4() {
		return nil, "route destination is not valid IPv4"
	}
	var candidates []firewall.PathRoute
	longest := -1
	for _, r := range routes {
		p, e := netip.ParsePrefix(r.Prefix)
		kind := strings.ToLower(r.Kind)
		if e != nil || !p.Addr().Is4() || p != p.Masked() || !p.Contains(a) || r.Interface == "" || r.ID == "" || (kind != "connected" && kind != "static") {
			continue
		}
		if p.Bits() > longest {
			longest, candidates = p.Bits(), nil
		}
		if p.Bits() == longest {
			candidates = append(candidates, r)
		}
	}
	if len(candidates) == 0 {
		return nil, "no supported connected or static route matches the destination"
	}
	min := candidates[0].Distance
	for _, r := range candidates[1:] {
		if r.Distance < min {
			min = r.Distance
		}
	}
	var best []firewall.PathRoute
	for _, r := range candidates {
		if r.Distance == min {
			best = append(best, r)
		}
	}
	if len(best) != 1 {
		return nil, "multiple equal-best routes match the destination"
	}
	r := best[0]
	if strings.EqualFold(r.Kind, "static") {
		nh, e := netip.ParseAddr(r.NextHop)
		if e != nil || !nh.Is4() {
			return nil, "selected static route has no valid IPv4 next hop"
		}
		valid := false
		for _, c := range routes {
			p, e := netip.ParsePrefix(c.Prefix)
			if e == nil && strings.EqualFold(c.Kind, "connected") && strings.EqualFold(c.Interface, r.Interface) && usableNextHop(nh, p) && connectedMatchesInterface(c, interfaces) {
				valid = true
				break
			}
		}
		if !valid {
			return nil, "static route next hop is not validated by a connected route on its egress interface"
		}
		if address, ok := interfaceAddress(interfaces, r.Interface); !ok || address == nh {
			return nil, "static route next hop equals or cannot be distinguished from its egress interface address"
		}
	}
	return &r, "selected"
}

func validateRouteInputs(routes []firewall.PathRoute, interfaces []firewall.PathInterface) string {
	ids := map[string]bool{}
	for _, r := range routes {
		if r.ID == "" || ids[r.ID] {
			return "route evidence has a missing or duplicate route identifier"
		}
		ids[r.ID] = true
		p, err := netip.ParsePrefix(r.Prefix)
		if err != nil || !p.Addr().Is4() || p != p.Masked() || r.Interface == "" {
			return "route evidence contains an invalid IPv4 prefix or interface"
		}
		switch strings.ToLower(r.Kind) {
		case "connected":
			if r.NextHop != "" || r.Distance != 0 || !connectedMatchesInterface(r, interfaces) {
				return "connected route does not match its resolved interface address and prefix"
			}
		case "static":
			nh, err := netip.ParseAddr(r.NextHop)
			if err != nil || !nh.Is4() || r.Distance < 1 || r.Distance > 254 {
				return "static route contains an unsupported next hop or administrative distance"
			}
		default:
			return "route evidence contains an unsupported route kind"
		}
	}
	return ""
}

func connectedRoute(routes []firewall.PathRoute, interfaces []firewall.PathInterface, iface, destination string) (*firewall.PathRoute, int) {
	a, err := netip.ParseAddr(destination)
	if err != nil || !a.Is4() {
		return nil, 0
	}
	var candidates []firewall.PathRoute
	longest := -1
	for _, r := range routes {
		p, e := netip.ParsePrefix(r.Prefix)
		if e != nil || !p.Addr().Is4() || !strings.EqualFold(r.Kind, "connected") || !strings.EqualFold(r.Interface, iface) || !p.Contains(a) || !connectedMatchesInterface(r, interfaces) {
			continue
		}
		if p.Bits() > longest {
			longest, candidates = p.Bits(), nil
		}
		if p.Bits() == longest {
			candidates = append(candidates, r)
		}
	}
	if len(candidates) != 1 {
		return nil, len(candidates)
	}
	return &candidates[0], 1
}

func connectedMatchesInterface(route firewall.PathRoute, interfaces []firewall.PathInterface) bool {
	iface, count := findInterface(interfaces, route.Interface)
	if count != 1 || !iface.Resolved || iface.Address == "" {
		return false
	}
	p, err := netip.ParsePrefix(iface.Address)
	return err == nil && p.Addr().Is4() && p.Masked().String() == route.Prefix
}

func interfaceAddress(interfaces []firewall.PathInterface, name string) (netip.Addr, bool) {
	iface, count := findInterface(interfaces, name)
	if count != 1 || !iface.Resolved || iface.Address == "" {
		return netip.Addr{}, false
	}
	p, err := netip.ParsePrefix(iface.Address)
	return p.Addr(), err == nil && p.Addr().Is4()
}

func isTransitAddress(addr netip.Addr) bool {
	return addr.Is4() && !addr.IsUnspecified() && !addr.IsMulticast() && !addr.IsLoopback() && addr != netip.MustParseAddr("255.255.255.255")
}

func usableNextHop(addr netip.Addr, prefix netip.Prefix) bool {
	if !isTransitAddress(addr) || !prefix.Contains(addr) {
		return false
	}
	if prefix.Bits() >= 31 {
		return true
	}
	base := prefix.Masked().Addr().As4()
	hop := addr.As4()
	baseN := uint32(base[0])<<24 | uint32(base[1])<<16 | uint32(base[2])<<8 | uint32(base[3])
	hopN := uint32(hop[0])<<24 | uint32(hop[1])<<16 | uint32(hop[2])<<8 | uint32(hop[3])
	hostBits := uint(32 - prefix.Bits())
	mask := uint32((uint64(1) << hostBits) - 1)
	return hopN != baseN && hopN != baseN|mask
}

func localAddressEvidence(interfaces []firewall.PathInterface, destination string) []string {
	a, err := netip.ParseAddr(destination)
	if err != nil {
		return nil
	}
	refs := []string{}
	for _, iface := range interfaces {
		if iface.Address == "" {
			continue
		}
		prefix, parseErr := netip.ParsePrefix(iface.Address)
		if parseErr == nil && prefix.Addr().Is4() && prefix.Addr() == a {
			refs = append(refs, iface.EvidenceRefs...)
		}
	}
	return appendUnique(nil, refs...)
}

func findInterface(all []firewall.PathInterface, name string) (firewall.PathInterface, int) {
	var value firewall.PathInterface
	count := 0
	for _, v := range all {
		if strings.EqualFold(v.Name, name) {
			value, count = v, count+1
		}
	}
	return value, count
}
func findIngressChain(all []firewall.Chain, iface string) (firewall.Chain, int) {
	var value firewall.Chain
	count := 0
	for _, c := range all {
		if strings.EqualFold(c.Interface, iface) && strings.EqualFold(c.Direction, "in") {
			value, count = c, count+1
		}
	}
	return value, count
}
func copyFlow(v policy.Flow) *policy.Flow                 { x := v; return &x }
func copyRoute(v *firewall.PathRoute) *firewall.PathRoute { x := *v; return &x }
func appendUnique(dst []string, values ...string) []string {
	out := append([]string(nil), dst...)
	seen := make(map[string]bool, len(out)+len(values))
	for _, v := range out {
		seen[v] = true
	}
	for _, v := range values {
		if v != "" && !seen[v] {
			out = append(out, v)
			seen[v] = true
		}
	}
	sort.Strings(out)
	return out
}

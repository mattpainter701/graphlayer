package pathanalysis

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/mattpainter701/graphlayer/platform/firewall"
	"github.com/mattpainter701/graphlayer/platform/policy"
)

const maxRouteCandidates = 8

// Candidate is one active equal-best route interpretation of a single-device
// path. Resolved describes tuple/interface availability, not session certainty.
type Candidate struct {
	ID            string `json:"id"`
	Result        Result `json:"result"`
	Resolved      bool   `json:"resolved"`
	SessionStatus string `json:"session_status"`
}

type routeContext struct {
	choice routeChoice
	id     string
}

// Explore evaluates the bounded equal-best route interpretations of a path.
func Explore(in Input) ([]Candidate, bool, error) {
	if err := policy.ValidateScope(in.Scope); err != nil {
		return nil, false, fmt.Errorf("scope: %w", err)
	}
	if in.Config.Path == nil {
		r, err := Evaluate(in)
		if err != nil {
			return nil, false, err
		}
		return []Candidate{{ID: "candidate:unresolved", Result: r, SessionStatus: "fresh_policy"}}, false, nil
	}
	p := in.Config.Path
	contexts := []routeContext{{id: "candidate:default"}}
	truncated := false
	if isPANVendor(in.Config.Vendor) {
		choices, _ := bestRoutes(p.Routes, p.Interfaces, in.Flow.Destination)
		if len(choices) > 1 {
			contexts = make([]routeContext, 0, min(len(choices), maxRouteCandidates))
			for _, r := range choices {
				if len(contexts) == maxRouteCandidates {
					truncated = true
					break
				}
				contexts = append(contexts, routeContext{choice: routeChoice{PreNAT: r.ID}, id: "candidate:" + r.ID})
			}
		}
	}
	var out []Candidate
	stop := false
	for _, ctx := range contexts {
		if len(out) == maxRouteCandidates {
			truncated = true
			break
		}
		if stop {
			break
		}
		preResult, err := evaluateWithRoute(in, nil, ctx.choice)
		if err != nil {
			return nil, false, err
		}
		postDestination := postNATRouteDestination(preResult, in.Flow.Destination)
		choices := []firewall.PathRoute{}
		if preResult.Disposition != "blocked" && (!isASAVendor(in.Config.Vendor) || !asaDNATApplied(preResult)) {
			choices, _ = bestRoutes(p.Routes, p.Interfaces, postDestination)
		}
		branches := []routeContext{ctx}
		if isASAVendor(in.Config.Vendor) && len(choices) > 1 {
			branches = nil
			for _, r := range choices {
				if len(out)+len(branches) == maxRouteCandidates {
					truncated = true
					break
				}
				c := ctx
				c.choice.PostNAT = r.ID
				c.id += "/" + r.ID
				branches = append(branches, c)
			}
		} else if isPANVendor(in.Config.Vendor) {
			if strings.EqualFold(postDestination, in.Flow.Destination) && ctx.choice.PreNAT != "" {
				ctx.choice.PostNAT = ctx.choice.PreNAT
				branches = []routeContext{ctx}
			} else if len(choices) > 1 {
				branches = nil
				for _, r := range choices {
					if len(out)+len(branches) == maxRouteCandidates {
						truncated = true
						break
					}
					c := ctx
					c.choice.PostNAT = r.ID
					c.id += "/" + r.ID
					branches = append(branches, c)
				}
			}
		} else if len(choices) > 1 {
			branches = nil
			for _, r := range choices {
				if len(out)+len(branches) == maxRouteCandidates {
					truncated = true
					break
				}
				c := ctx
				c.choice.PostNAT = r.ID
				c.id = "candidate:" + r.ID
				branches = append(branches, c)
			}
		}
		for _, branch := range branches {
			if len(out) == maxRouteCandidates {
				truncated = true
				stop = true
				break
			}
			r, err := evaluateWithRoute(in, nil, branch.choice)
			if err != nil {
				return nil, false, err
			}
			if branch.id == "candidate:default" {
				branch.id = candidateID(r)
			}
			out = append(out, Candidate{ID: branch.id, Result: r, Resolved: r.EgressInterface != "" && r.EgressFlow.Source != "" && r.EgressFlow.Destination != "", SessionStatus: "fresh_policy"})
		}
	}
	if len(out) == 0 {
		r, err := Evaluate(in)
		if err != nil {
			return nil, false, err
		}
		out = append(out, Candidate{ID: "candidate:unresolved", Result: r, SessionStatus: "fresh_policy"})
	}
	return out, truncated, nil
}

func postNATRouteDestination(r Result, fallback string) string {
	for _, stage := range r.Stages {
		if stage.Name == "destination_nat" && stage.Status == "matched" && stage.After != nil && stage.After.Destination != "" {
			return stage.After.Destination
		}
	}
	if r.PolicyFlow.Destination != "" {
		return r.PolicyFlow.Destination
	}
	return fallback
}

func candidateID(r Result) string {
	if r.SelectedRoute != nil {
		return "candidate:" + r.SelectedRoute.ID
	}
	return "candidate:unresolved"
}
func isPANVendor(v string) bool { return strings.Contains(strings.ToLower(v), "pan") }
func asaDNATApplied(r Result) bool {
	for _, s := range r.Stages {
		if s.Name == "destination_nat" && s.Status == "matched" {
			return true
		}
	}
	return false
}

// ExploreReturn investigates a return route against a verified forward
// candidate. A nil forward means this firewall was absent from the evidence;
// that mode can only report a fresh, no-NAT traversal as unproven.
func ExploreReturn(in Input, forward *Result) ([]Candidate, bool, error) {
	if forward == nil {
		c, trunc, err := Explore(in)
		if err != nil {
			return nil, trunc, err
		}
		natConfigured := configHasNonIdentityNAT(in.Config)
		for i := range c {
			if natConfigured {
				c[i].Result.Disposition = "indeterminate"
				c[i].Result.EgressFlow = policy.Flow{}
				c[i].Result.EgressInterface = ""
				c[i].Result.Reasons = append(c[i].Result.Reasons, "firewall is absent from forward evidence and configured NAT state cannot be established")
				c[i].Resolved = false
			}
			c[i].Result.Disposition = "indeterminate"
			c[i].Result.Reasons = append(c[i].Result.Reasons, "firewall is absent from forward evidence; fresh policy does not establish an existing session")
			c[i].SessionStatus = "unproven"
		}
		return c, trunc, nil
	}
	if forward.Disposition != "modeled_forward" || forward.Policy == nil || forward.Policy.Disposition != "permit" {
		return nil, false, fmt.Errorf("forward candidate must establish a modeled permitted path")
	}
	forwardCandidates, _, err := Explore(Input{Scope: forward.Scope, Config: in.Config, IngressInterface: forward.IngressInterface, Flow: forward.OriginalFlow})
	if err != nil {
		return nil, false, err
	}
	verified := false
	for _, c := range forwardCandidates {
		if reflect.DeepEqual(c.Result, *forward) {
			verified = true
			break
		}
	}
	if !verified {
		return nil, false, fmt.Errorf("forward result is not an Explore candidate for the supplied configuration")
	}
	if in.Scope != forward.Scope || in.Flow != reverseFlow(forward.EgressFlow, in.Scope) {
		return nil, false, fmt.Errorf("return scope and tuple must exactly reverse the forward candidate")
	}
	if !strings.EqualFold(in.IngressInterface, forward.EgressInterface) {
		if configHasNonIdentityNAT(in.Config) {
			c, truncated, err := Explore(in)
			if err != nil {
				return nil, truncated, err
			}
			for i := range c {
				c[i].Result.Disposition = "indeterminate"
				c[i].Result.EgressFlow = policy.Flow{}
				c[i].Result.EgressInterface = ""
				c[i].Result.Reasons = append(c[i].Result.Reasons, "return ingress differs from the forward egress and configured NAT state cannot be established")
				c[i].Resolved = false
				c[i].SessionStatus = "asymmetric_unknown"
			}
			return c, truncated, nil
		}
		return exploreReturnWithDifferentIngress(in, *forward)
	}
	proto := strings.ToLower(strings.TrimSpace(in.Flow.Protocol))
	if proto != "tcp" && proto != "udp" && proto != "6" && proto != "17" {
		return nil, false, fmt.Errorf("established return modeling supports TCP and UDP only")
	}
	// Existing return evaluators already apply only the exact forward NAT
	// evidence and validate the forward configuration. Preserve their policy
	// and translation semantics; expand only the resulting equal-best egresses.
	r, legacyErr := EvaluateReturn(in, *forward)
	if legacyErr == nil && r.Disposition == "modeled_forward" {
		return []Candidate{{ID: candidateID(r), Result: r, Resolved: true, SessionStatus: "established"}}, false, nil
	}
	// EvaluateReturn intentionally clears the tuple when it reports an
	// asymmetric route. Re-evaluate the already verified established packet to
	// retain its concrete candidate tuple and route evidence.
	return exploreEstablishedReturn(in, *forward)
}

func exploreReturnWithDifferentIngress(in Input, forward Result) ([]Candidate, bool, error) {
	routes, _ := bestRoutes(in.Config.Path.Routes, in.Config.Path.Interfaces, in.Flow.Destination)
	branches := routes
	if len(branches) == 0 {
		branches = []firewall.PathRoute{{}}
	}
	out := make([]Candidate, 0, min(len(branches), maxRouteCandidates))
	truncated := false
	for i, route := range branches {
		if i == maxRouteCandidates {
			truncated = true
			break
		}
		var r Result
		var err error
		if isASAVendor(in.Config.Vendor) {
			r, err = evaluateRoute(in, &forward, route.ID)
		} else {
			r, err = vendorEvaluateWithRoute(in, &forward, routeChoice{PostNAT: route.ID})
		}
		if err != nil {
			return nil, false, err
		}
		r.Disposition = "indeterminate"
		r.Reasons = append(r.Reasons, "return ingress differs from the forward egress; route candidate is concrete but session symmetry is unknown")
		id := candidateID(r)
		if route.ID != "" {
			id = "candidate:return:" + route.ID
		}
		out = append(out, Candidate{ID: id, Result: r, Resolved: r.EgressInterface != "" && r.EgressFlow.Source != "" && r.EgressFlow.Destination != "", SessionStatus: "asymmetric_unknown"})
	}
	return out, truncated, nil
}

func vendorNATApplied(r Result) bool {
	for _, s := range r.Stages {
		if (s.Name == "destination_nat" || s.Name == "source_nat") && s.Status == "matched" {
			return true
		}
	}
	return false
}

func configHasNonIdentityNAT(c firewall.Config) bool {
	if c.Path == nil {
		return false
	}
	for _, n := range c.Path.NATRules {
		if n.RealPrefix != "" && n.MappedPrefix != "" && (n.RealPrefix != n.MappedPrefix || n.RealPort != n.MappedPort || n.Protocol != "") {
			return true
		}
	}
	for _, n := range c.Path.Translations {
		kind := strings.ToLower(strings.TrimSpace(n.Kind))
		if (kind == "snat" || kind == "dnat") && n.OriginalPrefix != "" && n.TranslatedPrefix != "" && n.OriginalPrefix != n.TranslatedPrefix {
			return true
		}
	}
	return false
}

func exploreEstablishedReturn(in Input, forward Result) ([]Candidate, bool, error) {
	r, err := evaluateReturn(in, forward, "", true)
	if err != nil {
		return nil, false, err
	}
	choices := []firewall.PathRoute{}
	destination := r.PolicyFlow.Destination
	if destination == "" {
		destination = in.Flow.Destination
	}
	if !isASAVendor(in.Config.Vendor) || !asaDNATApplied(r) {
		choices, _ = bestRoutes(in.Config.Path.Routes, in.Config.Path.Interfaces, destination)
	}
	branches := []struct {
		id      string
		routeID string
	}{{id: candidateID(r)}}
	if len(choices) > 0 {
		branches = nil
		for _, route := range choices {
			if len(branches) == maxRouteCandidates {
				break
			}
			branches = append(branches, struct {
				id      string
				routeID string
			}{id: "candidate:return:" + route.ID, routeID: route.ID})
		}
	} else if r.SelectedRoute != nil {
		// Force the selected route through the candidate evaluator even when
		// there is only one choice. This preserves a concrete tuple when the
		// legacy return API clears it after detecting asymmetry.
		branches[0].routeID = r.SelectedRoute.ID
	}
	out := make([]Candidate, 0, len(branches))
	truncated := len(choices) > maxRouteCandidates
	for _, branch := range branches {
		candidate := r
		if branch.routeID != "" {
			candidate, err = evaluateReturn(in, forward, branch.routeID, true)
			if err != nil {
				return nil, false, err
			}
		}
		status := "established"
		if candidate.Disposition == "blocked" {
			// The forward session is known and complete routing evidence proves
			// this return leg has no supported route.
		} else if candidate.Disposition != "modeled_forward" || !strings.EqualFold(candidate.EgressInterface, forward.IngressInterface) || candidate.EgressFlow != reverseFlow(forward.OriginalFlow, in.Scope) {
			status = "asymmetric_unknown"
			candidate.Disposition = "indeterminate"
			candidate.Reasons = append(candidate.Reasons, "return route candidate does not prove symmetry or exact reverse translation")
		}
		out = append(out, Candidate{ID: branch.id, Result: candidate, Resolved: candidate.EgressInterface != "" && candidate.EgressFlow.Source != "" && candidate.EgressFlow.Destination != "", SessionStatus: status})
	}
	return out, truncated, nil
}

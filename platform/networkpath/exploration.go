package networkpath

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mattpainter701/graphlayer/platform/pathanalysis"
	"github.com/mattpainter701/graphlayer/platform/policy"
)

const (
	maxExplorationBranches    = 32
	maxExplorationEvaluations = 128
	maxExplorationHops        = 16
)

// Explore enumerates bounded, configuration-derived route branches. The input
// is treated as immutable; all evaluator calls receive value copies.
func Explore(in Input) (ExplorationResult, error) {
	r := ExplorationResult{
		EngineContractVersion: ExplorationContractVersion,
		OriginalFlow:          in.Flow,
		Branches:              []ExplorationBranch{},
		Disposition:           "indeterminate",
		Complete:              true,
		Reasons:               []string{},
		Assumptions:           []string{"configuration and topology are supplied evidence, not live device or link state", "only explicitly linked IPv4 paths and bounded route choices are explored", "equal-best route choices are configured alternatives; this does not prove ECMP is enabled or predict a hardware hash/session choice", "return session state is modeled only when the exact reverse tuple and selected forward state are supported"},
	}
	if len(in.Nodes) == 0 || len(in.Nodes) > maxNodes || len(in.Links) > maxLinks {
		r.Complete = false
		r.Reasons = append(r.Reasons, "topology exceeds supported node or link limits")
		return r, nil
	}
	if in.Source.NodeID == "" || in.Source.Interface == "" || in.Destination.NodeID == "" || in.Destination.Interface == "" || len(in.Source.NodeID) > 64 || len(in.Destination.NodeID) > 64 || len(in.Source.Interface) > 48 || len(in.Destination.Interface) > 48 {
		return ExplorationResult{}, fmt.Errorf("source and destination node/interface are required")
	}
	if err := policy.ValidateScope(in.Flow.Scope); err != nil {
		return ExplorationResult{}, fmt.Errorf("flow scope: %w", err)
	}
	nodes := make(map[string]Node, len(in.Nodes))
	for _, n := range in.Nodes {
		if n.ID == "" || len(n.ID) > 64 || nodes[n.ID].ID != "" {
			r.Complete = false
			r.Reasons = append(r.Reasons, "topology contains a missing, overlong, or duplicate node identifier")
			return r, nil
		}
		if err := policy.ValidateScope(n.Scope); err != nil {
			return ExplorationResult{}, fmt.Errorf("node %s scope: %w", n.ID, err)
		}
		nodes[n.ID] = n
	}
	if nodes[in.Source.NodeID].ID == "" || nodes[in.Destination.NodeID].ID == "" {
		r.Complete = false
		r.Reasons = append(r.Reasons, "source or destination node is absent from topology")
		return r, nil
	}
	if in.Flow.Scope != nodes[in.Source.NodeID].Scope {
		return ExplorationResult{}, fmt.Errorf("flow scope must match the source node scope")
	}
	links := append([]Link(nil), in.Links...)
	for _, l := range links {
		if l.A.NodeID == "" || l.B.NodeID == "" || l.A.Interface == "" || l.B.Interface == "" || len(l.A.NodeID) > 64 || len(l.B.NodeID) > 64 || len(l.A.Interface) > 48 || len(l.B.Interface) > 48 || l.A.NodeID == l.B.NodeID || nodes[l.A.NodeID].ID == "" || nodes[l.B.NodeID].ID == "" || !matchingLinkFacts(nodes, l) {
			r.Complete = false
			r.Reasons = append(r.Reasons, "topology contains an invalid, missing, or unresolved link")
			return r, nil
		}
	}
	c := &exploreContext{in: in, nodes: nodes, links: links, out: &r}
	if !addressOnInterface(nodes[in.Source.NodeID], in.Source.Interface, in.Flow.Source) {
		r.Complete = false
		r.Reasons = append(r.Reasons, "source address is not proven on the selected ingress interface subnet")
		return r, nil
	}
	first := explorationState{current: in.Source, flow: in.Flow, forward: Direction{Disposition: "indeterminate", Hops: []Hop{}, Reasons: []string{}}, visited: map[string]bool{}, forwardByNode: map[string]pathanalysis.Result{}}
	c.walk(first, false, nil)
	c.classify()
	return r, nil
}

type explorationState struct {
	current       Endpoint
	flow          policy.Flow
	forward       Direction
	visited       map[string]bool
	forwardByNode map[string]pathanalysis.Result
	candidateIDs  []string
	sessionStatus string
	asymmetric    bool
}

type exploreContext struct {
	in          Input
	nodes       map[string]Node
	links       []Link
	out         *ExplorationResult
	evaluations int
	terminal    int
}

func (c *exploreContext) walk(s explorationState, returning bool, parent *ExplorationBranch) {
	if c.terminal >= maxExplorationBranches {
		c.truncate("terminal branch limit reached")
		return
	}
	if len(s.forward.Hops) >= maxExplorationHops || len(s.visited) >= maxExplorationHops {
		c.finish(s, returning, parent, "indeterminate", "path exceeds supported hop limit")
		return
	}
	if s.visited[s.current.NodeID] {
		c.finish(s, returning, parent, "indeterminate", "topology traversal contains a cycle")
		return
	}
	n, ok := c.nodes[s.current.NodeID]
	if !ok {
		c.finish(s, returning, parent, "indeterminate", "selected link references a missing node")
		return
	}
	s.visited = cloneVisited(s.visited)
	s.visited[s.current.NodeID] = true
	s.flow.Scope = n.Scope
	if c.evaluations >= maxExplorationEvaluations {
		c.truncate("hop evaluation limit reached")
		return
	}
	input := pathanalysis.Input{Scope: n.Scope, Config: n.Config, IngressInterface: s.current.Interface, Flow: s.flow}
	var cands []pathanalysis.Candidate
	var truncated bool
	var err error
	if returning {
		if fwd, ok := s.forwardByNode[n.ID]; ok {
			cands, truncated, err = pathanalysis.ExploreReturn(input, &fwd)
		} else {
			cands, truncated, err = pathanalysis.ExploreReturn(input, nil)
		}
	} else {
		cands, truncated, err = pathanalysis.Explore(input)
	}
	if err != nil {
		c.finish(s, returning, parent, "indeterminate", "node path inputs could not be evaluated: "+err.Error())
		return
	}
	if truncated {
		c.truncate("node route choice limit reached")
	}
	if len(cands) == 0 {
		c.finish(s, returning, parent, "indeterminate", "no supported route candidate was produced")
		return
	}
	remaining := maxExplorationEvaluations - c.evaluations
	if len(cands) > remaining {
		cands = cands[:remaining]
		c.truncate("hop evaluation limit reached")
	}
	c.evaluations += len(cands)
	for _, cand := range cands {
		if c.terminal >= maxExplorationBranches {
			c.truncate("terminal branch limit reached")
			return
		}
		branchState := cloneState(s)
		branchState.candidateIDs = append(branchState.candidateIDs, cand.ID)
		p := cand.Result
		if !cand.Resolved && p.Disposition == "modeled_forward" {
			p.Disposition = "indeterminate"
			p.Reasons = append(p.Reasons, "candidate egress tuple is not fully resolved")
		}
		if returning {
			branchState.sessionStatus = mergeSessionStatus(branchState.sessionStatus, cand.SessionStatus)
			if fwd, ok := branchState.forwardByNode[n.ID]; !ok {
				branchState.asymmetric = true
				branchState.sessionStatus = mergeSessionStatus(branchState.sessionStatus, "unproven")
			} else if !strings.EqualFold(fwd.EgressInterface, s.current.Interface) {
				branchState.asymmetric = true
				branchState.sessionStatus = mergeSessionStatus(branchState.sessionStatus, "asymmetric_unknown")
			} else if branchState.sessionStatus == "asymmetric_unknown" {
				branchState.asymmetric = true
			}
		}
		h := Hop{NodeID: n.ID, SnapshotID: n.Scope.SnapshotID, Path: p, Evidence: refs(n.Scope.SnapshotID, p.EvidenceRefs)}
		branchState.forward.Hops = append(branchState.forward.Hops, h)
		concreteReturn := returning && cand.Resolved && p.EgressInterface != "" && p.EgressFlow.Source != "" && p.EgressFlow.Destination != ""
		if p.Disposition != "modeled_forward" && !concreteReturn {
			c.finish(branchState, returning, parent, p.Disposition, strings.Join(p.Reasons, "; "))
			continue
		}
		branchState.flow = p.EgressFlow
		if !returning {
			branchState.forwardByNode[n.ID] = p
		}
		if returning { // return endpoint is the original source
			if n.ID == c.in.Source.NodeID && strings.EqualFold(p.EgressInterface, c.in.Source.Interface) && addressOnInterface(n, c.in.Source.Interface, p.EgressFlow.Destination) {
				branchState.forward.Disposition = "modeled_forward"
				branchState.forward.FinalFlow = p.EgressFlow
				c.finish(branchState, true, parent, "modeled_forward", "")
			} else {
				c.advance(branchState, true, parent, p, cand.ID)
			}
			continue
		}
		if n.ID == c.in.Destination.NodeID {
			if strings.EqualFold(p.EgressInterface, c.in.Destination.Interface) && directlyConnected(n, c.in.Destination.Interface, p.EgressFlow.Destination) {
				branchState.forward.Disposition = "modeled_forward"
				branchState.forward.FinalFlow = p.EgressFlow
				c.makeReturns(branchState)
			} else {
				c.finish(branchState, false, nil, "indeterminate", "destination node egress differs from requested directly connected terminal")
			}
			continue
		}
		c.advance(branchState, false, parent, p, cand.ID)
	}
}

func (c *exploreContext) advance(s explorationState, returning bool, parent *ExplorationBranch, p pathanalysis.Result, candidateID string) {
	var next *Link
	var peer Endpoint
	endpointFound := false
	validPeers := 0
	for i := range c.links {
		l := &c.links[i]
		if l.A.NodeID == s.current.NodeID && strings.EqualFold(l.A.Interface, p.EgressInterface) {
			endpointFound = true
			if peerReachable(c.nodes[s.current.NodeID], c.nodes[l.B.NodeID], p, l.B.Interface) {
				next = &c.links[i]
				peer = l.B
				validPeers++
			}
		}
		if l.B.NodeID == s.current.NodeID && strings.EqualFold(l.B.Interface, p.EgressInterface) {
			endpointFound = true
			if peerReachable(c.nodes[s.current.NodeID], c.nodes[l.A.NodeID], p, l.A.Interface) {
				next = &c.links[i]
				peer = l.A
				validPeers++
			}
		}
	}
	if !endpointFound {
		c.finish(s, returning, parent, "indeterminate", "selected egress has no explicit topology link")
		return
	}
	if validPeers != 1 {
		c.finish(s, returning, parent, "indeterminate", "selected route does not identify exactly one linked peer as its next hop")
		return
	}
	lc := *next
	s.forward.Hops[len(s.forward.Hops)-1].Link = &lc
	s.forward.Hops[len(s.forward.Hops)-1].Evidence = append(s.forward.Hops[len(s.forward.Hops)-1].Evidence, linkEvidence(c.nodes, *next)...)
	s.current = peer
	c.walk(s, returning, parent)
}

func (c *exploreContext) makeReturns(forward explorationState) {
	// This return exploration follows configured routes from the destination. It
	// does not reverse the forward hop list and therefore preserves asymmetry.
	last := forward.forward.Hops[len(forward.forward.Hops)-1].Path
	flow := reverseFlow(last.EgressFlow, c.nodes[c.in.Destination.NodeID].Scope)
	ret := explorationState{current: c.in.Destination, flow: flow, forward: Direction{Disposition: "indeterminate", Hops: []Hop{}, Reasons: []string{}}, visited: map[string]bool{}, forwardByNode: forward.forwardByNode}
	start := len(c.out.Branches)
	c.walk(ret, true, nil)
	rows := append([]ExplorationBranch(nil), c.out.Branches[start:]...)
	c.out.Branches = c.out.Branches[:start]
	base := ExplorationBranch{ID: branchID(forward.forward.Hops, forward.candidateIDs), Forward: forward.forward, Returns: []ReturnBranch{}, Disposition: "indeterminate", Reasons: []string{}}
	allReturn, allBlocked := len(rows) > 0, len(rows) > 0
	for _, row := range rows {
		if len(row.Returns) == 0 {
			allReturn = false
			allBlocked = false
			continue
		}
		ret := row.Returns[0]
		ret.ID = base.ID + "-return-" + ret.ID
		base.Returns = append(base.Returns, ret)
		if ret.Disposition != "modeled_forward" || ret.SessionStatus != "established" {
			allReturn = false
		}
		if ret.Direction.Disposition == "modeled_forward" && !exactReversePath(base.Forward, ret.Direction) {
			ret.Asymmetric = true
			ret.Disposition = "indeterminate"
			ret.Reasons = append(ret.Reasons, "return node, link, interface, or tuple sequence does not exactly reverse the forward branch")
			base.Returns[len(base.Returns)-1] = ret
			allReturn = false
		}
		if ret.Disposition != "blocked" {
			allBlocked = false
		}
	}
	if allReturn && c.out.Complete && !c.out.Truncated {
		base.Disposition = "modeled_round_trip"
	} else if allBlocked && c.out.Complete {
		base.Disposition = "blocked"
	} else {
		base.Disposition = "indeterminate"
	}
	if base.Disposition != "modeled_round_trip" {
		base.Reasons = append(base.Reasons, "return route or established session state is asymmetric, blocked, or unproven")
	}
	c.out.Branches = append(c.out.Branches, base)
}

func (c *exploreContext) finish(s explorationState, returning bool, parent *ExplorationBranch, disposition, reason string) {
	if reason == "" {
		reason = disposition
	}
	if returning {
		c.terminal++
		id := branchID(s.forward.Hops, s.candidateIDs)
		d := s.forward
		d.Disposition = disposition
		status := s.sessionStatus
		if status == "" {
			status = "unproven"
		}
		branchDisposition := disposition
		if branchDisposition == "modeled_forward" && status != "established" {
			branchDisposition = "indeterminate"
			if reason == "modeled_forward" || reason == "" {
				reason = "configured return route is concrete, but established session state is unproven"
			}
		}
		c.out.Branches = append(c.out.Branches, ExplorationBranch{ID: id, Forward: Direction{Disposition: "indeterminate", Hops: []Hop{}, Reasons: []string{}}, Returns: []ReturnBranch{{ID: id + "-return", Direction: d, Asymmetric: s.asymmetric, SessionStatus: status, Disposition: branchDisposition, Reasons: []string{reason}}}, Disposition: "indeterminate", Reasons: []string{reason}})
		return
	}
	c.terminal++
	id := branchID(s.forward.Hops, s.candidateIDs)
	s.forward.Disposition = disposition
	if reason != "" {
		s.forward.Reasons = append(s.forward.Reasons, reason)
	}
	c.out.Branches = append(c.out.Branches, ExplorationBranch{ID: id, Forward: s.forward, Returns: []ReturnBranch{}, Disposition: disposition, Reasons: []string{reason}})
}

func (c *exploreContext) truncate(reason string) {
	c.out.Truncated = true
	c.out.Complete = false
	for _, existing := range c.out.Reasons {
		if existing == reason {
			return
		}
	}
	c.out.Reasons = append(c.out.Reasons, reason)
}

func (c *exploreContext) classify() {
	if len(c.out.Branches) == 0 {
		c.out.Disposition = "indeterminate"
		c.out.Complete = false
		return
	}
	allBlocked, allRoundTrip := true, true
	for _, b := range c.out.Branches {
		if b.Disposition != "blocked" {
			allBlocked = false
		}
		if b.Disposition != "modeled_round_trip" {
			allRoundTrip = false
		}
	}
	sort.SliceStable(c.out.Branches, func(i, j int) bool { return c.out.Branches[i].ID < c.out.Branches[j].ID })
	switch {
	case allBlocked && c.out.Complete && !c.out.Truncated:
		c.out.Disposition = "blocked"
	case allRoundTrip && c.out.Complete && !c.out.Truncated:
		c.out.Disposition = "modeled_round_trip"
	default:
		c.out.Disposition = "indeterminate"
	}
}

func branchID(hops []Hop, candidateIDs []string) string {
	type identity struct {
		Node, Candidate, Route string
		NAT                    []string
	}
	parts := make([]identity, 0, len(hops))
	for i, h := range hops {
		id := ""
		if h.Path.SelectedRoute != nil {
			id = h.Path.SelectedRoute.ID
		}
		candidate := ""
		if i < len(candidateIDs) {
			candidate = candidateIDs[i]
		}
		parts = append(parts, identity{Node: h.NodeID, Candidate: candidate, Route: id, NAT: h.Path.NATRuleIDs})
	}
	encoded, _ := json.Marshal(parts)
	return fmt.Sprintf("branch-%x", sha256.Sum256(encoded))
}

func mergeSessionStatus(current, next string) string {
	if current == "unproven" || next == "unproven" {
		return "unproven"
	}
	if current == "asymmetric_unknown" || next == "asymmetric_unknown" {
		return "asymmetric_unknown"
	}
	if current != "" {
		return current
	}
	if next != "" {
		return next
	}
	return ""
}

func exactReversePath(forward, reverse Direction) bool {
	if forward.Disposition != "modeled_forward" || reverse.Disposition != "modeled_forward" || len(forward.Hops) == 0 || len(forward.Hops) != len(reverse.Hops) {
		return false
	}
	for i, rh := range reverse.Hops {
		fh := forward.Hops[len(forward.Hops)-1-i]
		if rh.NodeID != fh.NodeID || !strings.EqualFold(rh.Path.EgressInterface, fh.Path.IngressInterface) || rh.Path.OriginalFlow != reverseFlow(fh.Path.EgressFlow, rh.Path.Scope) || rh.Path.EgressFlow != reverseFlow(fh.Path.OriginalFlow, rh.Path.Scope) {
			return false
		}
		if i < len(reverse.Hops)-1 {
			priorForward := forward.Hops[len(forward.Hops)-2-i]
			if !sameLink(rh.Link, priorForward.Link) {
				return false
			}
		} else if rh.Link != nil {
			return false
		}
	}
	return true
}

func sameLink(a, b *Link) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return (a.A == b.A && a.B == b.B) || (a.A == b.B && a.B == b.A)
}
func cloneVisited(in map[string]bool) map[string]bool {
	out := make(map[string]bool, len(in)+1)
	for k, v := range in {
		out[k] = v
	}
	return out
}
func cloneState(s explorationState) explorationState {
	s.forward.Hops = append([]Hop(nil), s.forward.Hops...)
	s.forward.Reasons = append([]string(nil), s.forward.Reasons...)
	s.candidateIDs = append([]string(nil), s.candidateIDs...)
	s.visited = cloneVisited(s.visited)
	m := make(map[string]pathanalysis.Result, len(s.forwardByNode))
	for k, v := range s.forwardByNode {
		m[k] = v
	}
	s.forwardByNode = m
	return s
}

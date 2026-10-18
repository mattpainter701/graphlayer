package networkpath

import (
	"fmt"
	"net/netip"
	"strings"

	"github.com/mattpainter701/graphlayer/platform/pathanalysis"
	"github.com/mattpainter701/graphlayer/platform/policy"
)

const (
	maxNodes = 16
	maxLinks = 32
	maxHops  = 16
)

// Evaluate follows only explicitly linked, configuration-derived firewall hops.
func Evaluate(in Input) (Result, error) {
	contract := ContractVersion
	for _, n := range in.Nodes {
		if n.Config.Vendor != "" && !pathanalysis.IsASAVendor(n.Config.Vendor) {
			contract = VendorContractVersion
			break
		}
	}
	r := Result{EngineContractVersion: contract, OriginalFlow: in.Flow, Forward: Direction{Disposition: "indeterminate", Hops: []Hop{}, Reasons: []string{}}, Return: Direction{Disposition: "not_evaluated", Hops: []Hop{}, Reasons: []string{}}, Disposition: "indeterminate", Reasons: []string{}, Assumptions: []string{"configuration and topology are supplied evidence, not live device or link state", "only static IPv4 forwarding supported by the single-device path model is evaluated", "a modeled return leg assumes the reverse TCP/UDP tuple belongs to the established session represented by the forward permit; live connection state is not queried"}}
	uncertain := func(reason string) (Result, error) {
		r.Forward.Reasons = append(r.Forward.Reasons, reason)
		r.Reasons = append(r.Reasons, reason)
		return r, nil
	}
	if len(in.Nodes) == 0 || len(in.Nodes) > maxNodes || len(in.Links) > maxLinks {
		return uncertain("topology exceeds supported node or link limits")
	}
	if in.Source.NodeID == "" || in.Source.Interface == "" || in.Destination.NodeID == "" || in.Destination.Interface == "" || len(in.Source.NodeID) > 64 || len(in.Destination.NodeID) > 64 || len(in.Source.Interface) > 48 || len(in.Destination.Interface) > 48 {
		return Result{}, fmt.Errorf("source and destination node/interface are required")
	}
	nodes := map[string]Node{}
	for _, n := range in.Nodes {
		if n.ID == "" || nodes[n.ID].ID != "" {
			return uncertain("topology contains a missing or duplicate node identifier")
		}
		nodes[n.ID] = n
	}
	if _, ok := nodes[in.Source.NodeID]; !ok {
		return uncertain("source node is absent from topology")
	}
	if _, ok := nodes[in.Destination.NodeID]; !ok {
		return uncertain("destination node is absent from topology")
	}
	links := make([]Link, len(in.Links))
	copy(links, in.Links)
	used := map[string]bool{}
	for _, l := range links {
		if l.A.NodeID == "" || l.B.NodeID == "" || l.A.Interface == "" || l.B.Interface == "" || l.A.NodeID == l.B.NodeID {
			return uncertain("topology contains an invalid link endpoint")
		}
		for _, e := range []Endpoint{l.A, l.B} {
			if _, ok := nodes[e.NodeID]; !ok {
				return uncertain("link references a missing node")
			}
			if len(e.NodeID) > 64 || len(e.Interface) > 48 {
				return uncertain("topology identifier exceeds supported length")
			}
			k := e.NodeID + "\x00" + strings.ToLower(e.Interface)
			if used[k] {
				return uncertain("interface endpoint is repeated in topology links")
			}
			used[k] = true
		}
		if !matchingLinkFacts(nodes, l) {
			return uncertain("link endpoint interfaces are unresolved or do not share one IPv4 subnet")
		}
	}
	if in.Flow.Scope != nodes[in.Source.NodeID].Scope {
		return Result{}, fmt.Errorf("flow scope must match the source node scope")
	}
	flow := in.Flow
	current := in.Source
	visited := map[string]bool{}
	for hop := 0; hop < maxHops; hop++ {
		if visited[current.NodeID] {
			return uncertain("topology traversal contains a cycle")
		}
		visited[current.NodeID] = true
		n := nodes[current.NodeID]
		if hop == 0 && !addressOnInterface(n, current.Interface, flow.Source) {
			return uncertain("source address is not proven on the selected ingress interface subnet")
		}
		flow.Scope = n.Scope
		pr, err := pathanalysis.Evaluate(pathanalysis.Input{Scope: n.Scope, Config: n.Config, IngressInterface: current.Interface, Flow: flow})
		if err != nil {
			return uncertain("node path inputs could not be evaluated: " + err.Error())
		}
		h := Hop{NodeID: n.ID, SnapshotID: n.Scope.SnapshotID, Path: pr, Evidence: refs(n.Scope.SnapshotID, pr.EvidenceRefs)}
		r.Forward.Hops = append(r.Forward.Hops, h)
		if pr.Disposition != "modeled_forward" {
			r.Forward.Disposition = pr.Disposition
			r.Forward.Reasons = append(r.Forward.Reasons, pr.Reasons...)
			r.Reasons = append(r.Reasons, pr.Reasons...)
			if pr.Disposition == "blocked" {
				r.Disposition = "blocked"
			}
			return r, nil
		}
		flow = pr.EgressFlow
		if current.NodeID == in.Destination.NodeID {
			if !strings.EqualFold(pr.EgressInterface, in.Destination.Interface) {
				return uncertain("destination node egress differs from requested terminal interface")
			}
			if !directlyConnected(n, in.Destination.Interface, flow.Destination) {
				return uncertain("terminal destination is not proven directly connected on requested egress interface")
			}
			r.Forward.Disposition = "modeled_forward"
			r.Forward.FinalFlow = flow
			if reason := evaluateReturn(in, nodes, links, &r); reason != "" {
				if r.Return.Disposition == "" || r.Return.Disposition == "not_evaluated" {
					r.Return.Disposition = "indeterminate"
				}
				r.Return.Reasons = append(r.Return.Reasons, reason)
				if r.Return.Disposition == "blocked" {
					r.Disposition = "blocked"
				} else {
					r.Disposition = "indeterminate"
				}
				r.Reasons = append(r.Reasons, reason)
			} else {
				r.Disposition = "modeled_round_trip"
			}
			return r, nil
		}
		var selected *Link
		var peer Endpoint
		for i := range links {
			l := &links[i]
			if l.A.NodeID == current.NodeID && strings.EqualFold(l.A.Interface, pr.EgressInterface) {
				selected = l
				peer = l.B
				break
			}
			if l.B.NodeID == current.NodeID && strings.EqualFold(l.B.Interface, pr.EgressInterface) {
				selected = l
				peer = l.A
				break
			}
		}
		if selected == nil {
			return uncertain("selected egress has no explicit topology link")
		}
		if !peerReachable(n, nodes[peer.NodeID], pr, peer.Interface) {
			return uncertain("selected route does not prove the linked peer interface as its next hop")
		}
		lcopy := *selected
		r.Forward.Hops[len(r.Forward.Hops)-1].Link = &lcopy
		r.Forward.Hops[len(r.Forward.Hops)-1].Evidence = append(r.Forward.Hops[len(r.Forward.Hops)-1].Evidence, linkEvidence(nodes, *selected)...)
		if _, exists := nodes[peer.NodeID]; !exists {
			return uncertain("linked peer node is absent")
		}
		current = peer
	}
	return uncertain("topology traversal exceeds supported hop limit")
}

func evaluateReturn(in Input, nodes map[string]Node, links []Link, result *Result) string {
	hops := result.Forward.Hops
	var carried *policy.Flow
	for i := len(hops) - 1; i >= 0; i-- {
		fwd := hops[i]
		n := nodes[fwd.NodeID]
		retFlow := reverseFlow(fwd.Path.EgressFlow, n.Scope)
		if carried != nil {
			carried.Scope = n.Scope
			if *carried != retFlow {
				return "return tuple from the next firewall does not match this firewall's expected reverse tuple"
			}
			retFlow = *carried
		}
		pr, err := pathanalysis.EvaluateReturn(pathanalysis.Input{Scope: n.Scope, Config: n.Config, IngressInterface: fwd.Path.EgressInterface, Flow: retFlow}, fwd.Path)
		if err != nil {
			return "return path is not established on node " + n.ID + ": " + err.Error()
		}
		rh := Hop{NodeID: n.ID, SnapshotID: n.Scope.SnapshotID, Path: pr, Evidence: refs(n.Scope.SnapshotID, pr.EvidenceRefs)}
		result.Return.Hops = append(result.Return.Hops, rh)
		if pr.Disposition != "modeled_forward" {
			result.Return.Disposition = pr.Disposition
			result.Return.Reasons = append(result.Return.Reasons, pr.Reasons...)
			return "return path is blocked or indeterminate on node " + n.ID
		}
		if i == 0 {
			if !addressOnInterface(n, in.Source.Interface, pr.EgressFlow.Destination) {
				return "return destination is not proven on the requested source interface subnet"
			}
			result.Return.Disposition = "modeled_forward"
			result.Return.FinalFlow = pr.EgressFlow
			return ""
		}
		prev := hops[i-1]
		link, peer, ok := linkBetween(links, n.ID, pr.EgressInterface, prev.NodeID, prev.Path.EgressInterface)
		if !ok {
			return "return egress does not use the explicit reverse topology link"
		}
		if !peerReachable(n, nodes[peer.NodeID], pr, peer.Interface) {
			return "return route does not prove the linked peer interface as its next hop"
		}
		copyLink := *link
		result.Return.Hops[len(result.Return.Hops)-1].Link = &copyLink
		result.Return.Hops[len(result.Return.Hops)-1].Evidence = append(result.Return.Hops[len(result.Return.Hops)-1].Evidence, linkEvidence(nodes, *link)...)
		carried = &pr.EgressFlow
	}
	return "return path did not reach its source"
}

func reverseFlow(f policy.Flow, scope policy.Scope) policy.Flow {
	f.Scope = scope
	f.Source, f.Destination = f.Destination, f.Source
	f.SourcePort, f.DestinationPort = f.DestinationPort, f.SourcePort
	return f
}

func linkBetween(links []Link, node, iface, peerNode, peerIface string) (*Link, Endpoint, bool) {
	for i := range links {
		l := &links[i]
		if l.A.NodeID == node && strings.EqualFold(l.A.Interface, iface) && l.B.NodeID == peerNode && strings.EqualFold(l.B.Interface, peerIface) {
			return l, l.B, true
		}
		if l.B.NodeID == node && strings.EqualFold(l.B.Interface, iface) && l.A.NodeID == peerNode && strings.EqualFold(l.A.Interface, peerIface) {
			return l, l.A, true
		}
	}
	return nil, Endpoint{}, false
}

func directlyConnected(n Node, egress, address string) bool {
	p := n.Config.Path
	if p == nil {
		return false
	}
	a, e := netip.ParseAddr(address)
	if e != nil || !a.Is4() {
		return false
	}
	for _, i := range p.Interfaces {
		if strings.EqualFold(i.Name, egress) && i.Resolved && !i.Shutdown {
			q, e := netip.ParsePrefix(i.Address)
			if e == nil && q.Contains(a) && usableAddress(a, q) {
				for _, r := range p.Routes {
					if strings.EqualFold(r.Kind, "connected") && strings.EqualFold(r.Interface, egress) && r.Prefix == q.Masked().String() {
						return true
					}
				}
			}
		}
	}
	return false
}
func peerReachable(n, peerNode Node, p pathanalysis.Result, peerInterface string) bool {
	if p.SelectedRoute == nil {
		return false
	}
	route := p.SelectedRoute
	peer, count := findIface(peerNode, peerInterface)
	if count != 1 || !peer.resolved || peer.shutdown {
		return false
	}
	peerPrefix, e := netip.ParsePrefix(peer.address)
	if e != nil || !strings.EqualFold(route.Interface, p.EgressInterface) {
		return false
	}
	if !strings.EqualFold(route.Kind, "static") {
		return false
	}
	nh, err := netip.ParseAddr(route.NextHop)
	if err != nil || peerPrefix.Addr() != nh {
		return false
	}
	local, count := findIface(n, p.EgressInterface)
	if count != 1 || !local.resolved || local.shutdown {
		return false
	}
	localPrefix, err := netip.ParsePrefix(local.address)
	return err == nil && localPrefix.Masked() == peerPrefix.Masked()
}

func matchingLinkFacts(nodes map[string]Node, l Link) bool {
	a, ac := findIface(nodes[l.A.NodeID], l.A.Interface)
	b, bc := findIface(nodes[l.B.NodeID], l.B.Interface)
	if ac != 1 || bc != 1 || !a.resolved || a.shutdown || !b.resolved || b.shutdown {
		return false
	}
	ap, ae := netip.ParsePrefix(a.address)
	bp, be := netip.ParsePrefix(b.address)
	if ae != nil || be != nil || !ap.Addr().Is4() || !bp.Addr().Is4() || ap.Masked() != bp.Masked() || ap.Addr() == bp.Addr() || !usableHost(ap) || !usableHost(bp) {
		return false
	}
	return true
}

func addressOnInterface(n Node, name, address string) bool {
	if n.Config.Path == nil {
		return false
	}
	a, err := netip.ParseAddr(address)
	if err != nil || !a.Is4() {
		return false
	}
	i, count := findIface(n, name)
	if count != 1 || !i.resolved || i.shutdown {
		return false
	}
	p, err := netip.ParsePrefix(i.address)
	return err == nil && p.Contains(a) && usableAddress(a, p) && p.Addr() != a
}

func usableHost(p netip.Prefix) bool { return usableAddress(p.Addr(), p) }

func usableAddress(a netip.Addr, p netip.Prefix) bool {
	if !a.Is4() || !p.Contains(a) {
		return false
	}
	if p.Bits() >= 31 {
		return true
	}
	toN := func(b [4]byte) uint32 { return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3]) }
	base, value := toN(p.Masked().Addr().As4()), toN(a.As4())
	mask := uint32((uint64(1) << uint(32-p.Bits())) - 1)
	return value != base && value != base|mask
}
func findIface(n Node, name string) (i interfaceInfo, count int) {
	if n.Config.Path == nil {
		return
	}
	for _, v := range n.Config.Path.Interfaces {
		if strings.EqualFold(v.Name, name) {
			i = interfaceInfo{v.Address, v.Resolved, v.Shutdown, v.EvidenceRefs}
			count++
		}
	}
	return
}

type interfaceInfo struct {
	address            string
	resolved, shutdown bool
	refs               []string
}

func refs(snapshot string, values []string) []EvidenceRef {
	o := []EvidenceRef{}
	for _, v := range values {
		if v != "" {
			o = append(o, EvidenceRef{SnapshotID: snapshot, Ref: v})
		}
	}
	return o
}
func linkEvidence(nodes map[string]Node, l Link) []EvidenceRef {
	o := []EvidenceRef{}
	for _, e := range []Endpoint{l.A, l.B} {
		n := nodes[e.NodeID]
		i, _ := findIface(n, e.Interface)
		o = append(o, refs(n.Scope.SnapshotID, i.refs)...)
	}
	return o
}

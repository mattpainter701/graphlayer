package networkpath

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/mattpainter701/graphlayer/platform/firewall"
	"github.com/mattpainter701/graphlayer/platform/policy"
)

func TestExploreProducesStableBoundedRoundTripBranch(t *testing.T) {
	in := fixtureInput()
	before, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	first, err := Explore(in)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Explore(in)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("exploration is not deterministic: %#v != %#v", first, second)
	}
	if first.EngineContractVersion != ExplorationContractVersion || !first.Complete || first.Truncated || first.Disposition != "modeled_round_trip" || len(first.Branches) != 1 {
		t.Fatalf("unexpected exploration result: %#v", first)
	}
	b := first.Branches[0]
	if len(b.Forward.Hops) != 2 || len(b.Returns) != 1 || b.Returns[0].SessionStatus != "established" || b.Returns[0].Disposition != "modeled_forward" {
		t.Fatalf("expected exact symmetric path and return state: %#v", b)
	}
	after, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(after, before) {
		t.Fatal("Explore mutated its input")
	}
}

func TestExploreKeepsEqualBestPermitAndBlockedRoutesSeparate(t *testing.T) {
	in := mixedOutcomeInput()
	got, err := Explore(in)
	if err != nil {
		t.Fatal(err)
	}
	if got.Disposition != "indeterminate" || !got.Complete || got.Truncated || len(got.Branches) != 2 {
		t.Fatalf("expected complete mixed branch coverage: %#v", got)
	}
	seen := map[string]bool{}
	for _, b := range got.Branches {
		seen[b.Disposition] = true
	}
	if !seen["modeled_round_trip"] || !seen["blocked"] {
		t.Fatalf("permit and block alternatives were not preserved: %#v", got.Branches)
	}
}

func TestExploreOrderDoesNotDependOnNodeOrLinkInputOrder(t *testing.T) {
	in := mixedOutcomeInput()
	first, err := Explore(in)
	if err != nil {
		t.Fatal(err)
	}
	for i, j := 0, len(in.Nodes)-1; i < j; i, j = i+1, j-1 {
		in.Nodes[i], in.Nodes[j] = in.Nodes[j], in.Nodes[i]
	}
	for i, j := 0, len(in.Links)-1; i < j; i, j = i+1, j-1 {
		in.Links[i], in.Links[j] = in.Links[j], in.Links[i]
	}
	second, err := Explore(in)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("node/link order changed exploration: %#v != %#v", first, second)
	}
}

func TestExploreAsymmetricReturnThroughUnseenFirewallKeepsConfiguredRoute(t *testing.T) {
	in := threeHopInput()
	a, c := &in.Nodes[0], &in.Nodes[2]
	low := 0
	a.Config.Path.Interfaces = append(a.Config.Path.Interfaces, firewall.PathInterface{Name: "return-from-south", Address: "10.3.0.1/24", SecurityLevel: &low, Resolved: true})
	a.Config.Path.Routes = append(a.Config.Path.Routes, firewall.PathRoute{ID: "return-south-connected", Kind: "connected", Prefix: "10.3.0.0/24", Interface: "return-from-south"})
	a.Config.Chains = append(a.Config.Chains, firewall.Chain{ID: "return-south-in", Interface: "return-from-south", Direction: "in", Complete: true, Rules: []policy.Rule{{ID: "return-allow", Position: 1, Enabled: true, Action: "permit", Source: "any", Destination: "any", Protocol: "tcp"}}})
	c.Config.Path.Interfaces = append(c.Config.Path.Interfaces, firewall.PathInterface{Name: "south", Address: "10.2.0.1/24", SecurityLevel: &low, Resolved: true})
	c.Config.Path.Routes = append(c.Config.Path.Routes, firewall.PathRoute{ID: "south-connected", Kind: "connected", Prefix: "10.2.0.0/24", Interface: "south"}, firewall.PathRoute{ID: "return-south", Kind: "static", Prefix: "198.51.100.0/24", Interface: "south", NextHop: "10.2.0.2", Distance: 1})
	for i, route := range c.Config.Path.Routes {
		if route.ID == "c-return" {
			c.Config.Path.Routes = append(c.Config.Path.Routes[:i], c.Config.Path.Routes[i+1:]...)
			break
		}
	}
	d := makeConfiguredNode("asa-south", []firewall.PathInterface{{Name: "from-core", Address: "10.2.0.2/24", Resolved: true}, {Name: "to-edge", Address: "10.3.0.2/24", Resolved: true}}, []firewall.PathRoute{{ID: "south-in", Kind: "connected", Prefix: "10.2.0.0/24", Interface: "from-core"}, {ID: "south-out", Kind: "connected", Prefix: "10.3.0.0/24", Interface: "to-edge"}, {ID: "south-return", Kind: "static", Prefix: "198.51.100.0/24", Interface: "to-edge", NextHop: "10.3.0.1", Distance: 1}})
	in.Nodes = append(in.Nodes, d)
	in.Links = append(in.Links, Link{A: Endpoint{NodeID: "asa-c", Interface: "south"}, B: Endpoint{NodeID: "asa-south", Interface: "from-core"}}, Link{A: Endpoint{NodeID: "asa-south", Interface: "to-edge"}, B: Endpoint{NodeID: "asa-a", Interface: "return-from-south"}})
	got, err := Explore(in)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Branches) != 1 || got.Branches[0].Disposition != "indeterminate" || len(got.Branches[0].Returns) != 1 {
		t.Fatalf("expected one unresolved asymmetric return: %#v", got)
	}
	ret := got.Branches[0].Returns[0]
	if ret.SessionStatus != "unproven" || !ret.Asymmetric || ret.Disposition != "indeterminate" || ret.Direction.Disposition != "modeled_forward" {
		t.Fatalf("configured route and unproven state were not distinguished: %#v", ret)
	}
	if gotNodes := hopNodeIDs(ret.Direction.Hops); !reflect.DeepEqual(gotNodes, []string{"asa-c", "asa-south", "asa-a"}) {
		t.Fatalf("return route did not retain unseen firewall traversal: %v", gotNodes)
	}
}

func TestExploreDetectsRouteCycle(t *testing.T) {
	in := threeHopInput()
	a, b := &in.Nodes[0], &in.Nodes[1]
	level := 30
	a.Config.Path.Interfaces = append(a.Config.Path.Interfaces, firewall.PathInterface{Name: "loop-in", Address: "10.2.0.1/24", SecurityLevel: &level, Resolved: true})
	a.Config.Path.Routes = append(a.Config.Path.Routes, firewall.PathRoute{ID: "loop-connected", Kind: "connected", Prefix: "10.2.0.0/24", Interface: "loop-in"})
	b.Config.Path.Interfaces = append(b.Config.Path.Interfaces, firewall.PathInterface{Name: "loop-out", Address: "10.2.0.2/24", SecurityLevel: &level, Resolved: true})
	b.Config.Path.Routes = append(b.Config.Path.Routes, firewall.PathRoute{ID: "loop-connected", Kind: "connected", Prefix: "10.2.0.0/24", Interface: "loop-out"}, firewall.PathRoute{ID: "loop-destination", Kind: "static", Prefix: "192.0.2.10/32", Interface: "loop-out", NextHop: "10.2.0.1", Distance: 1})
	in.Links = append(in.Links, Link{A: Endpoint{NodeID: "asa-a", Interface: "loop-in"}, B: Endpoint{NodeID: "asa-b", Interface: "loop-out"}})
	got, err := Explore(in)
	if err != nil {
		t.Fatal(err)
	}
	if got.Disposition != "indeterminate" || len(got.Branches) != 1 || !strings.Contains(strings.Join(got.Branches[0].Forward.Reasons, " "), "cycle") {
		t.Fatalf("cycle was not exposed as indeterminate: %#v", got)
	}
}

func TestExploreRequiresUniqueLinkedPeerMatchingNextHop(t *testing.T) {
	t.Run("mismatch", func(t *testing.T) {
		in := fixtureInput()
		for i := range in.Nodes[0].Config.Path.Routes {
			if in.Nodes[0].Config.Path.Routes[i].ID == "next" {
				in.Nodes[0].Config.Path.Routes[i].NextHop = "10.0.0.99"
			}
		}
		got, err := Explore(in)
		if err != nil {
			t.Fatal(err)
		}
		if got.Disposition != "indeterminate" || len(got.Branches) != 1 || !strings.Contains(strings.Join(got.Branches[0].Forward.Reasons, " "), "next hop") {
			t.Fatalf("next-hop mismatch was accepted: %#v", got)
		}
	})
	t.Run("shared-endpoint-disambiguates", func(t *testing.T) {
		in := fixtureInput()
		peer := minimalPeer("asa-extra", "10.0.0.3/24")
		in.Nodes = append(in.Nodes, peer)
		in.Links = append(in.Links, Link{A: Endpoint{NodeID: "asa-a", Interface: "transit-out"}, B: Endpoint{NodeID: "asa-extra", Interface: "transit-in"}})
		got, err := Explore(in)
		if err != nil {
			t.Fatal(err)
		}
		if got.Disposition != "modeled_round_trip" {
			t.Fatalf("exact next hop did not disambiguate shared endpoint: %#v", got)
		}
	})
	t.Run("shared-endpoint-ambiguous", func(t *testing.T) {
		in := fixtureInput()
		peer := minimalPeer("asa-extra", "10.0.0.2/24")
		in.Nodes = append(in.Nodes, peer)
		in.Links = append(in.Links, Link{A: Endpoint{NodeID: "asa-a", Interface: "transit-out"}, B: Endpoint{NodeID: "asa-extra", Interface: "transit-in"}})
		got, err := Explore(in)
		if err != nil {
			t.Fatal(err)
		}
		if got.Disposition != "indeterminate" || !strings.Contains(strings.Join(got.Branches[0].Forward.Reasons, " "), "exactly one") {
			t.Fatalf("ambiguous next-hop peers were guessed: %#v", got)
		}
	})
}

func TestExploreBranchBudgetTruncationCannotClaimUniversalOutcome(t *testing.T) {
	in := twoStageFanoutInput()
	got, err := Explore(in)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Truncated || got.Complete || got.Disposition != "indeterminate" || len(got.Branches) > maxExplorationBranches {
		t.Fatalf("branch-budget truncation was not conservative: complete=%v truncated=%v disposition=%s branches=%d reasons=%v", got.Complete, got.Truncated, got.Disposition, len(got.Branches), got.Reasons)
	}
}

func TestExploreEvaluationBudgetTruncationCannotClaimUniversalOutcome(t *testing.T) {
	in := longFanoutChainInput()
	got, err := Explore(in)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Truncated || got.Complete || got.Disposition != "indeterminate" || !containsReason(got.Reasons, "hop evaluation limit") {
		t.Fatalf("evaluation-budget truncation was not conservative: %#v", got)
	}
}

func mixedOutcomeInput() Input {
	in := fixtureInput()
	a := &in.Nodes[0]
	low := 0
	a.Config.Path.Interfaces = append(a.Config.Path.Interfaces, firewall.PathInterface{Name: "alt-out", Address: "10.0.0.3/24", SecurityLevel: &low, Resolved: true})
	a.Config.Path.Routes = append(a.Config.Path.Routes,
		firewall.PathRoute{ID: "alt-connected", Kind: "connected", Prefix: "10.0.0.0/24", Interface: "alt-out"},
		firewall.PathRoute{ID: "next-alt", Kind: "static", Prefix: "0.0.0.0/0", Interface: "alt-out", NextHop: "10.0.0.4", Distance: 1})
	c := fixtureNode("asa-c", "10.0.0.4/24", "10.0.1.4/24", "", false)
	c.Config.Path.Routes[0].Prefix = "10.0.0.0/24"
	routes := c.Config.Path.Routes[:0]
	for _, r := range c.Config.Path.Routes {
		if r.ID != "destination-connected" {
			routes = append(routes, r)
		}
	}
	c.Config.Path.Routes = routes
	in.Nodes = append(in.Nodes, c)
	in.Links = append(in.Links, Link{A: Endpoint{NodeID: "asa-a", Interface: "alt-out"}, B: Endpoint{NodeID: "asa-c", Interface: "transit-in"}})
	return in
}

func twoStageFanoutInput() Input {
	nodes := []Node{}
	links := []Link{}
	aIf := []firewall.PathInterface{{Name: "source", Address: "198.51.100.1/24", Resolved: true}}
	aRoutes := []firewall.PathRoute{{ID: "a-source", Kind: "connected", Prefix: "198.51.100.0/24", Interface: "source"}}
	bIf := []firewall.PathInterface{}
	bRoutes := []firewall.PathRoute{}
	dIf := []firewall.PathInterface{{Name: "destination", Address: "192.0.2.1/24", Resolved: true}}
	dRoutes := []firewall.PathRoute{{ID: "d-destination", Kind: "connected", Prefix: "192.0.2.0/24", Interface: "destination"}}
	for i := 1; i <= 8; i++ {
		name := fmt.Sprintf("a-out-%d", i)
		peer := fmt.Sprintf("b-in-%d", i)
		prefix := fmt.Sprintf("10.1.%d.0/24", i)
		aIf = append(aIf, firewall.PathInterface{Name: name, Address: fmt.Sprintf("10.1.%d.1/24", i), Resolved: true})
		aRoutes = append(aRoutes, firewall.PathRoute{ID: "a-connected-" + name, Kind: "connected", Prefix: prefix, Interface: name}, firewall.PathRoute{ID: "a-next-" + name, Kind: "static", Prefix: "0.0.0.0/0", Interface: name, NextHop: fmt.Sprintf("10.1.%d.2", i), Distance: 1})
		bIf = append(bIf, firewall.PathInterface{Name: peer, Address: fmt.Sprintf("10.1.%d.2/24", i), Resolved: true})
		bRoutes = append(bRoutes, firewall.PathRoute{ID: "b-connected-" + peer, Kind: "connected", Prefix: prefix, Interface: peer})
		links = append(links, Link{A: Endpoint{NodeID: "fan-a", Interface: name}, B: Endpoint{NodeID: "fan-b", Interface: peer}})
	}
	for i := 1; i <= 8; i++ {
		name := fmt.Sprintf("b-out-%d", i)
		peer := fmt.Sprintf("d-in-%d", i)
		prefix := fmt.Sprintf("10.2.%d.0/24", i)
		bIf = append(bIf, firewall.PathInterface{Name: name, Address: fmt.Sprintf("10.2.%d.1/24", i), Resolved: true})
		bRoutes = append(bRoutes, firewall.PathRoute{ID: "b-connected-" + name, Kind: "connected", Prefix: prefix, Interface: name}, firewall.PathRoute{ID: "b-next-" + name, Kind: "static", Prefix: "0.0.0.0/0", Interface: name, NextHop: fmt.Sprintf("10.2.%d.2", i), Distance: 1})
		dIf = append(dIf, firewall.PathInterface{Name: peer, Address: fmt.Sprintf("10.2.%d.2/24", i), Resolved: true})
		dRoutes = append(dRoutes, firewall.PathRoute{ID: "d-connected-" + peer, Kind: "connected", Prefix: prefix, Interface: peer})
		links = append(links, Link{A: Endpoint{NodeID: "fan-b", Interface: name}, B: Endpoint{NodeID: "fan-d", Interface: peer}})
	}
	nodes = append(nodes, makeConfiguredNode("fan-a", aIf, aRoutes), makeConfiguredNode("fan-b", bIf, bRoutes), makeConfiguredNode("fan-d", dIf, dRoutes))
	scope := nodes[0].Scope
	return Input{Nodes: nodes, Links: links, Source: Endpoint{NodeID: "fan-a", Interface: "source"}, Destination: Endpoint{NodeID: "fan-d", Interface: "destination"}, Flow: policy.Flow{Scope: scope, Source: "198.51.100.50", Destination: "192.0.2.10", Protocol: "tcp", SourcePort: 40000, DestinationPort: 443}}
}

func longFanoutChainInput() Input {
	const count = 16
	nodes := make([]Node, count)
	links := []Link{}
	aIf := []firewall.PathInterface{{Name: "source", Address: "198.51.100.1/24", Resolved: true}}
	aRoutes := []firewall.PathRoute{{ID: "source-connected", Kind: "connected", Prefix: "198.51.100.0/24", Interface: "source"}}
	for i := 1; i <= 8; i++ {
		name := fmt.Sprintf("out-%d", i)
		peer := fmt.Sprintf("in-%d", i)
		aIf = append(aIf, firewall.PathInterface{Name: name, Address: fmt.Sprintf("10.10.%d.1/24", i), Resolved: true})
		aRoutes = append(aRoutes, firewall.PathRoute{ID: "connected-" + name, Kind: "connected", Prefix: fmt.Sprintf("10.10.%d.0/24", i), Interface: name}, firewall.PathRoute{ID: "default-" + name, Kind: "static", Prefix: "0.0.0.0/0", Interface: name, NextHop: fmt.Sprintf("10.10.%d.2", i), Distance: 1})
		links = append(links, Link{A: Endpoint{NodeID: "long-0", Interface: name}, B: Endpoint{NodeID: "long-1", Interface: peer}})
	}
	nodes[0] = makeConfiguredNode("long-0", aIf, aRoutes)
	bIf := []firewall.PathInterface{}
	bRoutes := []firewall.PathRoute{}
	for i := 1; i <= 8; i++ {
		name := fmt.Sprintf("in-%d", i)
		bIf = append(bIf, firewall.PathInterface{Name: name, Address: fmt.Sprintf("10.10.%d.2/24", i), Resolved: true})
		bRoutes = append(bRoutes, firewall.PathRoute{ID: "connected-" + name, Kind: "connected", Prefix: fmt.Sprintf("10.10.%d.0/24", i), Interface: name})
	}
	bIf = append(bIf, firewall.PathInterface{Name: "out", Address: "10.30.1.1/24", Resolved: true})
	bRoutes = append(bRoutes, firewall.PathRoute{ID: "out-connected", Kind: "connected", Prefix: "10.30.1.0/24", Interface: "out"}, firewall.PathRoute{ID: "next", Kind: "static", Prefix: "0.0.0.0/0", Interface: "out", NextHop: "10.30.1.2", Distance: 1})
	nodes[1] = makeConfiguredNode("long-1", bIf, bRoutes)
	links = append(links, Link{A: Endpoint{NodeID: "long-1", Interface: "out"}, B: Endpoint{NodeID: "long-2", Interface: "in"}})
	for i := 2; i < count-1; i++ {
		prevPrefix := fmt.Sprintf("10.30.%d.0/24", i-1)
		nextPrefix := fmt.Sprintf("10.30.%d.0/24", i)
		ifs := []firewall.PathInterface{{Name: "in", Address: fmt.Sprintf("10.30.%d.2/24", i-1), Resolved: true}, {Name: "out", Address: fmt.Sprintf("10.30.%d.1/24", i), Resolved: true}}
		routes := []firewall.PathRoute{{ID: "in-connected", Kind: "connected", Prefix: prevPrefix, Interface: "in"}, {ID: "out-connected", Kind: "connected", Prefix: nextPrefix, Interface: "out"}, {ID: "next", Kind: "static", Prefix: "0.0.0.0/0", Interface: "out", NextHop: fmt.Sprintf("10.30.%d.2", i), Distance: 1}}
		nodeID := fmt.Sprintf("long-%d", i)
		nodes[i] = makeConfiguredNode(nodeID, ifs, routes)
		links = append(links, Link{A: Endpoint{NodeID: nodeID, Interface: "out"}, B: Endpoint{NodeID: fmt.Sprintf("long-%d", i+1), Interface: "in"}})
	}
	lastIn := firewall.PathInterface{Name: "in", Address: "10.30.14.2/24", Resolved: true}
	dest := firewall.PathInterface{Name: "destination", Address: "192.0.2.1/24", Resolved: true}
	nodes[count-1] = makeConfiguredNode("long-15", []firewall.PathInterface{lastIn, dest}, []firewall.PathRoute{{ID: "in-connected", Kind: "connected", Prefix: "10.30.14.0/24", Interface: "in"}, {ID: "dest-connected", Kind: "connected", Prefix: "192.0.2.0/24", Interface: "destination"}})
	scope := nodes[0].Scope
	return Input{Nodes: nodes, Links: links, Source: Endpoint{NodeID: "long-0", Interface: "source"}, Destination: Endpoint{NodeID: "long-15", Interface: "destination"}, Flow: policy.Flow{Scope: scope, Source: "198.51.100.50", Destination: "192.0.2.10", Protocol: "tcp", SourcePort: 40000, DestinationPort: 443}}
}

func makeConfiguredNode(id string, interfaces []firewall.PathInterface, routes []firewall.PathRoute) Node {
	scope := policy.Scope{SnapshotID: "snap-" + id, DeviceID: id, Context: "default"}
	chains := []firewall.Chain{}
	level := 50
	for i, iface := range interfaces {
		if iface.SecurityLevel == nil {
			interfaces[i].SecurityLevel = &level
		}
		chains = append(chains, firewall.Chain{ID: fmt.Sprintf("in-%d", i), Interface: iface.Name, Direction: "in", Complete: true, Rules: []policy.Rule{{ID: "allow", Position: 1, Enabled: true, Action: "permit", Source: "any", Destination: "any", Protocol: "tcp"}}})
	}
	return Node{ID: id, Scope: scope, Config: firewall.Config{Vendor: "cisco-asa", Context: "default", Chains: chains, Path: &firewall.PathConfig{Version: "asa-path-v1", NATComplete: true, RoutingComplete: true, InterInterface: true, Interfaces: interfaces, Routes: routes}}}
}

func minimalPeer(id, address string) Node {
	return makeConfiguredNode(id, []firewall.PathInterface{{Name: "transit-in", Address: address, Resolved: true}}, nil)
}
func hopNodeIDs(hops []Hop) []string {
	out := make([]string, len(hops))
	for i, h := range hops {
		out[i] = h.NodeID
	}
	return out
}
func containsReason(reasons []string, substring string) bool {
	for _, r := range reasons {
		if strings.Contains(r, substring) {
			return true
		}
	}
	return false
}

func TestExploreKeepsBlockedForwardAlternativeSeparate(t *testing.T) {
	in := fixtureInput()
	// The terminal firewall has no route for the requested destination.
	in.Nodes[1].Config.Path.Routes = append([]firewall.PathRoute(nil), in.Nodes[1].Config.Path.Routes...)
	filtered := in.Nodes[1].Config.Path.Routes[:0]
	for _, route := range in.Nodes[1].Config.Path.Routes {
		if route.ID != "destination-connected" {
			filtered = append(filtered, route)
		}
	}
	in.Nodes[1].Config.Path.Routes = filtered
	got, err := Explore(in)
	if err != nil {
		t.Fatal(err)
	}
	if got.Disposition != "blocked" || len(got.Branches) != 1 || got.Branches[0].Disposition != "blocked" {
		t.Fatalf("complete no-route branch was not kept as blocked: %#v", got)
	}
}

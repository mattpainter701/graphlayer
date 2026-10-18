package networkpath

import (
	"testing"

	"github.com/mattpainter701/graphlayer/platform/firewall"
	"github.com/mattpainter701/graphlayer/platform/policy"
)

func fixtureNode(id, transitIn, transitOut, dest string, transit bool) Node {
	scope := policy.Scope{SnapshotID: "snap-" + id, DeviceID: id, Context: "default"}
	low, high := 0, 100
	interfaces := []firewall.PathInterface{
		{Name: "transit-in", Address: transitIn, SecurityLevel: &high, Resolved: true, EvidenceRefs: []string{"interface-in"}},
		{Name: "transit-out", Address: transitOut, SecurityLevel: &low, Resolved: true, EvidenceRefs: []string{"interface-out"}},
	}
	routes := []firewall.PathRoute{{ID: "in-connected", Kind: "connected", Prefix: prefixOf(transitIn), Interface: "transit-in"}}
	if transit {
		routes = append(routes, firewall.PathRoute{ID: "out-connected", Kind: "connected", Prefix: prefixOf(transitOut), Interface: "transit-out"})
		routes = append(routes, firewall.PathRoute{ID: "next", Kind: "static", Prefix: "0.0.0.0/0", Interface: "transit-out", NextHop: "10.0.0.2", Distance: 1})
	} else {
		interfaces = append(interfaces, firewall.PathInterface{Name: "destination", Address: "192.0.2.1/24", SecurityLevel: &low, Resolved: true, EvidenceRefs: []string{"destination-interface"}})
		routes = append(routes, firewall.PathRoute{ID: "destination-connected", Kind: "connected", Prefix: "192.0.2.0/24", Interface: "destination"})
		routes = append(routes, firewall.PathRoute{ID: "return-client", Kind: "static", Prefix: "198.51.100.0/24", Interface: "transit-in", NextHop: "10.0.0.1", Distance: 1})
	}
	return Node{ID: id, Scope: scope, Config: firewall.Config{Vendor: "cisco-asa", Context: "default", Chains: []firewall.Chain{{ID: "in-acl", Interface: "transit-in", Direction: "in", Complete: true, EvidenceRefs: []string{"acl-in"}, Rules: []policy.Rule{{ID: "permit", Position: 1, Enabled: true, Action: "permit", Source: "any", Destination: "any", Protocol: "tcp"}}}}, Path: &firewall.PathConfig{Version: "asa-path-v1", NATComplete: true, RoutingComplete: true, InterInterface: true, Interfaces: interfaces, Routes: routes, EvidenceRefs: []string{"path-evidence"}}}}
}

func prefixOf(address string) string {
	if address == "10.0.0.1/24" || address == "10.0.0.2/24" {
		return "10.0.0.0/24"
	}
	return "198.51.100.0/24"
}

func fixtureInput() Input {
	a := fixtureNode("asa-a", "198.51.100.1/24", "10.0.0.1/24", "", true)
	b := fixtureNode("asa-b", "10.0.0.2/24", "198.51.100.2/24", "", false)
	flow := policy.Flow{Scope: a.Scope, Source: "198.51.100.50", Destination: "192.0.2.10", Protocol: "tcp", SourcePort: 40000, DestinationPort: 443}
	return Input{Nodes: []Node{a, b}, Links: []Link{{A: Endpoint{NodeID: "asa-a", Interface: "transit-out"}, B: Endpoint{NodeID: "asa-b", Interface: "transit-in"}}}, Source: Endpoint{NodeID: "asa-a", Interface: "transit-in"}, Destination: Endpoint{NodeID: "asa-b", Interface: "destination"}, Flow: flow}
}

func TestEvaluateComposesExplicitTwoFirewallForwardPath(t *testing.T) {
	in := fixtureInput()
	got, err := Evaluate(in)
	if err != nil {
		t.Fatal(err)
	}
	if got.EngineContractVersion != ContractVersion || got.Forward.Disposition != "modeled_forward" || len(got.Forward.Hops) != 2 {
		t.Fatalf("unexpected forward result: %#v", got)
	}
	if got.Forward.FinalFlow.Destination != "192.0.2.10" || got.Forward.Hops[0].Link == nil {
		t.Fatalf("final tuple or selected link missing: %#v", got.Forward)
	}
	if got.Disposition != "modeled_round_trip" || got.Return.Disposition != "modeled_forward" || len(got.Return.Hops) != 2 {
		t.Fatalf("expected modeled stateful return path: %#v", got)
	}
	if len(got.Forward.Hops[0].Evidence) == 0 || got.Forward.Hops[0].Evidence[0].SnapshotID != "snap-asa-a" {
		t.Fatalf("evidence is not namespaced by snapshot: %#v", got.Forward.Hops[0].Evidence)
	}
}

func TestEvaluateMissingLinkAndMismatchedPeerAreIndeterminate(t *testing.T) {
	in := fixtureInput()
	in.Links = nil
	got, err := Evaluate(in)
	if err != nil {
		t.Fatal(err)
	}
	if got.Forward.Disposition != "indeterminate" || len(got.Forward.Hops) != 1 {
		t.Fatalf("missing link produced false path: %#v", got)
	}
	in = fixtureInput()
	in.Nodes[1].Config.Path.Interfaces[0].Address = "10.1.0.2/24"
	got, err = Evaluate(in)
	if err != nil {
		t.Fatal(err)
	}
	if got.Forward.Disposition != "indeterminate" {
		t.Fatalf("mismatched transit subnet produced path: %#v", got)
	}
}

func TestEvaluateRepeatedEndpointIsIndeterminate(t *testing.T) {
	in := fixtureInput()
	in.Links = append(in.Links, Link{A: Endpoint{NodeID: "asa-a", Interface: "transit-out"}, B: Endpoint{NodeID: "asa-b", Interface: "destination"}})
	got, err := Evaluate(in)
	if err != nil {
		t.Fatal(err)
	}
	if got.Disposition != "indeterminate" || len(got.Forward.Hops) != 0 {
		t.Fatalf("repeated endpoint accepted: %#v", got)
	}
}

func TestEvaluateClassifiesCompleteForwardAndReturnRouteAbsenceAsBlocked(t *testing.T) {
	in := fixtureInput()
	filteredForward := in.Nodes[1].Config.Path.Routes[:0]
	for _, route := range in.Nodes[1].Config.Path.Routes {
		if route.ID != "destination-connected" {
			filteredForward = append(filteredForward, route)
		}
	}
	in.Nodes[1].Config.Path.Routes = filteredForward
	got, err := Evaluate(in)
	if err != nil {
		t.Fatal(err)
	}
	if got.Disposition != "blocked" || got.Forward.Disposition != "blocked" {
		t.Fatalf("complete no-route was not blocked: %#v", got)
	}

	in = fixtureInput()
	filtered := in.Nodes[1].Config.Path.Routes[:0]
	for _, route := range in.Nodes[1].Config.Path.Routes {
		if route.ID != "return-client" {
			filtered = append(filtered, route)
		}
	}
	in.Nodes[1].Config.Path.Routes = filtered
	got, err = Evaluate(in)
	if err != nil {
		t.Fatal(err)
	}
	if got.Disposition != "blocked" || got.Forward.Disposition != "modeled_forward" || got.Return.Disposition != "blocked" {
		t.Fatalf("complete missing return route was not blocked: %#v", got)
	}
}

func TestEvaluateModelsThreeHopNoNATRoundTrip(t *testing.T) {
	in := threeHopInput()
	got, err := Evaluate(in)
	if err != nil {
		t.Fatal(err)
	}
	if got.Disposition != "modeled_round_trip" || got.Forward.Disposition != "modeled_forward" || got.Return.Disposition != "modeled_forward" || len(got.Forward.Hops) != 3 || len(got.Return.Hops) != 3 {
		t.Fatalf("unexpected three-hop round trip: %#v", got)
	}
	if got.Return.FinalFlow.Destination != in.Flow.Source {
		t.Fatalf("return final tuple did not reach source: %#v", got.Return.FinalFlow)
	}
	in.Flow.Protocol = "6"
	got, err = Evaluate(in)
	if err != nil {
		t.Fatal(err)
	}
	if got.Disposition != "modeled_round_trip" {
		t.Fatalf("numeric TCP protocol was not modeled: %#v", got)
	}
}

func TestEvaluateRejectsSourceAndDestinationNetworkOrBroadcastAddresses(t *testing.T) {
	for _, source := range []string{"198.51.100.0", "198.51.100.255", "198.51.100.1"} {
		t.Run("source-"+source, func(t *testing.T) {
			in := fixtureInput()
			in.Flow.Source = source
			got, err := Evaluate(in)
			if err != nil {
				t.Fatal(err)
			}
			if got.Disposition != "indeterminate" {
				t.Fatalf("invalid source endpoint was modeled: %#v", got)
			}
		})
	}
	for _, destination := range []string{"192.0.2.0", "192.0.2.255"} {
		t.Run("destination-"+destination, func(t *testing.T) {
			in := fixtureInput()
			in.Flow.Destination = destination
			got, err := Evaluate(in)
			if err != nil {
				t.Fatal(err)
			}
			if got.Disposition != "indeterminate" {
				t.Fatalf("invalid destination endpoint was modeled: %#v", got)
			}
		})
	}
}

func TestEvaluateDetectsCycleAcrossDistinctTopologyLinks(t *testing.T) {
	in := threeHopInput()
	// Route the middle firewall's selected destination back to the first node.
	a := &in.Nodes[0]
	b := &in.Nodes[1]
	level := 30
	a.Config.Path.Interfaces = append(a.Config.Path.Interfaces, firewall.PathInterface{Name: "loop-in", Address: "10.2.0.1/24", SecurityLevel: &level, Resolved: true})
	a.Config.Path.Routes = append(a.Config.Path.Routes, firewall.PathRoute{ID: "loop-connected", Kind: "connected", Prefix: "10.2.0.0/24", Interface: "loop-in"})
	b.Config.Path.Interfaces = append(b.Config.Path.Interfaces, firewall.PathInterface{Name: "loop-out", Address: "10.2.0.2/24", SecurityLevel: &level, Resolved: true})
	b.Config.Path.Routes = append(b.Config.Path.Routes, firewall.PathRoute{ID: "loop-connected", Kind: "connected", Prefix: "10.2.0.0/24", Interface: "loop-out"}, firewall.PathRoute{ID: "loop-destination", Kind: "static", Prefix: "192.0.2.10/32", Interface: "loop-out", NextHop: "10.2.0.1", Distance: 1})
	in.Links = append(in.Links, Link{A: Endpoint{NodeID: "asa-a", Interface: "loop-in"}, B: Endpoint{NodeID: "asa-b", Interface: "loop-out"}})
	got, err := Evaluate(in)
	if err != nil {
		t.Fatal(err)
	}
	if got.Disposition != "indeterminate" || len(got.Forward.Hops) != 2 {
		t.Fatalf("cycle was not stopped conservatively: %#v", got)
	}
}

func threeHopInput() Input {
	makeNode := func(id, left, leftAddr, right, rightAddr string, routes []firewall.PathRoute, terminal bool) Node {
		scope := policy.Scope{SnapshotID: "snap-" + id, DeviceID: id, Context: "default"}
		high, low := 100, 0
		ifs := []firewall.PathInterface{{Name: left, Address: leftAddr, SecurityLevel: &high, Resolved: true, EvidenceRefs: []string{"if-left"}}, {Name: right, Address: rightAddr, SecurityLevel: &low, Resolved: true, EvidenceRefs: []string{"if-right"}}}
		if terminal {
			ifs = append(ifs, firewall.PathInterface{Name: "destination", Address: "192.0.2.1/24", SecurityLevel: &low, Resolved: true, EvidenceRefs: []string{"if-destination"}})
			routes = append(routes, firewall.PathRoute{ID: "destination", Kind: "connected", Prefix: "192.0.2.0/24", Interface: "destination"})
		}
		chain := firewall.Chain{ID: "in", Interface: left, Direction: "in", Complete: true, EvidenceRefs: []string{"acl"}, Rules: []policy.Rule{{ID: "allow", Position: 1, Enabled: true, Action: "permit", Source: "any", Destination: "any", Protocol: "tcp"}}}
		return Node{ID: id, Scope: scope, Config: firewall.Config{Vendor: "cisco-asa", Context: "default", Chains: []firewall.Chain{chain}, Path: &firewall.PathConfig{Version: "asa-path-v1", NATComplete: true, RoutingComplete: true, InterInterface: true, Interfaces: ifs, Routes: routes}}}
	}
	a := makeNode("asa-a", "left", "198.51.100.1/24", "right", "10.0.0.1/24", []firewall.PathRoute{{ID: "a-left", Kind: "connected", Prefix: "198.51.100.0/24", Interface: "left"}, {ID: "a-right", Kind: "connected", Prefix: "10.0.0.0/24", Interface: "right"}, {ID: "a-next", Kind: "static", Prefix: "0.0.0.0/0", Interface: "right", NextHop: "10.0.0.2", Distance: 1}}, false)
	b := makeNode("asa-b", "left", "10.0.0.2/24", "right", "10.1.0.1/24", []firewall.PathRoute{{ID: "b-left", Kind: "connected", Prefix: "10.0.0.0/24", Interface: "left"}, {ID: "b-right", Kind: "connected", Prefix: "10.1.0.0/24", Interface: "right"}, {ID: "b-next", Kind: "static", Prefix: "0.0.0.0/0", Interface: "right", NextHop: "10.1.0.2", Distance: 1}, {ID: "b-return", Kind: "static", Prefix: "198.51.100.0/24", Interface: "left", NextHop: "10.0.0.1", Distance: 1}}, false)
	c := makeNode("asa-c", "left", "10.1.0.2/24", "unused", "198.18.0.1/24", []firewall.PathRoute{{ID: "c-left", Kind: "connected", Prefix: "10.1.0.0/24", Interface: "left"}, {ID: "c-return", Kind: "static", Prefix: "198.51.100.0/24", Interface: "left", NextHop: "10.1.0.1", Distance: 1}}, true)
	return Input{Nodes: []Node{a, b, c}, Links: []Link{{A: Endpoint{NodeID: "asa-a", Interface: "right"}, B: Endpoint{NodeID: "asa-b", Interface: "left"}}, {A: Endpoint{NodeID: "asa-b", Interface: "right"}, B: Endpoint{NodeID: "asa-c", Interface: "left"}}}, Source: Endpoint{NodeID: "asa-a", Interface: "left"}, Destination: Endpoint{NodeID: "asa-c", Interface: "destination"}, Flow: policy.Flow{Scope: a.Scope, Source: "198.51.100.50", Destination: "192.0.2.10", Protocol: "tcp", SourcePort: 40000, DestinationPort: 443}}
}

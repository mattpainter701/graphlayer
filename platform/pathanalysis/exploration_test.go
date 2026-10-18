package pathanalysis

import (
	"fmt"
	"testing"

	"github.com/mattpainter701/graphlayer/platform/firewall"
)

func TestExploreBranchesOnlyEqualBestRoutes(t *testing.T) {
	in := explorationBaseInput()
	in.Config.Path.Routes = append(in.Config.Path.Routes,
		firewall.PathRoute{ID: "default-peer-b", Kind: "static", Prefix: "0.0.0.0/0", Interface: "outside", NextHop: "198.51.100.3", Distance: 1, EvidenceRefs: []string{"route-peer-b"}},
		firewall.PathRoute{ID: "default-worse", Kind: "static", Prefix: "0.0.0.0/0", Interface: "outside", NextHop: "198.51.100.4", Distance: 20, EvidenceRefs: []string{"route-worse"}},
	)
	candidates, truncated, err := Explore(in)
	if err != nil {
		t.Fatal(err)
	}
	if truncated {
		t.Fatal("small route set was truncated")
	}
	if len(candidates) != 2 {
		t.Fatalf("candidate count = %d, want 2", len(candidates))
	}
	got := map[string]bool{}
	for _, c := range candidates {
		if c.Result.SelectedRoute == nil {
			t.Fatalf("candidate %s has no selected route", c.ID)
		}
		got[c.Result.SelectedRoute.ID] = true
		if !c.Resolved || c.Result.Disposition != "modeled_forward" {
			t.Fatalf("candidate %s is not a resolved permitted route", c.ID)
		}
	}
	if !got["default"] || !got["default-peer-b"] || got["default-worse"] {
		t.Fatalf("wrong active routes: %v", got)
	}
}

func TestExploreKeepsEqualBestRouteWithUnprovenNextHop(t *testing.T) {
	in := explorationBaseInput()
	in.Config.Path.Routes = append(in.Config.Path.Routes, firewall.PathRoute{ID: "default-unproven-peer", Kind: "static", Prefix: "0.0.0.0/0", Interface: "outside", NextHop: "192.0.2.1", Distance: 1, EvidenceRefs: []string{"route-unproven"}})
	candidates, truncated, err := Explore(in)
	if err != nil {
		t.Fatal(err)
	}
	if truncated || len(candidates) != 2 {
		t.Fatalf("got %d candidates, truncated=%v", len(candidates), truncated)
	}
	var sawPermit, sawUnresolved bool
	for _, c := range candidates {
		switch c.ID {
		case "candidate:default/default":
			sawPermit = c.Result.Disposition == "modeled_forward" && c.Resolved
		case "candidate:default/default-unproven-peer":
			sawUnresolved = c.Result.Disposition == "indeterminate" && !c.Resolved && c.Result.Stages[3].Status == "unresolved"
		}
	}
	if !sawPermit || !sawUnresolved {
		t.Fatalf("equal-best route evidence was dropped or overclaimed: %#v", candidates)
	}
}

func TestExploreIsDeterministic(t *testing.T) {
	in := explorationBaseInput()
	in.Config.Path.Routes = append(in.Config.Path.Routes, firewall.PathRoute{ID: "default-peer-b", Kind: "static", Prefix: "0.0.0.0/0", Interface: "outside", NextHop: "198.51.100.3", Distance: 1})
	a, at, err := Explore(in)
	if err != nil {
		t.Fatal(err)
	}
	b, bt, err := Explore(in)
	if err != nil {
		t.Fatal(err)
	}
	if at != bt || len(a) != len(b) {
		t.Fatal("candidate set changed between identical evaluations")
	}
	for i := range a {
		if a[i].ID != b[i].ID || a[i].Result.SelectedRoute.ID != b[i].Result.SelectedRoute.ID {
			t.Fatalf("candidate %d changed order or route", i)
		}
	}
}

func TestExploreRouteBudgetReportsTruncation(t *testing.T) {
	in := explorationBaseInput()
	for i := 2; i < 12; i++ {
		nextHop := fmt.Sprintf("198.51.100.%d", i+1)
		in.Config.Path.Routes = append(in.Config.Path.Routes, firewall.PathRoute{ID: fmt.Sprintf("default-%02d", i), Kind: "static", Prefix: "0.0.0.0/0", Interface: "outside", NextHop: nextHop, Distance: 1})
	}
	candidates, truncated, err := Explore(in)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != maxRouteCandidates || !truncated {
		t.Fatalf("got %d candidates, truncated=%v", len(candidates), truncated)
	}
}

func TestExploreReturnRouteBudgetStopsAtEight(t *testing.T) {
	in := explorationBaseInput()
	for i := 2; i < 12; i++ {
		in.Config.Path.Routes = append(in.Config.Path.Routes, firewall.PathRoute{ID: fmt.Sprintf("return-%02d", i), Kind: "static", Prefix: "10.1.0.10/32", Interface: "inside", NextHop: fmt.Sprintf("10.1.0.%d", i+1), Distance: 1})
	}
	forwardCandidates, forwardTruncated, err := Explore(in)
	if err != nil {
		t.Fatal(err)
	}
	if forwardTruncated || len(forwardCandidates) != 1 || forwardCandidates[0].Result.Disposition != "modeled_forward" {
		t.Fatal("return-budget setup must establish one permitted forward path")
	}
	forward := forwardCandidates[0].Result
	ret := Input{Scope: in.Scope, Config: in.Config, IngressInterface: forward.EgressInterface, Flow: reverseFlow(forward.EgressFlow, in.Scope)}
	candidates, truncated, err := ExploreReturn(ret, &forward)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != maxRouteCandidates || !truncated {
		t.Fatalf("got %d return candidates, truncated=%v", len(candidates), truncated)
	}
}

func TestExploreReturnWithoutForwardRefusesConfiguredNAT(t *testing.T) {
	in := baseInput()
	in.Config.Path.NATRules = []firewall.PathNATRule{{ID: "web", ObjectName: "web", RealInterface: "inside", MappedInterface: "outside", RealPrefix: "10.1.0.10/32", MappedPrefix: "203.0.113.10/32"}}
	candidates, _, err := ExploreReturn(in, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) == 0 {
		t.Fatal("expected unresolved candidate")
	}
	for _, c := range candidates {
		if c.SessionStatus != "unproven" || c.Resolved || c.Result.EgressFlow.Source != "" || c.Result.EgressFlow.Destination != "" {
			t.Fatalf("NAT state was treated as traversable: %#v", c)
		}
	}
}

func TestExploreReturnWithoutForwardRefusesPortNATWithIdentityAddresses(t *testing.T) {
	in := baseInput()
	in.Config.Path.NATRules = []firewall.PathNATRule{{ID: "service-map", ObjectName: "service-map", RealInterface: "inside", MappedInterface: "outside", RealPrefix: "10.1.0.10/32", MappedPrefix: "10.1.0.10/32", Protocol: "tcp", RealPort: 80, MappedPort: 443}}
	candidates, _, err := ExploreReturn(in, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) == 0 {
		t.Fatal("expected unresolved candidate")
	}
	for _, c := range candidates {
		if c.SessionStatus != "unproven" || c.Resolved || c.Result.EgressFlow.Source != "" || c.Result.EgressFlow.Destination != "" {
			t.Fatalf("port NAT state was treated as traversable: %#v", c)
		}
	}
}

func TestExploreReturnInspectsNoNATRouteWithDifferentIngress(t *testing.T) {
	in := explorationBaseInput()
	forwardCandidates, _, err := Explore(in)
	if err != nil {
		t.Fatal(err)
	}
	forward := forwardCandidates[0].Result
	in.Config.Path.Interfaces = append(in.Config.Path.Interfaces, firewall.PathInterface{Name: "alternate", Address: "10.2.0.1/24", SecurityLevel: intPtr(50), Resolved: true})
	in.Config.Path.Routes = append(in.Config.Path.Routes,
		firewall.PathRoute{ID: "alternate-connected", Kind: "connected", Prefix: "10.2.0.0/24", Interface: "alternate"},
		firewall.PathRoute{ID: "alternate-source", Kind: "static", Prefix: "10.1.0.10/32", Interface: "alternate", NextHop: "10.2.0.2", Distance: 1},
	)
	ret := Input{Scope: in.Scope, Config: in.Config, IngressInterface: "inside", Flow: reverseFlow(forward.EgressFlow, in.Scope)}
	candidates, _, err := ExploreReturn(ret, &forward)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 || candidates[0].SessionStatus != "asymmetric_unknown" || !candidates[0].Resolved || candidates[0].Result.Disposition != "indeterminate" {
		t.Fatalf("unexpected asymmetric candidate: %#v", candidates)
	}
}

func TestExploreReturnPreservesSingleAsymmetricRouteTuple(t *testing.T) {
	in := explorationBaseInput()
	forwardCandidates, _, err := Explore(in)
	if err != nil {
		t.Fatal(err)
	}
	forward := forwardCandidates[0].Result
	in.Config.Path.Interfaces = append(in.Config.Path.Interfaces, firewall.PathInterface{Name: "alternate", Address: "10.2.0.1/24", SecurityLevel: intPtr(50), Resolved: true})
	in.Config.Path.Routes = append(in.Config.Path.Routes,
		firewall.PathRoute{ID: "alternate-connected", Kind: "connected", Prefix: "10.2.0.0/24", Interface: "alternate"},
		firewall.PathRoute{ID: "alternate-source", Kind: "static", Prefix: "10.1.0.10/32", Interface: "alternate", NextHop: "10.2.0.2", Distance: 1},
	)
	ret := Input{Scope: in.Scope, Config: in.Config, IngressInterface: forward.EgressInterface, Flow: reverseFlow(forward.EgressFlow, in.Scope)}
	candidates, _, err := ExploreReturn(ret, &forward)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 || candidates[0].SessionStatus != "asymmetric_unknown" || !candidates[0].Resolved || candidates[0].Result.EgressInterface != "alternate" {
		t.Fatalf("single asymmetric route lost its concrete tuple: %#v", candidates)
	}
}

func TestExploreReturnMissingRouteKeepsBlock(t *testing.T) {
	in := explorationBaseInput()
	in.Config.Path.Routes = []firewall.PathRoute{
		{ID: "outside-connected", Kind: "connected", Prefix: "198.51.100.0/24", Interface: "outside"},
		{ID: "forward-host", Kind: "static", Prefix: "203.0.113.10/32", Interface: "outside", NextHop: "198.51.100.1", Distance: 1},
	}
	forwardCandidates, _, err := Explore(in)
	if err != nil {
		t.Fatal(err)
	}
	forward := forwardCandidates[0].Result
	if forward.Disposition != "modeled_forward" {
		t.Fatalf("forward setup failed: %#v", forward)
	}
	ret := Input{Scope: in.Scope, Config: in.Config, IngressInterface: forward.EgressInterface, Flow: reverseFlow(forward.EgressFlow, in.Scope)}
	candidates, _, err := ExploreReturn(ret, &forward)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 || candidates[0].Result.Disposition != "blocked" || candidates[0].SessionStatus != "established" || candidates[0].Resolved {
		t.Fatalf("missing return route was not kept as a known block: %#v", candidates)
	}
}

func explorationBaseInput() Input {
	in := baseInput()
	in.IngressInterface = "inside"
	in.Flow.Source = "10.1.0.10"
	in.Config.Chains[0].Interface = "inside"
	return in
}

func intPtr(v int) *int { return &v }

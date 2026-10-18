package pathanalysis

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/mattpainter701/graphlayer/platform/firewall"
	"github.com/mattpainter701/graphlayer/platform/policy"
)

func baseInput() Input {
	scope := policy.Scope{SnapshotID: "snap-a", DeviceID: "asa-a", Context: "default"}
	inside, outside := 100, 0
	return Input{
		Scope: scope, IngressInterface: "outside",
		Flow: policy.Flow{Scope: scope, Source: "198.51.100.20", Destination: "203.0.113.10", Protocol: "tcp", SourcePort: 40000, DestinationPort: 443},
		Config: firewall.Config{Vendor: "cisco-asa", Context: "default", Chains: []firewall.Chain{{ID: "acl-outside-in", Interface: "outside", Direction: "in", Complete: true, Rules: []policy.Rule{{ID: "allow-https", Position: 0, Enabled: true, Action: "permit", Source: "any", Destination: "any", Protocol: "tcp", DestinationPort: "443", EvidenceRefs: []string{"acl-evidence"}}}}}, Path: &firewall.PathConfig{
			Version: "asa-path-v1", NATComplete: true, RoutingComplete: true,
			Interfaces: []firewall.PathInterface{{Name: "outside", Address: "198.51.100.2/24", SecurityLevel: &outside, Resolved: true, EvidenceRefs: []string{"if-outside"}}, {Name: "inside", Address: "10.1.0.1/24", SecurityLevel: &inside, Resolved: true, EvidenceRefs: []string{"if-inside"}}},
			Routes:     []firewall.PathRoute{{ID: "inside-connected", Kind: "connected", Prefix: "10.1.0.0/24", Interface: "inside", EvidenceRefs: []string{"route-inside"}}, {ID: "outside-connected", Kind: "connected", Prefix: "198.51.100.0/24", Interface: "outside", EvidenceRefs: []string{"route-outside"}}, {ID: "default", Kind: "static", Prefix: "0.0.0.0/0", Interface: "outside", NextHop: "198.51.100.1", Distance: 1, EvidenceRefs: []string{"route-default"}}},
		}},
	}
}

func TestDNATUsesRealAddressForACLAndSelectsDirectConnectedRealInterface(t *testing.T) {
	in := baseInput()
	in.Config.Path.NATRules = []firewall.PathNATRule{{ID: "web-static", ObjectName: "web", RealInterface: "inside", MappedInterface: "outside", RealPrefix: "10.1.0.10/32", MappedPrefix: "203.0.113.10/32", EvidenceRefs: []string{"nat-web"}}}
	in.Config.Chains[0].Rules[0].Destination = "10.1.0.10"
	got, err := Evaluate(in)
	if err != nil {
		t.Fatal(err)
	}
	if got.Disposition != "modeled_forward" {
		t.Fatalf("disposition = %q; reasons=%v", got.Disposition, got.Reasons)
	}
	if got.PolicyFlow.Destination != "10.1.0.10" {
		t.Fatalf("policy destination = %s", got.PolicyFlow.Destination)
	}
	if got.EgressInterface != "inside" || got.Stages[3].Status != "matched" {
		t.Fatalf("route stage = %#v", got.Stages[3])
	}
	if !contains(got.EvidenceRefs, "nat-web") || !contains(got.EvidenceRefs, "acl-evidence") {
		t.Fatalf("missing path evidence: %v", got.EvidenceRefs)
	}
}

func TestStaticObjectNATUsesSpecificObjectBeforeBroadObject(t *testing.T) {
	in := baseInput()
	in.Config.Path.NATRules = []firewall.PathNATRule{
		{ID: "broad", ObjectName: "broad", RealInterface: "inside", MappedInterface: "outside", RealPrefix: "10.1.0.0/24", MappedPrefix: "203.0.113.0/24", EvidenceRefs: []string{"nat-broad"}},
		{ID: "host", ObjectName: "host", RealInterface: "inside", MappedInterface: "outside", RealPrefix: "10.1.0.10/32", MappedPrefix: "203.0.113.10/32", EvidenceRefs: []string{"nat-host"}},
	}
	in.Config.Chains[0].Rules[0].Destination = "10.1.0.10"
	got, err := Evaluate(in)
	if err != nil {
		t.Fatal(err)
	}
	if got.Disposition != "modeled_forward" || len(got.NATRuleIDs) != 1 || got.NATRuleIDs[0] != "host" {
		t.Fatalf("unexpected NAT selection: %#v", got)
	}
}

func TestStaticObjectNATTieUsesPreservedObjectNameOrder(t *testing.T) {
	in := baseInput()
	in.Config.Path.NATRules = []firewall.PathNATRule{
		{ID: "lower", ObjectName: "aObject", RealInterface: "inside", MappedInterface: "outside", RealPrefix: "10.1.0.10/32", MappedPrefix: "203.0.113.10/32", EvidenceRefs: []string{"nat-lower"}},
		{ID: "upper", ObjectName: "AObject", RealInterface: "inside", MappedInterface: "outside", RealPrefix: "10.1.0.10/32", MappedPrefix: "203.0.113.10/32", EvidenceRefs: []string{"nat-upper"}},
	}
	in.Config.Chains[0].Rules[0].Destination = "10.1.0.10"
	got, err := Evaluate(in)
	if err != nil {
		t.Fatal(err)
	}
	if got.Disposition != "modeled_forward" || got.NATRuleIDs[0] != "upper" {
		t.Fatalf("unexpected object-name ordering: %#v", got.NATRuleIDs)
	}
}

func TestSourceNATRunsAfterRouteAndMustAgreeWithEgress(t *testing.T) {
	in := baseInput()
	in.IngressInterface = "inside"
	in.Flow.Source = "10.1.0.10"
	in.Config.Chains[0].Interface = "inside"
	in.Config.Path.NATRules = []firewall.PathNATRule{{ID: "source-static", ObjectName: "source", RealInterface: "inside", MappedInterface: "outside", RealPrefix: "10.1.0.10/32", MappedPrefix: "198.51.100.10/32", EvidenceRefs: []string{"nat-source"}}}
	got, err := Evaluate(in)
	if err != nil {
		t.Fatal(err)
	}
	if got.Disposition != "modeled_forward" || got.EgressFlow.Source != "198.51.100.10" || got.Stages[4].Status != "matched" {
		t.Fatalf("unexpected source translation: %#v", got)
	}
	in.Config.Path.NATRules[0].MappedInterface = "inside"
	got, err = Evaluate(in)
	if err != nil {
		t.Fatal(err)
	}
	if got.Disposition != "indeterminate" || got.EgressFlow.Source != "" || got.Stages[4].Status != "unresolved" {
		t.Fatalf("expected route/NAT mismatch without final tuple, got %#v", got)
	}
}

func TestDNATWithoutDirectConnectedRealRouteIsIndeterminate(t *testing.T) {
	in := baseInput()
	in.Config.Path.NATRules = []firewall.PathNATRule{{ID: "web-static", ObjectName: "web", RealInterface: "inside", MappedInterface: "outside", RealPrefix: "10.2.0.10/32", MappedPrefix: "203.0.113.10/32", EvidenceRefs: []string{"nat-web"}}}
	got, err := Evaluate(in)
	if err != nil {
		t.Fatal(err)
	}
	if got.Disposition != "indeterminate" || got.Stages[3].Status != "unresolved" {
		t.Fatalf("expected unresolved DNAT route, got %#v", got)
	}
}

func TestConnectedRouteMustMatchInterfaceAddressPrefix(t *testing.T) {
	in := baseInput()
	in.Config.Path.Routes[0].Prefix = "10.2.0.0/24"
	in.Config.Path.NATRules = []firewall.PathNATRule{{ID: "web-static", ObjectName: "web", RealInterface: "inside", MappedInterface: "outside", RealPrefix: "10.1.0.10/32", MappedPrefix: "203.0.113.10/32", EvidenceRefs: []string{"nat-web"}}}
	in.Config.Chains[0].Rules[0].Destination = "10.1.0.10"
	got, err := Evaluate(in)
	if err != nil {
		t.Fatal(err)
	}
	if got.Disposition != "indeterminate" || got.Stages[3].Status != "unresolved" {
		t.Fatalf("expected route/interface mismatch uncertainty, got %#v", got)
	}
}

func TestIncompleteRoutingCannotProduceForwardVerdict(t *testing.T) {
	in := baseInput()
	in.Config.Path.RoutingComplete = false
	got, err := Evaluate(in)
	if err != nil {
		t.Fatal(err)
	}
	if got.Disposition != "indeterminate" {
		t.Fatalf("disposition = %q", got.Disposition)
	}
}

func TestIncompleteRoutingAlsoBlocksDNATVerdict(t *testing.T) {
	in := baseInput()
	in.Config.Path.RoutingComplete = false
	in.Config.Path.NATRules = []firewall.PathNATRule{{ID: "web-static", ObjectName: "web", RealInterface: "inside", MappedInterface: "outside", RealPrefix: "10.1.0.10/32", MappedPrefix: "203.0.113.10/32", EvidenceRefs: []string{"nat-web"}}}
	in.Config.Chains[0].Rules[0].Destination = "10.1.0.10"
	got, err := Evaluate(in)
	if err != nil {
		t.Fatal(err)
	}
	if got.Disposition != "indeterminate" || got.Stages[3].Status != "unresolved" {
		t.Fatalf("expected routing uncertainty, got %#v", got)
	}
}

func TestEqualBestRoutesAreIndeterminate(t *testing.T) {
	in := baseInput()
	in.Config.Path.Routes = append(in.Config.Path.Routes, firewall.PathRoute{ID: "default-2", Kind: "static", Prefix: "0.0.0.0/0", Interface: "outside", NextHop: "198.51.100.2", Distance: 1, EvidenceRefs: []string{"route-default-2"}})
	got, err := Evaluate(in)
	if err != nil {
		t.Fatal(err)
	}
	if got.Disposition != "indeterminate" || got.Stages[3].Status != "unresolved" {
		t.Fatalf("expected route ambiguity, got %#v", got.Stages[3])
	}
}

func TestIncompleteNATCannotProduceForwardVerdict(t *testing.T) {
	in := baseInput()
	in.Config.Path.NATComplete = false
	got, err := Evaluate(in)
	if err != nil {
		t.Fatal(err)
	}
	if got.Disposition != "indeterminate" || got.Stages[1].Status != "unresolved" {
		t.Fatalf("expected incomplete NAT, got %#v", got)
	}
}

func TestCompleteModelWithoutRouteIsBlocked(t *testing.T) {
	in := baseInput()
	in.Config.Path.Routes = in.Config.Path.Routes[:2]
	got, err := Evaluate(in)
	if err != nil {
		t.Fatal(err)
	}
	if got.Disposition != "blocked" || got.Stages[3].Status != "blocked" {
		t.Fatalf("expected no-route block, got %#v", got.Stages[3])
	}
}

func TestStaticRouteNextHopCannotBeEgressAddress(t *testing.T) {
	in := baseInput()
	in.Config.Path.Routes[2].NextHop = "198.51.100.2"
	got, err := Evaluate(in)
	if err != nil {
		t.Fatal(err)
	}
	if got.Disposition != "indeterminate" || got.EgressFlow.Destination != "" {
		t.Fatalf("expected unresolved next hop, got %#v", got)
	}
}

func TestStaticRouteNextHopCannotUseNetworkOrDirectedBroadcast(t *testing.T) {
	for _, nextHop := range []string{"198.51.100.0", "198.51.100.255"} {
		t.Run(nextHop, func(t *testing.T) {
			in := baseInput()
			in.Config.Path.Routes[2].NextHop = nextHop
			got, err := Evaluate(in)
			if err != nil {
				t.Fatal(err)
			}
			if got.Disposition != "indeterminate" || got.Stages[3].Status != "unresolved" {
				t.Fatalf("expected unusable next-hop uncertainty, got %#v", got.Stages[3])
			}
		})
	}
}

func TestStaticRouteAllowsUsableNextHopOnPointToPointPrefix(t *testing.T) {
	in := baseInput()
	in.IngressInterface = "inside"
	in.Config.Chains[0].Interface = "inside"
	in.Config.Path.Interfaces[0].Address = "198.51.100.2/31"
	in.Config.Path.Routes[1].Prefix = "198.51.100.2/31"
	in.Config.Path.Routes[2].NextHop = "198.51.100.3"
	got, err := Evaluate(in)
	if err != nil {
		t.Fatal(err)
	}
	if got.Disposition != "modeled_forward" {
		t.Fatalf("expected usable /31 next hop, got %q (%v)", got.Disposition, got.Reasons)
	}
}

func TestInterfaceAndACLNamesMatchCaseInsensitively(t *testing.T) {
	in := baseInput()
	in.IngressInterface = "OUTSIDE"
	in.Config.Chains[0].Interface = "Outside"
	in.Config.Path.Routes = append(in.Config.Path.Routes, firewall.PathRoute{ID: "destination-inside", Kind: "static", Prefix: "203.0.113.0/24", Interface: "inside", NextHop: "10.1.0.2", Distance: 1, EvidenceRefs: []string{"route-inside-dest"}})
	got, err := Evaluate(in)
	if err != nil {
		t.Fatal(err)
	}
	if got.Disposition != "modeled_forward" || got.IngressInterface != "outside" {
		t.Fatalf("unexpected case-insensitive match: %#v", got)
	}
}

func TestTransitFlowRejectsMulticastAndLoopbackEndpoints(t *testing.T) {
	for _, destination := range []string{"224.0.0.1", "127.0.0.1", "255.255.255.255", "0.0.0.0"} {
		t.Run(destination, func(t *testing.T) {
			in := baseInput()
			in.Flow.Destination = destination
			if _, err := Evaluate(in); err == nil {
				t.Fatal("expected unsupported transit endpoint error")
			}
		})
	}
}

func TestOwnInterfaceDestinationIsOutsideTransitModel(t *testing.T) {
	in := baseInput()
	in.Flow.Destination = "198.51.100.2"
	got, err := Evaluate(in)
	if err != nil {
		t.Fatal(err)
	}
	if got.Disposition != "indeterminate" || got.Stages[1].Status != "unresolved" {
		t.Fatalf("expected control-plane uncertainty, got %#v", got)
	}
}

func TestUnknownIngressIsInvalidInput(t *testing.T) {
	in := baseInput()
	in.IngressInterface = "missing"
	if _, err := Evaluate(in); err == nil {
		t.Fatal("expected unknown ingress error")
	}
}

func TestNonASAVendorCannotBeEvaluated(t *testing.T) {
	in := baseInput()
	in.Config.Vendor = "other-vendor"
	got, err := Evaluate(in)
	if err != nil {
		t.Fatal(err)
	}
	if got.Disposition != "indeterminate" {
		t.Fatalf("disposition = %q", got.Disposition)
	}
}

func TestGlobalAccessGroupPreventsForwardVerdict(t *testing.T) {
	in := baseInput()
	in.Config.Warnings = []firewall.Warning{{Code: "unsupported_binding", Message: "Unsupported access-group binding"}}
	got, err := Evaluate(in)
	if err != nil {
		t.Fatal(err)
	}
	if got.Disposition != "indeterminate" || got.Stages[2].Status != "unresolved" {
		t.Fatalf("expected unsupported binding uncertainty, got %#v", got)
	}
}

func TestDefaultDenyCarriesAttachmentEvidence(t *testing.T) {
	in := baseInput()
	in.Config.Chains[0].Rules = []policy.Rule{}
	in.Config.Chains[0].EvidenceRefs = []string{"acl-attachment"}
	got, err := Evaluate(in)
	if err != nil {
		t.Fatal(err)
	}
	if got.Disposition != "blocked" || !contains(got.Policy.EvidenceRefs, "acl-attachment") || !contains(got.Stages[2].EvidenceRefs, "acl-attachment") {
		t.Fatalf("default deny lacks attachment evidence: %#v", got)
	}
}

func TestEvaluateIsDeterministicAndDoesNotMutateInput(t *testing.T) {
	in := baseInput()
	in.Config.Path.NATRules = []firewall.PathNATRule{{ID: "web-static", ObjectName: "web", RealInterface: "inside", MappedInterface: "outside", RealPrefix: "10.1.0.10/32", MappedPrefix: "203.0.113.10/32", EvidenceRefs: []string{"nat-web"}}}
	in.Config.Chains[0].Rules[0].Destination = "10.1.0.10"
	before, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	first, err := Evaluate(in)
	if err != nil {
		t.Fatal(err)
	}
	after, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("Evaluate mutated its input")
	}
	second, err := Evaluate(in)
	if err != nil {
		t.Fatal(err)
	}
	b1, _ := json.Marshal(first)
	b2, _ := json.Marshal(second)
	if !bytes.Equal(b1, b2) {
		t.Fatalf("replay output differs:\n%s\n%s", b1, b2)
	}
}

func TestIngressACLUsesRealDestination(t *testing.T) {
	in := baseInput()
	in.Config.Path.NATRules = []firewall.PathNATRule{{ID: "web-static", ObjectName: "web", RealInterface: "inside", MappedInterface: "outside", RealPrefix: "10.1.0.10/32", MappedPrefix: "203.0.113.10/32", EvidenceRefs: []string{"nat-web"}}}
	in.Config.Chains[0].Rules[0].Action = "deny"
	in.Config.Chains[0].Rules[0].Destination = "10.1.0.10"
	got, err := Evaluate(in)
	if err != nil {
		t.Fatal(err)
	}
	if got.Disposition != "blocked" || got.Stages[2].Status != "blocked" {
		t.Fatalf("expected ACL block, got %#v", got)
	}
}

func TestUnsupportedEgressACLPreventsAffirmativeVerdict(t *testing.T) {
	in := baseInput()
	in.Config.Chains = append(in.Config.Chains, firewall.Chain{ID: "egress-acl", Interface: "outside", Direction: "out", Complete: true})
	got, err := Evaluate(in)
	if err != nil {
		t.Fatal(err)
	}
	if got.Disposition != "indeterminate" {
		t.Fatalf("disposition = %q", got.Disposition)
	}
}

func contains(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}

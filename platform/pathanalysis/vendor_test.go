package pathanalysis

import (
	"strings"
	"testing"

	"github.com/mattpainter701/graphlayer/platform/firewall"
	"github.com/mattpainter701/graphlayer/platform/policy"
)

func vendorInput(vendor, version string) Input {
	in := baseInput()
	in.Config.Vendor = vendor
	in.Config.Path.Version = version
	in.Config.Path.PolicyComplete = true
	in.Config.Path.Interfaces[0].Zone = "untrust"
	in.Config.Path.Interfaces[1].Zone = "trust"
	in.Config.Path.EvidenceRefs = []string{"config-evidence"}
	in.Config.Path.Routes[0].EvidenceRefs = []string{"inside-route"}
	in.Config.Path.Routes[2].EvidenceRefs = []string{"default-route"}
	in.Config.Path.Interfaces[0].EvidenceRefs = []string{"outside-interface"}
	in.Config.Path.Interfaces[1].EvidenceRefs = []string{"inside-interface"}
	in.Config.Path.Routes[0].ID = "inside-connected"
	in.Config.Path.Routes[0].Prefix = "10.1.0.0/24"
	in.Config.Path.Routes[0].Interface = "inside"
	in.Config.Path.Routes[2].Prefix = "0.0.0.0/0"
	in.Config.Path.Routes[2].Interface = "outside"
	in.Config.Path.Routes[2].NextHop = "198.51.100.1"
	in.Config.Path.Routes[2].ID = "default"
	in.Config.Path.Translations = []firewall.PathTranslation{{ID: "vip-web", Kind: "dnat", From: "outside", To: "outside", OriginalPrefix: "203.0.113.10/32", TranslatedPrefix: "10.1.0.10/32", Match: policy.Rule{ID: "vip-match", Enabled: true, Action: "permit", Source: "any", Destination: "203.0.113.10", Protocol: "6"}, EvidenceRefs: []string{"vip-evidence"}}}
	in.Config.Path.Policies = []firewall.PathPolicyRule{{ID: "allow-vip", From: "outside", To: "inside", VIP: "vip-web", Match: policy.Rule{ID: "allow-vip", Position: 0, Enabled: true, Action: "permit", Source: "any", Destination: "203.0.113.10", Protocol: "6"}, EvidenceRefs: []string{"policy-evidence"}}}
	in.Flow.Protocol = "6"
	in.Config.Path.Translations[0].Match.Destination = in.Flow.Destination
	in.Config.Path.Policies[0].Match.Destination = in.Flow.Destination
	return in
}

func TestPANOSUsesOriginalTupleForNATAndPolicyButPostNATRoute(t *testing.T) {
	in := vendorInput("paloalto-panos", "panos-path-v1")
	in.IngressInterface = "outside"
	in.Config.Path.Interfaces[0].Zone = "untrust"
	in.Config.Path.Interfaces[1].Zone = "trust"
	in.Config.Path.Translations[0].From = "untrust"
	in.Config.Path.Translations[0].To = "untrust"
	in.Config.Path.Policies[0].From = "untrust"
	in.Config.Path.Policies[0].To = "trust"
	in.Config.Path.Policies[0].VIP = ""
	in.Config.Path.Translations[0].Match.Destination = "203.0.113.10"
	in.Config.Path.Policies[0].Match.Destination = "203.0.113.10"
	got, err := Evaluate(in)
	if err != nil {
		t.Fatal(err)
	}
	if got.Disposition != "modeled_forward" || got.EngineContractVersion != VendorContractVersion {
		t.Fatalf("unexpected PAN-OS result: %#v", got)
	}
	if got.EgressInterface != "inside" || got.PolicyFlow.Destination != "203.0.113.10" || got.EgressFlow.Destination != "10.1.0.10" {
		t.Fatalf("wrong original/post-NAT tuple handling: %#v", got)
	}
	if got.Policy == nil || got.Policy.Flow.Destination != "203.0.113.10" || got.SelectedRoute == nil || got.SelectedRoute.ID != "inside-connected" {
		t.Fatalf("wrong policy or route evidence: %#v", got)
	}
}

func TestPANOSDisabledEarlierNATIsSkippedForLaterActiveRule(t *testing.T) {
	disabled := strings.Join([]string{
		`set rulebase nat rules disabled-first from trust`,
		`set rulebase nat rules disabled-first to untrust`,
		`set rulebase nat rules disabled-first source client-10-1-0-42`,
		`set rulebase nat rules disabled-first destination any`,
		`set rulebase nat rules disabled-first service any`,
		`set rulebase nat rules disabled-first source-translation static-ip translated-address 198.51.100.99`,
		`set rulebase nat rules disabled-first disabled yes`,
	}, "\n") + "\n"
	c, err := firewall.ParsePANOSConfig(append([]byte(disabled), firewall.SamplePANOS()...))
	if err != nil {
		t.Fatal(err)
	}
	scope := policy.Scope{SnapshotID: "panos-disabled-nat", DeviceID: "edge-fw", Context: c.Context}
	flow := policy.Flow{Scope: scope, Source: "10.1.0.42", Destination: "203.0.113.15", Protocol: "tcp", SourcePort: 49152, DestinationPort: 443}
	got, err := Evaluate(Input{Scope: scope, Config: *c, IngressInterface: "ethernet1/1", Flow: flow})
	if err != nil {
		t.Fatal(err)
	}
	if got.Disposition != "modeled_forward" || len(got.NATRuleIDs) != 1 || got.NATRuleIDs[0] != "panos-nat:client-static" || got.EgressFlow.Source != "198.51.100.42" {
		t.Fatalf("disabled first NAT rule prevented the later active translation: %#v", got)
	}
}

func TestFortiGateVIPRequiresReferencedPolicyAndReturnInvertsForwardMapping(t *testing.T) {
	in := vendorInput("fortinet-fortigate", "fortios-path-v1")
	in.Config.Path.Translations[0].From = "outside"
	in.Config.Path.Translations[0].To = "outside"
	in.Config.Path.Policies[0].From = "outside"
	in.Config.Path.Policies[0].To = "inside"
	in.Config.Path.Policies[0].SourceNAT = ""
	in.Config.Path.Policies[0].Match.Destination = "10.1.0.10"
	fwd, err := Evaluate(in)
	if err != nil {
		t.Fatal(err)
	}
	if fwd.Disposition != "modeled_forward" || fwd.EgressFlow.Destination != "10.1.0.10" {
		t.Fatalf("unexpected FortiGate forward result: %#v", fwd)
	}
	ret := Input{Scope: in.Scope, Config: in.Config, IngressInterface: fwd.EgressInterface, Flow: reverseFlow(fwd.EgressFlow, in.Scope)}
	got, err := EvaluateReturn(ret, fwd)
	if err != nil {
		t.Fatal(err)
	}
	if got.Disposition != "modeled_forward" || got.EgressFlow != reverseFlow(fwd.OriginalFlow, in.Scope) {
		t.Fatalf("return did not invert selected VIP mapping: %#v", got)
	}
	if st := stageByName(got.Stages, "source_nat"); st.Status != "matched" || st.Before == nil || st.After == nil || st.Before.Source != "10.1.0.10" || st.After.Source != "203.0.113.10" || len(st.CandidateIDs) != 1 || st.CandidateIDs[0] != "vip-web" {
		t.Fatalf("return did not expose its selected reverse DNAT mapping: %#v", st)
	}
	wrongTuple := ret
	wrongTuple.Flow.DestinationPort++
	if _, err := EvaluateReturn(wrongTuple, fwd); err == nil {
		t.Fatal("accepted a return tuple with a mismatched port")
	}

	plain := in
	plain.Config.Path.Policies = append([]firewall.PathPolicyRule(nil), in.Config.Path.Policies...)
	plain.Config.Path.Policies[0].VIP = ""
	got, err = Evaluate(plain)
	if err != nil {
		t.Fatal(err)
	}
	if got.Disposition == "modeled_forward" {
		t.Fatalf("plain allow policy authorized VIP traffic: %#v", got)
	}
}

func TestFortiGateConfiguredVIPOnDifferentIngressCannotUsePlainPermit(t *testing.T) {
	in := vendorInput("fortinet-fortigate", "fortios-path-v1")
	in.IngressInterface = "inside"
	in.Flow.Source = "10.1.0.20"
	in.Config.Path.Policies = []firewall.PathPolicyRule{{
		ID: "plain-inside-out", From: "inside", To: "outside",
		Match: policy.Rule{ID: "plain-inside-out", Position: 0, Enabled: true, Action: "permit", Source: "any", Destination: "203.0.113.10", Protocol: "6"},
	}}
	got, err := Evaluate(in)
	if err != nil {
		t.Fatal(err)
	}
	if got.Disposition != "indeterminate" || got.EngineContractVersion != VendorContractVersion {
		t.Fatalf("configured VIP was routed under a plain permit from another interface: %#v", got)
	}
}

func TestVendorFirstMatchDenyAndIncompleteEvidenceStayConservative(t *testing.T) {
	in := vendorInput("paloalto-panos", "panos-path-v1")
	in.Config.Path.Interfaces[0].Zone = "untrust"
	in.Config.Path.Interfaces[1].Zone = "trust"
	in.Config.Path.Translations[0].From = "untrust"
	in.Config.Path.Translations[0].To = "untrust"
	in.Config.Path.Policies = []firewall.PathPolicyRule{
		{ID: "deny-first", From: "any", To: "any", Match: policy.Rule{ID: "deny-first", Position: 0, Enabled: true, Action: "deny", Source: "any", Destination: "any", Protocol: "6"}},
		{ID: "allow-later", From: "untrust", To: "trust", Match: policy.Rule{ID: "allow-later", Position: 1, Enabled: true, Action: "permit", Source: "any", Destination: "any", Protocol: "6"}},
	}
	got, err := Evaluate(in)
	if err != nil {
		t.Fatal(err)
	}
	if got.Disposition != "blocked" {
		t.Fatalf("first matching deny was not applied: %#v", got)
	}
	in.Config.Path.PolicyComplete = false
	got, err = Evaluate(in)
	if err != nil {
		t.Fatal(err)
	}
	if got.Disposition != "indeterminate" {
		t.Fatalf("incomplete vendor facts produced a conclusion: %#v", got)
	}
	in.Config.Path.PolicyComplete = true
	in.Config.Path.Policies = []firewall.PathPolicyRule{{ID: "disabled-allow", From: "any", To: "any", Match: policy.Rule{ID: "disabled-allow", Position: 0, Enabled: false, Action: "permit", Source: "any", Destination: "any", Protocol: "6"}}}
	got, err = Evaluate(in)
	if err != nil {
		t.Fatal(err)
	}
	if got.Disposition != "blocked" {
		t.Fatalf("disabled permit rule became active: %#v", got)
	}
}

func TestFortiGatePolicySNATMustBeBoundToSelectedPolicy(t *testing.T) {
	in := vendorInput("fortinet-fortigate", "fortios-path-v1")
	in.Flow.Destination = "198.51.100.20"
	in.Flow.Source = "10.1.0.10"
	in.IngressInterface = "inside"
	in.Config.Path.Interfaces[1].Zone = "inside"
	in.Config.Path.Interfaces[0].Zone = "outside"
	in.Config.Path.Translations = []firewall.PathTranslation{{ID: "pool-static", Kind: "snat", From: "inside", To: "outside", OriginalPrefix: "10.1.0.10/32", TranslatedPrefix: "198.51.100.10/32", PolicyID: "allow-out", Match: policy.Rule{ID: "snat-match", Enabled: true, Action: "permit", Source: "10.1.0.10", Destination: "any", Protocol: "6"}}}
	in.Config.Path.Policies = []firewall.PathPolicyRule{{ID: "allow-out", From: "inside", To: "outside", SourceNAT: "pool-static", Match: policy.Rule{ID: "allow-out", Enabled: true, Position: 0, Action: "permit", Source: "10.1.0.10", Destination: "any", Protocol: "6"}}}
	got, err := Evaluate(in)
	if err != nil {
		t.Fatal(err)
	}
	if got.Disposition != "modeled_forward" || got.EgressFlow.Source != "198.51.100.10" {
		t.Fatalf("selected policy SNAT was not applied: %#v", got)
	}
	ret := Input{Scope: in.Scope, Config: in.Config, IngressInterface: got.EgressInterface, Flow: reverseFlow(got.EgressFlow, in.Scope)}
	returnResult, err := EvaluateReturn(ret, got)
	if err != nil {
		t.Fatal(err)
	}
	if returnResult.Disposition != "modeled_forward" || returnResult.EgressFlow != reverseFlow(got.OriginalFlow, in.Scope) || returnResult.PolicyFlow.Destination != "10.1.0.10" || returnResult.OriginalFlow != ret.Flow {
		t.Fatalf("return did not use the selected forward SNAT mapping and actual reply tuple: %#v", returnResult)
	}
	if st := stageByName(returnResult.Stages, "destination_nat"); st.Status != "matched" || st.Before == nil || st.After == nil || st.Before.Destination != "198.51.100.10" || st.After.Destination != "10.1.0.10" || len(st.CandidateIDs) != 1 || st.CandidateIDs[0] != "pool-static" {
		t.Fatalf("return did not expose its selected reverse SNAT mapping: %#v", st)
	}
	wrong := in
	wrong.Config.Path.Policies = append([]firewall.PathPolicyRule(nil), in.Config.Path.Policies...)
	wrong.Config.Path.Policies[0].SourceNAT = "missing"
	got, err = Evaluate(wrong)
	if err != nil {
		t.Fatal(err)
	}
	if got.Disposition != "indeterminate" {
		t.Fatalf("missing selected-policy SNAT reference was treated as complete: %#v", got)
	}
}

func TestVendorInputDeterminism(t *testing.T) {
	in := vendorInput("paloalto-panos", "panos-path-v1")
	in.Config.Path.Interfaces[0].Zone = "untrust"
	in.Config.Path.Interfaces[1].Zone = "trust"
	in.Config.Path.Translations[0].From = "untrust"
	in.Config.Path.Translations[0].To = "untrust"
	in.Config.Path.Policies[0].From = "untrust"
	in.Config.Path.Policies[0].To = "trust"
	a, err := Evaluate(in)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Evaluate(in)
	if err != nil {
		t.Fatal(err)
	}
	if a.Disposition != "modeled_forward" || b.Disposition != "modeled_forward" || len(a.NATRuleIDs) != len(b.NATRuleIDs) || a.EgressFlow != b.EgressFlow {
		t.Fatalf("non-deterministic result: %#v / %#v", a, b)
	}
}

func TestVendorCompleteNoRouteIsBlocked(t *testing.T) {
	in := vendorInput("paloalto-panos", "panos-path-v1")
	in.Config.Path.Interfaces[0].Zone = "untrust"
	in.Config.Path.Interfaces[1].Zone = "trust"
	in.Config.Path.Translations = nil
	in.Config.Path.Routes = []firewall.PathRoute{{ID: "inside-connected", Kind: "connected", Prefix: "10.1.0.0/24", Interface: "inside"}}
	in.Config.Path.Policies = []firewall.PathPolicyRule{{ID: "allow", From: "untrust", To: "untrust", Match: policy.Rule{ID: "allow", Position: 0, Enabled: true, Action: "permit", Source: "any", Destination: "any", Protocol: "6"}}}
	got, err := Evaluate(in)
	if err != nil {
		t.Fatal(err)
	}
	if got.Disposition != "blocked" {
		t.Fatalf("complete missing route was not blocked: %#v", got)
	}
}

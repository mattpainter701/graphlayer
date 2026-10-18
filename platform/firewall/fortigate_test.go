package firewall

import (
	"encoding/json"
	"github.com/mattpainter701/graphlayer/platform/policy"
	"strings"
	"testing"
)

func TestFortiGatePreservesPhysicalPolicyOrderAndReadableEvidence(t *testing.T) {
	input := strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(string(SampleFortiGate()), "\r\n", "\n"), "edit 10\n", "edit 100\n"), "edit 20\n", "edit 1\n")
	c, err := ParseFortiGateConfig([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(c.Path.Policies[0].ID, "policy:100:") || !strings.HasPrefix(c.Path.Policies[1].ID, "policy:1:") {
		t.Fatalf("policy ID numbers changed configured order: %#v", c.Path.Policies)
	}
	b, _ := json.Marshal(c.SourceLines)
	for _, text := range []string{"set ip 172.16.0.2 255.255.255.252", "set extip 203.0.113.15", "set mappedip 10.2.0.15", "set dstaddr WEB", "set action accept"} {
		if !strings.Contains(string(b), text) {
			t.Fatalf("missing readable source command %q", text)
		}
	}
	first, _ := json.Marshal(c)
	again, err := ParseFortiGateConfig([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	second, _ := json.Marshal(again)
	if string(first) != string(second) {
		t.Fatal("FortiOS normalization is not deterministic")
	}
}

func TestParseFortiGateSamplePathPoliciesAndEvidence(t *testing.T) {
	c, err := ParseFortiGateConfig(SampleFortiGate())
	if err != nil {
		t.Fatal(err)
	}
	if c.Vendor != "fortinet-fortigate" || c.Context != "root" || c.ParserVersion != FortiGateParserVersion || c.Path == nil || c.Path.Version != fortiPathVersion {
		t.Fatalf("wrong parser identity/path: %#v", c)
	}
	p := c.Path
	if len(p.Interfaces) != 2 || len(p.Routes) != 3 || len(p.Policies) != 2 || len(p.Translations) != 1 {
		t.Fatalf("unexpected inventories: interfaces=%d routes=%d policies=%d translations=%d", len(p.Interfaces), len(p.Routes), len(p.Policies), len(p.Translations))
	}
	if p.Routes[0].Kind != "connected" || p.Routes[2].Prefix != "198.51.100.42/32" || p.Routes[2].NextHop != "172.16.0.1" {
		t.Fatalf("static/connected route mismatch: %#v", p.Routes)
	}
	if !p.PolicyComplete || !p.RoutingComplete || !p.NATComplete {
		t.Fatalf("complete sample marked incomplete: %#v", p)
	}
	pol := p.Policies[0]
	if pol.Match.Action != "permit" || pol.Match.Source != "198.51.100.42/32" || pol.Match.Destination != "10.2.0.15/32" || pol.Match.Protocol != "tcp" || pol.Match.DestinationPort != "443" || pol.VIP != "vip:WEB" {
		t.Fatalf("policy mismatch: %#v", pol)
	}
	tr := p.Translations[0]
	if tr.ID != "vip:WEB" || tr.Kind != "dnat" || tr.OriginalPrefix != "203.0.113.15/32" || tr.TranslatedPrefix != "10.2.0.15/32" || tr.From != "port1" || tr.To != "any" {
		t.Fatalf("VIP mismatch: %#v", tr)
	}
	if len(pol.EvidenceRefs) == 0 || len(tr.EvidenceRefs) == 0 {
		t.Fatal("missing evidence")
	}
	assertFortiEvidence(t, c)
	validateFortiRules(t, p)
}

func TestFortiGateNativeServiceProtocolSubset(t *testing.T) {
	base := string(SampleFortiGate())
	for _, tt := range []struct {
		name, replace, with string
		complete            bool
	}{
		{"aggregate with one TCP range", "set protocol TCP/UDP/SCTP", "set protocol TCP/UDP/SCTP", true},
		{"default aggregate with one TCP range", "set protocol TCP/UDP/SCTP", "", true},
		{"multi transport", "set tcp-portrange 443", "set tcp-portrange 443\n        set udp-portrange 443", false},
		{"SCTP only", "set protocol TCP/UDP/SCTP", "set protocol SCTP", false},
		{"UDP-Lite aggregate", "set protocol TCP/UDP/SCTP", "set protocol TCP/UDP/UDP-Lite/SCTP", false},
		{"unsupported service matcher", "set tcp-portrange 443", "set tcp-portrange 443\n        set icmp-type 8", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			input := strings.Replace(base, tt.replace, tt.with, 1)
			c, err := ParseFortiGateConfig([]byte(input))
			if err != nil {
				t.Fatal(err)
			}
			if c.Path.PolicyComplete != tt.complete {
				t.Fatalf("policy completeness=%v, want %v; warnings=%#v", c.Path.PolicyComplete, tt.complete, c.Path.Warnings)
			}
			if tt.complete && (len(c.Path.Policies) != 2 || c.Path.Policies[0].Match.Protocol != "tcp" || c.Path.Policies[0].Match.DestinationPort != "443") {
				t.Fatalf("native aggregate service was not narrowed to its TCP port: %#v", c.Path.Policies)
			}
		})
	}
}

func TestFortiGateStaticSNATIsTiedToPolicy(t *testing.T) {
	input := string(SampleFortiGate())
	input = strings.Replace(input, "end\nconfig firewall policy", "end\nconfig firewall ippool\n    edit \"OUT\"\n        set type one-to-one\n        set startip 203.0.113.9\n        set endip 203.0.113.9\n        set arp-reply disable\n    next\nend\nconfig firewall policy", 1)
	input = strings.Replace(input, "set status enable\n    next", "set nat enable\n        set ippool enable\n        set poolname \"OUT\"\n        set status enable\n    next", 1)
	c, err := ParseFortiGateConfig([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	p := c.Path
	if len(p.Translations) != 2 {
		t.Fatalf("expected VIP and SNAT translations: %#v", p.Translations)
	}
	var snat *PathTranslation
	for i := range p.Translations {
		if p.Translations[i].Kind == "snat" {
			snat = &p.Translations[i]
		}
	}
	if snat == nil || snat.ID != "snat:10:1" || snat.PolicyID == "" || snat.TranslatedPrefix != "203.0.113.9/32" || p.Policies[0].SourceNAT != snat.ID {
		t.Fatalf("SNAT relation mismatch: %#v", snat)
	}
	assertFortiEvidence(t, c)
	validateFortiRules(t, p)
}

func TestFortiGateRedactsUnsupportedAndSensitiveLines(t *testing.T) {
	input := string(SampleFortiGate()) + "config system admin\n    edit \"admin\"\n        set password ENC supersecretvalue\n        set description \"secret-description-value\"\n    next\nend\n"
	c, err := ParseFortiGateConfig([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range c.SourceLines {
		if strings.Contains(line.Text, "supersecretvalue") || strings.Contains(line.Text, "secret-description-value") {
			t.Fatalf("sensitive input leaked: %q", line.Text)
		}
	}
	if strings.Contains(strings.Join([]string{c.ArtifactSHA256}, ""), "supersecretvalue") {
		t.Fatal("input hash leaked a secret")
	}
	assertFortiEvidence(t, c)
}

func TestFortiGateUnsupportedNATAndRoutingMarkIncomplete(t *testing.T) {
	tests := []struct {
		name, insert, code string
		route, nat         bool
	}{
		{"central NAT", "config system settings\n    set central-nat enable\nend\n", "central_nat_unsupported", true, false},
		{"central SNAT", "config firewall central-snat-map\n    edit 1\n        set nat enable\n    next\nend\n", "central_snat_unsupported", true, false},
		{"port forward", "", "vip_port_forward_unsupported", true, false},
		{"dynamic route", "", "unsupported_route", false, true},
		{"PAT", "", "dynamic_snat_unsupported", true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := string(SampleFortiGate())
			switch tt.name {
			case "port forward":
				cfg = strings.Replace(cfg, "set mappedip \"10.2.0.15\"", "set mappedip \"10.2.0.15\"\n        set portforward enable", 1)
			case "dynamic route":
				cfg = strings.Replace(cfg, "set device \"port1\"", "set device \"port1\"\n        set priority 1", 1)
			case "PAT":
				cfg = strings.Replace(cfg, "set action accept", "set action accept\n        set nat enable", 1)
			}
			if tt.insert != "" {
				cfg = strings.Replace(cfg, "config router static", tt.insert+"config router static", 1)
			}
			c, err := ParseFortiGateConfig([]byte(cfg))
			if err != nil {
				t.Fatal(err)
			}
			if c.Path.RoutingComplete != tt.route || c.Path.NATComplete != tt.nat {
				t.Fatalf("completeness routing=%v NAT=%v", c.Path.RoutingComplete, c.Path.NATComplete)
			}
			if !fortiHasWarning(c.Path.Warnings, tt.code) {
				t.Fatalf("missing warning %s: %#v", tt.code, c.Path.Warnings)
			}
		})
	}
}

func TestFortiGatePoolBoundsAndUnsupportedFlags(t *testing.T) {
	input := string(SampleFortiGate())
	input = strings.Replace(input, "set status enable\n    next", "set nat enable\n        set ippool enable\n        set poolname \"UNKNOWN\"\n        set status enable\n    next", 1)
	c, err := ParseFortiGateConfig([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if c.Path.PolicyComplete || fortiHasWarning(c.Path.Warnings, "unsupported_policy_snat") { // unresolved named pool is reported as a safe unsupported NAT input
		if c.Path.PolicyComplete {
			t.Fatal("unresolved pool left policy inventory complete")
		}
	} else {
		t.Fatal("missing unresolved-pool warning")
	}
	bad := strings.Replace(string(SampleFortiGate()), "end\nconfig firewall policy", "end\nconfig firewall ippool\n edit \"OUT\"\n set type one-to-one\n set startip 203.0.113.9\n set endip 203.0.113.10\n next\nend\nconfig firewall policy", 1)
	_, err = ParseFortiGateConfig([]byte(bad))
	if err != nil {
		t.Fatal(err)
	}
}

func TestFortiGateIPPoolARPAndUnusedAddressCompleteness(t *testing.T) {
	base := string(SampleFortiGate())
	makeConfig := func(arp string, use bool) string {
		pool := "end\nconfig firewall ippool\n    edit \"OUT\"\n        set type one-to-one\n        set startip 203.0.113.9\n        set endip 203.0.113.9\n"
		if arp != "" {
			pool += "        set arp-reply " + arp + "\n"
		}
		pool += "    next\nend\nconfig firewall policy"
		cfg := strings.Replace(base, "end\nconfig firewall policy", pool, 1)
		if use {
			cfg = strings.Replace(cfg, "set status enable\n    next", "set nat enable\n        set ippool enable\n        set poolname \"OUT\"\n        set status enable\n    next", 1)
		}
		return cfg
	}
	for _, tt := range []struct {
		name, arp   string
		use, expect bool
	}{
		{"unused default ARP", "", false, false},
		{"unused enabled ARP", "enable", false, false},
		{"used default ARP", "", true, false},
		{"used enabled ARP", "enable", true, false},
		{"unused disabled ARP", "disable", false, true},
		{"used disabled ARP", "disable", true, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c, err := ParseFortiGateConfig([]byte(makeConfig(tt.arp, tt.use)))
			if err != nil {
				t.Fatal(err)
			}
			if c.Path.NATComplete != tt.expect {
				t.Fatalf("NAT completeness=%v, want %v; warnings=%#v", c.Path.NATComplete, tt.expect, c.Path.Warnings)
			}
			if !tt.expect && !fortiHasWarning(c.Path.Warnings, "ippool_local_address_unsupported") {
				t.Fatalf("missing pool ARP warning: %#v", c.Path.Warnings)
			}
		})
	}
}

func TestFortiGateOneToOnePoolAddressCannotServeDifferentSourcePrefixes(t *testing.T) {
	input := string(SampleFortiGate())
	input = strings.Replace(input, "end\nconfig firewall service custom", "    edit \"SECOND_HOST\"\n        set subnet 198.51.100.43 255.255.255.255\n    next\nend\nconfig firewall service custom", 1)
	input = strings.Replace(input, "end\nconfig firewall policy", "end\nconfig firewall ippool\n    edit \"OUT\"\n        set type one-to-one\n        set startip 203.0.113.9\n        set endip 203.0.113.9\n        set arp-reply disable\n    next\nend\nconfig firewall policy", 1)
	input = strings.Replace(input, "set status enable\n    next", "set nat enable\n        set ippool enable\n        set poolname \"OUT\"\n        set status enable\n    next", 1)
	second := "    edit 11\n" +
		"        set srcintf \"port1\"\n" +
		"        set dstintf \"port2\"\n" +
		"        set srcaddr \"SECOND_HOST\"\n" +
		"        set dstaddr \"all\"\n" +
		"        set service \"HTTPS\"\n" +
		"        set schedule \"always\"\n" +
		"        set action accept\n" +
		"        set nat enable\n" +
		"        set ippool enable\n" +
		"        set poolname \"OUT\"\n" +
		"        set status enable\n" +
		"    next\n"
	input = strings.Replace(input, "    edit 20", second+"    edit 20", 1)
	c, err := ParseFortiGateConfig([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if c.Path.NATComplete || !fortiHasWarning(c.Path.Warnings, "ippool_address_reused") {
		t.Fatalf("reused one-to-one address left NAT complete: %#v", c.Path.Warnings)
	}
}

func TestFortiGateOneToOnePoolRequiresSingleSourceHost(t *testing.T) {
	input := string(SampleFortiGate())
	input = strings.Replace(input, "end\nconfig firewall policy", "end\nconfig firewall ippool\n    edit \"OUT\"\n        set type one-to-one\n        set startip 203.0.113.9\n        set endip 203.0.113.9\n        set arp-reply disable\n    next\nend\nconfig firewall policy", 1)
	input = strings.Replace(input, "set status enable\n    next", "set nat enable\n        set ippool enable\n        set poolname \"OUT\"\n        set status enable\n    next", 1)
	input = strings.Replace(input, "set subnet 198.51.100.42 255.255.255.255", "set subnet 198.51.100.0 255.255.255.0", 1)
	c, err := ParseFortiGateConfig([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if c.Path.PolicyComplete || c.Path.NATComplete || fortiHasWarning(c.Path.Warnings, "unsupported_policy_snat") == false {
		t.Fatalf("multi-host SNAT source remained complete: %#v", c.Path)
	}
	for _, tr := range c.Path.Translations {
		if tr.Kind == "snat" {
			t.Fatalf("unsupported multi-host SNAT translation was emitted: %#v", tr)
		}
	}
}

func TestFortiGateInterfaceWithoutAddressIsIncompleteButValid(t *testing.T) {
	input := strings.Replace(string(SampleFortiGate()), "end\nconfig router static", "    edit \"port3\"\n        set vdom \"root\"\n        set status up\n    next\nend\nconfig router static", 1)
	c, err := ParseFortiGateConfig([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, iface := range c.Path.Interfaces {
		if iface.Name == "port3" {
			found = true
			if iface.Resolved || iface.Address != "" {
				t.Fatalf("addressless interface was treated as resolved: %#v", iface)
			}
		}
	}
	if !found || c.Path.RoutingComplete || !fortiHasWarning(c.Path.Warnings, "unsupported_interface") {
		t.Fatalf("addressless interface did not produce incomplete routing evidence: %#v", c.Path)
	}
}

func TestFortiGateInvalidPathBoundsMarkRoutingIncomplete(t *testing.T) {
	longName := strings.Repeat("x", 49)
	cases := map[string]string{
		"long interface": strings.Replace(string(SampleFortiGate()), "end\nconfig router static", "    edit \""+longName+"\"\n        set vdom \"root\"\n        set ip 10.3.0.1 255.255.255.0\n    next\nend\nconfig router static", 1),
		"large distance": strings.Replace(string(SampleFortiGate()), "set device \"port1\"", "set device \"port1\"\n        set distance 255", 1),
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			c, err := ParseFortiGateConfig([]byte(input))
			if err != nil {
				t.Fatal(err)
			}
			if c.Path.RoutingComplete {
				t.Fatalf("invalid path bound left routing complete: %#v", c.Path.Warnings)
			}
		})
	}
}

func TestFortiGateUnsupportedPolicyCommandMakesPathIncomplete(t *testing.T) {
	input := strings.Replace(string(SampleFortiGate()), "set action accept", "set action accept\n        append dstaddr \"all\"", 1)
	c, err := ParseFortiGateConfig([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if c.Path.RoutingComplete || c.Path.PolicyComplete || c.Path.NATComplete || !fortiHasWarning(c.Path.Warnings, "unsupported_config") {
		t.Fatalf("unsupported policy command left path complete: %#v", c.Path)
	}
	for _, warning := range c.Path.Warnings {
		if warning.Code == "unsupported_config" {
			if c.SourceLines[warning.Line-1].Text != "[UNSUPPORTED CONFIGURATION COMMAND]" {
				t.Fatalf("unsupported command source was not redacted: %q", c.SourceLines[warning.Line-1].Text)
			}
			return
		}
	}
	t.Fatal("unsupported command warning has no source line")
}

func TestFortiGateMalformedAndNestedBlocksFailClosed(t *testing.T) {
	for _, input := range []string{"config firewall policy\n edit 1\n set action accept\nend\n", "config firewall policy\n edit 1\n set action accept\n next\n next\nend\n", "config firewall policy\n edit 1\n set name \"x\n next\nend\n"} {
		if _, err := ParseFortiGateConfig([]byte(input)); err == nil {
			t.Fatalf("accepted malformed input %q", input)
		}
	}
	if _, err := ParseFortiGateConfig(make([]byte, fortiMaxBytes+1)); err == nil {
		t.Fatal("accepted oversized input")
	}
}

func fortiHasWarning(a []Warning, code string) bool {
	for _, w := range a {
		if w.Code == code {
			return true
		}
	}
	return false
}
func assertFortiEvidence(t *testing.T, c *Config) {
	t.Helper()
	by := map[string]bool{}
	lineByID := map[string]int{}
	for _, e := range c.Evidence {
		if e.ID == "" || e.StartLine < 1 || e.EndLine < e.StartLine {
			t.Fatalf("invalid evidence: %#v", e)
		}
		by[e.ID] = true
		lineByID[e.ID] = e.StartLine
	}
	check := func(label string, refs []string) {
		for _, r := range refs {
			if !by[r] {
				t.Errorf("%s refers to missing evidence %q", label, r)
			}
			if lineByID[r] > 0 && strings.HasPrefix(c.SourceLines[lineByID[r]-1].Text, "[OMITTED") {
				t.Errorf("%s refers to omitted source line %d", label, lineByID[r])
			}
		}
	}
	for _, r := range c.Path.EvidenceRefs {
		check("path", []string{r})
	}
	for _, i := range c.Path.Interfaces {
		check("interface", i.EvidenceRefs)
	}
	for _, r := range c.Path.Routes {
		check("route", r.EvidenceRefs)
	}
	for _, r := range c.Path.Policies {
		check("policy", r.EvidenceRefs)
		check("match", r.Match.EvidenceRefs)
	}
	for _, r := range c.Path.Translations {
		check("translation", r.EvidenceRefs)
	}
}

func validateFortiRules(t *testing.T, p *PathConfig) {
	t.Helper()
	scope := policy.Scope{SnapshotID: "sample", DeviceID: "fortigate", Context: "root"}
	flow := policy.Flow{Scope: scope, Source: "198.51.100.42", Destination: "203.0.113.15", Protocol: "tcp", SourcePort: 12345, DestinationPort: 443}
	for _, r := range p.Policies {
		if r.Match.Action != "permit" && r.Match.Action != "deny" {
			t.Fatalf("non-normalized policy action %q", r.Match.Action)
		}
		if err := policy.Validate(policy.Bundle{Scope: scope, Complete: true, DefaultAction: "deny", Rules: []policy.Rule{r.Match}}, flow); err != nil {
			t.Fatalf("invalid policy rule %#v: %v", r.Match, err)
		}
	}
	for _, tr := range p.Translations {
		if !tr.Match.Enabled {
			t.Fatalf("active translation match is disabled: %#v", tr)
		}
		if err := policy.Validate(policy.Bundle{Scope: scope, Complete: true, DefaultAction: "deny", Rules: []policy.Rule{tr.Match}}, flow); err != nil {
			t.Fatalf("invalid translation match %#v: %v", tr.Match, err)
		}
	}
}

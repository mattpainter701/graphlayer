package firewall

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mattpainter701/graphlayer/platform/policy"
)

func TestPANOSSampleBuildsRoutePolicyAndStaticSourceNAT(t *testing.T) {
	c, err := ParsePANOSConfig(SamplePANOS())
	if err != nil {
		t.Fatal(err)
	}
	if c.Vendor != "paloalto-panos" || c.Context != "vsys1" || c.ParserVersion != PANOSParserVersion {
		t.Fatalf("identity: %#v", c)
	}
	p := c.Path
	if p == nil || p.Version != "panos-path-v1" || !p.RoutingComplete || !p.NATComplete || !p.PolicyComplete || p.InterInterface {
		t.Fatalf("path completeness: %#v", p)
	}
	if len(p.Interfaces) != 2 || len(p.Routes) != 3 || len(p.Translations) != 1 || len(p.Policies) != 1 {
		t.Fatalf("inventory sizes: interfaces=%d routes=%d translations=%d policies=%d", len(p.Interfaces), len(p.Routes), len(p.Translations), len(p.Policies))
	}
	if p.Translations[0].Kind != "snat" || p.Translations[0].OriginalPrefix != "10.1.0.42/32" || p.Translations[0].TranslatedPrefix != "198.51.100.42/32" {
		t.Fatalf("translation: %#v", p.Translations[0])
	}
	r := p.Policies[0]
	if r.From != "trust" || r.To != "untrust" || r.Match.Source != "10.1.0.42/32" || r.Match.Destination != "203.0.113.15/32" || r.Match.Protocol != "tcp" || r.Match.DestinationPort != "443" || !r.Match.Enabled || r.Match.Action != "permit" {
		t.Fatalf("policy: %#v", r)
	}
	if len(c.Chains) != 1 || c.Chains[0].Complete || len(c.Chains[0].Rules) != 1 || len(c.RuleOrigins) != 1 || c.RuleOrigins[0].Line == 0 {
		t.Fatalf("policy chain evidence: %#v %#v", c.Chains, c.RuleOrigins)
	}
	if c.ArtifactSHA256 == "" || len(c.Evidence) == 0 {
		t.Fatal("missing source evidence")
	}
	assertSource := func(needle string) int {
		t.Helper()
		for _, line := range c.SourceLines {
			if strings.Contains(line.Text, needle) {
				return line.Number
			}
		}
		t.Fatalf("sanitized source is missing %q: %#v", needle, c.SourceLines)
		return 0
	}
	assertSource("ethernet1/1 layer3 ip 10.1.0.1/24")
	routeLine := assertSource("static-route backend destination 203.0.113.15/32")
	assertSource("rules allow-https source client-10-1-0-42")
	assertSource("rules allow-https destination backend-203-0-113-15")
	assertSource("rules client-static source-translation static-ip translated-address 198.51.100.42/32")
	foundRouteLine := false
	for _, ref := range p.Routes[2].EvidenceRefs {
		for _, ev := range c.Evidence {
			if ev.ID == ref && ev.StartLine == routeLine {
				foundRouteLine = true
			}
		}
	}
	if !foundRouteLine {
		t.Fatal("static route evidence does not point to its physical SET line")
	}
	assertReadableRefs := func(refs []string) {
		t.Helper()
		for _, ref := range refs {
			found := false
			for _, ev := range c.Evidence {
				if ev.ID == ref {
					found = true
					if ev.StartLine < 1 || ev.StartLine > len(c.SourceLines) || strings.HasPrefix(c.SourceLines[ev.StartLine-1].Text, "[OMITTED") {
						t.Errorf("evidence ref %s points at omitted source line %d", ref, ev.StartLine)
					}
				}
			}
			if !found {
				t.Errorf("evidence ref %s has no evidence record", ref)
			}
		}
	}
	for _, in := range p.Interfaces {
		assertReadableRefs(in.EvidenceRefs)
	}
	for _, route := range p.Routes {
		assertReadableRefs(route.EvidenceRefs)
	}
	for _, policy := range p.Policies {
		assertReadableRefs(policy.EvidenceRefs)
		assertReadableRefs(policy.Match.EvidenceRefs)
	}
	for _, trans := range p.Translations {
		assertReadableRefs(trans.EvidenceRefs)
		assertReadableRefs(trans.Match.EvidenceRefs)
	}
	for _, rule := range append([]policy.Rule{r.Match}, p.Translations[0].Match) {
		bundle := policy.Bundle{Scope: policy.Scope{SnapshotID: "sample", DeviceID: "edge-fw", Context: "vsys1"}, Complete: true, DefaultAction: "deny", Rules: []policy.Rule{rule}}
		flow := policy.Flow{Scope: bundle.Scope, Source: "10.1.0.42", Destination: "203.0.113.15", Protocol: "tcp", SourcePort: 49152, DestinationPort: 443}
		if err := policy.Validate(bundle, flow); err != nil {
			t.Fatalf("normalized policy rule is invalid: %v", err)
		}
	}
}

func TestPANOSDisabledNATRulePreservesDisabledState(t *testing.T) {
	disabled := strings.Join([]string{
		`set rulebase nat rules disabled-first from trust`,
		`set rulebase nat rules disabled-first to untrust`,
		`set rulebase nat rules disabled-first source client-10-1-0-42`,
		`set rulebase nat rules disabled-first destination any`,
		`set rulebase nat rules disabled-first service any`,
		`set rulebase nat rules disabled-first source-translation static-ip translated-address 198.51.100.99`,
		`set rulebase nat rules disabled-first disabled yes`,
	}, "\n") + "\n"
	c, err := ParsePANOSConfig(append([]byte(disabled), SamplePANOS()...))
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Path.Translations) != 2 {
		t.Fatalf("expected disabled and active ordered translations: %#v", c.Path.Translations)
	}
	if c.Path.Translations[0].ID != "panos-nat:disabled-first" || c.Path.Translations[0].Match.Enabled {
		t.Fatalf("disabled NAT rule became active in normalized state: %#v", c.Path.Translations[0])
	}
	if c.Path.Translations[1].ID != "panos-nat:client-static" || !c.Path.Translations[1].Match.Enabled {
		t.Fatalf("later active NAT rule lost its enabled state: %#v", c.Path.Translations[1])
	}
}

func TestPANOSUnsupportedSemanticsAreIncompleteAndSecretsOmitted(t *testing.T) {
	in := strings.Join([]string{
		`set deviceconfig system hostname edge-fw`,
		`set mgt-config users admin password "do-not-retain-this"`,
		`set service app-default protocol tcp port 443`,
		`set rulebase security rules web from trust`,
		`set rulebase security rules web to untrust`,
		`set rulebase security rules web source any`,
		`set rulebase security rules web destination any`,
		`set rulebase security rules web application web-browsing`,
		`set rulebase security rules web service application-default`,
		`set rulebase security rules web action allow`,
		`set rulebase nat rules dynamic from trust`,
		`set rulebase nat rules dynamic to untrust`,
		`set rulebase nat rules dynamic source any`,
		`set rulebase nat rules dynamic destination any`,
		`set rulebase nat rules dynamic service any`,
		`set rulebase nat rules dynamic source-translation dynamic-ip-and-port interface-address interface ethernet1/2`,
		`set network virtual-router vr protocol bgp enable yes`,
	}, "\n")
	c, e := ParsePANOSConfig([]byte(in))
	if e != nil {
		t.Fatal(e)
	}
	if c.Path.PolicyComplete || c.Path.NATComplete || c.Path.RoutingComplete {
		t.Fatalf("unsupported controls reported complete: %#v", c.Path)
	}
	if len(c.Path.Policies) != 0 || len(c.Path.Translations) != 0 {
		t.Fatalf("unsupported App-ID or dynamic NAT produced a modeled rule: %#v %#v", c.Path.Policies, c.Path.Translations)
	}
	encoded, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	joined := string(encoded)
	for _, l := range c.SourceLines {
		joined += " " + l.Text
	}
	for _, e := range c.Evidence {
		joined += " " + e.Summary
	}
	for _, w := range c.Warnings {
		joined += " " + w.Message
	}
	if strings.Contains(joined, "do-not-retain-this") {
		t.Fatal("secret leaked into normalized config")
	}
}

func TestPANOSUnknownZonesVsysAndMissingFieldsCannotProducePermits(t *testing.T) {
	base := string(SamplePANOS())
	for name, input := range map[string]string{
		"unknown zone":    strings.Replace(base, "rules allow-https from trust", "rules allow-https from secret-zone", 1),
		"mixed vsys":      base + "set vsys vsys2 rulebase security rules rogue action allow\n",
		"missing service": strings.Replace(base, "set rulebase security rules allow-https service svc-https\n", "", 1),
		"truncated scope": base + "set rulebase\n",
		"shared scope":    base + "set shared address inherited ip-netmask 10.1.0.0/24\n",
	} {
		t.Run(name, func(t *testing.T) {
			c, err := ParsePANOSConfig([]byte(input))
			if err != nil {
				t.Fatal(err)
			}
			if len(c.Path.Policies) != 0 || c.Path.PolicyComplete {
				t.Fatalf("unsafe policy was modeled as complete: %#v", c.Path)
			}
		})
	}
}

func TestPANOSRejectsUnmodeledInlineNATControlsAndInvalidState(t *testing.T) {
	base := string(SamplePANOS())
	for name, input := range map[string]string{
		"bidirectional NAT": strings.Replace(base, "static-ip translated-address 198.51.100.42", "static-ip translated-address 198.51.100.42 bi-directional yes", 1),
		"invalid NAT state": base + "set rulebase nat rules client-static disabled maybe\n",
	} {
		t.Run(name, func(t *testing.T) {
			c, err := ParsePANOSConfig([]byte(input))
			if err != nil {
				t.Fatal(err)
			}
			if c.Path.NATComplete {
				t.Fatal("unmodeled NAT control retained complete translation evidence")
			}
		})
	}
}

func TestPANOSStaticGroupRetainsEveryMemberAndServiceControlsStayIncomplete(t *testing.T) {
	base := strings.Replace(string(SamplePANOS()), "rules allow-https source client-10-1-0-42", "rules allow-https source clients", 1)
	base += "set address second-client ip-netmask 10.1.0.43/32\nset address-group clients static [ client-10-1-0-42 second-client ]\n"
	c, err := ParsePANOSConfig([]byte(base))
	if err != nil {
		t.Fatal(err)
	}
	if !c.Path.PolicyComplete || len(c.Path.Policies) != 2 || c.Path.Policies[1].Match.Source != "10.1.0.43/32" {
		t.Fatalf("static group dropped a configured member: %#v", c.Path)
	}
	c, err = ParsePANOSConfig([]byte(base + "set service svc-https protocol tcp source-port 12345\n"))
	if err != nil {
		t.Fatal(err)
	}
	if c.Path.PolicyComplete {
		t.Fatal("unmodeled source port widened service matching")
	}
}

func TestPANOSDuplicateNamesExpandWithUniqueIDsAndCRLF(t *testing.T) {
	in := strings.ReplaceAll(string(SamplePANOS()), "\n", "\r\n") + "set rulebase security rules allow-https from trust\r\n"
	c, e := ParsePANOSConfig([]byte(in))
	if e != nil {
		t.Fatal(e)
	}
	if len(c.Path.Policies) != 1 {
		t.Fatalf("repeated property created duplicate rule: %d", len(c.Path.Policies))
	}
	if c.Path.Policies[0].Match.ID == "" {
		t.Fatal("missing stable ID")
	}
}

func TestPANOSRejectsMalformedAndOversizedInput(t *testing.T) {
	if _, e := ParsePANOSConfig([]byte(`set address x ip-netmask "10.0.0.1/32`)); e == nil {
		t.Fatal("expected malformed quote rejection")
	}
	if _, e := ParsePANOSConfig(make([]byte, (2<<20)+1)); e == nil {
		t.Fatal("expected input size rejection")
	}
}

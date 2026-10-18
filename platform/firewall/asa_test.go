package firewall

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mattpainter701/graphlayer/platform/policy"
)

func TestParseASAConfigEvidenceAndRedaction(t *testing.T) {
	c, err := ParseASAConfig(SampleASA())
	if err != nil {
		t.Fatal(err)
	}
	if c.Vendor != "cisco-asa" || c.ParserVersion != ParserVersion || len(c.Chains) != 1 {
		t.Fatalf("unexpected config summary: %#v", c)
	}
	chain := c.Chains[0]
	if chain.ID != "asa:OUTSIDE-IN:outside:in" || !chain.Complete || len(chain.Rules) != 3 {
		t.Fatalf("unexpected chain: %#v", chain)
	}
	if chain.Rules[0].Source != "192.0.2.0/24" || chain.Rules[0].Destination != "10.20.4.15" || chain.Rules[0].DestinationPort != "443" {
		t.Fatalf("object expansion failed: %#v", chain.Rules[0])
	}
	if chain.Rules[1].Action != "deny" || chain.Rules[1].DestinationPort != "22" {
		t.Fatalf("deny rule lost: %#v", chain.Rules[1])
	}
	if !contains(chain.Rules[2].Unsupported, "object_expansion") || chain.Rules[2].Destination != "@UNRESOLVED-APP" {
		t.Fatalf("unresolved object not retained: %#v", chain.Rules[2])
	}
	for _, line := range c.SourceLines {
		if strings.Contains(line.Text, "CANARY") {
			t.Fatalf("secret canary leaked at source line %d: %q", line.Number, line.Text)
		}
	}
	for _, needle := range []string{"PASSWORD_CANARY_VALUE", "COMMUNITY_CANARY_VALUE", "TUNNEL_KEY_CANARY_VALUE"} {
		if bytes.Contains([]byte(strings.Join(sourceTexts(c), "\n")), []byte(needle)) {
			t.Fatalf("secret %s leaked", needle)
		}
	}
	if c.ArtifactSHA256 == "" || len(c.Evidence) != 4 {
		t.Fatalf("artifact/evidence missing: hash %q evidence %d", c.ArtifactSHA256, len(c.Evidence))
	}
	for _, e := range c.Evidence {
		if e.StartLine < 1 || e.EndLine < e.StartLine || e.EndLine > len(c.SourceLines) {
			t.Fatalf("invalid evidence range: %#v", e)
		}
	}
}

func TestSampleFlowsResolveThroughParsedChain(t *testing.T) {
	c, err := ParseASAConfig(SampleASA())
	if err != nil {
		t.Fatal(err)
	}
	chain := c.Chains[0]
	scope := policy.Scope{SnapshotID: "snapshot-1", DeviceID: "edge-fw", Context: c.Context}
	check := func(src, dst, proto string, sp, dp int, want string) {
		t.Helper()
		b := policy.Bundle{Scope: scope, Complete: chain.Complete, DefaultAction: "deny", Rules: chain.Rules}
		f := policy.Flow{Scope: scope, Source: src, Destination: dst, Protocol: proto, SourcePort: sp, DestinationPort: dp}
		got, err := policy.Evaluate(b, f)
		if err != nil {
			t.Fatal(err)
		}
		if got.Disposition != want {
			t.Fatalf("flow %s %s:%d->%s:%d: want %s got %#v", proto, src, sp, dst, dp, want, got)
		}
	}
	check("192.0.2.42", "10.20.4.15", "tcp", 49152, 443, "permit")
	check("192.0.2.42", "10.20.4.15", "tcp", 49152, 22, "deny")
	check("192.0.2.42", "203.0.113.200", "tcp", 49152, 8443, "indeterminate")
}

func TestSensitiveASACommandsNeverReachSnapshot(t *testing.T) {
	input := []byte("hostname scrub\naccess-list WEB extended permit tcp any host 10.0.0.1 eq https\naccess-group WEB in interface outside\nusername admin password 0 \"quoted secret phrase\"\nldap-login-password TWO_WORD_LDAP_CANARY\nsnmp-server community COMMUNITY_CANARY\ncrypto ca trustpoint cert-a\n-----BEGIN CERTIFICATE-----\nCERTIFICATE_BODY_CANARY\n-----END CERTIFICATE-----\n-----BEGIN PRIVATE KEY-----\nPRIVATE_KEY_CANARY\n-----END PRIVATE KEY-----\n")
	c, err := ParseASAConfig(input)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	for _, canary := range []string{"quoted secret phrase", "TWO_WORD_LDAP_CANARY", "COMMUNITY_CANARY", "CERTIFICATE_BODY_CANARY", "PRIVATE_KEY_CANARY"} {
		if bytes.Contains(encoded, []byte(canary)) {
			t.Fatalf("sensitive canary leaked into snapshot JSON: %s", canary)
		}
	}
	for i, line := range c.SourceLines {
		if line.Number != i+1 {
			t.Fatalf("source line numbering changed at %d: %#v", i, line)
		}
	}
}

func TestParseASAConfigKeepsUnsupportedACEAndIncompleteAttachment(t *testing.T) {
	input := []byte("hostname lab\naccess-list A extended permit tcp any host 203.0.113.7 eq 443 time-range WORK\naccess-group A in interface outside\n")
	c, err := ParseASAConfig(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Chains) != 1 || len(c.Chains[0].Rules) != 1 || !contains(c.Chains[0].Rules[0].Unsupported, "schedule") {
		t.Fatalf("schedule rule omitted: %#v", c)
	}
	if !c.Chains[0].Complete {
		t.Fatal("fully retained schedule ACE should keep the ordered chain complete")
	}
}

func TestUnsafeASABindingsRemainIncompleteWithoutPanics(t *testing.T) {
	for _, binding := range []string{"access-group A global", "access-group A in", "access-group A control-plane", "access-group A control-plane interface outside"} {
		input := []byte("hostname lab\naccess-list A extended permit tcp any any eq 443\n" + binding + "\n")
		c, err := ParseASAConfig(input)
		if err != nil {
			t.Fatalf("%q: %v", binding, err)
		}
		if len(c.Chains) != 1 || c.Chains[0].Complete || len(c.Chains[0].Rules) != 1 {
			t.Fatalf("unsafe binding %q was treated as evaluatable: %#v", binding, c)
		}
	}
	input := []byte("hostname lab\naccess-list A\naccess-group A in interface outside\n")
	c, err := ParseASAConfig(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Chains) != 1 || c.Chains[0].Complete || len(c.Chains[0].Rules) != 1 || len(c.Chains[0].Rules[0].Unsupported) == 0 {
		t.Fatalf("short ACL was not retained as unresolved: %#v", c)
	}
}

func TestASALineOrderAndConflictingBindingsFailClosed(t *testing.T) {
	for _, acl := range []string{
		"access-list A line 20 extended deny tcp any any eq 443\naccess-list A extended permit tcp any any eq 443",
		"access-list A line 100 extended deny tcp any any eq 443\naccess-list A line 50 extended permit tcp any any eq 443",
		"access-list A line 10 extended deny tcp any any eq 443\naccess-list A line 10 extended permit tcp any any eq 443",
	} {
		c, err := ParseASAConfig([]byte("hostname lab\n" + acl + "\naccess-group A in interface outside\n"))
		if err != nil {
			t.Fatal(err)
		}
		if len(c.Chains) != 1 || c.Chains[0].Complete {
			t.Fatalf("ambiguous line semantics were treated as complete: %#v", c)
		}
		for _, r := range c.Chains[0].Rules {
			if !contains(r.Unsupported, "vendor_default") {
				t.Fatalf("rule can still yield definitive match: %#v", r)
			}
		}
	}
	input := []byte("hostname lab\naccess-list A extended permit tcp any any eq 443\naccess-list B extended deny tcp any any eq 443\naccess-group A in interface outside\naccess-group B in interface outside\n")
	c, err := ParseASAConfig(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Chains) != 2 || c.Chains[0].Complete || c.Chains[1].Complete {
		t.Fatalf("conflicting bindings were treated as definitive: %#v", c)
	}
}

func TestUnsupportedNetworkObjectMemberCannotPartiallyPermit(t *testing.T) {
	input := []byte("hostname lab\nobject network TARGET\n host 10.0.0.1\n range 10.0.0.10 10.0.0.20\naccess-list A extended deny tcp any object TARGET eq 443\naccess-list A extended permit tcp any any eq 443\naccess-group A in interface outside\n")
	c, err := ParseASAConfig(input)
	if err != nil {
		t.Fatal(err)
	}
	chain := c.Chains[0]
	if !contains(chain.Rules[0].Unsupported, "object_expansion") || chain.Rules[0].Destination != "@TARGET" {
		t.Fatalf("partial object expansion was retained: %#v", chain.Rules[0])
	}
	flow := policy.Flow{Scope: policy.Scope{SnapshotID: "snapshot-1", DeviceID: "edge-fw", Context: c.Context}, Source: "192.0.2.1", Destination: "10.0.0.12", Protocol: "tcp", SourcePort: 49152, DestinationPort: 443}
	result, err := policy.Evaluate(policy.Bundle{Scope: flow.Scope, Complete: chain.Complete, DefaultAction: "deny", Rules: chain.Rules}, flow)
	if err != nil {
		t.Fatal(err)
	}
	if result.Disposition != "indeterminate" {
		t.Fatalf("unsupported range object yielded false definitive verdict: %#v", result)
	}
	for _, e := range c.Evidence {
		if e.Kind == "access_rule" && e.StartLine <= 4 && e.EndLine >= 4 {
			return
		}
	}
	t.Fatal("object member source line was not included in evidence")
}

func TestMalformedASAAddressGrammarDoesNotSkipEarlierDeny(t *testing.T) {
	input := []byte("hostname lab\naccess-list A extended deny ip 10.0.0.0 255.0.255.0 any\naccess-list A extended permit ip any any\naccess-group A in interface outside\n")
	c, err := ParseASAConfig(input)
	if err != nil {
		t.Fatal(err)
	}
	if c.Chains[0].Complete {
		t.Fatal("malformed source mask should make chain incomplete")
	}
	flow := policy.Flow{Scope: policy.Scope{SnapshotID: "snapshot-1", DeviceID: "edge-fw", Context: c.Context}, Source: "10.1.2.3", Destination: "198.51.100.1", Protocol: "tcp", SourcePort: 49152, DestinationPort: 443}
	result, err := policy.Evaluate(policy.Bundle{Scope: flow.Scope, Complete: c.Chains[0].Complete, DefaultAction: "deny", Rules: c.Chains[0].Rules}, flow)
	if err != nil {
		t.Fatal(err)
	}
	if result.Disposition != "indeterminate" {
		t.Fatalf("malformed early deny was skipped: %#v", result)
	}
}

func TestMissingASAHostnameUsesLocalLabel(t *testing.T) {
	c, err := ParseASAConfig([]byte("access-list A extended permit tcp any any eq 443\naccess-group A in interface outside\n"))
	if err != nil {
		t.Fatal(err)
	}
	if c.Hostname != "unnamed-asa" {
		t.Fatalf("unexpected fallback hostname %q", c.Hostname)
	}
	found := false
	for _, w := range c.Warnings {
		if w.Code == "missing_hostname" {
			found = true
		}
	}
	if !found {
		t.Fatal("missing hostname warning absent")
	}
}

func sourceTexts(c *Config) []string {
	out := make([]string, len(c.SourceLines))
	for i, l := range c.SourceLines {
		out[i] = l.Text
	}
	return out
}
func contains(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}

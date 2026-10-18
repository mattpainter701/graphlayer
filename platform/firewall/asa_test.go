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
	if !contains(chain.Rules[2].Unsupported, "object_expansion") || chain.Rules[2].Destination != "@network_object:UNRESOLVED-APP" {
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
	if c.ArtifactSHA256 == "" || len(c.Evidence) < 4 {
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

func TestServiceSampleExpandsNestedTCPUDPAndInventoryEvidence(t *testing.T) {
	c, err := ParseASAConfig(SampleServicesASA())
	if err != nil {
		t.Fatal(err)
	}
	if c.ParserVersion != ParserVersion || len(c.Objects) < 5 {
		t.Fatalf("service inventory missing: version=%s objects=%#v", c.ParserVersion, c.Objects)
	}
	if len(c.Chains) != 1 || len(c.Chains[0].Rules) != 10 {
		t.Fatalf("unexpected service expansion: %d rules %#v objects %#v", len(c.Chains[0].Rules), c.Chains[0].Rules, c.Objects)
	}
	seen := map[string]bool{}
	for _, r := range c.Chains[0].Rules {
		seen[r.Protocol+"/"+r.DestinationPort] = true
	}
	for _, want := range []string{"tcp/53", "udp/53", "tcp/443"} {
		if !seen[want] {
			t.Fatalf("missing service alternative %s in %#v", want, seen)
		}
	}
	foundSourceRange := false
	for _, r := range c.Chains[0].Rules {
		if r.SourcePort == "49152-65535" {
			foundSourceRange = true
		}
	}
	if !foundSourceRange {
		t.Fatal("service object source port range was lost")
	}
	if len(c.RuleOrigins) != len(c.Chains[0].Rules) {
		t.Fatalf("rule origins do not cover expanded rules: %d/%d", len(c.RuleOrigins), len(c.Chains[0].Rules))
	}
	evidence := map[string]bool{}
	for _, e := range c.Evidence {
		if evidence[e.ID] {
			t.Fatalf("duplicate evidence id %s", e.ID)
		}
		evidence[e.ID] = true
	}
	for _, o := range c.Objects {
		for _, ref := range o.EvidenceRefs {
			if !evidence[ref] {
				t.Fatalf("object %s points to missing evidence %s", o.ID, ref)
			}
		}
		for _, m := range o.Members {
			for _, ref := range m.EvidenceRefs {
				if !evidence[ref] {
					t.Fatalf("member %s points to missing evidence %s", o.ID, ref)
				}
			}
		}
	}
	for _, line := range c.SourceLines {
		if strings.Contains(line.Text, "ASA_SERVICE_CANARY") {
			t.Fatal("secret leaked through object inventory evidence")
		}
	}
	scope := policy.Scope{SnapshotID: "service-snapshot", DeviceID: c.Hostname, Context: c.Context}
	bundle := policy.Bundle{Scope: scope, Complete: c.Chains[0].Complete, DefaultAction: "deny", Rules: c.Chains[0].Rules}
	for _, flow := range []struct {
		src, dst, proto string
		sp, dp          int
	}{
		{"192.0.2.42", "10.20.4.15", "tcp", 50000, 443},
		{"198.51.100.8", "203.0.113.9", "udp", 50000, 53},
		{"198.51.100.8", "203.0.113.9", "tcp", 50000, 8443},
	} {
		result, err := policy.Evaluate(bundle, policy.Flow{Scope: scope, Source: flow.src, Destination: flow.dst, Protocol: flow.proto, SourcePort: flow.sp, DestinationPort: flow.dp})
		if err != nil {
			t.Fatal(err)
		}
		if result.Disposition != "permit" {
			t.Fatalf("service flow did not match: %#v result %#v", flow, result)
		}
	}
}

func TestUnsupportedServiceMemberKeepsEarlyDenyUnresolved(t *testing.T) {
	input := []byte("hostname lab\nobject-group service BROKEN tcp\n service-object tcp destination eq https\n service-object icmp\naccess-list A extended deny object-group BROKEN any any\naccess-list A extended permit tcp any any eq 443\naccess-group A in interface outside\n")
	c, err := ParseASAConfig(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Chains) != 1 || len(c.Chains[0].Rules) != 2 || !contains(c.Chains[0].Rules[0].Unsupported, "object_expansion") {
		t.Fatalf("unsupported service alternative was partially expanded: %#v", c)
	}
	flow := policy.Flow{Scope: policy.Scope{SnapshotID: "s", DeviceID: "d", Context: c.Context}, Source: "192.0.2.4", Destination: "198.51.100.4", Protocol: "tcp", SourcePort: 50000, DestinationPort: 443}
	result, err := policy.Evaluate(policy.Bundle{Scope: flow.Scope, Complete: c.Chains[0].Complete, DefaultAction: "deny", Rules: c.Chains[0].Rules}, flow)
	if err != nil {
		t.Fatal(err)
	}
	if result.Disposition != "indeterminate" {
		t.Fatalf("later permit bypassed unresolved earlier service rule: %#v", result)
	}
}

func TestSourceAndDestinationPortGroupsExpandAsCrossProduct(t *testing.T) {
	input := []byte("hostname lab\nobject-group service CLIENT-PORTS tcp\n port-object eq 40000\n port-object eq 40001\nobject-group service SERVER-PORTS tcp\n port-object eq https\n port-object eq 8443\naccess-list A extended permit tcp any object-group CLIENT-PORTS any object-group SERVER-PORTS\naccess-group A in interface outside\n")
	c, err := ParseASAConfig(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Chains) != 1 || len(c.Chains[0].Rules) != 4 {
		t.Fatalf("both port groups were not cross-expanded: %#v", c.Chains)
	}
	for _, origin := range c.RuleOrigins {
		if !contains(origin.ServiceObjectIDs, "service_group:CLIENT-PORTS") || !contains(origin.ServiceObjectIDs, "service_group:SERVER-PORTS") {
			t.Fatalf("rule origin lost one port-group dependency: %#v", origin)
		}
	}
	for _, r := range c.Chains[0].Rules {
		if r.SourcePort == "" || r.DestinationPort == "" {
			t.Fatalf("port group restriction was dropped: %#v", r)
		}
	}
	scope := policy.Scope{SnapshotID: "s", DeviceID: c.Hostname, Context: c.Context}
	bundle := policy.Bundle{Scope: scope, Complete: c.Chains[0].Complete, DefaultAction: "deny", Rules: c.Chains[0].Rules}
	check := func(srcPort, dstPort int, want string) {
		t.Helper()
		result, err := policy.Evaluate(bundle, policy.Flow{Scope: scope, Source: "192.0.2.2", Destination: "198.51.100.3", Protocol: "tcp", SourcePort: srcPort, DestinationPort: dstPort})
		if err != nil {
			t.Fatal(err)
		}
		if result.Disposition != want {
			t.Fatalf("flow %d -> %d: want %s got %#v", srcPort, dstPort, want, result)
		}
	}
	check(40001, 8443, "permit")
	check(80, 443, "deny")
}

func TestTCPACEFiltersTCPUDPPortGroupAndUnknownSiblingBlocksLaterPermit(t *testing.T) {
	input := []byte("hostname lab\nobject-group service BOTH tcp-udp\n port-object eq domain\nobject-group service BROKEN tcp\n port-object eq https\n service-object tcp destination eq 8443\naccess-list A extended permit tcp any any object-group BOTH\naccess-list B extended deny tcp any any object-group BROKEN\naccess-list B extended permit tcp any any eq https\naccess-group A in interface outside\naccess-group B in interface inside\n")
	c, err := ParseASAConfig(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Chains) != 2 || len(c.Chains[0].Rules) != 1 || c.Chains[0].Rules[0].Protocol != "tcp" || c.Chains[0].Rules[0].DestinationPort != "53" {
		t.Fatalf("tcp-udp group was not filtered to the TCP ACE: %#v", c.Chains)
	}
	flow := policy.Flow{Scope: policy.Scope{SnapshotID: "s", DeviceID: c.Hostname, Context: c.Context}, Source: "192.0.2.2", Destination: "198.51.100.3", Protocol: "tcp", SourcePort: 50000, DestinationPort: 53}
	result, err := policy.Evaluate(policy.Bundle{Scope: flow.Scope, Complete: c.Chains[0].Complete, DefaultAction: "deny", Rules: c.Chains[0].Rules}, flow)
	if err != nil || result.Disposition != "permit" {
		t.Fatalf("TCP alternative did not permit: result=%#v err=%v", result, err)
	}
	if len(c.Chains[1].Rules) != 2 || !contains(c.Chains[1].Rules[0].Unsupported, "object_expansion") {
		t.Fatalf("unsupported port-group sibling was partially trusted: %#v", c.Chains[1])
	}
	flow.Scope.Context = c.Context
	result, err = policy.Evaluate(policy.Bundle{Scope: flow.Scope, Complete: c.Chains[1].Complete, DefaultAction: "deny", Rules: c.Chains[1].Rules}, flow)
	if err != nil || result.Disposition != "indeterminate" {
		t.Fatalf("unknown earlier deny did not block later permit: result=%#v err=%v", result, err)
	}
}

func TestServiceProtocolSlotWithExtraPortPredicateIsUnresolved(t *testing.T) {
	input := []byte("hostname lab\nobject-group service WEB\n service-object tcp destination eq https\naccess-list A extended deny object-group WEB any any eq https\naccess-list A extended permit tcp any any eq https\naccess-group A in interface outside\n")
	c, err := ParseASAConfig(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Chains) != 1 || len(c.Chains[0].Rules) != 2 || !contains(c.Chains[0].Rules[0].Unsupported, "object_expansion") {
		t.Fatalf("additional port predicate was silently discarded: %#v", c.Chains)
	}
	flow := policy.Flow{Scope: policy.Scope{SnapshotID: "s", DeviceID: c.Hostname, Context: c.Context}, Source: "192.0.2.2", Destination: "198.51.100.3", Protocol: "tcp", SourcePort: 50000, DestinationPort: 443}
	result, err := policy.Evaluate(policy.Bundle{Scope: flow.Scope, Complete: c.Chains[0].Complete, DefaultAction: "deny", Rules: c.Chains[0].Rules}, flow)
	if err != nil || result.Disposition != "indeterminate" {
		t.Fatalf("constrained protocol-slot ACE yielded a definitive verdict: result=%#v err=%v", result, err)
	}
}

func TestNestedIncompatibleQualifiedServiceGroupIsUnresolved(t *testing.T) {
	input := []byte("hostname lab\nobject-group service UDP-PORTS udp\n port-object eq domain\nobject-group service TCP-PARENT tcp\n group-object UDP-PORTS\naccess-list A extended permit object-group TCP-PARENT any any\naccess-group A in interface outside\n")
	c, err := ParseASAConfig(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Chains) != 1 || len(c.Chains[0].Rules) != 1 || !contains(c.Chains[0].Rules[0].Unsupported, "object_expansion") {
		t.Fatalf("incompatible nested service group was silently accepted: %#v", c.Chains)
	}
	for _, o := range c.Objects {
		if o.ID == "service_group:TCP-PARENT" && o.Resolved {
			t.Fatalf("incompatible nested service group reported resolved: %#v", o)
		}
	}
}

func TestDocumentedServiceOperatorsExpandWithoutInventingPortObjectOperators(t *testing.T) {
	input := []byte("hostname lab\nobject service NOT-HTTPS\n service tcp destination neq https\nobject-group service LOW-PORTS\n service-object tcp destination lt https\nobject-group service INVALID-PORTS tcp\n port-object neq https\naccess-list A extended permit object NOT-HTTPS any any\naccess-list A extended permit object-group LOW-PORTS any any\naccess-list A extended permit tcp any any neq https\naccess-list B extended deny tcp any any object-group INVALID-PORTS\naccess-list B extended permit tcp any any eq http\naccess-group A in interface outside\naccess-group B in interface inside\n")
	c, err := ParseASAConfig(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Chains) != 2 || len(c.Chains[0].Rules) != 5 {
		t.Fatalf("documented service operators were not expanded: %#v", c.Chains)
	}
	for _, object := range c.Objects {
		if object.ID == "service_object:NOT-HTTPS" {
			if !object.Resolved || len(object.Members) != 2 {
				t.Fatalf("neq service object inventory alternatives malformed: %#v", object)
			}
			for _, member := range object.Members {
				if strings.Contains(member.DestinationPort, "|") {
					t.Fatalf("internal alternatives leaked into inventory: %#v", member)
				}
			}
		}
	}
	flow := policy.Flow{Scope: policy.Scope{SnapshotID: "s", DeviceID: c.Hostname, Context: c.Context}, Source: "192.0.2.2", Destination: "198.51.100.3", Protocol: "tcp", SourcePort: 50000, DestinationPort: 80}
	result, err := policy.Evaluate(policy.Bundle{Scope: flow.Scope, Complete: c.Chains[0].Complete, DefaultAction: "deny", Rules: c.Chains[0].Rules}, flow)
	if err != nil || result.Disposition != "permit" {
		t.Fatalf("valid service operators did not permit a matching port: result=%#v err=%v", result, err)
	}
	flow.DestinationPort = 443
	result, err = policy.Evaluate(policy.Bundle{Scope: flow.Scope, Complete: c.Chains[0].Complete, DefaultAction: "deny", Rules: c.Chains[0].Rules}, flow)
	if err != nil || result.Disposition != "deny" {
		t.Fatalf("neq service operators matched excluded port: result=%#v err=%v", result, err)
	}
	if len(c.Chains[1].Rules) != 2 || !contains(c.Chains[1].Rules[0].Unsupported, "object_expansion") {
		t.Fatalf("port-object neq was treated as supported: %#v", c.Chains[1])
	}
	flow.DestinationPort = 80
	result, err = policy.Evaluate(policy.Bundle{Scope: flow.Scope, Complete: c.Chains[1].Complete, DefaultAction: "deny", Rules: c.Chains[1].Rules}, flow)
	if err != nil || result.Disposition != "indeterminate" {
		t.Fatalf("unsupported port-object deny did not block later permit: result=%#v err=%v", result, err)
	}
}

func TestServiceReferenceKindsDoNotRebindSameNamedObject(t *testing.T) {
	input := []byte("hostname lab\nobject-group service SHARED-GROUP\n service-object tcp destination eq https\nobject service SHARED-OBJECT\n service udp destination eq domain\nobject-group service PARENT\n group-object SHARED-GROUP\nobject-group service OBJECT-PARENT\n service-object object SHARED-OBJECT\naccess-list A extended permit object-group PARENT any any\naccess-list A extended permit object-group OBJECT-PARENT any any\naccess-group A in interface outside\n")
	c, err := ParseASAConfig(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Chains) != 1 || len(c.Chains[0].Rules) != 2 || c.Chains[0].Rules[0].Protocol != "tcp" || c.Chains[0].Rules[0].DestinationPort != "443" || c.Chains[0].Rules[1].Protocol != "udp" || c.Chains[0].Rules[1].DestinationPort != "53" {
		t.Fatalf("same-named service namespaces rebound: %#v", c.Chains)
	}
	for _, o := range c.Objects {
		if o.ID == "service_group:PARENT" {
			if !o.Resolved || o.Members[0].ObjectID != "service_group:SHARED-GROUP" {
				t.Fatalf("group-object reference lost intended namespace: %#v", o)
			}
		}
		if o.ID == "service_group:OBJECT-PARENT" {
			if !o.Resolved || o.Members[0].ObjectID != "service_object:SHARED-OBJECT" {
				t.Fatalf("service-object reference lost intended namespace: %#v", o)
			}
		}
	}
}

func TestASACrossKindObjectNameCollisionIsRejected(t *testing.T) {
	if _, err := ParseASAConfig([]byte("object network SAME\n host 10.0.0.1\nobject-group network SAME\n network-object host 10.0.0.2\n")); err == nil {
		t.Fatal("same-name network object and group should be rejected")
	}
	if _, err := ParseASAConfig([]byte("object-group service SAME\n service-object tcp destination eq https\nobject service SAME\n service tcp destination eq https\n")); err == nil {
		t.Fatal("same-name service object and group should be rejected")
	}
}

func TestCyclicServiceGroupsAreMarkedUnresolvedInInventory(t *testing.T) {
	input := []byte("object-group service A\n group-object B\nobject-group service B\n group-object A\n")
	c, err := ParseASAConfig(input)
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range c.Objects {
		if o.Kind == "service_group" && o.Resolved {
			t.Fatalf("cyclic group reported resolved: %#v", o)
		}
	}
}

func TestUnusedMalformedAddressObjectIsExplicitlyUnresolved(t *testing.T) {
	c, err := ParseASAConfig([]byte("object network INVALID\n host not-an-ip\nobject network NAT-SAFE\n host 10.0.0.1\n nat (inside,outside) static 192.0.2.1\n"))
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range c.Objects {
		switch o.ID {
		case "network_object:INVALID":
			if o.Resolved || len(o.Members) != 1 || o.Members[0].Kind != "unsupported" {
				t.Fatalf("invalid address was normalized as resolved: %#v", o)
			}
		case "network_object:NAT-SAFE":
			if !o.Resolved || len(o.Members) != 1 || o.Members[0].Value != "10.0.0.1" {
				t.Fatalf("NAT metadata poisoned valid network object: %#v", o)
			}
		}
	}
}

func TestNetworkInventoryPreservesTypedMissingReferenceNamespace(t *testing.T) {
	input := []byte("hostname lab\nobject network SAME-OBJECT\n host 10.0.0.1\nobject-group network GROUP-PARENT\n group-object SAME-OBJECT\nobject-group network SAME-GROUP\n network-object host 10.0.0.2\nobject network OBJECT-PARENT\n network-object object SAME-GROUP\n")
	c, err := ParseASAConfig(input)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"network_group:GROUP-PARENT":   "network_group:SAME-OBJECT",
		"network_object:OBJECT-PARENT": "network_object:SAME-GROUP",
	}
	for _, object := range c.Objects {
		if objectID, ok := want[object.ID]; ok {
			if object.Resolved || len(object.Members) != 1 || object.Members[0].ObjectID != objectID {
				t.Fatalf("typed missing reference rebound to opposite namespace: %#v; want %s", object, objectID)
			}
			delete(want, object.ID)
		}
	}
	if len(want) != 0 {
		t.Fatalf("expected inventory objects were absent: %#v", want)
	}
}

func TestScopedAndMappedHostsRemainUnsupportedAndImportable(t *testing.T) {
	input := []byte("hostname lab\nobject network SCOPED\n host fe80::1%eth0\nobject-group network MAPPED\n network-object host ::ffff:192.0.2.1\naccess-list A extended deny ip any object-group MAPPED\naccess-list A extended deny ip any object SCOPED\naccess-list A extended permit ip any any\naccess-group A in interface outside\n")
	c, err := ParseASAConfig(input)
	if err != nil {
		t.Fatalf("malformed host literals should import as unsupported members: %v", err)
	}
	for _, id := range []string{"network_object:SCOPED", "network_group:MAPPED"} {
		found := false
		for _, object := range c.Objects {
			if object.ID == id {
				found = true
				if object.Resolved || len(object.Members) != 1 || object.Members[0].Kind != "unsupported" {
					t.Fatalf("invalid host became a resolved address member: %#v", object)
				}
			}
		}
		if !found {
			t.Fatalf("inventory omitted malformed object %s", id)
		}
	}
	if len(c.Chains) != 1 || len(c.Chains[0].Rules) != 3 || !contains(c.Chains[0].Rules[0].Unsupported, "object_expansion") || !contains(c.Chains[0].Rules[1].Unsupported, "object_expansion") {
		t.Fatalf("ACLs using malformed hosts were not kept unresolved: %#v", c.Chains)
	}
	flow := policy.Flow{Scope: policy.Scope{SnapshotID: "s", DeviceID: c.Hostname, Context: c.Context}, Source: "192.0.2.2", Destination: "192.0.2.1", Protocol: "tcp", SourcePort: 50000, DestinationPort: 443}
	result, err := policy.Evaluate(policy.Bundle{Scope: flow.Scope, Complete: c.Chains[0].Complete, DefaultAction: "deny", Rules: c.Chains[0].Rules}, flow)
	if err != nil || result.Disposition != "indeterminate" {
		t.Fatalf("malformed early denies yielded a definitive verdict: result=%#v err=%v", result, err)
	}
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
	if !contains(chain.Rules[0].Unsupported, "object_expansion") || chain.Rules[0].Destination != "@network_object:TARGET" {
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

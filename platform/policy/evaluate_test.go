package policy

import "testing"

func scope() Scope { return Scope{SnapshotID: "snap-1", DeviceID: "fw-1", Context: "default"} }
func flow() Flow {
	return Flow{Scope: scope(), Source: "10.1.2.3", Destination: "10.2.3.4", Protocol: "tcp", SourcePort: 49152, DestinationPort: 443}
}
func rule(id string, pos int, action, src, dst, proto, sp, dp string) Rule {
	return Rule{ID: id, Position: pos, Enabled: true, Action: action, Source: src, Destination: dst, Protocol: proto, SourcePort: sp, DestinationPort: dp, EvidenceRefs: []string{"evidence:" + id}}
}
func base(rs ...Rule) Bundle {
	return Bundle{Scope: scope(), Complete: true, DefaultAction: "deny", Rules: rs}
}

func TestEvaluateAdversarialCases(t *testing.T) {
	tests := []struct {
		name           string
		b              Bundle
		f              Flow
		want, decisive string
		defaultUsed    bool
	}{
		{"first match", base(rule("deny-first", 10, "deny", "any", "any", "tcp", "", "443"), rule("later", 20, "permit", "any", "any", "tcp", "", "443")), flow(), "deny", "deny-first", false},
		{"wrong scope", func() Bundle {
			b := base(rule("allow", 1, "permit", "any", "any", "tcp", "", "443"))
			b.Scope.Context = "other"
			return b
		}(), flow(), "error", "", false},
		{"earlier unresolved blocks later", base(rule("object", 1, "deny", "unresolved-group", "any", "tcp", "", "443"), rule("later", 2, "permit", "any", "any", "tcp", "", "443")), flow(), "indeterminate", "", false},
		{"unresolved protocol with port and ICMP flow", base(rule("service", 1, "deny", "any", "any", "unresolved-service-protocol", "", "443")), func() Flow { f := flow(); f.Protocol = "icmp"; f.SourcePort = 0; f.DestinationPort = 0; return f }(), "indeterminate", "", false},
		{"definite nonmatch eliminates unsupported rule", base(rule("no", 1, "deny", "192.0.2.0/24", "named-destination", "tcp", "", "443"), rule("yes", 2, "permit", "any", "any", "tcp", "", "443")), flow(), "permit", "yes", false},
		{"IPv6 literal and CIDR", base(rule("v6", 1, "permit", "2001:db8::/32", "2001:db8::8", "6", "49152-49152", "443")), func() Flow { f := flow(); f.Source = "2001:db8::1"; f.Destination = "2001:db8::8"; return f }(), "permit", "v6", false},
		{"default on complete bundle", base(), flow(), "deny", "", true},
		{"no default on incomplete bundle", func() Bundle { b := base(); b.Complete = false; return b }(), flow(), "indeterminate", "", false},
		{"incomplete bundle candidate remains unknown", func() Bundle {
			b := base(rule("maybe", 1, "permit", "any", "any", "tcp", "", "443"))
			b.Complete = false
			return b
		}(), flow(), "indeterminate", "", false},
		{"disabled rule skipped", base(func() Rule { r := rule("off", 1, "deny", "any", "any", "tcp", "", "443"); r.Enabled = false; return r }(), rule("on", 2, "permit", "any", "any", "tcp", "", "443")), flow(), "permit", "on", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Evaluate(tt.b, tt.f)
			if tt.want == "error" {
				if err == nil {
					t.Fatal("expected validation error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.Disposition != tt.want || got.DecisiveRule != tt.decisive || got.UsedDefault != tt.defaultUsed {
				t.Fatalf("got %+v", got)
			}
			if tt.name == "incomplete bundle candidate remains unknown" && got.CandidateRule != "maybe" {
				t.Fatalf("candidate trace missing: %+v", got)
			}
			if tt.name == "earlier unresolved blocks later" && len(got.EvidenceRefs) != 1 {
				t.Fatalf("unresolved rule evidence missing: %+v", got)
			}
		})
	}
}

func TestValidationAndDeterminism(t *testing.T) {
	t.Run("duplicate ids", func(t *testing.T) {
		b := base(rule("x", 1, "permit", "any", "any", "tcp", "", "443"), rule("x", 2, "deny", "any", "any", "tcp", "", "443"))
		if _, e := Evaluate(b, flow()); e == nil {
			t.Fatal("expected duplicate id error")
		}
	})
	t.Run("invalid port range", func(t *testing.T) {
		b := base(rule("x", 1, "permit", "any", "any", "tcp", "", "70000"))
		if _, e := Evaluate(b, flow()); e == nil {
			t.Fatal("expected port error")
		}
	})
	t.Run("invalid flow protocol", func(t *testing.T) {
		f := flow()
		f.Protocol = "256"
		if _, e := Evaluate(base(), f); e == nil {
			t.Fatal("expected protocol error")
		}
	})
	t.Run("tcp concrete ports", func(t *testing.T) {
		f := flow()
		f.DestinationPort = 0
		if _, e := Evaluate(base(), f); e == nil {
			t.Fatal("expected missing concrete port error")
		}
	})
	t.Run("ports on non-TCP rules", func(t *testing.T) {
		b := base(rule("icmp-port", 1, "deny", "any", "any", "icmp", "", "80"))
		if _, e := Evaluate(b, flow()); e == nil {
			t.Fatal("expected invalid port protocol combination")
		}
	})
	t.Run("ICMP family mismatch", func(t *testing.T) {
		f := flow()
		f.Source = "2001:db8::1"
		f.Destination = "2001:db8::8"
		f.Protocol = "icmp"
		f.SourcePort = 0
		f.DestinationPort = 0
		if _, e := Evaluate(base(), f); e == nil {
			t.Fatal("expected ICMP family error")
		}
	})
	t.Run("IPv4 mapped flow rejected", func(t *testing.T) {
		f := flow()
		f.Source = "::ffff:10.1.2.3"
		f.Destination = "::ffff:10.2.3.4"
		if _, e := Evaluate(base(), f); e == nil {
			t.Fatal("expected mapped IPv4 rejection")
		}
	})
	t.Run("IPv4 mapped rule predicate rejected", func(t *testing.T) {
		b := base(rule("mapped", 1, "permit", "::ffff:10.1.2.0/120", "any", "tcp", "", "443"))
		if _, e := Evaluate(b, flow()); e == nil {
			t.Fatal("expected mapped rule address rejection")
		}
	})
	t.Run("zone-scoped flow rejected", func(t *testing.T) {
		f := flow()
		f.Source = "fe80::1%eth0"
		f.Destination = "fe80::2%eth0"
		f.Protocol = "udp"
		f.SourcePort = 1234
		f.DestinationPort = 53
		if _, e := Evaluate(base(), f); e == nil {
			t.Fatal("expected zone-scoped address rejection")
		}
	})
	t.Run("duplicate positions", func(t *testing.T) {
		b := base(rule("a", 1, "permit", "any", "any", "tcp", "", "443"), rule("b", 1, "deny", "any", "any", "tcp", "", "443"))
		if _, e := Evaluate(b, flow()); e == nil {
			t.Fatal("expected duplicate position error")
		}
	})
	t.Run("invalid flow address", func(t *testing.T) {
		f := flow()
		f.Source = "not-an-ip"
		if _, e := Evaluate(base(), f); e == nil {
			t.Fatal("expected invalid flow address error")
		}
	})
	t.Run("deterministic and input preserving", func(t *testing.T) {
		b := base(rule("last", 20, "permit", "any", "any", "tcp", "", "443"), rule("first", 10, "deny", "any", "any", "udp", "", "53"))
		before := b.Rules[0].ID
		a, e := Evaluate(b, flow())
		if e != nil {
			t.Fatal(e)
		}
		c, e := Evaluate(b, flow())
		if e != nil {
			t.Fatal(e)
		}
		if a.Disposition != c.Disposition || a.DecisiveRule != c.DecisiveRule || b.Rules[0].ID != before {
			t.Fatal("evaluation was nondeterministic or mutated input")
		}
	})
}

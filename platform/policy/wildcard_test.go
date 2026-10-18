package policy

import "testing"

func TestASAProtocolAndFamilyWildcards(t *testing.T) {
	for _, proto := range []string{"any", "ip"} {
		b := base(rule("all-ip", 1, "permit", "any4", "any4", proto, "", ""))
		f := flow()
		got, err := Evaluate(b, f)
		if err != nil || got.Disposition != "permit" || got.EngineContractVersion != ContractVersion {
			t.Fatalf("wildcard TCP: %+v %v", got, err)
		}
		f.Protocol = "icmp"
		f.SourcePort = 0
		f.DestinationPort = 0
		got, err = Evaluate(b, f)
		if err != nil || got.Disposition != "permit" {
			t.Fatalf("wildcard ICMP: %+v %v", got, err)
		}
		f.Source = "2001:db8::1"
		f.Destination = "2001:db8::2"
		f.Protocol = "icmpv6"
		got, err = Evaluate(b, f)
		if err != nil || got.Disposition != "deny" || !got.UsedDefault {
			t.Fatalf("any4 must not include IPv6: %+v %v", got, err)
		}
		b.Rules[0].Source = "any6"
		b.Rules[0].Destination = "any6"
		got, err = Evaluate(b, f)
		if err != nil || got.Disposition != "permit" {
			t.Fatalf("any6: %+v %v", got, err)
		}
		b.Rules[0].DestinationPort = "443"
		if _, err = Evaluate(b, flow()); err == nil {
			t.Fatal("wildcard protocol with ports must be rejected")
		}
	}
	f := flow()
	f.Protocol = "any"
	if _, err := Evaluate(base(), f); err == nil {
		t.Fatal("a concrete flow cannot have a wildcard protocol")
	}
}

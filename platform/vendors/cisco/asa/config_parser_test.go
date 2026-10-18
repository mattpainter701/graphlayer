package cisco

import (
	"strings"
	"testing"
)

func TestParseRunningConfigKeepsOrderedAttachedACEAndObjects(t *testing.T) {
	src := `hostname asa-test
object network APP
 host 10.0.0.8
object-group network CLIENTS
 network-object 192.0.2.0 255.255.255.0
access-list IN extended permit tcp object-group CLIENTS object APP eq https
access-list IN extended deny tcp any host 10.0.0.8 eq ssh inactive
access-group IN in interface outside
`
	c, err := ParseRunningConfig([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if c.Hostname != "asa-test" || len(c.ACLs["IN"]) != 2 || len(c.Bindings) != 1 {
		t.Fatalf("unexpected parse: %#v", c)
	}
	first := c.ACLs["IN"][0]
	if first.Order != 1 || first.Line != 6 || first.Source != "@network_group:CLIENTS" || first.Destination != "@network_object:APP" || first.DestinationPort != "443" {
		t.Fatalf("unexpected first ACE: %#v", first)
	}
	addresses, lines, err := c.ExpandAddress(first.Source)
	if err != nil || len(addresses) != 1 || addresses[0] != "192.0.2.0/24" || len(lines) != 2 {
		t.Fatalf("unexpected source expansion %v %v %v", addresses, lines, err)
	}
	if !c.ACLs["IN"][1].Disabled || c.ACLs["IN"][1].Order != 2 {
		t.Fatalf("inactive/order lost: %#v", c.ACLs["IN"][1])
	}
}

func TestServiceProtocolSlotParsesWithoutShiftingAddresses(t *testing.T) {
	c, err := ParseRunningConfig([]byte("access-list A extended permit object-group WEB 192.0.2.0 255.255.255.0 object-group HOSTS\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(c.ACLs["A"]) != 1 {
		t.Fatalf("ACE missing: %#v", c.ACLs)
	}
	a := c.ACLs["A"][0]
	if a.ServiceGroup != "WEB" || a.Source != "192.0.2.0/24" || a.Destination != "@network_group:HOSTS" || len(a.Unsupported) > 0 {
		t.Fatalf("service protocol slot shifted address predicates: %#v", a)
	}
}

func TestASAServiceAliasesAreProtocolAware(t *testing.T) {
	for _, tc := range []struct {
		protocol, name, want string
		ok                   bool
	}{{"udp", "radius", "1645", true}, {"udp", "radius-acct", "1646", true}, {"tcp", "radius", "", false}, {"udp", "dns", "195", true}, {"tcp", "dns", "", false}, {"tcp", "kerberos", "750", true}, {"tcp-udp", "www", "80", true}, {"tcp-udp", "dns", "", false}} {
		got, ok := servicePortFor(tc.protocol, tc.name)
		if ok != tc.ok || got != tc.want {
			t.Fatalf("%s %s: got %q,%t want %q,%t", tc.protocol, tc.name, got, ok, tc.want, tc.ok)
		}
	}
}

func TestServiceComparisonOperatorsExpandExactlyAndPortObjectIsBounded(t *testing.T) {
	for _, tc := range []struct {
		expr string
		want []string
	}{{"lt 5", []string{"1-4"}}, {"gt 65533", []string{"65534-65535"}}, {"neq 443", []string{"1-442", "444-65535"}}} {
		got, _, ok := parsePortAlternativesFor(strings.Fields(tc.expr), "tcp")
		if !ok || strings.Join(got, ",") != strings.Join(tc.want, ",") {
			t.Fatalf("%s: got %v,%t want %v", tc.expr, got, ok, tc.want)
		}
		if _, _, ok := parsePortObjectAlternatives(strings.Fields(tc.expr), "tcp"); ok {
			t.Fatalf("port-object unexpectedly accepted %s", tc.expr)
		}
	}
	for _, expr := range []string{"eq 443", "range 40000 40100"} {
		if _, _, ok := parsePortObjectAlternatives(strings.Fields(expr), "tcp"); !ok {
			t.Fatalf("port-object rejected supported %s", expr)
		}
	}
}

func TestServiceObjectAndServiceGroupTcpUdpGrammar(t *testing.T) {
	src := `object service INVALID-OBJECT
 service tcp-udp destination eq domain
object-group service GENERIC
 service-object tcp-udp destination eq domain
object-group service INVALID-PORT tcp
 port-object neq 443
`
	c, err := ParseRunningConfig([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if c.ServiceObjects["INVALID-OBJECT"].Protocol != "" || len(c.ServiceObjects["INVALID-OBJECT"].UnsupportedLines) != 1 {
		t.Fatalf("object service accepted tcp-udp: %#v", c.ServiceObjects["INVALID-OBJECT"])
	}
	generic := c.ServiceGroups["GENERIC"]
	if len(generic.Members) != 1 || generic.Members[0].Kind != "service" || generic.Members[0].Protocol != "tcp-udp" {
		t.Fatalf("generic service group rejected service-object tcp-udp: %#v", generic)
	}
	invalidPort := c.ServiceGroups["INVALID-PORT"]
	if len(invalidPort.UnsupportedLines) != 1 || len(invalidPort.Members) != 1 || invalidPort.Members[0].Kind != "unsupported" {
		t.Fatalf("port-object neq was not retained as unsupported: %#v", invalidPort)
	}
}

func TestExpandAddressRejectsCycle(t *testing.T) {
	src := `object-group network A
 group-object B
object-group network B
 group-object A
`
	c, err := ParseRunningConfig([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := c.ExpandAddress("@A"); err == nil {
		t.Fatal("expected cycle error")
	}
}

func TestParseRunningConfigRejectsBounds(t *testing.T) {
	if _, err := ParseRunningConfig([]byte("hostname " + string(make([]byte, MaxConfigLineBytes+1)))); err == nil {
		t.Fatal("expected line limit")
	}
}

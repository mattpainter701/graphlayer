package cisco

import "testing"

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
	if first.Order != 1 || first.Line != 6 || first.Source != "@CLIENTS" || first.Destination != "@APP" || first.DestinationPort != "443" {
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

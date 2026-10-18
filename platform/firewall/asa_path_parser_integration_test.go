package firewall_test

import (
	"strings"
	"testing"

	"github.com/mattpainter701/graphlayer/platform/firewall"
	"github.com/mattpainter701/graphlayer/platform/pathanalysis"
	"github.com/mattpainter701/graphlayer/platform/policy"
)

func TestStandbyPeerAddressCannotBeModeledAsForwardedTransit(t *testing.T) {
	input := strings.Replace(string(firewall.SamplePathASA()), "ip address 192.0.2.1 255.255.255.0", "ip address 192.0.2.1 255.255.255.0 standby 192.0.2.2", 1)
	c, err := firewall.ParseASAConfig([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	scope := policy.Scope{SnapshotID: "standby-regression", DeviceID: c.Hostname, Context: c.Context}
	flow := policy.Flow{Scope: scope, Source: "10.0.0.20", Destination: "192.0.2.2", Protocol: "tcp", SourcePort: 52000, DestinationPort: 443}
	result, err := pathanalysis.Evaluate(pathanalysis.Input{Scope: scope, Config: *c, IngressInterface: "inside", Flow: flow})
	if err != nil {
		t.Fatal(err)
	}
	if result.Disposition == "modeled_forward" {
		t.Fatalf("standby peer address was modeled as transit: %#v", result)
	}
}

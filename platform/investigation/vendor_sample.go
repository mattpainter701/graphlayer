package investigation

import (
	"github.com/mattpainter701/graphlayer/platform/firewall"
	"github.com/mattpainter701/graphlayer/platform/networkpath"
	"strings"
)

func vendorNetworkSample() NetworkSample {
	forti := strings.ReplaceAll(string(firewall.SampleFortiGate()), "172.16.0.2", "172.16.0.6")
	forti = strings.ReplaceAll(forti, "172.16.0.1", "172.16.0.5")
	return NetworkSample{Name: "PAN-OS → ASA → FortiGate static NAT and return path", Configs: []NetworkSampleConfig{
		{NodeID: "edge", Name: "edge.panos.set.txt", Vendor: "paloalto-panos", Config: string(firewall.SamplePANOS())},
		{NodeID: "transit", Name: "transit.asa.txt", Vendor: "cisco-asa", Config: `hostname enterprise-transit
interface GigabitEthernet0/0
 nameif left
 security-level 100
 ip address 172.16.0.2 255.255.255.252
 no shutdown
!
interface GigabitEthernet0/1
 nameif right
 security-level 0
 ip address 172.16.0.5 255.255.255.252
 no shutdown
!
route right 203.0.113.15 255.255.255.255 172.16.0.6
route left 198.51.100.42 255.255.255.255 172.16.0.1
access-list LEFT-IN extended permit tcp host 198.51.100.42 host 203.0.113.15 eq 443
access-list LEFT-IN extended deny ip any any
access-group LEFT-IN in interface left
access-list RIGHT-IN extended deny ip any any
access-group RIGHT-IN in interface right
`},
		{NodeID: "core", Name: "core.fortios.conf", Vendor: "fortinet-fortigate", Config: forti},
	}, Links: []networkpath.Link{
		{A: networkpath.Endpoint{NodeID: "edge", Interface: "ethernet1/2"}, B: networkpath.Endpoint{NodeID: "transit", Interface: "left"}},
		{A: networkpath.Endpoint{NodeID: "transit", Interface: "right"}, B: networkpath.Endpoint{NodeID: "core", Interface: "port1"}},
	}, Source: networkpath.Endpoint{NodeID: "edge", Interface: "ethernet1/1"}, Destination: networkpath.Endpoint{NodeID: "core", Interface: "port2"}, Flow: NetworkFlow{Source: "10.1.0.42", Destination: "203.0.113.15", Protocol: "tcp", SourcePort: 52000, DestinationPort: 443}}
}

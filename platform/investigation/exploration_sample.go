package investigation

import (
	"strings"

	"github.com/mattpainter701/graphlayer/platform/firewall"
	"github.com/mattpainter701/graphlayer/platform/networkpath"
)

// explorationNetworkSample deliberately includes a return route through south
// for both equal-best forward choices. The north branch cannot establish state
// admission on south, even though its configured reverse policy permits.
func explorationNetworkSample() NetworkSample {
	edge := strings.ReplaceAll(string(firewall.SamplePANOS()), "edge-fw", "branch-edge")
	edge = strings.ReplaceAll(edge, "172.16.0.1/30", "172.16.0.1/29")
	edge = strings.ReplaceAll(edge, "static-route backend", "static-route via-north")
	edge += "set network virtual-router default routing-table ip static-route via-south destination 203.0.113.15/32\n" +
		"set network virtual-router default routing-table ip static-route via-south interface ethernet1/2\n" +
		"set network virtual-router default routing-table ip static-route via-south nexthop ip-address 172.16.0.3\n"
	core := strings.ReplaceAll(string(firewall.SampleFortiGate()), "172.16.0.2", "172.16.1.1")
	core = strings.ReplaceAll(core, "255.255.255.252", "255.255.255.248")
	core = strings.ReplaceAll(core, "172.16.0.1", "172.16.1.3")
	core += "config system global\n set hostname branch-core\nend\n"
	return NetworkSample{
		Name: "Equal-best route alternatives and an asymmetric configured return",
		Configs: []NetworkSampleConfig{
			{NodeID: "edge", Name: "branch-edge.panos.set.txt", Vendor: "paloalto-panos", Config: edge},
			{NodeID: "north", Name: "branch-north.asa.txt", Vendor: "cisco-asa", Config: `hostname branch-north
interface GigabitEthernet0/0
 nameif left
 security-level 100
 ip address 172.16.0.2 255.255.255.248
 no shutdown
!
interface GigabitEthernet0/1
 nameif right
 security-level 0
 ip address 172.16.1.2 255.255.255.248
 no shutdown
!
route right 203.0.113.15 255.255.255.255 172.16.1.1
route left 198.51.100.42 255.255.255.255 172.16.0.1
access-list LEFT-IN extended permit tcp host 198.51.100.42 host 203.0.113.15 eq 443
access-list LEFT-IN extended deny ip any any
access-group LEFT-IN in interface left
access-list RIGHT-IN extended deny ip any any
access-group RIGHT-IN in interface right
`},
			{NodeID: "south", Name: "branch-south.fortios.conf", Vendor: "fortinet-fortigate", Config: `config system global
 set hostname branch-south
end
config system interface
 edit "port1"
  set vdom "root"
  set ip 172.16.0.3 255.255.255.248
  set status up
 next
 edit "port2"
  set vdom "root"
  set ip 172.16.1.3 255.255.255.248
  set status up
 next
end
config router static
 edit 1
  set dst 203.0.113.15 255.255.255.255
  set gateway 172.16.1.1
  set device "port2"
 next
 edit 2
  set dst 198.51.100.42 255.255.255.255
  set gateway 172.16.0.1
  set device "port1"
 next
end
config firewall address
 edit "REMOTE"
  set subnet 198.51.100.42 255.255.255.255
 next
 edit "PUBLIC"
  set subnet 203.0.113.15 255.255.255.255
 next
end
config firewall service custom
 edit "HTTPS"
  set protocol TCP/UDP/SCTP
  set tcp-portrange 443
 next
end
config firewall policy
 edit 10
  set srcintf "port1"
  set dstintf "port2"
  set srcaddr "REMOTE"
  set dstaddr "PUBLIC"
  set service "HTTPS"
  set schedule "always"
  set action accept
 next
 edit 20
  set srcintf "port2"
  set dstintf "port1"
  set srcaddr "PUBLIC"
  set dstaddr "REMOTE"
  set service "ALL"
  set schedule "always"
  set action accept
 next
end
`},
			{NodeID: "core", Name: "branch-core.fortios.conf", Vendor: "fortinet-fortigate", Config: core},
		},
		Links: []networkpath.Link{
			{A: networkpath.Endpoint{NodeID: "edge", Interface: "ethernet1/2"}, B: networkpath.Endpoint{NodeID: "north", Interface: "left"}},
			{A: networkpath.Endpoint{NodeID: "edge", Interface: "ethernet1/2"}, B: networkpath.Endpoint{NodeID: "south", Interface: "port1"}},
			{A: networkpath.Endpoint{NodeID: "north", Interface: "right"}, B: networkpath.Endpoint{NodeID: "core", Interface: "port1"}},
			{A: networkpath.Endpoint{NodeID: "south", Interface: "port2"}, B: networkpath.Endpoint{NodeID: "core", Interface: "port1"}},
		},
		Source:      networkpath.Endpoint{NodeID: "edge", Interface: "ethernet1/1"},
		Destination: networkpath.Endpoint{NodeID: "core", Interface: "port2"},
		Flow:        NetworkFlow{Source: "10.1.0.42", Destination: "203.0.113.15", Protocol: "tcp", SourcePort: 52000, DestinationPort: 443},
	}
}

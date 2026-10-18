package investigation

import (
	"github.com/mattpainter701/graphlayer/platform/networkpath"
)

type NetworkSampleConfig struct {
	NodeID string `json:"node_id"`
	Name   string `json:"name"`
	Config string `json:"config"`
	Vendor string `json:"vendor,omitempty"`
}

type NetworkSample struct {
	Name        string                `json:"name"`
	Configs     []NetworkSampleConfig `json:"configs"`
	Links       []networkpath.Link    `json:"links"`
	Source      networkpath.Endpoint  `json:"source"`
	Destination networkpath.Endpoint  `json:"destination"`
	Flow        NetworkFlow           `json:"flow"`
}

func networkSample() NetworkSample {
	return NetworkSample{
		Name: "Two-firewall static NAT and return path",
		Configs: []NetworkSampleConfig{
			{NodeID: "edge", Name: "edge.asa.txt", Config: `hostname enterprise-edge
interface GigabitEthernet0/0
 nameif inside
 security-level 100
 ip address 10.1.0.1 255.255.255.0
 no shutdown
!
interface GigabitEthernet0/1
 nameif transit
 security-level 0
 ip address 172.16.0.1 255.255.255.252
 no shutdown
!
route transit 203.0.113.15 255.255.255.255 172.16.0.2
object network CLIENT
 host 10.1.0.42
 nat (inside,transit) static 198.51.100.42
!
access-list INSIDE-IN extended permit tcp host 10.1.0.42 host 203.0.113.15 eq 443
access-list INSIDE-IN extended deny ip any any
access-group INSIDE-IN in interface inside
access-list TRANSIT-IN extended deny ip any any
access-group TRANSIT-IN in interface transit
`},
			{NodeID: "core", Name: "core.asa.txt", Config: `hostname enterprise-core
interface GigabitEthernet0/0
 nameif transit
 security-level 0
 ip address 172.16.0.2 255.255.255.252
 no shutdown
!
interface GigabitEthernet0/1
 nameif inside
 security-level 100
 ip address 10.2.0.1 255.255.255.0
 no shutdown
!
route transit 198.51.100.42 255.255.255.255 172.16.0.1
object network WEB
 host 10.2.0.15
 nat (inside,transit) static 203.0.113.15
!
access-list TRANSIT-IN extended permit tcp host 198.51.100.42 host 10.2.0.15 eq 443
access-list TRANSIT-IN extended deny ip any any
access-group TRANSIT-IN in interface transit
access-list INSIDE-IN extended deny ip any any
access-group INSIDE-IN in interface inside
`},
		},
		Links:       []networkpath.Link{{A: networkpath.Endpoint{NodeID: "edge", Interface: "transit"}, B: networkpath.Endpoint{NodeID: "core", Interface: "transit"}}},
		Source:      networkpath.Endpoint{NodeID: "edge", Interface: "inside"},
		Destination: networkpath.Endpoint{NodeID: "core", Interface: "inside"},
		Flow:        NetworkFlow{Source: "10.1.0.42", Destination: "203.0.113.15", Protocol: "tcp", SourcePort: 52000, DestinationPort: 443},
	}
}

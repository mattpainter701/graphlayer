# Arista EOS — Vendor Guide

## Supported Platforms
- DCS-7050SX/SDX series
- DCS-7060CX series
- DCS-7150 series
- DCS-7250 series
- DCS-7280 series
- DCS-7300 series
- CCS-7000 series

## Required Command Privileges
All commands require **privilege level 15** or equivalent read-only access.

## Command Requirements

### Configuration Collection
```
show running-config
```
Alternative: `show running-config all` (includes defaults).

### Operational Data Collection

| Purpose | Command | Priority |
|---------|---------|----------|
| LLDP neighbors | `show lldp neighbors detail` | Required |
| MAC address table | `show mac address-table dynamic` | Required |
| ARP cache | `show ip arp` | Required |
| Routing table | `show ip route` | Required |
| Interface status | `show interfaces status` | Required |
| MLAG status | `show mlag` | Conditional (if MLAG configured) |
| Port security | `show port-security` | Optional |
| OSPF neighbors | `show ip ospf neighbor` | Conditional (if OSPF configured) |
| BGP summary | `show ip bgp summary` | Conditional (if BGP configured) |
| Spanning tree | `show spanning-tree` | Optional |

## EOS Config Structure

EOS uses a hierarchical indentation-based configuration format similar to Cisco IOS but with key differences:

### Top-level Commands
```
hostname <name>
!
interface Ethernet<id>
   description <text>
   <sub-commands indented by 3 spaces>
!
vlan <id>
   name <text>
!
mlag configuration
   domain-id <name>
   local-interface <interface>
   peer-address <ip>
   peer-link <port-channel>
!
router ospf <process-id>
   router-id <ip>
   network <net> <wildcard> area <area>
!
router bgp <asn>
   router-id <ip>
   neighbor <ip> remote-as <asn>
!
ip access-list <name>
   <seq> <action> <protocol> <src> <wildcard> <dest> <wildcard>
!
vrf instance <name>
   rd <rd-value>
   route-target import <rt-value>
   route-target export <rt-value>
!
interface Vxlan1
   vxlan source-interface <loopback>
   vxlan vlan <id> vni <id>
   vxlan flood vtep <ip> [<ip>...]
```

### Key Differences from Cisco IOS

1. **CIDR notation preferred**: `ip route 10.0.0.0/8 192.168.1.1` vs Cisco's `ip route 10.0.0.0 255.0.0.0 192.168.1.1`
2. **Named ACLs only**: No numbered ACLs in modern EOS
3. **MLAG instead of VPC**: Similar concept, different configuration
4. **VXLAN under Vxlan1**: VXLAN is always configured under `interface Vxlan1`
5. **Port-Channel naming**: `Port-Channel1` (not `port-channel 1`)
6. **VLAN interface naming**: `interface Vlan10` (not `interface Vlan10`)

## Topology Inference

### MLAG Topology
MLAG (Multi-chassis Link Aggregation) provides active-active L2/L3 forwarding between two Arista switches. The parser:

1. Extracts MLAG domain configuration: domain-id, peer-link, peer-address
2. Identifies peer-link interfaces from Port-Channel membership
3. Maps individual interfaces to their MLAG port-channels
4. Infers L2 links between the paired MLAG switches

### L2 Adjacency
L2 adjacency is inferred from:
- **LLDP**: Primary source for neighbor device/port mapping
- **MAC address table**: MAC-to-port mapping for endpoint placement
- **Interface status**: Connected/notconnected for link state

## Data Model Mapping

### Device Types
| EOS Role | GraphLayer Type |
|----------|----------------|
| Switch / Leaf | `switch` |
| Spine | `router` |
| Border leaf | `firewall` (if routing + ACLs) |

### Interface Types
| EOS Interface | GraphLayer Type |
|---------------|----------------|
| Ethernet X | `physical` |
| Vlan X | `vlan` (SVI) |
| Port-Channel X | `aggregated` |
| Loopback X | `loopback` |
| Management X | `management` |
| Vxlan X | `tunnel` |

## Troubleshooting

### Common Issues

1. **Empty hostname**: Verify `hostname` is present in the config
2. **Missing interfaces**: Check for unexpected whitespace in interface stanzas
3. **Empty routing table**: Verify operational command outputs cover all protocol routes
4. **No LLDP neighbors**: Check that `lldp` is globally enabled on the device
5. **Incomplete MLAG data**: Ensure `show mlag` output is complete with peer-link status

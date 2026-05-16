# Arista EOS Vendor Parser

## Overview
Arista EOS (Extensible Operating System) parser for the GraphLayer platform.
Provides comprehensive config and operational data parsing for Arista switches,
including all major features: interfaces, VLANs, MLAG, VXLAN, routing (static,
OSPF, BGP), ACLs, and VRF.

## Supported EOS Versions
- EOS 4.27.x and later
- Backward compatible to EOS 4.22.x

## Supported Features

### Config Parsers
| Feature | Status | Notes |
|---------|--------|-------|
| Interfaces (physical, SVI, loopback, management) | ✓ |
| Switchport config (access/trunk/routed) | ✓ |
| VLAN definitions | ✓ |
| Port-Channels (LACP) | ✓ |
| MLAG (Multi-chassis Link Aggregation) | ✓ | domain-id, peer-link, keepalive |
| VXLAN | ✓ | VNI-to-VLAN mapping, flood lists |
| Static routes | ✓ | with distance, tag, name |
| OSPF (multi-area) | ✓ | network statements, passive interfaces |
| BGP (IPv4, VRF-aware) | ✓ | neighbors, networks, maximum-paths |
| Extended ACLs (named) | ✓ |
| VRF instances | ✓ | RD, RT import/export |

### Operational Parsers
| Command | Status | Output Type |
|---------|--------|-------------|
| show lldp neighbors detail | ✓ | LLDP neighbor table |
| show mac address-table | ✓ | MAC forwarding table |
| show ip arp | ✓ | ARP cache |
| show ip route | ✓ | Routing table |
| show interfaces status | ✓ | Interface status |
| show mlag | ✓ | MLAG operational state |
| show port-security | ✓ | Port security status |
| show ip ospf neighbor | ✓ | OSPF neighbors |
| show ip bgp summary | ✓ | BGP peer summary |

### L2 Security
| Feature | Status | Notes |
|---------|--------|-------|
| Port security | ✓ | Max MAC, violation action |
| Storm control | ✓ | Broadcast level |
| Spanning-tree portfast | ✓ | Edge port detection |

## Key EOS Syntax Differences from Cisco IOS

1. **VLAN interface naming**: `interface Vlan10` (not `interface vlan10`)
2. **Port-Channel naming**: `interface Port-Channel1` (capitalized, hyphenated)
3. **MLAG configuration**: Hierarchical `mlag configuration` block
4. **VXLAN**: Configured under `interface Vxlan1` with `vxlan vlan <id> vni <id>`
5. **ACLs**: `ip access-list <name>` (named ACLs only, no numbered ACLs in modern EOS)
6. **VRF**: `vrf instance <name>` blocks with RD/RT
7. **Route syntax**: `ip route <prefix/length> <next-hop>` (CIDR notation preferred)

## File Structure

```
platform/vendors/arista/
├── arista_models.go                    # Arista-specific data models
└── eos/
    ├── arista_eos_config_parsers.go    # Config stanza parsers
    ├── arista_eos_operational_parsers.go  # Show command parsers
    ├── arista_eos_processor.go         # Main processor / orchestrator
    ├── arista_eos_mappers.go           # Topology model mappers
    ├── arista_eos_routing_mappers.go   # L3 routing model mappers
    ├── arista_eos_parsers_test.go      # Comprehensive tests
    └── testdata/                       # Sample EOS configurations
        ├── sample_eos_switch_config.txt   # Full switch config
        ├── sample_lldp_neighbors.txt
        ├── sample_mac_address_table.txt
        ├── sample_ip_arp.txt
        ├── sample_ip_route.txt
        └── sample_interfaces_status.txt
```

## Test Coverage
- Interface block parsing (access, trunk, routed, shutdown)
- MAC address table parsing with MAC normalization
- LLDP neighbor parsing
- ARP table parsing
- Routing table parsing (connected, static, OSPF, BGP)
- Interface status parsing
- MLAG config parsing and topology inference
- VXLAN config parsing
- Static route parsing
- OSPF/BGP router config parsing
- Full config integration (sample_eos_switch_config.txt)
- L2 config builder integration
- Port security parsing
- MAC address normalization

## Usage

```go
processor := NewAristaEOSProcessor("/path/to/eos/config.txt")
err := processor.ProcessEOSFile()
if err != nil {
    log.Fatal(err)
}

// Access parsed topology
device := processor.TopologyDevice
interfaces := processor.TopologyInterfaces
routes := processor.TopologyRoutingEntries

// Get L2-specific config
l2Config := processor.BuildEOSL2Config()

// Get full L3 routing table
routingTable := processor.BuildEOSRoutingTable()
```

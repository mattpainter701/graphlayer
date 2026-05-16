# Dell OS10 / FTOS Vendor Parser

This directory contains the Dell OS10 configuration and operational parsers, mappers, VLT topology inference, and FTOS backward compatibility layer.

## Structure

```
dell/
├── dell_models.go           # OS10-specific data models
└── os10/
    ├── dell_os10_processor.go          # Main processor: config parsing, L2 builder
    ├── dell_os10_config_parsers.go     # Interface, VLAN, VLT, ACL, port-security parsers
    ├── dell_os10_operational_parsers.go # LLDP, MAC, ARP, route, interface status, VLT ops
    ├── dell_os10_mappers.go           # Topology model mapping (Device, Interfaces, Routing)
    ├── dell_os10_routing_mappers.go   # L3 routing table builder
    ├── dell_os10_parsers_test.go      # Unit and integration tests
    └── testdata/                       # Sample config and operational outputs
        ├── sample_os10_switch_config.txt
        ├── sample_os10_interfaces_status.txt
        ├── sample_os10_mac_address_table.txt
        ├── sample_os10_lldp_neighbors.txt
        ├── sample_os10_ip_arp.txt
        └── sample_os10_ip_route.txt
```

## Key Features

### VLT (Virtual Link Trunking)
- Equivalent to Cisco vPC and Arista MLAG
- `vlt-domain <id>` config parsing with discovery interface, peer-link, backup destination
- VLT operational state parsing from `show vlt <domain-id>`
- VLT topology inference: peer-link, backup-link, vlt-member link types

### FTOS Backward Compatibility
- `NormalizeFTOSInterface()` converts legacy FTOS names:
  - `TenGigabitEthernet 0/1` → `ethernet0/1/1`
  - `GigabitEthernet 0/5` → `ethernet0/1/5`
  - `fortyGigE 0/49` → `ethernet0/1/49`
- `DetectOSVersion()` auto-detects OS10 vs FTOS based on config patterns

### Parsers
- **Config**: Interfaces (L2/L3), VLANs, VLT domains, port-channels, static routes, OSPF, BGP, VRFs, ACLs (standard/extended)
- **Operational**: LLDP neighbors, MAC address table, ARP table, routing table, interface status, VLT status, OSPF neighbors, port security

### Mappers
- Standard `ParsedNetworkTopology` model (compatible with all other vendors)
- L3 routing table with static/OSPF/BGP and VRF support
- L2 switch config builder with SVIs, port-channels, VLANs

## Usage

```go
import "github.com/mattpainter701/graphlayer/platform/platform/vendors/dell/os10"

// Parse a config file
processor := os10.NewDellOS10Processor("/path/to/config.txt")
err := processor.ProcessOS10File()

// Access parsed data
hostname := processor.os10Config.Hostname
vltConfig := processor.os10Config.VLTConfig
interfaces := processor.os10Config.Interfaces

// Build topology model
topology := os10.OS10Mapper(processor)

// Parse operational data
macEntries := os10.ParseOS10MACAddressTable(output, time.Now())
lldpNeighbors := os10.ParseOS10LLDPNeighbors(output)
arpEntries := os10.ParseOS10ARPTable(output, time.Now())
routeEntries := os10.ParseOS10RouteTable(output, time.Now())
```

## Mixed-Vendor Topology

The Dell OS10 mapper produces `ParsedNetworkTopology` — the same standard container used by Arista, Cisco, Fortinet, Palo Alto, and Juniper parsers. This enables mixed-vendor topology analysis across heterogeneous networks.

## Testing

```bash
cd platform/vendors/dell/os10
go test -v ./...
```

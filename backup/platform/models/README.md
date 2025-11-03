# Vendor-Agnostic Network Topology Models

This directory contains the core vendor-agnostic data models that all vendor-specific parsers and mappers output to. These models enable unified network topology analysis across Cisco, Palo Alto, Fortinet, and future vendors.

## Philosophy

**Vendor-Agnostic Design**: These models represent network topology concepts (devices, interfaces, VLANs, routes, policies) independent of vendor implementation details. Cisco IOS, Palo Alto PAN-OS, and FortiGate FortiOS all map to the same model structures.

**Benefits:**
- Single topology analysis engine works across all vendors
- Compare configurations between different vendor devices
- Migrate from one vendor to another with preserved topology
- Build vendor-independent network automation
- Simplify multi-vendor environment management

## Files

### network_topology_models.go
**Size:** ~40,000 lines
**Purpose:** Complete network topology data models

**Key Models:**

#### Device & Context
- `Device` - Network device (switch, router, firewall)
- `VirtualContext` - Multi-tenancy (VDOM, vsys, VRF, context)
- `Interface` - Physical/virtual/aggregate interfaces
- `VLAN` - VLAN configuration

#### L2 Topology
- `FDBEntry` - MAC address table entry (forwarding database)
- `L2Switch` - Layer 2 switch with ports and VLANs
- `L2SwitchPort` - Per-port L2 configuration
- `L2PortChannel` - Link Aggregation Group (LAG)
- `L2Neighbor` - CDP/LLDP neighbor discovery

#### L3 Topology
- `Route` - Routing table entry
- `ARPEntry` - IP-to-MAC mapping
- `BGPSession` - BGP peering session
- `VRF` - Virtual Routing and Forwarding

#### Security
- `SecurityZone` - Security zone (trust, untrust, DMZ)
- `FirewallPolicy` - Firewall rule (L3-L7)
- `NAT` - Network Address Translation (SNAT, DNAT)
- `Address` - Address object (IP, network, range, FQDN)
- `Service` - Service object (TCP/UDP ports, ICMP)

#### VPN
- `VPNTunnel` - IPsec/SSL VPN tunnel
- `VPN Endpoint` - VPN gateway configuration

#### Metadata
- `IntegrationMessage` - Warnings, errors, info during mapping
- `EvidenceMetadata` - Data provenance and hashing

### graph_models.go
**Size:** ~39,000 lines
**Purpose:** Graph representation of network topology

**Key Models:**

#### Graph Structure
- `NetworkGraph` - Complete network as a graph
- `Node` - Device, interface, or endpoint node
- `Edge` - Connection between nodes (physical, logical, virtual)
- `Path` - Multi-hop path through network

#### Graph Analysis
- `ConnectedComponent` - Isolated network segments
- `ShortestPath` - Least-cost path between nodes
- `SpanningTree` - Loop-free topology
- `ReachabilityMatrix` - Which nodes can reach which

#### Visualization
- `GraphLayout` - Node positioning for visualization
- `GraphStyle` - Rendering styles and colors
- `GraphFilter` - Filter nodes/edges by criteria

## Model Hierarchy

```
Device
├─ VirtualContexts (VDOMs, vsys, VRFs)
│  ├─ Interfaces
│  ├─ SecurityZones
│  ├─ FirewallPolicies
│  ├─ NAT Rules
│  └─ Routes
│
├─ VLANs
├─ L2Neighbors (CDP/LLDP)
└─ Management Info
```

## Vendor Mapping Examples

### Interface Mapping

**Cisco IOS:**
```go
// GigabitEthernet1/0/1 with VLAN 10
ciscoInterface → Interface{
    Name: "GigabitEthernet1/0/1",
    Type: "physical",
    Speed: 1000,
    VLAN: 10,
}
```

**Palo Alto:**
```go
// ethernet1/1 in Layer 3 mode
paloaltoInterface → Interface{
    Name: "ethernet1/1",
    Type: "layer3",
    Zone: "trust",
    IPAddress: "10.1.1.1/24",
}
```

**FortiGate:**
```go
// port1 with SD-WAN membership
fortinetInterface → Interface{
    Name: "port1",
    Type: "physical",
    Metadata: map[string]string{
        "sdwan_member": "true",
        "sdwan_latency": "15ms",
    },
}
```

### Firewall Policy Mapping

**Cisco ASA ACL:**
```go
// access-list OUTSIDE_IN extended permit tcp any host 10.1.1.10 eq 443
ciscoACL → FirewallPolicy{
    SourceZones:      []string{"outside"},
    DestinationZones: []string{"inside"},
    SourceAddresses:  []string{"any"},
    DestinationAddresses: []string{"10.1.1.10"},
    Services:         []string{"tcp-443"},
    Action:           "allow",
}
```

**Palo Alto Security Policy:**
```go
// Rule: Allow-Web-Traffic (with App-ID)
paloaltoPolicy → FirewallPolicy{
    SourceZones:      []string{"trust"},
    DestinationZones: []string{"untrust"},
    Applications:     []string{"web-browsing", "ssl"},
    Action:           "allow",
    Metadata: map[string]string{
        "app_id_enabled": "true",
    },
}
```

**FortiGate IPv4 Policy:**
```go
// Policy 10 with inline NAT
fortinetPolicy → FirewallPolicy{
    RuleNumber:       "ipv4-10",
    SourceZones:      []string{"internal"},
    DestinationZones: []string{"wan1"},
    Action:           "accept",
    Metadata: map[string]string{
        "ip_version": "4",
        "nat_enabled": "true",
    },
}
```

### NAT Mapping

**Cisco IOS NAT:**
```go
// ip nat inside source list 1 interface GigabitEthernet0/0 overload
ciscoNAT → NAT{
    Type:              "source",
    SourceAddresses:   []string{"192.168.1.0/24"},
    TranslatedSource:  "interface:GigabitEthernet0/0",
    Method:            "overload",  // PAT
}
```

**Palo Alto Source NAT:**
```go
// Source NAT with dynamic IP and port
paloaltoNAT → NAT{
    Type:              "source",
    SourceZones:       []string{"trust"},
    DestinationZones:  []string{"untrust"},
    TranslatedSource:  "dynamic-ip-and-port",
}
```

**FortiGate Central SNAT:**
```go
// Central SNAT policy 1
fortinetNAT → NAT{
    Type:              "source",
    SourceAddresses:   []string{"10.0.0.0/8"},
    TranslatedSource:  "ip-pool-1",
    Metadata: map[string]string{
        "source_type": "central_snat",  // Unique to FortiGate
    },
}
```

## Multi-Tenancy Mapping

| Vendor | Concept | Maps To |
|--------|---------|---------|
| **Cisco ASA** | Security Context | `VirtualContext` |
| **Cisco IOS** | VRF | `VRF` (separate model) |
| **Palo Alto** | Virtual System (vsys) | `VirtualContext` |
| **FortiGate** | Virtual Domain (VDOM) | `VirtualContext` |

## Package Structure

```go
package models

import (
    "time"
)
```

## Usage in Mappers

### Cisco IOS Mapper Example
```go
import "github.com/mattpainter701/graphlayer/platform/models"

func BuildIOSInterface(ciscoInterface CiscoInterface) models.Interface {
    return models.Interface{
        ID:          generateID(),
        Name:        ciscoInterface.Name,
        Type:        mapInterfaceType(ciscoInterface.Type),
        Description: ciscoInterface.Description,
        Speed:       ciscoInterface.Speed,
        Duplex:      ciscoInterface.Duplex,
        VLAN:        ciscoInterface.VLAN,
        IPAddress:   ciscoInterface.IPAddress,
        Netmask:     ciscoInterface.Netmask,
        Status:      ciscoInterface.Status,
        Protocol:    ciscoInterface.Protocol,
    }
}
```

### Palo Alto Mapper Example
```go
import "github.com/mattpainter701/graphlayer/platform/models"

func BuildPANOSFirewallPolicy(paloPolicy PANOSSecurityPolicy) models.FirewallPolicy {
    return models.FirewallPolicy{
        ID:                   paloPolicy.Name,
        RuleNumber:           paloPolicy.RuleNumber,
        SourceZones:          paloPolicy.SourceZones,
        DestinationZones:     paloPolicy.DestinationZones,
        SourceAddresses:      paloPolicy.SourceAddresses,
        DestinationAddresses: paloPolicy.DestinationAddresses,
        Applications:         paloPolicy.Applications,  // App-ID
        Action:               mapAction(paloPolicy.Action),
        Logging:              paloPolicy.LogSetting != "",
        Metadata: map[string]string{
            "app_id_enabled": "true",
            "security_profiles": strings.Join(paloPolicy.ProfileGroups, ","),
        },
    }
}
```

## Model Validation

Models include validation rules:
- Required fields must be non-empty
- IP addresses must be valid CIDR or dotted-decimal
- Port ranges must be 1-65535
- VLANs must be 1-4094

Example:
```go
func (i *Interface) Validate() error {
    if i.Name == "" {
        return fmt.Errorf("interface name is required")
    }
    if i.VLAN < 0 || i.VLAN > 4094 {
        return fmt.Errorf("invalid VLAN: %d", i.VLAN)
    }
    return nil
}
```

## Testing

No tests in this directory currently. Tests are in vendor-specific mapper tests:
- `vendors/cisco/ios/cisco_ios_mappers_test.go`
- `vendors/cisco/nxos/cisco_nxos_mappers_test.go`
- `vendors/paloalto/firewall/paloalto_mappers_test.go`

## Integration with Graph Models

Network topology models can be converted to graph representation:

```go
import (
    "github.com/mattpainter701/graphlayer/platform/models"
)

// Build topology from vendor parsers
device := models.Device{...}
interfaces := []models.Interface{...}
neighbors := []models.L2Neighbor{...}

// Convert to graph
graph := models.BuildNetworkGraph(device, interfaces, neighbors)

// Analyze graph
paths := graph.FindAllPaths(sourceNode, destNode)
components := graph.FindConnectedComponents()
```

## Metadata Usage

The `Metadata` field (map[string]string) preserves vendor-specific information:

### Common Metadata Keys
- `vendor` - "cisco", "paloalto", "fortinet"
- `platform` - "ios", "nxos", "asa", "panos", "fortios"
- `confidence` - "high", "medium", "low" (for L2 topology)
- `source_command` - CLI command or API endpoint
- `collection_timestamp` - ISO 8601 timestamp

### Vendor-Specific Keys
- Cisco: `cdp_capabilities`, `stp_port_role`, `vlan_type`
- Palo Alto: `app_id_enabled`, `security_profiles`, `vsys_id`
- FortiGate: `sdwan_member`, `sdwan_latency`, `ip_version`, `source_type`

## Future Enhancements

Planned additions:
1. **Wireless Models**: Access points, controllers, SSIDs
2. **SD-WAN Models**: Vendor-agnostic SD-WAN (currently FortiGate-specific)
3. **Cloud Integration**: AWS, Azure, GCP network topology
4. **Container Networks**: Kubernetes, Docker networking
5. **Validation Framework**: Comprehensive model validation
6. **Serialization**: JSON, YAML, Protocol Buffers
7. **Versioning**: Model version tracking for backward compatibility

## Model Change Policy

**Backward Compatibility:** New fields can be added, but existing fields should not be removed or renamed. Use deprecation warnings before removal.

**Versioning:** Models will use semantic versioning (v1, v2, etc.) for major changes.

## Documentation

Detailed model specifications:
- [L2 Models Specification](../docs/specifications/models/l2_models_spec.md)
- [Integration Guide](../docs/getting-started/INTEGRATION_GUIDE.md)

## Contributing

When modifying models:
1. Ensure backward compatibility
2. Add validation rules for new fields
3. Update all vendor mappers to populate new fields
4. Add examples to this README
5. Update documentation

## Contact

For questions about models:
- Review integration guide
- Check vendor mapper implementations
- See project history for design decisions

# Cisco Vendor Implementation

This directory contains all Cisco-specific parsers, mappers, and models for the multi-vendor network topology platform.

## Supported Platforms

### Cisco IOS
**Location:** `ios/`

Cisco IOS is the operating system for enterprise routers and switches (Catalyst 2960, 3750, 3850, 4500, etc.).

**Files:**
- `cisco_ios_parsers.go` - Operational command parsers (show mac address-table, show cdp neighbors, etc.)
- `cisco_ios_parsers_test.go` - Unit tests for parsers
- `cisco_ios_l2security_parsers.go` - L2 security control parsers (DHCP snooping, DAI, 802.1X, port security)
- `cisco_ios_mappers.go` - Maps parsed data to vendor-agnostic topology models
- `cisco_ios_mappers_test.go` - Unit tests for mappers
- `cisco_ios_processor.go` - High-level processor coordinating parsing and mapping
- `cisco_ios_config_parsers.go` - Configuration file parsers

**Commands Supported:** 25+ operational commands including:
- MAC address table
- ARP table
- CDP/LLDP neighbors
- VLAN configuration
- Interface status
- Spanning Tree
- DHCP snooping
- 802.1X authentication
- Port security

### Cisco NX-OS
**Location:** `nxos/`

Cisco NX-OS is the operating system for Nexus data center switches (Nexus 3000, 5000, 7000, 9000 series).

**Files:**
- `cisco_nxos_parsers.go` - Operational command parsers
- `cisco_nxos_parsers_test.go` - Unit tests for parsers
- `cisco_nxos_mappers.go` - Maps parsed data to vendor-agnostic topology models
- `cisco_nxos_mappers_test.go` - Unit tests for mappers

**Commands Supported:** 24+ operational commands including:
- MAC address table
- ARP table
- CDP/LLDP neighbors
- VLAN configuration
- VPC (Virtual Port Channel)
- FabricPath
- vPC peer-link
- L2 security controls

**Key Differences from IOS:**
- Multi-line command output format (e.g., routing table)
- VPC-specific commands and states
- FabricPath topology
- Different interface naming (Ethernet1/1 vs GigabitEthernet1/0/1)

### Cisco ASA
**Location:** `asa/`

Cisco ASA is the Adaptive Security Appliance for firewall and VPN services.

**Files:**
- `cisco_asa_parsers.go` - Operational command parsers
- `cisco_asa_parsers_test.go` - Unit tests for parsers

**Commands Supported:** 13+ operational commands including:
- Interface configuration
- Access lists
- NAT rules
- VPN configuration
- Failover status
- Object groups

## Data Models

**Location:** `cisco_models.go`

Contains Cisco-specific data structures before mapping to vendor-agnostic models:

### Core Models
- `CiscoDevice` - Device-level configuration
- `CiscoInterface` - Interface with L2/L3 properties
- `CiscoVLAN` - VLAN configuration
- `CiscoMACTableEntry` - MAC address table entry with aging
- `CiscoARPEntry` - ARP table entry
- `CiscoCDPNeighbor` - CDP neighbor discovery
- `CiscoLLDPNeighbor` - LLDP neighbor discovery

### L2 Security Models
- `DHCPSnoopingBinding` - DHCP snooping bindings for IP-MAC trust
- `ARPInspectionStatistics` - Dynamic ARP Inspection stats
- `AuthenticationSession` - 802.1X/MAB authentication state
- `PortSecurityStatus` - Port security violations and MAC limits
- `IPSGBinding` - IP Source Guard bindings
- `PrivateVLAN` - Private VLAN isolation

### Spanning Tree Models
- `CiscoSTPInstance` - STP instance per VLAN
- `CiscoSTPInterface` - Per-interface STP state

### NX-OS Specific Models
- `CiscoVPCStatus` - Virtual Port Channel status
- `CiscoVPCPeer` - VPC peer information
- `CiscoFabricPath` - FabricPath topology

## Package Structure

All Cisco code uses the `cisco` package for consistency:

```go
package cisco

import (
    "github.com/mattpainter701/graphlayer/platform/models"
    "github.com/mattpainter701/graphlayer/platform/vendors/shared"
)
```

## Testing

Run all Cisco tests:
```bash
cd vendors/cisco
go test ./...
```

Run specific platform tests:
```bash
go test ./ios -v
go test ./nxos -v
go test ./asa -v
```

## Integration

### With Platform Orchestrator

The platform orchestrator calls Cisco parsers based on device type:

```go
import (
    "github.com/mattpainter701/graphlayer/platform/platform/orchestration"
    "github.com/mattpainter701/graphlayer/platform/vendors/cisco/ios"
)

// Execute collection
result := orchestrator.ExecutePull(ctx, job)

// Parse collected data
macTable := cisco.ParseMACAddressTable(result.ConfigFiles["show_mac_address_table"])
```

### With Parser Bridge

The parser bridge provides vendor-agnostic access:

```go
import "github.com/mattpainter701/graphlayer/platform/platform/integration"

bridge := integration.NewParserBridge()
bridge.RegisterCiscoIOSParser()

// Bridge automatically selects correct parser based on device type
topology := bridge.ParseAndMap(deviceType, commandOutputs)
```

## Command Specifications

Detailed command specifications are in documentation:
- [Cisco IOS Auxiliary Commands](../../docs/specifications/commands/cisco_ios_aux_commands.md)
- [Cisco ASA Commands](../../docs/specifications/commands/cisco_asa_commands.md)
- [Cisco NX-OS Commands](../../docs/specifications/commands/cisco_nxos_commands.md)
- [Cisco IOS L2 Commands](../../docs/specifications/commands/cisco_ios_l2_commands.md)

## Mapper Guides

- [L2 Security Mapper Usage Guide](../../docs/vendor-guides/cisco/l2_security_mapper_guide.md)
- [Parser Documentation](../../docs/vendor-guides/cisco/parser_documentation.md)

## Adding New Cisco Platform Support

To add support for a new Cisco platform (e.g., IOS-XR, IOS-XE):

1. Create new subdirectory: `vendors/cisco/iosxr/`
2. Implement parsers: `cisco_iosxr_parsers.go`
3. Implement mappers: `cisco_iosxr_mappers.go`
4. Add tests: `cisco_iosxr_parsers_test.go`
5. Update `cisco_models.go` if needed
6. Update this README

## Version Support

| Platform | Tested Versions | Status |
|----------|----------------|--------|
| **IOS** | 12.2, 15.0, 15.2 | ✅ Production |
| **IOS-XE** | 16.x, 17.x | ⚠️ Partial (same as IOS) |
| **NX-OS** | 7.x, 9.x | ✅ Production |
| **ASA** | 9.x | ✅ Production |

## Known Limitations

1. **IOS-XE**: Uses same parsers as IOS, some output format differences may require additional handling
2. **NX-OS Multi-line Commands**: Requires special parsing logic (e.g., routing table)
3. **VPC**: Only supports VPC status, not full VPC+ features
4. **FabricPath**: Basic topology only, not full path calculation

## Contributing

When contributing Cisco-specific code:
- Follow existing file naming convention (`cisco_platform_purpose.go`)
- Add comprehensive unit tests
- Update this README with new platforms/commands
- Ensure package declaration is `package cisco`
- Document any platform-specific quirks

## Contact

For Cisco-specific questions or issues:
- See main project documentation
- Check vendor guides in `docs/vendor-guides/cisco/`
- Review project history for implementation details

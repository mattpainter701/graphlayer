# Palo Alto Networks Vendor Implementation

This directory contains all Palo Alto Networks-specific parsers, mappers, and models for the multi-vendor network topology platform.

## Supported Platforms

### PAN-OS Firewall
**Location:** `firewall/`

PAN-OS is the operating system for Palo Alto Networks next-generation firewalls (PA-220, PA-3000, PA-5000, PA-7000 series).

**Files:**
- `paloalto_parsers.go` - XML API response parsers for configuration and operational data
- `paloalto_parsers_test.go` - Unit tests for parsers
- `paloalto_mappers.go` - Maps parsed PAN-OS data to vendor-agnostic topology models
- `paloalto_mappers_test.go` - Unit tests for mappers

**API Endpoints Supported:**
- System information
- Interface configuration (Layer 2, Layer 3, virtual wire, tap)
- Security zones
- Address objects and groups
- Service objects and groups
- Security policies
- NAT policies (Source NAT, Destination NAT)
- Virtual routers and routes
- VPN configuration (IKE, IPsec)
- Virtual systems (vsys) configuration

### Panorama
**Location:** `panorama/`

Panorama is the centralized management system for Palo Alto Networks firewalls.

**Files:**
- `panorama_parsers.go` - Panorama-specific XML parsers
- `panorama_mappers.go` - Maps Panorama configurations to device-level models

**Features:**
- Device group management
- Template configuration
- Shared objects
- Pre/Post rulebase
- Log collector configuration
- Managed device inventory

## Key Concepts

### Virtual Systems (vsys)
PAN-OS uses virtual systems for multi-tenancy:
- Each vsys has its own zones, policies, objects
- Similar to VRFs but at the firewall level
- Default vsys is `vsys1`
- Maps to `VirtualContext` in vendor-agnostic models

### Security Zones
Explicit security model:
- All interfaces must belong to a zone
- Policies are zone-to-zone based
- Common zones: trust, untrust, DMZ
- Zone types: Layer 2, Layer 3, virtual wire, tap

### App-ID
Layer 7 application identification:
- Policies can specify applications instead of just ports
- Examples: ssl, http, ssh, dns
- App groups: web-browsing, file-sharing

### XML API
PAN-OS uses XML for all API interactions:
- Configuration API: `<set>`, `<get>`, `<edit>`, `<delete>`
- Operational API: `<show>` commands
- Commit API: Configuration changes
- All responses are XML format (not JSON)

## Data Structures

Key data structures defined in parsers:

### System
- `PANOSSystemInfo` - Hostname, serial, version, HA state
- `PANOSInterface` - Interface config with zone membership
- `PANOSZone` - Security zone with member interfaces

### Security Policy
- `PANOSSecurityPolicy` - L3-L7 firewall rules with App-ID
- `PANOSAddressObject` - IP/FQDN address objects
- `PANOSServiceObject` - TCP/UDP service objects
- `PANOSApplicationObject` - App-ID application objects

### NAT
- `PANOSSourceNAT` - SNAT with dynamic IP or PAT
- `PANOSDestinationNAT` - DNAT for inbound services
- `PANOSNATPool` - Dynamic NAT IP pools

### Routing
- `PANOSVirtualRouter` - Virtual router instance
- `PANOSStaticRoute` - Static routing entries
- `PANOSRoute` - Operational routing table (BGP, OSPF, static)

### VPN
- `PANOSIKEGateway` - IKE Phase 1 configuration
- `PANOSIPsecTunnel` - IPsec Phase 2 configuration
- `PANOSVPNTunnelStatus` - Operational VPN status

### Panorama-Specific
- `PanoramaDeviceGroup` - Device grouping for shared config
- `PanoramaTemplate` - Network template for devices
- `PanoramaManagedDevice` - Inventory of managed firewalls

## Package Structure

```go
package paloalto

import (
    "encoding/xml"
    "github.com/mattpainter701/graphlayer/platform/models"
)
```

## XML API Examples

### Get System Info
```xml
<show><system><info></info></system></show>
```

### Get Security Policies
```xml
<show><config><running><xpath>/config/devices/entry[@name='localhost.localdomain']/vsys/entry[@name='vsys1']/rulebase/security</xpath></running></config></show>
```

### Get Interface Configuration
```xml
<get><config><xpath>/config/devices/entry[@name='localhost.localdomain']/network/interface</xpath></config></get>
```

## Testing

Run all Palo Alto tests:
```bash
cd vendors/paloalto
go test ./...
```

Run specific tests:
```bash
go test ./firewall -v
go test ./panorama -v
```

Test specific functions:
```bash
go test ./firewall -run TestParsePANOSSecurityPolicy -v
```

## Integration

### Direct Firewall API

```go
import "github.com/mattpainter701/graphlayer/platform/vendors/paloalto/firewall"

// Parse system info from XML response
systemInfo := paloalto.ParsePANOSSystemInfo(xmlResponse)

// Parse security policies
policies := paloalto.ParsePANOSSecurityPolicies(xmlResponse)

// Map to vendor-agnostic topology
topology := paloalto.BuildPANOSNetworkTopology(systemInfo, interfaces, policies, ...)
```

### Via Panorama

```go
import (
    "github.com/mattpainter701/graphlayer/platform/vendors/paloalto/panorama"
)

// Get device groups
deviceGroups := panorama.ParsePanoramaDeviceGroups(xmlResponse)

// Get managed devices
devices := panorama.ParsePanoramaManagedDevices(xmlResponse)

// Get shared objects
sharedAddresses := panorama.ParsePanoramaSharedObjects(xmlResponse, "address")
```

## Architecture Differences from Cisco

| Feature | Cisco | Palo Alto |
|---------|-------|-----------|
| **API Protocol** | SSH CLI | XML HTTPS API |
| **Config Format** | Text CLI | XML |
| **Multi-Tenancy** | VRF/Context | Virtual Systems (vsys) |
| **Security Model** | ACLs | Zone-based + App-ID |
| **Management** | N/A (Prime) | Panorama |
| **L7 Inspection** | NBAR | App-ID (native) |

## Mapper Highlights

### Security Policy Mapping
PAN-OS policies map to `FirewallPolicy` model with:
- Source/destination zones → security zones
- Applications → service definitions (with metadata)
- Action (allow/deny/drop) → direct mapping
- Logging → preserved

### NAT Mapping
Both Source NAT and Destination NAT map to `NAT` model:
- SNAT: Dynamic IP, Dynamic IP and Port (PAT), Static IP
- DNAT: Static IP, port translation

### VPN Mapping
IKE Gateway + IPsec Tunnel → `VPNTunnel` model:
- Phase 1 (IKE) settings preserved
- Phase 2 (IPsec) settings preserved
- Proxy IDs mapped to traffic selectors

## Command Specifications

Detailed API endpoint specifications:
- [Palo Alto Mapper Usage Guide](../../docs/vendor-guides/paloalto/paloalto_mapper_guide.md)
- [Palo Alto Parser Documentation](../../docs/vendor-guides/paloalto/paloalto_parser_guide.md)

## Version Support

| Platform | Tested Versions | Status |
|----------|----------------|--------|
| **PAN-OS** | 9.x, 10.x, 11.x | ✅ Production |
| **Panorama** | 9.x, 10.x, 11.x | ✅ Production |

## Known Limitations

1. **Application Objects**: Only names parsed, not full application definitions
2. **Dynamic Address Groups**: Tags not fully resolved to IPs
3. **GlobalProtect**: VPN configuration only, not client status
4. **Panorama Pre/Post Rules**: Basic support, not full rule merging logic
5. **URL Filtering**: Profile names only, not full URL database

## PAN-OS Specific Features

### Advantages
- Native XML API (no CLI parsing)
- Rich operational data via API
- App-ID provides L7 visibility
- Consistent API across versions
- Panorama centralized management

### Challenges
- XML parsing complexity
- vsys hierarchy requires special handling
- Shared vs device-local object resolution
- Template inheritance in Panorama

## Adding New PAN-OS Features

To add support for new PAN-OS features:

1. Identify XML API endpoint and XPath
2. Add struct definitions to `paloalto_parsers.go`
3. Implement parser function with XML unmarshaling
4. Add unit tests with sample XML
5. Add mapper function to convert to vendor-agnostic models
6. Update this README

## Panorama Integration Workflow

```
Panorama
├─ Device Group: Production
│  ├─ Shared Objects (addresses, services)
│  ├─ Pre-Rules (global deny rules)
│  ├─ Post-Rules (logging rules)
│  └─ Devices: FW-01, FW-02, FW-03
│     └─ Each device has local rules + shared rules
│
└─ Template: Network-Template
   ├─ Interface configuration
   ├─ Zone configuration
   └─ Applied to: FW-01, FW-02, FW-03
```

## Security Considerations

1. **API Key Storage**: Use encrypted credential service
2. **HTTPS Only**: Always use HTTPS for API calls
3. **Certificate Validation**: Validate SSL certificates in production
4. **Read-Only Access**: Use read-only API roles when possible
5. **Rate Limiting**: Respect API rate limits (Panorama has limits)

## Contributing

When contributing Palo Alto-specific code:
- Use XML sample data from real PAN-OS devices
- Test against multiple PAN-OS versions
- Document XPath expressions
- Handle empty/missing XML elements gracefully
- Follow existing naming convention (`paloalto_purpose.go`)

## References

- [PAN-OS XML API Guide](https://docs.paloaltonetworks.com/pan-os/10-2/pan-os-panorama-api)
- [Panorama Administrator's Guide](https://docs.paloaltonetworks.com/panorama)
- [App-ID Documentation](https://applipedia.paloaltonetworks.com/)

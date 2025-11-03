# Device Types & Command Templates

Device type definitions, command templates, and capability mappings.

## Overview

- **DeviceType**: Enumeration of supported device types
- **CommandTemplate**: Structured command definitions with metadata
- **CommandCategory**: Command categorization (operational, security, diagnostic)
- **DeviceCapabilities**: Device capability tracking

## Device Types

### Cisco Devices

```go
const (
    DeviceTypeCiscoIOS     DeviceType = "cisco_ios"
    DeviceTypeCiscoASA     DeviceType = "cisco_asa"
    DeviceTypeCiscoNXOS    DeviceType = "cisco_nxos"
    DeviceTypeCiscoFMC     DeviceType = "cisco_fmc"  // Firepower Management Center
)
```

### Palo Alto Devices

```go
const (
    DeviceTypePaloAltoPA       DeviceType = "palo_alto_pa"
    DeviceTypePaloAltoPanorama DeviceType = "palo_alto_panorama"
)
```

### Other Vendors

```go
const (
    DeviceTypeJuniperJunos          DeviceType = "juniper_junos"
    DeviceTypeJuniperSRX            DeviceType = "juniper_srx"
    DeviceTypeFortinetFortiGate     DeviceType = "fortinet_fortigate"
    DeviceTypeFortinetFortiManager  DeviceType = "fortinet_fortimanager"
    DeviceTypeCheckPointGaia        DeviceType = "checkpoint_gaia"
    DeviceTypeF5BigIP               DeviceType = "f5_bigip"
)
```

### Management Systems

```go
const (
    DeviceTypeSolarWindsNCM         DeviceType = "solarwinds_ncm"        // Network Configuration Manager
    DeviceTypeFortinetFortiManager  DeviceType = "fortinet_fortimanager" // Moved from Other Vendors section
)
```

## Command Categories

```go
const (
    CommandCategoryConfig      CommandCategory = "config"
    CommandCategoryOperational CommandCategory = "operational"
    CommandCategoryDiagnostic  CommandCategory = "diagnostic"
    CommandCategoryMonitoring  CommandCategory = "monitoring"
    CommandCategorySecurity    CommandCategory = "security"
)
```

## CommandTemplate

Structured command definition with metadata:

```go
type CommandTemplate struct {
    Name        string           `json:"name"`
    Description string           `json:"description"`
    Category    CommandCategory  `json:"category"`
    Command     string           `json:"command"`
    Platforms   []DeviceType     `json:"platforms"`
    Timeout     time.Duration    `json:"timeout"`
    ParseOutput bool             `json:"parse_output"`
    Tags        map[string]string `json:"tags,omitempty"`
}
```

### Example Command Template

```go
{
    Name:        "show_mac_address_table_dynamic",
    Description: "MAC-to-port bindings for L2 adjacency and FDB",
    Category:    types.CommandCategorySecurity,
    Command:     "show mac address-table dynamic",
    Platforms:   []types.DeviceType{types.DeviceTypeCiscoIOS, types.DeviceTypeCiscoNXOS},
    Timeout:     15 * time.Second,
    Tags: map[string]string{
        "l2":       "true",
        "fdb":      "true",
        "priority": "p0",
    },
}
```

## Device Capabilities

Track what a device can do:

```go
type DeviceCapabilities struct {
    CanGetRunningConfig bool `json:"can_get_running_config"`
    CanGetStartupConfig bool `json:"can_get_startup_config"`
}
```

### Usage

```go
caps := types.GetDeviceCapabilities(types.DeviceTypeCiscoIOS)
if caps.CanGetStartupConfig {
    startupConfig, _ := connector.GetStartupConfig(ctx, 60*time.Second)
}
```

## OS Types

```go
const (
    OSTypeIOS      OSType = "ios"
    OSTypeASA      OSType = "asa"
    OSTypeNXOS     OSType = "nxos"
    OSTypePANOS    OSType = "panos"
    OSTypeJunos    OSType = "junos"
    OSTypeFortiOS  OSType = "fortios"
    OSTypeGaia     OSType = "gaia"
    OSTypeBigIP    OSType = "bigip"
)
```

## Config Types

```go
const (
    ConfigTypeRunning   ConfigType = "running"
    ConfigTypeStartup   ConfigType = "startup"
    ConfigTypeCandidate ConfigType = "candidate"
)
```

## Cisco IOS Command Set

### P0 Critical Commands (11)

L2 Topology & Adjacency:
- `show mac address-table dynamic`
- `show lldp neighbors detail`
- `show cdp neighbors detail`
- `show spanning-tree vlan 1-4094 detail`
- `show interfaces switchport`

Interface & VLAN State:
- `show interfaces status`
- `show interfaces trunk`
- `show vlan`
- `show etherchannel summary`

L3 Handoff:
- `show ip arp`
- `show ip interface brief`

### P1 Security Commands (3)

- `show ip dhcp snooping binding`
- `show ip arp inspection`
- `show port-security`

## Adding New Device Types

1. Define constant in `device_types.go`:

```go
const DeviceTypeMyVendor DeviceType = "my_vendor"
```

2. Register in `registry/factory.go`:

```go
cf.deviceTypeRegistry[types.DeviceTypeMyVendor] = "ssh"
```

3. Add commands in `orchestrator/connector_orchestrator.go`:

```go
case types.DeviceTypeMyVendor:
    return getMyVendorAuxCommands()
```

## Related Documentation

- [registry/README.md](../registry/README.md) - Device type registration
- [orchestrator/README.md](../orchestrator/README.md) - Command execution
- [ssh/README.md](../ssh/README.md) - SSH device support

---

**Status**: Production Ready  
**Version**: 1.0


# FortiManager Integration

FortiManager SDK integration for centralized management of FortiGate firewalls. This module provides programmatic access to FortiManager's JSON-RPC API for retrieving device configurations, policy packages, and global objects across multiple Administrative Domains (ADOMs).

## Overview

FortiManager is Fortinet's centralized management platform that enables:
- **Multi-ADOM Management**: Organize devices into Administrative Domains
- **Policy Packages**: Centralized policy management with installation to target devices
- **Global Objects**: Shared address and service objects across ADOMs
- **Device Monitoring**: Track connection status and configuration sync state
- **Configuration Retrieval**: Pull device configurations for analysis

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    FortiManager                          │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐             │
│  │  ADOM 1  │  │  ADOM 2  │  │  ADOM 3  │             │
│  │  (root)  │  │  (prod)  │  │   (dev)  │             │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘             │
│       │             │              │                     │
│  ┌────▼─────────────▼──────────────▼─────┐             │
│  │      Managed FortiGate Devices         │             │
│  │  • Firewall-01 (Connected)             │             │
│  │  • Firewall-02 (Connected)             │             │
│  │  • Firewall-03 (Disconnected)          │             │
│  └────────────────────────────────────────┘             │
└─────────────────────────────────────────────────────────┘
                       │
                       │ JSON-RPC API
                       │
               ┌───────▼────────┐
               │  Import Service │
               │                 │
               │  • List ADOMs   │
               │  • Get Devices  │
               │  • Get Configs  │
               │  • Get Packages │
               └─────────────────┘
```

## Quick Start

### Basic Import

```go
import (
    "github.com/your-org/connectors/integrations/fortimanager"
)

// Create FortiManager client
config := fortimanager.FortiManagerConfig{
    BaseURL:            "https://fortimanager.example.com",
    Username:           "admin",
    Password:           "password",
    InsecureSkipVerify: true, // Only for dev/self-signed certs
    Timeout:            60 * time.Second,
}

// Create import service
service, err := fortimanager.NewImportService(config)
if err != nil {
    log.Fatal(err)
}
defer service.Close()

// Import all configurations
request := fortimanager.ImportRequest{
    IncludeGlobalObjects:  true,
    IncludePolicyPackages: true,
}

summary, err := service.ImportConfigurations(request)
if err != nil {
    log.Fatal(err)
}

fmt.Println(summary.FormatSummary())
```

### List ADOMs

```go
service, _ := fortimanager.NewImportService(config)
defer service.Close()

adoms, err := service.ListADOMs()
for _, adom := range adoms {
    fmt.Printf("%s - %s (OS: %s)\n", adom.Name, adom.Description, adom.OSVersion)
}
```

### List Managed Devices

```go
devices, err := service.ListDevices("root") // ADOM name
for _, device := range devices {
    status := "Disconnected"
    if device.Connected == 1 {
        status = "Connected"
    }
    fmt.Printf("%s (%s) - %s - %s\n",
        device.Name, device.IPAddress, device.Model, status)
}
```

## Configuration

### FortiManagerConfig

| Field | Type | Description |
|-------|------|-------------|
| `BaseURL` | string | FortiManager base URL (e.g., `https://fmgr.example.com`) |
| `Username` | string | Admin username |
| `Password` | string | Admin password |
| `InsecureSkipVerify` | bool | Skip TLS certificate verification (dev only) |
| `Timeout` | time.Duration | HTTP request timeout (default: 30s) |

### ImportRequest

| Field | Type | Description |
|-------|------|-------------|
| `ADOMs` | []string | Filter by specific ADOMs (empty = all) |
| `DeviceNames` | []string | Filter by device names (empty = all) |
| `IncludeGlobalObjects` | bool | Include ADOM-level global objects |
| `IncludePolicyPackages` | bool | Include policy packages |

## API Reference

### ImportService

#### `NewImportService(config FortiManagerConfig) (*ImportService, error)`
Creates a new import service and authenticates with FortiManager.

#### `ImportConfigurations(request ImportRequest) (*ImportSummary, error)`
Imports configurations from FortiManager based on filter criteria.

**Returns**: Summary with ADOM details, device counts, and error list

#### `ListADOMs() ([]ADOMInfo, error)`
Lists all Administrative Domains.

**Returns**: Array of ADOM information including name, description, mode, OS version

#### `ListDevices(adomName string) ([]ManagedDevice, error)`
Lists managed devices in a specific ADOM.

**Returns**: Array of device information including:
- Name, serial number, IP address
- Model, version, connection status
- HA mode, VDOM mode, install status
- Device groups

#### `GetDeviceConfiguration(adomName, deviceName string) (map[string]interface{}, error)`
Retrieves configuration for a specific device.

**Returns**: Raw configuration data (requires FortiGate parsers for processing)

#### `GetADOMStatistics(adomName string) (*ADOMStatistics, error)`
Gets statistics for an ADOM.

**Returns**: Device counts, policy package count

#### `Close() error`
Logs out from FortiManager and closes the session.

### FortiManagerClient (Low-Level)

For advanced use cases, you can use the low-level client directly:

#### `Login() error`
Authenticates and obtains session ID.

#### `Logout() error`
Terminates the session.

#### `GetADOMs() ([]ADOMInfo, error)`
Gets ADOM list.

#### `GetManagedDevices(adomName string) ([]ManagedDevice, error)`
Gets devices in an ADOM.

#### `GetPolicyPackages(adomName string) ([]PolicyPackage, error)`
Gets policy packages in an ADOM.

#### `GetGlobalObjects(adomName, objectType string) ([]interface{}, error)`
Gets global objects (addresses, services, etc.).

**Object Types**:
- `firewall/address` - Address objects
- `firewall/addrgrp` - Address groups
- `firewall/service/custom` - Custom services
- `firewall/service/group` - Service groups

#### `InstallPolicyPackage(adomName, packageName string, targets []string) error`
Installs a policy package to target devices.

## Usage Examples

### Example 1: Import Only Connected Devices

```go
service, _ := fortimanager.NewImportService(config)
defer service.Close()

adoms, _ := service.ListADOMs()

for _, adom := range adoms {
    devices, _ := service.ListDevices(adom.Name)

    // Filter for connected devices only
    connectedNames := []string{}
    for _, device := range devices {
        if device.Connected == 1 {
            connectedNames = append(connectedNames, device.Name)
        }
    }

    // Import only connected devices
    request := fortimanager.ImportRequest{
        ADOMs:       []string{adom.Name},
        DeviceNames: connectedNames,
    }

    summary, _ := service.ImportConfigurations(request)
    fmt.Println(summary.FormatSummary())
}
```

### Example 2: Get ADOM Statistics

```go
service, _ := fortimanager.NewImportService(config)
defer service.Close()

stats, _ := service.GetADOMStatistics("root")

fmt.Printf("ADOM: %s\n", stats.ADOMName)
fmt.Printf("Total Devices: %d\n", stats.TotalDevices)
fmt.Printf("Connected: %d\n", stats.ConnectedDevices)
fmt.Printf("Policy Packages: %d\n", stats.PolicyPackages)
```

### Example 3: Filter by Device Groups

```go
service, _ := fortimanager.NewImportService(config)
defer service.Close()

devices, _ := service.ListDevices("production")

// Filter devices that belong to "datacenter" group
datacenterDevices := []string{}
for _, device := range devices {
    for _, group := range device.DeviceGroups {
        if group == "datacenter" {
            datacenterDevices = append(datacenterDevices, device.Name)
            break
        }
    }
}

request := fortimanager.ImportRequest{
    ADOMs:       []string{"production"},
    DeviceNames: datacenterDevices,
}

summary, _ := service.ImportConfigurations(request)
```

### Example 4: Monitor Install Status

```go
service, _ := fortimanager.NewImportService(config)
defer service.Close()

devices, _ := service.ListDevices("root")

for _, device := range devices {
    status := "Unknown"
    switch device.InstallStatus {
    case 1:
        status = "Installed (in sync)"
    case 2:
        status = "Modified (out of sync)"
    }

    fmt.Printf("%s: %s\n", device.Name, status)
}
```

## Integration with FortiGate Parsers

The FortiManager integration retrieves raw configuration data. To extract structured topology information, use FortiGate parsers:

```go
// 1. Import config from FortiManager
service, _ := fortimanager.NewImportService(config)
configData, _ := service.GetDeviceConfiguration("root", "firewall-01")

// 2. Parse with FortiGate parsers (from fortinet.parsers.go)
interfaces, _ := fortinet.ParseFortiGateInterfaces(configData["interfaces"])
policies, _ := fortinet.ParseFortiGateIPv4Policies(configData["policies"])

// 3. Map to topology models (from fortinet.mappers.go)
processor := fortinet.NewFortiGateProcessor(systemStatus)
processor.interfaces = interfaces
processor.ipv4Policies = policies
processor.BuildNetworkTopologyModels()

// 4. Access vendor-agnostic topology
topology := processor.TopologyInterfaces
```

## Error Handling

The SDK provides detailed error information:

```go
summary, err := service.ImportConfigurations(request)
if err != nil {
    log.Fatalf("Import failed: %v", err)
}

// Check for partial failures
if summary.FailedImports > 0 {
    fmt.Printf("Warning: %d/%d imports failed\n",
        summary.FailedImports, summary.TotalDevices)

    for _, importErr := range summary.Errors {
        fmt.Printf("[%s/%s] %s\n",
            importErr.ADOM, importErr.Device, importErr.Error)
    }
}
```

## JSON-RPC API Details

FortiManager uses JSON-RPC 2.0 protocol:

### Authentication Flow

```json
// Login Request
{
  "id": 1,
  "method": "exec",
  "params": [{
    "url": "/sys/login/user",
    "data": {
      "user": "admin",
      "passwd": "password"
    }
  }],
  "jsonrpc": "2.0"
}

// Login Response
{
  "id": 1,
  "result": {
    "status": {"code": 0, "message": "OK"},
    "session": "abc123xyz"
  }
}
```

### Get Devices Request

```json
{
  "id": 1,
  "method": "get",
  "params": [{
    "url": "/dvmdb/adom/root/device"
  }],
  "session": "abc123xyz",
  "jsonrpc": "2.0"
}
```

## Performance Considerations

### Pagination
FortiManager API supports pagination for large datasets:

```go
// Not currently implemented in SDK
// For large environments, consider adding pagination support
```

### Caching
Consider caching ADOM and device lists:

```go
type CachedService struct {
    service   *fortimanager.ImportService
    adomCache []fortimanager.ADOMInfo
    cacheTTL  time.Duration
    lastFetch time.Time
}

func (cs *CachedService) GetADOMs() ([]fortimanager.ADOMInfo, error) {
    if time.Since(cs.lastFetch) > cs.cacheTTL {
        adoms, err := cs.service.ListADOMs()
        if err != nil {
            return nil, err
        }
        cs.adomCache = adoms
        cs.lastFetch = time.Now()
    }
    return cs.adomCache, nil
}
```

### Parallel Processing
For large environments with multiple ADOMs:

```go
import "sync"

var wg sync.WaitGroup
results := make(chan *fortimanager.ImportSummary, len(adoms))

for _, adom := range adoms {
    wg.Add(1)
    go func(adomName string) {
        defer wg.Done()
        request := fortimanager.ImportRequest{
            ADOMs: []string{adomName},
        }
        summary, _ := service.ImportConfigurations(request)
        results <- summary
    }(adom.Name)
}

wg.Wait()
close(results)
```

## Security

### Credentials
- Store credentials securely (environment variables, secrets manager)
- Use service accounts with minimal required permissions
- Rotate passwords regularly

### TLS
- Always use HTTPS in production
- Only set `InsecureSkipVerify: true` for dev/testing with self-signed certs
- Verify certificate chain in production

### RBAC
FortiManager supports role-based access control:
- Read-only profiles for configuration retrieval
- Specific ADOM access restrictions
- Device group permissions

## Troubleshooting

### Connection Issues

**Problem**: `failed to login to FortiManager`
- Check BaseURL is correct (include https://)
- Verify credentials
- Check network connectivity
- Ensure FortiManager API access is enabled

**Problem**: `Device not connected`
- Device may be offline
- Check FortiManager device monitoring
- Verify device connectivity status in GUI

### API Errors

**Problem**: `FortiManager API error: Permission denied`
- Check user has required permissions for the ADOM
- Verify RBAC profile allows API access

**Problem**: `timeout exceeded`
- Increase `Timeout` in config
- Check FortiManager performance
- Consider reducing batch size

## Comparison: FortiManager vs Direct Device Access

| Aspect | FortiManager | Direct Device (REST API) |
|--------|-------------|--------------------------|
| **Scalability** | Hundreds of devices | Single device |
| **Management** | Centralized | Per-device |
| **Policy Sync** | Yes (policy packages) | Manual |
| **Global Objects** | Yes | No |
| **Offline Devices** | Can retrieve last-known config | Not accessible |
| **Authentication** | Single session | Per-device auth |
| **Use Case** | Large deployments, MSPs | Small deployments, direct access |

## Future Enhancements

Potential improvements for this SDK:

1. **Change Tracking**: Monitor configuration changes over time
2. **Revision History**: Access device configuration history
3. **Bulk Operations**: Install policies to multiple devices
4. **Event Streaming**: Real-time device status updates
5. **Approval Workflows**: Policy package approval automation
6. **Backup/Restore**: Automated configuration backups
7. **Compliance Checking**: Validate configurations against policies

## References

- [FortiManager Administration Guide](https://docs.fortinet.com/product/fortimanager)
- [FortiManager JSON-RPC API](https://fndn.fortinet.net/index.php?/topic/52-fortimanager-json-rpc-api/)
- [FortiGate REST API](https://docs.fortinet.com/document/fortigate/latest/rest-api-reference)
- [ADOM Concepts](https://docs.fortinet.com/document/fortimanager/latest/administration-guide/355734/administrative-domains)

## License

Copyright © 2025 Your Organization. All rights reserved.

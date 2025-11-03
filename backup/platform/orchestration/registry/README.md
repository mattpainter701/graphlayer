# Connector Registry & Factory

Factory pattern for creating and managing connector instances.

## Overview

- **ConnectorFactory**: Creates connectors based on device type
- **ConnectorManager**: Manages multiple connector instances
- Device type to connector type mapping

## ConnectorFactory

Creates appropriate connector (SSH or API) based on device type:

```go
type ConnectorFactory struct {
    deviceTypeRegistry map[types.DeviceType]string
}
```

### Usage

```go
factory := registry.NewConnectorFactory()

// Create connector
connector, _ := factory.CreateConnector(
    types.DeviceTypeCiscoIOS,
    config,
)

// Get supported device types
deviceTypes := factory.GetSupportedDeviceTypes()
```

## Device Type Mapping

### SSH Devices

```go
cf.deviceTypeRegistry[types.DeviceTypeCiscoIOS] = "ssh"
cf.deviceTypeRegistry[types.DeviceTypeCiscoASA] = "ssh"
cf.deviceTypeRegistry[types.DeviceTypeCiscoNXOS] = "ssh"
cf.deviceTypeRegistry[types.DeviceTypePaloAltoPA] = "ssh"
cf.deviceTypeRegistry[types.DeviceTypeJuniperJunos] = "ssh"
cf.deviceTypeRegistry[types.DeviceTypeFortinetFortiGate] = "ssh"
cf.deviceTypeRegistry[types.DeviceTypeCheckPointGaia] = "ssh"
cf.deviceTypeRegistry[types.DeviceTypeF5BigIP] = "ssh"
```

### API Devices

```go
cf.deviceTypeRegistry[types.DeviceTypeCiscoFMC] = "api"
cf.deviceTypeRegistry[types.DeviceTypePaloAltoPanorama] = "api"
cf.deviceTypeRegistry[types.DeviceTypeFortinetFortiManager] = "api"
cf.deviceTypeRegistry[types.DeviceTypeSolarWindsNCM] = "api"
```

## ConnectorManager

Manages multiple connector instances:

```go
type ConnectorManager struct {
    factory    *ConnectorFactory
    connectors map[string]base.Connector // deviceID -> connector
}
```

### Usage

```go
manager := registry.NewConnectorManager()

// Create and register connector
connector, _ := manager.CreateAndRegisterConnector(
    "device-01",
    types.DeviceTypeCiscoIOS,
    config,
)

// Retrieve connector
connector, _ := manager.GetConnector("device-01")

// Remove connector
manager.RemoveConnector("device-01")

// Get all connectors
allConnectors := manager.GetAllConnectors()
```

## Factory Methods

### CreateConnector()

```go
func (cf *ConnectorFactory) CreateConnector(
    deviceType types.DeviceType,
    config *base.ConnectionConfig,
) (base.Connector, error)
```

Creates appropriate connector based on device type.

### GetConnectorTypeForDevice()

```go
func (cf *ConnectorFactory) GetConnectorTypeForDevice(
    deviceType types.DeviceType,
) (string, error)
```

Returns "ssh" or "api" for a given device type.

### GetSupportedDeviceTypes()

```go
func (cf *ConnectorFactory) GetSupportedDeviceTypes() []types.DeviceType
```

Returns list of all supported device types.

## Integration Example

```go
package main

func main() {
    // Create manager
    manager := registry.NewConnectorManager()
    
    // Define devices
    devices := map[string]types.DeviceType{
        "10.1.1.1": types.DeviceTypeCiscoIOS,
        "10.1.1.2": types.DeviceTypeCiscoASA,
        "10.1.1.3": types.DeviceTypePaloAltoPA,
    }
    
    // Create connectors
    for deviceID, deviceType := range devices {
        config := &base.ConnectionConfig{
            Host:     deviceID,
            Port:     22,
            Username: "admin",
            Password: "password",
        }
        
        connector, _ := manager.CreateAndRegisterConnector(
            deviceID,
            deviceType,
            config,
        )
        
        // Use connector
        connector.Connect(ctx)
        result, _ := connector.GetRunningConfig(ctx, 60*time.Second)
        connector.Disconnect(ctx)
    }
}
```

## Related Documentation

- [types/README.md](../types/README.md) - Device type definitions
- [base/README.md](../base/README.md) - Connector interface
- [orchestrator/README.md](../orchestrator/README.md) - Using manager in workflows

---

**Status**: Production Ready  
**Version**: 1.0


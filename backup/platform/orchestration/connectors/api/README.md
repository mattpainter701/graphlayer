# API Connector

REST API connectivity for management systems (Cisco FMC, Panorama, SolarWinds NCM, etc.).

## Overview

Production-ready API connector with:
- RESTful API communication
- Token-based authentication
- Connection pooling
- Rate limiting
- TLS/SSL certificate validation

## Supported Systems

- **Cisco FMC**: Firepower Management Center - Multi-device security management
- **Palo Alto Panorama**: Centralized firewall management - Device groups, templates, policy management
- **Fortinet FortiManager**: FortiGate management platform - ADOM-based device management
- **SolarWinds NCM**: Network Configuration Manager - SWIS API for device inventory and config management
- **Generic API**: Custom REST endpoints for extensibility

## Usage

### Basic Connection

```go
import (
    "github.com/mattpainter701/graphlayer/platform/connectors/api"
    "github.com/mattpainter701/graphlayer/platform/connectors/base"
    "github.com/mattpainter701/graphlayer/platform/connectors/types"
)

config := &base.ConnectionConfig{
    Host:    "https://fmc.example.com",
    APIKey:  "your-api-key",
    Timeout: 30 * time.Second,
    Headers: map[string]string{
        "Content-Type": "application/json",
    },
}

connector, _ := api.NewAPIConnector(config, types.DeviceTypeCiscoFMC)

// Connect
connector.Connect(ctx)
defer connector.Disconnect(ctx)

// Get device inventory
devices, _ := connector.GetDeviceInventory(ctx)
```

### Multi-Device Operations

```go
// Get managed devices
devices, _ := connector.GetManagedDevices(ctx)

// Collect configs from multiple devices
deviceIDs := []string{"device-1", "device-2", "device-3"}
configs, _ := connector.GetDeviceConfigs(ctx, deviceIDs)

for deviceID, config := range configs {
    fmt.Printf("Device %s: %d bytes\n", deviceID, len(config.ConfigData))
}
```

## Configuration

### ConnectionConfig

```go
type ConnectionConfig struct {
    Host       string
    Port       int
    APIKey     string
    Token      string
    CertFile   string
    KeyFileAPI string
    CACert     string
    Headers    map[string]string
    Timeout    time.Duration
}
```

## Features

### Token Management

Automatic token refresh:

```go
// Token automatically refreshed when expired
connector.Connect(ctx)
// ... operations ...
// Token renewed if needed
```

### Rate Limiting

Built-in rate limiting to respect API limits:

```go
// 10 requests per second
rateLimiter := rate.NewLimiter(rate.Limit(10), 10)
```

### TLS/SSL Configuration

Custom certificate validation:

```go
config := &base.ConnectionConfig{
    Host:     "https://api.example.com",
    CACert:   "/path/to/ca.crt",
    CertFile: "/path/to/client.crt",
    KeyFileAPI: "/path/to/client.key",
}
```

## Error Handling

### HTTP Errors

```go
result, err := connector.ExecuteCommand(ctx, "GET /api/devices", 30*time.Second)
if err != nil {
    switch {
    case strings.Contains(err.Error(), "401"):
        // Authentication failed
    case strings.Contains(err.Error(), "429"):
        // Rate limit exceeded
    case strings.Contains(err.Error(), "timeout"):
        // Request timeout
    }
}
```

## Performance

| Operation | Time | Notes |
|-----------|------|-------|
| API Connection | <1s | Token acquisition |
| Device Inventory | 2-5s | Depends on device count |
| Config Collection | 1-3s | Per device |

## Related Documentation

- [base/README.md](../base/README.md) - Connector interface
- [types/README.md](../types/README.md) - Device types
- [orchestrator/README.md](../orchestrator/README.md) - Using in workflows

---

**Status**: Production Ready  
**Version**: 1.0


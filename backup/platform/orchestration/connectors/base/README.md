# Base Connector

Core connector interface and base implementation for all connector types.

## Overview

- **Connector Interface**: Contract that all connectors must implement
- **BaseConnector**: Common functionality and state management
- **ConnectionConfig**: Standard configuration structure

## Connector Interface

All connectors (SSH, API) implement this interface:

```go
type Connector interface {
    // Connection management
    Connect(ctx context.Context) error
    Disconnect(ctx context.Context) error
    IsConnected() bool
    GetConnectionState() ConnectionState

    // Device information
    GetDeviceInfo(ctx context.Context) (*DeviceInfo, error)
    GetSupportedCommands(ctx context.Context) ([]string, error)

    // Command execution
    ExecuteCommand(ctx context.Context, command string, timeout time.Duration) (*CommandResult, error)
    ExecuteCommands(ctx context.Context, commands []string, timeout time.Duration) ([]*CommandResult, error)

    // Configuration retrieval
    GetRunningConfig(ctx context.Context, timeout time.Duration) (*ConfigResult, error)
    GetStartupConfig(ctx context.Context, timeout time.Duration) (*ConfigResult, error)

    // Multi-device operations (for API connectors)
    GetDeviceInventory(ctx context.Context) ([]*DeviceInfo, error)
    GetManagedDevices(ctx context.Context) ([]*DeviceInfo, error)
    GetDeviceConfigs(ctx context.Context, deviceIDs []string) (map[string]*ConfigResult, error)

    // Metadata
    GetType() string
    GetConfig() *ConnectionConfig
}
```

## Data Structures

### ConnectionConfig

```go
type ConnectionConfig struct {
    // Common
    Host         string
    Port         int
    Timeout      time.Duration
    RetryCount   int
    RetryDelay   time.Duration

    // SSH specific
    Username     string
    Password     string
    KeyFile      string
    KeyPassword  string

    // API specific
    APIKey       string
    Token        string
    CertFile     string
    KeyFileAPI   string
    CACert       string
    Headers      map[string]string

    // Options
    Options      map[string]interface{}
}
```

### DeviceInfo

```go
type DeviceInfo struct {
    ID             string
    Hostname       string
    IPAddress      string
    Vendor         string
    Model          string
    OSType         string
    OSVersion      string
    ConnectionType string
    Capabilities   []string
    Tags           map[string]string
}
```

### CommandResult

```go
type CommandResult struct {
    DeviceID   string
    Command    string
    Output     string
    Error      string
    ExitCode   int
    ExecutedAt time.Time
    Duration   time.Duration
}
```

### ConfigResult

```go
type ConfigResult struct {
    DeviceID    string
    ConfigType  string  // "running", "startup", "candidate"
    ConfigData  string
    Hash        string
    RetrievedAt time.Time
    Duration    time.Duration
}
```

## Connection States

```go
const (
    ConnectionStateDisconnected ConnectionState = "disconnected"
    ConnectionStateConnecting   ConnectionState = "connecting"
    ConnectionStateConnected     ConnectionState = "connected"
    ConnectionStateError         ConnectionState = "error"
)
```

## BaseConnector

Common functionality for all connectors:

```go
type BaseConnector struct {
    config       *ConnectionConfig
    state        ConnectionState
    deviceType   types.DeviceType
    workingFolder string
    mu           sync.RWMutex
}
```

### Methods

- Configuration validation
- State management
- Error handling
- Context cancellation support

## Usage Examples

### Implementing a Connector

```go
type MyConnector struct {
    *base.BaseConnector
    client *MyClient
}

func (mc *MyConnector) Connect(ctx context.Context) error {
    // Implementation
}

func (mc *MyConnector) ExecuteCommand(ctx context.Context, command string, timeout time.Duration) (*base.CommandResult, error) {
    // Implementation
}
```

### Using a Connector

```go
var connector base.Connector

// Create appropriate connector
connector, _ = ssh.NewSSHConnector(config, types.DeviceTypeCiscoIOS)
// or
connector, _ = api.NewAPIConnector(config, types.DeviceTypeCiscoFMC)

// Use interface methods
connector.Connect(ctx)
result, _ := connector.GetRunningConfig(ctx, 60*time.Second)
connector.Disconnect(ctx)
```

## Related Documentation

- [ssh/README.md](../ssh/README.md) - SSH connector implementation
- [api/README.md](../api/README.md) - API connector implementation
- [registry/README.md](../registry/README.md) - Connector factory

---

**Status**: Production Ready  
**Version**: 1.0


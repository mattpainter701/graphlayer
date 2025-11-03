# SSH Connector

Direct SSH connectivity to network devices (IOS, ASA, PANOS, Junos, FortiOS, etc.).

## Overview

Production-ready SSH connector with:
- Password and public key authentication
- Interactive shell support (paramiko-style)
- Command execution with timeout handling
- Keep-alive mechanisms
- Device-specific prompt handling

## Supported Devices

- **Cisco**: IOS, ASA, NXOS
- **Palo Alto**: PANOS
- **Juniper**: Junos, SRX
- **Fortinet**: FortiGate
- **Check Point**: Gaia
- **F5**: BigIP
- **Generic**: Any SSH-accessible device

## Usage

### Basic Connection

```go
import (
    "github.com/mattpainter701/graphlayer/platform/connectors/ssh"
    "github.com/mattpainter701/graphlayer/platform/connectors/base"
    "github.com/mattpainter701/graphlayer/platform/connectors/types"
)

config := &base.ConnectionConfig{
    Host:     "10.1.1.1",
    Port:     22,
    Username: "admin",
    Password: "password",
    Timeout:  30 * time.Second,
}

connector, _ := ssh.NewSSHConnector(config, types.DeviceTypeCiscoIOS)

// Connect
connector.Connect(ctx)
defer connector.Disconnect(ctx)

// Execute command
result, _ := connector.ExecuteCommand(ctx, "show version", 10*time.Second)
fmt.Println(result.Output)
```

### Interactive Shell

```go
// Invoke interactive shell (like paramiko)
shell, _ := connector.InvokeShell(ctx)
defer shell.Close()

// Send commands
shell.SendCommand("terminal length 0", 5*time.Second)
output, _ := shell.SendCommand("show running-config", 60*time.Second)

fmt.Println(output)
```

### Public Key Authentication

```go
config := &base.ConnectionConfig{
    Host:        "10.1.1.1",
    Port:        22,
    Username:    "admin",
    KeyFile:     "/path/to/private_key",
    KeyPassword: "keypassword", // optional
}

connector, _ := ssh.NewSSHConnector(config, types.DeviceTypeCiscoIOS)
```

## Configuration

### ConnectionConfig

```go
type ConnectionConfig struct {
    Host         string
    Port         int
    Username     string
    Password     string
    KeyFile      string
    KeyPassword  string
    Timeout      time.Duration
    RetryCount   int
    RetryDelay   time.Duration
}
```

### Device-Specific Prompts

The connector automatically handles device-specific prompts:

```go
// Cisco IOS
"Switch>", "Switch#"

// Palo Alto
"admin@PA-220>", "admin@PA-220#"

// Juniper
"user@router>", "user@router#"

// Fortinet
"FortiGate #"
```

## Features

### Keep-Alive

Automatic keep-alive packets to prevent connection timeout:

```go
connector.SetKeepAlive(30 * time.Second) // Send keep-alive every 30s
```

### Command Timeout

Per-command timeout handling:

```go
// Long-running command
result, err := connector.ExecuteCommand(ctx, "show tech-support", 120*time.Second)

// Short command
result, err := connector.ExecuteCommand(ctx, "show clock", 5*time.Second)
```

### Retry Mechanism

Automatic retry with exponential backoff:

```go
config := &base.ConnectionConfig{
    RetryCount: 3,
    RetryDelay: 5 * time.Second,
    // ...
}
```

## Error Handling

### Connection Errors

```go
connector, err := ssh.NewSSHConnector(config, types.DeviceTypeCiscoIOS)
if err != nil {
    // Configuration validation failed
}

err = connector.Connect(ctx)
if err != nil {
    switch {
    case strings.Contains(err.Error(), "authentication"):
        // Handle auth failure
    case strings.Contains(err.Error(), "timeout"):
        // Handle timeout
    default:
        // Handle other errors
    }
}
```

### Command Execution Errors

```go
result, err := connector.ExecuteCommand(ctx, "show running-config", 60*time.Second)
if err != nil {
    log.Printf("Command failed: %v", err)
}

// Check output for errors
if strings.Contains(result.Output, "% Invalid input") {
    log.Println("Invalid command syntax")
}
```

## Performance

| Operation | Time | Notes |
|-----------|------|-------|
| SSH Connection | 1-2s | Including handshake |
| Command Execution | 0.5-5s | Depends on command |
| Config Collection | 2-10s | Running config |

## Related Documentation

- [base/README.md](../base/README.md) - Connector interface
- [types/README.md](../types/README.md) - Device types
- [orchestrator/README.md](../orchestrator/README.md) - Using in workflows

---

**Status**: Production Ready  
**Version**: 1.0


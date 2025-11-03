# Connector Orchestrator

High-level workflow coordination for device configuration collection and operational data gathering.

## Overview

The `ConnectorOrchestrator` is the central component that:
- Coordinates SSH/API connector operations
- Manages file storage and change detection
- Executes auxiliary commands with intelligent skipping
- Provides capability detection and conditional execution
- Tracks execution statistics and errors

## Key Components

### ConnectorOrchestrator

Main orchestration service:

```go
type ConnectorOrchestrator struct {
    manager            *registry.ConnectorManager
    storage            *storage.WorkspaceManager
    credService        CredentialService
    taskService        TaskService
    capabilityCache    map[string]*DeviceCapabilities
}
```

### ConnectorJob

Job configuration for collection tasks:

```go
type ConnectorJob struct {
    GUID                 string
    Name                 string
    DeviceType           types.DeviceType
    Config               *base.ConnectionConfig
    Devices              []string
    Workspaces           []string
    TaskGUID             string
    Enabled              bool
    AuxDataEnabled       bool                // Collect operational commands
    ConditionalExecution bool                // Skip unsupported commands
    CapabilityDetection  bool                // Auto-detect device features
}
```

### CollectionResult

Result of a collection operation:

```go
type CollectionResult struct {
    ConnectorGUID      string
    ConfigFiles        []string
    AuxiliaryFiles     []string
    Errors             []string
    Success            bool
    Duration           time.Duration
    CommandResults     []CommandExecutionResult
    Summary            *CommandExecutionSummary
}
```

## Core Operations

### ExecutePull()

Main collection workflow:

```go
func (co *ConnectorOrchestrator) ExecutePull(
    ctx context.Context,
    job *ConnectorJob,
) (*CollectionResult, error)
```

**Steps:**
1. Create working folder
2. Create and connect to device
3. Collect running configuration
4. Collect startup configuration (if supported)
5. Execute auxiliary commands (if enabled)
6. Disconnect and cleanup

**Example:**
```go
orch, _ := orchestrator.NewConnectorOrchestrator("/mount/connector", credService, taskService)

job := &orchestrator.ConnectorJob{
    GUID:       "conn-01",
    DeviceType: types.DeviceTypeCiscoIOS,
    Config: &base.ConnectionConfig{
        Host: "10.1.1.1",
        Port: 22,
        Username: "admin",
        Password: "password",
    },
    AuxDataEnabled: true,
}

result, err := orch.ExecutePull(context.Background(), job)
```

### GetFilesToCommit()

Change detection using MD5 hashes:

```go
func (co *ConnectorOrchestrator) GetFilesToCommit(
    job *ConnectorJob,
    workspace string,
) ([]string, error)
```

**Returns:** List of files that have changed since last commit.

**Example:**
```go
filesToCommit, _ := orch.GetFilesToCommit(job, "production")
if len(filesToCommit) > 0 {
    fmt.Printf("Changed files: %d\n", len(filesToCommit))
}
```

### TrackCommittedFiles()

Update tracking after Git commit:

```go
func (co *ConnectorOrchestrator) TrackCommittedFiles(
    job *ConnectorJob,
    workspace string,
    files []string,
) error
```

**Example:**
```go
// After committing to Git
orch.TrackCommittedFiles(job, "production", filesToCommit)
```

## Auxiliary Commands

### Command Execution

When `AuxDataEnabled` is true, the orchestrator executes operational commands:

#### Cisco IOS Commands (14 total)
- **P0 Critical (11 commands)**: L2 topology and adjacency
  - `show mac address-table dynamic`, `show lldp neighbors detail`, `show cdp neighbors detail`
  - `show vlan`, `show interfaces status`, `show interfaces switchport`, `show interfaces trunk`
  - `show etherchannel summary`, `show spanning-tree detail`
  - `show ip arp`, `show ip interface brief`

- **P1 Security (3 commands)**: Security controls and compliance
  - `show ip dhcp snooping binding`, `show ip arp inspection`
  - `show port-security`

#### Cisco NXOS Commands (17 total) ✨ **NEW**
- **P0 Critical (13 commands)**: L2/L3 topology + VPC
  - `show mac address-table`, `show vlan`, `show interface status`, `show interface switchport`
  - `show interface trunk`, `show port-channel summary`, `show spanning-tree summary`
  - `show lldp neighbors detail`, `show cdp neighbors detail`
  - `show ip arp`, `show ip route`
  - **VPC**: `show vpc`, `show vpc peer-keepalive` (critical for dual-homed topology)

- **P1 Advanced (4 commands)**: VDC and FabricPath
  - `show vpc consistency-parameters global`
  - `show vdc` (Nexus 7K Virtual Device Contexts)
  - `show fabricpath topology` (FabricPath IS-IS)
  - `show hsrp` (HSRP status)

#### Cisco ASA Commands (13 total) ✨ **NEW**
- **P0 Critical (8 commands)**: Firewall topology and NAT
  - `show interface`, `show nameif`, `show route`, `show arp`
  - `show xlate`, `show nat`, `show conn`, `show local-host`

- **P1 High (5 commands)**: Security policies and HA
  - `show access-list`, `show service-policy`
  - `show failover`, `show vpn-sessiondb`, `show object-group`

### Conditional Execution

Smart command skipping based on device capabilities:

```go
job := &orchestrator.ConnectorJob{
    AuxDataEnabled:       true,
    ConditionalExecution: true,  // Enable smart skipping
    CapabilityDetection:  true,  // Auto-detect features
    // ...
}
```

**Benefits:**
- 40-56% faster execution on constrained devices
- Reduces device CPU load
- Avoids error noise from unsupported commands

### Capability Detection

Auto-detects device features:

```go
type DeviceCapabilities struct {
    DeviceID          string
    DetectedAt        time.Time
    IOSVersion        string
    Features          map[string]bool        // lldp, cdp, dhcp_snooping, etc.
    SupportedCommands map[string]bool
    Metadata          map[string]string
}
```

**Detected Features:**
- LLDP enabled/disabled
- CDP enabled/disabled
- DHCP Snooping configured
- Port Security configured
- 802.1X/MAB available
- IGMP Snooping enabled

**Cache:**
- Results cached for 1 hour
- Reduces repeated detection overhead

## Error Handling

### Error Categorization

8 failure types for intelligent error handling:

```go
const (
    FailureTypeTimeout          // Command timeout
    FailureTypeAuthentication   // Auth failure
    FailureTypePermission        // Permission denied
    FailureTypeUnsupported       // Command not supported
    FailureTypeFeatureDisabled   // Feature disabled
    FailureTypeInvalidSyntax     // Invalid command
    FailureTypeDeviceError       // Device error
    FailureTypeUnknown           // Unknown error
)
```

### Graceful Degradation

Non-critical failures logged as INFO:

```go
[INFO] Command 'show lldp neighbors detail' skipped: LLDP not enabled
[INFO] Command 'show ip dhcp snooping binding' not available: DHCP Snooping disabled
```

Critical failures logged as ERROR:

```go
[ERROR] Command 'show running-config' failed: Permission denied
[ERROR] Connection timeout after 30s
```

### Command Execution Summary

Detailed statistics per execution:

```go
type CommandExecutionSummary struct {
    TotalCommands      int
    SuccessfulCommands int
    FailedCommands     int
    SkippedCommands    int
    ExecutionTime      time.Duration
    ByPriority         map[string]CommandStats  // p0, p1, p2
    ByFailureType      map[CommandFailureType]int
}
```

**Example Output:**
```
Command Execution Summary:
  Total:      14 commands
  Successful: 12
  Failed:     0
  Skipped:    2
  Duration:   8.3s

  By Priority:
    p0: 11 total, 10 success, 0 failed
    p1: 3 total, 2 success, 0 failed
```

## File Organization

### Output Structure

```
/mount/connector/
  {conn-guid}/
    {host}_running_config.txt
    {host}_startup_config.txt
    {host}_show_mac_address_table_dynamic.txt
    {host}_show_lldp_neighbors_detail.txt
    {host}_show_spanning_tree_detail.txt
    ...
  workspaces/
    {workspace}/
      uploaded_files.json  ← MD5 tracking
```

### Change Detection

`uploaded_files.json` structure:

```json
{
  "10.1.1.1_running_config.txt": {
    "hash": "md5:abc123...",
    "last_modified": "2025-10-26T10:30:00Z"
  }
}
```

## Performance

| Operation | Time | Notes |
|-----------|------|-------|
| Single Device | 3-8s | Config + aux commands |
| Capability Detection | 1-2s | Cached for 1 hour |
| Change Detection | <100ms | MD5 comparison |
| Conditional Execution | 40-56% faster | vs. running all commands |

## Best Practices

### Job Configuration
```go
// Production configuration
job := &orchestrator.ConnectorJob{
    GUID:                 generateGUID(),
    DeviceType:           types.DeviceTypeCiscoIOS,
    AuxDataEnabled:       true,   // Full topology data
    ConditionalExecution: true,   // Smart skipping
    CapabilityDetection:  true,   // Auto-detect
    Config: &base.ConnectionConfig{
        Timeout:    60 * time.Second,
        RetryCount: 3,
        RetryDelay: 5 * time.Second,
    },
    Workspaces: []string{"production"},
}
```

### Error Handling
```go
result, err := orch.ExecutePull(ctx, job)
if err != nil {
    log.Fatalf("Collection failed: %v", err)
}

// Check for partial success
if len(result.Errors) > 0 {
    for _, errMsg := range result.Errors {
        if strings.Contains(errMsg, "[ERROR]") {
            log.Printf("Critical error: %s", errMsg)
        }
    }
}
```

### Change Detection Workflow
```go
// Collect
result, _ := orch.ExecutePull(ctx, job)

// Check for changes
filesToCommit, _ := orch.GetFilesToCommit(job, "production")

if len(filesToCommit) == 0 {
    log.Println("No changes detected")
    return
}

// Commit to Git
commitToGit(filesToCommit)

// Track committed files
orch.TrackCommittedFiles(job, "production", filesToCommit)
```

## Testing

### Unit Tests

Test files:
- `error_handling_test.go` - Error categorization tests
- `conditional_execution_test.go` - Conditional execution tests

Run tests:
```bash
go test -v ./connectors/orchestrator/...
```

## Related Documentation

- [Main README](../README.md) - Framework overview
- [USE_CASES.md](../USE_CASES.md) - Practical examples
- [INTEGRATION_GUIDE.md](../INTEGRATION_GUIDE.md) - Integration patterns
- [storage/README.md](../storage/README.md) - File storage details
- [types/README.md](../types/README.md) - Device types and commands

---

**Status**: Production Ready  
**Version**: 1.0


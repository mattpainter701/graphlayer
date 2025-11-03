# Connector Examples

Production-ready examples for common connector workflows.

## Available Examples

### enterprise_workflow.go

Complete end-to-end examples:

1. **EnterpriseWorkflowExample()** - Full collection and parsing workflow
2. **MultiDeviceCollectionExample()** - Parallel device collection
3. **ScheduledCollectionExample()** - Automated scheduled backups

## Running Examples

### Basic Usage

```bash
cd connectors/examples
go run enterprise_workflow.go
```

### With Custom Configuration

```go
package main

import "github.com/mattpainter701/graphlayer/platform/connectors/examples"

func main() {
    // Run enterprise workflow
    examples.EnterpriseWorkflowExample()
}
```

## Example 1: Enterprise Workflow

**File:** `enterprise_workflow.go`  
**Function:** `EnterpriseWorkflowExample()`

Complete workflow with:
- Service initialization
- Configuration collection
- Change detection
- Parser integration
- MongoDB persistence

**Features:**
- ✅ AES-256 credential encryption
- ✅ MD5 change detection
- ✅ Task tracking
- ✅ Error handling
- ✅ L2 model population

## Example 2: Multi-Device Collection

**File:** `enterprise_workflow.go`  
**Function:** `MultiDeviceCollectionExample()`

Parallel collection from multiple devices:
- Goroutine-based concurrency
- Synchronization with WaitGroup
- Result aggregation
- Error tracking per device

**Performance:**
- 10 devices collected in ~8-10 seconds
- Automatic retry on failures
- Rate limiting support

## Example 3: Scheduled Collection

**File:** `enterprise_workflow.go`  
**Function:** `ScheduledCollectionExample()`

Automated nightly backups:
- Ticker-based scheduling
- Change detection
- Git integration
- Status reporting

**Use Cases:**
- Nightly configuration backups
- Continuous compliance monitoring
- Change tracking and alerting

## Customization

### Modify Configuration

```go
job := &orchestrator.ConnectorJob{
    GUID:                 "your-guid",
    DeviceType:           types.DeviceTypeCiscoIOS,
    AuxDataEnabled:       true,   // Your choice
    ConditionalExecution: true,   // Your choice
    Config: &base.ConnectionConfig{
        Host:     "your-host",
        Port:     22,
        Username: "your-username",
        Password: "your-password",
        Timeout:  60 * time.Second,
    },
    Workspaces: []string{"production"},
}
```

### Add Custom Parsers

```go
parserBridge := integration.NewParserBridge(orch)

parserBridge.RegisterParser("cisco_ios", func(filePath string) error {
    // Your custom parser logic
    return yourParser.Parse(filePath)
})
```

### Add Git Integration

```go
filesToCommit, _ := orch.GetFilesToCommit(job, "production")

if len(filesToCommit) > 0 {
    // Your Git integration
    gitCommit(filesToCommit, "Automated backup")
    orch.TrackCommittedFiles(job, "production", filesToCommit)
}
```

## Additional Examples

For more examples, see:
- [USE_CASES.md](../USE_CASES.md) - Comprehensive use case examples
- [INTEGRATION_GUIDE.md](../INTEGRATION_GUIDE.md) - Integration patterns

## Example Output

### Successful Collection

```
🚀 Starting Enterprise Workflow Example...

📥 Collecting configuration from 10.1.1.1...
✅ Collection complete (14 files in 7.2s)

📊 Command Execution Summary:
  Total:      14 commands
  Successful: 12
  Skipped:    2
  Duration:   7.2s

🔍 Checking for changes...
📝 3 files changed - committing

✅ Workflow complete!
```

### Multi-Device Collection

```
🚀 Multi-Device Collection Example...

✅ Device 10.1.1.1 collected (15 files)
✅ Device 10.1.1.2 collected (15 files)
✅ Device 10.1.1.3 collected (15 files)
...

🎉 Total: 150 files collected from 10 devices in 9.8s
```

## Related Documentation

- [Main README](../README.md) - Framework overview
- [USE_CASES.md](../USE_CASES.md) - Detailed use cases
- [INTEGRATION_GUIDE.md](../INTEGRATION_GUIDE.md) - Integration patterns

---

**Status**: Production Ready  
**Version**: 1.0


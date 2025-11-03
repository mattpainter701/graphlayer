# Parser Integration Bridge

Bridge between connector framework and existing parsers for seamless integration.

## Overview

The `ParserBridge` provides automatic routing of collected configurations and operational data to your existing parsers.

## ParserBridge

```go
type ParserBridge struct {
    orchestrator *orchestrator.ConnectorOrchestrator
}
```

### Usage

```go
import "github.com/mattpainter701/graphlayer/platform/connectors/integration"

// Create bridge
parserBridge := integration.NewParserBridge(orchestrator)

// Parse collection result
result, _ := orch.ExecutePull(ctx, job)
err := parserBridge.ParseCollectionResult(ctx, result)
```

## File Routing

### Automatic Routing

The bridge automatically routes files to appropriate parsers:

**Configuration Files:**
- `*_running_config.txt` → Config parser
- `*_startup_config.txt` → Config parser

**Operational Data:**
- `*_show_*.txt` → Operational data parser

### Custom Routing

Register custom parsers:

```go
parserBridge.RegisterParser("cisco_ios", func(filePath string) error {
    processor := cisco.NewCiscoIOSProcessor(
        filePath,
        store,
        messageBroker,
        logger,
        messageHelper,
    )
    return processor.ProcessIOSFileEnhanced()
})
```

## Integration with Existing Parsers

### Cisco IOS Processor

```go
func parseConfigFile(filePath string) error {
    processor := cisco.NewCiscoIOSProcessor(
        filePath,
        mongoStore,      // Your MongoDB store
        messageBroker,   // Your message broker
        logger,          // Your logger
        messageHelper,   // Your helper
    )
    
    // Parse config + build L2 models
    return processor.ProcessIOSFileEnhanced()
}
```

### Complete Workflow

```go
package main

func main() {
    // 1. Collect configurations
    orch, _ := orchestrator.NewConnectorOrchestrator("/mount/connector", credService, taskService)
    
    job := &orchestrator.ConnectorJob{
        GUID:           "conn-01",
        DeviceType:     types.DeviceTypeCiscoIOS,
        AuxDataEnabled: true,
        Config: &base.ConnectionConfig{
            Host:     "10.1.1.1",
            Port:     22,
            Username: "admin",
            Password: "password",
        },
    }
    
    result, _ := orch.ExecutePull(context.Background(), job)
    
    // 2. Parse automatically
    parserBridge := integration.NewParserBridge(orch)
    if err := parserBridge.ParseCollectionResult(ctx, result); err != nil {
        log.Fatalf("Parsing failed: %v", err)
    }
    
    // 3. Models automatically persisted to MongoDB by your processor
    fmt.Println("✅ Configuration collected and parsed")
}
```

## File Format Expectations

### Configuration Files

Expected format:
```
/mount/connector/{guid}/{hostname}_running_config.txt
/mount/connector/{guid}/{hostname}_startup_config.txt
```

### Operational Data Files

Expected format:
```
/mount/connector/{guid}/{hostname}_show_mac_address_table_dynamic.txt
/mount/connector/{guid}/{hostname}_show_lldp_neighbors_detail.txt
/mount/connector/{guid}/{hostname}_show_spanning_tree_detail.txt
...
```

## Model Population

### L2 Models

Files automatically populate L2 models:

| File | Populates |
|------|-----------|
| `show_mac_address_table_dynamic.txt` | `FDBEntry` |
| `show_lldp_neighbors_detail.txt` | `L2Link` |
| `show_spanning_tree_detail.txt` | `L2SwitchPort.STPRole` |
| `show_interfaces_switchport.txt` | `L2SwitchPort` |
| `show_vlan.txt` | `L2VLAN` |
| `show_ip_dhcp_snooping_binding.txt` | `L2GlobalControls` |

### Configuration Hash

MD5 hash from orchestrator flows to models:

```
Orchestrator → MD5 hash
    ↓
Parser reads file
    ↓
CiscoL2Config.ConfigHash = md5:abc123...
    ↓
L2Switch.ConfigHash = md5:abc123...
```

## Error Handling

```go
err := parserBridge.ParseCollectionResult(ctx, result)
if err != nil {
    switch {
    case strings.Contains(err.Error(), "parser not found"):
        // No parser registered for device type
    case strings.Contains(err.Error(), "invalid format"):
        // File format error
    default:
        // Other errors
    }
}
```

## Related Documentation

- [orchestrator/README.md](../orchestrator/README.md) - Collection workflow
- [INTEGRATION_GUIDE.md](../INTEGRATION_GUIDE.md) - Complete integration guide
- [suggested_modeling/](../../suggested_modeling/) - L2 model definitions

---

**Status**: Production Ready  
**Version**: 1.0


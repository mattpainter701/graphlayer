# Connector Framework - Use Cases & Examples

Practical examples for common network configuration collection scenarios.

## Table of Contents
1. [Single Device Collection](#single-device-collection)
2. [Multi-Device Parallel Collection](#multi-device-parallel-collection)
3. [Scheduled Configuration Backups](#scheduled-configuration-backups)
4. [Change Detection & Git Integration](#change-detection--git-integration)
5. [Layer-2 Topology Discovery](#layer-2-topology-discovery)
6. [Multi-Workspace Management](#multi-workspace-management)
7. [Error Handling & Recovery](#error-handling--recovery)
8. [Integration with Parsers](#integration-with-parsers)

---

## Single Device Collection

**Scenario**: Collect running configuration from a single Cisco IOS switch.

```go
package main

import (
    "context"
    "fmt"
    "time"
    
    "github.com/mattpainter701/graphlayer/platform/connectors/orchestrator"
    "github.com/mattpainter701/graphlayer/platform/connectors/services"
    "github.com/mattpainter701/graphlayer/platform/connectors/types"
    "github.com/mattpainter701/graphlayer/platform/connectors/base"
)

func main() {
    ctx := context.Background()
    
    // Initialize services
    credService, _ := services.NewCredentialService("your-32-byte-encryption-key-here")
    taskService := services.NewTaskService()
    orch, _ := orchestrator.NewConnectorOrchestrator("/mount/connector", credService, taskService)
    
    // Create job
    job := &orchestrator.ConnectorJob{
        GUID:       "conn-core-sw-01",
        Name:       "Core Switch 01",
        DeviceType: types.DeviceTypeCiscoIOS,
        Config: &base.ConnectionConfig{
            Host:     "10.1.1.1",
            Port:     22,
            Username: "admin",
            Password: "password",
            Timeout:  30 * time.Second,
        },
        AuxDataEnabled: false, // Only collect running config
    }
    
    // Execute collection
    result, err := orch.ExecutePull(ctx, job)
    if err != nil {
        fmt.Printf("Collection failed: %v\n", err)
        return
    }
    
    fmt.Printf("✅ Success! Collected %d files in %v\n", 
        len(result.ConfigFiles), result.Duration)
    for _, file := range result.ConfigFiles {
        fmt.Printf("  - %s\n", file)
    }
}
```

**Output:**
```
✅ Success! Collected 1 files in 3.2s
  - /mount/connector/conn-core-sw-01/10.1.1.1_running_config.txt
```

---

## Multi-Device Parallel Collection

**Scenario**: Collect configurations from 10 switches simultaneously using goroutines.

```go
func collectMultipleDevices() {
    ctx := context.Background()
    
    orch, _ := orchestrator.NewConnectorOrchestrator("/mount/connector", credService, taskService)
    
    devices := []string{
        "10.1.1.1", "10.1.1.2", "10.1.1.3", "10.1.1.4", "10.1.1.5",
        "10.1.1.6", "10.1.1.7", "10.1.1.8", "10.1.1.9", "10.1.1.10",
    }
    
    var wg sync.WaitGroup
    results := make(chan *orchestrator.CollectionResult, len(devices))
    
    for i, host := range devices {
        wg.Add(1)
        go func(deviceNum int, deviceHost string) {
            defer wg.Done()
            
            job := &orchestrator.ConnectorJob{
                GUID:       fmt.Sprintf("conn-sw-%02d", deviceNum),
                DeviceType: types.DeviceTypeCiscoIOS,
                Config: &base.ConnectionConfig{
                    Host:     deviceHost,
                    Port:     22,
                    Username: "admin",
                    Password: "password",
                    Timeout:  30 * time.Second,
                },
                AuxDataEnabled: true, // Collect operational commands too
            }
            
            result, err := orch.ExecutePull(ctx, job)
            if err != nil {
                fmt.Printf("❌ Device %s failed: %v\n", deviceHost, err)
                return
            }
            
            results <- result
            fmt.Printf("✅ Device %s collected (%d files)\n", deviceHost, len(result.ConfigFiles))
        }(i+1, host)
    }
    
    wg.Wait()
    close(results)
    
    // Process results
    totalFiles := 0
    for result := range results {
        totalFiles += len(result.ConfigFiles)
    }
    
    fmt.Printf("\n🎉 Total: %d files collected from %d devices\n", totalFiles, len(devices))
}
```

**Output:**
```
✅ Device 10.1.1.1 collected (15 files)
✅ Device 10.1.1.3 collected (15 files)
✅ Device 10.1.1.2 collected (15 files)
...
🎉 Total: 150 files collected from 10 devices
```

---

## Scheduled Configuration Backups

**Scenario**: Nightly configuration backups with change detection.

```go
func scheduledBackup() {
    orch, _ := orchestrator.NewConnectorOrchestrator("/mount/connector", credService, taskService)
    
    // Run every night at 2 AM
    ticker := time.NewTicker(24 * time.Hour)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            fmt.Printf("⏰ Starting scheduled backup at %s\n", time.Now().Format(time.RFC3339))
            
            job := &orchestrator.ConnectorJob{
                GUID:       "scheduled-backup-01",
                DeviceType: types.DeviceTypeCiscoIOS,
                Config: &base.ConnectionConfig{
                    Host:     "10.1.1.1",
                    Port:     22,
                    Username: "admin",
                    Password: "password",
                },
                Workspaces: []string{"production"},
            }
            
            // Collect configurations
            result, err := orch.ExecutePull(context.Background(), job)
            if err != nil {
                fmt.Printf("❌ Backup failed: %v\n", err)
                continue
            }
            
            // Check for changes
            filesToCommit, err := orch.GetFilesToCommit(job, "production")
            if err != nil {
                fmt.Printf("❌ Change detection failed: %v\n", err)
                continue
            }
            
            if len(filesToCommit) == 0 {
                fmt.Println("✅ No changes detected")
            } else {
                fmt.Printf("📝 %d files changed - committing to Git\n", len(filesToCommit))
                
                // Commit to Git (your implementation)
                commitToGit(filesToCommit)
                
                // Track committed files
                orch.TrackCommittedFiles(job, "production", filesToCommit)
            }
        }
    }
}
```

---

## Change Detection & Git Integration

**Scenario**: Detect configuration changes and commit only modified files to Git.

```go
func changeDetectionWorkflow() {
    ctx := context.Background()
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
        Workspaces: []string{"production", "staging"},
    }
    
    // Step 1: Collect configurations
    fmt.Println("📥 Collecting configurations...")
    result, _ := orch.ExecutePull(ctx, job)
    
    // Step 2: Check each workspace for changes
    for _, workspace := range job.Workspaces {
        fmt.Printf("\n🔍 Checking workspace: %s\n", workspace)
        
        filesToCommit, _ := orch.GetFilesToCommit(job, workspace)
        
        if len(filesToCommit) == 0 {
            fmt.Println("  ✅ No changes")
            continue
        }
        
        fmt.Printf("  📝 %d files changed:\n", len(filesToCommit))
        for _, file := range filesToCommit {
            fmt.Printf("    - %s\n", filepath.Base(file))
        }
        
        // Step 3: Commit to Git
        commitMessage := fmt.Sprintf("Config update from %s at %s", 
            job.Config.Host, time.Now().Format(time.RFC3339))
        
        if err := gitCommit(workspace, filesToCommit, commitMessage); err != nil {
            fmt.Printf("  ❌ Git commit failed: %v\n", err)
            continue
        }
        
        // Step 4: Track committed files
        orch.TrackCommittedFiles(job, workspace, filesToCommit)
        fmt.Println("  ✅ Committed to Git")
    }
}

func gitCommit(workspace string, files []string, message string) error {
    // Your Git implementation
    // git add files...
    // git commit -m message
    // git push origin main
    return nil
}
```

---

## Layer-2 Topology Discovery

**Scenario**: Collect comprehensive L2 data for topology building and security analysis.

```go
func layer2TopologyCollection() {
    ctx := context.Background()
    orch, _ := orchestrator.NewConnectorOrchestrator("/mount/connector", credService, taskService)
    
    job := &orchestrator.ConnectorJob{
        GUID:                 "l2-discovery-01",
        Name:                 "L2 Topology Discovery",
        DeviceType:           types.DeviceTypeCiscoIOS,
        AuxDataEnabled:       true,  // Collect all 14 operational commands
        ConditionalExecution: true,  // Skip unsupported commands
        CapabilityDetection:  true,  // Auto-detect device features
        Config: &base.ConnectionConfig{
            Host:     "10.1.1.1",
            Port:     22,
            Username: "admin",
            Password: "password",
            Timeout:  60 * time.Second,
        },
    }
    
    fmt.Println("🔍 Starting L2 topology discovery...")
    result, _ := orch.ExecutePull(ctx, job)
    
    // Check command execution summary
    if result.Summary != nil {
        fmt.Printf("\n📊 Command Execution Summary:\n")
        fmt.Printf("  Total:      %d commands\n", result.Summary.TotalCommands)
        fmt.Printf("  Successful: %d\n", result.Summary.SuccessfulCommands)
        fmt.Printf("  Failed:     %d\n", result.Summary.FailedCommands)
        fmt.Printf("  Skipped:    %d (conditional execution)\n", result.Summary.SkippedCommands)
        fmt.Printf("  Duration:   %v\n", result.Summary.ExecutionTime)
        
        // Priority breakdown
        fmt.Println("\n  By Priority:")
        for priority, stats := range result.Summary.ByPriority {
            fmt.Printf("    %s: %d total, %d success, %d failed\n", 
                priority, stats.Total, stats.Successful, stats.Failed)
        }
    }
    
    // Files collected for L2 analysis
    fmt.Println("\n📁 Files ready for parser:")
    for _, file := range result.ConfigFiles {
        fmt.Printf("  - %s\n", filepath.Base(file))
    }
}
```

**Output:**
```
🔍 Starting L2 topology discovery...

📊 Command Execution Summary:
  Total:      14 commands
  Successful: 12
  Failed:     0
  Skipped:    2 (conditional execution)
  Duration:   8.3s

  By Priority:
    p0: 11 total, 10 success, 0 failed
    p1: 3 total, 2 success, 0 failed

📁 Files ready for parser:
  - 10.1.1.1_running_config.txt
  - 10.1.1.1_show_mac_address_table_dynamic.txt
  - 10.1.1.1_show_lldp_neighbors_detail.txt
  - 10.1.1.1_show_spanning_tree_detail.txt
  - 10.1.1.1_show_interfaces_switchport.txt
  ...
```

---

## Multi-Workspace Management

**Scenario**: Collect configs for different environments (prod/staging/dev).

```go
func multiWorkspaceCollection() {
    ctx := context.Background()
    orch, _ := orchestrator.NewConnectorOrchestrator("/mount/connector", credService, taskService)
    
    devices := map[string][]string{
        "production": {"10.1.1.1", "10.1.1.2"},
        "staging":    {"10.2.1.1", "10.2.1.2"},
        "dev":        {"10.3.1.1"},
    }
    
    for workspace, hosts := range devices {
        fmt.Printf("\n🔧 Workspace: %s\n", workspace)
        
        for _, host := range hosts {
            job := &orchestrator.ConnectorJob{
                GUID:       fmt.Sprintf("conn-%s", host),
                DeviceType: types.DeviceTypeCiscoIOS,
                Workspaces: []string{workspace},
                Config: &base.ConnectionConfig{
                    Host:     host,
                    Port:     22,
                    Username: "admin",
                    Password: "password",
                },
            }
            
            result, err := orch.ExecutePull(ctx, job)
            if err != nil {
                fmt.Printf("  ❌ %s failed: %v\n", host, err)
                continue
            }
            
            fmt.Printf("  ✅ %s collected\n", host)
            
            // Commit only changes
            filesToCommit, _ := orch.GetFilesToCommit(job, workspace)
            if len(filesToCommit) > 0 {
                orch.TrackCommittedFiles(job, workspace, filesToCommit)
                fmt.Printf("     📝 %d files changed\n", len(filesToCommit))
            }
        }
    }
}
```

---

## Error Handling & Recovery

**Scenario**: Robust error handling with retry logic.

```go
func robustCollection() {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
    defer cancel()
    
    orch, _ := orchestrator.NewConnectorOrchestrator("/mount/connector", credService, taskService)
    
    job := &orchestrator.ConnectorJob{
        GUID:       "conn-01",
        DeviceType: types.DeviceTypeCiscoIOS,
        Config: &base.ConnectionConfig{
            Host:       "10.1.1.1",
            Port:       22,
            Username:   "admin",
            Password:   "password",
            Timeout:    30 * time.Second,
            RetryCount: 3,
            RetryDelay: 5 * time.Second,
        },
    }
    
    // Try collection with retry
    var result *orchestrator.CollectionResult
    var err error
    
    for attempt := 1; attempt <= 3; attempt++ {
        fmt.Printf("🔄 Attempt %d...\n", attempt)
        
        result, err = orch.ExecutePull(ctx, job)
        if err == nil {
            fmt.Println("✅ Success!")
            break
        }
        
        fmt.Printf("❌ Attempt %d failed: %v\n", attempt, err)
        
        if attempt < 3 {
            time.Sleep(5 * time.Second)
        }
    }
    
    if err != nil {
        fmt.Printf("🚫 All attempts failed\n")
        return
    }
    
    // Check for partial success
    if len(result.Errors) > 0 {
        fmt.Printf("\n⚠️  Partial success with errors:\n")
        for _, errMsg := range result.Errors {
            fmt.Printf("  - %s\n", errMsg)
        }
    }
    
    fmt.Printf("\n✅ Collected %d files\n", len(result.ConfigFiles))
}
```

---

## Integration with Parsers

**Scenario**: Collect configs and parse into L2 models.

```go
func collectAndParse() {
    ctx := context.Background()
    
    // Step 1: Collect configurations
    orch, _ := orchestrator.NewConnectorOrchestrator("/mount/connector", credService, taskService)
    
    job := &orchestrator.ConnectorJob{
        GUID:           "conn-core-01",
        DeviceType:     types.DeviceTypeCiscoIOS,
        AuxDataEnabled: true,
        Config: &base.ConnectionConfig{
            Host:     "10.1.1.1",
            Port:     22,
            Username: "admin",
            Password: "password",
        },
    }
    
    result, _ := orch.ExecutePull(ctx, job)
    
    // Step 2: Parse with your existing Cisco IOS processor
    for _, filePath := range result.ConfigFiles {
        // Your existing parser integration
        processor := cisco.NewCiscoIOSProcessor(
            filePath,
            store,              // MongoDB store
            messageBroker,      // Message broker
            logger,             // Logger
            messageHelper,      // Helper
        )
        
        if strings.Contains(filePath, "running_config") {
            // Parse config + build L2 models
            processor.ProcessIOSFileEnhanced()
            fmt.Printf("✅ Parsed: %s\n", filepath.Base(filePath))
        }
    }
    
    fmt.Println("\n🎉 Collection and parsing complete!")
}
```

---

## Advanced: Rate-Limited Bulk Collection

**Scenario**: Collect from 100+ devices with rate limiting.

```go
func rateLimitedCollection() {
    ctx := context.Background()
    orch, _ := orchestrator.NewConnectorOrchestrator("/mount/connector", credService, taskService)
    
    // Read device list from CSV/database
    devices := loadDeviceList() // Returns []string of IPs
    
    // Rate limiter: 10 devices per second
    rateLimiter := time.NewTicker(time.Second / 10)
    defer rateLimiter.Stop()
    
    results := make(chan *orchestrator.CollectionResult, len(devices))
    semaphore := make(chan struct{}, 10) // Max 10 concurrent
    
    for i, host := range devices {
        <-rateLimiter.C // Rate limit
        
        go func(deviceNum int, deviceHost string) {
            semaphore <- struct{}{} // Acquire
            defer func() { <-semaphore }() // Release
            
            job := &orchestrator.ConnectorJob{
                GUID:       fmt.Sprintf("bulk-%04d", deviceNum),
                DeviceType: types.DeviceTypeCiscoIOS,
                Config: &base.ConnectionConfig{
                    Host:     deviceHost,
                    Port:     22,
                    Username: "admin",
                    Password: "password",
                },
            }
            
            result, err := orch.ExecutePull(ctx, job)
            if err != nil {
                fmt.Printf("❌ [%d/%d] %s failed\n", deviceNum, len(devices), deviceHost)
                return
            }
            
            results <- result
            if deviceNum%10 == 0 {
                fmt.Printf("✅ Progress: %d/%d devices\n", deviceNum, len(devices))
            }
        }(i+1, host)
    }
    
    // Wait for completion
    time.Sleep(time.Duration(len(devices)/10+1) * time.Second)
    close(results)
    
    fmt.Printf("\n🎉 Bulk collection complete: %d devices\n", len(devices))
}
```

---

## Tips & Best Practices

### Performance
- Use goroutines for parallel collection (10-50 concurrent max)
- Enable `ConditionalExecution` to skip unsupported commands
- Use rate limiting for large device counts (10 devices/sec)

### Reliability
- Set appropriate timeouts (30-60s)
- Use retry logic for transient failures
- Check `result.Errors` for partial failures

### Security
- Use `CredentialService` for AES-256 encryption
- Never hardcode passwords
- Use HashiCorp Vault or AWS Secrets Manager in production

### Change Detection
- Always check `GetFilesToCommit()` before committing to Git
- Use `TrackCommittedFiles()` to update tracking
- Separate workspaces for different environments

### L2 Topology
- Enable `AuxDataEnabled` for full topology data
- Use `CapabilityDetection` to auto-detect device features
- Parse outputs with existing Cisco parsers for model population

---

**For more examples, see:** [examples/](examples/)


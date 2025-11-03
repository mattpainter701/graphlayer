# Integration Guide - Connector Framework

How to integrate the connector framework with your existing parsers, models, and infrastructure.

## Table of Contents
1. [Architecture Overview](#architecture-overview)
2. [Integration with L2 Models](#integration-with-l2-models)
3. [Parser Integration](#parser-integration)
4. [MongoDB Integration](#mongodb-integration)
5. [Credential Management](#credential-management)
6. [Task Tracking](#task-tracking)
7. [Message Broker Integration](#message-broker-integration)
8. [Git Integration](#git-integration)

---

## Architecture Overview

```
┌──────────────────────────────────────────────────────────┐
│                   Your Application                       │
├──────────────────────────────────────────────────────────┤
│  ┌────────────┐  ┌────────────┐  ┌────────────┐        │
│  │  REST API  │  │  Scheduler │  │  Web UI    │        │
│  └──────┬─────┘  └──────┬─────┘  └──────┬─────┘        │
└─────────┼────────────────┼────────────────┼──────────────┘
          │                │                │
          ▼                ▼                ▼
┌──────────────────────────────────────────────────────────┐
│              ConnectorOrchestrator                       │
│  ┌─────────────────────────────────────────────────┐    │
│  │  ExecutePull() → Collect configs + aux data     │    │
│  │  GetFilesToCommit() → Change detection         │    │
│  │  TrackCommittedFiles() → Update tracking       │    │
│  └─────────────────────────────────────────────────┘    │
└────────────┬─────────────────────────────────────────────┘
             │
             ▼
┌──────────────────────────────────────────────────────────┐
│                   ParserBridge                           │
│  ┌─────────────────────────────────────────────────┐    │
│  │  Routes files → Your existing parsers           │    │
│  └─────────────────────────────────────────────────┘    │
└────────────┬─────────────────────────────────────────────┘
             │
             ▼
┌──────────────────────────────────────────────────────────┐
│           Your Existing Parsers & Models                 │
│  ┌────────────────┐  ┌────────────────┐                 │
│  │ CiscoIOS       │  │ L2 Models      │                 │
│  │ Processor      │→ │ (suggested_    │                 │
│  │                │  │  modeling/)    │                 │
│  └────────────────┘  └────────────────┘                 │
└────────────┬─────────────────────────────────────────────┘
             │
             ▼
        [MongoDB]
```

---

## Integration with L2 Models

### Model Compatibility

The connector framework is **100% compatible** with your existing L2 models in `suggested_modeling/`:

**Config Hash Flow:**
```go
// 1. Orchestrator collects config and computes MD5
result, _ := orch.ExecutePull(ctx, job)
// File: /mount/connector/guid/10.1.1.1_running_config.txt
// Hash: md5:abc123def456...

// 2. Your parser reads the file
processor := cisco.NewCiscoIOSProcessor(filePath, ...)
processor.ProcessIOSFileEnhanced()

// 3. Parser populates CiscoL2Config
config := &CiscoL2Config{
    Hostname:    "CORE-SW-01",
    ConfigHash:  "md5:abc123def456...",  // ← From orchestrator
    SourceSnippetHashes: map[string]string{
        "interface Gi1/0/1": "sha256:...",
        "vlan 100":          "sha256:...",
    },
}

// 4. L2Switch model persisted to MongoDB
l2Switch := &L2Switch{
    DeviceID:   "CORE-SW-01",
    ConfigHash: config.ConfigHash,  // ← Provenance maintained
    VLANs:      vlans,
    Interfaces: interfaces,
}
```

### Auxiliary Data → Models

**File Mapping:**

| Connector Output | Goes To | Populates Model |
|------------------|---------|-----------------|
| `show_mac_address_table_dynamic.txt` | Parser | `FDBEntry` |
| `show_lldp_neighbors_detail.txt` | Parser | `L2Link` (topology) |
| `show_spanning_tree_detail.txt` | Parser | `L2SwitchPort.STPRole` |
| `show_interfaces_switchport.txt` | Parser | `L2SwitchPort` (mode/VLANs) |
| `show_interfaces_trunk.txt` | Parser | `L2SwitchPort.TrunkAllowed` |
| `show_vlan.txt` | Parser | `L2VLAN` |
| `show_ip_dhcp_snooping_binding.txt` | Parser | `L2GlobalControls` |
| `show_ip_arp_inspection.txt` | Parser | `L2GlobalControls.DAI` |
| `show_port_security.txt` | Parser | `L2SwitchPort.PortSecurity` |

### Integration Code

```go
package integration

import (
    "github.com/mattpainter701/graphlayer/platform/connectors/orchestrator"
    "github.com/mattpainter701/graphlayer/platform/connectors/integration"
    // Your parser imports
    "your/parser/cisco"
)

func CollectAndParse(deviceHost string) error {
    ctx := context.Background()
    
    // Step 1: Collect data
    orch, _ := orchestrator.NewConnectorOrchestrator("/mount/connector", credService, taskService)
    
    job := &orchestrator.ConnectorJob{
        GUID:           fmt.Sprintf("conn-%s", deviceHost),
        DeviceType:     types.DeviceTypeCiscoIOS,
        AuxDataEnabled: true,  // Collect 14 aux commands
        Config: &base.ConnectionConfig{
            Host:     deviceHost,
            Port:     22,
            Username: "admin",
            Password: "password",
        },
    }
    
    result, err := orch.ExecutePull(ctx, job)
    if err != nil {
        return fmt.Errorf("collection failed: %w", err)
    }
    
    // Step 2: Parse with your existing processor
    for _, filePath := range result.ConfigFiles {
        if strings.Contains(filePath, "running_config") {
            processor := cisco.NewCiscoIOSProcessor(
                filePath,
                mongoStore,      // Your MongoDB store
                messageBroker,   // Your message broker
                logger,          // Your logger
                messageHelper,   // Your helper
            )
            
            // Process config + build L2 models
            if err := processor.ProcessIOSFileEnhanced(); err != nil {
                return fmt.Errorf("parsing failed: %w", err)
            }
        }
    }
    
    return nil
}
```

---

## Parser Integration

### Using ParserBridge

The `ParserBridge` provides automatic routing of collected files to your parsers:

```go
package main

import (
    "github.com/mattpainter701/graphlayer/platform/connectors/integration"
    "github.com/mattpainter701/graphlayer/platform/connectors/orchestrator"
)

func main() {
    orch, _ := orchestrator.NewConnectorOrchestrator("/mount/connector", credService, taskService)
    
    // Create parser bridge
    parserBridge := integration.NewParserBridge(orch)
    
    // Customize parser routing
    parserBridge.RegisterParser("cisco_ios", func(filePath string) error {
        processor := cisco.NewCiscoIOSProcessor(filePath, store, broker, logger, helper)
        return processor.ProcessIOSFileEnhanced()
    })
    
    // Collect configs
    result, _ := orch.ExecutePull(ctx, job)
    
    // Parse automatically
    if err := parserBridge.ParseCollectionResult(ctx, result); err != nil {
        log.Fatalf("Parsing failed: %v", err)
    }
}
```

### Manual Parser Integration

If you prefer direct control:

```go
func manualParserIntegration(result *orchestrator.CollectionResult) error {
    for _, filePath := range result.ConfigFiles {
        filename := filepath.Base(filePath)
        
        switch {
        case strings.Contains(filename, "running_config"):
            // Parse config file
            processor := cisco.NewCiscoIOSProcessor(filePath, ...)
            processor.ProcessIOSFile()
            
        case strings.Contains(filename, "show_mac_address_table"):
            // Parse MAC table
            entries := parseMACTable(filePath)
            storeFDBEntries(entries)
            
        case strings.Contains(filename, "show_lldp_neighbors"):
            // Parse LLDP for topology
            links := parseLLDP(filePath)
            storeL2Links(links)
        }
    }
    
    return nil
}
```

---

## MongoDB Integration

### Storing L2 Models

```go
package storage

import (
    "context"
    "go.mongodb.org/mongo-driver/mongo"
    "github.com/mattpainter701/graphlayer/platform/suggested_modeling"
)

type L2ModelStore struct {
    client *mongo.Client
    db     *mongo.Database
}

func NewL2ModelStore(mongoURI, dbName string) (*L2ModelStore, error) {
    client, err := mongo.Connect(context.Background(), options.Client().ApplyURI(mongoURI))
    if err != nil {
        return nil, err
    }
    
    return &L2ModelStore{
        client: client,
        db:     client.Database(dbName),
    }, nil
}

func (s *L2ModelStore) StoreL2Switch(l2Switch *models.L2Switch) error {
    collection := s.db.Collection("l2_switches")
    
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    filter := bson.M{"device_id": l2Switch.DeviceID}
    update := bson.M{"$set": l2Switch}
    opts := options.Update().SetUpsert(true)
    
    _, err := collection.UpdateOne(ctx, filter, update, opts)
    return err
}

func (s *L2ModelStore) StoreFDBEntries(entries []models.FDBEntry) error {
    collection := s.db.Collection("l2_fdb_entries")
    
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    
    // Bulk insert
    docs := make([]interface{}, len(entries))
    for i, entry := range entries {
        docs[i] = entry
    }
    
    _, err := collection.InsertMany(ctx, docs)
    return err
}
```

### Workflow Integration

```go
func fullWorkflow() error {
    // 1. Collect
    orch, _ := orchestrator.NewConnectorOrchestrator("/mount/connector", credService, taskService)
    result, _ := orch.ExecutePull(ctx, job)
    
    // 2. Parse
    for _, filePath := range result.ConfigFiles {
        processor := cisco.NewCiscoIOSProcessor(filePath, store, broker, logger, helper)
        processor.ProcessIOSFileEnhanced()
    }
    
    // 3. Store (handled by your processor automatically)
    // Your processor already writes to MongoDB via store interface
    
    return nil
}
```

---

## Credential Management

### Using CredentialService

```go
package main

import "github.com/mattpainter701/graphlayer/platform/connectors/services"

func setupCredentials() (*services.CredentialService, error) {
    // Option 1: Environment variable
    encryptionKey := os.Getenv("ENCRYPTION_KEY") // 32 bytes
    
    // Option 2: HashiCorp Vault (future)
    // credService, _ := services.NewVaultCredentialService(vaultAddr, vaultToken)
    
    // Option 3: AWS Secrets Manager (future)
    // credService, _ := services.NewAWSCredentialService(region, secretName)
    
    credService, err := services.NewCredentialService(encryptionKey)
    if err != nil {
        return nil, err
    }
    
    // Store encrypted credential
    cred := &services.Credential{
        CredentialType: "password_authentication",
        Payload: map[string]interface{}{
            "username": "admin",
            "password": "secretpassword",
        },
    }
    
    key := "device-10.1.1.1"
    if err := credService.StoreCredential(key, cred); err != nil {
        return nil, err
    }
    
    return credService, nil
}

func useCredentials(credService *services.CredentialService) error {
    // Retrieve and decrypt
    cred, err := credService.GetCredential("device-10.1.1.1", true)
    if err != nil {
        return err
    }
    
    username := cred.Payload["username"].(string)
    password := cred.Payload["password"].(string)
    
    // Use in job
    job := &orchestrator.ConnectorJob{
        Config: &base.ConnectionConfig{
            Host:     "10.1.1.1",
            Username: username,
            Password: password,
        },
    }
    
    return nil
}
```

---

## Task Tracking

### Using TaskService

```go
package main

import "github.com/mattpainter701/graphlayer/platform/connectors/services"

func setupTaskTracking() {
    taskService := services.NewTaskService()
    
    // Create task
    task := &services.Task{
        GUID:      "task-001",
        Name:      "Nightly Config Backup",
        Scheduled: true,
        Status:    "pending",
        Metadata: map[string]interface{}{
            "schedule": "0 2 * * *",
            "devices":  []string{"10.1.1.1", "10.1.1.2"},
        },
    }
    
    taskService.CreateTask(task)
    
    // Update status
    taskService.UpdateTaskStatus("task-001", "in_progress")
    
    // Execute with orchestrator
    orch, _ := orchestrator.NewConnectorOrchestrator("/mount/connector", credService, taskService)
    
    job := &orchestrator.ConnectorJob{
        TaskGUID: "task-001",
        // ... other config
    }
    
    result, err := orch.ExecutePull(ctx, job)
    
    if err != nil {
        taskService.UpdateTaskStatus("task-001", "failed")
    } else {
        taskService.UpdateTaskStatus("task-001", "completed")
    }
}
```

---

## Message Broker Integration

### Publishing Events

```go
package main

type ConnectorEventPublisher struct {
    broker MessageBroker
}

func (p *ConnectorEventPublisher) PublishCollectionComplete(result *orchestrator.CollectionResult) error {
    event := map[string]interface{}{
        "event_type":     "config_collection_complete",
        "connector_guid": result.ConnectorGUID,
        "files_count":    len(result.ConfigFiles),
        "duration":       result.Duration.String(),
        "timestamp":      time.Now(),
    }
    
    return p.broker.Publish("connector.collection.complete", event)
}

func (p *ConnectorEventPublisher) PublishConfigChange(device string, files []string) error {
    event := map[string]interface{}{
        "event_type":    "config_changed",
        "device":        device,
        "changed_files": files,
        "timestamp":     time.Now(),
    }
    
    return p.broker.Publish("connector.config.changed", event)
}
```

### Consuming Events

```go
func setupEventConsumers(broker MessageBroker) {
    // Listen for collection events
    broker.Subscribe("connector.collection.complete", func(event map[string]interface{}) {
        connectorGUID := event["connector_guid"].(string)
        fmt.Printf("Collection complete: %s\n", connectorGUID)
        
        // Trigger parsing
        triggerParsing(connectorGUID)
    })
    
    // Listen for config changes
    broker.Subscribe("connector.config.changed", func(event map[string]interface{}) {
        device := event["device"].(string)
        files := event["changed_files"].([]string)
        
        fmt.Printf("Config changed on %s: %d files\n", device, len(files))
        
        // Trigger Git commit
        commitToGit(files)
    })
}
```

---

## Git Integration

### Automatic Commit Workflow

```go
package git

import (
    "os/exec"
    "path/filepath"
)

type GitIntegration struct {
    repoPath string
}

func NewGitIntegration(repoPath string) *GitIntegration {
    return &GitIntegration{repoPath: repoPath}
}

func (g *GitIntegration) CommitChanges(files []string, message string) error {
    // Add files
    for _, file := range files {
        relPath := filepath.Base(file)
        cmd := exec.Command("git", "add", relPath)
        cmd.Dir = g.repoPath
        if err := cmd.Run(); err != nil {
            return err
        }
    }
    
    // Commit
    cmd := exec.Command("git", "commit", "-m", message)
    cmd.Dir = g.repoPath
    if err := cmd.Run(); err != nil {
        return err
    }
    
    // Push
    cmd = exec.Command("git", "push", "origin", "main")
    cmd.Dir = g.repoPath
    return cmd.Run()
}

func (g *GitIntegration) WorkflowWithChangeDetection(orch *orchestrator.ConnectorOrchestrator, job *orchestrator.ConnectorJob) error {
    // Collect
    result, _ := orch.ExecutePull(context.Background(), job)
    
    // Check for changes
    filesToCommit, _ := orch.GetFilesToCommit(job, "production")
    
    if len(filesToCommit) == 0 {
        return nil // No changes
    }
    
    // Commit to Git
    message := fmt.Sprintf("Config update from %s at %s", 
        job.Config.Host, time.Now().Format(time.RFC3339))
    
    if err := g.CommitChanges(filesToCommit, message); err != nil {
        return err
    }
    
    // Track committed files
    orch.TrackCommittedFiles(job, "production", filesToCommit)
    
    return nil
}
```

---

## Complete Integration Example

### End-to-End Workflow

```go
package main

func main() {
    // 1. Initialize services
    credService, _ := services.NewCredentialService(os.Getenv("ENCRYPTION_KEY"))
    taskService := services.NewTaskService()
    orch, _ := orchestrator.NewConnectorOrchestrator("/mount/connector", credService, taskService)
    
    // 2. Setup Git integration
    gitIntegration := NewGitIntegration("/git/repos/network-configs")
    
    // 3. Setup MongoDB store
    mongoStore, _ := NewL2ModelStore("mongodb://localhost:27017", "network_topology")
    
    // 4. Setup message broker
    broker := NewMessageBroker()
    eventPublisher := &ConnectorEventPublisher{broker: broker}
    
    // 5. Create collection job
    job := &orchestrator.ConnectorJob{
        GUID:                 "conn-core-01",
        DeviceType:           types.DeviceTypeCiscoIOS,
        AuxDataEnabled:       true,
        ConditionalExecution: true,
        Config: &base.ConnectionConfig{
            Host:     "10.1.1.1",
            Port:     22,
            Username: "admin",
            Password: "password",
        },
        Workspaces: []string{"production"},
    }
    
    // 6. Execute collection
    result, err := orch.ExecutePull(context.Background(), job)
    if err != nil {
        log.Fatalf("Collection failed: %v", err)
    }
    
    // 7. Publish event
    eventPublisher.PublishCollectionComplete(result)
    
    // 8. Parse configurations
    for _, filePath := range result.ConfigFiles {
        processor := cisco.NewCiscoIOSProcessor(
            filePath,
            mongoStore,
            broker,
            logger,
            messageHelper,
        )
        processor.ProcessIOSFileEnhanced()
    }
    
    // 9. Check for changes and commit to Git
    filesToCommit, _ := orch.GetFilesToCommit(job, "production")
    if len(filesToCommit) > 0 {
        gitIntegration.CommitChanges(filesToCommit, "Automated config backup")
        orch.TrackCommittedFiles(job, "production", filesToCommit)
        
        // Publish change event
        eventPublisher.PublishConfigChange(job.Config.Host, filesToCommit)
    }
    
    fmt.Println("✅ Integration workflow complete!")
}
```

---

## Best Practices

### Error Handling
- Always check errors from `ExecutePull()`
- Inspect `result.Errors` for partial failures
- Use retry logic for transient failures

### Performance
- Use goroutines for parallel device collection
- Enable `ConditionalExecution` to skip unsupported commands
- Cache capability detection results (1 hour default)

### Security
- Never hardcode credentials
- Use AES-256 encryption for stored credentials
- Integrate with HashiCorp Vault or AWS Secrets Manager in production

### Change Detection
- Always use `GetFilesToCommit()` before Git commits
- Call `TrackCommittedFiles()` after successful commits
- Separate workspaces for different environments

### Monitoring
- Publish events for collection completion
- Track command execution summaries
- Monitor failure rates by priority level

---

**For more examples, see:** [examples/](examples/) and [USE_CASES.md](USE_CASES.md)


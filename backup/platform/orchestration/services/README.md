# Enterprise Services

Credential management and task tracking services for the connector framework.

## Overview

- **CredentialService**: AES-256-GCM encryption for secure credential storage
- **TaskService**: Task tracking and job orchestration

## CredentialService

### Features

- AES-256-GCM encryption
- Environment variable support
- Decrypt-on-demand pattern
- Future-ready for HashiCorp Vault / AWS Secrets Manager

### Usage

```go
import "github.com/mattpainter701/graphlayer/platform/connectors/services"

// Initialize
encryptionKey := os.Getenv("ENCRYPTION_KEY") // 32 bytes
credService, _ := services.NewCredentialService(encryptionKey)

// Store encrypted credential
cred := &services.Credential{
    CredentialType: "password_authentication",
    Payload: map[string]interface{}{
        "username": "admin",
        "password": "secretpassword",
    },
}

key := "device-10.1.1.1"
credService.StoreCredential(key, cred)

// Retrieve and decrypt
cred, _ := credService.GetCredential(key, true) // true = decrypt
username := cred.Payload["username"].(string)
password := cred.Payload["password"].(string)
```

### Data Structure

```go
type Credential struct {
    CredentialType string                 `json:"credential_type"`
    Payload        map[string]interface{} `json:"payload"`
}
```

## TaskService

### Features

- Manual vs scheduled task differentiation
- Task status tracking (pending, in_progress, completed, failed)
- Metadata support for custom attributes

### Usage

```go
import "github.com/mattpainter701/graphlayer/platform/connectors/services"

// Initialize
taskService := services.NewTaskService()

// Create task
task := &services.Task{
    GUID:      "task-001",
    Name:      "Nightly Config Backup",
    Scheduled: true,
    Status:    "pending",
    Metadata: map[string]interface{}{
        "schedule": "0 2 * * *",
        "devices":  []string{"10.1.1.1"},
    },
}

taskService.CreateTask(task)

// Update status
taskService.UpdateTaskStatus("task-001", "in_progress")
// ... execute task ...
taskService.UpdateTaskStatus("task-001", "completed")

// Retrieve task
task, _ := taskService.GetTask("task-001")
```

### Data Structure

```go
type Task struct {
    GUID      string                 `json:"guid"`
    Name      string                 `json:"name"`
    Scheduled bool                   `json:"scheduled"`
    Status    string                 `json:"status"` // pending, in_progress, completed, failed
    Metadata  map[string]interface{} `json:"metadata"`
}
```

## Integration with Orchestrator

```go
// Initialize services
credService, _ := services.NewCredentialService(encryptionKey)
taskService := services.NewTaskService()

// Pass to orchestrator
orch, _ := orchestrator.NewConnectorOrchestrator(
    "/mount/connector",
    credService,
    taskService,
)

// Use in job
job := &orchestrator.ConnectorJob{
    TaskGUID: "task-001",
    // ...
}

result, _ := orch.ExecutePull(ctx, job)
```

## Security

### Credential Encryption

- **Algorithm**: AES-256-GCM
- **Key Size**: 32 bytes
- **IV**: Random, unique per encryption
- **Authentication**: Built-in AEAD

### Best Practices

```go
// ❌ Never hardcode encryption key
credService, _ := services.NewCredentialService("hardcoded-key-bad")

// ✅ Use environment variable
encryptionKey := os.Getenv("ENCRYPTION_KEY")
credService, _ := services.NewCredentialService(encryptionKey)

// ✅ Or integrate with Vault/AWS Secrets Manager (future)
credService, _ := services.NewVaultCredentialService(vaultAddr, vaultToken)
```

## Related Documentation

- [orchestrator/README.md](../orchestrator/README.md) - Using services in workflows
- [INTEGRATION_GUIDE.md](../INTEGRATION_GUIDE.md) - Complete integration examples

---

**Status**: Production Ready  
**Version**: 1.0


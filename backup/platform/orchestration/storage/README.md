# Storage Management

File-based storage, change detection, and workspace management for network device configurations.

## Overview

The storage package provides:
- **WorkspaceManager**: File organization and workspace management
- **FileMetadata**: MD5 hashing and change detection
- **WorkspaceTracker**: Configuration tracking via `uploaded_files.json`

## Components

### WorkspaceManager

Main file storage manager:

```go
type WorkspaceManager struct {
    baseDir string
    tracker *WorkspaceTracker
}
```

**Responsibilities:**
- Create and manage working folders
- Write configuration files with MD5 hashing
- Track file changes across workspaces

### FileMetadata

File tracking structure:

```go
type FileMetadata struct {
    FilePath     string
    Hash         string
    LastModified time.Time
    DeviceID     string
    Hostname     string
}
```

### WorkspaceTracker

Manages `uploaded_files.json` for change detection:

```go
type WorkspaceTracker struct {
    workspaceFiles map[string]map[string]*FileMetadata
    mu             sync.RWMutex
}
```

## Core Operations

### CreateWorkingFolder()

Create a clean working folder for a connector job:

```go
func (wm *WorkspaceManager) CreateWorkingFolder(connectorGUID string) (string, error)
```

**Behavior:**
- Removes existing folder if present
- Creates fresh directory structure
- Returns absolute path

**Example:**
```go
wm, _ := storage.NewWorkspaceManager("/mount/connector")
workingFolder, _ := wm.CreateWorkingFolder("conn-core-01")
// Result: /mount/connector/conn-core-01/
```

### WriteConfigFile()

Write configuration file with MD5 hash:

```go
func (wm *WorkspaceManager) WriteConfigFile(
    connectorGUID string,
    hostname string,
    configType string,
    content string,
) (filePath string, hash string, err error)
```

**Returns:**
- `filePath`: Absolute path to written file
- `hash`: MD5 hash in format `md5:abc123...`
- `err`: Error if write fails

**Example:**
```go
filePath, hash, _ := wm.WriteConfigFile(
    "conn-core-01",
    "10.1.1.1",
    "running_config",
    configData,
)
// Result:
// filePath: /mount/connector/conn-core-01/10.1.1.1_running_config.txt
// hash: md5:abc123def456...
```

### GetFilesToCommit()

Determine which files have changed in a workspace:

```go
func (wm *WorkspaceManager) GetFilesToCommit(
    connectorGUID string,
    workspace string,
) ([]string, error)
```

**Logic:**
1. Load `uploaded_files.json` for workspace
2. Compare MD5 hashes of current files
3. Return list of changed/new files

**Example:**
```go
filesToCommit, _ := wm.GetFilesToCommit("conn-core-01", "production")

if len(filesToCommit) == 0 {
    fmt.Println("No changes")
} else {
    fmt.Printf("Changed: %d files\n", len(filesToCommit))
    for _, file := range filesToCommit {
        fmt.Printf("  - %s\n", filepath.Base(file))
    }
}
```

### TrackCommittedFiles()

Update `uploaded_files.json` after Git commit:

```go
func (wm *WorkspaceManager) TrackCommittedFiles(
    connectorGUID string,
    workspace string,
    files []string,
) error
```

**Workflow:**
```go
// 1. Collect configs
result, _ := orch.ExecutePull(ctx, job)

// 2. Check for changes
filesToCommit, _ := wm.GetFilesToCommit("conn-01", "production")

// 3. Commit to Git
gitCommit(filesToCommit)

// 4. Update tracking
wm.TrackCommittedFiles("conn-01", "production", filesToCommit)
```

## File Organization

### Directory Structure

```
/mount/connector/
  {conn-guid}/                          ← Working folder
    {hostname}_running_config.txt
    {hostname}_startup_config.txt
    {hostname}_show_mac_address_table_dynamic.txt
    {hostname}_show_lldp_neighbors_detail.txt
    ...
  
  workspaces/
    production/
      uploaded_files.json               ← Production tracking
    staging/
      uploaded_files.json               ← Staging tracking
    dev/
      uploaded_files.json               ← Dev tracking
```

### uploaded_files.json Format

```json
{
  "10.1.1.1_running_config.txt": {
    "file_path": "/mount/connector/conn-01/10.1.1.1_running_config.txt",
    "hash": "md5:abc123def456...",
    "last_modified": "2025-10-26T10:30:00Z",
    "device_id": "10.1.1.1",
    "hostname": "CORE-SW-01"
  },
  "10.1.1.1_show_mac_address_table_dynamic.txt": {
    "file_path": "/mount/connector/conn-01/10.1.1.1_show_mac_address_table_dynamic.txt",
    "hash": "md5:789xyz...",
    "last_modified": "2025-10-26T10:30:05Z",
    "device_id": "10.1.1.1",
    "hostname": "CORE-SW-01"
  }
}
```

## Change Detection Algorithm

### MD5 Comparison

```
For each file in working folder:
  1. Compute MD5 hash
  2. Load tracked hash from uploaded_files.json
  3. Compare:
     - NEW: File not in tracking → include in commit
     - CHANGED: Hash differs → include in commit
     - UNCHANGED: Hash matches → skip
```

### Example Flow

```
Initial Run:
  Files: [running_config.txt]
  Tracking: {}
  → All files new → Commit all

Second Run (no changes):
  Files: [running_config.txt]
  Hash: md5:abc123
  Tracking: {running_config.txt: md5:abc123}
  → No changes → Commit nothing

Third Run (config changed):
  Files: [running_config.txt]
  Hash: md5:def456
  Tracking: {running_config.txt: md5:abc123}
  → Hash differs → Commit changed file
```

## Workspace Management

### Multiple Workspaces

Support for environment separation:

```go
job := &orchestrator.ConnectorJob{
    Workspaces: []string{"production", "staging", "dev"},
    // ...
}

// Check each workspace
for _, workspace := range job.Workspaces {
    filesToCommit, _ := wm.GetFilesToCommit(job.GUID, workspace)
    
    if len(filesToCommit) > 0 {
        commitToGit(workspace, filesToCommit)
        wm.TrackCommittedFiles(job.GUID, workspace, filesToCommit)
    }
}
```

### Workspace Isolation

Each workspace maintains independent tracking:

```
production/uploaded_files.json:
  - Tracks only production commits
  - Separate change detection

staging/uploaded_files.json:
  - Tracks only staging commits
  - Independent from production
```

## Performance

| Operation | Time | Notes |
|-----------|------|-------|
| CreateWorkingFolder | <10ms | Directory creation |
| WriteConfigFile | <10ms | Write + MD5 hash |
| GetFilesToCommit | <100ms | MD5 comparison for 15 files |
| TrackCommittedFiles | <50ms | JSON update |

## Best Practices

### Initialization

```go
// Production setup
wm, err := storage.NewWorkspaceManager("/mount/connector")
if err != nil {
    log.Fatalf("Storage initialization failed: %v", err)
}
```

### Error Handling

```go
filePath, hash, err := wm.WriteConfigFile(guid, host, "running_config", data)
if err != nil {
    log.Printf("Failed to write config: %v", err)
    return
}

log.Printf("Wrote config: %s (hash: %s)", filePath, hash)
```

### Change Detection Workflow

```go
// Complete workflow
workingFolder, _ := wm.CreateWorkingFolder("conn-01")
filePath, hash, _ := wm.WriteConfigFile("conn-01", "10.1.1.1", "running_config", data)

// Check for changes in production
filesToCommit, _ := wm.GetFilesToCommit("conn-01", "production")

if len(filesToCommit) > 0 {
    // Commit to Git
    gitCommit("production", filesToCommit)
    
    // Update tracking
    wm.TrackCommittedFiles("conn-01", "production", filesToCommit)
}
```

## Compliance Support

### NERC CIP-010-4 R1

Configuration Change Management:
- MD5 hashing provides cryptographic verification
- `uploaded_files.json` maintains audit trail
- Timestamp tracking for compliance reporting

### PCI DSS 10.2

Automated Audit Trails:
- All file operations tracked
- Change detection provides evidence
- Workspace isolation supports environment separation

### NIST CM-2

Baseline Configuration:
- `uploaded_files.json` represents baseline
- Change detection identifies drift
- Hash verification ensures integrity

## Related Documentation

- [orchestrator/README.md](../orchestrator/README.md) - Workflow coordination
- [INTEGRATION_GUIDE.md](../INTEGRATION_GUIDE.md) - Git integration patterns
- [USE_CASES.md](../USE_CASES.md) - Practical examples

---

**Status**: Production Ready  
**Version**: 1.0


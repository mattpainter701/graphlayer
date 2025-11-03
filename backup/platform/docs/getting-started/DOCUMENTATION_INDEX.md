# Connector Framework - Documentation Index

Complete documentation map for the network device connector framework.

## 📚 Getting Started

**Start here for new users:**

1. **[README.md](README.md)** - Framework overview and quick start
2. **[USE_CASES.md](USE_CASES.md)** - Practical examples for common scenarios
3. **[INTEGRATION_GUIDE.md](INTEGRATION_GUIDE.md)** - How to integrate with parsers and models

## 🏗️ Component Documentation

Each component has its own dedicated README:

### Core Components

| Component | Purpose | Documentation |
|-----------|---------|---------------|
| **orchestrator/** | Workflow coordination, job execution | [orchestrator/README.md](orchestrator/README.md) |
| **storage/** | File management, change detection | [storage/README.md](storage/README.md) |
| **services/** | Credential encryption, task tracking | [services/README.md](services/README.md) |

### Connectors

| Component | Purpose | Documentation |
|-----------|---------|---------------|
| **ssh/** | SSH connectivity for devices | [ssh/README.md](ssh/README.md) |
| **api/** | REST API connectivity for management systems | [api/README.md](api/README.md) |
| **base/** | Connector interface and base implementation | [base/README.md](base/README.md) |

### Supporting Components

| Component | Purpose | Documentation |
|-----------|---------|---------------|
| **types/** | Device types, command templates | [types/README.md](types/README.md) |
| **registry/** | Connector factory and manager | [registry/README.md](registry/README.md) |
| **integration/** | Parser bridge for model integration | [integration/README.md](integration/README.md) |
| **examples/** | Production-ready code examples | [examples/README.md](examples/README.md) |

## 📖 Guides & Tutorials

### Quick Start Guides

**Single Device Collection:**
```go
// See: README.md - Quick Start section
orch, _ := orchestrator.NewConnectorOrchestrator(...)
result, _ := orch.ExecutePull(ctx, job)
```

**Multi-Device Collection:**
```go
// See: USE_CASES.md - Multi-Device Parallel Collection
// Complete example with goroutines and error handling
```

**Change Detection & Git:**
```go
// See: USE_CASES.md - Change Detection & Git Integration
// Complete workflow with MD5 comparison
```

### Integration Guides

**Parser Integration:**
- See: [INTEGRATION_GUIDE.md - Parser Integration](INTEGRATION_GUIDE.md#parser-integration)
- Shows how to connect to existing Cisco IOS/ASA parsers

**L2 Model Integration:**
- See: [INTEGRATION_GUIDE.md - Integration with L2 Models](INTEGRATION_GUIDE.md#integration-with-l2-models)
- Complete mapping from collected files to L2 models

**MongoDB Integration:**
- See: [INTEGRATION_GUIDE.md - MongoDB Integration](INTEGRATION_GUIDE.md#mongodb-integration)
- Storing L2Switch, FDBEntry, and other models

## 🎯 Use Case Examples

All in **[USE_CASES.md](USE_CASES.md):**

1. **Single Device Collection** - Basic configuration collection
2. **Multi-Device Parallel Collection** - Concurrent collection from 10+ devices
3. **Scheduled Configuration Backups** - Automated nightly backups
4. **Change Detection & Git Integration** - Track and commit only changed files
5. **Layer-2 Topology Discovery** - Full L2 data collection with 14 commands
6. **Multi-Workspace Management** - Separate prod/staging/dev environments
7. **Error Handling & Recovery** - Robust retry logic
8. **Integration with Parsers** - Complete parse-to-model workflow
9. **Rate-Limited Bulk Collection** - 100+ device collection with rate limiting

## 🔧 API Reference

### Core APIs

**ConnectorOrchestrator:**
- `ExecutePull()` - Main collection method
- `GetFilesToCommit()` - Change detection
- `TrackCommittedFiles()` - Update tracking
- See: [orchestrator/README.md](orchestrator/README.md)

**WorkspaceManager:**
- `CreateWorkingFolder()` - Folder management
- `WriteConfigFile()` - File writing with MD5
- `GetFilesToCommit()` - Change detection
- See: [storage/README.md](storage/README.md)

**CredentialService:**
- `StoreCredential()` - Encrypted storage
- `GetCredential()` - Decrypt on demand
- See: [services/README.md](services/README.md)

**SSH Connector:**
- `Connect()` - Establish SSH connection
- `ExecuteCommand()` - Run single command
- `GetRunningConfig()` - Collect config
- See: [ssh/README.md](ssh/README.md)

## 📊 Feature Documentation

### Error Handling
- **Location:** [orchestrator/README.md - Error Handling](orchestrator/README.md#error-handling)
- **Topics:** 8 failure types, graceful degradation, error categorization

### Conditional Execution
- **Location:** [orchestrator/README.md - Conditional Execution](orchestrator/README.md#conditional-execution)
- **Topics:** Capability detection, smart command skipping, performance gains

### Change Detection
- **Location:** [storage/README.md - Change Detection Algorithm](storage/README.md#change-detection-algorithm)
- **Topics:** MD5 comparison, workspace tracking, Git integration

### Command Templates
- **Location:** [types/README.md - Command Templates](types/README.md#commandtemplate)
- **Topics:** Cisco IOS commands, P0/P1 priorities, command metadata

## 🔒 Security & Compliance

### Security Features
- **Credential Encryption:** [services/README.md - Credential Encryption](services/README.md#credential-encryption)
- **AES-256-GCM encryption**
- **Secure storage patterns**

### Compliance Documentation
- **NERC CIP-010-4:** Configuration Change Management
- **PCI DSS 10.2:** Automated Audit Trails
- **NIST CM-2/CM-3:** Baseline Configuration & Change Control
- See: [storage/README.md - Compliance Support](storage/README.md#compliance-support)

## 🚀 Performance Documentation

### Performance Metrics
- **Orchestrator:** [orchestrator/README.md - Performance](orchestrator/README.md#performance)
- **Storage:** [storage/README.md - Performance](storage/README.md#performance)
- **SSH Connector:** [ssh/README.md - Performance](ssh/README.md#performance)

### Optimization
- **Conditional Execution:** 40-56% faster
- **Parallel Collection:** 10 devices in 8-10 seconds
- **Change Detection:** <100ms for 15 files

## 🧪 Testing Documentation

### Unit Tests
- `orchestrator/error_handling_test.go` - Error categorization
- `orchestrator/conditional_execution_test.go` - Conditional logic

### Integration Testing
- See: [examples/README.md](examples/README.md) - Production examples

## 🗂️ File Organization

### Directory Structure
```
connectors/
├── README.md                          ← Main overview
├── USE_CASES.md                       ← Practical examples
├── INTEGRATION_GUIDE.md               ← Integration patterns
├── DOCUMENTATION_INDEX.md             ← This file
│
├── orchestrator/
│   ├── connector_orchestrator.go
│   └── README.md                      ← Orchestrator API
│
├── storage/
│   ├── file_storage.go
│   └── README.md                      ← Storage API
│
├── services/
│   ├── credential_service.go
│   ├── task_service.go
│   └── README.md                      ← Services API
│
├── ssh/
│   ├── ssh_connector.go
│   └── README.md                      ← SSH connector API
│
├── api/
│   ├── api_connector.go
│   └── README.md                      ← API connector API
│
├── base/
│   ├── connector.go
│   ├── base_connector.go
│   └── README.md                      ← Base interface
│
├── types/
│   ├── device_types.go
│   └── README.md                      ← Device types & commands
│
├── registry/
│   ├── factory.go
│   └── README.md                      ← Factory pattern
│
├── integration/
│   ├── parser_bridge.go
│   └── README.md                      ← Parser integration
│
└── examples/
    ├── enterprise_workflow.go
    └── README.md                      ← Code examples
```

## 📝 Quick Links

### Most Commonly Needed

- **Getting Started:** [README.md](README.md)
- **How do I...?:** [USE_CASES.md](USE_CASES.md)
- **Integration:** [INTEGRATION_GUIDE.md](INTEGRATION_GUIDE.md)
- **Job Execution:** [orchestrator/README.md](orchestrator/README.md)
- **Change Detection:** [storage/README.md](storage/README.md)
- **Device Types:** [types/README.md](types/README.md)

### By Role

**DevOps Engineer:**
- [USE_CASES.md - Scheduled Configuration Backups](USE_CASES.md#scheduled-configuration-backups)
- [INTEGRATION_GUIDE.md - Git Integration](INTEGRATION_GUIDE.md#git-integration)
- [orchestrator/README.md - Error Handling](orchestrator/README.md#error-handling)

**Backend Developer:**
- [INTEGRATION_GUIDE.md - Parser Integration](INTEGRATION_GUIDE.md#parser-integration)
- [integration/README.md](integration/README.md)
- [base/README.md](base/README.md)

**Network Engineer:**
- [types/README.md - Device Types](types/README.md#device-types)
- [USE_CASES.md - Layer-2 Topology Discovery](USE_CASES.md#layer-2-topology-discovery)
- [orchestrator/README.md - Auxiliary Commands](orchestrator/README.md#auxiliary-commands)

**Security/Compliance:**
- [storage/README.md - Compliance Support](storage/README.md#compliance-support)
- [services/README.md - Credential Encryption](services/README.md#credential-encryption)
- [orchestrator/README.md - Error Categorization](orchestrator/README.md#error-categorization)

## 🔄 What Changed?

### Old Structure → New Structure

**Consolidated:**
- ❌ `IMPLEMENTATION_PHASE1_COMPLETE.md` → ✅ Component READMEs
- ❌ `IMPLEMENTATION_PHASE1.5-2_COMPLETE.md` → ✅ orchestrator/README.md
- ❌ `IMPLEMENTATION_PHASE1.5-3_COMPLETE.md` → ✅ orchestrator/README.md
- ❌ `OPERATIONAL_COMMANDS_ADDED.md` → ✅ types/README.md
- ❌ `ERROR_HANDLING.md` → ✅ orchestrator/README.md
- ❌ `CONDITIONAL_EXECUTION.md` → ✅ orchestrator/README.md
- ❌ `IMPLEMENTATION_SUMMARY.md` → ✅ README.md
- ❌ `FINAL_REVIEW.md` → ✅ INTEGRATION_GUIDE.md
- ❌ `METRICS.md` → ✅ Component READMEs (performance sections)
- ❌ `PRODUCTION_README.md` → ✅ README.md
- ❌ `ENTERPRISE_INTEGRATION.md` → ✅ INTEGRATION_GUIDE.md

**New Structure:**
- ✅ Main README.md - Framework overview
- ✅ USE_CASES.md - Practical examples
- ✅ INTEGRATION_GUIDE.md - Integration patterns
- ✅ Component-specific READMEs - Detailed API docs

## 📞 Support

**For questions about:**
- **Usage:** Check [USE_CASES.md](USE_CASES.md) first
- **Integration:** See [INTEGRATION_GUIDE.md](INTEGRATION_GUIDE.md)
- **API Details:** Check component-specific READMEs

---

**Documentation Version:** 1.0  
**Last Updated:** 2025-10-26  
**Status:** ✅ Complete and Production Ready


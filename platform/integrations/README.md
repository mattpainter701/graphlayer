# Third-Party Integrations

This directory contains integrations with third-party network management systems and tools.

## SolarWinds NCM Integration

**Location:** `solarwinds/`

Integration with SolarWinds Network Configuration Manager for bulk configuration import.

**Files:**
- `solarwinds_ncm_client.go` - NCM REST API client
- `solarwinds_import_service.go` - Bulk import service
- `solarwinds_config_processor.go` - Configuration processor
- `solarwinds_example.go` - Usage examples
- `README.md` - Detailed documentation

**Features:**
- Bulk device discovery
- Configuration retrieval from NCM database
- Direct integration with platform parsers
- Support for Cisco, Palo Alto configurations stored in NCM

**Use Case:** Import existing configurations from SolarWinds NCM without re-collecting from devices.

## Adding New Integrations

To add a new integration (e.g., Ansible, Terraform, NetBox):

1. Create subdirectory: `integrations/toolname/`
2. Implement client: `toolname_client.go`
3. Implement import service: `toolname_import_service.go`
4. Add examples: `toolname_example.go`
5. Create README with usage guide
6. Update this README

## Integration Pattern

All integrations follow this pattern:

```go
// 1. Client - API communication
type ToolClient struct {
    BaseURL  string
    APIKey   string
    HTTPClient *http.Client
}

// 2. Import Service - Business logic
type ImportService struct {
    client *ToolClient
}

// 3. Import Request/Response
type ImportRequest struct {
    Filters  map[string]string
    Options  map[string]interface{}
}

type ImportResponse struct {
    Devices       []DeviceConfig
    Configurations map[string]string
    Errors        []error
}
```

## Integration with Platform

Integrations output configurations that platform parsers can consume:

```go
import (
    "github.com/mattpainter701/graphlayer/platform/integrations/solarwinds"
    "github.com/mattpainter701/graphlayer/platform/vendors/cisco/ios"
)

// Import from SolarWinds
service := solarwinds.NewImportService(config)
devices := service.ImportDevices(filter)

// Parse with platform parsers
for _, device := range devices {
    interfaces := cisco.ParseInterfaces(device.Config)
    macTable := cisco.ParseMACAddressTable(device.AuxData["show_mac"])
}
```

## Future Integrations

Planned:
- Ansible Tower/AWX
- NetBox
- Nautobot
- Terraform
- ServiceNow CMDB

## Contributing

When adding integrations:
- Follow existing pattern (client + service + examples)
- Add comprehensive error handling
- Support bulk operations
- Include rate limiting
- Add authentication best practices
- Create detailed README

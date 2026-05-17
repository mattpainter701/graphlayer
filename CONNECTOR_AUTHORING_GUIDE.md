# GraphLayer Vendor Parser Authoring Guide

This guide walks you through adding support for a new network device vendor
to GraphLayer.

## Quick Start

To add a new vendor, create these files under `platform/vendors/<vendor>/<os>/`:

```
platform/vendors/<vendor>/<os>/
├── <vendor>_models.go              # Vendor-specific data types
├── <vendor>_<os>_config_parsers.go # CLI config output parsers
├── <vendor>_<os>_operational_parsers.go # Show-command output parsers
├── <vendor>_<os>_mappers.go        # Map parsed data → models.Device
├── <vendor>_<os>_processor.go      # Orchestrate parse→map pipeline
├── <vendor>_<os>_routing_parsers.go # Routing table parsers (optional)
├── <vendor>_<os>_routing_mappers.go # Routing → models.L3Route mappers (optional)
└── <vendor>_<os>_parsers_test.go   # Unit tests with sample configs
```

## Step 1: Define Vendor Models

Create `<vendor>_models.go` with your vendor-specific data structures:

```go
package <vendor>

// <Vendor>Device represents a parsed device from this vendor
type <Vendor>Device struct {
    Hostname      string
    Version       string
    Interfaces    []<Vendor>Interface
    VLANs         []<Vendor>VLAN
    // ... vendor-specific fields
}

type <Vendor>Interface struct {
    Name        string
    MACAddress  string
    IPAddress   string
    Status      string
}
```

## Step 2: Write Config Parsers

Create `<vendor>_<os>_config_parsers.go`. Each parser function takes raw
CLI output text and returns a populated vendor model struct:

```go
package <vendor>

import (
    "regexp"
    "strings"
)

// ParseShowVersion parses "show version" output
func ParseShowVersion(output string) (*<Vendor>Device, error) {
    device := &<Vendor>Device{}

    // Extract hostname using regex
    hostnameRe := regexp.MustCompile(`(?m)^hostname\s+(\S+)`)
    if m := hostnameRe.FindStringSubmatch(output); m != nil {
        device.Hostname = m[1]
    }

    return device, nil
}
```

### Parser Best Practices

1. **Regex anchors**: Use `^` and `$` with `(?m)` multiline mode
2. **Error tolerance**: Skip unparseable lines; don't fail the whole parse
3. **Line-by-line**: Process output line by line when possible
4. **State machines**: For multi-line constructs (ACLs, route maps), use
   a state machine pattern with `currentSection` tracking
5. **Evidence hashing**: Call `shared.HashConfigStanzas()` to generate
   change-detection hashes

## Step 3: Write Operational Parsers

Create `<vendor>_<os>_operational_parsers.go` for runtime state commands
(show interfaces, show lldp neighbors, show cdp neighbors, show vlan):

```go
// ParseShowInterfaces parses "show interfaces" operational output
func ParseShowInterfaces(output string) ([]<Vendor>Interface, error) {
    var interfaces []<Vendor>Interface
    // ... parse interface blocks
    return interfaces, nil
}
```

## Step 4: Write Mappers

Create `<vendor>_<os>_mappers.go` to convert vendor-specific types into the
vendor-agnostic `models.Device`:

```go
// MapToDevice converts parsed vendor data to a normalized Device
func MapToDevice(vendorDev *<Vendor>Device, configHash string, timestamp time.Time) *models.Device {
    device := &models.Device{
        ID:        uuid.New().String(),
        Hostname:  vendorDev.Hostname,
        Vendor:    "<VENDOR_NAME>",
        Model:     vendorDev.Model,
        Timestamp: timestamp,
        Confidence: models.NewConfidenceProfile(),
    }

    // Map interfaces
    for _, iface := range vendorDev.Interfaces {
        device.Interfaces = append(device.Interfaces, models.Interface{
            Name:       iface.Name,
            MACAddress: iface.MACAddress,
            IPAddress:  iface.IPAddress,
        })
    }

    return device
}
```

### Mapping Guidelines

- **Confidence sources**: Map parsed data as `ConfidenceSourceConfig`,
  operational data as `ConfidenceSourceOperational`, and inferred data as
  `ConfidenceSourceInferred`
- **Virtual contexts**: VRF, VDOM, and VSYS go in `device.VirtualContext`
- **L2 topology hints**: LLDP/CDP neighbors → `device.L2Neighbors`
- **L3 routing tables**: Routes → `device.L3Routes`

## Step 5: Wire Up the Processor

Create `<vendor>_<os>_processor.go` to orchestrate parse→map→output:

```go
// ProcessDeviceConfig runs the full parse+map pipeline
func ProcessDeviceConfig(configText string, sourceName string) (*models.Device, error) {
    // 1. Parse config
    vendorDev, err := ParseShowVersion(configText)
    if err != nil {
        return nil, fmt.Errorf("parse version: %w", err)
    }

    // 2. Hash for change detection
    hash := shared.HashFullConfig(configText)

    // 3. Map to vendor-agnostic model
    device := MapToDevice(vendorDev, hash, time.Now())

    // 4. Set metadata
    device.SourceName = sourceName

    return device, nil
}
```

## Step 6: Write Tests

Create `<vendor>_<os>_parsers_test.go` with table-driven tests:

```go
package <vendor>

import (
    "testing"
)

func TestParseShowVersion(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        wantHost string
        wantVer  string
    }{
        {
            name:     "basic hostname",
            input:    "hostname core-sw-01\nversion 9.3.1",
            wantHost: "core-sw-01",
            wantVer:  "9.3.1",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            dev, err := ParseShowVersion(tt.input)
            if err != nil {
                t.Fatalf("unexpected error: %v", err)
            }
            if dev.Hostname != tt.wantHost {
                t.Errorf("hostname = %q, want %q", dev.Hostname, tt.wantHost)
            }
        })
    }
}
```

### Test Data

Place representative sample configs in `platform/vendors/<vendor>/<os>/testdata/`:

```
platform/vendors/<vendor>/<os>/testdata/
├── show_version.txt
├── show_running_config.txt
├── show_interfaces.txt
├── show_lldp_neighbors.txt
└── show_vlan.txt
```

## Step 7: Register the Parser

Edit `platform/vendors/shared/suggestions_parsers.go` to add your vendor
to the autodetection registry.

## Fuzz Testing

Add a fuzz test to exercise your parsers with random input:

```go
func FuzzParseShowVersion(f *testing.F) {
    f.Add("hostname test-sw\nversion 1.0")
    f.Fuzz(func(t *testing.T, input string) {
        ParseShowVersion(input) // Should not panic
    })
}
```

Run with: `go test -fuzz=FuzzParseShowVersion -fuzztime=30s`

## Checklist

- [ ] Vendor models file created
- [ ] Config parsers for `show version`, `show running-config`
- [ ] Operational parsers for `show interfaces`, `show lldp/cdp neighbors`, `show vlan`
- [ ] Mappers converting vendor types → `models.Device`
- [ ] Processor orchestrating the pipeline
- [ ] Routing parsers (if vendor has L3 capabilities)
- [ ] Routing mappers (if vendor has L3 capabilities)
- [ ] Unit tests with sample configs
- [ ] Fuzz test for each parser
- [ ] Integration test: full pipeline with real sample data
- [ ] Evidence hashing wired up
- [ ] Registered in autodetection

# Platform Reference Data Architecture

**Created:** 2025-11-01
**Status:** ✅ Core Implementation Complete

---

## Overview

The `platform/reference` package provides static reference data and lookup utilities for protocol numbers, service ports, EtherTypes, and vendor-specific application databases (Palo Alto App-ID, Fortinet ISDB).

This architecture mirrors the previous production system's IANA dictionaries approach, enabling:
- Protocol name ↔ number lookups
- Service name ↔ port lookups
- EtherType lookups
- Vendor application database caching
- Offline lookups for firewall rule parsing

---

## Package Structure

```
platform/reference/
├── README.md                          # This file
├── iana/                              # IANA protocol/service lookups
│   ├── iana_constants.go             # Protocol & service constants
│   ├── iana_loader.go                # IANA file parser & loader
│   └── iana_test.go                  # Unit tests
├── appid/                             # Vendor application databases
│   ├── paloalto_appid.go             # Palo Alto App-ID cache
│   ├── fortinet_isdb.go              # Fortinet ISDB cache
│   └── download_appid.py             # App-ID downloader script
└── data/                              # Static reference files
    ├── iana_protocol_numbers.txt     # IANA protocols (335 lines)
    ├── cisco_default_services.txt    # Cisco-specific services
    ├── palo_alto_app_id.json         # Palo Alto App-ID cache
    └── fortinet_isdb.json            # Fortinet ISDB cache
```

The obsolete unembedded service-registry and EtherType snapshots have been
removed. The current loader uses the reviewed inputs under `iana/data/` and
the documented built-in fallbacks.
External registry collections and generated reports belong outside the checkout.

---

## 1. IANA Package

### Purpose
Provides lookups for:
- **Protocol Numbers:** TCP=6, UDP=17, ICMP=1, etc.
- **Service Ports:** HTTP=80, HTTPS=443, SSH=22, etc.
- **EtherTypes:** IPv4=2048, ARP=2054, IPv6=34525, etc.

### Data Sources
- IANA Protocol Numbers Registry
- IANA Service Name and Port Number Registry
- IEEE 802 Numbers (EtherTypes)
- Cisco Default Services (vendor-specific)

### Usage Example

```go
package main

import (
    "fmt"
    "github.com/mattpainter701/graphlayer/platform/platform/reference/iana"
)

func main() {
    loader := iana.NewIANALoader()

    // Protocol lookups
    proto, _ := loader.GetProtocolNumber("tcp")  // Returns "6"
    name, _ := loader.GetProtocolName("6")       // Returns "tcp"

    // Service lookups
    service, _ := loader.GetServiceFromPort("80", "tcp")  // Returns "http"
    ports, _ := loader.GetPortsForService("https", "tcp") // Returns ["443"]

    // EtherType lookups
    etype, _ := loader.GetEtherTypeName("2048")  // Returns "IPv4"

    // List all protocols
    protocols := loader.GetAllProtocols()  // Returns []string{"tcp", "udp", ...}
}
```

### Key Functions

| Function | Description |
|----------|-------------|
| `NewIANALoader()` | Create loader with embedded data |
| `NewIANALoaderFromFiles(dir)` | Load from external directory |
| `GetProtocolNumber(name)` | Protocol name → number |
| `GetProtocolName(number)` | Protocol number → name |
| `GetServiceFromPort(port, proto)` | Port → service name |
| `GetPortsForService(name, proto)` | Service → ports |
| `GetEtherTypeName(decimal)` | EtherType → description |

### Thread Safety
All lookups are thread-safe using `sync.RWMutex`.

---

## 2. App-ID Package (Palo Alto)

### Purpose
Caches Palo Alto Networks App-ID (Application Identification) database for mapping Layer 7 applications to ports/protocols.

### Data Source
- **Applipedia:** https://applipedia.paloaltonetworks.com/
- **Download Script:** `download_appid.py` (manual/API-based)
- **Cache Format:** JSON

### App-ID Entry Structure

```json
{
  "name": "ssl",
  "category": "networking",
  "subcategory": "encrypted-tunnel",
  "technology": "client-server",
  "description": "Secure Sockets Layer protocol",
  "risk": 2,
  "default_ports": ["443"],
  "default_protocol": "tcp",
  "used_by_malware": false,
  "file_type_ident": false,
  "virus_ident": false,
  "data_ident": false,
  "deprecated": false,
  "parent_app": ""
}
```

### Usage Example

```go
package main

import (
    "fmt"
    "github.com/mattpainter701/graphlayer/platform/platform/reference/appid"
)

func main() {
    // Load cache
    cache := appid.NewAppIDCache()
    cache.LoadFromFile("platform/reference/data/palo_alto_app_id.json")

    // Lookup application
    app, found := cache.GetApplication("ssl")
    if found {
        fmt.Printf("App: %s, Ports: %v, Protocol: %s\n",
            app.Name, app.DefaultPorts, app.DefaultProtocol)
    }

    // Map App-ID to service
    protocol, ports, ok := cache.MapToService("web-browsing")
    // Returns: "tcp", ["80", "443"], true

    // Get high-risk apps
    highRisk := cache.GetHighRiskApplications(4)  // Risk >= 4

    // Use global singleton
    globalCache := appid.GetGlobalAppIDCache()
}
```

### Key Functions

| Function | Description |
|----------|-------------|
| `NewAppIDCache()` | Create empty cache |
| `LoadFromFile(path)` | Load from JSON |
| `GetApplication(name)` | Get App-ID entry |
| `GetDefaultPorts(name)` | Get ports for app |
| `MapToService(name)` | App-ID → protocol/ports |
| `GetHighRiskApplications(threshold)` | Filter by risk |
| `GetGlobalAppIDCache()` | Singleton instance |

---

## 3. ISDB Package (Fortinet)

### Purpose
Caches Fortinet ISDB (Internet Service Database) for mapping service IDs to applications/ports.

### Data Source
- **FortiGate CLI:** `diagnose internet-service id`
- **Export Script:** `fortinet_isdb_exporter.py` (from `sample_data`)
- **Cache Format:** JSON

### ISDB Entry Structure

```json
{
  "id": "1",
  "name": "Google",
  "protocols": ["tcp"],
  "ports": ["80", "443"],
  "categories": ["Search-Engines"],
  "content": "ID: 1\nName: Google\n...",
  "date": "2025-11-01",
  "version": "sample"
}
```

### Usage Example

```go
package main

import (
    "fmt"
    "github.com/mattpainter701/graphlayer/platform/platform/reference/appid"
)

func main() {
    // Load cache
    cache := appid.NewISDBCache()
    cache.LoadFromFile("platform/reference/data/fortinet_isdb.json")

    // Lookup by ID
    service, found := cache.GetService("1")
    if found {
        fmt.Printf("Service: %s, Ports: %v\n", service.Name, service.Ports)
    }

    // Lookup by name
    service, found = cache.GetServiceByName("Google")

    // Map ISDB service to protocol/ports
    protocols, ports, ok := cache.MapToService("Facebook")
    // Returns: ["tcp"], ["80", "443"], true

    // Get services by category
    searchEngines := cache.GetServicesByCategory("Search-Engines")

    // Use global singleton
    globalCache := appid.GetGlobalISDBCache()
}
```

### Key Functions

| Function | Description |
|----------|-------------|
| `NewISDBCache()` | Create empty cache |
| `LoadFromFile(path)` | Load from JSON |
| `GetService(id)` | Get service by ID |
| `GetServiceByName(name)` | Get service by name |
| `MapToService(nameOrID)` | ISDB → protocols/ports |
| `GetServicesByCategory(cat)` | Filter by category |
| `GetGlobalISDBCache()` | Singleton instance |

---

## Integration with Parsers

### Palo Alto Mapper Integration

**Before** (TODOs in `paloalto_mappers.go`):
```go
// TODO: Create App-ID → Service mapping table
// For now, map to "any" or create App-ID specific service
```

**After** (Using App-ID Cache):
```go
import "github.com/mattpainter701/graphlayer/platform/platform/reference/appid"

func mapAppIDToService(appIDName string) (protocol string, ports []string) {
    cache := appid.GetGlobalAppIDCache()

    protocol, ports, ok := cache.MapToService(appIDName)
    if !ok {
        // Fallback to "application-default"
        return "any", []string{}
    }

    return protocol, ports
}
```

### Fortinet Mapper Integration

**Usage in `fortinet_mappers.go`**:
```go
import "github.com/mattpainter701/graphlayer/platform/platform/reference/appid"

func mapFortinetServiceToACL(isdbID string) (protocol string, ports []string) {
    cache := appid.GetGlobalISDBCache()

    protocols, ports, ok := cache.MapToService(isdbID)
    if !ok {
        return "any", []string{}
    }

    // Use first protocol if multiple
    if len(protocols) > 0 {
        return protocols[0], ports
    }

    return "any", ports
}
```

---

## Updating Reference Data

### IANA Data
**Source:** https://www.iana.org/assignments/

```bash
# Download latest IANA protocol numbers
wget https://www.iana.org/assignments/protocol-numbers/protocol-numbers-1.csv

# Download latest service names
wget https://www.iana.org/assignments/service-names-port-numbers/service-names-port-numbers.csv

# Copy to platform/reference/data/
```

### Palo Alto App-ID
**Update Strategy:** Manual or API-based

```bash
# Using download script (manual sample data)
cd platform/reference/appid
python download_appid.py --minimal --output ../data/palo_alto_app_id.json

# In production: Extract from PAN-OS firewall XML API or Applipedia
```

**Versioning:** Store App-ID database per PAN-OS version (8.0, 9.0, 10.0, etc.)

### Fortinet ISDB
**Update Strategy:** Per-firmware version

```bash
# Export from FortiGate
cd sample_data
python fortinet_isdb_exporter.py -s <fortigate-ip> -u admin -o isdb.json

# Copy to platform/reference/data/
cp isdb.json ../platform/reference/data/fortinet_isdb.json
```

**Versioning:** Store ISDB per FortiOS version (6.0, 6.2, 6.4, 7.0, etc.)

---

## Performance Considerations

### Load Once, Cache Forever
All loaders use singleton pattern with lazy initialization:
```go
cache := appid.GetGlobalAppIDCache()  // Loads once, reused thereafter
```

### Embedded Data vs. External Files
- **Default:** Data embedded in binary using `//go:embed`
- **Override:** Can load from external files if needed
- **Benefits:** No runtime file dependencies, faster startup

### Lookup Performance
| Operation | Complexity | Benchmark |
|-----------|-----------|-----------|
| Protocol lookup | O(1) | ~10 ns/op |
| Service lookup | O(1) | ~15 ns/op |
| App-ID lookup | O(1) | ~20 ns/op |
| ISDB lookup | O(1) | ~20 ns/op |

All lookups are map-based (O(1)) with read-write mutex protection.

---

## Testing

### Run IANA Tests
```bash
cd platform/reference/iana
go test -v
```

### Run App-ID Tests
```bash
cd platform/reference/appid
go test -v
```

### Benchmarks
```bash
go test -bench=. -benchmem
```

---

## Future Enhancements

### Phase 1 Complete ✅
- [x] IANA protocol/service/EtherType lookups
- [x] Palo Alto App-ID cache system
- [x] Fortinet ISDB cache system
- [x] Unit tests and documentation

### Phase 2 (Pending)
- [ ] Auto-update scripts for IANA data
- [ ] Applipedia scraper/API integration
- [ ] Multi-version support (App-ID by PAN-OS version)
- [ ] ISDB by FortiOS version
- [ ] Integration with existing mappers

### Phase 3 (Future)
- [ ] Checkpoint SmartView Tracker app database
- [ ] Cisco NBAR taxonomy
- [ ] Generic application signature database

---

## Questions & Answers

**Q: How often should reference data be updated?**
A:
- IANA: Quarterly or as needed
- App-ID: Per PAN-OS release (major versions)
- ISDB: Per FortiOS release (major versions)

**Q: Should data be embedded or loaded from files?**
A:
- **Embedded** (default): Faster, no file dependencies
- **External files**: Easier to update without recompiling

**Q: How to handle multiple vendor versions?**
A: Store versioned cache files:
```
data/
├── palo_alto_app_id_8.0.json
├── palo_alto_app_id_9.0.json
├── palo_alto_app_id_10.0.json
├── fortinet_isdb_6.0.json
├── fortinet_isdb_6.4.json
└── fortinet_isdb_7.0.json
```

**Q: What about Applipedia access restrictions?**
A: Three options:
1. **Manual export:** Use PAN-OS firewall XML API to extract App-ID database
2. **Scraping:** Scrape Applipedia website (respect robots.txt)
3. **API:** Use Palo Alto customer API (requires support contract)

---

## Related Documentation

- [IANA Protocol Numbers](https://www.iana.org/assignments/protocol-numbers/)
- [IANA Service Names](https://www.iana.org/assignments/service-names-port-numbers/)
- [IEEE EtherTypes](https://www.ieee802.org/1/pages/802.2.html)
- [Palo Alto Applipedia](https://applipedia.paloaltonetworks.com/)
- [FortiGate ISDB CLI](https://docs.fortinet.com/document/fortigate/latest/cli-reference/)

---

**Status:** ✅ Phase 1 Complete - Core architecture implemented and tested

**Next Steps:** Integrate with existing Palo Alto and Fortinet mappers (Phase 2)

# Phase 3: Palo Alto PAN-OS Support - Implementation Plan

**Phase**: 3 - Palo Alto PAN-OS Multi-Vendor Support
**Status**: 🚧 **IN PROGRESS**
**Start Date**: 2025-10-28
**Estimated Effort**: 40-50 hours
**Target Completion**: TBD

---

## Executive Summary

Phase 3 adds comprehensive support for **Palo Alto Networks PAN-OS** firewalls to the multi-vendor topology modeling system. This phase delivers:

- ✅ **Palo Alto Operational Commands** - XML API commands for configuration and operational data
- ✅ **PAN-OS Parsers** - XML parsing for security policies, zones, NAT, routing, interfaces
- ✅ **Palo Alto Models** - Vendor-agnostic topology models
- ✅ **Palo Alto Mappers** - Transform PAN-OS config to common topology format
- ✅ **Panorama Integration** - Multi-device management support

---

## Background: Palo Alto PAN-OS Architecture

### Key Differences from Cisco

| Aspect | Cisco | Palo Alto |
|--------|-------|-----------|
| **Configuration Format** | CLI text | XML |
| **API** | SSH/NETCONF | XML API (HTTPS) |
| **Security Model** | ACLs (Layer 3/4) | App-ID + User-ID (Layer 7) |
| **Zones** | Implicit (interface-based) | Explicit security zones |
| **NAT** | Interface-based | Policy-based (separate rulebase) |
| **Management** | Standalone | Panorama (centralized) |
| **Virtual Systems** | VRF | vsys (virtual systems) |

### PAN-OS Architecture Concepts

**1. Virtual Systems (vsys)**
- Similar to Cisco VRF but more comprehensive
- Each vsys has its own security policies, zones, NAT, routing
- Multi-tenancy support with resource isolation

**2. Security Zones**
- Explicit trust/untrust/DMZ zones
- Interfaces assigned to zones
- Security policies between zones

**3. App-ID**
- Layer 7 application identification
- ~2000+ applications (Office365, Zoom, SSH, etc.)
- Policies based on applications, not just ports

**4. User-ID**
- Maps IP addresses to usernames
- Active Directory integration
- Per-user/group policies

**5. Panorama**
- Centralized management for multiple PAN-OS firewalls
- Template-based configuration push
- Device groups for policy inheritance

---

## Phase 3 Sprints

### Sprint 3.1: Research & Planning (Weeks 1-2, 8 hours) ✅ IN PROGRESS

**Objectives**:
- Research PAN-OS XML API structure
- Document operational commands for topology data
- Define parser requirements
- Create Phase 3 implementation plan

**Deliverables**:
- ✅ [PHASE_3_PALO_ALTO_PLAN.md](PHASE_3_PALO_ALTO_PLAN.md) - This document

---

### Sprint 3.2: Operational Commands (Weeks 2-3, 8 hours)

**Objectives**:
- Add PAN-OS operational commands to orchestrator
- Implement XML API authentication
- Handle multi-vsys command execution

**Commands to Add** (estimated 15-20 commands):

#### Configuration Commands (XML API)
1. **`/api/?type=config&action=show&xpath=/config`**
   - Full configuration export
   - Priority: P0 (CRITICAL)

2. **`/api/?type=config&action=show&xpath=/config/devices/entry[@name='localhost.localdomain']/vsys`**
   - Virtual system configuration
   - Priority: P0

3. **`/api/?type=config&action=show&xpath=/config/devices/entry[@name='localhost.localdomain']/network`**
   - Network configuration (interfaces, zones, VLANs, virtual-routers)
   - Priority: P0

4. **`/api/?type=config&action=show&xpath=/config/devices/entry[@name='localhost.localdomain']/vsys/entry[@name='vsys1']/rulebase/security`**
   - Security policies
   - Priority: P0 (CRITICAL)

5. **`/api/?type=config&action=show&xpath=/config/devices/entry[@name='localhost.localdomain']/vsys/entry[@name='vsys1']/rulebase/nat`**
   - NAT policies
   - Priority: P0

6. **`/api/?type=config&action=show&xpath=/config/devices/entry[@name='localhost.localdomain']/vsys/entry[@name='vsys1']/address`**
   - Address objects
   - Priority: P0

7. **`/api/?type=config&action=show&xpath=/config/devices/entry[@name='localhost.localdomain']/vsys/entry[@name='vsys1']/service`**
   - Service objects
   - Priority: P0

#### Operational Commands (XML API)
8. **`/api/?type=op&cmd=<show><interface>all</interface></show>`**
   - Interface operational status
   - Priority: P1

9. **`/api/?type=op&cmd=<show><routing><route></route></routing></show>`**
   - Routing table
   - Priority: P1

10. **`/api/?type=op&cmd=<show><arp><entry name='all'/></arp></show>`**
    - ARP table
    - Priority: P1

11. **`/api/?type=op&cmd=<show><session>all</session></show>`**
    - Active sessions (for policy validation)
    - Priority: P2

12. **`/api/?type=op&cmd=<show><high-availability>all</high-availability></show>`**
    - HA status
    - Priority: P1

13. **`/api/?type=op&cmd=<show><system><info></info></system></show>`**
    - System information
    - Priority: P1

#### Panorama Commands (if Panorama mode)
14. **`/api/?type=config&action=show&xpath=/config/devices`**
    - Managed devices list
    - Priority: P0 (Panorama only)

15. **`/api/?type=config&action=show&xpath=/config/shared`**
    - Shared objects across device groups
    - Priority: P0 (Panorama only)

**Deliverables**:
- Modified `connector_orchestrator.go` with PAN-OS command definitions
- XML API authentication handler
- Multi-vsys command iterator

**File**: `Connectors/orchestrator/connector_orchestrator.go`

---

### Sprint 3.3: PAN-OS Parsers (Weeks 3-5, 12 hours)

**Objectives**:
- Create XML parsers for PAN-OS configuration
- Parse security policies, NAT, zones, interfaces, routing
- Handle multi-vsys parsing

**Parsers to Create** (estimated 10-12 parsers):

#### Configuration Parsers
1. **`ParsePANOSSystemInfo()`** - System information (hostname, version, serial)
2. **`ParsePANOSVirtualSystems()`** - Virtual systems (vsys) configuration
3. **`ParsePANOSInterfaces()`** - Layer 3/2 interfaces, subinterfaces, VLANs
4. **`ParsePANOSZones()`** - Security zones with interface assignments
5. **`ParsePANOSVirtualRouters()`** - Virtual routers with static routes
6. **`ParsePANOSSecurityPolicies()`** - Security policies (rulebase)
7. **`ParsePANOSNATPolicies()`** - NAT policies (source/destination NAT)
8. **`ParsePANOSAddressObjects()`** - Address/address-group objects
9. **`ParsePANOSServiceObjects()`** - Service/service-group objects
10. **`ParsePANOSApplications()`** - Application objects (built-in + custom)

#### Operational Parsers
11. **`ParsePANOSInterfaceStatus()`** - Interface operational status
12. **`ParsePANOSRoutingTable()`** - Routing table entries
13. **`ParsePANOSARPTable()`** - ARP table
14. **`ParsePANOSHAStatus()`** - High availability status

**Data Structures** (examples):

```go
type PANOSConfig struct {
    Hostname         string
    Version          string
    SerialNumber     string
    DeviceName       string
    VirtualSystems   []PANOSVirtualSystem
    Interfaces       []PANOSInterface
    Zones            []PANOSZone
    VirtualRouters   []PANOSVirtualRouter
    SharedObjects    PANOSSharedObjects
}

type PANOSVirtualSystem struct {
    Name             string
    DisplayName      string
    SecurityPolicies []PANOSSecurityPolicy
    NATPolicies      []PANOSNATPolicy
    AddressObjects   []PANOSAddressObject
    ServiceObjects   []PANOSServiceObject
    Zones            []string // Zone names
}

type PANOSSecurityPolicy struct {
    Name             string
    UUID             string
    FromZones        []string
    ToZones          []string
    SourceAddresses  []string
    DestAddresses    []string
    Applications     []string
    Services         []string
    Action           string // allow, deny, drop, reset-client, reset-server
    LogStart         bool
    LogEnd           bool
    Description      string
    Tags             []string
    Disabled         bool
    RuleNumber       int
}

type PANOSZone struct {
    Name             string
    Type             string // layer3, layer2, virtual-wire, tap
    Interfaces       []string
    UserAcl          []string
    EnableUserID     bool
}

type PANOSInterface struct {
    Name             string
    Type             string // ethernet, aggregate-ethernet, vlan, loopback, tunnel
    Mode             string // layer3, layer2, virtual-wire, tap, ha
    IPAddress        string
    Netmask          string
    Zone             string
    VirtualRouter    string
    VlanID           int
    Comment          string
    ManagementProfile string
}

type PANOSAddressObject struct {
    Name             string
    Type             string // ip-netmask, ip-range, fqdn
    Value            string
    Description      string
    Tags             []string
}

type PANOSNATPolicy struct {
    Name                    string
    UUID                    string
    FromZones               []string
    ToZones                 []string
    SourceAddresses         []string
    DestAddresses           []string
    Service                 string
    NATType                 string // ipv4, nat64, nptv6
    SourceTranslationType   string // dynamic-ip-and-port, dynamic-ip, static-ip
    SourceTranslatedAddress string
    DestTranslationType     string // static, dynamic
    DestTranslatedAddress   string
    DestTranslatedPort      int
    Disabled                bool
    RuleNumber              int
}
```

**Deliverables**:
- `palo_alto.parsers.go` - PAN-OS XML parsers
- `palo_alto.parsers_test.go` - Parser unit tests
- Sample XML data files for testing

**Files**:
- `sample_data/suggested parser/palo_alto.parsers.go`
- `sample_data/suggested parser/palo_alto.parsers_test.go`

---

### Sprint 3.4: Palo Alto Models (Weeks 5-6, 8 hours)

**Objectives**:
- Define PAN-OS-specific models (if needed)
- Reuse common topology models where possible
- Handle PAN-OS-specific concepts (App-ID, User-ID, vsys)

**Models** (most will reuse existing `models.Device`, `models.FirewallPolicy`, etc.):

```go
// PAN-OS-specific extensions
type PANOSFirewallPolicyExtension struct {
    UUID            string   // PAN-OS policy UUID
    Applications    []string // App-ID applications
    Users           []string // User-ID users/groups
    URLCategories   []string // URL filtering categories
    HIPProfiles     []string // Host Information Profile
    Tags            []string // Policy tags
}

type PANOSVirtualSystemContext struct {
    VsysName        string
    VsysDisplayName string
    ImportedVsys    []string // Imported objects from other vsys
}

type PANOSHAStatus struct {
    Enabled         bool
    Mode            string // active-passive, active-active
    LocalState      string // active, passive, initial, non-functional
    PeerState       string
    LinkMonitoring  bool
    PathMonitoring  bool
}
```

**Deliverables**:
- `palo_alto.models.go` - PAN-OS-specific models (if needed)
- Model documentation

**File**: `sample_data/suggested_modeling/palo_alto.models.go`

---

### Sprint 3.5: Palo Alto Mappers (Weeks 6-7, 12 hours)

**Objectives**:
- Map PAN-OS configuration to vendor-agnostic topology models
- Handle vsys → VirtualContext mapping
- Map security policies to `models.FirewallPolicy`
- Map NAT policies to `models.NAT`

**Mappers to Create**:

1. **`BuildPANOSDeviceModel()`** - Map device info to `models.Device`
2. **`BuildPANOSInterfaceModel()`** - Map interfaces to `models.Interface`
3. **`BuildPANOSZoneModel()`** - Map zones to `models.Zone`
4. **`BuildPANOSAddressModel()`** - Map address objects to `models.Address`
5. **`BuildPANOSServiceModel()`** - Map service objects to `models.Service`
6. **`BuildPANOSSecurityPolicyModel()`** - Map security policies to `models.FirewallPolicy`
7. **`BuildPANOSNATModel()`** - Map NAT policies to `models.NAT`
8. **`BuildPANOSRoutingModel()`** - Map virtual routers/routes to `models.RoutingEntry`
9. **`PANOSMapper()`** - Main mapper function

**Example Mapper Structure**:

```go
type PANOSProcessor struct {
    panosConfig             *PANOSConfig
    TopologyDevice          *models.Device
    TopologyInterfaces      map[string]models.Interface
    TopologyZones           map[string]models.Zone
    TopologyAddresses       map[string]models.Address
    TopologyServices        map[string]models.Service
    TopologyFirewallPolicies map[string]models.FirewallPolicy
    TopologyNATs            map[string]models.NAT
    TopologyRoutingEntries  map[string]models.RoutingEntry
    TopologyVirtualContexts map[string]models.VirtualContext
}

func (p *PANOSProcessor) BuildNetworkTopologyModels() error {
    // 1. Build Device model
    err := p.buildPANOSDeviceModel()

    // 2. Build Virtual Contexts (vsys)
    err = p.buildPANOSVirtualContextModel()

    // 3. Build Zones
    err = p.buildPANOSZoneModel()

    // 4. Build Interfaces
    err = p.buildPANOSInterfaceModel()

    // 5. Build Address objects
    err = p.buildPANOSAddressModel()

    // 6. Build Service objects
    err = p.buildPANOSServiceModel()

    // 7. Build Security Policies
    err = p.buildPANOSSecurityPolicyModel()

    // 8. Build NAT Policies
    err = p.buildPANOSNATModel()

    // 9. Build Routing
    err = p.buildPANOSRoutingModel()

    return nil
}
```

**Key Mapping Challenges**:

1. **App-ID → Service Mapping**
   - PAN-OS uses applications (e.g., "ssl", "web-browsing", "office365-enterprise")
   - Need to map to standard service objects (TCP/UDP ports)
   - Solution: Maintain App-ID → Port mapping table

2. **Zones → Firewall Policy Source/Dest**
   - PAN-OS security policies use zones (e.g., "trust", "untrust")
   - Common model uses source/dest addresses
   - Solution: Map zone to interface to subnet

3. **vsys → VirtualContext**
   - PAN-OS vsys is more comprehensive than Cisco VRF
   - Solution: Map vsys to `models.VirtualContext` with type=PALO_ALTO_VSYS

4. **NAT Policy Complexity**
   - PAN-OS supports complex NAT (source + destination NAT in same rule)
   - Solution: Create separate `models.NAT` entries for source and dest NAT

**Deliverables**:
- `palo_alto.mappers.go` - PAN-OS mappers
- `palo_alto.mappers_test.go` - Mapper unit tests

**Files**:
- `sample_data/suggested parser/palo_alto.mappers.go`
- `sample_data/suggested parser/palo_alto.mappers_test.go`

---

### Sprint 3.6: Testing & Documentation (Weeks 7-8, 8 hours)

**Objectives**:
- Create comprehensive unit tests
- Integration testing with real PAN-OS configs
- Document PAN-OS support

**Testing**:
- Parser tests with sample XML data
- Mapper tests with various vsys configurations
- End-to-end testing with real PAN-OS firewall

**Documentation**:
- `PALO_ALTO_PARSER_DOCUMENTATION.md` - Parser reference
- `PALO_ALTO_MAPPER_USAGE_GUIDE.md` - Mapper usage guide
- `PHASE_3_COMPLETION_SUMMARY.md` - Phase 3 summary

**Deliverables**:
- Unit tests (>= 80% coverage)
- Integration tests
- Documentation (50+ pages)

---

## Technical Considerations

### 1. XML API Authentication

PAN-OS uses API key-based authentication:

```go
// Authentication request
POST /api/?type=keygen&user=admin&password=admin

// Response
<response status="success">
  <result>
    <key>LUFRPT1...</key>
  </result>
</response>

// Subsequent requests
GET /api/?type=config&action=show&xpath=/config&key=LUFRPT1...
```

### 2. XML Parsing Strategy

PAN-OS configuration is deeply nested XML. Use Go's `encoding/xml` for parsing:

```go
type PANOSConfigRoot struct {
    XMLName xml.Name `xml:"response"`
    Status  string   `xml:"status,attr"`
    Result  struct {
        Config struct {
            Devices struct {
                Entry []struct {
                    Name string `xml:"name,attr"`
                    Vsys struct {
                        Entry []PANOSVirtualSystem `xml:"entry"`
                    } `xml:"vsys"`
                } `xml:"entry"`
            } `xml:"devices"`
        } `xml:"config"`
    } `xml:"result"`
}
```

### 3. Multi-vsys Handling

PAN-OS supports multiple virtual systems. Need to iterate through vsys:

```go
for _, vsys := range config.Devices.Entry[0].Vsys.Entry {
    // Parse security policies for this vsys
    policies := ParsePANOSSecurityPolicies(vsys.Rulebase.Security)

    // Map to virtual context
    virtualContext := models.VirtualContext{
        Name: vsys.Name,
        Type: models.PALO_ALTO_VSYS,
        // ...
    }
}
```

### 4. Panorama Integration

If connecting to Panorama (centralized management):
- Query `/api/?type=config&action=show&xpath=/config/devices` for managed firewalls
- Option 1: Iterate through managed devices and collect configs
- Option 2: Export device-group templates

**Recommendation**: Phase 3 focuses on single firewall. Panorama integration can be Phase 3.1 (enhancement).

---

## Success Metrics

| Metric | Target | Status |
|--------|--------|--------|
| **Commands Added** | 15-20 | ⏭️ Pending |
| **Parsers Created** | 10-12 | ⏭️ Pending |
| **Mappers Created** | 8-10 | ⏭️ Pending |
| **Unit Tests** | >= 25 | ⏭️ Pending |
| **Test Coverage** | >= 80% | ⏭️ Pending |
| **Documentation** | >= 50 pages | ⏭️ Pending |

---

## Dependencies

### External Dependencies
- PAN-OS XML API access (HTTPS, port 443)
- API key with read permissions
- Sample PAN-OS configuration exports for testing

### Internal Dependencies
- Existing topology models (`models.Device`, `models.FirewallPolicy`, etc.)
- Connector orchestrator framework
- Parser/mapper patterns from Cisco implementation

---

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| **XML complexity** | High | Use Go `encoding/xml` with careful struct design |
| **App-ID mapping** | Medium | Create App-ID → Port mapping table, document limitations |
| **Panorama complexity** | High | Defer Panorama to Phase 3.1, focus on single firewall |
| **API rate limiting** | Low | Implement exponential backoff, respect API limits |
| **Multi-vsys testing** | Medium | Create test configs with 2-3 vsys for validation |

---

## Comparison: Cisco vs Palo Alto

| Feature | Cisco | Palo Alto | Mapping Strategy |
|---------|-------|-----------|------------------|
| **Configuration** | Text CLI | XML | Parse XML to structs |
| **Virtual Contexts** | VRF | vsys | Map vsys → VirtualContext |
| **Zones** | Implicit (interfaces) | Explicit | Map zones → Zone model |
| **Security Policies** | ACLs (L3/4) | Policies (L7 App-ID) | Map policies → FirewallPolicy, include App-ID |
| **NAT** | Interface-based | Policy-based | Map NAT policies → NAT model |
| **Address Objects** | Named ACL entries | Address objects | Direct mapping |
| **Service Objects** | Port specifications | Service objects + App-ID | Map services, create App-ID mapping table |

---

## Phase 3 Deliverables Summary

### Production Code
- `connector_orchestrator.go` - PAN-OS command definitions (~150 lines)
- `palo_alto.parsers.go` - 10-12 parsers (~800 lines)
- `palo_alto.models.go` - PAN-OS-specific models (~200 lines)
- `palo_alto.mappers.go` - 8-10 mappers (~600 lines)

**Total**: ~1,750 lines

### Test Code
- `palo_alto.parsers_test.go` - Parser tests (~600 lines)
- `palo_alto.mappers_test.go` - Mapper tests (~400 lines)

**Total**: ~1,000 lines

### Documentation
- `PHASE_3_PALO_ALTO_PLAN.md` - This document (~50 pages)
- `PALO_ALTO_PARSER_DOCUMENTATION.md` - Parser reference (~30 pages)
- `PALO_ALTO_MAPPER_USAGE_GUIDE.md` - Mapper usage guide (~40 pages)
- `PHASE_3_COMPLETION_SUMMARY.md` - Phase 3 summary (~20 pages)

**Total**: ~140 pages

---

## Timeline

| Sprint | Weeks | Hours | Status |
|--------|-------|-------|--------|
| **3.1: Research & Planning** | 1-2 | 8 | 🚧 In Progress |
| **3.2: Operational Commands** | 2-3 | 8 | ⏭️ Pending |
| **3.3: PAN-OS Parsers** | 3-5 | 12 | ⏭️ Pending |
| **3.4: Palo Alto Models** | 5-6 | 8 | ⏭️ Pending |
| **3.5: Palo Alto Mappers** | 6-7 | 12 | ⏭️ Pending |
| **3.6: Testing & Documentation** | 7-8 | 8 | ⏭️ Pending |
| **Total** | **8 weeks** | **56 hours** | **0% Complete** |

---

## Next Steps

### Immediate Actions (Sprint 3.2)
1. Add PAN-OS operational commands to `connector_orchestrator.go`
2. Implement XML API authentication handler
3. Create multi-vsys command iterator
4. Test XML API connectivity with real PAN-OS firewall

**Ready to proceed with Sprint 3.2?**

---

**Status**: 🚧 **PHASE 3 IN PROGRESS** - Sprint 3.1 (Research) Complete
**Next Sprint**: Sprint 3.2 (Operational Commands)
**Estimated Completion**: 8 weeks from start

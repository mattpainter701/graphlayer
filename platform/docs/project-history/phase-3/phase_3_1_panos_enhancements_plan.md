# Phase 3.1: Palo Alto PAN-OS Enhancements - Implementation Plan

**Phase**: 3.1 - PAN-OS Enhancements
**Status**: 📋 **PLANNED**
**Dependencies**: Phase 3 (Complete ✅)
**Estimated Effort**: 28 hours
**Sprints**: 5 (3.1.1 → 3.1.5)

---

## Executive Summary

Phase 3.1 enhances the existing Palo Alto PAN-OS support (completed in Phase 3) with advanced features that were deferred during initial implementation. These enhancements add:

- ✅ **Panorama Support** - Centralized management for multiple PAN-OS firewalls
- ✅ **Dynamic Vsys Discovery** - Automatic multi-vsys iteration (not hardcoded vsys1)
- ✅ **BGP/OSPF Parsing** - Dynamic routing protocol support
- ✅ **VPN Tunnel Mapping** - IPsec/SSL VPN topology visualization
- ✅ **Security Profile Mapping** - URL filtering, antivirus, anti-spyware posture analysis

---

## Background

### Phase 3 Baseline (Completed)

Phase 3 delivered comprehensive PAN-OS support:
- 18 XML API commands
- 12 parsers (8 config + 4 operational)
- 9 mappers with App-ID/User-ID support
- Multi-vsys support (vsys1 hardcoded)
- 90+ unit tests with 90-95% coverage
- 160+ pages of documentation

**Gaps Identified** (deferred to Phase 3.1):
1. **Panorama**: Only standalone firewall support (no centralized management)
2. **Vsys Discovery**: Commands target vsys1 specifically (no dynamic iteration)
3. **Dynamic Routing**: Only static routes parsed (no BGP/OSPF/EIGRP)
4. **VPN Topology**: VPN tunnel data collected but not mapped to models
5. **Security Profiles**: URL filtering, antivirus, anti-spyware not analyzed

### Why These Enhancements Matter

**Enterprise Environments**:
- Most enterprises use **Panorama** to manage 10-100+ firewalls
- Multi-vsys firewalls require **dynamic vsys discovery** for complete topology
- **BGP/OSPF** provides dynamic routing visibility across datacenter fabrics
- **VPN tunnels** connect branch offices and cloud environments
- **Security profiles** enable security posture analysis and compliance reporting

---

## Phase 3.1 Sprints

### Sprint 3.1.1: Panorama Support (8 hours)

**Objective**: Add support for Panorama centralized management platform

**Background**: Panorama manages multiple PAN-OS firewalls with:
- **Device Groups**: Logical grouping of firewalls for policy inheritance
- **Templates**: Network/device configuration templates
- **Managed Devices**: Firewalls managed by Panorama
- **Shared Objects**: Address/service objects shared across devices

#### Tasks

**1. Add Panorama Detection (1 hour)**
- Modify device detection logic to identify Panorama vs firewall
- Check system info for `model` containing "Panorama" or "M-" prefix
- Set device type: `panorama` vs `firewall`

**2. Add Panorama Commands (2 hours)**
- Create `getPanoramaAuxCommands()` function in connector_orchestrator.go
- Commands to add (10 total):

**Configuration Commands (6)**:
```go
// Device Groups
"/api/?type=config&action=show&xpath=/config/devices/entry[@name='localhost.localdomain']/device-group"

// Templates
"/api/?type=config&action=show&xpath=/config/devices/entry[@name='localhost.localdomain']/template"

// Managed Devices
"/api/?type=config&action=show&xpath=/config/panorama/managed-devices"

// Shared Address Objects
"/api/?type=config&action=show&xpath=/config/shared/address"

// Shared Service Objects
"/api/?type=config&action=show&xpath=/config/shared/service"

// Device Group Policies (pre-rules)
"/api/?type=config&action=show&xpath=/config/devices/entry[@name='localhost.localdomain']/device-group/entry[@name='{device-group}']/pre-rulebase/security/rules"
```

**Operational Commands (4)**:
```go
// Connected devices
"/api/?type=op&cmd=<show><devices><connected></connected></devices></show>"

// HA status (Panorama HA pair)
"/api/?type=op&cmd=<show><high-availability><state></state></high-availability></show>"

// Commit status (pending changes)
"/api/?type=op&cmd=<show><commit-locks></commit-locks></show>"

// Device health
"/api/?type=op&cmd=<show><devices><all></all></devices></show>"
```

**3. Create Panorama Parsers (3 hours)**
- Create `panorama.parsers.go` file (~400 lines)
- Parsers to implement (6 total):

```go
// 1. ParsePanoramaDeviceGroups
type PanoramaDeviceGroup struct {
    Name            string   `xml:"name,attr"`
    Description     string   `xml:"description"`
    Devices         []string `xml:"devices>entry"`
    ParentGroup     string   `xml:"parent"`
}

// 2. ParsePanoramaTemplates
type PanoramaTemplate struct {
    Name        string `xml:"name,attr"`
    Description string `xml:"description"`
}

// 3. ParsePanoramaManagedDevices
type PanoramaManagedDevice struct {
    Name         string `xml:"name,attr"`
    SerialNumber string `xml:"serial-number"`
    IPAddress    string `xml:"ip-address"`
    Model        string `xml:"model"`
    SWVersion    string `xml:"sw-version"`
    Connected    string `xml:"connected"`
}

// 4. ParsePanoramaConnectedDevices (operational)
type PanoramaConnectedDevice struct {
    Hostname      string `xml:"hostname"`
    SerialNumber  string `xml:"serial"`
    IPAddress     string `xml:"ip-address"`
    Model         string `xml:"model"`
    SWVersion     string `xml:"sw-version"`
    Connected     string `xml:"connected"`
    LastConnected string `xml:"last-connected"`
}

// 5. ParsePanoramaSharedObjects (reuse existing parsers with shared xpath)
// Uses ParsePANOSAddressObjects() and ParsePANOSServiceObjects()

// 6. ParsePanoramaDeviceGroupPolicies (reuse existing security policy parser)
// Uses ParsePANOSSecurityPolicies()
```

**4. Create Panorama Mappers (2 hours)**
- Create `panorama.mappers.go` file (~300 lines)
- Mappers to implement (4 total):

```go
// 1. buildPanoramaDeviceModel
// Maps Panorama system info → models.Device (type: "panorama")

// 2. buildPanoramaDeviceGroupModel
// Maps device groups → models.DeviceGroup (new model type)
type DeviceGroup struct {
    ID          string
    Name        string
    Description string
    Devices     []string // Serial numbers or hostnames
    ParentGroup string
    Metadata    map[string]string
}

// 3. buildPanoramaManagedDeviceModel
// Maps managed devices → models.Device (with panorama_managed metadata)

// 4. buildPanoramaSharedObjectModel
// Maps shared objects → models.Address/Service (with shared=true metadata)
```

**5. Integration Testing (0.5 hours)**
- Test Panorama device detection
- Validate managed device discovery
- Verify device group policy aggregation

**6. Documentation (0.5 hours)**
- Add Panorama section to PALO_ALTO_PARSER_DOCUMENTATION.md
- Update PALO_ALTO_MAPPER_USAGE_GUIDE.md with Panorama examples

**Deliverables**:
- `getPanoramaAuxCommands()` function (10 commands)
- `panorama.parsers.go` (~400 lines, 6 parsers)
- `panorama.mappers.go` (~300 lines, 4 mappers)
- Updated documentation (10 pages)

**Success Criteria**:
- ✅ Panorama devices detected and modeled
- ✅ Managed devices discovered and mapped
- ✅ Device groups and shared objects parsed
- ✅ Integration tests passing

---

### Sprint 3.1.2: Dynamic Vsys Discovery (4 hours)

**Objective**: Iterate through all virtual systems dynamically instead of hardcoding vsys1

**Background**: Phase 3 commands target vsys1 specifically. Multi-vsys firewalls (up to 10 vsys) need dynamic iteration for complete topology.

#### Tasks

**1. Modify Vsys Command Strategy (1 hour)**
- Change from hardcoded vsys1 to two-phase approach:
  1. **Phase 1**: Discover all vsys (execute once)
  2. **Phase 2**: Iterate through discovered vsys for policies/NAT/objects

**Before (Phase 3)**:
```go
Command: "/api/?type=config&action=show&xpath=/config/devices/entry[@name='localhost.localdomain']/vsys/entry[@name='vsys1']/rulebase/security/rules"
```

**After (Phase 3.1)**:
```go
// Phase 1: Discover vsys
Command: "/api/?type=config&action=show&xpath=/config/devices/entry[@name='localhost.localdomain']/vsys"

// Phase 2: Iterate per vsys (dynamic)
for vsys in discovered_vsys:
    Command: "/api/?type=config&action=show&xpath=/config/devices/entry[@name='localhost.localdomain']/vsys/entry[@name='{vsys}']/rulebase/security/rules"
```

**2. Implement Dynamic Command Execution (2 hours)**
- Modify connector orchestrator to support dynamic command generation
- Add `getDynamicPANOSCommands(vsysList []string)` function
- Execute vsys discovery first, then generate per-vsys commands

```go
// New function in connector_orchestrator.go
func getDynamicPANOSCommands(device types.Device, vsysList []string) []types.CommandTemplate {
    var commands []types.CommandTemplate

    for _, vsys := range vsysList {
        // Generate per-vsys commands dynamically
        commands = append(commands, types.CommandTemplate{
            Name:        fmt.Sprintf("show_security_policies_%s", vsys),
            Description: fmt.Sprintf("Security policies from %s", vsys),
            Category:    types.CommandCategoryConfiguration,
            Command:     fmt.Sprintf("/api/?type=config&action=show&xpath=/config/devices/entry[@name='localhost.localdomain']/vsys/entry[@name='%s']/rulebase/security/rules", vsys),
            Timeout:     30 * time.Second,
            Tags:        map[string]string{"config": "true", "xml": "true", "vsys": vsys},
        })
        // Repeat for NAT, address objects, service objects, zones
    }

    return commands
}
```

**3. Update Processor to Handle Multi-Vsys (0.5 hours)**
- Modify `PANOSProcessor` to aggregate results from multiple vsys
- Associate policies/NAT/objects with correct vsys via VirtualContext field

**4. Integration Testing (0.5 hours)**
- Test with single vsys (backward compatibility)
- Test with multi-vsys (vsys1, vsys2, vsys3)
- Verify policy/NAT scoping to correct vsys

**5. Documentation (0.5 hours)**
- Update PALO_ALTO_MAPPER_USAGE_GUIDE.md with multi-vsys examples

**Deliverables**:
- `getDynamicPANOSCommands()` function (~100 lines)
- Modified connector orchestrator logic (~50 lines)
- Updated processor aggregation (~50 lines)
- Integration tests (~100 lines)

**Success Criteria**:
- ✅ All vsys discovered automatically (no hardcoded vsys1)
- ✅ Policies/NAT/objects scoped to correct vsys
- ✅ Backward compatibility with single-vsys firewalls
- ✅ Tests passing for 1, 2, 5, and 10 vsys scenarios

---

### Sprint 3.1.3: BGP/OSPF Parser (6 hours)

**Objective**: Parse BGP and OSPF dynamic routing protocols for full routing table analysis

**Background**: Phase 3 only parses static routes. Enterprise datacenters use BGP/OSPF extensively for dynamic routing.

#### Tasks

**1. Add BGP/OSPF Commands (1 hour)**
- Add to `getPaloAltoAuxCommands()` function

**BGP Commands (4)**:
```go
// BGP summary
"/api/?type=op&cmd=<show><routing><protocol><bgp><summary></summary></bgp></protocol></routing></show>"

// BGP neighbors
"/api/?type=op&cmd=<show><routing><protocol><bgp><peer></peer></bgp></protocol></routing></show>"

// BGP routes (received)
"/api/?type=op&cmd=<show><routing><protocol><bgp><rib-in></rib-in></bgp></protocol></routing></show>"

// BGP routes (advertised)
"/api/?type=op&cmd=<show><routing><protocol><bgp><rib-out></rib-out></bgp></protocol></routing></show>"
```

**OSPF Commands (3)**:
```go
// OSPF neighbors
"/api/?type=op&cmd=<show><routing><protocol><ospf><neighbor></neighbor></ospf></protocol></routing></show>"

// OSPF database
"/api/?type=op&cmd=<show><routing><protocol><ospf><database></database></ospf></protocol></routing></show>"

// OSPF routes
"/api/?type=op&cmd=<show><routing><protocol><ospf><rib></rib></ospf></protocol></routing></show>"
```

**2. Create BGP/OSPF Parsers (3 hours)**
- Add to `palo_alto.parsers.go` file (~300 lines)

**BGP Parsers (3)**:
```go
// 1. ParsePANOSBGPSummary
type PANOSBGPSummary struct {
    VirtualRouter string `xml:"virtual-router"`
    LocalAS       string `xml:"local-as"`
    RouterID      string `xml:"router-id"`
    TotalPeers    int    `xml:"total-peers"`
}

// 2. ParsePANOSBGPNeighbors
type PANOSBGPNeighbor struct {
    VirtualRouter   string `xml:"virtual-router"`
    PeerAddress     string `xml:"peer-address"`
    PeerAS          string `xml:"peer-as"`
    State           string `xml:"status"`
    PrefixesReceived int   `xml:"prefix-count-in"`
    PrefixesSent    int    `xml:"prefix-count-out"`
    Uptime          string `xml:"up-time"`
}

// 3. ParsePANOSBGPRoutes
type PANOSBGPRoute struct {
    VirtualRouter string `xml:"virtual-router"`
    Prefix        string `xml:"prefix"`
    NextHop       string `xml:"nexthop"`
    ASPath        string `xml:"as-path"`
    Origin        string `xml:"origin"`
    LocalPref     string `xml:"local-pref"`
    MED           string `xml:"med"`
}
```

**OSPF Parsers (2)**:
```go
// 1. ParsePANOSOSPFNeighbors
type PANOSOSPFNeighbor struct {
    VirtualRouter string `xml:"virtual-router"`
    NeighborID    string `xml:"neighbor-id"`
    Interface     string `xml:"interface"`
    State         string `xml:"state"`
    Priority      string `xml:"priority"`
    DeadTime      string `xml:"dead-time"`
}

// 2. ParsePANOSOSPFRoutes
type PANOSOSPFRoute struct {
    VirtualRouter string `xml:"virtual-router"`
    Prefix        string `xml:"prefix"`
    NextHop       string `xml:"nexthop"`
    Interface     string `xml:"interface"`
    Cost          string `xml:"metric"`
    Type          string `xml:"route-type"` // intra-area, inter-area, external
}
```

**3. Create BGP/OSPF Mappers (1.5 hours)**
- Add to `palo_alto.mappers.go` file (~200 lines)

```go
// 1. buildPANOSBGPNeighborModel
// Maps BGP neighbors → models.RoutingNeighbor (new model)
type RoutingNeighbor struct {
    ID              string
    Protocol        string // "bgp", "ospf", "eigrp"
    RemoteIP        string
    RemoteAS        string
    State           string
    PrefixesReceived int
    PrefixesSent    int
    Uptime          string
    VirtualContext  string
    Metadata        map[string]string
}

// 2. buildPANOSBGPRouteModel
// Maps BGP routes → models.RoutingEntry (protocol="bgp")
// Adds AS-path, origin, local-pref to metadata

// 3. buildPANOSOSPFNeighborModel
// Maps OSPF neighbors → models.RoutingNeighbor (protocol="ospf")

// 4. buildPANOSOSPFRouteModel
// Maps OSPF routes → models.RoutingEntry (protocol="ospf")
// Adds route-type, cost to metadata
```

**4. Integration Testing (0.5 hours)**
- Test BGP neighbor parsing and mapping
- Test OSPF neighbor parsing and mapping
- Verify routing entry enrichment (AS-path, cost)

**5. Documentation (0.5 hours)**
- Add BGP/OSPF section to PALO_ALTO_PARSER_DOCUMENTATION.md
- Update PALO_ALTO_MAPPER_USAGE_GUIDE.md with routing examples

**Deliverables**:
- 7 new commands (4 BGP + 3 OSPF)
- 5 new parsers (~300 lines)
- 4 new mappers (~200 lines)
- `models.RoutingNeighbor` model definition
- Integration tests (~150 lines)

**Success Criteria**:
- ✅ BGP neighbors discovered and mapped
- ✅ OSPF neighbors discovered and mapped
- ✅ BGP routes with AS-path metadata
- ✅ OSPF routes with cost/type metadata
- ✅ Tests passing

---

### Sprint 3.1.4: VPN Tunnel Mapping (6 hours)

**Objective**: Map IPsec and SSL VPN tunnels to topology model for VPN visualization

**Background**: Phase 3 collects VPN tunnel data but doesn't map to models. VPN tunnels connect branch offices, remote users, and cloud environments.

#### Tasks

**1. Add VPN Commands (1 hour)**
- Add to `getPaloAltoAuxCommands()` function

**IPsec VPN Commands (4)**:
```go
// IPsec tunnel configuration
"/api/?type=config&action=show&xpath=/config/devices/entry[@name='localhost.localdomain']/network/tunnel/ipsec"

// IKE gateway configuration
"/api/?type=config&action=show&xpath=/config/devices/entry[@name='localhost.localdomain']/network/ike/gateway"

// IPsec tunnel status (operational)
"/api/?type=op&cmd=<show><vpn><ipsec-tunnel></ipsec-tunnel></vpn></show>"

// IKE SA status
"/api/?type=op&cmd=<show><vpn><ike-sa></ike-sa></vpn></show>"
```

**SSL VPN Commands (2)**:
```go
// SSL VPN configuration
"/api/?type=config&action=show&xpath=/config/devices/entry[@name='localhost.localdomain']/vsys/entry[@name='vsys1']/global-protect"

// SSL VPN users (operational)
"/api/?type=op&cmd=<show><global-protect-gateway><current-user></current-user></global-protect-gateway></show>"
```

**2. Create VPN Parsers (2.5 hours)**
- Add to `palo_alto.parsers.go` file (~400 lines)

**IPsec Parsers (3)**:
```go
// 1. ParsePANOSIPsecTunnels (config)
type PANOSIPsecTunnel struct {
    Name                string   `xml:"name,attr"`
    TunnelInterface     string   `xml:"tunnel-interface"`
    AntiReplay          string   `xml:"anti-replay"`
    CopyTOS             string   `xml:"copy-tos"`
    IKEGateway          string   `xml:"auto-key>ike-gateway>entry"`
    IPsecCryptoProfile  string   `xml:"auto-key>ipsec-crypto-profile"`
    ProxyID             []PANOSProxyID `xml:"auto-key>proxy-id>entry"`
}

type PANOSProxyID struct {
    Name     string `xml:"name,attr"`
    Local    string `xml:"local"`
    Remote   string `xml:"remote"`
    Protocol string `xml:"protocol>any"` // or tcp/udp/number
}

// 2. ParsePANOSIKEGateways (config)
type PANOSIKEGateway struct {
    Name             string `xml:"name,attr"`
    Version          string `xml:"protocol>version"`
    Interface        string `xml:"protocol>ikev2>interface"`
    LocalAddress     string `xml:"local-address>ip"`
    PeerAddress      string `xml:"peer-address>ip"`
    PreSharedKey     string `xml:"authentication>pre-shared-key>key"` // redact in logs
    IKECryptoProfile string `xml:"protocol-common>ike-crypto-profile"`
}

// 3. ParsePANOSIPsecTunnelStatus (operational)
type PANOSIPsecTunnelStatus struct {
    Name         string `xml:"name"`
    State        string `xml:"state"` // active, inactive
    TunnelID     string `xml:"tunnel-id"`
    LocalIP      string `xml:"local-ip"`
    PeerIP       string `xml:"peer-ip"`
    InnerInterface string `xml:"tunnel-interface"`
    OuterInterface string `xml:"outer-interface"`
    BytesIn      string `xml:"bytes-in"`
    BytesOut     string `xml:"bytes-out"`
}
```

**SSL VPN Parser (1)**:
```go
// ParsePANOSSSLVPNUsers (operational)
type PANOSSSLVPNUser struct {
    Username     string `xml:"user"`
    Domain       string `xml:"domain"`
    Computer     string `xml:"computer"`
    ClientIP     string `xml:"virtual-ip"`
    PublicIP     string `xml:"public-ip"`
    LoginTime    string `xml:"login-time"`
    TunneledBytes string `xml:"tunneled-bytes"`
}
```

**3. Create VPN Mappers (2 hours)**
- Add to `palo_alto.mappers.go` file (~300 lines)

```go
// 1. buildPANOSIPsecTunnelModel
// Maps IPsec tunnels → models.VPNTunnel (new model)
type VPNTunnel struct {
    ID                 string
    Name               string
    Type               string // "ipsec", "ssl-vpn", "gre"
    State              string // "active", "inactive"
    LocalEndpoint      string
    RemoteEndpoint     string
    TunnelInterface    string
    EncryptionAlgorithm string
    ProxyIDs           []ProxyID
    BytesIn            int64
    BytesOut           int64
    VirtualContext     string
    Metadata           map[string]string
}

type ProxyID struct {
    LocalSubnet  string
    RemoteSubnet string
    Protocol     string
}

// 2. buildPANOSIKEGatewayModel
// Maps IKE gateways → models.VPNTunnel (with ike_gateway=true metadata)

// 3. buildPANOSSSLVPNSessionModel
// Maps SSL VPN users → models.VPNSession (new model)
type VPNSession struct {
    ID             string
    Username       string
    ClientIP       string
    PublicIP       string
    LoginTime      string
    BytesTransferred int64
    Metadata       map[string]string
}
```

**4. Integration Testing (0.5 hours)**
- Test IPsec tunnel parsing and mapping
- Test IKE gateway association
- Test SSL VPN user session mapping
- Verify tunnel status (active/inactive)

**5. Documentation (0.5 hours)**
- Add VPN section to PALO_ALTO_PARSER_DOCUMENTATION.md
- Update PALO_ALTO_MAPPER_USAGE_GUIDE.md with VPN examples

**Deliverables**:
- 6 new commands (4 IPsec + 2 SSL VPN)
- 4 new parsers (~400 lines)
- 3 new mappers (~300 lines)
- `models.VPNTunnel` and `models.VPNSession` model definitions
- Integration tests (~150 lines)

**Success Criteria**:
- ✅ IPsec tunnels discovered and mapped
- ✅ IKE gateways associated with tunnels
- ✅ SSL VPN sessions mapped
- ✅ Tunnel status (active/inactive) captured
- ✅ Tests passing

---

### Sprint 3.1.5: Security Profile Mapping (4 hours)

**Objective**: Map security profiles (URL filtering, antivirus, anti-spyware) for security posture analysis

**Background**: PAN-OS security profiles provide Layer 7 threat protection. Mapping these enables compliance reporting and security posture assessment.

#### Tasks

**1. Add Security Profile Commands (0.5 hours)**
- Add to `getPaloAltoAuxCommands()` function

**Security Profile Commands (5)**:
```go
// URL filtering profiles
"/api/?type=config&action=show&xpath=/config/devices/entry[@name='localhost.localdomain']/vsys/entry[@name='vsys1']/profiles/url-filtering"

// Antivirus profiles
"/api/?type=config&action=show&xpath=/config/devices/entry[@name='localhost.localdomain']/vsys/entry[@name='vsys1']/profiles/virus"

// Anti-spyware profiles
"/api/?type=config&action=show&xpath=/config/devices/entry[@name='localhost.localdomain']/vsys/entry[@name='vsys1']/profiles/spyware"

// Vulnerability protection profiles
"/api/?type=config&action=show&xpath=/config/devices/entry[@name='localhost.localdomain']/vsys/entry[@name='vsys1']/profiles/vulnerability"

// File blocking profiles
"/api/?type=config&action=show&xpath=/config/devices/entry[@name='localhost.localdomain']/vsys/entry[@name='vsys1']/profiles/file-blocking"
```

**2. Create Security Profile Parsers (1.5 hours)**
- Add to `palo_alto.parsers.go` file (~250 lines)

```go
// 1. ParsePANOSURLFilteringProfiles
type PANOSURLFilteringProfile struct {
    Name        string   `xml:"name,attr"`
    Description string   `xml:"description"`
    Block       []string `xml:"block>member"`
    Alert       []string `xml:"alert>member"`
    Allow       []string `xml:"allow>member"`
    Continue    []string `xml:"continue>member"`
}

// 2. ParsePANOSAntivirusProfiles
type PANOSAntivirusProfile struct {
    Name        string                     `xml:"name,attr"`
    Description string                     `xml:"description"`
    Decoders    []PANOSAntivirusDecoder    `xml:"decoder>entry"`
}

type PANOSAntivirusDecoder struct {
    Name   string `xml:"name,attr"` // http, smtp, ftp, etc.
    Action string `xml:"action"`     // default, allow, alert, drop, reset-both
}

// 3. ParsePANOSAntiSpywareProfiles
type PANOSAntiSpywareProfile struct {
    Name        string                      `xml:"name,attr"`
    Description string                      `xml:"description"`
    Rules       []PANOSAntiSpywareRule      `xml:"rules>entry"`
}

type PANOSAntiSpywareRule struct {
    Name     string   `xml:"name,attr"`
    Severity []string `xml:"severity>member"` // critical, high, medium, low
    Category string   `xml:"category"`
    Action   string   `xml:"action>reset-both"` // or alert, drop
}

// 4. ParsePANOSVulnerabilityProfiles (similar to anti-spyware)
// 5. ParsePANOSFileBlockingProfiles
```

**3. Create Security Profile Mappers (1.5 hours)**
- Add to `palo_alto.mappers.go` file (~200 lines)

```go
// 1. buildPANOSSecurityProfileModel
// Maps security profiles → models.SecurityProfile (new model)
type SecurityProfile struct {
    ID             string
    Name           string
    Type           string // "url-filtering", "antivirus", "anti-spyware", "vulnerability", "file-blocking"
    Description    string
    Actions        map[string]string // severity/category → action
    VirtualContext string
    Metadata       map[string]string
}

// 2. buildPANOSSecurityPolicyProfileAssociation
// Updates FirewallPolicy with associated security profiles
// Adds profile names to FirewallPolicy.Metadata["security_profiles"]
```

**4. Enrich Security Policy Model (0.5 hours)**
- Modify `buildPANOSSecurityPolicyModel()` to include security profile associations
- Add profile names to FirewallPolicy metadata:
```go
policy.Metadata["url_filtering_profile"] = "strict-url-filtering"
policy.Metadata["antivirus_profile"] = "default-antivirus"
policy.Metadata["anti_spyware_profile"] = "strict-anti-spyware"
```

**5. Integration Testing (0.5 hours)**
- Test security profile parsing
- Verify profile-to-policy association
- Test security posture queries (e.g., "policies without antivirus")

**6. Documentation (0.5 hours)**
- Add security profile section to PALO_ALTO_PARSER_DOCUMENTATION.md
- Add security posture analysis examples to documentation

**Deliverables**:
- 5 new commands (security profiles)
- 5 new parsers (~250 lines)
- 2 new mappers (~200 lines)
- `models.SecurityProfile` model definition
- Enhanced FirewallPolicy with profile associations
- Integration tests (~100 lines)

**Success Criteria**:
- ✅ Security profiles parsed and mapped
- ✅ Profiles associated with firewall policies
- ✅ Security posture queries enabled
- ✅ Tests passing

---

## Phase 3.1 Summary

### Total Effort

| **Sprint** | **Deliverable** | **Hours** | **LOC** |
|------------|-----------------|-----------|---------|
| 3.1.1 | Panorama Support | 8 | ~700 |
| 3.1.2 | Dynamic Vsys Discovery | 4 | ~300 |
| 3.1.3 | BGP/OSPF Parser | 6 | ~650 |
| 3.1.4 | VPN Tunnel Mapping | 6 | ~850 |
| 3.1.5 | Security Profile Mapping | 4 | ~650 |
| **Total** | **Phase 3.1** | **28** | **~3,150** |

### New Commands Added

| **Category** | **Count** | **Total** |
|--------------|-----------|-----------|
| Panorama Commands | 10 | 10 |
| BGP/OSPF Commands | 7 | 7 |
| VPN Commands | 6 | 6 |
| Security Profile Commands | 5 | 5 |
| **Total New Commands** | **28** | **28** |

**Phase 3 Baseline**: 18 commands
**After Phase 3.1**: 46 commands (+156% increase)

### New Models Added

1. **models.DeviceGroup** - Panorama device groups
2. **models.RoutingNeighbor** - BGP/OSPF neighbors
3. **models.VPNTunnel** - IPsec/SSL VPN tunnels
4. **models.VPNSession** - Active VPN user sessions
5. **models.SecurityProfile** - Security profiles (URL filtering, antivirus, etc.)

### Code Growth

| **Metric** | **Phase 3** | **Phase 3.1** | **Total** | **Growth** |
|------------|-------------|---------------|-----------|------------|
| Commands | 18 | +28 | 46 | +156% |
| Parsers | 12 | +19 | 31 | +158% |
| Mappers | 9 | +13 | 22 | +144% |
| Model Types | 22 | +5 | 27 | +23% |
| Production LOC | ~2,100 | +3,150 | ~5,250 | +150% |
| Test LOC | ~2,300 | +1,500 | ~3,800 | +65% |

---

## Success Metrics

### Functionality

| **Feature** | **Target** | **Metric** |
|-------------|------------|------------|
| Panorama Support | ✅ Complete | Device groups, managed devices, shared objects |
| Dynamic Vsys | ✅ Complete | Automatic vsys iteration (1-10 vsys) |
| BGP Support | ✅ Complete | Neighbors, routes, AS-path |
| OSPF Support | ✅ Complete | Neighbors, routes, cost |
| IPsec VPN | ✅ Complete | Tunnels, gateways, status |
| SSL VPN | ✅ Complete | User sessions, client IPs |
| Security Profiles | ✅ Complete | URL filtering, antivirus, anti-spyware |

### Quality

| **Metric** | **Target** | **Actual** |
|------------|------------|------------|
| Test Coverage | 90%+ | TBD |
| Integration Tests | 100% passing | TBD |
| Documentation | 60+ pages | TBD |

---

## Integration with Existing System

### Before Phase 3.1

**PAN-OS Support**:
- ✅ Standalone firewall support
- ✅ Static routing only
- ✅ vsys1 hardcoded
- ❌ No Panorama support
- ❌ No BGP/OSPF support
- ❌ No VPN mapping
- ❌ No security profile analysis

### After Phase 3.1

**PAN-OS Support**:
- ✅ Standalone firewall support
- ✅ **Panorama centralized management** ← New
- ✅ **Dynamic vsys discovery (1-10 vsys)** ← New
- ✅ Static routing support
- ✅ **BGP/OSPF dynamic routing** ← New
- ✅ **IPsec/SSL VPN tunnels** ← New
- ✅ **Security profile analysis** ← New

---

## Future Enhancements (Phase 3.2)

**Out of Scope for Phase 3.1** (deferred to Phase 3.2 if needed):

1. **SD-WAN Support**: PAN-OS SD-WAN configuration and path quality metrics
2. **Decryption Policies**: SSL/TLS decryption policy analysis
3. **WildFire Analysis**: Advanced threat analysis with WildFire cloud
4. **Application Usage Reports**: Traffic analytics by application
5. **Threat Logs**: Real-time threat detection logs and analytics

---

## Recommendations

1. **Start with Sprint 3.1.1 (Panorama)**: High-value enhancement for enterprises with multiple firewalls
2. **Prioritize Sprint 3.1.2 (Dynamic Vsys)**: Quick win (4 hours) for multi-vsys environments
3. **BGP/OSPF (Sprint 3.1.3)**: Essential for datacenter fabric visibility
4. **VPN and Security Profiles**: Lower priority unless specific customer need

---

## Related Documents

- [PHASE_3_COMPLETION_SUMMARY.md](PHASE_3_COMPLETION_SUMMARY.md) - Phase 3 baseline
- [PHASE_3_PALO_ALTO_PLAN.md](PHASE_3_PALO_ALTO_PLAN.md) - Original Phase 3 plan
- [PALO_ALTO_PARSER_DOCUMENTATION.md](PALO_ALTO_PARSER_DOCUMENTATION.md) - Parser reference
- [PALO_ALTO_MAPPER_USAGE_GUIDE.md](PALO_ALTO_MAPPER_USAGE_GUIDE.md) - Mapper usage guide

---

**Document Version**: 1.0
**Status**: Planned
**Created**: 2025-10-29
**Author**: Phase 3.1 Planning
**Approval**: Pending Review

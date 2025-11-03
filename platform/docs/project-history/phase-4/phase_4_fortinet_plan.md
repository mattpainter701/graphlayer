# Phase 4: Fortinet FortiGate/FortiManager Support - Implementation Plan

**Phase**: 4 - Fortinet FortiGate/FortiManager Multi-Vendor Support
**Status**: 📋 **PLANNED**
**Dependencies**: Phase 3 (Complete ✅)
**Estimated Effort**: 50 hours
**Sprints**: 7 (4.1 → 4.7)

---

## Executive Summary

Phase 4 adds comprehensive support for **Fortinet FortiGate** firewalls and **FortiManager** centralized management to the multi-vendor topology modeling system. This phase delivers:

- ✅ **FortiGate Operational Commands** - REST API commands for FortiOS configuration and operational data
- ✅ **FortiGate Parsers** - JSON parsing for security policies, VDOM, routing, interfaces, objects
- ✅ **FortiGate Models** - Vendor-agnostic topology models
- ✅ **FortiGate Mappers** - Transform FortiOS config to common topology format
- ✅ **FortiManager Integration** - Multi-device management and policy packages
- ✅ **SD-WAN Support** - FortiGate SD-WAN topology and path quality

---

## Background: Fortinet FortiOS Architecture

### Key Differences from Palo Alto and Cisco

| Aspect | Cisco | Palo Alto | **Fortinet** |
|--------|-------|-----------|--------------|
| **Configuration Format** | CLI text | XML | JSON |
| **API** | SSH/NETCONF | XML API | REST API |
| **Security Model** | ACLs | App-ID + Zones | UTM + NGFW |
| **Multi-Tenancy** | VRF | vsys | **VDOM** (Virtual Domain) |
| **Management** | Standalone | Panorama | **FortiManager** |
| **SD-WAN** | Limited | Limited | **Native SD-WAN** |
| **Routing** | OSPF/BGP | OSPF/BGP | OSPF/BGP + **SD-WAN routing** |

### FortiOS Architecture Concepts

#### 1. Virtual Domains (VDOM)
**What**: Multi-tenancy mechanism similar to PAN-OS vsys and Cisco VRF, but more comprehensive
- Each VDOM is a complete virtual firewall instance
- Independent security policies, routing tables, interfaces, objects
- Up to 500 VDOMs per device (hardware dependent)
- **Types**:
  - **Root VDOM**: Management VDOM (default)
  - **Regular VDOM**: Customer/tenant VDOMs

**Example Use Cases**:
- Service provider: One VDOM per customer
- Enterprise: Separate VDOMs for prod, dev, DMZ
- MSP: Multi-tenant firewall management

#### 2. Security Policies
**IPv4 Policies**:
- Source/destination zones
- Source/destination addresses
- Services (ports)
- Actions: accept, deny, ipsec
- **UTM Profiles**: Antivirus, web filtering, application control, IPS

**IPv6 Policies**:
- Separate policy table for IPv6 traffic
- Same structure as IPv4 policies

**Central NAT Table**:
- Separate NAT policies (not embedded in security policies)
- Source NAT (IP pool, PAT)
- Destination NAT (VIP - Virtual IP)
- DNAT + SNAT in same rule (similar to PAN-OS)

#### 3. Security Zones
**Zone Types**:
- Interface-based zones (similar to PAN-OS)
- Policies defined between zones
- Common zones: wan, lan, dmz, vpn

#### 4. Objects
**Address Objects**:
- IPv4/IPv6 addresses (single, range, subnet)
- FQDN (domain names)
- Geography (country-based)
- Dynamic addresses (SDN connectors - AWS, Azure, etc.)

**Service Objects**:
- TCP/UDP/ICMP
- Custom protocols
- Service groups

#### 5. Interfaces
**Interface Types**:
- Physical: port1, port2, etc.
- VLAN: vlan.100, vlan.200
- Loopback: loopback1, loopback2
- Tunnel: ipsec tunnels, GRE tunnels
- **Aggregate**: Link aggregation (LACP)
- **SD-WAN**: SD-WAN virtual interfaces

#### 6. Routing
**Routing Protocols**:
- Static routes
- OSPF (OSPFv2, OSPFv3)
- BGP (iBGP, eBGP)
- RIP
- **SD-WAN routing** (performance-based routing)

**Policy-Based Routing (PBR)**:
- Route traffic based on source, destination, service
- Integration with SD-WAN

#### 7. SD-WAN
**FortiGate Native SD-WAN**:
- Performance-based path selection (latency, jitter, packet loss)
- Application steering (Office365 → best path)
- Health checks per SD-WAN member
- **SLA targets**: Latency < 100ms, jitter < 50ms, packet loss < 1%
- **Members**: Physical interfaces (wan1, wan2) or VPN tunnels

#### 8. VPN
**IPsec VPN**:
- Site-to-site VPN
- Dial-up (hub-and-spoke)
- **Phase 1**: IKE gateway
- **Phase 2**: IPsec tunnel
- **Auto-discovery VPN (ADVPN)**: Dynamic spoke-to-spoke tunnels

**SSL VPN**:
- Remote access VPN
- Web mode or tunnel mode
- User authentication (LDAP, RADIUS, local)

#### 9. FortiManager
**Centralized Management**:
- Manage 1000+ FortiGate devices
- **Policy Packages**: Centralized policy templates
- **Device Groups**: Logical grouping (by region, function, etc.)
- **Provisioning Templates**: Configuration templates
- **Firmware Management**: Centralized firmware updates

**Key Concepts**:
- **ADOM** (Administrative Domain): Logical container for devices
- **Policy Package**: Collection of security policies
- **Install Targets**: Devices that receive policy package

---

## Supported FortiGate Models

**Physical Appliances**:
- FortiGate 40F, 60F, 80F, 100F (branch/SMB)
- FortiGate 200F, 400F, 600F (mid-range)
- FortiGate 1000F, 1500F, 3000F (datacenter)
- FortiGate 7000 series (carrier-grade)

**Virtual Appliances**:
- FortiGate-VM (VMware, Hyper-V, KVM)
- FortiGate-VM for AWS, Azure, GCP

**FortiOS Versions**:
- 6.4.x
- 7.0.x
- 7.2.x
- 7.4.x (latest)

---

## Phase 4 Sprints

### Sprint 4.1: Research & Planning (10 hours)

**Objectives**:
- Research FortiOS REST API structure
- Document operational endpoints for topology data
- Compare FortiOS vs PAN-OS architecture
- Define parser requirements
- Create Phase 4 implementation plan

#### Tasks

**1. REST API Research (3 hours)**
- Document FortiOS REST API authentication (API key, session token)
- Identify configuration endpoints vs operational endpoints
- Document VDOM parameter handling (`vdom=root`, `vdom=customer1`)
- Test API with FortiGate sandbox or lab device

**API Authentication**:
```bash
# API Key (recommended)
GET https://<fortigate-ip>/api/v2/cmdb/system/interface
Authorization: Bearer <api-token>

# Session Token (alternative)
POST https://<fortigate-ip>/logincheck
username=admin&secretkey=<password>
```

**2. Command Mapping (3 hours)**
- Map FortiOS CLI commands to REST API endpoints
- Document JSON response structures
- Identify multi-VDOM command strategies

**Example Mapping**:
```bash
# CLI Command
show system interface

# REST API Endpoint
GET /api/v2/cmdb/system/interface

# JSON Response
{
  "http_method": "GET",
  "results": [
    {
      "name": "port1",
      "vdom": "root",
      "ip": "10.1.1.1 255.255.255.0",
      "status": "up",
      "type": "physical"
    }
  ],
  "vdom": "root",
  "path": "system",
  "name": "interface",
  "status": "success"
}
```

**3. Architecture Analysis (2 hours)**
- Compare FortiOS to PAN-OS and Cisco architectures
- Document key mapping challenges:
  - VDOM → VirtualContext mapping
  - Central NAT vs policy-embedded NAT
  - SD-WAN member → interface relationship
  - IPv4 + IPv6 policy tables
  - UTM profile associations

**4. Create Implementation Plan (2 hours)**
- Document sprint breakdown (4.2 → 4.7)
- Estimate hours per sprint
- Define success criteria
- Create PHASE_4_FORTINET_PLAN.md (this document)

**Deliverables**:
- ✅ PHASE_4_FORTINET_PLAN.md (~60 pages)
- FortiOS REST API endpoint catalog (~20 pages)
- Architecture comparison matrix

**Success Criteria**:
- ✅ REST API authentication working
- ✅ Sample JSON responses collected
- ✅ Mapping challenges documented
- ✅ Implementation plan approved

---

### Sprint 4.2: FortiGate Commands (6 hours)

**Objective**: Add FortiGate REST API commands to connector orchestrator

#### Tasks

**1. Add FortiGate Command Function (1 hour)**
- Create `getFortiGateAuxCommands()` function in connector_orchestrator.go
- Handle VDOM parameter injection for multi-VDOM devices

**2. Configuration Commands (2.5 hours)**
- Add 15 configuration commands

**System & VDOM Commands (3)**:
```go
// System information
GET /api/v2/cmdb/system/status

// VDOM list
GET /api/v2/cmdb/system/vdom

// Global configuration
GET /api/v2/cmdb/system/global
```

**Network Commands (4)**:
```go
// Interfaces
GET /api/v2/cmdb/system/interface?vdom={vdom}

// Zones
GET /api/v2/cmdb/system/zone?vdom={vdom}

// Static routes
GET /api/v2/cmdb/router/static?vdom={vdom}

// BGP configuration
GET /api/v2/cmdb/router/bgp?vdom={vdom}
```

**Security Policy Commands (3)**:
```go
// IPv4 security policies
GET /api/v2/cmdb/firewall/policy?vdom={vdom}

// IPv6 security policies
GET /api/v2/cmdb/firewall/policy6?vdom={vdom}

// Central NAT policies
GET /api/v2/cmdb/firewall/central-snat-map?vdom={vdom}
```

**Object Commands (3)**:
```go
// Address objects
GET /api/v2/cmdb/firewall/address?vdom={vdom}

// Address groups
GET /api/v2/cmdb/firewall/addrgrp?vdom={vdom}

// Service objects
GET /api/v2/cmdb/firewall/service/custom?vdom={vdom}
```

**SD-WAN Commands (2)**:
```go
// SD-WAN configuration
GET /api/v2/cmdb/system/sdwan?vdom={vdom}

// SD-WAN health checks
GET /api/v2/cmdb/system/sdwan/health-check?vdom={vdom}
```

**3. Operational Commands (2.5 hours)**
- Add 10 operational commands

**Interface Status (2)**:
```go
// Interface operational status
GET /api/v2/monitor/system/interface?vdom={vdom}

// Interface statistics
GET /api/v2/monitor/system/interface/select?vdom={vdom}
```

**Routing Status (3)**:
```go
// Routing table
GET /api/v2/monitor/router/ipv4?vdom={vdom}

// BGP neighbors
GET /api/v2/monitor/router/bgp/neighbors?vdom={vdom}

// OSPF neighbors
GET /api/v2/monitor/router/ospf/neighbors?vdom={vdom}
```

**VPN Status (2)**:
```go
// IPsec VPN status
GET /api/v2/monitor/vpn/ipsec?vdom={vdom}

// SSL VPN users
GET /api/v2/monitor/vpn/ssl?vdom={vdom}
```

**SD-WAN Status (2)**:
```go
// SD-WAN member status
GET /api/v2/monitor/system/sdwan/health-check?vdom={vdom}

// SD-WAN routes
GET /api/v2/monitor/system/sdwan/service?vdom={vdom}
```

**Session Status (1)**:
```go
// Active sessions
GET /api/v2/monitor/firewall/session?vdom={vdom}
```

**4. VDOM Parameter Handling (0.5 hours)**
- Implement dynamic VDOM injection for multi-VDOM commands
- Add VDOM tag to command metadata

**5. Testing (0.5 hours)**
- Test commands against FortiGate sandbox
- Validate JSON response formats
- Test multi-VDOM scenarios

**Deliverables**:
- `getFortiGateAuxCommands()` function (~350 lines)
- 25 REST API commands (15 config + 10 operational)

**Success Criteria**:
- ✅ All 25 commands return valid JSON responses
- ✅ VDOM parameter injection working
- ✅ Commands tested on FortiOS 7.0+

---

### Sprint 4.3: FortiGate Parsers (14 hours)

**Objective**: Parse FortiOS JSON configuration and operational data into structured Go types

#### Tasks

**1. Create Parser File (0.5 hours)**
- Create `fortigate.parsers.go` file
- Set up JSON parsing utilities
- Define common response structures

**Common Response Structure**:
```go
type FortiGateAPIResponse struct {
    HTTPMethod    string          `json:"http_method"`
    Results       json.RawMessage `json:"results"` // Can be array or object
    VDOM          string          `json:"vdom"`
    Path          string          `json:"path"`
    Name          string          `json:"name"`
    Status        string          `json:"status"`
    Serial        string          `json:"serial,omitempty"`
    Version       string          `json:"version,omitempty"`
    Build         int             `json:"build,omitempty"`
    HTTPStatus    int             `json:"http_status"`
}
```

**2. System & VDOM Parsers (2 hours)**
- 3 parsers (~150 lines)

```go
// 1. ParseFortiGateSystemInfo
type FortiGateSystemInfo struct {
    Hostname       string `json:"hostname"`
    Serial         string `json:"serial"`
    Version        string `json:"version"`
    Build          int    `json:"build"`
    Model          string `json:"model"`
    ModelDescription string `json:"model_description"`
    OpMode         string `json:"op_mode"` // nat, transparent
    VDOMEnabled    bool   `json:"vdom_enabled"`
    VDOMCount      int    `json:"vdom_count"`
}

// 2. ParseFortiGateVDOMs
type FortiGateVDOM struct {
    Name       string `json:"name"`
    Comment    string `json:"comment"`
    ClusterID  int    `json:"cluster_id"`
}

// 3. ParseFortiGateGlobalConfig
type FortiGateGlobalConfig struct {
    Hostname         string `json:"hostname"`
    AdminSport       int    `json:"admin_sport"`
    AdminSSHPort     int    `json:"admin_ssh_port"`
    Timezone         string `json:"timezone"`
    AdminMaintainer  string `json:"admin_maintainer"`
}
```

**3. Network Interface Parsers (2.5 hours)**
- 2 parsers (~200 lines)

```go
// 1. ParseFortiGateInterfaces (config)
type FortiGateInterface struct {
    Name            string   `json:"name"`
    VDOM            string   `json:"vdom"`
    Type            string   `json:"type"` // physical, vlan, loopback, tunnel, aggregate
    IP              string   `json:"ip"`
    Allowaccess     []string `json:"allowaccess"` // ping, https, ssh, etc.
    Alias           string   `json:"alias"`
    Interface       string   `json:"interface"` // Parent interface for VLAN
    VlanID          int      `json:"vlanid"`
    Status          string   `json:"status"` // up, down
    Role            string   `json:"role"`   // lan, wan, dmz
    Mode            string   `json:"mode"`   // static, dhcp
    DHCPRelayService string  `json:"dhcp_relay_service"`
    SecondaryIP     []FortiGateSecondaryIP `json:"secondaryip"`
}

type FortiGateSecondaryIP struct {
    ID              int    `json:"id"`
    IP              string `json:"ip"`
    Allowaccess     string `json:"allowaccess"`
}

// 2. ParseFortiGateInterfaceStatus (operational)
type FortiGateInterfaceStatus struct {
    Name       string `json:"name"`
    VDOM       string `json:"vdom"`
    Link       bool   `json:"link"`
    Speed      int    `json:"speed"`
    Duplex     int    `json:"duplex"` // 0=half, 1=full
    TxPackets  int64  `json:"tx_packets"`
    RxPackets  int64  `json:"rx_packets"`
    TxBytes    int64  `json:"tx_bytes"`
    RxBytes    int64  `json:"rx_bytes"`
    TxErrors   int64  `json:"tx_errors"`
    RxErrors   int64  `json:"rx_errors"`
    MacAddress string `json:"mac_address"`
}
```

**4. Zone Parser (1 hour)**
- 1 parser (~80 lines)

```go
// ParseFortiGateZones
type FortiGateZone struct {
    Name        string   `json:"name"`
    Description string   `json:"description"`
    Interfaces  []FortiGateZoneInterface `json:"interface"`
    Intrazone   string   `json:"intrazone"` // allow, deny
}

type FortiGateZoneInterface struct {
    InterfaceName string `json:"interface-name"`
}
```

**5. Address & Service Object Parsers (2.5 hours)**
- 4 parsers (~300 lines)

```go
// 1. ParseFortiGateAddresses
type FortiGateAddress struct {
    Name        string   `json:"name"`
    UUID        string   `json:"uuid"`
    Subnet      string   `json:"subnet"` // "10.1.1.0 255.255.255.0"
    Type        string   `json:"type"`   // ipmask, iprange, fqdn, geography, dynamic
    StartIP     string   `json:"start-ip"`
    EndIP       string   `json:"end-ip"`
    FQDN        string   `json:"fqdn"`
    Country     string   `json:"country"`
    Comment     string   `json:"comment"`
    Visibility  string   `json:"visibility"` // enable, disable
    AssociatedIntf string `json:"associated-interface"`
}

// 2. ParseFortiGateAddressGroups
type FortiGateAddressGroup struct {
    Name       string                      `json:"name"`
    UUID       string                      `json:"uuid"`
    Members    []FortiGateAddressGroupMember `json:"member"`
    Comment    string                      `json:"comment"`
    Visibility string                      `json:"visibility"`
}

type FortiGateAddressGroupMember struct {
    Name string `json:"name"`
}

// 3. ParseFortiGateServices
type FortiGateService struct {
    Name         string `json:"name"`
    Protocol     string `json:"protocol"` // TCP/UDP/SCTP, ICMP, IP
    TCPPortRange string `json:"tcp-portrange"`
    UDPPortRange string `json:"udp-portrange"`
    ICMPType     string `json:"icmptype"`
    ICMPCode     string `json:"icmpcode"`
    ProtocolNumber int  `json:"protocol-number"`
    Comment      string `json:"comment"`
    Visibility   string `json:"visibility"`
}

// 4. ParseFortiGateServiceGroups
type FortiGateServiceGroup struct {
    Name       string                         `json:"name"`
    Members    []FortiGateServiceGroupMember  `json:"member"`
    Comment    string                         `json:"comment"`
    Visibility string                         `json:"visibility"`
}

type FortiGateServiceGroupMember struct {
    Name string `json:"name"`
}
```

**6. Security Policy Parsers (3 hours)**
- 3 parsers (~400 lines)

```go
// 1. ParseFortiGateIPv4Policies
type FortiGateIPv4Policy struct {
    PolicyID        int      `json:"policyid"`
    Name            string   `json:"name"`
    UUID            string   `json:"uuid"`
    Srcintf         []FortiGatePolicyInterface `json:"srcintf"`
    Dstintf         []FortiGatePolicyInterface `json:"dstintf"`
    Srcaddr         []FortiGatePolicyAddress   `json:"srcaddr"`
    Dstaddr         []FortiGatePolicyAddress   `json:"dstaddr"`
    Service         []FortiGatePolicyService   `json:"service"`
    Action          string   `json:"action"` // accept, deny, ipsec
    Status          string   `json:"status"` // enable, disable
    Schedule        string   `json:"schedule"`
    Logtraffic      string   `json:"logtraffic"` // all, utm, disable
    Comments        string   `json:"comments"`
    NATEnabled      string   `json:"nat"` // enable, disable
    IPPool          string   `json:"ippool"`
    Poolname        []FortiGatePolicyPoolname `json:"poolname"`
    AVProfile       string   `json:"av-profile"`
    WebfilterProfile string  `json:"webfilter-profile"`
    ApplicationList string   `json:"application-list"`
    IPSensor        string   `json:"ips-sensor"`
}

type FortiGatePolicyInterface struct {
    Name string `json:"name"`
}

type FortiGatePolicyAddress struct {
    Name string `json:"name"`
}

type FortiGatePolicyService struct {
    Name string `json:"name"`
}

type FortiGatePolicyPoolname struct {
    Name string `json:"name"`
}

// 2. ParseFortiGateIPv6Policies (similar to IPv4)
type FortiGateIPv6Policy struct {
    PolicyID    int      `json:"policyid"`
    Name        string   `json:"name"`
    UUID        string   `json:"uuid"`
    Srcintf     []FortiGatePolicyInterface `json:"srcintf"`
    Dstintf     []FortiGatePolicyInterface `json:"dstintf"`
    Srcaddr6    []FortiGatePolicyAddress   `json:"srcaddr6"`
    Dstaddr6    []FortiGatePolicyAddress   `json:"dstaddr6"`
    Service     []FortiGatePolicyService   `json:"service"`
    Action      string   `json:"action"`
    Status      string   `json:"status"`
    Logtraffic  string   `json:"logtraffic"`
    Comments    string   `json:"comments"`
}

// 3. ParseFortiGateCentralNATPolicies
type FortiGateCentralNATPolicy struct {
    PolicyID    int      `json:"policyid"`
    UUID        string   `json:"uuid"`
    Type        string   `json:"type"` // snat, dnat
    Srcintf     []FortiGatePolicyInterface `json:"srcintf"`
    Dstintf     []FortiGatePolicyInterface `json:"dstintf"`
    OrigAddr    []FortiGatePolicyAddress   `json:"orig-addr"`
    NATIPPool   []FortiGatePolicyPoolname  `json:"nat-ippool"`
    DstAddr     []FortiGatePolicyAddress   `json:"dst-addr"`
    Protocol    int      `json:"protocol"` // 6=TCP, 17=UDP
    OrigPort    string   `json:"orig-port"`
    NATPort     string   `json:"nat-port"`
    Status      string   `json:"status"`
    Comments    string   `json:"comments"`
}
```

**7. Routing Parsers (2 hours)**
- 4 parsers (~250 lines)

```go
// 1. ParseFortiGateStaticRoutes
type FortiGateStaticRoute struct {
    SeqNum      int    `json:"seq_num"`
    Dst         string `json:"dst"`
    Gateway     string `json:"gateway"`
    Distance    int    `json:"distance"`
    Weight      int    `json:"weight"`
    Priority    int    `json:"priority"`
    Device      string `json:"device"`
    Comment     string `json:"comment"`
    Status      string `json:"status"`
    VDOM        string `json:"vdom"`
}

// 2. ParseFortiGateBGPConfig
type FortiGateBGPConfig struct {
    AS              int    `json:"as"`
    RouterID        string `json:"router-id"`
    Neighbors       []FortiGateBGPNeighbor `json:"neighbor"`
}

type FortiGateBGPNeighbor struct {
    IP                  string `json:"ip"`
    RemoteAS            int    `json:"remote-as"`
    RouteMapIn          string `json:"route-map-in"`
    RouteMapOut         string `json:"route-map-out"`
    UpdateSource        string `json:"update-source"`
    Weight              int    `json:"weight"`
    HoldtimeTimer       int    `json:"holdtime-timer"`
    KeepaliveTimer      int    `json:"keepalive-timer"`
}

// 3. ParseFortiGateBGPNeighborStatus (operational)
type FortiGateBGPNeighborStatus struct {
    NeighborIP      string `json:"neighbor_ip"`
    RemoteAS        int    `json:"remote_as"`
    State           string `json:"state"` // Established, Idle, etc.
    Uptime          int    `json:"uptime"`
    PrefixReceived  int    `json:"prefix_received"`
    PrefixSent      int    `json:"prefix_sent"`
}

// 4. ParseFortiGateRoutingTable (operational)
type FortiGateRoutingEntry struct {
    Type        string `json:"type"` // static, connected, bgp, ospf, sdwan
    Destination string `json:"destination"`
    Gateway     string `json:"gateway"`
    Distance    int    `json:"distance"`
    Metric      int    `json:"metric"`
    Interface   string `json:"interface"`
    VDOM        string `json:"vdom"`
}
```

**8. SD-WAN Parsers (1.5 hours)**
- 3 parsers (~200 lines)

```go
// 1. ParseFortiGateSDWANConfig
type FortiGateSDWANConfig struct {
    Status       string                     `json:"status"` // enable, disable
    Members      []FortiGateSDWANMember     `json:"members"`
    HealthCheck  []FortiGateSDWANHealthCheck `json:"health-check"`
    Service      []FortiGateSDWANService    `json:"service"`
}

type FortiGateSDWANMember struct {
    SeqNum      int    `json:"seq-num"`
    Interface   string `json:"interface"`
    Zone        string `json:"zone"`
    Gateway     string `json:"gateway"`
    Priority    int    `json:"priority"`
    Source      string `json:"source"`
    Cost        int    `json:"cost"`
    Weight      int    `json:"weight"`
    Status      string `json:"status"`
}

type FortiGateSDWANHealthCheck struct {
    Name         string   `json:"name"`
    Probe        string   `json:"probe-packets"` // enable, disable
    Server       []string `json:"server"`
    Protocol     string   `json:"protocol"` // ping, tcp-echo, http, https
    Port         int      `json:"port"`
    Interval     int      `json:"interval"`
    RecoverTime  int      `json:"recoverytime"`
    FailTime     int      `json:"failtime"`
    HALPriority  int      `json:"ha-priority"`
    SLALatency   int      `json:"sla>latency-threshold"`
    SLAJitter    int      `json:"sla>jitter-threshold"`
    SLAPacketLoss int     `json:"sla>packetloss-threshold"`
}

type FortiGateSDWANService struct {
    ID              int      `json:"id"`
    Name            string   `json:"name"`
    Mode            string   `json:"mode"` // manual, priority, sla
    Dst             []FortiGatePolicyAddress `json:"dst"`
    Src             []FortiGatePolicyAddress `json:"src"`
    PriorityMembers []FortiGateSDWANServiceMember `json:"priority-members"`
    SLA             []FortiGateSDWANServiceSLA    `json:"sla"`
}

type FortiGateSDWANServiceMember struct {
    SeqNum int `json:"seq-num"`
}

type FortiGateSDWANServiceSLA struct {
    HealthCheck  string `json:"health-check"`
    ID           int    `json:"id"`
}

// 2. ParseFortiGateSDWANHealthCheckStatus (operational)
type FortiGateSDWANHealthCheckStatus struct {
    Name         string                              `json:"name"`
    Members      []FortiGateSDWANHealthCheckMember   `json:"members"`
}

type FortiGateSDWANHealthCheckMember struct {
    SeqNum       int    `json:"seq_num"`
    Interface    string `json:"interface"`
    State        string `json:"state"` // alive, dead
    Latency      float64 `json:"latency"`
    Jitter       float64 `json:"jitter"`
    PacketLoss   float64 `json:"packet_loss"`
    PacketSent   int    `json:"packet_sent"`
    PacketReceived int  `json:"packet_received"`
}

// 3. ParseFortiGateSDWANServiceStatus (operational)
type FortiGateSDWANServiceStatus struct {
    ServiceID    int    `json:"service_id"`
    Name         string `json:"name"`
    ActiveMember int    `json:"active_member"`
    State        string `json:"state"`
}
```

**9. VPN Parsers (1.5 hours)**
- 2 parsers (~150 lines)

```go
// 1. ParseFortiGateIPsecVPNStatus
type FortiGateIPsecVPNStatus struct {
    Name         string `json:"name"`
    Type         string `json:"type"` // dialup, static
    Status       string `json:"status"` // up, down
    IncomingBytes int64 `json:"incoming_bytes"`
    OutgoingBytes int64 `json:"outgoing_bytes"`
    RemoteGateway string `json:"remote_gateway"`
}

// 2. ParseFortiGateSSLVPNUsers
type FortiGateSSLVPNUser struct {
    Username     string `json:"user_name"`
    RemoteHost   string `json:"remote_host"`
    TunnelIP     string `json:"tunnel_ip"`
    Duration     int    `json:"duration"`
    BytesSent    int64  `json:"bytes_sent"`
    BytesReceived int64 `json:"bytes_received"`
}
```

**10. Testing (1 hour)**
- Unit tests for all parsers (~500 lines)
- Test with sample FortiOS JSON responses
- Edge case testing (empty results, missing fields)

**Deliverables**:
- `fortigate.parsers.go` (~2,000 lines)
- 25 parsers (config + operational)
- 50+ data structures
- Parser unit tests (~500 lines)

**Success Criteria**:
- ✅ All 25 parsers tested with sample JSON
- ✅ Error handling validated
- ✅ Test coverage 90%+

---

### Sprint 4.4: FortiGate Models (2 hours)

**Objective**: Evaluate if existing common models support FortiGate entities or if new models needed

#### Tasks

**1. Model Gap Analysis (1 hour)**
- Review existing common models vs FortiGate entities
- Identify gaps requiring new models

**Existing Models (Reuse)**:
- ✅ `models.Device` - FortiGate firewall
- ✅ `models.VirtualContext` - VDOM (type: FORTINET_VDOM)
- ✅ `models.Zone` - Security zones
- ✅ `models.Interface` - Physical, VLAN, loopback, tunnel, aggregate
- ✅ `models.Address` / `models.AddressGroup` - Address objects
- ✅ `models.Service` / `models.ServiceGroup` - Service objects
- ✅ `models.FirewallPolicy` - IPv4/IPv6 security policies
- ✅ `models.NAT` - Central NAT policies
- ✅ `models.RoutingEntry` - Static routes, BGP routes, OSPF routes
- ✅ `models.RoutingNeighbor` - BGP/OSPF neighbors (from Phase 3.1)
- ✅ `models.VPNTunnel` - IPsec VPN (from Phase 3.1)
- ✅ `models.VPNSession` - SSL VPN users (from Phase 3.1)

**New Models Needed (3)**:
1. **models.SDWANMember** - SD-WAN member links
2. **models.SDWANHealthCheck** - SD-WAN SLA monitoring
3. **models.SDWANService** - SD-WAN traffic steering rules

**2. Define New SD-WAN Models (0.5 hours)**
```go
// models.SDWANMember
type SDWANMember struct {
    ID             string
    SeqNum         int
    Interface      string
    Zone           string
    Gateway        string
    Priority       int
    Cost           int
    Weight         int
    Status         string
    VirtualContext string
    Metadata       map[string]string
}

// models.SDWANHealthCheck
type SDWANHealthCheck struct {
    ID                string
    Name              string
    Protocol          string // ping, tcp-echo, http, https
    Server            string
    Port              int
    Interval          int
    FailTime          int
    RecoverTime       int
    SLALatency        int // ms
    SLAJitter         int // ms
    SLAPacketLoss     int // percentage
    VirtualContext    string
    Metadata          map[string]string
}

// models.SDWANService
type SDWANService struct {
    ID                string
    Name              string
    Mode              string // manual, priority, sla
    DestinationAddresses []string
    SourceAddresses   []string
    PriorityMembers   []int // SeqNums
    SLATargets        []SDWANServiceSLA
    ActiveMember      int
    VirtualContext    string
    Metadata          map[string]string
}

type SDWANServiceSLA struct {
    HealthCheckName string
    ID              int
}
```

**3. Document Model Decisions (0.5 hours)**
- Update model documentation with FortiGate mappings
- Document VDOM → VirtualContext strategy

**Deliverables**:
- 3 new SD-WAN model definitions (~100 lines)
- Updated model documentation

**Success Criteria**:
- ✅ All FortiGate entities map to common models
- ✅ SD-WAN models defined
- ✅ Model decisions documented

---

### Sprint 4.5: FortiGate Mappers (12 hours)

**Objective**: Transform FortiOS configuration to vendor-agnostic topology models

#### Tasks

**1. Create Mapper File (0.5 hours)**
- Create `fortigate.mappers.go` file
- Define `FortiGateProcessor` struct

```go
type FortiGateProcessor struct {
    // Input: Parsed FortiGate data
    systemInfo         FortiGateSystemInfo
    vdoms              []FortiGateVDOM
    zones              []FortiGateZone
    interfaces         []FortiGateInterface
    interfaceStatus    []FortiGateInterfaceStatus
    addresses          []FortiGateAddress
    addressGroups      []FortiGateAddressGroup
    services           []FortiGateService
    serviceGroups      []FortiGateServiceGroup
    ipv4Policies       []FortiGateIPv4Policy
    ipv6Policies       []FortiGateIPv6Policy
    centralNATPolicies []FortiGateCentralNATPolicy
    staticRoutes       []FortiGateStaticRoute
    bgpConfig          FortiGateBGPConfig
    bgpNeighborStatus  []FortiGateBGPNeighborStatus
    routingTable       []FortiGateRoutingEntry
    sdwanConfig        FortiGateSDWANConfig
    sdwanHealthStatus  []FortiGateSDWANHealthCheckStatus
    ipsecVPNStatus     []FortiGateIPsecVPNStatus
    sslVPNUsers        []FortiGateSSLVPNUser

    // Output: Vendor-agnostic topology models
    TopologyDevices              map[string]models.Device
    TopologyVirtualContexts      map[string]models.VirtualContext
    TopologyZones                map[string]models.Zone
    TopologyInterfaces           map[string]models.Interface
    TopologyAddresses            map[string]models.Address
    TopologyAddressGroups        map[string]models.AddressGroup
    TopologyServices             map[string]models.Service
    TopologyServiceGroups        map[string]models.ServiceGroup
    TopologyFirewallPolicies     map[string]models.FirewallPolicy
    TopologyNATs                 map[string]models.NAT
    TopologyRoutingEntries       map[string]models.RoutingEntry
    TopologyRoutingNeighbors     map[string]models.RoutingNeighbor
    TopologySDWANMembers         map[string]models.SDWANMember
    TopologySDWANHealthChecks    map[string]models.SDWANHealthCheck
    TopologySDWANServices        map[string]models.SDWANService
    TopologyVPNTunnels           map[string]models.VPNTunnel
    TopologyVPNSessions          map[string]models.VPNSession
    TopologyIntegrationMessages  []models.IntegrationMessage
}
```

**2. Device & VDOM Mappers (1.5 hours)**
- 2 mappers (~150 lines)

```go
// 1. buildFortiGateDeviceModel
// Maps system info → models.Device
// DeviceType: "firewall"
// Vendor: "fortinet"
// Model: "FortiGate-100F", "FortiGate-VM64", etc.

// 2. buildFortiGateVDOMModel
// Maps VDOMs → models.VirtualContext
// Type: "FORTINET_VDOM"
// Creates default VDOM if VDOM not enabled
```

**3. Zone & Interface Mappers (2 hours)**
- 2 mappers (~200 lines)

```go
// 1. buildFortiGateZoneModel
// Maps zones → models.Zone
// Type: "fortinet-zone"

// 2. buildFortiGateInterfaceModel
// Maps interfaces → models.Interface
// Types: "physical", "vlan", "loopback", "tunnel", "aggregate", "sdwan"
// Challenge: Parse IP address from "10.1.1.1 255.255.255.0" format
// Solution: Split and convert to CIDR (e.g., "10.1.1.1/24")
```

**Helper Function**:
```go
func parseFortiGateIPAddress(ipWithMask string) string {
    // "10.1.1.1 255.255.255.0" → "10.1.1.1/24"
    parts := strings.Split(ipWithMask, " ")
    if len(parts) != 2 {
        return ipWithMask
    }
    ip := parts[0]
    mask := parts[1]
    cidr := maskToCIDR(mask)
    return fmt.Sprintf("%s/%d", ip, cidr)
}
```

**4. Address & Service Mappers (2 hours)**
- 4 mappers (~300 lines)

```go
// 1. buildFortiGateAddressModel
// Maps addresses → models.Address
// Challenge: Subnet format "10.1.1.0 255.255.255.0" → "10.1.1.0/24"
// Types: ipmask, iprange, fqdn, geography
// Geography addresses: Store country in metadata

// 2. buildFortiGateAddressGroupModel
// Maps address groups → models.AddressGroup

// 3. buildFortiGateServiceModel
// Maps services → models.Service
// Challenge: Port range format "80-443" vs "80:443"
// Protocol: TCP/UDP/SCTP, ICMP, IP

// 4. buildFortiGateServiceGroupModel
// Maps service groups → models.ServiceGroup
```

**5. Security Policy Mapper (2.5 hours)**
- 1 mapper (~400 lines)

```go
// buildFortiGateSecurityPolicyModel
// Maps IPv4 + IPv6 policies → models.FirewallPolicy

// Challenges:
// 1. Interface-based policies (srcintf/dstintf) vs zone-based
//    Solution: Map interfaces to zones via findZoneForInterface()
//    Fallback: Use interface names directly if not in zone

// 2. Embedded NAT in policy vs central NAT
//    Solution: If policy.NATEnabled == "enable", create NAT entry
//    Store NAT pool name in metadata

// 3. IPv4 + IPv6 separate policy tables
//    Solution: Parse both, add ip_version metadata

// 4. UTM profile associations
//    Solution: Store in metadata:
//    - av_profile
//    - webfilter_profile
//    - application_list
//    - ips_sensor

// Example:
policy := models.FirewallPolicy{
    Name:                 fmt.Sprintf("policy-%d", fgPolicy.PolicyID),
    Enabled:              fgPolicy.Status == "enable",
    SourceZones:          mapInterfacesToZones(fgPolicy.Srcintf),
    DestinationZones:     mapInterfacesToZones(fgPolicy.Dstintf),
    SourceAddresses:      extractAddressNames(fgPolicy.Srcaddr),
    DestinationAddresses: extractAddressNames(fgPolicy.Dstaddr),
    Services:             extractServiceNames(fgPolicy.Service),
    Action:               mapFortiGateAction(fgPolicy.Action), // accept → allow, deny → deny, ipsec → vpn
    LoggingEnabled:       fgPolicy.Logtraffic != "disable",
    Metadata: map[string]string{
        "fortigate_policy_id":     fmt.Sprintf("%d", fgPolicy.PolicyID),
        "fortigate_uuid":          fgPolicy.UUID,
        "ip_version":              "ipv4", // or "ipv6"
        "av_profile":              fgPolicy.AVProfile,
        "webfilter_profile":       fgPolicy.WebfilterProfile,
        "application_list":        fgPolicy.ApplicationList,
        "ips_sensor":              fgPolicy.IPSensor,
        "nat_enabled":             fgPolicy.NATEnabled,
    },
}
```

**6. NAT Mapper (1.5 hours)**
- 1 mapper (~200 lines)

```go
// buildFortiGateNATModel
// Maps central NAT policies + embedded NAT → models.NAT

// Two sources of NAT:
// 1. Central NAT policies (firewall/central-snat-map)
// 2. Embedded NAT in security policies (nat=enable, ippool, poolname)

// Central NAT:
natEntry := models.NAT{
    Name:               fmt.Sprintf("central-nat-%d", natPolicy.PolicyID),
    Type:               mapFortiGateNATType(natPolicy.Type), // snat → source-nat-dynamic, dnat → destination-nat
    SourceZones:        mapInterfacesToZones(natPolicy.Srcintf),
    DestinationZones:   mapInterfacesToZones(natPolicy.Dstintf),
    OriginalSource:     extractAddressNames(natPolicy.OrigAddr),
    TranslatedSource:   extractPoolNames(natPolicy.NATIPPool),
    OriginalDestination: extractAddressNames(natPolicy.DstAddr),
    TranslatedPort:     natPolicy.NATPort,
}

// Embedded NAT (from security policy):
if policy.NATEnabled == "enable" {
    natEntry := models.NAT{
        Name:             fmt.Sprintf("policy-%d-nat", policy.PolicyID),
        Type:             "source-nat-dynamic",
        TranslatedSource: extractPoolNames(policy.Poolname),
    }
}
```

**7. Routing Mapper (1.5 hours)**
- 2 mappers (~250 lines)

```go
// 1. buildFortiGateRoutingModel
// Maps static routes + BGP routes + OSPF routes → models.RoutingEntry
// Protocol: "static", "bgp", "ospf", "sdwan"

// 2. buildFortiGateBGPNeighborModel
// Maps BGP neighbors → models.RoutingNeighbor
// Protocol: "bgp"
```

**8. SD-WAN Mappers (2 hours)**
- 3 mappers (~350 lines)

```go
// 1. buildFortiGateSDWANMemberModel
// Maps SD-WAN members → models.SDWANMember
// Links SD-WAN member to physical interface

// 2. buildFortiGateSDWANHealthCheckModel
// Maps SD-WAN health checks → models.SDWANHealthCheck
// Includes SLA targets (latency, jitter, packet loss)
// Includes operational status (alive/dead, actual metrics)

// 3. buildFortiGateSDWANServiceModel
// Maps SD-WAN services → models.SDWANService
// Mode: manual, priority, sla
// Links to health checks for SLA-based routing
// Includes active member selection
```

**9. VPN Mapper (1 hour)**
- 2 mappers (~150 lines)

```go
// 1. buildFortiGateIPsecVPNModel
// Maps IPsec VPN status → models.VPNTunnel
// Type: "ipsec"
// State: "active" or "inactive"

// 2. buildFortiGateSSLVPNModel
// Maps SSL VPN users → models.VPNSession
```

**10. Integration Testing (0.5 hours)**
- Test all mappers with sample FortiGate data
- Validate VDOM scoping
- Test SD-WAN member → interface association

**11. Documentation (0.5 hours)**
- Document mapper functions
- Provide mapping examples

**Deliverables**:
- `fortigate.mappers.go` (~2,000 lines)
- 17 mapper functions
- Mapper unit tests (~800 lines)

**Success Criteria**:
- ✅ All 17 mappers tested
- ✅ VDOM scoping working correctly
- ✅ SD-WAN topology mapped
- ✅ Test coverage 90%+

---

### Sprint 4.6: FortiManager Integration (4 hours)

**Objective**: Add support for FortiManager centralized management platform

#### Tasks

**1. FortiManager Detection (0.5 hours)**
- Detect FortiManager vs FortiGate device
- Check model name for "FortiManager" prefix

**2. FortiManager Commands (1 hour)**
- Add 8 FortiManager-specific commands

```go
// ADOM list
GET /api/v2/dvmdb/adom

// Device groups
GET /api/v2/dvmdb/adom/{adom}/group

// Managed devices
GET /api/v2/dvmdb/adom/{adom}/device

// Policy packages
GET /api/v2/pm/pkg/adom/{adom}

// Policy package details
GET /api/v2/pm/config/adom/{adom}/pkg/{pkg-name}/firewall/policy

// Install targets
GET /api/v2/pm/pkg/adom/{adom}/install/target

// Device status
GET /api/v2/dvmdb/adom/{adom}/device/{device}/vdom

// Firmware info
GET /api/v2/um/image/version
```

**3. FortiManager Parsers (1.5 hours)**
- Add to `fortimanager.parsers.go` file (~300 lines)
- 6 parsers

```go
// 1. ParseFortiManagerADOMs
type FortiManagerADOM struct {
    Name        string `json:"name"`
    Description string `json:"desc"`
    DeviceCount int    `json:"dev_count"`
}

// 2. ParseFortiManagerDeviceGroups
type FortiManagerDeviceGroup struct {
    Name    string   `json:"name"`
    Desc    string   `json:"desc"`
    Members []string `json:"member"`
}

// 3. ParseFortiManagerManagedDevices
type FortiManagerManagedDevice struct {
    Name         string `json:"name"`
    SerialNumber string `json:"sn"`
    IP           string `json:"ip"`
    Model        string `json:"platform_str"`
    Version      string `json:"os_ver"`
    ConnStatus   string `json:"conn_status"` // connected, disconnected
    ConfigStatus string `json:"conf_status"` // insync, outofsync
}

// 4. ParseFortiManagerPolicyPackages
type FortiManagerPolicyPackage struct {
    Name     string `json:"name"`
    Type     string `json:"type"` // pkg, folder
    ObjVer   int    `json:"obj ver"`
    Scope    []FortiManagerPolicyPackageScope `json:"scope member"`
}

type FortiManagerPolicyPackageScope struct {
    Name string `json:"name"`
    VDOM string `json:"vdom"`
}

// 5. ParseFortiManagerPolicyPackagePolicies (reuse FortiGate policy parser)

// 6. ParseFortiManagerInstallTargets
type FortiManagerInstallTarget struct {
    Device string `json:"dev"`
    VDOM   string `json:"vdom"`
}
```

**4. FortiManager Mappers (1 hour)**
- Add to `fortimanager.mappers.go` file (~200 lines)
- 3 mappers

```go
// 1. buildFortiManagerDeviceModel
// Maps FortiManager system info → models.Device
// DeviceType: "manager" (not "firewall")

// 2. buildFortiManagerADOMModel
// Maps ADOMs → models.DeviceGroup (or new models.ADOM)

// 3. buildFortiManagerManagedDeviceModel
// Maps managed devices → models.Device
// Add fortimanager_managed=true metadata
```

**5. Documentation (0.5 hours)**
- Add FortiManager section to documentation

**Deliverables**:
- 8 FortiManager commands
- `fortimanager.parsers.go` (~300 lines, 6 parsers)
- `fortimanager.mappers.go` (~200 lines, 3 mappers)

**Success Criteria**:
- ✅ FortiManager devices detected
- ✅ Managed devices discovered
- ✅ Policy packages parsed
- ✅ ADOMs mapped

---

### Sprint 4.7: Testing & Documentation (10 hours)

**Objective**: Create comprehensive tests and documentation for FortiGate/FortiManager support

#### Tasks

**1. Parser Unit Tests (3 hours)**
- Create `fortigate.parsers_test.go` (~1,500 lines)
- 60+ test cases for all 25 parsers
- Sample FortiOS JSON responses
- Edge case testing

**2. Mapper Unit Tests (3 hours)**
- Create `fortigate.mappers_test.go` (~1,500 lines)
- 50+ test cases for all 17 mappers
- Test VDOM scoping
- Test SD-WAN mapping
- Test interface-to-zone resolution

**3. Parser Documentation (2 hours)**
- Create `FORTIGATE_PARSER_DOCUMENTATION.md` (~60 pages)
- Document all 25 parsers with JSON examples
- Document FortiOS API response structures
- Usage examples

**4. Mapper Documentation (2 hours)**
- Create `FORTIGATE_MAPPER_USAGE_GUIDE.md` (~50 pages)
- Document all 17 mappers
- Mapping challenges and solutions (VDOM, interface-zone, subnet format, SD-WAN)
- Usage examples
- Integration patterns

**5. Phase 4 Completion Summary (1 hour)**
- Create `PHASE_4_COMPLETION_SUMMARY.md` (~25 pages)
- Sprint breakdown with metrics
- Code statistics
- Testing results
- Success metrics

**Deliverables**:
- `fortigate.parsers_test.go` (~1,500 lines)
- `fortigate.mappers_test.go` (~1,500 lines)
- `FORTIGATE_PARSER_DOCUMENTATION.md` (~60 pages)
- `FORTIGATE_MAPPER_USAGE_GUIDE.md` (~50 pages)
- `PHASE_4_COMPLETION_SUMMARY.md` (~25 pages)

**Success Criteria**:
- ✅ 90%+ test coverage for parsers
- ✅ 90%+ test coverage for mappers
- ✅ All tests passing
- ✅ Documentation complete

---

## Phase 4 Summary

### Total Effort

| **Sprint** | **Deliverable** | **Hours** | **LOC** |
|------------|-----------------|-----------|---------|
| 4.1 | Research & Planning | 10 | 0 |
| 4.2 | Commands | 6 | ~350 |
| 4.3 | Parsers | 14 | ~2,500 |
| 4.4 | Models | 2 | ~100 |
| 4.5 | Mappers | 12 | ~2,400 |
| 4.6 | FortiManager Integration | 4 | ~500 |
| 4.7 | Testing & Documentation | 10 | ~3,000 |
| **Total** | **Phase 4** | **58** | **~8,850** |

**Note**: Adjusted from initial 50-hour estimate to 58 hours due to SD-WAN and FortiManager complexity.

### Commands Added

| **Category** | **FortiGate** | **FortiManager** | **Total** |
|--------------|---------------|------------------|-----------|
| Configuration | 15 | 5 | 20 |
| Operational | 10 | 3 | 13 |
| **Total** | **25** | **8** | **33** |

### Parsers Created

| **Category** | **Count** | **LOC** |
|--------------|-----------|---------|
| System & VDOM | 3 | ~150 |
| Network | 3 | ~280 |
| Objects | 4 | ~300 |
| Security Policies | 3 | ~400 |
| Routing | 4 | ~250 |
| SD-WAN | 3 | ~200 |
| VPN | 2 | ~150 |
| FortiManager | 6 | ~300 |
| **Total** | **28** | **~2,030** |

### Mappers Created

| **Category** | **Count** | **LOC** |
|--------------|-----------|---------|
| Device & VDOM | 2 | ~150 |
| Zone & Interface | 2 | ~200 |
| Objects | 4 | ~300 |
| Security Policies | 1 | ~400 |
| NAT | 1 | ~200 |
| Routing | 2 | ~250 |
| SD-WAN | 3 | ~350 |
| VPN | 2 | ~150 |
| FortiManager | 3 | ~200 |
| **Total** | **20** | **~2,200** |

### New Models

1. **models.SDWANMember** - SD-WAN member links
2. **models.SDWANHealthCheck** - SD-WAN SLA monitoring
3. **models.SDWANService** - SD-WAN traffic steering rules

**Reused Models**: 15 (Device, VirtualContext, Zone, Interface, Address, AddressGroup, Service, ServiceGroup, FirewallPolicy, NAT, RoutingEntry, RoutingNeighbor, VPNTunnel, VPNSession, DeviceGroup)

---

## Key Mapping Challenges

### 1. VDOM to VirtualContext Mapping

**Challenge**: Fortinet VDOM is similar to PAN-OS vsys but with different configuration structure.

**Solution**:
- Map VDOM → `models.VirtualContext` with type `FORTINET_VDOM`
- Create "root" VDOM if VDOM not enabled
- Associate all objects/policies with VDOM name

**Example**:
```go
VirtualContext{Name: "root", Type: "FORTINET_VDOM"}
VirtualContext{Name: "customer1", Type: "FORTINET_VDOM"}
```

---

### 2. Interface-Based Policies to Zone-Based

**Challenge**: FortiGate policies use interface lists (srcintf/dstintf) instead of zones like PAN-OS.

**Solution**:
- Map interfaces to zones via `findZoneForInterface()` helper
- Fallback: Use interface names directly if not in zone
- Store original interface list in metadata

**Example**:
```go
// FortiGate Policy
Srcintf: [{"name": "port1"}, {"name": "port2"}]
Dstintf: [{"name": "port3"}]

// Mapped Policy
SourceZones: ["lan"] // if port1, port2 in "lan" zone
DestinationZones: ["wan"] // if port3 in "wan" zone

// Fallback
SourceZones: ["port1", "port2"] // if no zone assignment
```

---

### 3. Subnet Format Conversion

**Challenge**: FortiOS uses "IP subnet-mask" format: `"10.1.1.0 255.255.255.0"` instead of CIDR.

**Solution**:
- Create `parseFortiGateIPAddress()` helper function
- Convert subnet mask to CIDR notation: `255.255.255.0` → `/24`

**Example**:
```go
Input:  "10.1.1.0 255.255.255.0"
Output: "10.1.1.0/24"

Input:  "192.168.1.100 255.255.255.255"
Output: "192.168.1.100/32"
```

---

### 4. Embedded NAT vs Central NAT

**Challenge**: FortiGate has two NAT mechanisms:
1. **Embedded NAT**: NAT enabled in security policy (`nat=enable`, `ippool`, `poolname`)
2. **Central NAT**: Separate NAT policy table (`firewall/central-snat-map`)

**Solution**:
- Parse both sources
- Create separate `models.NAT` entries
- Use naming convention: `policy-{id}-nat` for embedded, `central-nat-{id}` for central

---

### 5. IPv4 + IPv6 Policy Tables

**Challenge**: FortiGate has separate policy tables for IPv4 and IPv6 traffic.

**Solution**:
- Parse both `firewall/policy` (IPv4) and `firewall/policy6` (IPv6)
- Add `ip_version` metadata field ("ipv4" or "ipv6")
- Both map to `models.FirewallPolicy`

---

### 6. SD-WAN Member to Interface Association

**Challenge**: SD-WAN members reference interfaces by name. Need to link SD-WAN health metrics to actual interfaces.

**Solution**:
- Store interface name in `SDWANMember.Interface` field
- Create lookup map: interface name → interface object
- Enrich interface model with SD-WAN member metadata

---

## Success Metrics

### Functionality

| **Feature** | **Target** | **Metric** |
|-------------|------------|------------|
| FortiGate Support | ✅ Complete | Device, VDOM, policies, NAT, routing, SD-WAN, VPN |
| FortiManager Support | ✅ Complete | ADOM, managed devices, policy packages |
| Multi-VDOM | ✅ Complete | Automatic VDOM iteration |
| SD-WAN | ✅ Complete | Members, health checks, services, SLA |
| IPv4 + IPv6 Policies | ✅ Complete | Both policy tables mapped |
| Central + Embedded NAT | ✅ Complete | Both NAT mechanisms mapped |

### Quality

| **Metric** | **Target** | **Actual** |
|------------|------------|------------|
| Test Coverage (Parsers) | 90%+ | TBD |
| Test Coverage (Mappers) | 90%+ | TBD |
| Integration Tests | 100% passing | TBD |
| Documentation Pages | 130+ | TBD |

---

## Integration with Existing System

### Before Phase 4

**Supported Vendors**:
- ✅ Cisco IOS/IOS-XE (Phase 1)
- ✅ Cisco NX-OS (Phase 2)
- ✅ Arista EOS (Phase 2)
- ✅ Palo Alto PAN-OS (Phase 3)

**Firewall Support**:
- ✅ PAN-OS security policies (zone-based, App-ID)
- ✅ PAN-OS NAT (source/destination)
- ✅ PAN-OS VPN (IPsec, SSL)
- ❌ No SD-WAN support
- ❌ No UTM profile analysis

### After Phase 4

**Supported Vendors**:
- ✅ Cisco IOS/IOS-XE (Phase 1)
- ✅ Cisco NX-OS (Phase 2)
- ✅ Arista EOS (Phase 2)
- ✅ Palo Alto PAN-OS (Phase 3)
- ✅ **Fortinet FortiGate/FortiManager (Phase 4)** ← New

**New Capabilities**:
- ✅ **FortiGate VDOM Support** - Multi-tenant firewall modeling
- ✅ **IPv4 + IPv6 Policies** - Dual-stack firewall policy analysis
- ✅ **Central + Embedded NAT** - Both NAT mechanisms
- ✅ **SD-WAN Topology** - Performance-based routing visualization
- ✅ **SD-WAN SLA Monitoring** - Latency, jitter, packet loss metrics
- ✅ **FortiManager Integration** - Centralized multi-device management
- ✅ **UTM Profile Analysis** - Antivirus, web filtering, IPS associations

---

## Vendor Comparison

| **Feature** | **Cisco** | **Palo Alto** | **Fortinet** |
|-------------|-----------|---------------|--------------|
| **API Format** | CLI (text) | XML | **JSON** |
| **Multi-Tenancy** | VRF | vsys | **VDOM** |
| **Security Policies** | ACLs | Zone + App-ID | **Interface/Zone + UTM** |
| **NAT** | Interface-based | Policy-based | **Central + Embedded** |
| **Policy Tables** | Single | Single | **IPv4 + IPv6 separate** |
| **SD-WAN** | Limited | Limited | **Native SD-WAN** |
| **Application Control** | NBAR | App-ID (2000+ apps) | **Application Control (5000+ apps)** |
| **Centralized Management** | - | Panorama | **FortiManager** |

---

## Future Enhancements (Phase 4.1)

**Out of Scope for Phase 4** (deferred to Phase 4.1 if needed):

1. **FortiAnalyzer Integration**: Log aggregation and analytics
2. **Security Fabric**: Fortinet Security Fabric topology
3. **FortiSwitch Integration**: Switch topology via FortiGate
4. **FortiAP Integration**: Wireless AP topology
5. **Threat Intelligence**: FortiGuard threat feed integration
6. **ZTNA (Zero Trust)**: FortiGate ZTNA policies

---

## Recommendations

1. **Start with Sprint 4.1 (Research)**: Critical for understanding FortiOS REST API
2. **Prioritize FortiGate (Sprint 4.2-4.5)**: Core firewall support before FortiManager
3. **SD-WAN is High-Value**: Differentiator from PAN-OS (native SD-WAN)
4. **FortiManager (Sprint 4.6)**: Add after FortiGate foundation complete
5. **Test with Real Devices**: Validate with FortiGate 100F-7000 series

---

## Related Documents

- [PHASE_3_COMPLETION_SUMMARY.md](PHASE_3_COMPLETION_SUMMARY.md) - PAN-OS baseline
- [PHASE_3_1_PANOS_ENHANCEMENTS_PLAN.md](PHASE_3_1_PANOS_ENHANCEMENTS_PLAN.md) - PAN-OS enhancements
- [PHASE_3_PALO_ALTO_PLAN.md](PHASE_3_PALO_ALTO_PLAN.md) - PAN-OS original plan

---

**Document Version**: 1.0
**Status**: Planned
**Created**: 2025-10-29
**Author**: Phase 4 Planning
**Approval**: Pending Review

# Auxiliary Commands Specification for All Device Types

**Purpose**: Define operational commands for L2/L3+ topology discovery across all supported platforms  
**Status**: Specification - Implementation Required  
**Date**: 2025-10-26

---

## Command Priority Levels

- **P0 Critical**: Essential for L2 topology, device adjacency, MAC placement
- **P1 High**: Security controls, compliance evidence
- **P2 Medium**: System info, diagnostics, performance

---

## 1. Cisco IOS (Reference Implementation - UPDATED FOR L2)

### P0 Critical - L2 Topology (11 commands)

```yaml
# MAC & FDB
show mac address-table dynamic:
  timeout: 15s
  priority: p0
  category: operational
  purpose: MAC-to-port bindings for L2 adjacency and host placement
  output_format: text
  l2_relevant: true

# Neighbor Discovery
show lldp neighbors detail:
  timeout: 15s
  priority: p0
  category: operational
  purpose: LLDP topology discovery
  output_format: text
  l2_relevant: true
  topology_relevant: true

show cdp neighbors detail:
  timeout: 15s
  priority: p0
  category: operational
  purpose: CDP topology discovery (fallback)
  output_format: text
  l2_relevant: true
  topology_relevant: true

# VLAN & Interfaces
show vlan:
  timeout: 10s
  priority: p0
  category: operational
  purpose: VLAN operational state and port assignments
  output_format: text
  l2_relevant: true

show interfaces status:
  timeout: 15s
  priority: p0
  category: operational
  purpose: Interface operational status
  output_format: text
  l2_relevant: true

show interfaces switchport:
  timeout: 15s
  priority: p0
  category: operational
  purpose: Switchport mode and VLAN assignment
  output_format: text
  l2_relevant: true

show interfaces trunk:
  timeout: 10s
  priority: p0
  category: operational
  purpose: Trunk configuration and allowed VLANs
  output_format: text
  l2_relevant: true

# Port Channels
show etherchannel summary:
  timeout: 10s
  priority: p0
  category: operational
  purpose: LAG membership and status
  output_format: text
  l2_relevant: true

# Spanning Tree
show spanning-tree:
  timeout: 20s
  priority: p0
  category: operational
  purpose: STP topology and port roles
  output_format: text
  l2_relevant: true
  topology_relevant: true

# ARP
show ip arp:
  timeout: 15s
  priority: p0
  category: operational
  purpose: ARP table for IP-to-MAC correlation
  output_format: text
  l2_relevant: true
  l3_relevant: true

# Routing
show ip route:
  timeout: 15s
  priority: p0
  category: operational
  purpose: Routing table for L3 topology
  output_format: text
  l3_relevant: true
```

### P0 Critical - L2 Security Controls (6 commands - NEW)

```yaml
# DHCP Snooping
show ip dhcp snooping binding:
  timeout: 15s
  priority: p0
  category: operational
  purpose: DHCP snooping bindings for host placement confidence scoring
  output_format: text
  l2_relevant: true
  security_relevant: true
  confidence_scoring: true

# Dynamic ARP Inspection
show ip arp inspection statistics:
  timeout: 10s
  priority: p0
  category: operational
  purpose: DAI statistics and operational state
  output_format: text
  l2_relevant: true
  security_relevant: true

# 802.1X and MAB Sessions
show authentication sessions:
  timeout: 15s
  priority: p0
  category: operational
  purpose: 802.1X/MAB session state for Allowed vs Conditional decisions
  output_format: text
  l2_relevant: true
  security_relevant: true
  decision_critical: true

# Port Security
show port-security:
  timeout: 10s
  priority: p0
  category: operational
  purpose: Port security status and violations
  output_format: text
  l2_relevant: true
  security_relevant: true

show port-security interface:
  timeout: 15s
  priority: p0
  category: operational
  purpose: Per-interface port security details
  output_format: text
  l2_relevant: true
  security_relevant: true

# IP Source Guard
show ip verify source:
  timeout: 10s
  priority: p0
  category: operational
  purpose: IPSG bindings and enforcement status
  output_format: text
  l2_relevant: true
  security_relevant: true
```

### P1 High - L2 Advanced (3 commands)

```yaml
# PVLAN (Private VLAN)
show vlan private-vlan:
  timeout: 10s
  priority: p1
  category: operational
  purpose: PVLAN mappings for isolation modeling
  output_format: text
  l2_relevant: true

# STP Security
show spanning-tree interface detail:
  timeout: 15s
  priority: p1
  category: operational
  purpose: Per-interface STP guards (BPDU Guard, Root Guard)
  output_format: text
  l2_relevant: true

# System Info
show version:
  timeout: 10s
  priority: p1
  category: operational
  purpose: System version and model for parser selection
  output_format: text
  device_info: true
```

---

## 2. Cisco ASA (Firewall)

### P0 Critical - Security Topology (8 commands)

```yaml
# Interface Status & Configuration
show interface:
  timeout: 15s
  priority: p0
  category: operational
  purpose: Interface status, IPs, security levels
  output_format: text
  l2_relevant: false
  l3_relevant: true

show nameif:
  timeout: 5s
  priority: p0
  category: operational
  purpose: Interface name mappings and security zones
  output_format: text
  l3_relevant: true

# Routing & ARP
show route:
  timeout: 10s
  priority: p0
  category: operational
  purpose: Routing table for L3 topology
  output_format: text
  l3_relevant: true

show arp:
  timeout: 10s
  priority: p0
  category: operational
  purpose: ARP table for IP-to-MAC binding
  output_format: text
  l2_relevant: true
  l3_relevant: true

# NAT & Translation
show xlate:
  timeout: 15s
  priority: p0
  category: operational
  purpose: Active NAT translations
  output_format: text
  l3_relevant: true

show nat:
  timeout: 10s
  priority: p0
  category: operational
  purpose: NAT configuration summary
  output_format: text
  l3_relevant: true

# Connection State
show conn:
  timeout: 20s
  priority: p0
  category: operational
  purpose: Active connections for traffic flow analysis
  output_format: text
  l3_relevant: true

show local-host:
  timeout: 10s
  priority: p0
  category: operational
  purpose: Hosts behind ASA with their connections
  output_format: text
  l3_relevant: true
```

### P1 High - Security & ACLs (5 commands)

```yaml
show access-list:
  timeout: 20s
  priority: p1
  category: security
  purpose: ACL hit counts and active rules
  output_format: text
  security_relevant: true

show service-policy:
  timeout: 10s
  priority: p1
  category: security
  purpose: Applied service policies per interface
  output_format: text
  security_relevant: true

show failover:
  timeout: 5s
  priority: p1
  category: operational
  purpose: HA status and sync state
  output_format: text
  ha_relevant: true

show vpn-sessiondb:
  timeout: 10s
  priority: p1
  category: security
  purpose: Active VPN sessions
  output_format: text
  security_relevant: true

show object-group:
  timeout: 10s
  priority: p1
  category: security
  purpose: Network/service object groups
  output_format: text
  l3_relevant: true
```

---

## 2. Cisco NXOS (Data Center Switch)

### P0 Critical - L2 Topology (11 commands)

```yaml
# MAC & FDB
show mac address-table dynamic:
  timeout: 15s
  priority: p0
  category: operational
  purpose: MAC-to-port bindings for L2 adjacency
  output_format: text
  l2_relevant: true

show mac address-table count:
  timeout: 5s
  priority: p0
  category: operational
  purpose: MAC table utilization
  output_format: text
  l2_relevant: true

# Neighbor Discovery
show lldp neighbors detail:
  timeout: 15s
  priority: p0
  category: operational
  purpose: LLDP topology discovery
  output_format: text
  l2_relevant: true
  topology_relevant: true

show cdp neighbors detail:
  timeout: 15s
  priority: p0
  category: operational
  purpose: CDP topology discovery (fallback)
  output_format: text
  l2_relevant: true
  topology_relevant: true

# VLAN & Interfaces
show vlan:
  timeout: 10s
  priority: p0
  category: operational
  purpose: VLAN configuration and port assignments
  output_format: text
  l2_relevant: true

show interface status:
  timeout: 15s
  priority: p0
  category: operational
  purpose: Interface operational status
  output_format: text
  l2_relevant: true

show interface switchport:
  timeout: 15s
  priority: p0
  category: operational
  purpose: Switchport mode and VLAN assignment
  output_format: text
  l2_relevant: true

show interface trunk:
  timeout: 10s
  priority: p0
  category: operational
  purpose: Trunk configuration and allowed VLANs
  output_format: text
  l2_relevant: true

# Port Channels
show port-channel summary:
  timeout: 10s
  priority: p0
  category: operational
  purpose: LAG membership and status
  output_format: text
  l2_relevant: true

# Spanning Tree
show spanning-tree:
  timeout: 20s
  priority: p0
  category: operational
  purpose: STP topology and port roles
  output_format: text
  l2_relevant: true
  topology_relevant: true

# ARP
show ip arp:
  timeout: 15s
  priority: p0
  category: operational
  purpose: ARP table for IP-to-MAC correlation
  output_format: text
  l2_relevant: true
  l3_relevant: true
```

### P0 Critical - L2 Security Controls (6 commands - NEW)

```yaml
# DHCP Snooping
show ip dhcp snooping binding:
  timeout: 15s
  priority: p0
  category: operational
  purpose: DHCP snooping bindings for host placement confidence scoring
  output_format: text
  l2_relevant: true
  security_relevant: true
  confidence_scoring: true

# Dynamic ARP Inspection
show ip arp inspection statistics:
  timeout: 10s
  priority: p0
  category: operational
  purpose: DAI statistics and operational state
  output_format: text
  l2_relevant: true
  security_relevant: true

# 802.1X and MAB Sessions
show authentication sessions:
  timeout: 15s
  priority: p0
  category: operational
  purpose: 802.1X/MAB session state for Allowed vs Conditional decisions
  output_format: text
  l2_relevant: true
  security_relevant: true
  decision_critical: true

# Port Security
show port-security:
  timeout: 10s
  priority: p0
  category: operational
  purpose: Port security status and violations
  output_format: text
  l2_relevant: true
  security_relevant: true

show port-security interface:
  timeout: 15s
  priority: p0
  category: operational
  purpose: Per-interface port security details
  output_format: text
  l2_relevant: true
  security_relevant: true

# IP Source Guard
show ip verify source:
  timeout: 10s
  priority: p0
  category: operational
  purpose: IPSG bindings and enforcement status
  output_format: text
  l2_relevant: true
  security_relevant: true
```

### P1 High - VPC & Fabric (7 commands)

```yaml
# PVLAN (Private VLAN)
show vlan private-vlan:
  timeout: 10s
  priority: p1
  category: operational
  purpose: PVLAN mappings for isolation modeling
  output_format: text
  l2_relevant: true

show vpc:
  timeout: 10s
  priority: p1
  category: operational
  purpose: Virtual Port Channel status (NXOS-specific)
  output_format: text
  l2_relevant: true
  ha_relevant: true

show vpc consistency-parameters global:
  timeout: 10s
  priority: p1
  category: operational
  purpose: VPC configuration consistency
  output_format: text
  l2_relevant: true

show vdc:
  timeout: 5s
  priority: p1
  category: operational
  purpose: Virtual Device Context configuration
  output_format: text
  context_relevant: true

show fabricpath isis adjacency:
  timeout: 10s
  priority: p1
  category: operational
  purpose: FabricPath IS-IS neighbors (if enabled)
  output_format: text
  l2_relevant: true

show hsrp:
  timeout: 10s
  priority: p1
  category: operational
  purpose: HSRP status for L3 redundancy
  output_format: text
  l3_relevant: true
  ha_relevant: true

show vpc role:
  timeout: 5s
  priority: p1
  category: operational
  purpose: VPC role (primary/secondary)
  output_format: text
  ha_relevant: true
```

---

## 3. Palo Alto PA-Series (Next-Gen Firewall)

### P0 Critical - Security Topology (10 commands)

```yaml
# System & Interfaces
show system info:
  timeout: 10s
  priority: p0
  category: operational
  purpose: System version, serial, uptime
  output_format: text
  device_info: true

show interface all:
  timeout: 15s
  priority: p0
  category: operational
  purpose: All interface status and IPs
  output_format: text
  l3_relevant: true

show interface hardware:
  timeout: 10s
  priority: p0
  category: operational
  purpose: Physical interface status
  output_format: text
  l2_relevant: true

# Routing & ARP
show routing route:
  timeout: 15s
  priority: p0
  category: operational
  purpose: Routing table
  output_format: text
  l3_relevant: true

show arp all:
  timeout: 15s
  priority: p0
  category: operational
  purpose: ARP table for IP-to-MAC binding
  output_format: text
  l2_relevant: true
  l3_relevant: true

# NAT
show running nat-policy:
  timeout: 10s
  priority: p0
  category: operational
  purpose: Active NAT policies
  output_format: text
  l3_relevant: true

# Sessions
show session all:
  timeout: 20s
  priority: p0
  category: operational
  purpose: Active sessions for traffic analysis
  output_format: text
  l3_relevant: true
  performance_warning: Can be large output

show session info:
  timeout: 5s
  priority: p0
  category: operational
  purpose: Session table statistics
  output_format: text
  l3_relevant: true

# Zones
show zone:
  timeout: 5s
  priority: p0
  category: operational
  purpose: Security zone configuration
  output_format: text
  l3_relevant: true
  security_relevant: true

show routing protocol bgp summary:
  timeout: 10s
  priority: p0
  category: operational
  purpose: BGP neighbor status (if configured)
  output_format: text
  l3_relevant: true
```

### P1 High - Security & HA (7 commands)

```yaml
show high-availability all:
  timeout: 10s
  priority: p1
  category: operational
  purpose: HA status and sync state
  output_format: text
  ha_relevant: true

show vpn ipsec-sa:
  timeout: 10s
  priority: p1
  category: security
  purpose: IPSec VPN security associations
  output_format: text
  security_relevant: true

show vpn flow:
  timeout: 10s
  priority: p1
  category: security
  purpose: Active VPN tunnels
  output_format: text
  security_relevant: true

show user ip-user-mapping all:
  timeout: 15s
  priority: p1
  category: security
  purpose: User-ID mappings
  output_format: text
  security_relevant: true

show global-protect-gateway current-user:
  timeout: 10s
  priority: p1
  category: security
  purpose: GlobalProtect VPN users
  output_format: text
  security_relevant: true

show counter global filter severity drop:
  timeout: 10s
  priority: p1
  category: security
  purpose: Drop counters for security analysis
  output_format: text
  security_relevant: true

show running security-policy:
  timeout: 15s
  priority: p1
  category: security
  purpose: Active security policies with hit counts
  output_format: text
  security_relevant: true
```

---

## 4. Juniper Junos (Router/Switch)

### P0 Critical - L2/L3 Topology (12 commands)

```yaml
# MAC & Ethernet Switching
show ethernet-switching table:
  timeout: 15s
  priority: p0
  category: operational
  purpose: MAC address table
  output_format: text
  l2_relevant: true

show ethernet-switching interfaces:
  timeout: 10s
  priority: p0
  category: operational
  purpose: Switchport status and VLANs
  output_format: text
  l2_relevant: true

# Neighbor Discovery
show lldp neighbors:
  timeout: 15s
  priority: p0
  category: operational
  purpose: LLDP topology discovery
  output_format: text
  l2_relevant: true
  topology_relevant: true

show lldp neighbors detail:
  timeout: 15s
  priority: p0
  category: operational
  purpose: Detailed LLDP information
  output_format: text
  l2_relevant: true
  topology_relevant: true

# Interfaces
show interfaces terse:
  timeout: 10s
  priority: p0
  category: operational
  purpose: Interface summary
  output_format: text
  l2_relevant: true
  l3_relevant: true

show interfaces descriptions:
  timeout: 10s
  priority: p0
  category: operational
  purpose: Interface descriptions
  output_format: text
  l2_relevant: true

# VLANs
show vlans:
  timeout: 10s
  priority: p0
  category: operational
  purpose: VLAN configuration
  output_format: text
  l2_relevant: true

# LAG
show lacp interfaces:
  timeout: 10s
  priority: p0
  category: operational
  purpose: Link aggregation status
  output_format: text
  l2_relevant: true

# Spanning Tree
show spanning-tree bridge:
  timeout: 10s
  priority: p0
  category: operational
  purpose: STP bridge information
  output_format: text
  l2_relevant: true

show spanning-tree interface:
  timeout: 15s
  priority: p0
  category: operational
  purpose: STP port roles and state
  output_format: text
  l2_relevant: true

# ARP & Routes
show arp no-resolve:
  timeout: 15s
  priority: p0
  category: operational
  purpose: ARP table
  output_format: text
  l2_relevant: true
  l3_relevant: true

show route:
  timeout: 15s
  priority: p0
  category: operational
  purpose: Routing table
  output_format: text
  l3_relevant: true
```

### P1 High - Security & Chassis (6 commands)

```yaml
show chassis hardware:
  timeout: 10s
  priority: p1
  category: operational
  purpose: Hardware inventory
  output_format: text
  device_info: true

show chassis cluster status:
  timeout: 10s
  priority: p1
  category: operational
  purpose: Chassis cluster HA status (SRX)
  output_format: text
  ha_relevant: true

show security zones:
  timeout: 10s
  priority: p1
  category: security
  purpose: Security zone configuration (SRX)
  output_format: text
  security_relevant: true

show security policies:
  timeout: 15s
  priority: p1
  category: security
  purpose: Security policies (SRX)
  output_format: text
  security_relevant: true

show security nat source rule all:
  timeout: 10s
  priority: p1
  category: security
  purpose: NAT rules (SRX)
  output_format: text
  l3_relevant: true

show security ike security-associations:
  timeout: 10s
  priority: p1
  category: security
  purpose: IKE VPN SAs (SRX)
  output_format: text
  security_relevant: true
```

---

## 5. Fortinet FortiGate (Firewall)

### P0 Critical - Security Topology (10 commands)

```yaml
# System
get system status:
  timeout: 10s
  priority: p0
  category: operational
  purpose: System information and version
  output_format: text
  device_info: true

# Interfaces
get system interface physical:
  timeout: 10s
  priority: p0
  category: operational
  purpose: Physical interface status
  output_format: text
  l2_relevant: true
  l3_relevant: true

show system interface:
  timeout: 10s
  priority: p0
  category: operational
  purpose: All interface configuration
  output_format: text
  l3_relevant: true

# Routing & ARP
get router info routing-table all:
  timeout: 15s
  priority: p0
  category: operational
  purpose: Routing table
  output_format: text
  l3_relevant: true

get system arp:
  timeout: 15s
  priority: p0
  category: operational
  purpose: ARP table
  output_format: text
  l2_relevant: true
  l3_relevant: true

# Sessions
get system session list:
  timeout: 20s
  priority: p0
  category: operational
  purpose: Active sessions
  output_format: text
  l3_relevant: true
  performance_warning: Can be very large

diagnose sys session stat:
  timeout: 5s
  priority: p0
  category: operational
  purpose: Session statistics
  output_format: text
  l3_relevant: true

# NAT
diagnose firewall ippool list:
  timeout: 10s
  priority: p0
  category: operational
  purpose: IP pool NAT usage
  output_format: text
  l3_relevant: true

# Policy
diagnose firewall policy list:
  timeout: 15s
  priority: p0
  category: security
  purpose: Firewall policies with hit counts
  output_format: text
  security_relevant: true

# Zones
show firewall zone:
  timeout: 5s
  priority: p0
  category: operational
  purpose: Security zones
  output_format: text
  security_relevant: true
```

### P1 High - HA & VPN (8 commands)

```yaml
get system ha status:
  timeout: 10s
  priority: p1
  category: operational
  purpose: HA cluster status
  output_format: text
  ha_relevant: true

diagnose sys ha status:
  timeout: 10s
  priority: p1
  category: operational
  purpose: Detailed HA synchronization
  output_format: text
  ha_relevant: true

get vpn ipsec tunnel summary:
  timeout: 10s
  priority: p1
  category: security
  purpose: IPSec VPN tunnel status
  output_format: text
  security_relevant: true

get vpn ssl monitor:
  timeout: 10s
  priority: p1
  category: security
  purpose: SSL VPN users
  output_format: text
  security_relevant: true

diagnose vpn tunnel list:
  timeout: 10s
  priority: p1
  category: security
  purpose: VPN tunnel details
  output_format: text
  security_relevant: true

get router info bgp summary:
  timeout: 10s
  priority: p1
  category: operational
  purpose: BGP neighbors (if configured)
  output_format: text
  l3_relevant: true

show system vdom:
  timeout: 5s
  priority: p1
  category: operational
  purpose: Virtual domain configuration
  output_format: text
  context_relevant: true

diagnose sys vd list:
  timeout: 5s
  priority: p1
  category: operational
  purpose: VDOM resource usage
  output_format: text
  context_relevant: true
```

---

## 6. Check Point Gaia (Firewall)

### P0 Critical - Security Topology (9 commands)

```yaml
# Interfaces
show interfaces all:
  timeout: 15s
  priority: p0
  category: operational
  purpose: Interface status and IPs
  output_format: text
  l3_relevant: true

show interface:
  timeout: 10s
  priority: p0
  category: operational
  purpose: Interface summary
  output_format: text
  l3_relevant: true

# Routing
show route:
  timeout: 15s
  priority: p0
  category: operational
  purpose: Routing table
  output_format: text
  l3_relevant: true

show arp dynamic all:
  timeout: 15s
  priority: p0
  category: operational
  purpose: ARP table
  output_format: text
  l2_relevant: true
  l3_relevant: true

# Clustering
show cluster state:
  timeout: 10s
  priority: p0
  category: operational
  purpose: Cluster HA status
  output_format: text
  ha_relevant: true

# NAT
fw ctl pstat:
  timeout: 10s
  priority: p0
  category: operational
  purpose: Firewall connection statistics
  output_format: text
  l3_relevant: true

fw tab -t connections -s:
  timeout: 15s
  priority: p0
  category: operational
  purpose: Connection table summary
  output_format: text
  l3_relevant: true

# VPN
vpn tu:
  timeout: 10s
  priority: p0
  category: security
  purpose: VPN tunnel status
  output_format: text
  security_relevant: true

show vpn tunnels:
  timeout: 10s
  priority: p0
  category: security
  purpose: VPN tunnel details
  output_format: text
  security_relevant: true
```

### P1 High - Security & Management (6 commands)

```yaml
show asset all:
  timeout: 10s
  priority: p1
  category: operational
  purpose: System hardware information
  output_format: text
  device_info: true

cphaprob state:
  timeout: 5s
  priority: p1
  category: operational
  purpose: High availability state
  output_format: text
  ha_relevant: true

fw ctl multik stat:
  timeout: 10s
  priority: p1
  category: operational
  purpose: Multi-queue kernel statistics
  output_format: text
  performance_relevant: true

show routed interfaces:
  timeout: 10s
  priority: p1
  category: operational
  purpose: Routed interface configuration
  output_format: text
  l3_relevant: true

show vsx:
  timeout: 10s
  priority: p1
  category: operational
  purpose: VSX virtual system status (if applicable)
  output_format: text
  context_relevant: true

show bgp peers:
  timeout: 10s
  priority: p1
  category: operational
  purpose: BGP neighbor status
  output_format: text
  l3_relevant: true
```

---

## 7. F5 BigIP (Load Balancer)

### P0 Critical - ADC Topology (11 commands)

```yaml
# System
show sys version:
  timeout: 10s
  priority: p0
  category: operational
  purpose: System version information
  output_format: text
  device_info: true

show sys hardware:
  timeout: 10s
  priority: p0
  category: operational
  purpose: Hardware platform details
  output_format: text
  device_info: true

# Interfaces
show net interface:
  timeout: 15s
  priority: p0
  category: operational
  purpose: Network interface status
  output_format: text
  l2_relevant: true
  l3_relevant: true

show net vlan:
  timeout: 10s
  priority: p0
  category: operational
  purpose: VLAN configuration
  output_format: text
  l2_relevant: true

show net self:
  timeout: 10s
  priority: p0
  category: operational
  purpose: Self IPs and floating IPs
  output_format: text
  l3_relevant: true

# Routing
show net route:
  timeout: 15s
  priority: p0
  category: operational
  purpose: Routing table
  output_format: text
  l3_relevant: true

show net arp:
  timeout: 15s
  priority: p0
  category: operational
  purpose: ARP table
  output_format: text
  l2_relevant: true
  l3_relevant: true

# Virtual Servers & Pools
show ltm virtual:
  timeout: 15s
  priority: p0
  category: operational
  purpose: Virtual server status and statistics
  output_format: text
  adc_relevant: true

show ltm pool:
  timeout: 15s
  priority: p0
  category: operational
  purpose: Pool member status
  output_format: text
  adc_relevant: true

show ltm node:
  timeout: 15s
  priority: p0
  category: operational
  purpose: Node (server) status
  output_format: text
  adc_relevant: true

# NAT
show ltm snat:
  timeout: 10s
  priority: p0
  category: operational
  purpose: SNAT configuration and usage
  output_format: text
  l3_relevant: true
```

### P1 High - HA & Persistence (7 commands)

```yaml
show cm device:
  timeout: 10s
  priority: p1
  category: operational
  purpose: Device trust and HA status
  output_format: text
  ha_relevant: true

show cm failover-status:
  timeout: 10s
  priority: p1
  category: operational
  purpose: Failover status and sync state
  output_format: text
  ha_relevant: true

show cm sync-status:
  timeout: 10s
  priority: p1
  category: operational
  purpose: Configuration sync status
  output_format: text
  ha_relevant: true

show ltm persistence persist-records:
  timeout: 15s
  priority: p1
  category: operational
  purpose: Active persistence records
  output_format: text
  adc_relevant: true

show sys connection:
  timeout: 20s
  priority: p1
  category: operational
  purpose: Active connections
  output_format: text
  l3_relevant: true
  performance_warning: Can be large

show net trunk:
  timeout: 10s
  priority: p1
  category: operational
  purpose: Link aggregation status
  output_format: text
  l2_relevant: true

show sys performance:
  timeout: 10s
  priority: p1
  category: operational
  purpose: System performance metrics
  output_format: text
  performance_relevant: true
```

---

## 8. Generic SSH Device

### P0 Critical - Basic Discovery (6 commands)

```yaml
# Try common show commands
show version:
  timeout: 10s
  priority: p0
  category: operational
  purpose: System version (universal)
  output_format: text
  device_info: true

show interfaces:
  timeout: 15s
  priority: p0
  category: operational
  purpose: Interface status (common)
  output_format: text
  l2_relevant: true
  l3_relevant: true

show ip route:
  timeout: 15s
  priority: p0
  category: operational
  purpose: Routing table (common)
  output_format: text
  l3_relevant: true

show arp:
  timeout: 15s
  priority: p0
  category: operational
  purpose: ARP table (common)
  output_format: text
  l2_relevant: true
  l3_relevant: true

show mac-address-table:
  timeout: 15s
  priority: p0
  category: operational
  purpose: MAC table (common for switches)
  output_format: text
  l2_relevant: true

show vlan:
  timeout: 10s
  priority: p0
  category: operational
  purpose: VLAN configuration (common)
  output_format: text
  l2_relevant: true
```

---

## Implementation Summary

### Command Count by Device Type

| Device Type | P0 Commands | P1 Commands | Total | Notes |
|-------------|-------------|-------------|-------|-------|
| **Cisco IOS** | 17 (11 basic + 6 L2 sec) | 3 | 20 (✅ Reference Implementation) | ⭐ Includes L2 security controls |
| **Cisco ASA** | 8 | 5 | 13 | Firewall focus |
| **Cisco NXOS** | 17 (11 basic + 6 L2 sec) | 7 (1 PVLAN + 6 VPC) | 24 | ⭐ Includes L2 security controls + VPC |
| **Palo Alto PA** | 10 | 7 | 17 | Next-gen firewall |
| **Juniper Junos/SRX** | 12 | 6 | 18 | Router/switch/firewall |
| **Fortinet FortiGate** | 10 | 8 | 18 | Firewall with VDOM |
| **Check Point Gaia** | 9 | 6 | 15 | Firewall cluster |
| **F5 BigIP** | 11 | 7 | 18 | ADC load balancer |
| **Generic SSH** | 6 | 0 | 6 | Fallback |

### Total Commands: **149 commands** across 9 device types (+27 from L2 security controls)

**L2 Security Control Commands Added**:
- Cisco IOS: +6 P0 commands (DHCP snooping, DAI, 802.1X, port-sec, IPSG)
- Cisco NXOS: +6 P0 commands + 1 P1 (PVLAN)

**Why These Matter**: These commands enable **Allowed/Conditional/Blocked** decision engine per `l2_project.md` requirements.

---

## Public Dataset References

### 1. Cisco Documentation
- **IOS Command Reference**: https://www.cisco.com/c/en/us/support/ios-nx-os-software/ios-15-4m-t/products-command-reference-list.html
- **ASA Command Reference**: https://www.cisco.com/c/en/us/td/docs/security/asa/asa-command-reference/
- **NXOS Command Reference**: https://www.cisco.com/c/en/us/support/switches/nexus-9000-series-switches/products-command-reference-list.html

### 2. Palo Alto Documentation
- **PAN-OS CLI Reference**: https://docs.paloaltonetworks.com/pan-os/10-2/pan-os-cli-quick-start
- **Command Reference Guide**: https://docs.paloaltonetworks.com/pan-os

### 3. Juniper Documentation
- **Junos CLI Explorer**: https://apps.juniper.net/cli-explorer/
- **Junos Command Reference**: https://www.juniper.net/documentation/product/us/en/junos-os/

### 4. Fortinet Documentation
- **FortiOS CLI Reference**: https://docs.fortinet.com/document/fortigate/7.4.0/cli-reference
- **FortiGate Handbook**: https://docs.fortinet.com/product/fortigate/

### 5. Check Point Documentation
- **Gaia CLI Reference**: https://sc1.checkpoint.com/documents/R81/WebAdminGuides/EN/CP_R81_Gaia_CLI_ReferenceGuide/
- **Check Point SK Articles**: https://supportcenter.checkpoint.com/

### 6. F5 Documentation
- **BIG-IP TMSH Reference**: https://techdocs.f5.com/en-us/bigip-15-0-0/big-ip-tmsh-reference.html
- **Configuration Guide**: https://techdocs.f5.com/kb/en-us/products/big-ip_ltm/

### 7. Public Configuration Samples
- **GitHub - Network Configuration Samples**: https://github.com/topics/network-configuration
- **PacketLife - Sample Configs**: https://packetlife.net/library/
- **NetworkLessons.com - Lab Configs**: https://networklessons.com/

### 8. Command Output Examples
- **Cisco Learning Network**: https://learningnetwork.cisco.com/
- **Juniper Day One Books**: https://www.juniper.net/documentation/jnbooks/us/en/day-one-books
- **Fortinet Training**: https://training.fortinet.com/

---

## Next Steps

1. ✅ Specification complete
2. ⏭️ Implement command execution in `connector_orchestrator.go`
3. ⏭️ Create operational parsers for each platform
4. ⏭️ Test with real device outputs
5. ⏭️ Create unit tests
6. ⏭️ Update documentation

---

**Status**: Ready for Implementation  
**Estimated Effort**: 8-12 weeks for all platforms  
**Priority Order**: ASA → NXOS → Palo Alto → Fortinet → Others


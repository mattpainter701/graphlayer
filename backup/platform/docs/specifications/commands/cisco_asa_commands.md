# Cisco ASA Auxiliary Commands Specification

**Purpose**: Define operational commands for Cisco ASA firewall topology and security policy discovery  
**Status**: Phase 1.2.1 - Implementation Ready  
**Date**: 2025-10-26

---

## Overview

This document specifies 13 operational commands for Cisco ASA firewalls that enable L3+ topology mapping, NAT discovery, firewall policy extraction, and failover state monitoring.

---

## Command Categories

- **P0 Critical** (8 commands): Essential for base topology and NAT mapping
- **P1 High** (5 commands): Security policy, failover, and VPN state

---

## P0 Critical Commands

### 1. Interface Status (P0)

```yaml
show interface:
  timeout: 15s
  priority: p0
  category: operational
  purpose: Interface operational state, IP addresses, and statistics
  output_format: text
  l3_relevant: true
  topology_relevant: true
```

**Expected Output**:
```
Interface GigabitEthernet0/0 "outside", is up, line protocol is up
  Hardware is i82546GB rev03, BW 1000 Mbps, DLY 10 usec
  MAC address 0050.56bb.e14c, MTU 1500
  IP address 192.168.1.1, subnet mask 255.255.255.0
```

**Parser Required**: `ParseASAInterface(output string) ([]ASAInterface, error)`

---

### 2. Routing Table (P0)

```yaml
show route:
  timeout: 10s
  priority: p0
  category: operational
  purpose: Active routing table for topology mapping
  output_format: text
  l3_relevant: true
  topology_relevant: true
```

**Expected Output**:
```
Codes: C - connected, S - static, I - IGRP, R - RIP, M - mobile, B - BGP
Gateway of last resort is 192.168.1.254 to network 0.0.0.0

C    192.168.1.0 255.255.255.0 is directly connected, outside
S    0.0.0.0 0.0.0.0 [1/0] via 192.168.1.254, outside
```

**Parser Required**: `ParseASARoute(output string) ([]ASARoute, error)`

---

### 3. ARP Table (P0)

```yaml
show arp:
  timeout: 10s
  priority: p0
  category: operational
  purpose: ARP table for MAC-to-IP mapping
  output_format: text
  topology_relevant: true
  confidence_scoring: true
```

**Expected Output**:
```
outside 192.168.1.254 0050.56bb.0001 3599
inside 10.0.0.10 00:11:22:33:44:55 3599
dmz 172.16.1.50 00:aa:bb:cc:dd:ee 3599
```

**Parser Required**: `ParseASAARP(output string) ([]ASAARPEntry, error)`

---

### 4. Address Translation (Xlate) Table (P0 - **CRITICAL FOR NAT**)

```yaml
show xlate:
  timeout: 15s
  priority: p0
  category: operational
  purpose: Active NAT translations (dynamic and static)
  output_format: text
  nat_relevant: true
  decision_critical: true
```

**Expected Output**:
```
2 in use, 2 most used
Flags: D - DNS, i - dynamic, r - portmap, s - static, I - identity, T - twice
NAT from inside:10.0.0.10 to outside:192.168.1.100
    flags s idle 0:00:05 timeout 0:00:00
NAT from inside:10.0.0.0/24 to outside:192.168.1.200
    flags i idle 0:00:12 timeout 3:00:00
```

**Parser Required**: `ParseASAXlate(output string) ([]ASAXlateEntry, error)`

---

### 5. NAT Configuration (P0)

```yaml
show nat:
  timeout: 10s
  priority: p0
  category: operational
  purpose: NAT policy rules and hit counts
  output_format: text
  nat_relevant: true
```

**Expected Output**:
```
Manual NAT Policies (Section 1)
1 (inside) to (outside) source static obj-10.0.0.10 obj-192.168.1.100
    translate_hits = 150, untranslate_hits = 145

Auto NAT Policies (Section 2)
1 (inside) to (outside) source dynamic inside-network interface
    translate_hits = 5000
```

**Parser Required**: `ParseASANAT(output string) ([]ASANATRule, error)`

---

### 6. Active Connections (P0)

```yaml
show conn:
  timeout: 20s
  priority: p0
  category: operational
  purpose: Active firewall connections
  output_format: text
  security_relevant: true
```

**Expected Output**:
```
TCP outside 192.168.1.50:54321 inside 10.0.0.10:443, idle 0:00:05, bytes 5000, flags UIO
UDP inside 10.0.0.10:53 outside 8.8.8.8:53, idle 0:00:01, bytes 150, flags -
```

**Parser Required**: `ParseASAConn(output string) ([]ASAConnection, error)`

---

### 7. Local-Host Table (P0)

```yaml
show local-host:
  timeout: 15s
  priority: p0
  category: operational
  purpose: Local host tracking with connection state
  output_format: text
  topology_relevant: true
```

**Expected Output**:
```
Interface inside: 50 active, 100 maximum active, 0 denied
local host: <10.0.0.10>,
  TCP flow count/limit = 10/unlimited
  UDP flow count/limit = 5/unlimited
  TCP embryonic count = 0
```

**Parser Required**: `ParseASALocalHost(output string) ([]ASALocalHost, error)`

---

### 8. Security Zones (Nameif) (P0)

```yaml
show nameif:
  timeout: 5s
  priority: p0
  category: operational
  purpose: Interface to security zone mapping
  output_format: text
  security_relevant: true
  zone_mapping: true
```

**Expected Output**:
```
Interface                Name                     Security
GigabitEthernet0/0       outside                  0
GigabitEthernet0/1       inside                   100
GigabitEthernet0/2       dmz                      50
```

**Parser Required**: `ParseASANameif(output string) ([]ASAZone, error)`

---

## P1 High Priority Commands

### 9. Access Control Lists (P1)

```yaml
show access-list:
  timeout: 20s
  priority: p1
  category: operational
  purpose: Firewall policy rules with hit counts
  output_format: text
  security_relevant: true
```

**Expected Output**:
```
access-list outside_access_in; 3 elements; name hash: 0x12345678
access-list outside_access_in line 1 extended permit tcp any host 192.168.1.100 eq https (hitcnt=150)
access-list outside_access_in line 2 extended deny ip any any (hitcnt=500)
```

**Parser Required**: `ParseASAACL(output string) ([]ASAACLEntry, error)`

---

### 10. Service Policy (P1)

```yaml
show service-policy:
  timeout: 15s
  priority: p1
  category: operational
  purpose: Applied QoS, inspection, and connection policies
  output_format: text
  security_relevant: true
```

**Expected Output**:
```
Global policy:
  Service-policy: global_policy
    Class-map: inspection_default
      Inspect: dns preset_dns_map, packet 1000, drop 0, reset-drop 0
      Inspect: http, packet 5000, drop 0, reset-drop 0
```

**Parser Required**: `ParseASAServicePolicy(output string) ([]ASAServicePolicy, error)`

---

### 11. Failover State (P1)

```yaml
show failover:
  timeout: 10s
  priority: p1
  category: operational
  purpose: High-availability state and sync status
  output_format: text
  ha_relevant: true
```

**Expected Output**:
```
Failover On
Failover unit Primary
Failover LAN Interface: Failover GigabitEthernet0/3 (up)
Unit Poll frequency 1 seconds, holdtime 15 seconds
Interface Poll frequency 5 seconds, holdtime 25 seconds
This host: Primary - Active
  Active time: 3600 (sec)
  Standby ready time: 300 (sec)
Other host: Secondary - Standby Ready
```

**Parser Required**: `ParseASAFailover(output string) (*ASAFailoverState, error)`

---

### 12. VPN Sessions (P1)

```yaml
show vpn-sessiondb:
  timeout: 15s
  priority: p1
  category: operational
  purpose: Active VPN sessions (IPsec and SSL)
  output_format: text
  security_relevant: true
```

**Expected Output**:
```
Session Type: IPsec
Username     : user1                  Index    : 1
Public IP    : 203.0.113.50           Protocol : IKEv2 IPsec
Session ID   : 1000                   Duration : 0h:30m:15s
Bytes Tx     : 50000                  Bytes Rx : 150000
```

**Parser Required**: `ParseASAVPN(output string) ([]ASAVPNSession, error)`

---

### 13. Object Groups (P1)

```yaml
show object-group:
  timeout: 10s
  priority: p1
  category: operational
  purpose: Network and service object groups used in policies
  output_format: text
  security_relevant: true
```

**Expected Output**:
```
object-group network inside-networks
 network-object 10.0.0.0 255.255.255.0
 network-object 10.1.0.0 255.255.255.0
object-group service web-services
 service-object tcp eq www
 service-object tcp eq https
```

**Parser Required**: `ParseASAObjectGroup(output string) ([]ASAObjectGroup, error)`

---

## Integration with Orchestrator

### Pseudo-code for Orchestrator Integration

```go
// In connector_orchestrator.go

func (o *Orchestrator) getCiscoASACommands() []AuxCommand {
    return []AuxCommand{
        // P0 Critical Commands
        {
            Command:  "show interface",
            Timeout:  15 * time.Second,
            Priority: "P0",
            Parser:   "ParseASAInterface",
        },
        {
            Command:  "show route",
            Timeout:  10 * time.Second,
            Priority: "P0",
            Parser:   "ParseASARoute",
        },
        {
            Command:  "show arp",
            Timeout:  10 * time.Second,
            Priority: "P0",
            Parser:   "ParseASAARP",
        },
        {
            Command:  "show xlate",
            Timeout:  15 * time.Second,
            Priority: "P0",
            Parser:   "ParseASAXlate",
        },
        {
            Command:  "show nat",
            Timeout:  10 * time.Second,
            Priority: "P0",
            Parser:   "ParseASANAT",
        },
        {
            Command:  "show conn",
            Timeout:  20 * time.Second,
            Priority: "P0",
            Parser:   "ParseASAConn",
        },
        {
            Command:  "show local-host",
            Timeout:  15 * time.Second,
            Priority: "P0",
            Parser:   "ParseASALocalHost",
        },
        {
            Command:  "show nameif",
            Timeout:  5 * time.Second,
            Priority: "P0",
            Parser:   "ParseASANameif",
        },
        
        // P1 High Priority Commands
        {
            Command:  "show access-list",
            Timeout:  20 * time.Second,
            Priority: "P1",
            Parser:   "ParseASAACL",
        },
        {
            Command:  "show service-policy",
            Timeout:  15 * time.Second,
            Priority: "P1",
            Parser:   "ParseASAServicePolicy",
        },
        {
            Command:  "show failover",
            Timeout:  10 * time.Second,
            Priority: "P1",
            Parser:   "ParseASAFailover",
        },
        {
            Command:  "show vpn-sessiondb",
            Timeout:  15 * time.Second,
            Priority: "P1",
            Parser:   "ParseASAVPN",
        },
        {
            Command:  "show object-group",
            Timeout:  10 * time.Second,
            Priority: "P1",
            Parser:   "ParseASAObjectGroup",
        },
    }
}
```

---

## Command Execution Flow

```
1. Connector establishes SSH session to Cisco ASA
2. Orchestrator retrieves auxiliary commands based on device type
3. For each ASA command:
   a. Execute command with timeout
   b. Capture output
   c. Call appropriate parser
   d. Store parsed results in ASA operational data structures
4. Map to generic topology models (VirtualContext, Zone, Interface, NAT, FirewallPolicy)
5. Store in database with config provenance
```

---

## Error Handling

### Command Not Supported
```go
if strings.Contains(output, "Invalid input") {
    log.Warnf("Command '%s' not supported on ASA version %s", cmd, osVersion)
    return nil, ErrCommandNotSupported
}
```

### Privilege Level Issues
```go
if strings.Contains(output, "% Insufficient privileges") {
    return nil, ErrInsufficientPrivileges
}
```

---

## Performance Considerations

### Command Execution Time Budget

| Command | Expected Duration | Timeout | Large Deployment (1000 connections) |
|---------|-------------------|---------|-------------------------------------|
| `show interface` | 2-5s | 15s | Up to 10s |
| `show route` | 1-3s | 10s | Up to 5s |
| `show arp` | 1-2s | 10s | Up to 5s |
| `show xlate` | 5-10s | 15s | Up to 30s (critical path) |
| `show nat` | 2-5s | 10s | Up to 10s |
| `show conn` | 10-20s | 20s | Up to 60s (critical path) |
| `show local-host` | 5-10s | 15s | Up to 20s |
| `show nameif` | <1s | 5s | <2s |
| `show access-list` | 5-15s | 20s | Up to 30s |
| `show service-policy` | 3-8s | 15s | Up to 15s |
| `show failover` | 1-3s | 10s | <5s |
| `show vpn-sessiondb` | 3-10s | 15s | Up to 20s |
| `show object-group` | 2-5s | 10s | Up to 10s |

**Total ASA Commands**: ~40-100 seconds (small deployment), up to 200 seconds (large deployment)

---

## Status

✅ **Phase 1.2.1 Spec Complete** - Ready for parser implementation  
⏭️ **Next**: Phase 1.2.2 - Implement P0 parsers

---

**Document Version**: 1.0  
**Last Updated**: 2025-10-26


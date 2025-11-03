# Cisco NXOS Auxiliary Commands Specification

**Purpose**: Define operational commands for Cisco Nexus NXOS L2+ topology discovery, VPC, and FabricPath  
**Status**: Phase 2.1.1 - Implementation Ready  
**Date**: 2025-10-26

---

## Overview

This document specifies **24 operational commands** for Cisco NXOS (Nexus switches) that enable comprehensive L2 topology mapping, VPC peer detection, FabricPath fabric discovery, and data center network visibility.

**Key Features**:
- **9 L2 Base Commands**: MAC, VLAN, interfaces, trunking, port-channels, STP, LLDP, CDP
- **5 VPC/Fabric Commands**: VPC peer state, consistency, VDC, FabricPath, HSRP
- **6 Context Commands**: System info, resources, inventory, feature status
- **4 L3+ Commands**: Routing, ARP, VRRP, interface counters

---

## Command Categories

- **P0 Critical** (10 commands): Essential for L2 topology and VPC state
- **P1 High** (8 commands): VPC consistency, FabricPath, HSRP, advanced L2
- **P2 Medium** (6 commands): Context, inventory, features

**Total**: 24 commands

---

## P0 Critical Commands (10)

### 1. MAC Address Table (P0)

```yaml
show mac address-table:
  timeout: 20s
  priority: p0
  category: operational
  purpose: Layer-2 forwarding table (FDB) for host placement
  output_format: text
  l2_relevant: true
  topology_relevant: true
  confidence_scoring: true
```

**Expected Output**:
```
Legend: 
        * - primary entry, G - Gateway MAC, (R) - Routed MAC, O - Overlay MAC
        age - seconds since last seen,+ - primary entry using vPC Peer-Link

   VLAN     MAC Address      Type      age     Secure NTFY Ports
---------+-----------------+--------+---------+------+----+------------------
*  10     0050.56bb.0001     dynamic  300        F      F    Eth1/1
*  10     0050.56bb.0002     dynamic  150        F      F    Po10
   20     0011.2233.4455     static   -          F      F    Eth1/5
```

**Parser Required**: `ParseNXOSMACTable(output string) ([]NXOSMACEntry, error)`

---

### 2. VLAN Information (P0)

```yaml
show vlan:
  timeout: 10s
  priority: p0
  category: operational
  purpose: VLAN configuration and port assignments
  output_format: text
  l2_relevant: true
```

**Expected Output**:
```
VLAN Name                             Status    Ports
---- -------------------------------- --------- -------------------------------
1    default                          active    Eth1/10, Eth1/11, Eth1/12
10   Engineering                      active    Eth1/1, Eth1/2, Po10
20   Operations                       active    Eth1/5, Eth1/6
```

**Parser Required**: `ParseNXOSVLAN(output string) ([]NXOSVLAN, error)`

---

### 3. Interface Status (P0)

```yaml
show interface status:
  timeout: 15s
  priority: p0
  category: operational
  purpose: Interface operational state, speed, duplex, VLAN assignments
  output_format: text
  l2_relevant: true
  topology_relevant: true
```

**Expected Output**:
```
Port          Name               Status    Vlan      Duplex  Speed   Type
Eth1/1        uplink-to-core     connected 10        full    10G     10Gbase-SR
Eth1/2        --                 connected trunk     full    10G     10Gbase-SR
Eth1/3        --                 notconnect 1        auto    auto    10Gbase-SR
Po10          VPC-Peer-Link      connected trunk     full    20G     --
```

**Parser Required**: `ParseNXOSInterfaceStatus(output string) ([]NXOSInterfaceStatus, error)`

---

### 4. Trunk Interface Details (P0)

```yaml
show interface trunk:
  timeout: 10s
  priority: p0
  category: operational
  purpose: Trunk port VLAN memberships and native VLAN
  output_format: text
  l2_relevant: true
  topology_relevant: true
```

**Expected Output**:
```
Port          Native  Status        Port
              Vlan                  Channel
------------  ------  ------------  -------
Eth1/2        1       trunking      --
Eth1/3        1       trunking      Po10
Eth1/4        1       trunking      Po10

Port          Vlans Allowed on Trunk
------------  ---------------------------------------------------------------------
Eth1/2        1,10,20,30,40,50
Eth1/3        1-4094
Eth1/4        1-4094
```

**Parser Required**: `ParseNXOSTrunk(output string) ([]NXOSTrunk, error)`

---

### 5. Port-Channel Summary (P0 - **VPC CRITICAL**)

```yaml
show port-channel summary:
  timeout: 10s
  priority: p0
  category: operational
  purpose: Port-channel (LAG) state and member interfaces
  output_format: text
  l2_relevant: true
  vpc_relevant: true
  topology_relevant: true
```

**Expected Output**:
```
Flags:  D - Down        P - Up in port-channel (members)
        I - Individual  H - Hot-standby (LACP only)
        s - Suspended   r - Module-removed
        S - Switched    R - Routed
        U - Up (port-channel)
        M - Not in use. Min-links not met
--------------------------------------------------------------------------------
Group Port-       Type     Protocol  Member Ports
      Channel
--------------------------------------------------------------------------------
10    Po10(SU)    Eth      LACP      Eth1/3(P)    Eth1/4(P)
20    Po20(SU)    Eth      LACP      Eth1/5(P)    Eth1/6(P)
```

**Parser Required**: `ParseNXOSPortChannel(output string) ([]NXOSPortChannel, error)`

---

### 6. Spanning Tree Summary (P0)

```yaml
show spanning-tree summary:
  timeout: 10s
  priority: p0
  category: operational
  purpose: STP mode, root bridge, and topology change counts
  output_format: text
  l2_relevant: true
  topology_relevant: true
```

**Expected Output**:
```
Switch is in rapid-pvst mode
Root bridge for: VLAN0010, VLAN0020
Extended system ID           : enabled
Portfast Default             : disabled
PortFast BPDU Guard Default  : disabled
Portfast BPDU Filter Default : disabled
Loopguard Default            : disabled
EtherChannel misconfig guard : enabled
UplinkFast                   : disabled
BackboneFast                 : disabled
Configured Pathcost method used is short
```

**Parser Required**: `ParseNXOSSTP(output string) (*NXOSSTPSummary, error)`

---

### 7. LLDP Neighbors (P0 - **TOPOLOGY CRITICAL**)

```yaml
show lldp neighbors detail:
  timeout: 20s
  priority: p0
  category: operational
  purpose: Layer-2 neighbor discovery for topology mapping
  output_format: text
  topology_relevant: true
```

**Expected Output**:
```
Capability codes:
  (R) Router, (B) Bridge, (T) Telephone, (C) DOCSIS Cable Device
  (W) WLAN Access Point, (P) Repeater, (S) Station, (O) Other

Chassis id: 0050.56bb.1234
Port id: Ethernet1/1
Local Port id: Eth1/1
Port Description: Connection to DIST-01
System Name: DIST-SWITCH-01
System Description: Cisco Nexus Operating System (NX-OS) Software 7.0(3)
Time remaining: 98 seconds
System Capabilities: B,R
Enabled Capabilities: B,R
Management Address: 192.168.1.10
Vlan ID: 10
```

**Parser Required**: `ParseNXOSLLDP(output string) ([]NXOSLLDPNeighbor, error)`

---

### 8. CDP Neighbors (P0)

```yaml
show cdp neighbors detail:
  timeout: 15s
  priority: p0
  category: operational
  purpose: Cisco Discovery Protocol neighbors for Cisco-to-Cisco topology
  output_format: text
  topology_relevant: true
```

**Expected Output**:
```
Device ID: CORE-SWITCH-01
Interface address(es):
    IPv4 Address: 10.0.0.1
Platform: cisco Nexus 9000 C93180YC-FX,  Capabilities: Router Switch IGMP Filtering 
Interface: Ethernet1/48,  Port ID (outgoing port): Ethernet1/1
Holdtime: 154 sec

Version:
Cisco Nexus Operating System (NX-OS) Software, Version 7.0(3)I7(6)

Native VLAN: 1
Duplex: full
Management address(es):
    IPv4 Address: 192.168.1.1
```

**Parser Required**: `ParseNXOSCDP(output string) ([]NXOSCDPNeighbor, error)`

---

### 9. VPC Status (P0 - **VPC CRITICAL**)

```yaml
show vpc:
  timeout: 15s
  priority: p0
  category: operational
  purpose: Virtual Port Channel (vPC) peer state and role
  output_format: text
  vpc_relevant: true
  ha_relevant: true
  decision_critical: true
```

**Expected Output**:
```
Legend:
                (*) - local vPC is down, forwarding via vPC peer-link

vPC domain id                     : 10
Peer status                       : peer adjacency formed ok
vPC keep-alive status             : peer is alive
Configuration consistency status  : success
Per-vlan consistency status       : success
Type-2 consistency status         : success
vPC role                          : secondary
Number of vPCs configured         : 5
Peer Gateway                      : Enabled
Dual-active excluded VLANs        : -
Graceful Consistency Check        : Enabled
Auto-recovery status              : Enabled, timer is off.(timeout = 240s)
Delay-restore status              : Timer is off.(timeout = 30s)
Delay-restore SVI status          : Timer is off.(timeout = 10s)
Operational Layer3 Peer-router    : Enabled

vPC Peer-link status
---------------------------------------------------------------------
id    Port   Status Active vlans
--    ----   ------ -------------------------------------------------
1     Po10   up     1,10,20,30,40,50
```

**Parser Required**: `ParseNXOSVPC(output string) (*NXOSVPCStatus, error)`

---

### 10. VPC Peer-Keepalive (P0)

```yaml
show vpc peer-keepalive:
  timeout: 10s
  priority: p0
  category: operational
  purpose: VPC peer-keepalive link health and reachability
  output_format: text
  vpc_relevant: true
  ha_relevant: true
```

**Expected Output**:
```
vPC keep-alive status           : peer is alive
--Peer is alive for             : (166978) seconds, (997) msec
--Send status                   : Success
--Last send at                  : 2023.Oct.26 12:15:30 123 ms
--Sent on interface             : mgmt0
--Receive status                : Success
--Last receive at               : 2023.Oct.26 12:15:30 456 ms
--Received on interface         : mgmt0
--Last update from peer         : (0) seconds, (123) msec

vPC Keep-alive parameters
--Destination                   : 192.168.100.2
--Keepalive interval            : 1000 msec
--Keepalive timeout             : 5 seconds
--Keepalive hold timeout        : 3 seconds
--Keepalive vrf                 : management
--Keepalive udp port            : 3200
--Keepalive tos                 : 192
```

**Parser Required**: `ParseNXOSVPCKeepalive(output string) (*NXOSVPCKeepalive, error)`

---

## P1 High Priority Commands (8)

### 11. VPC Consistency Parameters (P1 - **VPC CRITICAL**)

```yaml
show vpc consistency-parameters global:
  timeout: 15s
  priority: p1
  category: operational
  purpose: VPC configuration consistency validation
  output_format: text
  vpc_relevant: true
  decision_critical: true
```

**Expected Output**:
```
Legend:
        Type 1 : vPC will be suspended in case of mismatch

Name                        Type  Local Value            Peer Value             
-------------               ----  ---------------------- -----------------------
STP MST Simulate PVST       1     Enabled                Enabled                
STP Port Type, Edge         1     Normal, Disabled,      Normal, Disabled,      
BPDUFilter, Edge BPDUGuard        Disabled               Disabled               
STP MST Region Name         1     ""                     ""                     
STP Disabled                1     None                   None                   
STP Mode                    1     Rapid-PVST             Rapid-PVST             
STP Bridge Assurance        1     Enabled                Enabled                
STP Loopguard               1     Disabled               Disabled               
STP Global BPDU Guard       1     Enabled                Enabled
```

**Parser Required**: `ParseNXOSVPCConsistency(output string) ([]NXOSVPCConsistencyParam, error)`

---

### 12. VDC Information (P1)

```yaml
show vdc:
  timeout: 10s
  priority: p1
  category: operational
  purpose: Virtual Device Context (VDC) for multi-tenant visibility
  output_format: text
  context_relevant: true
```

**Expected Output**:
```
Switchwide mode: f-series

vdc_id  vdc_name                          state               mac                 type        lc      
------  --------                          -----               ----------          ----        ------  
1       PROD-VDC                          active              00:0c:29:f2:f8:2c   Ethernet    f3-line-card
2       TEST-VDC                          active              00:0c:29:f2:f8:2d   Ethernet    None    
```

**Parser Required**: `ParseNXOSVDC(output string) ([]NXOSVDC, error)`

---

### 13. FabricPath Topology (P1 - **FABRICPATH CRITICAL**)

```yaml
show fabricpath topology:
  timeout: 15s
  priority: p1
  category: operational
  purpose: FabricPath IS-IS topology for spine-leaf fabric
  output_format: text
  fabricpath_relevant: true
  topology_relevant: true
```

**Expected Output**:
```
Fabricpath IS-IS domain: default
  Fabricpath IS-IS tag: 0
  System ID: 0016.4748.ac4e  IS-Type: L1L2
  SAP: 400  Queue Handle: 16
  Maximum LSP MTU: 1492

Topology: 0
  Adjacency Database for Switch-ID 1:
  * indicates local switch, interfaces in bold are up

Switch-ID System-ID          Name             Port       Up Time            BW
      1   0016.4748.ac4e     Leaf-01         *local*     2d17h              --
     10   0050.56bb.1234     Spine-01         Eth1/1     2d17h              10000000
     11   0050.56bb.1235     Spine-02         Eth1/2     2d17h              10000000
     20   0050.56bb.2345     Leaf-02          Po10       2d17h              20000000
```

**Parser Required**: `ParseNXOSFabricPath(output string) (*NXOSFabricPathTopology, error)`

---

### 14. FabricPath Switch-ID (P1)

```yaml
show fabricpath switch-id:
  timeout: 10s
  priority: p1
  category: operational
  purpose: FabricPath switch ID mappings
  output_format: text
  fabricpath_relevant: true
```

**Expected Output**:
```
Legend: 
    * - this switch

Fabricpath IS-IS domain: default
  Fabricpath IS-IS tag: 0

Switch-ID  MAC-Address    Serial-Number    Local  Model
---------  -------------  ---------------  -----  -----
   *1      0016.4748.ac4e N9K-C9396PX-001  Yes    N9K-C9396PX
    10     0050.56bb.1234 N9K-C9508-001    No     N9K-C9508
    11     0050.56bb.1235 N9K-C9508-002    No     N9K-C9508
```

**Parser Required**: `ParseNXOSFabricPathSwitchID(output string) ([]NXOSFabricPathSwitch, error)`

---

### 15. HSRP Status (P1)

```yaml
show hsrp:
  timeout: 10s
  priority: p1
  category: operational
  purpose: Hot Standby Router Protocol state for L3 HA
  output_format: text
  ha_relevant: true
  l3_relevant: true
```

**Expected Output**:
```
Vlan10 - Group 1 (version 2)
  Local state is Active, priority 110 (Cfged 110), may preempt
    Forwarding threshold(for vPC), lower: 1 upper: 110
  Hellotime 3 sec, holdtime 10 sec
  Next hello sent in 1.840 secs(s)
  Virtual IP address is 10.0.10.1 (Cfged)
  Active router is local
  Standby router is 10.0.10.3 , priority 100 (expires in 8.832 sec(s))
  Authentication MD5, key-chain "hsrp-key"
  Virtual mac address is 0000.0c9f.f001 (Default MAC)
  2 state changes, last state change 2d17h
  IP redundancy name is "hsrp-Vlan10-1" (default)
```

**Parser Required**: `ParseNXOSHSRP(output string) ([]NXOSHSRPGroup, error)`

---

### 16. Spanning Tree Detail (P1)

```yaml
show spanning-tree detail:
  timeout: 20s
  priority: p1
  category: operational
  purpose: Per-VLAN STP state, root bridge, port roles
  output_format: text
  l2_relevant: true
  topology_relevant: true
```

**Expected Output**:
```
VLAN0010 is executing the rstp compatible Spanning Tree protocol
  Bridge Identifier has priority 4096, sysid 10, address 0050.56bb.e14c
  Configured hello time 2, max age 20, forward delay 15
  Current root has priority 4096, address 0050.56bb.1234
  Root port is 1 (Ethernet1/1), cost of root path is 4
  Topology change flag not set, detected flag not set
  Number of topology changes 3 last change occurred 2d17h ago
          from Ethernet1/5
  Times:  hold 1, topology change 35, notification 2
          hello 2, max age 20, forward delay 15

Port 1 (Ethernet1/1) of VLAN0010 is forwarding
   Port path cost 4, Port priority 128, Port Identifier 128.1
   Designated root has priority 4096, address 0050.56bb.1234
   Designated bridge has priority 4096, address 0050.56bb.1234
   Designated port id is 128.1, designated path cost 0
   Timers: message age 0, forward delay 0, hold 0
   BPDU: sent 150000, received 145000
```

**Parser Required**: `ParseNXOSSTPDetail(output string) ([]NXOSSTPDetail, error)`

---

### 17. IP Route Table (P1)

```yaml
show ip route:
  timeout: 15s
  priority: p1
  category: operational
  purpose: IP routing table for L3 topology
  output_format: text
  l3_relevant: true
  topology_relevant: true
```

**Expected Output**:
```
IP Route Table for VRF "default"
'*' denotes best ucast next-hop
'**' denotes best mcast next-hop
'[x/y]' denotes [preference/metric]
'%<string>' in via output denotes VRF <string>

10.0.0.0/8, ubest/mbest: 1/0
    *via 10.0.255.1, Eth1/1, [110/41], 2d17h, ospf-1, intra
10.0.10.0/24, ubest/mbest: 1/0, attached
    *via 10.0.10.1, Vlan10, [0/0], 2d17h, direct
```

**Parser Required**: `ParseNXOSRoute(output string) ([]NXOSRoute, error)`

---

### 18. ARP Table (P1)

```yaml
show ip arp:
  timeout: 15s
  priority: p1
  category: operational
  purpose: ARP table for IP-to-MAC resolution
  output_format: text
  l3_relevant: true
  topology_relevant: true
```

**Expected Output**:
```
IP ARP Table for context default
Total number of entries: 5
Address         Age       MAC Address     Interface
10.0.10.2       00:05:12  0050.56bb.0001  Vlan10
10.0.10.3       00:02:34  0050.56bb.0002  Vlan10
10.0.20.5       00:10:45  0011.2233.4455  Vlan20
```

**Parser Required**: `ParseNXOSARP(output string) ([]NXOSARPEntry, error)`

---

## P2 Medium Priority Commands (6)

### 19. System Version (P2)

```yaml
show version:
  timeout: 10s
  priority: p2
  category: context
  purpose: Software version, hardware model, uptime
  output_format: text
  device_info: true
```

**Parser Required**: `ParseNXOSVersion(output string) (*NXOSVersion, error)`

---

### 20. System Resources (P2)

```yaml
show system resources:
  timeout: 10s
  priority: p2
  category: context
  purpose: CPU, memory utilization
  output_format: text
  device_info: true
```

**Parser Required**: `ParseNXOSSystemResources(output string) (*NXOSResources, error)`

---

### 21. Inventory (P2)

```yaml
show inventory:
  timeout: 15s
  priority: p2
  category: context
  purpose: Chassis, modules, serial numbers
  output_format: text
  device_info: true
```

**Parser Required**: `ParseNXOSInventory(output string) ([]NXOSInventoryItem, error)`

---

### 22. Feature Status (P2)

```yaml
show feature:
  timeout: 5s
  priority: p2
  category: context
  purpose: Enabled features (LACP, VPC, FabricPath, etc.)
  output_format: text
  device_info: true
```

**Parser Required**: `ParseNXOSFeatures(output string) ([]NXOSFeature, error)`

---

### 23. VRRP Status (P2)

```yaml
show vrrp:
  timeout: 10s
  priority: p2
  category: operational
  purpose: Virtual Router Redundancy Protocol state
  output_format: text
  ha_relevant: true
  l3_relevant: true
```

**Parser Required**: `ParseNXOSVRRP(output string) ([]NXOSVRRPGroup, error)`

---

### 24. Interface Counters (P2)

```yaml
show interface counters:
  timeout: 15s
  priority: p2
  category: operational
  purpose: Packet/byte counters, errors, discards
  output_format: text
  monitoring_relevant: true
```

**Parser Required**: `ParseNXOSInterfaceCounters(output string) ([]NXOSInterfaceCounters, error)`

---

## Performance Considerations

### Command Execution Time Budget

| Command | Expected Duration | Timeout | Large Deployment (1000 ports) |
|---------|-------------------|---------|-------------------------------|
| `show mac address-table` | 5-15s | 20s | Up to 30s (critical path) |
| `show vlan` | 1-3s | 10s | Up to 5s |
| `show interface status` | 5-10s | 15s | Up to 20s |
| `show interface trunk` | 2-5s | 10s | Up to 10s |
| `show port-channel summary` | 1-3s | 10s | Up to 5s |
| `show spanning-tree summary` | 2-5s | 10s | Up to 10s |
| `show lldp neighbors detail` | 10-20s | 20s | Up to 40s (critical path) |
| `show cdp neighbors detail` | 5-15s | 15s | Up to 30s |
| `show vpc` | 3-8s | 15s | Up to 15s (**VPC critical**) |
| `show vpc peer-keepalive` | 1-3s | 10s | Up to 5s |
| `show vpc consistency-parameters` | 5-10s | 15s | Up to 20s |
| `show vdc` | 1-3s | 10s | Up to 5s |
| `show fabricpath topology` | 5-10s | 15s | Up to 20s |
| `show fabricpath switch-id` | 2-5s | 10s | Up to 10s |
| `show hsrp` | 2-5s | 10s | Up to 10s |
| `show spanning-tree detail` | 10-20s | 20s | Up to 40s |
| `show ip route` | 5-15s | 15s | Up to 30s |
| `show ip arp` | 5-10s | 15s | Up to 20s |

**Total NXOS Commands**: ~80-180 seconds (small deployment), up to 320 seconds (large deployment)

---

## Summary Table

| Category | Commands | Priority | Purpose |
|----------|----------|----------|---------|
| **L2 Base** | 9 | P0-P1 | MAC, VLAN, interfaces, trunks, port-channels, STP, LLDP, CDP |
| **VPC** | 5 | P0-P1 | VPC status, keepalive, consistency, peer-link health |
| **FabricPath** | 2 | P1 | IS-IS topology, switch-ID mappings |
| **L3** | 3 | P1-P2 | Routing, ARP, HSRP, VRRP |
| **Context** | 5 | P2 | Version, resources, inventory, features, counters |
| **Total** | **24** | P0-P2 | **Comprehensive NXOS visibility** |

---

## NXOS-Specific Features

### Virtual Port Channel (vPC)
- **Peer State**: Primary/Secondary role detection
- **Keepalive**: Management link health monitoring
- **Consistency**: Configuration parameter validation (Type 1/Type 2)
- **Peer-Link**: Dedicated VLAN trunk for control plane and orphan traffic
- **Critical for**: Dual-homed server detection, MLAG topology mapping

### FabricPath
- **IS-IS Based**: Layer-2 routing protocol
- **Switch-ID**: Unique identifier for each switch in fabric
- **Spine-Leaf**: Topology detection
- **Multi-path**: Equal-cost multi-path (ECMP) for L2

### Virtual Device Context (VDC)
- **Multi-tenancy**: Logical switch partitions
- **Resource Isolation**: Dedicated CPU, memory, interfaces
- **Separate Control Planes**: Independent routing, STP, VPC domains

---

## Integration with Orchestrator

### Pseudo-code for Orchestrator Integration

```go
// In connector_orchestrator.go

func (o *Orchestrator) getCiscoNXOSCommands() []AuxCommand {
    return []AuxCommand{
        // P0 L2 Commands
        {Command: "show mac address-table", Timeout: 20*time.Second, Priority: "P0", Parser: "ParseNXOSMACTable"},
        {Command: "show vlan", Timeout: 10*time.Second, Priority: "P0", Parser: "ParseNXOSVLAN"},
        {Command: "show interface status", Timeout: 15*time.Second, Priority: "P0", Parser: "ParseNXOSInterfaceStatus"},
        {Command: "show interface trunk", Timeout: 10*time.Second, Priority: "P0", Parser: "ParseNXOSTrunk"},
        {Command: "show port-channel summary", Timeout: 10*time.Second, Priority: "P0", Parser: "ParseNXOSPortChannel"},
        {Command: "show spanning-tree summary", Timeout: 10*time.Second, Priority: "P0", Parser: "ParseNXOSSTP"},
        {Command: "show lldp neighbors detail", Timeout: 20*time.Second, Priority: "P0", Parser: "ParseNXOSLLDP"},
        {Command: "show cdp neighbors detail", Timeout: 15*time.Second, Priority: "P0", Parser: "ParseNXOSCDP"},
        
        // P0 VPC Commands
        {Command: "show vpc", Timeout: 15*time.Second, Priority: "P0", Parser: "ParseNXOSVPC"},
        {Command: "show vpc peer-keepalive", Timeout: 10*time.Second, Priority: "P0", Parser: "ParseNXOSVPCKeepalive"},
        
        // P1 Commands
        {Command: "show vpc consistency-parameters global", Timeout: 15*time.Second, Priority: "P1", Parser: "ParseNXOSVPCConsistency"},
        {Command: "show vdc", Timeout: 10*time.Second, Priority: "P1", Parser: "ParseNXOSVDC"},
        {Command: "show fabricpath topology", Timeout: 15*time.Second, Priority: "P1", Parser: "ParseNXOSFabricPath"},
        {Command: "show fabricpath switch-id", Timeout: 10*time.Second, Priority: "P1", Parser: "ParseNXOSFabricPathSwitchID"},
        {Command: "show hsrp", Timeout: 10*time.Second, Priority: "P1", Parser: "ParseNXOSHSRP"},
        {Command: "show spanning-tree detail", Timeout: 20*time.Second, Priority: "P1", Parser: "ParseNXOSSTPDetail"},
        {Command: "show ip route", Timeout: 15*time.Second, Priority: "P1", Parser: "ParseNXOSRoute"},
        {Command: "show ip arp", Timeout: 15*time.Second, Priority: "P1", Parser: "ParseNXOSARP"},
        
        // P2 Context Commands
        {Command: "show version", Timeout: 10*time.Second, Priority: "P2", Parser: "ParseNXOSVersion"},
        {Command: "show system resources", Timeout: 10*time.Second, Priority: "P2", Parser: "ParseNXOSSystemResources"},
        {Command: "show inventory", Timeout: 15*time.Second, Priority: "P2", Parser: "ParseNXOSInventory"},
        {Command: "show feature", Timeout: 5*time.Second, Priority: "P2", Parser: "ParseNXOSFeatures"},
        {Command: "show vrrp", Timeout: 10*time.Second, Priority: "P2", Parser: "ParseNXOSVRRP"},
        {Command: "show interface counters", Timeout: 15*time.Second, Priority: "P2", Parser: "ParseNXOSInterfaceCounters"},
    }
}
```

---

## Status

✅ **Phase 2.1.1 Spec Complete** - Ready for parser implementation  
⏭️ **Next**: Phase 2.1.2 - Implement 9 L2 operational parsers  
⏭️ **Next**: Phase 2.1.3 - Implement 5 VPC/Fabric parsers

---

**Document Version**: 1.0  
**Last Updated**: 2025-10-26  
**Total Commands**: 24 (10 P0 + 8 P1 + 6 P2)


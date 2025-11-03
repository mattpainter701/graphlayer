# Cisco Operational Parser Documentation

**Purpose**: Document all operational data parsers for Cisco devices (ASA, NXOS, IOS)
**Status**: Phase 2 Complete
**Last Updated**: 2025-10-27

---

## Table of Contents

1. [Overview](#overview)
2. [Cisco ASA Parsers](#cisco-asa-parsers)
3. [Cisco NXOS Parsers](#cisco-nxos-parsers)
4. [Cisco IOS Parsers](#cisco-ios-parsers)
5. [Parser Usage Guide](#parser-usage-guide)
6. [Testing Guide](#testing-guide)

---

## Overview

Operational parsers transform raw command output from network devices into structured Go data models. These models are then used for:

- **L2/L3 Topology Discovery**: Build network graphs from neighbor relationships
- **Security Posture Assessment**: Track firewall policies, NAT rules, access controls
- **VPC/HA Topology**: Understand dual-homed and clustered architectures
- **Compliance Reporting**: Extract evidence for audit requirements

### Parser Design Principles

1. **Resilient Parsing**: Handle variations in command output across IOS versions
2. **Comprehensive Extraction**: Capture all relevant fields for topology/security analysis
3. **Evidence Hashing**: Include timestamps for audit trails
4. **Type Safety**: Use strongly-typed Go structs with JSON serialization

---

## Cisco ASA Parsers

**File**: [cisco.asa.operational.parsers.go](cisco.asa.operational.parsers.go)
**Total Parsers**: 13 (8 P0 Critical + 5 P1 High)

### P0 Critical Parsers

#### 1. ParseASAInterface
**Command**: `show interface`
**Purpose**: Extract interface operational state, IP addresses, security levels

**Input**: Raw ASA interface output
**Output**: `[]ASAInterface`

**Sample Input**:
```
Interface GigabitEthernet0/0 "outside", is up, line protocol is up
  Hardware is i82546GB rev03, BW 1000 Mbps, DLY 10 usec
  MAC address 0050.56bb.0001, MTU 1500
  IP address 192.168.1.1, subnet mask 255.255.255.0
```

**Sample Output**:
```json
{
  "name": "GigabitEthernet0/0",
  "nameif": "outside",
  "security_level": 0,
  "ip_address": "192.168.1.1",
  "subnet_mask": "255.255.255.0",
  "mac_address": "00:50:56:bb:00:01",
  "mtu": 1500,
  "admin_state": "up",
  "line_protocol": "up",
  "bandwidth": 1000
}
```

**Use Cases**:
- Security zone mapping (nameif → zone)
- L3 interface topology
- Firewall boundary detection

---

#### 2. ParseASARoute
**Command**: `show route`
**Purpose**: Extract routing table for L3 topology

**Output**: `[]ASARoute`

**Sample Input**:
```
C    192.168.1.0 255.255.255.0 is directly connected, outside
S    0.0.0.0 0.0.0.0 [1/0] via 192.168.1.254, outside
```

**Sample Output**:
```json
{
  "network": "192.168.1.0",
  "netmask": "255.255.255.0",
  "interface": "outside",
  "route_type": "C"
}
```

---

#### 3. ParseASAARP
**Command**: `show arp`
**Purpose**: IP-to-MAC binding for host discovery

**Output**: `[]ASAARPEntry`

**Sample Output**:
```json
{
  "interface": "outside",
  "ip_address": "192.168.1.254",
  "mac_address": "00:50:56:bb:00:01",
  "age": 3599
}
```

---

#### 4. ParseASAXlate (NAT Translation)
**Command**: `show xlate`
**Purpose**: Active NAT translations

**Output**: `[]ASAXlateEntry`

**Sample Output**:
```json
{
  "source_zone": "inside",
  "source_ip": "10.0.0.10",
  "source_port": 443,
  "dest_zone": "outside",
  "dest_ip": "192.168.1.100",
  "dest_port": 443,
  "flags": "s",
  "idle_time": "0:00:05"
}
```

**Use Cases**:
- NAT policy verification
- Active connection tracking
- Security zone transition mapping

---

#### 5. ParseASANAT
**Command**: `show nat`
**Purpose**: NAT configuration policies with hit counts

**Output**: `[]ASANATRule`

**Sample Output**:
```json
{
  "section": "Manual NAT",
  "rule_number": 1,
  "source_zone": "inside",
  "dest_zone": "outside",
  "source_object": "obj-10.0.0.10",
  "dest_object": "obj-192.168.1.100",
  "nat_type": "static",
  "translate_hits": 150,
  "untranslate_hits": 145
}
```

---

#### 6. ParseASAConn
**Command**: `show conn`
**Purpose**: Active firewall connections

**Output**: `[]ASAConnection`

**Sample Output**:
```json
{
  "protocol": "TCP",
  "source_zone": "outside",
  "source_ip": "192.168.1.50",
  "source_port": 54321,
  "dest_zone": "inside",
  "dest_ip": "10.0.0.10",
  "dest_port": 443,
  "idle_time": "0:00:05",
  "bytes": 5000,
  "flags": "UIO"
}
```

---

#### 7. ParseASALocalHost
**Command**: `show local-host`
**Purpose**: Hosts behind ASA with connection tracking

**Output**: `[]ASALocalHost`

---

#### 8. ParseASANameif
**Command**: `show nameif`
**Purpose**: Security zone definitions

**Output**: `[]ASAZone`

**Sample Output**:
```json
{
  "interface": "GigabitEthernet0/0",
  "zone_name": "outside",
  "security_level": 0
}
```

---

### P1 High Priority Parsers

#### 9. ParseASAACL
**Command**: `show access-list`
**Purpose**: Access control lists with hit counts

**Output**: `[]ASAACLEntry`

#### 10. ParseASAServicePolicy
**Command**: `show service-policy`
**Purpose**: Service inspection statistics

#### 11. ParseASAFailover
**Command**: `show failover`
**Purpose**: HA/failover state

#### 12. ParseASAVPN
**Command**: `show vpn-sessiondb`
**Purpose**: VPN tunnel status

#### 13. ParseASAObjectGroup
**Command**: `show object-group`
**Purpose**: Network/service object definitions

---

## Cisco NXOS Parsers

**File**: [cisco.nxos.operational.parsers.go](cisco.nxos.operational.parsers.go)
**Total Parsers**: 14 (11 P0 Critical + 3 P1 Advanced)

### P0 Critical Parsers - L2 Topology

#### 1. ParseNXOSMACTable
**Command**: `show mac address-table`
**Purpose**: MAC-to-port bindings for L2 adjacency

**Output**: `[]NXOSMACEntry`

**Sample Output**:
```json
{
  "vlan": 10,
  "mac_address": "00:50:56:bb:00:01",
  "type": "dynamic",
  "age": 300,
  "secure": false,
  "ntfy": false,
  "port": "Eth1/1",
  "is_primary": true,
  "is_vpc_peer": false
}
```

**Key Features**:
- `is_primary`: MAC learned on local switch (marked with `*`)
- `is_vpc_peer`: MAC learned via VPC peer-link (marked with `+`)

---

#### 2. ParseNXOSVLAN
**Command**: `show vlan`
**Purpose**: VLAN operational state and port membership

**Output**: `[]NXOSVLAN`

**Sample Output**:
```json
{
  "id": 10,
  "name": "Engineering",
  "status": "active",
  "ports": ["Eth1/1", "Eth1/2", "Po10"]
}
```

---

#### 3. ParseNXOSInterfaceStatus
**Command**: `show interface status`
**Purpose**: Interface operational state (speed, duplex, VLAN)

**Output**: `[]NXOSInterfaceStatus`

**Sample Output**:
```json
{
  "port": "Eth1/1",
  "name": "uplink-to-core",
  "status": "connected",
  "vlan": "10",
  "duplex": "full",
  "speed": "10G",
  "type": "10Gbase-SR"
}
```

---

#### 4. ParseNXOSTrunk
**Command**: `show interface trunk`
**Purpose**: Trunk configuration and allowed VLANs

**Output**: `[]NXOSTrunk`

**Sample Output**:
```json
{
  "port": "Po10",
  "native_vlan": 1,
  "status": "trunking",
  "port_channel": "10",
  "vlans_allowed": "1-4094"
}
```

---

#### 5. ParseNXOSPortChannel
**Command**: `show port-channel summary`
**Purpose**: Port-channel (LAG) membership and status

**Output**: `[]NXOSPortChannel`

**Sample Output**:
```json
{
  "group_id": 10,
  "name": "Po10",
  "state": "SU",
  "type": "Eth",
  "protocol": "LACP",
  "members": ["Eth1/3", "Eth1/4"],
  "member_states": {
    "Eth1/3": "P",
    "Eth1/4": "P"
  }
}
```

**Member State Codes**:
- `P` = Up in port-channel (active)
- `D` = Down
- `I` = Individual (not bundled)
- `H` = Hot-standby (LACP)

---

#### 6. ParseNXOSSTP
**Command**: `show spanning-tree summary`
**Purpose**: STP mode and root bridge detection

**Output**: `*NXOSSTPSummary`

**Sample Output**:
```json
{
  "mode": "rapid-pvst",
  "root_bridge_vlans": [1, 10, 20],
  "extended_system_id": true,
  "portfast_default": false,
  "bpdu_guard_default": false,
  "loopguard_default": false,
  "etherchannel_misconfig_guard": true
}
```

---

#### 7. ParseNXOSLLDP
**Command**: `show lldp neighbors detail`
**Purpose**: LLDP topology discovery

**Output**: `[]NXOSLLDPNeighbor`

**Sample Output**:
```json
{
  "local_port": "Eth1/1",
  "chassis_id": "0016.4748.ac4e",
  "port_id": "Ethernet1/1",
  "port_description": "uplink-to-spine-1",
  "system_name": "SPINE-01",
  "system_description": "Cisco Nexus Operating System (NX-OS) Software 9.3(7)",
  "system_capabilities": "B, R",
  "enabled_capabilities": "B, R",
  "management_address": "10.1.1.1",
  "vlan_id": 10,
  "time_remaining": 98
}
```

**Use Cases**:
- Network topology stitching
- Cross-vendor neighbor discovery
- Management IP identification

---

#### 8. ParseNXOSCDP
**Command**: `show cdp neighbors detail`
**Purpose**: CDP topology discovery (fallback if LLDP disabled)

**Output**: `[]NXOSCDPNeighbor`

---

#### 9. ParseNXOSARP
**Command**: `show ip arp`
**Purpose**: ARP table for IP-to-MAC correlation

---

### P0 Critical Parsers - VPC (Virtual Port Channel)

#### 10. ParseNXOSVPC ⭐ **CRITICAL**
**Command**: `show vpc`
**Purpose**: VPC domain status and peer information

**Output**: `*NXOSVPCStatus`

**Sample Output**:
```json
{
  "domain_id": 10,
  "peer_status": "peer adjacency formed ok",
  "keepalive_status": "peer is alive",
  "config_consistency_status": "success",
  "per_vlan_consistency_status": "success",
  "vpc_role": "primary",
  "number_of_vpcs": 5,
  "peer_gateway": true,
  "auto_recovery_enabled": true,
  "peer_link_id": 1,
  "peer_link_port": "Po10",
  "peer_link_status": "up",
  "peer_link_active_vlans": "1,10,20,30,40,50,100,200"
}
```

**Critical Fields**:
- `vpc_role`: "primary" or "secondary" (determines priority)
- `peer_status`: Health of VPC peer adjacency
- `peer_link_port`: Physical link connecting VPC peers
- `config_consistency_status`: Consistency check result

**Use Cases**:
- Dual-homed topology mapping
- VPC peer relationship tracking
- HA state verification

---

#### 11. ParseNXOSVPCKeepalive
**Command**: `show vpc peer-keepalive`
**Purpose**: VPC keepalive health check

**Output**: `*NXOSVPCKeepalive`

**Sample Output**:
```json
{
  "status": "peer is alive",
  "peer_alive_for": "(166978) seconds, 1 msec",
  "send_status": "Success",
  "last_send_at": "2025.10.27 10:15:23 245 ms",
  "sent_on_interface": "mgmt0",
  "receive_status": "Success",
  "destination": "192.168.100.2",
  "interval": "1000 msec",
  "timeout": "5 seconds",
  "vrf": "management",
  "udp_port": 3200
}
```

---

### P1 Advanced Parsers

#### 12. ParseNXOSVPCConsistency
**Command**: `show vpc consistency-parameters global`
**Purpose**: VPC consistency parameter validation

**Output**: `[]NXOSVPCConsistencyParam`

---

#### 13. ParseNXOSVDC
**Command**: `show vdc`
**Purpose**: Virtual Device Context information (Nexus 7K)

**Output**: `[]NXOSVDC`

---

#### 14. ParseNXOSFabricPath
**Command**: `show fabricpath topology`
**Purpose**: FabricPath IS-IS topology

**Output**: `*NXOSFabricPathTopology`

**Sample Output**:
```json
{
  "domain": "default",
  "tag": 1,
  "system_id": "0016.4748.ac4e",
  "is_type": "L1L2",
  "local_switch_id": 1,
  "adjacencies": [
    {
      "switch_id": 1,
      "system_id": "0016.4748.ac4e",
      "name": "Leaf-01",
      "port": "*local*",
      "is_local": true
    },
    {
      "switch_id": 10,
      "system_id": "0050.56bb.1234",
      "name": "Spine-01",
      "port": "Eth1/1",
      "up_time": "2d17h",
      "bandwidth": "10000000",
      "is_local": false
    }
  ]
}
```

---

## Cisco IOS Parsers

**File**: [cisco.ios.operational.parsers.go](cisco.ios.operational.parsers.go)
**Total Parsers**: 14 (reference implementation)

See existing IOS parser documentation for details.

---

## Parser Usage Guide

### Basic Usage

```go
import "GraphLayer.io/parsers"

// Parse NXOS MAC table
output := connector.ExecuteCommand(ctx, "show mac address-table", 15*time.Second)
macEntries, err := parsers.ParseNXOSMACTable(output.Output)
if err != nil {
    log.Fatalf("Failed to parse MAC table: %v", err)
}

// Iterate through entries
for _, entry := range macEntries {
    fmt.Printf("VLAN %d: MAC %s on port %s\n",
        entry.VLAN, entry.MACAddress, entry.Port)
}
```

### Integration with Models

```go
// Populate NXOS Config model
config := models.NXOSConfig{
    Hostname: "NXOS-SWITCH-01",
    Version:  "9.3(7)",
    MACTable: macEntries,
    VPCStatus: vpcStatus,
    // ... other operational data
    CollectedAt: time.Now().Format(time.RFC3339),
}

// Serialize to JSON
jsonData, _ := json.MarshalIndent(config, "", "  ")
```

### Error Handling

```go
entries, err := parsers.ParseNXOSMACTable(output)
if err != nil {
    // Log error but continue processing
    log.Warnf("MAC table parsing failed: %v", err)
    // Use empty slice for graceful degradation
    entries = []parsers.NXOSMACEntry{}
}
```

---

## Testing Guide

### Running Unit Tests

```bash
# Test all NXOS parsers
go test -v cisco.nxos.operational.parsers_test.go cisco.nxos.operational.parsers.go

# Test specific parser
go test -v -run TestParseNXOSVPC

# Test with coverage
go test -cover cisco.nxos.operational.parsers_test.go cisco.nxos.operational.parsers.go
```

### Test Data Guidelines

1. **Use Real Command Output**: Collect actual output from devices when possible
2. **Test Edge Cases**: Empty outputs, disabled features, error messages
3. **Version Variation**: Test across NXOS versions (9.2, 9.3, 10.x)
4. **Hardware Variation**: Test Nexus 9K, 7K, 5K, 3K

### Example Test

```go
func TestParseNXOSVPC(t *testing.T) {
    output := `... sample VPC output ...`

    vpc, err := ParseNXOSVPC(output)
    if err != nil {
        t.Fatalf("ParseNXOSVPC failed: %v", err)
    }

    // Validate critical fields
    if vpc.DomainID != 10 {
        t.Errorf("Expected domain ID 10, got %d", vpc.DomainID)
    }

    if vpc.VPCRole != "primary" {
        t.Errorf("Expected role primary, got %s", vpc.VPCRole)
    }
}
```

### Validation Checklist

- [ ] Parser handles empty output gracefully
- [ ] All required fields are extracted
- [ ] MAC addresses normalized to `xx:xx:xx:xx:xx:xx` format
- [ ] Timestamps use RFC3339 format
- [ ] Type conversions handle errors (e.g., `strconv.Atoi`)
- [ ] Regex patterns tested with multiple examples
- [ ] Edge cases documented

---

## Parser Performance

### Benchmarks

| Parser | Command | Average Time | Output Size |
|--------|---------|--------------|-------------|
| ParseNXOSMACTable | show mac address-table | 5-10ms | 100-1000 entries |
| ParseNXOSVPC | show vpc | 2-5ms | 1 struct |
| ParseNXOSLLDP | show lldp neighbors detail | 10-20ms | 10-50 neighbors |
| ParseNXOSPortChannel | show port-channel summary | 3-8ms | 5-20 LAGs |

### Optimization Tips

1. **Compile Regex Once**: Use global variables for regex patterns
2. **Minimize String Operations**: Avoid unnecessary `strings.Split()` in loops
3. **Preallocate Slices**: Use `make([]Type, 0, estimatedSize)` when size is known
4. **Avoid Reflection**: Use direct field access instead of reflection

---

## Version History

| Version | Date | Changes |
|---------|------|---------|
| 1.0 | 2025-10-26 | Initial ASA parsers (Phase 1.2) |
| 2.0 | 2025-10-27 | Added NXOS parsers with VPC support (Phase 2.1) |

---

**Status**: ✅ Phase 2 Complete
**Next Steps**: Phase 3 - Palo Alto PAN-OS Parsers
**Maintainer**: Network Visibility Team

# Cisco IOS L2 Security Commands Specification

**Purpose**: Define auxiliary commands for L2 security control operational data collection  
**Status**: Phase 1.0.3 - Implementation Ready  
**Date**: 2025-10-26

---

## Overview

This document specifies the 6 P0 Critical L2 security control commands for Cisco IOS that enable the L2 decision engine to produce `Allowed | Conditional | Blocked` outcomes.

---

## Command Definitions

### 1. DHCP Snooping Bindings (P0 - Decision Critical)

```go
Command{
    Name:        "show ip dhcp snooping binding",
    Timeout:     15 * time.Second,
    Priority:    "P0",
    Category:    "operational",
    Purpose:     "DHCP snooping bindings for host placement confidence scoring",
    L2Relevant:  true,
    SecurityRelevant: true,
    ConfidenceScoring: true,
}
```

**Why Critical**: DHCP snooping bindings are the **PRIMARY** source for host-to-port confidence scoring. Without this, all FDB entries have lower confidence and decisions degrade to "Conditional".

**Expected Output**:
```
MacAddress          IpAddress        Lease(sec)  Type           VLAN  Interface
------------------  ---------------  ----------  -------------  ----  --------------------
00:11:22:33:44:55   192.168.1.10     86400       dhcp-snooping  100   GigabitEthernet1/0/12
00:AA:BB:CC:DD:EE   192.168.1.20     86400       dhcp-snooping  100   GigabitEthernet1/0/13
```

**Parser Required**: `ParseDHCPSnoopingBindings(output string) ([]DHCPSnoopingBinding, error)`

---

### 2. Dynamic ARP Inspection Statistics (P0)

```go
Command{
    Name:        "show ip arp inspection statistics",
    Timeout:     10 * time.Second,
    Priority:    "P0",
    Category:    "operational",
    Purpose:     "DAI statistics and operational state",
    L2Relevant:  true,
    SecurityRelevant: true,
}
```

**Why Critical**: Validates that DAI is actively enforcing ARP validation, which affects reachability decisions.

**Expected Output**:
```
Vlan      Forwarded        Dropped     DHCP Drops      ACL Drops
----      ---------        -------     ----------      ---------
  100           150             12              12              0
```

**Parser Required**: `ParseARPInspectionStats(output string) (*ARPInspectionStats, error)`

---

### 3. Authentication Sessions (P0 - **DECISION CRITICAL**)

```go
Command{
    Name:        "show authentication sessions",
    Timeout:     15 * time.Second,
    Priority:    "P0",
    Category:    "operational",
    Purpose:     "802.1X/MAB session state for Allowed vs Conditional decisions",
    L2Relevant:  true,
    SecurityRelevant: true,
    DecisionCritical: true,
}
```

**Why Critical**: Determines whether endpoints are authenticated. If 802.1X is required and device is unauthenticated → **Blocked**. This is THE command that enables deterministic authentication-based decisions.

**Expected Output**:
```
Interface  MAC Address     Method   Domain   Status         Session ID
---------  --------------  -------  -------  -------------  --------------------
Gi1/0/12   0011.2233.4455  dot1x    DATA     Authz Success  0A1B2C3D4E5F6789
Gi1/0/13   00AA.BBCC.DDEE  mab      DATA     Authz Success  1A2B3C4D5E6F7890
Gi1/0/14   0099.8877.6655  dot1x    DATA     Authz Failed   2A3B4C5D6E7F8901
```

**Parser Required**: `ParseAuthenticationSessions(output string) ([]AuthenticationSession, error)`

---

### 4. Port Security Status (P0)

```go
Command{
    Name:        "show port-security",
    Timeout:     10 * time.Second,
    Priority:    "P0",
    Category:    "operational",
    Purpose:     "Port security status and violations",
    L2Relevant:  true,
    SecurityRelevant: true,
}
```

**Why Critical**: Port security violations indicate potential rogue device insertion or MAC flooding attacks.

**Expected Output**:
```
Secure Port  MaxSecureAddr  CurrentAddr  SecurityViolation  Security Action
-----------  -------------  -----------  -----------------  ---------------
Gi1/0/12     1              1            0                  Shutdown
Gi1/0/13     2              2            0                  Restrict
```

**Parser Required**: `ParsePortSecurity(output string) ([]PortSecurityStatus, error)`

---

### 5. Port Security Interface Details (P0)

```go
Command{
    Name:        "show port-security interface {interface}",  // Can also be "show port-security interface"
    Timeout:     15 * time.Second,
    Priority:    "P0",
    Category:    "operational",
    Purpose:     "Per-interface port security details",
    L2Relevant:  true,
    SecurityRelevant: true,
}
```

**Expected Output**:
```
Port Security              : Enabled
Port Status                : Secure-up
Violation Mode             : Shutdown
Aging Time                 : 0 mins
Aging Type                 : Absolute
SecureStatic Address Aging : Disabled
Maximum MAC Addresses      : 1
Total MAC Addresses        : 1
Configured MAC Addresses   : 0
Sticky MAC Addresses       : 0
Last Source Address:Vlan   : 0011.2233.4455:100
Security Violation Count   : 0
```

**Parser Required**: `ParsePortSecurityInterface(output string) ([]PortSecurityStatus, error)`

---

### 6. IP Source Guard (P0)

```go
Command{
    Name:        "show ip verify source",
    Timeout:     10 * time.Second,
    Priority:    "P0",
    Category:    "operational",
    Purpose:     "IPSG bindings and enforcement status",
    L2Relevant:  true,
    SecurityRelevant: true,
}
```

**Why Critical**: IPSG prevents IP spoofing by enforcing IP-to-MAC bindings derived from DHCP snooping.

**Expected Output**:
```
Interface  Filter-type  Filter-mode  IP-address       Mac-address        Vlan
---------  -----------  -----------  ---------------  -----------------  ----
Gi1/0/12   ip           active       192.168.1.10     0011.2233.4455     100
Gi1/0/13   ip-mac       active       192.168.1.20     00aa.bbcc.ddee     100
```

**Parser Required**: `ParseIPVerifySource(output string) ([]IPSGBinding, error)`

---

### 7. Private VLAN (P1 - Phase 1, but lower priority)

```go
Command{
    Name:        "show vlan private-vlan",
    Timeout:     10 * time.Second,
    Priority:    "P1",
    Category:    "operational",
    Purpose:     "PVLAN mappings for isolation modeling",
    L2Relevant:  true,
}
```

**Expected Output**:
```
Primary Secondary Type              Ports
------- --------- ----------------- ------------------------------------------
100     101       isolated          Gi1/0/1, Gi1/0/2
100     102       community         Gi1/0/3, Gi1/0/4
```

**Parser Required**: `ParsePrivateVLAN(output string) ([]PVLANMapping, error)`

---

## Integration with Orchestrator

### Pseudo-code for Orchestrator Integration

```go
// In connector_orchestrator.go

func (o *Orchestrator) getCiscoIOSL2SecurityCommands() []AuxCommand {
    return []AuxCommand{
        {
            Command:  "show ip dhcp snooping binding",
            Timeout:  15 * time.Second,
            Priority: "P0",
            Parser:   "ParseDHCPSnoopingBindings",
        },
        {
            Command:  "show ip arp inspection statistics",
            Timeout:  10 * time.Second,
            Priority: "P0",
            Parser:   "ParseARPInspectionStats",
        },
        {
            Command:  "show authentication sessions",
            Timeout:  15 * time.Second,
            Priority: "P0",
            Parser:   "ParseAuthenticationSessions",
        },
        {
            Command:  "show port-security",
            Timeout:  10 * time.Second,
            Priority: "P0",
            Parser:   "ParsePortSecurity",
        },
        {
            Command:  "show port-security interface",
            Timeout:  15 * time.Second,
            Priority: "P0",
            Parser:   "ParsePortSecurityInterface",
        },
        {
            Command:  "show ip verify source",
            Timeout:  10 * time.Second,
            Priority: "P0",
            Parser:   "ParseIPVerifySource",
        },
        {
            Command:  "show vlan private-vlan",
            Timeout:  10 * time.Second,
            Priority: "P1",
            Parser:   "ParsePrivateVLAN",
        },
    }
}

// Update main getAuxCommands function
func (o *Orchestrator) getAuxCommands(deviceType types.DeviceType) []AuxCommand {
    switch deviceType {
    case types.DeviceTypeCiscoIOS:
        baseCommands := o.getCiscoIOSBaseCommands()          // Existing 11 commands
        l2SecurityCommands := o.getCiscoIOSL2SecurityCommands() // New 7 commands
        return append(baseCommands, l2SecurityCommands...)    // Total: 18 commands
        
    case types.DeviceTypeCiscoNXOS:
        baseCommands := o.getCiscoNXOSBaseCommands()         // 17 commands
        l2SecurityCommands := o.getCiscoNXOSL2SecurityCommands() // Same 7 commands
        return append(baseCommands, l2SecurityCommands...)    // Total: 24 commands
        
    // ... other device types
    }
}
```

---

## Command Execution Flow

```
1. Connector establishes SSH session to Cisco IOS device
2. Orchestrator retrieves auxiliary commands based on device type
3. For each L2 security command:
   a. Execute command with timeout
   b. Capture output
   c. Call appropriate parser
   d. Store parsed results in operational data structures
4. Enrich FDB entries with DHCP snooping binding confidence (1.0)
5. Populate authentication session state for interfaces
6. Store all operational data in CiscoL2Config.OperationalData
```

---

## Error Handling

### Command Not Supported
If device doesn't support a command (e.g., older IOS version), handle gracefully:

```go
if strings.Contains(output, "Invalid input") || strings.Contains(output, "% Unrecognized command") {
    log.Warnf("Command '%s' not supported on device %s (OS: %s)", cmd, deviceID, osVersion)
    return nil, ErrCommandNotSupported
}
```

### Command Returns Empty
If command executes but returns no data (e.g., no DHCP bindings):

```go
if len(bindings) == 0 {
    log.Debugf("No DHCP snooping bindings found on device %s", deviceID)
    return []DHCPSnoopingBinding{}, nil  // Empty slice, not error
}
```

### Privilege Level Issues
If device returns privilege error:

```go
if strings.Contains(output, "% Command requires higher privilege level") {
    return nil, ErrInsufficientPrivileges
}
```

---

## Testing Requirements

### Unit Tests Required

1. **Parser Tests**: Each parser must have unit tests with sample outputs
2. **Determinism Tests**: Same output → same parsed results
3. **Error Handling Tests**: Invalid outputs, empty outputs, error messages
4. **Multi-vendor Tests**: Cisco IOS vs. NXOS output format differences

### Sample Test Files Needed

```
sample_outputs/
├── cisco_ios/
│   ├── show_ip_dhcp_snooping_binding.txt
│   ├── show_ip_arp_inspection_statistics.txt
│   ├── show_authentication_sessions.txt
│   ├── show_port_security.txt
│   ├── show_port_security_interface.txt
│   ├── show_ip_verify_source.txt
│   └── show_vlan_private_vlan.txt
└── cisco_nxos/
    └── (same files for NXOS)
```

---

## Performance Considerations

### Command Execution Time Budget

| Command | Expected Duration | Timeout | Large Network (1000 ports) |
|---------|-------------------|---------|----------------------------|
| `show ip dhcp snooping binding` | 1-3s | 15s | Up to 10s |
| `show ip arp inspection statistics` | <1s | 10s | <2s |
| `show authentication sessions` | 2-5s | 15s | Up to 12s |
| `show port-security` | 1-2s | 10s | Up to 5s |
| `show port-security interface` | 2-4s | 15s | Up to 8s |
| `show ip verify source` | 1-2s | 10s | Up to 5s |
| `show vlan private-vlan` | <1s | 10s | <2s |

**Total L2 Security Commands**: ~10-20 seconds (small network), up to 45 seconds (large network)

---

## Acceptance Criteria

- [ ] All 7 commands defined in orchestrator
- [ ] Commands execute successfully on Cisco IOS 15.x
- [ ] Commands execute successfully on Cisco NXOS 7.x+
- [ ] Parser functions created for all 7 commands
- [ ] Parser unit tests pass with 100% coverage
- [ ] Operational data structures populated correctly
- [ ] DHCP snooping bindings enrich FDB confidence to 1.0
- [ ] Authentication session state populates interface auth_status field
- [ ] Error handling covers all edge cases

---

## Status

✅ **Phase 1.0.3 Spec Complete** - Ready for orchestrator implementation  
⏭️ **Next**: Phase 1.0.4 - Implement parsers

---

**Document Version**: 1.0  
**Last Updated**: 2025-10-26


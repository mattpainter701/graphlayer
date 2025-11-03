# Cisco L2 Security Mapper Usage Guide

**Version**: 1.0
**Date**: 2025-10-28
**Author**: AI Assistant (Phase 1.5 - Sprint 1.5.5)

---

## Table of Contents

1. [Overview](#overview)
2. [Architecture](#architecture)
3. [Confidence Scoring System](#confidence-scoring-system)
4. [L2 Decision Engine](#l2-decision-engine)
5. [Usage Examples](#usage-examples)
6. [API Reference](#api-reference)
7. [Testing](#testing)
8. [Troubleshooting](#troubleshooting)
9. [Best Practices](#best-practices)

---

## Overview

The Cisco L2 Security Mappers transform vendor-specific operational security data (DHCP snooping, 802.1X/MAB, DAI, IPSG, port security) into vendor-agnostic models with **confidence scoring** and **L2 decision caching** for the L2 decision engine.

### Key Features

- ✅ **Evidence-Based Confidence Scoring** - Multi-source scoring (0-100) for host-to-port bindings
- ✅ **L2 Decision Engine Integration** - Allowed/Conditional/Blocked decisions with reasons
- ✅ **Interface Security Posture** - Per-interface security score (0-100) and level (high/medium/low/none)
- ✅ **VPC-Aware Scoring (NXOS)** - VPC peer-link and primary entry marker preservation
- ✅ **TTL-Based Decision Caching** - Performance optimization with configurable TTL
- ✅ **Freshness Classification** - fresh/stale/old classification based on MAC age

### Supported Platforms

- **Cisco IOS** - Full L2 security mapper support
- **Cisco NXOS** - Full L2 security mapper support with VPC features

---

## Architecture

### Data Flow

```
┌─────────────────────────────────────────────────────────────────────┐
│                        Operational Commands                          │
│  (show ip dhcp snooping binding, show authentication sessions, etc.) │
└──────────────────────────────┬──────────────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────────────┐
│                             Parsers                                  │
│  (ParseDHCPSnoopingBinding, ParseAuthenticationSessions, etc.)       │
└──────────────────────────────┬──────────────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────────────┐
│                          L2 Mappers                                  │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │ buildL2SecurityControls()                                   │    │
│  │ → Global L2 controls + Per-interface security state         │    │
│  └─────────────────────────────────────────────────────────────┘    │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │ buildEnhancedFDBEntries()                                   │    │
│  │ → FDB entries with confidence scoring from evidence sources │    │
│  └─────────────────────────────────────────────────────────────┘    │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │ buildL2DecisionCache()                                      │    │
│  │ → Allowed/Conditional/Blocked decisions with TTL caching    │    │
│  └─────────────────────────────────────────────────────────────┘    │
└──────────────────────────────┬──────────────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────────────┐
│                        L2 Models                                     │
│  (CiscoGlobalL2Controls, CiscoL2InterfaceSecurity,                  │
│   CiscoFDBEntry, L2DecisionCacheEntry)                              │
└──────────────────────────────┬──────────────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────────────┐
│                      L2 Decision Engine                              │
│  (Topology modeling, attack-path analysis, host placement)           │
└─────────────────────────────────────────────────────────────────────┘
```

### Mapper Functions

| Function | Platform | Purpose |
|----------|----------|---------|
| `buildL2SecurityControls()` | IOS | Maps DHCP/DAI/802.1X/IPSG/port-sec to global + per-interface models |
| `buildEnhancedFDBEntries()` | IOS | Creates FDB entries with evidence-based confidence scoring |
| `buildL2DecisionCache()` | IOS | Generates Allowed/Conditional/Blocked decisions with TTL |
| `buildNXOSL2SecurityControls()` | NXOS | Same as IOS but for NXOS operational data |
| `buildNXOSEnhancedFDBEntries()` | NXOS | FDB entries with VPC-aware confidence scoring |
| `buildNXOSL2DecisionCache()` | NXOS | L2 decisions with VPC metadata preservation |

---

## Confidence Scoring System

### Evidence Hierarchy

The confidence scoring system uses **6 evidence sources** ranked by trust level:

| Rank | Evidence Source | Points | Description |
|------|-----------------|--------|-------------|
| 1 | **DHCP Snooping Binding** | +40 | PRIMARY evidence - proves host legitimacy via DHCP |
| 2 | **802.1X/MAB Auth (success)** | +35 | Proves host identity via network authentication |
| 2b | **802.1X/MAB Auth (enabled)** | +15 | Auth enabled but not successful |
| 3 | **IPSG Binding** | +15 | Validates source IP/MAC pairing |
| 4 | **Port Security Learned MAC** | +20 | MAC explicitly allowed on port (type=secure) |
| 5 | **ARP Table Correlation** | +10 | Secondary evidence for IP-to-MAC correlation |
| 6 | **MAC Table Entry Type** | +5 to +20 | Base confidence from MAC table |
|   | - Dynamic | +5 | Normal learned entry |
|   | - Static | +10 | Manually configured |
|   | - Secure | +20 | Port security learned |

### Confidence Classification

| Confidence Score | Classification | Typical Use Case |
|------------------|----------------|------------------|
| **70-100** | **High** | Fully authenticated + DHCP bound + multiple evidence sources |
| **40-69** | **Medium** | Partial evidence (DHCP OR auth, but not both) |
| **20-39** | **Low** | Minimal evidence (MAC table + ARP only) |
| **0-19** | **Low** | MAC table only, no corroborating evidence |

### Example Calculations

#### Example 1: Full Security Stack (Score = 100)
```
Host: 00:50:56:bb:00:01 on GigabitEthernet1/0/1
Evidence Sources:
- MAC table (dynamic): +5
- DHCP snooping binding: +40
- 802.1X authentication (success): +35
- IPSG binding: +15
- ARP table correlation: +10
---
Total: 105 → capped at 100
Confidence: HIGH
Decision: ALLOWED
```

#### Example 2: DHCP Only (Score = 55)
```
Host: 00:50:56:bb:00:02 on GigabitEthernet1/0/2
Evidence Sources:
- MAC table (dynamic): +5
- DHCP snooping binding: +40
- ARP table correlation: +10
---
Total: 55
Confidence: MEDIUM
Decision: CONDITIONAL (DHCP bound but not authenticated)
```

#### Example 3: MAC Table Only (Score = 5)
```
Host: aa:bb:cc:dd:ee:ff on GigabitEthernet1/0/3
Evidence Sources:
- MAC table (dynamic): +5
---
Total: 5
Confidence: LOW
Decision: BLOCKED (insufficient evidence)
```

---

## L2 Decision Engine

### Decision Logic

The L2 decision engine provides **Allowed/Conditional/Blocked** determinations based on confidence score and security controls state.

#### Decision Matrix

| Authentication | DHCP Snooping | Confidence Score | Decision | Reason |
|----------------|---------------|------------------|----------|--------|
| ✅ Success | ✅ Bound | >= 70 | **Allowed** | Full security stack |
| ✅ Success | ❌ Not Bound | >= 60 | **Conditional** | Auth but no DHCP |
| ❌ Not Auth | ✅ Bound | >= 50 | **Conditional** | DHCP but no auth |
| ❌ Not Auth | ❌ Not Bound | 40-69 | **Conditional** | Partial evidence |
| ❌ Not Auth | ❌ Not Bound | < 40 | **Blocked** | Insufficient evidence |
| Any | Any | Violations | **Blocked** | Security violations |

### Decision Reasons

Each decision includes a human-readable reason:

```go
// Example decision reasons:
"Authenticated (802.1X/MAB) + DHCP snooping bound + high confidence"
"Authenticated but missing DHCP snooping binding"
"DHCP snooping bound but not authenticated"
"Medium confidence (55) with partial evidence: [mac-table, dhcp-snooping, arp]"
"Low confidence (5), insufficient evidence"
"Port security violations detected (5)"
```

### Decision Cache with TTL

Decisions are cached with TTL for performance:

```go
decision := L2DecisionCacheEntry{
    DeviceID:            "switch-01",
    MAC:                 "00:50:56:bb:00:01",
    Decision:            "Allowed",
    DecisionReason:      "Authenticated (802.1X/MAB) + DHCP snooping bound + high confidence",
    DecisionConfidence:  85,
    CachedAt:            "2025-10-28T10:00:00Z",  // RFC3339
    ExpiresAt:           "2025-10-28T11:00:00Z",  // RFC3339 (1 hour later)
    TTLSeconds:          3600,                     // 1 hour
    // ... security controls state ...
}
```

**TTL Recommendations**:
- High-confidence decisions: 1-4 hours
- Medium-confidence decisions: 30-60 minutes
- Low-confidence decisions: 5-15 minutes

---

## Usage Examples

### Example 1: IOS L2 Security Mapping

```go
package main

import (
    "fmt"
    "github.com/your-org/cisco"
)

func main() {
    // Create IOS processor
    processor := &cisco.CiscoIOSProcessor{
        iosConfig: &cisco.IOSConfig{Hostname: "switch-01"},
    }

    // Parse operational data
    dhcpBindings := cisco.ParseDHCPSnoopingBinding(dhcpOutput)
    daiStats := cisco.ParseARPInspectionStatistics(daiOutput)
    authSessions := cisco.ParseAuthenticationSessions(authOutput)
    portSecEntries := cisco.ParsePortSecurity(portSecOutput)
    ipsgBindings := cisco.ParseIPVerifySource(ipsgOutput)
    pvlanInfo := cisco.ParsePrivateVLAN(pvlanOutput)

    // Build L2 security controls
    globalControls, interfaceSecurity := processor.buildL2SecurityControls(
        dhcpBindings,
        daiStats,
        authSessions,
        portSecEntries,
        ipsgBindings,
        pvlanInfo,
    )

    // Print global controls
    fmt.Printf("Global DHCP Snooping: %v\n", globalControls.DHCPSnoopingEnabled)
    fmt.Printf("Global DAI: %v\n", globalControls.DAIEnabled)
    fmt.Printf("Global 802.1X: %v\n", globalControls.Dot1XEnabled)

    // Print interface security
    for iface, sec := range interfaceSecurity {
        fmt.Printf("\nInterface: %s\n", iface)
        fmt.Printf("  Security Score: %d/100\n", sec.SecurityScore)
        fmt.Printf("  Security Level: %s\n", sec.SecurityLevel)
        fmt.Printf("  DHCP Snooping Bindings: %d\n", sec.DHCPSnoopingBindingCount)
        fmt.Printf("  Authentication Status: %s\n", sec.AuthenticationStatus)
        fmt.Printf("  IPSG Enabled: %v\n", sec.IPSGEnabled)
        fmt.Printf("  Port Security Violations: %d\n", sec.PortSecurityViolationCount)
    }
}
```

**Output**:
```
Global DHCP Snooping: true
Global DAI: true
Global 802.1X: true

Interface: GigabitEthernet1/0/1
  Security Score: 85/100
  Security Level: high
  DHCP Snooping Bindings: 1
  Authentication Status: Authc Success
  IPSG Enabled: true
  Port Security Violations: 0

Interface: GigabitEthernet1/0/2
  Security Score: 50/100
  Security Level: medium
  DHCP Snooping Bindings: 1
  Authentication Status:
  IPSG Enabled: false
  Port Security Violations: 0
```

---

### Example 2: FDB Entries with Confidence Scoring

```go
package main

import (
    "fmt"
    "github.com/your-org/cisco"
)

func main() {
    processor := &cisco.CiscoIOSProcessor{
        iosConfig: &cisco.IOSConfig{Hostname: "switch-01"},
    }

    // Parse operational data
    macTable := []models.MACTableEntry{
        {MACAddress: "00:50:56:bb:00:01", VLAN: 10, Port: "Gi1/0/1", Type: "dynamic", Age: 300},
        {MACAddress: "00:50:56:bb:00:02", VLAN: 10, Port: "Gi1/0/2", Type: "dynamic", Age: 600},
    }
    dhcpBindings := cisco.ParseDHCPSnoopingBinding(dhcpOutput)
    authSessions := cisco.ParseAuthenticationSessions(authOutput)
    ipsgBindings := cisco.ParseIPVerifySource(ipsgOutput)
    arpEntries := []models.ARPEntry{
        {IPAddress: "10.0.10.2", MACAddress: "00:50:56:bb:00:01", Interface: "Gi1/0/1"},
    }

    // Build enhanced FDB entries
    fdbEntries := processor.buildEnhancedFDBEntries(
        macTable,
        dhcpBindings,
        authSessions,
        ipsgBindings,
        arpEntries,
    )

    // Print FDB entries with confidence scoring
    for _, fdb := range fdbEntries {
        fmt.Printf("\nMAC: %s\n", fdb.MAC)
        fmt.Printf("  Interface: %s\n", fdb.Interface)
        fmt.Printf("  IP Address: %s\n", fdb.IPAddress)
        fmt.Printf("  Confidence: %s (%d/100)\n", fdb.Confidence, fdb.ConfidenceScore)
        fmt.Printf("  Freshness: %s\n", fdb.Freshness)
        fmt.Printf("  Evidence Sources: %v\n", fdb.EvidenceSources)
        fmt.Printf("  DHCP Bound: %v\n", fdb.DHCPSnoopingBound)
        fmt.Printf("  Authenticated: %v\n", fdb.Authenticated)
        fmt.Printf("  IPSG Bound: %v\n", fdb.IPSGBound)
    }
}
```

**Output**:
```
MAC: 00:50:56:bb:00:01
  Interface: GigabitEthernet1/0/1
  IP Address: 10.0.10.2
  Confidence: high (85/100)
  Freshness: fresh
  Evidence Sources: [mac-table dhcp-snooping dot1x ipsg arp]
  DHCP Bound: true
  Authenticated: true
  IPSG Bound: true

MAC: 00:50:56:bb:00:02
  Interface: GigabitEthernet1/0/2
  IP Address: 10.0.10.3
  Confidence: medium (55/100)
  Freshness: fresh
  Evidence Sources: [mac-table dhcp-snooping arp]
  DHCP Bound: true
  Authenticated: false
  IPSG Bound: false
```

---

### Example 3: L2 Decision Cache

```go
package main

import (
    "fmt"
    "github.com/your-org/cisco"
)

func main() {
    processor := &cisco.CiscoIOSProcessor{
        iosConfig: &cisco.IOSConfig{Hostname: "switch-01"},
    }

    // Build FDB entries (from previous example)
    fdbEntries := processor.buildEnhancedFDBEntries(...)

    // Build interface security (from Example 1)
    _, interfaceSecurity := processor.buildL2SecurityControls(...)

    // Build L2 decision cache with 1 hour TTL
    ttlSeconds := 3600
    decisions := processor.buildL2DecisionCache(
        fdbEntries,
        interfaceSecurity,
        ttlSeconds,
    )

    // Print decisions
    for _, decision := range decisions {
        fmt.Printf("\nMAC: %s\n", decision.MAC)
        fmt.Printf("  Decision: %s\n", decision.Decision)
        fmt.Printf("  Reason: %s\n", decision.DecisionReason)
        fmt.Printf("  Confidence: %d/100\n", decision.DecisionConfidence)
        fmt.Printf("  Security Score: %d/100\n", decision.SecurityScore)
        fmt.Printf("  Authenticated: %v\n", decision.Authenticated)
        fmt.Printf("  DHCP Bound: %v\n", decision.DHCPSnoopingBound)
        fmt.Printf("  IPSG Valid: %v\n", decision.IPSGValid)
        fmt.Printf("  Cached At: %s\n", decision.CachedAt)
        fmt.Printf("  Expires At: %s\n", decision.ExpiresAt)
    }
}
```

**Output**:
```
MAC: 00:50:56:bb:00:01
  Decision: Allowed
  Reason: Authenticated (802.1X/MAB) + DHCP snooping bound + high confidence
  Confidence: 85/100
  Security Score: 90/100
  Authenticated: true
  DHCP Bound: true
  IPSG Valid: true
  Cached At: 2025-10-28T10:00:00Z
  Expires At: 2025-10-28T11:00:00Z

MAC: 00:50:56:bb:00:02
  Decision: Conditional
  Reason: DHCP snooping bound but not authenticated
  Confidence: 55/100
  Security Score: 50/100
  Authenticated: false
  DHCP Bound: true
  IPSG Valid: false
  Cached At: 2025-10-28T10:00:00Z
  Expires At: 2025-10-28T11:00:00Z

MAC: aa:bb:cc:dd:ee:ff
  Decision: Blocked
  Reason: Low confidence (5), insufficient evidence
  Confidence: 5/100
  Security Score: 10/100
  Authenticated: false
  DHCP Bound: false
  IPSG Valid: false
  Cached At: 2025-10-28T10:00:00Z
  Expires At: 2025-10-28T11:00:00Z
```

---

### Example 4: NXOS with VPC

```go
package main

import (
    "fmt"
    "github.com/your-org/cisco"
)

func main() {
    processor := &cisco.CiscoNXOSProcessor{
        nxosConfig: &cisco.NXOSConfig{Hostname: "nexus-01"},
    }

    // Parse NXOS operational data
    macTable := []cisco.NXOSMACEntry{
        {
            MACAddress:    "00:50:56:bb:00:01",
            VLAN:          10,
            Port:          "Ethernet1/1",
            Type:          "dynamic",
            Age:           300,
            IsVPCPeerLink: true,  // VPC peer-link marker
            IsPrimary:     true,  // Primary entry marker
        },
    }
    dhcpBindings := cisco.ParseDHCPSnoopingBinding(dhcpOutput)
    authSessions := cisco.ParseNXOSAuthenticationSessions(authOutput)
    ipsgBindings := []cisco.IPVerifySourceBinding{}
    arpEntries := []cisco.NXOSARPEntry{}

    // Build enhanced FDB entries (VPC-aware)
    fdbEntries := processor.buildNXOSEnhancedFDBEntries(
        macTable,
        dhcpBindings,
        authSessions,
        ipsgBindings,
        arpEntries,
    )

    // Print VPC-specific metadata
    for _, fdb := range fdbEntries {
        fmt.Printf("\nMAC: %s\n", fdb.MAC)
        fmt.Printf("  Confidence: %d/100\n", fdb.ConfidenceScore)
        fmt.Printf("  Evidence Sources: %v\n", fdb.EvidenceSources)

        // VPC metadata
        if vpcMarker, exists := fdb.Metadata["vpc_peer_link"]; exists {
            fmt.Printf("  VPC Peer-Link: %s\n", vpcMarker)
        }
        if primaryMarker, exists := fdb.Metadata["primary_entry"]; exists {
            fmt.Printf("  Primary Entry: %s\n", primaryMarker)
        }
    }
}
```

**Output**:
```
MAC: 00:50:56:bb:00:01
  Confidence: 85/100
  Evidence Sources: [mac-table vpc-peer-link dhcp-snooping dot1x]
  VPC Peer-Link: true
  Primary Entry: true
```

---

## API Reference

### IOS Mapper Functions

#### `buildL2SecurityControls()`

Maps L2 security data to global and per-interface models.

**Signature**:
```go
func (p *CiscoIOSProcessor) buildL2SecurityControls(
    dhcpBindings []DHCPSnoopingBinding,
    daiStats ARPInspectionStatistics,
    authSessions []AuthenticationSession,
    portSecEntries []PortSecurityEntry,
    ipsgBindings []IPVerifySourceBinding,
    pvlanInfo []PrivateVLANInfo,
) (CiscoGlobalL2Controls, map[string]CiscoL2InterfaceSecurity)
```

**Parameters**:
- `dhcpBindings`: DHCP snooping bindings from `ParseDHCPSnoopingBinding()`
- `daiStats`: DAI statistics from `ParseARPInspectionStatistics()`
- `authSessions`: 802.1X/MAB sessions from `ParseAuthenticationSessions()`
- `portSecEntries`: Port security entries from `ParsePortSecurity()`
- `ipsgBindings`: IPSG bindings from `ParseIPVerifySource()`
- `pvlanInfo`: PVLAN information from `ParsePrivateVLAN()`

**Returns**:
- `CiscoGlobalL2Controls`: Global security toggles (DHCP/DAI/IPSG/Dot1X enabled)
- `map[string]CiscoL2InterfaceSecurity`: Per-interface security state

---

#### `buildEnhancedFDBEntries()`

Creates FDB entries with confidence scoring from multiple evidence sources.

**Signature**:
```go
func (p *CiscoIOSProcessor) buildEnhancedFDBEntries(
    macTable []models.MACTableEntry,
    dhcpBindings []DHCPSnoopingBinding,
    authSessions []AuthenticationSession,
    ipsgBindings []IPVerifySourceBinding,
    arpEntries []models.ARPEntry,
) []CiscoFDBEntry
```

**Parameters**:
- `macTable`: MAC table entries
- `dhcpBindings`: DHCP snooping bindings
- `authSessions`: 802.1X/MAB sessions
- `ipsgBindings`: IPSG bindings
- `arpEntries`: ARP table entries

**Returns**:
- `[]CiscoFDBEntry`: FDB entries with confidence scores and evidence sources

---

#### `buildL2DecisionCache()`

Builds L2 decisions (Allowed/Conditional/Blocked) with TTL caching.

**Signature**:
```go
func (p *CiscoIOSProcessor) buildL2DecisionCache(
    fdbEntries []CiscoFDBEntry,
    interfaceSecurity map[string]CiscoL2InterfaceSecurity,
    ttlSeconds int,
) []L2DecisionCacheEntry
```

**Parameters**:
- `fdbEntries`: Enhanced FDB entries with confidence scoring
- `interfaceSecurity`: Per-interface security state
- `ttlSeconds`: TTL in seconds for decision cache

**Returns**:
- `[]L2DecisionCacheEntry`: L2 decisions with Allowed/Conditional/Blocked determinations

---

### NXOS Mapper Functions

#### `buildNXOSL2SecurityControls()`

NXOS-specific L2 security controls mapping (same interface as IOS).

#### `buildNXOSEnhancedFDBEntries()`

NXOS FDB entries with VPC-aware confidence scoring.

**Signature**:
```go
func (p *CiscoNXOSProcessor) buildNXOSEnhancedFDBEntries(
    macTable []NXOSMACEntry,  // NXOS-specific MAC entry with VPC markers
    dhcpBindings []DHCPSnoopingBinding,
    authSessions []AuthenticationSession,
    ipsgBindings []IPVerifySourceBinding,
    arpEntries []NXOSARPEntry,
) []CiscoFDBEntry
```

**VPC-Specific Features**:
- Detects `IsVPCPeerLink` marker (+5 confidence)
- Preserves `IsPrimary` marker in metadata
- Adds `vpc-peer-link` to evidence sources

#### `buildNXOSL2DecisionCache()`

NXOS L2 decision caching with VPC metadata preservation (same interface as IOS).

---

## Testing

### Running Unit Tests

```bash
# Run IOS mapper tests
go test -v -run TestBuildL2SecurityControls ./cisco
go test -v -run TestCalculateInterfaceSecurityScore ./cisco
go test -v -run TestBuildEnhancedFDBEntries ./cisco
go test -v -run TestBuildL2DecisionCache ./cisco

# Run NXOS mapper tests
go test -v -run TestBuildNXOSL2SecurityControls ./cisco
go test -v -run TestBuildNXOSEnhancedFDBEntriesVPC ./cisco
go test -v -run TestNXOSIOSConsistency ./cisco

# Run all mapper tests
go test -v ./cisco -run ".*Mapper.*"
```

### Test Coverage

```bash
go test -coverprofile=coverage.out ./cisco
go tool cover -html=coverage.out
```

**Target Coverage**: >= 80% for mapper functions

---

## Troubleshooting

### Issue 1: Low Confidence Scores

**Symptom**: FDB entries have unexpectedly low confidence scores.

**Diagnosis**:
```go
// Print evidence sources for troubleshooting
for _, fdb := range fdbEntries {
    fmt.Printf("MAC %s: Confidence=%d, Sources=%v\n",
        fdb.MAC, fdb.ConfidenceScore, fdb.EvidenceSources)
}
```

**Possible Causes**:
- DHCP snooping not configured/operational
- 802.1X/MAB not configured
- Parser failing to extract data

**Solution**:
- Verify DHCP snooping is enabled: `show ip dhcp snooping`
- Verify 802.1X is configured: `show authentication sessions`
- Check parser output for errors

---

### Issue 2: All Decisions Are "Blocked"

**Symptom**: All L2 decisions are "Blocked" despite security controls being enabled.

**Diagnosis**:
```go
// Print decision reasons
for _, decision := range decisions {
    fmt.Printf("MAC %s: Decision=%s, Reason=%s, Confidence=%d\n",
        decision.MAC, decision.Decision, decision.DecisionReason, decision.DecisionConfidence)
}
```

**Possible Causes**:
- Confidence thresholds too high
- Security controls not operational
- Parser data missing

**Solution**:
- Lower confidence thresholds if appropriate
- Verify operational state of security controls
- Check FDB entries have evidence sources populated

---

### Issue 3: VPC Metadata Not Preserved (NXOS)

**Symptom**: VPC metadata missing in decision cache.

**Diagnosis**:
```go
// Check FDB metadata
for _, fdb := range fdbEntries {
    fmt.Printf("MAC %s: Metadata=%v\n", fdb.MAC, fdb.Metadata)
}

// Check decision metadata
for _, decision := range decisions {
    fmt.Printf("MAC %s: Metadata=%v\n", decision.MAC, decision.Metadata)
}
```

**Solution**:
- Verify NXOS MAC table parser detects `IsVPCPeerLink` and `IsPrimary` markers
- Ensure `buildNXOSL2DecisionCache()` preserves metadata: `decision.Metadata = fdb.Metadata`

---

## Best Practices

### 1. TTL Management

**Recommendation**: Adjust TTL based on confidence level.

```go
// Dynamic TTL based on confidence
func calculateTTL(confidence int) int {
    switch {
    case confidence >= 70:
        return 3600 * 4  // 4 hours for high confidence
    case confidence >= 40:
        return 1800      // 30 minutes for medium confidence
    default:
        return 600       // 10 minutes for low confidence
    }
}

ttl := calculateTTL(fdbEntry.ConfidenceScore)
decisions := processor.buildL2DecisionCache(fdbEntries, interfaceSecurity, ttl)
```

---

### 2. Periodic Re-Evaluation

**Recommendation**: Re-run mappers periodically to refresh confidence scores.

```go
ticker := time.NewTicker(15 * time.Minute)
defer ticker.Stop()

for range ticker.C {
    // Re-parse operational data
    dhcpBindings := ParseDHCPSnoopingBinding(fetchDHCPOutput())
    authSessions := ParseAuthenticationSessions(fetchAuthOutput())
    // ... parse other sources ...

    // Re-build L2 security controls
    globalControls, interfaceSecurity := processor.buildL2SecurityControls(...)

    // Re-build FDB entries with updated confidence
    fdbEntries := processor.buildEnhancedFDBEntries(...)

    // Re-build decision cache
    decisions := processor.buildL2DecisionCache(...)

    // Store updated decisions
    storeDecisions(decisions)
}
```

---

### 3. Evidence Source Validation

**Recommendation**: Validate evidence sources before using confidence scores.

```go
// Require minimum number of evidence sources
func isHighConfidence(fdb CiscoFDBEntry) bool {
    if fdb.ConfidenceScore < 70 {
        return false
    }

    // Require at least 3 evidence sources for high confidence
    if len(fdb.EvidenceSources) < 3 {
        return false
    }

    // Require DHCP or authentication for high confidence
    hasDHCP := contains(fdb.EvidenceSources, "dhcp-snooping")
    hasAuth := contains(fdb.EvidenceSources, "dot1x")

    return hasDHCP || hasAuth
}
```

---

### 4. Monitoring & Alerting

**Recommendation**: Monitor security posture changes.

```go
// Track security score changes
func monitorSecurityChanges(oldSec, newSec CiscoL2InterfaceSecurity) {
    scoreDelta := newSec.SecurityScore - oldSec.SecurityScore

    if scoreDelta <= -20 {
        // Security score dropped significantly
        alert := fmt.Sprintf("Security score dropped on %s: %d → %d",
            newSec.Interface, oldSec.SecurityScore, newSec.SecurityScore)
        sendAlert(alert)
    }

    if oldSec.PortSecurityViolationCount == 0 && newSec.PortSecurityViolationCount > 0 {
        // New port security violations
        alert := fmt.Sprintf("Port security violations detected on %s: %d violations",
            newSec.Interface, newSec.PortSecurityViolationCount)
        sendAlert(alert)
    }
}
```

---

## Additional Resources

### Documentation
- [PHASE_1_5_PROGRESS.md](PHASE_1_5_PROGRESS.md) - Phase 1.5 overall progress
- [SPRINT_1_5_4_MAPPER_COMPLETION.md](SPRINT_1_5_4_MAPPER_COMPLETION.md) - Sprint 1.5.4 completion details
- [PARSER_DOCUMENTATION.md](../sample_data/suggested parser/PARSER_DOCUMENTATION.md) - Parser reference

### Code Files
- [completed_cisco.ios.mappers.go](../sample_data/suggested parser/completed_cisco.ios.mappers.go) - IOS mappers
- [cisco.nxos.mappers.go](../sample_data/suggested parser/cisco.nxos.mappers.go) - NXOS mappers
- [cisco.ios.mappers_test.go](../sample_data/suggested parser/cisco.ios.mappers_test.go) - IOS tests
- [cisco.nxos.mappers_test.go](../sample_data/suggested parser/cisco.nxos.mappers_test.go) - NXOS tests
- [completed_cisco.models.go](../sample_data/suggested_modeling/completed_cisco.models.go) - L2 models

---

**Version**: 1.0
**Last Updated**: 2025-10-28
**Author**: AI Assistant (Phase 1.5 - Sprint 1.5.5)

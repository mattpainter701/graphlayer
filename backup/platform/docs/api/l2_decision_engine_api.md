# L2 Decision Engine API Documentation

**Version**: 1.0.0
**Last Updated**: 2025-10-31
**Status**: Production Ready

---

## Overview

The L2 Decision Engine provides reference-like Layer 2 reachability determination with evidence-based reasoning. It produces **Allowed**, **Conditional**, or **Blocked** outcomes with transparent reason codes for network threat simulation and security posture analysis.

### Key Features

- ✅ **Multi-Source Evidence Correlation** - DHCP, 802.1X, IPSG, ARP, MAC table
- ✅ **Confidence Scoring** - 0-100 scoring with transparent evidence sources
- ✅ **L2→L3 Handoff Detection** - Automatic detection for attack path stitching
- ✅ **PVLAN Isolation** - Private VLAN isolation rule enforcement
- ✅ **Configurable Thresholds** - Adjustable Allowed/Conditional thresholds
- ✅ **Decision Caching** - TTL-based caching for what-if analysis performance

---

## Table of Contents

1. [Quick Start](#quick-start)
2. [Core API](#core-api)
3. [Configuration Options](#configuration-options)
4. [Decision Logic](#decision-logic)
5. [Reason Codes Reference](#reason-codes-reference)
6. [Evidence Structure](#evidence-structure)
7. [Usage Examples](#usage-examples)
8. [Performance Considerations](#performance-considerations)

---

## Quick Start

### Installation

```go
import "github.com/mattpainter701/graphlayer/platform/platform/orchestration/decision"
```

### Basic Usage

```go
// Create decision engine with default configuration
engine := decision.NewL2DecisionEngine()

// Load evidence sources
engine.LoadMACTable(macTableEntries)
engine.LoadDHCPBindings(dhcpBindings)
engine.LoadAuthSessions(authSessions)

// Make a reachability decision
query := decision.L2ReachabilityQuery{
    SourceMAC:       "00:11:22:33:44:55",
    DestMAC:         "aa:bb:cc:dd:ee:ff",
    VLANID:          100,
    SourceInterface: "GigabitEthernet1/0/1",
    DestInterface:   "GigabitEthernet1/0/2",
}

result := engine.DetermineL2Reachability(query)

fmt.Printf("Outcome: %s\n", result.Outcome)             // "Allowed", "Conditional", or "Blocked"
fmt.Printf("Reason: %s\n", result.ReasonSummary)         // Human-readable reason
fmt.Printf("Confidence: %d\n", result.ConfidenceScore)   // 0-100
fmt.Printf("Reason Codes: %v\n", result.ReasonCodes)     // Machine-readable codes
```

---

## Core API

### L2DecisionEngine

#### Constructor

```go
func NewL2DecisionEngine() *L2DecisionEngine
```

Creates a new decision engine with default configuration:
- **EnableStrictMode**: `true` (require both auth + DHCP for Allowed)
- **ConditionalThreshold**: `40` (minimum confidence for Conditional)
- **AllowedThreshold**: `70` (minimum confidence for Allowed)
- **DecisionTTL**: `5 minutes` (decision cache lifetime)

**Returns**: Configured decision engine ready for use

---

#### Primary Decision Method

```go
func (e *L2DecisionEngine) DetermineL2Reachability(query L2ReachabilityQuery) L2Decision
```

**Purpose**: Evaluates whether source MAC can reach destination MAC at Layer 2.

**Parameters**:
- `query` - L2ReachabilityQuery containing:
  - `SourceMAC` (string, required) - Source MAC address
  - `DestMAC` (string, required) - Destination MAC address
  - `VLANID` (int, required) - VLAN ID for the query
  - `SourceInterface` (string, optional) - Source interface if known
  - `DestInterface` (string, optional) - Destination interface if known

**Returns**: `L2Decision` containing:
- `Outcome` - "Allowed", "Conditional", or "Blocked"
- `ReasonCodes` - Array of reason code constants
- `ReasonSummary` - Human-readable explanation
- `SourceEvidence` - Evidence for source MAC
- `DestEvidence` - Evidence for destination MAC
- `ConfidenceScore` - Overall confidence (0-100)
- `HandoffToL3` - Boolean indicating if L3 routing needed
- `EvaluatedAt` - Decision timestamp
- `ExpiresAt` - Decision expiration (for caching)

**Example**:
```go
query := decision.L2ReachabilityQuery{
    SourceMAC: "00:11:22:33:44:55",
    DestMAC:   "aa:bb:cc:dd:ee:ff",
    VLANID:    100,
}

decision := engine.DetermineL2Reachability(query)

switch decision.Outcome {
case "Allowed":
    fmt.Println("Communication permitted at L2")
case "Conditional":
    fmt.Printf("Conditional access: %s\n", decision.ReasonSummary)
case "Blocked":
    fmt.Printf("Communication blocked: %s\n", decision.ReasonSummary)
    if decision.HandoffToL3 {
        fmt.Println("Requires L3 routing")
    }
}
```

---

#### Evidence Loading Methods

```go
func (e *L2DecisionEngine) LoadMACTable(entries []MACTableEntry)
```
**Purpose**: Load MAC address table entries

**Parameters**:
```go
type MACTableEntry struct {
    MAC       string
    VLAN      int
    Interface string
    Type      string // "dynamic", "static", "secure"
    Age       int    // Age in seconds
}
```

**Example**:
```go
macTable := []decision.MACTableEntry{
    {
        MAC:       "00:11:22:33:44:55",
        VLAN:      100,
        Interface: "GigabitEthernet1/0/1",
        Type:      "dynamic",
        Age:       300,
    },
}
engine.LoadMACTable(macTable)
```

---

```go
func (e *L2DecisionEngine) LoadDHCPBindings(bindings []DHCPBinding)
```
**Purpose**: Load DHCP snooping bindings (PRIMARY evidence source)

**Parameters**:
```go
type DHCPBinding struct {
    MAC         string
    IP          string
    VLAN        int
    Interface   string
    LeaseTime   int
    CollectedAt time.Time
}
```

**Confidence Impact**: +40 points (highest single source)

---

```go
func (e *L2DecisionEngine) LoadAuthSessions(sessions []AuthSession)
```
**Purpose**: Load 802.1X/MAB authentication sessions (CRITICAL for Allowed)

**Parameters**:
```go
type AuthSession struct {
    MAC         string
    IP          string
    VLAN        int
    Interface   string
    Method      string // "dot1x", "mab"
    Status      string // "Authc Success", "Authorized", "Authc Failed"
    CollectedAt time.Time
}
```

**Confidence Impact**:
- **+35 points** if Status = "Authc Success" or "Authorized"
- **+10 points** if auth enabled but not successful

---

```go
func (e *L2DecisionEngine) LoadInterfaceSecurity(states map[string]InterfaceSecurityState)
```
**Purpose**: Load per-interface security control state

**Parameters**:
```go
type InterfaceSecurityState struct {
    Interface            string
    DHCPSnoopingTrust    bool
    ARPInspectionTrust   bool
    IPSGEnabled          bool
    Dot1XEnabled         bool
    MABEnabled           bool
    PortSecurityEnabled  bool
    UnauthVLAN           int
    SecurityScore        int    // 0-100
    SecurityLevel        string // "high", "medium", "low", "none"
}
```

---

```go
func (e *L2DecisionEngine) LoadPVLANIsolation(isolation map[int]PVLANIsolation)
```
**Purpose**: Load Private VLAN isolation rules

**Parameters**:
```go
type PVLANIsolation struct {
    PrimaryVLAN      int
    SecondaryVLANs   map[int]string // VLAN → type ("isolated", "community")
    IsolatedPorts    []string
    PromiscuousPorts []string
}
```

---

## Configuration Options

### Strict Mode

**Default**: `true`

```go
engine.EnableStrictMode = true  // Require both auth + DHCP for Allowed
engine.EnableStrictMode = false // Auth OR DHCP sufficient for Allowed
```

**Strict Mode** (default):
- **Allowed** outcome requires: Authenticated **AND** DHCP bound **AND** confidence ≥ 70
- Most secure setting
- Recommended for production environments

**Non-Strict Mode**:
- **Allowed** outcome requires: (Authenticated **OR** DHCP bound) **AND** confidence ≥ 70
- More permissive
- Useful for environments with incomplete evidence

---

### Confidence Thresholds

```go
engine.ConditionalThreshold = 40  // Minimum confidence for Conditional (default: 40)
engine.AllowedThreshold = 70      // Minimum confidence for Allowed (default: 70)
```

**Threshold Guidelines**:
- **Allowed Threshold**: 70-90 (default: 70)
  - Higher = more strict (requires more evidence)
  - Lower = more permissive (less evidence needed)

- **Conditional Threshold**: 30-50 (default: 40)
  - Defines boundary between Conditional and Blocked
  - Should be significantly lower than Allowed threshold

**Example Configurations**:

```go
// High Security Environment
engine.AllowedThreshold = 80
engine.ConditionalThreshold = 50
engine.EnableStrictMode = true

// Medium Security Environment (default)
engine.AllowedThreshold = 70
engine.ConditionalThreshold = 40
engine.EnableStrictMode = true

// Low Security / Legacy Environment
engine.AllowedThreshold = 60
engine.ConditionalThreshold = 30
engine.EnableStrictMode = false
```

---

### Decision Caching TTL

```go
engine.DecisionTTL = 5 * time.Minute  // Default: 5 minutes
```

**Purpose**: Control how long decisions are cached for what-if analysis performance.

**Recommendations**:
- **5 minutes** - Default, good balance for most environments
- **1 minute** - Rapidly changing environments (conferences, guest networks)
- **15 minutes** - Stable environments with infrequent changes
- **1 hour** - Static environments, historical analysis

---

## Decision Logic

### Decision Flow

```
1. Gather Evidence
   ├─ Source MAC: Check MAC table, DHCP, 802.1X, IPSG, ARP
   └─ Dest MAC: Check MAC table, DHCP, 802.1X, IPSG, ARP

2. Calculate Confidence Scores
   ├─ Source confidence (0-100)
   ├─ Dest confidence (0-100)
   └─ Overall = min(source, dest)

3. Check Blocking Conditions
   ├─ No MAC table entry → BLOCKED
   ├─ Different VLANs → BLOCKED + L3 handoff
   ├─ PVLAN isolation → BLOCKED
   ├─ Port security violations → BLOCKED
   └─ Authentication failed → BLOCKED

4. Apply Decision Logic
   ├─ Strict Mode:
   │   ├─ Auth + DHCP + conf≥70 → ALLOWED
   │   ├─ Auth XOR DHCP + conf≥40 → CONDITIONAL
   │   └─ conf < 40 → BLOCKED
   └─ Non-Strict Mode:
       ├─ (Auth OR DHCP) + conf≥70 → ALLOWED
       ├─ conf≥40 → CONDITIONAL
       └─ conf < 40 → BLOCKED
```

---

### Confidence Scoring Algorithm

**Evidence Source Contributions**:

| Evidence Source | Confidence Points | Notes |
|----------------|------------------|-------|
| DHCP Snooping Binding | **+40** | PRIMARY evidence source |
| 802.1X/MAB Success | **+35** | CRITICAL for Allowed |
| 802.1X/MAB Enabled | **+10** | Auth enabled but not successful |
| IPSG Binding | **+15** | IP Source Guard validation |
| Port Security (secure MAC) | **+20** | Port security learned MAC |
| ARP Table Entry | **+10** | IP-to-MAC correlation |
| Fresh MAC Entry (≤10 min) | **+5** | Recent MAC table entry |
| Static MAC | **+10** | Manually configured MAC |

**Total Possible**: ~100+ (capped at practical maximum)

**Confidence Tiers**:
- **High (70-100)**: Multiple evidence sources, authenticated, DHCP bound
- **Medium (40-69)**: Partial evidence (e.g., DHCP only or auth only)
- **Low (0-39)**: Minimal evidence (e.g., MAC table only, stale)

---

## Reason Codes Reference

### Allowed Reason Codes

| Code | Constant | Meaning |
|------|----------|---------|
| `AUTHENTICATED_AND_DHCP_BOUND` | `ReasonAuthenticatedAndBound` | Authenticated (802.1X/MAB) + DHCP snooping bound + high confidence |
| `AUTHENTICATED_HIGH_CONFIDENCE` | `ReasonAuthenticatedHighConf` | Authenticated with high confidence (non-strict mode) |
| `STATIC_MAC_ENTRY` | `ReasonStaticMAC` | Static/secure MAC entry (port security) |

---

### Conditional Reason Codes

| Code | Constant | Meaning |
|------|----------|---------|
| `AUTHENTICATED_NO_DHCP_BINDING` | `ReasonAuthenticatedNoDHCP` | Authenticated but missing DHCP snooping binding |
| `DHCP_BOUND_NOT_AUTHENTICATED` | `ReasonDHCPBoundNotAuth` | DHCP snooping bound but not authenticated |
| `MEDIUM_CONFIDENCE_PARTIAL_EVIDENCE` | `ReasonMediumConfidence` | Medium confidence (40-69) with partial evidence |
| `FRESH_MAC_ENTRY` | `ReasonFreshMACEntry` | Recent MAC table entry (≤10 min) |

---

### Blocked Reason Codes

| Code | Constant | Meaning |
|------|----------|---------|
| `LOW_CONFIDENCE_INSUFFICIENT_EVIDENCE` | `ReasonLowConfidence` | Low confidence (<40), insufficient evidence |
| `PORT_SECURITY_VIOLATION` | `ReasonPortSecurityViolation` | Port security violations detected |
| `AUTHENTICATION_FAILED` | `ReasonAuthenticationFailed` | 802.1X/MAB authentication failed |
| `PVLAN_ISOLATED` | `ReasonPVLANIsolated` | Private VLAN isolation blocks communication |
| `DIFFERENT_VLAN` | `ReasonDifferentVLAN` | Source and dest in different VLANs (L3 handoff) |
| `NO_MAC_TABLE_ENTRY` | `ReasonNoMACEntry` | MAC not found in MAC address table |
| `STALE_EVIDENCE` | `ReasonStaleEvidence` | All evidence > 1 hour old |

---

## Evidence Structure

### Evidence Type

```go
type Evidence struct {
    MACTableEntry      bool     // MAC found in table
    DHCPBound          bool     // DHCP snooping binding exists
    Authenticated      bool     // 802.1X/MAB success
    AuthMethod         string   // "dot1x", "mab", ""
    IPSGBound          bool     // IP Source Guard binding
    PortSecurityValid  bool     // Port security learned (no violations)
    ARPEntryExists     bool     // ARP table correlation
    EvidenceSources    []string // List of evidence sources (e.g., ["mac-table", "dhcp-snooping", "dot1x"])
    ConfidenceScore    int      // Aggregated 0-100 score
}
```

**Example Evidence**:
```go
// High confidence evidence
evidence := Evidence{
    MACTableEntry:   true,
    DHCPBound:       true,
    Authenticated:   true,
    AuthMethod:      "dot1x",
    IPSGBound:       true,
    ARPEntryExists:  true,
    EvidenceSources: []string{"mac-table", "dhcp-snooping", "dot1x", "ipsg", "arp"},
    ConfidenceScore: 90,
}
```

---

## Usage Examples

### Example 1: High Security Campus Network

```go
// Configure for high security environment
engine := decision.NewL2DecisionEngine()
engine.EnableStrictMode = true
engine.AllowedThreshold = 75  // Require strong evidence
engine.ConditionalThreshold = 45

// Load comprehensive evidence
engine.LoadMACTable(macTable)
engine.LoadDHCPBindings(dhcpBindings)
engine.LoadAuthSessions(authSessions)
engine.LoadInterfaceSecurity(interfaceSecurity)

// Make decision
query := decision.L2ReachabilityQuery{
    SourceMAC:       "00:11:22:33:44:55",
    DestMAC:         "aa:bb:cc:dd:ee:ff",
    VLANID:          100,
    SourceInterface: "Gi1/0/1",
    DestInterface:   "Gi1/0/2",
}

result := engine.DetermineL2Reachability(query)

if result.Outcome == "Allowed" {
    fmt.Printf("✓ Communication ALLOWED\n")
    fmt.Printf("  Confidence: %d/100\n", result.ConfidenceScore)
    fmt.Printf("  Source Evidence: %v\n", result.SourceEvidence.EvidenceSources)
} else {
    fmt.Printf("✗ Communication %s\n", result.Outcome)
    fmt.Printf("  Reason: %s\n", result.ReasonSummary)
}
```

---

### Example 2: Guest Network (Permissive)

```go
// Configure for guest network (more permissive)
engine := decision.NewL2DecisionEngine()
engine.EnableStrictMode = false  // Auth OR DHCP sufficient
engine.AllowedThreshold = 60
engine.ConditionalThreshold = 30

// Load minimal evidence (MAB only, no 802.1X)
engine.LoadMACTable(macTable)
engine.LoadDHCPBindings(dhcpBindings)
engine.LoadAuthSessions(mabSessions)  // MAB sessions only

query := decision.L2ReachabilityQuery{
    SourceMAC: "guest:device:mac",
    DestMAC:   "internet:gateway",
    VLANID:    200,
}

result := engine.DetermineL2Reachability(query)
fmt.Printf("Guest access: %s\n", result.Outcome)
```

---

### Example 3: Threat Simulation (Attack Path Analysis)

```go
// Check if compromised host can reach critical server
engine := decision.NewL2DecisionEngine()
engine.LoadMACTable(macTable)
engine.LoadDHCPBindings(dhcpBindings)

query := decision.L2ReachabilityQuery{
    SourceMAC: "compromised:workstation",
    DestMAC:   "database:server",
    VLANID:    100,
}

result := engine.DetermineL2Reachability(query)

if result.HandoffToL3 {
    fmt.Printf("✓ Attack path requires L3 routing\n")
    fmt.Printf("  Analyze firewall policies for VLAN %d → VLAN %d\n",
        srcVLAN, dstVLAN)
} else if result.Outcome == "Allowed" {
    fmt.Printf("⚠ ALERT: Direct L2 path available!\n")
    fmt.Printf("  Lateral movement possible\n")
    fmt.Printf("  Confidence: %d/100\n", result.ConfidenceScore)
} else {
    fmt.Printf("✓ L2 communication blocked: %s\n", result.ReasonSummary)
}
```

---

### Example 4: What-If Analysis (Security Control Impact)

```go
// Scenario 1: Current state
engineBefore := decision.NewL2DecisionEngine()
engineBefore.LoadMACTable(macTable)
engineBefore.LoadDHCPBindings(dhcpBindings)
// No auth sessions loaded

resultBefore := engineBefore.DetermineL2Reachability(query)
fmt.Printf("Before 802.1X: %s (Confidence: %d)\n",
    resultBefore.Outcome, resultBefore.ConfidenceScore)

// Scenario 2: After enabling 802.1X
engineAfter := decision.NewL2DecisionEngine()
engineAfter.LoadMACTable(macTable)
engineAfter.LoadDHCPBindings(dhcpBindings)
engineAfter.LoadAuthSessions(authSessions)  // Now with 802.1X

resultAfter := engineAfter.DetermineL2Reachability(query)
fmt.Printf("After 802.1X: %s (Confidence: %d)\n",
    resultAfter.Outcome, resultAfter.ConfidenceScore)

// Show impact
if resultBefore.Outcome != resultAfter.Outcome {
    fmt.Printf("✓ 802.1X changes decision: %s → %s\n",
        resultBefore.Outcome, resultAfter.Outcome)
    fmt.Printf("  Confidence improvement: +%d points\n",
        resultAfter.ConfidenceScore - resultBefore.ConfidenceScore)
}
```

---

## Performance Considerations

### Decision Performance

**Typical Performance**:
- **Single decision**: <1 ms
- **1000 decisions**: ~500 ms
- **10,000 decisions**: ~5 seconds

**Optimization Tips**:
1. **Load evidence once**, reuse engine for multiple queries
2. **Use decision caching** with appropriate TTL
3. **Batch decisions** when analyzing large topologies
4. **Pre-compute** interface security scores

### Memory Footprint

**Approximate Memory Usage**:
- **Empty engine**: ~10 KB
- **1000 MAC entries**: ~200 KB
- **1000 DHCP bindings**: ~100 KB
- **1000 auth sessions**: ~150 KB
- **1000 decisions cached**: ~1 MB

**Total for large campus**: ~2-3 MB per 1000 devices

---

### Scaling Recommendations

**Small Network** (< 100 devices):
- Load all evidence into single engine
- No caching needed
- Real-time decisions

**Medium Network** (100-1000 devices):
- Use decision caching (5-min TTL)
- Pre-compute security scores
- Background evidence refresh

**Large Network** (1000+ devices):
- Partition by switch/location
- Distributed decision engines
- Redis/Memcached for decision cache
- Async evidence loading

---

## Error Handling

```go
result := engine.DetermineL2Reachability(query)

// Check for missing evidence
if len(result.SourceEvidence.EvidenceSources) == 0 {
    fmt.Println("⚠ WARNING: No evidence for source MAC")
}

if result.ConfidenceScore < 20 {
    fmt.Println("⚠ WARNING: Very low confidence, results may be unreliable")
}

// Handle L3 handoff
if result.HandoffToL3 {
    fmt.Printf("→ Handoff to L3: %s\n", result.HandoffReason)
    // Invoke L3 routing analysis here
}
```

---

## Version History

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2025-10-31 | Initial production release |

---

## Support & Feedback

**Documentation**: `platform/docs/api/l2_decision_engine_api.md`
**Integration Guide**: `platform/docs/integration/l2_integration_guide.md`
**Operator Guide**: `platform/docs/operations/l2_operations_guide.md`

For issues, questions, or feature requests, please contact the platform team.

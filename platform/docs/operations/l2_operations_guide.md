# L2 Operations Guide

**Version**: 1.0
**Last Updated**: 2025-01-01
**Target Audience**: Network Operators, Security Analysts, DevOps Engineers

---

## Table of Contents

1. [Overview](#overview)
2. [Understanding L2 Decisions](#understanding-l2-decisions)
3. [Interpreting Confidence Scores](#interpreting-confidence-scores)
4. [Evidence Sources Explained](#evidence-sources-explained)
5. [Common Scenarios](#common-scenarios)
6. [Troubleshooting Low Confidence](#troubleshooting-low-confidence)
7. [Reason Codes Reference](#reason-codes-reference)
8. [Monitoring and Alerting](#monitoring-and-alerting)
9. [Best Practices](#best-practices)
10. [FAQ](#faq)

---

## Overview

The **L2 Decision Engine** determines whether network traffic can flow between two MAC addresses at Layer 2. It analyzes multiple security controls (DHCP snooping, 802.1X authentication, port security, etc.) and produces one of three outcomes:

- **Allowed** - Traffic can flow with high confidence (≥70)
- **Conditional** - Traffic may flow but with warnings (40-69)
- **Blocked** - Traffic cannot flow (<40)

This guide helps operators understand these decisions, interpret evidence, and troubleshoot issues.

---

## Understanding L2 Decisions

### Decision Outcomes

#### Allowed
- **Confidence**: 70-100
- **Meaning**: Both source and destination MACs are authenticated, have valid DHCP bindings, and are in the same VLAN
- **Action**: Traffic flows normally
- **Example**: Corporate workstation talking to file server

```json
{
  "outcome": "Allowed",
  "confidence_score": 90,
  "reason_code": "L2_ALLOWED_HIGH_CONFIDENCE",
  "reason_detail": "Both MACs authenticated, DHCP-bound, same VLAN"
}
```

#### Conditional
- **Confidence**: 40-69
- **Meaning**: Some security controls are present but not all requirements met
- **Action**: Traffic may flow but should be monitored
- **Example**: Device has DHCP binding but no 802.1X authentication

```json
{
  "outcome": "Conditional",
  "confidence_score": 55,
  "reason_code": "L2_CONDITIONAL_PARTIAL_AUTH",
  "reason_detail": "Source authenticated but destination only has DHCP binding"
}
```

#### Blocked
- **Confidence**: 0-39
- **Meaning**: Security controls prevent traffic or MACs are in different VLANs
- **Action**: Traffic blocked at Layer 2
- **Example**: Device not in MAC table, failed authentication, or PVLAN isolation

```json
{
  "outcome": "Blocked",
  "confidence_score": 0,
  "reason_code": "L2_BLOCKED_SRC_MAC_NOT_FOUND",
  "reason_detail": "Source MAC not found in forwarding table"
}
```

### Decision Flow

```
┌─────────────────────────────────────────────────────┐
│           L2 Reachability Query                     │
│  (Source MAC, Dest MAC, VLAN ID)                    │
└────────────────┬────────────────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────────────────┐
│       Step 1: Gather Evidence                       │
│  • MAC table lookup                                 │
│  • DHCP binding lookup                              │
│  • 802.1X session lookup                            │
│  • IP Source Guard lookup                           │
│  • ARP entry lookup                                 │
└────────────────┬────────────────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────────────────┐
│       Step 2: Calculate Confidence                  │
│  • DHCP Snooping: +40 points                        │
│  • 802.1X Auth: +35 points                          │
│  • IP Source Guard: +15 points                      │
│  • Port Security: +20 points (if no violations)     │
│  • MAC Table: +10 points (baseline)                 │
└────────────────┬────────────────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────────────────┐
│       Step 3: Apply Decision Logic                  │
│  • Confidence ≥ 70? → Allowed                       │
│  • Confidence 40-69? → Conditional                  │
│  • Confidence < 40? → Blocked                       │
│  • Different VLANs? → Blocked (L3 handoff)          │
│  • PVLAN isolation? → Blocked                       │
└────────────────┬────────────────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────────────────┐
│       Step 4: Return Result                         │
│  • Outcome (Allowed/Conditional/Blocked)            │
│  • Confidence Score (0-100)                         │
│  • Reason Code                                      │
│  • Evidence Trail                                   │
└─────────────────────────────────────────────────────┘
```

---

## Interpreting Confidence Scores

Confidence scores range from **0 to 100** and represent the certainty that traffic is authorized to flow.

### Confidence Score Breakdown

| Score Range | Level | Meaning | Typical Scenario |
|-------------|-------|---------|------------------|
| **90-100** | Very High | Multiple strong evidence sources | 802.1X + DHCP + IPSG + Port Security |
| **70-89** | High | Strong evidence with minor gaps | 802.1X + DHCP (no IPSG) |
| **50-69** | Medium | Some evidence but missing authentication | DHCP binding only (no 802.1X) |
| **40-49** | Low-Medium | Weak evidence | MAC table entry only |
| **20-39** | Low | Very weak evidence | Stale MAC entry |
| **0-19** | Very Low | No evidence or violations | No MAC entry, auth failed, port security violation |

### Confidence Score Calculation

The engine adds confidence points from each evidence source:

```
Total Confidence = Base (10) + DHCP (40) + 802.1X (35) + IPSG (15) + Port Security (20)
Maximum Possible = 120 (capped at 100)
```

**Example 1: High Security Workstation**
```
Base MAC Entry:       +10
DHCP Snooping:        +40
802.1X (dot1x):       +35
IP Source Guard:      +15
Port Security (OK):   +20
─────────────────────────
Total:                120 → Capped at 100
Outcome:              Allowed (Very High Confidence)
```

**Example 2: Guest Device**
```
Base MAC Entry:       +10
DHCP Snooping:        +40
802.1X (MAB):         +25
IP Source Guard:      +0  (not enabled)
Port Security:        +0  (not enabled)
─────────────────────────
Total:                75
Outcome:              Allowed (High Confidence)
```

**Example 3: Legacy Device**
```
Base MAC Entry:       +10
DHCP Snooping:        +0  (not enabled)
802.1X:               +0  (not enabled)
IP Source Guard:      +0  (not enabled)
Port Security:        +0  (not enabled)
─────────────────────────
Total:                10
Outcome:              Blocked (Very Low Confidence)
```

### Confidence Thresholds

Default thresholds (configurable):
- **Allowed Threshold**: 70
- **Conditional Threshold**: 40

```go
// Adjust thresholds for stricter security
engine := decision.NewL2DecisionEngine()
engine.SetAllowedThreshold(80)    // Require 80+ for Allowed
engine.SetConditionalThreshold(50) // Require 50+ for Conditional
```

---

## Evidence Sources Explained

### 1. DHCP Snooping (+40 confidence)

**What it is**: Tracks dynamic IP-to-MAC bindings from DHCP servers

**Why it matters**: Proves the device obtained a legitimate IP address through DHCP

**How to verify**:
```bash
show ip dhcp snooping binding
```

**Expected output**:
```
MacAddress          IpAddress        Lease(sec)  Type           VLAN  Interface
------------------  ---------------  ----------  -------------  ----  --------------------
00:11:22:33:44:55   192.168.1.10     86356       dhcp-snooping  100   GigabitEthernet1/0/1
```

**Troubleshooting**:
- No bindings? Check if DHCP snooping is enabled globally and on VLAN
- Bindings expired? Device may have released DHCP lease
- Wrong interface? Device may have moved ports

### 2. 802.1X Authentication (+35 confidence for dot1x, +25 for MAB)

**What it is**: Network access control using username/password or MAC-based authentication

**Why it matters**: Proves the device (or user) was authenticated by a RADIUS server

**How to verify**:
```bash
show authentication sessions
```

**Expected output**:
```
Interface: GigabitEthernet1/0/1
  MAC Address: 0011.2233.4455
  Status: Authc Success
  Method: dot1x
  Username: john.doe
```

**Troubleshooting**:
- Status "Authc Failed"? Check RADIUS server logs
- Method "mab"? MAC Authentication Bypass (lower confidence than dot1x)
- No session? 802.1X may not be enabled on interface

### 3. IP Source Guard (+15 confidence)

**What it is**: Filters traffic based on DHCP snooping binding table

**Why it matters**: Prevents IP spoofing by validating source IP matches MAC

**How to verify**:
```bash
show ip verify source
```

**Expected output**:
```
Interface  Filter-type  Filter-mode  IP-address       Mac-address        Vlan
---------  -----------  -----------  ---------------  -----------------  ----
Gi1/0/1    ip-mac       active       192.168.1.10     0011.2233.4455     100
```

**Troubleshooting**:
- Not active? Check `ip verify source port-security` command on interface
- Wrong IP? DHCP binding may have changed

### 4. Port Security (+20 confidence if no violations)

**What it is**: Limits MAC addresses allowed on a port and takes action on violations

**Why it matters**: Prevents MAC spoofing and unauthorized devices

**How to verify**:
```bash
show port-security interface GigabitEthernet1/0/1
```

**Expected output**:
```
Port Security              : Enabled
Port Status                : Secure-up
Violation Mode             : Restrict
Maximum MAC Addresses      : 2
Total MAC Addresses        : 1
Security Violation Count   : 0
```

**Troubleshooting**:
- Violation count > 0? Device may be attempting MAC spoofing
- Port status "Secure-down"? Port shut down due to violation
- Total MACs exceeds maximum? Remove unauthorized devices

### 5. MAC Address Table (+10 baseline)

**What it is**: Forwarding table mapping MAC addresses to switch ports

**Why it matters**: Proves the device is actively sending traffic

**How to verify**:
```bash
show mac address-table address 0011.2233.4455
```

**Expected output**:
```
Mac Address Table
-------------------------------------------
Vlan    Mac Address       Type        Ports
----    -----------       --------    -----
100     0011.2233.4455    DYNAMIC     Gi1/0/1
```

**Troubleshooting**:
- No entry? Device may be offline or not sending traffic
- Type "STATIC"? Manually configured (still valid)
- Multiple ports? LAG/Port-Channel or MAC flapping issue

### 6. ARP Table (supplemental)

**What it is**: IP-to-MAC address resolution cache

**Why it matters**: Confirms IP address reachability at Layer 3

**How to verify**:
```bash
show ip arp | include 192.168.1.10
```

**Expected output**:
```
Internet  192.168.1.10            0   0011.2233.4455  ARPA   Vlan100
```

---

## Common Scenarios

### Scenario 1: Corporate Workstation (High Security)

**Configuration**:
```
interface GigabitEthernet1/0/1
 switchport mode access
 switchport access vlan 100
 switchport port-security
 ip dhcp snooping limit rate 10
 ip arp inspection trust
 ip verify source port-security
 authentication port-control auto
 dot1x pae authenticator
```

**Expected Decision**:
```json
{
  "outcome": "Allowed",
  "confidence_score": 90,
  "reason_code": "L2_ALLOWED_HIGH_CONFIDENCE",
  "evidence": {
    "dhcp_bound": true,
    "authenticated": true,
    "auth_method": "dot1x",
    "ipsg_enabled": true,
    "port_security_ok": true
  }
}
```

**Operator Action**: None - traffic flows normally

---

### Scenario 2: Guest Device (Medium Security)

**Configuration**:
```
interface GigabitEthernet1/0/2
 switchport mode access
 switchport access vlan 200
 ip dhcp snooping limit rate 15
 authentication port-control auto
 mab
```

**Expected Decision**:
```json
{
  "outcome": "Allowed",
  "confidence_score": 75,
  "reason_code": "L2_ALLOWED_HIGH_CONFIDENCE",
  "evidence": {
    "dhcp_bound": true,
    "authenticated": true,
    "auth_method": "mab",
    "ipsg_enabled": false,
    "port_security_ok": false
  }
}
```

**Operator Action**: Monitor for suspicious behavior

---

### Scenario 3: Legacy Printer (Low Security)

**Configuration**:
```
interface GigabitEthernet1/0/3
 switchport mode access
 switchport access vlan 100
```

**Expected Decision**:
```json
{
  "outcome": "Blocked",
  "confidence_score": 10,
  "reason_code": "L2_BLOCKED_LOW_CONFIDENCE",
  "evidence": {
    "dhcp_bound": false,
    "authenticated": false,
    "ipsg_enabled": false,
    "port_security_ok": false
  }
}
```

**Operator Action**:
1. Enable DHCP snooping on VLAN 100
2. Consider enabling MAB for legacy device authentication
3. Or, create exception policy for known legacy devices

---

### Scenario 4: Unauthorized Device (Security Violation)

**Configuration**:
```
interface GigabitEthernet1/0/1
 switchport port-security
 switchport port-security maximum 1
 switchport port-security violation restrict
```

**Device Behavior**: Second MAC address appears on port

**Expected Decision**:
```json
{
  "outcome": "Blocked",
  "confidence_score": 0,
  "reason_code": "L2_BLOCKED_PORT_SECURITY_VIOLATION",
  "evidence": {
    "port_security_violation": true,
    "violation_count": 1
  }
}
```

**Operator Action**:
1. Investigate unauthorized device
2. If legitimate, increase `switchport port-security maximum`
3. If malicious, clear violation: `clear port-security sticky interface Gi1/0/1`

---

### Scenario 5: Different VLANs (L3 Handoff)

**Query**:
```json
{
  "source_mac": "00:11:22:33:44:55",
  "source_vlan": 100,
  "dest_mac": "aa:bb:cc:dd:ee:ff",
  "dest_vlan": 200
}
```

**Expected Decision**:
```json
{
  "outcome": "Blocked",
  "confidence_score": 0,
  "reason_code": "L2_BLOCKED_DIFFERENT_VLAN",
  "requires_l3": true,
  "reason_detail": "Source and destination in different VLANs - routing required"
}
```

**Operator Action**: Check L3 routing engine for inter-VLAN routing decision

---

### Scenario 6: PVLAN Isolation

**Configuration**:
```
vlan 100
 private-vlan primary
vlan 101
 private-vlan isolated

interface GigabitEthernet1/0/1
 switchport mode private-vlan host
 switchport private-vlan host-association 100 101

interface GigabitEthernet1/0/2
 switchport mode private-vlan host
 switchport private-vlan host-association 100 101
```

**Query**: Traffic between Gi1/0/1 and Gi1/0/2

**Expected Decision**:
```json
{
  "outcome": "Blocked",
  "confidence_score": 0,
  "reason_code": "L2_BLOCKED_PVLAN_ISOLATION",
  "reason_detail": "Private VLAN isolation prevents communication"
}
```

**Operator Action**:
- Expected behavior for isolated PVLAN
- Devices can only communicate with promiscuous port (typically router)

---

## Troubleshooting Low Confidence

### Symptom: All devices show confidence <40

**Possible Causes**:
1. DHCP snooping not enabled
2. 802.1X not configured
3. Operational data not parsed correctly

**Diagnostic Steps**:

```bash
# Step 1: Verify DHCP snooping enabled globally
show run | include dhcp snooping
# Expected: ip dhcp snooping

# Step 2: Verify DHCP snooping enabled on VLANs
show ip dhcp snooping
# Expected: DHCP snooping is enabled

# Step 3: Check DHCP bindings exist
show ip dhcp snooping binding
# Expected: Non-empty binding table

# Step 4: Verify 802.1X enabled globally
show run | include dot1x
# Expected: dot1x system-auth-control

# Step 5: Check authentication sessions
show authentication sessions
# Expected: Active sessions with "Authc Success"
```

**Solution**:
```bash
# Enable DHCP snooping
conf t
ip dhcp snooping
ip dhcp snooping vlan 100,200

# Enable 802.1X
aaa new-model
dot1x system-auth-control

# Enable on interface
interface GigabitEthernet1/0/1
 authentication port-control auto
 dot1x pae authenticator
```

---

### Symptom: Specific device has low confidence

**Diagnostic Steps**:

```bash
# Check MAC table
show mac address-table address 0011.2233.4455

# Check DHCP binding
show ip dhcp snooping binding | include 0011.2233.4455

# Check authentication
show authentication sessions interface Gi1/0/1

# Check port security
show port-security interface Gi1/0/1
```

**Common Issues**:

| Issue | Evidence | Solution |
|-------|----------|----------|
| No DHCP binding | Device using static IP | Enable DHCP or create static binding |
| Auth failed | Status: "Authc Failed" | Check RADIUS server, verify credentials |
| Port security violation | Violation Count > 0 | Clear violation, verify single MAC |
| Stale MAC entry | Age > 5 minutes | Wait for refresh or clear MAC table |

---

### Symptom: Confidence drops suddenly

**Possible Causes**:
1. DHCP lease expired
2. Authentication session timed out
3. Device moved to different port

**Diagnostic Steps**:

```bash
# Check recent DHCP changes
show ip dhcp snooping binding | include <MAC>

# Check authentication session status
show authentication sessions interface <interface>

# Check MAC table for port changes
show mac address-table dynamic address <MAC>
```

**Solution**:
- DHCP expired: Device will renew automatically
- Auth timeout: Re-authenticate (automatic if 802.1X configured)
- Port change: Evidence will update on next poll

---

## Reason Codes Reference

The L2 Decision Engine returns one of 13 reason codes with each decision.

### Allowed Reason Codes

| Code | Description | Confidence Range | Action |
|------|-------------|------------------|--------|
| `L2_ALLOWED_HIGH_CONFIDENCE` | Both MACs authenticated, DHCP-bound, same VLAN | 70-100 | Allow traffic |
| `L2_ALLOWED_MEDIUM_CONFIDENCE` | Some evidence present but not all controls | 70-89 | Allow with monitoring |

### Conditional Reason Codes

| Code | Description | Confidence Range | Action |
|------|-------------|------------------|--------|
| `L2_CONDITIONAL_PARTIAL_AUTH` | One MAC authenticated, other not | 40-69 | Allow with warning |
| `L2_CONDITIONAL_DHCP_ONLY` | DHCP binding present but no auth | 40-69 | Allow with monitoring |
| `L2_CONDITIONAL_LOW_CONFIDENCE` | Weak evidence sources | 40-49 | Review security controls |

### Blocked Reason Codes

| Code | Description | Confidence | Action |
|------|-------------|------------|--------|
| `L2_BLOCKED_SRC_MAC_NOT_FOUND` | Source MAC not in forwarding table | 0 | Device offline or not sending traffic |
| `L2_BLOCKED_DEST_MAC_NOT_FOUND` | Destination MAC not in forwarding table | 0 | Destination offline or not sending traffic |
| `L2_BLOCKED_DIFFERENT_VLAN` | Source and dest in different VLANs | 0 | Check L3 routing (handoff to L3 engine) |
| `L2_BLOCKED_PORT_SECURITY_VIOLATION` | Port security violation detected | 0 | Investigate unauthorized device |
| `L2_BLOCKED_AUTH_FAILED` | 802.1X authentication failed | 0 | Check credentials, RADIUS server |
| `L2_BLOCKED_PVLAN_ISOLATION` | Private VLAN isolation enforced | 0 | Expected behavior for isolated PVLAN |
| `L2_BLOCKED_LOW_CONFIDENCE` | Confidence below threshold | 0-39 | Enable security controls |
| `L2_BLOCKED_NO_EVIDENCE` | No evidence sources available | 0 | Enable DHCP snooping, 802.1X, etc. |

### Reason Code Usage Example

```go
result := engine.DetermineL2Reachability(query)

switch result.ReasonCode {
case "L2_ALLOWED_HIGH_CONFIDENCE":
    // Normal operation
    allowTraffic()

case "L2_CONDITIONAL_PARTIAL_AUTH":
    // Allow but log for review
    allowTraffic()
    logWarning("Partial authentication for %s", query.SourceMAC)

case "L2_BLOCKED_PORT_SECURITY_VIOLATION":
    // Block and alert
    blockTraffic()
    alertSecurityTeam("Port security violation for %s", query.SourceMAC)

case "L2_BLOCKED_DIFFERENT_VLAN":
    // Handoff to L3 routing engine
    if result.RequiresL3 {
        checkL3Routing(query)
    }
}
```

---

## Monitoring and Alerting

### Key Metrics to Monitor

1. **Decision Distribution**
   - % Allowed decisions
   - % Conditional decisions
   - % Blocked decisions
   - Target: >80% Allowed, <5% Blocked

2. **Confidence Score Distribution**
   - Average confidence score
   - % decisions with confidence >70
   - % decisions with confidence <40
   - Target: Average >75

3. **Evidence Source Coverage**
   - % MACs with DHCP binding
   - % MACs with 802.1X session
   - % MACs with IPSG enabled
   - Target: >90% for production networks

4. **Decision Latency**
   - Average decision time (milliseconds)
   - 95th percentile decision time
   - Target: <50ms average, <100ms p95

### Prometheus Metrics Example

```go
// Define metrics
var (
    decisionTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "l2_decisions_total",
            Help: "Total L2 reachability decisions",
        },
        []string{"outcome", "reason_code"},
    )

    confidenceScore = prometheus.NewHistogram(
        prometheus.HistogramOpts{
            Name:    "l2_confidence_score",
            Help:    "Distribution of L2 confidence scores",
            Buckets: []float64{0, 20, 40, 60, 70, 80, 90, 100},
        },
    )

    decisionDuration = prometheus.NewHistogram(
        prometheus.HistogramOpts{
            Name:    "l2_decision_duration_ms",
            Help:    "L2 decision latency in milliseconds",
            Buckets: prometheus.ExponentialBuckets(1, 2, 10),
        },
    )
)

// Instrument decision engine
func (e *L2DecisionEngine) DetermineL2Reachability(query L2ReachabilityQuery) L2ReachabilityResult {
    start := time.Now()
    result := e.determineL2Reachability(query)
    duration := time.Since(start).Milliseconds()

    // Record metrics
    decisionTotal.WithLabelValues(result.Outcome, result.ReasonCode).Inc()
    confidenceScore.Observe(float64(result.ConfidenceScore))
    decisionDuration.Observe(float64(duration))

    return result
}
```

### Alerting Rules

```yaml
# Alert on high blocked decision rate
- alert: HighL2BlockedRate
  expr: |
    rate(l2_decisions_total{outcome="Blocked"}[5m])
    /
    rate(l2_decisions_total[5m]) > 0.1
  for: 10m
  labels:
    severity: warning
  annotations:
    summary: "High L2 blocked decision rate"
    description: "{{ $value | humanizePercentage }} of L2 decisions are Blocked"

# Alert on low average confidence
- alert: LowL2Confidence
  expr: |
    histogram_quantile(0.5, l2_confidence_score) < 50
  for: 15m
  labels:
    severity: warning
  annotations:
    summary: "Low L2 confidence scores"
    description: "Median confidence score is {{ $value }}"

# Alert on high decision latency
- alert: HighL2DecisionLatency
  expr: |
    histogram_quantile(0.95, l2_decision_duration_ms) > 100
  for: 5m
  labels:
    severity: warning
  annotations:
    summary: "High L2 decision latency"
    description: "95th percentile latency is {{ $value }}ms"

# Alert on missing evidence sources
- alert: MissingDHCPBindings
  expr: |
    l2_evidence_sources{type="dhcp_snooping"}
    /
    l2_evidence_sources{type="mac_table"} < 0.8
  for: 30m
  labels:
    severity: warning
  annotations:
    summary: "Low DHCP binding coverage"
    description: "Only {{ $value | humanizePercentage }} of MACs have DHCP bindings"
```

---

## Best Practices

### 1. Enable All Security Controls

For maximum confidence, enable all L2 security controls:

```cisco
! Global configuration
ip dhcp snooping
ip dhcp snooping vlan 10,20,30,40,50
ip arp inspection vlan 10,20,30,40,50
ip arp inspection validate src-mac dst-mac ip

aaa new-model
dot1x system-auth-control

! Interface configuration (access ports)
interface range GigabitEthernet1/0/1-48
 switchport mode access
 switchport port-security
 switchport port-security maximum 2
 switchport port-security violation restrict
 ip dhcp snooping limit rate 10
 ip verify source port-security
 authentication port-control auto
 dot1x pae authenticator
 mab
 spanning-tree portfast
 spanning-tree bpduguard enable
```

### 2. Trust Uplink Ports

Always trust DHCP snooping and DAI on uplink/trunk ports:

```cisco
interface GigabitEthernet1/0/49
 description Uplink to Distribution
 switchport mode trunk
 ip dhcp snooping trust
 ip arp inspection trust
```

### 3. Regular Evidence Polling

Poll operational data frequently to keep evidence fresh:

```bash
# Recommended polling intervals
show ip dhcp snooping binding      # Every 5 minutes
show authentication sessions       # Every 5 minutes
show mac address-table dynamic     # Every 2 minutes
show port-security                 # Every 10 minutes
```

### 4. Monitor Confidence Trends

Track confidence score trends over time to detect security drift:

```sql
-- Query MongoDB for confidence trends
db.l2_fdb_entries.aggregate([
  {
    $match: {
      timestamp: { $gte: ISODate("2025-01-01T00:00:00Z") }
    }
  },
  {
    $group: {
      _id: { $dateToString: { format: "%Y-%m-%d %H:00", date: "$timestamp" } },
      avg_confidence: { $avg: "$confidence_score" },
      count: { $sum: 1 }
    }
  },
  {
    $sort: { _id: 1 }
  }
])
```

### 5. Use Strict Mode in Production

Enable strict mode to require both authentication and DHCP binding:

```go
engine := decision.NewL2DecisionEngine()
engine.SetStrictMode(true)  // Default: true
```

### 6. Document Exceptions

Maintain a list of devices that legitimately have low confidence:

```yaml
# low_confidence_exceptions.yaml
exceptions:
  - mac: "aa:bb:cc:dd:ee:01"
    reason: "Legacy printer - no 802.1X support"
    approved_by: "security-team"
    expiration: "2025-12-31"

  - mac: "aa:bb:cc:dd:ee:02"
    reason: "Building management system - static IP"
    approved_by: "facilities-team"
    expiration: "2026-06-30"
```

---

## FAQ

### Q: Why is my device showing "Conditional" instead of "Allowed"?

**A**: The device likely has DHCP binding but no 802.1X authentication, resulting in a confidence score between 40-69. To reach "Allowed" status, enable 802.1X or MAB on the interface.

---

### Q: Can I adjust the confidence thresholds?

**A**: Yes, thresholds are configurable:

```go
engine.SetAllowedThreshold(80)      // Require 80+ for Allowed (default: 70)
engine.SetConditionalThreshold(50)  // Require 50+ for Conditional (default: 40)
```

---

### Q: What's the difference between dot1x and MAB authentication?

**A**:
- **dot1x**: User/device authenticates with username/password (+35 confidence)
- **MAB**: Switch authenticates device based on MAC address (+25 confidence)

MAB is useful for devices that don't support 802.1X (printers, cameras, etc.) but provides lower confidence.

---

### Q: How long are decisions cached?

**A**: By default, decisions are cached for **5 minutes** (TTL). After 5 minutes, the decision is recomputed with fresh evidence.

```go
// Adjust TTL
engine.SetDecisionTTL(10 * time.Minute)  // Cache for 10 minutes
```

---

### Q: What happens when evidence conflicts?

**A**: The engine uses a weighted scoring system. For example:
- DHCP binding (+40) + Auth failed (0) = 50 confidence = Conditional
- The engine prioritizes stronger evidence sources (DHCP, 802.1X) over weaker ones (MAC table age)

---

### Q: Can I override a "Blocked" decision?

**A**: Yes, using exception policies:

```go
// Add exception for specific MAC pair
engine.AddException(decision.L2Exception{
    SourceMAC:  "00:11:22:33:44:55",
    DestMAC:    "aa:bb:cc:dd:ee:ff",
    Outcome:    "Allowed",
    Reason:     "Management override - legacy device exception",
    Expiration: time.Now().Add(90 * 24 * time.Hour), // 90 days
})
```

---

### Q: How do I troubleshoot "L2_BLOCKED_DIFFERENT_VLAN"?

**A**: This is expected when source and destination are in different VLANs. Check if `RequiresL3 = true` in the result, then handoff to the L3 routing engine to determine if inter-VLAN routing is permitted.

---

### Q: What's the impact of stale MAC entries?

**A**: MAC entries are classified by age:
- **Fresh** (<5 minutes): Full confidence contribution
- **Stale** (5-30 minutes): Reduced confidence (-10 points)
- **Old** (>30 minutes): Minimal confidence (-20 points)

Stale entries may cause confidence to drop below the Allowed threshold.

---

### Q: How do I integrate L2 decisions with my SIEM?

**A**: Export decisions to syslog or forward to SIEM via API:

```go
// Syslog integration
result := engine.DetermineL2Reachability(query)
syslog.Send(syslog.INFO, fmt.Sprintf(
    "L2Decision: src=%s dst=%s vlan=%d outcome=%s confidence=%d reason=%s",
    query.SourceMAC, query.DestMAC, query.VLANID,
    result.Outcome, result.ConfidenceScore, result.ReasonCode,
))
```

---

## Summary

The L2 Decision Engine provides transparent, evidence-based reachability decisions. Key takeaways:

1. **Confidence scores** (0-100) indicate certainty that traffic is authorized
2. **Three outcomes**: Allowed (≥70), Conditional (40-69), Blocked (<40)
3. **Multi-source evidence**: DHCP, 802.1X, port security, IPSG, MAC table
4. **13 reason codes** provide machine-readable decision reasoning
5. **Enable all security controls** for maximum confidence
6. **Monitor trends** to detect security drift

For integration details, see the [L2 Integration Guide](../integration/l2_integration_guide.md).

For API reference, see the [L2 Decision Engine API](../api/l2_decision_engine_api.md).

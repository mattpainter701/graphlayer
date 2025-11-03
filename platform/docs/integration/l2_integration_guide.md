# L2 Integration Guide

**Version**: 1.0
**Last Updated**: 2025-01-01
**Target Audience**: Platform Engineers, Integration Developers

---

## Table of Contents

1. [Overview](#overview)
2. [Integration Architecture](#integration-architecture)
3. [MongoDB Schema](#mongodb-schema)
4. [Integration Patterns](#integration-patterns)
5. [Evidence Loading Pipeline](#evidence-loading-pipeline)
6. [L2-to-L3 Handoff](#l2-to-l3-handoff)
7. [Multi-Vendor Integration](#multi-vendor-integration)
8. [Performance Optimization](#performance-optimization)
9. [Troubleshooting](#troubleshooting)

---

## Overview

This guide explains how to integrate the **L2 Decision Engine** into your existing network topology platform. The L2 Decision Engine determines whether traffic can flow between two MAC addresses at Layer 2, using multi-source evidence (DHCP, 802.1X, port security, etc.) to calculate confidence scores and make Allowed/Conditional/Blocked decisions.

### Key Integration Points

```
┌─────────────────────────────────────────────────────────────┐
│                    Network Topology Platform                │
└─────────────────────────────────────────────────────────────┘
                              │
                 ┌────────────┼────────────┐
                 │            │            │
         ┌───────▼───────┐    │    ┌───────▼───────┐
         │  Config Parse  │    │    │  Operational  │
         │  & Evidence    │    │    │  Data Parse   │
         │  Hashing       │    │    │  (DHCP, 802.1X)│
         └───────┬───────┘    │    └───────┬───────┘
                 │            │            │
                 └────────────┼────────────┘
                              │
                    ┌─────────▼─────────┐
                    │   L2 Decision     │
                    │   Engine          │
                    │   (This Module)   │
                    └─────────┬─────────┘
                              │
                 ┌────────────┼────────────┐
                 │            │            │
         ┌───────▼───────┐    │    ┌───────▼───────┐
         │  L2 Reachability │    │    │  L3 Routing   │
         │  Decisions      │    │    │  Engine       │
         └─────────────────┘    │    └───────────────┘
                                │
                        ┌───────▼───────┐
                        │  Graph/Map    │
                        │  Visualization│
                        └───────────────┘
```

### Prerequisites

- Go 1.21+
- MongoDB 4.4+ (for data persistence)
- Vendor-specific parsers configured
- Network device credentials configured

---

## Integration Architecture

### Component Interaction

The L2 Decision Engine integrates with three primary components:

1. **Configuration Parsers** - Extract L2 security controls from device configs
2. **Operational Data Parsers** - Extract runtime state (DHCP, 802.1X, MAC tables)
3. **L3 Routing Engine** - Handoff when L2 decision requires routing

### Data Flow

```go
// Simplified integration example
package main

import (
    "github.com/mattpainter701/graphlayer/platform/platform/orchestration/decision"
    "github.com/mattpainter701/graphlayer/platform/platform/vendors/cisco/ios"
    "github.com/mattpainter701/graphlayer/platform/platform/models"
)

func main() {
    // 1. Parse device configuration
    parser := ios.NewCiscoIOSParser()
    device, err := parser.ParseConfig("configs/sw-core-01.txt")
    if err != nil {
        panic(err)
    }

    // 2. Parse operational data
    dhcpBindings := parser.ParseDHCPSnooping("show_ip_dhcp_snooping_binding.txt")
    authSessions := parser.ParseAuthSessions("show_authentication_sessions.txt")
    macTable := parser.ParseMACTable("show_mac_address_table.txt")

    // 3. Initialize L2 Decision Engine
    engine := decision.NewL2DecisionEngine()
    engine.LoadMACTable(macTable)
    engine.LoadDHCPBindings(dhcpBindings)
    engine.LoadAuthSessions(authSessions)

    // 4. Make L2 reachability decision
    query := decision.L2ReachabilityQuery{
        SourceMAC: "00:11:22:33:44:55",
        DestMAC:   "aa:bb:cc:dd:ee:ff",
        VLANID:    100,
    }
    result := engine.DetermineL2Reachability(query)

    // 5. Act on decision
    switch result.Outcome {
    case "Allowed":
        // Build L2 adjacency in graph
    case "Conditional":
        // Mark as conditional path with warning
    case "Blocked":
        if result.RequiresL3 {
            // Handoff to L3 routing engine
        } else {
            // No path exists
        }
    }
}
```

---

## MongoDB Schema

The L2 Decision Engine persists data to MongoDB for query performance, historical tracking, and drift detection.

### Collections

#### 1. `l2_switches`

Stores L2 switch devices with their interfaces and security controls.

```json
{
  "_id": "ObjectId()",
  "device_id": "sw-core-01",
  "hostname": "SW-CORE-01",
  "vendor": "cisco",
  "platform": "ios",
  "config_hash": "sha256:abc123...",
  "last_updated": "ISODate('2025-01-01T00:00:00Z')",
  "interfaces": [
    {
      "name": "GigabitEthernet1/0/1",
      "type": "access",
      "vlan": 100,
      "security_score": 95,
      "security_controls": {
        "dhcp_snooping_enabled": true,
        "dhcp_snooping_rate_limit": 10,
        "dai_enabled": true,
        "ipsg_enabled": true,
        "ipsg_mode": "port-security",
        "port_security_enabled": true,
        "port_security_max": 2,
        "port_security_violation": "restrict",
        "auth_802_1x_enabled": true,
        "auth_mab_enabled": true,
        "spanning_tree_portfast": true,
        "spanning_tree_bpduguard": true
      },
      "evidence_hash": "sha256:def456..."
    }
  ],
  "vlans": [
    {
      "id": 100,
      "name": "Engineering",
      "dhcp_snooping_enabled": true,
      "dai_enabled": true
    }
  ],
  "global_features": {
    "dhcp_snooping_enabled": true,
    "dai_enabled": true,
    "dot1x_enabled": true
  }
}
```

**Indexes**:
```javascript
db.l2_switches.createIndex({ "device_id": 1 }, { unique: true })
db.l2_switches.createIndex({ "config_hash": 1 })
db.l2_switches.createIndex({ "last_updated": -1 })
db.l2_switches.createIndex({ "interfaces.name": 1 })
```

#### 2. `l2_fdb_entries`

Stores enhanced Forwarding Database (FDB) entries with multi-source evidence.

```json
{
  "_id": "ObjectId()",
  "device_id": "sw-core-01",
  "mac_address": "00:11:22:33:44:55",
  "vlan_id": 100,
  "interface": "GigabitEthernet1/0/1",
  "ip_address": "192.168.1.10",
  "confidence_level": "high",
  "confidence_score": 90,
  "evidence_sources": [
    {
      "type": "dhcp_snooping",
      "confidence_contribution": 40,
      "last_seen": "ISODate('2025-01-01T12:00:00Z')",
      "data": {
        "lease_time": 86400,
        "binding_age": 3600
      }
    },
    {
      "type": "802.1x",
      "confidence_contribution": 35,
      "last_seen": "ISODate('2025-01-01T12:00:00Z')",
      "data": {
        "method": "dot1x",
        "username": "john.doe",
        "status": "Authc Success"
      }
    },
    {
      "type": "ipsg",
      "confidence_contribution": 15,
      "last_seen": "ISODate('2025-01-01T12:00:00Z')",
      "data": {
        "filter_mode": "ip-mac",
        "binding_active": true
      }
    }
  ],
  "age_minutes": 60,
  "freshness": "fresh",
  "last_updated": "ISODate('2025-01-01T12:00:00Z')"
}
```

**Indexes**:
```javascript
db.l2_fdb_entries.createIndex({ "device_id": 1, "mac_address": 1, "vlan_id": 1 }, { unique: true })
db.l2_fdb_entries.createIndex({ "confidence_level": 1 })
db.l2_fdb_entries.createIndex({ "freshness": 1 })
db.l2_fdb_entries.createIndex({ "last_updated": -1 })
db.l2_fdb_entries.createIndex({ "evidence_sources.type": 1 })
```

#### 3. `l2_decisions`

Stores L2 reachability decisions with reason codes and evidence trails.

```json
{
  "_id": "ObjectId()",
  "query_id": "uuid-v4",
  "source_mac": "00:11:22:33:44:55",
  "dest_mac": "aa:bb:cc:dd:ee:ff",
  "vlan_id": 100,
  "outcome": "Allowed",
  "confidence_score": 90,
  "reason_code": "L2_ALLOWED_HIGH_CONFIDENCE",
  "reason_detail": "Both MACs authenticated, DHCP-bound, same VLAN",
  "requires_l3": false,
  "evidence": {
    "source_evidence": {
      "mac_entry_found": true,
      "vlan_match": true,
      "dhcp_bound": true,
      "authenticated": true,
      "confidence": 90
    },
    "dest_evidence": {
      "mac_entry_found": true,
      "vlan_match": true,
      "dhcp_bound": true,
      "authenticated": true,
      "confidence": 85
    },
    "pvlan_isolation": false,
    "port_security_violations": []
  },
  "decision_metadata": {
    "engine_version": "1.0.0",
    "strict_mode": true,
    "allowed_threshold": 70,
    "conditional_threshold": 40,
    "decision_time_ms": 12
  },
  "timestamp": "ISODate('2025-01-01T12:00:00Z')",
  "ttl_expires": "ISODate('2025-01-01T12:05:00Z')"
}
```

**Indexes**:
```javascript
db.l2_decisions.createIndex({ "query_id": 1 }, { unique: true })
db.l2_decisions.createIndex({ "source_mac": 1, "dest_mac": 1, "vlan_id": 1 })
db.l2_decisions.createIndex({ "outcome": 1 })
db.l2_decisions.createIndex({ "reason_code": 1 })
db.l2_decisions.createIndex({ "timestamp": -1 })
db.l2_decisions.createIndex({ "ttl_expires": 1 }, { expireAfterSeconds: 0 })
```

### Schema Initialization

```go
package storage

import (
    "context"
    "go.mongodb.org/mongo-driver/mongo"
    "go.mongodb.org/mongo-driver/mongo/options"
)

func InitializeL2Schema(client *mongo.Client, dbName string) error {
    db := client.Database(dbName)

    // Create l2_switches indexes
    _, err := db.Collection("l2_switches").Indexes().CreateMany(
        context.Background(),
        []mongo.IndexModel{
            {Keys: bson.D{{"device_id", 1}}, Options: options.Index().SetUnique(true)},
            {Keys: bson.D{{"config_hash", 1}}},
            {Keys: bson.D{{"last_updated", -1}}},
            {Keys: bson.D{{"interfaces.name", 1}}},
        },
    )
    if err != nil {
        return err
    }

    // Create l2_fdb_entries indexes
    _, err = db.Collection("l2_fdb_entries").Indexes().CreateMany(
        context.Background(),
        []mongo.IndexModel{
            {Keys: bson.D{{"device_id", 1}, {"mac_address", 1}, {"vlan_id", 1}},
             Options: options.Index().SetUnique(true)},
            {Keys: bson.D{{"confidence_level", 1}}},
            {Keys: bson.D{{"freshness", 1}}},
            {Keys: bson.D{{"last_updated", -1}}},
        },
    )
    if err != nil {
        return err
    }

    // Create l2_decisions indexes with TTL
    _, err = db.Collection("l2_decisions").Indexes().CreateMany(
        context.Background(),
        []mongo.IndexModel{
            {Keys: bson.D{{"query_id", 1}}, Options: options.Index().SetUnique(true)},
            {Keys: bson.D{{"source_mac", 1}, {"dest_mac", 1}, {"vlan_id", 1}}},
            {Keys: bson.D{{"outcome", 1}}},
            {Keys: bson.D{{"timestamp", -1}}},
            {Keys: bson.D{{"ttl_expires", 1}},
             Options: options.Index().SetExpireAfterSeconds(0)},
        },
    )
    return err
}
```

---

## Integration Patterns

### Pattern 1: Real-Time Decision Making

Use this pattern for live threat simulation or network mapping where decisions are made on-demand.

```go
package integration

import (
    "github.com/mattpainter701/graphlayer/platform/platform/orchestration/decision"
    "github.com/mattpainter701/graphlayer/platform/platform/storage"
)

type L2DecisionService struct {
    engine  *decision.L2DecisionEngine
    storage *storage.L2Storage
}

func NewL2DecisionService(mongoURI, dbName string) (*L2DecisionService, error) {
    store, err := storage.NewL2Storage(mongoURI, dbName)
    if err != nil {
        return nil, err
    }

    engine := decision.NewL2DecisionEngine()

    return &L2DecisionService{
        engine:  engine,
        storage: store,
    }, nil
}

func (s *L2DecisionService) MakeDecision(sourceMAC, destMAC string, vlanID int) (*decision.L2ReachabilityResult, error) {
    // 1. Load evidence from storage
    macTable, err := s.storage.GetMACTableEntries(vlanID)
    if err != nil {
        return nil, err
    }

    dhcpBindings, err := s.storage.GetDHCPBindings(vlanID)
    if err != nil {
        return nil, err
    }

    authSessions, err := s.storage.GetAuthSessions(vlanID)
    if err != nil {
        return nil, err
    }

    // 2. Load into engine
    s.engine.LoadMACTable(macTable)
    s.engine.LoadDHCPBindings(dhcpBindings)
    s.engine.LoadAuthSessions(authSessions)

    // 3. Make decision
    query := decision.L2ReachabilityQuery{
        SourceMAC: sourceMAC,
        DestMAC:   destMAC,
        VLANID:    vlanID,
    }
    result := s.engine.DetermineL2Reachability(query)

    // 4. Persist decision
    err = s.storage.SaveDecision(result)
    if err != nil {
        return nil, err
    }

    return &result, nil
}
```

### Pattern 2: Batch Pre-Computation

Use this pattern to pre-compute all L2 decisions for faster graph queries.

```go
func (s *L2DecisionService) PreComputeAllDecisions(vlanID int) error {
    // 1. Load all evidence
    fdbEntries, err := s.storage.GetFDBEntries(vlanID)
    if err != nil {
        return err
    }

    // 2. Generate all MAC pairs
    var macPairs []struct{ Source, Dest string }
    for i, src := range fdbEntries {
        for j, dst := range fdbEntries {
            if i != j {
                macPairs = append(macPairs, struct{ Source, Dest string }{
                    Source: src.MACAddress,
                    Dest:   dst.MACAddress,
                })
            }
        }
    }

    // 3. Compute decisions in parallel
    decisions := make(chan decision.L2ReachabilityResult, len(macPairs))
    errors := make(chan error, len(macPairs))

    for _, pair := range macPairs {
        go func(src, dst string) {
            result, err := s.MakeDecision(src, dst, vlanID)
            if err != nil {
                errors <- err
                return
            }
            decisions <- *result
        }(pair.Source, pair.Dest)
    }

    // 4. Collect results
    for i := 0; i < len(macPairs); i++ {
        select {
        case result := <-decisions:
            // Store in cache
            s.storage.SaveDecision(result)
        case err := <-errors:
            return err
        }
    }

    return nil
}
```

### Pattern 3: Incremental Updates

Use this pattern to update decisions only when evidence changes.

```go
func (s *L2DecisionService) UpdateDecisionsForMAC(macAddress string, vlanID int) error {
    // 1. Find all decisions involving this MAC
    affectedDecisions, err := s.storage.FindDecisionsByMAC(macAddress, vlanID)
    if err != nil {
        return err
    }

    // 2. Recompute each affected decision
    for _, oldDecision := range affectedDecisions {
        newResult, err := s.MakeDecision(
            oldDecision.SourceMAC,
            oldDecision.DestMAC,
            vlanID,
        )
        if err != nil {
            return err
        }

        // 3. Check if decision changed
        if newResult.Outcome != oldDecision.Outcome {
            // Trigger alert or notification
            s.notifyDecisionChange(oldDecision, *newResult)
        }
    }

    return nil
}
```

---

## Evidence Loading Pipeline

### Step 1: Parse Device Configuration

```go
// Parse config and compute evidence hashes
parser := ios.NewCiscoIOSParser()
device, err := parser.ParseConfig("configs/sw-core-01.txt")
if err != nil {
    return err
}

// Evidence hashes are automatically computed for each interface's
// security control configuration
for _, iface := range device.Interfaces {
    fmt.Printf("Interface %s evidence hash: %s\n",
        iface.Name, iface.SecurityControls.EvidenceHash)
}
```

### Step 2: Parse Operational Data

```go
// Parse DHCP snooping bindings
dhcpData, err := ioutil.ReadFile("show_ip_dhcp_snooping_binding.txt")
if err != nil {
    return err
}
dhcpBindings := parser.ParseDHCPSnooping(string(dhcpData))

// Parse 802.1X authentication sessions
authData, err := ioutil.ReadFile("show_authentication_sessions.txt")
if err != nil {
    return err
}
authSessions := parser.ParseAuthSessions(string(authData))

// Parse MAC address table
macData, err := ioutil.ReadFile("show_mac_address_table.txt")
if err != nil {
    return err
}
macTable := parser.ParseMACTable(string(macData))

// Parse ARP table (optional)
arpData, err := ioutil.ReadFile("show_ip_arp.txt")
if err != nil {
    return err
}
arpEntries := parser.ParseARPTable(string(arpData))
```

### Step 3: Build Enhanced FDB Entries

```go
// Combine all evidence sources into enhanced FDB entries
fdbEntries := parser.BuildEnhancedFDBEntries(
    macTable,
    dhcpBindings,
    authSessions,
    ipsgBindings,
    arpEntries,
)

// Store in MongoDB
for _, entry := range fdbEntries {
    err := storage.SaveFDBEntry(entry)
    if err != nil {
        return err
    }
}
```

### Step 4: Load Evidence into Engine

```go
engine := decision.NewL2DecisionEngine()
engine.LoadMACTable(macTable)
engine.LoadDHCPBindings(dhcpBindings)
engine.LoadAuthSessions(authSessions)
engine.LoadIPSourceGuard(ipsgBindings)
engine.LoadARPEntries(arpEntries)
```

### Complete Pipeline Example

```go
func LoadEvidencePipeline(deviceID string) error {
    // 1. Initialize parser
    parser := ios.NewCiscoIOSParser()

    // 2. Parse configuration
    configPath := fmt.Sprintf("configs/%s.txt", deviceID)
    device, err := parser.ParseConfig(configPath)
    if err != nil {
        return fmt.Errorf("config parse failed: %w", err)
    }

    // 3. Parse operational data
    dhcpBindings := parser.ParseDHCPSnooping(
        readFile(fmt.Sprintf("operational/%s_dhcp.txt", deviceID)))
    authSessions := parser.ParseAuthSessions(
        readFile(fmt.Sprintf("operational/%s_auth.txt", deviceID)))
    macTable := parser.ParseMACTable(
        readFile(fmt.Sprintf("operational/%s_mac.txt", deviceID)))

    // 4. Build enhanced FDB
    fdbEntries := parser.BuildEnhancedFDBEntries(
        macTable, dhcpBindings, authSessions, nil, nil)

    // 5. Store in MongoDB
    storage := storage.NewL2Storage(mongoURI, dbName)
    err = storage.SaveDevice(device)
    if err != nil {
        return fmt.Errorf("device save failed: %w", err)
    }

    for _, entry := range fdbEntries {
        err := storage.SaveFDBEntry(entry)
        if err != nil {
            return fmt.Errorf("fdb save failed: %w", err)
        }
    }

    return nil
}
```

---

## L2-to-L3 Handoff

When an L2 decision determines that routing is required, the result will have `RequiresL3 = true`. Integrate with your L3 routing engine as follows:

### Handoff Detection

```go
result := engine.DetermineL2Reachability(query)

if result.RequiresL3 {
    // Traffic requires routing - handoff to L3 engine
    l3Result := l3Engine.DetermineL3Reachability(
        sourceIP,
        destIP,
        protocol,
        port,
    )

    // Combine L2 and L3 results
    finalDecision := combineDecisions(result, l3Result)
}
```

### Common L2-to-L3 Handoff Scenarios

| Scenario | L2 Outcome | RequiresL3 | Reason Code | Next Step |
|----------|------------|------------|-------------|-----------|
| Different VLANs | Blocked | true | `L2_BLOCKED_DIFFERENT_VLAN` | Check for inter-VLAN routing |
| Same VLAN, both authenticated | Allowed | false | `L2_ALLOWED_HIGH_CONFIDENCE` | L2 adjacency confirmed |
| No MAC entry for dest | Blocked | false | `L2_BLOCKED_DEST_MAC_NOT_FOUND` | No path exists |
| PVLAN isolation | Blocked | false | `L2_BLOCKED_PVLAN_ISOLATION` | No path exists |

### Integration Code Example

```go
type UnifiedReachabilityEngine struct {
    l2Engine *decision.L2DecisionEngine
    l3Engine *routing.L3RoutingEngine
}

func (e *UnifiedReachabilityEngine) DetermineReachability(
    srcMAC, dstMAC string,
    srcIP, dstIP string,
    vlanID int,
    protocol string,
    port int,
) (*UnifiedReachabilityResult, error) {
    // 1. Always try L2 first
    l2Query := decision.L2ReachabilityQuery{
        SourceMAC: srcMAC,
        DestMAC:   dstMAC,
        VLANID:    vlanID,
    }
    l2Result := e.l2Engine.DetermineL2Reachability(l2Query)

    result := &UnifiedReachabilityResult{
        L2Result: l2Result,
    }

    // 2. Check if L3 routing required
    if l2Result.RequiresL3 {
        l3Query := routing.L3RoutingQuery{
            SourceIP:  srcIP,
            DestIP:    dstIP,
            Protocol:  protocol,
            Port:      port,
        }
        l3Result := e.l3Engine.DetermineRouting(l3Query)
        result.L3Result = &l3Result

        // 3. Combine decisions
        if l2Result.Outcome == "Blocked" && l3Result.Outcome == "Allowed" {
            result.FinalOutcome = "Allowed (via L3)"
        } else if l3Result.Outcome == "Blocked" {
            result.FinalOutcome = "Blocked (L3 denied)"
        }
    } else {
        // L2 decision is final
        result.FinalOutcome = l2Result.Outcome
    }

    return result, nil
}
```

---

## Multi-Vendor Integration

The L2 Decision Engine is vendor-agnostic. Integrate multiple vendors by using the appropriate parsers.

### Cisco IOS Integration

```go
import "github.com/mattpainter701/graphlayer/platform/platform/vendors/cisco/ios"

parser := ios.NewCiscoIOSParser()
device, _ := parser.ParseConfig("cisco_switch.txt")
dhcpBindings := parser.ParseDHCPSnooping(dhcpOutput)
```

### Cisco NX-OS Integration

```go
import "github.com/mattpainter701/graphlayer/platform/platform/vendors/cisco/nxos"

parser := nxos.NewCiscoNXOSParser()
device, _ := parser.ParseConfig("nexus_switch.txt")
dhcpBindings := parser.ParseDHCPSnooping(dhcpOutput)
```

### Arista EOS Integration

```go
import "github.com/mattpainter701/graphlayer/platform/platform/vendors/arista/eos"

parser := eos.NewAristaEOSParser()
device, _ := parser.ParseConfig("arista_switch.txt")
dhcpBindings := parser.ParseDHCPSnooping(dhcpOutput)
```

### Multi-Vendor Pipeline

```go
func LoadMultiVendorEvidence(devices []Device) error {
    for _, device := range devices {
        var parser VendorParser

        switch device.Vendor {
        case "cisco-ios":
            parser = ios.NewCiscoIOSParser()
        case "cisco-nxos":
            parser = nxos.NewCiscoNXOSParser()
        case "arista-eos":
            parser = eos.NewAristaEOSParser()
        default:
            return fmt.Errorf("unsupported vendor: %s", device.Vendor)
        }

        // Parse configuration
        deviceModel, err := parser.ParseConfig(device.ConfigPath)
        if err != nil {
            return err
        }

        // Parse operational data
        dhcpBindings := parser.ParseDHCPSnooping(device.DHCPOutput)
        authSessions := parser.ParseAuthSessions(device.AuthOutput)
        macTable := parser.ParseMACTable(device.MACOutput)

        // Build enhanced FDB
        fdbEntries := parser.BuildEnhancedFDBEntries(
            macTable, dhcpBindings, authSessions, nil, nil)

        // Store in unified schema
        storage.SaveDevice(deviceModel)
        storage.SaveFDBEntries(fdbEntries)
    }
    return nil
}
```

---

## Performance Optimization

### Caching Strategy

```go
type CachedL2DecisionEngine struct {
    engine *decision.L2DecisionEngine
    cache  *cache.LRUCache
}

func (e *CachedL2DecisionEngine) DetermineL2Reachability(
    query decision.L2ReachabilityQuery,
) decision.L2ReachabilityResult {
    // Check cache first
    cacheKey := fmt.Sprintf("%s-%s-%d",
        query.SourceMAC, query.DestMAC, query.VLANID)

    if cached, found := e.cache.Get(cacheKey); found {
        return cached.(decision.L2ReachabilityResult)
    }

    // Compute decision
    result := e.engine.DetermineL2Reachability(query)

    // Cache with 5-minute TTL
    e.cache.SetWithTTL(cacheKey, result, 5*time.Minute)

    return result
}
```

### Batch Processing

```go
func BatchLoadEvidence(devices []string) error {
    var wg sync.WaitGroup
    errors := make(chan error, len(devices))

    for _, deviceID := range devices {
        wg.Add(1)
        go func(id string) {
            defer wg.Done()
            err := LoadEvidencePipeline(id)
            if err != nil {
                errors <- err
            }
        }(deviceID)
    }

    wg.Wait()
    close(errors)

    for err := range errors {
        if err != nil {
            return err
        }
    }
    return nil
}
```

### Index Optimization

Ensure MongoDB indexes are created for common query patterns:

```javascript
// Index for MAC lookups
db.l2_fdb_entries.createIndex({ "mac_address": 1, "vlan_id": 1 })

// Index for confidence queries
db.l2_fdb_entries.createIndex({ "confidence_level": 1, "last_updated": -1 })

// Index for decision lookups
db.l2_decisions.createIndex({ "source_mac": 1, "dest_mac": 1, "vlan_id": 1 })
```

---

## Troubleshooting

### Issue: Low Confidence Scores

**Symptom**: FDB entries have low confidence (<40) despite correct configuration.

**Possible Causes**:
1. DHCP snooping disabled or not collecting bindings
2. 802.1X not configured or sessions not authenticated
3. Operational data outdated

**Solution**:
```go
// Enable debug logging
engine.SetDebugMode(true)

// Check evidence sources
result := engine.DetermineL2Reachability(query)
for _, evidence := range result.Evidence {
    fmt.Printf("Evidence: %s, Confidence: %d\n",
        evidence.Type, evidence.ConfidenceContribution)
}
```

### Issue: All Decisions Show "Blocked"

**Symptom**: All L2 decisions return "Blocked" outcome.

**Possible Causes**:
1. MAC table not loaded
2. VLANs misconfigured
3. Strict mode enabled but DHCP/auth missing

**Solution**:
```go
// Verify MAC table loaded
if len(engine.GetMACTable()) == 0 {
    panic("MAC table not loaded!")
}

// Try non-strict mode
engine.SetStrictMode(false)
```

### Issue: Decisions Not Persisting

**Symptom**: Decisions not saved to MongoDB.

**Solution**:
```go
// Verify MongoDB connection
client, err := mongo.Connect(context.Background(), options.Client().ApplyURI(mongoURI))
if err != nil {
    panic(err)
}

// Verify indexes created
err = storage.InitializeL2Schema(client, dbName)
if err != nil {
    panic(err)
}
```

---

## Next Steps

1. Review the [L2 Operations Guide](../operations/l2_operations_guide.md) for day-to-day usage
2. Review the [L2 Decision Engine API Reference](../api/l2_decision_engine_api.md) for detailed API documentation
3. Run integration tests: `go test ./platform/vendors/cisco/ios/... -v`
4. Deploy to production with monitoring and alerting configured

# L2 Requirements Gap Analysis

**Date**: 2025-10-26  
**Purpose**: Cross-reference `l2_project.md` requirements against multi-vendor support planning  
**Status**: 🔴 GAPS IDENTIFIED - ACTION REQUIRED

---

## Executive Summary

The multi-vendor support task plan has **significant gaps** for L2-specific requirements. While the plan covers basic operational commands (MAC tables, VLANs, interfaces), it **misses critical L2 security control operational data** and **L2-specific parsers** needed for the reference tool Layer-2 MVP.

---

## ✅ What's Covered (Good Alignment)

### 1. Basic L2 Topology Discovery
| Requirement | Coverage Status | Notes |
|-------------|----------------|-------|
| MAC address tables | ✅ COVERED | All platforms have MAC/FDB commands |
| VLAN cataloging | ✅ COVERED | `show vlan` equivalents across vendors |
| Interface status | ✅ COVERED | Interface operational state commands |
| LLDP/CDP neighbors | ✅ COVERED | Topology discovery commands present |
| Port-channel/LAG | ✅ COVERED | LAG membership commands defined |
| Spanning Tree | ✅ COVERED | STP status commands included |
| ARP tables | ✅ COVERED | ARP commands for IP-to-MAC correlation |

### 2. Config Collection
| Requirement | Coverage Status | Notes |
|-------------|----------------|-------|
| Config retrieval (API) | ✅ COVERED | Phase 1 addresses FMC, Panorama, FortiManager, NCM |
| Config retrieval (SSH) | ✅ COVERED | SSH connectors already working |
| Switchport config parsing | ⚠️ PARTIAL | Cisco IOS only; other vendors TBD |

---

## 🔴 Critical Gaps (High Priority)

### Gap 1: L2 Security Control Operational Commands

The `l2_project.md` requires operational data for these **P0 Critical** security controls, but they are **MISSING** from `AUX_COMMANDS_SPECIFICATION.md`:

| Control | Required Command | Platform | Gap | Impact |
|---------|------------------|----------|-----|--------|
| **DHCP Snooping Bindings** | `show ip dhcp snooping binding` | Cisco IOS/NXOS | ❌ MISSING | Cannot validate host placement or compute confidence scores |
| **DAI Statistics** | `show ip arp inspection statistics` | Cisco IOS/NXOS | ❌ MISSING | Cannot verify DAI effectiveness |
| **802.1X/MAB Sessions** | `show authentication sessions` | Cisco IOS/NXOS | ❌ MISSING | Cannot determine auth state (Allowed vs. Conditional) |
| **Port Security Status** | `show port-security` | Cisco IOS/NXOS | ❌ MISSING | Cannot credit MAC limiting controls |
| **IP Source Guard** | `show ip verify source` | Cisco IOS/NXOS | ❌ MISSING | Cannot validate IPSG bindings |
| **PVLAN Mappings** | `show vlan private-vlan` | Cisco IOS/NXOS | ❌ MISSING | Cannot model PVLAN isolation (P1) |

**Why This Matters**:
- L2 project requires **Allowed/Conditional/Blocked** decisions based on security posture
- Without these operational commands, decisions will always be "Conditional" (unknown state)
- DHCP snooping bindings are the **PRIMARY** source for host-to-port confidence scoring
- 802.1X session state determines whether endpoints are authenticated

**Recommendation**: Add these 6 commands to `AUX_COMMANDS_SPECIFICATION.md` as **P0 Critical** for Cisco IOS, ASA, NXOS.

---

### Gap 2: L2-Specific Parsers Not in Task Plan

The multi-vendor task plan defines **operational parsers** for each platform, but doesn't explicitly call out **L2 security control parsers**. The `l2_project.md` requires:

| Parser Required | Current Status | Gap |
|-----------------|---------------|-----|
| Parse DHCP Snooping bindings | ❌ NOT IN TASK PLAN | Need `ParseDHCPSnoopingBindings()` |
| Parse 802.1X sessions | ❌ NOT IN TASK PLAN | Need `ParseAuthenticationSessions()` |
| Parse Port Security | ❌ NOT IN TASK PLAN | Need `ParsePortSecurity()` |
| Parse IPSG bindings | ❌ NOT IN TASK PLAN | Need `ParseIPVerifySource()` |
| Parse PVLAN mappings | ❌ NOT IN TASK PLAN | Need `ParsePrivateVLAN()` |
| Parse VACL/PACL | ⚠️ PARTIAL | Mentioned in l2_project as P1 (Phase 1.5) |

**Impact**: Without these parsers, the **L2 decision engine cannot function** as specified in `l2_project.md` §8.

**Recommendation**: Add explicit parser tasks to Phase 1 (Cisco IOS) and Phase 2 (NXOS) for L2 security controls.

---

### Gap 3: L2-Specific Models Not Detailed

The `l2_project.md` defines detailed L2 models in §5 (Data Contract), but the task plan only mentions "models" generically:

| Model Required (from l2_project.md) | Task Plan Coverage | Gap |
|-------------------------------------|-------------------|-----|
| `CiscoL2Config` | ⚠️ MENTIONED | Not detailed; needs DHCP/DAI/IPSG fields |
| `CiscoL2VLAN` with VACL | ⚠️ MENTIONED | VACL reference field missing? |
| `CiscoL2Interface` with security flags | ⚠️ MENTIONED | 15+ security fields from §5.3 |
| `CiscoPortChannel` | ⚠️ MENTIONED | VLAN propagation fields |
| `CiscoGlobalL2Controls` | ❌ NOT MENTIONED | Global DHCP/DAI/IPSG/dot1x toggles |
| `CiscoSVI` | ⚠️ MENTIONED | L3 handoff markers |
| `CiscoFDBEntry` with confidence | ❌ NOT DETAILED | Needs confidence, freshness, source |
| `L2Switch` (topology) | ⚠️ MENTIONED | Mongo schema not defined |
| `L2SwitchPort` (topology) | ⚠️ MENTIONED | Mongo schema not defined |
| `L2PortChannel` (topology) | ⚠️ MENTIONED | Mongo schema not defined |
| `FDBEntry` (topology) | ⚠️ MENTIONED | Mongo schema not defined |
| `L2Decision` (cache) | ❌ NOT IN TASK PLAN | Deferred to "Phase 2" |

**Impact**: Without detailed model definitions, parsers cannot be implemented correctly.

**Recommendation**: Create a specific task in Phase 1 to define all L2 models per `l2_project.md` §5 before starting parser work.

---

### Gap 4: Evidence Hashing (Snippet Hashing)

The `l2_project.md` requires **evidence hashing** (§6) for every stanza:
- `source_block`: stable identifier (e.g., `interface Gi1/0/12`)
- `source_snippet_hash`: SHA-256 of normalized stanza text

**Current Task Plan Coverage**: ❌ **NOT MENTIONED**

This is a **P0 requirement** from the L2 project because it enables:
- Drift detection
- Config provenance
- Deterministic evidence tracking
- Audit trail for decisions

**Impact**: Without snippet hashing, the L2 MVP fails the **determinism** acceptance criterion (§15).

**Recommendation**: Add an explicit task to Phase 1 to implement stanza extraction and hashing for Cisco IOS config parser. Apply to all subsequent vendor parsers.

---

### Gap 5: FDB Ingestion Pipeline

The `l2_project.md` dedicates significant attention to **FDB ingestion** (§3, §7, §10.3) with:
- CLI collection (`show mac address-table dynamic`)
- SNMP collection (BRIDGE/Q-BRIDGE MIB)
- API enrichment (Claroty, Nozomi)
- Freshness tagging (fresh ≤ 10 min, stale 10-60 min, old > 60 min)
- Confidence scoring (FDB > controller > Claroty)
- Bulk persistence to `l2_fdb_entries` Mongo collection

**Current Task Plan Coverage**: ⚠️ **VAGUE** - "Operational parsers" mention MAC tables, but no explicit FDB **ingestion pipeline** or **confidence scoring** tasks.

**Impact**: L2 decisions require host placement confidence. Without this, all decisions degrade to "Conditional."

**Recommendation**: Add explicit tasks for:
1. FDB CLI collector (Cisco IOS, NXOS, etc.)
2. FDB SNMP collector (SNMPv3)
3. Freshness tagging logic
4. Confidence scoring algorithm
5. Bulk Mongo persistence
6. Claroty API integration (secondary evidence)

---

### Gap 6: L2 Decision Engine (Phase 2 / Not in Multi-Vendor Plan)

The `l2_project.md` defines a **decision engine** (§8) that produces:
- `Allowed | Conditional | Blocked` with reason codes
- 6 deterministic rules (placement, PVLAN, 802.1X, DAI/IPSG, path feasibility, handoff)
- `L2Decision` cache for what-if analysis

**Current Task Plan Coverage**: ❌ **NOT IN MULTI-VENDOR TASK PLAN**

The task plan focuses on **data collection** (connectors, parsers, models) but does **NOT** address:
- Decision logic implementation
- Reason code generation
- What-if simulation engine
- L2Decision Mongo collection

**Impact**: This is acceptable **IF** the multi-vendor task plan is scoped to "data collection only" and decision engine is a separate workstream. However, this should be **explicitly stated**.

**Recommendation**: 
- Add a Phase 8 (or separate project) for "L2 Decision Engine Implementation"
- OR update planning docs to clarify that multi-vendor support is "data collection only" and decision engine is out of scope

---

## 🟡 Medium Priority Gaps

### Gap 7: Switch Support Matrix (L2 Security Controls by Vendor)

The `l2_project.md` includes a **switch support matrix** (§13) showing which L2 controls each vendor supports. This is critical for:
- Parser capability flags
- Control-credit logic
- Downgrade-to-Conditional when controls are unsupported

**Current Task Plan Coverage**: ❌ **NOT MENTIONED**

**Recommendation**: Create a `VENDOR_L2_CAPABILITY_MATRIX.md` document that maps:
- DHCP Snooping support by vendor
- DAI support by vendor
- IPSG support by vendor
- 802.1X/MAB support by vendor
- PVLAN support by vendor
- Port Security support by vendor

---

### Gap 8: Graph Enrichment (Asset Placement)

The `l2_project.md` requires **graph enrichment** (§10.4):
- Add `L2AssetPlacement` to `AssetNode` (switch, port, VLAN, security flags)
- Add VLAN metadata to `NetworkNode`
- API exposure for "click to expand" UI

**Current Task Plan Coverage**: ⚠️ **VAGUE** - Mentioned as "Graph integration" but no specific tasks.

**Recommendation**: Add explicit tasks to Phase 7 (Testing & Documentation) or a new Phase 8:
1. Extend `AssetNode` with `l2_placement` field
2. Populate `l2_placement` from FDB entries
3. Add VLAN metadata to `NetworkNode`
4. Update GraphQL/REST API schemas
5. UI component for L2 context expansion

---

### Gap 9: MITRE ATT&CK Mapping (L2 Controls)

The `l2_project.md` includes a detailed **MITRE ATT&CK relationship table** (§14) showing:
- Which controls mitigate which TTPs
- How L2 controls affect attack paths (T1557 AiTM, T1021 Lateral Movement, T1200 Hardware Additions)

**Current Task Plan Coverage**: ❌ **NOT MENTIONED**

This is more of a **documentation/product** gap than implementation, but it should be captured.

**Recommendation**: Add to Phase 7 documentation:
- Create `L2_CONTROLS_MITRE_MAPPING.md`
- Document how each L2 control affects ATT&CK TTPs
- Feed into decision engine reason codes

---

### Gap 10: Golden Scenario Library (Test Suite)

The `l2_project.md` requires a **golden scenario library** (§12 user story) with:
- Curated L2 scenarios (VLAN isolation, PVLAN, DHCP Snooping, DAI, 802.1X)
- Expected decisions and reason codes
- CI integration

**Current Task Plan Coverage**: ⚠️ **PARTIAL** - Phase 7 mentions "Integration Testing" but not L2-specific scenarios.

**Recommendation**: Add to Phase 7:
- Create L2 golden scenario suite
- Define expected outcomes per scenario
- Integrate into CI pipeline

---

## 🟢 Low Priority Gaps (Future Work)

These are mentioned in `l2_project.md` but are explicitly **P1** (Phase 1.5+) or **P2** (future):

| Gap | Priority in l2_project.md | Notes |
|-----|--------------------------|-------|
| NAC Enrichment (ISE/ClearPass) | P1 (Phase 1.5+) | Secondary; can defer |
| VACL/PACL Rule Parsing | P1 (Phase 1.5+) | Names only in MVP |
| MACsec Annotation | P1 | Metadata only; no decision impact |
| Multicast/Protocol Hints (GOOSE/SMV) | P1 | OT-specific; can defer |
| Virtual-Wire Firewall Modeling | P1 | Palo Alto/FortiGate specific |
| What-If Evaluation Engine | P1 (Phase 2) | Deferred |
| Batch Evaluation Service | P1 | Deferred |
| Evidence Export Service | P1 | Deferred |

These are appropriately deferred and do not need to be in the current multi-vendor task plan.

---

## Reconciliation Summary

### What Needs to be Added to Multi-Vendor Task Plan

**Phase 1 (Weeks 1-2) - ADD:**
1. **Task 1.1.6**: Define L2 models per `l2_project.md` §5 (8 hours)
2. **Task 1.2.6**: Implement evidence hashing (snippet hashing) for Cisco IOS (6 hours)
3. **Task 1.2.7**: Add 6 L2 security control commands to Cisco IOS aux commands (2 hours)
4. **Task 1.2.8**: Implement 5 L2 security control parsers (DHCP bindings, 802.1X, port-sec, IPSG, PVLAN) (14 hours)

**Phase 2 (Weeks 3-4) - ADD:**
5. **Task 2.1.6**: Add 6 L2 security control commands to Cisco NXOS aux commands (2 hours)
6. **Task 2.1.7**: Implement 5 L2 security control parsers for NXOS (14 hours)
7. **Task 2.3.1**: Build FDB ingestion pipeline (CLI + SNMP collectors) (16 hours)
8. **Task 2.3.2**: Implement freshness tagging and confidence scoring (8 hours)
9. **Task 2.3.3**: Integrate Claroty API for secondary FDB evidence (12 hours)

**Phase 7 (Weeks 15-16) - ADD:**
10. **Task 7.1.4**: L2-specific golden scenario testing (12 hours)
11. **Task 7.2.4**: Create L2 controls capability matrix documentation (4 hours)
12. **Task 7.2.5**: Document MITRE ATT&CK mapping for L2 controls (4 hours)

**New Phase 8 (Weeks 17-20) - ADD (IF IN SCOPE):**
13. **Sprint 8.1**: L2 Decision Engine Implementation (80 hours)
    - Implement 6 deterministic rules
    - Create `L2Decision` cache
    - Implement reason code generation
    - Build what-if simulation API

**Total Additional Effort**: 
- **Phases 1-7**: +102 hours (≈ 2.5 weeks)
- **Phase 8** (if in scope): +80 hours (≈ 2 weeks)

---

## Recommendations

### Option A: Integrate L2 Requirements into Multi-Vendor Plan (Recommended)
- Update `AUX_COMMANDS_SPECIFICATION.md` to add 6 L2 security control commands
- Update `TASK_PLAN_MULTI_VENDOR_SUPPORT.md` to add 12 new tasks (102 hours)
- Extend timeline to **18 weeks** (from 16 weeks)
- Increase budget by ~$12,000 for additional 102 hours

### Option B: Separate L2 Security Controls into Dedicated Project
- Keep multi-vendor plan focused on "basic topology" (MAC, VLAN, interfaces)
- Create separate "L2 Security Controls Project" for DHCP/DAI/IPSG/802.1X
- Timeline: 8 weeks after multi-vendor completion
- Budget: ~$40,000 (240 hours)

### Option C: L2 Security Controls as Vendor-Specific Add-Ons
- Implement L2 security control parsers **ONLY** for Cisco IOS/NXOS (Phases 1-2)
- Defer L2 security controls for other vendors (Palo Alto, Fortinet, Juniper) to future work
- Timeline: +2 weeks for Cisco-only L2 controls
- Budget: +$10,000

---

## Action Items

1. [ ] **Decision**: Choose Option A, B, or C based on project priorities
2. [ ] **Update `AUX_COMMANDS_SPECIFICATION.md`**: Add 6 L2 security control commands (Cisco IOS, NXOS, ASA)
3. [ ] **Update `TASK_PLAN_MULTI_VENDOR_SUPPORT.md`**: Add 12 new tasks for L2 requirements
4. [ ] **Create `L2_MODELS_SPECIFICATION.md`**: Detailed spec for all L2 models per `l2_project.md` §5
5. [ ] **Create `VENDOR_L2_CAPABILITY_MATRIX.md`**: Document which vendors support which L2 controls
6. [ ] **Create `L2_DECISION_ENGINE_SPECIFICATION.md`**: IF in scope, detailed spec for decision engine
7. [ ] **Revise budget and timeline**: Based on chosen option

---

## Conclusion

The multi-vendor support task plan is **excellent** for basic L2 topology (MAC tables, VLANs, interfaces, neighbors) but has **critical gaps** for the **L2 security control operational data** required by `l2_project.md`.

**Primary Concern**: Without DHCP snooping bindings, 802.1X session state, and other L2 security operational commands, the L2 decision engine **cannot function** as designed. All decisions will degrade to "Conditional" (unknown state).

**Recommended Next Step**: Choose **Option A** (integrate L2 requirements) or **Option C** (Cisco-only L2 controls) and update the task plan accordingly.

---

**Status**: 🔴 GAPS IDENTIFIED - AWAITING DECISION  
**Document Version**: 1.0  
**Last Updated**: 2025-10-26


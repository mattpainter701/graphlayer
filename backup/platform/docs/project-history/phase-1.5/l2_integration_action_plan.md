# L2 Requirements Integration - Action Plan

**Date**: 2025-10-26  
**Status**: 🟢 READY FOR EXECUTION  
**Purpose**: Integrate L2 project requirements into multi-vendor support plan

---

## Executive Decision Required

Based on the gap analysis, we need to choose an integration strategy:

### ✅ Recommended: Option A - Full Integration (Cisco IOS & NXOS)

**Scope**: Add L2 security control support to Cisco IOS and NXOS connectors  
**Timeline**: +2 weeks (18 weeks total, from 16)  
**Budget**: +$12,240 (102 additional hours)  
**Impact**: 🟢 Enables full L2 decision engine for Cisco platforms

**Rationale**: 
- Cisco platforms are the primary target per `l2_project.md`
- L2 security controls (DHCP snooping, 802.1X, DAI, IPSG) are critical for MVP
- Other vendors (Palo Alto, Fortinet) can follow in subsequent phases

---

## What Changed

### 1. Updated `AUX_COMMANDS_SPECIFICATION.md`
✅ **COMPLETE** - Added 13 new L2 security control commands:

| Platform | Commands Added | Total Commands |
|----------|---------------|----------------|
| Cisco IOS | +6 P0 (DHCP/DAI/802.1X/port-sec/IPSG) + 1 P1 (PVLAN) | 14 → 20 |
| Cisco NXOS | +6 P0 (same as IOS) + 1 P1 (PVLAN) | 17 → 24 |

**New Commands**:
1. `show ip dhcp snooping binding` - P0, confidence scoring
2. `show ip arp inspection statistics` - P0, DAI state
3. `show authentication sessions` - P0, **decision-critical** for Allowed vs. Conditional
4. `show port-security` - P0, port security violations
5. `show port-security interface` - P0, per-interface details
6. `show ip verify source` - P0, IPSG bindings
7. `show vlan private-vlan` - P1, PVLAN isolation

---

## New Tasks to Add

### Phase 1 - Cisco IOS L2 Foundation (Week 1-2)

**NEW Task 1.0.1: Define L2 Models** (Before parsers)
- **Owner**: Backend Team
- **Effort**: 8 hours
- **Priority**: 🔴 CRITICAL
- **Deliverables**:
  - [ ] Create `L2_MODELS_SPECIFICATION.md` per `l2_project.md` §5
  - [ ] Define `CiscoL2Config` struct with all 15 interface security fields
  - [ ] Define `CiscoL2VLAN` with VACL references
  - [ ] Define `CiscoL2Interface` with security flags (dhcp_snooping_trust, arp_inspection_trust, ip_source_guard, dot1x_required, mab_enabled, unauth_vlan, etc.)
  - [ ] Define `CiscoPortChannel` with VLAN propagation
  - [ ] Define `CiscoGlobalL2Controls` (global DHCP/DAI/IPSG/dot1x toggles)
  - [ ] Define `CiscoSVI` for L3 handoff markers
  - [ ] Define `CiscoFDBEntry` with confidence/freshness/source
- **Files**: Create `sample_data/suggested_modeling/L2_MODELS_SPECIFICATION.md`

**NEW Task 1.0.2: Implement Evidence Hashing (Snippet Hashing)**
- **Owner**: Backend Team
- **Effort**: 6 hours
- **Priority**: 🔴 CRITICAL
- **Deliverables**:
  - [ ] Implement stanza extraction per `l2_project.md` §6
  - [ ] Implement normalization rules (strip comments, normalize indent, case-fold)
  - [ ] Implement SHA-256 hashing
  - [ ] Add `source_block` and `source_snippet_hash` to all parsers
  - [ ] Unit tests for hash determinism (same config = same hash)
- **Files**: `parsers/cisco/cisco.ios.parsers.go`
- **Acceptance**: Re-ingest same config → identical `source_snippet_hash` values

**UPDATED Task 1.2.1: Add IOS Command Definitions (EXPANDED)**
- **Original**: Add Cisco IOS aux commands
- **NEW**: Add 14 IOS commands (11 basic + 3 L2 security - existing) + 6 NEW L2 security
- **Effort**: 4 hours → 6 hours (+2 hours)
- **Deliverables**:
  - [ ] Add 6 P0 L2 security control commands to orchestrator
  - [ ] Add 1 P1 PVLAN command
  - [ ] Set appropriate timeouts and priorities
- **Files**: `orchestrator/connector_orchestrator.go`

**NEW Task 1.2.9: Implement L2 Security Control Parsers**
- **Owner**: Network Team
- **Effort**: 14 hours
- **Priority**: 🔴 CRITICAL
- **Deliverables**:
  - [ ] `ParseDHCPSnoopingBindings()` - parse `show ip dhcp snooping binding` → FDB confidence scoring
  - [ ] `ParseARPInspectionStats()` - parse `show ip arp inspection statistics` → DAI state
  - [ ] `ParseAuthenticationSessions()` - parse `show authentication sessions` → 802.1X/MAB session state
  - [ ] `ParsePortSecurity()` - parse `show port-security` → port security violations
  - [ ] `ParseIPVerifySource()` - parse `show ip verify source` → IPSG bindings
  - [ ] `ParsePrivateVLAN()` - parse `show vlan private-vlan` → PVLAN mappings (P1)
- **Files**: Create `parsers/cisco/ios.l2security.parsers.go`
- **Why Critical**: Without these, **ALL L2 decisions degrade to "Conditional"** (unknown state)

**NEW Task 1.2.10: Extend Cisco IOS Models with L2 Security Fields**
- **Owner**: Backend Team
- **Effort**: 4 hours
- **Priority**: 🔴 CRITICAL
- **Deliverables**:
  - [ ] Extend `CiscoL2Interface` with operational security state
  - [ ] Add `DHCPSnoopingBinding` struct
  - [ ] Add `AuthenticationSession` struct
  - [ ] Add `PortSecurityStatus` struct
  - [ ] Add `IPSGBinding` struct
  - [ ] Add `PVLANMapping` struct
- **Files**: `sample_data/suggested_modeling/completed_cisco.models.go`

---

### Phase 2 - Cisco NXOS L2 Support (Week 3-4)

**UPDATED Task 2.1.1: Add NXOS Command Definitions (EXPANDED)**
- **Original**: Add 17 NXOS commands
- **NEW**: Add 17 basic + 6 L2 security + 1 PVLAN = 24 total
- **Effort**: 4 hours → 6 hours (+2 hours)

**NEW Task 2.1.8: Implement NXOS L2 Security Control Parsers**
- **Owner**: Network Team
- **Effort**: 14 hours
- **Priority**: 🔴 CRITICAL
- **Deliverables**: Same 6 parsers as IOS, adapted for NXOS output format
- **Files**: Create `parsers/cisco/nxos.l2security.parsers.go`

**NEW Task 2.3: FDB Ingestion Pipeline**
- **Owner**: Network Team
- **Effort**: 36 hours (3 sub-tasks)
- **Priority**: 🔴 CRITICAL

**Task 2.3.1: Build FDB Collectors**
- **Effort**: 16 hours
- **Deliverables**:
  - [ ] CLI collector: Execute `show mac address-table dynamic` on Cisco IOS/NXOS
  - [ ] SNMP collector: Query BRIDGE-MIB and Q-BRIDGE-MIB (SNMPv3)
  - [ ] Parse MAC table outputs into `FDBEntry` structs
  - [ ] Bulk Mongo persistence to `l2_fdb_entries` collection
  - [ ] Unit tests with sample FDB outputs (100k entries)
- **Files**: Create `collectors/fdb_collector.go`

**Task 2.3.2: Implement Freshness & Confidence Scoring**
- **Effort**: 8 hours
- **Deliverables**:
  - [ ] Freshness tagging: fresh (≤10 min), stale (10-60 min), old (>60 min)
  - [ ] Confidence scoring algorithm: FDB=1.0, controller=0.8, Claroty=0.6
  - [ ] Source weighting: prefer DHCP bindings > MAC table > API
  - [ ] Emit confidence scores per placement
  - [ ] Downgrade to "Conditional" when confidence < threshold (configurable)
- **Files**: `collectors/fdb_collector.go`
- **Acceptance**: Per `l2_project.md` §9

**Task 2.3.3: Integrate Claroty API (Secondary Evidence)**
- **Effort**: 12 hours
- **Deliverables**:
  - [ ] Claroty CTD/xDome `/assets` API integration
  - [ ] Extract MAC/IP/vendor/model/site/zone
  - [ ] Enrich FDB entries with Claroty asset data (lower confidence)
  - [ ] Handle API rate limits and errors gracefully
  - [ ] Unit tests with Claroty sample data
- **Files**: Create `integrations/claroty/claroty_client.go`
- **Note**: Per `l2_project.md` §7.4, Claroty is P0 for MVP

---

### Phase 7 - Testing & Documentation (Week 15-16)

**NEW Task 7.1.4: L2 Golden Scenario Testing**
- **Owner**: QA Team
- **Effort**: 12 hours
- **Priority**: 🟡 HIGH
- **Deliverables**:
  - [ ] Create L2 scenario suite per `l2_project.md` §12
  - [ ] Scenario 1: VLAN isolation (same VLAN, no controls)
  - [ ] Scenario 2: PVLAN isolation (community/isolated ports)
  - [ ] Scenario 3: DHCP snooping + IPSG enforcement
  - [ ] Scenario 4: DAI blocking (untrusted port)
  - [ ] Scenario 5: 802.1X required, device unauthenticated
  - [ ] Scenario 6: Multiple controls (defense in depth)
  - [ ] For each scenario: define expected decision + reason codes
  - [ ] Integrate into CI pipeline
- **Files**: `tests/golden_scenarios/l2_scenarios.yaml`

**NEW Task 7.2.4: L2 Capability Matrix Documentation**
- **Owner**: Documentation Team
- **Effort**: 4 hours
- **Priority**: 🟡 HIGH
- **Deliverables**:
  - [ ] Create `VENDOR_L2_CAPABILITY_MATRIX.md`
  - [ ] Document DHCP Snooping support by vendor
  - [ ] Document DAI support by vendor
  - [ ] Document IPSG support by vendor
  - [ ] Document 802.1X/MAB support by vendor
  - [ ] Document PVLAN support by vendor
  - [ ] Document Port Security support by vendor
  - [ ] Map to `l2_project.md` §13 switch support matrix
- **Files**: Create `Connectors/VENDOR_L2_CAPABILITY_MATRIX.md`

**NEW Task 7.2.5: MITRE ATT&CK Mapping Documentation**
- **Owner**: Security Team
- **Effort**: 4 hours
- **Priority**: 🟢 MEDIUM
- **Deliverables**:
  - [ ] Create `L2_CONTROLS_MITRE_MAPPING.md`
  - [ ] Document how DHCP Snooping mitigates T1557.003 (DHCP Spoofing)
  - [ ] Document how DAI mitigates T1557.002 (ARP Cache Poisoning)
  - [ ] Document how IPSG supports T1557 mitigation
  - [ ] Document how 802.1X/MAB mitigates T1200 (Hardware Additions)
  - [ ] Document how Port Security limits T1557 setup
  - [ ] Document how PVLAN mitigates T1021 (Lateral Movement)
  - [ ] Map to `l2_project.md` §14 MITRE ATT&CK table
- **Files**: Create `Connectors/L2_CONTROLS_MITRE_MAPPING.md`

---

## Updated TODO List

### New TODOs to Add (15 tasks)

```yaml
# Phase 1 - L2 Foundation
phase1-l2-models:
  content: "Phase 1.0.1: Define L2 models per l2_project.md §5 (8h)"
  status: pending
  priority: critical

phase1-l2-hashing:
  content: "Phase 1.0.2: Implement evidence hashing (snippet hashing) for Cisco IOS (6h)"
  status: pending
  priority: critical

phase1-ios-l2-commands:
  content: "Phase 1.2.1 (UPDATED): Add 20 IOS commands including 6 L2 security controls (6h)"
  status: pending
  priority: critical
  
phase1-ios-l2-parsers:
  content: "Phase 1.2.9: Implement 6 L2 security control parsers (DHCP, DAI, 802.1X, port-sec, IPSG, PVLAN) (14h)"
  status: pending
  priority: critical

phase1-ios-l2-models:
  content: "Phase 1.2.10: Extend Cisco IOS models with L2 security operational state (4h)"
  status: pending
  priority: critical

# Phase 2 - NXOS L2
phase2-nxos-l2-commands:
  content: "Phase 2.1.1 (UPDATED): Add 24 NXOS commands including 6 L2 security controls (6h)"
  status: pending
  priority: critical

phase2-nxos-l2-parsers:
  content: "Phase 2.1.8: Implement 6 NXOS L2 security control parsers (14h)"
  status: pending
  priority: critical

phase2-fdb-collectors:
  content: "Phase 2.3.1: Build FDB CLI and SNMP collectors with bulk persistence (16h)"
  status: pending
  priority: critical

phase2-fdb-confidence:
  content: "Phase 2.3.2: Implement freshness tagging and confidence scoring (8h)"
  status: pending
  priority: critical

phase2-claroty-integration:
  content: "Phase 2.3.3: Integrate Claroty API for secondary FDB evidence (12h)"
  status: pending
  priority: high

# Phase 7 - L2 Testing & Docs
phase7-l2-scenarios:
  content: "Phase 7.1.4: Create L2 golden scenario test suite (12h)"
  status: pending
  priority: high

phase7-l2-capability-matrix:
  content: "Phase 7.2.4: Create L2 capability matrix documentation (4h)"
  status: pending
  priority: high

phase7-l2-mitre-mapping:
  content: "Phase 7.2.5: Document MITRE ATT&CK mapping for L2 controls (4h)"
  status: pending
  priority: medium
```

---

## Updated Effort & Timeline

### Original Plan (16 weeks)
- Phase 1-7: 680 hours (16 weeks @ 42.5 hrs/week)

### Updated Plan with L2 Requirements (18 weeks)

| Phase | Original | L2 Additions | New Total | Duration |
|-------|----------|--------------|-----------|----------|
| **Phase 1** | 80 hrs | +32 hrs (models 8 + hashing 6 + parsers 14 + models 4) | 112 hrs | 2.5 weeks |
| **Phase 2** | 64 hrs | +38 hrs (commands 2 + parsers 14 + FDB pipeline 36) | 102 hrs | 2.5 weeks |
| **Phase 3** | 108 hrs | 0 | 108 hrs | 3 weeks |
| **Phase 4** | 108 hrs | 0 | 108 hrs | 3 weeks |
| **Phase 5** | 96 hrs | 0 | 96 hrs | 2 weeks |
| **Phase 6** | 88 hrs | 0 | 88 hrs | 2 weeks |
| **Phase 7** | 72 hrs | +20 hrs (scenarios 12 + docs 8) | 92 hrs | 2.5 weeks |

**Total**: 708 hours → **798 hours** (+90 hours, +13%)  
**Timeline**: 16 weeks → **18 weeks** (+2 weeks)  
**Budget**: $164,500 → **$175,300** (+$10,800)

---

## Budget Breakdown (L2 Additions Only)

| Resource | Rate | Hours | Cost |
|----------|------|-------|------|
| Backend Engineer (models, hashing) | $120/hr | 18 hrs | $2,160 |
| Network Engineer (parsers, FDB) | $110/hr | 66 hrs | $7,260 |
| QA Engineer (L2 scenarios) | $90/hr | 12 hrs | $1,080 |
| Documentation (capability matrix, MITRE) | $80/hr | 8 hrs | $640 |

**Total L2 Cost**: $11,140

---

## Acceptance Criteria (L2-Specific)

From `l2_project.md` §15 - Definition of Done:

1. ✅ **Config-only graph**: Build per-VLAN components across Cisco IOS/NXOS
2. ✅ **FDB placement**: Ingest 100k FDB entries; resolve MAC→port with freshness
3. ✅ **Decision samples**: 20 curated pairs ⇒ expected Allowed/Conditional/Blocked with reason codes
4. ✅ **Determinism**: Re-ingest same configs ⇒ identical `source_snippet_hash` values
5. ✅ **Performance**: 25k ports + 150k FDB ⇒ end-to-end under 2 minutes

**New Quality Gates**:
- [ ] All L2 security control parsers have unit tests (>80% coverage)
- [ ] Golden scenario suite passes in CI (6 scenarios minimum)
- [ ] FDB confidence scoring produces expected scores (±0.05)
- [ ] DHCP snooping bindings correctly populate FDB entries
- [ ] 802.1X session state correctly drives Allowed vs. Conditional
- [ ] Evidence hashing produces deterministic results (hash stability test)

---

## Risk Mitigation

### Risk 1: Device Access for L2 Security Command Testing
- **Risk**: Lack of sample outputs for L2 security commands
- **Mitigation**: 
  - Use demo-data configs that include DHCP snooping, 802.1X
  - Request sample outputs from user's environment
  - Use Cisco DevNet sandbox for live testing
- **Backup**: Use best-effort parsing with error handling

### Risk 2: L2 Security Control Complexity
- **Risk**: Parsing 802.1X session state and DHCP bindings is non-trivial
- **Mitigation**: 
  - Break parsers into small, testable functions
  - Reference `prod_samples` Python parsers for examples
  - Start with simple cases, add edge cases iteratively
- **Backup**: Mark complex fields as "optional" (Phase 1.5)

### Risk 3: FDB Pipeline Performance
- **Risk**: 100k+ FDB entries may be slow to ingest
- **Mitigation**:
  - Use bulk MongoDB inserts (batch 1000 entries)
  - Implement connection pooling and parallel processing
  - Add performance benchmarks to CI
- **Backup**: Add pagination and streaming support

---

## Out of Scope (Phase 2 / Future Work)

These are mentioned in `l2_project.md` but explicitly deferred:

| Item | Priority | Deferred To |
|------|----------|-------------|
| L2 Decision Engine Implementation | Phase 2 | Q2 2026 (separate project) |
| What-If Evaluation Engine | P1 | After decision engine |
| VACL/PACL Rule Parsing | P1 | Phase 1.5 (Q2 2026) |
| MACsec Annotation | P1 | Q2 2026 |
| Multicast/Protocol Hints (GOOSE/SMV) | P1 | OT-specific work (Q2 2026) |
| Virtual-Wire Firewall Modeling | P1 | Palo Alto Phase 3+ |
| NAC Enrichment (ISE/ClearPass) | P1 | Phase 1.5 (Q2 2026) |
| L2 Security Controls for Other Vendors | P2 | Post-MVP (Q3 2026) |

---

## Next Steps

### Immediate Actions (This Week)
1. ✅ Review and approve this action plan
2. ⏭️ Update `TASK_PLAN_MULTI_VENDOR_SUPPORT.md` with new tasks
3. ⏭️ Create `L2_MODELS_SPECIFICATION.md` (Task 1.0.1)
4. ⏭️ Begin Phase 1.0.2 (evidence hashing implementation)

### Week 1 (Phase 1 Start)
5. ⏭️ Complete L2 model definitions
6. ⏭️ Implement evidence hashing with unit tests
7. ⏭️ Add 6 L2 security commands to Cisco IOS orchestrator
8. ⏭️ Begin L2 security control parsers

### Week 3 (Phase 2 Start)
9. ⏭️ Replicate L2 security parsers for NXOS
10. ⏭️ Build FDB ingestion pipeline (CLI + SNMP)
11. ⏭️ Implement confidence scoring algorithm
12. ⏭️ Integrate Claroty API

---

## Communication Plan

### Stakeholder Updates
- **Weekly**: Progress on L2 security parser implementation
- **Bi-weekly**: Demo FDB ingestion and confidence scoring
- **End of Phase 2**: Demo complete L2 data collection for Cisco platforms

### Documentation Deliverables
- **Week 1**: `L2_MODELS_SPECIFICATION.md`
- **Week 4**: `L2_SECURITY_PARSERS_GUIDE.md`
- **Week 16**: `VENDOR_L2_CAPABILITY_MATRIX.md` + `L2_CONTROLS_MITRE_MAPPING.md`

---

## Approval & Sign-Off

**L2 Integration Approved**: ☐ YES ☐ NO  
**Approved By**: _________________________  
**Date**: _________________________

**Budget Increase Approved** (+$11,140): ☐ YES ☐ NO  
**Approved By**: _________________________  
**Date**: _________________________

**Timeline Extension Approved** (+2 weeks): ☐ YES ☐ NO  
**Approved By**: _________________________  
**Date**: _________________________

---

**Status**: 🟢 READY FOR EXECUTION  
**Document Version**: 1.0  
**Last Updated**: 2025-10-26

**Next Action**: Update `TASK_PLAN_MULTI_VENDOR_SUPPORT.md` and create TODOs


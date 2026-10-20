# GraphLayer — Project Spec v2

## Purpose
Build a multi-vendor network topology and data-ingestion platform that collects configuration and operational data from many connectors, normalizes the results through vendor-specific parsers, and produces topology graphs that surpass `reference` in breadth, accuracy, confidence scoring, and practical analyst value.

## Product vision
GraphLayer should become the definitive place where raw network data turns into actionable, trustworthy topology. It should:
- ingest from more sources than any single-vendor tool
- parse more vendor formats with higher fidelity
- produce better topology graphs with evidence and confidence
- improve graph intelligence through deduplication and cross-source correlation
- outperform `reference` across coverage, fidelity, and analyst workflow value

## Current architecture
### Platform core (`platform/`)
- **Orchestration**: connector orchestrator, task engine, conditional execution, error handling
- **Types**: unified device types, topology record model
- **Registry**: factory pattern for vendor/connector registration
- **Storage**: file-based persistence
- **Integration**: parser bridge for unified ingestion

### Vendors (`platform/vendors/`)
- **Cisco**: IOS (config + operational + routing + L2 security), NX-OS, ASA (config + routing)
- **Palo Alto**: firewall (config + routing), Panorama
- **Fortinet**: FortiGate (config + routing), FortiManager (import service + client)
- **Juniper**: Junos (config + operational + L2)
- **Aruba**: AOS-CX (config + operational)

### Reference data (`platform/reference/`)
- **IANA**: protocol/service lookups with loader and constants
- **App-ID**: Palo Alto App-ID cache, Fortinet ISDB cache, merger, metadata, health, config

### Connectors (`platform/orchestration/connectors/`)
- SSH connector with tests
- FortiGate ISDB collector
- SMB/SSH file share connector (unified)

### Sample data
Vendor configs: Cisco, Palo Alto, Fortinet, Juniper
Asset/vuln data: RunZero, Claroty, ForeScout, Nozomi, Tenable

## Non-goals
- Not a single-vendor toy parser
- Not just a collection of sample configuration files
- Not a graph renderer with no real data model or provenance
- Not a rewrite that loses existing vendor coverage
- Not a replacement for monitoring/SNMP tools

---

## Sprint/Epic Roadmap (14 sprints)

### Sprint 1 — Topology Record Model v2
**Goal:** Define the universal topology schema that all connectors and parsers converge to.

**Tasks:**
- Define normalized topology record model: Device, Interface, Link, VLAN, Route, SecurityPolicy, NATRule, VPN
- Add evidence provenance to every record: source connector, parser version, raw input hash, confidence score, timestamp
- Define confidence scoring model: 0.0-1.0 with source weight (config > operational > inferred), corroboration bonus, staleness penalty
- Implement schema versioning and migration path
- Write JSON Schema for topology output
- Document the model with examples for every record type
- Add model validation: every parser must produce records that validate against schema
- Migrate existing parsers to produce the v2 model

**Acceptance:** A complete topology export includes device inventory, interface table, L2 adjacency, L3 routes, security policy, with evidence for every claim.

### Sprint 2 — Connector Expansion Roadmap & Normalization Contract
**Goal:** A systematic framework for adding new data sources.

**Tasks:**
- Define connector interface contract: Connect(), Collect(), Transform(), Close()
- Document connector normalization rules: what raw data maps to which topology record fields
- Build connector test harness: mock data source, verify transform output
- Prioritize connector backlog:
  - Tier 1: SolarWinds NCM (existing), SSH device collection (existing)
  - Tier 2: ServiceNow CMDB, NetBox, phpIPAM
  - Tier 3: Ansible inventory, Terraform state, cloud provider APIs (AWS VPC, Azure VNet, GCP)
  - Tier 4: Custom REST APIs, CSV imports
- Implement connector registration: discover and load connectors at runtime
- Write connector authoring guide with template

**Acceptance:** A developer can add a new connector by implementing 4 methods and registering it.

### Sprint 3 — Next Connector: ServiceNow CMDB
**Goal:** Ingest device inventory and relationships from ServiceNow.

**Tasks:**
- Implement ServiceNow REST API client (table API with pagination)
- Map CMDB CI classes to topology device types: cmdb_ci_server → server, cmdb_ci_netgear → switch/router/firewall based on model
- Extract relationships: cmdb_rel_ci for topology links
- Normalize: ServiceNow fields → GraphLayer topology records
- Add evidence: CMDB record sys_id, last_updated timestamp
- Implement incremental sync: only fetch records changed since last run
- Write integration tests with recorded API responses
- Document ServiceNow setup prerequisites

**Acceptance:** Running GraphLayer against a ServiceNow instance produces a device inventory with relationships.

### Sprint 4 — Next Connector: NetBox
**Goal:** Ingest network documentation from NetBox as topology source.

**Tasks:**
- Implement NetBox REST API client (pynetbox or direct HTTP)
- Map NetBox models: devices → topology devices, interfaces → interfaces, cables → links, IPAM → routes/subnets
- Extract device roles, sites, racks for hierarchical topology
- Normalize to GraphLayer topology records
- Handle NetBox custom fields as extensible metadata
- Add evidence: NetBox record ID, last_updated
- Write integration tests
- Add NetBox to connector registry

**Acceptance:** A documented network in NetBox produces a validated topology in GraphLayer.

### Sprint 5 — Vendor Parser: Arista EOS
**Goal:** Full Arista EOS support — config + operational parsing.

**Tasks:**
- Implement EOS config parsers: interfaces, VLANs, routing (OSPF, BGP, static), MLAG, VXLAN
- Implement EOS operational parsers: show lldp neighbors, show mac address-table, show ip arp, show ip route, show interfaces status
- Build EOS mappers: config records → topology model
- Implement EOS L2 security parsers (port-security, storm-control if applicable)
- Add MLAG-to-topology-link mapping logic
- Write comprehensive tests with sample EOS configs and show outputs
- Document EOS command requirements and supported versions

**Acceptance:** An EOS switch configuration produces accurate device, interface, and link records with L2 adjacency.

### Sprint 6 — Vendor Parser: Dell OS10/FTOS
**Goal:** Dell networking support with config + operational parsing.

**Tasks:**
- Implement OS10 config parsers: interfaces, VLANs, routing, VLT, port-channels
- Implement OS10 operational parsers: show lldp neighbors, show mac address-table, show ip arp, show ip route
- Build OS10 mappers to topology model
- Add VLT (Virtual Link Trunking) topology inference
- Implement FTOS backward compatibility layer for older Dell switches
- Write tests with sample OS10 and FTOS configs
- Document OS10 command requirements

**Acceptance:** Dell switches in a mixed-vendor topology are correctly represented alongside Cisco and Arista.

### Sprint 7 — Vendor Parser: Check Point & Additional Firewalls
**Goal:** Expand firewall coverage beyond Palo Alto and Fortinet.

**Tasks:**
- Implement Check Point Gaia/CLISH config parsers: interfaces, routes, security policy, NAT
- Implement Check Point operational parsers: show route, show arp, fw tab connections
- Add Check Point mappers to topology model
- Evaluate and plan for: pfSense/OPNsense, Sophos, WatchGuard
- Build security policy normalization: all firewalls → unified SecurityPolicy record
- Add rule-ordering awareness (rule position matters for firewall processing)
- Write tests with sample Check Point configs

**Acceptance:** A mixed-vendor security audit produces a unified policy view across Palo Alto, Fortinet, and Check Point.

### Sprint 8 — Cross-Source Deduplication & Correlation Engine
**Goal:** When two sources describe the same device or link, merge intelligently.

**Tasks:**
- Implement device deduplication: match by management IP, hostname, serial number, MAC address
- Build confidence-weighted merge: when two sources conflict, prefer the source with higher confidence
- Implement link correlation: match interfaces across different sources by neighbor info
- Add cross-source conflict detection: flag when two sources disagree (e.g., CMDB says switch X has 24 ports, config shows 48)
- Build correlation report: what matched, what didn't, what conflicted
- Implement manual override: allow users to specify which source wins for specific conflicts
- Add correlation rules engine: user-definable matching rules
- Write integration tests with multi-source scenarios

**Acceptance:** Running GraphLayer with 3 data sources (SSH configs, NetBox, ServiceNow) produces a deduplicated topology where each device appears once with merged evidence.

### Sprint 9 — Topology Confidence Scoring v2
**Goal:** Every device, interface, and link has a quantified confidence score that analysts can trust.

**Tasks:**
- Implement per-attribute confidence: each field has its own confidence (e.g., high confidence on hostname from config, medium on vendor from MAC OUI)
- Build confidence propagation: if device A has 0.9 confidence, its interfaces inherit 0.9 with decay
- Add corroboration multiplier: if two independent sources agree, boost confidence
- Implement staleness decay: records not refreshed in N days lose confidence
- Add confidence visualization hints for graph consumers
- Build confidence summary stats per topology export
- Document confidence model with examples
- Add —min-confidence flag to filter output

**Acceptance:** A topology export includes confidence scores. An analyst can filter to "high confidence only" and trust the results.

### Sprint 10 — reference tool Comparison Harness
**Goal:** Objective, automated comparison showing where GraphLayer beats reference tool.

**Tasks:**
- Build reference tool-compatible input format: translate GraphLayer topology to reference tool comparison format
- Implement comparison metrics:
  - Device count and accuracy (matched devices / total devices)
  - Link accuracy (matched links / total links)
  - Vendor coverage breadth (how many vendors correctly parsed)
  - Evidence quality (records with provenance / total records)
  - False positive rate (fabricated devices/links)
- Create comparison test suite with known-good reference topologies
- Build automated comparison report: tables, charts, per-vendor breakdown
- Run comparison on each release, include in release notes
- Document methodology so results are reproducible

**Acceptance:** Each release includes a comparison report quantifying improvement over reference tool.

### Sprint 11 — Graph Intelligence & Topology Inference
**Goal:** Fill in gaps — infer what isn't explicitly configured.

**Tasks:**
- Implement topology inference from routing tables: next-hop relationships imply physical/logical links
- Infer L2 topology from MAC tables + CDP/LLDP: complete missing switch-switch links
- Implement subnet discovery: group devices by common IP prefixes, infer gateway devices
- Build path inference: given device A and B, find intermediate hops from routing tables
- Add implicit link detection: two devices with matching subnet on interfaces → possible direct link
- Implement network boundary detection: where does the managed network end?
- Add NAT boundary inference: private IPs behind public IPs
- Write inference engine with rule priority and conflict resolution

**Acceptance:** On a network with partial data, GraphLayer infers more topology than explicit parsing alone would yield.

### Sprint 12 — Richer Graph Output & Analyst Views
**Goal:** The topology graph is useful for real analysis, not just pretty pictures.

**Tasks:**
- Implement graph export formats: GraphML, GEXF, Cytoscape JSON, D3 JSON
- Build vendor-colored graph output: color-code by vendor for quick visual scan
- Add confidence-weighted edge thickness
- Implement security policy overlay: firewall rules as graph annotations
- Build VLAN/segment grouping: collapse or expand by network segment
- Add traffic flow overlay: if NetFlow/sFlow data available, show flow volume on links
- Implement filterable views: show only routing devices, only access switches, only security devices
- Write graph consumer documentation for common tools (Gephi, Cytoscape, D3)

**Acceptance:** Exporting a topology as GraphML and opening it in a graph tool shows a meaningful, labeled network diagram.

### Sprint 13 — Orchestration Engine v2
**Goal:** Robust, parallelized ingestion with scheduling, retry, and monitoring.

**Tasks:**
- Implement parallel connector execution with configurable concurrency
- Add retry logic with exponential backoff per connector
- Build ingestion scheduling: cron-style recurring collections
- Implement incremental collection: only pull changed data
- Add collection health monitoring: connector success/failure rates, latency, data freshness
- Build ingestion pipeline visualization (what ran when, what succeeded/failed)
- Implement collection artifact storage: keep raw data for reprocessing
- Add notification hooks for collection failures

**Acceptance:** A scheduled run collects from 5+ connectors in parallel, retries failures, and reports status.

### Sprint 14 — Testing, Documentation & Ship Readiness
**Goal:** Production-grade quality and developer experience.

**Tasks:**
- Achieve >80% test coverage across all vendor packages (go test -cover)
- Add integration tests: full pipeline from connector → parser → topology → export
- Build fuzz testing for all parsers (malformed configs must not panic)
- Write comprehensive architecture docs: data flow, component diagram, extension guide
- Create vendor parser authoring guide with template and checklist
- Build Docker Compose demo: sample data → GraphLayer → export → visualize
- Add CI/CD: lint (golangci-lint), test, coverage report, build
- Performance benchmarks: parse rate (configs/sec), memory per 10K devices
- Write getting-started guide: 5-minute path from zero to first topology export

**Acceptance:** `go test ./...` passes cleanly. Docker Compose demo produces a topology graph from sample data in one command. A developer can add a new vendor parser in an afternoon.

---

## Strategic benchmark
`reference` is a benchmark, not the ceiling. GraphLayer should be better by being:
- more complete across vendors (Cisco, Palo Alto, Fortinet, Juniper, Aruba, Dell, Check Point, Arista vs reference tool's narrower coverage)
- better at topology inference and gap-filling
- better at evidence, provenance, and confidence scoring
- better at central orchestration and multi-source correlation
- better at practical analyst workflows with richer graph exports

## Architecture principles
- Favor normalization over one-off special cases
- Keep evidence and provenance visible on every record
- Treat topology inference as a first-class feature
- Preserve testability across vendor packages
- Make expansion easy without making the system brittle
- Confidence scores must be quantitative, not hand-wavy

## Development setup
- Go-based platform code under `platform/`
- Vendor packages under `platform/vendors/{vendor}/`
- Orchestration, models, integrations, reference data under `platform/`
- Tests alongside source files (`*_test.go`)
- Docker for integration testing and demo

## Validation
Use the Docker dev/test server for:
- Go test runs across affected packages
- Parser regression checks against sample configs
- Graph generation and topology sanity tests
- Full pipeline integration tests
- reference tool comparison runs
- Performance benchmarks

Validation should answer:
- did ingest still work across all connectors?
- did topology quality improve or regress?
- are graphs more accurate and more useful?
- does the new connector/parser fit the model cleanly?
- is evidence preserved end-to-end?

## Priorities
1. Solidify the topology record model and confidence framework
2. Expand connector coverage (ServiceNow, NetBox, cloud APIs)
3. Add vendor parsers (Arista, Dell, Check Point)
4. Build cross-source correlation and deduplication
5. Ship reference tool comparison harness with quantified superiority
6. Polish graph output and analyst workflows

## Success definition
GraphLayer is successful when it can ingest data from 8+ diverse sources, parse 8+ vendor formats, and produce topology graphs that are measurably more complete, more accurate, and more useful than reference tool — with evidence and confidence scores backing every claim.

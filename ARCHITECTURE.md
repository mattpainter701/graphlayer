# GraphLayer Architecture

## Overview

GraphLayer is a multi-vendor network topology graph engine that ingests network
device configurations and operational state from diverse sources, parses them
through vendor-specific adapters, correlates devices across sources, infers
topology links, and exports rich graph representations.

The required implementation language for parsers, connectors and domain
engines is Rust. See [Rust architecture](docs/RUST_ARCHITECTURE.md) and the
[worker contract](docs/RUST_WORKER.md). The ASA forwarding inventory parser
has moved to Rust; the Go/Python layers described below are legacy components
under a CI-enforced migration boundary, not a claim of a completed rewrite.

## Architecture Layers

```
┌─────────────────────────────────────────────────────────────┐
│                      EXPORT LAYER                            │
│  GraphML │ GEXF │ Cytoscape JSON │ D3 JSON │ reference Format  │
├─────────────────────────────────────────────────────────────┤
│                     GRAPH LAYER                              │
│  Builder │ Types │ Export Engine │ Color/Shape Hints        │
├─────────────────────────────────────────────────────────────┤
│                  INTELLIGENCE LAYER                          │
│  L3 Inference │ L2 Inference │ Subnet Discovery             │
│  Boundary Detection │ Implicit Links │ Rule Engine          │
├─────────────────────────────────────────────────────────────┤
│                 CORRELATION LAYER                            │
│  Dedup Engine │ Conflict Resolution │ Link Correlation      │
│  Transitive Closure │ Cross-Source Merging                  │
├─────────────────────────────────────────────────────────────┤
│                  SCORING LAYER                               │
│  Confidence Engine │ Propagation │ Corroboration            │
│  Staleness Decay │ Summary Stats │ Filtering               │
├─────────────────────────────────────────────────────────────┤
│               COMPARISON LAYER                               │
│  reference Comparison │ Metrics Engine │ Diff/Reports          │
├─────────────────────────────────────────────────────────────┤
│                  PARSER LAYER                                │
│  ┌──────────┬──────────┬──────────┬──────────┬──────────┐  │
│  │  Cisco   │  Arista  │  Dell    │  Juniper │  Palo    │  │
│  │ IOS/NXOS │   EOS    │  OS10    │  Junos   │  Alto    │  │
│  │   /ASA   │          │          │          │          │  │
│  ├──────────┼──────────┼──────────┼──────────┼──────────┤  │
│  │ Fortinet │  Aruba   │  Shared: Evidence Hashing,       │  │
│  │          │  AOS-CX  │  Suggestions                    │  │
│  └──────────┴──────────┴─────────────────────────────────┘  │
├─────────────────────────────────────────────────────────────┤
│                 CONNECTOR LAYER                              │
│  NetBox │ ServiceNow CMDB │ SolarWinds NCM │ Fileshare     │
│  SSH Collection │ API Collection │ Registry                │
├─────────────────────────────────────────────────────────────┤
│                  MODELS LAYER                                │
│  Network Topology Entities │ Graph Models │ L3 Models      │
│  Confidence Models │ Supplemental Types                     │
├─────────────────────────────────────────────────────────────┤
│               REFERENCE DATA LAYER                           │
│  IANA Constants │ App-ID Cache (PAN-OS, FortiGuard ISDB)   │
└─────────────────────────────────────────────────────────────┘
```

## Data Flow

### Ingest → Parse → Correlate → Score → Infer → Graph → Export

1. **Ingest**: Connectors pull raw configurations and operational state from
   network devices, CMDBs, and fileshares. Connectors implement a common
   interface (`BaseConnector` in Python, `Connector` interface in Go).

2. **Parse**: Vendor-specific parsers transform raw CLI/API output into
   normalized `models.Device` records. Each vendor has config parsers,
   operational parsers, and mappers that translate vendor-specific structures.

3. **Correlate**: The deduplication engine identifies the same device across
   multiple sources using serial numbers, MAC addresses, IP addresses, and
   hostname normalization. Transitive closure resolves chains of matches.

4. **Score**: The confidence engine assigns per-attribute confidence scores,
   propagates confidence through topology relationships, boosts corroborated
   attributes, and applies staleness decay to aging data.

5. **Infer**: The intelligence layer discovers implicit topology links (L3
   routing adjacency, L2 neighbor discovery, subnet membership, VLAN
   boundaries) and builds a connected graph.

6. **Graph**: The graph builder constructs a unified topology graph with
   vendor-specific color/shape hints, role annotations, and edge weighting.

7. **Export**: The graph is serialized to standard formats (GraphML, GEXF,
   Cytoscape JSON, D3 JSON) for visualization tools, and to reference format for
   comparison against reference topologies.

## Key Design Decisions

### Multi-Source Confidence
Each device attribute carries a confidence score derived from the evidence
source (config parsing = 0.95, connector = 0.80, operational = 0.70, inferred
= 0.50). When multiple sources agree, corroboration boosts confidence. Stale
data decays exponentially with configurable half-life.

### Vendor-Agnostic Core
The correlation, scoring, intelligence, graph, and export layers operate on
vendor-agnostic `models.Device` and `models.Link` types. Only the parser
layer is vendor-specific, using a plugin architecture where each vendor
implements config parsers, operational parsers, and mappers.

### Evidence Hashing
Raw configuration stanzas are hashed (SHA-256) for change detection. When a
device's configuration hasn't changed, reprocessing is skipped. Hash chains
provide audit trails for configuration drift analysis.

### Comparison Engine
The reference comparison harness lets operators validate GraphLayer output
against known-good reference topologies. Five metrics: device accuracy, link
accuracy, attribute accuracy, false positive rate, and false negative rate.

## Component Map

| Component | Location | Language |
|-----------|----------|----------|
| Connectors | `platform/connectors/` | Python |
| Connector Registry | `platform/connectors/registry.py` | Python |
| Core Interfaces | `platform/core/` | Go |
| Models | `platform/models/` | Go |
| Parsers | `platform/vendors/*/` | Go |
| Correlation | `platform/correlation/` | Go |
| Scoring | `platform/scoring/` | Go |
| Intelligence | `platform/intelligence/` | Go |
| Graph | `platform/graph/` | Go |
| Comparison | `platform/comparison/` | Go |
| Orchestration | `platform/orchestration/` | Go |
| Reference Data | `platform/reference/` | Go |
| Docs | `platform/docs/` | Markdown |

## Testing Strategy

- **Unit tests**: Per-package `*_test.go` files with table-driven tests
- **Integration tests**: End-to-end connector→parser→correlate→infer→graph→export
- **Fuzz tests**: Randomized config input to parsers to detect panics
- **Benchmarks**: Go benchmark functions for parse throughput and graph build time
- **Comparison harness**: reference diff against golden reference topologies

## Next Steps

See [GETTING_STARTED.md](GETTING_STARTED.md) for setup and
[CONNECTOR_AUTHORING_GUIDE.md](CONNECTOR_AUTHORING_GUIDE.md) for adding
vendor support.

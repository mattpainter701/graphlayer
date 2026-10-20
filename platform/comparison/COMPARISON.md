# GraphLayer reference tool Comparison Harness (Sprint 10)

## Overview

The comparison harness provides objective, automated comparison between GraphLayer and reference tool topology outputs. It normalizes both outputs to a common format, computes quantified metrics across five dimensions, and generates markdown comparison reports suitable for release notes.

## Quick Start

```bash
# Self-test using built-in multi-vendor reference topology
cd platform
go run ./cmd/graphlayer-compare --self-test

# Compare two reference-format JSON files
go run ./cmd/graphlayer-compare \
  --graphlayer=graphlayer_export.json \
  --reference=reference_export.json \
  --report=comparison_report.md

# Use a reference topology as baseline
go run ./cmd/graphlayer-compare \
  --ref=testdata/cisco-3-switch-l2.json \
  --reference=reference_export.json
```

## Architecture

```
platform/comparison/
├── reference_format.go          # reference-compatible format & GraphLayer translation
├── metrics.go                # Comparison engine + all 5 metric groups
├── report.go                 # Markdown report generation
├── reference.go              # Reference topology management
├── comparison_engine_test.go # 16 tests (all passing)
└── testdata/
    └── cisco-3-switch-l2.json # Sample reference topology
```

## Comparison Metrics

### 1. Device Count & Accuracy
- **Matched devices**: Found by both tools
- **Extra devices**: GraphLayer-only (potential added value or false positives)
- **Missing devices**: reference tool-only (parsing gaps)
- **Accuracy**: matched / total_unique_devices
- **Precision**: matched / graphlayer_total
- **Recall**: matched / reference tool_total
- **F1 Score**: harmonic mean of precision and recall

### 2. Link Accuracy
- Same metrics as devices, applied to topology links
- Separate L2 and L3 link breakdown
- Fuzzy port-name matching handles vendor naming differences (e.g., "Gi1/0/1" ↔ "GigabitEthernet1/0/1")

### 3. Vendor Coverage Breadth
- Count of unique vendor families covered by each tool
- Extra vendors (GraphLayer covers, reference tool doesn't)
- Missing vendors (reference tool covers, GraphLayer doesn't)
- Vendor breadth score

### 4. Evidence Quality
- Average evidence count per device
- Evidence ratio (devices with provenance / total)
- Average confidence score
- Evidence quality score (GL ratio / reference tool ratio)

### 5. False Positive Rate
- Unmatched devices/links counted as potential false positives
- FP rate = potential FPs / total
- Low-confidence unmatched tracking

## Overall Score

Weighted composite (0.0–1.0):
- Device F1: 30%
- Link F1: 25%
- Vendor breadth: 15%
- Evidence quality: 20%
- FP inversion: 10%

## Device Matching

Devices are matched by ID in priority order:
1. Serial number (`serial:<value>`)
2. Management IP (`ip:<value>`)
3. Hostname (`hostname:<value>`)
4. Field hash (fallback)

Fuzzy fallback: If exact ID fails, matches by management IP, then hostname.

## Link Matching

Links are matched by endpoint device IDs and port names. Fuzzy port matching handles:
- Short names: "Gi1/0/1" ↔ "GigabitEthernet1/0/1"
- Short names: "Et1" ↔ "Ethernet1"
- Direction swapping: A→B matches B→A

## Reference Topologies

Two built-in reference topologies for reproducible testing:

1. **Small 3-device**: Cisco + Arista switches + Palo Alto firewall, one L2 link
2. **Multi-vendor 7-device**: Cisco NX-OS, Arista EOS, Dell OS10, Palo Alto, Fortinet, Juniper, Aruba — 5 links

Load from JSON:
```go
ref, err := comparison.LoadReferenceTopology("testdata/cisco-3-switch-l2.json")
```

## reference tool-Compatible Format

The interchange format normalizes both tools to a common schema:

```json
{
  "metadata": {
    "source": "graphlayer",
    "version": "v2.0.0",
    "generated_at": "2026-05-16T00:00:00Z",
    "total_devices": 7,
    "total_links": 5
  },
  "devices": [
    {
      "id": "hostname:core-sw-01",
      "hostname": "core-sw-01",
      "vendor": "cisco",
      "family": "NX-OS",
      "model": "N9K-C93180YC-FX",
      "management_ip": "10.1.0.1",
      "device_type": "switch",
      "confidence": 0.95,
      "evidence_count": 3,
      "evidence_sources": ["config", "operational", "netbox"]
    }
  ],
  "links": [
    {
      "id": "l2link:core-sw|Eth1/1|dist-sw|Eth1",
      "from_device": "hostname:core-sw-01",
      "from_port": "Eth1/1",
      "to_device": "hostname:dist-sw-01",
      "to_port": "Eth1",
      "link_type": "l2",
      "protocol": "lldp",
      "confidence": 0.90
    }
  ]
}
```

## Translation: GraphLayer → reference tool Format

```go
import "github.com/mattpainter701/graphlayer/platform/comparison"

// Convert GraphLayer assets to reference devices
devices := comparison.TranslateAssetsToReference(assets)

// Convert GraphLayer links to reference links
links := comparison.TranslateLinksToReference(graphLinks, l2Links)
```

## Per-Release Workflow

1. Export GraphLayer topology to reference tool-format JSON
2. Export reference tool topology (or use reference)
3. Run: `go run ./cmd/graphlayer-compare --graphlayer=gl.json --reference=np.json --report=release_comparison.md`
4. Include report in release notes
5. Archive report alongside release artifacts

## Reproducibility

- All reference topologies are stored as version-controlled JSON
- Device and link matching is deterministic (hash-based IDs, exact string matching with fuzzy fallback)
- The methodology is documented in this file
- Reports include a methodology section explaining scoring weights
- Use `--self-test` to verify the harness works correctly on any machine

## Test Suite

16 tests covering:
- reference tool format round-trip
- Identical topology comparison (100% match)
- Extra/missing device detection
- Link fuzzy matching (abbreviated port names)
- Vendor coverage counting
- Evidence quality metrics
- False positive analysis
- Overall score computation
- Report generation (all sections present)
- Per-vendor breakdown
- Reference topology save/load
- Device ID generation priority
- GraphLayer→reference tool translation
- Empty topology handling
- Device fuzzy matching (IP, hostname)

Run: `go test ./comparison/... -v`

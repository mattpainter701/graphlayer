# GraphLayer Getting Started Guide

GraphLayer ingests network device configurations and builds an interactive
multi-vendor topology graph with confidence scoring and graph export.

## Prerequisites

- **Go 1.24+** (for the platform components)
- **Python 3.10+** (for connectors)
- **Docker & Docker Compose** (for the demo environment)

## Quick Start (5 minutes)

### 1. Clone and build

```bash
git clone https://github.com/mattpainter701/graphlayer.git
cd graphlayer/platform
go build ./...
```

### 2. Run the tests

```bash
TMPDIR=/tmp go test ./correlation/... ./comparison/... ./graph/... \
  ./intelligence/... ./scoring/... ./vendors/shared/...
```

### 3. Run the comparison demo

```bash
go run ./cmd/graphlayer-compare/main.go \
  --reference platform/comparison/testdata/cisco-3-switch-l2.json \
  --export-format graphml
```

## Docker Compose Demo

The Docker Compose environment provides a full GraphLayer pipeline:

```bash
docker compose up -d
```

This starts:
- **graphlayer-core**: The Go platform (API server on :8080)
- **graphlayer-connectors**: Python connector service (on :8081)
- **graphlayer-reference tool**: reference tool-compatible comparison endpoint (on :8082)

### Ingest a sample topology

```bash
# Via the API
curl -X POST http://localhost:8080/ingest \
  -H "Content-Type: application/json" \
  -d @platform/comparison/testdata/cisco-3-switch-l2.json
```

### Export the graph

```bash
# GraphML (for yEd, Gephi)
curl http://localhost:8080/graph/graphml > topology.graphml

# D3 JSON (for web visualization)
curl http://localhost:8080/graph/d3 > topology.json
```

## Platform Package Map

| Package | Purpose | Key Types |
|---------|---------|-----------|
| `models/` | Shared data types | `Device`, `Link`, `ConfidenceProfile` |
| `correlation/` | Cross-source dedup | `CorrelationEngine`, `DedupEngine` |
| `scoring/` | Confidence scoring | `ConfidenceEngine` |
| `intelligence/` | Topology inference | `InferenceEngine`, L2/L3 rules |
| `graph/` | Graph construction | `GraphBuilder`, export formats |
| `comparison/` | reference tool diff engine | `ComparisonEngine`, metrics |
| `vendors/` | Vendor parsers | Cisco, Arista, Dell, Juniper, Palo Alto, Fortinet, Aruba |
| `connectors/` | Python connectors | NetBox, ServiceNow, SolarWinds, Fileshare |
| `reference/` | IANA/App-ID data | Protocol numbers, service ports |
| `orchestration/` | Collection orchestration | Connector orchestration, decision engines |

## Adding a Device

### Via API (Python connector)

```python
import requests

device = {
    "hostname": "core-sw-01",
    "vendor": "cisco",
    "model": "IOS-XE",
    "interfaces": [
        {"name": "GigabitEthernet0/1", "ip_address": "10.0.0.1/24"}
    ]
}
requests.post("http://localhost:8081/devices", json=device)
```

### Via CLI (direct parse)

```bash
# Parse a Cisco IOS running-config
go run ./cmd/graphlayer-parse/main.go \
  --vendor cisco --os ios \
  --input configs/core-sw-01.cfg \
  --output core-sw-01.topology.json
```

## Running Benchmarks

```bash
go test -bench=. -benchmem ./correlation/... ./scoring/... ./graph/...
```

## CI/CD

GitHub Actions workflows run on every push:
- **test.yml**: Unit tests + coverage (Go 1.24)
- **lint.yml**: golangci-lint
- **bench.yml**: Benchmark comparison against `main`

## Configuration

### Confidence Tuning

Edit `platform/models/confidence_model.go` or pass via API:

```json
{
  "propagation": {
    "decay_factor": 0.8,
    "max_hops": 5
  },
  "corroboration": {
    "min_sources": 2,
    "boost_factor": 0.15
  },
  "staleness": {
    "half_life_hours": 168,
    "model": "exponential"
  }
}
```

## Next Steps

- Read [ARCHITECTURE.md](ARCHITECTURE.md) for the full design
- Read [CONNECTOR_AUTHORING_GUIDE.md](CONNECTOR_AUTHORING_GUIDE.md) to add vendor support
- See `platform/docs/` for detailed integration and operations guides

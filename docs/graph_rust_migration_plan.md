# GraphLayer Rust Graph Engine Migration Plan

## Goal

Move the graphing hot path toward a Rust implementation without rewriting the
entire platform. The Rust scope should stay focused on graph construction,
validation, algorithms, and export serialization.

## Why Rust for graphing

- Strong typing for graph IDs, node/edge kinds, optional VLANs, and export
  formats.
- Predictable performance for large topology traversal and export workloads.
- Memory safety for dense graph processing.
- A clean boundary for consolidating graph logic currently split between Go
  graph export code and Python graph algorithms.

## Non-goals

- Do not rewrite Python connectors as part of the graph migration.
- Do not rewrite all vendor parsers up front.
- Do not start with FFI; prefer a CLI/JSON boundary first.

## Proposed boundary

Producers emit a canonical topology JSON document:

```json
{
  "schema_version": "0.1.0",
  "devices": [],
  "assets": [],
  "networks": [],
  "links": [],
  "l2_links": [],
  "policies": []
}
```

The Rust graph engine consumes that document and emits a validated graph:

```json
{
  "schema_version": "0.1.0",
  "nodes": [],
  "edges": [],
  "metadata": {}
}
```

## Phased execution

1. Stabilize the current graph/test baseline.
2. Define JSON schemas for normalized topology input and graph output.
3. Add a small Rust CLI proof of concept that reads topology JSON and exports
   D3 JSON or GraphML.
4. Compare Rust output with existing Go graph output using golden fixtures.
5. Switch the export path only after parity tests pass.

## First Rust proof-of-concept crate

Recommended crate layout:

```text
graphlayer-core-rs/
├── Cargo.toml
└── crates/
    ├── graphlayer-models/
    ├── graphlayer-graph/
    ├── graphlayer-export/
    └── graphlayer-cli/
```

Recommended initial dependencies:

```toml
serde = { version = "1", features = ["derive"] }
serde_json = "1"
thiserror = "1"
petgraph = "0.6"
clap = { version = "4", features = ["derive"] }
quick-xml = "0.36"
```

## Immediate remediation targets

- Keep non-benchmark Python tests green by separating benchmark tests from the
  default test command.
- Keep focused Go graph/reference checks green while broader experimental
  packages are repaired.
- Fix graph-builder handling of partial topology records before porting logic
  to Rust.

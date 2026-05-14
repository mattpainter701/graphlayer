# GraphLayer — Project Spec

## Purpose
Build a multi-vendor network topology and data-ingestion platform that collects configuration and operational data from many connectors, normalizes the results, and produces topology graphs that are better than `reference` in breadth, accuracy, and usefulness.

## Product vision
GraphLayer should become the place where network data turns into actionable topology.
It should:
- ingest more sources
- parse more vendor formats
- produce better topology graphs
- improve graph intelligence over time
- outperform `reference` in coverage, fidelity, and practical value

## Current direction
Continue building this out by expanding:
- connector coverage
- parser coverage
- graphing / topology fidelity
- vendor-specific enrichment
- orchestration and normalization quality

## Non-goals
- Not a single-vendor toy parser
- Not just a collection of sample files
- Not a graph renderer with no real data model
- Not a rewrite that loses existing vendor coverage or operational value

## Core capabilities
### 1. Connectors
Support pulling data from more systems and protocols, including but not limited to:
- SSH device collection
- API-based vendor management platforms
- file-share and export sources
- inventory / reference systems
- future custom connectors where justified

### 2. Parsers
Expand the number and quality of parsers for:
- vendor configs
- operational state
- L2 / L3 topology clues
- security policy and application context
- evidence / provenance data

### 3. Graphing and topology
Improve the ability to:
- build accurate device and link graphs
- represent multi-hop and multi-source relationships
- annotate confidence and evidence
- normalize vendor-specific data into a consistent model
- support richer topology views than current alternatives

### 4. Reference / enrichment data
Continue improving reference data and normalization so graph output becomes more useful and trustworthy.

## Strategic benchmark
`reference` is a benchmark, not the ceiling.
GraphLayer should be better by being:
- more complete across vendors
- better at topology inference
- better at evidence/provenance
- better at central orchestration
- better at practical analyst workflows

## Milestones
### Milestone A — Broaden ingest coverage
- Add more connectors / sources
- Reduce manual preprocessing
- Make ingestion easier to extend
- Keep connector interfaces consistent

### Milestone B — Expand parser coverage
- Add new vendor parsers and command handlers
- Improve existing parser quality and edge cases
- Make vendor support easier to maintain
- Increase test coverage for representative outputs

### Milestone C — Improve graph intelligence
- Strengthen graph building rules
- Improve node/link confidence scoring
- Improve topology inference and deduplication
- Support better cross-source correlation

### Milestone D — Topology UX and usefulness
- Improve graph readability
- Improve output structure for downstream tools
- Support richer analyst views and exports
- Make the platform easier to trust in real investigations

## Acceptance criteria
A release is healthy when:
- new data sources can be added without breaking the model
- new vendor parsers fit the existing architecture cleanly
- graph output is accurate enough to be useful without excessive manual cleanup
- evidence and provenance are preserved
- the topology output is genuinely better than `reference` for the target use cases

## Development principles
- Favor normalization over one-off special cases
- Keep evidence and provenance visible
- Treat topology inference as a first-class feature
- Preserve testability across vendor packages
- Make expansion easy without making the system brittle

## Development setup
- Go-based platform code
- vendor packages under `platform/vendors/`
- orchestration, models, integrations, and reference data under `platform/`
- tests for vendor parsers and mapping logic

## Validation
Use the Docker dev/test server for:
- Go test runs across affected packages
- parser regression checks
- graph generation / topology sanity tests
- sample-data processing pipelines
- comparison runs against previous outputs and `reference`

Validation should answer:
- did ingest still work across vendors?
- did topology quality improve or regress?
- are graphs more accurate and more useful?
- does the new connector/parser fit the model cleanly?

## Priorities
1. Add more connectors and parsers
2. Improve graphing and topology intelligence
3. Preserve evidence and provenance
4. Keep the architecture maintainable as coverage grows
5. Win against `reference` on practical analyst value

## Success definition
This project is successful when GraphLayer can ingest more network data, represent topology more accurately, and produce more useful graphs than the alternatives users compare it against.

# Overhaul audit and recovery record

Audit date: October 17, 2026. Starting main: `859d6c447f706b54544f3eaafc4d6109d8067fcd`. Evidence references below are relative to that revision unless labelled local prototype. Line numbers refer to the starting revision and may move after implementation.

## Committed baseline findings

| Finding | Evidence | Interpretation |
|---|---|---|
| Complete in-memory export graph, no application service | `platform/graph/builder.go:230`; `platform/graph/types.go:130` | Useful engine foundation; not yet a bounded operator query service |
| Filtered nodes returned alongside all edges | `platform/graph/builder.go:245` | Graph projection can retain endpoints outside filtered set; regression test needed |
| Grouping uses formatted text child IDs and retains children | `platform/graph/builder.go:289` | Not an expandable/collapsed hierarchy |
| Lifecycle confidence fixed to 0.5 | `platform/graph/builder.go:389` | Visual confidence cannot be interpreted as live source-quality evidence |
| Zone IDs omit virtual context | `platform/graph/builder.go:445`; `platform/models/network_topology_models.go:107` | Same zone name on same firewall in distinct contexts can collide |
| Policy overlay is a rule-derived edge, not evaluated reachability | `platform/graph/builder.go:167` | Matrix/topology must not imply a concrete flow verdict |
| Vendor dispatch takes first populated policy slice | `platform/orchestration/decision/l4_firewall_decision_engine.go:133` | Shared mixed-device evaluation requires a new scope boundary |
| Named address/service evaluation incomplete | `platform/orchestration/decision/l4_firewall_decision_engine.go:381` | Unsupported predicates can pass unchecked; unsafe to expose as confident permission |
| Toolchain disagreement | `platform/go.mod:3`; `.github/workflows/test.yml:15`; `Dockerfile.core:4` | Go 1.25 module versus 1.24 setup/images; implicit download obscures build requirements |
| Missing LFS configuration object | `.gitattributes:5`; `pytest.ini` pointer | Normal checkout attempted missing object `2ccfa38...`; skip-smudge checkout required for recovery |
| IANA embed paths inconsistent | `platform/reference/iana/iana_loader.go:16,85,290` | Paths outside embed root cannot load; catalogs incomplete/fallback behavior needs proof |
| Three IANA CSVs appear dirty on new checkout | blanket CSV LFS filter vs checked-in header content | Git filter artifact, not user edits; preserved during audit |
| Comparator lacks advertised server/bench flags | `platform/cmd/graphlayer-compare/main.go:23`; `Dockerfile.core:21`; `.github/workflows/bench.yml:32` | Existing smoke comparison is a CLI self-test, not a service or performance regression gate |
| Connector container targets missing module and invalid COPY syntax | `Dockerfile.connectors:4`; `docker-compose.yml:22` | Do not describe compose as a working production app |
| CI only selected root packages | `.github/workflows/test.yml:19`; nested `platform/orchestration/go.mod` | Nested module and vendor semantics need independent gates |

## Initial executed checks

An isolated Go 1.25.14 runtime was downloaded from the official Go distribution and verified using its published SHA-256. This is a task-local tool outside the repository, not a machine-wide installation.

Initial command in `platform/`: `go test ./graph/... ./correlation/... ./comparison/... ./intelligence/... ./scoring/... ./models/... ./reference/iana/...`.

Passed: graph, correlation, comparison, intelligence. Models had no test files. Failed: confidence propagation/corroboration/staleness/summary/filter cases in scoring; unknown protocol and IPv4/ARP/IPv6 EtherType lookup cases in IANA. These are measured starting failures, not assumed defects in every subsystem. Later repair evidence belongs in the implementation handoff/PR.

A broader root `go test ./...` also exposed pre-existing vendor build failures: missing required core/testify modules, Check Point import/unused-variable errors, unresolved Arista/Dell model types, duplicate Juniper helpers/model drift, and unresolved Panorama types. The nested orchestration `go test ./decision/...` fails module resolution before evaluation tests can compile. A read-only type audit additionally found decision/parser field drift. These are explicit F03/I01 blockers for integrating real vendor parsers; the new pure evaluator does not silently route around them and claim vendor integration.

Initial Python check could not run because the bundled Python had no pytest. A separate task virtual environment was prepared. With a temporary external configuration, 56 tests passed and 17 benchmark tests could not set up because the benchmark plugin was absent; Windows fixture-server thread warnings also occurred. The LFS-pointer `pytest.ini` and declared test dependencies must be repaired before the standard repository command can be meaningful. No full-system deployment, real device collection, competitor head-to-head test or browser usability study was performed during discovery.

## Preserved older local prototype

`F:\deepseek\graphlayer`, local `main` at `34c08093fc1dbc2bee702aee48b33652f2aea695`, contains broad modified parser/orchestration/model files plus an untracked UI/API setup. It was not reset, cleaned, merged, deleted or used for feature edits.

Read-only agents found an unfinished Next.js/React Flow UI with inconsistent API prefixes, topology/upload/devices calls absent from the running route table, and a canned allowed result from path analysis. The Go prototype includes possible compile mismatches between parse service, collection result and model metadata. Static findings require reproduction in an owned recovery branch before repair.

The prototype also has unprotected API/admin collection routes and direct credential writes in its filesystem version. Those are reasons to avoid promoting that prototype unchanged. They do not prove a remotely exposed vulnerability in current committed main, which contains no such application server.

Recovery procedure: identify owner; inventory scope without printing secrets; preserve a recoverable copy/branch under explicit ownership; compare each behavior to current main; port only useful coherent behavior with tests; leave original dirty work untouched until its owner approves retirement. No old UI dependency versions, database choices or mock flows are automatically adopted.

## Older topology-record branch

`git cherry origin/main origin/sprint1/topology-record-model-v2` found one patch-unique commit `c4c0999c23437ac1a6bafcab05966cd3215239cf`. Focused diff adds five files: schema/docs, v2 records, adapters and tests. Its valuable record/evidence validation is not integrated into main. Existing confidence types overlap, and the adapter's unknown/zero-quality handling needs review. Preserve branch as a reference; recover selectively after the canonical-model ADR. Unique commit count alone does not justify a PR.

## Discovery scope and limits

Separate delegated reviews covered backend, local frontend, fresh-baseline UX, security/ops, competition, policy correctness and confidence determinism. Agent assertions were checked against repository reads or test output where practical. Remaining claims are identified as proposals/risks. Neither documentation nor a green self-comparison establishes enterprise readiness or superiority.

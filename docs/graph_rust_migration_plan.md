# Rust domain migration

GraphLayer's domain implementation is moving to Rust. This plan replaces the
older graph-only proposal: the target includes parsers, connectors, graph and
topology construction, policy, route and NAT evaluation, network analysis,
validation, inference, and performance-sensitive algorithms.

The root Cargo workspace contains the `graphlayer-engine` application crate
and the reusable domain crates under `crates/`. Rust owns domain rules and
decisions. Go HTTP, storage, and CLI compatibility code may remain as adapters
while callers migrate. Browser code may manage view state and render results.
A migrated slice must not call the old Go or Python engine for its domain work.

## Migration slices

1. **Shipped investigation workflows.** Rust owns ASA/PAN-OS/FortiOS canonical
   parsing, evidence projection, validation, policy, routing/NAT, return paths
   and recursive multi-firewall exploration. Public Go entry points call the
   worker; the replaced domain implementations are removed.
2. **Connector acquisition.** Rust owns NetBox, ServiceNow, SolarWinds, native
   SFTP/SMB and operational SSH/API sessions, command parsing and App-ID/ISDB
   reference collection. Python and Go expose compatibility APIs over the same
   required worker.
3. **Preserve release contracts.** Require the 1,764 captured released parser and
   investigation cases, the 72 earlier forwarding cases, genuine workspace-v1
   through v7 replay, process-failure checks, browser workflows and actual
   connector service fixtures. Changing implementation language must not change
   saved identities, evidence or history bytes.
4. **Frozen experimental libraries.** Older vendor operational parsers, graph
   analytics and orchestration prototypes outside the shipped investigation
   runtime remain explicit migration work. Port each behavior and its tests
   before enabling it; do not add a Rust facade around legacy algorithms.
5. **Reduce compatibility overhead.** Keep Go transport, storage and CLI code
   limited to compatibility work. Measure process overhead and bounded large
   inputs before claiming performance improvements or introducing a pool.

The [Rust architecture](RUST_ARCHITECTURE.md) describes the language boundary.
[`AGENTS.md`](../AGENTS.md) makes it repository policy, and
[`config/rust-boundary.json`](../config/rust-boundary.json) plus
[`scripts/check-rust-boundary.py`](../scripts/check-rust-boundary.py) enforce a
freeze on the remaining legacy domain files. The guard allows deletions and
accepts only exact-path exceptions that pin the approved SHA256 and name an
owner, reason, and expiry within 90 days.
CI runs the guard.

Fixtures must be synthetic or sanitized and must not contain credentials, raw
customer configurations, or unredacted logs.

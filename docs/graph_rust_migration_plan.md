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

1. **Establish the boundary with Cisco ASA forwarding inventory parsing.** Implement
   interface/route/NAT parsing in Rust, preserve its contract, and compare sanitized
   golden fixtures. This is the first port, not complete Rust runtime support.
2. **Move remaining domain behavior.** Port the other Go vendor parsers,
   graph/topology and policy/routing/NAT analysis, then every Python connector.
   Keep each slice independently reviewable with focused fixtures and parity
   checks.
3. **Retire legacy implementations.** Switch the owning caller to Rust, delete
   the old implementation, and remove any temporary exception. Update the
   frozen inventory only with the reviewed port.
4. **Reduce compatibility code.** Keep Go transport, storage, and CLI code
   limited to compatibility work as consumers move to the Rust engine.

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
